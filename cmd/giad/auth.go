package main

import (
	"fmt"

	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Check personal GitHub authentication"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Verify the GitHub account GIAD will use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			login, err := githubauth.VerifyUser(cmd.Context(), githubauth.UserAuth{})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Authenticated as %s on github.com (personal mode).\nFuture reviews will appear from this account. Repository permissions are checked separately.\n", login)
			return err
		},
	})
	return cmd
}
