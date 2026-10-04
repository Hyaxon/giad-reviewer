# GIAD

A self-hosted runtime for programmable pull-request review agents. GIAD handles
GitHub access, checkouts, sandboxed execution, repository tools, optional models,
and confirmed publication. Agents supply the review judgment.

V1 provides a manual workflow: inspect a PR, run one installed agent, save a
draft, and publish a confirmed review. Builds use the release version in
[VERSION](VERSION); check your binary with `./bin/giad --version`.

## Get started

V1 targets Linux amd64 with Docker Engine and macOS Apple Silicon with Docker
Desktop. Native Windows is unsupported; other architectures and WSL2 are not yet
validated. See [platforms and setup](docs/getting-started.md) for the support matrix,
fresh installation, authentication, and troubleshooting.

Install Go (version in `go.mod`), Git, and a running Linux Docker engine.
Personal authentication also needs GitHub CLI; model-backed agents need Ollama.

Run from the repository root:

```sh
make
make images
gh auth login --hostname github.com
./bin/giad auth status
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml
```

This model-free example retrieves the diff and returns no findings. It demonstrates
the protocol, not defect analysis. Full GitHub PR URLs also work.

For the general model-backed reviewer, follow [the examples](example/README.md).
The optional [official GIAD Agents collection](https://github.com/Hyaxon/giad-agents)
now provides `pr-summary`, `diff-inspector`, `test-summary`, and `code-review`,
with a source catalog, manifests, and setup instructions. Build its image locally
and select a package explicitly; it uses the same protocol and permissions as
third-party agents.
For your own GitHub App, pass `--identity identity.toml` to `auth status`, `pr`,
`review`, and `publish`; see [authentication](docs/configuration.md#authentication).

## Save and publish

Add `--json > draft.json` to the review command to save its result. Inspect the
publication preview, then repeat with the printed confirmation hash:

```sh
./bin/giad publish draft.json --inline
./bin/giad publish draft.json --inline --confirm HASH
```

The default event is `COMMENT`. Add `--event REQUEST_CHANGES` to both commands to
request changes. With App auth, include `--identity identity.toml` in both.
Personal accounts cannot request changes on their own PRs.
Inline findings must fall within a head-side diff hunk. Omit `--inline` to publish
findings elsewhere in an inspected changed head file in the review body.

GIAD checks current revisions and inline anchors before posting. Confirmation
covers the action, body, and comments. Rerunning the same confirmed command
recognizes completed reviews even after a push or merge. Uncertain attempts stay
guarded against resending. Comments preserve code formatting.
Base/head hashes stay in the draft and validation, outside the visible review body.
Nothing publishes during `review`; agents have no publication capability.

## Current limitations

- **Workflow:** github.com only, one agent per command, and serial execution.
  There is no hosted service, webhook automation, automatic catalog discovery,
  or automatic agent installation. The official source catalog is available separately.
- **Agents and models:** install trusted Linux images and pass configuration
  explicitly. Ollama is the only model provider. A remote model endpoint receives
  the source conversations sent to it. The examples teach the protocol; their
  findings and model accuracy need independent evaluation.
- **Repository coverage:** reads require regular UTF-8 text files and exclude
  symlinks and Git metadata. Submodules and Git LFS content are not fetched.
  Large jobs can fail a budget check; bounded tools report incomplete coverage.
  Exact limits are in the [agent protocol](docs/agent-protocol.md#finish).
- **Tests:** agents must request a host-approved profile. Runs are head-only,
  with no base comparison or network access. Dependencies must be in the trusted
  image. Test snapshots reject symlinks and special files; passing tests do not
  establish that tests existed or that the code is correct.
- **Publication:** only `COMMENT` and `REQUEST_CHANGES` are supported. Inline
  findings use single head-side diff lines; deleted-line and multiline comments
  are unavailable. GitHub has no atomic base/head publication precondition, so a
  final revision check narrows the remaining write race.
- **Validation:** Ubuntu CI is configured for builds, tests, and Docker isolation;
  macOS Apple Silicon has been exercised locally. A fresh end-to-end setup on a
  separate repository and live App/model checks still need release validation.

## Planned features and checks

V1 supports github.com, one agent per review, preinstalled Docker images, optional
Ollama models, approved head-only tests, and confirmed `COMMENT` or
`REQUEST_CHANGES` publication. The public contract is `giad/v1`; incompatible
wire changes require a new protocol version. Model accuracy and each project's
agent/test-image setup need their own validation.

See the [runtime roadmap](docs/roadmap.md) for proposed local/range reviews,
automation and queues, execution/publication controls, coding-agent integrations,
broker tools, installation, and distributed workers. Scope, order, and command
names remain open. The official catalog now contains its first starter collection;
planned specialist agents belong in the
[GIAD Agents roadmap](https://github.com/Hyaxon/giad-agents/blob/main/docs/roadmap.md).

## Reference and development

- [Getting started](docs/getting-started.md): platforms, setup, first draft, publication.
- [Examples](example/README.md): runnable agents and optional Go tests.
- [Configuration](docs/configuration.md): trusted policy, models, tests, authentication.
- [Agent protocol](docs/agent-protocol.md): the public `giad/v1` contract.
- [Architecture](docs/architecture.md): boundaries and supported scope.
- [Runtime roadmap](docs/roadmap.md): proposed features and their implementation boundaries.
- [Contributing](CONTRIBUTING.md): development workflow and PR expectations.
- [AGENTS.md](AGENTS.md): instructions for coding agents working in this repository.

```sh
make check                 # Module checks, vet, and race tests.
make smoke                 # Offline agent/broker integration.
make sandbox-smoke         # Real Docker isolation; build images first.
make lint                  # Markdown checks.
```

## License

GIAD is licensed under the [Apache License, Version 2.0](LICENSE).
