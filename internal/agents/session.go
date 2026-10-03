package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hyaxon/giad/internal/model"
	"github.com/hyaxon/giad/internal/sandbox"
	"github.com/hyaxon/giad/internal/tools"
	"github.com/hyaxon/giad/pkg/protocol"
)

const MaxRequests = 64
const MaxModelCalls = 16
const MaxTestRuns = 2

type TestExecutor interface {
	Run(context.Context, string) (protocol.TestResult, error)
}

type Profile struct {
	Provider model.Provider
	Model    string
}

type Session struct {
	Launcher   sandbox.Launcher
	Manifest   protocol.Manifest
	Job        protocol.Job
	Repository *tools.Repository
	Profiles   map[string]Profile
	Progress   func(string)
	Tests      TestExecutor
}

// Run owns the review protocol and broker. The explicitly selected launcher owns
// execution; the session never falls back to host execution on a launch failure.
func (s Session) Run(ctx context.Context) (report protocol.Report, err error) {
	if s.Launcher == nil {
		return report, errors.New("agent session requires an explicitly selected launcher")
	}
	if s.Repository == nil {
		return report, errors.New("agent session requires repository tools")
	}
	if s.Manifest.APIVersion != protocol.Version {
		return report, fmt.Errorf("unsupported agent apiVersion %q; expected %q", s.Manifest.APIVersion, protocol.Version)
	}
	for _, name := range s.Manifest.ModelProfiles {
		p, ok := s.Profiles[name]
		if !ok || p.Provider == nil || p.Model == "" {
			return report, fmt.Errorf("model profile %q is not configured", name)
		}
	}
	broker := broker{session: s, read: map[string]map[int]bool{}}
	defer func() { err = errors.Join(err, broker.unload(ctx)) }()
	process, err := s.Launcher.Launch(ctx, sandbox.Command{Path: s.Manifest.Entrypoint.Command, Args: s.Manifest.Entrypoint.Args})
	if err != nil {
		return report, err
	}
	defer func() {
		err = errors.Join(err, process.Close())
		if diagnostics, ok := process.(sandbox.Diagnostics); err != nil && ok {
			if text := diagnostics.Diagnostics(); text != "" {
				err = fmt.Errorf("%w\nAgent diagnostics: %s", err, text)
			}
		}
	}()
	encoder := frameWriter{process.Stdin()}
	job := s.Job
	if !s.allowed("github.linked_issues") {
		job.LinkedIssues = nil
		job.IssuesError = "github.linked_issues capability denied"
	}
	if !s.allowed("repository.instructions") {
		job.TrustedInstructions = nil
	}
	job.ModelProfiles = s.Manifest.ModelProfiles
	if !s.allowed("tests.run") {
		job.TestProfiles = nil
	}
	data, err := json.Marshal(job)
	if err != nil {
		return report, err
	}
	if len(data) > 256*1024 {
		return report, errors.New("review job exceeds 256 KiB context budget")
	}
	if err := encoder.Encode(protocol.Frame{APIVersion: protocol.Version, Method: "review.start", Params: data}); err != nil {
		return report, err
	}
	scanner := bufio.NewScanner(process.Stdout())
	scanner.Buffer(make([]byte, 4096), protocol.MaxMessageBytes)
	totalBytes := 0
	seen := map[string]bool{}
	for count := 0; count < MaxRequests && scanner.Scan(); count++ {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		totalBytes += len(scanner.Bytes())
		if totalBytes > 4*1024*1024 {
			return report, errors.New("agent session exceeds 4 MiB input budget")
		}
		var frame protocol.Frame
		if err := decode(scanner.Bytes(), &frame); err != nil {
			return report, fmt.Errorf("invalid agent frame: %w", err)
		}
		if frame.APIVersion != protocol.Version || frame.ID == "" || len(frame.ID) > 128 || seen[frame.ID] || frame.Method == "" || len(frame.Method) > 128 || frame.Result != nil || frame.Error != "" {
			return report, errors.New("agent request requires matching apiVersion, unique string id, method and params")
		}
		seen[frame.ID] = true
		if s.Progress != nil {
			s.Progress(s.Manifest.Name + ": " + frame.Method)
		}
		var result any
		var callErr error
		if frame.Method == "review.finish" {
			callErr = decode(frame.Params, &report)
			if callErr == nil {
				callErr = broker.validate(report)
			}
			if callErr == nil {
				if len(broker.tests) == 0 {
					report.Limitations += "\nGIAD: no test profile was run."
				}
				for _, test := range broker.tests {
					status := "passed"
					if test.TimedOut {
						status = "timed out"
					} else if test.ExitCode == nil || *test.ExitCode != 0 {
						status = "failed"
					}
					report.Limitations += fmt.Sprintf("\nGIAD: test profile %s %s (head only; no base comparison).", test.Profile, status)
					if test.Truncated {
						report.Limitations += " Test output was truncated."
					}
					if test.OOMKilled {
						report.Limitations += " Container exceeded its memory limit."
					}
				}
				if broker.incomplete || s.Job.IssuesError != "" {
					report.Limitations += "\nGIAD: repository tools or linked-issue coverage were incomplete."
				}
				if err := encoder.Encode(protocol.Frame{APIVersion: protocol.Version, ID: frame.ID, Result: json.RawMessage(`{"accepted":true}`)}); err != nil {
					return protocol.Report{}, err
				}
				return report, nil
			}
		} else if !s.allowed(frame.Method) {
			callErr = fmt.Errorf("capability %q denied", frame.Method)
		} else {
			result, callErr = broker.call(ctx, frame.Method, frame.Params)
		}
		if err := ctx.Err(); err != nil {
			return protocol.Report{}, err
		}
		reply := protocol.Frame{APIVersion: protocol.Version, ID: frame.ID}
		if callErr != nil {
			broker.incomplete = true
			reply.Error = callErr.Error()
		} else {
			reply.Result, err = json.Marshal(result)
			if err != nil {
				return protocol.Report{}, err
			}
		}
		if err := encoder.Encode(reply); err != nil {
			return protocol.Report{}, err
		}
	}
	if ctx.Err() != nil {
		return protocol.Report{}, ctx.Err()
	}
	if err := scanner.Err(); err != nil {
		return protocol.Report{}, fmt.Errorf("read agent protocol: %w", err)
	}
	return protocol.Report{}, errors.New("agent exited or exhausted its request budget before a valid review.finish; session incomplete")
}
func (s Session) allowed(name string) bool {
	for _, allowed := range s.Job.AllowedCapabilities {
		if name == allowed && supported(name) {
			return true
		}
	}
	return false
}

