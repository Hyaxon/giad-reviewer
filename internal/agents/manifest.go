// Package agents launches explicitly approved local agent processes and brokers capabilities.
package agents

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyaxon/giad/pkg/protocol"
)

func decode(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}

// LoadManifest never searches the PR checkout for configuration or executables.
// Commands are absolute paths inside the trusted installed agent image.
func LoadManifest(path string) (protocol.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return protocol.Manifest{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return protocol.Manifest{}, err
	}
	if len(data) > 64*1024 {
		return protocol.Manifest{}, errors.New("agent manifest exceeds 64 KiB")
	}
	var m protocol.Manifest
	if err := decode(data, &m); err != nil {
		return m, fmt.Errorf("parse agent manifest: %w", err)
	}
	if m.APIVersion != protocol.Version {
		return m, fmt.Errorf("unsupported agent apiVersion %q; expected %q", m.APIVersion, protocol.Version)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Version) == "" {
		return m, errors.New("agent name and version are required")
	}
	if !filepath.IsAbs(m.Entrypoint.Command) {
		return m, errors.New("agent entrypoint must be an absolute path to a trusted executable")
	}
	return m, nil
}

// Grants intersect agent declarations with host policy. Required capabilities fail closed.
func Grants(m protocol.Manifest, policy []string) ([]string, error) {
	allowed := map[string]bool{}
	for _, name := range policy {
		allowed[name] = true
	}
	var result []string
	seen := map[string]bool{}
	for _, name := range m.Capabilities.Required {
		if !supported(name) || !allowed[name] {
			return nil, fmt.Errorf("required capability %q is unavailable or denied", name)
		}
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}
	for _, name := range m.Capabilities.Optional {
		if supported(name) && allowed[name] && !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}
	return result, nil
}
func supported(name string) bool {
	switch name {
	case "repository.read", "repository.search", "git.diff", "repository.instructions", "github.linked_issues", "model.chat", "tests.run":
		return true
	}
	return false
}
