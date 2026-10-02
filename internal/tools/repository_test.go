package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryInspection(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "example.go"), []byte("first\nneedle here\nlast\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".git", "config"), []byte("needle secret"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("needle secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "escape")); err != nil {
		t.Fatal(err)
	}
	r, err := Open(directory, "a diff")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	got, err := r.ReadLines(ctx, "example.go", 2, 2)
	if err != nil || got.Text != "2: needle here\n" {
		t.Fatalf("ReadLines = %+v, %v", got, err)
	}
	for _, name := range []string{"../secret", outside, ".git/config", "escape"} {
		if _, err := r.ReadLines(ctx, name, 1, 0); err == nil {
			t.Errorf("accepted forbidden path %q", name)
		}
	}
	got, err = r.Search(ctx, "needle")
	if err != nil || got.Text != "example.go:2:needle here\n" {
		t.Fatalf("Search = %+v, %v", got, err)
	}
	if r.Diff().Text != "a diff" {
		t.Fatal("wrong diff")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Search(canceled, "needle"); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestRepositoryBudgets(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "many.txt"), []byte(strings.Repeat("needle\n", MaxMatches+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "huge.txt"), []byte(strings.Repeat("x", MaxFileBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(directory, strings.Repeat("é", MaxOutputBytes))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := r.Search(context.Background(), "needle")
	if err != nil || !got.Truncated || got.SkippedFiles != 1 || strings.Count(got.Text, "needle") != MaxMatches {
		t.Fatalf("Search = %+v, %v", got, err)
	}
	if _, err := r.ReadLines(context.Background(), "huge.txt", 1, 0); err == nil {
		t.Fatal("accepted oversized file")
	}
	if got := r.Diff(); !got.Truncated || len(got.Text) > MaxOutputBytes {
		t.Fatal("unbounded diff")
	}
}
