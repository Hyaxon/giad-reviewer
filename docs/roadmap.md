# Implementation roadmap

This roadmap combines the PDF's setup/build sequences with subsequent decisions.
It describes future work, not functionality implemented by this documentation task.

## 0. Scaffold and preparation

The Go module, Cobra help/version CLI, package placeholders, and example settings
exist. Models are downloaded; inference smoke tests remain. The owner reports a
shared App installed across personal repositories and a local key; API validation remains.

## 1. Configuration loading

Build typed settings and deliberate lookup/override rules. Support explicit user/App auth selection, optional App identity,
per-role models, endpoints, and review policy. Keep repositories as invocation inputs.

Acceptance: parse representative settings; reject missing/invalid values and
unsupported publication events; handle `~/` paths; report field-specific errors
without leaking credentials. Add focused parsing/validation tests.

## 2. Authentication and PR acquisition

Personal mode is the first implementation path. Its provider and read-only
`magi auth status` check exist. Next, use that provider to fetch PR context. Defer
App installation-token exchange and the App setup branch until the local personal
review flow works. Existing App helpers remain available for that later work.

Implement App JWTs, installation tokens, cancellation, and HTTP errors. Resolve
PR URLs, explicit repositories, and local remotes. Fetch title/body, base/head SHAs,
changed files, and diff through the App identity.

Acceptance: read a real accessible PR without posting; test expiration/errors;
reject conflicting or ambiguous targets; do not assume a `main` base or depend on
a personal `gh` token in App mode. Add a separate user provider for explicit
personal mode, reusing the same GitHub client and review pipeline. Choose a test PR when ready for live verification.

## 2a. Interactive onboarding

Build the planned [`magi setup` flow](cli.md#planned-interactive-setup) on top of
configuration and authentication. Walk users through App installation, private-key
storage, read-only access checks, models, and branch-protection guidance. Share
diagnostics with the future `magi doctor` command.

Acceptance: a new user can configure MAGI without a local checkout or fixed target
repository; terminal-only users receive usable URLs/instructions; secrets are never
echoed; saving and overwriting are deliberate; cancellation preserves existing
settings. Report skipped/failed checks, leave repository rules unchanged, and offer
a noninteractive configuration/validation path for containers. Model checks can be
extended as the provider integration lands.

## 3. Linked issues and isolated checkout

Fetch formal links with provenance, pagination, and explicit access failures.
Prepare a disposable checkout bound to the acquired revisions.

Acceptance: distinguish missing links from failed retrieval; support invocation
outside a checkout; preserve the user's working tree; clean up on cancellation.

## 4. Read-only tools

Add bounded file/search/diff/history operations. Defer execution until its trust
boundary exists.

Acceptance: reject traversal and symlink escapes; enforce sizes/counts/deadlines;
report truncation; never execute model-generated shell text.

## 5. MELCHIOR model/tool loop

Implement the provider interface, Ollama adapter, prompts, structured observations,
session budgets, and explicit model lifecycle.

Acceptance: investigate a PR through multiple bounded tool calls; handle malformed
requests and cancellation; unload after the session; measure time and memory.

## 6. Findings and preview

Parse findings, run a disproof pass, validate evidence/coordinates, and render the
proposed feedback locally.

Acceptance: reject unsupported/style-only findings and invalid inline anchors;
support summary-only findings; distinguish incomplete sessions from clean reviews;
show repository, PR, revision, role, and publication intent.

## 7. First COMMENT publication

Publish MELCHIOR feedback through the shared App after confirmation. Bind to the
reviewed head and reconcile partial or uncertain API outcomes.

Acceptance: a disposable PR receives a correctly attributed COMMENT review and
valid thread; no approve, request-changes, merge, push, or auto-resolve paths exist.
Test conversation resolution separately through owner-configured repository rules.

## 8. BALTHASAR and CASPER

Add prompts and per-role configuration using the same identity. Implement per-issue
requirement extraction and satisfied/unsatisfied/unclear assessments.

Acceptance: sequential sessions with explicit unloading; role-labeled reviews;
evidence-backed coverage; no inferred or inaccessible requirements promoted into
authoritative defects; no consensus dependency.

## 9. Quality and execution hardening

Benchmark historical PRs and tune prompts/thresholds. Add a disposable test sandbox
when execution is needed. Consider deduplication only after observing noise.

Acceptance: measure false positives, misses, runtime, tool reliability, and memory;
tests cannot access credentials; publication retries do not duplicate reviews.

## 10. Services and multiple devices

Add worker mode, durable jobs, scheduling, and authenticated triggers after the
manual workflow is useful. Preserve the runner and provider boundaries.

Acceptance: jobs recover from relevant failures; shutdown is safe; publication is
revision-bound and idempotent; mixed workers run jobs without rewriting review logic.

Resolve configuration precedence, remote inference details, severity values,
budgets, sandbox technology, stale-head behavior, and retention at their respective
milestones. See the [decision log](decisions.md).
