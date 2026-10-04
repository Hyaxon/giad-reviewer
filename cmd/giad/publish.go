package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/hyaxon/giad/internal/publication"
	"github.com/spf13/cobra"
)

type publicationClient interface {
	publication.API
	GetPRContext(context.Context, githubauth.Repository, int) (githubapi.PRContext, error)
}

func newPublishCommand() *cobra.Command {
	return newPublishCommandWithClient(nil)
}

func newPublishCommandWithClient(client publicationClient) *cobra.Command {
	var confirmation, event string
	var inline bool
	cmd := &cobra.Command{
		Use: "publish <draft.json>", Short: "Preview a saved draft's GitHub review; publish only with its confirmation hash",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options, err := (publication.Options{Event: event, Inline: inline}).Normalize()
			if err != nil {
				return err
			}
			draft, err := publication.LoadDraft(args[0])
			if err != nil {
				return err
			}
			repo, err := publication.Target(draft)
			if err != nil {
				return err
			}
			if client == nil {
				auth, err := commandAuth(cmd)
				if err != nil {
					return err
				}
				client = githubapi.NewClient(auth)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
			defer cancel()
			if confirmation != "" {
				// Recognize the confirmed write even after a push or merge. This
				// path performs no POST; fresh checks below still govern new writes.
				prior, err := publication.PrepareForReconciliation(draft, options)
				if err != nil {
					return err
				}
				outcome, err := publication.Reconcile(ctx, client, prior, confirmation)
				if err != nil {
					return err
				}
				if outcome.Existing {
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "Already published: %s\n", outcome.Review.URL)
					return err
				}
			}
			current, err := client.GetPRContext(ctx, repo, draft.Job.Number)
			if err != nil {
				return err
			}
			plan, err := publication.PrepareWithOptions(draft, current, options)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s review: %s/%s #%d\n\n%s\n", plan.Event, repo.Owner, repo.Name, plan.Number, plan.Body); err != nil {
				return err
			}
			for _, comment := range plan.Comments {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Inline comment: %s:%d (%s)\n%s\n", comment.Path, comment.Line, comment.Side, comment.Body); err != nil {
					return err
				}
			}
			if confirmation == "" {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "Nothing published. Rerun with the same --identity/--event/--inline options and --confirm %s\n", plan.Key)
				return err
			}
			cache, err := os.UserCacheDir()
			if err != nil {
				return err
			}
			outcome, err := publication.Publish(ctx, client, plan, confirmation, filepath.Join(cache, "giad", "publications"))
			if err != nil {
				return err
			}
			verb := "Published"
			if outcome.Existing {
				verb = "Already published"
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", verb, outcome.Review.URL)
			return err
		},
	}
	addIdentityFlag(cmd)
	cmd.Flags().StringVar(&confirmation, "confirm", "", "Confirmation hash printed by a prior publication preview")
	cmd.Flags().StringVar(&event, "event", "COMMENT", "GitHub review event: COMMENT or REQUEST_CHANGES")
	cmd.Flags().BoolVar(&inline, "inline", false, "Attach each finding as a head-side inline comment")
	return cmd
}
