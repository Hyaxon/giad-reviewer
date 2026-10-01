# Deployment and operations

Status: native CLI is the initial target; services and multiple workers are future work.

## Manual native CLI

Run the compiled Go binary and native Ollama on the Mac. Use an isolated checkout
for each review. No containers, inbound webhook server, queue, or database are
required to prove review quality. The CLI owns terminal interaction; the reusable
runner owns review behavior. Propagate cancellation through all operations.

## Always-on machine

A Mac mini or equivalent host can run the same engine through a worker process.
Use a host supervisor or container restart policy rather than building a supervisor
inside MAGI. Add health checks, structured logs, and graceful shutdown.

Containerizing MAGI and containerizing inference are separate decisions. Keep
Ollama native on Apple Silicon initially. Linux containers on macOS introduce a VM
boundary; do not assume native Apple GPU acceleration. Check current runtime support
before revisiting this decision.

A container may reach the host through `host.docker.internal`, but Ollama must listen
on an interface reachable from that environment. Binding beyond loopback changes
exposure; choose firewall/network policy deliberately. Endpoints remain configuration.

When adding a Dockerfile, pin concrete Go and runtime versions, use a build stage,
include required runtime tools, and mount credentials read-only into trusted
components. The PDF's `latest` example is illustrative, not a production pin.

## Coordinator and multiple workers

Workers make outbound connections, advertise models/memory/platform/slots, lease
jobs, heartbeat, and return results. The coordinator owns scheduling and durable state.

A job identity should include:

```text
repository + pull_request + head_sha + agent + prompt_version
```

Record model identity and configuration as execution metadata. Distinguish deliberate
reruns from retries so idempotency does not prevent intentional re-review.

Schedule by model and capacity rather than role name. Default to one large-model
session on a 32 GB worker. Different devices can review in parallel without loading
multiple large models on the laptop.

Introduce durable leases and storage when jobs must survive restarts or workers
compete. PostgreSQL is an option, not a V1 prerequisite. Kubernetes is not part of
the initial plan; reconsider orchestration complexity only when real needs warrant it.

## Operational requirements

| Concern | Direction |
| --- | --- |
| Shutdown | Stop accepting work, finish/cancel safely, clean up and unload as appropriate |
| Health | Distinguish liveness from config, disk, GitHub, and inference readiness |
| Retries | Bound transient retries; do not repeat deterministic failures blindly |
| Observability | Repository, PR, SHAs, role, model, prompt version, durations, tool/result counts |
| Publication | Store review/comment IDs; reconcile uncertain API outcomes |
| Credentials | Keep keys in trusted components and out of test sandboxes |
| Worker trust | Authenticate workers and protect coordinator traffic |
| Upgrades | Version configuration, prompts, and worker protocols |
| Storage | Separate durable job records from checkouts and model caches |

Future webhook receivers must authenticate deliveries. Comment/label triggers add
authorization and duplicate-delivery concerns; arbitrary commenters must not gain
unrestricted execution on a worker.

Execution containers in [security](security.md) serve a different purpose from
service containers: isolating PR code from the trusted application.
