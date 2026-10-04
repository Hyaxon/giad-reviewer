package publication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
)

type API interface {
	GetPullRequest(context.Context, githubauth.Repository, int) (githubapi.PullRequest, error)
	ReviewAuthor(context.Context, githubauth.Repository) (int64, error)
	ListReviews(context.Context, githubauth.Repository, int) ([]githubapi.Review, error)
	CreateReview(context.Context, githubauth.Repository, int, githubapi.ReviewSubmission) (githubapi.Review, error)
	ListReviewComments(context.Context, githubauth.Repository, int, int64) ([]githubapi.ReviewComment, error)
}

type Outcome struct {
	Review   githubapi.Review
	Existing bool
}

func confirmedPlans(plan Plan, confirmation string) ([]Plan, bool, error) {
	candidates := []Plan{plan}
	legacyConfirmed := false
	for previous := plan.legacy; previous != nil; previous = previous.legacy {
		candidates = append(candidates, *previous)
		legacyConfirmed = legacyConfirmed || confirmation == previous.Key
	}
	if confirmation == "" || (confirmation != plan.Key && !legacyConfirmed) {
		return nil, false, errors.New("confirmation does not match this publication; preview the draft again")
	}
	return candidates, legacyConfirmed, nil
}

// Reconcile recognizes a completed confirmed review without posting or requiring
// current PR revisions. A missing match returns an empty outcome; errors remain
// explicit so an unavailable retry check cannot become permission to resend.
func Reconcile(ctx context.Context, api API, plan Plan, confirmation string) (Outcome, error) {
	candidates, _, err := confirmedPlans(plan, confirmation)
	if err != nil {
		return Outcome{}, err
	}
	outcome, _, err := reconcile(ctx, api, plan, candidates)
	return outcome, err
}

func reconcile(ctx context.Context, api API, plan Plan, candidates []Plan) (Outcome, int64, error) {
	state, err := githubapi.ReviewState(plan.Event)
	if err != nil {
		return Outcome{}, 0, err
	}
	author, err := api.ReviewAuthor(ctx, plan.Repository)
	if err != nil {
		return Outcome{}, 0, err
	}
	reviews, err := api.ListReviews(ctx, plan.Repository, plan.Number)
	if err != nil {
		return Outcome{}, 0, err
	}
	for _, candidate := range candidates {
		for _, existing := range reviews {
			completed := existing.State == state || (plan.Event == "REQUEST_CHANGES" && existing.State == "DISMISSED")
			if existing.User.ID == author && completed && existing.CommitID == candidate.HeadSHA && existing.Body == candidate.Body {
				if err := verifyComments(ctx, api, candidate, existing.ID); err != nil {
					return Outcome{}, 0, err
				}
				return Outcome{Review: existing, Existing: true}, author, nil
			}
		}
	}
	return Outcome{}, author, nil
}

