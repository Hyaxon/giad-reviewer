package publication

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/hyaxon/giad/internal/review"
	"github.com/hyaxon/giad/pkg/protocol"
)

func fixture() (review.Result, githubapi.PRContext) {
	base, head := strings.Repeat("a", 40), strings.Repeat("b", 40)
	finding := protocol.Finding{Source: "reviewer", Category: "security", File: "auth.go", Line: 2, Severity: "high", Confidence: .9, Title: "Wrong owner", Explanation: "Owner check is missing", Evidence: "return accountID", FailureScenario: "Wrong account is accepted", SuggestedFix: "Compare the token owner"}
	draft := review.Result{APIVersion: protocol.Version, Agent: "fixture", Job: protocol.Job{Repository: "Owner/Repo", Number: 2, BaseSHA: base, HeadSHA: head}, Report: protocol.Report{Summary: "Reviewed authentication", Limitations: "No caller context", Findings: []protocol.Finding{finding}}}
	current := githubapi.PRContext{PullRequest: githubapi.PullRequest{Number: 2, State: "open", Base: githubapi.Ref{SHA: base}, Head: githubapi.Ref{SHA: head}}, Files: []githubapi.ChangedFile{{Filename: "auth.go", Status: "modified"}}, Diff: "diff --git a/auth.go b/auth.go\n--- a/auth.go\n+++ b/auth.go\n@@ -1,2 +1,2 @@\n package auth\n-return tokenOwner\n+return accountID\n"}
	return draft, current
}

func TestPlanValidatesRevisionsAndAnchors(t *testing.T) {
	for _, mode := range []string{"valid", "stale head", "stale base", "closed", "outside hunk", "deleted", "malformed diff", "literal text", "deletion elsewhere", "quoted path"} {
		t.Run(mode, func(t *testing.T) {
			draft, current := fixture()
			switch mode {
			case "stale head":
				current.PullRequest.Head.SHA = strings.Repeat("c", 40)
			case "stale base":
				current.PullRequest.Base.SHA = strings.Repeat("c", 40)
			case "closed":
				current.PullRequest.State = "closed"
			case "outside hunk":
				draft.Report.Findings[0].Line = 99
			case "deleted":
				current.Files[0].Status = "removed"
			case "malformed diff":
				current.Diff = strings.TrimSuffix(current.Diff, "+return accountID\n")
			case "literal text":
				draft.Report.Summary = "<script>@someone [link](https://example.test)</script>"
			case "deletion elsewhere":
				current.Diff += "diff --git a/old.go b/old.go\n--- a/old.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\n"
			case "quoted path":
				draft.Report.Findings[0].File = "auth space.go"
				current.Files[0].Filename = "auth space.go"
				current.Diff = strings.Replace(current.Diff, "+++ b/auth.go", `+++ "b/auth space.go"`, 1)
			}
			plan, err := PrepareWithOptions(draft, current, Options{Inline: true})
			valid := mode == "valid" || mode == "literal text" || mode == "deletion elsewhere" || mode == "quoted path"
			if (err == nil) != valid {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if valid && (len(plan.Key) != 64 || !strings.Contains(plan.Body, "giad-publication:"+plan.Key)) {
				t.Fatal("missing content identity")
			}
			if mode == "literal text" && (strings.Contains(plan.Body, "<script>") || strings.Contains(plan.Body, "@someone") || strings.Contains(plan.Body, "[link](")) {
				t.Fatal("agent text rendered as active markup")
			}
		})
	}
}

type fakeAPI struct {
	mu       sync.Mutex
	current  githubapi.PullRequest
	reviews  []githubapi.Review
	posts    int
	unknown  bool
	retain   bool
	comments []githubapi.ReviewComment
}

func (a *fakeAPI) GetPullRequest(context.Context, githubauth.Repository, int) (githubapi.PullRequest, error) {
	return a.current, nil
}
func (a *fakeAPI) ReviewAuthor(context.Context, githubauth.Repository) (int64, error) { return 7, nil }
func (a *fakeAPI) ListReviews(context.Context, githubauth.Repository, int) ([]githubapi.Review, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]githubapi.Review{}, a.reviews...), nil
}
func (a *fakeAPI) CreateReview(_ context.Context, _ githubauth.Repository, _ int, submission githubapi.ReviewSubmission) (githubapi.Review, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.posts++
	r := githubapi.Review{ID: 42, URL: "https://github.com/owner/repo/pull/2#pullrequestreview-42", State: mustReviewState(submission.Event), CommitID: submission.CommitID, Body: submission.Body}
	r.User.ID = 7
	if a.retain || !a.unknown {
		a.reviews = append(a.reviews, r)
		for _, comment := range submission.Comments {
			a.comments = append(a.comments, githubapi.ReviewComment{Path: comment.Path, Line: comment.Line, OriginalLine: comment.Line, Side: comment.Side, Body: comment.Body, OriginalCommitID: submission.CommitID})
		}
	}
	if a.unknown {
		return githubapi.Review{}, errors.New("connection lost")
	}
	return r, nil
}

