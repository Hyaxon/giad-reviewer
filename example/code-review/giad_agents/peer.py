"""Synchronous GIAD peer. Repository and model access remain in the host."""

import json
import math
import sys

VERSION = "giad/v1"
MAX_FRAME = 1024 * 1024
MAX_REQUESTS = 64
MAX_MODEL_CALLS = 16
MAX_MODEL_PARAMS = 96 * 1024
MAX_TEST_RUNS = 2
REPORT_FIELDS = {"summary", "limitations", "findings"}
TEXT_FIELDS = {
    "source", "category", "file", "title", "explanation", "evidence",
    "failure_scenario", "suggested_fix",
}
FINDING_FIELDS = TEXT_FIELDS | {"line", "severity", "confidence"}


class ToolError(ValueError):
    """The host denied or could not complete a capability call."""


def encode(value):
    return json.dumps(value, ensure_ascii=False, allow_nan=False, separators=(",", ":"))


def reject_constant(value):
    raise ValueError(f"invalid JSON constant: {value}")


def decode(text):
    return json.loads(text, parse_constant=reject_constant)


def receive():
    line = sys.stdin.buffer.readline(MAX_FRAME + 1)
    if not line.endswith(b"\n") or len(line) >= MAX_FRAME:
        raise ValueError("missing or oversized host frame")
    frame = decode(line.decode("utf-8"))
    if not isinstance(frame, dict) or frame.get("apiVersion") != VERSION:
        raise ValueError("incompatible host protocol")
    return frame


def start_job():
    frame = receive()
    if (frame.get("method") != "review.start" or frame.get("id")
            or "result" in frame or "error" in frame
            or not isinstance(frame.get("params"), dict)):
        raise ValueError("expected review.start notification")
    return frame["params"]


class Host:
    def __init__(self):
        self.calls = 0
        self.total_bytes = 0

    def call(self, method, params):
        if not isinstance(params, dict):
            raise ValueError("request params must be an object")
        if method == "model.chat" and len(encode(params).encode("utf-8")) > MAX_MODEL_PARAMS:
            raise ValueError("model context exceeds 96 KiB; guidance was not truncated")
        self.calls += 1
        ident = str(self.calls)
        wire = encode({"apiVersion": VERSION, "id": ident, "method": method, "params": params})
        size = len(wire.encode("utf-8")) + 1
        self.total_bytes += size
        if size >= MAX_FRAME or self.calls > MAX_REQUESTS or self.total_bytes > 4 * MAX_FRAME:
            raise ValueError("request exceeds GIAD session budget")
        sys.stdout.write(wire + "\n")
        sys.stdout.flush()
        reply = receive()
        if reply.get("id") != ident or reply.get("method") or "params" in reply:
            raise ValueError("unexpected host response")
        if reply.get("error"):
            raise ToolError(f"{method} failed: {reply['error']}")
        if "result" not in reply:
            raise ValueError("host response has no result")
        return reply["result"]

    def finish(self, report, observed=None):
        validate_report(report, observed)
        reply = self.call("review.finish", report)
        if not isinstance(reply, dict) or reply.get("accepted") is not True:
            raise ValueError("report not accepted")


def validate_report(report, observed=None, max_findings=20):
    if not isinstance(report, dict) or set(report) != REPORT_FIELDS:
        raise ValueError("report requires only summary, limitations, and findings")
    for key in ("summary", "limitations"):
        value = report[key]
        if not isinstance(value, str) or len(value.encode("utf-8")) > 8192:
            raise ValueError(f"{key} must be text of at most 8 KiB")
    if not report["summary"].strip():
        raise ValueError("summary must be nonempty")
    if not isinstance(report["findings"], list) or len(report["findings"]) > max_findings:
        raise ValueError(f"findings must be an array of at most {max_findings} entries")
    for finding in report["findings"]:
        if not isinstance(finding, dict) or set(finding) != FINDING_FIELDS:
            raise ValueError("finding fields do not match giad/v1")
        for key in TEXT_FIELDS:
            value = finding[key]
            if not isinstance(value, str) or not value.strip() or len(value.encode("utf-8")) > 8192:
                raise ValueError(f"finding {key} must be nonempty text of at most 8 KiB")
        line = finding["line"]
        if type(line) is not int or line < 1:
            raise ValueError("finding line must be a positive integer")
        if observed is None or line not in observed.get(finding["file"], set()):
            raise ValueError("finding cites uninspected source")
        if finding["severity"] not in ("high", "medium", "low"):
            raise ValueError("severity must be high, medium, or low")
        confidence = finding["confidence"]
        if (type(confidence) not in (int, float) or not 0 <= confidence <= 1
                or not math.isfinite(confidence)):
            raise ValueError("confidence must be a finite number in [0, 1]")
    if len(encode(report).encode("utf-8")) > 64 * 1024:
        raise ValueError("report exceeds 64 KiB")


def run(main, name):
    try:
        main()
    except (ValueError, KeyError, TypeError, AttributeError, OSError) as error:
        print(f"{name}: {error}", file=sys.stderr)
        raise SystemExit(1) from error
