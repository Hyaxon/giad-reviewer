package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

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
Ollama and publish COMMENT-only feedback using your personal GitHub account.
Dedicated GitHub App authentication will be available later.

This is the project scaffold. The review workflow is not implemented yet.`,
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAuthCommand())
	cmd.AddCommand(newPRCommand())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "magi:", err)
		os.Exit(1)
	}
}
