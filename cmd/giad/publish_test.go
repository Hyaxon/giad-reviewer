package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/hyaxon/giad/internal/publication"
	"github.com/hyaxon/giad/internal/review"
	"github.com/hyaxon/giad/pkg/protocol"
)

func TestPublishRejectsUnsupportedEventBeforeReadingDraft(t *testing.T) {
	cmd := newPublishCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"nonexistent.json", "--event", "APPROVE"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "review event must be") {
		t.Fatalf("unsupported event reached draft/network access: %v", err)
	}
}

type retryPublicationClient struct {
	current             githubapi.PRContext
	reviews             []githubapi.Review
	comments            []githubapi.ReviewComment
	contextCalls, posts int
	listError           error
}

func (c *retryPublicationClient) GetPRContext(context.Context, githubauth.Repository, int) (githubapi.PRContext, error) {
	c.contextCalls++
	return c.current, nil
}
func (c *retryPublicationClient) GetPullRequest(context.Context, githubauth.Repository, int) (githubapi.PullRequest, error) {
	return c.current.PullRequest, nil
}
func (c *retryPublicationClient) ReviewAuthor(context.Context, githubauth.Repository) (int64, error) {
	return 7, nil
}
func (c *retryPublicationClient) ListReviews(context.Context, githubauth.Repository, int) ([]githubapi.Review, error) {
	return c.reviews, c.listError
}
func (c *retryPublicationClient) ListReviewComments(context.Context, githubauth.Repository, int, int64) ([]githubapi.ReviewComment, error) {
	return c.comments, nil
}
func (c *retryPublicationClient) CreateReview(context.Context, githubauth.Repository, int, githubapi.ReviewSubmission) (githubapi.Review, error) {
	c.posts++
	return githubapi.Review{}, errors.New("unexpected POST")
}

func TestPublishCommandReconcilesBeforeCurrentPRValidation(t *testing.T) {
	for _, mode := range []string{"changed head", "changed base", "closed", "missing review", "foreign author", "edited inline", "listing failed", "wrong confirmation"} {
		t.Run(mode, func(t *testing.T) {
			base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
			draft := review.Result{APIVersion: protocol.Version, Agent: "test", Job: protocol.Job{Repository: "owner/repo", Number: 2, BaseSHA: base, HeadSHA: head}, Report: protocol.Report{Summary: "Reviewed", Findings: []protocol.Finding{{Source: "test", Category: "correctness", File: "auth.go", Line: 1, Severity: "high", Confidence: .9, Title: "Defect", Explanation: "Cause", Evidence: "Code", FailureScenario: "Trigger", SuggestedFix: "Fix"}}}}
			current := githubapi.PRContext{PullRequest: githubapi.PullRequest{Number: 2, State: "open", Base: githubapi.Ref{SHA: base}, Head: githubapi.Ref{SHA: head}}, Files: []githubapi.ChangedFile{{Filename: "auth.go", Status: "modified"}}, Diff: "diff --git a/auth.go b/auth.go\n--- a/auth.go\n+++ b/auth.go\n@@ -1 +1 @@\n-old\n+new\n"}
			plan, err := publication.PrepareWithOptions(draft, current, publication.Options{Inline: true})
			if err != nil {
				t.Fatal(err)
			}
			existing := githubapi.Review{ID: 42, URL: "https://github.com/owner/repo/pull/2#pullrequestreview-42", State: "COMMENTED", CommitID: head, Body: plan.Body}
			existing.User.ID = 7
			comment := plan.Comments[0]
			client := &retryPublicationClient{current: current, reviews: []githubapi.Review{existing}, comments: []githubapi.ReviewComment{{Path: comment.Path, Line: comment.Line, OriginalLine: comment.Line, Side: comment.Side, Body: comment.Body, OriginalCommitID: head}}}
			confirm := plan.Key
			client.current.PullRequest.Head.SHA = strings.Repeat("c", 40)
			switch mode {
			case "changed base":
				client.current.PullRequest.Base.SHA = strings.Repeat("d", 40)
			case "closed":
				client.current.PullRequest.State = "closed"
			case "missing review":
				client.reviews = nil
			case "foreign author":
				client.reviews[0].User.ID = 99
			case "edited inline":
				client.comments[0].Body = "edited"
			case "listing failed":
				client.listError = errors.New("listing failed")
			case "wrong confirmation":
				confirm = strings.Repeat("e", 64)
			}
			data, err := json.Marshal(draft)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "draft.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			cmd := newPublishCommandWithClient(client)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{path, "--inline", "--confirm", confirm})
			err = cmd.Execute()
			completed := mode == "changed head" || mode == "changed base" || mode == "closed"
			if (err == nil) != completed || client.posts != 0 {
				t.Fatalf("unsafe retry: posts=%d output=%s err=%v", client.posts, &output, err)
			}
			if completed && (client.contextCalls != 0 || !strings.Contains(output.String(), "Already published:")) {
				t.Fatalf("completed review required fresh context: calls=%d output=%s", client.contextCalls, &output)
			}
			if (mode == "missing review" || mode == "foreign author") && client.contextCalls != 1 {
				t.Fatal("unmatched retry skipped fresh revision checks")
			}
		})
	}
}
