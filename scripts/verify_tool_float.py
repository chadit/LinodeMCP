#!/usr/bin/env python3
"""Offline gate: gate tooling floats at latest; only app dependencies pin.

The repo's convention is that every CI and local gate tool (linters, scanners,
formatters, type checkers) tracks its latest release: `go run tool@latest`,
`npx tool@latest`, and floor-only specifiers in the Python dev group. App
dependencies pin for reproducible builds; the toolchain does not, because a
pinned gate tool silently stops finding new problems and a capped one turns
every tool release into a conflict with the update automation.

The failure mode this guards against has happened more than once: a tool
release ships new diagnostics, a change appears that caps the tool's version
to dodge them, and the cap outlives everyone's memory of why. The fix the
convention wants is the opposite: adopt the release and resolve what it
flags. See the ruff <0.16 cap proposed in
https://github.com/chadit/LinodeMCP/pull/749 for the incident that led to
this gate.

Checks:

1. python/pyproject.toml `dev = [...]` group: every entry is name-only or
   floor-only (`>=`). Caps and pins (`<`, `<=`, `==`, `~=`, `!=`) fail.
   [project.dependencies] is exempt on purpose; app deps must pin.
2. Makefiles, scripts/ci-setup.sh, and workflow run commands: every
   `go run`/`go install`/`npx` tool reference uses `@latest`. Explicitly
   versioned references fail unless the module is in the deliberate-pin
   allowlist below.

A new deliberate pin means adding the module here with its reason, so the
exception list is code review's problem instead of tribal memory.

Stdlib only, so no venv is needed. Run via `make tool-float` (in
`make check`, and so the pre-push hook and the CI gate on every branch).

Usage: verify_tool_float.py
"""

from __future__ import annotations

import re
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parents[1]

# Module path -> reason. The only sanctioned non-latest tool references.
_DELIBERATE_PINS = {
    "github.com/bufbuild/buf/cmd/buf": (
        "codegen determinism: generated proto output must not drift with buf"
    ),
}

_SCAN_FILES = (
    "Makefile",
    "go/Makefile",
    "python/Makefile",
    "scripts/ci-setup.sh",
)

_WORKFLOW_DIR = ".github/workflows"

# A dev-group entry like "ruff>=0.16.0" or "types-PyYAML>=6.0.12". The
# specifier tail is everything after the name/extras.
_DEV_ENTRY = re.compile(r'^\s*"(?P<name>[A-Za-z0-9._\[\]-]+)(?P<spec>[^"]*)",')

# Operators that freeze or cap a tool version.
_PIN_OPERATORS = re.compile(r"==|~=|!=|<=|<")

# go run/go install/npx references carrying an explicit @version.
_AT_VERSION = re.compile(
    r"(?:go\s+(?:run|install)\s+|npx\s+(?:--yes\s+)?)"
    r"(?P<module>[A-Za-z0-9._/@-]+?)@(?P<version>[A-Za-z0-9.+-]+)"
)


def dev_group_violations(pyproject_text: str) -> list[str]:
    """Capped or pinned entries inside the pyproject dev group."""
    violations: list[str] = []
    in_dev = False
    for raw in pyproject_text.splitlines():
        stripped = raw.strip()
        if stripped.startswith("dev = ["):
            in_dev = True
            continue
        if in_dev and stripped.startswith("]"):
            break
        if not in_dev or stripped.startswith("#"):
            continue
        match = _DEV_ENTRY.match(raw)
        if match and _PIN_OPERATORS.search(match.group("spec")):
            violations.append(
                f"python/pyproject.toml dev group: "
                f"{match.group('name')}{match.group('spec')}"
            )
    return violations


def invocation_violations(text: str, label: str) -> list[str]:
    """Non-latest @version tool references outside the deliberate-pin list."""
    violations: list[str] = []
    for number, raw in enumerate(text.splitlines(), start=1):
        stripped = raw.strip()
        if stripped.startswith(("#", "@#")):
            continue
        for match in _AT_VERSION.finditer(raw):
            if match.group("version") == "latest":
                continue
            module = match.group("module")
            if module in _DELIBERATE_PINS:
                continue
            violations.append(f"{label}:{number}: {module}@{match.group('version')}")
    return violations


def current_violations() -> list[str]:
    """All floating-tool violations across the scanned surfaces."""
    violations = dev_group_violations(
        (_REPO_ROOT / "python" / "pyproject.toml").read_text(encoding="utf-8")
    )
    for name in _SCAN_FILES:
        path = _REPO_ROOT / name
        if path.exists():
            violations.extend(
                invocation_violations(path.read_text(encoding="utf-8"), name)
            )
    workflow_dir = _REPO_ROOT / _WORKFLOW_DIR
    if workflow_dir.is_dir():
        for workflow in sorted(workflow_dir.glob("*.yml")):
            violations.extend(
                invocation_violations(
                    workflow.read_text(encoding="utf-8"),
                    f"{_WORKFLOW_DIR}/{workflow.name}",
                )
            )
    return violations


def main() -> int:
    violations = current_violations()
    if violations:
        print("gate tooling must float at latest (app deps pin; tools do not):")
        for violation in violations:
            print(f"  {violation}")
        print(
            "\nAdopt the new tool release and fix what it flags, or add a"
            " deliberate pin with its reason to _DELIBERATE_PINS in"
            " scripts/verify_tool_float.py."
        )
        return 1

    print("tool-float gate OK: dev tools float; deliberate pins are allowlisted")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
