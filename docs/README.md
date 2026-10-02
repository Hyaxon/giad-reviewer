# MAGI documentation

MAGI is a general-purpose, local-first GitHub pull-request reviewer written in
Go. Three independent review roles use local models and publish through a personal account or one user-owned
GitHub App. Humans own review resolution and merge decisions.

These documents combine the 28-page **MAGI Code Review System - Design, Setup,
and Deployment** guide (September 30, 2026) with subsequent project decisions.
They describe the intended product, not an already functioning review engine.

## Start here

| Document | What it explains |
| --- | --- |
| [Goals and scope](goals.md) | Purpose, V1 success criteria, and non-goals |
| [Current status](status.md) | What exists, what is reported configured, and what is unimplemented |
| [Go orientation](go-development.md) | Packages, dependencies, builds, and this repository's layout |
| [Architecture](architecture.md) | Component boundaries and the review lifecycle |
| [Reviewers and findings](review-policy.md) | Agent roles, evidence, verification, and publication rules |
| [Requirements traceability](requirements.md) | Formal issue links and requirement assessments |
| [CLI and repository selection](cli.md) | Current commands and proposed general-purpose interface |
| [Configuration](configuration.md) | Model settings, one App identity, and local configuration proposal |
| [Authentication modes](authentication.md) | Personal-account and dedicated-App setup without a central backend |
| [GitHub App setup](github-app.md) | Personal-account installation, permissions, keys, and authentication design |
| [Local models](models.md) | Ollama, sequential execution, context limits, and benchmarking |
| [Tools and security](security.md) | Controlled repository tools, isolation, and credential boundaries |
| [Deployment and operations](deployment.md) | Native CLI, future services, containers, and multiple workers |
| [Implementation roadmap](roadmap.md) | Incremental milestones and acceptance criteria |
| [Decision log](decisions.md) | Accepted decisions, PDF supersessions, and open questions |
| [Sources](sources.md) | PDF section mapping and official external references |

For a first read, follow goals → status → Go orientation → architecture → roadmap.
Use the other pages as implementation references.

## How to interpret the docs

- **Implemented:** verified in this repository's code.
- **Accepted design:** agreed behavior, still requiring implementation unless stated otherwise.
- **Proposed:** a concrete starting point that has not yet been agreed or implemented.
- **Future:** beyond the first manual review workflow.
- **Reported:** setup described by the owner, not independently verified end to end.

The [decision log](decisions.md) takes precedence over conflicting details in the
original PDF. Most importantly, the three reviewers now share **one MAGI App**,
and the target repository is selected per invocation. Credentials are local;
this documentation deliberately contains no private keys or real App IDs.

No `review`, `doctor`, worker, or configuration-loading functionality exists yet.
Examples for those features are explicitly marked as planned.
