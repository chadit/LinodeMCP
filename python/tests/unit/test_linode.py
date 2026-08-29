"""Unit tests for Linode client."""

import asyncio
import json
from typing import Any, cast
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.config import Config, EnvironmentConfig, LinodeConfig
from linodemcp.gentools import (
    create_linode_monitor_alert_channel_list_tool,
    create_linode_monitor_alert_definition_list_tool,
    create_linode_monitor_dashboard_get_tool,
    create_linode_monitor_dashboard_list_tool,
    create_linode_monitor_service_alert_definition_delete_tool,
    create_linode_monitor_service_alert_definition_get_tool,
    create_linode_monitor_service_alert_definition_list_tool,
    create_linode_monitor_service_dashboard_list_tool,
    handle_linode_monitor_alert_channel_list,
    handle_linode_monitor_alert_definition_list,
    handle_linode_monitor_dashboard_get,
    handle_linode_monitor_dashboard_list,
    handle_linode_monitor_service_alert_definition_delete,
    handle_linode_monitor_service_alert_definition_get,
    handle_linode_monitor_service_alert_definition_list,
    handle_linode_monitor_service_dashboard_list,
)
from linodemcp.gentools.longview import handle_linode_longview_client_create
from linodemcp.linode import (
    Addons,
    APIError,
    BackupsAddon,
    CircuitBreaker,
    CircuitOpenError,
    Client,
    FirewallRules,
    Grant,
    Grants,
    Image,
    Instance,
    InstanceType,
    NetworkError,
    Price,
    Profile,
    RateLimiter,
    RetryableClient,
    RetryConfig,
    instance_to_response_dict,
    is_retryable,
    validate_disk_size,
    validate_dns_record_name,
    validate_dns_record_target,
    validate_firewall_policy,
    validate_label,
    validate_root_password,
    validate_ssh_key,
    validate_volume_size,
)
from linodemcp.profiles import Capability

# The tools whose routes the transport primitives address, named once so the
# same string is not spelled in every case.
ATTACHMENT_TOOL = "linode_support_ticket_attachment_create"
THUMBNAIL_GET_TOOL = "linode_account_oauth_client_thumbnail_get"
THUMBNAIL_UPDATE_TOOL = "linode_account_oauth_client_thumbnail_update"


@pytest.fixture
def mock_httpx_client() -> MagicMock:
    """Mock httpx.AsyncClient."""
    return MagicMock()


@pytest.fixture
def linode_client() -> Client:
    """Create a Linode client for testing."""
    return Client("https://api.linode.com/v4", "test-token")


async def test_client_creation() -> None:
    """Test client creation."""
    client = Client("https://api.linode.com/v4", "test-token")
    assert client.base_url == "https://api.linode.com/v4"
    assert client.token == "test-token"
    await client.close()


async def test_get_profile(sample_profile_data: dict[str, Any]) -> None:
    """Test getting user profile."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = sample_profile_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.get_profile()

        assert isinstance(profile, Profile)
        assert profile.username == "testuser"
        assert profile.email == "test@example.com"
        assert profile.uid == 12345

    await client.close()


async def test_get_profile_parses_pat_scopes(
    sample_profile_data: dict[str, Any],
) -> None:
    """PAT response with scopes string must round-trip into Profile.scopes.

    The Linode API returns the space-delimited scope string on /profile
    for personal access tokens; the Phase 6 loader reads this field
    instead of /profile/grants when it's non-empty.
    """
    client = Client("https://api.linode.com/v4", "test-token")

    pat_response = {**sample_profile_data, "scopes": "linodes:read_write *"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = pat_response

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.get_profile()

    assert profile.scopes == "linodes:read_write *", (
        "PAT scopes from /profile must populate Profile.scopes"
    )
    await client.close()


async def test_get_profile_oauth_leaves_scopes_empty(
    sample_profile_data: dict[str, Any],
) -> None:
    """OAuth /profile response without scopes leaves Profile.scopes empty.

    The Phase 6 loader uses Profile.scopes == "" as the signal to fall
    back to /profile/grants. Tests guarantee that signal stays accurate.
    """
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = sample_profile_data  # no scopes key

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.get_profile()

    assert profile.scopes == "", (
        "OAuth /profile (no scopes field) must leave Profile.scopes empty"
    )
    await client.close()


async def test_get_profile_grants_parses_oauth_response() -> None:
    """OAuth /profile/grants populates Grants with structured per-resource lists.

    Verifies the global block, per-resource lists, and the GrantPermission
    string values all round-trip. The Phase 6 loader walks this exact
    shape to determine what an OAuth token can do.
    """
    client = Client("https://api.linode.com/v4", "test-token")

    grants_payload = {
        "global": {
            "account_access": "read_write",
            "add_linodes": True,
            "add_domains": False,
            "cancel_account": False,
        },
        "linode": [
            {"id": 42, "label": "web-1", "permissions": "read_write"},
        ],
        "domain": [
            {"id": 7, "label": "example.com", "permissions": "read_only"},
        ],
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = grants_payload

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        grants = await client.get_profile_grants()

        mock_request.assert_called_once_with("GET", "/profile/grants")

    assert isinstance(grants, Grants)
    assert grants.global_.account_access == "read_write"
    assert grants.global_.add_linodes is True
    assert grants.global_.add_domains is False
    assert len(grants.linode) == 1
    assert grants.linode[0] == Grant(id=42, label="web-1", permissions="read_write")
    assert len(grants.domain) == 1
    assert grants.domain[0].permissions == "read_only"
    # Unprovided categories default to empty lists.
    assert grants.nodebalancer == []
    assert grants.image == []

    await client.close()


async def test_get_profile_grants_pat_empty_payload() -> None:
    """PAT /profile/grants returns an empty Grants without error.

    The Linode API still answers 200 for PAT tokens hitting
    /profile/grants but returns zero-valued fields. The parser must not
    raise; the loader uses Profile.scopes to detect the PAT path anyway.
    """
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        grants = await client.get_profile_grants()

    assert isinstance(grants, Grants)
    assert grants.linode == []
    assert grants.global_.account_access == ""
    assert grants.global_.add_linodes is False

    await client.close()


async def test_get_profile_grants_propagates_http_errors() -> None:
    """A 401 on /profile/grants surfaces as NetworkError (wrapped httpx)."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("unauthorized")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_profile_grants()

    assert "GetProfileGrants" in str(excinfo.value)
    await client.close()


async def test_get_account_settings_sends_exact_route() -> None:
    """Account settings get sends GET /account/settings."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "backups_enabled": True,
        "managed": False,
        "network_helper": True,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_settings()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/settings")
    await client.close()


async def test_get_account_settings_wraps_http_errors() -> None:
    """Account settings get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_settings()

    assert "GetAccountSettings" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_settings_delegates_to_client() -> None:
    """RetryableClient delegates account settings get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_settings", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"managed": False}
        result = await retryable.get_account_settings()

    assert result == {"managed": False}
    mock_get.assert_awaited_once_with()
    await retryable.close()


async def test_route_raw_body_sends_the_bytes_as_the_whole_body() -> None:
    """A raw-body route PUTs the bytes it was handed under the declared type."""
    client = Client("https://api.linode.com/v4", "test-token")
    thumbnail = b"\x89PNG\r\n\x1a\n"
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.content = b""

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.route_raw_body(
            THUMBNAIL_UPDATE_TOOL,
            "client-1",
            content_type="image/png",
            payload=thumbnail,
        )

    mock_request.assert_awaited_once_with(
        "PUT",
        "https://api.linode.com/v4/account/oauth-clients/client-1/thumbnail",
        headers={
            "Authorization": "Bearer test-token",
            "Content-Type": "image/png",
            "User-Agent": "LinodeMCP/1.0",
        },
        content=thumbnail,
    )
    await client.close()


async def test_route_raw_body_encodes_the_path_argument() -> None:
    """A raw-body route escapes its slot the way every routed primitive does."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.content = b""

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.route_raw_body(
            THUMBNAIL_UPDATE_TOOL,
            "client/123?query",
            content_type="image/png",
            payload=b"png",
        )

    await_args = mock_request.await_args
    assert await_args is not None
    assert await_args.args == (
        "PUT",
        "https://api.linode.com/v4/account/oauth-clients/client%2F123%3Fquery/thumbnail",
    )
    await client.close()


async def test_retryable_route_raw_body_can_take_one_protected_attempt() -> None:
    """The thumbnail update declares retry_disabled, and the caller selects the
    unprotected executor rather than the client deciding for it."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with (
        patch.object(
            retryable.client, "route_raw_body", new_callable=AsyncMock
        ) as mock_send,
        patch.object(
            retryable, "_execute_with_retry", new_callable=AsyncMock
        ) as mock_retry,
    ):
        await retryable.route_raw_body(
            THUMBNAIL_UPDATE_TOOL,
            "client-1",
            content_type="image/png",
            payload=b"png",
            retry=False,
        )

    mock_send.assert_awaited_once()
    mock_retry.assert_not_called()
    await retryable.close()


async def test_get_type_sends_get_to_encoded_linode_type_route() -> None:
    """Getting a Linode type sends GET /linode/types/{typeId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "id": "g6-nanode-1",
        "label": "Nanode 1GB",
        "class": "nanode",
        "disk": 25600,
        "memory": 1024,
        "vcpus": 1,
        "gpus": 0,
        "network_out": 1000,
        "transfer": 1000,
        "price": {"hourly": 0.0075, "monthly": 5.0},
        "addons": {"backups": {"price": {"hourly": 0.003, "monthly": 2.0}}},
        "successor": None,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_type("g6-nanode-1")

    assert result.id == "g6-nanode-1"
    assert result.label == "Nanode 1GB"
    mock_request.assert_called_once_with("GET", "/linode/types/g6-nanode-1")
    await client.close()


async def test_get_type_url_encodes_type_id_at_client_boundary() -> None:
    """Client URL-encodes type_id path segments."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "bad/type?x=1"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_type("bad/type?x=1")

    assert result.id == "bad/type?x=1"
    mock_request.assert_called_once_with("GET", "/linode/types/bad%2Ftype%3Fx%3D1")
    await client.close()


async def test_get_type_requires_type_id_before_request() -> None:
    """Client rejects empty type IDs before request dispatch."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="type_id is required"),
    ):
        await client.get_type("")

    mock_request.assert_not_called()
    await client.close()


async def test_get_type_wraps_http_errors() -> None:
    """Getting a Linode type wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_type("g6-nanode-1")

    assert "GetType" in str(excinfo.value)
    await client.close()


async def test_retryable_get_type_delegates_to_client() -> None:
    """RetryableClient delegates Linode type get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(retryable.client, "get_type", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = InstanceType(
            id="g6-nanode-1",
            label="Nanode 1GB",
            class_="nanode",
            disk=25600,
            memory=1024,
            vcpus=1,
            gpus=0,
            network_out=1000,
            transfer=1000,
            price=Price(hourly=0.0075, monthly=5.0),
            addons=Addons(backups=BackupsAddon(price=Price(hourly=0.003, monthly=2.0))),
            successor=None,
        )
        result = await retryable.get_type("g6-nanode-1")

    mock_get.assert_awaited_once_with("g6-nanode-1")
    assert result.id == "g6-nanode-1"
    await retryable.close()


async def test_get_account_event_sends_exact_route() -> None:
    """Account event get sends GET /account/events/{eventId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "action": "linode_create", "status": "finished"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_event(123)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/events/123")
    await client.close()


async def test_get_account_event_url_encodes_event_id() -> None:
    """Account event get URL-encodes the event_id path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "1/2?x"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_event(cast("int", "1/2?x"))

    mock_request.assert_called_once_with("GET", "/account/events/1%2F2%3Fx")
    await client.close()


async def test_get_account_event_wraps_http_errors() -> None:
    """Account event get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_event(123)

    assert "GetAccountEvent" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_event_delegates_to_client() -> None:
    """RetryableClient delegates account event get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_event", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 123}
        result = await retryable.get_account_event(123)

    mock_get.assert_awaited_once_with(123)
    assert result == {"id": 123}
    await retryable.close()


async def test_get_account_child_account_sends_exact_route() -> None:
    """Child account get sends GET /account/child-accounts/{euuId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56",
        "company": "Example Child",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_child_account(
            "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
        )

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56"
    )
    await client.close()


async def test_get_account_child_account_url_encodes_euuid() -> None:
    """Child account get URL-encodes the euuid path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"euuid": "child/account?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_child_account("child/account?query")

    mock_request.assert_called_once_with(
        "GET", "/account/child-accounts/child%2Faccount%3Fquery"
    )
    await client.close()


async def test_get_account_child_account_wraps_http_errors() -> None:
    """Child account get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_child_account(
                "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
            )

    assert "GetAccountChildAccount" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_child_account_delegates_to_client() -> None:
    """RetryableClient delegates child account get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_child_account", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56"}
        result = await retryable.get_account_child_account(
            "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
        )

    assert result["euuid"] == "A1BC2DEF-34GH-567I-J890KLMN12O34P56"
    mock_get.assert_awaited_once_with("A1BC2DEF-34GH-567I-J890KLMN12O34P56")
    await retryable.close()


async def test_get_account_service_transfer_sends_exact_route() -> None:
    """Service transfer get sends GET /account/service-transfers/{token}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "token": "transfer-token",
        "entities": {"linodes": [123]},
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_service_transfer("transfer-token")

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/service-transfers/transfer-token"
    )
    await client.close()


async def test_get_account_service_transfer_url_encodes_token() -> None:
    """Service transfer get URL-encodes the token path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"token": "transfer/token?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_service_transfer("transfer/token?query")

    mock_request.assert_called_once_with(
        "GET", "/account/service-transfers/transfer%2Ftoken%3Fquery"
    )
    await client.close()


async def test_get_account_service_transfer_wraps_http_errors() -> None:
    """Service transfer get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_service_transfer("transfer-token")

    assert "GetAccountServiceTransfer" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_service_transfer_delegates_to_client() -> None:
    """RetryableClient delegates service transfer get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_service_transfer", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"token": "transfer-token"}
        result = await retryable.get_account_service_transfer("transfer-token")

    assert result["token"] == "transfer-token"
    mock_get.assert_awaited_once_with("transfer-token")
    await retryable.close()


async def test_get_account_oauth_client_sends_exact_route() -> None:
    """OAuth client get sends GET /account/oauth-clients/{clientId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "id": "client-123",
        "label": "Example OAuth Client",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_oauth_client("client-123")

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/oauth-clients/client-123")
    await client.close()


async def test_get_account_oauth_client_url_encodes_client_id() -> None:
    """OAuth client get URL-encodes the client_id path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "client/id?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_oauth_client("client/id?query")

    mock_request.assert_called_once_with(
        "GET", "/account/oauth-clients/client%2Fid%3Fquery"
    )
    await client.close()


async def test_route_raw_body_read_answers_with_the_bytes() -> None:
    """A raw-body read negotiates the declared type and hands back the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    thumbnail = b"\x89PNG\r\n\x1a\n"
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.headers = {"Content-Type": "image/png"}
    mock_response.content = thumbnail

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.route_raw_body_read(
            THUMBNAIL_GET_TOOL, "client-123", accept="image/png"
        )

    assert result == thumbnail
    mock_request.assert_awaited_once_with(
        "GET",
        "https://api.linode.com/v4/account/oauth-clients/client-123/thumbnail",
        headers={
            "Authorization": "Bearer test-token",
            "Accept": "image/png",
            "User-Agent": "LinodeMCP/1.0",
        },
    )
    await client.close()


async def test_route_raw_body_read_encodes_the_path_argument() -> None:
    """A raw-body read escapes its slot the way every routed primitive does."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.headers = {"Content-Type": "image/png; charset=binary"}
    mock_response.content = b"png"

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.route_raw_body_read(
            THUMBNAIL_GET_TOOL, "client/id?query", accept="image/png"
        )

    assert result == b"png"
    await_args = mock_request.await_args
    assert await_args is not None
    assert await_args.args == (
        "GET",
        "https://api.linode.com/v4/account/oauth-clients/client%2Fid%3Fquery/thumbnail",
    )
    await client.close()


