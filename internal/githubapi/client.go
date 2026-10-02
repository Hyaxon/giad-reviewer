package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hyaxon/agentic-review/internal/githubauth"
)

type Client struct {
	auth githubauth.Provider
	http *http.Client
}

func NewClient(auth githubauth.Provider) *Client {
	return &Client{
		auth: auth,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) GetPullRequest(
	ctx context.Context,
	repo githubauth.Repository,
	number int,
) (PullRequest, error) {
	if repo.Host != "" && repo.Host != "github.com" {
		return PullRequest{}, fmt.Errorf("only github.com is supported")
	}
	if repo.Owner == "" || repo.Name == "" || number <= 0 {
		return PullRequest{}, fmt.Errorf("repository owner, name, and positive PR number are required")
	}

	endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number)
	data, err := c.request(ctx, repo, http.MethodGet, endpoint, "application/vnd.github+json", nil)
	if err != nil {
		return PullRequest{}, fmt.Errorf("fetch PR: %w", err)
	}
	var pr PullRequest
	if err := json.Unmarshal(data, &pr); err != nil {
		return PullRequest{}, fmt.Errorf("decode PR: %w", err)
	}

	return pr, nil
}

// request keeps credentials on api.github.com and bounds response memory use.
func (c *Client) request(ctx context.Context, repo githubauth.Repository, method, path, accept string, body io.Reader) ([]byte, error) {
	if repo.Host != "" && repo.Host != "github.com" {
		return nil, fmt.Errorf("only github.com is supported")
	}
	token, err := c.auth.Token(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, body)
	if err != nil {
		return nil, fmt.Errorf("create GitHub request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "agentic-review")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("GitHub returned HTTP 404: resource does not exist or this account cannot access it; check repository and PR number")
		}
		return nil, fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
	}
	const maxResponse = 16 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read GitHub response: %w", err)
	}
	if len(data) > maxResponse {
		return nil, fmt.Errorf("GitHub response exceeds 16 MiB limit; refusing incomplete context")
	}
	return data, nil
}
