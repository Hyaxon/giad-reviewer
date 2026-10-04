# Repository instructions

GIAD is a general pull-request review runtime. Keep review judgment in agent
packages; do not add MAGI-specific behavior to the host. Start with [README.md](README.md),
[CONTRIBUTING.md](CONTRIBUTING.md), and the relevant public reference in `docs/`.

## Code map

- `cmd/giad`: CLI commands and explicit configuration/authentication selection.
- `pkg/protocol`: public `giad/v1` frames, manifests, jobs, reports, and tool types.
- `internal/agents`, `internal/review`, `internal/tools`: sessions, orchestration,
  and capability broker operations.
- `internal/sandbox`: isolated agent and approved test execution.
- `internal/githubauth`, `internal/githubapp`, `internal/githubapi`: host credentials
  and GitHub access.
- `internal/publication`: draft validation, preview, confirmation, and retry handling.
- `example/`: small runnable agents and trusted test images.

## Boundaries to preserve

- CLI agents and tests run in isolated containers. Do not add host-execution
  fallback, network access, credential mounts, or a Docker socket mount.
- Credentials, model endpoints, and trusted policy stay in the host. Required
  capabilities must be declared, granted, and implemented.
- Repository instructions come from the pinned base revision. PR text, issues,
  head code/guidance, model responses, and test output are untrusted evidence.
- A failed or incomplete session must not become a successful empty review.
- Publication stays separate from review execution, with confirmation bound to
  the action/content and revision checks. Preserve retry reconciliation and the
  guard against resending an uncertain publication.
- Protocol changes must update `pkg/protocol`, documentation, and examples together.
  Do not silently reinterpret incompatible frames.

## Working practices and checks

Preserve unrelated work and local settings. Never include secrets in code, logs,
fixtures, or documentation. Private PDFs are not public documentation: do not link
to them or require contributors to have them. Keep public docs concise and complete.
Avoid unused scaffolding for roadmap features.

Format changed Go files with `gofmt`. For code changes, run `make check` and
`make all smoke`. For sandbox, test-runner, or packaged example changes, also run
`make images` and `make sandbox-smoke`. For Markdown changes, run `make lint` and
check relative links. Add regression tests for changed behavior, not tests that
merely duplicate implementation. State what changed, which checks ran, and any
remaining limitations.
