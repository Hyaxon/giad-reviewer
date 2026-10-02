# One MAGI GitHub App

This guide covers the optional dedicated-App mode. See [authentication modes](authentication.md)
for personal-account setup. Key loading and client-ID JWT signing exist; installation
token exchange and publishing remain unimplemented.

## Identity and ownership

Register one App under a personal GitHub account. All reviewer sessions publish
through that identity, with MELCHIOR, BALTHASAR, or CASPER named in review text.
Separate model sessions and prompts preserve distinct review roles.

One App means one private key and installation configuration. Three Apps would
permit independently revoking or permissioning roles, but those benefits do not
justify the overhead for the current shared-runtime design.

## Registration checklist

Open [GitHub App registration](https://github.com/settings/apps/new), or navigate
to personal Settings → Developer settings → GitHub Apps → New GitHub App.

| Field | Intended setting |
| --- | --- |
| Name | An available name such as `hyaxon-magi` |
| Homepage | Your GitHub profile or project URL |
| Callback and Setup URLs | Blank |
| User OAuth authorization during installation | Disabled |
| Device flow | Disabled |
| Webhook Active | Disabled for manual V1 |
| Installation visibility | Only on this account |

Record the App ID, generate a private key, and install the App on the personal
account. The owner chose **All repositories**, which supports selecting among
their repositories per command. The original PDF's selected-test-repository
recommendation is superseded by that local setup choice.

The Installation ID identifies the App's installation on an account, not a single
repository. It is distinct from the App ID and appears in the installation settings
URL, for example `https://github.com/settings/installations/12345678`.

See GitHub's [registration](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/registering-a-github-app)
and [installation](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app) instructions.

## Permissions

Enable these repository permissions for V1:

| Permission | Access | Purpose |
| --- | --- | --- |
| Contents | Read-only | Source and revision context |
| Issues | Read-only | Linked issue requirements |
| Metadata | Read-only | Repository metadata |
| Pull requests | Read and write | PR data, reviews, inline comments |

Potential later additions, only when the corresponding feature is implemented:

| Permission | Access | Feature |
| --- | --- | --- |
| Actions | Read-only | CI runs and logs |
| Checks | Read-only or read/write | Read checks or publish MAGI check runs |
| Commit statuses | Read-only or read/write | Read or publish commit statuses |
| Code scanning alerts | Read-only | Existing scanner findings |
| Dependabot alerts | Read-only | Existing dependency findings |
| Issues | Read and write | Creating or modifying actual issues |

Leave other permissions disabled. No account or organization permissions are
needed for this design. Workflows write is not required to read workflow files.
Repository Webhooks permission manages hooks; receiving the App's own future
events is configured separately. Administration is unnecessary when the owner
configures branch rules manually.

These are endpoint-based requirements, not a guarantee about every future GraphQL
query. Verify real calls with the installation token, particularly linked issues.
References: [App permission matrix](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps),
[review API](https://docs.github.com/en/rest/pulls/reviews#create-a-review-for-a-pull-request),
[check runs](https://docs.github.com/en/rest/checks/runs), and
[workflow runs](https://docs.github.com/en/rest/actions/workflow-runs).

## Private key storage

The proposed local location is `~/.config/magi/keys/magi.pem`, outside source control.
To prepare it manually:

```sh
mkdir -p ~/.config/magi/keys
chmod 700 ~/.config/magi ~/.config/magi/keys
# Move the downloaded key here and name it magi.pem, then:
chmod 600 ~/.config/magi/keys/magi.pem
```

Do not expose keys or installation tokens to model context, PR tools, test processes,
logs, or published findings. If a key is exposed, revoke/rotate it through GitHub.
See [private key management](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/managing-private-keys-for-github-apps).

## Planned authentication flow

The identity file uses `[github.magi]` with `app_id`, `client_id`,
`installation_id`, and `private_key`. JWT signing uses the **client ID** as
`iss`, following GitHub's recommendation. The App ID is retained as metadata;
the Installation ID selects the account installation. A client ID is not a
client secret. Copy it from the App's General settings into local `identity.toml`.
See the current [identity template](../identity.example.toml) and
[JWT requirements](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app).

1. Load the configured App identity and private key in trusted host code.
2. Sign a short-lived App JWT according to GitHub's current requirements.
3. Exchange it for an installation access token.
4. Use the installation token for allowed repository API requests.
5. Refresh expired tokens; keep them in memory rather than persistent logs/files.

In App mode, publication must use the configured App installation; do not silently
fall back to personal credentials. Personal `gh` authentication is supported by design
only when the user explicitly chooses personal-account mode.
The first live validation should be read-only. Test token acquisition and repository
access before publishing anything. [Installation authentication](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)

## Branch rules

The owner may enable conversation resolution and maintain separate human review
requirements. Availability depends on the repository/account plan and rule type;
check GitHub's current UI. Test with a disposable PR. MAGI should not change rules
or automatically resolve its own feedback.

## Distribution and authentication for other users

The accepted design offers personal-account authentication through `gh` and an
optional user-owned App. Both run locally without a central MAGI backend. In App
mode, each operator owns and stores their own App key; MAGI never distributes a
shared private key. See [authentication modes](authentication.md).

A single publicly installed MAGI bot with browser-only onboarding would require a
separate trusted service design. It is not required or planned for these two modes.
