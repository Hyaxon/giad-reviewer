# Getting started with GIAD V1

GIAD runs on your machine, reviews PRs on github.com, and saves local drafts.
Publication is a separate command that requires confirmation of the preview.

## Supported platforms

| Host | V1 status | Container engine |
| --- | --- | --- |
| Linux amd64 | Primary release target; Ubuntu is the configured CI platform | Docker Engine running Linux containers |
| macOS arm64 (Apple Silicon) | Development platform exercised with the Docker isolation checks | Docker Desktop running Linux containers |
| Linux arm64 or macOS amd64 (Intel) | Not covered by the current release validation | Linux containers matching the agent's architecture are required |
| Native Windows | Unsupported in V1 | WSL2 is not yet validated; use a supported Linux environment |

The CLI runs on the host; agents and approved tests always run in Linux containers.
Agent executables must match the container engine's CPU architecture. The bundled
Go agent build uses the build host's architecture; cross-architecture and remote
Docker configurations are not part of the validated setup.

## Prerequisites

Install these before building:

- Go matching [go.mod](../go.mod) (V1 uses Go 1.27.1), Git, and Make.
- Docker CLI and a running Linux container engine accessible to your user.
- GitHub CLI (`gh`) for personal authentication, or a configured GitHub App.
- Ollama and an already-downloaded model only if your agent uses `model.chat`.

The host needs access to github.com and api.github.com. Initial builds also need
access to Go module and container registries. Agent and test containers have no
network access; test dependencies must be prepared in their images before review.
Node.js is needed only for development Markdown checks.

Check the required tools and daemon:

```sh
go version
git --version
make --version
docker version
gh --version
```

Skip the `gh` check if you will use App authentication.

## Build from source

Once the release tag is available on GitHub, use a fresh directory:

```sh
git clone --branch v1.0.0 https://github.com/Hyaxon/giad.git
cd giad
make all
make images
./bin/giad --version
```

Expect `giad version 1.0.0`. V1 is distributed through the source repository;
`make all` builds the CLI and generates the Go example's manifest. `make images`
builds the trusted example agent and Go test images. These setup commands do not
review a PR or publish anything. Reviews never build or pull missing images.

Use `./bin/giad` from this directory, or add `"$PWD/bin"` to your shell's `PATH`.
Keep manifests and runtime configuration outside the PR checkout being reviewed.

## Authenticate and inspect a PR

For a personal account:

```sh
gh auth login --hostname github.com
./bin/giad auth status
./bin/giad pr view 42 --repo OWNER/REPO
```

Replace `OWNER/REPO` and `42` with a repository and PR you can access. Full GitHub
PR URLs also work. Personal mode uses GitHub CLI's effective token, including
`GH_TOKEN` overrides.

For App mode, follow [App authentication](configuration.md#authentication).
Add `--identity identity.toml` to every authentication, inspection, review, and
publication command. Keep the PEM key outside the repository. Explicit App mode
fails on invalid credentials and never falls back to your personal account.

## Save your first draft

The model-free diff-inspector checks the protocol and retrieves a diff. It does
not detect defects and requires no model:

```sh
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml --json > draft.json
```

Check that the command succeeded before using the saved draft. A failed command
can leave an empty redirected file. Inspect `report`, `job`, and `testRuns` in the
JSON. Progress is written to stderr; stdout contains the draft.

For defect analysis, install a compatible agent or follow the
[model-backed example](../example/README.md#model-backed-reviewer). Set its exact
installed model tag in your local configuration; example tags are not a guarantee
that the model exists on your machine. Configure
[approved tests](configuration.md#optional-tests) separately if needed.

## Preview and publish

Preview a body-only review, inspect the output, and repeat with the printed hash:

```sh
./bin/giad publish draft.json
./bin/giad publish draft.json --confirm HASH
```

The default event is `COMMENT`. For inline findings or a change request, add
`--inline` or `--event REQUEST_CHANGES` to both commands. Keep the same draft,
authentication, and options between preview and confirmation. Personal accounts
cannot request changes on their own PRs.

Inline findings require a current head-side diff line; omit `--inline` for other
inspected lines in changed head files. New writes require an open PR and matching
base/head revisions. If either revision changed, generate and inspect a new draft.

After an uncertain publication response, rerun the same confirmed command to
reconcile. Completed reviews can be recognized after a push or merge. If GIAD
reports an unknown outcome without a match, inspect GitHub manually and retain the
local attempt record; deleting it can permit a duplicate write.

## Common setup problems

| Symptom | Next step |
| --- | --- |
| Docker image inspection fails | Start Docker and build or explicitly pull the configured trusted image before review |
| Entrypoint cannot execute | Check the image contains the absolute manifest path and matches the engine's architecture |
| GitHub authentication or access fails | Run `auth status`; verify the effective account, repository access, or App installation permissions |
| Model is unavailable | Start Ollama and compare the configured model tag with `ollama list` |
| Tests cannot resolve dependencies | Prepare that project's dependencies in its trusted test image; runtime tests have no network |
| Draft revisions no longer match | Generate a fresh draft and preview it again before a new publication |

See [current limitations](../README.md#current-limitations),
[configuration](configuration.md), and the [agent contract](agent-protocol.md)
for supported behavior and exact limits. Report problems using the information in
[Contributing](../CONTRIBUTING.md).
