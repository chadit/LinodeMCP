#!/usr/bin/env python3
"""Handler-level read-proto completeness gate.

The write-proto gate (scripts/verify_write_proto.py) covers the mutating
surface. This is its read-surface sibling: the conformance corpus proves a
proto MESSAGE round-trips Go==Python, but not that any HANDLER emits it, so a
Go read handler can emit proto-canonical output while its Python twin curates
a dict, and both pass. This gate statically classifies every READ tool
(capability Read) on BOTH sides as proto-routed or legacy, then ratchets the
straggler set down so a read handler cannot stay or go legacy unnoticed.

Two independent classifiers do the static analysis (no handler is executed):

  Go:     go run ./cmd/write-proto-dump -surface read -> {tool: "proto"|"legacy"}
          proto = the handler reaches MarshalProtoToolResponse.
          legacy = it reaches only MarshalToolResponse.
  Python: linodemcp.tools._write_proto_classifier classify("read")
          proto = the handler reaches serialize_api_response/serialize_list_response.
          legacy = it builds a curated dict with no serialize call.

A tool is a STRAGGLER when either side is not proto. The gate passes iff the
current straggler set is a subset of docs/contracts/read-proto-baseline.txt: a NEW
straggler fails, and a straggler that got fixed must be dropped from the
baseline (the file only shrinks). Regenerate with --update-baseline. The
baseline file doubles as the authoritative remaining-work list for the
read-surface conversion.

Run directly, via `make read-proto` (root Makefile), or as a pre-commit hook.
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

_STRAGGLER_BASELINE = _REPO_ROOT / "docs" / "contracts" / "read-proto-baseline.txt"


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

    return classify("read")


def _stragglers(go: dict[str, str], py: dict[str, str]) -> list[str]:
    """Return sorted "tool\tgo_status\tpy_status" lines for every straggler.

    A straggler is any read tool that is not proto-routed on both sides. The
    line carries both statuses so the baseline documents which side needs work
    and a status flip (legacy -> review, say) shows up as a new line.
    """
    lines: list[str] = []

    for tool in sorted(set(go) | set(py)):
        go_status = go.get(tool, "missing")
        py_status = py.get(tool, "missing")

        if go_status != "proto" or py_status != "proto":
            lines.append(f"{tool}\t{go_status}\t{py_status}")

    return lines


def _load_baseline(path: Path) -> set[str]:
    """Read the baseline's entries with "  # accepted ..." annotations stripped."""
    return _baselines.read_entries(path)


_STRAGGLER_HEADER = (
    "# Handler-level read-proto stragglers: read tools not yet proto-routed on\n"
    "# both sides. One line per straggler: <tool>\\t<go_status>\\t<py_status>.\n"
    "# This file is the authoritative remaining-work list for the read-surface\n"
    "# proto conversion. Ratchet: convert a handler to proto on both sides,\n"
    "# then remove its line; never add a line by hand. Regenerate:\n"
    "#   python scripts/verify_read_proto.py --update-baseline\n"
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
    _say(f"baseline updated: {len(stragglers)} read straggler(s)")
    return 0


def _report_drift(current: set[str], baseline: set[str]) -> bool:
    """Report new/fixed drift for the ratchet. Return True when it is clean."""
    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        _say(f"read-proto stragglers OK: {len(baseline)} known, unchanged")
        return True

    if new:
        _say(f"NEW read-proto stragglers ({len(new)}):")
        for line in new:
            _say(f"  {line}")

    if fixed:
        _say(f"\nFIXED read-proto stragglers ({len(fixed)}) - remove these lines:")
        for line in fixed:
            _say(f"  {line}")
        _say("\nRun: python scripts/verify_read_proto.py --update-baseline")

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