async def test_route_raw_body_read_maps_http_status_errors() -> None:
    """A failing status is read off the same bytes the answer would have been,
    so the API's own reason survives instead of an empty body."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 404
    mock_response.headers = {"Content-Type": "application/json"}
    mock_response.json.return_value = {"errors": [{"reason": "Not found"}]}
    mock_response.content = b'{"errors":[{"reason":"Not found"}]}'

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        with pytest.raises(APIError, match="Not found"):
            await client.route_raw_body_read(
                THUMBNAIL_GET_TOOL, "client-123", accept="image/png"
            )

    mock_request.assert_awaited_once()
    await client.close()


async def test_retryable_route_raw_body_read_delegates() -> None:
    """The read declares no retry policy of its own, so it replays by default."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "route_raw_body_read", new_callable=AsyncMock
    ) as mock_read:
        mock_read.return_value = b"png"

        result = await retryable.route_raw_body_read(
            THUMBNAIL_GET_TOOL, "client-123", accept="image/png"
        )

    assert result == b"png"
    mock_read.assert_awaited_once()
    await retryable.close()


async def test_get_account_oauth_client_wraps_http_errors() -> None:
    """OAuth client get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_oauth_client("client-123")

    assert "GetAccountOAuthClient" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_oauth_client_delegates_to_client() -> None:
    """RetryableClient delegates OAuth client get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_oauth_client", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": "client-123"}
        result = await retryable.get_account_oauth_client("client-123")

    assert result["id"] == "client-123"
    mock_get.assert_awaited_once_with("client-123")
    await retryable.close()


async def test_get_instance_ip_url_encodes_address() -> None:
    """Instance IP get URL-encodes the address path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"address": "2001:db8::1"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_instance_ip(123, "2001:db8::1")

    call_args = mock_request.call_args
    assert call_args[0][1] == "/linode/instances/123/ips/2001:db8::1"

    await client.close()


async def test_list_instance_configs() -> None:
    """List Linode instance configs sends GET to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"id": 6, "label": "boot-config"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_instance_configs(123)

    assert result["data"][0]["label"] == "boot-config"
    mock_request.assert_called_once_with("GET", "/linode/instances/123/configs")
    await client.close()


async def test_list_instance_configs_with_pagination() -> None:
    """List Linode instance configs forwards pagination query params."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [], "page": 2, "pages": 3}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_instance_configs(123, page=2, page_size=50)

    assert result == {"data": [], "page": 2, "pages": 3}
    mock_request.assert_called_once_with(
        "GET", "/linode/instances/123/configs?page=2&page_size=50"
    )
    await client.close()


@pytest.mark.parametrize(
    ("linode_id", "encoded"),
    [
        ("1/2", "1%2F2"),
        ("1?x", "1%3Fx"),
        ("..", "%2E%2E"),
    ],
)
async def test_list_instance_configs_encodes_path_params(
    linode_id: str, encoded: str
) -> None:
    """Linode instance config list path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_instance_configs(cast("Any", linode_id))

    mock_request.assert_called_once_with("GET", f"/linode/instances/{encoded}/configs")
    await client.close()


async def test_list_instance_configs_wraps_http_errors() -> None:
    """List Linode instance configs wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_instance_configs(123)

    assert "ListInstanceConfigs" in str(excinfo.value)
    await client.close()


async def test_retryable_list_instance_configs_delegates_to_client() -> None:
    """RetryableClient delegates Linode instance config list with retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_instance_configs", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_instance_configs(123, page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(123, page=1, page_size=100)
    await retryable.close()


async def test_get_instance_config() -> None:
    """Get Linode instance config sends GET to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 6, "label": "boot-config"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_instance_config(123, 6)

    assert result["label"] == "boot-config"
    mock_request.assert_called_once_with("GET", "/linode/instances/123/configs/6")
    await client.close()


@pytest.mark.parametrize(
    ("linode_id", "config_id", "expected"),
    [
        ("1/2", 6, "/linode/instances/1%2F2/configs/6"),
        ("1?x", 6, "/linode/instances/1%3Fx/configs/6"),
        (123, "6/7", "/linode/instances/123/configs/6%2F7"),
        (123, "6?x", "/linode/instances/123/configs/6%3Fx"),
    ],
)
async def test_get_instance_config_escapes_path_separators(
    linode_id: Any, config_id: Any, expected: str
) -> None:
    """Linode instance config get escapes path separators and query markers."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 6}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_instance_config(linode_id, config_id)

    mock_request.assert_called_once_with("GET", expected)
    await client.close()


async def test_get_instance_config_wraps_http_errors() -> None:
    """Get Linode instance config wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_instance_config(123, 6)

    assert "GetInstanceConfig" in str(excinfo.value)
    await client.close()


async def test_retryable_get_instance_config_delegates_to_client() -> None:
    """RetryableClient delegates Linode instance config get with retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_instance_config", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 6, "label": "boot-config"}
        result = await retryable.get_instance_config(123, 6)

    assert result["id"] == 6
    mock_get.assert_awaited_once_with(123, 6)
    await retryable.close()


async def test_get_instance_config_interface() -> None:
    """Get Linode instance config interface sends GET to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 9, "purpose": "vlan"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_instance_config_interface(123, 6, 9)

    assert result["purpose"] == "vlan"
    mock_request.assert_called_once_with(
        "GET", "/linode/instances/123/configs/6/interfaces/9"
    )
    await client.close()


@pytest.mark.parametrize(
    "arguments",
    [
        ("1/2", 6, 9),
        ("1?x", 6, 9),
        ("..", 6, 9),
        (0, 6, 9),
        (True, 6, 9),
        (123, "6/7", 9),
        (123, "6?x", 9),
        (123, "..", 9),
        (123, 0, 9),
        (123, True, 9),
        (123, 6, "9/10"),
        (123, 6, "9?x"),
        (123, 6, ".."),
        (123, 6, 0),
        (123, 6, True),
    ],
)
async def test_get_instance_config_interface_rejects_malformed_path_params(
    arguments: tuple[Any, Any, Any],
) -> None:
    """Linode instance config interface get rejects malformed path parameters."""
    client = Client("https://api.linode.com/v4", "test-token")
    linode_id, config_id, interface_id = arguments

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="positive integer"),
    ):
        await client.get_instance_config_interface(linode_id, config_id, interface_id)

    mock_request.assert_not_called()
    await client.close()


async def test_get_instance_config_interface_wraps_http_errors() -> None:
    """Get Linode instance config interface wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_instance_config_interface(123, 6, 9)

    assert "GetInstanceConfigInterface" in str(excinfo.value)
    await client.close()


async def test_retryable_get_instance_config_interface_delegates_to_client() -> None:
    """RetryableClient delegates Linode instance config interface get with retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_instance_config_interface", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 9, "purpose": "vlan"}
        result = await retryable.get_instance_config_interface(123, 6, 9)

    assert result["id"] == 9
    mock_get.assert_awaited_once_with(123, 6, 9)
    await retryable.close()


async def test_get_instance(sample_instance_data: dict[str, Any]) -> None:
    """Test getting specific instance."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = sample_instance_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        instance = await client.get_instance(123456)

        assert instance.id == 123456
        assert instance.label == "test-instance"

    await client.close()


async def test_list_tagged_objects_sends_get_to_tag_route() -> None:
    """Test listing tagged objects sends GET /tags/{tagLabel}."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [
            {
                "type": "linode",
                "data": {"id": 123, "label": "web-1"},
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_tagged_objects("team/blue tag", page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/tags/team%2Fblue%20tag?page=2&page_size=25"
    )
    await client.close()


async def test_list_tagged_objects_wraps_http_errors() -> None:
    """Test listing tagged objects wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_tagged_objects("production")

    assert "ListTaggedObjects" in str(excinfo.value)
    await client.close()


async def test_retryable_list_tagged_objects_delegates_to_client() -> None:
    """Test RetryableClient delegates tagged object listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_tagged_objects", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_tagged_objects(
            "production", page=1, page_size=100
        )

    assert result["data"] == []
    mock_list.assert_awaited_once_with("production", page=1, page_size=100)
    await retryable.close()


async def test_get_managed_credential_sends_get_to_managed_credential_route() -> None:
    """Managed credential get sends GET to the documented route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "label": "db-root"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_managed_credential(123)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/managed/credentials/123")
    await client.close()


@pytest.mark.parametrize("credential_id", [0, -1, True, "/", "1?", ".."])
async def test_get_managed_credential_rejects_invalid_credential_id(
    credential_id: object,
) -> None:
    """Managed credential get validates finite integer IDs before dispatch."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="credential_id must be a positive integer"),
    ):
        await client.get_managed_credential(cast("int", credential_id))

    mock_request.assert_not_called()
    await client.close()


async def test_get_managed_credential_wraps_http_errors() -> None:
    """Managed credential get maps HTTP errors to GetManagedCredential."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_managed_credential(123)

    assert "GetManagedCredential" in str(excinfo.value)
    await client.close()


async def test_retryable_get_managed_credential_delegates_to_client() -> None:
    """RetryableClient delegates Managed credential retrieval."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_managed_credential", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 123}
        result = await retryable.get_managed_credential(123)

    mock_get.assert_awaited_once_with(123)
    assert result == {"id": 123}
    await retryable.close()


async def test_get_managed_linode_settings_sends_exact_route() -> None:
    """Managed Linode settings sends the documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {
        "ssh": {"access": True, "user": "linode"},
        "group": "web",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_managed_linode_settings(123)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/managed/linode-settings/123")
    await client.close()


@pytest.mark.parametrize("bad_linode_id", [0, -1, True, "1/2", "1?x", ".."])
async def test_get_managed_linode_settings_rejects_invalid_linode_id(
    bad_linode_id: object,
) -> None:
    """Managed Linode settings validates the path ID before request dispatch."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises((TypeError, ValueError)),
    ):
        await client.get_managed_linode_settings(bad_linode_id)  # type: ignore[arg-type]

    mock_request.assert_not_called()
    await client.close()


async def test_get_managed_linode_settings_wraps_http_errors() -> None:
    """Managed Linode settings wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_managed_linode_settings(123)

    assert "GetManagedLinodeSettings" in str(excinfo.value)
    await client.close()


async def test_retryable_get_managed_linode_settings_delegates_to_client() -> None:
    """RetryableClient delegates Managed Linode settings retrieval."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_managed_linode_settings", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"ssh": {"access": True}}
        result = await retryable.get_managed_linode_settings(123)

    assert result == {"ssh": {"access": True}}
    mock_get.assert_awaited_once_with(123)
    await retryable.close()


async def test_get_managed_service_sends_get_to_service_route() -> None:
    """Test Managed service retrieval sends documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 314, "label": "web monitor"}
    mock_response = MagicMock()
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_managed_service(314)

    assert result == response_data
    mock_request.assert_awaited_once_with("GET", "/managed/services/314")
    await client.close()


@pytest.mark.parametrize("bad_service_id", [0, -1, True, "1/2", "1?x", ".."])
async def test_get_managed_service_rejects_bad_service_id(
    bad_service_id: object,
) -> None:
    """Test Managed service retrieval rejects malformed service IDs."""
    client = Client("https://api.linode.com/v4", "test-token")

    get_managed_service = cast("Any", client.get_managed_service)
    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="service_id must be a positive integer"),
    ):
        await get_managed_service(bad_service_id)

    mock_request.assert_not_called()
    await client.close()


async def test_get_managed_service_wraps_http_errors() -> None:
    """Test Managed service retrieval wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_managed_service(314)

    assert "GetManagedService" in str(excinfo.value)
    await client.close()


async def test_retryable_get_managed_service_delegates_to_client() -> None:
    """Test RetryableClient delegates Managed service retrieval."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_managed_service", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 314}
        result = await retryable.get_managed_service(314)

    assert result == {"id": 314}
    mock_get.assert_awaited_once_with(314)
    await retryable.close()


async def test_get_managed_contact_sends_get_to_contact_route() -> None:
    """Test Managed contact retrieval sends documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "id": 42,
        "name": "Primary on-call",
        "email": "ops@example.com",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_managed_contact(42)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/managed/contacts/42")
    await client.close()


@pytest.mark.parametrize("bad_contact_id", [0, -1, True, "1/2", "1?x", ".."])
async def test_get_managed_contact_rejects_bad_contact_id(
    bad_contact_id: object,
) -> None:
    """Test Managed contact retrieval rejects malformed contact IDs."""
    client = Client("https://api.linode.com/v4", "test-token")

    get_managed_contact = cast("Any", client.get_managed_contact)
    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="contact_id must be a positive integer"),
    ):
        await get_managed_contact(bad_contact_id)

    mock_request.assert_not_called()
    await client.close()


async def test_get_managed_contact_wraps_http_errors() -> None:
    """Test Managed contact retrieval wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_managed_contact(42)

    assert "GetManagedContact" in str(excinfo.value)
    await client.close()


async def test_retryable_get_managed_contact_delegates_to_client() -> None:
    """Test RetryableClient delegates Managed contact retrieval."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_managed_contact", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 42}
        result = await retryable.get_managed_contact(42)

    assert result == {"id": 42}
    mock_get.assert_awaited_once_with(42)
    await retryable.close()


async def test_get_support_ticket_sends_get_to_ticket_route() -> None:
    """Test support ticket retrieval sends documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {"id": 123, "summary": "Need help"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_support_ticket(123)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/support/tickets/123")
    await client.close()


async def test_get_support_ticket_wraps_http_errors() -> None:
    """Test support ticket retrieval wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_support_ticket(123)

    assert "GetSupportTicket" in str(excinfo.value)
    await client.close()


async def test_retryable_get_support_ticket_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket retrieval."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_support_ticket", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 123}
        result = await retryable.get_support_ticket(123)

    assert result == {"id": 123}
    mock_get.assert_awaited_once_with(123)
    await retryable.close()


async def test_route_multipart_sends_the_file_contents(tmp_path: Any) -> None:
    """A multipart route frames the named file into the declared form field."""
    client = Client("https://api.linode.com/v4", "test-token")
    attachment = tmp_path / "attachment.txt"
    attachment.write_text("attachment-content")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.content = b"{}"

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.route_multipart(
            ATTACHMENT_TOOL, 123, part_name="file", file_path=str(attachment)
        )

    mock_request.assert_awaited_once()
    await_args = mock_request.await_args
    assert await_args is not None
    args = await_args.args
    kwargs = await_args.kwargs
    assert args == (
        "POST",
        "https://api.linode.com/v4/support/tickets/123/attachments",
    )
    headers = kwargs["headers"]
    content_type = headers["Content-Type"]
    assert content_type.startswith("multipart/form-data; boundary=")
    assert headers["Authorization"] == "Bearer test-token"
    assert headers["User-Agent"] == "LinodeMCP/1.0"
    assert set(headers) == {"Authorization", "Content-Type", "User-Agent"}

    # Pinned byte for byte around the random boundary, so the routed request
    # keeps sending exactly what the files= upload sent.
    boundary = content_type.removeprefix("multipart/form-data; boundary=")
    assert kwargs["content"] == (
        f"--{boundary}\r\n"
        'Content-Disposition: form-data; name="file"; filename="attachment.txt"\r\n'
        "Content-Type: text/plain\r\n"
        "\r\n"
        "attachment-content\r\n"
        f"--{boundary}--\r\n"
    ).encode("ascii")

    await client.close()


async def test_retryable_route_multipart_can_take_one_protected_attempt() -> None:
    """The attachment declares retry_disabled, and the caller selects the
    unprotected executor rather than the client deciding for it."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with (
        patch.object(
            retryable.client, "route_multipart", new_callable=AsyncMock
        ) as mock_send,
        patch.object(
            retryable, "_execute_with_retry", new_callable=AsyncMock
        ) as mock_retry,
    ):
        await retryable.route_multipart(
            ATTACHMENT_TOOL,
            123,
            part_name="file",
            file_path="/Users/e/a.txt",
            retry=False,
        )

    mock_send.assert_awaited_once()
    mock_retry.assert_not_called()
    await retryable.close()


