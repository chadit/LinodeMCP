"""Print the resolved built-in profiles and the tool category table as JSON.

Twin of go/cmd/builtin-parity-dump: both read one tool catalog from stdin and
print the same two halves. The profiles answer what each built-in serves today;
the categories answer what any future category-scoped profile would serve, which
is where the two resolvers drifted apart without any gate seeing it.

scripts/verify_profile_resolution.py runs both and diffs them.

Run as: python -m linodemcp.builtin_parity_dump
"""

from __future__ import annotations

import json
import sys
from typing import Any

from linodemcp.profiles.builtin import (
    ToolDescriptor,
    builtin_catalog_json,
    categories,
)
from linodemcp.profiles.capability import Capability


def read_catalog(raw: str) -> list[ToolDescriptor]:
    """Parse the stdin fixture into descriptors, refusing unknown tiers."""
    entries: list[dict[str, Any]] = json.loads(raw)
    catalog: list[ToolDescriptor] = []

    for entry in entries:
        name = entry["name"]
        tier = entry["capability"]
        capability = Capability.__members__.get(tier)
        if capability is None:
            msg = f"tool {name!r}: unknown capability {tier!r}"
            raise SystemExit(msg)
        catalog.append(ToolDescriptor(name=name, capability=capability))

    return catalog


def dump(catalog: list[ToolDescriptor]) -> str:
    """Render both halves against one catalog."""
    payload = {
        "categories": {tool.name: categories(tool.name) for tool in catalog},
        "profiles": json.loads(builtin_catalog_json(catalog)),
    }
    return json.dumps(payload, sort_keys=True, indent=2)


def main() -> int:
    """Write the resolver dump to stdout as JSON."""
    sys.stdout.write(dump(read_catalog(sys.stdin.read())))
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
