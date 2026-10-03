# GIAD

A self-hosted runtime for programmable pull-request review agents. GIAD handles
GitHub access, PR checkouts, repository tools, optional local models, and draft
previews. Agents supply review judgment; MAGI will be an optional separate package.

**Prototype:** PR inspection and external-agent previews work. A model-free
`diff-inspector` example is included. Agent isolation, publishing, and test execution
are not implemented; run only trusted executables.

## Get started

Install Go (version in `go.mod`), Git, and GitHub CLI, then:

```sh
make                       # Build GIAD and the example agent/manifest.
make smoke                 # Offline runtime + real agent integration check.
gh auth login --hostname github.com
./bin/giad auth status
./bin/giad pr view 42 --repo OWNER/REPO --diff
```

Run the example on a PR (no Ollama required):

```sh
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml --preview
```

The example retrieves the diff and returns no findings; it performs no defect
analysis. All results are local. Add `--json` for structured output. Full GitHub PR
URLs also work; use `--help` for command options.

## Build your own agent

Start with the [tiny Python example](example/README.md).
Follow [configuration](docs/configuration.md) and [the protocol](docs/agent-protocol.md).
Use [giad.example.toml](giad.example.toml) for model-backed agents; model-free agents
need no model configuration. Start Ollama only when your selected agent uses it.

## Development

```sh
gofmt -w cmd internal pkg example
make check                 # Module checks, vet, and race tests.
make lint                  # Pinned Markdown linter.
```

[Architecture](docs/architecture.md) explains the boundaries and next milestones.
The revised `GIAD_Architecture_and_Setup.pdf` is the design reference.
