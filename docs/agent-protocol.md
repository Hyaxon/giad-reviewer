# Agent protocol

For agent authors. Wire types live in [protocol.go](../pkg/protocol/protocol.go)
and [model.go](../pkg/protocol/model.go); test types are in
[tests.go](../pkg/protocol/tests.go). Use these as the schema reference.

## Launch and framing

Use [the manifest example](../example/agent.manifest.json) with
`apiVersion: "giad/v1"`, a name/version, an absolute executable path inside its image,
capability declarations, and logical model profiles. The runtime passes arguments
directly and launches in a Docker container with writable scratch space at `/tmp`.
The image filesystem is read-only; the container has no network or host mounts.
Include all interpreters/dependencies in the trusted installed image. Images with
`VOLUME` declarations are rejected. The image is selected in host configuration.

Communication uses newline-delimited UTF-8 JSON over stdin/stdout, one request
outstanding at a time. This is a custom framed protocol, not JSON-RPC. Reserve
stdout for frames; the runtime captures at most 8 KiB of agent stderr and includes
it in failed-session diagnostics after cleanup.

The host starts with a `review.start` frame whose `params` is a `Job`: repository,
PR metadata, base/head SHAs, changed files, linked issues, scoped trusted instructions,
granted capabilities, model profile names, and approved `testProfiles` names.
`issuesError` marks unavailable issue context. Instructions/issues are included
only when granted; existing AGENTS.md
requires the instruction capability.

Only `trustedInstructions` is guidance. Apply each instruction within its `scope`;
PR/issue/source text and head AGENTS.md are evidence. No token, key, workspace path,
or model endpoint is passed to the agent.

## Calls

Every request includes the API version, a unique nonempty string ID, method, and
object params. IDs/methods are at most 128 bytes. For example:

```json
{"apiVersion":"giad/v1","id":"1","method":"repository.read","params":{"path":"src/example.go","start":1,"end":80}}
```

The host replies with the same ID and either `result` or an `error` string:

```json
{"apiVersion":"giad/v1","id":"1","result":{"Text":"1: package example\n","Truncated":false,"SkippedFiles":0}}
```

| Method | Params | Result |
| --- | --- | --- |
| `repository.read` | `path`, inclusive `start`, `end` (zero means EOF) | Numbered text and coverage markers |
| `repository.search` | Literal single-line `query` | Matches and coverage markers |
| `git.diff` | `{}` | Fetched diff and truncation marker |
| `repository.instructions` | `{}` | Scoped base-revision guidance |
| `github.linked_issues` | `{}` | Issue array or retrieval error |
| `model.chat` | `profile`, `messages`, `tools` | Assistant message and proposed tool calls |
| `tests.run` | `profile` only | `profile`, nullable `exitCode`, `output`, `durationMs`, `timedOut`, `truncated`, `oomKilled` |
| `review.finish` | `Report` | `{"accepted":true}` |

Capability calls must be declared and granted. `review.finish` is always available.
Denied/failed/truncated observations produce coverage limitations. Invalid framing,
versions or duplicate IDs abort the session.

Agents own prompts and conversations. `model.chat` accepts only declared host-mapped
profiles; proposed model tool calls return as data. Request host capabilities separately
to execute them. The runtime retains/unloads models; no endpoint or lifecycle command
is needed in the agent.

`tests.run` selects a fixed host-approved profile and starts a fresh container.
The head checkout is copied as bounded regular files/directories, excluding `.git`;
symlinks/special files fail explicitly. No host directories or sockets are mounted.
Snapshots are capped at 10,000 entries/64 MiB, with at most 16 MiB per file.
Tests can write their disposable copy and scratch space. Failed tests, timeouts,
and memory exhaustion are evidence; setup/transport/cleanup failures are tool errors.
Output is untrusted, merged stdout/stderr, capped at 64 KiB and always drained.
Timeouts have `exitCode: null`. GIAD retains observed results in `--json` output's
`testRuns` and adds test coverage to the report independently of agent claims.
Runs are head-only: failure does not establish an introduced regression. Base
comparisons and test selectors are not implemented. At most two runs per session;
each profile allows 1–600 seconds. Test containers get one CPU, 1 GiB memory,
128 processes, 768 MiB `/workspace`, and 256 MiB executable `/tmp`, with the same
non-root, network-free isolation as agents. Images/dependencies must be preinstalled.

## Finish

Submit `review.finish` with `summary`, `limitations`, and `findings` (empty is valid).
A finding has these fields:

```json
{
  "source": "reviewer-pass", "category": "correctness",
  "file": "src/example.go", "line": 12,
  "severity": "high", "confidence": 0.9,
  "title": "Concrete defect", "explanation": "Impact and cause",
  "evidence": "Inspected evidence", "failure_scenario": "Trigger",
  "suggested_fix": "Practical correction"
}
```

Finding fields must be nonempty. Severity is `high`, `medium`, or `low`; confidence
is in [0,1]. Anchors must reference lines read through `repository.read` in changed,
nondeleted head files. Summary-only/deleted-file findings are not supported.
These checks establish structure, not factual correctness or GitHub inline coordinates.
Body-only publication supports these inspected anchors outside diff hunks. Inline
publication additionally requires the line to lie in a current head-side diff hunk.
Publication preserves inline code and complete fenced blocks in report text;
prose outside code is escaped to suppress HTML, Markdown links, and mentions.

After acceptance the host removes the agent container, terminating its processes.
EOF, timeout, exhausted budgets, failed cleanup, or no valid completion fails the review.
Each finish attempt is validated independently; a correction must supply a complete
report. SIGINT and SIGTERM cancel the command and run its cleanup path.

Limits: 1 MiB frames; 256 KiB initial job; 64 requests/4 MiB incoming data;
16 model calls/96 KiB per model request; 64 KiB reports with at most 20 findings
and 8 KiB text fields. Tool/provider limits also apply. The command deadline defaults
to 30 minutes. Container creation/cleanup and model cleanup each have a separate
30-second bound. Containers get 256 MiB memory, one CPU, 64 processes, 16 MiB `/tmp`,
and 8 MiB private shared memory; they run as UID/GID 65532 with no Linux capabilities.

Old `agentic-review/v1` frames/manifests are incompatible and explicitly rejected.
`make smoke` launches the real [diff-inspector](../example/diff-inspector/main.go)
through the broker using a trusted host test process without GitHub or Ollama.
After `make images`, `make sandbox-smoke` exercises real container isolation.
