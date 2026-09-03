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

The Linode API is the reference, read through the OpenAPI mirror.
docs/contracts/api-response-shapes-baseline.txt is the reviewed snapshot of
every route's success response shape for every method (written by the
scheduled scripts/verify_sync_response_shapes.py, so this gate stays hermetic).
For each fixture case body whose route and method can be resolved, the body's
shape must match the snapshot:

    envelope  {data, page, pages, results} page object
    array     bare top-level JSON array
    object    any other JSON object

A case body is judged when its route is explicit (an api_responses key or an
expect_request), or when the case has a single api_response and the tool
declares a route in the proto contract. Empty bodies ({} or [])
assert nothing about shape and are skipped. Pre-request expect_error cases
(which never reach the API) and post-request expect_api_error cases (which
deliberately serve a rejected body) are also skipped. Routes absent from the
snapshot are skipped too (the spec lags TechDocs, so absence is not a signal).

This is a HARD gate: any fixture body whose shape contradicts the spec fails by
name. There is no baseline file and no acceptance path, because an accepted
wrong shape is the one state worse than no fixture at all: every language is
proven to conform to a contract the API never had, and they agree with each
other all the way to the wire.

Stdlib plus scripts/_toolroutes.py, which reads the declared routes from the
generated descriptors (through python/.venv/bin/python when the running
interpreter cannot import them). Run via `make response-shapes` (in
`make check`, and so the pre-push hook and the CI gate on every branch).

Authority note: the snapshot this gate judges by comes from the OpenAPI
mirror, which is the secondary source. TechDocs is the API contract's
authority, so a failure a TechDocs page contradicts means the mirror is stale.
This gate has no acceptance path, so the exit is a snapshot refresh through the
scheduled gate's --update-baseline, never a proto edit. docs/gates.md, "Network
sync gates", carries the rule.

Usage: verify_response_shapes.py
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any, NamedTuple, cast

import _hardgate
import _toolroutes

_REPO_ROOT = Path(__file__).resolve().parents[1]
_SNAPSHOT = _REPO_ROOT / "docs" / "contracts" / "api-response-shapes-baseline.txt"
_FIXTURES = _REPO_ROOT / "testdata" / "behavior"

_ENVELOPE_KEYS = {"data", "page", "pages", "results"}


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


def tool_routes() -> dict[str, tuple[str, str]]:
    """Tool name to its one (METHOD, template path) from the proto contract.

    Declared parameter names are normalized off before matching. The snapshot
    names the same parameters the way the spec does, so leaving both names in
    would make the two sides look comparable when they are not; matching is on
    shape, and only shape.
    """
    return {
        tool: (method, _toolroutes.norm_template(path))
        for tool, (method, path) in _toolroutes.routes().items()
    }


def _segments_match(left: str, right: str) -> bool:
    """Two path segments match when either is a placeholder or both are equal."""
    if left.startswith("{") or right.startswith("{"):
        return True
    return left == right


def _segment_score(path_part: str, template_part: str) -> int:
    """How well two matching segments line up.

    Two literals are the strongest match, and two placeholders are the next:
    a declared route's own {p} against a snapshot's {imageId} is the same slot,
    where {p} against the literal "sharegroups" is a different operation the
    placeholder merely tolerates. Without the middle rank those two tie and the
    winner is whichever the snapshot happened to list first, which is how
    /images/{p} resolved to /images/sharegroups.
    """
    path_slot = path_part.startswith("{")
    template_slot = template_part.startswith("{")
    if not path_slot and not template_slot:
        return 2
    if path_slot and template_slot:
        return 1
    return 0


def match_template(
    path: str, method: str, shapes: dict[tuple[str, str], str]
) -> tuple[str, str] | None:
    """Best snapshot key for a path: literal segments beat placeholders.

    The path may itself contain placeholders (a declared route uses {p}), so
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
            score += _segment_score(path_part, template_part)
        if score > best_score:
            best = key
            best_score = score
    return best if best_score >= 0 else None


def _json_object(value: object) -> dict[str, Any] | None:
    """The value as a JSON object, or None when it holds any other shape.

    Fixture documents arrive from json.load untyped, so every nested lookup
    starts as Any. Narrowing here once keeps the rest of the walk typed, and
    JSON guarantees the keys are strings.
    """
    return cast("dict[str, Any]", value) if isinstance(value, dict) else None


def classify_body(body: Any) -> str | None:
    """Shape keyword for a fixture body, or None when it asserts nothing."""
    if isinstance(body, list):
        return "array" if body else None
    obj = _json_object(body)
    if not obj:
        return None
    if _ENVELOPE_KEYS.issubset(obj.keys()):
        return "envelope"
    return "object"


def _case_bodies(
    tool: str,
    case: dict[str, Any],
    routes: dict[str, tuple[str, str]],
) -> list[tuple[str, str, Any]]:
    """(METHOD, path, body) entries this case serves with a resolvable route."""
    if case.get("expect_error") or case.get("expect_api_error"):
        return []

    entries: list[tuple[str, str, Any]] = []
    responses = _json_object(case.get("api_responses"))
    if responses is not None:
        for key, body in responses.items():
            method, _, path = str(key).partition(" ")
            if method and path:
                entries.append((method, path.split("?")[0], body))
        return entries

    if "api_response" not in case:
        return []
    body = case["api_response"]

    request = _json_object(case.get("expect_request"))
    if request is not None and request.get("method") and request.get("path"):
        path = str(request["path"]).split("?")[0]
        entries.append((str(request["method"]), path, body))
        return entries

    # A dry-run walk reads sibling GET routes, not the tool's own write
    # route, so the declared-route fallback would judge the body against the
    # wrong operation.
    if case.get("args", {}).get("dry_run") is True:
        return []

    route = routes.get(tool)
    if route is not None:
        entries.append((route[0], route[1], body))
    return entries


class Judged(NamedTuple):
    """The divergences found, and how many case bodies were judged at all.

    judged is the gate's reach: case bodies whose route resolved into the
    snapshot and whose shape said something. It is reported because a fixture
    tree that moved, a snapshot that stopped parsing, or a contract that
    stopped declaring routes would each leave nothing to judge, and nothing to
    judge reads exactly like nothing wrong.
    """

    violations: list[str]
    judged: int


def current_violations() -> Judged:
    """One entry per fixture case body whose shape diverges from the spec."""
    shapes = snapshot_shapes(_SNAPSHOT)
    routes = tool_routes()

    judged = 0
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
                if spec_shape in {"none", "unknown"}:
                    continue
                judged += 1
                if spec_shape == fixture_shape:
                    continue
                violations.add(
                    f"{tool}: {key[0]} {key[1]}"
                    f" fixture={fixture_shape} spec={spec_shape}"
                )
    return Judged(sorted(violations), judged)


def main(argv: list[str]) -> int:
    del argv  # no options: a hard gate has nothing to record

    result = current_violations()

    _hardgate.measured("the fixture-body shape comparison", result.judged)

    if result.violations:
        print("fixture bodies diverging from the spec response shape:", file=sys.stderr)
        for entry in result.violations:
            print(f"  {entry}", file=sys.stderr)
        print(
            "\nServe the shape the spec documents (docs/contracts/"
            "api-response-shapes-baseline.txt) and fix every language that"
            " depended on the wrong shape.",
            file=sys.stderr,
        )
        return 1

    print(f"response-shape gate OK: {result.judged} fixture body(s) match the spec")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
