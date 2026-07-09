#!/usr/bin/env python3
"""Live sync gate: watch the Linode API's documented wire-body defaults for drift.

Companion to verify_sync_enums.py, and like it NOT part of the offline `make
check` (it fetches the live spec). The repo's rule is strip-and-defer: neither
language injects a request-body default, so the API owns every default. This gate
snapshots the set of defaults the API documents and fails when that set changes,
so the scheduled agent notices when the API adds, removes, or changes a default
and can decide whether the strip-and-defer behavior still matches.

The code side (that both languages omit the field and produce identical bodies)
is already pinned by the behavior fixtures; this gate only watches the API side.

Usage: verify_sync_defaults.py [--spec PATH] [--update-baseline]
"""

from __future__ import annotations

import json
import sys
import urllib.request
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
BASELINE = REPO_ROOT / "docs" / "api-defaults-baseline.txt"
SPEC_URL = (
    "https://raw.githubusercontent.com/linode/linode-api-openapi/main/openapi.json"
)


def _resolve(doc: dict[str, Any], ref: str) -> Any:
    node: Any = doc
    for part in ref.lstrip("#/").split("/"):
        node = node[part]
    return node


def _walk(
    doc: dict[str, Any], schema: Any, out: set[str], depth: int, prop: str | None
) -> None:
    if depth > 12 or not isinstance(schema, dict):
        return
    ref = schema.get("$ref")
    if isinstance(ref, str):
        _walk(doc, _resolve(doc, ref), out, depth + 1, prop)
        return
    if prop is not None and "default" in schema and "properties" not in schema:
        out.add(f"{prop} = {json.dumps(schema['default'], sort_keys=True)}")
    for comb in ("oneOf", "anyOf", "allOf"):
        for sub in schema.get(comb, []):
            _walk(doc, sub, out, depth + 1, prop)
    props = schema.get("properties")
    if isinstance(props, dict):
        for name, sub in props.items():
            _walk(doc, sub, out, depth + 1, name)
    item = schema.get("items")
    if isinstance(item, dict):
        _walk(doc, item, out, depth + 1, prop)


def spec_defaults(doc: dict[str, Any]) -> set[str]:
    """Collect every '<field> = <default>' the API documents on a request body."""
    out: set[str] = set()
    for _, ops in doc.get("paths", {}).items():
        for method, op in ops.items():
            if method not in ("post", "put", "patch") or not isinstance(op, dict):
                continue
            body = op.get("requestBody")
            if isinstance(body, dict):
                for media in body.get("content", {}).values():
                    if media.get("schema"):
                        _walk(doc, media["schema"], out, 0, None)
    return out


def load_spec(spec_path: str | None) -> dict[str, Any]:
    if spec_path:
        return json.loads(Path(spec_path).read_text(encoding="utf-8"))
    with urllib.request.urlopen(SPEC_URL, timeout=60) as resp:  # noqa: S310 - fixed HTTPS URL
        return json.loads(resp.read().decode("utf-8"))


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
    current = spec_defaults(doc)

    if "--update-baseline" in argv:
        header = (
            "# API wire-body defaults snapshot (reviewed at linode-api-openapi %s)\n"
            % (doc.get("info", {}).get("version", "unknown"))
        )
        BASELINE.parent.mkdir(parents=True, exist_ok=True)
        BASELINE.write_text(
            header + "".join(f"{d}\n" for d in sorted(current)), encoding="utf-8"
        )
        print(f"wrote {len(current)} default(s)", file=sys.stderr)
        return 0

    baseline = read_baseline()
    added = current - baseline
    removed = baseline - current
    if added or removed:
        print(
            "API default drift (review whether strip-and-defer still matches, then rebaseline):",
            file=sys.stderr,
        )
        for d in sorted(added):
            print(f"  ADDED   {d}", file=sys.stderr)
        for d in sorted(removed):
            print(f"  REMOVED {d}", file=sys.stderr)
        return 1
    print(f"sync-defaults OK: {len(current)} API default(s) unchanged", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