async def test_get_volume_sends_get_to_volume_route() -> None:
    """Test getting a volume sends GET /volumes/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {
        "id": 12345,
        "label": "data-volume",
        "status": "active",
        "size": 20,
        "region": "us-east",
        "linode_id": None,
        "linode_label": None,
        "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_data-volume",
        "created": "2024-01-15T10:00:00",
        "updated": "2024-01-15T12:00:00",
        "tags": ["prod"],
        "hardware_type": "nvme",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        volume = await client.get_volume(12345)

    assert volume.id == 12345
    assert volume.label == "data-volume"
    assert volume.tags == ["prod"]
    mock_request.assert_called_once_with("GET", "/volumes/12345")

    await client.close()


async def test_get_instance_parses_interfaces(
    sample_instance_data: dict[str, Any],
) -> None:
    """A GET /linode/instances/{id} response carrying interface_generation +
    interfaces[] must surface those fields on the parsed Instance.
    """
    response_data = {
        **sample_instance_data,
        "interface_generation": "linode",
        "interfaces": [
            {
                "id": 1,
                "public": {},
                "default_route": {"ipv4": True, "ipv6": True},
                "firewall_id": 12345,
            },
        ],
    }

    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        instance = await client.get_instance(123456)

    assert instance.interface_generation == "linode"
    assert len(instance.interfaces) == 1
    assert instance.interfaces[0].id == 1
    assert instance.interfaces[0].firewall_id == 12345
    assert instance.interfaces[0].default_route is not None
    assert instance.interfaces[0].default_route.ipv4 is True
    assert instance.interfaces[0].default_route.ipv6 is True

    await client.close()


async def _instance_from_response(data: dict[str, Any]) -> Instance:
    """Parse an Instance through the client's get_instance with a mocked HTTP
    layer, so the test exercises the real _parse_instance path.
    """
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = data
    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        instance = await client.get_instance(123456)
    await client.close()
    return instance


async def test_parse_instance_interface_parses_public_vpc_vlan_subconfigs(
    sample_instance_data: dict[str, Any],
) -> None:
    """A read must surface the public ipv4/ipv6, vpc, and vlan sub-configs plus
    the created/updated/version fields, matching the Go client's struct-tag
    deserialization of Instance.Interfaces.
    """
    response_data = {
        **sample_instance_data,
        "interface_generation": "linode",
        "interfaces": [
            {
                "id": 1,
                "public": {
                    "ipv4": {"addresses": [{"address": "1.2.3.4", "primary": True}]},
                    "ipv6": {"ranges": [{"range": "2600::/64"}]},
                },
                "default_route": {"ipv4": True},
            },
            {
                "id": 2,
                "vpc": {
                    "subnet_id": 99,
                    "ipv4": {"addresses": [{"address": "10.0.0.5"}]},
                },
                "firewall_id": 777,
            },
            {
                "id": 3,
                "vlan": {"vlan_label": "lan-1", "ipam_address": "10.0.0.6/24"},
                "mac_address": "aa:bb:cc:dd:ee:ff",
                "created": "2026-01-01T00:00:00",
                "updated": "2026-01-02T00:00:00",
                "version": 2,
            },
        ],
    }

    instance = await _instance_from_response(response_data)

    public = instance.interfaces[0].public
    assert public is not None
    assert public.ipv4 is not None
    assert public.ipv4.addresses[0].address == "1.2.3.4"
    assert public.ipv4.addresses[0].primary is True
    assert public.ipv6 is not None
    assert public.ipv6.ranges[0].range == "2600::/64"

    vpc = instance.interfaces[1].vpc
    assert vpc is not None
    assert vpc.subnet_id == 99
    assert vpc.ipv4 is not None
    assert vpc.ipv4.addresses[0].address == "10.0.0.5"
    assert vpc.ipv4.addresses[0].primary is False

    third = instance.interfaces[2]
    assert third.vlan is not None
    assert third.vlan.vlan_label == "lan-1"
    assert third.vlan.ipam_address == "10.0.0.6/24"
    assert third.mac_address == "aa:bb:cc:dd:ee:ff"
    assert third.created == "2026-01-01T00:00:00"
    assert third.updated == "2026-01-02T00:00:00"
    assert third.version == 2

    # Round-trip the same interfaces through the serializer and confirm the
    # vpc, vlan, firewall_id, and default-route-ipv4-only forms match Go's
    # omitempty output.
    assert instance_to_response_dict(instance)["interfaces"] == [
        {
            "id": 1,
            "public": {
                "ipv4": {"addresses": [{"address": "1.2.3.4", "primary": True}]},
                "ipv6": {"ranges": [{"range": "2600::/64"}]},
            },
            "default_route": {"ipv4": True},
        },
        {
            "id": 2,
            "vpc": {"subnet_id": 99, "ipv4": {"addresses": [{"address": "10.0.0.5"}]}},
            "firewall_id": 777,
        },
        {
            "id": 3,
            "vlan": {"vlan_label": "lan-1", "ipam_address": "10.0.0.6/24"},
            "mac_address": "aa:bb:cc:dd:ee:ff",
            "created": "2026-01-01T00:00:00",
            "updated": "2026-01-02T00:00:00",
            "version": 2,
        },
    ]


async def test_instance_to_response_dict_matches_go_interface_shape(
    sample_instance_data: dict[str, Any],
) -> None:
    """instance_to_response_dict emits the proto-canonical shape: the full message
    in field order, with interface_generation and the deep interface subtree
    present, last_successful omitted when absent, and interfaces always present.
    """
    response_data = {
        **sample_instance_data,
        "interface_generation": "linode",
        "interfaces": [
            {
                "id": 1234,
                "mac_address": "22:00:AB:CD:EF:01",
                "version": 1,
                "public": {
                    "ipv4": {"addresses": [{"address": "172.30.0.50", "primary": True}]}
                },
                "default_route": {"ipv4": True, "ipv6": True},
            }
        ],
    }

    instance = await _instance_from_response(response_data)
    result = instance_to_response_dict(instance)

    assert list(result.keys()) == [
        "id",
        "label",
        "status",
        "type",
        "region",
        "image",
        "ipv4",
        "ipv6",
        "hypervisor",
        "specs",
        "alerts",
        "backups",
        "created",
        "updated",
        "group",
        "tags",
        "watchdog_enabled",
        "interface_generation",
        "interfaces",
    ]
    assert result["interface_generation"] == "linode"
    assert result["interfaces"] == [
        {
            "id": 1234,
            "public": {
                "ipv4": {"addresses": [{"address": "172.30.0.50", "primary": True}]}
            },
            "default_route": {"ipv4": True, "ipv6": True},
            "mac_address": "22:00:AB:CD:EF:01",
            "version": 1,
        }
    ]
    assert set(result["specs"].keys()) == {
        "disk",
        "memory",
        "vcpus",
        "gpus",
        "transfer",
    }
    assert list(result["backups"].keys()) == [
        "schedule",
        "enabled",
        "available",
    ]


async def test_instance_to_response_dict_omits_empty_interface_fields(
    sample_instance_data: dict[str, Any],
) -> None:
    """With no interfaces, interface_generation is omitted (proto optional), while
    interfaces is always present as an empty list (proto repeated under
    EmitDefaultValues), and the always-present struct fields stay.
    """
    instance = await _instance_from_response(dict(sample_instance_data))
    result = instance_to_response_dict(instance)

    assert result["interfaces"] == []
    assert "interface_generation" not in result
    assert "watchdog_enabled" in result
    assert "specs" in result
    assert "backups" in result


async def test_api_error_401() -> None:
    """Test handling 401 authentication error."""
    client = Client("https://api.linode.com/v4", "bad-token")

    mock_response = MagicMock()
    mock_response.status_code = 401
    mock_response.json.return_value = {}
    mock_response.headers = {}

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        with pytest.raises(APIError) as exc_info:
            await client.make_request("GET", "/profile")

        assert exc_info.value.status_code == 401
        assert "Authentication failed" in str(exc_info.value)

    await client.close()


async def test_api_error_429() -> None:
    """Test handling 429 rate limit error."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 429
    mock_response.json.return_value = {}
    mock_response.headers = {"Retry-After": "60"}

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        with pytest.raises(APIError) as exc_info:
            await client.make_request("GET", "/profile")

        assert exc_info.value.status_code == 429
        assert exc_info.value.is_rate_limit_error()

    await client.close()


async def test_api_error_500() -> None:
    """Test handling 500 server error."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 500
    mock_response.json.return_value = {}
    mock_response.headers = {}

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        with pytest.raises(APIError) as exc_info:
            await client.make_request("GET", "/profile")

        assert exc_info.value.status_code == 500
        assert exc_info.value.is_server_error()

    await client.close()


async def test_network_error() -> None:
    """Test network error handling."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ConnectError("Connection failed")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_profile()

        assert "GetProfile" in str(exc_info.value)

    await client.close()


async def test_get_object_storage_bucket_encodes_path_params() -> None:
    """Object Storage bucket get URL-encodes path parameters."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"label": "bucket"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.get_object_storage_bucket("us/east?1", "bad/bucket?x")

    mock_request.assert_awaited_once_with(
        "GET", "/object-storage/buckets/us%2Feast%3F1/bad%2Fbucket%3Fx"
    )
    await client.close()


def test_deprecated_object_storage_cluster_client_methods_absent() -> None:
    """Deprecated Object Storage cluster client methods should be removed.

    The region read that replaced them is a generated tool now, so no typed
    replacement method is pinned here anymore.
    """
    assert not hasattr(Client, "list_object_storage_clusters")
    assert not hasattr(RetryableClient, "list_object_storage_clusters")
    assert not hasattr(Client, "get_object_storage_cluster")
    assert not hasattr(RetryableClient, "get_object_storage_cluster")


async def test_get_placement_group_sends_get() -> None:
    """Getting a placement group should issue GET for the group path."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 789, "label": "pg-a"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_placement_group(789)

        assert result == {"id": 789, "label": "pg-a"}
        mock_request.assert_awaited_once_with("GET", "/placement/groups/789")

    await client.close()


async def test_get_placement_group_encodes_group_path() -> None:
    """Placement group get should encode the group path segment."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}
    group_id: Any = "12/../?x=1"

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.get_placement_group(group_id)

        mock_request.assert_awaited_once_with(
            "GET",
            "/placement/groups/12%2F..%2F%3Fx%3D1",
        )

    await client.close()


async def test_get_placement_group_wraps_http_errors() -> None:
    """Getting a placement group should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_placement_group(789)

    assert "GetPlacementGroup" in str(exc_info.value)
    await client.close()


async def test_retryable_get_placement_group_delegates_to_client() -> None:
    """Retryable client should delegate placement group get."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_data = {"id": 789, "label": "pg-a"}

    with patch.object(
        client.client,
        "get_placement_group",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = response_data

        result = await client.get_placement_group(789)

        assert result == response_data
        mock_get.assert_awaited_once_with(789)

    await client.close()


async def test_get_ipv6_range_encodes_range_path() -> None:
    """Getting an IPv6 range should encode the complete path segment."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "range": "2001:0db8::",
        "region": "us-east",
        "prefix": 64,
    }
    mock_response = MagicMock()
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_ipv6_range("2001:0db8::/64")

        assert result == response_data
        mock_request.assert_awaited_once_with(
            "GET",
            "/networking/ipv6/ranges/2001:0db8::%2F64",
        )

    await client.close()


async def test_retryable_get_ipv6_range_delegates_to_client() -> None:
    """Retryable client should delegate IPv6 range retrieval."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_data = {"range": "2001:0db8::", "region": "us-east"}

    with patch.object(
        client.client,
        "get_ipv6_range",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = response_data

        result = await client.get_ipv6_range("2001:0db8::/64")

        assert result == response_data
        mock_get.assert_awaited_once_with("2001:0db8::/64")

    await client.close()


async def test_retryable_client_success(sample_profile_data: dict[str, Any]) -> None:
    """Test retryable client successful request."""
    client = RetryableClient(
        "https://api.linode.com/v4", "test-token", RetryConfig(max_retries=3)
    )

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = sample_profile_data

    with patch.object(
        client.client, "make_request", new_callable=AsyncMock
    ) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.get_profile()

        assert profile.username == "testuser"

    await client.close()


async def test_retryable_client_list_vlans() -> None:
    """Test retryable client delegates VLAN listing."""
    client = RetryableClient(
        "https://api.linode.com/v4", "test-token", RetryConfig(max_retries=3)
    )
    expected_vlans = [{"label": "app-vlan", "region": "us-east"}]

    with patch.object(
        client.client, "list_vlans", new_callable=AsyncMock
    ) as mock_list_vlans:
        mock_list_vlans.return_value = expected_vlans

        vlans = await client.list_vlans()

        assert vlans == expected_vlans
        mock_list_vlans.assert_awaited_once_with(page=None, page_size=None)

    await client.close()


async def test_retryable_client_retry_on_rate_limit(
    sample_profile_data: dict[str, Any],
) -> None:
    """Test retryable client retries on rate limit."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=2, base_delay=0.01),
    )

    mock_error_response = MagicMock()
    mock_error_response.status_code = 429
    mock_error_response.json.return_value = {}
    mock_error_response.headers = {}

    mock_success_response = MagicMock()
    mock_success_response.status_code = 200
    mock_success_response.json.return_value = sample_profile_data

    call_count = 0

    async def mock_request(*args: Any, **kwargs: Any) -> MagicMock:
        nonlocal call_count
        call_count += 1
        if call_count == 1:
            return mock_error_response
        return mock_success_response

    with patch.object(
        client.client.client, "request", new_callable=AsyncMock
    ) as mock_req:
        mock_req.side_effect = mock_request

        profile = await client.get_profile()

        assert profile.username == "testuser"
        assert call_count == 2

    await client.close()


async def test_retryable_client_max_retries_exceeded() -> None:
    """Test retryable client fails after max retries."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=2, base_delay=0.01),
    )

    mock_response = MagicMock()
    mock_response.status_code = 500
    mock_response.json.return_value = {}
    mock_response.headers = {}

    with patch.object(
        client.client.client, "request", new_callable=AsyncMock
    ) as mock_request:
        mock_request.return_value = mock_response

        with pytest.raises(APIError):
            await client.get_profile()

    await client.close()


def test_is_retryable_api_error() -> None:
    """Test is_retryable with API errors."""
    assert is_retryable(APIError(429, "Rate limit"))
    assert is_retryable(APIError(500, "Server error"))
    assert not is_retryable(APIError(401, "Unauthorized"))
    assert not is_retryable(APIError(403, "Forbidden"))


def test_is_retryable_network_error() -> None:
    """Test is_retryable with network errors."""
    assert is_retryable(NetworkError("operation", Exception("error")))


async def test_get_account_payment_method_sends_get_to_exact_route() -> None:
    """Getting an account payment method sends GET /account/payment-methods/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "type": "credit_card"}
    response = MagicMock()
    response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_account_payment_method(123)

    assert result == response_data
    mock_request.assert_awaited_once_with("GET", "/account/payment-methods/123")
    await client.close()


async def test_get_account_payment_method_encodes_path_param() -> None:
    """Client URL-encodes the payment method path parameter boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "123/456?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_account_payment_method("123/456?query")  # type: ignore[arg-type]

    assert result == {"id": "123/456?query"}
    mock_request.assert_awaited_once_with(
        "GET", "/account/payment-methods/123%2F456%3Fquery"
    )
    await client.close()


async def test_get_account_payment_method_wraps_http_errors() -> None:
    """HTTP errors from payment method reads are wrapped."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_payment_method(123)

    assert "GetAccountPaymentMethod" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_payment_method_delegates_to_client() -> None:
    """Retryable payment method get delegates to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "type": "credit_card"}

    with patch.object(
        client.client, "get_account_payment_method", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = response_data

        result = await client.get_account_payment_method(123)

    assert result == response_data
    mock_get.assert_awaited_once_with(123)
    await client.close()


def test_api_error_methods() -> None:
    """Test APIError helper methods."""
    auth_error = APIError(401, "Unauthorized")
    assert auth_error.is_authentication_error()
    assert not auth_error.is_rate_limit_error()

    rate_error = APIError(429, "Rate limit")
    assert rate_error.is_rate_limit_error()
    assert not rate_error.is_server_error()

    server_error = APIError(500, "Server error")
    assert server_error.is_server_error()
    assert not server_error.is_forbidden_error()


async def test_get_ssh_key_sends_get_to_profile_route() -> None:
    """Test getting an SSH key sends GET /profile/sshkeys/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 12345,
        "label": "work-key",
        "ssh_key": "ssh-rsa AAAA...",
        "created": "2024-01-01T00:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        key = await client.get_ssh_key(12345)

        assert key.id == 12345
        assert key.label == "work-key"
        mock_request.assert_awaited_once_with("GET", "/profile/sshkeys/12345")

    await client.close()


