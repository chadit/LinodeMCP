#!/usr/bin/env python3
"""Hard gate: no droast error finding in any Dockerfile the repository tracks.

The rule that reached the tree was DF021, a remotely downloaded script executed
without verification: `curl ... | sh` in ci/Dockerfile. Nothing in `make check`
looked at a Dockerfile at all, so the pipe sat there through twenty sealed
batches. This runs the same linter the shared lint script runs, at the same
severity, so a reintroduction fails by rule ID.

What is scanned: every Dockerfile git reports as tracked or untracked-and-not-
ignored, found by name rather than by a path list, so a new image's Dockerfile
is in scope the moment it exists. Both the plain name and the `<thing>.Dockerfile`
spelling are recognized.

Errors fail; warnings and info lines are printed and do not. That split is
droast's own severity model, and it is the one the shared lint script uses.
droast is a hard requirement, not a warn-skip: a machine without it would
otherwise pass a scan CI ran. scripts/ci-setup.sh installs it.

Run via `make dockerfiles` (in `make check`, and so in the pre-push hook and the
CI gate on every branch).

Usage: verify_dockerfiles.py
"""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

import _hardgate

_ROOT = Path(__file__).resolve().parent.parent

_FIX = (
    "Fix the instruction droast names. A `# droast ignore=<rule>` marker needs a\n"
    "reason and an expiry, and it is a last resort: the finding is usually real."
)


def _dockerfiles() -> list[str]:
    """Return the repository-relative path of every Dockerfile git reports."""
    listed = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard"],
        capture_output=True,
        text=True,
        check=True,
        cwd=_ROOT,
    )
    found = [
        path
        for path in listed.stdout.splitlines()
        if Path(path).name == "Dockerfile" or path.endswith(".Dockerfile")
    ]
    return sorted(path for path in found if (_ROOT / path).is_file())


def _errors(lines: list[str]) -> list[str]:
    """Return one line per ERROR droast reported, location first.

    droast prints the whole report, most of which is warnings and info that do
    not fail, and it puts each finding's location on the line below it.
    Reporting all of that buries the entry that failed, so the gate keeps the
    errors and folds each one onto its own location.
    """
    kept: list[str] = []
    pending = ""
    for raw in lines:
        line = raw.strip()
        if line.startswith("ERROR ["):
            pending = line.removeprefix("ERROR ").replace("]  ", "] ", 1)
            continue
        if pending and line.startswith("at "):
            kept.append(f"{line.removeprefix('at ')}: {pending}")
        pending = ""

    return kept


def main() -> int:
    """Run droast over every tracked Dockerfile and report what it found."""
    if shutil.which("droast") is None:
        raise SystemExit(
            "droast is required (release binary:"
            " https://github.com/immanuwell/dockerfile-roast/releases,"
            " or run scripts/ci-setup.sh)"
        )

    dockerfiles = _dockerfiles()
    _hardgate.measured("droast Dockerfile scan", len(dockerfiles))

    result = subprocess.run(
        ["droast", "--no-roast", "--fail-on", "error", *dockerfiles],
        capture_output=True,
        text=True,
        check=False,
        cwd=_ROOT,
    )
    _hardgate.say(f"scanned {len(dockerfiles)} Dockerfile(s) with droast")
    if result.returncode == 0:
        return 0

    findings = _errors(result.stdout.splitlines() + result.stderr.splitlines())

    return _hardgate.report("droast Dockerfile errors", findings, _FIX)


if __name__ == "__main__":
    sys.exit(main())
