package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyaxon/giad/pkg/protocol"
)

func TestManifestVersionBoundary(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"giad/v1", "agentic-review/v1", "giad/v2"} {
		t.Run(version, func(t *testing.T) {
			data, err := json.Marshal(protocol.Manifest{APIVersion: version, Name: "example", Version: "1", Entrypoint: protocol.Entrypoint{Command: executable}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "agent.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = LoadManifest(path)
			if version == protocol.Version {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), version) || !strings.Contains(err.Error(), protocol.Version) {
				t.Fatalf("expected explicit incompatible-version error, got %v", err)
			}
		})
	}
}
