"""Tests for the StackScript update route."""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import AsyncMock

import httpx
import pytest

from linodemcp.linode import Client, NetworkError, RetryableClient, StackScript
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_stackscripts import (
    create_linode_stackscript_update_tool,
    handle_linode_stackscript_update,
)
from linodemcp.version import FEATURE_TOOLS_LIST


def _stackscript() -> StackScript:
    return StackScript(
        id=123,
        label="updated-script",
        username="test-user",
        description="updated description",
        images=["linode/debian12"],
        is_public=False,
        mine=True,
        deployments_total=1,
        deployments_active=0,
        created="2026-01-01T00:00:00",
        updated="2026-01-02T00:00:00",
        user_gravatar_id="abc123",
        script="#!/bin/bash\necho ok",
        user_defined_fields=[],
    )


def _stackscript_json() -> dict[str, Any]:
    return {
        "id": 123,
        "label": "updated-script",
        "username": "test-user",
        "description": "updated description",
        "images": ["linode/debian12"],
        "is_public": False,
        "mine": True,
        "deployments_total": 1,
        "deployments_active": 0,
        "created": "2026-01-01T00:00:00",
        "updated": "2026-01-02T00:00:00",
        "user_gravatar_id": "abc123",
        "script": "#!/bin/bash\necho ok",
        "user_defined_fields": [],
    }


@pytest.mark.asyncio
async def test_client_update_stackscript_sends_exact_path_and_body() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json=_stackscript_json())

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        result = await client.update_stackscript(
            123,
            label="updated-script",
            images=["linode/debian12"],
            script="#!/bin/bash\necho ok",
            description="updated description",
            is_public=False,
            rev_note="route test",
        )
    finally:
        await client.close()

    assert result.label == "updated-script"
    assert len(seen) == 1
    request = seen[0]
    assert request.method == "PUT"
    assert request.url.path == "/v4/linode/stackscripts/123"
    assert request.url.query == b""
    assert request.headers["Authorization"] == "Bearer test-token"
    assert json.loads(request.content) == {
        "label": "updated-script",
        "images": ["linode/debian12"],
        "script": "#!/bin/bash\necho ok",
        "description": "updated description",
        "is_public": False,
        "rev_note": "route test",
    }


@pytest.mark.asyncio
@pytest.mark.parametrize("stackscript_id", ["1/2", "1?x=2", "..", 0, -1, True])
async def test_client_update_stackscript_rejects_invalid_stackscript_id(
    stackscript_id: Any,
) -> None:
    called = False

    def handler(request: httpx.Request) -> httpx.Response:
        nonlocal called
        called = True
        return httpx.Response(200, json={})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(
            ValueError, match="stackscript_id must be a positive integer"
        ):
            await client.update_stackscript(stackscript_id, label="updated-script")
    finally:
        await client.close()

    assert called is False


@pytest.mark.asyncio
async def test_client_update_stackscript_translates_http_errors() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("timeout")

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(NetworkError, match="UpdateStackScript"):
            await client.update_stackscript(123, label="updated-script")
    finally:
        await client.close()


@pytest.mark.asyncio
async def test_retryable_client_update_stackscript_does_not_replay_put() -> None:
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    network_error = NetworkError("UpdateStackScript", httpx.ConnectTimeout("boom"))
    mock_update = AsyncMock(side_effect=network_error)
    object.__setattr__(retryable.client, "update_stackscript", mock_update)

    try:
        with pytest.raises(NetworkError):
            await retryable.update_stackscript(123, label="updated-script")
    finally:
        await retryable.close()

    mock_update.assert_awaited_once_with(
        123,
        label="updated-script",
        images=None,
        script=None,
        description=None,
        is_public=None,
        rev_note=None,
    )