async def test_get_domain() -> None:
    """Test getting a specific domain."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 1,
        "domain": "example.com",
        "type": "master",
        "status": "active",
        "soa_email": "admin@example.com",
        "description": "Test",
        "tags": [],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        domain = await client.get_domain(1)

        assert domain.id == 1
        assert domain.domain == "example.com"

    await client.close()


async def test_list_domain_records() -> None:
    """Test listing domain records."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "id": 1,
                "type": "A",
                "name": "www",
                "target": "192.0.2.1",
                "priority": 0,
                "weight": 0,
                "port": 0,
                "ttl_sec": 300,
                "created": "2024-01-01T00:00:00",
                "updated": "2024-01-15T12:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        records = await client.list_domain_records(1)

        assert len(records) == 1
        assert records[0].id == 1
        assert records[0].type == "A"

    await client.close()


async def test_get_domain_record() -> None:
    """Test getting a domain record."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 1,
        "type": "A",
        "name": "www",
        "target": "192.0.2.1",
        "priority": 0,
        "weight": 0,
        "port": 0,
        "ttl_sec": 300,
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        record = await client.get_domain_record(1, 2)

        assert record.id == 1
        assert record.type == "A"
        assert record.name == "www"
        mock_request.assert_awaited_once_with("GET", "/domains/1/records/2")

    await client.close()


async def test_get_firewall_settings() -> None:
    """Test listing default firewall settings."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {
        "default_firewall_ids": {
            "linode": 100,
            "nodebalancer": 101,
            "public_interface": 200,
            "vpc_interface": 201,
        },
        "page": 2,
        "pages": 4,
        "results": 1,
    }

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = payload

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_firewall_settings(page=2, page_size=25)

    assert result == payload
    mock_request.assert_awaited_once_with(
        "GET", "/networking/firewalls/settings?page=2&page_size=25"
    )
    await client.close()


@pytest.mark.parametrize(
    ("kwargs", "message"),
    [
        ({"page": 0}, "page must be a positive integer"),
        ({"page": -1}, "page must be a positive integer"),
        ({"page": True}, "page must be a positive integer"),
        ({"page": "bad"}, "page must be a positive integer"),
        ({"page_size": 0}, "page_size must be a positive integer"),
        ({"page_size": -1}, "page_size must be a positive integer"),
        ({"page_size": True}, "page_size must be a positive integer"),
        ({"page_size": "bad"}, "page_size must be a positive integer"),
    ],
)
async def test_get_firewall_settings_rejects_invalid_pagination(
    kwargs: dict[str, Any], message: str
) -> None:
    """Test default firewall settings list validates pagination."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match=message),
    ):
        await client.get_firewall_settings(**kwargs)

    mock_request.assert_not_called()
    await client.close()


async def test_get_firewall_settings_wraps_http_errors() -> None:
    """Test default firewall settings list wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="GetFirewallSettings"):
            await client.get_firewall_settings()

    await client.close()


async def test_retryable_get_firewall_settings_uses_retry() -> None:
    """Test RetryableClient wraps default firewall settings list in retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable, "_execute_with_retry", new_callable=AsyncMock
    ) as mock_retry:
        mock_retry.return_value = {"default_firewall_ids": {"linode": 100}}

        result = await retryable.get_firewall_settings(page=2, page_size=25)

    assert result == {"default_firewall_ids": {"linode": 100}}
    mock_retry.assert_awaited_once_with(retryable.client.get_firewall_settings, 2, 25)
    await retryable.close()


async def test_get_firewall() -> None:
    """Test getting a firewall."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 12345,
        "label": "web-fw",
        "status": "enabled",
        "rules": {
            "inbound": [],
            "outbound": [],
            "inbound_policy": "DROP",
            "outbound_policy": "ACCEPT",
        },
        "tags": ["production"],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        firewall = await client.get_firewall(12345)

        assert firewall.id == 12345
        assert firewall.label == "web-fw"
        assert firewall.tags == ["production"]
        mock_request.assert_awaited_once_with("GET", "/networking/firewalls/12345")

    await client.close()


async def test_get_firewall_rules() -> None:
    """Test getting firewall rules."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "inbound": [
            {
                "action": "ACCEPT",
                "protocol": "TCP",
                "ports": "22",
                "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
                "label": "allow-ssh",
                "description": "",
            }
        ],
        "inbound_policy": "DROP",
        "outbound": [],
        "outbound_policy": "ACCEPT",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        rules = await client.get_firewall_rules(12345)

        assert isinstance(rules, FirewallRules)
        assert len(rules.inbound) == 1
        assert rules.inbound[0].action == "ACCEPT"
        assert rules.inbound_policy == "DROP"
        assert rules.outbound == []
        assert rules.outbound_policy == "ACCEPT"
        mock_request.assert_awaited_once_with(
            "GET", "/networking/firewalls/12345/rules"
        )

    await client.close()


async def test_get_firewall_rules_wraps_http_errors() -> None:
    """Test get_firewall_rules wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=MagicMock(status_code=404)
        )

        with pytest.raises(NetworkError):
            await client.get_firewall_rules(12345)

    await client.close()


async def test_retryable_get_firewall_rules_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall rules get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    mock_rules = FirewallRules(
        inbound=[],
        inbound_policy="DROP",
        outbound=[],
        outbound_policy="ACCEPT",
    )

    with patch.object(
        retryable.client, "get_firewall_rules", new_callable=AsyncMock
    ) as mock_method:
        mock_method.return_value = mock_rules

        result = await retryable.get_firewall_rules(12345)

        assert result is mock_rules
        mock_method.assert_awaited_once_with(12345)

    await retryable.close()


async def test_get_object_storage_bucket_access_wraps_http_error() -> None:
    """Bucket access fetch wraps HTTP errors as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_object_storage_bucket_access("us-east-1", "my-bucket")

    assert "GetObjectStorageBucketAccess" in str(exc_info.value)

    await client.close()


async def test_list_vlans() -> None:
    """Test listing VLANs."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "label": "app-vlan",
                "region": "us-east",
                "linodes": [123],
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        vlans = await client.list_vlans()

        assert vlans == mock_response.json.return_value["data"]
        mock_request.assert_awaited_once_with("GET", "/networking/vlans")

    await client.close()


async def test_get_nodebalancer() -> None:
    """Test getting a specific nodebalancer."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 1,
        "label": "web-lb",
        "hostname": "nb-1.linode.com",
        "ipv4": "192.0.2.1",
        "ipv6": "2001:db8::1",
        "region": "us-east",
        "client_conn_throttle": 0,
        "transfer": {"in": 1000.0, "out": 2000.0, "total": 3000.0},
        "tags": [],
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-15T12:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        nb = await client.get_nodebalancer(1)

        assert nb.id == 1
        assert nb.label == "web-lb"

    await client.close()


async def test_list_nodebalancer_configs() -> None:
    """Test listing NodeBalancer configs."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"id": 6, "port": 80, "protocol": "http"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_nodebalancer_configs(8)

        assert result["data"][0]["id"] == 6
        mock_request.assert_called_once_with("GET", "/nodebalancers/8/configs")

    await client.close()


async def test_list_nodebalancer_configs_with_pagination() -> None:
    """Test listing NodeBalancer configs with pagination params."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [], "page": 2, "pages": 3}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_nodebalancer_configs(8, page=2, page_size=50)

        assert result == {"data": [], "page": 2, "pages": 3}
        mock_request.assert_called_once_with(
            "GET", "/nodebalancers/8/configs?page=2&page_size=50"
        )

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "encoded"),
    [
        ("1/2", "1%2F2"),
        ("1?x", "1%3Fx"),
        ("..", "%2E%2E"),
    ],
)
async def test_list_nodebalancer_configs_encodes_path_params(
    nodebalancer_id: str, encoded: str
) -> None:
    """NodeBalancer config list path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_nodebalancer_configs(cast("Any", nodebalancer_id))

        mock_request.assert_called_once_with("GET", f"/nodebalancers/{encoded}/configs")

    await client.close()


async def test_list_nodebalancer_configs_wraps_http_errors() -> None:
    """Test listing NodeBalancer configs wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_nodebalancer_configs(8)

    assert "ListNodeBalancerConfigs" in str(excinfo.value)
    await client.close()


async def test_retryable_list_nodebalancer_configs_delegates_to_client() -> None:
    """RetryableClient delegates config list with retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_nodebalancer_configs", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_nodebalancer_configs(8, page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(8, page=1, page_size=100)
    await retryable.close()


async def test_list_nodebalancer_config_nodes() -> None:
    """Test listing NodeBalancer config nodes."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {"id": 1, "label": "node-1", "address": "192.0.2.4:80", "weight": 100},
            {"id": 2, "label": "node-2", "address": "192.0.2.5:80", "weight": 200},
        ],
        "page": 1,
        "pages": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_nodebalancer_config_nodes(8, 6)

        assert result == {
            "data": [
                {"id": 1, "label": "node-1", "address": "192.0.2.4:80", "weight": 100},
                {"id": 2, "label": "node-2", "address": "192.0.2.5:80", "weight": 200},
            ],
            "page": 1,
            "pages": 1,
        }
        mock_request.assert_called_once_with(
            "GET",
            "/nodebalancers/8/configs/6/nodes",
        )

    await client.close()


async def test_list_nodebalancer_config_nodes_with_pagination() -> None:
    """Test listing NodeBalancer config nodes with pagination params."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [], "page": 2, "pages": 3}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_nodebalancer_config_nodes(8, 6, page=2, page_size=50)

        assert result == {"data": [], "page": 2, "pages": 3}
        mock_request.assert_called_once_with(
            "GET",
            "/nodebalancers/8/configs/6/nodes?page=2&page_size=50",
        )

    await client.close()


@pytest.mark.parametrize(
    (
        "nodebalancer_id",
        "config_id",
        "encoded_nodebalancer_id",
        "encoded_config_id",
    ),
    [
        ("1/2", "4", "1%2F2", "4"),
        ("8", "3?x", "8", "3%3Fx"),
        ("..", "../6", "%2E%2E", "..%2F6"),
    ],
)
async def test_list_nodebalancer_config_nodes_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
) -> None:
    """NodeBalancer config node list path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_nodebalancer_config_nodes(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
        )

        mock_request.assert_called_once_with(
            "GET",
            (
                f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
                f"{encoded_config_id}/nodes"
            ),
        )

    await client.close()


async def test_list_nodebalancer_config_nodes_wraps_http_errors() -> None:
    """Test listing NodeBalancer config nodes wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_nodebalancer_config_nodes(8, 6)

    assert "ListNodeBalancerConfigNodes" in str(excinfo.value)
    await client.close()


async def test_retryable_list_nodebalancer_config_nodes() -> None:
    """RetryableClient delegates config node list with retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [{"id": 1}]}

    with patch.object(
        retryable.client, "make_request", new_callable=AsyncMock
    ) as mock_request:
        mock_request.return_value = mock_response

        result = await retryable.list_nodebalancer_config_nodes(8, 6)

        assert result == {"data": [{"id": 1}]}

    await retryable.close()


async def test_get_nodebalancer_config_node() -> None:
    """Test getting a NodeBalancer config node."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {
            "id": 4,
            "label": "node-1",
            "address": "192.168.1.10:80",
            "weight": 100,
            "mode": "accept",
        }
        mock_request.return_value = mock_response

        result = await client.get_nodebalancer_config_node(8, 6, 4)

        mock_request.assert_called_once_with(
            "GET", "/nodebalancers/8/configs/6/nodes/4"
        )
        assert result["id"] == 4
        assert result["label"] == "node-1"

    await client.close()


@pytest.mark.parametrize(
    (
        "nodebalancer_id",
        "config_id",
        "node_id",
        "encoded_nodebalancer_id",
        "encoded_config_id",
        "encoded_node_id",
    ),
    [
        ("1/2", "4", "7", "1%2F2", "4", "7"),
        ("8", "3?x", "7", "8", "3%3Fx", "7"),
        ("8", "6", "../5", "8", "6", "..%2F5"),
    ],
)
async def test_get_nodebalancer_config_node_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    node_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
    encoded_node_id: str,
) -> None:
    """NodeBalancer config node get path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {"id": 4}
        mock_request.return_value = mock_response

        await client.get_nodebalancer_config_node(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
            cast("Any", node_id),
        )

        mock_request.assert_called_once_with(
            "GET",
            (
                f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
                f"{encoded_config_id}/nodes/{encoded_node_id}"
            ),
        )

    await client.close()


async def test_get_nodebalancer_config_node_wraps_http_errors() -> None:
    """Test getting a NodeBalancer config node wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_nodebalancer_config_node(8, 6, 4)

    assert "GetNodeBalancerConfigNode" in str(excinfo.value)
    await client.close()


async def test_list_nodebalancer_firewalls() -> None:
    """Test listing firewalls assigned to a NodeBalancer."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"id": 123, "label": "web-fw"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_nodebalancer_firewalls(8, page=1, page_size=25)

        assert result["data"][0]["id"] == 123
        assert result["results"] == 1
        mock_request.assert_called_once_with(
            "GET", "/nodebalancers/8/firewalls?page=1&page_size=25"
        )

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "encoded"),
    [
        ("1/2", "1%2F2"),
        ("1?x", "1%3Fx"),
        ("..", "%2E%2E"),
    ],
)
async def test_list_nodebalancer_firewalls_encodes_path_params(
    nodebalancer_id: str, encoded: str
) -> None:
    """NodeBalancer firewall list path parameter is URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_nodebalancer_firewalls(nodebalancer_id)  # type: ignore[arg-type]

        mock_request.assert_called_once_with(
            "GET", f"/nodebalancers/{encoded}/firewalls"
        )

    await client.close()


async def test_list_nodebalancer_firewalls_wraps_http_errors() -> None:
    """Test listing NodeBalancer firewalls wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_nodebalancer_firewalls(8)

    assert "ListNodeBalancerFirewalls" in str(excinfo.value)
    await client.close()


async def test_retryable_list_nodebalancer_firewalls_delegates_with_retry() -> None:
    """RetryableClient delegates firewall listing through retry wrapper."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_nodebalancer_firewalls", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [{"id": 1}], "results": 1}
        result = await retryable.list_nodebalancer_firewalls(8, page=1, page_size=100)

        assert result["results"] == 1
        mock_list.assert_awaited_once_with(8, page=1, page_size=100)

    await retryable.close()


async def test_get_stackscript_url_encodes_stackscript_id() -> None:
    """Getting a StackScript URL-encodes the path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.json.return_value = {
        "id": 1,
        "username": "testuser",
        "user_gravatar_id": "abc123",
        "label": "encoded",
        "description": "Encoded",
        "images": [],
        "deployments_total": 0,
        "deployments_active": 0,
        "is_public": False,
        "mine": True,
        "created": "2024-01-01T00:00:00",
        "updated": "2024-01-01T00:00:00",
        "script": "#!/bin/bash",
        "user_defined_fields": [],
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_stackscript("1/2?x=..")

    assert result.id == 1
    mock_request.assert_called_once_with("GET", "/linode/stackscripts/1%2F2%3Fx%3D..")
    await client.close()


async def test_get_image_sharegroup_sends_get_to_encoded_path() -> None:
    """Image share group get should issue GET to the encoded share group path."""
    response_data = {"id": "11111111-1111-4111-8111-111111111111"}
    response = MagicMock()
    response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_image_sharegroup(
            "11111111-1111-4111-8111-111111111111"
        )

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "GET", "/images/sharegroups/11111111-1111-4111-8111-111111111111"
    )
    await client.close()


async def test_get_image_sharegroup_encodes_path_segment() -> None:
    """Image share group get should encode separator characters."""
    response = MagicMock()
    response.json.return_value = {}
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.get_image_sharegroup("12/../?x=1")

    mock_request.assert_awaited_once_with(
        "GET", "/images/sharegroups/12%2F..%2F%3Fx%3D1"
    )
    await client.close()


async def test_get_image_sharegroup_wraps_http_errors() -> None:
    """Image share group get should map HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_image_sharegroup("11111111-1111-4111-8111-111111111111")

    assert "GetImageSharegroup" in str(exc_info.value)
    await client.close()


async def test_retryable_get_image_sharegroup_uses_retry_wrapper() -> None:
    """Read-only image share group get delegates through retry."""
    response_data = {"id": "11111111-1111-4111-8111-111111111111"}
    retry_client = RetryableClient.__new__(RetryableClient)
    retry_client.client = MagicMock()
    retry_client.client.get_image_sharegroup = AsyncMock(return_value=response_data)

    async def _execute(call: Any) -> Any:
        return await call()

    with patch.object(
        retry_client, "_execute_with_retry", AsyncMock(side_effect=_execute)
    ) as execute_with_retry:
        result = await retry_client.get_image_sharegroup(
            "11111111-1111-4111-8111-111111111111"
        )

    assert result == response_data
    execute_with_retry.assert_awaited_once()
    retry_client.client.get_image_sharegroup.assert_awaited_once_with(
        "11111111-1111-4111-8111-111111111111"
    )


