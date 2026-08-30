"""Tests for the Longview client update route."""

from __future__ import annotations

from linodemcp.gentools import (
    categories_for,
    create_linode_longview_client_get_tool,
    create_linode_longview_client_update_tool,
    handle_linode_longview_client_get,
    handle_linode_longview_client_update,
    scopes_for,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry


def test_create_linode_longview_client_update_tool_schema() -> None:
    tool, capability = create_linode_longview_client_update_tool()

    assert tool.name == "linode_longview_client_update"
    assert capability is Capability.Write
    assert tool.input_schema["required"] == ["client_id", "label", "confirm"]
    properties = tool.input_schema["properties"]
    assert properties["client_id"]["type"] == "integer"
    assert properties["label"]["type"] == "string"
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    for field in ("api_key", "apps", "created", "id", "install_code", "updated"):
        assert field not in properties


def test_linode_longview_client_update_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_longview_client_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_longview_client_update"
    assert entry.handle_fn is handle_linode_longview_client_update


def test_linode_longview_client_update_profile_metadata() -> None:
    assert categories_for("linode_longview_client_update") == ["longview"]
    assert scopes_for("linode_longview_client_update") == ["longview:read_write"]


def test_create_linode_longview_client_get_tool_schema() -> None:
    tool, capability = create_linode_longview_client_get_tool()

    assert tool.name == "linode_longview_client_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["client_id"]
    assert "client_id" in tool.input_schema["properties"]


def test_linode_longview_client_get_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_longview_client_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_longview_client_get"
    assert entry.handle_fn is handle_linode_longview_client_get


def test_linode_longview_client_get_profile_metadata() -> None:
    assert categories_for("linode_longview_client_get") == ["longview"]
    assert scopes_for("linode_longview_client_get") == ["longview:read_only"]
