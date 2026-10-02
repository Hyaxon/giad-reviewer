package githubauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VerifyUser checks the identity represented by a user token, not gh's display
// settings. App installation tokens will need a different identity check.
func VerifyUser(ctx context.Context, auth Provider) (string, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return verifyUser(ctx, auth, client)
}

func verifyUser(ctx context.Context, auth Provider, client *http.Client) (string, error) {
	token, err := auth.Token(ctx, Repository{Host: "github.com"})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return "", fmt.Errorf("create identity request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "magi")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("verify GitHub identity: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Do not expose response bodies; only the status is needed for diagnosis.
		return "", fmt.Errorf("GitHub identity check returned HTTP %d; check gh auth status --hostname github.com", resp.StatusCode)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&user); err != nil {
		return "", fmt.Errorf("decode GitHub identity: %w", err)
	}
	if user.Login == "" {
		return "", fmt.Errorf("GitHub identity response has no login")
	}
	return user.Login, nil
}
