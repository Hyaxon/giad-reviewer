# Goals and scope

Status: accepted design.

## The problem

A useful code review often needs more than a diff. It needs surrounding functions,
callers, tests, error paths, and the requirements that motivated the change.
Putting an entire repository into a model prompt is expensive and unreliable.

MAGI should let a model investigate through bounded repository tools, then
report a small number of concrete, evidence-backed findings. The model provides
reasoning; MAGI owns context retrieval, execution policy, validation, and publication.

## Product goals

1. Review GitHub PRs from any supported repository the App can access.
2. Run locally on the initial 32 GB M1 MacBook Pro with native Ollama.
3. Use three independent review lenses without requiring consensus.
4. Compare formally linked issue requirements with implementation and tests.
5. Preview findings before publishing through a single MAGI GitHub App.
6. Keep false positives low enough that developers trust the feedback.
7. Reuse the Go review engine when moving to dedicated or multiple workers.

The repository is named `magi-agents`; the user-facing executable is `magi`.
The Evangelion theme appears in reviewer names, not in technical verdicts.

## Authority

MAGI publishes `COMMENT` reviews and inline review conversations. It does not
approve PRs, request changes, merge, push fixes, or resolve conversations.

A repository rule requiring conversation resolution can make unresolved inline
threads block merging. Humans decide whether a finding is fixed or inapplicable.
Existing human approval requirements remain separate. A top-level summary alone
is not an inline thread and must not be presented as an equivalent merge gate.

## First usable slice

Prove MELCHIOR end to end before enabling all three roles:

- Resolve a repository and PR, acquire stable base/head SHAs, and read context.
- Prepare an isolated checkout and let MELCHIOR inspect it through controlled tools.
- Keep its configured model loaded during the session, then unload it.
- Validate and preview evidence-backed findings.
- After human confirmation, publish a COMMENT review under the shared MAGI identity.
- Clean up the checkout and report any incomplete work.

The broader V1 adds BALTHASAR's requirements coverage and CASPER's adversarial
review, executed sequentially on the initial laptop.

## Non-goals

- Automatic merge decisions, agent voting, or approval authority.
- Style policing, naming preferences, and speculative architecture rewrites.
- Unrestricted model-generated shell commands.
- Loading three large models simultaneously on the initial machine.
- Requiring Docker, a hosted service, or Kubernetes for the manual CLI.
- A newcomer-installation testing agent: the PDF's NOOB idea remains a separate
  possible project with different execution requirements.

Local-first means inference runs locally by default. GitHub access still uses
the network, and publication sends selected review content to GitHub.