async def test_get_image_sends_get_to_encoded_image_route() -> None:
    """Getting an image sends GET to the encoded image route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": "linode/ubuntu24.04",
        "label": "Ubuntu 24.04 LTS",
        "description": "Ubuntu image",
        "type": "manual",
        "is_public": True,
        "deprecated": False,
        "size": 2500,
        "vendor": "Ubuntu",
        "status": "available",
        "created": "2024-04-25T00:00:00",
        "created_by": "linode",
        "capabilities": ["cloud-init"],
        "tags": [],
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_image("linode/ubuntu24.04")

        mock_request.assert_awaited_once_with("GET", "/images/linode%2Fubuntu24.04")
        assert result.id == "linode/ubuntu24.04"
        assert result.label == "Ubuntu 24.04 LTS"

    await client.close()


async def test_get_image_encodes_separator_characters() -> None:
    """Getting an image encodes untrusted separators at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.get_image("private/../?x=1")

        mock_request.assert_awaited_once_with("GET", "/images/private%2F..%2F%3Fx%3D1")

    await client.close()


async def test_get_image_wraps_http_errors() -> None:
    """Getting an image should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_image("linode/ubuntu24.04")

    assert "GetImage" in str(exc_info.value)
    await client.close()


async def test_retryable_get_image_delegates_to_client() -> None:
    """Retryable client should delegate image get."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_image = Image(
        id="linode/ubuntu24.04",
        label="Ubuntu 24.04 LTS",
        description="",
        type="manual",
        is_public=True,
        deprecated=False,
        size=0,
        vendor="Ubuntu",
        status="available",
        created="2024-04-25T00:00:00",
        created_by="linode",
        expiry=None,
        eol=None,
        capabilities=[],
        tags=[],
    )

    with patch.object(
        client.client,
        "get_image",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = response_image

        result = await client.get_image("linode/ubuntu24.04")

        assert result.id == "linode/ubuntu24.04"
        mock_get.assert_awaited_once_with("linode/ubuntu24.04")

    await client.close()


class TestValidateSSHKey:
    """Tests for SSH key validation."""

    def test_valid_rsa_key(self) -> None:
        """Test valid RSA SSH key."""
        key = "ssh-rsa " + "A" * 100  # Minimum length
        validate_ssh_key(key)  # Should not raise

    def test_valid_ed25519_key(self) -> None:
        """Test valid ed25519 SSH key."""
        key = "ssh-ed25519 " + "A" * 100
        validate_ssh_key(key)

    def test_empty_key_raises(self) -> None:
        """Test empty key raises error."""
        with pytest.raises(ValueError, match="ssh_key is required"):
            validate_ssh_key("")

    def test_invalid_prefix_raises(self) -> None:
        """Test invalid prefix raises error."""
        with pytest.raises(ValueError, match="invalid SSH key format"):
            validate_ssh_key("invalid-prefix " + "A" * 100)

    def test_key_too_short_raises(self) -> None:
        """Test short key raises error."""
        with pytest.raises(ValueError, match="invalid SSH key length"):
            validate_ssh_key("ssh-rsa " + "A" * 10)


class TestValidateRootPassword:
    """Tests for root password validation."""

    def test_valid_password(self) -> None:
        """Test valid password."""
        validate_root_password("ValidPass123!")  # Should not raise

    def test_empty_password_allowed(self) -> None:
        """Test empty password is allowed (optional)."""
        validate_root_password(None)  # Should not raise
        validate_root_password("")  # Should not raise

    def test_password_too_short(self) -> None:
        """Test short password raises error."""
        with pytest.raises(ValueError, match="at least 12 characters"):
            validate_root_password("Short1!")

    def test_password_too_long(self) -> None:
        """Test long password raises error."""
        with pytest.raises(ValueError, match="not exceed 128"):
            validate_root_password("A" * 129 + "a1")

    def test_password_missing_uppercase(self) -> None:
        """Test password without uppercase raises error."""
        with pytest.raises(ValueError, match="uppercase, lowercase, and digits"):
            validate_root_password("lowercase123456")

    def test_password_missing_lowercase(self) -> None:
        """Test password without lowercase raises error."""
        with pytest.raises(ValueError, match="uppercase, lowercase, and digits"):
            validate_root_password("UPPERCASE123456")

    def test_password_missing_digit(self) -> None:
        """Test password without digit raises error."""
        with pytest.raises(ValueError, match="uppercase, lowercase, and digits"):
            validate_root_password("NoDigitsHere!")


class TestValidateDNSRecordName:
    """Tests for DNS record name validation."""

    def test_valid_name(self) -> None:
        """Test valid DNS name."""
        validate_dns_record_name("www")  # Should not raise
        validate_dns_record_name("sub.domain")

    def test_empty_name_allowed(self) -> None:
        """Test empty name is allowed."""
        validate_dns_record_name("")  # Should not raise

    def test_at_sign_allowed(self) -> None:
        """Test @ symbol is allowed."""
        validate_dns_record_name("@")  # Should not raise

    def test_name_too_long(self) -> None:
        """Test long name raises error."""
        with pytest.raises(ValueError, match="maximum length"):
            validate_dns_record_name("a" * 254)


class TestValidateDNSRecordTarget:
    """Tests for DNS record target validation."""

    def test_valid_public_ipv4(self) -> None:
        """Test valid public IPv4 addresses pass."""
        validate_dns_record_target("A", "8.8.8.8")
        validate_dns_record_target("A", "1.1.1.1")
        validate_dns_record_target("A", "104.237.137.1")

    def test_172_outside_private_range_allowed(self) -> None:
        """Test 172.x IPs outside 172.16-31.x.x pass."""
        validate_dns_record_target("A", "172.15.0.1")
        validate_dns_record_target("A", "172.32.0.1")

    def test_private_10_range_rejected(self) -> None:
        """Test 10.x.x.x private range is rejected."""
        with pytest.raises(ValueError, match="private IP"):
            validate_dns_record_target("A", "10.0.0.1")

    def test_private_192_168_range_rejected(self) -> None:
        """Test 192.168.x.x private range is rejected."""
        with pytest.raises(ValueError, match="private IP"):
            validate_dns_record_target("A", "192.168.1.1")

    def test_private_172_16_range_rejected(self) -> None:
        """Test 172.16-31.x.x private range is rejected."""
        with pytest.raises(ValueError, match="private IP"):
            validate_dns_record_target("A", "172.16.0.1")
        with pytest.raises(ValueError, match="private IP"):
            validate_dns_record_target("A", "172.31.255.255")
        with pytest.raises(ValueError, match="private IP"):
            validate_dns_record_target("A", "172.20.10.5")

    def test_loopback_rejected(self) -> None:
        """Test 127.x.x.x loopback is rejected."""
        with pytest.raises(ValueError, match="private IP"):
            validate_dns_record_target("A", "127.0.0.1")

    def test_invalid_ipv4_rejected(self) -> None:
        """Test invalid IPv4 like 999.999.999.999 is rejected."""
        with pytest.raises(ValueError, match="valid IPv4"):
            validate_dns_record_target("A", "999.999.999.999")
        with pytest.raises(ValueError, match="valid IPv4"):
            validate_dns_record_target("A", "not-an-ip")

    def test_empty_target_rejected(self) -> None:
        """Test empty target is rejected."""
        with pytest.raises(ValueError, match="required"):
            validate_dns_record_target("A", "")


class TestValidateFirewallPolicy:
    """Tests for firewall policy validation."""

    def test_accept_policy(self) -> None:
        """Test ACCEPT policy is valid."""
        validate_firewall_policy("ACCEPT")  # Should not raise
        validate_firewall_policy("accept")  # Case insensitive

    def test_drop_policy(self) -> None:
        """Test DROP policy is valid."""
        validate_firewall_policy("DROP")  # Should not raise
        validate_firewall_policy("drop")  # Case insensitive

    def test_invalid_policy(self) -> None:
        """Test invalid policy raises error."""
        with pytest.raises(ValueError, match=r"ACCEPT.*DROP"):
            validate_firewall_policy("INVALID")


class TestValidateVolumeSize:
    """Tests for volume size validation."""

    def test_valid_size(self) -> None:
        """Test valid volume size."""
        validate_volume_size(10)  # Minimum
        validate_volume_size(100)
        validate_volume_size(10240)  # Maximum

    def test_size_too_small(self) -> None:
        """Test small size raises error."""
        with pytest.raises(ValueError, match="at least 10"):
            validate_volume_size(5)

    def test_size_too_large(self) -> None:
        """Test large size raises error."""
        with pytest.raises(ValueError, match="cannot exceed"):
            validate_volume_size(10241)


class TestValidateLabel:
    """Tests for label validation."""

    def test_valid_label(self) -> None:
        """Test valid labels."""
        validate_label("my-label")  # Should not raise
        validate_label("my_label")
        validate_label("my.label")
        validate_label("label123")

    def test_empty_label_allowed(self) -> None:
        """Test empty label is allowed."""
        validate_label(None)  # Should not raise
        validate_label("")

    def test_label_too_long(self) -> None:
        """Test long label raises error."""
        with pytest.raises(ValueError, match="not exceed 64"):
            validate_label("a" * 65)

    def test_invalid_characters(self) -> None:
        """Test invalid characters raise error."""
        with pytest.raises(ValueError, match="invalid character"):
            validate_label("label with spaces")


class TestMakeRequestURLConstruction:
    """Verify that make_request builds the full URL from base_url + endpoint."""

    async def test_url_is_base_plus_endpoint(self) -> None:
        """The request URL should be base_url concatenated with the endpoint."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.make_request("GET", "/linode/instances")

            call_args = mock_req.call_args
            assert call_args[0][0] == "GET"
            assert call_args[0][1] == "https://api.linode.com/v4/linode/instances"

        await client.close()


class TestMakeRequestHeaders:
    """Verify that make_request sets the correct headers."""

    async def test_authorization_header(self) -> None:
        """Authorization header should be Bearer + the token."""
        client = Client("https://api.linode.com/v4", "my-secret-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.make_request("GET", "/profile")

            headers = mock_req.call_args[1]["headers"]
            assert headers["Authorization"] == "Bearer my-secret-token"

        await client.close()

    async def test_content_type_header(self) -> None:
        """Content-Type header should be application/json."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.make_request("GET", "/profile")

            headers = mock_req.call_args[1]["headers"]
            assert headers["Content-Type"] == "application/json"

        await client.close()

    async def test_user_agent_header(self) -> None:
        """User-Agent header should identify the LinodeMCP client."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.make_request("GET", "/profile")

            headers = mock_req.call_args[1]["headers"]
            assert "LinodeMCP" in headers["User-Agent"]

        await client.close()


class TestMakeRequestBody:
    """Verify body handling for different HTTP methods."""

    async def test_post_sends_json_body(self) -> None:
        """POST with body should pass json= to the underlying client."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.make_request(
                "POST", "/linode/instances", body={"label": "test"}
            )

            assert mock_req.call_args[1]["json"] == {"label": "test"}

        await client.close()

    async def test_get_monitor_service_alert_definition_get_shape(self) -> None:
        """GET alert definition endpoint URL-encodes path params."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"id": 12345, "label": "CPU high"}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.get_monitor_service_alert_definition(
                "weird/type with space?and=query", 12345
            )

            url_arg = mock_req.call_args[0][1]
            assert result == {"id": 12345, "label": "CPU high"}
            assert mock_req.call_args[0][0] == "GET"
            assert url_arg.endswith(
                "/monitor/services/"
                "weird%2Ftype%20with%20space%3Fand%3Dquery"
                "/alert-definitions/12345"
            )
            assert "json" not in mock_req.call_args[1]

        await client.close()

    async def test_get_monitor_service_alert_definition_rejects_invalid_inputs(
        self,
    ) -> None:
        """Client rejects invalid get inputs before issuing a request."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.get_monitor_service_alert_definition("", 12345)
            with pytest.raises(TypeError, match="alert_id"):
                await client.get_monitor_service_alert_definition("dbaas", True)
            with pytest.raises(TypeError, match="alert_id"):
                await client.get_monitor_service_alert_definition(
                    "dbaas", cast("Any", 12.9)
                )
            with pytest.raises(ValueError, match="positive"):
                await client.get_monitor_service_alert_definition("dbaas", 0)
            with pytest.raises(ValueError, match="positive"):
                await client.get_monitor_service_alert_definition("dbaas", -1)
            mock_req.assert_not_called()

        await client.close()

    async def test_get_monitor_service_alert_definition_wraps_http_errors(
        self,
    ) -> None:
        """Client wraps HTTP errors with the get alert definition operation."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("timeout")
            with pytest.raises(NetworkError) as exc_info:
                await client.get_monitor_service_alert_definition("dbaas", 12345)

        assert exc_info.value.operation == "GetMonitorServiceAlertDefinition"
        await client.close()

    async def test_get_has_no_json_body(self) -> None:
        """GET without body should not pass json= to the underlying client."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.make_request("GET", "/linode/instances")

            assert "json" not in mock_req.call_args[1]

        await client.close()


class TestMakeRequestErrorCodes:
    """Verify that error status codes raise APIError."""

    async def test_400_raises_api_error(self) -> None:
        """400 Bad Request should raise APIError."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 400
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.make_request("GET", "/bad")

            assert exc_info.value.status_code == 400

        await client.close()

    async def test_401_raises_authentication_error(self) -> None:
        """401 should raise APIError flagged as authentication error."""
        client = Client("https://api.linode.com/v4", "bad-token")

        mock_response = MagicMock()
        mock_response.status_code = 401
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.make_request("GET", "/profile")

            assert exc_info.value.is_authentication_error()

        await client.close()

    async def test_429_raises_rate_limit_error(self) -> None:
        """429 should raise APIError flagged as rate limit error."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 429
        mock_response.json.return_value = {}
        mock_response.headers = {"Retry-After": "30"}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.make_request("GET", "/profile")

            assert exc_info.value.is_rate_limit_error()

        await client.close()

    async def test_500_raises_server_error(self) -> None:
        """500 should raise APIError flagged as server error."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 500
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.make_request("GET", "/profile")

            assert exc_info.value.is_server_error()

        await client.close()


class TestMakeRequestErrorResponseParsing:
    """Verify that structured error responses are parsed into APIError fields."""

    async def test_structured_error_extracts_reason(self) -> None:
        """When the API returns {errors: [{reason, field}]}, those get extracted."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 400
        mock_response.json.return_value = {
            "errors": [{"reason": "label is required", "field": "label"}]
        }
        mock_response.headers = {}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.make_request("POST", "/linode/instances")

            assert "label is required" in str(exc_info.value)
            assert exc_info.value.field == "label"

        await client.close()


class TestValidateDiskSize:
    """Tests for disk size validation."""

    def test_valid_size(self) -> None:
        """Test a typical valid disk size."""
        validate_disk_size(100)

    def test_minimum_boundary(self) -> None:
        """Test the minimum allowed size (1 MB)."""
        validate_disk_size(1)

    def test_maximum_boundary(self) -> None:
        """Test the maximum allowed size (524288 MB)."""
        validate_disk_size(524288)

    def test_too_small(self) -> None:
        """Test that 0 MB is rejected."""
        with pytest.raises(ValueError, match="disk size"):
            validate_disk_size(0)

    def test_too_large(self) -> None:
        """Test that exceeding 524288 MB is rejected."""
        with pytest.raises(ValueError, match="disk size"):
            validate_disk_size(524289)

    def test_negative(self) -> None:
        """Test that negative values are rejected."""
        with pytest.raises(ValueError, match="disk size"):
            validate_disk_size(-1)


