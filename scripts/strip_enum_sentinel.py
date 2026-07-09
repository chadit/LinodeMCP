#!/usr/bin/env python3
"""Strip the proto zero-value enum sentinel from generated JSON Schemas.

Proto3 requires every enum to have a zero value, so each MCP enum defines an
`unspecified = 0` sentinel (see proto/linode/mcp/v1/*.proto). The
protoschema-jsonschema plugin renders that sentinel into the schema's `enum`
array alongside the real Linode API values, which would advertise
`"unspecified"` to MCP clients as a selectable option. This post-generation
step removes it so the generated schema lists only real API values.

It runs inside the `proto` make target after `buf generate`, over both schema
output dirs (Go embed + Python load), so the Go and Python schemas stay
byte-identical (the tool-parity gate depends on that). buf emits
`json.dumps(obj, indent=2, sort_keys=True) + "\n"`, so this rewrites in the same
format; a schema with no sentinel is left byte-for-byte unchanged.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any, cast

SENTINEL = "unspecified"

# Schema dirs, relative to the repo root (this script's parent's parent).
DEFAULT_DIRS = (
    "go/internal/toolschemas/data",
    "python/src/linodemcp/genpb/schemas",
)


def _strip(node: object) -> bool:
    """Remove the sentinel from every `enum` array under node.

    Returns True if anything was removed. Recurses through dicts and lists.
    """
    changed = False
    if isinstance(node, dict):
        node_dict = cast("dict[str, Any]", node)
        enum = node_dict.get("enum")
        if isinstance(enum, list) and SENTINEL in enum:
            enum_list = cast("list[Any]", enum)
            node_dict["enum"] = [v for v in enum_list if v != SENTINEL]
            changed = True
        for value in node_dict.values():
            changed = _strip(value) or changed
    elif isinstance(node, list):
        for value in cast("list[Any]", node):
            changed = _strip(value) or changed
    return changed


def _process_file(path: Path) -> bool:
    """Strip the sentinel from one schema file, rewriting only if it changed."""
    obj = json.loads(path.read_text(encoding="utf-8"))
    if not _strip(obj):
        return False
    path.write_text(
        json.dumps(obj, indent=2, sort_keys=True, ensure_ascii=False) + "\n",
        encoding="utf-8",
    )
    return True


def main(argv: list[str]) -> int:
    """Strip the sentinel across the given (or default) schema dirs."""
    repo_root = Path(__file__).resolve().parent.parent
    dirs = argv[1:] or [str(repo_root / d) for d in DEFAULT_DIRS]

    stripped = 0
    for directory in dirs:
        base = Path(directory)
        if not base.is_dir():
            print(f"W: schema dir not found, skipping: {base}", file=sys.stderr)
            continue
        for path in sorted(base.glob("*.json")):
            if _process_file(path):
                stripped += 1

    print(f"strip_enum_sentinel: rewrote {stripped} schema file(s)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
