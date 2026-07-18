#!/usr/bin/env python3
"""Live sync gate: watch which Linode API GET routes paginate, and their bounds.

Companion to verify_sync_defaults.py, and like it NOT part of the offline `make
check` (it fetches the live spec). This gate snapshots every GET operation that
both takes page/page_size query params and returns the paginated envelope
(data/page/pages/results), together with the documented page_size bounds and
default. It fails when that set or those bounds change, so the scheduled agent
notices when the API adds pagination to a route, drops it, or moves a bound,
and the offline `make pagination` gate (scripts/verify_pagination.py) always
judges the tool surface against a reviewed snapshot instead of a live fetch.

Routes whose GET carries page params but returns a bare object (a handful of
single-resource spec quirks) are excluded on purpose: without the envelope
there is nothing to page through.

Usage: verify_sync_pagination.py [--spec PATH] [--update-baseline]
"""

from __future__ import annotations

import json
import sys
import urllib.request
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
BASELINE = REPO_ROOT / "docs" / "contracts" / "api-pagination-baseline.txt"
SPEC_URL = (
    "https://raw.githubusercontent.com/linode/linode-api-openapi/main/openapi.json"
)

_ENVELOPE_KEYS = {"data", "page", "pages", "results"}


def _resolve(doc: dict[str, Any], node: Any) -> Any:
    if isinstance(node, dict) and "$ref" in node:
        cur: Any = doc
        for part in str(node["$ref"]).lstrip("#/").split("/"):
            cur = cur[part]
        return cur
    return node


def _bound(schema: dict[str, Any], key: str) -> str:
    value = schema.get(key)
    return str(value) if isinstance(value, int) else "-"


def spec_pagination(doc: dict[str, Any]) -> set[str]:
    """One line per paginated GET route: method, path, and page_size bounds."""
    lines: set[str] = set()
    for path, ops in doc.get("paths", {}).items():
        get = ops.get("get")
        if not isinstance(get, dict):
            continue
        params: dict[str, dict[str, Any]] = {}
        for raw in get.get("parameters", []):
            param = _resolve(doc, raw)
            name = param.get("name")
            if param.get("in") == "query" and name in ("page", "page_size"):
                params[str(name)] = _resolve(doc, param.get("schema", {}))
        if "page" not in params or "page_size" not in params:
            continue
        if not _returns_envelope(doc, get):
            continue
        short = str(path).replace("/{apiVersion}", "", 1)
        size = params["page_size"]
        lines.add(
            f"GET {short} page_size={_bound(size, 'minimum')}-{_bound(size, 'maximum')}"
            f" default={_bound(size, 'default')}"
        )
    return lines


def _returns_envelope(doc: dict[str, Any], get: dict[str, Any]) -> bool:
    """True when the 200 response is the paginated data/page/pages/results shape."""
    response = _resolve(doc, get.get("responses", {}).get("200", {}))
    content = response.get("content", {}).get("application/json", {})
    schema = _resolve(doc, content.get("schema", {}))
    return _ENVELOPE_KEYS.issubset(_schema_properties(doc, schema, depth=0))


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
    current = spec_pagination(doc)

    if "--update-baseline" in argv:
        version = doc.get("info", {}).get("version", "unknown")
        header = (
            f"# Paginated GET routes snapshot "
            f"(reviewed at linode-api-openapi {version})\n"
            "# One line per GET route that takes page/page_size and returns the\n"
            "# data/page/pages/results envelope, with the spec's page_size bounds.\n"
            "# Generated; regenerate with:\n"
            "#   python3 scripts/verify_sync_pagination.py --update-baseline\n"
            "# Consumed offline by scripts/verify_pagination.py (make pagination).\n"
        )
        BASELINE.parent.mkdir(parents=True, exist_ok=True)
        BASELINE.write_text(
            header + "".join(f"{line}\n" for line in sorted(current)),
            encoding="utf-8",
        )
        print(f"wrote {len(current)} paginated route(s)", file=sys.stderr)
        return 0

    baseline = read_baseline()
    added = current - baseline
    removed = baseline - current
    if added or removed:
        print(
            "pagination drift vs live API (review, then rebaseline):",
            file=sys.stderr,
        )
        for line in sorted(added):
            print(f"  NEW:     {line}", file=sys.stderr)
        for line in sorted(removed):
            print(f"  REMOVED: {line}", file=sys.stderr)
        return 1

    print(f"pagination sync OK: {len(current)} paginated route(s) match the snapshot")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
