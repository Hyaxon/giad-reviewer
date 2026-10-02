package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitRuntimeConfig(t *testing.T) {
	valid := `[models.review]
provider = "ollama"
endpoint = "http://127.0.0.1:11434"
model = "local-model"
[agents.reviewer]
capabilities = ["repository.read", "model.chat"]
`
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", valid, true},
		{"unknown field", valid + "\nunknown = true\n", false},
		{"unknown provider", `[models.review]
provider="shell"
endpoint="http://127.0.0.1:11434"
model="anything"
`, false},
		{"credential endpoint", `[models.review]
provider="ollama"
endpoint="http://secret@example.test"
model="anything"
`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadRuntime(path)
			if (err == nil) != test.valid {
				t.Fatalf("cfg=%+v err=%v", cfg, err)
			}
		})
	}
}
