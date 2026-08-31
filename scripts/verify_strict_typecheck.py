#!/usr/bin/env python3
"""Hard gate: basedpyright strict over every Python tree the repository owns.

Three checkers already ran and still left a hole. `make -C python check` runs
pyright over `python/`, `tools-typecheck` runs mypy over `tools/`, and neither
one ever looked at `scripts/`: the gate scripts that decide whether every other
gate passes were the only Python in the tree nothing type-checked. This runs one
strict checker over all three at once, so a tree cannot be linted-but-unchecked
again.

The scanned surface is `pyrightconfig.json`'s `include` and nothing else. No file
targets are passed on the command line, because a target list here would quietly
outrank the config and leave two declarations of the same surface to drift apart.
Adding a tree means adding it there, where the editor and a bare `basedpyright`
run read it too.

Two ways this could pass while measuring nothing are checked, the same pair
`tools-typecheck` checks. An `include` entry that no longer exists on disk fails
by name rather than being skipped with a warning, since pyright walks on and
reports a clean run over whatever is left. And a run that reports no analyzed
files fails rather than reading as green, which is what an aborted config load or
a crashed checker looks like: no diagnostics, exit code zero-ish, nothing looked
at.

What this cannot see: `include` proves the entries it names exist, not that every
Python tree in the repository is named. A new tree is unchecked until someone
adds it. Strict mode reports unknown types, so it is loud about inference that
gave up, but a `cast()` asserting a shape the data does not have type-checks
clean forever. That is the point of keeping casts to JSON boundaries where the
assumed shape is written down next to them.

Run via `make strict-typecheck` (in `make check`, and so in the pre-push hook and
the CI gate on every branch).

Usage: verify_strict_typecheck.py
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from typing import cast

import _hardgate

_ROOT = Path(__file__).resolve().parent.parent
_CONFIG = _ROOT / "pyrightconfig.json"

# Severities that fail. `information` is basedpyright's advisory level and never
# fails a build; error and warning both name something the checker wants fixed,
# and strict mode raises nearly everything to error anyway.
_FAILING = frozenset({"error", "warning"})

_FIX = (
    "Fix the type basedpyright names, at the annotation rather than with a\n"
    "suppression comment: this gate exists because a silenced finding reads\n"
    "identical to a checked one. A tree that belongs in scope is added to\n"
    "pyrightconfig.json `include`, which is the only place the surface is\n"
    "declared."
)


def _obj(value: object, where: str) -> dict[str, object]:
    """Narrow one JSON value to an object, naming where the shape broke."""
    if not isinstance(value, dict):
        msg = f"{where}: expected a JSON object, got {type(value).__name__}"
        raise SystemExit(msg)

    return cast("dict[str, object]", value)


def _seq(value: object, where: str) -> list[object]:
    """Narrow one JSON value to an array, naming where the shape broke."""
    if not isinstance(value, list):
        msg = f"{where}: expected a JSON array, got {type(value).__name__}"
        raise SystemExit(msg)

    return cast("list[object]", value)


def _text(value: object, where: str) -> str:
    """Narrow one JSON value to a string, naming where the shape broke."""
    if not isinstance(value, str):
        msg = f"{where}: expected a JSON string, got {type(value).__name__}"
        raise SystemExit(msg)

    return value


def _number(value: object, where: str) -> int:
    """Narrow one JSON value to an integer, naming where the shape broke."""
    if not isinstance(value, int) or isinstance(value, bool):
        msg = f"{where}: expected a JSON integer, got {type(value).__name__}"
        raise SystemExit(msg)

    return value


def _included() -> list[str]:
    """The trees pyrightconfig.json declares in scope, in declaration order."""
    declared = _obj(
        json.loads(_CONFIG.read_text(encoding="utf-8")), "pyrightconfig.json"
    )
    entries = _seq(declared.get("include", []), "pyrightconfig.json include")

    return [
        _text(entry, f"pyrightconfig.json include[{index}]")
        for index, entry in enumerate(entries)
    ]


def _missing(includes: list[str]) -> list[str]:
    """Declared trees that are not on disk, so the run silently skips them."""
    return [
        f"pyrightconfig.json include names {entry}, which does not exist, so"
        " basedpyright walks past it and reports a clean run over the rest"
        for entry in includes
        if not (_ROOT / entry).exists()
    ]


def _location(diagnostic: dict[str, object], where: str) -> str:
    """A `file:line:col` anchor, with pyright's 0-based positions made 1-based."""
    path = Path(_text(diagnostic.get("file", ""), f"{where} file"))
    span = _obj(diagnostic.get("range", {}), f"{where} range")
    start = _obj(span.get("start", {}), f"{where} range.start")
    line = _number(start.get("line", 0), f"{where} range.start.line") + 1
    column = _number(start.get("character", 0), f"{where} range.start.character") + 1

    try:
        shown = path.relative_to(_ROOT)
    except ValueError:
        shown = path

    return f"{shown}:{line}:{column}"


