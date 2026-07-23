#!/usr/bin/env python3
"""Offline gate: behavior fixtures must serve the response shape the spec does.

The behavior fixtures under testdata/behavior/ are the shared reference every
registered language's conformance runner is judged against (each language in
docs/contracts/languages.txt consumes the same fixture bodies). That makes a
fixture with the wrong response shape worse than no fixture: both languages
are proven to conform to a contract the API never had, and a cross-language
decode divergence ships with every gate green. The config-interface list
fixture served a page envelope for a bare-array route and did exactly that
(https://github.com/chadit/LinodeMCP-Issue/issues/1057, then
https://github.com/chadit/LinodeMCP-Issue/issues/1058 for the firewall
routes).

The Linode API is the reference. docs/contracts/api-response-shapes-baseline.txt
is the reviewed snapshot of every route's success response shape for every
method (written by the scheduled scripts/verify_sync_response_shapes.py, so
this gate stays hermetic). For each fixture case body whose route and method
can be resolved, the body's shape must match the snapshot:

    envelope  {data, page, pages, results} page object
    array     bare top-level JSON array
    object    any other JSON object

A case body is judged when its route is explicit (an api_responses key or an
expect_request), or when the case has a single api_response and the tool has
exactly one route in docs/contracts/tool-routes.txt. Empty bodies ({} or [])
assert nothing about shape and are skipped, as are expect_error cases (the
harness proves those never reach the API) and routes absent from the
snapshot (the spec lags TechDocs, so absence is not a signal). Cases with
``expect_api_error`` deliberately serve a response that the client must reject,
so they do not assert the route's successful response shape and are skipped.

Known gaps live in docs/contracts/response-shape-baseline.txt, a ratchet: fix
the fixture in a shape-correct way (and every language along with it) and
remove its line; never add a line by hand (regenerate with --update-baseline,
then attach the required acceptance annotation).

Stdlib only, so no venv is needed. Run via `make response-shapes` (in
`make check`, and so the pre-push hook and the CI gate on every branch).

Usage: verify_response_shapes.py [--update-baseline]
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

import _baselines

_REPO_ROOT = Path(__file__).resolve().parents[1]
_SNAPSHOT = _REPO_ROOT / "docs" / "contracts" / "api-response-shapes-baseline.txt"
_BASELINE = _REPO_ROOT / "docs" / "contracts" / "response-shape-baseline.txt"
_ROUTES = _REPO_ROOT / "docs" / "contracts" / "tool-routes.txt"
_FIXTURES = _REPO_ROOT / "testdata" / "behavior"

_ENVELOPE_KEYS = {"data", "page", "pages", "results"}

_BASELINE_HEADER = (
    "# Fixture cases whose served body shape diverges from the route's spec\n"
    "# response shape. Ratchet: correct the fixture body (and every language\n"
    "# implementation that depended on the wrong shape) and remove its line;\n"
    "# never add a line by hand (regenerate instead, then attach the required\n"
    "# annotation).\n"
    "# Regenerate with:\n"
    "#   python3 scripts/verify_response_shapes.py --update-baseline\n"
    "# Spec side comes from docs/contracts/api-response-shapes-baseline.txt\n"
    "# (scripts/verify_sync_response_shapes.py owns that snapshot).\n"
)


def snapshot_shapes(path: Path) -> dict[tuple[str, str], str]:
    """(METHOD, template path) to shape, from the sync snapshot."""
    shapes: dict[tuple[str, str], str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        parts = stripped.split()
        if len(parts) == 3:
            shapes[(parts[0], parts[1])] = parts[2]
    return shapes


def tool_routes(path: Path) -> dict[str, tuple[str, str]]:
    """Tool name to its one (METHOD, template path) from tool-routes.txt."""
    routes: dict[str, tuple[str, str]] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        name, _, rest = stripped.partition(":")
        parts = rest.split()
        if len(parts) == 2:
            routes[name.strip()] = (parts[0], parts[1])
    return routes


def _segments_match(left: str, right: str) -> bool:
    """Two path segments match when either is a placeholder or both are equal."""
    if left.startswith("{") or right.startswith("{"):
        return True
    return left == right


def match_template(
    path: str, method: str, shapes: dict[tuple[str, str], str]
) -> tuple[str, str] | None:
    """Best snapshot key for a path: literal segments beat placeholders.

    The path may itself contain placeholders (tool-routes.txt uses {p}), so
    matching is placeholder-tolerant on both sides.
    """
    path_parts = path.strip("/").split("/")
    best: tuple[str, str] | None = None
    best_score = -1
    for key in shapes:
        if key[0] != method:
            continue
        template_parts = key[1].strip("/").split("/")
        if len(template_parts) != len(path_parts):
            continue
        score = 0
        for path_part, template_part in zip(path_parts, template_parts, strict=True):
            if not _segments_match(path_part, template_part):
                score = -1
                break
            if not template_part.startswith("{") and not path_part.startswith("{"):
                score += 1
        if score > best_score:
            best = key
            best_score = score
    return best if best_score >= 0 else None


def classify_body(body: Any) -> str | None:
    """Shape keyword for a fixture body, or None when it asserts nothing."""
    if isinstance(body, list):
        return "array" if body else None
    if isinstance(body, dict):
        if not body:
            return None
        if _ENVELOPE_KEYS.issubset(body.keys()):
            return "envelope"
        return "object"
    return None


def _case_bodies(
    tool: str,
    case: dict[str, Any],
    routes: dict[str, tuple[str, str]],
) -> list[tuple[str, str, Any]]:
    """(METHOD, path, body) entries this case serves with a resolvable route."""
    if case.get("expect_error") is not None or case.get("expect_api_error") is not None:
        return []

    entries: list[tuple[str, str, Any]] = []
    responses = case.get("api_responses")
    if isinstance(responses, dict):
        for key, body in responses.items():
            method, _, path = str(key).partition(" ")
            if method and path:
                entries.append((method, path.split("?")[0], body))
        return entries

    if "api_response" not in case:
        return []
    body = case["api_response"]

    request = case.get("expect_request")
    if isinstance(request, dict) and request.get("method") and request.get("path"):
        path = str(request["path"]).split("?")[0]
        entries.append((str(request["method"]), path, body))
        return entries

    # A dry-run walk reads sibling GET routes, not the tool's own write
    # route, so the tool-routes fallback would judge the body against the
    # wrong operation.
    if case.get("args", {}).get("dry_run") is True:
        return []

    route = routes.get(tool)
    if route is not None:
        entries.append((route[0], route[1], body))
    return entries


def current_violations() -> list[str]:
    """One entry per fixture case body whose shape diverges from the spec."""
    shapes = snapshot_shapes(_SNAPSHOT)
    routes = tool_routes(_ROUTES)

    violations: set[str] = set()
    for fixture in sorted(_FIXTURES.glob("*.json")):
        doc = json.loads(fixture.read_text(encoding="utf-8"))
        tool = doc.get("tool")
        if not isinstance(tool, str):
            continue
        for case in doc.get("cases", []):
            for method, path, body in _case_bodies(tool, case, routes):
                fixture_shape = classify_body(body)
                if fixture_shape is None:
                    continue
                key = match_template(path, method, shapes)
                if key is None:
                    continue
                spec_shape = shapes[key]
                if spec_shape in {"none", "unknown", fixture_shape}:
                    continue
                violations.add(
                    f"{tool}: {key[0]} {key[1]}"
                    f" fixture={fixture_shape} spec={spec_shape}"
                )
    return sorted(violations)


def main(argv: list[str]) -> int:
    violations = current_violations()

    if "--update-baseline" in argv:
        _baselines.write_baseline(
            _BASELINE, _BASELINE_HEADER, violations, _baselines.read_baseline(_BASELINE)
        )
        print(f"wrote {len(violations)} response-shape gap(s)", file=sys.stderr)
        return 0

    baseline = _baselines.read_entries(_BASELINE)
    new = [entry for entry in violations if entry not in baseline]
    fixed = sorted(baseline - set(violations))

    if new:
        print("fixture bodies diverging from the spec response shape:", file=sys.stderr)
        for entry in new:
            print(f"  {entry}", file=sys.stderr)
        print(
            "\nServe the shape the spec documents (docs/contracts/"
            "api-response-shapes-baseline.txt), fix every language that"
            " depended on the wrong shape, or regenerate the baseline and"
            " annotate the accepted gap.",
            file=sys.stderr,
        )
    if fixed:
        print(
            "response-shape gaps fixed; remove their baseline lines:", file=sys.stderr
        )
        for entry in fixed:
            print(f"  {entry}", file=sys.stderr)
    if new or fixed:
        return 1

    print(
        f"response-shape gate OK: {len(violations)} accepted gap(s),"
        f" no drift vs {_BASELINE.name}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
