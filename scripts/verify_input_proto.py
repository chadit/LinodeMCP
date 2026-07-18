#!/usr/bin/env python3
"""Factory-level input-proto completeness gate.

The tool-parity gate (scripts/verify_tool_parity.py) proves the Go and Python
input schemas agree in shape. It does NOT prove where that shape comes from: a
tool can advertise the same schema on both sides while Go generates it from the
proto contract and Python hand-maintains a matching dict (or the reverse). This
gate is the input-schema sibling of read-proto/write-proto: it statically
classifies every tool (all capabilities) on BOTH sides as proto-generated or
hand-built, then ratchets the straggler set down so an input schema cannot stay
hand-built unnoticed.

Two independent classifiers do the static analysis (no factory is executed):

  Go:     go run ./cmd/write-proto-dump -surface input -> {tool: "generated"|"hand"}
          generated = the factory reaches mcp.NewToolWithRawSchema /
          toolschemas.Schema. hand = it builds the schema from mcp.With* options.
  Python: linodemcp.tools._write_proto_classifier classify("input")
          generated = the create_<tool>_tool factory sets inputSchema=schema(...).
          hand = it passes a dict-literal inputSchema.

A tool is a STRAGGLER when either side is not generated. The gate passes iff the
current straggler set is a subset of docs/contracts/input-proto-baseline.txt: a NEW
straggler fails, and a straggler that got fixed must be dropped from the
baseline (the file only shrinks). Regenerate with --update-baseline. The
baseline file doubles as the authoritative remaining-work list for the
input-surface proto conversion.

Run directly, via `make input-proto` (root Makefile), or as a pre-commit hook.
The Go dumper needs the Go toolchain; the Python classifier is imported under
the venv, so this runs under the venv interpreter (the Makefile/hook do that).
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_GO_DIR = _REPO_ROOT / "go"
_PY_SRC = _REPO_ROOT / "python" / "src"

_STRAGGLER_BASELINE = _REPO_ROOT / "docs" / "contracts" / "input-proto-baseline.txt"


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

    return classify("input")


def _stragglers(go: dict[str, str], py: dict[str, str]) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for every straggler.

    A straggler is any tool that is not proto-generated on both sides. The line
    carries both statuses so the baseline documents which side needs work and a
    status flip (hand -> review, say) shows up as a new line.
    """
    lines: list[str] = []

    for tool in sorted(set(go) | set(py)):
        go_status = go.get(tool, "missing")
        py_status = py.get(tool, "missing")

        if go_status != "generated" or py_status != "generated":
            lines.append(f"{tool}\t{go_status}\t{py_status}")

    return lines


def _load_baseline(path: Path) -> set[str]:
    """Read the baseline's entries with "  # accepted ..." annotations stripped."""
    return _baselines.read_entries(path)


_STRAGGLER_HEADER = (
    "# Factory-level input-proto stragglers: tools whose MCP input schema is not\n"
    "# proto-generated on both sides. One line per straggler:\n"
    "# <tool>\\t<go_status>\\t<py_status>. This file is the authoritative\n"
    "# remaining-work list for the input-surface proto conversion. Ratchet:\n"
    "# convert a factory to a proto schema on both sides, then remove its line;\n"
    "# never add a line by hand. Regenerate:\n"
    "#   python scripts/verify_input_proto.py --update-baseline\n"
)


def _say(line: str) -> None:
    """Emit one report line on stdout (gate output, not debug logging)."""
    sys.stdout.write(line + "\n")


def _update_baseline(stragglers: list[str]) -> int:
    """Rewrite the baseline to the current set and report the count."""
    _baselines.write_baseline(
        _STRAGGLER_BASELINE,
        _STRAGGLER_HEADER,
        stragglers,
        _baselines.read_baseline(_STRAGGLER_BASELINE),
    )
    _say(f"baseline updated: {len(stragglers)} input straggler(s)")
    return 0


def _report_drift(current: set[str], baseline: set[str]) -> bool:
    """Report new/fixed drift for the ratchet. Return True when it is clean."""
    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        _say(f"input-proto stragglers OK: {len(baseline)} known, unchanged")
        return True

    if new:
        _say(f"NEW input-proto stragglers ({len(new)}):")
        for line in new:
            _say(f"  {line}")

    if fixed:
        _say(f"\nFIXED input-proto stragglers ({len(fixed)}) - remove these lines:")
        for line in fixed:
            _say(f"  {line}")
        _say("\nRun: python scripts/verify_input_proto.py --update-baseline")

    return False


def main() -> int:
    go = _dump_go()
    py = _dump_python()

    stragglers = _stragglers(go, py)

    if "--update-baseline" in sys.argv:
        return _update_baseline(stragglers)

    ok = _report_drift(set(stragglers), _load_baseline(_STRAGGLER_BASELINE))

    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