def test_create_linode_stackscript_update_tool_schema() -> None:
    tool, capability = create_linode_stackscript_update_tool()

    assert tool.name == "linode_stackscript_update"
    assert capability is Capability.Write
    assert tool.inputSchema["required"] == ["stackscript_id", "confirm"]
    properties = tool.inputSchema["properties"]
    assert properties["stackscript_id"]["type"] == "integer"
    assert properties["label"]["type"] == "string"
    assert properties["images"]["type"] == "array"
    assert properties["script"]["type"] == "string"
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_stackscript_update_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_stackscript.return_value = _stackscript()

    result = await handle_linode_stackscript_update(
        {
            "stackscript_id": 123,
            "label": "updated-script",
            "images": ["linode/debian12"],
            "script": "#!/bin/bash\necho ok",
            "description": "updated description",
            "is_public": False,
            "rev_note": "route test",
            "confirm": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["message"] == (
        "StackScript 'updated-script' (ID: 123) updated successfully"
    )
    assert payload["stackscript"]["id"] == 123
    mock_linode_client.update_stackscript.assert_awaited_once_with(
        123,
        label="updated-script",
        images=["linode/debian12"],
        script="#!/bin/bash\necho ok",
        description="updated description",
        is_public=False,
        rev_note="route test",
    )


@pytest.mark.asyncio
async def test_handle_linode_stackscript_update_dry_run_skips_client(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_stackscript_update(
        {
            "stackscript_id": 123,
            "label": "updated-script",
            "confirm": True,
            "dry_run": True,
        },
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload["dry_run"] is True
    assert payload["would_execute"] == {
        "method": "PUT",
        "path": "/linode/stackscripts/123",
        "body": {"label": "updated-script"},
    }
    assert len(payload["side_effects"]) == 1
    mock_linode_client.update_stackscript.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm_value", [None, False, "true", 1])
async def test_handle_linode_stackscript_update_rejects_non_true_confirm(
    confirm_value: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    arguments: dict[str, Any] = {"stackscript_id": 123, "label": "updated-script"}
    if confirm_value is not None:
        arguments["confirm"] = confirm_value

    result = await handle_linode_stackscript_update(arguments, sample_config)

    assert result[0].text.startswith("Error: This updates a StackScript")
    mock_linode_client.update_stackscript.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"label": "updated-script", "confirm": True},
        {"stackscript_id": "1/2", "label": "updated-script", "confirm": True},
        {"stackscript_id": "1?x=2", "label": "updated-script", "confirm": True},
        {"stackscript_id": "..", "label": "updated-script", "confirm": True},
        {"stackscript_id": 0, "label": "updated-script", "confirm": True},
        {"stackscript_id": True, "label": "updated-script", "confirm": True},
        {"stackscript_id": 123, "images": [], "confirm": True},
        {"stackscript_id": 123, "images": [""], "confirm": True},
        {"stackscript_id": 123, "label": 123, "confirm": True},
        {"stackscript_id": 123, "script": 123, "confirm": True},
        {"stackscript_id": 123, "is_public": "false", "confirm": True},
    ],
)
async def test_handle_linode_stackscript_update_rejects_invalid_arguments(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    result = await handle_linode_stackscript_update(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.update_stackscript.assert_not_called()


@pytest.mark.asyncio
async def test_handle_linode_stackscript_update_reports_client_errors(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    mock_linode_client.update_stackscript.side_effect = NetworkError(
        "UpdateStackScript", httpx.ConnectTimeout("boom")
    )

    result = await handle_linode_stackscript_update(
        {"stackscript_id": 123, "label": "updated-script", "confirm": True},
        sample_config,
    )

    assert result[0].text.startswith("Failed to update StackScript: ")
    mock_linode_client.update_stackscript.assert_awaited_once()


def test_linode_stackscript_update_registered() -> None:
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_stackscript_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_stackscript_update"
    assert entry.handle_fn is handle_linode_stackscript_update


def test_linode_stackscript_update_in_version_features() -> None:
    assert "linode_stackscript_update" in FEATURE_TOOLS_LIST.split(",")
