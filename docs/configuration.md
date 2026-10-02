# Configuration

Status: accepted settings plus a proposed loading design. No configuration is
currently read by the CLI.

## What belongs in configuration

- Authentication mode: personal account or a dedicated user-owned GitHub App.
- App identity and private-key path only when App mode is selected.
- Model choice for each reviewer and configurable inference endpoints.
- Sequential execution, context budgets, and unload policy.
- Publication thresholds and preview behavior.
- Requirements discovery policy.

The repository and PR are invocation inputs. The three roles do not need three
credential blocks. The PDF's `[github.melchior]`, `[github.balthasar]`, and
`[github.casper]` examples are superseded.

The proposed mode selector and migration from the current identity file are described
in [authentication modes](authentication.md). The existing loader handles App identity
only; it must not be required by future personal-account mode.

## Existing example

[`magi.example.toml`](../magi.example.toml) currently records these defaults:

| Setting | Proposed V1 value |
| --- | --- |
| MELCHIOR model | `qwen3.6:27b` |
| BALTHASAR model | `devstral-small-2` |
| CASPER model | `qwen3-coder:30b` |
| Enabled reviewers | All three in the example; first implemented slice is MELCHIOR |
| Sequential execution | `true` |
| Unload after each reviewer | `true` |
| Context tokens | `32768` starting budget |
| Publication event | `COMMENT` |
| Preview before publishing | `true` |
| Minimum confidence | `0.85` starting heuristic |
| Requirements source | `github_linked_issues` |
| Missing link | `warn` |
| Read issue comments | `false` |
| Allow inferred blocking findings | `false` |

This is example data, not validation already enforced in code.

## Proposed local configuration location

Use a user-level file such as `~/.config/magi/config.toml` so the CLI can be run
from many repositories. The filename and lookup rules are proposals to implement.
Keep the private key separately at `~/.config/magi/keys/magi.pem`.

`~` means the current user's home directory. On the initial Mac, the key location
is `/Users/hydeharris/.config/magi/keys/magi.pem`. It is neither the filesystem root
`/` nor the project's `.config` directory.

An illustrative addition to the example settings would be:

```toml
# Proposed schema; replace these placeholder IDs with local values.
[github]
app_id = 123456
installation_id = 12345678
private_key = "~/.config/magi/keys/magi.pem"

[ollama]
endpoint = "http://localhost:11434"
```

Do not commit real credentials or paste key contents into config. App IDs are not
private keys, but machine/account-specific values still belong in local configuration.

## Proposed loading and validation

Prefer an explicit `--config` path when supplied; otherwise use the user-level
configuration. Resolve supported command-line overrides after loading defaults
and the file. Environment override names and precedence are not finalized.

Do not automatically trust a configuration file inside a PR checkout. In particular,
repository content must not replace credentials, model destinations, executable
commands, or publication policy. Per-project policy requires an explicit trust design.

Validate required fields, positive IDs, model names, endpoint URLs, confidence
range, and resource budgets. Expand `~/` deliberately; Go's file APIs do not
automatically provide shell expansion. Give field-specific errors without printing
private keys or authentication tokens.

Treat COMMENT-only publication as a program invariant. Reject unsupported event
values instead of allowing a config edit to silently enable approvals or changes requests.

## Future model profiles

The PDF also sketches reusable `[models.<name>]` entries containing provider,
endpoint, and model tag, with agents referring to profile names. That is a useful
future extension for remote/mixed providers. Do not ambiguously interpret an agent's
`model` as both an Ollama tag and a profile reference; choose and document a schema
before implementing profiles.
