# Configuration

Choose trusted files explicitly, outside the PR checkout:

- `--agent-manifest`: installed executable, declared capabilities, model profiles.
- `--config`: granted capabilities, Docker images, models, and test profiles.
- `--identity`: optional GitHub App IDs and private-key path.

For a custom agent, copy [the manifest template](../example/agent.manifest.json)
and [giad.example.toml](../giad.example.toml). Agent/profile names must match.
Entrypoint paths refer to files inside the installed Linux image. Reviews do not
build or pull images; interpreters and dependencies must already be packaged.

## Runtime policy

| Entry | Meaning |
| --- | --- |
| `[models.NAME]` | Host-mapped logical profile; omit for model-free agents |
| `provider` | Currently `"ollama"` |
| `endpoint` | HTTP(S) origin; remote origins receive source conversations |
| `model` | Exact installed model tag from `ollama list` |
| `[agents.NAME].capabilities` | Permitted broker methods |
| `[agents.NAME].sandbox_image` | Locally installed Linux agent image |
| `[agents.NAME].test_profiles` | Approved test names; requires declared/granted `tests.run` |
| `[tests.NAME]` | Fixed image, absolute command, args, timeout (1–600 seconds) |

Required capabilities must be declared, granted, and implemented; otherwise launch
fails. Optional capabilities remain off unless all three hold. Unknown fields fail
validation. There is no automatic config lookup or agent registry.

## Optional tests

After `make images`, enable Go tests by updating the agent block in your local
config and adding the test profile below. Keep your existing model settings.

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

Profiles can run other languages with the appropriate image and fixed command.
Every image needs a trusted `/giad-test` helper to extract the sanitized stdin tar
into `/workspace` and execute that command. The Go example caches this repository's
modules at image build time; prepare another project's dependencies in its own
trusted image. Reviews have no network or dependency installation step.

Tests are agent-requested and head-only. GIAD retains exit status, bounded merged
output, duration, timeout, truncation, and memory-limit status. A passing command
does not establish that tests existed or that the code is correct.

## Authentication

Personal mode is the default:

```sh
gh auth login --hostname github.com
./bin/giad auth status
```

For App mode, copy [identity.example.toml](../identity.example.toml). On the App's
GitHub settings page, copy the App ID and Client ID, generate a private key, and
install the App on the repositories you want to review. The installation Configure
page's URL ends in the installation ID.

Use `[github.app]` and a local PEM file path. Grant Contents read, Issues read,
and Pull requests read/write; Metadata read is automatic. This manual workflow
needs no client secret or webhook. Keep the private key outside the repository.

```sh
./bin/giad auth status --identity identity.toml
./bin/giad pr view 42 --repo OWNER/REPO --identity identity.toml
```

Pass the same identity flag to `review` and both publication preview/confirmation
commands. Status verifies App identity, installation grants, token exchange, and
bot identity. Repository access is checked separately. Tokens stay in host memory,
are repository-scoped, and refresh before expiry. Invalid App auth fails without
personal fallback. Legacy `[github.magi]` is accepted separately for existing files.
