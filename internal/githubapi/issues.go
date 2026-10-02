package githubapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hyaxon/agentic-review/internal/githubauth"
)

type LinkedIssue struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	URL        string `json:"url"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

const linkedIssuesQuery = `query($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      closingIssuesReferences(first: 100, after: $after) {
        nodes { number title body url repository { nameWithOwner } }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`

func (c *Client) GetLinkedIssues(ctx context.Context, repo githubauth.Repository, number int) ([]LinkedIssue, error) {
	var issues []LinkedIssue
	var after *string
	for page := 0; page < 10; page++ {
		payload, err := json.Marshal(map[string]any{"query": linkedIssuesQuery, "variables": map[string]any{
			"owner": repo.Owner, "repo": repo.Name, "number": number, "after": after,
		}})
		if err != nil {
			return nil, err
		}
		data, err := c.request(ctx, repo, http.MethodPost, "/graphql", "application/vnd.github+json", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("fetch linked issues: %w", err)
		}
		var response struct {
			Errors []json.RawMessage `json:"errors"`
			Data   struct {
				Repository *struct {
					PullRequest *struct {
						Issues *struct {
							Nodes    []*LinkedIssue `json:"nodes"`
							PageInfo struct {
								HasNextPage bool    `json:"hasNextPage"`
								EndCursor   *string `json:"endCursor"`
							} `json:"pageInfo"`
						} `json:"closingIssuesReferences"`
					} `json:"pullRequest"`
				} `json:"repository"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("decode linked issues: %w", err)
		}
		if len(response.Errors) != 0 {
			return nil, fmt.Errorf("GitHub GraphQL returned errors; linked-issue context is unavailable or incomplete")
		}
		if response.Data.Repository == nil || response.Data.Repository.PullRequest == nil || response.Data.Repository.PullRequest.Issues == nil {
			return nil, fmt.Errorf("GitHub did not return the linked-issue relationship; check repository access")
		}
		connection := response.Data.Repository.PullRequest.Issues
		for _, issue := range connection.Nodes {
			if issue == nil {
				return nil, fmt.Errorf("a linked issue is inaccessible")
			}
			issues = append(issues, *issue)
		}
		if !connection.PageInfo.HasNextPage {
			return issues, nil
		}
		next := connection.PageInfo.EndCursor
		if next == nil || *next == "" || (after != nil && *next == *after) {
			return nil, fmt.Errorf("linked-issue pagination did not advance")
		}
		after = next
	}
	return nil, fmt.Errorf("linked issues exceed the 1000-issue retrieval limit")
}
