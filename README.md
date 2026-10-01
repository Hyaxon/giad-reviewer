# MAGI

Local-first GitHub pull-request review in Go, following
`MAGI_Code_Review_System_Design_and_Setup.pdf`.

MELCHIOR focuses on correctness, BALTHASAR on requirements and tests, and
CASPER on security and performance. The design runs reviewers sequentially
through local Ollama models and publishes COMMENT-only feedback through
one shared MAGI GitHub App. Each review names its reviewer role. Humans own
resolution and merge decisions.

Inspired by the MAGI supercomputer system from Evangelion.

[![Magi Evangelion GIF](https://media1.tenor.com/m/nzH_xPTQmhQAAAAd/magi-evangelion.gif)](https://tenor.com/view/magi-evangelion-voting-ai-gif-11471230947275348500)

## Documentation and status

Start with the [documentation index](docs/README.md) for goals, architecture,
setup, security, and the implementation roadmap. The [decision log](docs/decisions.md)
records updates to the original PDF, including one shared App and repository
selection per command.

The current implementation is a scaffold with help/version output. Configuration
loading, GitHub authentication, and reviews are not implemented. See
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
There are no automated tests yet.

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
The guide's JWT and TOML dependencies are optional and will be added when
authentication and configuration loading use them. `go mod tidy` removes unused
dependencies, so adding them to this scaffold alone would not retain them.

## Package layout

| Path | Intended responsibility |
| --- | --- |
| `cmd/magi` | CLI entry point |
| `internal/agent` | Reviewer state machine and controlled tool loop |
| `internal/config` | TOML and environment configuration |
| `internal/githubapp` | App JWTs, installation tokens, PR APIs and publishing |
| `internal/model` | Provider-independent model interface |
| `internal/model/ollama` | Ollama HTTP adapter and model unloading |
| `internal/review` | Review runner, findings and publication validation |
| `internal/repo` | Worktrees and base/head revision boundaries |
| `internal/tools` | Bounded repository read/search/diff/test operations |
| `internal/sandbox` | Future isolated execution of PR code |
| `internal/worker` | Future job leasing and worker lifecycle |
| `internal/coordinator` | Future scheduling and queues |
| `prompts` | Future shared and reviewer-specific policies |
