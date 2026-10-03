package main

import (
	"errors"
	"fmt"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/hyaxon/giad/internal/repo"
	"github.com/hyaxon/giad/internal/tools"
	"github.com/spf13/cobra"
)

func newInspectCommand() *cobra.Command {
	var repository, file, query string
	var start, end int
	var diff bool
	cmd := &cobra.Command{
		Use:   "inspect <PR-URL|number>",
		Short: "Read files, search text, or inspect a diff in a disposable PR checkout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if (cmd.Flags().Changed("start") || cmd.Flags().Changed("end")) && file == "" {
				return errors.New("line ranges require --file")
			}
			if start < 1 || end < 0 || (end != 0 && end < start) {
				return errors.New("invalid line range")
			}
			if !diff && file == "" && query == "" {
				return errors.New("provide a nonempty --file or --search, or --diff")
			}
			target, number, err := parsePRTarget(args[0], repository)
			if err != nil {
				return err
			}
			auth := githubauth.UserAuth{}
			context, err := githubapi.NewClient(auth).GetPRContext(cmd.Context(), target, number)
			if err != nil {
				return err
			}
			checkout, err := (repo.Manager{Auth: auth}).Prepare(cmd.Context(), repo.CheckoutRequest{
				Repository: target, Number: number, BaseSHA: context.PullRequest.Base.SHA, HeadSHA: context.PullRequest.Head.SHA,
			})
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, checkout.Close()) }()
			reader, err := tools.Open(checkout.Path, context.Diff)
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, reader.Close()) }()
			var result tools.Result
			switch {
			case file != "":
				result, err = reader.ReadLines(cmd.Context(), file, start, end)
			case query != "":
				result, err = reader.Search(cmd.Context(), query)
			case diff:
				result = reader.Diff()
			default:
				return errors.New("provide a nonempty --file or --search, or --diff")
			}
			if err != nil {
				return err
			}
			if _, err = fmt.Fprint(cmd.OutOrStdout(), result.Text); err != nil {
				return err
			}
			if result.Truncated {
				if _, err = fmt.Fprintln(cmd.OutOrStdout(), "\n[Output incomplete: tool budget reached.]"); err != nil {
					return err
				}
			}
			if result.SkippedFiles > 0 {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "\n[Skipped %d unreadable, oversized, or non-text files.]\n", result.SkippedFiles)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&repository, "repo", "", "GitHub repository in OWNER/REPO form")
	cmd.Flags().StringVar(&file, "file", "", "Read a checkout-relative UTF-8 file")
	cmd.Flags().StringVar(&query, "search", "", "Search for literal text")
	cmd.Flags().BoolVar(&diff, "diff", false, "Read the fetched PR diff")
	cmd.Flags().IntVar(&start, "start", 1, "First line to read (inclusive)")
	cmd.Flags().IntVar(&end, "end", 0, "Last line to read (zero means end of file)")
	cmd.MarkFlagsOneRequired("file", "search", "diff")
	cmd.MarkFlagsMutuallyExclusive("file", "search", "diff")
	return cmd
}
