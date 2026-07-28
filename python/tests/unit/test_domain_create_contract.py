"""Frozen contract coverage for ``POST /v4/domains``."""

from __future__ import annotations

import json
from types import SimpleNamespace
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import tools as tools_module
from linodemcp.linode import APIError, RetryableClient
from linodemcp.profiles import Capability, Scope, required_scopes
from linodemcp.server import get_tool_registry
from linodemcp.tools.linode_domains_write import (
    create_linode_domain_create_tool,
    handle_linode_domain_create,
)

if TYPE_CHECKING:
    from linodemcp.config import Config


def _client_with_response(response: Any) -> AsyncMock:
    client = AsyncMock()
    client.post_raw.return_value = response
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


def test_domain_create_is_exported_registered_scoped_and_schema_backed() -> None:
    assert "create_linode_domain_create_tool" in tools_module.__all__
    assert "handle_linode_domain_create" in tools_module.__all__

    tool, capability = create_linode_domain_create_tool()
    registry = {entry.name: entry for entry in get_tool_registry()}
    assert registry["linode_domain_create"].capability is Capability.Write
    assert capability is Capability.Write
    assert required_scopes(tool.name, capability) == [Scope.DomainsReadWrite]

    assert set(tool.inputSchema["properties"]) >= {
        "environment",
        "domain",
        "type",
        "axfr_ips",
        "description",
        "expire_sec",
        "group",
        "master_ips",
        "refresh_sec",
        "retry_sec",
        "soa_email",
        "status",
        "tags",
        "ttl_sec",
        "confirm",
        "dry_run",
    }
    assert set(tool.inputSchema["required"]) == {"domain", "type", "confirm"}
    assert tool.inputSchema["properties"]["type"]["type"] == "string"
    assert tool.inputSchema["properties"]["status"]["enum"] == [
        "active",
        "disabled",
    ]


@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"type": "master", "soa_email": "a@example.com"}, "domain is required"),
        (
            {"domain": "a" * 254, "type": "master", "soa_email": "a@example.com"},
            "domain must be between 1 and 253 characters",
        ),
        (
            {"domain": "bad domain", "type": "master", "soa_email": "a@example.com"},
            "domain must match the documented domain-name pattern",
        ),
        ({"domain": "example.com"}, "type is required"),
        (
            {"domain": "example.com", "type": "primary"},
            "type must be one of: master, slave",
        ),
        (
            {"domain": "example.com", "type": "master"},
            "soa_email is required for master domains",
        ),
        (
            {"domain": "example.com", "type": "slave"},
            "master_ips must include at least one value for slave domains",
        ),
        (
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "a@example.com",
                "status": "edit_mode",
            },
            "status must be one of: active, disabled",
        ),
        (
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "a@example.com",
                "status": 1,
            },
            "status must be a string",
        ),
        (
            {
                "domain": "example.com",
                "type": "slave",
                "master_ips": ["192.0.2.1", 2],
            },
            "master_ips must be an array of strings",
        ),
        (
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "a@example.com",
                "description": None,
            },
            "description must be a string",
        ),
        (
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "a@example.com",
                "expire_sec": "1",
            },
            "expire_sec must be an integer",
        ),
        (
            {
                "domain": "example.com",
                "type": "slave",
                "soa_email": 1,
                "master_ips": ["192.0.2.1"],
            },
            "soa_email must be a string",
        ),
    ],
)
async def test_domain_create_validates_frozen_contract(
    sample_config: Config, arguments: dict[str, Any], message: str
) -> None:
    result = await handle_linode_domain_create(
        {**arguments, "confirm": True}, sample_config
    )
    assert message in result[0].text


async def test_domain_create_requires_literal_true_confirmation(
    sample_config: Config,
) -> None:
    result = await handle_linode_domain_create(
        {
            "domain": "example.com",
            "type": "master",
            "soa_email": "admin@example.com",
            "confirm": 1,
        },
        sample_config,
    )
    assert "Set confirm=true to proceed" in result[0].text


