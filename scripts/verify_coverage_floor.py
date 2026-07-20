#!/usr/bin/env python3
"""Offline gate: each language's total unit-test coverage meets its floor.

docs/contracts/coverage-floors.txt pins a minimum total statement coverage
per registered language. Floors only RISE, and only because real tests
pushed coverage up (never padding); lowering one is a reviewed human
decision, not a side effect of a failing run.

Enforcement matches how each toolchain reports:

- go: parses go/coverage.out (the go test target writes it on every run)
  and computes the hand-written total. The generated internal/genpb tree
  and the cmd/ entrypoint mains are excluded (see _coverage.py), matching
  the generated-code exclusions fmt and gosec already apply; the raw
  profile total would be dominated by genpb's ~20k never-executed
  generated statements no test should chase.
- python: the floor is enforced at test time by pytest itself
  (--cov-fail-under in pyproject addopts, with its [tool.coverage.run]
  omit list); this gate verifies the pyproject value and the contract
  agree so the two cannot drift apart.

Every language registered in docs/contracts/languages.txt must carry a
floor entry AND an enforcement arm here, so a new language cannot join the
surface with its coverage unwatched.

The aggregate floor is deliberately half the story: a small untested
addition can ride in while the total stays above the floor. The per-line
half is verify_diff_coverage.py (diff-aware, beside baseline-guard).

Stdlib plus scripts/_coverage.py, so no venv is needed. Run via
`make coverage-floor` (in `make check`, after the language suites).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import _coverage

_REPO_ROOT = Path(__file__).resolve().parents[1]
_FLOORS = _REPO_ROOT / "docs" / "contracts" / "coverage-floors.txt"
_LANGUAGES = _REPO_ROOT / "docs" / "contracts" / "languages.txt"
_GO_PROFILE = _REPO_ROOT / "go" / "coverage.out"
_GO_MOD = _REPO_ROOT / "go" / "go.mod"
_PYPROJECT = _REPO_ROOT / "python" / "pyproject.toml"

_FAIL_UNDER = re.compile(r'"--cov-fail-under=(\d+(?:\.\d+)?)"')


def read_floors(path: Path) -> dict[str, float]:
    """Language-to-floor map from the contract file."""
    floors: dict[str, float] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        language, value = stripped.split()
        floors[language] = float(value)
    return floors


def registered_languages(path: Path) -> set[str]:
    """Language names from the languages registry (first tab field)."""
    names: set[str] = set()
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            names.add(stripped.split("\t")[0])
    return names


def go_coverage_percent() -> float:
    """Total hand-written Go statement coverage from go/coverage.out."""
    module = _coverage.go_module_name(_GO_MOD)
    blocks = _coverage.parse_go_profile(_GO_PROFILE, module, "go")
    covered, total = _coverage.go_statement_totals(blocks)
    if total == 0:
        msg = f"{_GO_PROFILE} holds no in-scope statements; profile format changed?"
        raise ValueError(msg)
    return 100.0 * covered / total


def pyproject_fail_under() -> float:
    """The --cov-fail-under value pinned in python/pyproject.toml addopts."""
    match = _FAIL_UNDER.search(_PYPROJECT.read_text(encoding="utf-8"))
    if match is None:
        msg = f"no --cov-fail-under in {_PYPROJECT}; the pytest floor is gone"
        raise ValueError(msg)
    return float(match.group(1))


def floor_violations() -> list[str]:
    """Human-readable floor problems across the registered languages."""
    floors = read_floors(_FLOORS)
    languages = registered_languages(_LANGUAGES)
    problems = [
        f"language {name} is registered but has no floor in {_FLOORS.name};"
        " add one (and an enforcement arm in this script)"
        for name in sorted(languages - set(floors))
    ]
    problems.extend(
        f"floor for {name} has no registered language; remove or fix the entry"
        for name in sorted(set(floors) - languages)
    )
    if problems:
        return problems

    checkers = {"go": _check_go, "python": _check_python}
    for name, floor in sorted(floors.items()):
        check = checkers.get(name)
        if check is None:
            problems.append(
                f"no coverage enforcement implemented for {name};"
                " add its arm to this script before registering it"
            )
            continue
        problems.extend(check(floor))
    return problems


def _check_go(floor: float) -> list[str]:
    """Go arm: measured profile total against the contracted floor."""
    if not _GO_PROFILE.exists():
        return [
            f"{_GO_PROFILE} is missing; run `make go-test` first"
            " (the test target writes it)"
        ]
    percent = go_coverage_percent()
    if percent < floor:
        return [
            f"go coverage {percent:.1f}% is below the contracted floor"
            f" {floor:.1f}%; add tests for the uncovered code"
            " (never lower the floor to pass)"
        ]
    return []


def _check_python(floor: float) -> list[str]:
    """Python arm: pytest's own --cov-fail-under must match the contract."""
    configured = pyproject_fail_under()
    if configured != floor:
        return [
            f"python floor mismatch: contract says {floor:g},"
            f" pyproject --cov-fail-under says {configured:g};"
            " keep the two identical"
        ]
    return []


def main() -> int:
    problems = floor_violations()
    if problems:
        print("coverage floor gate failed:", file=sys.stderr)
        for problem in problems:
            print(f"  {problem}", file=sys.stderr)
        return 1

    floors = read_floors(_FLOORS)
    print(
        f"coverage-floor gate OK: go {go_coverage_percent():.1f}%"
        f" >= {floors['go']:g}%, python enforced at {floors['python']:g}%"
        " by pytest"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