// Publish requires confirmation of the exact prepared body and revisions.
// An exclusive durable local attempt file prevents blind resends after crashes
// or ambiguous HTTP failures. Reconciliation is read-only and author-specific.
func Publish(ctx context.Context, api API, plan Plan, confirmation, stateDir string) (Outcome, error) {
	candidates, legacyConfirmed, err := confirmedPlans(plan, confirmation)
	if err != nil {
		return Outcome{}, err
	}
	outcome, author, err := reconcile(ctx, api, plan, candidates)
	if err != nil || outcome.Existing {
		return outcome, err
	}
	if plan.reconcileOnly {
		return Outcome{}, errors.New("new publication requires fresh PR and anchor validation")
	}
	if legacyConfirmed {
		return Outcome{}, errors.New("publication formatting changed; preview again and confirm the new hash before posting")
	}
	for _, previous := range candidates[1:] {
		oldAttempt := filepath.Join(stateDir, fmt.Sprintf("%d-%s.json", author, previous.Key))
		if _, err := os.Stat(oldAttempt); err == nil {
			return Outcome{}, errors.New("a previous publication attempt has no matching GitHub review; outcome unknown, refusing another POST. Check GitHub manually and retain the attempt record")
		} else if !errors.Is(err, os.ErrNotExist) {
			return Outcome{}, err
		}
	}
	latest, err := api.GetPullRequest(ctx, plan.Repository, plan.Number)
	if err != nil {
		return Outcome{}, err
	}
	if latest.State != "open" || latest.Head.SHA != plan.HeadSHA || latest.Base.SHA != plan.BaseSHA {
		return Outcome{}, errors.New("PR changed before publication; run a new review")
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return Outcome{}, err
	}
	// Include the authenticated author, so switching accounts cannot reuse an attempt.
	attemptPath := filepath.Join(stateDir, fmt.Sprintf("%d-%s.json", author, plan.Key))
	attempt, err := os.OpenFile(attemptPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return Outcome{}, errors.New("a previous publication attempt has no matching GitHub review; outcome unknown, refusing another POST. Check GitHub manually and retain the attempt record")
	}
	if err != nil {
		return Outcome{}, err
	}
	record := struct {
		Key        string `json:"key"`
		Repository string `json:"repository"`
		Number     int    `json:"number"`
		Head       string `json:"headSHA"`
	}{plan.Key, plan.Repository.Owner + "/" + plan.Repository.Name, plan.Number, plan.HeadSHA}
	writeErr := json.NewEncoder(attempt).Encode(record)
	syncErr := attempt.Sync()
	closeErr := attempt.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return Outcome{}, fmt.Errorf("record publication attempt: %w", err)
	}
	// Persist the directory entry before the network write, including across power loss.
	directory, err := os.Open(stateDir)
	if err != nil {
		return Outcome{}, err
	}
	syncErr = directory.Sync()
	closeErr = directory.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return Outcome{}, err
	}
	published, err := api.CreateReview(ctx, plan.Repository, plan.Number, githubapi.ReviewSubmission{CommitID: plan.HeadSHA, Body: plan.Body, Event: plan.Event, Comments: plan.Comments})
	if err != nil {
		var rejected *githubapi.HTTPError
		if errors.As(err, &rejected) && (rejected.Status == 401 || rejected.Status == 403 || rejected.Status == 404 || rejected.Status == 422) {
			// GitHub explicitly refused the write. It is safe to correct the problem
			// and retry; leave ambiguous failures guarded by the durable attempt.
			if cleanupErr := os.Remove(attemptPath); cleanupErr != nil {
				return Outcome{}, errors.Join(err, cleanupErr)
			}
			return Outcome{}, fmt.Errorf("GitHub rejected the review: %w", err)
		}
		return Outcome{}, fmt.Errorf("publication outcome may be unknown; rerun the same command to reconcile, without blindly reposting: %w", err)
	}
	if err := verifyComments(ctx, api, plan, published.ID); err != nil {
		return Outcome{}, fmt.Errorf("review was submitted, but inline verification failed; retry to reconcile: %w", err)
	}
	return Outcome{Review: published}, nil
}

func verifyComments(ctx context.Context, api API, plan Plan, id int64) error {
	if len(plan.Comments) == 0 {
		return nil
	}
	observed, err := api.ListReviewComments(ctx, plan.Repository, plan.Number, id)
	if err != nil {
		return fmt.Errorf("read published inline comments: %w", err)
	}
	expected := map[githubapi.InlineComment]int{}
	for _, comment := range plan.Comments {
		expected[comment]++
	}
	for _, comment := range observed {
		if comment.InReplyToID != 0 {
			continue
		}
		line := comment.OriginalLine
		if line == 0 {
			line = comment.Line
		}
		anchor := githubapi.InlineComment{Path: comment.Path, Line: line, Side: comment.Side, Body: comment.Body}
		if comment.OriginalCommitID != plan.HeadSHA || expected[anchor] == 0 {
			return errors.New("matching review has different inline comments; inspect GitHub manually before retrying")
		}
		expected[anchor]--
	}
	for _, count := range expected {
		if count != 0 {
			return errors.New("matching review is missing inline comments; inspect GitHub manually before retrying")
		}
	}
	return nil
}
