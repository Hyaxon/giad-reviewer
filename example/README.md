# Example agents

This folder contains [pr-summary](pr-summary/agent.py), the Go
[diff-inspector](diff-inspector/main.go), [code-review](code-review/agent.py), and a generic
[manifest template](agent.manifest.json).

[pr-summary](pr-summary/agent.py) is a tiny Python 3 agent with no dependencies or
model. It receives `review.start`, summarizes PR metadata, sends `review.finish`,
and waits for acceptance. Empty findings mean no analysis was performed here.

Run from the repository root:

```sh
make build
docker build -t giad-pr-summary:example example/pr-summary
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest example/pr-summary/agent.manifest.json \
  --config example/pr-summary/config.toml
```

The [explicit manifest](pr-summary/agent.manifest.json) uses paths inside the image,
so it works across checkouts. Alternatively, `--manifest --container` prints the
same manifest. The instruction capability supplies base AGENTS.md; this example
reports that it does not evaluate that guidance. Docker must be running.

## Model-backed reviewer

[code-review](code-review/agent.py) reads the diff and up to three changed head
files (first 160 lines each), applies scoped base guidance, and asks the host's
`review` model for anchored findings. If `tests.run` is granted, it runs the first
approved profile once and supplies its results to the model. JSON code blocks are
accepted; invalid reports get one correction attempt and then fail. An empty
findings array means no findings within this limited pass.

Start Ollama and choose a downloaded model that follows JSON instructions. Then:

```sh
make build
docker build -t giad-code-review:example example/code-review
cp example/code-review/config.toml giad.toml
ollama list
# Set models.review.model in giad.toml to an exact installed tag from ollama list.
# Copying the template resets your local settings.
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest example/code-review/agent.manifest.json --config giad.toml
```

`make images sandbox-smoke` tests the actual container and broker using scripted
model responses. Those checks verify integration and failure handling, not model
judgment quality. The manifest names a logical profile; Ollama access stays in GIAD.

Optional Go tests use [the test image](go-tests/Dockerfile), built by `make images`.
Enable them using [the configuration example](../docs/configuration.md).
[The tiny fixture](go-tests/fixture/add.go) deliberately fails `TestAdd`;
`make sandbox-smoke` checks that failure, passing tests, timeouts, output limits,
and cleanup in real containers. Fixing `a - b` to `a + b` makes the fixture pass.
