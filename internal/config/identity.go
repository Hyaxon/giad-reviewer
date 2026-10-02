package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type IdentityConfig struct {
	GitHub GitHubConfig `toml:"github"`
}

type GitHubConfig struct {
	App AppIdentity `toml:"app"`
	// LegacyApp accepts the pre-split identity file without rewriting local credentials.
	LegacyApp AppIdentity `toml:"magi"`
}

type AppIdentity struct {
	AppID          int64  `toml:"app_id"`
	ClientID       string `toml:"client_id"`
	InstallationID int64  `toml:"installation_id"`
	PrivateKey     string `toml:"private_key"`
}

func LoadIdentity(path string) (IdentityConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IdentityConfig{}, fmt.Errorf("read identity file: %w", err)
	}

	var identity IdentityConfig

	if err := toml.Unmarshal(data, &identity); err != nil {
		return IdentityConfig{}, fmt.Errorf("parse identity file: %w", err)
	}

	if identity.GitHub.LegacyApp != (AppIdentity{}) {
		if identity.GitHub.App != (AppIdentity{}) {
			return IdentityConfig{}, fmt.Errorf("use only [github.app]; legacy [github.magi] cannot be combined")
		}
		identity.GitHub.App = identity.GitHub.LegacyApp
		identity.GitHub.LegacyApp = AppIdentity{}
	}

	if err := identity.GitHub.App.validate(); err != nil {
		return IdentityConfig{}, fmt.Errorf("validate identity: %w", err)
	}

	keyPath, err := expandHomePath(identity.GitHub.App.PrivateKey)
	if err != nil {
		return IdentityConfig{}, fmt.Errorf("resolve private_key: %w", err)
	}
	identity.GitHub.App.PrivateKey = keyPath

	return identity, nil
}

func (identity AppIdentity) validate() error {
	if identity.AppID <= 0 {
		return fmt.Errorf("app_id must be greater than zero")
	}
	if strings.TrimSpace(identity.ClientID) == "" {
		return fmt.Errorf("client_id must not be empty")
	}
	if identity.ClientID != strings.TrimSpace(identity.ClientID) {
		return fmt.Errorf("client_id must not contain surrounding whitespace")
	}

	if identity.InstallationID <= 0 {
		return fmt.Errorf("installation_id must be greater than zero")
	}

	if strings.TrimSpace(identity.PrivateKey) == "" {
		return fmt.Errorf("private_key must not be empty")
	}

	return nil
}
