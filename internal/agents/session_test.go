package agents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyaxon/giad/internal/model"
	"github.com/hyaxon/giad/internal/sandbox"
	"github.com/hyaxon/giad/internal/tools"
	"github.com/hyaxon/giad/pkg/protocol"
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
	case "old-version":
		_ = encoder.Encode(protocol.Frame{APIVersion: "agentic-review/v1", ID: "legacy", Method: "git.diff", Params: json.RawMessage(`{}`)})
		os.Exit(0)
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
	return Session{Launcher: sandbox.TrustedHostLauncher{}, Manifest: protocol.Manifest{APIVersion: protocol.Version, Name: "fixture", Version: "1", Entrypoint: protocol.Entrypoint{Command: executable, Args: []string{"-test.run=^TestAgentProcess$", "--", "--agent-fixture", mode}}},
		Job: protocol.Job{ChangedFiles: []protocol.ChangedFile{{Path: "example.go", Status: "modified"}}, AllowedCapabilities: []string{"repository.read", "model.chat"}}, Repository: reader}
}
func TestExternalAgentSession(t *testing.T) {
	t.Setenv("GH_TOKEN", "not-for-agent")
	for _, mode := range []string{"success", "denied", "bad-anchor", "malformed", "premature", "hang", "models", "old-version"} {
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

type launchFunc func(context.Context, sandbox.Command) (sandbox.Process, error)

type blockingProvider struct{ entered bool }

func (p *blockingProvider) Chat(ctx context.Context, _ string, _ []model.Message, _ []model.Tool) (model.Message, error) {
	p.entered = true
	<-ctx.Done()
	return model.Message{}, ctx.Err()
}
func (*blockingProvider) Unload(context.Context, string) error { return nil }

func TestModelTimeoutPreservesDeadline(t *testing.T) {
	s := fixtureSession(t, "models")
	provider := &blockingProvider{}
	s.Manifest.ModelProfiles = []string{"review"}
	s.Profiles = map[string]Profile{"review": {Provider: provider, Model: "fixture"}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := s.Run(ctx)
	if !provider.entered || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed-out model call must report its deadline, entered=%v err=%v", provider.entered, err)
	}
}

func (f launchFunc) Launch(ctx context.Context, command sandbox.Command) (sandbox.Process, error) {
	return f(ctx, command)
}

func TestSessionRequiresLauncher(t *testing.T) {
	s := fixtureSession(t, "success")
	s.Launcher = nil
	if _, err := s.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "explicitly selected launcher") {
		t.Fatalf("missing launcher must fail before execution, got %v", err)
	}
}

func TestLaunchFailureDoesNotFallBack(t *testing.T) {
	s := fixtureSession(t, "success")
	launchErr := errors.New("isolation unavailable")
	s.Launcher = launchFunc(func(context.Context, sandbox.Command) (sandbox.Process, error) {
		return nil, launchErr
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := s.Run(ctx); !errors.Is(err, launchErr) {
		t.Fatalf("launcher failure must remain a failure, got %v", err)
	}
}

type cleanupFailureProcess struct {
	input  bytes.Buffer
	output io.Reader
	err    error
	closed bool
}

func (p *cleanupFailureProcess) Stdin() io.Writer  { return &p.input }
func (p *cleanupFailureProcess) Stdout() io.Reader { return p.output }
func (p *cleanupFailureProcess) Close() error {
	p.closed = true
	return p.err
}

func TestLauncherCleanupFailureFailsSession(t *testing.T) {
	s := fixtureSession(t, "success")
	cleanupErr := errors.New("could not release agent resources")
	p := &cleanupFailureProcess{err: cleanupErr, output: strings.NewReader(`{"apiVersion":"giad/v1","id":"finish","method":"review.finish","params":{"summary":"Draft","limitations":"","findings":[]}}` + "\n")}
	s.Launcher = launchFunc(func(context.Context, sandbox.Command) (sandbox.Process, error) {
		return p, nil
	})
	if report, err := s.Run(context.Background()); report.Summary != "Draft" || !errors.Is(err, cleanupErr) || !p.closed {
		t.Fatalf("accepted report must not hide failed cleanup: report=%+v err=%v closed=%v", report, err, p.closed)
	}
}

func TestRequiredAndOptionalCapabilities(t *testing.T) {
	m := protocol.Manifest{Capabilities: protocol.Capabilities{Required: []string{"repository.read"}, Optional: []string{"unsupported.tool", "tests.run", "repository.search"}}}
	if _, err := Grants(m, nil); err == nil {
		t.Fatal("missing required grant must refuse launch")
	}
	grants, err := Grants(m, []string{"repository.read", "repository.search", "tests.run"})
	if err != nil || strings.Join(grants, ",") != "repository.read,tests.run,repository.search" {
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

// Build and launch an independent example that imports only the public wire types.
func TestDiffInspectorIntegration(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "diff-inspector")
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), time.Minute)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "../../example/diff-inspector")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build example: %v\n%s", err, output)
	}
	manifestData, err := exec.Command(binary, "--manifest").Output()
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(t.TempDir(), "agent.json")
	if err := os.WriteFile(manifestPath, manifestData, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.ModelProfiles) != 0 || m.APIVersion != "giad/v1" {
		t.Fatalf("manifest=%+v", m)
	}
	grants, err := Grants(m, []string{"git.diff", "repository.instructions"})
	if err != nil {
		t.Fatal(err)
	}
	for _, truncated := range []bool{false, true} {
		t.Run(fmt.Sprintf("truncated=%v", truncated), func(t *testing.T) {
			diff := "diff --git a/example.go b/example.go\n+new line\n"
			if truncated {
				diff = strings.Repeat("x", tools.MaxOutputBytes+1)
			}
			reader, err := tools.Open(t.TempDir(), diff)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			report, err := (Session{Launcher: sandbox.TrustedHostLauncher{}, Manifest: m, Repository: reader, Job: protocol.Job{AllowedCapabilities: grants, ChangedFiles: []protocol.ChangedFile{{Path: "example.go", Status: "modified"}}}}).Run(ctx)
			if err != nil || report.Findings == nil || len(report.Findings) != 0 || !strings.Contains(report.Summary, "1 changed files") {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			if truncated && !strings.Contains(report.Limitations, "truncated") {
				t.Fatal("truncation missing from draft")
			}
		})
	}
}

func TestDockerExampleIntegration(t *testing.T) {
	image := os.Getenv("GIAD_TEST_DOCKER_IMAGE")
	if image == "" {
		t.Skip("set GIAD_TEST_DOCKER_IMAGE to run the real isolated Python example")
	}
	m, err := LoadManifest("../../example/pr-summary/agent.manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	grants, err := Grants(m, []string{"repository.instructions"})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := tools.Open(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := (Session{
		Launcher: sandbox.DockerLauncher{Image: image}, Manifest: m, Repository: reader,
		Job: protocol.Job{Number: 42, Title: "Example PR", AllowedCapabilities: grants, ChangedFiles: []protocol.ChangedFile{{Path: "example.go", Status: "modified"}}},
	}).Run(ctx)
	if err != nil || report.Summary != "PR #42: Example PR (1 changed files)." || report.Findings == nil || len(report.Findings) != 0 {
		t.Fatalf("isolated review: report=%+v err=%v", report, err)
	}
}

