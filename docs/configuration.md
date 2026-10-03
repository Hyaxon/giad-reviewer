# Configure an agent

Use explicit trusted files outside the PR checkout: a manifest describing the
executable and declarations, and TOML granting capabilities/mapping model profiles.

For the runnable model-free example:

```sh
make example
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml
```

The generated manifest contains the built executable's absolute path. Rebuild it
if you move the checkout. No model endpoint or credentials belong in the agent.

For a custom agent, copy [the manifest template](../example/agent.manifest.json)
and [giad.example.toml](../giad.example.toml). Match the agent name/profile names;
set the absolute executable/script paths and any downloaded model tags.

| Config entry | Meaning |
| --- | --- |
| `[models.NAME]` | Logical profile declared by the agent; omit for model-free agents |
| `provider` | Currently `"ollama"` |
| `endpoint` | HTTP(S) origin; remote origins send your source conversations there |
| `model` | Downloaded model tag |
| `[agents.NAME].capabilities` | Agent's permitted broker methods |

Required capabilities must be declared, granted, and implemented; otherwise launch
fails. Optional capabilities remain off unless all three conditions hold. See
[the protocol](agent-protocol.md). There is no automatic config lookup or agent registry.

Auth currently uses GitHub CLI. App helpers exist but token exchange is not wired.
The separate [identity example](../identity.example.toml) uses `[github.app]`; legacy
`[github.magi]` remains accepted by the helper. Do not combine both sections.
