package githubapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/hyaxon/magi-agents/internal/githubauth"
)

type fakeAuth struct{}

func (fakeAuth) Token(context.Context, githubauth.Repository) (string, error) {
	return "test-token", nil
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetPRContext(t *testing.T) {
	for _, mode := range []string{"complete", "issue-error", "stale"} {
		t.Run(mode, func(t *testing.T) {
			metadataCalls, filePages, issuePages := 0, 0, 0
			client := NewClient(fakeAuth{})
			client.http.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Fatal("incorrect authenticated destination")
				}
				var payload any
				var raw string
				switch {
				case r.URL.Path == "/graphql":
					issuePages++
					if r.Method != http.MethodPost {
						t.Fatal("GraphQL request must be POST")
					}
					if mode == "issue-error" {
						raw = `{"errors":[{"message":"unavailable"}]}`
						break
					}
					var request struct {
						Variables struct {
							After *string `json:"after"`
						} `json:"variables"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Fatal(err)
					}
					if issuePages == 1 && request.Variables.After != nil {
						t.Fatal("unexpected initial cursor")
					}
					if issuePages == 2 && (request.Variables.After == nil || *request.Variables.After != "cursor1") {
						t.Fatal("missing next-page cursor")
					}
					raw = fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"closingIssuesReferences":{"nodes":[{"number":%d,"title":"Requirement","body":"Must work","repository":{"nameWithOwner":"owner/repo"}}],"pageInfo":{"hasNextPage":%t,"endCursor":"cursor1"}}}}}}`, issuePages, issuePages == 1)
				case strings.HasSuffix(r.URL.Path, "/files"):
					filePages++
					if r.URL.Query().Get("page") != fmt.Sprint(filePages) {
						t.Fatal("incorrect files pagination")
					}
					count := 100
					if filePages == 2 {
						count = 1
					}
					files := make([]ChangedFile, count)
					for i := range files {
						files[i] = ChangedFile{Filename: fmt.Sprintf("page%d-file%d.go", filePages, i), Status: "modified"}
					}
					payload = files
				case r.Header.Get("Accept") == "application/vnd.github.diff":
					raw = "diff --git a/test.md b/test.md\n"
				default:
					metadataCalls++
					head := "head1"
					if mode == "stale" && metadataCalls == 2 {
						head = "head2"
					}
					payload = PullRequest{Number: 2, ChangedFiles: 101, Base: Ref{SHA: "base"}, Head: Ref{SHA: head}}
				}
				if payload != nil {
					data, err := json.Marshal(payload)
					if err != nil {
						t.Fatal(err)
					}
					raw = string(data)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(raw)), Header: make(http.Header)}, nil
			})
			result, err := client.GetPRContext(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"}, 2)
			if mode == "stale" {
				if err == nil || !strings.Contains(err.Error(), "revisions changed") {
					t.Fatalf("expected stale revision error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Files) != 101 || !strings.HasPrefix(result.Diff, "diff --git") || metadataCalls != 2 {
				t.Fatal("incomplete PR context")
			}
			if mode == "issue-error" {
				if result.IssuesError == "" || len(result.LinkedIssues) != 0 {
					t.Fatal("issue failure must remain explicit")
				}
			} else if result.IssuesError != "" || len(result.LinkedIssues) != 2 {
				t.Fatal("incomplete linked issues")
			}
		})
	}
}
