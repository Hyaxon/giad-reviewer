package agents

import (
	"context"
	"encoding/json"
	"errors"
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

// Scripted judgments verify the real agent/broker contract, not model accuracy.
type reviewProvider struct {
	response      string
	firstResponse string
	plan          []protocol.ToolCall
	err           error
	chats         int
	unloads       int
	messages      []model.Message
}

func (p *reviewProvider) Chat(_ context.Context, tag string, messages []model.Message, toolDefinitions []model.Tool) (model.Message, error) {
	if tag != "fixture" {
		return model.Message{}, errors.New("unexpected profile or model tools")
	}
	p.chats++
	p.messages = messages
	if p.err != nil {
		return model.Message{}, p.err
	}
	if p.chats == 1 {
		for _, call := range p.plan {
			advertised := false
			for _, definition := range toolDefinitions {
				advertised = advertised || definition.Function.Name == call.Function.Name
			}
			if !advertised {
				return model.Message{}, errors.New("research tool was not advertised")
			}
		}
		return model.Message{Role: "assistant", ToolCalls: p.plan}, nil
	}
	if p.chats == 2 && p.firstResponse != "" {
		return model.Message{Role: "assistant", Content: p.firstResponse}, p.err
	}
	return model.Message{Role: "assistant", Content: p.response}, p.err
}
func (p *reviewProvider) Unload(context.Context, string) error { p.unloads++; return nil }

func reviewPlan(path, testProfile string) []protocol.ToolCall {
	args, _ := json.Marshal(map[string]any{"path": path, "start": 1, "end": 0})
	plan := []protocol.ToolCall{
		{Function: protocol.CallFunction{Name: "repository_search", Arguments: json.RawMessage(`{"query":"return"}`)}},
		{Function: protocol.CallFunction{Name: "repository_read", Arguments: args}},
	}
	if testProfile != "" {
		args, _ := json.Marshal(protocol.TestRequest{Profile: testProfile})
		plan = append(plan, protocol.ToolCall{Function: protocol.CallFunction{Name: "tests_run", Arguments: args}})
	}
	return plan
}

func observedToolResult(messages []model.Message, name, evidence string) bool {
	for _, message := range messages {
		if message.Role == "tool" && message.ToolName == name && strings.Contains(message.Content, evidence) {
			return true
		}
	}
	return false
}

func TestCodeReviewIntegration(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python 3 is required for the independent reviewer example")
	}
	python, err = filepath.Abs(python)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := filepath.Abs("../../example/code-review/agent.py")
	if err != nil {
		t.Fatal(err)
	}
	runCodeReviewIntegration(t, sandbox.TrustedHostLauncher{}, protocol.Entrypoint{Command: python, Args: []string{agent}})
}

func TestDockerCodeReviewIntegration(t *testing.T) {
	image := os.Getenv("GIAD_TEST_REVIEW_IMAGE")
	if image == "" {
		t.Skip("set GIAD_TEST_REVIEW_IMAGE to exercise the isolated model-backed reviewer")
	}
	runCodeReviewIntegration(t, sandbox.DockerLauncher{Image: image}, protocol.Entrypoint{Command: "/usr/local/bin/python3", Args: []string{"/agent/agent.py"}})
}