class TestRetryableClientRetryScenarios:
    """Tests for retry behavior across different HTTP error codes."""

    async def test_retry_on_server_error(self) -> None:
        """500 then 200 should retry once and succeed."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=2, base_delay=0.01),
        )

        mock_error_response = MagicMock()
        mock_error_response.status_code = 500
        mock_error_response.json.return_value = {}
        mock_error_response.headers = {}

        mock_success_response = MagicMock()
        mock_success_response.status_code = 200
        mock_success_response.json.return_value = {
            "username": "retryuser",
            "email": "retry@test.com",
            "timezone": "UTC",
            "email_notifications": False,
            "restricted": False,
            "two_factor_auth": False,
            "uid": 1,
        }

        call_count = 0

        async def mock_request(*args: Any, **kwargs: Any) -> MagicMock:
            nonlocal call_count
            call_count += 1
            _ = args, kwargs
            if call_count == 1:
                return mock_error_response
            return mock_success_response

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.side_effect = mock_request

            profile = await client.get_profile()

            assert profile.username == "retryuser"
            assert call_count == 2

        await client.close()

    async def test_no_retry_on_auth_error(self) -> None:
        """401 should not be retried."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "bad-token",
            RetryConfig(max_retries=3, base_delay=0.01),
        )

        mock_response = MagicMock()
        mock_response.status_code = 401
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.get_profile()

            assert exc_info.value.status_code == 401
            assert mock_req.call_count == 1

        await client.close()

    async def test_no_retry_on_forbidden(self) -> None:
        """403 should not be retried."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=3, base_delay=0.01),
        )

        mock_response = MagicMock()
        mock_response.status_code = 403
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.get_profile()

            assert exc_info.value.status_code == 403
            assert mock_req.call_count == 1

        await client.close()

    async def test_retry_on_network_error(self) -> None:
        """NetworkError then success should retry once and succeed."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=2, base_delay=0.01),
        )

        mock_success_response = MagicMock()
        mock_success_response.status_code = 200
        mock_success_response.json.return_value = {
            "username": "retryuser",
            "email": "retry@test.com",
            "timezone": "UTC",
            "email_notifications": False,
            "restricted": False,
            "two_factor_auth": False,
            "uid": 1,
        }

        call_count = 0

        async def mock_request(*args: Any, **kwargs: Any) -> MagicMock:
            nonlocal call_count
            call_count += 1
            _ = args, kwargs
            if call_count == 1:
                raise httpx.ConnectError("Connection failed")
            return mock_success_response

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.side_effect = mock_request

            profile = await client.get_profile()

            assert profile.username == "retryuser"
            assert call_count == 2

        await client.close()

    async def test_backoff_timing(self) -> None:
        """Backoff delays should increase exponentially."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=3, base_delay=1.0, backoff_factor=2.0),
        )

        mock_response = MagicMock()
        mock_response.status_code = 429
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = mock_response

            with patch(
                "linodemcp.linode.asyncio.sleep",
                new_callable=AsyncMock,
            ) as mock_sleep:
                with pytest.raises(APIError) as exc_info:
                    await client.get_profile()

                assert exc_info.value.status_code == 429

                assert mock_sleep.call_count == 3
                delays = [call.args[0] for call in mock_sleep.call_args_list]
                # base_delay * backoff_factor^(attempt-1) plus up to 10% jitter
                assert delays[0] >= 1.0, f"first delay {delays[0]} should be >= 1.0"
                assert delays[0] <= 1.2, f"first delay {delays[0]} should be <= 1.2"
                assert delays[1] > delays[0], "second delay should be larger than first"
                assert delays[2] > delays[1], "third delay should be larger than second"

        await client.close()

    async def test_retry_exhaustion_with_rate_limit(self) -> None:
        """429 three times should exhaust retries and raise."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=2, base_delay=0.01),
        )

        mock_response = MagicMock()
        mock_response.status_code = 429
        mock_response.json.return_value = {}
        mock_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = mock_response

            with pytest.raises(APIError) as exc_info:
                await client.get_profile()

            assert exc_info.value.status_code == 429
            # max_retries=2 means 1 initial + 2 retries = 3 total calls
            assert mock_req.call_count == 3

        await client.close()


class TestCircuitBreaker:
    """Tests for the CircuitBreaker state machine.

    These cover the contract: trip after threshold consecutive failures,
    reject while open until cooldown elapses, admit one probe in half-open,
    close on probe success, re-open on probe failure, reset on success.
    """

    def test_disabled_when_threshold_zero(self) -> None:
        """A non-positive threshold disables the breaker entirely."""
        breaker = CircuitBreaker(0, 1.0)
        for _ in range(100):
            breaker.record_failure()
        # Must not raise: threshold 0 means allow always returns.
        breaker.allow()

    def test_trips_at_threshold(self) -> None:
        """Breaker opens exactly when consecutive failures reach threshold."""
        breaker = CircuitBreaker(3, 60.0)

        breaker.record_failure()
        breaker.record_failure()
        # Two failures (below threshold) must not trip.
        breaker.allow()

        breaker.record_failure()
        with pytest.raises(CircuitOpenError):
            breaker.allow()

    def test_half_open_after_timeout(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """After cooldown elapses, exactly one probe is admitted."""
        clock = [0.0]
        monkeypatch.setattr("linodemcp.linode.time.monotonic", lambda: clock[0])

        breaker = CircuitBreaker(2, timeout=10.0)

        breaker.record_failure()
        breaker.record_failure()
        with pytest.raises(CircuitOpenError):
            breaker.allow()

        # Advance synthetic time past the cooldown.
        clock[0] = 11.0

        # First call after cooldown: probe admitted (half-open).
        breaker.allow()

        # Subsequent concurrent calls during in-flight probe: rejected.
        with pytest.raises(CircuitOpenError):
            breaker.allow()

    def test_closes_on_successful_probe(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """A successful probe in half-open closes the breaker fully."""
        clock = [0.0]
        monkeypatch.setattr("linodemcp.linode.time.monotonic", lambda: clock[0])

        breaker = CircuitBreaker(2, timeout=5.0)

        breaker.record_failure()
        breaker.record_failure()
        clock[0] = 6.0
        breaker.allow()  # half-open probe admitted

        breaker.record_success()

        # Closed: subsequent calls all pass.
        breaker.allow()
        breaker.allow()

    def test_reopens_on_failed_probe(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """A failed probe in half-open re-opens the breaker."""
        clock = [0.0]
        monkeypatch.setattr("linodemcp.linode.time.monotonic", lambda: clock[0])

        breaker = CircuitBreaker(2, timeout=5.0)

        breaker.record_failure()
        breaker.record_failure()
        clock[0] = 6.0
        breaker.allow()  # probe admitted

        breaker.record_failure()  # probe failed

        with pytest.raises(CircuitOpenError):
            breaker.allow()

    def test_success_resets_failure_count(self) -> None:
        """A success between failures restarts the failure counter."""
        breaker = CircuitBreaker(3, 60.0)

        breaker.record_failure()
        breaker.record_failure()
        breaker.record_success()

        # Two more failures alone (below threshold from zero) must not trip.
        breaker.record_failure()
        breaker.record_failure()
        breaker.allow()


class TestRetryableClientCircuitBreaker:
    """Tests for the breaker's integration with RetryableClient."""

    async def test_breaker_trips_after_repeated_exhaustion(self) -> None:
        """After threshold retry exhaustions, calls fail fast with CircuitOpenError."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(
                max_retries=1,
                base_delay=0.001,
                max_delay=0.001,
                circuit_breaker_threshold=2,
                circuit_breaker_timeout=60.0,
            ),
        )

        mock_error_response = MagicMock()
        mock_error_response.status_code = 500
        mock_error_response.json.return_value = {}
        mock_error_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = mock_error_response

            # First exhaustion: 1 initial + 1 retry = 2 upstream calls.
            with pytest.raises(APIError):
                await client.get_profile()
            assert mock_req.call_count == 2

            # Second exhaustion: another 2 calls. Breaker trips after.
            with pytest.raises(APIError):
                await client.get_profile()
            assert mock_req.call_count == 4

            # Third call: breaker open. Upstream must NOT be touched.
            with pytest.raises(CircuitOpenError):
                await client.get_profile()
            assert mock_req.call_count == 4

        await client.close()

    async def test_breaker_disabled_with_zero_threshold(self) -> None:
        """Threshold 0 keeps the breaker dormant: failures don't trip."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(
                max_retries=1,
                base_delay=0.001,
                max_delay=0.001,
                circuit_breaker_threshold=0,
                circuit_breaker_timeout=60.0,
            ),
        )

        mock_error_response = MagicMock()
        mock_error_response.status_code = 500
        mock_error_response.json.return_value = {}
        mock_error_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = mock_error_response

            # Three exhaustions, breaker disabled, every call hits upstream.
            for _ in range(3):
                with pytest.raises(APIError):
                    await client.get_profile()

            # 3 * (1 initial + 1 retry) = 6 total calls.
            assert mock_req.call_count == 6

        await client.close()

    async def test_breaker_resets_on_success(self) -> None:
        """A success between failures clears the consecutive-failure counter."""
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(
                max_retries=0,
                base_delay=0.001,
                circuit_breaker_threshold=3,
                circuit_breaker_timeout=60.0,
            ),
        )

        mock_error_response = MagicMock()
        mock_error_response.status_code = 500
        mock_error_response.json.return_value = {}
        mock_error_response.headers = {}

        mock_success_response = MagicMock()
        mock_success_response.status_code = 200
        mock_success_response.json.return_value = {
            "username": "ok",
            "email": "ok@test.com",
            "timezone": "UTC",
            "email_notifications": False,
            "restricted": False,
            "two_factor_auth": False,
            "uid": 1,
        }

        responses = [
            mock_error_response,
            mock_error_response,
            mock_success_response,
            mock_error_response,
            mock_error_response,
        ]

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.side_effect = responses

            # 2 failures
            for _ in range(2):
                with pytest.raises(APIError):
                    await client.get_profile()

            # Success in between resets counter.
            profile = await client.get_profile()
            assert profile.username == "ok"

            # 2 more failures: still below threshold (3) thanks to reset.
            for _ in range(2):
                with pytest.raises(APIError):
                    await client.get_profile()

            # Breaker should still be closed.
            assert mock_req.call_count == 5

        await client.close()


