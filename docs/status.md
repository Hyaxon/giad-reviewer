# Current status

Snapshot: October 1, 2026. Authentication groundwork exists; reviews do not yet run.

## Implemented

- Cobra CLI: help, version, `magi auth status`, and `magi pr view`.
- Read-only PR context retrieval by URL or number plus `--repo`: metadata, changed
  files, unified diff, and formal linked issues. `--diff` displays the patch.
- Personal token provider using `gh auth token --hostname github.com`.
- Read-only GitHub identity verification; tokens are not displayed or saved in TOML.
- Provider interface with repository context for future App authentication.
- Optional App identity loading/validation, home-path expansion, PEM parsing,
  and client-ID JWT signing. Personal mode does not load these credentials.
- Tests for configuration, keys, JWTs, and personal authentication.
- CI formatting, vet, tests, build, and Markdown lint checks.

## Setup

Run `gh auth login --hostname github.com` if needed, then
`go run ./cmd/magi auth status`. This checks the effective user token, including
GitHub CLI environment overrides. Repository-specific permissions are checked later.

The initial Ollama models were confirmed downloaded during scaffold setup.
The owner also reports a configured App and local key, but App integration is
now deferred. General runtime/model TOML loading and the setup wizard are not wired
into the CLI.

## Next milestone

Prepare an isolated checkout bound to the fetched PR revisions, then add controlled
repository-reading tools and the first model/tool review loop.

Changed files and formal issue links are paginated. Linked-issue failures are explicit
warnings. PR revisions are rechecked after retrieval; changed revisions fail the
command. GitHub responses are capped at 16 MiB each, file listings at GitHub's 3000
files, and issue retrieval at 1000 issues. These are retrieval guards, not an atomic
snapshot guarantee; revalidate again before eventual publication.

Repository inference, local checkout, installation-token exchange, model inference,
review publication, setup, worker, and coordinator modes remain unimplemented.
