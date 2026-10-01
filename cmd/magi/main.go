package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	cmd := &cobra.Command{
		Use:   "magi",
		Short: "Local-first code review with the three MAGI reviewers",
		Long: `MAGI is a local-first GitHub pull-request review system.

MELCHIOR reviews correctness, BALTHASAR requirements and tests, and
CASPER security and performance. Reviews will run sequentially through
Ollama and publish COMMENT-only feedback through separate GitHub Apps.

This is the project scaffold. The review workflow is not implemented yet.`,
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "magi:", err)
		os.Exit(1)
	}
}
