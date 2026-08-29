#!/usr/bin/env python3
"""Hard gate: no buf lint finding anywhere in the proto contract.

The contract is the source every tool in every language is generated from, so a
lint finding on it is a finding on the whole surface. `make proto` runs `buf
generate`, which does not lint, and nothing else in `make check` ran buf at all:
four findings sat in the tree through twenty sealed batches.

What is scanned: whatever buf.yaml's modules declare, which is why this gate
carries no path of its own. Rule selection and per-path exemptions belong in
buf.yaml too, where `buf lint` on a developer's machine reads the same config
this does.

This stays offline. Every import the contract makes resolves inside proto/, the
workspace declares no `deps`, and there is no buf.lock, so `buf lint` never
reaches the Buf Schema Registry. `make proto` is the one target that does.

buf is a hard requirement, not a warn-skip: a machine without it would otherwise
pass a scan CI ran. scripts/ci-setup.sh installs it, pinned.

Run via `make proto-lint` (in `make check`, and so in the pre-push hook and the
CI gate on every branch).

Usage: verify_proto_lint.py
"""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

import _hardgate

_ROOT = Path(__file__).resolve().parent.parent

_FIX = (
    "Fix the declaration buf names. When the file is vendored third-party proto\n"
    "nobody here owns, exempt its directory under `lint.ignore` in buf.yaml\n"
    "instead, which leaves generation reading it."
)


def _module_paths() -> list[str]:
    """Return the module paths buf.yaml declares, for the scanned-nothing check.

    Read as text rather than as YAML: the gates run on the plain interpreter,
    which carries no YAML parser, and one key off a two-space indent is all this
    needs to know whether the workspace declares a module at all.
    """
    paths: list[str] = []
    in_modules = False
    for raw in (_ROOT / "buf.yaml").read_text(encoding="utf-8").splitlines():
        if not raw.startswith((" ", "\t", "-")):
            in_modules = raw.strip() == "modules:"
            continue
        if in_modules and raw.strip().startswith("- path:"):
            paths.append(raw.split(":", 1)[1].strip())

    return paths


def main() -> int:
    """Run buf lint over the declared modules and report what it found."""
    if shutil.which("buf") is None:
        raise SystemExit(
            "buf is required (https://buf.build/docs/installation, or run"
            " scripts/ci-setup.sh)"
        )

    modules = _module_paths()
    _hardgate.measured("buf lint module scan", len(modules))

    result = subprocess.run(
        ["buf", "lint"],
        capture_output=True,
        text=True,
        check=False,
        cwd=_ROOT,
    )
    _hardgate.say(f"linted {len(modules)} proto module(s): {', '.join(modules)}")
    if result.returncode == 0:
        return 0

    findings = [
        line.rstrip()
        for line in result.stdout.splitlines() + result.stderr.splitlines()
        if line.strip()
    ]

    return _hardgate.report("buf lint findings", findings, _FIX)


if __name__ == "__main__":
    sys.exit(main())
