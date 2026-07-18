#!/usr/bin/env python3
"""Behavioral-conformance coverage gate.

The proto gates prove both languages advertise the same input schema and
serialize the same proto messages, but the handler layer between them
(argument validation, coercion, error text, the HTTP request the handler
builds) is hand-written per language and can drift silently. The behavior
fixtures in testdata/behavior/ close that seam: each fixture case replays the
same arguments through BOTH languages' real dispatch paths (Go
go/internal/server/behavior_conformance_test.go, Python
tests/unit/test_behavior_conformance.py) with the HTTP transport faked, and
both must produce the contracted outcome.

The test runners enforce fixture CORRECTNESS; this gate enforces fixture
COVERAGE. A tool is UNCOVERED when no behavior fixture names it. The gate
passes iff the uncovered set is a subset of docs/contracts/behavior-baseline.txt: a new
tool without fixtures fails, and a tool that gained fixtures must be dropped
from the baseline (the file only shrinks). Regenerate with --update-baseline.

Run directly, via `make behavior` (root Makefile), or as a pre-commit hook.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_BEHAVIOR_DIR = _REPO_ROOT / "testdata" / "behavior"
_MANIFEST = _REPO_ROOT / "docs" / "contracts" / "tools-manifest.txt"
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "behavior-baseline.txt"
_DRYRUN_BASELINE = _REPO_ROOT / "docs" / "contracts" / "behavior-dryrun-baseline.txt"
_EXEMPT = _REPO_ROOT / "docs" / "contracts" / "behavior-exempt.txt"
_CAPABILITIES = _REPO_ROOT / "docs" / "contracts" / "tools-capabilities.txt"

# The destroy bypass gate's signature line; every Destroy tool's fixture must
# pin it so an ungated destroy can never land in either language again.
_DESTROY_GATE_MARK = "is destructive. Either:"

# Every confirm-gate message ends with this; every Write tool's fixture must
# pin a confirm rejection so an unguarded mutator can never land again.
_CONFIRM_MARK = "confirm=true to proceed"

# Write tools whose confirm rejection cannot be pinned, with reasons.
# Currently empty; add entries only with a documented reason.
_CONFIRM_CHECK_SKIP: set[str] = set()

_BASELINE_HEADER = (
    "# Behavior-conformance coverage: tools with no shared behavior fixture in\n"
    "# testdata/behavior/. One line per uncovered tool. Ratchet: add a fixture\n"
    "# file exercising the tool's validation and outgoing request in both\n"
    "# language runners, then remove its line; never add a line by hand.\n"
    "# Regenerate:\n"
    "#   python scripts/verify_behavior.py --update-baseline\n"
)

_DRYRUN_HEADER = (
    "# Mutating tools (Write/Admin/Destroy) whose behavior fixture lacks a\n"
    "# dry-run preview case: one with dry_run: true in its args and an\n"
    "# expect_result pinning the preview output in both languages. Previews\n"
    "# are hand-written per language, so an unpinned preview can drift\n"
    "# silently while every other gate stays green; this ratchet makes each\n"
    "# remaining gap visible. Destroy previews are the hard floor: no Destroy\n"
    "# entry may ever be (re)added here. Ratchet: add the dry-run case\n"
    "# (reconciling any preview divergence it exposes), then remove the line;\n"
    "# never add a line by hand. Regenerate:\n"
    "#   python scripts/verify_behavior.py --update-baseline\n"
)


def _load_fixtures() -> dict[str, list[dict[str, Any]]]:
    """Return {tool: cases} for every behavior fixture."""
    fixtures: dict[str, list[dict[str, Any]]] = {}

    if not _BEHAVIOR_DIR.exists():
        return fixtures

    for path in sorted(_BEHAVIOR_DIR.glob("*.json")):
        fixture = json.loads(path.read_text(encoding="utf-8"))
        tool = fixture.get("tool")
        cases = fixture.get("cases")

        if not isinstance(tool, str) or not tool:
            msg = f"{path.name}: missing or invalid 'tool' field"
            raise SystemExit(msg)

        if not isinstance(cases, list) or not cases:
            msg = f"{path.name}: fixture has no cases"
            raise SystemExit(msg)

        fixtures[tool] = cases

    return fixtures


def _capabilities() -> dict[str, str]:
    """Return {tool: capability} from the canonical capability tags."""
    capabilities: dict[str, str] = {}
    for raw in _CAPABILITIES.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            tool, _, capability = stripped.partition("\t")
            capabilities[tool] = capability

    return capabilities


def _case_expects(cases: list[dict[str, Any]], mark: str) -> bool:
    """Report whether any case's expect_error contains mark."""
    return any(mark in case.get("expect_error", "") for case in cases)


def _completeness_failures(
    fixtures: dict[str, list[dict[str, Any]]],
) -> list[str]:
    """Enforce the safety-case completeness rules per capability tier.

    Coverage alone let 12 ungated Go destroys and 7 unguarded Python
    mutators exist while every gate was green; these rules make the two
    safety cases structurally mandatory, so the runners re-prove the gates
    on every test run in both languages.
    """
    failures: list[str] = []
    capabilities = _capabilities()

    for tool, cases in sorted(fixtures.items()):
        capability = capabilities.get(tool, "")

        if capability == "Destroy" and not _case_expects(cases, _DESTROY_GATE_MARK):
            failures.append(f"{tool}: Destroy fixture lacks the destroy-gate case")

        if (
            capability == "Write"
            and tool not in _CONFIRM_CHECK_SKIP
            and not _case_expects(cases, _CONFIRM_MARK)
        ):
            failures.append(f"{tool}: Write fixture lacks a confirm-rejection case")

    return failures


