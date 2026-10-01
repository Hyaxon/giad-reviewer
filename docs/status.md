# Current status

Snapshot: September 30, 2026. This is a scaffold, not a working review tool.

## Implemented and inspected

| Item | State |
| --- | --- |
| Go module | `github.com/hyaxon/magi-agents`, Go directive `1.27.1` |
| CLI | Cobra root command in `cmd/magi/main.go` |
| Help | Running `magi` or `magi --help` prints the scaffold description |
| Version | `magi --version` prints the development version |
| Dependencies | Cobra plus its indirect dependencies; no JWT or TOML library yet |
| Package boundaries | `internal/*/doc.go` files reserve responsibilities |
| Prompts | A README only; no operational reviewer prompts |
| Configuration | `magi.example.toml` exists but is not loaded by the program |
| Tests | No automated test cases yet |

The scaffold previously built successfully and passed `go vet ./...`.
`go test ./...` completed with no test files; that is not evidence of review behavior.

## Local setup evidence

- Go `1.27.1` was available when the scaffold was created.
- Ollama's model list confirmed `qwen3.6:27b`, `devstral-small-2:latest`, and
  `qwen3-coder:30b` were downloaded. Inference smoke tests have not been recorded.
- The owner reports creating one personal-account MAGI GitHub App and installing
  it with access to all their repositories.
- The owner supplied App and Installation IDs and reports storing the key at
  `~/.config/magi/keys/magi.pem`. The key and API authentication have not been validated.

Actual App permissions and repository access still need a live authentication
check during implementation. No repository is permanently selected.

## Not implemented

Configuration parsing; credential loading; App JWTs; installation tokens;
repository inference; PR fetching; linked issues; worktrees; model inference;
tool execution; finding validation; preview/confirmation; publication; worker
and coordinator modes.

The scaffold's CLI help still mentions separate GitHub Apps. That text reflects
the original PDF and is superseded by the [single-App decision](decisions.md).
It is recorded here rather than changed as part of this documentation-only task.

## Immediate next milestone

Implement configuration loading with one shared GitHub identity, then read-only
authentication and PR context retrieval. See [the roadmap](roadmap.md) for
acceptance criteria. Creating these docs does not implement those milestones.