func TestPublishRetryAndConfirmation(t *testing.T) {
	for _, mode := range []string{"success", "unconfirmed", "wrong confirmation", "stale", "ambiguous accepted", "ambiguous absent", "foreign author"} {
		t.Run(mode, func(t *testing.T) {
			draft, current := fixture()
			plan, err := Prepare(draft, current)
			if err != nil {
				t.Fatal(err)
			}
			api := &fakeAPI{current: current.PullRequest}
			confirm := plan.Key
			switch mode {
			case "unconfirmed":
				confirm = ""
			case "wrong confirmation":
				confirm = strings.Repeat("c", 64)
			case "stale":
				api.current.Head.SHA = strings.Repeat("d", 40)
			case "ambiguous accepted":
				api.unknown = true
				api.retain = true
			case "ambiguous absent":
				api.unknown = true
			case "foreign author":
				r := githubapi.Review{State: "COMMENTED", Body: plan.Body, CommitID: plan.HeadSHA}
				r.User.ID = 999
				api.reviews = []githubapi.Review{r}
			}
			dir := t.TempDir()
			result, err := Publish(context.Background(), api, plan, confirm, dir)
			if mode == "unconfirmed" || mode == "wrong confirmation" || mode == "stale" {
				if err == nil || api.posts != 0 {
					t.Fatal("unapproved/stale publication reached POST")
				}
				return
			}
			if strings.HasPrefix(mode, "ambiguous") && err == nil {
				t.Fatal("transport failure must remain explicit")
			}
			if mode == "success" && (err != nil || result.Existing) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			result, err = Publish(context.Background(), api, plan, confirm, dir)
			if mode == "ambiguous absent" {
				if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
					t.Fatalf("unsafe retry: %v", err)
				}
			} else if err != nil || !result.Existing {
				t.Fatalf("failed reconciliation: %+v %v", result, err)
			}
			if api.posts != 1 {
				t.Fatalf("duplicate POST: %d", api.posts)
			}
		})
	}
}

func TestConcurrentPublishPostsOnce(t *testing.T) {
	draft, current := fixture()
	plan, err := Prepare(draft, current)
	if err != nil {
		t.Fatal(err)
	}
	api := &fakeAPI{current: current.PullRequest}
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = Publish(context.Background(), api, plan, plan.Key, dir) }()
	}
	wg.Wait()
	if api.posts != 1 {
		t.Fatalf("concurrent duplicate publication: %d", api.posts)
	}
}

func TestLoadDraftRejectsIncompleteInput(t *testing.T) {
	for _, body := range []string{"", `{}`, `{"apiVersion":"giad/v1","unknown":true}`, `{"apiVersion":"giad/v1"} {}`, strings.Repeat("x", 1024*1024+1)} {
		path := filepath.Join(t.TempDir(), "draft.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadDraft(path); err == nil {
			t.Fatalf("invalid saved draft accepted: %.100s", body)
		}
	}
}

