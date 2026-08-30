"""Tests for the StackScripts get route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.gentools import (
    create_linode_stackscript_get_tool,
    handle_linode_stackscript_get,
)
from linodemcp.profiles import Capability

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


def _stackscript_payload(stackscript_id: int = 123) -> dict[str, Any]:
    return {
        "id": stackscript_id,
        "username": "testuser",
        "user_gravatar_id": "abc123",
        "label": "my-script",
        "description": "Test script",
        "images": ["linode/ubuntu22.04"],
        "deployments_total": 10,
        "deployments_active": 5,
        "is_public": False,
        "mine": True,
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
        "script": "#!/bin/bash",
        "user_defined_fields": [],
        "rev_note": "first cut",
    }


def test_create_linode_stackscript_get_tool_schema() -> None:
    """Tool schema exposes the documented StackScript ID path parameter."""
    tool, capability = create_linode_stackscript_get_tool()

    assert tool.name == "linode_stackscript_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["stackscript_id"]
    assert "stackscript_id" in tool.input_schema["properties"]


@pytest.mark.asyncio
async def test_handle_linode_stackscript_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns StackScript details."""
    mock_linode_client.route_raw.return_value = _stackscript_payload(123)

    result = await handle_linode_stackscript_get({"stackscript_id": 123}, sample_config)

    payload = json.loads(result[0].text)
    assert payload == {
        "username": "testuser",
        "user_gravatar_id": "abc123",
        "label": "my-script",
        "description": "Test script",
        "images": ["linode/ubuntu22.04"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
        "script": "#!/bin/bash",
        "user_defined_fields": [],
        "id": 123,
        "deployments_total": 10,
        "deployments_active": 5,
        "is_public": False,
        "mine": True,
        "rev_note": "first cut",
    }
    mock_linode_client.route_raw.assert_awaited_once_with("linode_stackscript_get", 123)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"stackscript_id": ""},
        {"stackscript_id": 0},
        {"stackscript_id": -1},
        {"stackscript_id": True},
        {"stackscript_id": 1.2},
        {"stackscript_id": "123/456"},
        {"stackscript_id": "123?foo=bar"},
        {"stackscript_id": ".."},
    ],
)
async def test_handle_linode_stackscript_get_rejects_invalid_ids(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid StackScript IDs before a client call."""
    result = await handle_linode_stackscript_get(arguments, sample_config)

    assert result[0].text in (
        "Error: stackscript_id is required",
        "Error: stackscript_id must be a positive integer",
    )
    mock_linode_client.route_raw.assert_not_called()