func runCodeReviewIntegration(t *testing.T, launcher sandbox.Launcher, entrypoint protocol.Entrypoint) {
	t.Helper()
	manifest, err := LoadManifest("../../example/code-review/agent.manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest.Entrypoint = entrypoint
	grants, err := Grants(manifest, []string{"git.diff", "repository.read", "repository.search", "repository.instructions", "model.chat"})
	if err != nil {
		t.Fatal(err)
	}
	finding := protocol.Finding{
		Source: "code-review", Category: "security", File: "auth.py", Line: 2, Severity: "high", Confidence: .9,
		Title: "Fixed password grants access", Explanation: "All users share a password embedded in source.",
		Evidence: "return password == 'secret'", FailureScenario: "Anyone knowing the literal authenticates.",
		SuggestedFix: "Verify a per-user password hash with the authentication service.",
	}
	for _, test := range []struct {
		name       string
		findings   []protocol.Finding
		malformed  bool
		fenced     bool
		repair     bool
		modelErr   bool
		valid      bool
		testResult *protocol.TestResult
	}{
		{name: "anchored finding", findings: []protocol.Finding{finding}, valid: true},
		{name: "no findings", findings: []protocol.Finding{}, valid: true},
		{name: "fenced JSON", findings: []protocol.Finding{finding}, fenced: true, valid: true},
		{name: "repair severity", findings: []protocol.Finding{finding}, repair: true, valid: true},
		{name: "malformed model JSON", malformed: true},
		{name: "model failure", modelErr: true},
		{name: "unread anchor", findings: []protocol.Finding{finding}},
		{name: "failed tests are evidence", findings: []protocol.Finding{}, valid: true, testResult: &protocol.TestResult{Profile: "go", ExitCode: func() *int { n := 1; return &n }(), Output: "TestLogin FAILED"}},
		{name: "test timeout", findings: []protocol.Finding{}, valid: true, testResult: &protocol.TestResult{Profile: "go", TimedOut: true, Truncated: true, Output: "partial output"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.name == "unread anchor" {
				test.findings[0].Line = 999
			}
			response, err := json.Marshal(protocol.Report{Summary: "Inspected authentication", Limitations: "No caller context available.", Findings: test.findings})
			if err != nil {
				t.Fatal(err)
			}
			provider := &reviewProvider{response: string(response), plan: reviewPlan("auth.py", "")}
			if test.fenced {
				provider.response = "```json\n" + provider.response + "\n```"
			}
			if test.repair {
				provider.firstResponse = strings.Replace(provider.response, `"severity":"high"`, `"severity":"Critical"`, 1)
			}
			if test.malformed {
				provider.response = "not JSON"
			}
			if test.modelErr {
				provider.err = errors.New("model unavailable")
			}
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "auth.py"), []byte("def authenticate(password):\n    return password == 'secret'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			reader, err := tools.Open(root, "diff --git a/auth.py b/auth.py\n+    return password == 'secret'\n")
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			jobGrants := append([]string{}, grants...)
			var testProfiles []string
			var tests TestExecutor
			testCalls := 0
			if test.testResult != nil {
				provider.plan = reviewPlan("auth.py", "go")
				jobGrants = append(jobGrants, "tests.run")
				testProfiles = []string{"go"}
				tests = testExecFunc(func(_ context.Context, name string) (protocol.TestResult, error) {
					testCalls++
					if name != "go" {
						t.Fatalf("unexpected test profile %q", name)
					}
					return *test.testResult, nil
				})
			}
			report, err := (Session{
				Launcher: launcher, Manifest: manifest, Repository: reader,
				Tests:    tests,
				Profiles: map[string]Profile{"review": {Provider: provider, Model: "fixture"}},
				Job: protocol.Job{ChangedFiles: []protocol.ChangedFile{{Path: "auth.py", Status: "added"}}, AllowedCapabilities: jobGrants, TestProfiles: testProfiles,
					TrustedInstructions: []protocol.Instruction{
						{Path: "AGENTS.md", Scope: ".", Content: "root-guidance-marker"},
						{Path: "other/AGENTS.md", Scope: "other", Content: "unrelated-guidance-marker"},
					}},
			}).Run(ctx)
			if test.valid {
				if err != nil || report.Findings == nil || len(report.Findings) != len(test.findings) {
					t.Fatalf("review report=%+v err=%v", report, err)
				}
			} else if err == nil {
				t.Fatalf("invalid model result must fail, got report=%+v", report)
			}
			if test.testResult == nil && test.valid && !strings.Contains(report.Limitations, "No tests ran") {
				t.Fatal("missing test coverage limitation")
			}
			if test.testResult != nil && (testCalls != 1 || !observedToolResult(provider.messages, "tests_run", test.testResult.Output) || strings.Contains(report.Limitations, "No tests ran")) {
				t.Fatalf("test evidence missing: calls=%d report=%+v messages=%+v", testCalls, report, provider.messages)
			}
			if _, isolated := launcher.(sandbox.DockerLauncher); isolated && !test.valid && !strings.Contains(err.Error(), "Agent diagnostics: code-review:") {
				t.Fatalf("failed isolated agent lost its diagnostic: %v", err)
			}
			wantChats := 2
			if test.repair {
				wantChats = 3
			}
			if test.malformed || test.name == "unread anchor" {
				wantChats = MaxModelCalls
			}
			if test.modelErr {
				wantChats = 1
			}
			if provider.chats != wantChats || provider.unloads != 1 {
				t.Fatalf("model lifecycle: chats=%d unloads=%d", provider.chats, provider.unloads)
			}
			if len(provider.messages) < 3 || provider.messages[0].Role != "system" || provider.messages[1].Role != "user" || !strings.Contains(provider.messages[0].Content, "root-guidance-marker") || strings.Contains(provider.messages[0].Content, "unrelated-guidance-marker") {
				t.Fatalf("model did not receive scoped guidance and inspected source: %+v", provider.messages)
			}
			if !test.modelErr && (!observedToolResult(provider.messages, "repository_search", "auth.py") || !observedToolResult(provider.messages, "repository_read", "2:     return password")) {
				t.Fatalf("model did not receive search results and inspected source: %+v", provider.messages)
			}
		})
	}
}

// Exercise the complete chain: isolated Python agent -> broker -> separate Go
// test container -> model evidence -> local report. No real model is needed.
func TestDockerCodeReviewWithTests(t *testing.T) {
	agentImage, testImage := os.Getenv("GIAD_TEST_REVIEW_IMAGE"), os.Getenv("GIAD_TEST_RUNNER_IMAGE")
	if agentImage == "" || testImage == "" {
		t.Skip("set reviewer and test runner images for the full test/review integration")
	}
	manifest, err := LoadManifest("../../example/code-review/agent.manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, name := range []string{"go.mod", "add.go", "add_test.go"} {
		data, err := os.ReadFile(filepath.Join("../../example/go-tests/fixture", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	reader, err := tools.Open(root, "diff --git a/add.go b/add.go\n+func Add(a, b int) int { return a - b }\n")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	provider := &reviewProvider{response: `{"summary":"Inspected Add with test evidence","limitations":"Head tests only","findings":[]}`, plan: reviewPlan("add.go", "go")}
	tests := &sandbox.TestRunner{Checkout: root, Profiles: map[string]sandbox.TestProfile{"go": {Image: testImage, Command: "/usr/local/go/bin/go", Args: []string{"test", "-p", "1", "./..."}, TimeoutSeconds: 90}}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	report, err := (Session{
		Launcher: sandbox.DockerLauncher{Image: agentImage}, Manifest: manifest, Repository: reader,
		Profiles: map[string]Profile{"review": {Provider: provider, Model: "fixture"}}, Tests: tests,
		Job: protocol.Job{ChangedFiles: []protocol.ChangedFile{{Path: "add.go", Status: "modified"}}, AllowedCapabilities: []string{"git.diff", "repository.read", "repository.search", "repository.instructions", "model.chat", "tests.run"}, TestProfiles: []string{"go"}},
	}).Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tests.Results) != 1 || tests.Results[0].ExitCode == nil || *tests.Results[0].ExitCode != 1 || !observedToolResult(provider.messages, "tests_run", "TestAdd") || !strings.Contains(report.Limitations, "test profile go failed") {
		t.Fatalf("test evidence lost: results=%+v messages=%+v report=%+v", tests.Results, provider.messages, report)
	}
}
