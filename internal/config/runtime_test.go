package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplicitRuntimeConfig(t *testing.T) {
	valid := `[models.review]
provider = "ollama"
endpoint = "http://127.0.0.1:11434"
model = "local-model"
[agents.reviewer]
sandbox_image = "giad-reviewer:local"
capabilities = ["repository.read", "model.chat"]
`
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", valid, true},
		{"unknown field", valid + "\nunknown = true\n", false},
		{"unconfigured example model", `[models.review]
provider="ollama"
endpoint="http://127.0.0.1:11434"
model="REPLACE_WITH_DOWNLOADED_MODEL"
`, false},
		{"missing sandbox image", `[agents.reviewer]
capabilities = []
`, false},
		{"image cannot be CLI options", `[agents.reviewer]
sandbox_image = "--privileged"
`, false},
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

func TestTrustedTestProfiles(t *testing.T) {
	valid := `[agents.reviewer]
sandbox_image="giad-reviewer:local"
capabilities=["tests.run"]
test_profiles=["go"]
[tests.go]
image="giad-go-tests:example"
command="/usr/local/go/bin/go"
args=["test", "./..."]
timeout_seconds=120
`
	for _, test := range []struct {
		name, from, to string
		valid          bool
	}{
		{name: "valid", valid: true},
		{name: "unknown profile", from: `test_profiles=["go"]`, to: `test_profiles=["other"]`},
		{name: "command must be absolute", from: `command="/usr/local/go/bin/go"`, to: `command="go"`},
		{name: "zero timeout", from: `timeout_seconds=120`, to: `timeout_seconds=0`},
		{name: "unbounded timeout", from: `timeout_seconds=120`, to: `timeout_seconds=601`},
		{name: "image options", from: `image="giad-go-tests:example"`, to: `image="--privileged"`},
		{name: "unknown test field", from: `timeout_seconds=120`, to: "timeout_seconds=120\nnetwork=true"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := valid
			if test.from != "" {
				body = strings.Replace(body, test.from, test.to, 1)
			}
			path := filepath.Join(t.TempDir(), "runtime.toml")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadRuntime(path)
			if (err == nil) != test.valid {
				t.Fatalf("cfg=%+v err=%v", cfg, err)
			}
		})
	}
}
