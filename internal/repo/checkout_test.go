package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyaxon/agentic-review/internal/githubauth"
)

type testAuth struct{}

func (testAuth) Token(context.Context, githubauth.Repository) (string, error) {
	return "test-credential", nil
}

func TestPrepareAndCleanup(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := runGit(ctx, source, nil, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	git("init", "--quiet", "--template=", ".")
	if err := os.WriteFile(filepath.Join(source, "example.txt"), []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "base")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(source, "example.txt"), []byte("head\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "head")
	head := git("rev-parse", "HEAD")
	git("update-ref", "refs/pull/2/head", head)

	// A hostile inherited setting must not redirect writes into another repository.
	t.Setenv("GIT_DIR", filepath.Join(source, "does-not-exist"))
	for _, mode := range []string{"success", "stale", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			workspace := t.TempDir()
			fetches := 0
			manager := Manager{Auth: testAuth{}, TempDir: workspace}
			manager.git = func(ctx context.Context, dir string, env []string, args ...string) (string, error) {
				if args[0] == "fetch" {
					fetches++
					if len(env) != 3 || !strings.HasPrefix(env[2], "GIT_CONFIG_VALUE_0=Authorization: Basic ") {
						t.Fatal("missing fetch credentials")
					}
					// Substitute only the network transport for this local integration test.
					args = append([]string(nil), args...)
					for i, arg := range args {
						if arg == "origin" {
							args[i] = "file://" + filepath.ToSlash(source)
						}
					}
					args = append([]string{"-c", "protocol.file.allow=always"}, args...)
				} else if len(env) != 0 {
					t.Fatal("credentials exposed outside fetch")
				}
				return runGit(ctx, dir, env, args...)
			}
			req := CheckoutRequest{Repository: githubauth.Repository{Owner: "owner", Name: "repo"}, Number: 2, BaseSHA: base, HeadSHA: head}
			if mode == "stale" {
				req.HeadSHA = base
			}
			requestCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			checkout, err := manager.Prepare(requestCtx, req)
			if mode != "success" {
				if err == nil {
					t.Fatal("expected failure")
				}
				if mode == "stale" && !strings.Contains(err.Error(), "differs from metadata") {
					t.Fatal(err)
				}
				if mode == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = checkout.Close() })
				data, err := os.ReadFile(filepath.Join(checkout.Path, "example.txt"))
				if err != nil || string(data) != "head\n" {
					t.Fatalf("wrong checkout contents: %v", err)
				}
				config, err := os.ReadFile(filepath.Join(checkout.Path, ".git", "config"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(config), "extraHeader") || strings.Contains(string(config), "test-credential") {
					t.Fatal("persisted credentials")
				}
				if fetches != 1 {
					t.Fatalf("fetch count = %d", fetches)
				}
				if _, err := runGit(ctx, checkout.Path, nil, "symbolic-ref", "-q", "HEAD"); err == nil {
					t.Fatal("expected detached HEAD")
				}
				if err := checkout.Close(); err != nil {
					t.Fatal(err)
				}
				if err := checkout.Close(); err != nil {
					t.Fatal(err)
				}
			}
			entries, err := os.ReadDir(workspace)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary checkout leaked: %v", err)
			}
		})
	}
}