def _has_dryrun_case(cases: list[dict[str, Any]]) -> bool:
    """Report whether any case previews with dry_run and pins its result."""
    return any(
        case.get("args", {}).get("dry_run") is True and "expect_result" in case
        for case in cases
    )


def _missing_dryrun(fixtures: dict[str, list[dict[str, Any]]]) -> set[str]:
    """Return the fixtured mutating tools with no pinned dry-run preview case.

    Scope is fixtured tools only: a tool with no fixture at all is already
    debt in the coverage baseline. Every mutating tier counts, since every
    mutator advertises dry_run (scripts/verify_dryrun.py pins that) and an
    advertised preview nobody pins can drift between languages unnoticed.
    Destroy entered with a clean slate and stays the hard floor; Write and
    Admin ratchet down from the accepted backlog.
    """
    capabilities = _capabilities()

    return {
        tool
        for tool, cases in fixtures.items()
        if capabilities.get(tool, "") in ("Write", "Admin", "Destroy")
        and not _has_dryrun_case(cases)
    }


def _manifest_tools() -> set[str]:
    """Return the full tool surface from the manifest."""
    tools: set[str] = set()
    for raw in _MANIFEST.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            tools.add(stripped)

    return tools


def _exempt_tools() -> set[str]:
    """Return the documented exemptions (no-HTTP local-data tools).

    Each line is <tool>\\t<reason>; the reason is mandatory documentation but
    only the tool name matters here.
    """
    exempt: set[str] = set()

    if not _EXEMPT.exists():
        return exempt

    for raw in _EXEMPT.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if stripped and not stripped.startswith("#"):
            exempt.add(stripped.split("\t")[0])

    return exempt


def _say(line: str) -> None:
    """Emit one report line on stdout (gate output, not debug logging)."""
    sys.stdout.write(line + "\n")


def _update_baselines(uncovered: set[str], missing_dryrun: set[str]) -> int:
    """Rewrite both baselines to the current sets and report the counts.

    Annotations ("  # accepted ...") on surviving entries are preserved so a
    regeneration cannot silently drop the audit trail the baseline guard
    checks.
    """
    _baselines.write_baseline(
        _BASELINE, _BASELINE_HEADER, uncovered, _baselines.read_baseline(_BASELINE)
    )
    _baselines.write_baseline(
        _DRYRUN_BASELINE,
        _DRYRUN_HEADER,
        missing_dryrun,
        _baselines.read_baseline(_DRYRUN_BASELINE),
    )
    _say(
        f"baselines updated: {len(uncovered)} uncovered tool(s), "
        f"{len(missing_dryrun)} without a dry-run preview case"
    )
    return 0


def _report_drift(
    label: str, fix_hint: str, current: set[str], baseline: set[str]
) -> bool:
    """Report new/fixed drift for one ratchet. Return True when it is clean."""
    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        _say(f"{label} OK: {len(baseline)} known, unchanged")
        return True

    if new:
        _say(f"NEW {label} ({len(new)}) - {fix_hint}:")
        for line in new:
            _say(f"  {line}")

    if fixed:
        _say(f"\nFIXED {label} ({len(fixed)}) - remove these lines:")
        for line in fixed:
            _say(f"  {line}")
        _say("\nRun: python scripts/verify_behavior.py --update-baseline")

    return False


def main() -> int:
    fixtures = _load_fixtures()
    covered = set(fixtures)
    manifest = _manifest_tools()
    exempt = _exempt_tools()

    incomplete = _completeness_failures(fixtures)
    if incomplete:
        _say(f"fixtures missing mandatory safety cases ({len(incomplete)}):")
        for line in incomplete:
            _say(f"  {line}")
        return 1

    unknown = sorted(covered - manifest)
    if unknown:
        _say(f"fixtures name tools not in the manifest ({len(unknown)}):")
        for tool in unknown:
            _say(f"  {tool}")
        return 1

    stale_exempt = sorted(exempt - manifest)
    if stale_exempt:
        _say(f"exemptions name tools not in the manifest ({len(stale_exempt)}):")
        for tool in stale_exempt:
            _say(f"  {tool}")
        return 1

    # A fixtured tool must not stay exempt: the exemption would mask a
    # future fixture regression.
    fixtured_exempt = sorted(exempt & covered)
    if fixtured_exempt:
        _say(f"exempt tools with fixtures ({len(fixtured_exempt)}) - drop exemption:")
        for tool in fixtured_exempt:
            _say(f"  {tool}")
        return 1

    uncovered = manifest - covered - exempt
    missing_dryrun = _missing_dryrun(fixtures)

    if "--update-baseline" in sys.argv:
        return _update_baselines(uncovered, missing_dryrun)

    coverage_ok = _report_drift(
        "uncovered tools",
        "add behavior fixtures",
        uncovered,
        _baselines.read_entries(_BASELINE),
    )
    dryrun_ok = _report_drift(
        "mutating tools without a dry-run preview case",
        "add a dry_run: true case with expect_result",
        missing_dryrun,
        _baselines.read_entries(_DRYRUN_BASELINE),
    )

    ok = coverage_ok and dryrun_ok
    if ok:
        _say(f"behavior exemptions: {len(exempt)} (docs/contracts/behavior-exempt.txt)")

    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
