# MAGI

Local-first GitHub pull-request review in Go, following
`MAGI_Code_Review_System_Design_and_Setup.pdf`.

MELCHIOR focuses on correctness, BALTHASAR on requirements and tests, and
CASPER on security and performance. The design runs reviewers sequentially
through local Ollama models and publishes COMMENT-only feedback through
one shared MAGI GitHub App. Each review names its reviewer role. Humans own
resolution and merge decisions.

Inspired by the MAGI supercomputer system from Evangelion.

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
