package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hyaxon/magi-agents/internal/githubapi"
	"github.com/hyaxon/magi-agents/internal/githubauth"
	"github.com/spf13/cobra"
)

type fetchPullRequest func(context.Context, githubauth.Repository, int) (githubapi.PRContext, error)

func newPRCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "pr", Short: "Read GitHub pull requests"}
	client := githubapi.NewClient(githubauth.UserAuth{})
	cmd.AddCommand(newPRViewCommand(client.GetPRContext))
	return cmd
}

func newPRViewCommand(fetch fetchPullRequest) *cobra.Command {
	var repository string
	var showDiff bool
	cmd := &cobra.Command{
		Use:     "view <PR-URL|number>",
		Short:   "Read PR metadata, changed files, and linked issues",
		Example: "  magi pr view https://github.com/OWNER/REPO/pull/42\n  magi pr view 42 --repo OWNER/REPO",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, number, err := parsePRTarget(args[0], repository)
			if err != nil {
				return err
			}
			result, err := fetch(cmd.Context(), repo, number)
			if err != nil {
				return err
			}
			pr := result.PullRequest
			var output strings.Builder
			fmt.Fprintf(&output, "%s/%s #%d: %s\n%s\n\nBase: %s (%s)\nHead: %s (%s)\n\n%s\n", repo.Owner, repo.Name, pr.Number, pr.Title, pr.URL, pr.Base.Name, pr.Base.SHA, pr.Head.Name, pr.Head.SHA, pr.Body)
			fmt.Fprintf(&output, "\nChanged files (%d):\n", len(result.Files))
			for _, file := range result.Files {
				fmt.Fprintf(&output, "  %s %s (+%d -%d)\n", file.Status, file.Filename, file.Additions, file.Deletions)
				if file.PreviousFilename != "" {
					fmt.Fprintf(&output, "    previously: %s\n", file.PreviousFilename)
				}
			}
			if result.IssuesError != "" {
				fmt.Fprintf(&output, "\nWARNING: Linked issues could not be fully retrieved: %s\nRequirement coverage is unavailable.\n", result.IssuesError)
			} else if len(result.LinkedIssues) == 0 {
				fmt.Fprintln(&output, "\nNo formally linked issues found.")
			} else {
				fmt.Fprintf(&output, "\nLinked issues (%d):\n", len(result.LinkedIssues))
				for _, issue := range result.LinkedIssues {
					fmt.Fprintf(&output, "\n%s #%d: %s\n%s\n%s\n", issue.Repository.NameWithOwner, issue.Number, issue.Title, issue.URL, issue.Body)
				}
			}
			if showDiff {
				fmt.Fprintf(&output, "\nDiff:\n%s\n", result.Diff)
			} else {
				fmt.Fprintf(&output, "\nDiff fetched (%d bytes). Use --diff to display it.\n", len(result.Diff))
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), output.String())
			return err
		},
	}
	cmd.Flags().StringVar(&repository, "repo", "", "GitHub repository in OWNER/REPO form (required with a number)")
	cmd.Flags().BoolVar(&showDiff, "diff", false, "Display the fetched unified diff")
	return cmd
}

var repositoryPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var prNumber = regexp.MustCompile(`^[0-9]+$`)

func parsePRTarget(input, explicitRepo string) (githubauth.Repository, int, error) {
	fail := func(message string) (githubauth.Repository, int, error) {
		return githubauth.Repository{}, 0, fmt.Errorf("%s", message)
	}
	repository, numberText := explicitRepo, input
	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fail("use a PR URL like https://github.com/OWNER/REPO/pull/42 without query parameters or fragments")
		}
		parts := strings.Split(strings.TrimSuffix(u.Path, "/"), "/")
		if len(parts) != 5 || parts[0] != "" || parts[3] != "pull" {
			return fail("expected a PR URL ending in /OWNER/REPO/pull/NUMBER")
		}
		repository = parts[1] + "/" + parts[2]
		numberText = parts[4]
		if explicitRepo != "" && !strings.EqualFold(explicitRepo, repository) {
			return fail("--repo conflicts with the repository in the PR URL")
		}
	}
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fail("provide --repo OWNER/REPO or a full PR URL; Git remote inference is not implemented yet")
	}
	for _, part := range parts {
		if !repositoryPart.MatchString(part) || part == "." || part == ".." {
			return fail("repository must have a valid owner and name in OWNER/REPO form")
		}
	}
	number, err := strconv.Atoi(numberText)
	if err != nil || number <= 0 || !prNumber.MatchString(numberText) {
		return fail("PR number must be a positive integer")
	}
	return githubauth.Repository{Host: "github.com", Owner: parts[0], Name: parts[1]}, number, nil
}
