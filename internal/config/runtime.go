package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hyaxon/giad/internal/model/ollama"
	"github.com/pelletier/go-toml/v2"
)

type Runtime struct {
	Models map[string]ModelProfile `toml:"models"`
	Agents map[string]AgentPolicy  `toml:"agents"`
}
type ModelProfile struct {
	Provider string `toml:"provider"`
	Endpoint string `toml:"endpoint"`
	Model    string `toml:"model"`
}
type AgentPolicy struct {
	Capabilities []string `toml:"capabilities"`
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
		if profile.Provider != "ollama" || strings.TrimSpace(profile.Model) == "" {
			return cfg, fmt.Errorf("models.%s requires provider ollama and a model tag", name)
		}
		if _, err := ollama.New(profile.Endpoint); err != nil {
			return cfg, fmt.Errorf("models.%s: %w", name, err)
		}
	}
	return cfg, nil
}
