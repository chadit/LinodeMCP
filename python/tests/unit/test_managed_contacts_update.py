"""Tests for Managed contact update route tooling."""

import json
from typing import Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.config import Config
from linodemcp.genpb.linode.mcp.v1 import managed_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.linode_account import (
    create_linode_managed_contact_update_tool,
    handle_linode_managed_contact_update,
)
from linodemcp.tools.proto_response import serialize_api_response


def test_create_linode_managed_contacts_update_tool() -> None:
    """Test linode_managed_contact_update tool schema."""
    tool, capability = create_linode_managed_contact_update_tool()

    assert tool.name == "linode_managed_contact_update"
    assert capability is Capability.Admin
    assert tool.inputSchema["type"] == "object"
    assert tool.inputSchema["required"] == ["contact_id", "confirm"]
    properties = tool.inputSchema["properties"]
    assert properties["contact_id"]["minimum"] == 1
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    assert properties["group"]["type"] == "string"
    assert properties["phone"]["properties"]["primary"]["type"] == [
        "string",
        "null",
    ]
    assert "id" not in properties
    assert "updated" not in properties


async def test_handle_linode_managed_contacts_update(sample_config: Config) -> None:
    """Test linode_managed_contact_update tool."""
    response_data: dict[str, Any] = {
        "id": 174,
        "name": "Ops",
        "email": "ops@example.com",
        "group": "on-call",
    }
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_managed_contact.return_value = response_data
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_update(
            {
                "contact_id": 174,
                "email": "ops@example.com",
                "group": "on-call",
                "name": "Ops",
                "phone": {"primary": "123-456-7890"},
                "confirm": True,
            },
            sample_config,
        )

        assert len(result) == 1
        assert json.loads(result[0].text) == serialize_api_response(
            {
                "message": "Managed contact 174 updated successfully",
                "contact": response_data,
            },
            managed_pb2.ManagedContactWriteResponse(),
        )
        mock_client.update_managed_contact.assert_awaited_once_with(
            174,
            email="ops@example.com",
            group="on-call",
            name="Ops",
            phone={"primary": "123-456-7890"},
        )


async def test_handle_linode_managed_contacts_update_allows_null_group(
    sample_config: Config,
) -> None:
    """Managed contact update can clear a nullable group."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.update_managed_contact.return_value = {"id": 174, "group": None}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_managed_contact_update(
            {"contact_id": 174, "group": None, "confirm": True}, sample_config
        )

    assert json.loads(result[0].text) == serialize_api_response(
        {
            "message": "Managed contact 174 updated successfully",
            "contact": {"id": 174, "group": None},
        },
        managed_pb2.ManagedContactWriteResponse(),
    )
    mock_client.update_managed_contact.assert_awaited_once_with(174, group=None)


@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handle_linode_managed_contacts_update_requires_boolean_confirm(
    sample_config: Config, confirm: object
) -> None:
    """Managed contact update requires explicit boolean confirmation."""
    arguments: dict[str, object] = {"contact_id": 174, "email": "ops@example.com"}
    if confirm is not None:
        arguments["confirm"] = confirm
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(arguments, sample_config)

    assert "confirm=true" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize("contact_id", [0, -1, True, "/", "1?", ".."])
async def test_handle_linode_managed_contacts_update_rejects_invalid_contact_id(
    sample_config: Config, contact_id: object
) -> None:
    """Managed contact update validates contact_id before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(
            {"contact_id": contact_id, "email": "ops@example.com", "confirm": True},
            sample_config,
        )

    assert "contact_id must be a positive integer" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("field", "value", "expected"),
    [
        ("email", "", "email must be a non-empty string"),
        ("email", 123, "email must be a non-empty string"),
        ("name", "", "name must be a non-empty string"),
        ("name", 123, "name must be a non-empty string"),
        ("group", "", "group must be a non-empty string or null"),
        ("group", 123, "group must be a non-empty string or null"),
    ],
)
async def test_handle_linode_managed_contacts_update_rejects_invalid_writable_strings(
    sample_config: Config, field: str, value: object, expected: str
) -> None:
    """Managed contact update validates writable string fields."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(
            {"contact_id": 174, field: value, "confirm": True}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_contacts_update_rejects_read_only_fields(
    sample_config: Config,
) -> None:
    """Managed contact update does not forward read-only fields."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(
            {"contact_id": 174, "id": 174, "updated": "2024-01-01", "confirm": True},
            sample_config,
        )

    assert "Read-only fields are not accepted: id, updated" in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_contacts_update_requires_field(
    sample_config: Config,
) -> None:
    """Managed contact update requires at least one writable field."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(
            {"contact_id": 174, "confirm": True}, sample_config
        )

    assert "At least one managed contact field is required" in result[0].text
    mock_client_class.assert_not_called()


@pytest.mark.parametrize(
    ("phone", "expected"),
    [
        ("123", "phone must be an object"),
        ({}, "phone must include primary or secondary"),
        ({"mobile": "123"}, "phone has unknown fields: mobile"),
        ({"primary": ""}, "phone.primary must be a non-empty string or null"),
        ({"secondary": 123}, "phone.secondary must be a non-empty string or null"),
    ],
)
async def test_handle_linode_managed_contacts_update_rejects_malformed_phone(
    sample_config: Config, phone: object, expected: str
) -> None:
    """Managed contact update validates phone before client calls."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(
            {"contact_id": 174, "phone": phone, "confirm": True}, sample_config
        )

    assert expected in result[0].text
    mock_client_class.assert_not_called()


async def test_handle_linode_managed_contacts_update_dry_run(
    sample_config: Config,
) -> None:
    """Managed contact update dry-run previews the request without calling client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        result = await handle_linode_managed_contact_update(
            {
                "contact_id": 174,
                "email": "ops@example.com",
                "group": None,
                "confirm": True,
                "dry_run": True,
            },
            sample_config,
        )

    data = json.loads(result[0].text)
    assert data["would_execute"]["method"] == "PUT"
    assert data["would_execute"]["path"] == "/managed/contacts/174"
    assert data["would_execute"]["body"] == {
        "email": "ops@example.com",
        "group": None,
    }
    mock_client_class.assert_not_called()
