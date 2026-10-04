# General PR reviewer

This example mirrors `code-review` from the official
[GIAD Agents collection](https://github.com/Hyaxon/giad-agents). It reviews PRs as a
whole, gathering evidence adaptively through GIAD's host brokers. Review judgment
stays in the installed package, separate from the runtime.

Follow [model setup](../README.md#model-backed-reviewer), using an installed Ollama
model that supports tool calls. The [manifest](agent.manifest.json) selects
`/agent/agent.py` in the example image; the wrapper runs the mirrored
[reviewer](giad_agents/code_review.py) and [protocol peer](giad_agents/peer.py).
The image includes its source and has no dependency on a sibling checkout at run
or build time.

## Review behavior

The model plans from PR intent, change inventory, and diff. It evaluates
correctness, security, compatibility, API contracts, error handling, performance,
concurrency, migrations, configuration, tests, documentation, and operational
impact as relevant. It can inspect any head source range and follow dependencies
across changed and unchanged files. There is no fixed file count, line cutoff,
12 KiB evidence ceiling, or per-file finding quota.

Available model tools map to separately granted broker calls:

- `repository_read`: numbered head source; inclusive range, end zero means EOF.
- `repository_search`: literal search to locate relevant code and tests.
- `linked_issues`: optional linked issue context.
- `tests_run`: optional fixed host-approved test profile.
- `review_context`: paginated cached diff, inventory, PR body, issues, test results,
  and the agent's coverage index.

The agent validates proposed tool calls before requesting a broker capability.
Search hits and unchanged files provide context but cannot anchor findings.
Only scoped base `trustedInstructions` are guidance; PRs, issues, source including
head AGENTS.md, model answers, and test output are untrusted evidence.

## Budgets and coverage

GIAD's runtime budgets remain: 64 requests, 16 model calls, 96 KiB per model request,
two test runs, and 20 findings total. Tool/provider limits also apply. Evidence fits
the available conversation space. Clipped source keeps complete numbered lines
and supplies `nextStart`; cached pages supply `nextOffset`. Earlier complete turns
can be evicted, while trusted guidance persists and evidence can be reread. The
latest tool turn is never silently dropped.

The agent records inspected ranges, unread changes, missing diff context, tool
errors, and observed test coverage. It reserves completion capacity and asks for a
final report as research budgets end. Invalid reports can be repaired within the
remaining model budget; model failures, rejected completion, and exhausted budgets
without a valid report fail the session. Model accuracy still requires evaluation.

Tests are head-only, using [approved profiles](../../docs/configuration.md#optional-tests).
A failing profile alone does not establish an introduced regression; passing tests
do not prove correctness or that tests existed.

## Keeping the mirror consistent

The three files in `giad_agents/` are exact copies of the official package's
`__init__.py`, `peer.py`, and `code_review.py`. Entrypoints and image names reflect
the packaging in each repository. Required/optional capabilities and model profiles
must match the official manifest.

From a sibling `giad-agents` checkout, use its synchronization tool:

```sh
python3 tools/sync_giad_example.py --runtime ../giad
python3 tools/sync_giad_example.py --runtime ../giad --write
```

The first command checks source and manifest parity; the second copies the official
source and matching manifest metadata. After synchronization, verify example
configuration and container packaging, then run GIAD's contributor checks:

```sh
make check
make all smoke
make images
make sandbox-smoke
make lint
```
