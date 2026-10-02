package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyaxon/agentic-review/internal/agents"
	"github.com/hyaxon/agentic-review/internal/config"
	"github.com/hyaxon/agentic-review/internal/githubauth"
	"github.com/hyaxon/agentic-review/internal/review"
	"github.com/spf13/cobra"
)

func newReviewCommand() *cobra.Command {
	var repository, manifestPath, configPath string
	var timeout time.Duration
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "review <PR-URL|number>",
		Short: "Launch a trusted external agent and preview its local draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout <= 0 {
				return fmt.Errorf("timeout must be positive")
			}
			manifest, err := agents.LoadManifest(manifestPath)
			if err != nil {
				return err
			}
			settings, err := config.LoadRuntime(configPath)
			if err != nil {
				return err
			}
			target, number, err := parsePRTarget(args[0], repository)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			runner := review.Runner{Auth: githubauth.UserAuth{}, Progress: func(message string) { fmt.Fprintln(cmd.ErrOrStderr(), message) }}
			result, err := runner.Run(ctx, review.Request{Repository: target, Number: number, Manifest: manifest, Config: settings})
			if err != nil {
				return fmt.Errorf("%s review incomplete: %w", manifest.Name, err)
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Local draft: %s #%d\nAgent: %s\nBase: %s\nHead: %s\n\n%s\n", result.Job.Repository, number, result.Agent, result.Job.BaseSHA, result.Job.HeadSHA, result.Report.Summary); err != nil {
				return err
			}
			for _, f := range result.Report.Findings {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "\n[%s] %s — %s/%s\n%s:%d (confidence %.2f)\n%s\nEvidence: %s\nScenario: %s\nSuggested fix: %s\n", f.Severity, f.Title, result.Agent, f.Source, f.File, f.Line, f.Confidence, f.Explanation, f.Evidence, f.FailureScenario, f.SuggestedFix); err != nil {
					return err
				}
			}
			if len(result.Report.Findings) == 0 {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nNo draft findings reported; this is not proof of correctness."); err != nil {
					return err
				}
			}
			if result.Report.Limitations != "" {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nCoverage limitations:", result.Report.Limitations); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "\nLocal preview only. Nothing was published.")
			return err
		},
	}
	cmd.Flags().StringVar(&repository, "repo", "", "GitHub repository in OWNER/REPO form")
	cmd.Flags().StringVar(&manifestPath, "agent-manifest", "", "Explicit trusted local agent manifest (JSON)")
	cmd.Flags().StringVar(&configPath, "config", "", "Explicit trusted runtime settings (TOML)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Overall review timeout")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the revision-bound draft as JSON")
	_ = cmd.MarkFlagRequired("agent-manifest")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}
