# Runtime design

GIAD owns execution and access. Separate agents own review judgment. A composite
agent such as MAGI is one installed package, with its own reviewer names and prompts.

```text
GitHub PR -> verified checkout + base instructions -> agent process
                                                     |
                                            repository/model broker
                                                     |
                                               local draft
```

| Location | Responsibility |
| --- | --- |
| `cmd/giad` | CLI and output |
| `internal/review`, `internal/agents` | Orchestration, process launch, capability checks, draft validation |
| `internal/github*`, `internal/repo`, `internal/instructions` | Auth, PR context, disposable checkouts, base-revision AGENTS.md |
| `internal/tools`, `internal/model` | Bounded repository access and optional model profiles/lifecycle |
| `internal/sandbox` | Separate Docker agent/test isolation and a trusted host backend used only by offline tests |
| `pkg/protocol` | Public `giad/v1` wire types |
| `example/diff-inspector` | Independent, model-free protocol example |

The runtime verifies exact base/head commits and cleans up disposable resources.
Scoped AGENTS.md guidance comes from the base revision; head edits and other
repository/issue text remain evidence. Agents request only declared/granted
capabilities. GitHub credentials stay in the host; model endpoints/tags come from
host configuration. Models unload before profile switches and at session cleanup.

Reviews are serial and return local drafts. `--preview` is explicit but optional;
disabling it is rejected. A separate `publish` command previews and confirms a
saved draft's GitHub review. Structural finding validation is not proof of
correctness. Failed sessions do not become clean reviews.

## Next milestones

The GIAD naming migration and real diff-inspector integration are in place.

CLI reviews use `DockerLauncher`. Installed images are read-only, non-root, without
network or host mounts, with resource limits and bounded temporary space. Removing
the container terminates its processes. Launch failures never trigger a host fallback.
Docker and agent images must be available before review; no PR code is executed to
install/build an agent. The Docker engine and installed images are trusted infrastructure.

The optional `example/code-review` package now exercises source reads, scoped
guidance, host model calls, and anchored findings through the same public contract.
Integration checks use scripted judgments; model quality needs separate evaluation.

Approved `tests.run` profiles now execute in fresh containers with copied head
files, no Git metadata, and bounded scratch/output/time. The host retains actual
results independently of the agent report. Agent-requested runs are supported;
baseline runs and base comparisons remain future work. Test code never runs on
the host. Dependencies are prepared in trusted images before review.

Saved drafts support separately confirmed `COMMENT` or `REQUEST_CHANGES` publication. The command
rechecks base/head commits and finding anchors against the current diff, presents
the body, and requires its confirmation hash. `--inline` attaches findings to their
validated head-side lines; the preview displays every comment. The confirmation
hash binds the action and all comment paths, lines, sides, and bodies. Approval
and deleted-line/multiline anchors are not implemented.
Exact body/commit/author/action and inline-comment matching reconcile retries.
Dismissed request-changes reviews remain completed attempts. Durable exclusive attempt
records in the user's cache prevent blind resends after crashes or network errors;
unresolved attempts require checking GitHub manually. Explicit GitHub rejections
release the attempt record and report the refusal reason. GitHub has no atomic
base/head precondition for this write: the final revision check narrows the race,
and `commit_id` pins the review to the inspected head. Draft files are trusted,
user-editable artifacts; a changed body requires a new confirmation hash.

Next: GitHub App authentication/setup and richer issue selection. Model-resource
leases must precede parallel jobs. Workers can wait.

## Compatibility

The module/CLI/config names are GIAD; the wire prefix is `giad/v1`. Old
`agentic-review/v1` manifests/frames are rejected with no silent alias. Retain
`github.linked_issues` as the capability name. The local folder and GitHub remote
still use the historical repository name until an administrative rename occurs.
