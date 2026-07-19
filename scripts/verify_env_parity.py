#!/usr/bin/env python3
"""Offline gate: every language reads exactly the contracted env vars.

docs/contracts/env-vars.txt pins the complete environment-variable surface.
This gate extracts the env-lookup literals from each implementation's
non-test source (Go: os.Getenv; Python: os.getenv / os.environ) and fails
on any difference in either direction, per language. A variable read by one
language and not another is exactly how the observability env overrides
drifted Go-only, so the check is a hard set equality, not a ratchet.

Lookups are matched across line breaks (calls wrap), and only uppercase
names are considered: lowercase lookups are not configuration surface.

Stdlib only, so no venv is needed. Run via `make env-parity` (in `make check`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

_REPO_ROOT = Path(__file__).resolve().parent.parent
_CONTRACT = _REPO_ROOT / "docs" / "contracts" / "env-vars.txt"

_GO_ROOTS = (
    _REPO_ROOT / "go" / "cmd" / "linodemcp",
    _REPO_ROOT / "go" / "internal",
)
_PYTHON_ROOT = _REPO_ROOT / "python" / "src" / "linodemcp"

_GO_LOOKUP = re.compile(r'os\.Getenv\(\s*"([A-Z][A-Z0-9_]*)"')
_PY_LOOKUPS = (
    re.compile(r'os\.getenv\(\s*"([A-Z][A-Z0-9_]*)"'),
    re.compile(r'os\.environ\.get\(\s*"([A-Z][A-Z0-9_]*)"'),
    re.compile(r'os\.environ\[\s*"([A-Z][A-Z0-9_]*)"'),
)


def contract_vars() -> set[str]:
    """The pinned env surface: non-comment lines of env-vars.txt."""
    entries: set[str] = set()
    for raw in _CONTRACT.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if line and not line.startswith("#"):
            entries.add(line)
    return entries


def go_env_reads() -> set[str]:
    """Env vars the Go binary's non-test source looks up."""
    found: set[str] = set()
    for root in _GO_ROOTS:
        for path in sorted(root.rglob("*.go")):
            if path.name.endswith("_test.go"):
                continue
            found.update(_GO_LOOKUP.findall(path.read_text(encoding="utf-8")))
    return found


def python_env_reads() -> set[str]:
    """Env vars the Python package's source looks up (genpb excluded)."""
    found: set[str] = set()
    for path in sorted(_PYTHON_ROOT.rglob("*.py")):
        if "genpb" in path.parts:
            continue
        text = path.read_text(encoding="utf-8")
        for pattern in _PY_LOOKUPS:
            found.update(pattern.findall(text))
    return found


def env_violations() -> dict[str, tuple[list[str], list[str]]]:
    """Per-language (reads not in contract, contract entries never read)."""
    contract = contract_vars()
    surfaces = {"go": go_env_reads(), "python": python_env_reads()}
    violations: dict[str, tuple[list[str], list[str]]] = {}
    for language, reads in surfaces.items():
        extra = sorted(reads - contract)
        missing = sorted(contract - reads)
        if extra or missing:
            violations[language] = (extra, missing)
    return violations


def main() -> int:
    violations = env_violations()

    for language, (extra, missing) in sorted(violations.items()):
        if extra:
            print(f"{language} reads env vars not in the contract:", file=sys.stderr)
            for name in extra:
                print(f"  {name}", file=sys.stderr)
            print(
                "  (either add the var to docs/contracts/env-vars.txt AND"
                " implement the read in every language, or drop the read)",
                file=sys.stderr,
            )
        if missing:
            print(f"{language} never reads contracted env vars:", file=sys.stderr)
            for name in missing:
                print(f"  {name}", file=sys.stderr)
            print(
                "  (implement the read, or remove the var from every language"
                " and from docs/contracts/env-vars.txt)",
                file=sys.stderr,
            )
    if violations:
        return 1

    print(f"env-parity gate OK: {len(contract_vars())} vars, every language matches")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
