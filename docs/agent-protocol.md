# Agent protocol

For agent authors. Wire types live in [protocol.go](../pkg/protocol/protocol.go)
and [model.go](../pkg/protocol/model.go); use those as the schema reference.

## Launch and framing

Use [the manifest example](../example/agent.manifest.json) with
`apiVersion: "giad/v1"`, a name/version, an absolute executable path,
capability declarations, and logical model profiles. The runtime passes arguments
directly and launches in an empty directory with a minimal environment. This is
currently a trusted process, not an OS sandbox.

Communication uses newline-delimited UTF-8 JSON over stdin/stdout, one request
outstanding at a time. This is a custom framed protocol, not JSON-RPC. Reserve
stdout for frames; the current runtime discards agent stderr.

The host starts with a `review.start` frame whose `params` is a `Job`: repository,
PR metadata, base/head SHAs, changed files, linked issues, scoped trusted instructions,
granted capabilities, and model profile names. `issuesError` marks unavailable issue
context. Instructions/issues are included only when granted; existing AGENTS.md
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
| `review.finish` | `Report` | `{"accepted":true}` |

Capability calls must be declared and granted. `review.finish` is always available.
Denied/failed/truncated observations produce coverage limitations. Invalid framing,
versions or duplicate IDs abort the session.

Agents own prompts and conversations. `model.chat` accepts only declared host-mapped
profiles; proposed model tool calls return as data. Request host capabilities separately
to execute them. The runtime retains/unloads models; no endpoint or lifecycle command
is needed in the agent.

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

After acceptance the host terminates and reaps the direct child. EOF, timeout,
exhausted budgets, or no valid completion fails the review.

Limits: 1 MiB frames; 256 KiB initial job; 64 requests/4 MiB incoming data;
16 model calls/96 KiB per model request; 64 KiB reports with at most 20 findings
and 8 KiB text fields. Tool/provider limits also apply. The command deadline defaults
to 30 minutes; model cleanup gets a separate 30 seconds after cancellation.

Old `agentic-review/v1` frames/manifests are incompatible and explicitly rejected.
`make smoke` launches the real [diff-inspector](../example/diff-inspector/main.go)
through the host broker without GitHub or Ollama.
