package main

import (
	"fmt"

	"github.com/hyaxon/giad/internal/config"
	"github.com/hyaxon/giad/internal/githubapp"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/spf13/cobra"
)

func addIdentityFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().String("identity", "", "Explicit trusted GitHub App identity TOML (omit for personal GitHub CLI auth)")
}

func commandAuth(cmd *cobra.Command) (githubauth.Provider, error) {
	flag := cmd.Flag("identity")
	if flag == nil || !flag.Changed {
		return githubauth.UserAuth{}, nil
	}
	path := flag.Value.String()
	if path == "" {
		return nil, fmt.Errorf("--identity requires a nonempty file path")
	}
	identity, err := config.LoadIdentity(path)
	if err != nil {
		return nil, err
	}
	return githubapp.NewProvider(identity.GitHub.App)
}
