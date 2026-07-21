"""Print the Python tool surface as JSON for the cross-language parity gate.

The dump mirrors go/cmd/parity-dump: one record per registered tool with its
name, capability tier, input parameter types, required set, and required
OAuth scopes. Descriptions are excluded because wording may differ across
implementations. Scopes are included because the tool-to-scope mapping is
hand-written per language, so without this field a one-sided scope change
passes every gate.

scripts/verify_tool_parity.py runs every dumper registered in
docs/contracts/languages.txt (this module is the Python entry) and diffs the surfaces,
so the registry import below is the single source of what Python advertises.

Run as: python -m linodemcp.parity_dump
"""

from __future__ import annotations

import json
import sys
from typing import Any, cast

from linodemcp.profiles.scope import required_scopes
from linodemcp.server import get_tool_registry


def dump_records() -> list[dict[str, Any]]:
    """Build the normalized, language-agnostic view of every tool."""
    records: list[dict[str, Any]] = []

    for entry in get_tool_registry():
        schema: dict[str, Any] = entry.tool.inputSchema or {}
        properties: dict[str, Any] = schema.get("properties", {})

        params: dict[str, str] = {}
        for name, prop in properties.items():
            prop_type = ""
            if isinstance(prop, dict):
                raw = cast("dict[str, Any]", prop).get("type", "")
                prop_type = raw if isinstance(raw, str) else ""
            params[name] = prop_type

        records.append(
            {
                "name": entry.name,
                "capability": entry.capability.name,
                "params": params,
                "required": schema.get("required", []),
                "scopes": sorted(
                    str(scope)
                    for scope in required_scopes(entry.name, entry.capability)
                ),
            }
        )

    return sorted(records, key=lambda record: str(record["name"]))


def main() -> int:
    """Write the tool-surface records to stdout as JSON."""
    json.dump(dump_records(), sys.stdout, indent=2)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