async def test_domain_create_forwards_exact_full_body_and_route(
    sample_config: Config,
) -> None:
    raw_domain = {"id": 7, "domain": "example.com", "type": "slave"}
    client = _client_with_response(raw_domain)
    arguments = {
        "domain": "example.com",
        "type": "slave",
        "axfr_ips": ["192.0.2.10"],
        "description": "",
        "expire_sec": 0,
        "group": "",
        "master_ips": ["192.0.2.20"],
        "refresh_sec": 0,
        "retry_sec": 3600,
        "soa_email": "",
        "status": "disabled",
        "tags": [],
        "ttl_sec": 0,
        "confirm": True,
    }
    expected_body = {key: value for key, value in arguments.items() if key != "confirm"}

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_domain_create(arguments, sample_config)

    assert json.loads(result[0].text)["domain"]["id"] == 7
    client.post_raw.assert_awaited_once_with("/domains", expected_body, retry=False)


async def test_domain_create_omits_absent_optional_fields(
    sample_config: Config,
) -> None:
    client = _client_with_response({})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await handle_linode_domain_create(
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "confirm": True,
            },
            sample_config,
        )

    client.post_raw.assert_awaited_once_with(
        "/domains",
        {
            "domain": "example.com",
            "type": "master",
            "soa_email": "admin@example.com",
        },
        retry=False,
    )


async def test_domain_create_dry_run_previews_exact_request(
    sample_config: Config,
) -> None:
    arguments = {
        "domain": "example.com",
        "type": "slave",
        "description": "",
        "expire_sec": 0,
        "master_ips": ["192.0.2.20"],
        "tags": [],
        "dry_run": True,
    }
    result = await handle_linode_domain_create(arguments, sample_config)
    preview = json.loads(result[0].text)

    assert preview["would_execute"] == {
        "method": "POST",
        "path": "/domains",
        "body": {key: value for key, value in arguments.items() if key != "dry_run"},
    }


@pytest.mark.parametrize(
    "response",
    [
        None,
        [],
        "not-an-object",
        ["future_field"],
        7,
        True,
        1.5,
        [{"id": 7}],
        ["id"],
        [7],
        [True],
        {"id": []},
    ],
)
async def test_domain_create_rejects_incompatible_success_response_shapes(
    sample_config: Config, response: Any
) -> None:
    client = _client_with_response(response)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_domain_create(
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "confirm": True,
            },
            sample_config,
        )

    assert result[0].text.startswith(("Error: ", "Failed to create domain:"))


@pytest.mark.parametrize(
    "response",
    [
        {},
        {"future_field": "kept by the API"},
    ],
)
async def test_domain_create_preserves_compatible_legacy_response_shapes(
    sample_config: Config, response: Any
) -> None:
    client = _client_with_response(response)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_domain_create(
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "confirm": True,
            },
            sample_config,
        )

    assert not result[0].text.startswith("Failed to create domain:")


async def test_domain_create_surfaces_standard_api_error(
    sample_config: Config,
) -> None:
    client = _client_with_response({})
    client.post_raw.side_effect = APIError(400, "soa_email is required", "soa_email")
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_domain_create(
            {
                "domain": "example.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "confirm": True,
            },
            sample_config,
        )

    assert "soa_email is required (field: soa_email)" in result[0].text


async def test_raw_post_uses_selected_single_attempt_protected_path() -> None:
    client = object.__new__(RetryableClient)
    client.client = cast("Any", SimpleNamespace(post_raw=AsyncMock()))
    without_retry = AsyncMock(return_value={"id": 7})
    with_retry = AsyncMock()

    with (
        patch.object(client, "_execute_without_retry", without_retry),
        patch.object(client, "_execute_with_retry", with_retry),
    ):
        result = await client.post_raw(
            "/domains", {"domain": "example.com"}, retry=False
        )

    assert result == {"id": 7}
    without_retry.assert_awaited_once_with(
        client.client.post_raw, "/domains", {"domain": "example.com"}
    )
    with_retry.assert_not_awaited()
