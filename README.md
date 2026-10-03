# GIAD

A self-hosted runtime for programmable pull-request review agents. GIAD handles
GitHub access, PR checkouts, repository tools, optional local models, and draft
previews. Agents supply review judgment; MAGI will be an optional separate package.

**Prototype:** PR inspection and external-agent previews work. A model-free
`diff-inspector` example is included. Reviews and optional approved test profiles
run in separate Docker containers. Saved drafts can be published as confirmed
GitHub reviews, including request-changes reviews and inline findings.

## Get started

Install Go (version in `go.mod`), Git, GitHub CLI, and a running Linux Docker engine:

```sh
make                       # Build GIAD and the example agent/manifest.
make images                # Package the examples as local agent images.
make sandbox-smoke         # Verify isolation and run an isolated example.
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
analysis. Review results are local. Add `--json` for structured output. Full GitHub PR
URLs also work; use `--help` for command options.

To publish, first save and preview the draft:

```sh
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml --json > draft.json
./bin/giad publish draft.json
# Inspect the displayed body, then rerun with its --confirm HASH.
```

For request-changes reviews with findings attached to their source lines:

```sh
./bin/giad publish draft.json --event REQUEST_CHANGES --inline
# Inspect the summary and inline comments, then use the new printed hash:
./bin/giad publish draft.json --event REQUEST_CHANGES --inline --confirm HASH
```

The default remains a body-only `COMMENT` review. `--inline` also works with
`COMMENT`. Publication rechecks PR revisions and head-side diff anchors; the
confirmation covers the action and every inline comment. Retries reconcile the
review and its comments without blindly resending uncertain attempts. GitHub
rejections include their validation reason. Agents have no publication capability.

## Build your own agent

Start with the [tiny Python example](example/README.md).
The same page includes a small model-backed reviewer that returns anchored findings.
Follow [configuration](docs/configuration.md) and [the protocol](docs/agent-protocol.md).
Use [giad.example.toml](giad.example.toml) for model-backed agents; model-free agents
need no model configuration. Start Ollama only when your selected agent uses it.

## Development

```sh
gofmt -w cmd internal pkg example
make check                 # Module checks, vet, and race tests.
make smoke                 # Offline protocol check using a trusted test process.
make lint                  # Pinned Markdown linter.
```

[Architecture](docs/architecture.md) explains the boundaries and next milestones.
The revised `GIAD_Architecture_and_Setup.pdf` is the design reference.