def _findings(payload: dict[str, object]) -> list[str]:
    """One line per diagnostic basedpyright wants fixed, ordered as reported."""
    reported = _seq(payload.get("generalDiagnostics", []), "generalDiagnostics")
    lines: list[str] = []
    for index, entry in enumerate(reported):
        where = f"generalDiagnostics[{index}]"
        diagnostic = _obj(entry, where)
        severity = _text(diagnostic.get("severity", ""), f"{where} severity")
        if severity not in _FAILING:
            continue

        rule = diagnostic.get("rule")
        named = f" ({_text(rule, f'{where} rule')})" if rule is not None else ""
        message = _text(diagnostic.get("message", ""), f"{where} message")
        summary = message.splitlines()[0] if message else "(no message)"
        lines.append(f"{_location(diagnostic, where)}: {severity}{named}: {summary}")

    return lines


def _run() -> tuple[dict[str, object] | None, str]:
    """Type-check the declared surface, returning the report or why there is none."""
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "basedpyright",
            "--project",
            str(_ROOT),
            "--outputjson",
        ],
        capture_output=True,
        text=True,
        check=False,
        cwd=_ROOT,
    )
    try:
        payload: object = json.loads(result.stdout)
    except json.JSONDecodeError:
        tail = (result.stderr or result.stdout).strip().splitlines()[-5:]
        joined = " / ".join(line.strip() for line in tail) or "(no output)"

        return None, (
            f"basedpyright exited {result.returncode} without a JSON report,"
            f" so nothing was type-checked: {joined}"
        )

    return _obj(payload, "basedpyright report"), ""


def main() -> int:
    """Type-check every declared tree and report what basedpyright found."""
    includes = _included()
    _hardgate.measured("pyrightconfig.json include surface", len(includes))

    missing = _missing(includes)
    if missing:
        return _hardgate.report("basedpyright scope findings", missing, _FIX)

    payload, failure = _run()
    if payload is None:
        return _hardgate.report("basedpyright run findings", [failure], _FIX)

    # Findings before the coverage measure, because a run that dies partway
    # through covers little AND says why. Checking coverage first would answer
    # "scanned nothing" and swallow the lines naming what stopped it.
    findings = _findings(payload)
    if findings:
        return _hardgate.report("basedpyright strict findings", findings, _FIX)

    summary = _obj(payload.get("summary", {}), "basedpyright report summary")
    analyzed = _number(summary.get("filesAnalyzed", 0), "summary.filesAnalyzed")
    _hardgate.say(
        f"basedpyright strict: {analyzed} source file(s) type-checked across"
        f" {len(includes)} declared tree(s)"
    )
    _hardgate.measured("basedpyright strict type-check", analyzed)

    return 0


if __name__ == "__main__":
    sys.exit(main())