type broker struct {
	session    Session
	read       map[string]map[int]bool
	incomplete bool
	modelCalls int
	active     string
	testCalls  int
	tests      []protocol.TestResult
}

func (b *broker) unload(ctx context.Context) error {
	if b.active == "" {
		return nil
	}
	p := b.session.Profiles[b.active]
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := p.Provider.Unload(cleanup, p.Model); err != nil {
		return fmt.Errorf("unload model: %w", err)
	}
	b.active = ""
	return nil
}
func (b *broker) call(ctx context.Context, method string, params json.RawMessage) (any, error) {
	s := b.session
	switch method {
	case "tests.run":
		var args protocol.TestRequest
		if err := decode(params, &args); err != nil {
			return nil, err
		}
		approved := false
		for _, name := range s.Job.TestProfiles {
			approved = approved || name == args.Profile
		}
		if !approved || s.Tests == nil {
			return nil, errors.New("test profile is not approved for this agent")
		}
		b.testCalls++
		if b.testCalls > MaxTestRuns {
			return nil, errors.New("test-run budget exhausted")
		}
		result, err := s.Tests.Run(ctx, args.Profile)
		if err == nil {
			b.tests = append(b.tests, result)
		}
		return result, err
	case "git.diff", "repository.instructions", "github.linked_issues":
		if err := decode(params, &struct{}{}); err != nil {
			return nil, err
		}
		switch method {
		case "repository.instructions":
			return s.Job.TrustedInstructions, nil
		case "github.linked_issues":
			if s.Job.IssuesError != "" {
				return nil, errors.New(s.Job.IssuesError)
			}
			return s.Job.LinkedIssues, nil
		default:
			result := s.Repository.Diff()
			b.incomplete = b.incomplete || result.Truncated
			return result, nil
		}
	case "repository.read":
		var args struct {
			Path  string `json:"path"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		}
		if err := decode(params, &args); err != nil {
			return nil, err
		}
		result, err := s.Repository.ReadLines(ctx, args.Path, args.Start, args.End)
		if err == nil {
			b.incomplete = b.incomplete || result.Truncated
			if b.read[args.Path] == nil {
				b.read[args.Path] = map[int]bool{}
			}
			lines := strings.Split(result.Text, "\n")
			for _, line := range lines[:len(lines)-1] {
				prefix, _, found := strings.Cut(line, ":")
				if number, err := strconv.Atoi(prefix); found && err == nil {
					b.read[args.Path][number] = true
				}
			}
		}
		return result, err
	case "repository.search":
		var args struct {
			Query string `json:"query"`
		}
		if err := decode(params, &args); err != nil {
			return nil, err
		}
		result, err := s.Repository.Search(ctx, args.Query)
		b.incomplete = b.incomplete || result.Truncated || result.SkippedFiles > 0
		return result, err
	case "model.chat":
		var args protocol.ChatRequest
		if len(params) > 96*1024 {
			return nil, errors.New("model request exceeds 96 KiB")
		}
		if err := decode(params, &args); err != nil {
			return nil, err
		}
		declared := false
		for _, name := range s.Manifest.ModelProfiles {
			declared = declared || name == args.Profile
		}
		if !declared {
			return nil, errors.New("model profile is not declared by agent")
		}
		b.modelCalls++
		if b.modelCalls > MaxModelCalls {
			return nil, errors.New("model-call budget exhausted")
		}
		if b.active != "" && b.active != args.Profile {
			if err := b.unload(ctx); err != nil {
				return nil, err
			}
		}
		b.active = args.Profile
		p := s.Profiles[args.Profile]
		return p.Provider.Chat(ctx, p.Model, args.Messages, args.Tools)
	}
	return nil, errors.New("unsupported capability")
}
func (b *broker) validate(report protocol.Report) error {
	data, err := json.Marshal(report)
	if err != nil || len(data) > 64*1024 {
		return errors.New("report exceeds 64 KiB")
	}
	if strings.TrimSpace(report.Summary) == "" || len(report.Summary) > 8192 || len(report.Limitations) > 8192 || report.Findings == nil || len(report.Findings) > 20 {
		return errors.New("report requires bounded summary, limitations and findings array (at most 20)")
	}
	changed := map[string]bool{}
	for _, file := range b.session.Job.ChangedFiles {
		if file.Status != "removed" {
			changed[file.Path] = true
		}
	}
	for _, f := range report.Findings {
		if !changed[f.File] || !b.read[f.File][f.Line] {
			return fmt.Errorf("finding must anchor a line read from a changed head file: %s:%d", f.File, f.Line)
		}
		if f.Severity != "high" && f.Severity != "medium" && f.Severity != "low" {
			return errors.New("invalid severity")
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			return errors.New("invalid confidence")
		}
		for _, text := range []string{f.Source, f.Category, f.Title, f.Explanation, f.Evidence, f.FailureScenario, f.SuggestedFix} {
			if strings.TrimSpace(text) == "" || len(text) > 8192 {
				return errors.New("finding fields must be nonempty and at most 8 KiB")
			}
		}
	}
	return nil
}

// Encode applies the same frame budget to host responses as incoming requests.
type frameWriter struct{ io.Writer }

func (w frameWriter) Encode(frame protocol.Frame) error {
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if len(data)+1 >= protocol.MaxMessageBytes {
		return errors.New("host response exceeds protocol frame budget")
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
