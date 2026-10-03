package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hyaxon/giad/internal/githubauth"
)

type ChangedFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
}

type PRContext struct {
	PullRequest  PullRequest
	Files        []ChangedFile
	Diff         string
	LinkedIssues []LinkedIssue
	// A retrieval failure is not equivalent to finding no linked issues.
	IssuesError string
}

// GetPRContext reads context without checking out code or publishing anything.
func (c *Client) GetPRContext(ctx context.Context, repo githubauth.Repository, number int) (PRContext, error) {
	pr, err := c.GetPullRequest(ctx, repo, number)
	if err != nil {
		return PRContext{}, err
	}
	if pr.ChangedFiles > 3000 {
		return PRContext{}, fmt.Errorf("PR exceeds GitHub's 3000-file listing limit")
	}
	result := PRContext{PullRequest: pr}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number)
	for page := 1; page <= 30; page++ {
		data, err := c.request(ctx, repo, http.MethodGet, fmt.Sprintf("%s/files?per_page=100&page=%d", path, page), "application/vnd.github+json", nil)
		if err != nil {
			return PRContext{}, fmt.Errorf("fetch changed files: %w", err)
		}
		var files []ChangedFile
		if err := json.Unmarshal(data, &files); err != nil {
			return PRContext{}, fmt.Errorf("decode changed files: %w", err)
		}
		result.Files = append(result.Files, files...)
		if len(files) < 100 || len(result.Files) >= pr.ChangedFiles {
			break
		}
	}
	if len(result.Files) != pr.ChangedFiles {
		return PRContext{}, fmt.Errorf("changed-file count differs from PR metadata; PR may have changed, retry")
	}
	data, err := c.request(ctx, repo, http.MethodGet, path, "application/vnd.github.diff", nil)
	if err != nil {
		return PRContext{}, fmt.Errorf("fetch diff: %w", err)
	}
	result.Diff = string(data)
	result.LinkedIssues, err = c.GetLinkedIssues(ctx, repo, number)
	if err != nil {
		result.IssuesError = err.Error()
	}
	latest, err := c.GetPullRequest(ctx, repo, number)
	if err != nil {
		return PRContext{}, err
	}
	if latest.Head.SHA != pr.Head.SHA || latest.Base.SHA != pr.Base.SHA || latest.ChangedFiles != pr.ChangedFiles {
		return PRContext{}, fmt.Errorf("PR revisions changed while fetching context; retry")
	}
	return result, nil
}
