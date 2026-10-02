package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrivateKeyPKCS1(t *testing.T) {
	original, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	data := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(original),
	})

	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey() failed: %v", err)
	}

	if !original.Equal(loaded) {
		t.Fatal("loaded key does not match the original")
	}
}

func TestLoadPrivateKeyPKCS8(t *testing.T) {
	original, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := x509.MarshalPKCS8PrivateKey(original)
	if err != nil {
		t.Fatal(err)
	}

	data := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: encoded,
	})

	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadPrivateKey(path)
	if err != nil {
		t.Fatalf("LoadPrivateKey() failed: %v", err)
	}

	if !original.Equal(loaded) {
		t.Fatal("loaded key does not match the original")
	}
}

func TestLoadPrivateKeyMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.pem")

	_, err := LoadPrivateKey(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected a missing-file error, got: %v", err)
	}
}

func TestLoadPrivateKeyInvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")

	if err := os.WriteFile(path, []byte("not a PEM key"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected an error for invalid PEM")
	}

	want := "private key file contains no PEM block"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestLoadPrivateKeyInvalidRSA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.pem")

	data := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: []byte("not an RSA key"),
	})

	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPrivateKey(path)
	if err == nil {
		t.Fatal("expected an error for invalid RSA data")
	}

	if !strings.HasPrefix(err.Error(), "parse RSA private key: ") {
		t.Errorf("expected an RSA parsing error, got: %v", err)
	}
}
