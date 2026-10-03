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
		Use:   "giad",
		Short: "Local runtime for independent review agents",
		Long: `GIAD prepares GitHub pull requests and brokers controlled
repository and model capabilities for separately installed review agents.

Use giad review with an explicit agent manifest and an installed Docker image to
preview a local draft. Agents can request approved test profiles. Save a draft with
--json and use giad publish to preview and confirm a GitHub review. Use --inline
to attach findings to source lines and --event REQUEST_CHANGES to request changes.`,
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
	cmd.AddCommand(newPublishCommand())
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "giad:", err)
		os.Exit(1)
	}
}
