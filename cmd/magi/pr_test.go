package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/hyaxon/magi-agents/internal/githubapi"
	"github.com/hyaxon/magi-agents/internal/githubauth"
)

func TestParsePRTarget(t *testing.T) {
	for _, tc := range []struct {
		input, repo string
		valid       bool
	}{
		{"https://github.com/owner/repo/pull/42", "", true},
		{"https://github.com/owner/repo/pull/42/", "OWNER/REPO", true},
		{"42", "owner/repo", true},
		{"42", "", false},
		{"0", "owner/repo", false},
		{"-1", "owner/repo", false},
		{"https://github.com/owner/repo/pull/42", "other/repo", false},
		{"https://other.example/owner/repo/pull/42", "", false},
		{"https://github.com/owner/repo/issues/42", "", false},
		{"https://github.com/owner/repo/pull/42/files", "", false},
		{"42", "owner/..", false},
	} {
		t.Run(tc.input+"_"+tc.repo, func(t *testing.T) {
			repo, number, err := parsePRTarget(tc.input, tc.repo)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
			if tc.valid && (number != 42 || !strings.EqualFold(repo.Owner+"/"+repo.Name, "owner/repo") || repo.Host != "github.com") {
				t.Fatalf("incorrect target: %+v #%d", repo, number)
			}
		})
	}
}

func TestPRView(t *testing.T) {
	cmd := newPRViewCommand(func(ctx context.Context, repo githubauth.Repository, number int) (githubapi.PRContext, error) {
		if repo.Owner != "owner" || repo.Name != "repo" || number != 42 {
			t.Fatal("wrong request target")
		}
		return githubapi.PRContext{PullRequest: githubapi.PullRequest{Number: 42, Title: "Example change", Body: "Description", URL: "https://github.com/owner/repo/pull/42", Base: githubapi.Ref{Name: "main", SHA: "base-sha"}, Head: githubapi.Ref{Name: "feature", SHA: "head-sha"}}, Diff: "example patch", IssuesError: "access denied"}, nil
	})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"42", "--repo", "owner/repo", "--diff"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"owner/repo #42: Example change", "Base: main (base-sha)", "Head: feature (head-sha)", "Description", "WARNING: Linked issues", "example patch"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing output %q", want)
		}
	}
}
