package main

import (
	"errors"
	"fmt"

	"github.com/hyaxon/agentic-review/internal/githubapi"
	"github.com/hyaxon/agentic-review/internal/githubauth"
	"github.com/hyaxon/agentic-review/internal/repo"
	"github.com/spf13/cobra"
)

func newCheckoutCommand() *cobra.Command {
	var repository string
	var keep bool
	cmd := &cobra.Command{
		Use:   "checkout <PR-URL|number>",
		Short: "Prepare and verify a disposable PR checkout without executing its code",
		Long:  "Fetch exact PR revisions into a temporary directory. The checkout is removed\non completion unless --keep is supplied. This does not run tests or sandbox code.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			target, number, err := parsePRTarget(args[0], repository)
			if err != nil {
				return err
			}
			auth := githubauth.UserAuth{}
			pr, err := githubapi.NewClient(auth).GetPullRequest(cmd.Context(), target, number)
			if err != nil {
				return err
			}
			checkout, err := (repo.Manager{Auth: auth}).Prepare(cmd.Context(), repo.CheckoutRequest{
				Repository: target, Number: number, BaseSHA: pr.Base.SHA, HeadSHA: pr.Head.SHA,
			})
			if err != nil {
				return err
			}
			defer func() {
				if !keep || err != nil {
					err = errors.Join(err, checkout.Close())
				}
			}()
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Verified isolated checkout for %s/%s #%d\nBase: %s\nHead: %s\nNo PR code or tests were executed.\n", target.Owner, target.Name, number, checkout.BaseSHA, checkout.HeadSHA)
			if err != nil {
				return err
			}
			if keep {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Kept for manual inspection: %s\nRemove its agentic-review-* parent directory when finished. This directory is not an execution sandbox.\n", checkout.Path)
			} else {
				if err = checkout.Close(); err != nil {
					return err
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "Temporary checkout removed. Use --keep to retain it for inspection.")
			}
			return err
		},
	}
	cmd.Flags().StringVar(&repository, "repo", "", "GitHub repository in OWNER/REPO form (required with a number)")
	cmd.Flags().BoolVar(&keep, "keep", false, "Retain the temporary checkout for manual inspection")
	return cmd
}
