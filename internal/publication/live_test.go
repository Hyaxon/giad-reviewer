package publication

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
)

// This opt-in integration check cannot publish, even if reconciliation fails.
// Run with GIAD_TEST_PUBLISHED_DRAFT pointing to an already-published inline draft.
type readOnlyPublicationAPI struct{ *githubapi.Client }

func (readOnlyPublicationAPI) CreateReview(context.Context, githubauth.Repository, int, githubapi.ReviewSubmission) (githubapi.Review, error) {
	return githubapi.Review{}, errors.New("read-only reconciliation check blocked a publication attempt")
}

func TestPublishedInlineReviewReconciliationReadOnly(t *testing.T) {
	path := os.Getenv("GIAD_TEST_PUBLISHED_DRAFT")
	if path == "" {
		t.Skip("set GIAD_TEST_PUBLISHED_DRAFT to check an existing inline review without posting")
	}
	draft, err := LoadDraft(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := githubapi.NewClient(githubauth.UserAuth{})
	plan, err := PrepareForReconciliation(draft, Options{Event: "COMMENT", Inline: true})
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := Reconcile(ctx, readOnlyPublicationAPI{client}, plan, plan.Key)
	if err != nil || !outcome.Existing || outcome.Review.ID <= 0 {
		t.Fatalf("read-only reconciliation failed: outcome=%+v err=%v", outcome, err)
	}
	t.Logf("Verified existing review %d: %s", outcome.Review.ID, outcome.Review.URL)
}
