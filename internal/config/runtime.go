package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hyaxon/giad/internal/model/ollama"
	"github.com/hyaxon/giad/internal/sandbox"
	"github.com/pelletier/go-toml/v2"
)

type Runtime struct {
	Models map[string]ModelProfile        `toml:"models"`
	Agents map[string]AgentPolicy         `toml:"agents"`
	Tests  map[string]sandbox.TestProfile `toml:"tests"`
}
type ModelProfile struct {
	Provider string `toml:"provider"`
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
}
type AgentPolicy struct {
	Capabilities []string `toml:"capabilities"`
	SandboxImage string   `toml:"sandbox_image"`
	TestProfiles []string `toml:"test_profiles"`
}

// LoadRuntime reads only the explicitly selected trusted file.
func LoadRuntime(path string) (Runtime, error) {
	f, err := os.Open(path)
	if err != nil {
		return Runtime{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return Runtime{}, err
	}
	if len(data) > 64*1024 {
		return Runtime{}, errors.New("runtime config exceeds 64 KiB")
	}
	var cfg Runtime
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("parse runtime config: %w", err)
	}
	for name, profile := range cfg.Models {
		if profile.Model == "REPLACE_WITH_DOWNLOADED_MODEL" {
			return cfg, fmt.Errorf("models.%s.model is an example placeholder; replace it with a downloaded Ollama model tag", name)
		}
		if profile.Provider != "ollama" || strings.TrimSpace(profile.Model) == "" {
			return cfg, fmt.Errorf("models.%s requires provider ollama and a model tag", name)
		}
		if _, err := ollama.New(profile.Endpoint); err != nil {
			return cfg, fmt.Errorf("models.%s: %w", name, err)
		}
	}
	for name, policy := range cfg.Agents {
		if err := sandbox.ValidateImage(policy.SandboxImage); err != nil {
			return cfg, fmt.Errorf("agents.%s: %w", name, err)
		}
		for _, profile := range policy.TestProfiles {
			if _, ok := cfg.Tests[profile]; !ok {
				return cfg, fmt.Errorf("agents.%s: test profile %q is not configured", name, profile)
			}
		}
	}
	for name, profile := range cfg.Tests {
		if name == "" || len(name) > 128 {
			return cfg, errors.New("test profile name must be nonempty and at most 128 bytes")
		}
		if err := profile.Validate(); err != nil {
			return cfg, fmt.Errorf("tests.%s: %w", name, err)
		}
	}
	return cfg, nil
}
