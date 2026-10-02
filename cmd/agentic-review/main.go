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
		Use:   "agentic-review",
		Short: "Local runtime for independent review agents",
		Long: `Agentic Review prepares GitHub pull requests and brokers controlled
repository and model capabilities for separately installed review agents.

Use agentic-review review with an explicit trusted agent manifest to preview a
local draft. GitHub publication and sandboxed test execution are not implemented.`,
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
	cmd.AddCommand(newReviewCommand())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "agentic-review:", err)
		os.Exit(1)
	}
}
