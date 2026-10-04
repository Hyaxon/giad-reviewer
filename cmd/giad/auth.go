package main

import (
	"fmt"

	"github.com/hyaxon/giad/internal/githubapp"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/spf13/cobra"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Check GitHub authentication"}
	addIdentityFlag(cmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Verify the GitHub account GIAD will use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			auth, err := commandAuth(cmd)
			if err != nil {
				return err
			}
			if app, ok := auth.(*githubapp.Provider); ok {
				status, err := app.Verify(cmd.Context())
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Authenticated as %s on github.com (App mode).\nInstallation %d on %s; token exchange and review permissions verified.\n", status.Login, status.InstallationID, status.Account)
				return err
			}
			login, err := githubauth.VerifyUser(cmd.Context(), auth)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Authenticated as %s on github.com (personal mode).\nFuture reviews will appear from this account. Repository permissions are checked separately.\n", login)
			return err
		},
	})
	return cmd
}
