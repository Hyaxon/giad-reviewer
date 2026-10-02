package repo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hyaxon/agentic-review/internal/githubauth"
)

type CheckoutRequest struct {
	Repository githubauth.Repository
	Number     int
	BaseSHA    string
	HeadSHA    string
}

// Checkout owns only a directory created by Prepare. Call Close when finished.
type Checkout struct {
	Path    string
	BaseSHA string
	HeadSHA string
	root    string
}

func (c *Checkout) Close() error {
	if c.root == "" {
		return nil
	}
	if err := os.RemoveAll(c.root); err != nil {
		return fmt.Errorf("remove temporary checkout: %w", err)
	}
	c.root = ""
	return nil
}

type gitCommand func(context.Context, string, []string, ...string) (string, error)

type Manager struct {
	Auth githubauth.Provider
	// TempDir is optional; empty uses the operating system's temporary directory.
	TempDir string
	git     gitCommand
}

var repoPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var commitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func (m Manager) Prepare(ctx context.Context, req CheckoutRequest) (_ *Checkout, err error) {
	if req.Repository.Host != "" && req.Repository.Host != "github.com" {
		return nil, fmt.Errorf("checkout supports github.com only")
	}
	for _, part := range []string{req.Repository.Owner, req.Repository.Name} {
		if !repoPart.MatchString(part) || part == "." || part == ".." {
			return nil, fmt.Errorf("invalid repository owner or name")
		}
	}
	if req.Number <= 0 || !commitSHA.MatchString(req.BaseSHA) || !commitSHA.MatchString(req.HeadSHA) {
		return nil, fmt.Errorf("checkout requires a positive PR number and full base/head commit SHAs")
	}
	if m.Auth == nil {
		return nil, fmt.Errorf("checkout requires an authentication provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	token, err := m.Auth.Token(ctx, req.Repository)
	if err != nil {
		return nil, fmt.Errorf("authenticate checkout: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("checkout credentials are empty")
	}
	root, err := os.MkdirTemp(m.TempDir, "agentic-review-")
	if err != nil {
		return nil, fmt.Errorf("create checkout directory: %w", err)
	}
	checkout := &Checkout{Path: filepath.Join(root, "repo"), BaseSHA: strings.ToLower(req.BaseSHA), HeadSHA: strings.ToLower(req.HeadSHA), root: root}
	defer func() {
		if err != nil {
			err = errors.Join(err, checkout.Close())
		}
	}()
	run := m.git
	if run == nil {
		run = runGit
	}
	if _, err = run(ctx, root, nil, "init", "--quiet", "--template=", checkout.Path); err != nil {
		return nil, err
	}
	remote := "https://github.com/" + req.Repository.Owner + "/" + req.Repository.Name + ".git"
	if _, err = run(ctx, checkout.Path, nil, "remote", "add", "origin", remote); err != nil {
		return nil, err
	}
	// Token is supplied only to the fetch process, never argv, Git config, or disk.
	header := "Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+token))
	env := []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraHeader", "GIT_CONFIG_VALUE_0=" + header}
	if _, err = run(ctx, checkout.Path, env, "fetch", "--quiet", "--no-tags", "--no-recurse-submodules", "--depth=1", "origin",
		checkout.BaseSHA+":refs/agentic-review/base", fmt.Sprintf("refs/pull/%d/head:refs/agentic-review/head", req.Number)); err != nil {
		return nil, err
	}
	for ref, want := range map[string]string{"refs/agentic-review/base": checkout.BaseSHA, "refs/agentic-review/head": checkout.HeadSHA} {
		got, gitErr := run(ctx, checkout.Path, nil, "rev-parse", "--verify", ref+"^{commit}")
		if gitErr != nil {
			return nil, gitErr
		}
		if strings.TrimSpace(got) != want {
			return nil, fmt.Errorf("fetched PR revision differs from metadata; retry")
		}
	}
	if _, err = run(ctx, checkout.Path, nil, "checkout", "--quiet", "--detach", checkout.HeadSHA, "--"); err != nil {
		return nil, err
	}
	return checkout, nil
}

// No inherited Git config, credential helpers, hooks, filters, or secret env vars.
// Git still parses untrusted data: this is not an OS sandbox.
func runGit(ctx context.Context, dir string, extraEnv []string, args ...string) (string, error) {
	return runGitBounded(ctx, dir, extraEnv, 8192, args...)
}

func runGitBounded(ctx context.Context, dir string, extraEnv []string, limit int, args ...string) (string, error) {
	options := []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "core.attributesFile=" + os.DevNull,
		"-c", "credential.helper=", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always",
		"-c", "http.followRedirects=false", "-c", "http.lowSpeedLimit=1", "-c", "http.lowSpeedTime=30",
		"-c", "submodule.recurse=false", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}
	cmd := exec.CommandContext(ctx, "git", append(options, args...)...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1", "LANG=C", "LC_ALL=C"}
	if systemRoot := os.Getenv("SYSTEMROOT"); systemRoot != "" {
		cmd.Env = append(cmd.Env, "SYSTEMROOT="+systemRoot)
	}
	cmd.Env = append(cmd.Env, extraEnv...)
	output := cappedOutput{limit: limit}
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("git %s: %w", args[0], ctx.Err())
		}
		return "", fmt.Errorf("git %s failed (check Git installation, repository access, and PR revisions): %w", args[0], err)
	}
	if output.overflow {
		return "", fmt.Errorf("git %s output exceeds %d bytes", args[0], limit)
	}
	return output.String(), nil
}

type cappedOutput struct {
	strings.Builder
	limit    int
	overflow bool
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := b.limit - b.Len(); remaining > 0 {
		if len(p) > remaining {
			b.overflow = true
			p = p[:remaining]
		}
		_, _ = b.Builder.Write(p)
	} else if n > 0 {
		b.overflow = true
	}
	return n, nil
}
