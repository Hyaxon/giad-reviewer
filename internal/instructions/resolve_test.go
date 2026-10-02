package instructions

import (
	"context"
	"strings"
	"testing"

	"github.com/hyaxon/agentic-review/pkg/protocol"
)

type baseFiles map[string]string

func (b baseFiles) ReadBaseFile(_ context.Context, path string) (string, bool, error) {
	text, ok := b[path]
	return text, ok, nil
}
func TestRootNestedAndRenameScopes(t *testing.T) {
	files := baseFiles{"AGENTS.md": "root", "backend/AGENTS.md": "backend", "backend/auth/AGENTS.md": "auth", "old/AGENTS.md": "old", "unrelated/AGENTS.md": "unrelated"}
	result, err := Resolve(context.Background(), files, "base", []protocol.ChangedFile{{Path: "backend/auth/session.go", PreviousPath: "old/session.go"}, {Path: "backend/api.go"}})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, i := range result {
		names = append(names, i.Path)
		if i.BaseSHA != "base" {
			t.Fatal("missing provenance")
		}
	}
	if strings.Join(names, ",") != "AGENTS.md,backend/AGENTS.md,old/AGENTS.md,backend/auth/AGENTS.md" {
		t.Fatal(names)
	}
	if _, err := Resolve(context.Background(), files, "base", []protocol.ChangedFile{{Path: "../escape"}}); err == nil {
		t.Fatal("unsafe path accepted")
	}
}
