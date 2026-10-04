package publication

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBodyOnlyFindingsOutsideDiffHunks(t *testing.T) {
	for _, line := range []int{-1, 0, 99} {
		draft, current := fixture()
		draft.Report.Findings[0].Line = line
		plan, err := Prepare(draft, current)
		if (err == nil) != (line > 0) {
			t.Fatalf("body-only finding at line %d: %v", line, err)
		}
		if line > 0 && (!strings.Contains(plan.Body, "auth.go:99") || len(plan.Comments) != 0) {
			t.Fatal("body-only finding was lost")
		}
		if _, err := PrepareWithOptions(draft, current, Options{Inline: true}); err == nil {
			t.Fatal("an invalid inline anchor was accepted")
		}
	}
	draft, current := fixture()
	current.Files[0].Status = "removed"
	if _, err := Prepare(draft, current); err == nil {
		t.Fatal("body-only publication accepted a deleted file")
	}
}

func TestInlineAnchorsFromRealGitPaths(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "core.quotePath=true", "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v: %s", err, data)
		}
		return string(data)
	}
	git("init", "--quiet", "--template=")
	names := []string{"auth space.go", "trailing space .go", "unicode-♥.go", "literal\ttab.go"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte("old\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "--quiet", "-m", "base")
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte("new\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	diff := git("diff")
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			draft, current := fixture()
			draft.Report.Findings[0].File = name
			draft.Report.Findings[0].Line = 1
			current.Files[0].Filename = name
			current.Diff = diff
			plan, err := PrepareWithOptions(draft, current, Options{Inline: true})
			if err != nil || len(plan.Comments) != 1 || plan.Comments[0].Path != name {
				t.Fatalf("real Git filename was misparsed: plan=%+v err=%v", plan, err)
			}
		})
	}
}
