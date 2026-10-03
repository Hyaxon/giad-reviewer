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
| `pkg/protocol` | Public `giad/v1` wire types |
| `example/diff-inspector` | Independent, model-free protocol example |

The runtime verifies exact base/head commits and cleans up disposable resources.
Scoped AGENTS.md guidance comes from the base revision; head edits and other
repository/issue text remain evidence. Agents request only declared/granted
capabilities. GitHub credentials stay in the host; model endpoints/tags come from
host configuration. Models unload before profile switches and at session cleanup.

Reviews are serial and local-only. `--preview` is explicit but optional; disabling
it is rejected. Structural finding validation is not proof of correctness or valid
GitHub inline coordinates. Failed sessions do not become clean reviews.

## Next milestones

The GIAD naming migration and real diff-inspector integration are in place. Next:

1. Enforce isolation for every agent and terminate its full process tree. Current
   agents run as the local user and must be trusted; no OS sandbox exists yet.
2. Run an optional specialist through the same public contract.
3. Add separately confirmed COMMENT-only publication with revision/diff validation
   and retry-safe outcomes.

Test execution will use a separate sandbox through approved `tests.run` profiles;
agent-requested runs are primary, baseline runs optional. Tests/builds/install hooks
must never fall back to host execution. App tokens/setup and richer issue selection
follow as needed; model-resource leases must precede parallel jobs. Workers can wait.

## Compatibility

The module/CLI/config names are GIAD; the wire prefix is `giad/v1`. Old
`agentic-review/v1` manifests/frames are rejected with no silent alias. Retain
`github.linked_issues` as the capability name. The local folder and GitHub remote
still use the historical repository name until an administrative rename occurs.
