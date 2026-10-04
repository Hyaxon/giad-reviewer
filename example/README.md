# Example agents

| Agent | Demonstrates |
| --- | --- |
| [pr-summary](pr-summary/agent.py) | Tiny Python metadata summary with an explicit manifest |
| [diff-inspector](diff-inspector/main.go) | Go peer requesting the diff through the broker |
| [code-review](code-review/README.md) | General PR review with adaptive tools and optional sandboxed tests |

Use [the generic manifest](agent.manifest.json) when defining another package.
Every runnable example uses an installed Docker image.

The optional [official GIAD Agents collection](https://github.com/Hyaxon/giad-agents)
is available separately with package manifests, a shared image, and a source
catalog. It also includes the model-free `test-summary`, which summarizes approved
test runs. Planned specialists such as MAGI, Test Writer, and Wacht are described
in its [agent roadmap](https://github.com/Hyaxon/giad-agents/blob/main/docs/roadmap.md).

## Smallest example

From the repository root, with Docker running:

```sh
make build
docker build -t giad-pr-summary:example example/pr-summary
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest example/pr-summary/agent.manifest.json \
  --config example/pr-summary/config.toml
```

This agent performs no defect analysis. It returns a metadata summary and empty
findings, then waits for host acceptance. Its manifest paths are inside the image.
The Go diff-inspector is the model-free quickstart in [the main README](../README.md).

## Save and publish model-free examples

These commands use GitHub App authentication. First copy
[identity.example.toml](../identity.example.toml) to `identity.toml` and fill in
your App ID, Client ID, installation ID, and private-key path using
[the App authentication setup](../docs/configuration.md#authentication).

From the repository root, build both examples and add the local CLI to this shell's
`PATH`. Neither agent needs Ollama:

```sh
make all && make images
export PATH="$PWD/bin:$PATH"
giad auth status --identity identity.toml
giad review 42 --repo OWNER/REPO --identity identity.toml \
  --agent-manifest example/pr-summary/agent.manifest.json \
  --config example/pr-summary/config.toml --json > draft-pr-summary.json
giad review 42 --repo OWNER/REPO --identity identity.toml \
  --agent-manifest bin/diff-inspector.agent.json \
  --config example/diff-inspector/config.toml --json > draft-diff-inspector.json
```

Preview a saved draft, inspect the body, then publish with the printed hash:

```sh
giad publish draft-pr-summary.json --identity identity.toml
giad publish draft-pr-summary.json --identity identity.toml --confirm HASH
```

Repeat with `draft-diff-inspector.json` and its own preview hash to publish the other
agent's result. Both publish `COMMENT` reviews with summaries and coverage
limitations; they return no findings or inline comments. Omit `--identity` from
all commands to use personal GitHub CLI authentication instead.

## Model-backed reviewer

Start Ollama, inspect `ollama list`, and choose an installed model that supports
tool calls. Copy the
configuration once, then edit `models.review.model` in your local `giad.toml`:

```sh
make build
docker build -t giad-code-review:example example/code-review
cp -n example/code-review/config.toml giad.toml
ollama list
```

Run after setting the model:

```sh
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest example/code-review/agent.manifest.json \
  --config giad.toml --json > draft.json
```

For bot authentication, append `--identity identity.toml`. Then follow
[the publication commands](../README.md#save-and-publish).

The reviewer mirrors the general reviewer in the official
[GIAD Agents collection](https://github.com/Hyaxon/giad-agents). It plans from the PR,
change inventory, and diff, then chooses source ranges, repository searches,
dependency/context reads, linked issues, and approved tests as needed. There is no
three-file, 160-line, 12 KiB evidence, or per-file finding cutoff.

Scoped base guidance remains trusted; source, PR/issue text, and test logs remain
evidence. Finding anchors must come from reads of changed head source. Context
pagination, whole-turn eviction, and explicit coverage tracking keep reviews
within GIAD's runtime budgets. Invalid reports can be corrected within the model
budget; model/transport failures and unfinished reviews fail. See the
[reviewer reference](code-review/README.md) for tools, limits, and synchronization.
Model judgment still needs independent quality evaluation.

## Optional tests and integration checks

Enable `tests.run` using [configuration](../docs/configuration.md#optional-tests).
The model can select approved profiles within GIAD's two-run budget and receives
their observed results as evidence. Tests are head-only and may use any trusted
profile/image; arbitrary commands and unapproved profiles are rejected.

```sh
make images
make sandbox-smoke
```

These checks exercise real agent/test isolation, the broker, output limits,
timeouts, and cleanup using scripted model responses. The nested
[Go fixture](go-tests/fixture/add.go) deliberately fails `TestAdd`; changing
`a - b` to `a + b` makes it pass. Its nested module keeps it outside root Go tests.
