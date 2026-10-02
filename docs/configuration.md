# Configure an agent

The runtime needs two local files: an agent manifest describing what to launch,
and a TOML config granting capabilities and mapping model profiles.

1. Copy [the example manifest](../examples/agent.manifest.json). Set its `name`,
   absolute executable path, arguments, required capabilities, and model profile names.
2. Copy [the example config](../agentic-review.example.toml). Match the agent name
   and profile names from the manifest; set your downloaded Ollama model tags.
3. Pass both files explicitly with `review --agent-manifest PATH --config PATH`.

There is no automatic config lookup or bundled reviewer. Use files and executables
you trust, outside the PR checkout.

| Config entry | Meaning |
| --- | --- |
| `[models.NAME]` | Logical profile requested by the agent |
| `provider` | Currently `"ollama"` |
| `endpoint` | Ollama HTTP(S) origin; use the local example unless you intend to send code elsewhere |
| `model` | Downloaded model tag |
| `[agents.NAME].capabilities` | Capabilities this agent may request |

Required capabilities must be declared, granted, and implemented; otherwise launch
fails. Optional capabilities stay off unless all three conditions hold. Supported
operations are listed in [the protocol](agent-protocol.md).

Authentication currently uses your GitHub CLI account. App key/JWT helpers exist,
but App authentication is not connected to the CLI. The separate
[identity example](../identity.example.toml) uses `[github.app]`; legacy
`[github.magi]` files remain supported by the helper. Do not combine both sections.
