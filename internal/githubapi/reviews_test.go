package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hyaxon/giad/internal/githubauth"
)

func TestCreateReviewOnlyCommentsOnPinnedCommit(t *testing.T) {
	client := NewClient(fakeAuth{})
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Path != "/repos/owner/repo/pulls/2/reviews" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("wrong publication destination")
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["event"] != "COMMENT" || request["commit_id"] != "head" || request["body"] != "previewed body" || len(request) != 3 {
			t.Fatalf("unsafe review request: %v", request)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":42,"html_url":"https://github.com/owner/repo/pull/2#pullrequestreview-42","state":"COMMENTED","commit_id":"head","body":"previewed body"}`)), Header: make(http.Header)}, nil
	})
	if _, err := client.CreateCommentReview(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2, "head", "previewed body"); err != nil {
		t.Fatal(err)
	}
}

func TestReviewListingPaginates(t *testing.T) {
	client := NewClient(fakeAuth{})
	pages := 0
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		pages++
		count := 100
		if pages == 2 {
			count = 1
		}
		data, _ := json.Marshal(make([]Review, count))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})
	reviews, err := client.ListReviews(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2)
	if err != nil || len(reviews) != 101 || pages != 2 {
		t.Fatalf("incomplete retry coverage: %d pages=%d err=%v", len(reviews), pages, err)
	}
}

func TestCreateRequestChangesWithInlineComments(t *testing.T) {
	client := NewClient(fakeAuth{})
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		var submission ReviewSubmission
		if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
			t.Fatal(err)
		}
		if submission.Event != "REQUEST_CHANGES" || submission.CommitID != "head" || len(submission.Comments) != 1 || submission.Comments[0].Path != "auth.go" || submission.Comments[0].Line != 42 || submission.Comments[0].Side != "RIGHT" || submission.Comments[0].Body != "finding" {
			t.Fatalf("wrong inline request: %+v", submission)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":42,"html_url":"https://github.com/owner/repo/pull/2#pullrequestreview-42","state":"CHANGES_REQUESTED","commit_id":"head","body":"summary"}`)), Header: make(http.Header)}, nil
	})
	_, err := client.CreateReview(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2, ReviewSubmission{CommitID: "head", Body: "summary", Event: "REQUEST_CHANGES", Comments: []InlineComment{{Path: "auth.go", Line: 42, Side: "RIGHT", Body: "finding"}}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListReviewCommentsPaginatesAndKeepsOriginalAnchors(t *testing.T) {
	client := NewClient(fakeAuth{})
	pages := 0
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		pages++
		if r.URL.Path != "/repos/owner/repo/pulls/2/reviews/42/comments" || r.URL.Query().Get("page") != fmt.Sprint(pages) {
			t.Fatalf("wrong comment endpoint: %s", r.URL)
		}
		count := 100
		if pages == 2 {
			count = 1
		}
		batch := make([]ReviewComment, count)
		for i := range batch {
			batch[i] = ReviewComment{Path: "auth.go", OriginalLine: 42, OriginalCommitID: "head", Side: "RIGHT", Body: "finding"}
		}
		data, _ := json.Marshal(batch)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: make(http.Header)}, nil
	})
	comments, err := client.ListReviewComments(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2, 42)
	if err != nil || len(comments) != 101 || comments[0].OriginalLine != 42 {
		t.Fatalf("inline anchor lost: comments=%+v err=%v", comments, err)
	}
}

func TestReviewValidationFailureIncludesGitHubReason(t *testing.T) {
	client := NewClient(fakeAuth{})
	client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 422, Body: io.NopCloser(strings.NewReader(`{"message":"Validation Failed","errors":["Can not request changes on your own pull request"]}`)), Header: make(http.Header)}, nil
	})
	_, err := client.CreateReview(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2, ReviewSubmission{CommitID: "head", Body: "summary", Event: "REQUEST_CHANGES"})
	if err == nil || !strings.Contains(err.Error(), "own pull request") {
		t.Fatalf("GitHub refusal reason was lost: %v", err)
	}
}

// The live review-list endpoint omits modern anchors even for line/side writes.
// Diff positions intentionally differ from source lines in this regression case.
func TestReviewListingResolvesLegacyPositionsThroughCommentDetails(t *testing.T) {
	for _, mode := range []string{"complete", "detail unavailable", "wrong review", "wrong comment", "missing anchor"} {
		t.Run(mode, func(t *testing.T) {
			client := NewClient(fakeAuth{})
			calls := 0
			client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet {
					t.Fatal("verification made a write request")
				}
				payload := `[{"id":123,"pull_request_review_id":42,"path":"auth.go","position":7,"original_position":7,"original_commit_id":"head","body":"finding"}]`
				if calls == 1 {
					if r.URL.Path != "/repos/owner/repo/pulls/2/reviews/42/comments" {
						t.Fatalf("wrong list endpoint: %s", r.URL)
					}
				} else {
					if calls != 2 || r.URL.Path != "/repos/owner/repo/pulls/comments/123" {
						t.Fatalf("wrong detail endpoint: %s", r.URL)
					}
					if mode == "detail unavailable" {
						return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
					}
					payload = `{"id":123,"pull_request_review_id":42,"path":"auth.go","line":37,"original_line":37,"side":"RIGHT","original_commit_id":"head","body":"finding"}`
					switch mode {
					case "wrong review":
						payload = strings.Replace(payload, `"pull_request_review_id":42`, `"pull_request_review_id":99`, 1)
					case "wrong comment":
						payload = strings.Replace(payload, `"id":123`, `"id":999`, 1)
					case "missing anchor":
						payload = strings.Replace(payload, `"side":"RIGHT"`, `"side":""`, 1)
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
			})
			comments, err := client.ListReviewComments(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2, 42)
			if (err == nil) != (mode == "complete") || calls != 2 {
				t.Fatalf("unexpected verification result: comments=%+v calls=%d err=%v", comments, calls, err)
			}
			if mode == "complete" && (comments[0].OriginalLine != 37 || comments[0].Side != "RIGHT") {
				t.Fatal("diff position was used as the source anchor")
			}
		})
	}
}
