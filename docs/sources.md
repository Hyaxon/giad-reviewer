# Sources and provenance

## Original design

**MAGI Code Review System - Design, Setup, and Deployment**, September 30, 2026,
28 pages. Local filename: `MAGI_Code_Review_System_Design_and_Setup.pdf`.
It was parsed for this task. It may not be present in every checkout; these
Markdown guides are intended to stand on their own.

| PDF sections/pages | Destination |
| --- | --- |
| 1–2, pp. 2–5: summary and architecture | goals, architecture |
| 3–4, pp. 5–6: reviewers and authority | review-policy, github-app |
| 5, pp. 6–9: issue traceability | requirements |
| 6–7, pp. 9–10: tools and findings | security, review-policy |
| 8, pp. 10–12: models | models |
| 9, pp. 12–14: Go and packages | architecture, go-development |
| 10, pp. 14–15: local setup | status, configuration, roadmap |
| 11–12, pp. 15–17: Apps and branch rules | github-app, decisions |
| 13–15, pp. 17–20: first slice, prompts, CLI | roadmap, review-policy, cli |
| 16–17, pp. 20–24: deployment/evolution | deployment |
| 18, p. 25: guardrails | security |
| 19, pp. 25–27: build order | roadmap |
| 20, pp. 27–28: references | this page |
| 21, p. 28: success criteria | goals, roadmap |

Later discussion supersedes separate App identities and clarifies per-command
repository selection. Code inspection establishes current functionality.
Credential setup is owner-reported, not verified through a live API check.

## GitHub references

Registration and permission references were checked during the setup discussion.
Recheck exact contracts during implementation; this is not a frozen API specification.

- [Registering an App](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
- [Installing an App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [Private key management](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps)
- [Installation authentication](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)
- [App permission matrix](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)
- [PR reviews](https://docs.github.com/en/rest/pulls/reviews)
- [Inline review comments](https://docs.github.com/en/rest/pulls/comments)
- [Issue comments](https://docs.github.com/en/rest/issues/comments)
- [Check runs](https://docs.github.com/en/rest/checks/runs)
- [Workflow runs](https://docs.github.com/en/rest/actions/workflow-runs)
- [Repository rules](https://docs.github.com/en/rest/repos/rules)
- [App webhooks](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/using-webhooks-with-github-apps)
- [GraphQL PullRequest](https://docs.github.com/en/graphql/reference/objects#pullrequest)

## Implementation reference starting points

These official entry points are not additional evidence that future features work:

- [Go documentation](https://go.dev/doc/)
- [Go module reference](https://go.dev/ref/mod)
- [Cobra](https://cobra.dev/docs/)
- [Ollama documentation](https://docs.ollama.com/)
- [Ollama FAQ](https://docs.ollama.com/faq)
- [Devstral Small 2](https://ollama.com/library/devstral-small-2)
- [Qwen3.6](https://ollama.com/library/qwen3.6)
- [Qwen3-Coder](https://ollama.com/library/qwen3-coder)
- [Docker Desktop on Mac](https://docs.docker.com/desktop/setup/install/mac-install/)

Model tags/sizes, API versions, and acceleration support can change. Record actual
versions and consult primary documentation during implementation and benchmarking.
