# Local models and memory

Status: downloaded models confirmed during scaffold setup; inference integration
and model quality benchmarking remain unimplemented.

## Initial machine and candidates

The design targets a 32 GB M1 MacBook Pro with Ollama running natively on macOS.
Downloaded model storage is an SSD concern. Loaded weights, context/KV caches,
the OS, and tests/build tools compete for unified memory.

| Reviewer | Initial model assignment |
| --- | --- |
| MELCHIOR | `qwen3.6:27b` |
| BALTHASAR | `devstral-small-2` (downloaded as `:latest`) |
| CASPER | `qwen3-coder:30b` |

These assignments are experiments, not claims of proven specialty superiority.
The PDF describes approximately 15–19 GB packages for the initial pool and a roughly
23 GB Qwen coding variant as a tighter-memory experiment. Package sizes and tags
can change; use the actual local inventory instead of treating those estimates as
runtime memory guarantees.

## Session lifecycle

Keep one model resident throughout an agent's reasoning/tool loop. Repeatedly
unloading between tool calls would waste time reloading weights. When the session
finishes, explicitly unload before starting the next reviewer. The PDF identifies
Ollama's `keep_alive: 0` as the immediate-unload mechanism.

Start around a 32K working context, then measure actual memory use. An advertised
maximum context is not a sensible automatic default on the laptop. Retrieve focused
code context through tools rather than sending the entire repository.

Lifecycle ownership belongs to MAGI: unload on completion and handle failures or
cancellation deliberately. Define behavior if another local client is sharing the
same Ollama instance; MAGI should not casually terminate unrelated model work.

## Manual preparation

```sh
ollama list
ollama ps
```

The first lists downloaded models; the second shows loaded models. Smoke-test one
candidate at a time before integration. Download availability does not establish
that tool calling, structured output, latency, or memory use is suitable.

## Provider abstraction

The runner knows a model interface, not Ollama-specific request fields. Provider
configuration supplies endpoint and model tag. Keep local URLs out of core reviewer
logic so native, container-adjacent, LAN, or future provider endpoints can be selected.

For repeatable benchmarks, record the actual resolved model identity/digest when
available. Mutable tags alone do not fully describe what ran.

## Benchmark plan

Use historical PRs with known defects and clean changes. Record useful verified
findings, false positives, serious misses, tool-call correctness, completion rate,
latency, load/unload time, and memory pressure. Penalize noise heavily: three agents
can multiply false positives as easily as useful coverage.

Compare model assignments and prompt versions on the same revisions. Distinguish
model quality from failures in retrieval, tool handling, or output validation.
See the [official model/runtime references](sources.md).
