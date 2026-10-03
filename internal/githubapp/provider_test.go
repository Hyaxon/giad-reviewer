package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hyaxon/giad/internal/config"
	"github.com/hyaxon/giad/internal/githubauth"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testProvider(t *testing.T) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := NewProvider(config.AppIdentity{AppID: 123, ClientID: "client", InstallationID: 456, PrivateKey: path})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

const installedJSON = `{"id":456,"app_id":123,"account":{"login":"owner"},"permissions":{"contents":"read","issues":"read","pull_requests":"write"}}`

func tokenJSON(expires time.Time, token string) string {
	return fmt.Sprintf(`{"token":%q,"expires_at":%q,"permissions":{"contents":"read","issues":"read","pull_requests":"write"}}`, token, expires.Format(time.RFC3339))
}

func checkJWT(t *testing.T, p *Provider, r *http.Request) {
	t.Helper()
	if r.URL.Host != "api.github.com" || r.URL.Scheme != "https" || r.Header.Get("X-GitHub-Api-Version") == "" {
		t.Error("authentication request left its fixed API origin")
	}
	parsed, err := jwt.Parse(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), func(token *jwt.Token) (any, error) {
		return &p.key.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer("client"))
	if err != nil || !parsed.Valid {
		t.Errorf("App request did not carry a valid signed JWT: %v", err)
	}
}

