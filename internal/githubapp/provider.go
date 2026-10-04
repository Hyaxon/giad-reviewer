package githubapp

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hyaxon/giad/internal/config"
	"github.com/hyaxon/giad/internal/githubauth"
)

// Provider keeps keys and installation tokens in the trusted host's memory.
// One instance is shared by API and checkout operations within a CLI invocation.
type Provider struct {
	identity  config.AppIdentity
	key       *rsa.PrivateKey
	http      *http.Client
	now       func() time.Time
	mu        sync.Mutex
	tokens    map[string]accessToken
	refreshes map[string]chan struct{}
}

var _ githubauth.Provider = (*Provider)(nil)
var _ githubauth.AuthorProvider = (*Provider)(nil)

type accessToken struct {
	Token       string            `json:"token"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Permissions map[string]string `json:"permissions"`
}

type app struct {
	ID       int64  `json:"id"`
	ClientID string `json:"client_id"`
	Slug     string `json:"slug"`
}

type installation struct {
	ID          int64             `json:"id"`
	AppID       int64             `json:"app_id"`
	SuspendedAt *time.Time        `json:"suspended_at"`
	Permissions map[string]string `json:"permissions"`
	Account     struct {
		Login string `json:"login"`
	} `json:"account"`
}

type Status struct {
	Login          string
	Account        string
	InstallationID int64
}

func NewProvider(identity config.AppIdentity) (*Provider, error) {
	if identity.AppID <= 0 || identity.InstallationID <= 0 || strings.TrimSpace(identity.ClientID) == "" || identity.ClientID != strings.TrimSpace(identity.ClientID) {
		return nil, fmt.Errorf("invalid GitHub App identity")
	}
	key, err := LoadPrivateKey(identity.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &Provider{identity: identity, key: key, now: time.Now, tokens: map[string]accessToken{}, refreshes: map[string]chan struct{}{}, http: &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

var namePart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// Token requires an explicit repository and never falls back to personal auth.
// Tokens are restricted to that repository and GIAD's required permissions.
func (p *Provider) Token(ctx context.Context, repo githubauth.Repository) (string, error) {
	if repo.Host != "" && repo.Host != "github.com" {
		return "", fmt.Errorf("GitHub App authentication supports github.com only")
	}
	for _, part := range []string{repo.Owner, repo.Name} {
		if !namePart.MatchString(part) || part == "." || part == ".." {
			return "", fmt.Errorf("GitHub App authentication requires a repository owner and name")
		}
	}
	cacheKey := strings.ToLower(repo.Owner + "/" + repo.Name)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		p.mu.Lock()
		if cached, ok := p.tokens[cacheKey]; ok && cached.ExpiresAt.After(p.now().Add(2*time.Minute)) {
			p.mu.Unlock()
			return cached.Token, nil
		}
		if done, ok := p.refreshes[cacheKey]; ok {
			p.mu.Unlock()
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-done:
				// Recheck the cache; a failed or canceled refresh can be retried
				// using this caller's context.
				continue
			}
		}
		// Discard an expired entry even when refresh subsequently fails.
		delete(p.tokens, cacheKey)
		done := make(chan struct{})
		p.refreshes[cacheKey] = done
		p.mu.Unlock()

		// Only cache and refresh coordination use the mutex. JWT generation
		// and HTTP requests run under the refreshing caller's context.
		token, err := p.repositoryToken(ctx, repo)
		p.mu.Lock()
		if err == nil {
			p.tokens[cacheKey] = token
		}
		delete(p.refreshes, cacheKey)
		close(done)
		p.mu.Unlock()
		return token.Token, err
	}
}

func (p *Provider) repositoryToken(ctx context.Context, repo githubauth.Repository) (accessToken, error) {
	jwt, err := GenerateJWT(p.identity.ClientID, p.key)
	if err != nil {
		return accessToken{}, err
	}
	var installed installation
	path := "/repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name) + "/installation"
	if err := p.request(ctx, http.MethodGet, path, jwt, nil, http.StatusOK, &installed); err != nil {
		return accessToken{}, err
	}
	if err := p.validateInstallation(installed); err != nil {
		return accessToken{}, err
	}
	if !strings.EqualFold(installed.Account.Login, repo.Owner) {
		return accessToken{}, fmt.Errorf("GitHub App installation belongs to a different repository owner")
	}
	return p.exchange(ctx, jwt, []string{repo.Name})
}

func requiredPermissions() map[string]string {
	return map[string]string{"contents": "read", "issues": "read", "pull_requests": "write"}
}

func (p *Provider) validateInstallation(installed installation) error {
	if installed.ID != p.identity.InstallationID || installed.AppID != p.identity.AppID || installed.Account.Login == "" {
		return fmt.Errorf("installation_id or app_id does not match this GitHub App installation")
	}
	if installed.SuspendedAt != nil {
		return fmt.Errorf("GitHub App installation is suspended")
	}
	for name, needed := range requiredPermissions() {
		actual := installed.Permissions[name]
		if actual != needed && !(needed == "read" && actual == "write") {
			return fmt.Errorf("GitHub App installation needs %s: %s permission; approve updated permissions in GitHub", name, needed)
		}
	}
	return nil
}

func (p *Provider) exchange(ctx context.Context, jwt string, repositories []string) (accessToken, error) {
	body := struct {
		Repositories []string          `json:"repositories,omitempty"`
		Permissions  map[string]string `json:"permissions"`
	}{repositories, requiredPermissions()}
	payload, err := json.Marshal(body)
	if err != nil {
		return accessToken{}, err
	}
	var token accessToken
	path := fmt.Sprintf("/app/installations/%d/access_tokens", p.identity.InstallationID)
	if err := p.request(ctx, http.MethodPost, path, jwt, bytes.NewReader(payload), http.StatusCreated, &token); err != nil {
		return accessToken{}, err
	}
	if strings.TrimSpace(token.Token) == "" || token.Token != strings.TrimSpace(token.Token) || !token.ExpiresAt.After(p.now().Add(2*time.Minute)) {
		return accessToken{}, fmt.Errorf("GitHub returned an invalid or nearly expired installation token")
	}
	for name, needed := range requiredPermissions() {
		if token.Permissions[name] != needed {
			return accessToken{}, fmt.Errorf("GitHub installation token has unexpected %s permission", name)
		}
	}
	return token, nil
}

func (p *Provider) authenticatedApp(ctx context.Context) (app, string, error) {
	jwt, err := GenerateJWT(p.identity.ClientID, p.key)
	if err != nil {
		return app{}, "", err
	}
	var current app
	if err := p.request(ctx, http.MethodGet, "/app", jwt, nil, http.StatusOK, &current); err != nil {
		return app{}, "", err
	}
	if current.ID != p.identity.AppID || current.ClientID != p.identity.ClientID || !namePart.MatchString(current.Slug) {
		return app{}, "", fmt.Errorf("app_id or client_id does not match the private key's GitHub App")
	}
	return current, jwt, nil
}

func (p *Provider) bot(ctx context.Context, slug, token string) (int64, string, error) {
	login := slug + "[bot]"
	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	if err := p.request(ctx, http.MethodGet, "/users/"+url.PathEscape(login), token, nil, http.StatusOK, &user); err != nil {
		return 0, "", err
	}
	if user.ID <= 0 || !strings.EqualFold(user.Login, login) || user.Type != "Bot" {
		return 0, "", fmt.Errorf("GitHub returned an unexpected App bot identity")
	}
	return user.ID, user.Login, nil
}

// ReviewAuthor returns the durable bot user ID used by GitHub review authors.
func (p *Provider) ReviewAuthor(ctx context.Context, repo githubauth.Repository) (int64, error) {
	current, _, err := p.authenticatedApp(ctx)
	if err != nil {
		return 0, err
	}
	token, err := p.Token(ctx, repo)
	if err != nil {
		return 0, err
	}
	id, _, err := p.bot(ctx, current.Slug, token)
	return id, err
}

// Verify checks key/App identity, installation grants, token exchange and bot identity.
// Its unscoped status token is not cached or reused for repository operations.
func (p *Provider) Verify(ctx context.Context) (Status, error) {
	current, jwt, err := p.authenticatedApp(ctx)
	if err != nil {
		return Status{}, err
	}
	var installed installation
	path := fmt.Sprintf("/app/installations/%d", p.identity.InstallationID)
	if err := p.request(ctx, http.MethodGet, path, jwt, nil, http.StatusOK, &installed); err != nil {
		return Status{}, err
	}
	if err := p.validateInstallation(installed); err != nil {
		return Status{}, err
	}
	token, err := p.exchange(ctx, jwt, nil)
	if err != nil {
		return Status{}, err
	}
	_, login, err := p.bot(ctx, current.Slug, token.Token)
	if err != nil {
		return Status{}, err
	}
	return Status{Login: login, Account: installed.Account.Login, InstallationID: installed.ID}, nil
}

// Never print API bodies, tokens, or transport errors from authentication calls.
func (p *Provider) request(ctx context.Context, method, path, token string, body io.Reader, expected int, result any) error {
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, body)
	if err != nil {
		return fmt.Errorf("create GitHub App authentication request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "giad")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := p.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("GitHub App authentication request failed; check network connectivity")
	}
	defer response.Body.Close()
	if response.StatusCode != expected {
		return fmt.Errorf("GitHub App authentication returned HTTP %d; check App identity, installation access, and approved permissions", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return fmt.Errorf("GitHub App authentication response is unreadable or exceeds 1 MiB")
	}
	if json.Unmarshal(data, result) != nil {
		return fmt.Errorf("GitHub App authentication returned invalid JSON")
	}
	return nil
}