func mustReviewState(event string) string {
	state, err := githubapi.ReviewState(event)
	if err != nil {
		panic(err)
	}
	return state
}
func (a *fakeAPI) ListReviewComments(context.Context, githubauth.Repository, int, int64) ([]githubapi.ReviewComment, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]githubapi.ReviewComment{}, a.comments...), nil
}

func TestPublicationOptionsBindConfirmation(t *testing.T) {
	draft, current := fixture()
	baseline, err := Prepare(draft, current)
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []Options{{Event: "REQUEST_CHANGES"}, {Inline: true}, {Event: "request-changes", Inline: true}} {
		plan, err := PrepareWithOptions(draft, current, options)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Key == baseline.Key {
			t.Fatal("old confirmation authorized different publication options")
		}
		if options.Inline {
			if len(plan.Comments) != 1 || plan.Comments[0].Path != "auth.go" || plan.Comments[0].Line != 2 || plan.Comments[0].Side != "RIGHT" || !strings.Contains(plan.Comments[0].Body, "Wrong owner") || strings.Contains(plan.Body, "Wrong owner") {
				t.Fatalf("finding was not attached inline: %+v", plan)
			}
		}
		api := &fakeAPI{current: current.PullRequest}
		if _, err := Publish(context.Background(), api, plan, baseline.Key, t.TempDir()); err == nil || api.posts != 0 {
			t.Fatal("mismatched confirmation reached POST")
		}
	}
	if _, err := PrepareWithOptions(draft, current, Options{Event: "APPROVE"}); err == nil {
		t.Fatal("unsupported event was accepted")
	}
}

func TestInlineDiffAnchorsAcrossHunks(t *testing.T) {
	for _, line := range []int{11, 20, 21, 22, 23, 24, 109, 110} {
		draft, current := fixture()
		draft.Report.Findings[0].Line = line
		current.Diff = "diff --git a/auth.go b/auth.go\n--- a/auth.go\n+++ b/auth.go\n@@ -10,3 +20,4 @@\n shared\n-old\n+replacement\n+addition\n tail\n@@ -100 +110 @@\n-old\n+new\n"
		plan, err := PrepareWithOptions(draft, current, Options{Inline: true})
		valid := line >= 20 && line <= 23 || line == 110
		if (err == nil) != valid {
			t.Fatalf("incorrect head diff mapping for line %d: %v", line, err)
		}
		if valid && plan.Comments[0].Line != line {
			t.Fatal("inline comment used a diff position instead of a head line")
		}
	}
}

