#!/usr/bin/env python3
"""Handler-level read-proto completeness gate.

The write-proto gate (scripts/verify_write_proto.py) covers the mutating
surface. This is its read-surface sibling: the conformance corpus proves a
proto MESSAGE round-trips Go==Python, but not that any HANDLER emits it, so a
Go read handler can emit proto-canonical output while its Python twin curates
a dict, and both pass. This gate statically classifies every READ tool
(capability Read) on BOTH sides as proto-routed or legacy, so a read handler
cannot stay or go legacy unnoticed.

Two independent classifiers do the static analysis (no handler is executed):

  Go:     go run ./cmd/write-proto-dump -surface read -> {tool: "proto"|"legacy"}
          proto = the handler reaches MarshalProtoToolResponse.
          legacy = it reaches only MarshalToolResponse.
  Python: linodemcp.tools._write_proto_classifier classify("read")
          proto = the handler reaches serialize_api_response/serialize_list_response.
          legacy = it builds a curated dict with no serialize call.

A tool is a STRAGGLER when either side is not proto. This is a HARD gate: any
straggler fails by name. There is no baseline file and no acceptance path,
because the read-surface conversion is finished and a tool is born proto-routed
now that the emitter writes every handler from the contract. A straggler means
someone hand-wrote a read path, which is the thing to fix rather than to record.

Run directly, via `make read-proto` (root Makefile), or as a pre-commit hook.
The Go dumper needs the Go toolchain; the Python classifier is imported under
the venv, so this runs under the venv interpreter (the Makefile/hook do that).
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import _hardgate

_REPO_ROOT = Path(__file__).resolve().parents[1]
_GO_DIR = _REPO_ROOT / "go"
_PY_SRC = _REPO_ROOT / "python" / "src"


def _dump_go() -> dict[str, str]:
    """Run the Go classifier in read mode and return {tool: "proto"|"legacy"}."""
    result = subprocess.run(
        ["go", "run", "./cmd/write-proto-dump", "-surface", "read"],
        cwd=_GO_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"go write-proto-dump -surface read failed (exit {result.returncode})"
        raise SystemExit(msg)

    parsed: dict[str, str] = json.loads(result.stdout)
    return parsed


def _dump_python() -> dict[str, str]:
    """Import the Python classifier in read mode."""
    if str(_PY_SRC) not in sys.path:
        sys.path.insert(0, str(_PY_SRC))

    from linodemcp.tools._write_proto_classifier import classify  # noqa: PLC0415

    parsed: dict[str, str] = classify("read")
    return parsed


def _stragglers(go: dict[str, str], py: dict[str, str]) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for every straggler.

    A straggler is any read tool that is not proto-routed on both sides. The
    line carries both statuses so the finding says which side needs work rather
    than only that the two disagree.
    """
    lines: list[str] = []

    for tool in sorted(set(go) | set(py)):
        go_status = go.get(tool, "missing")
        py_status = py.get(tool, "missing")

        if go_status != "proto" or py_status != "proto":
            lines.append(f"{tool}\t{go_status}\t{py_status}")

    return lines


def main() -> int:
    go = _dump_go()
    py = _dump_python()

    _hardgate.measured("the Go read-surface classifier", len(go))
    _hardgate.measured("the Python read-surface classifier", len(py))

    code = _hardgate.report(
        "read handlers not proto-routed on both sides",
        _stragglers(go, py),
        "Route the handler's output through its proto message in every"
        " language. A generated tool gets that from the contract, so a straggler"
        " is hand-written code standing where generated code belongs.",
    )

    if code == 0:
        _hardgate.say(
            f"read-proto OK: {len(go)} Go and {len(py)} Python read handlers"
            " proto-routed"
        )

    return code


if __name__ == "__main__":
    raise SystemExit(main())
