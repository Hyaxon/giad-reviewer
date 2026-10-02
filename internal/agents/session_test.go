package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyaxon/agentic-review/internal/model"
	"github.com/hyaxon/agentic-review/internal/tools"
	"github.com/hyaxon/agentic-review/pkg/protocol"
)

// The test executable doubles as a language-independent protocol peer.
func TestAgentProcess(t *testing.T) {
	mode := ""
	for i, arg := range os.Args {
		if arg == "--agent-fixture" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
	}
	if mode == "" {
		return
	}
	if os.Getenv("GH_TOKEN") != "" || os.Getenv("HOME") != "" {
		os.Exit(10)
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	if !scanner.Scan() {
		os.Exit(11)
	}
	var start protocol.Frame
	if json.Unmarshal(scanner.Bytes(), &start) != nil || start.Method != "review.start" {
		os.Exit(12)
	}
	request := func(id, method string, params any) protocol.Frame {
		data, _ := json.Marshal(params)
		_ = encoder.Encode(protocol.Frame{APIVersion: protocol.Version, ID: id, Method: method, Params: data})
		if !scanner.Scan() {
			os.Exit(13)
		}
		var reply protocol.Frame
		if json.Unmarshal(scanner.Bytes(), &reply) != nil {
			os.Exit(14)
		}
		return reply
	}
	switch mode {
	case "malformed":
		fmt.Println("this is not JSON")
		os.Exit(0)
	case "premature":
		os.Exit(0)
	case "hang":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	case "models":
		for i, profile := range []string{"review", "review", "adversarial"} {
			reply := request(fmt.Sprint(i), "model.chat", map[string]any{"profile": profile, "messages": []any{}, "tools": []any{}})
			if reply.Error != "" {
				os.Exit(15)
			}
		}
	case "denied":
		reply := request("denied", "tests.run", map[string]any{"command": "go test"})
		if !strings.Contains(reply.Error, "denied") {
			os.Exit(16)
		}
	}
	reply := request("read", "repository.read", map[string]any{"path": "example.go", "start": 1, "end": 0})
	if reply.Error != "" {
		os.Exit(17)
	}
	line := 1
	if mode == "bad-anchor" {
		line = 99
	}
	report := protocol.Report{Summary: "Draft", Findings: []protocol.Finding{{Source: "fixture", Category: "correctness", File: "example.go", Line: line, Severity: "high", Confidence: .9, Title: "Defect", Explanation: "Failure", Evidence: "Read line 1", FailureScenario: "Trigger", SuggestedFix: "Fix"}}}
	reply = request("finish", "review.finish", report)
	if mode == "bad-anchor" {
		if reply.Error == "" {
			os.Exit(18)
		}
		os.Exit(0)
	}
	if reply.Error != "" {
		os.Exit(19)
	}
	os.Exit(0)
}

type fakeProvider struct{ events []string }

func (f *fakeProvider) Chat(_ context.Context, tag string, _ []model.Message, _ []model.Tool) (model.Message, error) {
	f.events = append(f.events, "chat:"+tag)
	return model.Message{Role: "assistant", Content: "Continue"}, nil
}
func (f *fakeProvider) Unload(ctx context.Context, tag string) error {
	f.events = append(f.events, "unload:"+tag)
	return ctx.Err()
}

func fixtureSession(t *testing.T, mode string) Session {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte("bug here\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := tools.Open(dir, "diff")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Session{Manifest: protocol.Manifest{APIVersion: protocol.Version, Name: "fixture", Version: "1", Entrypoint: protocol.Entrypoint{Command: executable, Args: []string{"-test.run=^TestAgentProcess$", "--", "--agent-fixture", mode}}},
		Job: protocol.Job{ChangedFiles: []protocol.ChangedFile{{Path: "example.go", Status: "modified"}}, AllowedCapabilities: []string{"repository.read", "model.chat"}}, Repository: reader}
}
func TestExternalAgentSession(t *testing.T) {
	t.Setenv("GH_TOKEN", "not-for-agent")
	for _, mode := range []string{"success", "denied", "bad-anchor", "malformed", "premature", "hang", "models"} {
		t.Run(mode, func(t *testing.T) {
			s := fixtureSession(t, mode)
			provider := &fakeProvider{}
			if mode == "models" {
				s.Manifest.ModelProfiles = []string{"review", "adversarial"}
				s.Profiles = map[string]Profile{"review": {Provider: provider, Model: "one"}, "adversarial": {Provider: provider, Model: "two"}}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if mode == "hang" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			report, err := s.Run(ctx)
			if mode == "success" || mode == "denied" || mode == "models" {
				if err != nil || len(report.Findings) != 1 {
					t.Fatalf("report=%+v err=%v", report, err)
				}
				if mode == "denied" && report.Limitations == "" {
					t.Fatal("denial must mark incomplete coverage")
				}
			} else if err == nil {
				t.Fatal("invalid or unfinished review must fail")
			}
			if mode == "models" && strings.Join(provider.events, ",") != "chat:one,chat:one,unload:one,chat:two,unload:two" {
				t.Fatalf("lifecycle: %v", provider.events)
			}
		})
	}
}
func TestRequiredAndOptionalCapabilities(t *testing.T) {
	m := protocol.Manifest{Capabilities: protocol.Capabilities{Required: []string{"repository.read"}, Optional: []string{"tests.run", "repository.search"}}}
	if _, err := Grants(m, nil); err == nil {
		t.Fatal("missing required grant must refuse launch")
	}
	grants, err := Grants(m, []string{"repository.read", "repository.search", "tests.run"})
	if err != nil || strings.Join(grants, ",") != "repository.read,repository.search" {
		t.Fatalf("grants=%v err=%v", grants, err)
	}
}

func TestHostFrameBudget(t *testing.T) {
	var output strings.Builder
	if err := (frameWriter{&output}).Encode(protocol.Frame{APIVersion: protocol.Version, ID: "1", Result: json.RawMessage(`{"ok":true}`)}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(output.String(), "\n") {
		t.Fatal("frame must be newline-delimited")
	}
	output.Reset()
	if err := (frameWriter{&output}).Encode(protocol.Frame{APIVersion: protocol.Version, Error: strings.Repeat("x", protocol.MaxMessageBytes)}); err == nil || output.Len() != 0 {
		t.Fatal("oversized host frame must fail before writing")
	}
}

type failingUnload struct{ fakeProvider }

func (f *failingUnload) Unload(context.Context, string) error { return fmt.Errorf("unload failed") }

func TestFailedUnloadPreventsProfileSwitch(t *testing.T) {
	first := &failingUnload{}
	second := &fakeProvider{}
	b := broker{session: Session{Manifest: protocol.Manifest{ModelProfiles: []string{"one", "two"}}, Profiles: map[string]Profile{"one": {Provider: first, Model: "first"}, "two": {Provider: second, Model: "second"}}}, active: "one"}
	params := json.RawMessage(`{"profile":"two","messages":[],"tools":[]}`)
	for i := 0; i < 2; i++ {
		if _, err := b.call(context.Background(), "model.chat", params); err == nil {
			t.Fatal("failed unload must prevent loading another model")
		}
	}
	if b.active != "one" || len(second.events) != 0 {
		t.Fatal("failed unload lost lifecycle ownership")
	}
}