func TestInlineRetryReconcilesActionAndComments(t *testing.T) {
	for _, mode := range []string{"success", "ambiguous accepted", "missing comment", "edited comment", "wrong side", "wrong commit", "outdated line", "dismissed", "thread reply"} {
		t.Run(mode, func(t *testing.T) {
			draft, current := fixture()
			plan, err := PrepareWithOptions(draft, current, Options{Event: "REQUEST_CHANGES", Inline: true})
			if err != nil {
				t.Fatal(err)
			}
			api := &fakeAPI{current: current.PullRequest}
			dir := t.TempDir()
			if mode == "ambiguous accepted" {
				api.unknown = true
				api.retain = true
			}
			_, err = Publish(context.Background(), api, plan, plan.Key, dir)
			if mode == "ambiguous accepted" {
				if err == nil {
					t.Fatal("network failure was hidden")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if api.reviews[0].State != "CHANGES_REQUESTED" {
				t.Fatal("request-changes event was lost")
			}
			switch mode {
			case "missing comment":
				api.comments = nil
			case "edited comment":
				api.comments[0].Body = "edited"
			case "wrong side":
				api.comments[0].Side = "LEFT"
			case "wrong commit":
				api.comments[0].OriginalCommitID = strings.Repeat("c", 40)
			case "outdated line":
				api.comments[0].Line = 0
			case "dismissed":
				api.reviews[0].State = "DISMISSED"
			case "thread reply":
				api.comments = append(api.comments, githubapi.ReviewComment{Body: "Thanks, fixed", InReplyToID: 123})
			}
			outcome, err := Publish(context.Background(), api, plan, plan.Key, dir)
			valid := mode == "success" || mode == "ambiguous accepted" || mode == "outdated line" || mode == "dismissed" || mode == "thread reply"
			if (err == nil) != valid || (valid && !outcome.Existing) || api.posts != 1 {
				t.Fatalf("unsafe inline reconciliation: outcome=%+v posts=%d err=%v", outcome, api.posts, err)
			}
		})
	}
}

type rejectingAPI struct{ fakeAPI }

func (a *rejectingAPI) CreateReview(context.Context, githubauth.Repository, int, githubapi.ReviewSubmission) (githubapi.Review, error) {
	a.posts++
	return githubapi.Review{}, &githubapi.HTTPError{Status: 422, Message: "Cannot request changes"}
}
func TestExplicitGitHubRejectionAllowsCorrectedRetry(t *testing.T) {
	draft, current := fixture()
	plan, err := PrepareWithOptions(draft, current, Options{Event: "REQUEST_CHANGES"})
	if err != nil {
		t.Fatal(err)
	}
	api := &rejectingAPI{fakeAPI: fakeAPI{current: current.PullRequest}}
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if _, err := Publish(context.Background(), api, plan, plan.Key, dir); err == nil || !strings.Contains(err.Error(), "GitHub rejected") {
			t.Fatalf("explicit rejection incorrectly left an unknown outcome: %v", err)
		}
	}
	if api.posts != 2 {
		t.Fatal("explicitly rejected attempt was not released")
	}
}

func TestFormattingMigrationReconcilesWithoutReposting(t *testing.T) {
	for _, inline := range []bool{false, true} {
		for _, mode := range []string{"completed with new hash", "completed with old hash", "uncertain old attempt", "unsubmitted old preview", "edited old comment"} {
			t.Run(fmt.Sprintf("inline=%v/%s", inline, mode), func(t *testing.T) {
				draft, current := fixture()
				draft.Report.Findings[0].FailureScenario = "The `AuthorizeReset` function doesn't check the account."
				plan, err := PrepareWithOptions(draft, current, Options{Event: "REQUEST_CHANGES", Inline: inline})
				if err != nil {
					t.Fatal(err)
				}
				old := plan.legacy
				if old == nil {
					t.Fatal("test requires a changed rendering identity")
				}
				api := &fakeAPI{current: current.PullRequest}
				dir := t.TempDir()
				confirm := plan.Key
				if mode == "uncertain old attempt" {
					path := filepath.Join(dir, "7-"+old.Key+".json")
					if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
						t.Fatal(err)
					}
				} else if mode == "unsubmitted old preview" {
					confirm = old.Key
				} else {
					if _, err := Publish(context.Background(), api, *old, old.Key, dir); err != nil {
						t.Fatal(err)
					}
					if mode == "completed with old hash" {
						confirm = old.Key
					}
					if mode == "edited old comment" {
						if inline {
							api.comments[0].Body = "edited"
						} else {
							api.reviews[0].Body = "edited"
						}
					}
				}
				before := api.posts
				outcome, err := Publish(context.Background(), api, plan, confirm, dir)
				completed := strings.HasPrefix(mode, "completed")
				if (err == nil) != completed || (completed && !outcome.Existing) || api.posts != before {
					t.Fatalf("formatting change duplicated a review or hid an uncertain result: %+v err=%v posts=%d", outcome, err, api.posts)
				}
			})
		}
	}
}

