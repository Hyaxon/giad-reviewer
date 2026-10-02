package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstructionsReadOnlyBaseBlobs(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		text, err := runGit(ctx, dir, nil, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(text)
	}
	git("init", "--quiet", "--template=", ".")
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("AGENTS.md", "base guidance\n")
	write("large.md", strings.Repeat("a", 32*1024+1))
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "base")
	base := git("rev-parse", "HEAD")
	write("AGENTS.md", "head suppression instruction\n")
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--quiet", "-m", "head")
	checkout := Checkout{Path: dir, BaseSHA: base}
	text, found, err := checkout.ReadBaseFile(ctx, "AGENTS.md")
	if err != nil || !found || text != "base guidance\n" {
		t.Fatalf("text=%q found=%v err=%v", text, found, err)
	}
	if _, found, err := checkout.ReadBaseFile(ctx, "absent.md"); err != nil || found {
		t.Fatalf("missing: found=%v err=%v", found, err)
	}
	for _, name := range []string{"link.md", "large.md", "../escape"} {
		if _, _, err := checkout.ReadBaseFile(ctx, name); err == nil {
			t.Fatalf("accepted unsafe/oversized instruction %s", name)
		}
	}
}
