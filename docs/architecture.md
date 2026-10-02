# Runtime design

Agentic Review owns execution and access. Separate agents own review judgment.
A composite agent such as MAGI is one installed package; its internal reviewer
names and prompts never belong in this runtime.

A review follows this sequence:

```text
GitHub PR → verified checkout + base instructions → agent process
                                                    ↕
                                            repository/model broker
                                                    ↓
                                              draft preview
```

## Code layout

| Location | Responsibility |
| --- | --- |
| `cmd/agentic-review` | CLI and output |
| `internal/review`, `internal/agents` | Orchestration, process launch, capability checks, draft validation |
| `internal/github*`, `internal/repo`, `internal/instructions` | Authentication, PR context, disposable checkouts, base-revision AGENTS.md |
| `internal/tools`, `internal/model` | Bounded repository access and Ollama profiles/lifecycle |
| `pkg/protocol` | Shared agent wire contract |

The runtime verifies exact base/head commits and removes temporary resources after
use. Agents receive normalized PR/issue data and scoped `AGENTS.md` guidance from
the **base revision only**. Head edits and other repository text remain evidence.

Agents request only declared and granted capabilities. GitHub credentials stay in
the host; model endpoints/tags come from host configuration. Models remain loaded
between calls, unload before profile switches, and unload on session cleanup.

The broker offers no shell, tests, writes, web, browser, or publication operation.
Agent binaries still run as the local user: process separation is not OS isolation.
Draft checks validate structure and inspected line anchors, not whether a finding
is true. An aborted session fails rather than reporting a clean review.

## Next steps

1. Validate the contract with an independently installed agent.
2. Harden agent isolation and process cleanup.
3. Add human-confirmed, revision-bound COMMENT publication, then App auth/setup
   and sandboxed test execution. Workers can wait.

## Migration

The module and CLI are now `agentic-review`; the folder/remote rename is still pending.
The former role/model flags were replaced by manifests and config. Original MAGI
source is saved in [migration/magi](../migration/magi/README.md), with historical
docs in a single ZIP alongside it. This design follows the local
`Agentic_Review_MAGI_Architecture_and_Setup.pdf`.