func TestPublishedSummaryOmitsRevisionHashes(t *testing.T) {
	for _, inline := range []bool{false, true} {
		draft, current := fixture()
		plan, err := PrepareWithOptions(draft, current, Options{Inline: inline})
		if err != nil {
			t.Fatal(err)
		}
		for _, removed := range []string{"Base:", "Head:", draft.Job.BaseSHA, draft.Job.HeadSHA} {
			if strings.Contains(plan.Body, removed) {
				t.Fatalf("revision metadata still appears in the comment: %q", removed)
			}
		}
		if plan.BaseSHA != draft.Job.BaseSHA || plan.HeadSHA != draft.Job.HeadSHA {
			t.Fatal("revision metadata was removed from publication validation")
		}
		for _, changeBase := range []bool{false, true} {
			changedDraft, changedCurrent := draft, current
			if changeBase {
				changedDraft.Job.BaseSHA = strings.Repeat("c", 40)
				changedCurrent.PullRequest.Base.SHA = changedDraft.Job.BaseSHA
			} else {
				changedDraft.Job.HeadSHA = strings.Repeat("c", 40)
				changedCurrent.PullRequest.Head.SHA = changedDraft.Job.HeadSHA
			}
			changed, err := PrepareWithOptions(changedDraft, changedCurrent, Options{Inline: inline})
			if err != nil || changed.Key == plan.Key {
				t.Fatalf("confirmation no longer binds the reviewed revisions: %v", err)
			}
		}
	}
}

func TestHashRemovalRecognizesBothOlderCommentFormats(t *testing.T) {
	draft, current := fixture()
	draft.Report.Findings[0].FailureScenario = "The `AuthorizeReset` function doesn't check the account."
	plan, err := PrepareWithOptions(draft, current, Options{Inline: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.legacy == nil || plan.legacy.legacy == nil {
		t.Fatal("both previous publication formats are required")
	}
	for old := plan.legacy; old != nil; old = old.legacy {
		for _, uncertain := range []bool{false, true} {
			api := &fakeAPI{current: current.PullRequest}
			dir := t.TempDir()
			if uncertain {
				if err := os.WriteFile(filepath.Join(dir, "7-"+old.Key+".json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if _, err := Publish(context.Background(), api, *old, old.Key, dir); err != nil {
				t.Fatal(err)
			}
			before := api.posts
			outcome, err := Publish(context.Background(), api, plan, plan.Key, dir)
			if (err == nil) == uncertain || (!uncertain && !outcome.Existing) || api.posts != before {
				t.Fatalf("header change caused a duplicate review: outcome=%+v err=%v", outcome, err)
			}
		}
	}
}

func TestReconciliationPlanCannotAuthorizeNewPublication(t *testing.T) {
	draft, current := fixture()
	for _, inline := range []bool{false, true} {
		fresh, err := PrepareWithOptions(draft, current, Options{Inline: inline})
		if err != nil {
			t.Fatal(err)
		}
		prior, err := PrepareForReconciliation(draft, Options{Inline: inline})
		if err != nil || prior.Key != fresh.Key || prior.Body != fresh.Body {
			t.Fatalf("retry rendering changed the confirmation: %v", err)
		}
		api := &fakeAPI{current: current.PullRequest}
		if outcome, err := Reconcile(context.Background(), api, prior, prior.Key); err != nil || outcome.Existing || api.posts != 0 {
			t.Fatalf("reconciliation must remain read-only: %+v %v", outcome, err)
		}
		if _, err := Publish(context.Background(), api, prior, prior.Key, t.TempDir()); err == nil || api.posts != 0 {
			t.Fatal("a reconciliation plan authorized a new POST")
		}
		if _, err := Publish(context.Background(), api, fresh, fresh.Key, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		api.current.State = "closed"
		api.current.Head.SHA = strings.Repeat("c", 40)
		if outcome, err := Reconcile(context.Background(), api, prior, prior.Key); err != nil || !outcome.Existing || api.posts != 1 {
			t.Fatalf("completed review did not reconcile after closure: %+v %v", outcome, err)
		}
	}
}
