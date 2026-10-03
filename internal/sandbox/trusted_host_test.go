package sandbox

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The child reports its environment, then waits for its input to close.
func TestHostProcessFixture(t *testing.T) {
	for _, arg := range os.Args {
		if arg == "--host-process-fixture" {
			cwd, err := os.Getwd()
			if err != nil {
				os.Exit(10)
			}
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
				"cwd": cwd, "token": os.Getenv("GH_TOKEN"), "home": os.Getenv("HOME"),
			})
			_, _ = io.Copy(io.Discard, os.Stdin)
			os.Exit(0)
		}
	}
}

func TestTrustedHostLifecycle(t *testing.T) {
	t.Setenv("GH_TOKEN", "must-not-be-inherited")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p, err := (TrustedHostLauncher{}).Launch(ctx, Command{
		Path: executable, Args: []string{"-test.run=^TestHostProcessFixture$", "--", "--host-process-fixture"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	var observed map[string]string
	if err := json.NewDecoder(p.Stdout()).Decode(&observed); err != nil {
		t.Fatal(err)
	}
	if observed["token"] != "" || observed["home"] != "" {
		t.Fatalf("host environment leaked: %v", observed)
	}
	if _, err := os.Stat(observed["cwd"]); err != nil {
		t.Fatalf("private working directory unavailable during execution: %v", err)
	}
	// A blocked protocol read must end on cancellation, even with no output.
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, p.Stdout()); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not unblock protocol output")
	}
	for i := 0; i < 2; i++ {
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(observed["cwd"]); !os.IsNotExist(err) {
		t.Fatalf("working directory survived cleanup: %v", err)
	}
}

func TestTrustedHostStartFailureCleansUp(t *testing.T) {
	privateTemp := t.TempDir()
	t.Setenv("TMPDIR", privateTemp)
	t.Setenv("TMP", privateTemp)
	t.Setenv("TEMP", privateTemp)
	// Inspect the launcher's temporary directory location after a start failure.
	root := os.TempDir()
	before, err := filepath.Glob(filepath.Join(root, "giad-agent-*"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := (TrustedHostLauncher{}).Launch(context.Background(), Command{Path: filepath.Join(t.TempDir(), "missing-agent")})
	if err == nil || p != nil {
		t.Fatalf("missing executable must fail: process=%v err=%v", p, err)
	}
	after, err := filepath.Glob(filepath.Join(root, "giad-agent-*"))
	if err != nil || len(after) != len(before) {
		t.Fatalf("failed launch left working directories: before=%v after=%v err=%v", before, after, err)
	}
}
