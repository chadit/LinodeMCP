#!/usr/bin/env python3
"""Print one coverage line per registered language after a test run.

`make test` runs each language's suite in sequence, and each one reports in
its own format: Go emits per-package percentages across hundreds of lines,
pytest-cov prints a table. Neither leaves a single number you can read at a
glance, and comparing the two means knowing that Go's headline figure counts
generated code the gates exclude. This collapses both into one block:

    Coverage by language
      go      86.3%  (floor 85.0)
      python  87.2%  (floor 85)

Scope comes from docs/contracts/languages.txt, never from a path literal, so
registering a third language surfaces it here automatically. A registered
language with no reader below fails by name rather than being skipped, which
is the point: a silent omission would read as "covered" when nothing measured
it. Add a reader beside the others, or record the language in
COVERAGE_EXEMPT with the reason.

Reads the same artifacts as the coverage-floor gate (go/coverage.out,
python/coverage.json) via _coverage.py, so this never disagrees with the gate
about what "in scope" means. Reporting only: the floor gate owns pass/fail,
and this exits non-zero solely when a language cannot be measured at all.

Stdlib plus scripts/_coverage.py, so no venv is needed.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import _coverage

_ROOT = Path(__file__).resolve().parent.parent
_LANGUAGES = _ROOT / "docs" / "contracts" / "languages.txt"
_FLOORS = _ROOT / "docs" / "contracts" / "coverage-floors.txt"

# Languages deliberately outside coverage reporting, with the reason. Empty
# today; an entry here is a claim that measuring the language is meaningless,
# not that measuring it is inconvenient.
COVERAGE_EXEMPT: dict[str, str] = {}


def _registered_languages() -> list[str]:
    """Return language names from the registry, in file order."""
    names: list[str] = []
    for raw in _LANGUAGES.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        names.append(line.split("\t")[0].strip())
    return names


def _floors() -> dict[str, str]:
    """Return the declared floor per language from the floors contract."""
    floors: dict[str, str] = {}
    for raw in _FLOORS.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) >= 2:
            floors[parts[0]] = parts[1]
    return floors


def _go_percent() -> float | None:
    """Return Go statement coverage over hand-written code, or None."""
    profile = _ROOT / "go" / "coverage.out"
    if not profile.is_file():
        return None
    module = _coverage.go_module_name(_ROOT / "go" / "go.mod")
    blocks = _coverage.parse_go_profile(profile, module, "go")
    covered, total = _coverage.go_statement_totals(blocks)
    return None if not total else covered * 100.0 / total


def _python_percent() -> float | None:
    """Return Python line coverage from the pytest-cov JSON report, or None."""
    report = _ROOT / "python" / "coverage.json"
    if not report.is_file():
        return None
    data = json.loads(report.read_text(encoding="utf-8"))
    percent = data.get("totals", {}).get("percent_covered")
    return None if percent is None else float(percent)


# One reader per registered language. A language in the registry with no
# entry here is an error, not a skip.
READERS = {
    "go": _go_percent,
    "python": _python_percent,
}


def main() -> int:
    """Print the per-language coverage block. Non-zero if unmeasurable."""
    languages = _registered_languages()
    floors = _floors()
    missing: list[str] = []
    unmeasured: list[str] = []
    rows: list[tuple[str, float]] = []

    for name in languages:
        if name in COVERAGE_EXEMPT:
            continue
        reader = READERS.get(name)
        if reader is None:
            missing.append(name)
            continue
        percent = reader()
        if percent is None:
            unmeasured.append(name)
            continue
        rows.append((name, percent))

    width = max((len(name) for name, _ in rows), default=0)
    print("Coverage by language")
    for name, percent in rows:
        floor = floors.get(name)
        suffix = f"  (floor {floor})" if floor else ""
        print(f"  {name.ljust(width)}  {percent:.1f}%{suffix}")

    for name in unmeasured:
        print(
            f"  {name}: no coverage artifact; run this language's test target first",
            file=sys.stderr,
        )
    for name in missing:
        print(
            f"  {name}: registered in languages.txt but report_coverage.py has no "
            f"reader for it. Add one beside the others, or record it in "
            f"COVERAGE_EXEMPT with the reason.",
            file=sys.stderr,
        )

    return 1 if missing or unmeasured else 0


if __name__ == "__main__":
    sys.exit(main())
