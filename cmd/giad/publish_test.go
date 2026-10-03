package main

import (
	"io"
	"strings"
	"testing"
)

func TestPublishRejectsUnsupportedEventBeforeReadingDraft(t *testing.T) {
	cmd := newPublishCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"nonexistent.json", "--event", "APPROVE"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "review event must be") {
		t.Fatalf("unsupported event reached draft/network access: %v", err)
	}
}
