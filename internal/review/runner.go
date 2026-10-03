package review

import (
	"context"
	"errors"
	"fmt"

	"github.com/hyaxon/giad/internal/agents"
	"github.com/hyaxon/giad/internal/config"
	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/hyaxon/giad/internal/instructions"
	"github.com/hyaxon/giad/internal/model/ollama"
	"github.com/hyaxon/giad/internal/repo"
	"github.com/hyaxon/giad/internal/sandbox"
	"github.com/hyaxon/giad/internal/tools"
	"github.com/hyaxon/giad/pkg/protocol"
)

type Request struct {
	Repository githubauth.Repository
	Number     int
	Manifest   protocol.Manifest
	Config     config.Runtime
}
type Result struct {
	APIVersion string                `json:"apiVersion"`
	Agent      string                `json:"agent"`
	Job        protocol.Job          `json:"job"`
	Report     protocol.Report       `json:"report"`
	TestRuns   []protocol.TestResult `json:"testRuns"`
}
type Runner struct {
	Auth     githubauth.Provider
	Progress func(string)
}

// Run acquires immutable review boundaries and returns a local draft. It never publishes.
func (r Runner) Run(ctx context.Context, request Request) (_ Result, err error) {
	policy, ok := request.Config.Agents[request.Manifest.Name]
	if !ok {
		return Result{}, fmt.Errorf("agent %q has no trusted runtime policy", request.Manifest.Name)
	}
	if err := sandbox.ValidateImage(policy.SandboxImage); err != nil {
		return Result{}, fmt.Errorf("agent %q: %w", request.Manifest.Name, err)
	}
	grants, err := agents.Grants(request.Manifest, policy.Capabilities)
	if err != nil {
		return Result{}, err
	}
	testProfiles := map[string]sandbox.TestProfile{}
	for _, name := range policy.TestProfiles {
		profile, ok := request.Config.Tests[name]
		if !ok {
			return Result{}, fmt.Errorf("test profile %q is not configured", name)
		}
		if err := profile.Validate(); err != nil {
			return Result{}, fmt.Errorf("test profile %q: %w", name, err)
		}
		testProfiles[name] = profile
	}
	profiles := map[string]agents.Profile{}
	for _, name := range request.Manifest.ModelProfiles {
		profile, ok := request.Config.Models[name]
		if !ok {
			return Result{}, fmt.Errorf("model profile %q is not configured", name)
		}
		if profile.Provider != "ollama" || profile.Model == "" {
			return Result{}, fmt.Errorf("invalid model profile %q", name)
		}
		provider, err := ollama.New(profile.Endpoint)
		if err != nil {
			return Result{}, err
		}
		profiles[name] = agents.Profile{Provider: provider, Model: profile.Model}
	}
	input, err := githubapi.NewClient(r.Auth).GetPRContext(ctx, request.Repository, request.Number)
	if err != nil {
		return Result{}, err
	}
	checkout, err := (repo.Manager{Auth: r.Auth}).Prepare(ctx, repo.CheckoutRequest{Repository: request.Repository, Number: request.Number, BaseSHA: input.PullRequest.Base.SHA, HeadSHA: input.PullRequest.Head.SHA})
	if err != nil {
		return Result{}, err
	}
	defer func() { err = errors.Join(err, checkout.Close()) }()
	reader, err := tools.Open(checkout.Path, input.Diff)
	if err != nil {
		return Result{}, err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	job := protocol.Job{Repository: request.Repository.Owner + "/" + request.Repository.Name, Number: request.Number,
		URL: input.PullRequest.URL, Title: input.PullRequest.Title, Body: input.PullRequest.Body,
		BaseSHA: checkout.BaseSHA, HeadSHA: checkout.HeadSHA, IssuesError: input.IssuesError,
		AllowedCapabilities: grants, ModelProfiles: request.Manifest.ModelProfiles,
		ChangedFiles: []protocol.ChangedFile{}, LinkedIssues: []protocol.Issue{}}
	for _, grant := range grants {
		if grant == "tests.run" {
			job.TestProfiles = append([]string{}, policy.TestProfiles...)
		}
	}
	for _, f := range input.Files {
		job.ChangedFiles = append(job.ChangedFiles, protocol.ChangedFile{Path: f.Filename, PreviousPath: f.PreviousFilename, Status: f.Status})
	}
	for _, i := range input.LinkedIssues {
		job.LinkedIssues = append(job.LinkedIssues, protocol.Issue{Repository: i.Repository.NameWithOwner, Number: i.Number, URL: i.URL, Title: i.Title, Body: i.Body})
	}
	job.TrustedInstructions, err = instructions.Resolve(ctx, checkout, checkout.BaseSHA, job.ChangedFiles)
	if err != nil {
		return Result{}, fmt.Errorf("resolve trusted base instructions: %w", err)
	}
	// Repository guidance is mandatory context whenever it exists.
	if len(job.TrustedInstructions) > 0 {
		granted := false
		for _, name := range grants {
			granted = granted || name == "repository.instructions"
		}
		if !granted {
			return Result{}, errors.New("repository has AGENTS.md; repository.instructions must be declared and granted")
		}
	}
	tests := &sandbox.TestRunner{Checkout: checkout.Path, Profiles: testProfiles, Results: []protocol.TestResult{}}
	report, err := (agents.Session{Launcher: sandbox.DockerLauncher{Image: policy.SandboxImage}, Manifest: request.Manifest, Job: job, Repository: reader, Profiles: profiles, Tests: tests, Progress: r.Progress}).Run(ctx)
	if err != nil {
		return Result{}, err
	}
	return Result{APIVersion: protocol.Version, Agent: request.Manifest.Name, Job: job, Report: report, TestRuns: tests.Results}, nil
}
