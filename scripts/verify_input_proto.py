#!/usr/bin/env python3
"""Factory-level input-proto completeness gate.

The tool-parity gate (scripts/verify_tool_parity.py) proves the Go and Python
input schemas agree in shape. It does NOT prove where that shape comes from: a
tool can advertise the same schema on both sides while Go generates it from the
proto contract and Python hand-maintains a matching dict (or the reverse). This
gate is the input-schema sibling of read-proto/write-proto: it statically
classifies every tool (all capabilities) on BOTH sides as proto-generated or
hand-built, so an input schema cannot stay hand-built unnoticed.

Two independent classifiers do the static analysis (no factory is executed):

  Go:     go run ./cmd/write-proto-dump -surface input -> {tool: "generated"|"hand"}
          generated = the factory reaches mcp.NewToolWithRawSchema /
          toolschemas.Schema. hand = it builds the schema from mcp.With* options.
  Python: linodemcp.tools._write_proto_classifier classify("input")
          generated = the create_<tool>_tool factory sets input_schema=schema(...).
          hand = it passes a dict-literal input_schema.

A tool is a STRAGGLER when either side is not generated. This is a HARD gate:
any straggler fails by name. There is no baseline file and no acceptance path,
because the input-surface conversion is finished and a tool's schema comes off
its proto message now that the emitter writes every factory from the contract.
A straggler means someone hand-built a schema, which is the thing to fix rather
than to record.

Run directly, via `make input-proto` (root Makefile), or as a pre-commit hook.
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
    """Run the Go classifier in input mode and return {tool: "generated"|"hand"}."""
    result = subprocess.run(
        ["go", "run", "./cmd/write-proto-dump", "-surface", "input"],
        cwd=_GO_DIR,
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"go write-proto-dump -surface input failed (exit {result.returncode})"
        raise SystemExit(msg)

    parsed: dict[str, str] = json.loads(result.stdout)
    return parsed


def _dump_python() -> dict[str, str]:
    """Import the Python classifier in input mode."""
    if str(_PY_SRC) not in sys.path:
        sys.path.insert(0, str(_PY_SRC))

    from linodemcp.tools._write_proto_classifier import classify  # noqa: PLC0415

    parsed: dict[str, str] = classify("input")
    return parsed


def _stragglers(go: dict[str, str], py: dict[str, str]) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for every straggler.

    A straggler is any tool that is not proto-generated on both sides. The line
    carries both statuses so the finding says which side needs work rather than
    only that the two disagree.
    """
    lines: list[str] = []

    for tool in sorted(set(go) | set(py)):
        go_status = go.get(tool, "missing")
        py_status = py.get(tool, "missing")

        if go_status != "generated" or py_status != "generated":
            lines.append(f"{tool}\t{go_status}\t{py_status}")

    return lines


def main() -> int:
    go = _dump_go()
    py = _dump_python()

    _hardgate.measured("the Go input-schema classifier", len(go))
    _hardgate.measured("the Python input-schema classifier", len(py))

    code = _hardgate.report(
        "tool input schemas not proto-generated on both sides",
        _stragglers(go, py),
        "Build the factory's advertised input from the tool's strict schema in"
        " every language. A generated tool gets that from the contract, so a"
        " straggler is a hand-built schema standing where generated code belongs.",
    )

    if code == 0:
        _hardgate.say(
            f"input-proto OK: {len(go)} Go and {len(py)} Python tool factories"
            " advertise a proto-generated schema"
        )

    return code


if __name__ == "__main__":
    raise SystemExit(main())
