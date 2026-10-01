# Decision log

## D01: Go and a reusable runner

Accepted from the PDF. Native CLI first; later workers call the same review engine.
Inference dominates runtime cost. Keep CLI, providers, and deployment out of core
review logic.

## D02: One GitHub App, three reviewer roles

Accepted in discussion. Supersedes the PDF's three-App identities, keys, and
per-reviewer installation blocks throughout. One MAGI App publishes role-labeled
reviews. Prompts, model choices, and verification remain independent. Shared
permissions/repositories make separate credentials unnecessary for this runtime.

## D03: Repository selected per invocation

Accepted in discussion. MAGI is general purpose. Use an explicit PR URL, `--repo`,
or the current directory's Git remote. Explicit input wins; unavailable/ambiguous
inference requires explicit selection. Detailed remote rules remain proposed.

## D04: Personal account and all repositories

Owner-reported setup. One App is installed on all personal repositories; no
organization is needed. This supersedes the PDF's selected-test-repository setup
recommendation for this owner. Other users may restrict installations. Live API
verification is still pending.

## D05: COMMENT-only and human authority

Accepted from the PDF. No approvals, requests for changes, merging, pushing, or
automatic resolution. Code must enforce this because GitHub permissions are broader.
Human approvals and conversation-resolution rules remain independent.

## D06: Sequential local model sessions

Accepted for the 32 GB M1 target. Keep the active model loaded through its tool loop,
then unload. Per-role assignments are experiments. Parallel devices are future work.

## D07: Formal links establish requirement authority

Accepted from the PDF. Informal references are candidates, not automatic specifications.
Preserve evidence and distinguish explicit/stated/inferred sources. Use `unclear`
when behavior cannot be established from available context.

## D08: Controlled tools and isolated execution

Accepted from the PDF. Bounded operations replace arbitrary shell access. PR code
must not inherit credentials. Containers are an option for isolation/deployment,
not a mandatory model-runtime abstraction.

## D09: Documentation before implementation

The implementation request was interrupted before code changes. The subsequent
instruction asks for documentation instead. This task changes prose only; roadmap
commands and configuration proposals do not imply implemented behavior.

## Open decisions

| Topic | Starting point or unresolved detail |
| --- | --- |
| Config lookup | Proposed `~/.config/magi/config.toml` and explicit `--config` |
| Environment overrides | Names and precedence pending |
| Repository-local policy | Do not automatically trust PR-controlled configuration |
| Remotes | Proposed GitHub `origin`, then a sole unambiguous GitHub remote |
| Providers | Explicit model profiles later; avoid ambiguous tag/profile references |
| Findings | Severity values, diff sides/ranges, and summary-only representation |
| Budgets | Around 32K initial context; tool/time/output bounds pending |
| Stale PR head | Detect before publication; exact recovery UX pending |
| Publication recovery | Reconcile IDs and uncertain requests; persistence design pending |
| Sandbox | Choose before enabling untrusted execution |
| Permissions | Expand only with features requiring them |
| GitHub Enterprise | Decide host support before accepting arbitrary URLs |
| Retention | Source-content logging, redaction, and cleanup policy |

Update this log and affected guides together when a proposal becomes an accepted
decision, so outdated examples do not compete as sources of truth.
