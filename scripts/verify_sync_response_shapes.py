#!/usr/bin/env python3
"""Live sync gate: watch the JSON shape every Linode API route responds with.

Companion to verify_sync_pagination.py, and like it NOT part of the offline
`make check` (it fetches the live spec). This gate snapshots the success
response shape of every operation on every route, for every method:

    envelope  the paginated {data, page, pages, results} page object
    array     a bare top-level JSON array
    object    any other JSON object with discoverable properties
    none      no JSON success body (204-style responses)
    unknown   a schema that supports no shape judgment (bare oneOf/anyOf)

It fails when a route's shape changes, so the scheduled agent notices when
the API moves a route between shapes, and the offline `make response-shapes`
gate (scripts/verify_response_shapes.py) always judges the behavior fixtures
against a reviewed snapshot instead of a live fetch.

The distinction exists because a wrong shape in a fixture is how a
cross-language decode divergence ships: both language runners are fed the
fixture body, so a fixture that serves an envelope for a bare-array route
proves both implementations conform to a contract the API never had. The
config-interface list route shipped exactly that way; see
https://github.com/chadit/LinodeMCP-Issue/issues/1057 and the firewall
follow-up https://github.com/chadit/LinodeMCP-Issue/issues/1058.

Usage: verify_sync_response_shapes.py [--spec PATH] [--update-baseline]
"""

from __future__ import annotations

import json
import sys
import urllib.request
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
BASELINE = REPO_ROOT / "docs" / "contracts" / "api-response-shapes-baseline.txt"
SPEC_URL = (
    "https://raw.githubusercontent.com/linode/linode-api-openapi/main/openapi.json"
)

_ENVELOPE_KEYS = {"data", "page", "pages", "results"}
_METHODS = ("get", "post", "put", "delete")


def _resolve(doc: dict[str, Any], node: Any) -> Any:
    if isinstance(node, dict) and "$ref" in node:
        cur: Any = doc
        for part in str(node["$ref"]).lstrip("#/").split("/"):
            cur = cur[part]
        return cur
    return node


def _schema_properties(doc: dict[str, Any], schema: Any, depth: int) -> set[str]:
    """Property names of a schema, merged across allOf composition."""
    schema = _resolve(doc, schema)
    if depth > 6 or not isinstance(schema, dict):
        return set()
    names: set[str] = set()
    properties = schema.get("properties")
    if isinstance(properties, dict):
        names.update(str(key) for key in properties)
    for member in schema.get("allOf", []):
        names.update(_schema_properties(doc, member, depth + 1))
    return names


def _success_schema(doc: dict[str, Any], operation: dict[str, Any]) -> Any:
    """The JSON schema of the operation's first 2xx response, or None."""
    responses = operation.get("responses", {})
    for status in sorted(str(code) for code in responses):
        if not status.startswith("2"):
            continue
        response = _resolve(doc, responses[status])
        content = response.get("content", {}).get("application/json", {})
        schema = _resolve(doc, content.get("schema", {}))
        if isinstance(schema, dict) and schema:
            return schema
    return None


def _classify(doc: dict[str, Any], operation: dict[str, Any]) -> str:
    """Shape keyword for the operation's success response."""
    schema = _success_schema(doc, operation)
    if schema is None:
        return "none"
    if schema.get("type") == "array":
        return "array"
    properties = _schema_properties(doc, schema, depth=0)
    if _ENVELOPE_KEYS.issubset(properties):
        return "envelope"
    if properties:
        return "object"
    # A schema with no array type and no discoverable properties (a bare
    # oneOf/anyOf, or an empty schema) supports no shape judgment; a handful
    # of spec quirks model list routes this way, and guessing "object" for
    # them flags correct fixtures.
    return "unknown"


def spec_response_shapes(doc: dict[str, Any]) -> set[str]:
    """One line per operation: method, path, and success response shape."""
    lines: set[str] = set()
    for path, ops in doc.get("paths", {}).items():
        for method in _METHODS:
            operation = ops.get(method)
            if not isinstance(operation, dict):
                continue
            short = str(path).replace("/{apiVersion}", "", 1)
            lines.add(f"{method.upper()} {short} {_classify(doc, operation)}")
    return lines


def load_spec(spec_path: str | None) -> dict[str, Any]:
    if spec_path:
        doc: dict[str, Any] = json.loads(Path(spec_path).read_text(encoding="utf-8"))
        return doc
    with urllib.request.urlopen(SPEC_URL, timeout=60) as resp:  # noqa: S310 - fixed HTTPS URL
        fetched: dict[str, Any] = json.loads(resp.read().decode("utf-8"))
        return fetched


def read_baseline() -> set[str]:
    if not BASELINE.exists():
        return set()
    return {
        line.strip()
        for line in BASELINE.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.startswith("#")
    }


def main(argv: list[str]) -> int:
    spec_path = argv[argv.index("--spec") + 1] if "--spec" in argv else None
    doc = load_spec(spec_path)
    current = spec_response_shapes(doc)

    if "--update-baseline" in argv:
        version = doc.get("info", {}).get("version", "unknown")
        header = (
            f"# Route response-shape snapshot "
            f"(reviewed at linode-api-openapi {version})\n"
            "# One line per operation: METHOD /route shape, where shape is\n"
            "# envelope (paginated data/page/pages/results), array (bare\n"
            "# top-level JSON array), object (any other JSON object), or\n"
            "# none (no JSON success body).\n"
            "# Generated; regenerate with:\n"
            "#   python3 scripts/verify_sync_response_shapes.py --update-baseline\n"
            "# Consumed offline by scripts/verify_response_shapes.py\n"
            "# (make response-shapes).\n"
        )
        BASELINE.parent.mkdir(parents=True, exist_ok=True)
        BASELINE.write_text(
            header + "".join(f"{line}\n" for line in sorted(current)),
            encoding="utf-8",
        )
        print(f"wrote {len(current)} route shape(s)", file=sys.stderr)
        return 0

    baseline = read_baseline()
    added = current - baseline
    removed = baseline - current
    if added or removed:
        print(
            "response-shape drift vs live API (review, then rebaseline):",
            file=sys.stderr,
        )
        for line in sorted(added):
            print(f"  NEW:     {line}", file=sys.stderr)
        for line in sorted(removed):
            print(f"  REMOVED: {line}", file=sys.stderr)
        return 1

    print(f"response-shape sync OK: {len(current)} route shape(s) match the snapshot")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
