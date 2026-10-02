# Agentic Review

A local runtime for independent code-review agents. It handles GitHub access,
PR checkouts, repository tools, local models, and draft previews. Custom agents such as
MAGI supply their own prompts and review judgment in separate repositories.

**Early development:** PR inspection works. Agent reviews require a separately
installed agent; none ships here yet. Publishing and test execution are not implemented.

## Get started

Install Go (version in `go.mod`), Git, and GitHub CLI, then:

```sh
go build -o bin/agentic-review ./cmd/agentic-review
gh auth login --hostname github.com
./bin/agentic-review auth status
./bin/agentic-review pr view 42 --repo OWNER/REPO --diff
```

A full GitHub PR URL also works. Use `pr inspect` to read/search files and
`pr checkout --keep` to retain a disposable checkout. Run any command with `--help`
for its options.

## Run an agent

Start Ollama with your chosen model downloaded. Follow [configuration](docs/configuration.md)
to select an installed agent and grant its capabilities, then:

```sh
./bin/agentic-review review 42 --repo OWNER/REPO \
  --agent-manifest /path/to/agent.json --config /path/to/config.toml
```

Results are local drafts. Add `--json` for structured output. Run only trusted
agent executables; they are not sandboxed from your computer.

## Development

```sh
gofmt -w cmd internal pkg
go vet ./...
go test -race -count=1 ./...
npx --yes markdownlint-cli2@0.23.3
```

See [architecture](docs/architecture.md) for runtime boundaries and next steps,
and [the protocol](docs/agent-protocol.md) when building an agent.