func TestRepositoryTokensScopeCacheAndRefresh(t *testing.T) {
	p := testProvider(t)
	now := time.Now().Truncate(time.Second)
	p.now = func() time.Time { return now }
	lookups, exchanges := 0, 0
	fail := false
	p.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		checkJWT(t, p, r)
		switch r.URL.Path {
		case "/repos/owner/repo/installation", "/repos/owner/other/installation":
			lookups++
			return response(200, installedJSON), nil
		case "/app/installations/456/access_tokens":
			exchanges++
			if r.Method != "POST" {
				t.Error("exchange was not a POST")
			}
			var payload struct {
				Repositories []string          `json:"repositories"`
				Permissions  map[string]string `json:"permissions"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if len(payload.Repositories) != 1 || len(payload.Permissions) != 3 || payload.Permissions["contents"] != "read" || payload.Permissions["issues"] != "read" || payload.Permissions["pull_requests"] != "write" {
				t.Error("token was not restricted to one repository and required permissions")
			}
			if fail {
				return response(403, `{"message":"secret-token"}`), nil
			}
			return response(201, tokenJSON(now.Add(time.Hour), fmt.Sprintf("token-%d", exchanges))), nil
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			return response(404, `{}`), nil
		}
	})
	repo := githubauth.Repository{Owner: "owner", Name: "repo"}
	first, err := p.Token(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	// Concurrent readers reuse one exchange instead of minting multiple tokens.
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			got, err := p.Token(context.Background(), repo)
			if err != nil || got != first {
				t.Errorf("cached token unavailable: %v", err)
			}
		}()
	}
	group.Wait()
	if lookups != 1 || exchanges != 1 {
		t.Fatalf("cache was not reused: %d lookups, %d exchanges", lookups, exchanges)
	}
	now = now.Add(59 * time.Minute)
	refreshed, err := p.Token(context.Background(), repo)
	if err != nil || refreshed == first || exchanges != 2 {
		t.Fatalf("near-expiry token was not refreshed: %v", err)
	}
	if _, err := p.Token(context.Background(), githubauth.Repository{Owner: "owner", Name: "other"}); err != nil || exchanges != 3 {
		t.Fatalf("token reused across repositories: %v", err)
	}
	now = now.Add(time.Hour)
	fail = true
	got, err := p.Token(context.Background(), repo)
	if got != "" || err == nil || strings.Contains(err.Error(), "secret-token") || len(p.tokens) != 1 {
		t.Fatalf("failed refresh returned stale credentials or exposed response: %v", err)
	}
}

func TestInstallationGuardsAndInvalidTokens(t *testing.T) {
	p := testProvider(t)
	for _, mode := range []string{"different installation", "different app", "different owner", "suspended", "missing permission", "expired token", "empty token", "wrong token permissions", "invalid JSON", "oversized response", "unauthorized"} {
		t.Run(mode, func(t *testing.T) {
			p.tokens = map[string]accessToken{}
			posts := 0
			p.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == "GET" {
					body := installedJSON
					switch mode {
					case "different installation":
						body = strings.Replace(body, `"id":456`, `"id":789`, 1)
					case "different app":
						body = strings.Replace(body, `"app_id":123`, `"app_id":789`, 1)
					case "different owner":
						body = strings.Replace(body, `"owner"`, `"elsewhere"`, 1)
					case "suspended":
						body = strings.Replace(body, `"id":456`, `"id":456,"suspended_at":"2026-01-01T00:00:00Z"`, 1)
					case "missing permission":
						body = strings.Replace(body, `"pull_requests":"write"`, `"pull_requests":"read"`, 1)
					case "unauthorized":
						return response(401, `{"message":"secret"}`), nil
					}
					return response(200, body), nil
				}
				posts++
				body := tokenJSON(time.Now().Add(time.Hour), "secret")
				switch mode {
				case "expired token":
					body = tokenJSON(time.Now().Add(-time.Minute), "secret")
				case "empty token":
					body = tokenJSON(time.Now().Add(time.Hour), "")
				case "wrong token permissions":
					body = strings.Replace(body, `"contents":"read"`, `"contents":"write"`, 1)
				case "invalid JSON":
					body = "secret bad JSON"
				case "oversized response":
					body = strings.Repeat("s", (1<<20)+1)
				}
				return response(201, body), nil
			})
			got, err := p.Token(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"})
			if err == nil || got != "" || len(p.tokens) != 0 || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe auth result: %v", err)
			}
			if (strings.HasPrefix(mode, "different") || mode == "suspended" || mode == "missing permission" || mode == "unauthorized") && posts != 0 {
				t.Error("invalid installation reached token exchange")
			}
		})
	}
}

func TestAppStatusAndBotAuthor(t *testing.T) {
	p := testProvider(t)
	for _, mode := range []string{"valid", "wrong app", "wrong client", "wrong bot", "not bot"} {
		t.Run(mode, func(t *testing.T) {
			p.tokens = map[string]accessToken{}
			p.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if strings.HasPrefix(r.URL.Path, "/users/") {
					if r.URL.Path != "/users/example[bot]" || r.Header.Get("Authorization") != "Bearer secret" {
						t.Error("incorrect bot lookup")
					}
					body := `{"id":999,"login":"example[bot]","type":"Bot"}`
					if mode == "wrong bot" {
						body = strings.Replace(body, "example", "another", 1)
					} else if mode == "not bot" {
						body = strings.Replace(body, `"Bot"`, `"User"`, 1)
					}
					return response(200, body), nil
				}
				checkJWT(t, p, r)
				switch r.URL.Path {
				case "/app":
					body := `{"id":123,"client_id":"client","slug":"example"}`
					if mode == "wrong app" {
						body = strings.Replace(body, "123", "789", 1)
					} else if mode == "wrong client" {
						body = strings.Replace(body, `"client_id":"client"`, `"client_id":"other"`, 1)
					}
					return response(200, body), nil
				case "/app/installations/456", "/repos/owner/repo/installation":
					return response(200, installedJSON), nil
				case "/app/installations/456/access_tokens":
					return response(201, tokenJSON(time.Now().Add(time.Hour), "secret")), nil
				default:
					t.Errorf("unexpected endpoint %s", r.URL.Path)
					return response(404, `{}`), nil
				}
			})
			status, err := p.Verify(context.Background())
			if (err == nil) != (mode == "valid") {
				t.Fatalf("status accepted mismatched credentials: %v", err)
			}
			if mode == "valid" && (status.Login != "example[bot]" || status.Account != "owner" || status.InstallationID != 456 || len(p.tokens) != 0) {
				t.Fatalf("status or token isolation incorrect: %+v", status)
			}
			id, err := p.ReviewAuthor(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"})
			if (err == nil) != (mode == "valid") || (err == nil && id != 999) {
				t.Fatalf("publication author is incorrect: id=%d err=%v", id, err)
			}
		})
	}
}

func TestTokenRejectsInvalidTargetAndCancellationBeforeNetwork(t *testing.T) {
	p := testProvider(t)
	p.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("invalid request reached network")
		return response(500, `{}`), nil
	})
	for _, repo := range []githubauth.Repository{{}, {Owner: "owner", Name: ".."}, {Owner: "owner", Name: "repo", Host: "elsewhere"}} {
		if _, err := p.Token(context.Background(), repo); err == nil {
			t.Error("invalid repository accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Token(ctx, githubauth.Repository{Owner: "owner", Name: "repo"}); err != context.Canceled {
		t.Errorf("cancellation was lost: %v", err)
	}
}

func TestAuthDoesNotForwardCredentialsOrExposeTransportErrors(t *testing.T) {
	p := testProvider(t)
	for _, mode := range []string{"redirect", "transport error"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			p.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != "api.github.com" {
					t.Error("credentials followed a redirect to another origin")
				}
				if mode == "transport error" {
					return nil, fmt.Errorf("secret transport detail %s", r.Header.Get("Authorization"))
				}
				resp := response(302, "secret response")
				resp.Header.Set("Location", "https://elsewhere.example/secret")
				return resp, nil
			})
			_, err := p.Token(context.Background(), githubauth.Repository{Owner: "owner", Name: "repo"})
			if err == nil || strings.Contains(err.Error(), "secret") || calls != 1 {
				t.Fatalf("unsafe authentication failure: calls=%d err=%v", calls, err)
			}
		})
	}
}
