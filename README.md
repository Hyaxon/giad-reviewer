# MAGI

Local-first GitHub pull-request review in Go.

MELCHIOR focuses on correctness, BALTHASAR on requirements and tests, and
CASPER on security and performance. The design runs reviewers sequentially
through local Ollama models and publishes COMMENT-only feedback through
a personal GitHub account or one shared, user-owned MAGI GitHub App. Each review names its reviewer role. Humans own
resolution and merge decisions.

Inspired by the MAGI supercomputer system from Evangelion.

[![Magi Evangelion GIF](https://media1.tenor.com/m/nzH_xPTQmhQAAAAd/magi-evangelion.gif)](https://tenor.com/view/magi-evangelion-voting-ai-gif-11471230947275348500)

## Documentation and status

Start with the [documentation index](docs/README.md) for goals, architecture,
setup, security, and the implementation roadmap. The [decision log](docs/decisions.md)
records updates to the original PDF, including one shared App and repository
selection per command.

The CLI supports help/version output, personal authentication verification, and
read-only PR context retrieval.
App identity/key/JWT helpers exist, but reviews and App token exchange are not implemented. See
[current status](docs/status.md) and the [Go orientation guide](docs/go-development.md).

## Build and run

Use Go 1.27.1 or newer (matching `go.mod`). From this directory:

```sh
go mod download
go build -o bin/magi ./cmd/magi
./bin/magi --help
./bin/magi --version
```

Or run without building a persistent binary:

```sh
go run ./cmd/magi --help
```

Check the scaffold with `go test ./...` and `go vet ./...`.
Unit tests cover identity loading, key/JWT helpers, and personal authentication.

## Continuous integration

[GitHub Actions](.github/workflows/ci.yml) runs on pushes, pull requests, and
manual dispatch. It checks Go formatting, module-file consistency, dependency
checksums, `go vet`, tests with the race detector, and a CLI build/smoke test.
The Go version comes from `go.mod`. No Ollama service or MAGI App credentials
are needed for these checks.

Markdown is checked with [markdownlint-cli2](https://github.com/DavidAnson/markdownlint-cli2)
using the same pinned version locally and in CI. Node.js 22 or newer with npm
is needed only for this documentation tooling, not for building or running MAGI.

Before committing, run from the repository root:

```sh
# Fix Go formatting.
gofmt -w cmd internal

# Fix automatically repairable Markdown issues, then check all Markdown.
npx --yes markdownlint-cli2@0.23.3 --fix
npx --yes markdownlint-cli2@0.23.3

# Check the Go code and module files.
go mod tidy -diff
go vet ./...
go test -race -count=1 ./...
go build -o bin/magi ./cmd/magi
```

The first `npx` run downloads the linter. Review the formatting changes before
staging your commit. Any remaining Markdown errors include a file, line, and rule
to fix manually. CI checks formatting without changing files. Markdown line-length
limits are disabled so long tables and links remain readable in source.

The CLI uses [Cobra](https://cobra.dev/docs/tutorials/getting-started/).
JWT and TOML libraries support the optional App helpers already in progress.

## Personal authentication

Personal mode is the initial CLI path. Install GitHub CLI, then run:

```sh
gh auth login --hostname github.com
go run ./cmd/magi auth status
```

If already signed in, just run the status command. It verifies the effective
account with GitHub and never prints the token. No identity file, App, or private
key is needed. GitHub CLI environment overrides such as `GH_TOKEN` affect the
account used. This verifies identity, not write access to every repository.

Read PR metadata with either form:

```sh
go run ./cmd/magi pr view https://github.com/OWNER/REPO/pull/42
go run ./cmd/magi pr view 42 --repo OWNER/REPO
```

This prints the description, base/head revisions, changed files, and formal linked
issues. Add `--diff` to display the fetched unified diff. Issue retrieval failures
are shown as warnings, distinct from having no linked issues.
Reviews, when implemented, will appear under that account. Dedicated App setup
is deferred; existing App helpers are preserved. See [authentication](docs/authentication.md).

## Package layout

| Path | Intended responsibility |
| --- | --- |
| `cmd/magi` | CLI entry point |
| `internal/agent` | Reviewer state machine and controlled tool loop |
| `internal/config` | TOML and environment configuration |
| `internal/githubauth` | Shared token provider and personal GitHub CLI authentication |
| `internal/githubapp` | App key/JWT helpers; future installation-token provider |
| `internal/model` | Provider-independent model interface |
| `internal/model/ollama` | Ollama HTTP adapter and model unloading |
| `internal/review` | Review runner, findings and publication validation |
| `internal/repo` | Worktrees and base/head revision boundaries |
| `internal/tools` | Bounded repository read/search/diff/test operations |
| `internal/sandbox` | Future isolated execution of PR code |
| `internal/worker` | Future job leasing and worker lifecycle |
| `internal/coordinator` | Future scheduling and queues |
| `prompts` | Future shared and reviewer-specific policies |
