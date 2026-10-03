package main

import (
	"io"
	"strings"
	"testing"
)

func TestReviewRejectsDisablingPreview(t *testing.T) {
	cmd := newReviewCommand()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"42", "--repo", "owner/repo", "--agent-manifest", "unused", "--config", "unused", "--preview=false"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "use giad publish separately") {
		t.Fatalf("expected explicit local-only error before reading configuration, got %v", err)
	}
}
