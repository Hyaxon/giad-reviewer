# Runtime roadmap

This document covers proposed GIAD runtime work. None of the features below is
available unless the current references explicitly document it. Scope, order,
command names, and capability names remain open; this is not a release commitment.

The [current workflow and limitations](../README.md#current-limitations),
[architecture](architecture.md), and [protocol](agent-protocol.md) describe the
implemented local-first runtime. The optional
[GIAD Agents collection](https://github.com/Hyaxon/giad-agents) already has four
starter packages and a source catalog. Its
[agent roadmap](https://github.com/Hyaxon/giad-agents/blob/main/docs/roadmap.md)
owns MAGI, Test Writer, Wacht/CVE review, and other specialist strategies.

## Review inputs and requirements

- **Working-tree reviews:** inspect uncommitted local changes without requiring
  a GitHub PR, with an explicit snapshot of the reviewed state.
- **Diff/range reviews:** inspect a commit such as `HEAD~1`, a branch diff, or an
  arbitrary git range. Define source anchors and trusted instruction revisions
  for these inputs before extending the PR-oriented job contract.
- **Issue selection:** extend existing formally linked issue retrieval with
  explicit requirements selection. Candidate options are
  `--issues auto|linked|require|none`, specific issue numbers, and `--add-issue`.
  Define each mode's handling of missing or unavailable issues and retain issue
  content as untrusted requirements evidence.
- **Test comparisons:** compare approved base and head test runs to distinguish
  introduced failures from existing failures. Generated test patches need a
  separate, explicitly approved path into a disposable test workspace before
  agents can validate proposed tests.
- **Broader job contexts:** make the runtime usable for software-engineering
  workflows beyond PR review. Specialist behavior remains in agent packages;
  job types, evidence access, and report validation belong here.

## Execution and job control

- **Serial or parallel execution:** run multiple agents in one job with shared
  model/resource limits and explicit model leases. Preserve model retention
  across each agent's tool loop and cleanup on completion or cancellation.
- **Execution controls:** candidate options include `--execution serial|parallel`,
  `--max-parallel`, overall and per-agent timeouts, and optional fail-fast behavior.
  These extend the current single-agent deadline and fixed session budgets.
- **Queueing:** detached jobs, status, cancellation, retries, priorities, and
  agent-specific reruns. Failed or superseded work must remain distinguishable
  from a completed review with zero findings.
- **Multiple devices:** a coordinator with Mac mini or Linux workers advertising
  available models and resources. Start with a lightweight queue/worker design
  suited to small deployments; Kubernetes should not be required. Define
  authenticated worker access, source handling, and resource ownership.
- **Model providers:** add providers behind the host's logical model profiles,
  preserving agent independence from endpoints and credentials. Ollama remains
  the current provider; other providers need compatible tool-call and lifecycle
  behavior or explicit limits.

## Automatic PR reviews

- **Optional daemon:** choose automatic review agents per repository, triggering
  events, and whether results may publish. Manual local review remains available.
- **Webhook and polling inputs:** handle GitHub events such as `opened`,
  `reopened`, `ready_for_review`, and `synchronize`; offer polling for local
  deployments without an inbound webhook endpoint.
- **Path/event filters:** run only relevant agents, such as Wacht when dependency
  manifests or lockfiles change.
- **Debouncing and superseding:** combine rapid pushes and cancel or supersede
  stale jobs before they can publish obsolete reviews.
- **Idempotent work:** identify jobs by repository, PR, head SHA, and agent;
  retain the pinned base revision and policy in the job record. Define how an
  intentional rerun differs from a duplicate event. Existing publication retry
  reconciliation does not provide this scheduler.
- **Analysis and publication policy:** automatic analysis can produce local
  drafts without posting. Any automatic publication needs an explicit trusted
  policy, revision checks, and retry handling that preserves the current manual
  preview/confirmation workflow.

## Publication and repository policy

- **Repository roles:** an explicit publication gate for invoking users with
  `write`, `maintain`, or `admin` access, while permitting read-only users to run
  local analysis where source access allows it. Define how a user's role is
  verified when a user-owned GitHub App supplies publication credentials.
  Current authentication/access checks are not this role policy.
- **Publication controls:** candidate `--publish per-agent|batch|none` selection,
  grouping by agent, severity, or file, and preview/dry-run modes for multiple
  agents. Build on the existing separate publication command and confirmed preview.
- **Anti-spam:** cross-agent deduplication, configurable comment budgets, and
  summary aggregation in addition to current report limits and retry protection.
  Zero findings must remain valid; never invent findings to fill a quota.
- **Additional anchors:** deleted-line and multiline comments with inspected
  evidence and current diff validation.
- **Human-controlled merge policy:** keep `COMMENT` as the normal publication
  event. Use human-managed unresolved conversations and branch protection for
  merge blocking; do not give agents approval or merge authority. The existing
  explicit `REQUEST_CHANGES` option remains a user-selected action.

## Brokered capabilities

- **Controlled web research:** web search, page retrieval, and integrations
  through explicitly granted, bounded host capabilities. Define provider/source
  controls and limits on repository data sent externally. External content
  remains untrusted; agent and test containers retain network isolation.
- **Curated vulnerability lookup:** consider a narrower
  `vulnerabilities.lookup` capability for current advisories and affected-version
  data, so dependency reviewers need not request unrestricted web access. Specify
  ecosystem/version matching, provenance, freshness, and unavailable-data behavior.
  Neither this capability nor `web.search` is implemented today.

Official agents receive no privileged capabilities. New tools must be part of
the public protocol and trusted permission system before any package uses them.
Repository code continues to execute only through approved disposable sandboxes
with no secrets, bounded resources, deadlines, and no container network access.

## Coding-agent integrations

- **MCP server:** a possible `giad mcp` command exposing review, run-agent,
  findings, and job status operations to external coding agents.
- **Host integrations:** thin skills/plugins for Codex, Claude Code, Cursor,
  Gemini CLI, OpenCode, and similar hosts, delegating execution and policy to GIAD.
- **Pre-PR workflow:** a coding agent implements a change, requests a local GIAD
  review, addresses findings, and reruns before opening a PR. This depends on
  local review inputs and must keep publication separately controlled.

## Distribution and evaluation

- **Agent installation:** catalog-assisted selection and a possible
  `giad agent install github.com/...` command, with version/protocol compatibility
  checks and explicit package trust. The existing source catalog is not an installer.
- **CLI distribution:** versioned binaries and installation on `PATH`, without
  requiring a source checkout or a `./bin/giad` path.
- **Optional centralized service:** user sign-in and GitHub App installation for
  hosted execution and review history. Design credential handling, tenant
  isolation, repository access, source retention, and publication policy before
  implementation. Local-first operation must remain independent of this service.
- **Review evaluation:** repeatable defect, false-positive, and coverage fixtures
  for the bundled examples. Agent-specific quality evaluation belongs alongside
  packages in the official collection or their third-party repositories.
