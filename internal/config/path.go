package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandHomePath expands ~ and ~/ using the current user's home directory.
// Other paths are preserved; shell variables and named-user expansion are unsupported.
func expandHomePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return "", fmt.Errorf("unsupported home path %q: use ~ or ~/", path)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}
	if path == "~" {
		return homeDir, nil
	}
	return filepath.Join(homeDir, path[2:]), nil
}
