package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandHomePath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{"home", "~", homeDir},
		{"home key", "~/.config/agentic-review/keys/agentic-review.pem", filepath.Join(homeDir, ".config", "agentic-review", "keys", "agentic-review.pem")},
		{"absolute", "/example/agentic-review.pem", "/example/agentic-review.pem"},
		{"relative", "keys/agentic-review.pem", "keys/agentic-review.pem"},
		{"spaces", "keys/my key.pem", "keys/my key.pem"},
		{"literal variable", "$HOME/key.pem", "$HOME/key.pem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := expandHomePath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("expanded path = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadIdentityHomePath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "identity.toml")
	content := `[github.app]
client_id = "Iv1.example"
app_id = 123
installation_id = 456
private_key = "~/.config/agentic-review/keys/agentic-review.pem"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	identity, err := LoadIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(homeDir, ".config", "agentic-review", "keys", "agentic-review.pem")
	if got := identity.GitHub.App.PrivateKey; got != want {
		t.Errorf("private key path = %q, want %q", got, want)
	}
}

func TestLoadIdentityUnsupportedHomePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")
	content := `[github.app]
client_id = "Iv1.example"
app_id = 123
installation_id = 456
private_key = "~another-user/key.pem"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadIdentity(path)
	if err == nil || !strings.HasPrefix(err.Error(), "resolve private_key: unsupported home path") {
		t.Fatalf("expected unsupported home path error, got %v", err)
	}
}
