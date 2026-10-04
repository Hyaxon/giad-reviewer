# Runtime roadmap

Planned features for making custom agents easy to build and use on GitHub PRs.
[Issues](https://github.com/Hyaxon/giad/issues) track the work;
[Discussions](https://github.com/Hyaxon/giad/discussions) are for ideas and design choices.

- Easier installation with prebuilt binaries, an installer, and Homebrew.
- Guided `giad setup` for personal GitHub authentication or your own GitHub App.
- Saved defaults and shorthand PR targets such as `OWNER/REPO#123`.
- Minimal agent templates, `giad agent init`, and an isolated `giad agent dev`.
- Agent installation, selection, and per-repository configuration.
- Automatic PR reviews through webhooks or polling.
- Repository rules for agents, PR events, paths, and automatic feedback.
- Queued reviews with status, cancellation, retries, and stale-job handling.
- Multiple agents on one PR, with clearly attributed results.
- Explicit issue context and pinned base/head file reads.
- Approved base/head test comparisons.
- Optional host-brokered web search, page retrieval, and vulnerability lookup.
- Opt-in automatic publication with comment limits and repeat suppression.
- Deleted-line and multiline findings.
- Real PR demos, a clearer README, and small contributor tasks.
- Verified releases and tested agent compatibility.
- A shared visual theme, artwork, and iconography.
- Integrations with coding-agent tools.

Local reviews, additional model providers, parallel/distributed execution,
generated-test validation, and a hosted service remain ideas for discussion.
