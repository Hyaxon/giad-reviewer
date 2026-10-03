package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/hyaxon/giad/internal/githubapp"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/spf13/cobra"
)

func TestExplicitIdentitySelection(t *testing.T) {
	root := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(root, "key.pem")
	keyData := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyPath, keyData, 0600); err != nil {
		t.Fatal(err)
	}
	identityPath := filepath.Join(root, "identity.toml")
	content := fmt.Sprintf("[github.app]\napp_id=123\nclient_id='client'\ninstallation_id=456\nprivate_key=%q\n", keyPath)
	if err := os.WriteFile(identityPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"personal", "app", "missing", "empty"} {
		t.Run(mode, func(t *testing.T) {
			// Exercise inherited flags as used by auth status and pr subcommands.
			parent := &cobra.Command{Use: "parent", SilenceErrors: true, SilenceUsage: true}
			addIdentityFlag(parent)
			parent.SetOut(io.Discard)
			parent.SetErr(io.Discard)
			child := &cobra.Command{Use: "child", RunE: func(cmd *cobra.Command, _ []string) error {
				auth, err := commandAuth(cmd)
				if mode == "missing" || mode == "empty" {
					if err == nil || auth != nil {
						t.Fatal("invalid App config fell back to personal auth")
					}
					return nil
				}
				if err != nil {
					return err
				}
				if mode == "app" {
					if _, ok := auth.(*githubapp.Provider); !ok {
						t.Fatal("explicit identity did not select App auth")
					}
				} else if _, ok := auth.(githubauth.UserAuth); !ok {
					t.Fatal("personal mode unexpectedly loaded an identity")
				}
				return nil
			}}
			parent.AddCommand(child)
			args := []string{"child"}
			switch mode {
			case "app":
				args = append(args, "--identity", identityPath)
			case "missing":
				args = append(args, "--identity", filepath.Join(root, "absent.toml"))
			case "empty":
				args = append(args, "--identity=")
			}
			parent.SetArgs(args)
			if err := parent.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
