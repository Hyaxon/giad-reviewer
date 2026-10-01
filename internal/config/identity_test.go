package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.toml")

	content := `
[github.magi]
app_id = 123
installation_id = 456
private_key = "/example/magi.pem"
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
		InstallationID: 456,
		PrivateKey:     "/example/magi.pem",
	}

	if got := identity.GitHub.MAGI; got != want {
		t.Errorf("MAGI identity = %+v, want %+v", got, want)
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

	content := "[github.magi" // Missing the closing bracket.

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
[github.magi]
app_id = -123
installation_id = 456
private_key = "/example/magi.pem"
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
[github.magi]
app_id = "abc"
installation_id = 456
private_key = "/example/magi.pem"
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
[github.magi]
app_id = 123
installation_id = -456
private_key = "/example/magi.pem"
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
[github.magi]
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
