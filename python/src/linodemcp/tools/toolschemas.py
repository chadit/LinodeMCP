"""Load the MCP input schemas generated from the proto contract.

buf writes the schemas into ``linodemcp/genpb/schemas`` (gitignored); they are
loaded here so each tool factory advertises the same proto-derived schema the
Go side embeds, keeping the two implementations identical.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

_SCHEMA_DIR = Path(__file__).resolve().parent.parent / "genpb" / "schemas"


def schema(full_name: str) -> dict[str, Any]:
    """Return the input JSON Schema for a proto message full name, such as
    ``linode.mcp.v1.InstanceGetInput``."""
    path = _SCHEMA_DIR / f"{full_name}.schema.strict.json"
    loaded: dict[str, Any] = json.loads(path.read_text(encoding="utf-8"))
    return loaded
