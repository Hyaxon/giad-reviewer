# Authentication modes

Status: personal authentication is the first supported CLI path. `magi auth status`
uses GitHub CLI credentials and verifies the account with GitHub. Interactive
setup and App token exchange are deferred. Existing identity/key/JWT helpers are
preserved for App mode; they are not loaded by personal mode.

```sh
gh auth login --hostname github.com
go run ./cmd/magi auth status
```

## Setup choice

```text
How should MAGI publish GitHub reviews?

1. Use my GitHub account
   Easiest setup. Reviews appear from your account.

2. Use a dedicated GitHub App
   More setup. Reviews appear from your bot.
```

Both modes run locally and use the same review engine. Neither needs a central
MAGI backend. All reviewer outputs remain role-labeled and COMMENT-only.

## Personal-account mode

Start with GitHub CLI authentication rather than implementing custom OAuth.
Setup checks the selected host/account and guides `gh auth login` when needed.
The provider obtains the token via `gh auth token` for that host/account, captures
it privately, and verifies the actual identity. Never print it or save a duplicate
in TOML. Environment-provided credentials and multiple accounts must be resolved
explicitly so preview identifies the account that will actually publish.

Authentication does not guarantee repository access: validate relevant API calls
and surface permission, token-expiration, or organization-policy failures. Do not
silently switch accounts or auth modes. In unattended use, fail with actionable
instructions instead of launching an interactive login.

References: [GitHub CLI login](https://cli.github.com/manual/gh_auth_login) and
[token retrieval](https://cli.github.com/manual/gh_auth_token).

## Dedicated-App mode

The operator creates or owns an App and configures its App ID, client ID,
Installation ID, and private-key path. JWT signing uses the client ID. The provider
obtains installation tokens and refreshes them as needed. All three reviewer roles
share that App identity. Follow [App setup](github-app.md) for permissions and keys.

This is distinct from installing a centrally owned public MAGI App. The local CLI
must never contain a shared App private key. Browser-only onboarding for a shared
bot would require an additional trusted service and is outside this local design.

## Shared provider boundary

Implemented as `githubauth.Provider` (the conceptual interface below):

```go
type GitHubAuth interface {
    Token(ctx context.Context, repo Repository) (string, error)
}
```

`UserAuth` wraps the effective local GitHub CLI identity. Future `AppAuth` will own App JWT and
installation-token acquisition. Token expiry/caching belongs inside each provider.
The repository includes its host so credentials cannot be sent to arbitrary hosts.
The GitHub client uses the provider for acquisition and publication; the agent sees
neither tokens nor provider internals. Preview must show verified publishing identity.

PR retrieval, linked issues, local checkout, model inference, validation, and review
payload construction stay shared. Differences in permission errors, expiry, and
identity discovery remain adapter concerns. Broader personal-token permissions do
not relax MAGI's COMMENT-only and no-code-push rules.

## Proposed configuration

Personal mode in the main configuration:

```toml
[github]
auth_mode = "user"
```

App mode in the main configuration:

```toml
[github]
auth_mode = "app"
```

Keep App metadata and the private-key path in the separate identity file. Its current
section is `[github.magi]`. A clearer future name is `[github.app]`, but changing it
requires a deliberate loader/template migration; it is not implemented by these docs.
Neither mode selector is currently consumed by the CLI; the current auth command
explicitly uses personal mode. Personal mode must skip
App identity loading entirely. Unknown modes should fail rather than fall back.

Rerun `magi setup` to switch modes once implemented. A dedicated `magi config`
command is optional future UX. Confirm the new publishing identity before saving;
do not revoke credentials for the previous mode implicitly.
