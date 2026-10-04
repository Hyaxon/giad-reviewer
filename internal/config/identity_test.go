package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppIdentityClientID(t *testing.T) {
	for _, clientID := range []string{"", "   ", " Iv1.example"} {
		identity := AppIdentity{AppID: 123, ClientID: clientID, InstallationID: 456, PrivateKey: "/example/key.pem"}
		if err := identity.validate(); err == nil || !strings.Contains(err.Error(), "client_id") {
			t.Errorf("client ID %q: expected client_id validation error, got %v", clientID, err)
		}
	}
}

func TestLoadIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := `
[github.app]
client_id = "Iv1.example"
app_id = 123
installation_id = 456
private_key = "/example/github-app.pem"
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	identity, err := LoadIdentity(path)
	if err != nil {
		t.Fatalf("LoadIdentity() failed: %v", err)
	}

	want := AppIdentity{
		AppID:          123,
		ClientID:       "Iv1.example",
		InstallationID: 456,
		PrivateKey:     "/example/github-app.pem",
	}

	if got := identity.GitHub.App; got != want {
		t.Errorf("App identity = %+v, want %+v", got, want)
	}
}

func TestLoadIdentityMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")

	_, err := LoadIdentity(path)
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestLoadIdentityInvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := "[github.app" // Missing the closing bracket.

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIdentity(path)
	if err == nil {
		t.Fatal("expected an error for invalid TOML")
	}
}

func TestLoadIdentityInvalidAppId(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := `
[github.app]
client_id = "Iv1.example"
app_id = -123
installation_id = 456
private_key = "/example/github-app.pem"
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIdentity(path)
	if err == nil {
		t.Fatal("expected an error for a negative app_id")
	}

	want := "validate identity: app_id must be greater than zero"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestLoadIdentityInvalidAppIdType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := `
[github.app]
client_id = "Iv1.example"
app_id = "abc"
installation_id = 456
private_key = "/example/github-app.pem"
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIdentity(path)
	if err == nil {
		t.Fatal("expected a parsing error for a string app_id")
	}

	if !strings.HasPrefix(err.Error(), "parse identity file: ") {
		t.Errorf("expected a parsing error, got: %v", err)
	}
}

func TestLoadIdentityInvalidInstallationId(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := `
[github.app]
client_id = "Iv1.example"
app_id = 123
installation_id = -456
private_key = "/example/github-app.pem"
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIdentity(path)
	if err == nil {
		t.Fatal("expected an error for a negative installation_id")
	}

	want := "validate identity: installation_id must be greater than zero"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestLoadIdentityInvalidPrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := `
[github.app]
client_id = "Iv1.example"
app_id = 123
installation_id = 456
private_key = ""
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadIdentity(path)
	if err == nil {
		t.Fatal("expected an error for a empty private_key")
	}

	want := "validate identity: private_key must not be empty"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestLegacyIdentityMigration(t *testing.T) {
	content := `[github.magi]
app_id = 1
client_id = "client"
installation_id = 2
private_key = "/example/key.pem"
`
	path := filepath.Join(t.TempDir(), "identity.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	identity, err := LoadIdentity(path)
	if err != nil || identity.GitHub.App.AppID != 1 || identity.GitHub.LegacyApp != (AppIdentity{}) {
		t.Fatalf("identity=%+v err=%v", identity, err)
	}
	if err := os.WriteFile(path, []byte(content+"\n[github.app]\napp_id = 3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIdentity(path); err == nil {
		t.Fatal("ambiguous old and new identity blocks accepted")
	}
}

func TestIdentityRejectsUnknownSectionsAndFields(t *testing.T) {
	for _, content := range []string{
		"[github.giad]\napp_id = 123\n",
		"[github.app]\napp_id = 123\nclient_secret = 'unused'\n",
	} {
		path := filepath.Join(t.TempDir(), "identity.toml")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadIdentity(path); err == nil || !strings.Contains(err.Error(), "parse identity file") {
			t.Fatalf("unknown identity configuration was silently ignored: %v", err)
		}
	}
}
