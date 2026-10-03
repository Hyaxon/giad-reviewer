# Example agents

This folder contains [pr-summary](pr-summary/agent.py), the Go
[diff-inspector](diff-inspector/main.go), and a generic
[manifest template](agent.manifest.json).

[pr-summary](pr-summary/agent.py) is a tiny Python 3 agent with no dependencies or
model. It receives `review.start`, summarizes PR metadata, sends `review.finish`,
and waits for acceptance. Empty findings mean no analysis was performed here.

Run from the repository root:

```sh
make build
python3 example/pr-summary/agent.py --manifest > bin/pr-summary.agent.json
./bin/giad review 42 --repo OWNER/REPO \
  --agent-manifest bin/pr-summary.agent.json \
  --config example/pr-summary/config.toml
```

The generated manifest uses absolute Python/script paths; regenerate it if you move
the checkout. The instruction capability allows GIAD to include base AGENTS.md;
this example explicitly reports that it does not evaluate that guidance.
