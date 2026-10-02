package githubauth

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// UserAuth uses gh's effective credentials, including GH_TOKEN overrides.
// Login is explicit: this provider never opens a browser or prompts.
type UserAuth struct {
	run func(context.Context, string, ...string) ([]byte, error)
}

var _ Provider = UserAuth{}

func (a UserAuth) Token(ctx context.Context, repo Repository) (string, error) {
	if repo.Host != "" && repo.Host != "github.com" {
		return "", fmt.Errorf("only github.com authentication is currently supported")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	run := a.run
	if run == nil {
		run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		}
	}
	output, err := run(ctx, "gh", "auth", "token", "--hostname", "github.com")
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("get GitHub credentials: %w", ctx.Err())
		}
		// Do not echo subprocess output: it may contain credentials.
		return "", fmt.Errorf("cannot get GitHub credentials; ensure gh is installed and run gh auth login --hostname github.com")
	}
	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", fmt.Errorf("GitHub credentials are empty; run gh auth login --hostname github.com")
	}
	return token, nil
}