class TestRateLimiter:
    """Tests for the asyncio token-bucket rate limiter.

    These cover the contract: capacity equals the per-minute rate, refill is
    rate/60 tokens per second, wait blocks until a token is available, and a
    non-positive rate disables the limiter entirely.
    """

    async def test_disabled_when_rate_zero(self) -> None:
        """A non-positive rate yields a no-op limiter."""
        limiter = RateLimiter(0)
        # 100 calls in tight succession must not block or raise.
        for _ in range(100):
            await limiter.wait()

    async def test_allows_burst_up_to_capacity(self) -> None:
        """A fresh bucket should grant `capacity` tokens before blocking."""
        burst = 60
        limiter = RateLimiter(burst)
        for _ in range(burst):
            await limiter.wait()

    async def test_blocks_beyond_burst(self, monkeypatch: pytest.MonkeyPatch) -> None:
        """Once drained, the limiter waits for the next refill cycle."""
        clock = [0.0]
        sleeps: list[float] = []

        async def fake_sleep(delay: float) -> None:
            sleeps.append(delay)
            clock[0] += delay

        monkeypatch.setattr("linodemcp.linode.time.monotonic", lambda: clock[0])
        monkeypatch.setattr("linodemcp.linode.asyncio.sleep", fake_sleep)

        # 60/min => 1 token/sec refill, 60 capacity. Burn the burst, then
        # the next wait should park for ~1s of synthetic time.
        limiter = RateLimiter(60)
        for _ in range(60):
            await limiter.wait()

        await limiter.wait()
        assert sleeps, "limiter should have called asyncio.sleep at least once"
        total: float = sum(sleeps)
        assert 0.9 <= total <= 1.1, f"expected ~1.0s total sleep, got {total}"

    async def test_refill_caps_at_capacity(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """Long idle periods do not let the bucket overflow past capacity.

        Behavior under test: with a 60/min limiter idle for 5 minutes, only
        `capacity` (60) consecutive calls must succeed without waiting. The
        61st call must trigger an asyncio.sleep. If the bucket overflowed to
        300 (5 minutes * 60/min), this test would fail because all 300+ calls
        would pass without sleeping.
        """
        clock = [0.0]
        sleeps: list[float] = []

        async def fake_sleep(delay: float) -> None:
            sleeps.append(delay)
            clock[0] += delay

        monkeypatch.setattr("linodemcp.linode.time.monotonic", lambda: clock[0])
        monkeypatch.setattr("linodemcp.linode.asyncio.sleep", fake_sleep)

        limiter = RateLimiter(60)

        # Idle 5 minutes. A naive implementation would refill to 300 tokens.
        clock[0] = 300.0

        # First 60 calls should NOT trigger sleep (within capacity).
        for _ in range(60):
            await limiter.wait()
        assert not sleeps, f"first {60} calls should not block, but sleeps={sleeps}"

        # The 61st call must require waiting for a refill, proving the
        # bucket capped at capacity rather than overflowing.
        await limiter.wait()
        assert sleeps, (
            "61st call must trigger asyncio.sleep when bucket caps at capacity"
        )

    async def test_cancellation_propagates(self) -> None:
        """A canceled wait raises CancelledError instead of swallowing it."""
        limiter = RateLimiter(1)
        await limiter.wait()  # drain the single token

        async def waiter() -> None:
            await limiter.wait()

        task = asyncio.create_task(waiter())
        # Yield once so the task starts its asyncio.sleep.
        await asyncio.sleep(0)
        task.cancel()

        with pytest.raises(asyncio.CancelledError):
            await task


class TestClientConnectionPool:
    """Tests for httpx.Limits configuration on the underlying Client."""

    async def test_default_pool_limits(self) -> None:
        """Default Client construction uses the documented pool defaults."""
        client = Client("https://api.linode.com/v4", "test-token")
        try:
            assert client.limits.max_connections == 10
            assert client.limits.max_keepalive_connections == 10
            assert client.limits.keepalive_expiry == 30.0
        finally:
            await client.close()

    async def test_custom_pool_limits(self) -> None:
        """Pool kwargs flow into the retained httpx.Limits object."""
        client = Client(
            "https://api.linode.com/v4",
            "test-token",
            max_connections=50,
            max_keepalive_connections=25,
            keepalive_expiry=60.0,
        )
        try:
            assert client.limits.max_connections == 50
            assert client.limits.max_keepalive_connections == 25
            assert client.limits.keepalive_expiry == 60.0
        finally:
            await client.close()

    async def test_retryable_client_threads_pool_config(self) -> None:
        """RetryableClient passes pool fields from RetryConfig to Client."""
        cfg = RetryConfig(
            pool_max_connections=42,
            pool_max_keepalive_connections=21,
            pool_keepalive_expiry=15.0,
        )
        client = RetryableClient("https://api.linode.com/v4", "test-token", cfg)
        try:
            assert client.client.limits.max_connections == 42
            assert client.client.limits.max_keepalive_connections == 21
            assert client.client.limits.keepalive_expiry == 15.0
        finally:
            await client.close()


class TestRetryableClientRateLimiter:
    """Tests for the limiter's integration with RetryableClient."""

    async def test_limiter_gates_upstream_calls(
        self, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        """Drained bucket blocks the next upstream call until refill.

        Patches asyncio.sleep so the synthetic delay records without burning
        real time. The check that matters is upstream call count between the
        first and second invocations.
        """
        clock = [0.0]
        sleeps: list[float] = []

        async def fake_sleep(delay: float) -> None:
            sleeps.append(delay)
            clock[0] += delay

        monkeypatch.setattr("linodemcp.linode.time.monotonic", lambda: clock[0])
        monkeypatch.setattr("linodemcp.linode.asyncio.sleep", fake_sleep)

        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(
                max_retries=0,
                base_delay=0.001,
                max_delay=0.001,
                circuit_breaker_threshold=0,
                rate_limit_per_minute=60,
            ),
        )

        ok_response = MagicMock()
        ok_response.status_code = 200
        ok_response.json.return_value = {
            "username": "u",
            "email": "e@example.com",
            "uid": 1,
            "timezone": "UTC",
            "email_notifications": False,
            "ip_whitelist_enabled": False,
            "lish_auth_method": "password_keys",
            "two_factor_auth": False,
            "restricted": False,
        }
        ok_response.headers = {}

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.return_value = ok_response

            # Burn the 60-token burst.
            for _ in range(60):
                await client.get_profile()

            assert mock_req.call_count == 60
            sleeps.clear()  # ignore any sleeps during burst

            # 61st call must wait for the limiter (~1s synthetic).
            await client.get_profile()
            assert mock_req.call_count == 61
            assert sleeps, "limiter must have parked the 61st call on sleep"

        await client.close()


async def test_get_profile_token_sends_get_to_profile_token_route() -> None:
    """Profile token get sends GET /profile/tokens/{tokenId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "label": "api-token"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.get_profile_token(12345)

    assert result == {"id": 12345, "label": "api-token"}
    mock_request.assert_called_once_with("GET", "/profile/tokens/12345")
    await client.close()


async def test_get_profile_token_encodes_path_parameter() -> None:
    """Profile token get path segment is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "label": "api-token"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.get_profile_token("12/../34?x=1")  # type: ignore[arg-type]

    mock_request.assert_called_once_with("GET", "/profile/tokens/12%2F..%2F34%3Fx%3D1")
    await client.close()


async def test_get_profile_token_wraps_http_errors() -> None:
    """Profile token get maps HTTP errors to GetProfileToken."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.get_profile_token(12345)

    assert exc_info.value.operation == "GetProfileToken"
    await client.close()


async def test_get_profile_app_sends_get_to_profile_app_route() -> None:
    """Profile app get sends GET /profile/apps/{appId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "label": "authorized-app"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.get_profile_app(12345)

    assert result == {"id": 12345, "label": "authorized-app"}
    mock_request.assert_awaited_once_with("GET", "/profile/apps/12345")
    await client.close()


async def test_get_profile_app_encodes_path_parameter() -> None:
    """Profile app get path segment is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "12/../34?x=1"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.get_profile_app("12/../34?x=1")  # type: ignore[arg-type]

    mock_request.assert_awaited_once_with("GET", "/profile/apps/12%2F..%2F34%3Fx%3D1")
    await client.close()


async def test_get_profile_app_wraps_http_errors() -> None:
    """Profile app get maps HTTP errors to GetProfileApp."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.get_profile_app(12345)

    assert exc_info.value.operation == "GetProfileApp"
    await client.close()


async def test_retryable_client_get_profile_app_delegates() -> None:
    """Retryable profile app get delegates to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "get_profile_app", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 12345}
        result = await client.get_profile_app(12345)

    assert result == {"id": 12345}
    mock_get.assert_awaited_once_with(12345)
    await client.close()


async def test_get_profile_device_uses_get_method_and_encoded_path() -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": 123,
        "created": "2018-01-01T01:01:01",
        "expiry": "2018-01-31T01:01:01",
        "last_authenticated": "2018-01-05T12:57:12",
        "last_remote_addr": "203.0.113.1",
        "user_agent": "Mozilla/5.0",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.get_profile_device(123)

    assert result["id"] == 123
    mock_request.assert_awaited_once_with("GET", "/profile/devices/123")
    await client.close()


async def test_get_profile_device_encodes_path_parameter() -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    unsafe_device_id: Any = "12/../34?x=1"
    response = MagicMock()
    response.json.return_value = {"id": 123}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.get_profile_device(unsafe_device_id)

    mock_request.assert_awaited_once_with(
        "GET", "/profile/devices/12%2F..%2F34%3Fx%3D1"
    )
    await client.close()


async def test_retryable_client_get_profile_device_delegates() -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "get_profile_device", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 123}
        result = await client.get_profile_device(123)

    assert result == {"id": 123}
    mock_get.assert_awaited_once_with(123)
    await client.close()


async def test_get_profile_device_wraps_http_errors() -> None:
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.get_profile_device(123)

    assert exc_info.value.operation == "GetProfileDevice"
    await client.close()


async def test_get_firewall_device() -> None:
    """Test getting a specific firewall device."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 456,
        "label": "linode-123",
        "type": "linode",
        "created": "2018-01-01T01:01:01",
        "updated": "2018-01-01T01:01:01",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_firewall_device(12345, 456)

        assert result["id"] == 456
        assert result["label"] == "linode-123"
        mock_request.assert_awaited_once()
        call_args = mock_request.call_args
        assert call_args[0][0] == "GET"
        assert "/networking/firewalls/" in call_args[0][1]
        assert "/devices/" in call_args[0][1]

    await client.close()


async def test_get_firewall_device_encodes_path_params() -> None:
    """Test that both path params are URL-encoded."""
    from urllib.parse import quote

    unsafe_device_id: Any = "456/../../../etc/passwd"

    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 456}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_firewall_device(12345, unsafe_device_id)

        call_args = mock_request.call_args
        endpoint = call_args[0][1]
        safe_fw = quote(str(12345), safe="")
        safe_dev = quote("456/../../../etc/passwd", safe="")
        expected = f"/networking/firewalls/{safe_fw}/devices/{safe_dev}"
        assert endpoint == expected

    await client.close()


async def test_get_firewall_device_wraps_http_errors() -> None:
    """Test get_firewall_device wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=MagicMock(status_code=404)
        )

        with pytest.raises(NetworkError):
            await client.get_firewall_device(12345, 456)

    await client.close()


async def test_retryable_get_firewall_device_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall device get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_firewall_device", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 456, "label": "linode-123"}
        result = await retryable.get_firewall_device(12345, 456)

    assert result == {"id": 456, "label": "linode-123"}
    mock_get.assert_awaited_once_with(12345, 456)
    await retryable.close()


async def test_list_firewall_devices() -> None:
    """Test listing firewall devices."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"id": 123, "entity": {"id": 456, "type": "linode"}}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        result = await client.list_firewall_devices(12345)

    assert result["results"] == 1
    assert result["data"][0]["id"] == 123
    mock_request.assert_called_once_with("GET", "/networking/firewalls/12345/devices")
    await client.close()


async def test_list_firewall_devices_encodes_firewall_id() -> None:
    """Firewall device list path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        await client.list_firewall_devices(cast("Any", "../12345"))

    mock_request.assert_called_once_with(
        "GET", "/networking/firewalls/..%2F12345/devices"
    )
    await client.close()


async def test_list_firewall_devices_with_pagination() -> None:
    """Test listing firewall devices with pagination parameters."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [], "page": 2, "pages": 5, "results": 0}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        result = await client.list_firewall_devices(12345, page=2, page_size=25)

    assert result["page"] == 2
    mock_request.assert_called_once_with(
        "GET", "/networking/firewalls/12345/devices?page=2&page_size=25"
    )
    await client.close()


async def test_list_firewall_devices_wraps_http_errors() -> None:
    """Test list_firewall_devices wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.list_firewall_devices(12345)

    assert "ListFirewallDevices" in str(excinfo.value)
    await client.close()


async def test_retryable_list_firewall_devices_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall device list to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_firewall_devices", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": []}
        result = await retryable.list_firewall_devices(12345, page=2, page_size=25)

    assert result == {"data": []}
    mock_list.assert_awaited_once_with(12345, page=2, page_size=25)
    await retryable.close()


async def test_get_nodebalancer_config() -> None:
    """Test getting a NodeBalancer config."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 6,
        "nodebalancer_id": 8,
        "port": 80,
        "protocol": "http",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_nodebalancer_config(8, 6)

        assert result == {
            "id": 6,
            "nodebalancer_id": 8,
            "port": 80,
            "protocol": "http",
        }
        mock_request.assert_called_once_with("GET", "/nodebalancers/8/configs/6")

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "config_id", "encoded_nodebalancer_id", "encoded_config_id"),
    [
        ("8/9", "6", "8%2F9", "6"),
        ("8", "6?x", "8", "6%3Fx"),
        ("8", "../6", "8", "..%2F6"),
    ],
)
async def test_get_nodebalancer_config_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
) -> None:
    """NodeBalancer config get path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 6}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_nodebalancer_config(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
        )

        mock_request.assert_called_once_with(
            "GET",
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/{encoded_config_id}",
        )

    await client.close()


async def test_get_nodebalancer_config_wraps_http_errors() -> None:
    """Test getting a NodeBalancer config wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_nodebalancer_config(8, 6)

    assert "GetNodeBalancerConfig" in str(excinfo.value)
    await client.close()


async def test_retryable_get_nodebalancer_config_retries_read() -> None:
    """RetryableClient routes config get through the read retry helper."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable, "_execute_with_retry", new_callable=AsyncMock
    ) as mock_retry:
        mock_retry.return_value = {"id": 6}

        result = await retryable.get_nodebalancer_config(8, 6)

    assert result == {"id": 6}
    mock_retry.assert_awaited_once_with(retryable.client.get_nodebalancer_config, 8, 6)
    await retryable.close()


async def test_get_networking_ip_sends_get_to_networking_ips_route() -> None:
    """Getting a networking IP sends GET to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "address": "198.51.100.5",
        "rdns": "example.example.com",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_networking_ip("198.51.100.5")

    assert result["address"] == "198.51.100.5"
    mock_request.assert_called_once_with("GET", "/networking/ips/198.51.100.5")

    await client.close()


async def test_get_networking_ip_url_encodes_address() -> None:
    """Path param address is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"address": "2001:db8::1", "rdns": None}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_networking_ip("2001:db8::1")

    call_args = mock_request.call_args
    assert call_args[0][1] == "/networking/ips/2001:db8::1"

    await client.close()


async def test_get_networking_ip_wraps_http_errors() -> None:
    """Getting a networking IP wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="GetNetworkingIP"):
            await client.get_networking_ip("198.51.100.5")

    mock_request.assert_awaited_once_with("GET", "/networking/ips/198.51.100.5")

    await client.close()


async def test_retryable_get_networking_ip_delegates_to_client() -> None:
    """RetryableClient delegates get_networking_ip to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_networking_ip", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"address": "10.0.0.1", "rdns": "host.example.com"}
        result = await retryable.get_networking_ip("10.0.0.1")

    assert result["address"] == "10.0.0.1"
    mock_get.assert_awaited_once_with("10.0.0.1")
    await retryable.close()


async def test_retryable_get_monitor_service_alert_definition_delegates_to_client() -> (
    None
):
    """Retryable monitor alert definition get delegates to the client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    payload = {"id": 12345, "label": "CPU high"}

    with patch.object(
        retryable,
        "_execute_with_retry",
        new_callable=AsyncMock,
    ) as mock_execute:
        mock_execute.return_value = payload
        result = await retryable.get_monitor_service_alert_definition("dbaas", 12345)

    assert result == payload
    mock_execute.assert_awaited_once_with(
        retryable.client.get_monitor_service_alert_definition, "dbaas", 12345
    )
    await retryable.close()


async def test_monitor_alert_definition_get_tool_schema_and_handler_success() -> None:
    """Monitor alert definition get tool is read-only and returns output."""
    tool, capability = create_linode_monitor_service_alert_definition_get_tool()
    assert tool.name == "linode_monitor_service_alert_definition_get"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]
    assert "service_type" in tool.input_schema["properties"]
    assert sorted(tool.input_schema["required"]) == ["alert_id", "service_type"]
    assert tool.input_schema["properties"]["alert_id"]["type"] == "integer"

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )
    payload = {"id": 12345, "label": "CPU high", "service_type": "dbaas"}

    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = payload
        result = await handle_linode_monitor_service_alert_definition_get(
            {"service_type": "dbaas", "alert_id": 12345}, cfg
        )

    mock_get.assert_awaited_once_with(
        "linode_monitor_service_alert_definition_get", "dbaas", 12345
    )
    assert "CPU high" in result[0].text
    assert "dbaas" in result[0].text


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_alert_definition_get_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "get_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_service_alert_definition_get(
            {"service_type": bad_service_type, "alert_id": 12345}, cfg
        )

    mock_get.assert_not_called()
    assert result[0].text == (
        "Error: service_type must be a single non-empty service type slug"
    )


@pytest.mark.parametrize(
    ("bad_alert_id", "expected"),
    [
        (None, "alert_id is required"),
        (True, "alert_id must be a positive integer"),
        ("12345", "alert_id must be a positive integer"),
        ("1/2", "alert_id must be a positive integer"),
        ("1?x", "alert_id must be a positive integer"),
        ("..", "alert_id must be a positive integer"),
        (12.9, "alert_id must be a positive integer"),
    ],
)
async def test_monitor_alert_definition_get_rejects_invalid_alert_id(
    bad_alert_id: object, expected: str
) -> None:
    """Handler rejects invalid alert IDs before client construction.

    The shared id reader tells an absent id from one no id can be, where the
    hook it replaces answered one sentence to both.
    """
    cfg = Config()
    args: dict[str, object] = {"service_type": "dbaas"}
    if bad_alert_id is not None:
        args["alert_id"] = bad_alert_id

    with patch.object(
        RetryableClient,
        "get_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_service_alert_definition_get(
            cast("dict[str, Any]", args), cfg
        )

    mock_get.assert_not_called()
    assert result[0].text == f"Error: {expected}"


@pytest.mark.parametrize("bad_alert_id", [0, -1])
async def test_monitor_alert_definition_get_rejects_non_positive_alert_id(
    bad_alert_id: int,
) -> None:
    """Handler rejects non-positive alert IDs before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "get_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_service_alert_definition_get(
            {"service_type": "dbaas", "alert_id": bad_alert_id}, cfg
        )

    mock_get.assert_not_called()
    assert result[0].text == "Error: alert_id must be a positive integer"


async def test_monitor_alert_definition_delete_tool_schema_and_handler_success() -> (
    None
):
    """Monitor alert definition delete tool requires confirm and returns output."""
    tool, capability = create_linode_monitor_service_alert_definition_delete_tool()
    assert tool.name == "linode_monitor_service_alert_definition_delete"
    assert capability == Capability.Destroy
    assert tool.input_schema["properties"]["confirm"]["type"] == "boolean"
    assert "service_type" in tool.input_schema["properties"]
    assert sorted(tool.input_schema["required"]) == [
        "alert_id",
        "confirm",
        "service_type",
    ]

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )

    with patch.object(
        RetryableClient,
        "route_call",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            {
                "service_type": "dbaas",
                "alert_id": 12345,
                "confirm": True,
                "confirm_bypass_dry_run": True,
            },
            cfg,
        )

    mock_delete.assert_awaited_once_with(
        "linode_monitor_service_alert_definition_delete", "dbaas", 12345, retry=False
    )
    assert "Monitor service alert definition 12345 deleted for 'dbaas'" in (
        result[0].text
    )


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_monitor_alert_definition_delete_requires_boolean_confirm(
    bad_confirm: object,
) -> None:
    """Handler rejects missing/non-true confirm before client call."""
    cfg = Config()
    args: dict[str, object] = {"service_type": "dbaas", "alert_id": 12345}
    if bad_confirm is not None:
        args["confirm"] = bad_confirm

    with (
        patch.object(
            RetryableClient, "route_call", new_callable=AsyncMock
        ) as mock_call,
        patch.object(RetryableClient, "route_raw", new_callable=AsyncMock) as mock_raw,
    ):
        result = await handle_linode_monitor_service_alert_definition_delete(
            cast("dict[str, Any]", args), cfg
        )

    mock_call.assert_not_called()
    mock_raw.assert_not_called()
    assert result[0].text == (
        "Error: This deletes a monitor alert definition. Set confirm=true to proceed."
    )


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_alert_definition_delete_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with (
        patch.object(
            RetryableClient, "route_call", new_callable=AsyncMock
        ) as mock_call,
        patch.object(RetryableClient, "route_raw", new_callable=AsyncMock) as mock_raw,
    ):
        result = await handle_linode_monitor_service_alert_definition_delete(
            {"service_type": bad_service_type, "alert_id": 12345, "confirm": True},
            cfg,
        )

    mock_call.assert_not_called()
    mock_raw.assert_not_called()
    assert result[0].text == (
        "Error: service_type must be a single non-empty service type slug"
    )


@pytest.mark.parametrize(
    ("bad_alert_id", "expected_error"),
    [
        (None, "alert_id is required"),
        (True, "alert_id must be a positive integer"),
        ("not-an-int", "alert_id must be a positive integer"),
    ],
)
async def test_monitor_alert_definition_delete_rejects_invalid_alert_id(
    bad_alert_id: object, expected_error: str
) -> None:
    """A value that is not an id never reaches the route.

    Only leaving the argument out reads as missing. A bool is not an id and a
    word is not a number, so this tier refuses both rather than reading them as
    absent, which is what Go's DestroyID does with them.
    """
    cfg = Config()
    args: dict[str, object] = {"service_type": "dbaas", "confirm": True}
    if bad_alert_id is not None:
        args["alert_id"] = bad_alert_id

    with patch.object(
        RetryableClient,
        "route_call",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            cast("dict[str, Any]", args), cfg
        )

    mock_delete.assert_not_called()
    assert result[0].text == f"Error: {expected_error}"


async def test_monitor_alert_definition_delete_rejects_a_zero_alert_id() -> None:
    """Zero is what an omitted id reads as, so it answers the same sentence."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "route_call",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            {"service_type": "dbaas", "alert_id": 0, "confirm": True},
            cfg,
        )

    mock_delete.assert_not_called()
    assert result[0].text == "Error: alert_id is required"


