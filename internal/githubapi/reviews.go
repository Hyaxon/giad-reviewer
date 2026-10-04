package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hyaxon/giad/internal/githubauth"
)

type Review struct {
	ID       int64  `json:"id"`
	URL      string `json:"html_url"`
	Body     string `json:"body"`
	State    string `json:"state"`
	CommitID string `json:"commit_id"`
	User     struct {
		ID int64 `json:"id"`
	} `json:"user"`
}

type InlineComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

type ReviewSubmission struct {
	CommitID string          `json:"commit_id"`
	Body     string          `json:"body"`
	Event    string          `json:"event"`
	Comments []InlineComment `json:"comments,omitempty"`
}

type ReviewComment struct {
	ID               int64  `json:"id"`
	ReviewID         int64  `json:"pull_request_review_id"`
	Path             string `json:"path"`
	Line             int    `json:"line"`
	OriginalLine     int    `json:"original_line"`
	Side             string `json:"side"`
	Body             string `json:"body"`
	OriginalCommitID string `json:"original_commit_id"`
	InReplyToID      int64  `json:"in_reply_to_id"`
}

func ReviewState(event string) (string, error) {
	switch event {
	case "COMMENT":
		return "COMMENTED", nil
	case "REQUEST_CHANGES":
		return "CHANGES_REQUESTED", nil
	default:
		return "", errors.New("review event must be COMMENT or REQUEST_CHANGES")
	}
}

func reviewsPath(repo githubauth.Repository, number int) string {
	return fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number)
}
func (c *Client) ReviewAuthor(ctx context.Context, repo githubauth.Repository) (int64, error) {
	if auth, ok := c.auth.(githubauth.AuthorProvider); ok {
		return auth.ReviewAuthor(ctx, repo)
	}
	data, err := c.request(ctx, repo, http.MethodGet, "/user", "application/vnd.github+json", nil)
	if err != nil {
		return 0, err
	}
	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		return 0, err
	}
	if user.ID <= 0 {
		return 0, errors.New("invalid authenticated GitHub user")
	}
	return user.ID, nil
}
func (c *Client) ListReviews(ctx context.Context, repo githubauth.Repository, number int) ([]Review, error) {
	var result []Review
	for page := 1; page <= 100; page++ {
		data, err := c.request(ctx, repo, http.MethodGet, fmt.Sprintf("%s?per_page=100&page=%d", reviewsPath(repo, number), page), "application/vnd.github+json", nil)
		if err != nil {
			return nil, err
		}
		var batch []Review
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, err
		}
		result = append(result, batch...)
		if len(batch) < 100 {
			return result, nil
		}
	}
	return nil, errors.New("review listing exceeds 10000 entries; refusing incomplete retry check")
}

// CreateReview submits the event, summary and inline comments in a single write.
func (c *Client) CreateReview(ctx context.Context, repo githubauth.Repository, number int, submission ReviewSubmission) (Review, error) {
	state, err := ReviewState(submission.Event)
	if err != nil {
		return Review{}, err
	}
	if len(submission.Comments) > 20 {
		return Review{}, errors.New("at most 20 inline comments are supported")
	}
	for _, comment := range submission.Comments {
		if comment.Path == "" || comment.Line < 1 || comment.Side != "RIGHT" || comment.Body == "" || len(comment.Body) > 60000 {
			return Review{}, errors.New("invalid inline comment")
		}
	}
	payload, err := json.Marshal(submission)
	if err != nil {
		return Review{}, err
	}
	data, err := c.request(ctx, repo, http.MethodPost, reviewsPath(repo, number), "application/vnd.github+json", bytes.NewReader(payload))
	if err != nil {
		return Review{}, err
	}
	var review Review
	if err := json.Unmarshal(data, &review); err != nil {
		return review, err
	}
	if review.ID <= 0 || review.URL == "" || review.State != state || review.CommitID != submission.CommitID || review.Body != submission.Body {
		return Review{}, errors.New("unexpected GitHub publication response")
	}
	return review, nil
}

func (c *Client) ListReviewComments(ctx context.Context, repo githubauth.Repository, number int, reviewID int64) ([]ReviewComment, error) {
	var result []ReviewComment
	detailsRead := 0
	for page := 1; page <= 100; page++ {
		endpoint := fmt.Sprintf("%s/%d/comments?per_page=100&page=%d", reviewsPath(repo, number), reviewID, page)
		data, err := c.request(ctx, repo, http.MethodGet, endpoint, "application/vnd.github+json", nil)
		if err != nil {
			return nil, err
		}
		var batch []ReviewComment
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, err
		}
		for i, comment := range batch {
			// This endpoint can return only legacy diff positions, even when the
			// comment was submitted using line/side. Never interpret position as a
			// source line; the individual-comment endpoint supplies modern anchors.
			if comment.InReplyToID != 0 || ((comment.Line > 0 || comment.OriginalLine > 0) && comment.Side != "" && comment.OriginalCommitID != "") {
				continue
			}
			if comment.ID <= 0 {
				return nil, errors.New("review comment is missing its ID and line anchor")
			}
			detailsRead++
			if detailsRead > 20 {
				return nil, errors.New("review exceeds 20 comments requiring individual anchor reads")
			}
			detailPath := fmt.Sprintf("/repos/%s/%s/pulls/comments/%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), comment.ID)
			detailData, err := c.request(ctx, repo, http.MethodGet, detailPath, "application/vnd.github+json", nil)
			if err != nil {
				return nil, fmt.Errorf("read inline comment %d: %w", comment.ID, err)
			}
			var detail ReviewComment
			if err := json.Unmarshal(detailData, &detail); err != nil {
				return nil, err
			}
			if detail.ID != comment.ID || detail.ReviewID != reviewID || detail.InReplyToID != 0 || detail.Side == "" || (detail.OriginalLine < 1 && detail.Line < 1) {
				return nil, errors.New("individual review comment returned invalid identity or line anchor")
			}
			batch[i] = detail
		}
		result = append(result, batch...)
		if len(batch) < 100 {
			return result, nil
		}
	}
	return nil, errors.New("inline comment listing exceeds 10000 entries; refusing incomplete retry check")
}
