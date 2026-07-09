#!/usr/bin/env python3
"""Cross-language confirm-message parity gate.

The behavior fixtures pin the confirm messages someone wrote a case for; this
gate diffs EVERY extractable confirm-gate message across both languages, so a
text drift on a branch no fixture exercises still fails. The extraction is
heuristic (scripts/_msg_extract_go.py / _msg_extract_py.py, promoted from the
P1 sweep's tooling): it pairs each tool with the message its confirm gate
emits, and the gate compares the intersection of tools both extractors could
resolve. Tools only one side resolves are a coverage note, not a failure (the
heuristics have documented blind spots, e.g. messages built in helpers).

The gate passes iff the divergence set is a subset of
docs/message-parity-baseline.txt (expected empty after the P1 sweep): a NEW
divergence fails, and a fixed one must be dropped (the file only shrinks).
Regenerate with --update-baseline.

Run directly, via `make messages` (root Makefile), or as a pre-commit hook.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parents[1]
_SCRIPTS = _REPO_ROOT / "scripts"
_BASELINE = _REPO_ROOT / "docs" / "message-parity-baseline.txt"

_BASELINE_HEADER = (
    "# Cross-language confirm-message divergences the extractors can see.\n"
    "# One line per divergent tool: <tool>\\t<go_text>\\t<py_text>. Ratchet:\n"
    "# align the Python text to Go's, then remove the line; never add a line\n"
    "# by hand. Regenerate:\n"
    "#   python scripts/verify_messages.py --update-baseline\n"
)


def _extract(script: str, *args: str) -> dict[str, str]:
    """Run one extractor and parse its JSON stdout."""
    result = subprocess.run(
        [sys.executable, str(_SCRIPTS / script), *args],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr)
        msg = f"{script} failed (exit {result.returncode})"
        raise SystemExit(msg)

    parsed: dict[str, str] = json.loads(result.stdout)
    return parsed


def _divergences() -> list[str]:
    """Return sorted tab-separated divergence lines over the intersection."""
    go_map = _extract(
        "_msg_extract_go.py",
        str(_REPO_ROOT / "go" / "internal" / "tools"),
        str(_REPO_ROOT / "docs" / "tools-manifest.txt"),
    )
    py_map = _extract(
        "_msg_extract_py.py",
        str(_REPO_ROOT / "python" / "src" / "linodemcp" / "tools"),
    )

    lines: list[str] = []
    for tool in sorted(set(go_map) & set(py_map)):
        if go_map[tool] != py_map[tool]:
            lines.append(f"{tool}\t{go_map[tool]}\t{py_map[tool]}")

    return lines


def _load_baseline() -> set[str]:
    """Read the ratchet baseline, skipping comments and blanks."""
    if not _BASELINE.exists():
        return set()

    entries: set[str] = set()
    for raw in _BASELINE.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            entries.add(stripped)

    return entries


def _say(line: str) -> None:
    """Emit one report line on stdout (gate output, not debug logging)."""
    sys.stdout.write(line + "\n")


def main() -> int:
    divergences = _divergences()

    if "--update-baseline" in sys.argv:
        _BASELINE.write_text(
            _BASELINE_HEADER + "\n".join(divergences) + "\n", encoding="utf-8"
        )
        _say(f"baseline updated: {len(divergences)} divergence(s)")
        return 0

    baseline = _load_baseline()
    current = set(divergences)
    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        _say(f"message parity OK: {len(baseline)} known divergence(s), unchanged")
        return 0

    if new:
        _say(f"NEW confirm-message divergences ({len(new)}):")
        for line in new:
            _say(f"  {line}")

    if fixed:
        _say(f"\nFIXED divergences ({len(fixed)}) - remove these lines:")
        for line in fixed:
            _say(f"  {line}")
        _say("\nRun: python scripts/verify_messages.py --update-baseline")

    return 1


if __name__ == "__main__":
    raise SystemExit(main())