func TestDockerDiffInspectorIntegration(t *testing.T) {
	image := os.Getenv("GIAD_TEST_DIFF_IMAGE")
	if image == "" {
		t.Skip("set GIAD_TEST_DIFF_IMAGE to exercise the isolated diff broker")
	}
	m := protocol.Manifest{
		APIVersion: protocol.Version, Name: "diff-inspector", Version: "0.1.0",
		Entrypoint:   protocol.Entrypoint{Command: "/agent/diff-inspector", Args: []string{"--stdio"}},
		Capabilities: protocol.Capabilities{Required: []string{"git.diff", "repository.instructions"}},
	}
	grants, err := Grants(m, []string{"git.diff", "repository.instructions"})
	if err != nil {
		t.Fatal(err)
	}
	diff := "diff --git a/example.go b/example.go\n+new line\n"
	reader, err := tools.Open(t.TempDir(), diff)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := (Session{
		Launcher: sandbox.DockerLauncher{Image: image}, Manifest: m, Repository: reader,
		Job: protocol.Job{AllowedCapabilities: grants, ChangedFiles: []protocol.ChangedFile{{Path: "example.go", Status: "modified"}}},
	}).Run(ctx)
	expected := fmt.Sprintf("Retrieved %d bytes of diff for 1 changed files.", len(diff))
	if err != nil || report.Summary != expected || report.Findings == nil || len(report.Findings) != 0 {
		t.Fatalf("isolated diff broker: report=%+v err=%v", report, err)
	}
}

func TestRejectedFinishDoesNotPopulateLaterRequests(t *testing.T) {
	for _, retry := range []string{`{}`, `null`, `{"summary":"New attempt"}`} {
		t.Run(retry, func(t *testing.T) {
			s := fixtureSession(t, "success")
			frames := `{"apiVersion":"giad/v1","id":"bad","method":"review.finish","params":{"summary":"Rejected report","limitations":"","findings":[],"unsupported":true}}` + "\n" +
				`{"apiVersion":"giad/v1","id":"retry","method":"review.finish","params":` + retry + "}\n"
			p := &cleanupFailureProcess{output: strings.NewReader(frames)}
			s.Launcher = launchFunc(func(context.Context, sandbox.Command) (sandbox.Process, error) { return p, nil })
			report, err := s.Run(context.Background())
			if err == nil || report.Summary != "" || strings.Contains(p.input.String(), `"accepted":true`) {
				t.Fatalf("rejected report became an accepted completion: report=%+v replies=%s err=%v", report, p.input.String(), err)
			}
		})
	}
}

func TestRejectedFinishAllowsCompleteCorrection(t *testing.T) {
	s := fixtureSession(t, "success")
	frames := `{"apiVersion":"giad/v1","id":"bad","method":"review.finish","params":{"summary":"Rejected report","limitations":"","findings":[],"unsupported":true}}` + "\n" +
		`{"apiVersion":"giad/v1","id":"fixed","method":"review.finish","params":{"summary":"Corrected report","limitations":"New limitations","findings":[]}}` + "\n"
	p := &cleanupFailureProcess{output: strings.NewReader(frames)}
	s.Launcher = launchFunc(func(context.Context, sandbox.Command) (sandbox.Process, error) { return p, nil })
	report, err := s.Run(context.Background())
	if err != nil || report.Summary != "Corrected report" || !strings.Contains(report.Limitations, "New limitations") {
		t.Fatalf("complete correction was not accepted: report=%+v err=%v", report, err)
	}
}
