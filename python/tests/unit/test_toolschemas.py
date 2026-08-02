"""Lookup behavior of the generated MCP input schemas.

Mirrors ``go/internal/toolschemas/schemas_test.go``. Both languages serve the
same generated files to their tool factories and both have to fail the same way
on a name the proto contract does not define: a miss is a build defect, and a
tool registered with a permissive fallback schema would accept anything. Go
panics, Python raises; either way startup stops.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

import linodemcp
from linodemcp.tools.toolschemas import schema

_SCHEMA_DIR = Path(str(linodemcp.__file__)).parent / "genpb" / "schemas"
_SUFFIX = ".schema.strict.json"


def test_every_generated_name_resolves() -> None:
    """Each generated strict schema is served verbatim for its proto name."""
    names = sorted(
        path.name.removesuffix(_SUFFIX) for path in _SCHEMA_DIR.glob(f"*{_SUFFIX}")
    )
    assert names, "expected the generated schema directory to be populated"

    for name in names:
        expected = json.loads((_SCHEMA_DIR / f"{name}{_SUFFIX}").read_text("utf-8"))
        assert schema(name) == expected


def test_tool_input_schemas_describe_objects() -> None:
    """A served schema is a usable MCP input schema, not arbitrary JSON."""
    for name in (
        "linode.mcp.v1.HelloInput",
        "linode.mcp.v1.InstanceGetInput",
        "linode.mcp.v1.AuditHealthInput",
    ):
        assert schema(name)["type"] == "object"


def test_unknown_name_raises() -> None:
    """A name with no generated schema fails loudly instead of falling back."""
    with pytest.raises(FileNotFoundError):
        schema("linode.mcp.v1.NoSuchMessageInput")


def test_empty_name_raises() -> None:
    """The degenerate lookup fails the same way rather than hitting the dir."""
    with pytest.raises(FileNotFoundError):
        schema("")
