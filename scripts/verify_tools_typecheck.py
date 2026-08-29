#!/usr/bin/env python3
"""Hard gate: mypy over every tool project under tools/, at its own Python target.

`make python-check` runs mypy from python/ over src/ and tests/, so nothing type
-checked tools/ at all. `make tools-lint` runs ruff there, which is what made the
tree read as covered: linted but never type-checked is a hole shaped exactly
like the three this batch closed.

The Python target comes from each project's own `requires-python`, never a
literal here and never the shipped package's `python_version`. That matters more
than it sounds. The comparator uses PEP 758 unparenthesized except-tuples, which
are 3.14 syntax; mypy aimed at 3.13 stops at the parse error and checks nothing
in that file. Reading the target from the project keeps the two from drifting
apart again, because a project that raises its floor raises this with it.

Two ways this could pass while measuring nothing are checked. A `tools/` that
holds no project at all fails rather than reporting a clean run. So does a run
that never reports a checked-file count, which is what an aborted parse looks
like: mypy says "errors prevented further checking" and never reaches the files
it was pointed at.

What this gate cannot see: a function declared `-> Any` that returns Any is
invisible to mypy by construction, so a caller misusing its result is not a
finding here and never will be. Honest `Any` at a JSON boundary is fine; an
`Any` standing in for a shape someone did not want to write is not, and no type
checker will tell you which is which.

Run via `make tools-typecheck` (in `make check`, and so in the pre-push hook and
the CI gate on every branch).

Usage: verify_tools_typecheck.py
"""

from __future__ import annotations

import re
import subprocess
import sys
import tomllib
from pathlib import Path

import _hardgate

_ROOT = Path(__file__).resolve().parent.parent

# tools/ holds the projects that ship with neither language package. The
# language registry does not name them, because they are not implementations of
# the tool surface, so this is the one place the directory is spelled.
_TOOLS = _ROOT / "tools"
_MYPY_CONFIG = _ROOT / "python" / "pyproject.toml"

# mypy reports its coverage two ways depending on whether it found anything.
_CHECKED = re.compile(r"(?:no issues found in|checked) (\d+) source files?")
_VERSION = re.compile(r"(\d+)\.(\d+)")

_FIX = (
    "Fix the annotation mypy names. A tool project that needs a different\n"
    "Python target says so in its own pyproject `requires-python`, which this\n"
    "gate reads; do not pin a version here."
)


def _projects() -> list[Path]:
    """Every tool project, found by the pyproject.toml that declares it."""
    if not _TOOLS.is_dir():
        return []

    return sorted(path.parent for path in _TOOLS.glob("*/pyproject.toml"))


def _target_version(project: Path) -> str | None:
    """The X.Y mypy should target, read from the project's requires-python."""
    declared = tomllib.loads((project / "pyproject.toml").read_text(encoding="utf-8"))
    requires = declared.get("project", {}).get("requires-python", "")
    found = _VERSION.search(str(requires))

    return f"{found.group(1)}.{found.group(2)}" if found else None


def _run(project: Path, target: str) -> tuple[int, list[str]]:
    """Type-check one project, returning the files covered and any findings."""
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "mypy",
            "--config-file",
            str(_MYPY_CONFIG),
            "--python-version",
            target,
            str(project),
        ],
        capture_output=True,
        text=True,
        check=False,
        cwd=_ROOT,
    )
    lines = [
        line.rstrip()
        for line in result.stdout.splitlines() + result.stderr.splitlines()
        if line.strip()
    ]
    covered = 0
    for line in lines:
        found = _CHECKED.search(line)
        if found:
            covered = int(found.group(1))

    if result.returncode == 0:
        return covered, []

    return covered, [line for line in lines if ": note:" not in line]


def main() -> int:
    """Type-check every tool project and report what mypy found."""
    projects = _projects()
    _hardgate.measured("mypy tool-project scan", len(projects))

    findings: list[str] = []
    covered = 0
    for project in projects:
        target = _target_version(project)
        if target is None:
            findings.append(
                f"{project.relative_to(_ROOT)}: pyproject declares no requires-python,"
                " so there is no Python target to check it against"
            )
            continue

        checked, reported = _run(project, target)
        covered += checked
        findings.extend(reported)
        _hardgate.say(
            f"{project.relative_to(_ROOT)}: {checked} source file(s) type-checked"
            f" against Python {target}"
        )

    # Findings first. A run that aborts on a parse error covers nothing AND
    # reports why, so checking coverage before findings would answer "scanned
    # nothing" and swallow the line naming the file that stopped it.
    if findings:
        return _hardgate.report("mypy tool-project findings", findings, _FIX)

    _hardgate.measured("mypy type-check", covered)

    return 0


if __name__ == "__main__":
    sys.exit(main())
