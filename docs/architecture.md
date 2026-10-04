# Runtime design

GIAD owns execution and access. Installed agents own prompts and review judgment.
The optional [official GIAD Agents collection](https://github.com/Hyaxon/giad-agents)
already provides starter packages and a source catalog. MAGI and other planned
specialists belong there or in third-party agent projects. Official packages use
the same public protocol and capability grants as any other installed agent.

```text
GitHub PR -> pinned checkout + base guidance -> sandboxed agent
                                                  |
                                        host capability broker
                                                  |
                                             local draft
                                                  |
                                       preview + confirmation
                                                  |
                                           GitHub review
```

## Boundaries

- **Host:** GitHub credentials, trusted configuration, checkout, repository reads,
  model transport/lifecycle, sandbox management, draft validation, publication.
- **Agent container:** installed review package and private temporary space;
  no network, host mounts, credentials, or container-engine socket.
- **Test container:** a copied head workspace and a fixed approved command;
  separate from the agent, with no network or host mounts.

All CLI agents use Docker, including model-free examples. Locally installed Linux
images are resolved to immutable IDs; images with `VOLUME` declarations are rejected.
Containers are non-root, have dropped privileges, and enforce resource/deadline
limits. Cleanup removes their processes. Launch failures never fall back to host
execution. The trusted host launcher is used only by offline tests.

Root and scoped AGENTS.md come from exact base Git objects, including previous
paths for renames. PR text, issues, source, head guidance, model answers, and test
output remain evidence. Required capabilities must be declared, granted, and
implemented. GitHub keys/tokens and model endpoints stay in the host.

## Current scope

Reviews run serially, one agent package per command. Models are optional and
currently use Ollama. GIAD retains a model through the session, unloads before
profile switches, and cleans it up afterward. Parallel use needs shared-resource
leases before it can be supported.

Tests run only when an agent requests an approved profile. GIAD records observed
results independently of the agent. Runs are head-only; profiles and dependencies
must already exist in trusted images. Failed sessions never become clean reviews.
Finding validation checks structure and inspected anchors, not factual correctness.

Publication is a separate, confirmed `COMMENT` or `REQUEST_CHANGES` review.
Optional inline findings use single head-side diff lines. Retries match the author,
action, commit, body, and comments; durable attempt records prevent blind resends.
Completed reviews can reconcile after the PR changes or closes; only new writes
require current revisions, an open PR, and valid inline anchors. Body-only findings
can refer to inspected lines elsewhere in a changed head file.
Previous comment formats remain recognizable after presentation changes.
A final revision check narrows the write race; GitHub's review `commit_id` pins the
inspected head but supplies no atomic base/head precondition.

Personal auth uses GitHub CLI. Explicit App mode uses repository-scoped installation
tokens with in-memory caching/refresh and the bot's durable user ID for retries.

## Planned work

See [current limitations](../README.md#current-limitations) and
the [runtime roadmap](roadmap.md) for supported
scope and future direction. The optional hosted service is a proposal;
the current implementation is the manual self-hosted workflow. Generated test
patches and approval are also outside the current scope.

The CLI/module/config names are GIAD and the wire version is `giad/v1`.
Old `agentic-review/v1` frames and manifests are rejected.
The V1 public contract is `giad/v1`; incompatible changes require a new wire version.
