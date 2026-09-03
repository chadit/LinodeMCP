"""Tests for the image share groups route."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.gentools import (
    create_linode_image_delete_tool,
    create_linode_image_sharegroup_by_image_list_tool,
    create_linode_image_sharegroup_by_token_get_tool,
    create_linode_image_sharegroup_image_delete_tool,
    create_linode_image_sharegroup_image_list_tool,
    create_linode_image_sharegroup_list_tool,
    create_linode_image_sharegroup_member_list_tool,
    create_linode_image_sharegroup_member_token_delete_tool,
    create_linode_image_sharegroup_member_token_get_tool,
    create_linode_image_sharegroup_token_delete_tool,
    create_linode_image_sharegroup_token_get_tool,
    create_linode_image_sharegroup_token_image_list_tool,
    create_linode_image_sharegroup_token_list_tool,
    handle_linode_image_delete,
    handle_linode_image_sharegroup_by_image_list,
    handle_linode_image_sharegroup_by_token_get,
    handle_linode_image_sharegroup_image_delete,
    handle_linode_image_sharegroup_image_list,
    handle_linode_image_sharegroup_image_update,
    handle_linode_image_sharegroup_list,
    handle_linode_image_sharegroup_member_list,
    handle_linode_image_sharegroup_member_token_delete,
    handle_linode_image_sharegroup_member_token_get,
    handle_linode_image_sharegroup_token_delete,
    handle_linode_image_sharegroup_token_get,
    handle_linode_image_sharegroup_token_image_list,
    handle_linode_image_sharegroup_token_list,
    scopes_for,
)
from linodemcp.profiles import Capability
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from unittest.mock import AsyncMock


def test_create_linode_image_sharegroups_by_image_list_tool_schema() -> None:
    """Tool schema exposes the documented image_id path param."""
    tool, capability = create_linode_image_sharegroup_by_image_list_tool()

    assert tool.name == "linode_image_sharegroup_by_image_list"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["image_id"]
    assert tool.input_schema["properties"]["image_id"]["type"] == "string"


@pytest.mark.asyncio
async def test_handle_linode_image_sharegroups_by_image_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns share groups for an image."""
    mock_linode_client.route_raw.return_value = {
        "data": [
            {
                "id": 4242,
                "uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "label": "shared images",
                "is_suspended": False,
                "created": "2026-01-15T10:00:00",
                "images_count": 7,
                "members_count": 3,
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_image_sharegroup_by_image_list(
        {"image_id": "private/12345"}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "image_sharegroups": [
            {
                "id": 4242,
                "uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "label": "shared images",
                "is_suspended": False,
                "created": "2026-01-15T10:00:00",
                "images_count": 7,
                "members_count": 3,
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_by_image_list", "private/12345", query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "image_id",
    [
        None,
        "",
        "ubuntu24.04",
        "linode/",
        "linode/../ubuntu",
        "linode/ubuntu?x=1",
        "linode/ubuntu/24.04",
    ],
)
async def test_handle_linode_image_sharegroups_by_image_list_rejects_bad_image_id(
    image_id: Any, sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed image IDs before client calls."""
    result = await handle_linode_image_sharegroup_by_image_list(
        {"image_id": image_id}, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"image_id": "private/12345", "page": 0},
        {"image_id": "private/12345", "page_size": 24},
        {"image_id": "private/12345", "page_size": 501},
    ],
)
async def test_handle_linode_image_sharegroups_by_image_list_rejects_invalid_pagination(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects out-of-range pagination after a valid image ID."""
    result = await handle_linode_image_sharegroup_by_image_list(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_image_sharegroups_by_image_list_registered() -> None:
    """Dynamic registry exports the by-image sharegroups tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_by_image_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_by_image_list"
    assert entry.handle_fn is handle_linode_image_sharegroup_by_image_list


def test_linode_image_sharegroups_by_image_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_by_image_list")

    assert scopes == ["images:read_only"]


def test_create_linode_images_sharegroups_list_tool_schema() -> None:
    """Tool schema exposes the documented pagination params."""
    tool, capability = create_linode_image_sharegroup_list_tool()

    assert tool.name == "linode_image_sharegroup_list"
    assert capability is Capability.Read
    assert tool.input_schema["properties"]["page"]["type"] == "integer"
    assert tool.input_schema["properties"]["page_size"]["type"] == "integer"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns image share groups in the proto list envelope."""
    mock_linode_client.route_raw.return_value = {
        "data": [
            {
                "id": 4242,
                "uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "label": "shared-images",
                "is_suspended": False,
                "created": "2026-01-15T10:00:00",
                "images_count": 7,
                "members_count": 3,
            }
        ],
        "page": 2,
        "pages": 3,
        "results": 7,
    }

    result = await handle_linode_image_sharegroup_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "image_sharegroups": [
            {
                "id": 4242,
                "uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "label": "shared-images",
                "is_suspended": False,
                "created": "2026-01-15T10:00:00",
                "images_count": 7,
                "members_count": 3,
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_list", query="page=2&page_size=50"
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {"page": 0},
        {"page": "2"},
        {"page": True},
        {"page_size": 24},
        {"page_size": 501},
        {"page_size": "50"},
        {"page_size": False},
    ],
)
async def test_handle_linode_images_sharegroups_list_rejects_invalid_pagination(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects invalid pagination before creating a client call."""
    result = await handle_linode_image_sharegroup_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroups_list_registered() -> None:
    """Dynamic registry exports the new tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_list"
    assert entry.handle_fn is handle_linode_image_sharegroup_list


def test_linode_images_sharegroups_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_list")

    assert scopes == ["images:read_only"]


def test_linode_image_sharegroup_create_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the create route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_create")

    assert scopes == ["images:read_write"]


def test_create_linode_images_sharegroups_token_get_tool_schema() -> None:
    """Tool schema requires the documented token UUID path param."""
    tool, capability = create_linode_image_sharegroup_token_get_tool()

    assert tool.name == "linode_image_sharegroup_token_get"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {"environment", "token_uuid"}
    assert tool.input_schema["required"] == ["token_uuid"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns a single image share group token."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.route_raw.return_value = {
        "id": "sharegroup-record-1",
        "token_uuid": token_uuid,
        "created": "2026-01-01T00:00:00",
    }

    result = await handle_linode_image_sharegroup_token_get(
        {"token_uuid": token_uuid}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["token_uuid"] == token_uuid
    assert payload["created"] == "2026-01-01T00:00:00"
    assert payload["status"] == ""
    assert "id" not in payload
    assert "updated" not in payload
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_token_get", token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
        {"token_uuid": "123e4567e89b12d3a456426614174000"},  # betterleaks:allow fake
    ],
)
async def test_handle_linode_images_sharegroups_token_get_rejects_invalid_token_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    result = await handle_linode_image_sharegroup_token_get(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroups_token_get_registered() -> None:
    """Dynamic registry exports the token get tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_token_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_token_get"
    assert entry.handle_fn is handle_linode_image_sharegroup_token_get


def test_linode_images_sharegroups_token_get_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_token_get")

    assert scopes == ["images:read_only"]


def test_create_linode_images_sharegroups_token_sharegroup_get_tool_schema() -> None:
    """Tool schema requires the documented token UUID path param."""
    tool, capability = create_linode_image_sharegroup_by_token_get_tool()

    assert tool.name == "linode_image_sharegroup_by_token_get"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {"environment", "token_uuid"}
    assert tool.input_schema["required"] == ["token_uuid"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_sharegroup_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns the share group associated with a token."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.route_raw.return_value = {
        "uuid": "22222222-2222-4222-8222-222222222222",
        "label": "shared-images",
    }

    result = await handle_linode_image_sharegroup_by_token_get(
        {"token_uuid": token_uuid}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload["uuid"] == "22222222-2222-4222-8222-222222222222"
    assert payload["label"] == "shared-images"
    assert payload["id"] == 0
    assert "description" not in payload
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_by_token_get", token_uuid
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_token_sharegroup_get_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    result = await handle_linode_image_sharegroup_by_token_get(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroups_token_sharegroup_get_registered() -> None:
    """Dynamic registry exports the share group by token tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_by_token_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_by_token_get"
    assert entry.handle_fn is handle_linode_image_sharegroup_by_token_get


def test_linode_images_sharegroups_token_sharegroup_get_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_by_token_get")

    assert scopes == ["images:read_only"]


def test_create_token_sharegroup_images_list_tool_schema() -> None:
    """Tool schema requires the documented token UUID path param."""
    tool, capability = create_linode_image_sharegroup_token_image_list_tool()

    assert tool.name == "linode_image_sharegroup_token_image_list"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "token_uuid",
        "page",
        "page_size",
    }
    assert tool.input_schema["required"] == ["token_uuid"]


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_sharegroup_images_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns images associated with a share group token."""
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_image_sharegroup_token_image_list(
        {"token_uuid": token_uuid}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "images": [
            {
                "id": "private/ubuntu",
                "label": "Private Ubuntu",
                "description": "",
                "type": "",
                "vendor": "",
                "status": "",
                "created": "",
                "created_by": "",
                "capabilities": [],
                "tags": [],
                "size": 0,
                "is_public": False,
                "deprecated": False,
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_token_image_list", token_uuid, query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"token_uuid": ""},
        {"token_uuid": "not-a-uuid"},
        {"token_uuid": "11111111/1111-4111-8111-111111111111"},
        {"token_uuid": "11111111?1111-4111-8111-111111111111"},
        {"token_uuid": ".."},
        {"token_uuid": 123},
    ],
)
async def test_handle_token_sharegroup_images_list_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed token UUIDs before the client call."""
    result = await handle_linode_image_sharegroup_token_image_list(
        arguments, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroups_token_sharegroup_images_list_registered() -> None:
    """Dynamic registry exports the images by token tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_token_image_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_token_image_list"
    assert entry.handle_fn is handle_linode_image_sharegroup_token_image_list


def test_token_sharegroup_images_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_token_image_list")

    assert scopes == ["images:read_only"]


def test_create_linode_images_sharegroup_members_list_tool_schema() -> None:
    """Tool schema requires the documented sharegroup UUID path param."""
    tool, capability = create_linode_image_sharegroup_member_list_tool()

    assert tool.name == "linode_image_sharegroup_member_list"
    assert capability is Capability.Read
    sharegroup_id_schema = tool.input_schema["properties"]["sharegroup_id"]
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "sharegroup_id",
        "page",
        "page_size",
    }
    assert tool.input_schema["required"] == ["sharegroup_id"]
    assert sharegroup_id_schema["type"] == "integer"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_members_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns members in the proto list envelope."""
    sharegroup_id = 3
    mock_linode_client.route_raw.return_value = {
        "data": [
            {
                "token_uuid": "11112222-3333-4444-5555-666677778888",
                "status": "active",
                "label": "partner-account",
                "created": "2026-01-15T10:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_image_sharegroup_member_list(
        {"sharegroup_id": sharegroup_id}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "image_sharegroup_members": [
            {
                "token_uuid": "11112222-3333-4444-5555-666677778888",
                "status": "active",
                "label": "partner-account",
                "created": "2026-01-15T10:00:00",
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_member_list", 3, query=""
    )


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_members_list_defaults_missing_pagination(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns an empty proto envelope for sparse API responses."""
    sharegroup_id = 3
    mock_linode_client.route_raw.return_value = {}

    result = await handle_linode_image_sharegroup_member_list(
        {"sharegroup_id": sharegroup_id}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 0,
        "image_sharegroup_members": [],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_member_list", sharegroup_id, query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"sharegroup_id": ""},
        {"sharegroup_id": "not-a-uuid"},
        {"sharegroup_id": "22222222/2222-4222-8222-222222222222"},
        {"sharegroup_id": "22222222?2222-4222-8222-222222222222"},
        {"sharegroup_id": ".."},
        {"sharegroup_id": 0},
    ],
)
async def test_handle_linode_images_sharegroup_members_list_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed sharegroup UUIDs before the client call."""
    result = await handle_linode_image_sharegroup_member_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroup_members_list_registered() -> None:
    """Dynamic registry exports the members by share group tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_member_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_member_list"
    assert entry.handle_fn is handle_linode_image_sharegroup_member_list


def test_linode_images_sharegroup_members_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_member_list")

    assert scopes == ["images:read_only"]


def test_create_linode_images_sharegroup_member_token_get_tool_schema() -> None:
    """Tool schema requires both documented path params."""
    tool, capability = create_linode_image_sharegroup_member_token_get_tool()

    assert tool.name == "linode_image_sharegroup_member_token_get"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "sharegroup_id",
        "token_uuid",
    }
    assert tool.input_schema["required"] == ["sharegroup_id", "token_uuid"]
    sharegroup_schema = tool.input_schema["properties"]["sharegroup_id"]
    token_schema = tool.input_schema["properties"]["token_uuid"]
    assert sharegroup_schema["type"] == "integer"
    assert token_schema["type"] == "string"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_member_token_get_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns a membership token associated with a share group."""
    sharegroup_id = 3
    token_uuid = "11111111-1111-4111-8111-111111111111"
    mock_linode_client.route_raw.return_value = {
        "id": "member-token-record-1",
        "token_uuid": token_uuid,
        "created": "2026-01-01T00:00:00",
    }

    result = await handle_linode_image_sharegroup_member_token_get(
        {"sharegroup_id": sharegroup_id, "token_uuid": token_uuid},
        sample_config,
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "token_uuid": token_uuid,
        "status": "",
        "label": "",
        "created": "2026-01-01T00:00:00",
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_member_token_get",
        3,
        token_uuid,
    )


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_member_token_get_refuses_padding(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """A padded token_uuid is refused rather than trimmed.

    This language used to strip it and send the trimmed value; the raw reading
    is Go's, and trimming rewrites a credential before sending it.
    """
    result = await handle_linode_image_sharegroup_member_token_get(
        {
            "sharegroup_id": 3,
            "token_uuid": " 11111111-1111-4111-8111-111111111111 ",
        },
        sample_config,
    )

    assert "token_uuid must be a UUID" in result[0].text
    mock_linode_client.route_raw.assert_not_awaited()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("sharegroup_id", "token_uuid"),
    [
        (None, "11111111-1111-4111-8111-111111111111"),
        ("not-an-integer", "11111111-1111-4111-8111-111111111111"),
        (0, "11111111-1111-4111-8111-111111111111"),
        (-1, "11111111-1111-4111-8111-111111111111"),
        (3, None),
        (3, ""),
        (3, "not-a-uuid"),
        (3, "11111111/1111-4111-8111-111111111111"),
        (3, "11111111?1111-4111-8111-111111111111"),
        (3, ".."),
        (3, 123),
    ],
)
async def test_member_token_get_rejects_invalid_path_params(
    sharegroup_id: Any,
    token_uuid: Any,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Handler rejects malformed path params before the client call."""
    result = await handle_linode_image_sharegroup_member_token_get(
        {"sharegroup_id": sharegroup_id, "token_uuid": token_uuid}, sample_config
    )

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroup_member_token_get_registered() -> None:
    """Dynamic registry exports the member token get tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_member_token_get"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_member_token_get"
    assert entry.handle_fn is handle_linode_image_sharegroup_member_token_get


def test_linode_images_sharegroup_member_token_get_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_member_token_get")

    assert scopes == ["images:read_only"]


def test_linode_images_sharegroup_member_token_update_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_member_token_update")

    assert scopes == ["images:read_write"]


def test_create_linode_images_sharegroup_member_token_delete_tool_schema() -> None:
    """Tool schema requires both path params, confirm, and dry_run."""
    tool, capability = create_linode_image_sharegroup_member_token_delete_tool()

    assert tool.name == "linode_image_sharegroup_member_token_delete"
    assert capability is Capability.Destroy
    assert tool.input_schema["required"] == [
        "sharegroup_id",
        "token_uuid",
        "confirm",
    ]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"
    assert tool.input_schema["properties"]["sharegroup_id"]["type"] == "integer"
    assert tool.input_schema["properties"]["token_uuid"]["type"] == "string"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_member_token_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler revokes a member token through the client."""
    sharegroup_id = 3
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_image_sharegroup_member_token_delete(
        {
            "sharegroup_id": sharegroup_id,
            "token_uuid": token_uuid,
            "confirm": True,
        },
        sample_config,
    )

    assert json.loads(result[0].text) == {
        "message": (
            "Image share group member token "
            "11111111-1111-4111-8111-111111111111 "
            "revoked from share group 3 successfully"
        )
    }
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_image_sharegroup_member_token_delete",
        sharegroup_id,
        token_uuid,
        retry=False,
    )


@pytest.mark.asyncio
async def test_image_sharegroup_member_token_delete_dry_run_previews_without_confirm(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry-run previews without requiring the confirm gate."""
    mock_linode_client.route_raw.return_value = {
        "id": 3,
        "label": "share",
    }
    result = await handle_linode_image_sharegroup_member_token_delete(
        {
            "sharegroup_id": 3,
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "dry_run": True,
        },
        sample_config,
    )

    assert '"dry_run": true' in result[0].text


def test_linode_images_sharegroup_member_token_delete_registered() -> None:
    """Dynamic registry exports the member token delete tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_member_token_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_image_sharegroup_member_token_delete"
    assert entry.handle_fn is handle_linode_image_sharegroup_member_token_delete


def test_linode_images_sharegroup_member_token_delete_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_member_token_delete")

    assert scopes == ["images:read_write"]


def test_create_linode_images_sharegroup_image_delete_tool_schema() -> None:
    """Delete-image tool schema exposes path params, confirm, and dry_run."""
    tool, capability = create_linode_image_sharegroup_image_delete_tool()

    assert tool.name == "linode_image_sharegroup_image_delete"
    assert capability is Capability.Destroy
    schema = tool.input_schema
    assert schema["required"] == ["sharegroup_id", "image_id", "confirm"]
    assert schema["properties"]["sharegroup_id"]["type"] == "integer"
    assert schema["properties"]["image_id"]["type"] == "string"
    assert schema["properties"]["confirm"]["type"] == "boolean"
    assert schema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_image_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler revokes one shared image and returns a success message."""
    result = await handle_linode_image_sharegroup_image_delete(
        {"sharegroup_id": 123, "image_id": "shared/456", "confirm": True},
        sample_config,
    )

    assert json.loads(result[0].text) == {
        "message": (
            "Shared image shared/456 removed from image share group 123 successfully"
        )
    }
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_image_sharegroup_image_delete", 123, "shared/456", retry=False
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "expected_error"),
    [
        (
            {"image_id": 456, "confirm": True},
            "sharegroup_id is required",
        ),
        (
            {"sharegroup_id": "", "image_id": 456, "confirm": True},
            "sharegroup_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 0, "image_id": 456, "confirm": True},
            "sharegroup_id is required",
        ),
        (
            {"sharegroup_id": True, "image_id": 456, "confirm": True},
            "sharegroup_id must be a positive integer",
        ),
        (
            {"sharegroup_id": 123, "confirm": True},
            "image_id is required",
        ),
        (
            {"sharegroup_id": 123, "image_id": 0, "confirm": True},
            "image_id is required",
        ),
        (
            {"sharegroup_id": 123, "image_id": False, "confirm": True},
            "image_id is required",
        ),
        (
            {"sharegroup_id": 123, "image_id": -5, "confirm": True},
            "image_id is required",
        ),
    ],
)
async def test_handle_linode_images_sharegroup_image_delete_rejects_invalid_path_params(
    sample_config: Any,
    mock_linode_client: AsyncMock,
    arguments: dict[str, object],
    expected_error: str,
) -> None:
    """Handler rejects malformed path params before client calls."""
    result = await handle_linode_image_sharegroup_image_delete(arguments, sample_config)

    assert expected_error in result[0].text
    mock_linode_client.route_call.assert_not_called()


def test_linode_images_sharegroup_image_delete_registered() -> None:
    """Dynamic registry exports the delete-image tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_image_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_image_sharegroup_image_delete"
    assert entry.handle_fn is handle_linode_image_sharegroup_image_delete


def test_linode_images_sharegroup_image_delete_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_image_delete")

    assert scopes == ["images:read_write"]


def test_linode_images_sharegroup_images_add_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_image_add")

    assert scopes == ["images:read_write"]


def test_linode_images_sharegroup_members_add_registered() -> None:
    """Dynamic registry exports the add-members tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_member_add"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_image_sharegroup_member_add"


def test_linode_images_sharegroup_members_add_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_member_add")

    assert scopes == ["images:read_write"]


def test_create_linode_images_sharegroup_images_list_tool_schema() -> None:
    """Tool schema requires the documented sharegroup UUID path param."""
    tool, capability = create_linode_image_sharegroup_image_list_tool()

    assert tool.name == "linode_image_sharegroup_image_list"
    assert capability is Capability.Read
    sharegroup_id_schema = tool.input_schema["properties"]["sharegroup_id"]
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "sharegroup_id",
        "page",
        "page_size",
    }
    assert tool.input_schema["required"] == ["sharegroup_id"]
    assert sharegroup_id_schema["type"] == "integer"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroup_images_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns images associated with a share group."""
    sharegroup_id = 3
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": "private/ubuntu", "label": "Private Ubuntu"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_image_sharegroup_image_list(
        {"sharegroup_id": sharegroup_id}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "images": [
            {
                "id": "private/ubuntu",
                "label": "Private Ubuntu",
                "description": "",
                "type": "",
                "vendor": "",
                "status": "",
                "created": "",
                "created_by": "",
                "capabilities": [],
                "tags": [],
                "size": 0,
                "is_public": False,
                "deprecated": False,
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_image_list", 3, query=""
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("pagination", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
    ],
)
async def test_handle_linode_images_sharegroup_images_list_rejects_bad_pagination(
    pagination: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Invalid pagination is rejected before the client is called."""
    arguments: dict[str, Any] = {"sharegroup_id": 3, **pagination}

    result = await handle_linode_image_sharegroup_image_list(arguments, sample_config)

    assert len(result) == 1
    assert message in result[0].text
    mock_linode_client.route_raw.assert_not_awaited()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("pagination", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
    ],
)
async def test_handle_linode_images_token_images_list_rejects_bad_pagination(
    pagination: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Token image list rejects invalid pagination before calling the client."""
    arguments: dict[str, Any] = {
        "token_uuid": "11111111-1111-4111-8111-111111111111",
        **pagination,
    }

    result = await handle_linode_image_sharegroup_token_image_list(
        arguments, sample_config
    )

    assert len(result) == 1
    assert message in result[0].text
    mock_linode_client.route_raw.assert_not_awaited()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("pagination", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
    ],
)
async def test_handle_linode_images_by_image_list_rejects_bad_pagination(
    pagination: dict[str, Any],
    message: str,
    sample_config: Any,
    mock_linode_client: AsyncMock,
) -> None:
    """Share-groups-by-image list rejects bad pagination before the client call."""
    arguments: dict[str, Any] = {"image_id": "private/12345", **pagination}

    result = await handle_linode_image_sharegroup_by_image_list(
        arguments, sample_config
    )

    assert len(result) == 1
    assert message in result[0].text
    mock_linode_client.route_raw.assert_not_awaited()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "arguments",
    [
        {},
        {"sharegroup_id": ""},
        {"sharegroup_id": "not-a-uuid"},
        {"sharegroup_id": "22222222/2222-4222-8222-222222222222"},
        {"sharegroup_id": "22222222?2222-4222-8222-222222222222"},
        {"sharegroup_id": ".."},
        {"sharegroup_id": 0},
    ],
)
async def test_handle_linode_images_sharegroup_images_list_rejects_invalid_uuid(
    arguments: dict[str, Any], sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler rejects malformed sharegroup UUIDs before the client call."""
    result = await handle_linode_image_sharegroup_image_list(arguments, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_linode_client.route_raw.assert_not_called()


def test_linode_images_sharegroup_images_list_registered() -> None:
    """Dynamic registry exports the images by share group tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_image_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_image_list"
    assert entry.handle_fn is handle_linode_image_sharegroup_image_list


def test_linode_images_sharegroup_images_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_image_list")

    assert scopes == ["images:read_only"]


def test_linode_images_sharegroups_token_update_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_token_update")

    assert scopes == ["images:read_write"]


def test_create_linode_images_sharegroups_token_delete_tool_schema() -> None:
    """Tool schema requires token UUID and confirm."""
    tool, capability = create_linode_image_sharegroup_token_delete_tool()

    assert tool.name == "linode_image_sharegroup_token_delete"
    assert capability is Capability.Destroy
    assert tool.input_schema["required"] == ["token_uuid", "confirm"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert tool.input_schema["properties"]["dry_run"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler deletes a token through the client."""
    token_uuid = "11111111-1111-4111-8111-111111111111"

    result = await handle_linode_image_sharegroup_token_delete(
        {"token_uuid": token_uuid, "confirm": True}, sample_config
    )

    payload = json.loads(result[0].text)
    assert payload == {"message": "Image share group token removed successfully"}
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_image_sharegroup_token_delete", token_uuid, retry=False
    )


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_token_delete_refuses_a_padded_uuid(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """A padded token_uuid is refused rather than trimmed.

    The hand handlers stripped it and sent the trimmed value; the contract's
    rule reads the argument as it arrived, so both languages now answer the
    same refusal instead of one of them silently rewriting a credential.
    """
    result = await handle_linode_image_sharegroup_token_delete(
        {"token_uuid": " 11111111-1111-4111-8111-111111111111 ", "confirm": True},
        sample_config,
    )

    assert "token_uuid must be a UUID" in result[0].text
    mock_linode_client.route_call.assert_not_called()


@pytest.mark.asyncio
async def test_image_sharegroup_token_delete_dry_run_previews_without_confirm(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Dry-run previews without requiring the confirm gate."""
    mock_linode_client.route_raw.return_value = {
        "id": 3,
        "label": "share",
    }
    result = await handle_linode_image_sharegroup_token_delete(
        {
            "token_uuid": "11111111-1111-4111-8111-111111111111",
            "dry_run": True,
        },
        sample_config,
    )

    assert '"dry_run": true' in result[0].text


def test_linode_images_sharegroups_token_delete_registered() -> None:
    """Dynamic registry exports the token delete tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_token_delete"]
    assert entry.capability is Capability.Destroy
    assert entry.tool.name == "linode_image_sharegroup_token_delete"
    assert entry.handle_fn is handle_linode_image_sharegroup_token_delete


def test_linode_images_sharegroups_token_delete_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_token_delete")

    assert scopes == ["images:read_write"]


def test_create_linode_images_sharegroups_tokens_list_tool_schema() -> None:
    """Tool schema exposes the documented environment and pagination arguments."""
    tool, capability = create_linode_image_sharegroup_token_list_tool()

    assert tool.name == "linode_image_sharegroup_token_list"
    assert capability is Capability.Read
    assert set(tool.input_schema["properties"]) == {
        "environment",
        "page",
        "page_size",
    }
    assert "required" not in tool.input_schema


@pytest.mark.asyncio
async def test_handle_linode_images_sharegroups_tokens_list_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler returns image share group tokens in the proto list envelope."""
    mock_linode_client.route_raw.return_value = {
        "data": [
            {
                "token": "tok_abcdef1234567890",
                "token_uuid": "99998888-7777-6666-5555-444433332222",
                "status": "active",
                "label": "partner-token",
                "created": "2026-01-01T00:00:00",
                "valid_for_sharegroup_uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "sharegroup_uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "sharegroup_label": "shared-images",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    result = await handle_linode_image_sharegroup_token_list({}, sample_config)

    payload = json.loads(result[0].text)
    assert payload == {
        "count": 1,
        "image_sharegroup_tokens": [
            {
                "token": "tok_abcdef1234567890",
                "token_uuid": "99998888-7777-6666-5555-444433332222",
                "status": "active",
                "label": "partner-token",
                "created": "2026-01-01T00:00:00",
                "valid_for_sharegroup_uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "sharegroup_uuid": "abc12345-def6-7890-abcd-ef1234567890",
                "sharegroup_label": "shared-images",
            }
        ],
    }
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_image_sharegroup_token_list", query=""
    )


def test_linode_images_sharegroups_tokens_list_registered() -> None:
    """Dynamic registry exports the token list tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_token_list"]
    assert entry.capability is Capability.Read
    assert entry.tool.name == "linode_image_sharegroup_token_list"
    assert entry.handle_fn is handle_linode_image_sharegroup_token_list


def test_linode_images_sharegroups_tokens_list_scopes_to_images_read() -> None:
    """Profile scope mapping keeps the route in the Images read category."""
    scopes = scopes_for("linode_image_sharegroup_token_list")

    assert scopes == ["images:read_only"]


def test_linode_images_sharegroup_image_update_registered() -> None:
    """Dynamic registry exports the shared-image update tool and handler pair."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    entry = entries["linode_image_sharegroup_image_update"]
    assert entry.capability is Capability.Write
    assert entry.tool.name == "linode_image_sharegroup_image_update"
    assert entry.handle_fn is handle_linode_image_sharegroup_image_update


def test_linode_images_sharegroup_image_update_scopes_to_images_write() -> None:
    """Profile scope mapping keeps the route in the Images write category."""
    scopes = scopes_for("linode_image_sharegroup_image_update")

    assert scopes == ["images:read_write"]


def test_linode_image_delete_tool_schema_requires_confirm() -> None:
    """Tool schema requires image_id and explicit confirm."""
    tool, capability = create_linode_image_delete_tool()

    assert tool.name == "linode_image_delete"
    assert capability is Capability.Destroy
    assert tool.input_schema["required"] == ["image_id", "confirm"]
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"


@pytest.mark.asyncio
async def test_handle_linode_image_delete_success(
    sample_config: Any, mock_linode_client: AsyncMock
) -> None:
    """Handler deletes a private image after validation and confirmation."""
    result = await handle_linode_image_delete(
        {"image_id": "private/123", "confirm": True}, sample_config
    )

    assert json.loads(result[0].text) == {
        "message": "Image private/123 deleted successfully"
    }
    mock_linode_client.route_call.assert_awaited_once_with(
        "linode_image_delete", "private/123", retry=False
    )


def test_linode_image_delete_is_registered() -> None:
    """Server registry includes the image delete tool."""
    entries = {entry.name: entry for entry in get_tool_registry()}

    assert "linode_image_delete" in entries
    assert entries["linode_image_delete"].capability is Capability.Destroy
