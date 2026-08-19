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
COVERAGE. A tool is UNCOVERED when no behavior fixture names it. Coverage is a
HARD rule: every tool in the manifest needs a fixture or a documented line in
docs/contracts/behavior-exempt.txt, and an uncovered tool fails by name with no
baseline to record it in. The exempt file stays the one way out, because the
tools that belong there (local data, no HTTP) are a permanent class rather than
work someone will come back to.

The malformed-response rule is hard for the same reason: a mutating fixture
that decodes an API response body must also prove the tool rejects a badly
shaped one, since decoding is hand-written per language.

One ratchet is left, and it is not empty: docs/contracts/behavior-dryrun-baseline.txt
holds the fixtured mutators with no pinned dry-run preview case. Regenerate it
with --update-baseline.

Run directly, via `make behavior` (root Makefile), or as a pre-commit hook.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

import _baselines
import _hardgate

_REPO_ROOT = Path(__file__).resolve().parents[1]
_BEHAVIOR_DIR = _REPO_ROOT / "testdata" / "behavior"
_MANIFEST = _REPO_ROOT / "docs" / "contracts" / "tools-manifest.txt"
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

_DRYRUN_HEADER = (
    "# Mutating tools (Write/Admin/Destroy) whose behavior fixture lacks a\n"
    "# dry-run preview case: one with dry_run: true in its args and an\n"
    "# expect_result pinning the preview output in both languages. A preview\n"
    "# whose prose the contract does not declare is written per language, so\n"
    "# an unpinned one can drift silently while every other gate stays green;\n"
    "# this ratchet makes each remaining gap visible. Destroy previews are the\n"
    "# hard floor: no Destroy entry may ever be (re)added here. Ratchet: add\n"
    "# the dry-run case (reconciling any preview divergence it exposes), then\n"
    "# remove the line; never add a line by hand. Regenerate:\n"
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

    Scope is fixtured tools only: a tool with no fixture at all fails the
    coverage rule above and is that finding. Every mutating tier counts, since every
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


def _decodes_response_body(cases: list[dict[str, Any]]) -> bool:
    """Report whether any case hands the tool a populated object to decode."""
    return any(
        isinstance(case.get("api_response"), dict) and case["api_response"]
        for case in cases
    )


def _serves_non_object_body(case: dict[str, Any]) -> bool:
    """Report whether the case answers with something other than a JSON object.

    api_response_raw serves exact bytes, which is the only way to reach the
    empty, truncated and trailing-junk bodies; a present-but-non-dict
    api_response serves a well-formed JSON scalar or array. Membership decides
    the second one because JSON null and an absent field both read as None.
    """
    if "api_response_raw" in case:
        return True

    return "api_response" in case and not isinstance(case["api_response"], dict)


def _has_shape_rejection_case(cases: list[dict[str, Any]]) -> bool:
    """Report whether any case serves a malformed body and pins the rejection."""
    return any(
        _serves_non_object_body(case) and case.get("expect_api_error") for case in cases
    )


def _missing_shape_rejection(fixtures: dict[str, list[dict[str, Any]]]) -> set[str]:
    """Return fixtured mutators that decode a body but never reject a bad one.

    Response decoding is hand-written per language, so a handler that accepts
    a wrong-shaped body, or fails on it with different text, diverges with
    nothing to catch it: the input schema, the outgoing request and the
    happy-path result all still match. Scope is Write and Destroy fixtures
    that actually decode a body, since a fixture pinning validation alone has
    no decode path to attack, and a tool with no fixture fails the coverage
    rule instead.
    """
    capabilities = _capabilities()

    return {
        tool
        for tool, cases in fixtures.items()
        if capabilities.get(tool, "") in ("Write", "Destroy")
        and _decodes_response_body(cases)
        and not _has_shape_rejection_case(cases)
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


def _update_baseline(missing_dryrun: set[str]) -> int:
    """Rewrite the dry-run ratchet to the current set and report the count.

    Annotations ("  # accepted ...") on surviving entries are preserved so a
    regeneration cannot silently drop the audit trail the baseline guard
    checks. Coverage and the malformed-response rule have no baseline to
    rewrite: both fail outright.
    """
    _baselines.write_baseline(
        _DRYRUN_BASELINE,
        _DRYRUN_HEADER,
        missing_dryrun,
        _baselines.read_baseline(_DRYRUN_BASELINE),
    )
    _hardgate.say(
        f"baseline updated: {len(missing_dryrun)} without a dry-run preview case"
    )
    return 0


def _report_drift(
    label: str, fix_hint: str, current: set[str], baseline: set[str]
) -> bool:
    """Report new/fixed drift for one ratchet. Return True when it is clean."""
    new = sorted(current - baseline)
    fixed = sorted(baseline - current)

    if not new and not fixed:
        _hardgate.say(f"{label} OK: {len(baseline)} known, unchanged")
        return True

    if new:
        _hardgate.say(f"NEW {label} ({len(new)}) - {fix_hint}:")
        for line in new:
            _hardgate.say(f"  {line}")

    if fixed:
        _hardgate.say(f"\nFIXED {label} ({len(fixed)}) - remove these lines:")
        for line in fixed:
            _hardgate.say(f"  {line}")
        _hardgate.say("\nRun: python scripts/verify_behavior.py --update-baseline")

    return False


def main() -> int:
    fixtures = _load_fixtures()
    covered = set(fixtures)
    manifest = _manifest_tools()
    exempt = _exempt_tools()

    _hardgate.measured("the behavior fixture tree", len(fixtures))
    _hardgate.measured("the tool manifest", len(manifest))

    incomplete = _completeness_failures(fixtures)
    if incomplete:
        _hardgate.say(f"fixtures missing mandatory safety cases ({len(incomplete)}):")
        for line in incomplete:
            _hardgate.say(f"  {line}")
        return 1

    unknown = sorted(covered - manifest)
    if unknown:
        _hardgate.say(f"fixtures name tools not in the manifest ({len(unknown)}):")
        for tool in unknown:
            _hardgate.say(f"  {tool}")
        return 1

    stale_exempt = sorted(exempt - manifest)
    if stale_exempt:
        _hardgate.say(
            f"exemptions name tools not in the manifest ({len(stale_exempt)}):"
        )
        for tool in stale_exempt:
            _hardgate.say(f"  {tool}")
        return 1

    # A fixtured tool must not stay exempt: the exemption would mask a
    # future fixture regression.
    fixtured_exempt = sorted(exempt & covered)
    if fixtured_exempt:
        _hardgate.say(
            f"exempt tools with fixtures ({len(fixtured_exempt)}) - drop exemption:"
        )
        for tool in fixtured_exempt:
            _hardgate.say(f"  {tool}")
        return 1

    uncovered = manifest - covered - exempt
    missing_dryrun = _missing_dryrun(fixtures)
    missing_shape = _missing_shape_rejection(fixtures)

    if "--update-baseline" in sys.argv:
        return _update_baseline(missing_dryrun)

    coverage_code = _hardgate.report(
        "manifest tools with no behavior fixture",
        sorted(uncovered),
        "Add a fixture under testdata/behavior/ exercising the tool's"
        " validation and outgoing request, so every language runner is judged"
        " against the same case. A tool that reaches no API belongs in"
        " docs/contracts/behavior-exempt.txt with its reason instead.",
    )
    shape_code = _hardgate.report(
        "mutating tools with no malformed-response case",
        sorted(missing_shape),
        "Add a case serving a non-object api_response (or an api_response_raw"
        " body) with an expect_api_error pinning the rejection, since decoding"
        " is hand-written per language.",
    )
    dryrun_ok = _report_drift(
        "mutating tools without a dry-run preview case",
        "add a dry_run: true case with expect_result",
        missing_dryrun,
        _baselines.read_entries(_DRYRUN_BASELINE),
    )

    ok = coverage_code == 0 and shape_code == 0 and dryrun_ok
    if ok:
        _hardgate.say(
            f"behavior coverage OK: {len(covered)} fixtured tool(s),"
            f" {len(exempt)} exemption(s) (docs/contracts/behavior-exempt.txt)"
        )

    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
