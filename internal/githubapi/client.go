package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hyaxon/giad/internal/githubauth"
)

type Client struct {
	auth githubauth.Provider
	http *http.Client
}

// HTTPError represents an explicit GitHub response, rather than an ambiguous
// connection failure. Preserve bounded validation details for publication errors.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("GitHub returned HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("GitHub returned HTTP %d", e.Status)
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
	req.Header.Set("User-Agent", "giad")
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
			return nil, &HTTPError{Status: resp.StatusCode, Message: "resource does not exist or this account cannot access it; check repository and PR number"}
		}
		var details struct {
			Message string            `json:"message"`
			Errors  []json.RawMessage `json:"errors"`
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = json.Unmarshal(data, &details)
		message := details.Message
		for _, raw := range details.Errors[:min(len(details.Errors), 3)] {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				var entry struct{ Message, Field, Code string }
				if json.Unmarshal(raw, &entry) == nil {
					value = entry.Message
					if value == "" {
						value = strings.TrimSpace(entry.Field + " " + entry.Code)
					}
				}
			}
			if value != "" {
				message += "; " + value
			}
		}
		if len(message) > 2000 {
			message = message[:2000]
		}
		return nil, &HTTPError{Status: resp.StatusCode, Message: message}
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
