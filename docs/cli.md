# CLI and repository selection

## Available today

```sh
go run ./cmd/magi --help
go run ./cmd/magi --version
```

The compiled `bin/magi` supports the same flags. All commands below are planned;
they currently do not perform reviews.

## Accepted invocation design

```sh
# Inside a local Git repository: infer the GitHub repository from its remote.
magi review 42

# From any directory: select the repository explicitly.
magi review 42 --repo hyaxon/some-project

# From any directory: use a full PR URL.
magi review https://github.com/hyaxon/some-project/pull/42
```

Explicit input takes precedence over remote inference. No permanent target
repository belongs in the global configuration. The App installation still needs
access to the chosen repository; credentials do not confer access to arbitrary
repositories outside the installation.

## Proposed resolution rules

These details need implementation and tests:

1. Parse and validate an explicit PR URL or `--repo owner/name`.
2. If both are supplied and disagree, return an actionable error.
3. Otherwise inspect remotes in the current Git working directory, including when
   invoked from a subdirectory.
4. Prefer a valid GitHub `origin`; if no `origin` exists and there is exactly one
   distinct GitHub repository among the remotes, use it.
5. If the choice is ambiguous or no suitable remote exists, request `--repo`.

Recognize standard HTTPS and SSH remote forms, with or without `.git`. Forks can
have both `origin` and `upstream`; show the resolved target in the preview and allow
explicit selection. GitHub Enterprise hosts and SSH host aliases need an explicit
support decision rather than accidental parsing behavior.

Read local Git configuration without executing repository-provided scripts. When
given an explicit remote repository, acquisition must work without an existing
local checkout and must not disturb the user's current working tree.

## Planned review controls

| Example | Intended behavior |
| --- | --- |
| `magi review 42 --agent melchior` | Run one reviewer |
| `magi review 42 --agents melchior,casper` | Run a selected sequence |
| `magi review 42 --preview` | Show results without publishing |
| `magi review 42 --publish` | Explicitly opt into noninteractive publication |
| `magi review 42 --model-all devstral-small-2` | Override all selected models |
| `magi review 42 --model melchior=qwen3.6:27b` | Override one role's model |
| `magi review 42 --requirements` | Enable requirement analysis |
| `magi review 42 --require-linked-issue` | Require formal issue context |

Flag conflicts, exit codes, machine-readable output, and noninteractive defaults
are open interface details. Noninteractive execution must not accidentally treat
the absence of an answer as publication approval.

## Future commands

- `magi doctor`: validate configuration, tools, App access, and model availability.
- `magi benchmark`: compare model/prompt assignments on known PRs.
- `magi worker`: execute leased reviews.
- `magi coordinator`: schedule jobs and maintain durable state.
- `magi webhook`: receive and authenticate GitHub event deliveries.

These are design targets, not advertised working commands.
