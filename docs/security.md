# Controlled tools and security boundaries

Status: design requirements. The scaffold does not implement these protections yet.

## Controlled tool surface

| Tool | Intended purpose |
| --- | --- |
| `get_pr_context()` | PR metadata, revisions, changed-file summary, linked context |
| `get_diff()` | Full or targeted bounded patch context |
| `read_file(path)` | Size-limited repository file read |
| `read_lines(path, start, end)` | Focused source range |
| `search_repo(query)` | Bounded text/symbol search, preferably ripgrep-backed |
| `find_files(pattern)` | Discover related sources and tests |
| `git_log(path)` | Relevant history |
| `git_blame(path, line)` | Optional history when intent is unclear |
| `run_tests(target)` | A configured, allowlisted test operation |
| `get_linked_issues()` | Formal linked issue context and retrieval status |

These names describe the tool protocol to build. They are not implemented functions.

## Filesystem and process boundaries

Confine file operations to the disposable checkout, including symlink resolution
and path traversal handling. Limit individual reads, result counts, total output,
tool calls, and overall session duration. Report truncation rather than silently
making incomplete output look complete.

Translate test targets to predefined executable/argument lists. Never pass model
text to a shell. An allowlisted `go test` or `npm test` still executes PR-controlled
code, so command allowlisting alone is not a sandbox.

Do not enable unattended execution of untrusted PR code until the execution
boundary is in place. A first read-only review can omit test execution and report
that tests were not run. Running tests must remain distinct from reading tests.

## Prompt and credential boundaries

Code, READMEs, issue bodies, and PR descriptions are untrusted data. They cannot
change policy, request secret files, choose arbitrary commands, or override publisher
validation. Model prompts alone are not sufficient enforcement; tool and API layers
must impose the restrictions.

GitHub private keys stay with trusted authentication code. Short-lived installation
tokens belong only in the GitHub/acquisition path where needed. Avoid placing tokens
in clone URLs, persisted Git remotes, command-line arguments, or logs. The model
and test environment must never receive App keys or installation tokens.

User-level configuration is trusted input; PR-local configuration is not automatically
trusted. Agent/tool traces can contain private source code, so define retention and
redaction deliberately. Remote inference requires an explicit endpoint decision.

## Future execution sandbox

- Use disposable storage and an unprivileged user.
- Exclude App credentials, developer dotfiles, SSH agents, and unrelated host paths.
- Apply CPU, memory, PID, disk, and wall-clock limits.
- Default to no network; dependencies need a deliberate cache/egress policy.
- Do not mount a Docker socket or use privileged containers.
- Drop capabilities and apply no-new-privileges where supported.
- Destroy writable state between unrelated jobs.

Containers reduce exposure but are not proof against hostile code. Stronger VM
isolation may be needed for adversarial public contributions. Keep that evolution
separate from the model-provider interface.

## Publication and failure safety

Validate coordinates and reviewed SHAs before creating threads. Reject unsupported
review events in code. Keep humans responsible for resolution. Track publication IDs
and reconcile retries to avoid duplicates. Cancellation must trigger cleanup even
when the model, a tool, or GitHub fails midway through the workflow.
