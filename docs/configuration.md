# Configure an agent

Use explicit trusted files outside the PR checkout: a manifest describing the
executable and declarations, and TOML granting capabilities/mapping model profiles.

For the runnable model-free example:

```sh
make example
make images
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml
```

Manifest executable/script paths refer to files inside the installed Linux image.
No model endpoint or credentials belong in the agent. Reviews never build or pull
images and fail if Docker or the configured image is unavailable.

For a custom agent, copy [the manifest template](../example/agent.manifest.json)
and [giad.example.toml](../giad.example.toml). Match the agent name/profile names;
set the image's absolute executable/script paths and any downloaded model tags.

| Config entry | Meaning |
| --- | --- |
| `[models.NAME]` | Logical profile declared by the agent; omit for model-free agents |
| `provider` | Currently `"ollama"` |
| `endpoint` | HTTP(S) origin; remote origins send your source conversations there |
| `model` | Downloaded model tag |
| `[agents.NAME].capabilities` | Agent's permitted broker methods |
| `[agents.NAME].sandbox_image` | Required locally installed Linux agent image; tags resolve to an immutable ID per launch |
| `[agents.NAME].test_profiles` | Approved profile names for this agent; also requires declared/granted `tests.run` |
| `[tests.NAME]` | Fixed test `image`, absolute `command`, `args`, and `timeout_seconds` (1–600) |

Required capabilities must be declared, granted, and implemented; otherwise launch
fails. Optional capabilities remain off unless all three conditions hold. See
[the protocol](agent-protocol.md). There is no automatic config lookup or agent registry.

To enable Go tests for the code-review example after `make images`:

```toml
[agents.code-review]
sandbox_image = "giad-code-review:example"
capabilities = ["git.diff", "repository.read", "repository.instructions", "model.chat", "tests.run"]
test_profiles = ["go"]

[tests.go]
image = "giad-go-tests:example"
command = "/usr/local/go/bin/go"
args = ["test", "-p", "1", "./..."]
timeout_seconds = 120
```

Keep your existing `[models.review]` settings. Tests run only on request; there is
no automatic baseline. Images must contain `/giad-test`, a trusted helper that
extracts the sanitized stdin tar into `/workspace` and executes the configured
command. The Go example caches this repository's modules at image build time.
For other projects, prepare their dependencies in a trusted image beforehand;
reviews have no network or dependency installation step.

Auth currently uses GitHub CLI. App helpers exist but token exchange is not wired.
The separate [identity example](../identity.example.toml) uses `[github.app]`; legacy
`[github.magi]` remains accepted by the helper. Do not combine both sections.
