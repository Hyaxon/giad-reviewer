"""Minimal GIAD agent: summarize job metadata without a model or dependencies."""

import json
from pathlib import Path
import sys

VERSION = "giad/v1"


def receive():
    line = sys.stdin.readline(1024 * 1024 + 1)
    if not line.endswith("\n") or len(line.encode("utf-8")) > 1024 * 1024:
        raise ValueError("missing or oversized host frame")
    frame = json.loads(line)
    if frame.get("apiVersion") != VERSION:
        raise ValueError("incompatible host protocol")
    return frame


def main():
    if sys.argv[1:] == ["--manifest"]:
        print(json.dumps({
            "apiVersion": VERSION, "name": "pr-summary", "version": "0.1.0",
            "entrypoint": {
                "command": str(Path(sys.executable).resolve()),
                "args": [str(Path(__file__).resolve())],
            },
            "capabilities": {"required": ["repository.instructions"], "optional": []},
            "modelProfiles": [],
        }, indent=2))
        return
    if sys.argv[1:]:
        raise ValueError("use --manifest to print the manifest, or no args to run")

    start = receive()
    if start.get("method") != "review.start" or start.get("id"):
        raise ValueError("expected review.start")
    job = start["params"]
    report = {
        "summary": f"PR #{job['number']}: {job['title']} ({len(job['changedFiles'])} changed files).",
        "limitations": "Metadata example only; code, repository guidance, and tests were not evaluated.",
        "findings": [],
    }
    print(json.dumps({
        "apiVersion": VERSION, "id": "finish", "method": "review.finish", "params": report,
    }), flush=True)
    reply = receive()
    if (reply.get("id") != "finish" or reply.get("method") or reply.get("error")
            or reply.get("result", {}).get("accepted") is not True):
        raise ValueError(f"report rejected: {reply}")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, TypeError, AttributeError) as error:
        print(f"pr-summary: {error}", file=sys.stderr)
        sys.exit(1)
