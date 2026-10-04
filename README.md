# GIAD

A self-hosted runtime for programmable pull-request review agents. GIAD handles
GitHub access, checkouts, sandboxed execution, repository tools, optional models,
and confirmed publication. Agents supply the review judgment.

V1 provides a manual workflow: inspect a PR, run one installed agent, save a
draft, and publish a confirmed review. Builds use the release version in
[VERSION](VERSION); check your binary with `./bin/giad --version`.

## Get started

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

For a small model-backed reviewer, follow [the examples](example/README.md).
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

## Planned features and checks

V1 supports github.com, one agent per review, preinstalled Docker images, optional
Ollama models, approved head-only tests, and confirmed `COMMENT` or
`REQUEST_CHANGES` publication. The public contract is `giad/v1`; incompatible
wire changes require a new protocol version. Model accuracy and each project's
agent/test-image setup need their own validation.

Future features, with scope and order still open:

- **Optional centralized service:** user sign-in and GitHub App installation for
  people who prefer hosted reviews. It would manage execution, credentials, and
  review history; repository access, isolation between users, source retention,
  and publication controls need a design before implementation.
- **Automation:** webhook-triggered reviews, queued jobs, and workers.
- **Official agent catalog:** an officially maintained list of agents with setup
  instructions, protocol compatibility, and clear maintainer/support information.
- **Agent management:** easier package installation and selection from the catalog.
- **CLI distribution:** versioned binaries and installation on `PATH`, so users
  run `giad <command>` without a repository checkout or `./bin/giad` path.
- **Broader reviews:** base/head test comparisons, explicit issue selection, and
  deleted-line/multiline comments.
- **Parallel execution:** concurrent reviews with shared model/resource limits.
- **Review evaluation:** repeatable fixtures for real defects, false positives,
  and coverage gaps in example agents.

## Reference and development

- [Examples](example/README.md): runnable agents and optional Go tests.
- [Configuration](docs/configuration.md): trusted policy, models, tests, authentication.
- [Agent protocol](docs/agent-protocol.md): the public `giad/v1` contract.
- [Architecture](docs/architecture.md): boundaries and supported scope.
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
