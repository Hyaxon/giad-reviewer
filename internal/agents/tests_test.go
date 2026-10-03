package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hyaxon/giad/internal/sandbox"
	"github.com/hyaxon/giad/pkg/protocol"
)

type testExecFunc func(context.Context, string) (protocol.TestResult, error)

func (f testExecFunc) Run(ctx context.Context, name string) (protocol.TestResult, error) {
	return f(ctx, name)
}

func TestTestBrokerApprovalAndBudget(t *testing.T) {
	calls := 0
	code := 1
	executor := testExecFunc(func(context.Context, string) (protocol.TestResult, error) {
		calls++
		return protocol.TestResult{Profile: "go", ExitCode: &code, Output: "test failed"}, nil
	})
	b := broker{session: Session{Job: protocol.Job{TestProfiles: []string{"go"}}, Tests: executor}}
	for _, params := range []string{`{"profile":"other"}`, `{"profile":"go","args":["./secret"]}`, `{"profile":"go","timeout":999}`} {
		if _, err := b.call(context.Background(), "tests.run", json.RawMessage(params)); err == nil {
			t.Fatalf("unsafe request accepted: %s", params)
		}
	}
	if calls != 0 {
		t.Fatal("denied request reached executor")
	}
	for i := 0; i < MaxTestRuns; i++ {
		result, err := b.call(context.Background(), "tests.run", json.RawMessage(`{"profile":"go"}`))
		if err != nil || *result.(protocol.TestResult).ExitCode != 1 {
			t.Fatalf("test failure must remain evidence: result=%+v err=%v", result, err)
		}
	}
	if _, err := b.call(context.Background(), "tests.run", json.RawMessage(`{"profile":"go"}`)); err == nil || calls != MaxTestRuns {
		t.Fatal("test run budget not enforced")
	}
}

func TestDeniedTestsNeverReachExecutor(t *testing.T) {
	s := fixtureSession(t, "success")
	calls := 0
	s.Tests = testExecFunc(func(context.Context, string) (protocol.TestResult, error) { calls++; return protocol.TestResult{}, nil })
	s.Job.TestProfiles = []string{"go"}
	p := &cleanupFailureProcess{output: strings.NewReader(`{"apiVersion":"giad/v1","id":"test","method":"tests.run","params":{"profile":"go"}}` + "\n" + `{"apiVersion":"giad/v1","id":"finish","method":"review.finish","params":{"summary":"Draft","limitations":"","findings":[]}}` + "\n")}
	s.Launcher = launchFunc(func(context.Context, sandbox.Command) (sandbox.Process, error) { return p, nil })
	report, err := s.Run(context.Background())
	if err != nil || calls != 0 || !strings.Contains(p.input.String(), `capability \"tests.run\" denied`) || !strings.Contains(report.Limitations, "no test profile was run") {
		t.Fatalf("denied tests: calls=%d report=%+v replies=%s err=%v", calls, report, p.input.String(), err)
	}
	var start protocol.Frame
	if err := json.Unmarshal([]byte(strings.Split(p.input.String(), "\n")[0]), &start); err != nil {
		t.Fatal(err)
	}
	var job protocol.Job
	if err := json.Unmarshal(start.Params, &job); err != nil || len(job.TestProfiles) != 0 {
		t.Fatalf("denied profiles leaked: %+v err=%v", job, err)
	}
}