async def test_monitor_alert_definition_delete_rejects_a_negative_alert_id() -> None:
    """A supplied id below zero names no resource, so the tier refuses it."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "route_call",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            {"service_type": "dbaas", "alert_id": -1, "confirm": True},
            cfg,
        )

    mock_delete.assert_not_called()
    assert result[0].text == "Error: alert_id must be a positive integer"


async def test_monitor_global_alert_definitions_list_tool_success() -> None:
    """Global monitor alert definitions list is read-only and returns output."""
    tool, capability = create_linode_monitor_alert_definition_list_tool()
    assert tool.name == "linode_monitor_alert_definition_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]
    assert "required" not in tool.input_schema

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )

    response_payload = {
        "data": [{"id": 123, "label": "CPU Usage"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_alert_definition_list({}, cfg)

    mock_list.assert_awaited_once_with("linode_monitor_alert_definition_list", query="")
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["alert_definitions"][0]["id"] == 123
    assert payload["alert_definitions"][0]["label"] == "CPU Usage"


async def test_monitor_alert_channels_list_tool_schema_and_handler_success() -> None:
    """Monitor alert channels list tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_alert_channel_list_tool()
    assert tool.name == "linode_monitor_alert_channel_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )

    response_payload = {
        "data": [{"id": 10000, "label": "Email Ops", "type": "email"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_alert_channel_list({}, cfg)

    mock_list.assert_awaited_once_with("linode_monitor_alert_channel_list", query="")
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["alert_channels"][0]["id"] == 10000
    assert payload["alert_channels"][0]["label"] == "Email Ops"
    assert payload["alert_channels"][0]["channel_type"] == ""


async def test_monitor_alert_definitions_list_tool_schema_and_handler_success() -> None:
    """Monitor alert definitions list tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_service_alert_definition_list_tool()
    assert tool.name == "linode_monitor_service_alert_definition_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["service_type"]

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )

    response_payload = {
        "data": [{"id": 123, "label": "CPU Usage"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_service_alert_definition_list(
            {"service_type": "dbaas"}, cfg
        )

    mock_list.assert_awaited_once_with(
        "linode_monitor_service_alert_definition_list", "dbaas", query=""
    )
    payload = json.loads(result[0].text)
    assert payload["count"] == 1
    assert payload["alert_definitions"][0]["id"] == 123
    assert payload["alert_definitions"][0]["label"] == "CPU Usage"


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_alert_definitions_list_handler_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        result = await handle_linode_monitor_service_alert_definition_list(
            {"service_type": bad_service_type}, cfg
        )

    mock_list.assert_not_called()
    assert result[0].text == (
        "Error: service_type must be a single non-empty service type slug"
    )


async def test_monitor_dashboard_get_tool_schema_and_handler_success() -> None:
    """Monitor dashboard get tool is read-only and returns output."""
    tool, capability = create_linode_monitor_dashboard_get_tool()
    assert tool.name == "linode_monitor_dashboard_get"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["dashboard_id"]
    assert tool.input_schema["properties"]["dashboard_id"]["type"] == "integer"

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )
    payload = {
        "id": 12345,
        "label": "Resource Usage",
        "widgets": [{"metric": "cpu"}],
        "not_in_proto": "dropped",
    }

    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = payload
        result = await handle_linode_monitor_dashboard_get({"dashboard_id": 12345}, cfg)

    mock_get.assert_awaited_once_with("linode_monitor_dashboard_get", 12345)
    assert "Resource Usage" in result[0].text
    assert "not_in_proto" not in result[0].text


@pytest.mark.parametrize(
    ("bad_dashboard_id", "expected"),
    [
        (None, "Error: dashboard_id is required"),
        (True, "Error: dashboard_id must be a positive integer"),
        ("12345", "Error: dashboard_id must be a positive integer"),
        ("1/2", "Error: dashboard_id must be a positive integer"),
        ("1?x", "Error: dashboard_id must be a positive integer"),
        ("..", "Error: dashboard_id must be a positive integer"),
        (12.9, "Error: dashboard_id must be a positive integer"),
    ],
)
async def test_monitor_dashboard_get_rejects_invalid_dashboard_id(
    bad_dashboard_id: object, expected: str
) -> None:
    """Handler rejects invalid dashboard IDs before client construction."""
    cfg = Config()
    args: dict[str, object] = {}
    if bad_dashboard_id is not None:
        args["dashboard_id"] = bad_dashboard_id

    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_dashboard_get(
            cast("dict[str, Any]", args), cfg
        )

    mock_get.assert_not_called()
    # The tool reads its id through the contract's positive-id reader now, which
    # separates an absent argument from a present one that is not an id. Both
    # sentences are Go's, which this tool used to answer neither of.
    assert result[0].text == expected


@pytest.mark.parametrize("bad_dashboard_id", [0, -1])
async def test_monitor_dashboard_get_rejects_non_positive_dashboard_id(
    bad_dashboard_id: int,
) -> None:
    """Handler rejects non-positive dashboard IDs before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_dashboard_get(
            {"dashboard_id": bad_dashboard_id}, cfg
        )

    mock_get.assert_not_called()
    assert result[0].text == "Error: dashboard_id must be a positive integer"


async def test_monitor_dashboards_list_tool_schema_and_handler_success() -> None:
    """Monitor dashboards list tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_dashboard_list_tool()
    assert tool.name == "linode_monitor_dashboard_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]
    assert "required" not in tool.input_schema

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )

    response_payload = {
        "data": [
            {
                "id": 1,
                "label": "Resource Usage",
                "type": "standard",
                "service_type": "dbaas",
                "widgets": [{"metric": "cpu_usage", "chart_type": "line"}],
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_dashboard_list({}, cfg)

    mock_list.assert_awaited_once_with("linode_monitor_dashboard_list", query="")
    body = json.loads(result[0].text)
    assert body["count"] == 1
    assert body["dashboards"][0]["label"] == "Resource Usage"
    assert body["dashboards"][0]["widgets"][0]["metric"] == "cpu_usage"
    assert "message" not in body


async def test_monitor_service_dashboards_tool_schema_and_handler_success() -> None:
    """Monitor service dashboards tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_service_dashboard_list_tool()
    assert tool.name == "linode_monitor_service_dashboard_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.input_schema["properties"]
    assert tool.input_schema["required"] == ["service_type"]

    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="test-token",
                ),
            )
        }
    )

    response_payload = {
        "data": [
            {
                "id": 1,
                "label": "Resource Usage",
                "type": "standard",
                "service_type": "dbaas",
                "widgets": [{"metric": "memory_usage", "chart_type": "area"}],
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_service_dashboard_list(
            {"service_type": "dbaas"}, cfg
        )

    mock_list.assert_awaited_once_with(
        "linode_monitor_service_dashboard_list", "dbaas", query=""
    )
    body = json.loads(result[0].text)
    assert body["count"] == 1
    assert body["dashboards"][0]["label"] == "Resource Usage"
    assert body["dashboards"][0]["widgets"][0]["metric"] == "memory_usage"
    assert "message" not in body
    assert "service_type" not in body


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_dashboards_handler_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "route_raw",
        new_callable=AsyncMock,
    ) as mock_list:
        result = await handle_linode_monitor_service_dashboard_list(
            {"service_type": bad_service_type}, cfg
        )

    mock_list.assert_not_called()
    assert result[0].text == (
        "Error: service_type must be a single non-empty service type slug"
    )


async def test_list_instance_volumes_sends_exact_method_path_query() -> None:
    """Linode volumes list sends documented method, path, and query."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.json.return_value = {"data": [{"id": 123}], "page": 1, "results": 1}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        result = await client.list_instance_volumes(42, page=2, page_size=25)

    assert result["data"][0]["id"] == 123
    mock_request.assert_awaited_once_with(
        "GET", "/linode/instances/42/volumes?page=2&page_size=25"
    )
    await client.close()


@pytest.mark.parametrize("linode_id", ["bad/id", "bad?query", "..", True, 0, -1])
async def test_list_instance_volumes_rejects_malformed_linode_id(
    linode_id: object,
) -> None:
    """Linode volumes list rejects malformed path params before request."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="linode_id must be a positive integer"),
    ):
        await client.list_instance_volumes(linode_id)  # type: ignore[arg-type]

    mock_request.assert_not_called()
    await client.close()


async def test_list_instance_volumes_wraps_http_errors() -> None:
    """Linode volumes list maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.list_instance_volumes(42)

    assert "ListInstanceVolumes" in str(exc_info.value)
    await client.close()


async def test_retryable_list_instance_volumes_delegates_with_retry() -> None:
    """Retryable Linode volumes list delegates through retry wrapper."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_instance_volumes", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [{"id": 123}], "results": 1}
        result = await client.list_instance_volumes(42, page=1, page_size=25)

    assert result["results"] == 1
    mock_list.assert_awaited_once_with(42, page=1, page_size=25)
    await client.close()


async def test_list_instance_firewalls_sends_exact_method_path_query() -> None:
    """Linode firewalls list sends documented method, path, and query."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.json.return_value = {"data": [{"id": 123}], "page": 1, "results": 1}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        result = await client.list_instance_firewalls(42, page=2, page_size=25)

    assert result["data"][0]["id"] == 123
    mock_request.assert_awaited_once_with(
        "GET", "/linode/instances/42/firewalls?page=2&page_size=25"
    )
    await client.close()


@pytest.mark.parametrize("linode_id", ["bad/id", "bad?query", "..", True, 0, -1])
async def test_list_instance_firewalls_rejects_malformed_linode_id(
    linode_id: object,
) -> None:
    """Linode firewalls list rejects malformed path params before request."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="linode_id must be a positive integer"),
    ):
        await client.list_instance_firewalls(linode_id)  # type: ignore[arg-type]

    mock_request.assert_not_called()
    await client.close()


async def test_list_instance_firewalls_wraps_http_errors() -> None:
    """Linode firewalls list maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.list_instance_firewalls(42)

    assert "ListInstanceFirewalls" in str(exc_info.value)
    await client.close()


async def test_retryable_list_instance_firewalls_delegates_with_retry() -> None:
    """Retryable Linode firewalls list delegates through retry wrapper."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_instance_firewalls", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [{"id": 123}], "results": 1}
        result = await client.list_instance_firewalls(42, page=1, page_size=25)

    assert result["results"] == 1
    mock_list.assert_awaited_once_with(42, page=1, page_size=25)
    await client.close()


async def test_handle_linode_longview_subscription_get_success(
    sample_config: Config,
) -> None:
    """Longview subscription handler returns the client result."""
    from linodemcp.gentools import handle_linode_longview_subscription_get

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.route_raw.return_value = {
            "id": "longview-10",
            "label": "Longview Pro",
            "price": {"hourly": 0, "monthly": 0},
        }
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await handle_linode_longview_subscription_get(
            {"subscription_id": "longview-10"}, sample_config
        )

    body = json.loads(result[0].text)
    assert body["id"] == "longview-10"
    assert body["label"] == "Longview Pro"
    assert body["price"] == {"hourly": 0.0, "monthly": 0.0}
    mock_client.route_raw.assert_awaited_once_with(
        "linode_longview_subscription_get", "longview-10"
    )


@pytest.mark.parametrize("subscription_id", [None, 0, True, "123/../x", "123?x=1"])
async def test_handle_linode_longview_subscription_get_rejects_bad_id(
    sample_config: Config, subscription_id: object
) -> None:
    """Longview subscription handler rejects bad IDs before client dispatch."""
    from linodemcp.gentools import handle_linode_longview_subscription_get

    arguments = {} if subscription_id is None else {"subscription_id": subscription_id}
    with patch("linodemcp.gentools.longview.run_get_tool") as execute_tool:
        result = await handle_linode_longview_subscription_get(arguments, sample_config)

    if subscription_id is None:
        expected = "Error: subscription_id is required"
    elif not isinstance(subscription_id, str):
        expected = "Error: subscription_id must be a non-empty string"
    else:
        expected = (
            "Error: subscription_id must not contain path separators, "
            "query separators, or traversal segments"
        )
    assert result[0].text == expected
    execute_tool.assert_not_called()


async def test_get_longview_plan_sends_get_to_longview_plan_route() -> None:
    """Longview plan get sends GET /longview/plan."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = httpx.Response(200, json={"label": "Longview Pro"})

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.get_longview_plan()

    assert result == {"label": "Longview Pro"}
    mock_request.assert_awaited_once_with("GET", "/longview/plan")
    await client.close()


async def test_get_longview_plan_wraps_http_errors() -> None:
    """Longview plan get maps HTTP errors to GetLongviewPlan."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError, match="GetLongviewPlan"):
            await client.get_longview_plan()

    await client.close()


async def test_retryable_client_get_longview_plan_delegates() -> None:
    """RetryableClient delegates Longview plan get to Client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    mock_method = AsyncMock(return_value={"label": "Longview Pro"})
    object.__setattr__(client.client, "get_longview_plan", mock_method)

    try:
        result = await client.get_longview_plan()

        assert result == {"label": "Longview Pro"}
        mock_method.assert_awaited_once_with()
    finally:
        await client.close()


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_longview_client_create_requires_boolean_confirm(
    sample_config: Config, bad_confirm: object
) -> None:
    """Handler rejects missing/non-true confirm before client construction."""
    args: dict[str, Any] = {"label": "web-01"}
    if bad_confirm is not None:
        args["confirm"] = bad_confirm

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_retryable:
        result = await handle_linode_longview_client_create(args, sample_config)

    assert result[0].text == (
        "Error: This creates a Longview client and returns setup credentials. Set "
        "confirm=true to proceed."
    )
    mock_retryable.assert_not_called()


@pytest.mark.parametrize(
    "bad_label",
    [None, "", "ab", "bad label", "åbc", "x" * 33, "foo/bar", "foo?bar", "../foo"],
)
async def test_longview_client_create_rejects_invalid_label(
    sample_config: Config, bad_label: object
) -> None:
    """Handler rejects malformed labels before client construction."""
    args: dict[str, Any] = {"label": bad_label, "confirm": True}

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_retryable:
        result = await handle_linode_longview_client_create(args, sample_config)

    assert result[0].text.startswith("Error: ")
    mock_retryable.assert_not_called()


async def test_longview_client_create_handler_returns_success(
    sample_config: Config,
) -> None:
    """Handler returns the created Longview client as a proto-canonical envelope
    with the save-the-secret warning and the full CreatedLongviewClient element.
    """
    response_data = {
        "id": 123,
        "label": "web-01",
        "install_code": "abc",
        "api_key": "key-xyz",
    }
    mock_client = AsyncMock()
    mock_client.route_raw.return_value = response_data
    mock_cm = AsyncMock()
    mock_cm.__aenter__.return_value = mock_client
    mock_cm.__aexit__.return_value = None

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=mock_cm):
        result = await handle_linode_longview_client_create(
            {"label": "web-01", "confirm": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["message"] == "Longview client created successfully"
    assert payload["warning"].startswith("IMPORTANT: Save the API key")
    # The one-time install secret survives onto the output element by design.
    assert payload["longview_client"]["api_key"] == "key-xyz"
    assert payload["longview_client"]["install_code"] == "abc"
    assert payload["longview_client"]["id"] == 123
    assert payload["longview_client"]["label"] == "web-01"
    # retry_disabled on the contract: a replay would file a second client under
    # the same label.
    mock_client.route_raw.assert_awaited_once_with(
        "linode_longview_client_create", body={"label": "web-01"}, retry=False
    )


async def test_longview_client_create_dry_run_includes_request_body(
    sample_config: Config,
) -> None:
    """Dry-run previews the Longview client create request without a client call."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_retryable:
        result = await handle_linode_longview_client_create(
            {"label": "web-01", "confirm": True, "dry_run": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert payload["would_execute"] == {
        "method": "POST",
        "path": "/longview/clients",
        "body": {"label": "web-01"},
    }
    assert payload["current_state"] is None
    mock_retryable.assert_not_called()
