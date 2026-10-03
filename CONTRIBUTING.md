# Contributing to GIAD

GIAD provides the runtime and public agent protocol. Specialist reviewers belong
in separate agent packages; examples here should stay small and teach the protocol.
See the [roadmap](README.md#planned-features-and-checks) for planned work.

## Development

Install the Go version in [go.mod](go.mod), Git, and Node.js for Markdown linting.
Docker is required for container checks. GitHub authentication and Ollama are
needed only for the workflows that use them.

```sh
make all
make check
make smoke
make lint
```

Run `gofmt` on changed Go files. For changes to container execution, test profiles,
or packaged examples, also run:

```sh
make images
make sandbox-smoke
```

Use focused tests while developing, then run the relevant checks before opening a
PR. Documentation-only changes need Markdown linting and a check that links and
commands are accurate. CI also checks formatting, builds, and Docker isolation.
Report checks that could not run and why.

## Changes and pull requests

- Keep changes focused. Describe the problem, resulting behavior, and validation.
- Add regression tests for behavioral changes, especially protocol validation,
  sandbox boundaries, authentication, and publication retries.
- Discuss public protocol changes before implementation; update the schema,
  protocol reference, and affected examples together.
- Keep credentials in the host and agent capabilities explicitly granted. Reviews
  must remain drafts until publication is separately confirmed.
- Update public docs when behavior changes. They must stand on their own without
  private design documents.
- Keep keys, tokens, local configuration, and generated drafts out of commits.
  Use example configuration and sanitized fixtures in bug reports.

For an issue, include the GIAD version, relevant command, expected/actual behavior,
and sanitized diagnostics. For review-quality problems, distinguish an agent's
incorrect finding from a runtime failure.
