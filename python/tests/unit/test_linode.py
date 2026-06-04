"""Unit tests for Linode client."""

import asyncio
from typing import Any, cast
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.config import Config, EnvironmentConfig, LinodeConfig
from linodemcp.linode import (
    Account,
    APIError,
    CircuitBreaker,
    CircuitOpenError,
    Client,
    DomainZoneFile,
    Firewall,
    FirewallAddresses,
    FirewallRule,
    FirewallRules,
    Grant,
    Grants,
    NetworkError,
    Profile,
    RateLimiter,
    Region,
    Resolver,
    RetryableClient,
    RetryConfig,
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
from linodemcp.tools.linode_monitor_write import (
    create_linode_monitor_alert_channels_list_tool,
    create_linode_monitor_alert_definitions_list_tool,
    create_linode_monitor_dashboard_get_tool,
    create_linode_monitor_dashboards_list_tool,
    create_linode_monitor_service_alert_definition_create_tool,
    create_linode_monitor_service_alert_definition_delete_tool,
    create_linode_monitor_service_alert_definition_get_tool,
    create_linode_monitor_service_alert_definitions_list_tool,
    create_linode_monitor_service_dashboards_list_tool,
    handle_linode_monitor_alert_channels_list,
    handle_linode_monitor_alert_definitions_list,
    handle_linode_monitor_dashboard_get,
    handle_linode_monitor_dashboards_list,
    handle_linode_monitor_service_alert_definition_create,
    handle_linode_monitor_service_alert_definition_delete,
    handle_linode_monitor_service_alert_definition_get,
    handle_linode_monitor_service_alert_definitions_list,
    handle_linode_monitor_service_dashboards_list,
)


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


async def test_update_profile_sends_put_to_profile_route(
    sample_profile_data: dict[str, Any],
) -> None:
    """Test updating user profile sends PUT /profile."""
    client = Client("https://api.linode.com/v4", "test-token")

    updated_profile = {**sample_profile_data, "email": "updated@example.com"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = updated_profile

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.update_profile(
            email="updated@example.com",
            timezone="UTC",
            email_notifications=None,
        )

    assert isinstance(profile, Profile)
    assert profile.email == "updated@example.com"
    mock_request.assert_called_once_with(
        "PUT",
        "/profile",
        {"email": "updated@example.com", "timezone": "UTC"},
    )

    await client.close()


async def test_get_profile_preferences_sends_get_to_preferences_route() -> None:
    """Getting profile preferences sends GET /profile/preferences."""
    client = Client("https://api.linode.com/v4", "test-token")
    preferences = {"dashboard": {"theme": "dark"}, "dismissed": ["welcome"]}

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = preferences

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_profile_preferences()

    assert result == preferences
    mock_request.assert_called_once_with("GET", "/profile/preferences")

    await client.close()


async def test_get_profile_preferences_non_dict_response_empty() -> None:
    """Unexpected profile preferences GET response shapes return an empty object."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = []

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_profile_preferences()

    assert result == {}
    mock_request.assert_called_once_with("GET", "/profile/preferences")

    await client.close()


async def test_get_profile_preferences_wraps_http_errors() -> None:
    """HTTP errors from profile preferences reads are wrapped."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_profile_preferences()

    assert "GetProfilePreferences" in str(excinfo.value)
    await client.close()


async def test_retryable_get_profile_preferences_delegates_to_client() -> None:
    """Retryable profile preferences get delegates to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    preferences = {"dashboard": {"theme": "dark"}}

    with patch.object(
        client.client, "get_profile_preferences", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = preferences

        result = await client.get_profile_preferences()

    assert result == preferences
    mock_get.assert_awaited_once_with()
    await client.close()


async def test_update_profile_preferences_sends_put_to_preferences_route() -> None:
    """Updating profile preferences sends PUT /profile/preferences."""
    client = Client("https://api.linode.com/v4", "test-token")
    preferences = {"dashboard": {"theme": "dark"}, "dismissed": ["welcome"]}

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = preferences

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_profile_preferences(preferences)

    assert result == preferences
    mock_request.assert_called_once_with("PUT", "/profile/preferences", preferences)

    await client.close()


async def test_update_profile_preferences_non_dict_response_empty() -> None:
    """Unexpected profile preferences response shapes return an empty object."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = []

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_profile_preferences({})

    assert result == {}
    mock_request.assert_called_once_with("PUT", "/profile/preferences", {})

    await client.close()


async def test_update_profile_preferences_wraps_http_errors() -> None:
    """HTTP errors from profile preferences updates are wrapped."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_profile_preferences({})

    assert "UpdateProfilePreferences" in str(excinfo.value)
    await client.close()


async def test_retryable_update_profile_preferences_delegates_to_client() -> None:
    """Retryable profile preferences update delegates to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    preferences = {"dashboard": {"theme": "dark"}}

    with patch.object(
        client.client, "update_profile_preferences", new_callable=AsyncMock
    ) as mock_update:
        mock_update.return_value = preferences

        result = await client.update_profile_preferences(preferences)

    assert result == preferences
    mock_update.assert_awaited_once_with(preferences)
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


async def test_get_account_agreements_sends_get_to_account_agreements_route() -> None:
    """Test listing account agreements sends GET /account/agreements."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [
            {
                "id": "eu_model",
                "label": "EU Model Contract",
                "description": "Contract terms",
            }
        ]
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_agreements()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/agreements")
    await client.close()


async def test_get_account_agreements_wraps_http_errors() -> None:
    """Test listing account agreements wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_agreements()

    assert "GetAccountAgreements" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_agreements_delegates_to_client() -> None:
    """Test RetryableClient delegates account agreements listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_agreements", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"data": []}
        result = await retryable.get_account_agreements()

    assert result["data"] == []
    mock_get.assert_awaited_once_with()
    await retryable.close()


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


async def test_enable_account_managed_sends_exact_route() -> None:
    """Account managed enable sends POST /account/settings/managed-enable."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"managed": True}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.enable_account_managed()

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/settings/managed-enable")
    await client.close()


async def test_enable_account_managed_wraps_http_errors() -> None:
    """Account managed enable wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.enable_account_managed()

    assert "EnableAccountManaged" in str(excinfo.value)
    await client.close()


async def test_retryable_enable_account_managed_delegates_once_without_retry() -> None:
    """RetryableClient does not replay account managed enable on errors."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "enable_account_managed", new_callable=AsyncMock
    ) as mock_enable:
        mock_enable.side_effect = httpx.HTTPError("boom")

        with pytest.raises(httpx.HTTPError):
            await retryable.enable_account_managed()

    mock_enable.assert_awaited_once_with()
    await retryable.close()


async def test_get_account_transfer_sends_exact_route() -> None:
    """Account transfer get sends GET /account/transfer."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "billable": 12.5,
        "quota": 5000,
        "used": 42.0,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_transfer()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/transfer")
    await client.close()


async def test_get_account_transfer_wraps_http_errors() -> None:
    """Account transfer get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_transfer()

    assert "GetAccountTransfer" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_transfer_delegates_to_client() -> None:
    """RetryableClient delegates account transfer get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_transfer", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"quota": 5000}
        result = await retryable.get_account_transfer()

    assert result == {"quota": 5000}
    mock_get.assert_awaited_once_with()
    await retryable.close()


async def test_list_account_maintenance_sends_exact_route() -> None:
    """Account maintenance listing sends GET /account/maintenance."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {
        "data": [{"entity": {"id": 123, "type": "linode"}, "status": "pending"}],
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_maintenance()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/maintenance")
    await client.close()


async def test_list_account_maintenance_wraps_http_errors() -> None:
    """Account maintenance listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_maintenance()

    assert "ListAccountMaintenance" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_maintenance_delegates_to_client() -> None:
    """RetryableClient delegates account maintenance listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_maintenance", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": []}
        result = await retryable.list_account_maintenance()

    mock_list.assert_awaited_once_with()
    assert result == {"data": []}
    await retryable.close()


async def test_get_account_beta_sends_exact_route() -> None:
    """Test getting an account beta sends GET /account/betas/{betaId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "example-open", "label": "Example Open Beta"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = response_data
        mock_request.return_value = mock_response

        result = await client.get_account_beta("example-open")

    mock_request.assert_called_once_with("GET", "/account/betas/example-open")
    assert result == response_data


async def test_get_account_beta_url_encodes_beta_id() -> None:
    """Test account beta ID is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {"id": "example/open?query"}
        mock_request.return_value = mock_response

        await client.get_account_beta("example/open?query")

    mock_request.assert_called_once_with("GET", "/account/betas/example%2Fopen%3Fquery")


async def test_get_account_beta_wraps_http_errors() -> None:
    """Test getting an account beta wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("Network error")

        with pytest.raises(NetworkError, match="GetAccountBeta"):
            await client.get_account_beta("example-open")


async def test_retryable_get_account_beta_delegates_to_client() -> None:
    """Test RetryableClient delegates account beta get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    response_data = {"id": "example-open"}

    with patch.object(
        retryable.client, "get_account_beta", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = response_data

        result = await retryable.get_account_beta("example-open")

    mock_get.assert_awaited_once_with("example-open")
    assert result == response_data


async def test_list_account_logins_sends_exact_route_with_query() -> None:
    """Account logins listing sends GET /account/logins with pagination query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [
            {"id": 123, "ip": "192.0.2.10"},
            {"id": 456, "ip": "192.0.2.11"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_logins(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/logins?page=2&page_size=25")
    await client.close()


async def test_list_account_logins_sends_exact_route_without_query_or_body() -> None:
    """Account logins listing sends GET /account/logins without pagination query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, object] = {"data": [], "page": 1, "pages": 1, "results": 0}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_logins()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/logins")
    await client.close()


async def test_list_account_logins_wraps_http_errors() -> None:
    """Account logins listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_logins()

    assert "ListAccountLogins" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_logins_delegates_to_client() -> None:
    """RetryableClient delegates account login listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_logins", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_logins(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_list_account_users_sends_exact_route_with_query() -> None:
    """Account users listing sends GET /account/users with pagination query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [
            {"username": "alice", "email": "alice@example.com", "restricted": False},
            {"username": "bob", "email": "bob@example.com", "restricted": True},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_users(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/users?page=2&page_size=25")
    await client.close()


async def test_list_account_users_sends_exact_route_without_query_or_body() -> None:
    """Account users listing sends GET /account/users without query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, object] = {"data": [], "page": 1, "pages": 1, "results": 0}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_users()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/users")
    await client.close()


async def test_list_account_users_wraps_http_errors() -> None:
    """Account users listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_users()

    assert "ListAccountUsers" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_users_delegates_to_client() -> None:
    """RetryableClient delegates account users listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_users", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_users(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_delete_account_user_sends_exact_route() -> None:
    """Account user deletion sends DELETE /account/users/{username}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, object] = {}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.delete_account_user("alice")

    assert result == response_data
    mock_request.assert_called_once_with("DELETE", "/account/users/alice")
    await client.close()


async def test_delete_account_user_url_encodes_username() -> None:
    """Account user deletion URL-encodes username at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.delete_account_user("team/user?x")

    mock_request.assert_called_once_with("DELETE", "/account/users/team%2Fuser%3Fx")
    await client.close()


async def test_delete_account_user_wraps_http_errors() -> None:
    """Account user deletion wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_account_user("alice")

    assert "DeleteAccountUser" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_account_user_delegates_once() -> None:
    """RetryableClient does not replay destructive account user deletion."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_account_user", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await retryable.delete_account_user("alice")

    mock_delete.assert_awaited_once_with("alice")
    await retryable.close()


async def test_list_account_oauth_clients_sends_exact_route_with_query() -> None:
    """Account OAuth clients listing sends GET /account/oauth-clients."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"id": "client-1", "label": "Example client"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_oauth_clients(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/oauth-clients?page=2&page_size=25"
    )
    await client.close()


async def test_list_account_oauth_clients_sends_exact_route_without_query() -> None:
    """Account OAuth clients listing omits query when none are provided."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"data": [{"id": "client-1"}], "page": 1, "pages": 1, "results": 1}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_oauth_clients()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/oauth-clients")
    await client.close()


async def test_list_account_oauth_clients_wraps_http_errors() -> None:
    """Account OAuth clients listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_oauth_clients()

    assert "ListAccountOAuthClients" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_oauth_clients_delegates_to_client() -> None:
    """RetryableClient delegates account OAuth client listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_oauth_clients", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_oauth_clients(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_update_account_oauth_client_sends_exact_route_and_body() -> None:
    """Account OAuth client update sends PUT /account/oauth-clients/{clientId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "client-1", "label": "Updated client", "public": True}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_account_oauth_client(
            "client-1", label="Updated client", public=True, secret=None
        )

    assert result == response_data
    mock_request.assert_called_once_with(
        "PUT",
        "/account/oauth-clients/client-1",
        {"label": "Updated client", "public": True},
    )
    await client.close()


async def test_update_account_oauth_client_encodes_path_parameter() -> None:
    """Account OAuth client update URL-encodes path separators."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "client/123?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_account_oauth_client("client/123?query", label="Updated")

    mock_request.assert_called_once_with(
        "PUT",
        "/account/oauth-clients/client%2F123%3Fquery",
        {"label": "Updated"},
    )
    await client.close()


async def test_update_account_oauth_client_wraps_http_errors() -> None:
    """Account OAuth client update wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_account_oauth_client("client-1", label="Updated")

    assert "UpdateAccountOAuthClient" in str(excinfo.value)
    await client.close()


async def test_retryable_update_account_oauth_client_delegates_to_client() -> None:
    """RetryableClient delegates account OAuth client update to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_account_oauth_client", new_callable=AsyncMock
    ) as mock_update:
        mock_update.return_value = {"id": "client-1", "label": "Updated"}
        result = await retryable.update_account_oauth_client(
            "client-1", label="Updated"
        )

    mock_update.assert_awaited_once_with("client-1", label="Updated")
    assert result == {"id": "client-1", "label": "Updated"}
    await retryable.close()


async def test_update_account_oauth_client_thumbnail_sends_route() -> None:
    """OAuth client thumbnail update sends PUT with an empty JSON body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "client-1", "thumbnail_url": "https://example.com/t.png"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_account_oauth_client_thumbnail("client-1")

    assert result == response_data
    mock_request.assert_called_once_with(
        "PUT", "/account/oauth-clients/client-1/thumbnail", {}
    )
    await client.close()


async def test_update_account_oauth_client_thumbnail_encodes_client_id() -> None:
    """OAuth client thumbnail update URL-encodes path separators."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "client/123?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_account_oauth_client_thumbnail("client/123?query")

    mock_request.assert_called_once_with(
        "PUT", "/account/oauth-clients/client%2F123%3Fquery/thumbnail", {}
    )
    await client.close()


async def test_update_account_oauth_client_thumbnail_wraps_http_errors() -> None:
    """OAuth client thumbnail update wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_account_oauth_client_thumbnail("client-1")

    assert "UpdateAccountOAuthClientThumbnail" in str(excinfo.value)
    await client.close()


async def test_retryable_update_account_oauth_client_thumbnail_delegates_once() -> None:
    """RetryableClient delegates thumbnail update without replaying the write."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with (
        patch.object(
            retryable.client,
            "update_account_oauth_client_thumbnail",
            new_callable=AsyncMock,
        ) as mock_update,
        patch.object(
            retryable, "_execute_with_retry", new_callable=AsyncMock
        ) as mock_retry,
    ):
        mock_update.return_value = {"id": "client-1"}
        result = await retryable.update_account_oauth_client_thumbnail("client-1")

    mock_update.assert_awaited_once_with("client-1")
    mock_retry.assert_not_called()
    assert result == {"id": "client-1"}
    await retryable.close()


async def test_list_account_events_sends_exact_route_with_query() -> None:
    """Account events listing sends GET /account/events."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"id": 123, "action": "linode_create", "status": "finished"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_events(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/events?page=2&page_size=25")
    await client.close()


async def test_list_account_events_wraps_http_errors() -> None:
    """Account events listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_events()

    assert "ListAccountEvents" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_events_delegates_to_client() -> None:
    """RetryableClient delegates account event listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_events", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_events(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_list_account_invoices_sends_exact_route_with_query() -> None:
    """Account invoices listing sends GET /account/invoices."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"id": 123, "label": "Invoice #123"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_invoices(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/invoices?page=2&page_size=25")
    await client.close()


async def test_list_account_invoices_sends_exact_route_without_query() -> None:
    """Account invoices listing omits query parameters when none are provided."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"data": [{"id": 123}], "page": 1, "pages": 1, "results": 1}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_invoices()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/invoices")
    await client.close()


async def test_list_account_invoices_wraps_http_errors() -> None:
    """Account invoices listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_invoices()

    assert "ListAccountInvoices" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_invoices_delegates_to_client() -> None:
    """RetryableClient delegates account invoice listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_invoices", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_invoices(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_list_account_payments_sends_exact_route_with_query() -> None:
    """Account payments listing sends GET /account/payments."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"id": 123, "date": "2024-01-02T03:04:05"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_payments(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/payments?page=2&page_size=25")
    await client.close()


async def test_list_account_payments_sends_exact_route_without_query() -> None:
    """Account payments listing omits query parameters when none are provided."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"data": [{"id": 123}], "page": 1, "pages": 1, "results": 1}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_payments()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/payments")
    await client.close()


async def test_list_account_payments_wraps_http_errors() -> None:
    """Account payments listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_payments()

    assert "ListAccountPayments" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_payments_delegates_to_client() -> None:
    """RetryableClient delegates account payment listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_payments", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_payments(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_list_account_payment_methods_sends_exact_route_with_query() -> None:
    """Account payment method listing sends GET /account/payment-methods."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"id": 123, "type": "credit_card", "is_default": True}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_payment_methods(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/payment-methods?page=2&page_size=25"
    )
    await client.close()


async def test_list_account_payment_methods_sends_exact_route_without_query() -> None:
    """Account payment method listing omits query params by default."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"id": 123, "type": "credit_card"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_payment_methods()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/payment-methods")
    await client.close()


async def test_list_account_payment_methods_wraps_http_errors() -> None:
    """Account payment method listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_payment_methods()

    assert "ListAccountPaymentMethods" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_payment_methods_delegates_to_client() -> None:
    """RetryableClient delegates account payment method listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_payment_methods", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_payment_methods(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_list_account_notifications_sends_exact_route_with_query() -> None:
    """Account notifications listing sends GET /account/notifications."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"type": "ticket_important", "message": "Ticket updated"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_notifications(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/notifications?page=2&page_size=25"
    )
    await client.close()


async def test_list_account_notifications_sends_exact_route_without_query() -> None:
    """Account notifications listing omits query parameters when none are provided."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"type": "ticket_important"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_notifications()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/notifications")
    await client.close()


async def test_list_account_notifications_wraps_http_errors() -> None:
    """Account notifications listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_notifications()

    assert "ListAccountNotifications" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_notifications_delegates_to_client() -> None:
    """RetryableClient delegates account notification listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_notifications", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_notifications(page=1, page_size=100)

    mock_list.assert_awaited_once_with(page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_list_account_invoice_items_sends_exact_route_with_query() -> None:
    """Account invoice item listing sends GET /account/invoices/{invoiceId}/items."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"label": "Compute Instance", "amount": 12.34}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_invoice_items(123, page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/invoices/123/items?page=2&page_size=25"
    )
    await client.close()


async def test_list_account_invoice_items_encodes_invoice_id() -> None:
    """Account invoice item listing URL-encodes the invoice path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, object] = {"data": [], "page": 1, "pages": 1, "results": 0}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_invoice_items("123/../456")  # type: ignore[arg-type]

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/invoices/123%2F..%2F456/items"
    )
    await client.close()


async def test_list_account_invoice_items_wraps_http_errors() -> None:
    """Account invoice item listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_invoice_items(123)

    assert "ListAccountInvoiceItems" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_invoice_items_delegates_to_client() -> None:
    """RetryableClient delegates account invoice item listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_invoice_items", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_invoice_items(123, page=1, page_size=100)

    mock_list.assert_awaited_once_with(123, page=1, page_size=100)
    assert result == {"data": [], "page": 1, "pages": 1, "results": 0}
    await retryable.close()


async def test_delete_account_payment_method_sends_exact_route() -> None:
    """Account payment-method delete sends DELETE /account/payment-methods/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.delete_account_payment_method(123)

    assert result == response_data
    mock_request.assert_called_once_with("DELETE", "/account/payment-methods/123")
    await client.close()


async def test_delete_account_payment_method_url_encodes_id() -> None:
    """Account payment-method delete URL-encodes the ID path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.delete_account_payment_method("1/2?x")

    mock_request.assert_called_once_with("DELETE", "/account/payment-methods/1%2F2%3Fx")
    await client.close()


async def test_delete_account_payment_method_wraps_http_errors() -> None:
    """Account payment-method delete wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_account_payment_method(123)

    assert "DeleteAccountPaymentMethod" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_account_payment_method_delegates_once() -> None:
    """RetryableClient delegates destructive payment-method deletion once."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_account_payment_method", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await retryable.delete_account_payment_method(123)

    mock_delete.assert_awaited_once_with(123)
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


async def test_mark_account_event_seen_sends_exact_route() -> None:
    """Account event seen sends POST /account/events/{eventId}/seen."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.mark_account_event_seen(123)

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/events/123/seen")
    await client.close()


async def test_mark_account_event_seen_url_encodes_event_id() -> None:
    """Account event seen URL-encodes the event_id path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.mark_account_event_seen(cast("int", "1/2?x"))

    mock_request.assert_called_once_with("POST", "/account/events/1%2F2%3Fx/seen")
    await client.close()


async def test_mark_account_event_seen_wraps_http_errors() -> None:
    """Account event seen wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.mark_account_event_seen(123)

    assert "MarkAccountEventSeen" in str(excinfo.value)
    await client.close()


async def test_retryable_mark_account_event_seen_delegates_without_retry() -> None:
    """RetryableClient delegates mark seen once without retry replay."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "mark_account_event_seen", new_callable=AsyncMock
    ) as mock_mark:
        mock_mark.return_value = {}
        result = await retryable.mark_account_event_seen(123)

    mock_mark.assert_awaited_once_with(123)
    assert result == {}
    await retryable.close()


async def test_retryable_mark_account_event_seen_does_not_replay_transient_errors() -> (
    None
):
    """Mutating mark-seen is not retried after a transient failure."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "mark_account_event_seen", new_callable=AsyncMock
    ) as mock_mark:
        mock_mark.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await retryable.mark_account_event_seen(123)

    mock_mark.assert_awaited_once_with(123)
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


async def test_accept_account_service_transfer_sends_exact_route() -> None:
    """Service transfer accept sends POST /account/service-transfers/{token}/accept."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "token": "***",
        "accepted": True,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.accept_account_service_transfer("transfer-token")

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST", "/account/service-transfers/transfer-token/accept"
    )
    await client.close()


async def test_accept_account_service_transfer_url_encodes_token() -> None:
    """Service transfer accept URL-encodes the token path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"token": "transf...uery"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.accept_account_service_transfer("transfer/token?query")

    mock_request.assert_called_once_with(
        "POST", "/account/service-transfers/transfer%2Ftoken%3Fquery/accept"
    )
    await client.close()


async def test_accept_account_service_transfer_wraps_http_errors() -> None:
    """Service transfer accept wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.accept_account_service_transfer("transfer-token")

    assert "AcceptAccountServiceTransfer" in str(excinfo.value)
    await client.close()


async def test_retryable_accept_account_service_transfer_delegates_once() -> None:
    """RetryableClient delegates service transfer accept once without retry replay."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "accept_account_service_transfer", new_callable=AsyncMock
    ) as mock_accept:
        mock_accept.side_effect = [httpx.HTTPError("transient")]

        with pytest.raises(httpx.HTTPError):
            await retryable.accept_account_service_transfer("transfer-token")

    mock_accept.assert_awaited_once_with("transfer-token")
    await retryable.close()


async def test_delete_account_service_transfer_sends_exact_route() -> None:
    """Service transfer delete sends DELETE /account/service-transfers/{token}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"token": "***", "message": "canceled"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.delete_account_service_transfer("transfer-token")

    assert result == response_data
    mock_request.assert_called_once_with(
        "DELETE", "/account/service-transfers/transfer-token"
    )
    await client.close()


async def test_delete_account_service_transfer_url_encodes_token() -> None:
    """Service transfer delete URL-encodes the token path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"token": "transf...uery"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.delete_account_service_transfer("transfer/token?query")

    mock_request.assert_called_once_with(
        "DELETE", "/account/service-transfers/transfer%2Ftoken%3Fquery"
    )
    await client.close()


async def test_delete_account_service_transfer_wraps_http_errors() -> None:
    """Service transfer delete wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_account_service_transfer("transfer-token")

    assert "DeleteAccountServiceTransfer" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_account_service_transfer_delegates_once() -> None:
    """RetryableClient delegates service transfer deletion once."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_account_service_transfer", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await retryable.delete_account_service_transfer("transfer-token")

    mock_delete.assert_awaited_once_with("transfer-token")
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


async def test_get_account_oauth_client_thumbnail_sends_exact_route() -> None:
    """OAuth client thumbnail get sends the documented PNG route."""
    client = Client("https://api.linode.com/v4", "test-token")
    thumbnail = b"\x89PNG\r\n\x1a\n"
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.headers = {"Content-Type": "image/png"}
    mock_response.content = thumbnail

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_oauth_client_thumbnail("client-123")

    assert result == {
        "content_type": "image/png",
        "encoding": "base64",
        "data": "iVBORw0KGgo=",
    }
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


async def test_get_account_oauth_client_thumbnail_url_encodes_client_id() -> None:
    """OAuth client thumbnail get URL-encodes the client ID path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.headers = {"Content-Type": "image/png; charset=binary"}
    mock_response.content = b"png"

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_oauth_client_thumbnail("client/id?query")

    assert result["content_type"] == "image/png"
    assert result["data"] == "cG5n"
    await_args = mock_request.await_args
    assert await_args is not None
    assert await_args.args == (
        "GET",
        "https://api.linode.com/v4/account/oauth-clients/client%2Fid%3Fquery/thumbnail",
    )
    await client.close()


async def test_get_account_oauth_client_thumbnail_wraps_http_errors() -> None:
    """OAuth client thumbnail get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="GetAccountOAuthClientThumbnail"):
            await client.get_account_oauth_client_thumbnail("client-123")
    await client.close()


async def test_get_account_oauth_client_thumbnail_maps_http_status_errors() -> None:
    """OAuth client thumbnail get maps non-2xx API responses."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 404
    mock_response.headers = {"Content-Type": "application/json"}
    mock_response.json.return_value = {"errors": [{"reason": "Not found"}]}
    mock_response.content = b'{"errors":[{"reason":"Not found"}]}'

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        with pytest.raises(APIError, match="Not found"):
            await client.get_account_oauth_client_thumbnail("client-123")

    mock_request.assert_awaited_once()
    await client.close()


async def test_retryable_get_account_oauth_client_thumbnail_delegates() -> None:
    """Retryable OAuth client thumbnail get delegates to the client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    response_data = {
        "content_type": "image/png",
        "encoding": "base64",
        "data": "iVBORw0KGgo=",
    }

    with patch.object(
        retryable.client, "get_account_oauth_client_thumbnail", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = response_data

        result = await retryable.get_account_oauth_client_thumbnail("client-123")

    assert result == response_data
    mock_get.assert_awaited_once_with("client-123")
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


async def test_reset_account_oauth_client_secret_sends_exact_route() -> None:
    """OAuth client secret reset sends the exact POST route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "client-123", "secret": "shown-once"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.reset_account_oauth_client_secret("client-123")

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST", "/account/oauth-clients/client-123/reset-secret"
    )
    await client.close()


async def test_reset_account_oauth_client_secret_url_encodes_client_id() -> None:
    """OAuth client secret reset URL-encodes the client_id path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "client/id?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.reset_account_oauth_client_secret("client/id?query")

    mock_request.assert_called_once_with(
        "POST", "/account/oauth-clients/client%2Fid%3Fquery/reset-secret"
    )
    await client.close()


async def test_reset_account_oauth_client_secret_wraps_http_errors() -> None:
    """OAuth client secret reset wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.reset_account_oauth_client_secret("client-123")

    assert "ResetAccountOAuthClientSecret" in str(excinfo.value)
    await client.close()


async def test_retryable_reset_account_oauth_client_secret_does_not_replay() -> None:
    """RetryableClient delegates OAuth client secret reset once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "reset_account_oauth_client_secret", new_callable=AsyncMock
    ) as mock_reset:
        mock_reset.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.reset_account_oauth_client_secret("client-123")

    mock_reset.assert_awaited_once_with("client-123")
    await retryable.close()


async def test_list_account_availability_sends_exact_route_with_query() -> None:
    """Account availability listing sends GET /account/availability."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"service": "Linodes", "available": True}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_availability(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/availability?page=2&page_size=25"
    )
    await client.close()


async def test_get_account_availability_sends_exact_route() -> None:
    """Account availability get sends GET /account/availability/{regionId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"available": ["Linodes", "NodeBalancers"]}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_availability("us-east")

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/availability/us-east")
    await client.close()


async def test_get_account_availability_url_encodes_region_id() -> None:
    """Account availability get URL-encodes path parameters."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"available": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_availability("us/east")

    mock_request.assert_called_once_with("GET", "/account/availability/us%2Feast")
    await client.close()


async def test_get_account_availability_wraps_http_errors() -> None:
    """Account availability get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_availability("us-east")

    assert "GetAccountAvailability" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_availability_delegates_to_client() -> None:
    """RetryableClient delegates account availability get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_availability", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"available": []}
        result = await retryable.get_account_availability("us-east")

    assert result["available"] == []
    mock_get.assert_awaited_once_with("us-east")
    await retryable.close()


async def test_list_account_availability_wraps_http_errors() -> None:
    """Account availability listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_availability()

    assert "ListAccountAvailability" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_availability_delegates_to_client() -> None:
    """RetryableClient delegates account availability listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_availability", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": []}
        result = await retryable.list_account_availability(page=2, page_size=25)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=2, page_size=25)
    await retryable.close()


async def test_acknowledge_account_agreements_sends_post_body() -> None:
    """Test acknowledging account agreements sends POST /account/agreements."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {
        "billing_agreement": True,
        "eu_model": True,
        "master_service_agreement": False,
        "privacy_policy": True,
    }
    response_data = {"accepted": True}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.acknowledge_account_agreements(payload)

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/agreements", payload)
    await client.close()


async def test_acknowledge_account_agreements_wraps_http_errors() -> None:
    """Test acknowledging account agreements wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.acknowledge_account_agreements({"eu_model": True})

    assert "AcknowledgeAccountAgreements" in str(excinfo.value)
    await client.close()


async def test_retryable_acknowledge_account_agreements_does_not_replay() -> None:
    """RetryableClient delegates agreement acknowledgement once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    payload = {"eu_model": True}

    with patch.object(
        retryable.client, "acknowledge_account_agreements", new_callable=AsyncMock
    ) as mock_acknowledge:
        mock_acknowledge.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.acknowledge_account_agreements(payload)

    mock_acknowledge.assert_awaited_once_with(payload)
    await retryable.close()


async def test_create_account_oauth_client_sends_post_body() -> None:
    """OAuth client creation sends POST /account/oauth-clients."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {"label": "demo-client", "redirect_uri": "https://example.com/cb"}
    response_data = {
        "id": "client-123",
        "label": "demo-client",
        "redirect_uri": "https://example.com/cb",
        "secret": "shown-once",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_account_oauth_client(
            "demo-client", "https://example.com/cb"
        )

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/oauth-clients", payload)
    await client.close()


async def test_create_account_payment_method_sends_post_body() -> None:
    """Payment method creation sends POST /account/payment-methods."""
    client = Client("https://api.linode.com/v4", "test-token")
    provider_data = {"nonce": "payment-token"}
    payload = {"type": "credit_card", "data": provider_data, "is_default": True}
    response_data = {
        "id": 123,
        "type": "credit_card",
        "is_default": True,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_account_payment_method(
            "credit_card", {"nonce": "payment-token"}, True
        )

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/payment-methods", payload)
    await client.close()


async def test_create_account_payment_sends_post_body() -> None:
    """Payment creation sends POST /account/payments."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {"payment_method_id": 123, "usd": "25.00"}
    response_data = {"id": 456, "payment_method_id": 123, "usd": "25.00"}
    mock_response = MagicMock()
    mock_response.status_code = 202
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_account_payment(123, "25.00")

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/payments", payload)
    await client.close()


async def test_add_account_promo_credit_sends_post_body() -> None:
    """Promo credit creation sends POST /account/promo-codes."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {"promo_code": "PROMO123"}
    response_data = {
        "description": "Promo credit",
        "summary": "$100 credit",
        "credit_remaining": "100.00",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.add_account_promo_credit("PROMO123")

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/promo-codes", payload)
    await client.close()


async def test_create_account_service_transfer_sends_post_body() -> None:
    """Service transfer creation sends POST /account/service-transfers."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {"entities": {"linodes": [123, 456]}}
    response_data = {
        "token": "service-transfer-token",
        "status": "pending",
        "entities": {"linodes": [123, 456]},
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_account_service_transfer([123, 456])

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/account/service-transfers", payload)
    await client.close()


async def test_delete_account_oauth_client_sends_delete_to_encoded_route() -> None:
    """OAuth client deletion sends DELETE with an encoded client ID path."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "client/123", "deleted": True}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.delete_account_oauth_client("client/123")

    assert result == response_data
    mock_request.assert_called_once_with(
        "DELETE", "/account/oauth-clients/client%2F123"
    )
    await client.close()


async def test_create_account_oauth_client_wraps_http_errors() -> None:
    """OAuth client creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_account_oauth_client(
                "demo-client", "https://example.com/cb"
            )

    assert "CreateAccountOAuthClient" in str(excinfo.value)
    await client.close()


async def test_create_account_payment_method_wraps_http_errors() -> None:
    """Payment method creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_account_payment_method(
                "credit_card", {"nonce": "payment-token"}, True
            )

    assert "CreateAccountPaymentMethod" in str(excinfo.value)
    await client.close()


async def test_create_account_payment_wraps_http_errors() -> None:
    """Payment creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_account_payment(123, "25.00")

    assert "CreateAccountPayment" in str(excinfo.value)
    await client.close()


async def test_add_account_promo_credit_wraps_http_errors() -> None:
    """Promo credit creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.add_account_promo_credit("PROMO123")

    assert "AddAccountPromoCredit" in str(excinfo.value)
    await client.close()


async def test_create_account_service_transfer_wraps_http_errors() -> None:
    """Service transfer creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_account_service_transfer([123])

    assert "CreateAccountServiceTransfer" in str(excinfo.value)
    await client.close()


async def test_delete_account_oauth_client_wraps_http_errors() -> None:
    """OAuth client deletion wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_account_oauth_client("client-123")

    assert "DeleteAccountOAuthClient" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_account_oauth_client_does_not_replay() -> None:
    """RetryableClient delegates OAuth client deletion once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_account_oauth_client", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.delete_account_oauth_client("client-123")

    mock_delete.assert_awaited_once_with("client-123")
    await retryable.close()


async def test_retryable_create_account_payment_method_does_not_replay() -> None:
    """RetryableClient delegates payment method creation once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_account_payment_method", new_callable=AsyncMock
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.create_account_payment_method(
                "credit_card", {"nonce": "payment-token"}, True
            )

    mock_create.assert_awaited_once_with(
        "credit_card", {"nonce": "payment-token"}, True
    )
    await retryable.close()


async def test_retryable_create_account_payment_does_not_replay() -> None:
    """RetryableClient delegates payment creation once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_account_payment", new_callable=AsyncMock
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.create_account_payment(123, "25.00")

    mock_create.assert_awaited_once_with(123, "25.00")
    await retryable.close()


async def test_retryable_add_account_promo_credit_does_not_replay() -> None:
    """RetryableClient delegates promo credit creation once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "add_account_promo_credit", new_callable=AsyncMock
    ) as mock_add:
        mock_add.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.add_account_promo_credit("PROMO123")

    mock_add.assert_awaited_once_with("PROMO123")
    await retryable.close()


async def test_retryable_create_account_service_transfer_does_not_replay() -> None:
    """RetryableClient delegates service transfer creation once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_account_service_transfer", new_callable=AsyncMock
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.create_account_service_transfer([123])

    mock_create.assert_awaited_once_with([123])
    await retryable.close()


async def test_retryable_create_account_oauth_client_does_not_replay() -> None:
    """RetryableClient delegates OAuth client creation once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_account_oauth_client", new_callable=AsyncMock
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.create_account_oauth_client(
                "demo-client", "https://example.com/cb"
            )

    mock_create.assert_awaited_once_with("demo-client", "https://example.com/cb")
    await retryable.close()


async def test_enroll_account_beta_sends_post_body() -> None:
    """Test beta enrollment sends POST /account/betas."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "distributed-beta", "label": "Distributed Beta"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.enroll_account_beta("distributed-beta")

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST", "/account/betas", {"id": "distributed-beta"}
    )
    await client.close()


async def test_enroll_account_beta_wraps_http_errors() -> None:
    """Test beta enrollment wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.enroll_account_beta("distributed-beta")

    assert "EnrollAccountBeta" in str(excinfo.value)
    await client.close()


async def test_retryable_enroll_account_beta_does_not_replay() -> None:
    """RetryableClient delegates beta enrollment once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "enroll_account_beta", new_callable=AsyncMock
    ) as mock_enroll:
        mock_enroll.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.enroll_account_beta("distributed-beta")

    mock_enroll.assert_awaited_once_with("distributed-beta")
    await retryable.close()


async def test_update_account_settings_sends_put_to_settings_route() -> None:
    """Account settings update sends PUT /account/settings."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "backups_enabled": True,
        "interfaces_for_new_linodes": "linode_default",
        "longview_subscription": "longview-10",
        "maintenance_policy": "linode/migrate",
        "managed": False,
        "network_helper": True,
        "object_storage": "active",
    }

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_account_settings(
            backups_enabled=True,
            interfaces_for_new_linodes="linode_default",
            longview_subscription="longview-10",
            maintenance_policy="linode/migrate",
            managed=False,
            network_helper=True,
            object_storage="active",
            ignored=None,
        )

    assert result == response_data
    mock_request.assert_awaited_once_with("PUT", "/account/settings", response_data)
    await client.close()


async def test_update_account_settings_rejects_empty_body() -> None:
    """Account settings update requires at least one body field."""
    client = Client("https://api.linode.com/v4", "test-token")

    with pytest.raises(ValueError, match="At least one account settings field"):
        await client.update_account_settings()

    await client.close()


async def test_update_account_settings_wraps_http_errors() -> None:
    """Account settings update wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="UpdateAccountSettings"):
            await client.update_account_settings(network_helper=False)

    await client.close()


async def test_retryable_update_account_settings_does_not_replay_put() -> None:
    """RetryableClient delegates account settings update once."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_account_settings", new_callable=AsyncMock
    ) as mock_update:
        mock_update.side_effect = httpx.HTTPError("transient")

        with pytest.raises(httpx.HTTPError, match="transient"):
            await retryable.update_account_settings(network_helper=False)

    mock_update.assert_awaited_once_with(network_helper=False)
    await retryable.close()


async def test_update_account_sends_put_to_account_route() -> None:
    """Test updating account sends PUT /account."""
    client = Client("https://api.linode.com/v4", "test-token")

    updated_account = {
        "first_name": "Test",
        "last_name": "User",
        "email": "updated@example.com",
        "company": "Updated Co",
        "address_1": "123 Test St",
        "address_2": "",
        "city": "Test City",
        "state": "TS",
        "zip": "12345",
        "country": "US",
        "phone": "555-1234",
        "balance": 100.50,
        "balance_uninvoiced": 50.25,
        "capabilities": ["Linodes", "Block Storage"],
        "active_since": "2020-01-01T00:00:00",
        "euuid": "abcd-1234",
        "billing_source": "linode",
        "active_promotions": [],
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = updated_account

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        account = await client.update_account(
            email="updated@example.com",
            company="Updated Co",
            phone=None,
        )

    assert isinstance(account, Account)
    assert account.email == "updated@example.com"
    mock_request.assert_called_once_with(
        "PUT",
        "/account",
        {"email": "updated@example.com", "company": "Updated Co"},
    )

    await client.close()


async def test_cancel_account_sends_post_to_account_cancel_route() -> None:
    """Canceling an account sends POST /account/cancel with comments."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"survey_link": "https://example.com/survey"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.cancel_account(comments="No longer needed")

    assert result == {"survey_link": "https://example.com/survey"}
    mock_request.assert_called_once_with(
        "POST", "/account/cancel", {"comments": "No longer needed"}
    )

    await client.close()


async def test_cancel_account_without_comments_sends_empty_body() -> None:
    """Canceling an account without comments sends an empty JSON body."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.cancel_account()

    assert result == {}
    mock_request.assert_called_once_with("POST", "/account/cancel", {})

    await client.close()


async def test_cancel_account_wraps_http_errors() -> None:
    """Canceling account wraps HTTP transport errors with route context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.cancel_account(comments="cancel")

    assert exc_info.value.operation == "CancelAccount"

    await client.close()


async def test_retryable_cancel_account_does_not_retry_transient_errors() -> None:
    """Account cancellation is delegated once to avoid replaying side effects."""
    client = AsyncMock()
    client.cancel_account.side_effect = httpx.HTTPError("temporary failure")
    retryable = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        retry_config=RetryConfig(max_retries=3),
    )
    retryable.client = client

    with pytest.raises(httpx.HTTPError):
        await retryable.cancel_account(comments="cancel")

    client.cancel_account.assert_awaited_once_with(comments="cancel")


async def test_update_instance_ip_sends_put_to_instance_ip_route() -> None:
    """Updating instance IP RDNS sends PUT to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "address": "203.0.113.1",
        "rdns": "host.example.com",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_instance_ip(
            123,
            "203.0.113.1",
            "host.example.com",
        )

    assert result["rdns"] == "host.example.com"
    mock_request.assert_called_once_with(
        "PUT",
        "/linode/instances/123/ips/203.0.113.1",
        {"rdns": "host.example.com"},
    )

    await client.close()


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
    assert call_args[0][1] == "/linode/instances/123/ips/2001%3Adb8%3A%3A1"

    await client.close()


async def test_update_instance_ip_url_encodes_address() -> None:
    """Instance IP update URL-encodes the address path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"address": "2001:db8::1", "rdns": None}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_instance_ip(123, "2001:db8::1", None)

    call_args = mock_request.call_args
    assert call_args[0][1] == "/linode/instances/123/ips/2001%3Adb8%3A%3A1"

    await client.close()


async def test_delete_instance_ip_url_encodes_address() -> None:
    """Instance IP delete URL-encodes the address path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_instance_ip(123, "2001:db8::1")

    mock_request.assert_awaited_once_with(
        "DELETE", "/linode/instances/123/ips/2001%3Adb8%3A%3A1"
    )

    await client.close()


async def test_list_instances(sample_instance_data: dict[str, Any]) -> None:
    """Test listing instances."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [sample_instance_data],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        instances = await client.list_instances()

        assert len(instances) == 1
        assert instances[0].id == 123456
        assert instances[0].label == "test-instance"
        assert instances[0].status == "running"

    await client.close()


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


async def test_clone_domain_sends_exact_route_and_body() -> None:
    """Domain clone sends POST /domains/{domainId}/clone."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {
        "id": 23456,
        "domain": "clone.example.com",
        "type": "master",
        "status": "active",
        "soa_email": "admin@example.com",
        "description": "",
        "tags": [],
        "created": "2024-01-15T10:00:00",
        "updated": "2024-01-15T10:00:00",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        domain = await client.clone_domain("123/456", "clone.example.com")

    assert domain.id == 23456
    assert domain.domain == "clone.example.com"
    mock_request.assert_called_once_with(
        "POST", "/domains/123%2F456/clone", {"domain": "clone.example.com"}
    )
    await client.close()


async def test_clone_domain_wraps_http_errors() -> None:
    """Domain clone wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.clone_domain(12345, "clone.example.com")

    assert "CloneDomain" in str(excinfo.value)
    await client.close()


async def test_retryable_clone_domain_delegates_once_without_retry() -> None:
    """RetryableClient does not replay domain clone on errors."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "clone_domain", new_callable=AsyncMock
    ) as mock_clone:
        mock_clone.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await retryable.clone_domain(12345, "clone.example.com")

    mock_clone.assert_awaited_once_with(12345, "clone.example.com")
    await retryable.close()


async def test_list_account_betas_sends_get_to_account_betas_route() -> None:
    """Test listing account betas sends GET /account/betas."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"id": "VPC", "label": "VPC Beta", "started": "2024-01-01T00:00:00"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_betas(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/betas?page=2&page_size=25")
    await client.close()


async def test_list_betas_sends_get_to_betas_route() -> None:
    """Test listing available betas sends GET /betas."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"id": "VPC", "label": "VPC Beta", "started": "2024-01-01T00:00:00"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_betas(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/betas?page=2&page_size=25")
    await client.close()


async def test_list_account_betas_wraps_http_errors() -> None:
    """Test listing account betas wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_betas()

    assert "ListAccountBetas" in str(excinfo.value)
    await client.close()


async def test_list_betas_wraps_http_errors() -> None:
    """Test listing available betas wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_betas()

    assert "ListBetas" in str(excinfo.value)
    await client.close()


async def test_list_mysql_database_instances_uses_mysql_route() -> None:
    """Test listing MySQL Managed Databases sends GET /databases/mysql/instances."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"id": 123, "label": "primary-db", "type": "g6-dedicated-2"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_mysql_database_instances(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/databases/mysql/instances?page=2&page_size=25"
    )
    await client.close()


async def test_list_database_instances_sends_get_to_databases_instances_route() -> None:
    """Test listing Managed Databases sends GET /databases/instances."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"id": 123, "label": "primary-db", "type": "g6-dedicated-2"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_database_instances(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/databases/instances?page=2&page_size=25"
    )
    await client.close()


async def test_list_database_instances_wraps_http_errors() -> None:
    """Test listing Managed Databases wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_database_instances()

    assert "ListDatabaseInstances" in str(excinfo.value)
    await client.close()


async def test_list_mysql_database_instances_wraps_http_errors() -> None:
    """Test listing MySQL Managed Databases wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_mysql_database_instances()

    assert "ListMysqlDatabaseInstances" in str(excinfo.value)
    await client.close()


async def test_retryable_list_account_betas_delegates_to_client() -> None:
    """Test RetryableClient delegates account beta listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_betas", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_betas(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_retryable_list_betas_delegates_to_client() -> None:
    """Test RetryableClient delegates available beta listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_betas", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_betas(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_retryable_list_mysql_database_instances_delegates_to_client() -> None:
    """Test RetryableClient delegates MySQL Managed Database listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_mysql_database_instances", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_mysql_database_instances(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_retryable_list_database_instances_delegates_to_client() -> None:
    """Test RetryableClient delegates Managed Database listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_database_instances", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_database_instances(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_create_mysql_database_instance_sends_post_body() -> None:
    """Creating a MySQL database sends POST /databases/mysql/instances."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload: dict[str, Any] = {
        "label": "primary-db",
        "type": "g6-dedicated-2",
        "engine": "mysql/8.0",
        "region": "us-east",
        "allow_list": ["192.0.2.1/32"],
        "cluster_size": 3,
        "engine_config": {"binlog_retention_period": 600},
        "fork": {"source": 123},
        "private_network": "vpc-1",
        "ssl_connection": True,
    }
    response_data = {"id": 123, "label": "primary-db"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_mysql_database_instance(payload)

    assert result == response_data
    mock_request.assert_awaited_once_with("POST", "/databases/mysql/instances", payload)
    await client.close()


async def test_create_mysql_database_instance_wraps_http_errors() -> None:
    """Creating a MySQL database maps HTTP errors to NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_mysql_database_instance({"label": "primary-db"})

    assert "CreateMysqlDatabaseInstance" in str(excinfo.value)
    await client.close()


async def test_reset_mysql_database_credentials_sends_encoded_post() -> None:
    """Resetting MySQL database credentials sends POST with encoded instance ID."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "username": "linode", "password": "secret"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.reset_mysql_database_credentials("123/456")

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "POST", "/databases/mysql/instances/123%2F456/credentials/reset"
    )
    await client.close()


async def test_reset_mysql_database_credentials_wraps_http_errors() -> None:
    """Resetting MySQL database credentials maps HTTP errors to NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.reset_mysql_database_credentials(123)

    assert "ResetMysqlDatabaseCredentials" in str(excinfo.value)
    await client.close()


async def test_delete_mysql_database_instance_sends_encoded_delete() -> None:
    """Deleting a MySQL database sends DELETE with an encoded instance ID."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "label": "primary-db"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.delete_mysql_database_instance("123/456")

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "DELETE", "/databases/mysql/instances/123%2F456"
    )
    await client.close()


async def test_delete_mysql_database_instance_wraps_http_errors() -> None:
    """Deleting a MySQL database maps HTTP errors to NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_mysql_database_instance(123)

    assert "DeleteMysqlDatabaseInstance" in str(excinfo.value)
    await client.close()


async def test_patch_mysql_database_instance_sends_encoded_post() -> None:
    """Patching a MySQL database sends POST with an encoded instance ID."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "label": "primary-db"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.patch_mysql_database_instance("123/456")

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "POST", "/databases/mysql/instances/123%2F456/patch"
    )
    await client.close()


async def test_patch_mysql_database_instance_wraps_http_errors() -> None:
    """Patching a MySQL database maps HTTP errors to NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.patch_mysql_database_instance(123)

    assert "PatchMysqlDatabaseInstance" in str(excinfo.value)
    await client.close()


async def test_suspend_postgresql_database_instance_sends_encoded_post() -> None:
    """Low-level client sends the documented PostgreSQL suspend route."""
    response_data = {"id": 123, "label": "primary-db"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data
    client = Client("https://api.linode.test/v4", "token")
    with patch.object(
        client, "make_request", AsyncMock(return_value=mock_response)
    ) as make_request:
        result = await client.suspend_postgresql_database_instance("123/456?")

    assert result == response_data
    make_request.assert_awaited_once_with(
        "POST", "/databases/postgresql/instances/123%2F456%3F/suspend"
    )
    await client.close()


async def test_suspend_postgresql_database_instance_wraps_http_errors() -> None:
    """Low-level client maps PostgreSQL suspend HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")
    with (
        patch.object(
            client, "make_request", AsyncMock(side_effect=httpx.ConnectError("boom"))
        ),
        pytest.raises(NetworkError, match="SuspendPostgreSQLDatabaseInstance"),
    ):
        await client.suspend_postgresql_database_instance(123)
    await client.close()


async def test_retryable_suspend_postgresql_database_instance_delegates_once() -> None:
    """Retryable PostgreSQL suspend delegates once to avoid replaying side effects."""
    retryable = RetryableClient("https://api.linode.test/v4", "token")
    with patch.object(
        retryable.client, "suspend_postgresql_database_instance", new_callable=AsyncMock
    ) as suspend:
        suspend.side_effect = NetworkError(
            "SuspendPostgreSQLDatabaseInstance", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.suspend_postgresql_database_instance(123)

    suspend.assert_awaited_once_with(123)
    await retryable.close()


async def test_retryable_create_mysql_database_instance_delegates_once() -> None:
    """Retryable create delegates once and does not replay POST failures."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    payload = {
        "label": "primary-db",
        "type": "g6-dedicated-2",
        "engine": "mysql/8.0",
        "region": "us-east",
    }

    with patch.object(
        retryable.client, "create_mysql_database_instance", new_callable=AsyncMock
    ) as mock_create:
        mock_create.side_effect = NetworkError(
            "CreateMysqlDatabaseInstance", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.create_mysql_database_instance(payload)

    mock_create.assert_awaited_once_with(payload)
    await retryable.close()


async def test_retryable_reset_mysql_database_credentials_delegates_once() -> None:
    """Retryable credential reset delegates once and does not replay POST failures."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "reset_mysql_database_credentials", new_callable=AsyncMock
    ) as mock_reset:
        mock_reset.side_effect = NetworkError(
            "ResetMysqlDatabaseCredentials", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.reset_mysql_database_credentials(123)

    mock_reset.assert_awaited_once_with(123)
    await retryable.close()


async def test_retryable_delete_mysql_database_instance_delegates_once() -> None:
    """Retryable delete delegates once and does not replay DELETE failures."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_mysql_database_instance", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = NetworkError(
            "DeleteMysqlDatabaseInstance", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.delete_mysql_database_instance(123)

    mock_delete.assert_awaited_once_with(123)
    await retryable.close()


async def test_retryable_patch_mysql_database_instance_delegates_once() -> None:
    """Retryable patch delegates once and does not replay POST failures."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "patch_mysql_database_instance", new_callable=AsyncMock
    ) as mock_patch:
        mock_patch.side_effect = NetworkError(
            "PatchMysqlDatabaseInstance", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.patch_mysql_database_instance(123)

    mock_patch.assert_awaited_once_with(123)
    await retryable.close()


async def test_resume_mysql_database_instance_sends_encoded_post() -> None:
    """Resuming a MySQL database sends POST with an encoded instance ID."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "label": "primary-db"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.resume_mysql_database_instance("123/456?")

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "POST", "/databases/mysql/instances/123%2F456%3F/resume"
    )
    await client.close()


async def test_resume_mysql_database_instance_wraps_http_errors() -> None:
    """Resuming a MySQL database maps HTTP errors to NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.resume_mysql_database_instance(123)

    assert "ResumeMysqlDatabaseInstance" in str(excinfo.value)
    await client.close()


async def test_retryable_resume_mysql_database_instance_delegates_once() -> None:
    """Retryable resume delegates once and does not replay POST failures."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "resume_mysql_database_instance", new_callable=AsyncMock
    ) as mock_resume:
        mock_resume.side_effect = NetworkError(
            "ResumeMysqlDatabaseInstance", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.resume_mysql_database_instance(123)

    mock_resume.assert_awaited_once_with(123)
    await retryable.close()


async def test_resume_postgresql_database_instance_sends_encoded_post() -> None:
    """Resuming a PostgreSQL database sends POST with an encoded instance ID."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "label": "primary-db"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.resume_postgresql_database_instance("123/456?")

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "POST", "/databases/postgresql/instances/123%2F456%3F/resume"
    )
    await client.close()


async def test_resume_postgresql_database_instance_wraps_http_errors() -> None:
    """Resuming a PostgreSQL database maps HTTP errors to NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.resume_postgresql_database_instance(123)

    assert "ResumePostgreSQLDatabaseInstance" in str(excinfo.value)
    await client.close()


async def test_retryable_resume_postgresql_database_instance_delegates_once() -> None:
    """Retryable PostgreSQL resume delegates once and does not replay failures."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "resume_postgresql_database_instance", new_callable=AsyncMock
    ) as mock_resume:
        mock_resume.side_effect = NetworkError(
            "ResumePostgreSQLDatabaseInstance", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.resume_postgresql_database_instance(123)

    mock_resume.assert_awaited_once_with(123)
    await retryable.close()


async def test_list_account_child_accounts_sends_get_to_child_accounts_route() -> None:
    """Test listing child accounts sends GET /account/child-accounts."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [
            {
                "euuid": "A1BC2DEF-34GH-567I-J890KLMN12O34P56",
                "company": "Example Child",
            }
        ],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_child_accounts(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/child-accounts?page=2&page_size=25"
    )
    await client.close()


async def test_list_account_child_accounts_wraps_http_errors() -> None:
    """Test listing child accounts wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_account_child_accounts()

    assert "ListAccountChildAccounts" in str(excinfo.value)
    await client.close()


async def test_create_account_child_account_token_sends_post_to_encoded_route() -> None:
    """Create proxy token sends POST to encoded child-account route."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {"token": "proxy-token", "expiry": "2026-06-02T00:00:00"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_account_child_account_token("child/account")

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST", "/account/child-accounts/child%2Faccount/token"
    )
    await client.close()


async def test_create_account_child_account_token_wraps_http_errors() -> None:
    """Creating a child-account proxy token wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_account_child_account_token("child-123")

    assert "CreateAccountChildAccountToken" in str(excinfo.value)
    await client.close()


async def test_retryable_create_account_child_account_token_delegates_once() -> None:
    """Retryable wrapper must not replay child-account token creation."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client,
        "create_account_child_account_token",
        new_callable=AsyncMock,
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("transient")

        with pytest.raises(httpx.HTTPError):
            await retryable.create_account_child_account_token("child-123")

    mock_create.assert_awaited_once_with("child-123")
    await retryable.close()


async def test_retryable_list_account_child_accounts_delegates_to_client() -> None:
    """Test RetryableClient delegates child account listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_child_accounts", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_child_accounts(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_list_account_service_transfers_sends_get_route() -> None:
    """Test listing account service transfers sends GET /account/service-transfers."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"token": "service-token-example", "status": "pending"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_account_service_transfers(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/account/service-transfers?page=2&page_size=25"
    )
    await client.close()


async def test_list_account_service_transfers_wraps_http_errors() -> None:
    """Test listing account service transfers wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("temporary failure")

        with pytest.raises(NetworkError) as exc_info:
            await client.list_account_service_transfers()

    assert "ListAccountServiceTransfers" in str(exc_info.value)
    await client.close()


async def test_retryable_list_account_service_transfers_delegates_to_client() -> None:
    """Test RetryableClient delegates account service transfer listing."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_account_service_transfers", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_account_service_transfers(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_get_account_invoice_sends_exact_route() -> None:
    """Account invoice get sends GET /account/invoices/{invoiceId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 123, "label": "Invoice 123", "total": 42.5}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_invoice(123)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/invoices/123")
    await client.close()


async def test_get_account_invoice_url_encodes_invoice_id() -> None:
    """Account invoice get URL-encodes the invoice_id path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "12/3?x"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_invoice(cast("int", "12/3?x"))

    mock_request.assert_called_once_with("GET", "/account/invoices/12%2F3%3Fx")
    await client.close()


async def test_get_account_invoice_wraps_http_errors() -> None:
    """Account invoice get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_invoice(123)

    assert "GetAccountInvoice" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_invoice_delegates_to_client() -> None:
    """RetryableClient delegates account invoice get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_invoice", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 123}
        result = await retryable.get_account_invoice(123)

    mock_get.assert_awaited_once_with(123)
    assert result == {"id": 123}
    await retryable.close()


async def test_get_account_login_sends_exact_route() -> None:
    """Account login get sends GET /account/logins/{loginId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": 456, "username": "alice", "status": "successful"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_account_login(456)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/account/logins/456")
    await client.close()


async def test_get_account_login_url_encodes_login_id() -> None:
    """Account login get URL-encodes the login_id path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": "45/6?x"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_account_login(cast("int", "45/6?x"))

    mock_request.assert_called_once_with("GET", "/account/logins/45%2F6%3Fx")
    await client.close()


async def test_get_account_login_wraps_http_errors() -> None:
    """Account login get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_login(456)

    assert "GetAccountLogin" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_login_delegates_to_client() -> None:
    """RetryableClient delegates account login get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_account_login", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 456}
        result = await retryable.get_account_login(456)

    mock_get.assert_awaited_once_with(456)
    assert result == {"id": 456}
    await retryable.close()


async def test_list_tags_sends_get_to_tags_route() -> None:
    """Test listing account tags sends GET /tags."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"label": "production"}, {"label": "web"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_tags(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/tags?page=2&page_size=25")
    await client.close()


async def test_list_tags_wraps_http_errors() -> None:
    """Test listing account tags wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_tags()

    assert "ListTags" in str(excinfo.value)
    await client.close()


async def test_retryable_list_tags_delegates_to_client() -> None:
    """Test RetryableClient delegates account tag listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_tags", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_tags(page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


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


async def test_create_tag_sends_post_to_tags_route() -> None:
    """Test creating a tag sends POST /tags with documented body fields."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {"label": "production"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_tag(
            "production",
            domains=[1],
            linodes=[2],
            nodebalancers=[3],
            volumes=[4],
        )

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST",
        "/tags",
        {
            "label": "production",
            "domains": [1],
            "linodes": [2],
            "nodebalancers": [3],
            "volumes": [4],
        },
    )
    await client.close()


async def test_create_tag_wraps_http_errors() -> None:
    """Test creating a tag wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_tag("production")

    assert "CreateTag" in str(excinfo.value)
    await client.close()


async def test_retryable_create_tag_delegates_to_client() -> None:
    """Test RetryableClient delegates tag creation to Client.create_tag."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_tag", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"label": "production"}
        result = await retryable.create_tag("production", linodes=[123])

    assert result == {"label": "production"}
    mock_create.assert_awaited_once_with(
        "production",
        domains=None,
        linodes=[123],
        nodebalancers=None,
        volumes=None,
    )
    await retryable.close()


async def test_create_support_ticket_sends_post_to_tickets_route() -> None:
    """Test support ticket creation sends documented POST body."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {"id": 789, "summary": "Need help"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_support_ticket(
            "Need help",
            "Details",
            linode_id=123,
            managed_issue=False,
            severity=2,
        )

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST",
        "/support/tickets",
        {
            "summary": "Need help",
            "description": "Details",
            "linode_id": 123,
            "managed_issue": False,
            "severity": 2,
        },
    )
    await client.close()


async def test_create_support_ticket_wraps_http_errors() -> None:
    """Test support ticket creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_support_ticket("Need help", "Details")

    assert "CreateSupportTicket" in str(excinfo.value)
    await client.close()


async def test_retryable_create_support_ticket_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket creation."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_support_ticket", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"id": 789}
        result = await retryable.create_support_ticket(
            "Need help", "Details", severity=2
        )

    assert result == {"id": 789}
    mock_create.assert_awaited_once_with(
        "Need help",
        "Details",
        bucket=None,
        database_id=None,
        domain_id=None,
        firewall_id=None,
        linode_id=None,
        lkecluster_id=None,
        longviewclient_id=None,
        managed_issue=None,
        nodebalancer_id=None,
        region=None,
        severity=2,
        vlan=None,
        volume_id=None,
        vpc_id=None,
    )
    await retryable.close()


async def test_get_managed_stats_sends_get_to_managed_stats_route() -> None:
    """Test Managed stats sends documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": {
            "cpu": [{"x": 11513761600000, "y": 29.94}],
            "disk": [{"x": 11513761600000, "y": 12.5}],
            "net_in": [{"x": 11513761600000, "y": 2.0}],
            "net_out": [{"x": 11513761600000, "y": 3.0}],
        }
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_managed_stats()

    assert result == response_data
    mock_request.assert_called_once_with("GET", "/managed/stats")
    await client.close()


async def test_get_managed_stats_wraps_http_errors() -> None:
    """Test Managed stats wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_managed_stats()

    assert "GetManagedStats" in str(excinfo.value)
    await client.close()


async def test_retryable_get_managed_stats_delegates_to_client() -> None:
    """Test RetryableClient delegates Managed stats retrieval."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "get_managed_stats", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"data": {"cpu": []}}
        result = await retryable.get_managed_stats()

    assert result == {"data": {"cpu": []}}
    mock_get.assert_awaited_once_with()
    await retryable.close()


async def test_list_support_tickets_sends_get_to_tickets_route() -> None:
    """Test support ticket listing sends documented GET query."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"id": 789, "summary": "Need help"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_support_tickets(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET",
        "/support/tickets?page=2&page_size=25",
    )
    await client.close()


async def test_list_support_tickets_wraps_http_errors() -> None:
    """Test support ticket listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_support_tickets()

    assert "ListSupportTickets" in str(excinfo.value)
    await client.close()


async def test_retryable_list_support_tickets_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket listing."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_support_tickets", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [{"id": 789}]}
        result = await retryable.list_support_tickets(page=2, page_size=25)

    assert result == {"data": [{"id": 789}]}
    mock_list.assert_awaited_once_with(page=2, page_size=25)
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


async def test_list_support_ticket_replies_sends_get_to_ticket_replies_route() -> None:
    """Test support ticket reply listing sends documented GET query."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {
        "data": [{"id": 456, "description": "Thanks"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_support_ticket_replies(123, page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET",
        "/support/tickets/123/replies?page=2&page_size=25",
    )
    await client.close()


async def test_list_support_ticket_replies_wraps_http_errors() -> None:
    """Test support ticket reply listing wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_support_ticket_replies(123)

    assert "ListSupportTicketReplies" in str(excinfo.value)
    await client.close()


async def test_retryable_list_support_ticket_replies_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket reply listing."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_support_ticket_replies", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [{"id": 456}]}
        result = await retryable.list_support_ticket_replies(123, page=2, page_size=25)

    assert result == {"data": [{"id": 456}]}
    mock_list.assert_awaited_once_with(123, page=2, page_size=25)
    await retryable.close()


async def test_create_support_ticket_reply_sends_post_to_ticket_replies_route() -> None:
    """Test support ticket reply creation sends documented POST body."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {"id": 456, "description": "Thanks"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_support_ticket_reply(123, "Thanks")

    assert result == response_data
    mock_request.assert_called_once_with(
        "POST",
        "/support/tickets/123/replies",
        {"description": "Thanks"},
    )
    await client.close()


async def test_create_support_ticket_reply_wraps_http_errors() -> None:
    """Test support ticket reply creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_support_ticket_reply(123, "Thanks")

    assert "CreateSupportTicketReply" in str(excinfo.value)
    await client.close()


async def test_retryable_create_support_ticket_reply_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket reply creation."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_support_ticket_reply", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"id": 456}
        result = await retryable.create_support_ticket_reply(123, "Thanks")

    assert result == {"id": 456}
    mock_create.assert_awaited_once_with(123, "Thanks")
    await retryable.close()


async def test_create_support_ticket_attachment_route(tmp_path: Any) -> None:
    """Test support ticket attachment creation sends multipart file upload."""
    client = Client("https://api.linode.com/v4", "test-token")
    attachment = tmp_path / "attachment.txt"
    attachment.write_text("attachment-content")

    response_data: dict[str, Any] = {}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_support_ticket_attachment(123, str(attachment))

    assert result == response_data
    mock_request.assert_awaited_once()
    await_args = mock_request.await_args
    assert await_args is not None
    args = await_args.args
    kwargs = await_args.kwargs
    assert args == (
        "POST",
        "https://api.linode.com/v4/support/tickets/123/attachments",
    )
    assert kwargs["headers"] == {
        "Authorization": "Bearer test-token",
        "User-Agent": "LinodeMCP/1.0",
    }
    filename, file_obj = kwargs["files"]["file"]
    assert filename == "attachment.txt"
    assert file_obj.name == str(attachment)

    await client.close()


async def test_create_support_ticket_attachment_wraps_http_errors(
    tmp_path: Any,
) -> None:
    """Test support ticket attachment creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")
    attachment = tmp_path / "attachment.txt"
    attachment.write_text("attachment-content")

    with patch.object(client.client, "request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_support_ticket_attachment(123, str(attachment))

    assert "CreateSupportTicketAttachment" in str(excinfo.value)
    await client.close()


async def test_retryable_create_support_ticket_attachment_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket attachment creation."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_support_ticket_attachment", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"id": 789}
        result = await retryable.create_support_ticket_attachment(123, "/Users/e/a.txt")

    assert result == {"id": 789}
    mock_create.assert_awaited_once_with(123, "/Users/e/a.txt")
    await retryable.close()


async def test_close_support_ticket_sends_post_to_ticket_close_route() -> None:
    """Test support ticket close sends documented POST route."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data: dict[str, Any] = {"id": 123, "status": "closed"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.close_support_ticket(123)

    assert result == response_data
    mock_request.assert_called_once_with("POST", "/support/tickets/123/close")
    await client.close()


async def test_close_support_ticket_wraps_http_errors() -> None:
    """Test support ticket close wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.close_support_ticket(123)

    assert "CloseSupportTicket" in str(excinfo.value)
    await client.close()


async def test_retryable_close_support_ticket_delegates_to_client() -> None:
    """Test RetryableClient delegates support ticket close."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "close_support_ticket", new_callable=AsyncMock
    ) as mock_close:
        mock_close.return_value = {"id": 123, "status": "closed"}
        result = await retryable.close_support_ticket(123)

    assert result == {"id": 123, "status": "closed"}
    mock_close.assert_awaited_once_with(123)
    await retryable.close()


async def test_delete_tag_sends_delete_to_tag_route() -> None:
    """Test deleting a tag sends DELETE /tags/{tagLabel}."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_tag("obsolete")

    mock_request.assert_called_once_with("DELETE", "/tags/obsolete")
    await client.close()


async def test_delete_tag_encodes_label_path_segment() -> None:
    """Test deleting a tag URL-encodes tagLabel as one path segment."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_tag("team/blue tag")

    mock_request.assert_called_once_with("DELETE", "/tags/team%2Fblue%20tag")
    await client.close()


async def test_delete_tag_wraps_http_errors() -> None:
    """Test deleting a tag wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_tag("obsolete")

    assert "DeleteTag" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_tag_delegates_to_client() -> None:
    """Test RetryableClient delegates tag delete to Client.delete_tag."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_tag", new_callable=AsyncMock
    ) as mock_delete:
        await retryable.delete_tag("obsolete")

    mock_delete.assert_awaited_once_with("obsolete")
    await retryable.close()


async def test_list_volume_types_sends_get_to_volume_types_route() -> None:
    """Test listing volume types sends GET /volumes/types."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {
        "data": [
            {
                "id": "volume",
                "label": "Storage Volume",
                "price": {"hourly": 0.0015, "monthly": 0.10},
                "region_prices": [
                    {"id": "us-iad", "hourly": 0.00018, "monthly": 0.12},
                ],
                "transfer": 0,
            },
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

        volume_types = await client.list_volume_types()

    assert volume_types == response_data["data"]
    mock_request.assert_called_once_with("GET", "/volumes/types")

    await client.close()


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


async def test_clone_volume_sends_post_to_volume_clone_route() -> None:
    """Test cloning a volume sends POST /volumes/{id}/clone."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 23456,
        "label": "my-volume-clone",
        "status": "creating",
        "size": 20,
        "region": "us-east",
        "linode_id": None,
        "linode_label": None,
        "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_my-volume-clone",
        "created": "2024-01-15T10:00:00",
        "updated": "2024-01-15T10:00:00",
        "tags": [],
        "hardware_type": "nvme",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        volume = await client.clone_volume(12345, "my-volume-clone")

    assert volume.id == 23456
    assert volume.label == "my-volume-clone"
    mock_request.assert_called_once_with(
        "POST",
        "/volumes/12345/clone",
        {"label": "my-volume-clone"},
    )

    await client.close()


async def test_update_volume_sends_put_to_volume_route() -> None:
    """Test updating a volume sends PUT /volumes/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {
        "id": 12345,
        "label": "renamed-volume",
        "status": "active",
        "size": 20,
        "region": "us-east",
        "linode_id": None,
        "linode_label": None,
        "filesystem_path": "/dev/disk/by-id/scsi-0Linode_Volume_renamed-volume",
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

        volume = await client.update_volume(
            12345,
            label="renamed-volume",
            tags=["prod"],
        )

    assert volume.id == 12345
    assert volume.label == "renamed-volume"
    assert volume.tags == ["prod"]
    mock_request.assert_called_once_with(
        "PUT",
        "/volumes/12345",
        {"label": "renamed-volume", "tags": ["prod"]},
    )

    await client.close()


async def test_update_ssh_key_sends_put_to_profile_route() -> None:
    """Test updating an SSH key sends PUT /profile/sshkeys/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {
        "id": 12345,
        "label": "renamed-key",
        "ssh_key": "ssh-rsa AAAA_valid_public_ssh_key user@example",
        "created": "2024-01-15T10:00:00",
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        ssh_key = await client.update_ssh_key(12345, "renamed-key")

    assert ssh_key.id == 12345
    assert ssh_key.label == "renamed-key"
    mock_request.assert_called_once_with(
        "PUT",
        "/profile/sshkeys/12345",
        {"label": "renamed-key"},
    )

    await client.close()


async def test_create_instance_sends_interfaces_body(
    sample_instance_data: dict[str, Any],
) -> None:
    """The POST body for create_instance must match BIMHelperScripts
    linode_add_network at api-common.sh:378: interface_generation = "linode"
    plus a single-element interfaces[] with public={}, default_route, and an
    interface-level firewall_id.
    """
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = sample_instance_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.create_instance(
            region="us-east",
            instance_type="g6-nanode-1",
            firewall_id=12345,
        )

    assert mock_request.called, "make_request should have been called"
    _method, _path, body = mock_request.call_args.args
    assert body["interface_generation"] == "linode"
    assert len(body["interfaces"]) == 1
    iface = body["interfaces"][0]
    assert iface["public"] == {}, (
        "public must be an empty object so the API assigns defaults"
    )
    assert iface["default_route"] == {"ipv4": True, "ipv6": True}
    assert iface["firewall_id"] == 12345
    assert "firewall_id" not in body, (
        "firewall_id must be at the interface level, not top level"
    )

    await client.close()


async def test_create_instance_omits_route_keys_when_false(
    sample_instance_data: dict[str, Any],
) -> None:
    """When route_ipv4=False the ipv4 key must be absent from default_route,
    not sent as False. The API treats absence as "not the default route" for
    that family.
    """
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = sample_instance_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.create_instance(
            region="us-east",
            instance_type="g6-nanode-1",
            firewall_id=12345,
            route_ipv4=False,
            route_ipv6=True,
        )

    _method, _path, body = mock_request.call_args.args
    default_route = body["interfaces"][0]["default_route"]
    assert "ipv4" not in default_route, (
        "default_route.ipv4 must be omitted when route_ipv4 is False"
    )
    assert default_route["ipv6"] is True

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


async def test_get_region_sends_exact_route() -> None:
    """Getting a region sends GET /regions/{regionId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {
        "id": "us-east",
        "label": "Newark, NJ",
        "country": "us",
        "capabilities": ["Linodes", "Block Storage"],
        "status": "ok",
        "resolvers": {"ipv4": "192.0.2.1", "ipv6": "2001:db8::1"},
        "site_type": "core",
    }
    response = MagicMock()
    response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_region("us-east")

    assert result.id == "us-east"
    assert result.label == "Newark, NJ"
    assert result.resolvers.ipv4 == "192.0.2.1"
    mock_request.assert_called_once_with("GET", "/regions/us-east")

    await client.close()


async def test_get_region_url_encodes_region_id() -> None:
    """Region get escapes path separators before sending request."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": "us/east?x=1",
        "label": "Encoded",
        "country": "us",
        "capabilities": [],
        "status": "ok",
        "resolvers": {},
        "site_type": "core",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_region("us/east?x=1")

    assert result.id == "us/east?x=1"
    mock_request.assert_called_once_with("GET", "/regions/us%2Feast%3Fx%3D1")

    await client.close()


async def test_get_region_wraps_http_error() -> None:
    """Region get wraps client HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_region("us-east")

    assert "GetRegion" in str(exc_info.value)

    await client.close()


async def test_retryable_get_region_delegates() -> None:
    """Retryable client delegates region get to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    region = Region(
        id="us-east",
        label="Newark, NJ",
        country="us",
        capabilities=["Linodes"],
        status="ok",
        resolvers=Resolver(ipv4="192.0.2.1", ipv6="2001:db8::1"),
        site_type="core",
    )

    with patch.object(client.client, "get_region", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = region

        result = await client.get_region("us-east")

    assert result is region
    mock_get.assert_awaited_once_with("us-east")

    await client.close()


async def test_get_region_availability_sends_exact_route() -> None:
    """Getting region availability sends GET /regions/{regionId}/availability."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = [
        {"available": True, "plan": "g6-standard-1", "region": "us-east"}
    ]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_region_availability("us-east")

    assert result == [{"available": True, "plan": "g6-standard-1", "region": "us-east"}]
    mock_request.assert_called_once_with("GET", "/regions/us-east/availability")

    await client.close()


async def test_get_region_availability_url_encodes_region_id() -> None:
    """Region availability escapes path separators before sending request."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = []

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_region_availability("us/east?x=1")

    assert result == []
    mock_request.assert_called_once_with(
        "GET", "/regions/us%2Feast%3Fx%3D1/availability"
    )

    await client.close()


async def test_get_region_availability_wraps_http_error() -> None:
    """Region availability wraps client HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_region_availability("us-east")

    assert "GetRegionAvailability" in str(exc_info.value)

    await client.close()


async def test_retryable_get_region_availability_delegates() -> None:
    """Retryable client delegates region availability to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "get_region_availability", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = [
            {"available": True, "plan": "g6-standard-1", "region": "us-east"}
        ]

        result = await client.get_region_availability("us-east")

    assert result == [{"available": True, "plan": "g6-standard-1", "region": "us-east"}]
    mock_get.assert_awaited_once_with("us-east")

    await client.close()


async def test_list_regions_availability_sends_exact_route() -> None:
    """Listing regions availability sends GET /regions/availability."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = [
        {"available": True, "plan": "g6-standard-1", "region": "us-east"},
        {"available": False, "plan": "g6-standard-2", "region": "us-west"},
    ]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.list_regions_availability()

    assert result == [
        {"available": True, "plan": "g6-standard-1", "region": "us-east"},
        {"available": False, "plan": "g6-standard-2", "region": "us-west"},
    ]
    mock_request.assert_called_once_with("GET", "/regions/availability")

    await client.close()


async def test_list_regions_availability_wraps_http_error() -> None:
    """Regions availability wraps client HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.list_regions_availability()

    assert "ListRegionsAvailability" in str(exc_info.value)

    await client.close()


async def test_retryable_list_regions_availability_delegates() -> None:
    """Retryable client delegates regions availability to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_regions_availability", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = [
            {"available": True, "plan": "g6-standard-1", "region": "us-east"}
        ]

        result = await client.list_regions_availability()

    assert result == [{"available": True, "plan": "g6-standard-1", "region": "us-east"}]
    mock_list.assert_awaited_once_with()

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


async def test_create_ipv6_range_posts_linode_id_body() -> None:
    """Creating an IPv6 range should POST the linode_id payload."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "range": "2001:0db8::/64",
        "route_target": "2001:0db8::1",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.create_ipv6_range(64, linode_id=123)

        assert result == response.json.return_value
        mock_request.assert_awaited_once_with(
            "POST",
            "/networking/ipv6/ranges",
            {"prefix_length": 64, "linode_id": 123},
        )

    await client.close()


async def test_create_ipv6_range_posts_route_target_body() -> None:
    """Creating an IPv6 range should POST the route_target payload."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "range": "2001:0db8::/56",
        "route_target": "2001:0db8::1",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.create_ipv6_range(56, route_target="2001:0db8::1")

        mock_request.assert_awaited_once_with(
            "POST",
            "/networking/ipv6/ranges",
            {"prefix_length": 56, "route_target": "2001:0db8::1"},
        )

    await client.close()


async def test_list_ipv6_ranges_sends_get() -> None:
    """Listing IPv6 ranges should send GET /networking/ipv6/ranges."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"range": "2001:0db8::/64", "region": "us-east", "prefix": 64}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_ipv6_ranges()

        assert result == mock_response.json.return_value
        mock_request.assert_awaited_once_with("GET", "/networking/ipv6/ranges")

    await client.close()


async def test_list_ipv6_ranges_sends_get_with_pagination() -> None:
    """Listing IPv6 ranges with pagination should include query params."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [],
        "page": 1,
        "pages": 1,
        "results": 0,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_ipv6_ranges(page=2, page_size=50)

        mock_request.assert_awaited_once()
        call_args = mock_request.call_args
        assert call_args[0][0] == "GET"
        assert "page=2" in call_args[0][1]
        assert "page_size=50" in call_args[0][1]

    await client.close()


async def test_list_ipv6_pools_sends_get() -> None:
    """Listing IPv6 pools should send GET /networking/ipv6/pools."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "range": "2001:0db8::",
                "region": "us-east",
                "prefix": 124,
                "route_target": "2001:0db8::f",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_ipv6_pools()

        assert result == mock_response.json.return_value
        mock_request.assert_awaited_once_with("GET", "/networking/ipv6/pools")

    await client.close()


async def test_list_ipv6_pools_sends_get_with_pagination() -> None:
    """Listing IPv6 pools with pagination should include query params."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [],
        "page": 1,
        "pages": 1,
        "results": 0,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_ipv6_pools(page=2, page_size=50)

        mock_request.assert_awaited_once()
        call_args = mock_request.call_args
        assert call_args[0][0] == "GET"
        assert "page=2" in call_args[0][1]
        assert "page_size=50" in call_args[0][1]

    await client.close()


async def test_list_ipv6_pools_wraps_http_errors() -> None:
    """IPv6 pools list wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.list_ipv6_pools()

    assert "ListIPv6Pools" in str(exc_info.value)
    await client.close()


async def test_list_placement_groups_sends_get_with_pagination() -> None:
    """Placement groups list sends query pagination params."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {
        "data": [{"id": 123, "label": "pg-a"}],
        "page": 2,
        "pages": 3,
        "results": 51,
    }
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_placement_groups(page=2, page_size=25)

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "GET", "/placement/groups?page=2&page_size=25"
    )
    await client.close()


async def test_list_placement_groups_wraps_http_errors() -> None:
    """Placement groups list wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.list_placement_groups()

    assert "ListPlacementGroups" in str(exc_info.value)
    await client.close()


async def test_retryable_list_placement_groups_delegates_to_client() -> None:
    """Retryable client delegates placement groups list calls."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_placement_groups", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": []}
        result = await retryable.list_placement_groups(page=1, page_size=100)

    assert result == {"data": []}
    mock_list.assert_awaited_once_with(page=1, page_size=100)
    await retryable.close()


async def test_create_placement_group_posts_required_body() -> None:
    """Creating a placement group should POST the documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": 789,
        "label": "pg-a",
        "region": "us-mia",
        "placement_group_type": "anti_affinity:local",
        "placement_group_policy": "strict",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.create_placement_group(
            "pg-a", "us-mia", "anti_affinity:local", "strict"
        )

        assert result == response.json.return_value
        mock_request.assert_awaited_once_with(
            "POST",
            "/placement/groups",
            {
                "label": "pg-a",
                "region": "us-mia",
                "placement_group_type": "anti_affinity:local",
                "placement_group_policy": "strict",
            },
        )

    await client.close()


@pytest.mark.parametrize("label", ["", "-bad", "bad-", "bad/label", "bad?label"])
async def test_create_placement_group_rejects_invalid_label(label: str) -> None:
    """Creating a placement group should reject invalid labels locally."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="label must start and end"),
    ):
        await client.create_placement_group(
            label, "us-mia", "anti_affinity:local", "strict"
        )

    mock_request.assert_not_called()
    await client.close()


@pytest.mark.parametrize(
    ("region", "placement_group_type", "placement_group_policy", "error"),
    [
        ("", "anti_affinity:local", "strict", "region is required"),
        ("us-mia", "affinity:local", "strict", "placement_group_type"),
        ("us-mia", "anti_affinity:local", "best-effort", "placement_group_policy"),
    ],
)
async def test_create_placement_group_rejects_invalid_values(
    region: str, placement_group_type: str, placement_group_policy: str, error: str
) -> None:
    """Creating a placement group should reject invalid body values locally."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match=error),
    ):
        await client.create_placement_group(
            "pg-a", region, placement_group_type, placement_group_policy
        )

    mock_request.assert_not_called()
    await client.close()


async def test_get_object_storage_cluster_sends_get_to_cluster_route() -> None:
    """Object Storage cluster get sends GET to the single-cluster route."""
    client = Client("https://api.linode.com/v4", "test-token")
    cluster = {
        "id": "us-east-1",
        "region": "us-east",
        "domain": "us-east-1.linodeobjects.com",
        "status": "available",
    }

    mock_response = MagicMock()
    mock_response.json.return_value = cluster

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_object_storage_cluster("us-east-1")

    assert result == cluster
    mock_request.assert_called_once_with("GET", "/object-storage/clusters/us-east-1")
    await client.close()


async def test_get_object_storage_cluster_url_encodes_cluster_id() -> None:
    """Object Storage cluster path parameter is URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.json.return_value = {"id": "escaped"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_object_storage_cluster("us/east?1..")

    assert result == {"id": "escaped"}
    mock_request.assert_called_once_with(
        "GET", "/object-storage/clusters/us%2Feast%3F1.."
    )
    await client.close()


async def test_get_object_storage_cluster_wraps_http_errors() -> None:
    """Object Storage cluster get wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_object_storage_cluster("us-east-1")

    assert "GetObjectStorageCluster" in str(excinfo.value)
    await client.close()


async def test_retryable_get_object_storage_cluster_delegates_to_client() -> None:
    """Retryable Object Storage cluster get delegates to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    cluster = {"id": "us-east-1"}

    with patch.object(
        client.client, "get_object_storage_cluster", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = cluster

        result = await client.get_object_storage_cluster("us-east-1")

    assert result == cluster
    mock_get.assert_awaited_once_with("us-east-1")
    await client.close()


async def test_create_placement_group_wraps_http_errors() -> None:
    """Creating a placement group should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.create_placement_group(
                "pg-a", "us-mia", "anti_affinity:local", "strict"
            )

    assert "CreatePlacementGroup" in str(exc_info.value)
    await client.close()


async def test_retryable_create_placement_group_delegates_to_client() -> None:
    """Retryable client should delegate placement group creation."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_data = {"id": 789, "label": "pg-a"}

    with patch.object(
        client.client,
        "create_placement_group",
        new_callable=AsyncMock,
    ) as mock_create:
        mock_create.return_value = response_data

        result = await client.create_placement_group(
            "pg-a", "us-mia", "anti_affinity:local", "strict"
        )

        assert result == response_data
        mock_create.assert_awaited_once_with(
            "pg-a", "us-mia", "anti_affinity:local", "strict"
        )

    await client.close()


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


async def test_assign_placement_group_posts_linode_body() -> None:
    """Assigning a placement group should POST the Linode IDs."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"linodes": [123, 456]}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.assign_placement_group(789, [123, 456])

        assert result == {"linodes": [123, 456]}
        mock_request.assert_awaited_once_with(
            "POST",
            "/placement/groups/789/assign",
            {"linodes": [123, 456]},
        )

    await client.close()


async def test_assign_placement_group_encodes_group_path() -> None:
    """Placement group ID interpolation should encode the path segment."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}
    group_id: Any = "12/../?x=1"

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.assign_placement_group(group_id, [123])

        mock_request.assert_awaited_once_with(
            "POST",
            "/placement/groups/12%2F..%2F%3Fx%3D1/assign",
            {"linodes": [123]},
        )

    await client.close()


async def test_retryable_assign_placement_group_delegates_to_client() -> None:
    """Retryable client should delegate placement group assign."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_data = {"linodes": [123]}

    with patch.object(
        client.client,
        "assign_placement_group",
        new_callable=AsyncMock,
    ) as mock_assign:
        mock_assign.return_value = response_data

        result = await client.assign_placement_group(789, [123])

        assert result == response_data
        mock_assign.assert_awaited_once_with(789, [123])

    await client.close()


async def test_unassign_placement_group_posts_linode_body() -> None:
    """Unassigning a placement group should POST the Linode IDs."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"linodes": [123, 456]}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.unassign_placement_group(789, [123, 456])

        assert result == {"linodes": [123, 456]}
        mock_request.assert_awaited_once_with(
            "POST",
            "/placement/groups/789/unassign",
            {"linodes": [123, 456]},
        )

    await client.close()


async def test_unassign_placement_group_encodes_group_path() -> None:
    """Placement group ID interpolation should encode the path segment."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}
    group_id: Any = "12/../?x=1"

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.unassign_placement_group(group_id, [123])

        mock_request.assert_awaited_once_with(
            "POST",
            "/placement/groups/12%2F..%2F%3Fx%3D1/unassign",
            {"linodes": [123]},
        )

    await client.close()


async def test_delete_placement_group_sends_delete() -> None:
    """Deleting a placement group should issue DELETE for the group path."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_placement_group(789)

        mock_request.assert_awaited_once_with("DELETE", "/placement/groups/789")

    await client.close()


async def test_delete_placement_group_encodes_group_path() -> None:
    """Placement group delete should encode the group path segment."""
    client = Client("https://api.linode.com/v4", "test-token")
    group_id: Any = "12/../?x=1"

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_placement_group(group_id)

        mock_request.assert_awaited_once_with(
            "DELETE",
            "/placement/groups/12%2F..%2F%3Fx%3D1",
        )

    await client.close()


async def test_delete_placement_group_wraps_http_errors() -> None:
    """Deleting a placement group should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.delete_placement_group(789)

    assert "DeletePlacementGroup" in str(exc_info.value)
    await client.close()


async def test_retryable_delete_placement_group_delegates_to_client() -> None:
    """Retryable client should delegate placement group deletion."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "delete_placement_group",
        new_callable=AsyncMock,
    ) as mock_delete:
        await client.delete_placement_group(789)

        mock_delete.assert_awaited_once_with(789)

    await client.close()


async def test_update_placement_group_puts_label_body() -> None:
    """Updating a placement group should PUT the label body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 789, "label": "new-label"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.update_placement_group(789, "new-label")

        assert result == {"id": 789, "label": "new-label"}
        mock_request.assert_awaited_once_with(
            "PUT",
            "/placement/groups/789",
            {"label": "new-label"},
        )

    await client.close()


async def test_update_placement_group_encodes_group_path() -> None:
    """Placement group update should encode the group path segment."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}
    group_id: Any = "12/../?x=1"

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.update_placement_group(group_id, "new-label")

        mock_request.assert_awaited_once_with(
            "PUT",
            "/placement/groups/12%2F..%2F%3Fx%3D1",
            {"label": "new-label"},
        )

    await client.close()


@pytest.mark.parametrize("label", ["", "/", "?", "..", "bad/label", "bad?label"])
async def test_update_placement_group_rejects_invalid_label(label: str) -> None:
    """Updating a placement group should reject invalid labels locally."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="label must start and end"),
    ):
        await client.update_placement_group(789, label)

    mock_request.assert_not_called()
    await client.close()


async def test_update_placement_group_wraps_http_errors() -> None:
    """Updating a placement group should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.update_placement_group(789, "new-label")

    assert "UpdatePlacementGroup" in str(exc_info.value)
    await client.close()


async def test_retryable_update_placement_group_delegates_to_client() -> None:
    """Retryable client should delegate placement group update."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_data = {"id": 789, "label": "new-label"}

    with patch.object(
        client.client,
        "update_placement_group",
        new_callable=AsyncMock,
    ) as mock_update:
        mock_update.return_value = response_data

        result = await client.update_placement_group(789, "new-label")

        assert result == response_data
        mock_update.assert_awaited_once_with(789, "new-label")

    await client.close()


async def test_retryable_unassign_placement_group_delegates_to_client() -> None:
    """Retryable client should delegate placement group unassign."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    response_data = {"linodes": [123]}

    with patch.object(
        client.client,
        "unassign_placement_group",
        new_callable=AsyncMock,
    ) as mock_unassign:
        mock_unassign.return_value = response_data

        result = await client.unassign_placement_group(789, [123])

        assert result == response_data
        mock_unassign.assert_awaited_once_with(789, [123])

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
            "/networking/ipv6/ranges/2001%3A0db8%3A%3A%2F64",
        )

    await client.close()


async def test_delete_ipv6_range_encodes_range_path() -> None:
    """Deleting an IPv6 range should encode the complete path segment."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_ipv6_range("2001:0db8::/64")

        mock_request.assert_awaited_once_with(
            "DELETE",
            "/networking/ipv6/ranges/2001%3A0db8%3A%3A%2F64",
        )

    await client.close()


async def test_retryable_create_ipv6_range_delegates_to_client() -> None:
    """Retryable client should delegate IPv6 range creation."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "create_ipv6_range",
        new_callable=AsyncMock,
    ) as mock_create:
        await client.create_ipv6_range(64, linode_id=123)

        mock_create.assert_awaited_once_with(64, 123, None)

    await client.close()


async def test_retryable_list_ipv6_ranges_delegates_to_client() -> None:
    """Retryable client should delegate IPv6 ranges listing."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    expected_response = {
        "data": [{"range": "2001:0db8::/64", "region": "us-east"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(
        client.client, "list_ipv6_ranges", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = expected_response

        result = await client.list_ipv6_ranges()

        assert result == expected_response
        mock_list.assert_awaited_once_with(page=None, page_size=None)

    await client.close()


async def test_retryable_list_ipv6_pools_delegates_to_client() -> None:
    """Retryable client should delegate IPv6 pools listing."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )
    expected_response = {
        "data": [{"range": "2001:0db8::", "region": "us-east", "prefix": 124}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(
        client.client, "list_ipv6_pools", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = expected_response

        result = await client.list_ipv6_pools()

        assert result == expected_response
        mock_list.assert_awaited_once_with(page=None, page_size=None)

    await client.close()


async def test_retryable_upload_image_does_not_replay() -> None:
    """Retryable client delegates image uploads once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "upload_image", new_callable=AsyncMock
    ) as mock_upload:
        mock_upload.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.upload_image(
                label="upload-image",
                region="us-east",
                cloud_init=True,
                description="Uploaded image",
                tags=["prod"],
            )

    mock_upload.assert_awaited_once_with(
        label="upload-image",
        region="us-east",
        cloud_init=True,
        description="Uploaded image",
        tags=["prod"],
    )
    await retryable.close()


async def test_retryable_create_image_delegates_to_client() -> None:
    """Retryable client should delegate image creation."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "create_image",
        new_callable=AsyncMock,
    ) as mock_create:
        await client.create_image(
            123,
            label="app-image",
            description="Application image",
            cloud_init=True,
            tags=["prod"],
        )

        mock_create.assert_awaited_once_with(
            disk_id=123,
            label="app-image",
            description="Application image",
            cloud_init=True,
            tags=["prod"],
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


async def test_retryable_delete_ipv6_range_delegates_to_client() -> None:
    """Retryable client should delegate IPv6 range deletion."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=1, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "delete_ipv6_range",
        new_callable=AsyncMock,
    ) as mock_delete:
        await client.delete_ipv6_range("2001:0db8::")

        mock_delete.assert_awaited_once_with("2001:0db8::")

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
        mock_list_vlans.assert_awaited_once_with()

    await client.close()


async def test_retryable_client_delete_vlan() -> None:
    """Test retryable client delegates VLAN deletion."""
    client = RetryableClient(
        "https://api.linode.com/v4", "test-token", RetryConfig(max_retries=3)
    )

    with patch.object(
        client.client, "delete_vlan", new_callable=AsyncMock
    ) as mock_delete_vlan:
        await client.delete_vlan("us-east", "app-vlan")

        mock_delete_vlan.assert_awaited_once_with("us-east", "app-vlan")

    await client.close()


async def test_client_share_ipv4s(linode_client: Client) -> None:
    """Test Client.share_ipv4s sends POST /networking/ips/share."""
    mock_response = MagicMock()
    mock_response.json.return_value = {"success": True, "shared": ["192.168.1.1"]}
    with patch.object(
        linode_client,
        "make_request",
        new_callable=AsyncMock,
    ) as mock_req:
        mock_req.return_value = mock_response
        result = await linode_client.share_ipv4s(["192.168.1.1"], 12345)
        mock_req.assert_awaited_once_with(
            "POST",
            "/networking/ips/share",
            {"ips": ["192.168.1.1"], "linode_id": 12345},
        )
        assert result == {"success": True, "shared": ["192.168.1.1"]}


async def test_client_share_ipv4s_network_error(linode_client: Client) -> None:
    """Test Client.share_ipv4s raises NetworkError on HTTP failure."""
    with patch.object(
        linode_client,
        "make_request",
        new_callable=AsyncMock,
    ) as mock_req:
        mock_req.side_effect = httpx.ConnectError("connection refused")
        with pytest.raises(NetworkError) as exc_info:
            await linode_client.share_ipv4s(["192.168.1.1"], 12345)
        assert "ShareIPv4s" in str(exc_info.value)


async def test_retryable_client_share_ipv4s() -> None:
    """Test retryable client delegates IPv4 sharing."""
    client = RetryableClient(
        "https://api.linode.com/v4", "test-token", RetryConfig(max_retries=3)
    )
    expected_result = {"success": True, "shared": ["192.168.1.1"]}

    with patch.object(
        client.client, "share_ipv4s", new_callable=AsyncMock
    ) as mock_share:
        mock_share.return_value = expected_result
        result = await client.share_ipv4s(["192.168.1.1"], 12345)

        assert result == expected_result
        mock_share.assert_awaited_once_with(["192.168.1.1"], 12345)

    await client.close()


async def test_client_assign_ipv4s(linode_client: Client) -> None:
    """Client.assign_ipv4s sends POST /networking/ips/assign."""
    assignments = [{"address": "192.0.2.1", "linode_id": 123}]
    mock_response = MagicMock()
    mock_response.json.return_value = {}

    with patch.object(
        linode_client,
        "make_request",
        new_callable=AsyncMock,
    ) as mock_req:
        mock_req.return_value = mock_response
        result = await linode_client.assign_ipv4s("us-east", assignments)

    mock_req.assert_awaited_once_with(
        "POST",
        "/networking/ips/assign",
        {"region": "us-east", "assignments": assignments},
    )
    assert result == {}


async def test_client_assign_ipv4s_network_error(linode_client: Client) -> None:
    """Client.assign_ipv4s raises NetworkError on HTTP failure."""
    with patch.object(
        linode_client,
        "make_request",
        new_callable=AsyncMock,
    ) as mock_req:
        mock_req.side_effect = httpx.ConnectError("connection refused")
        with pytest.raises(NetworkError) as exc_info:
            await linode_client.assign_ipv4s(
                "us-east", [{"address": "192.0.2.1", "linode_id": 123}]
            )

    assert "AssignIPv4s" in str(exc_info.value)


async def test_retryable_client_assign_ipv4s_delegates_without_retry() -> None:
    """RetryableClient.assign_ipv4s should not replay assignment calls."""
    client = RetryableClient(
        "https://api.linode.com/v4", "test-token", RetryConfig(max_retries=3)
    )
    assignments = [{"address": "192.0.2.1", "linode_id": 123}]

    with patch.object(
        client.client, "assign_ipv4s", new_callable=AsyncMock
    ) as mock_assign:
        mock_assign.side_effect = httpx.ConnectError("connection refused")
        with pytest.raises(httpx.ConnectError):
            await client.assign_ipv4s("us-east", assignments)

        mock_assign.assert_awaited_once_with("us-east", assignments)

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


async def test_get_account_payment_sends_get_to_exact_route() -> None:
    """Getting an account payment sends GET /account/payments/{id}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "usd": "10.00"}
    response = MagicMock()
    response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_account_payment(123)

    assert result == response_data
    mock_request.assert_awaited_once_with("GET", "/account/payments/123")
    await client.close()


async def test_get_account_payment_encodes_path_param() -> None:
    """Client URL-encodes the payment path parameter boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "123/456?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.get_account_payment("123/456?query")  # type: ignore[arg-type]

    assert result == {"id": "123/456?query"}
    mock_request.assert_awaited_once_with("GET", "/account/payments/123%2F456%3Fquery")
    await client.close()


async def test_get_account_payment_wraps_http_errors() -> None:
    """HTTP errors from payment reads are wrapped."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_account_payment(123)

    assert "GetAccountPayment" in str(excinfo.value)
    await client.close()


async def test_retryable_get_account_payment_delegates_to_client() -> None:
    """Retryable payment get delegates to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "usd": "10.00"}

    with patch.object(
        client.client, "get_account_payment", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = response_data

        result = await client.get_account_payment(123)

    assert result == response_data
    mock_get.assert_awaited_once_with(123)
    await client.close()


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


async def test_make_account_payment_method_default_sends_exact_route() -> None:
    """Setting the default payment method sends POST to the documented route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "is_default": True}
    response = MagicMock()
    response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.make_account_payment_method_default(123)

    assert result == response_data
    mock_request.assert_awaited_once_with(
        "POST", "/account/payment-methods/123/make-default"
    )
    await client.close()


async def test_make_account_payment_method_default_encodes_path_param() -> None:
    """Client URL-encodes the make-default payment method path parameter."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "123/456?query"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.make_account_payment_method_default(
            "123/456?query"  # type: ignore[arg-type]
        )

    assert result == {"id": "123/456?query"}
    mock_request.assert_awaited_once_with(
        "POST", "/account/payment-methods/123%2F456%3Fquery/make-default"
    )
    await client.close()


async def test_make_account_payment_method_default_wraps_http_errors() -> None:
    """HTTP errors from payment method make-default are wrapped."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.make_account_payment_method_default(123)

    assert "MakeAccountPaymentMethodDefault" in str(excinfo.value)
    await client.close()


async def test_retryable_make_account_payment_method_default_delegates_once() -> None:
    """Retryable make-default delegates once without generic retry replay."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    response_data: dict[str, Any] = {"id": 123, "is_default": True}

    with patch.object(
        client.client, "make_account_payment_method_default", new_callable=AsyncMock
    ) as mock_make_default:
        mock_make_default.return_value = response_data

        result = await client.make_account_payment_method_default(123)

    assert result == response_data
    mock_make_default.assert_awaited_once_with(123)
    await client.close()


async def test_retryable_make_payment_method_default_no_replay_errors() -> None:
    """Retryable make-default does not replay a transient failure."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "make_account_payment_method_default", new_callable=AsyncMock
    ) as mock_make_default:
        mock_make_default.side_effect = NetworkError("temporary", Exception("boom"))

        with pytest.raises(NetworkError):
            await client.make_account_payment_method_default(123)

    mock_make_default.assert_awaited_once_with(123)
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


# Stage 3 Client Tests


async def test_list_ssh_keys() -> None:
    """Test listing SSH keys."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "id": 1,
                "label": "work-key",
                "ssh_key": "ssh-rsa AAAA...",
                "created": "2024-01-01T00:00:00",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        keys = await client.list_ssh_keys()

        assert len(keys) == 1
        assert keys[0].id == 1
        assert keys[0].label == "work-key"

    await client.close()


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


async def test_list_domains() -> None:
    """Test listing domains."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
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
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        domains = await client.list_domains()

        assert len(domains) == 1
        assert domains[0].id == 1
        assert domains[0].domain == "example.com"

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


async def test_get_domain_zone_file() -> None:
    """Test getting a domain zone file."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"zone_file": ["$ORIGIN example.com."]}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        zone_file = await client.get_domain_zone_file(1)

    assert zone_file == DomainZoneFile(zone_file=["$ORIGIN example.com."])
    mock_request.assert_awaited_once_with("GET", "/domains/1/zone-file")
    await client.close()


@pytest.mark.parametrize(
    ("domain_id", "message"),
    [
        (0, "domain_id must be a positive integer"),
        (-1, "domain_id must be a positive integer"),
        (True, "domain_id must be a positive integer"),
        ("1/2", "domain_id must be a positive integer"),
        ("1?x=y", "domain_id must be a positive integer"),
        ("..", "domain_id must be a positive integer"),
    ],
)
async def test_get_domain_zone_file_rejects_invalid_domain_id(
    domain_id: Any, message: str
) -> None:
    """Domain zone file client rejects invalid path parameters."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match=message),
    ):
        await client.get_domain_zone_file(domain_id)

    mock_request.assert_not_called()
    await client.close()


async def test_retryable_get_domain_zone_file_uses_retry() -> None:
    """RetryableClient wraps domain zone file retrieval in retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable, "_execute_with_retry", new_callable=AsyncMock
    ) as mock_retry:
        mock_retry.return_value = DomainZoneFile(zone_file=["$ORIGIN example.com."])

        result = await retryable.get_domain_zone_file(1)

    assert result == DomainZoneFile(zone_file=["$ORIGIN example.com."])
    mock_retry.assert_awaited_once_with(retryable.client.get_domain_zone_file, 1)
    await retryable.close()


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


async def test_update_firewall_rules() -> None:
    """Test updating firewall rules."""
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
        "outbound": [],
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_firewall_rules(
            12345,
            inbound=[
                {
                    "action": "ACCEPT",
                    "protocol": "TCP",
                    "ports": "22",
                    "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
                    "label": "allow-ssh",
                    "description": "",
                }
            ],
            outbound=[],
        )

        assert result["inbound"] == [
            {
                "action": "ACCEPT",
                "protocol": "TCP",
                "ports": "22",
                "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
                "label": "allow-ssh",
                "description": "",
            }
        ]
        assert result["outbound"] == []
        mock_request.assert_awaited_once()
        args, _kwargs = mock_request.await_args_list[0]
        assert args[0] == "PUT"
        assert args[1] == "/networking/firewalls/12345/rules"
        assert args[2] == {
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
            "outbound": [],
        }

    await client.close()


@pytest.mark.parametrize("firewall_id", [0, -1, "12345", True])
async def test_update_firewall_rules_rejects_invalid_firewall_id(
    firewall_id: Any,
) -> None:
    """Test firewall rule update rejects invalid firewall IDs."""
    client = Client("https://api.linode.com/v4", "test-token")

    with pytest.raises(ValueError, match="firewall_id must be a positive integer"):
        await client.update_firewall_rules(firewall_id, inbound=[], outbound=[])

    await client.close()


@pytest.mark.parametrize(
    ("inbound", "outbound", "message"),
    [
        ({}, [], "inbound must be a list of rule objects"),
        (["bad-rule"], [], "inbound must be a list of rule objects"),
        ([], {}, "outbound must be a list of rule objects"),
        ([], ["bad-rule"], "outbound must be a list of rule objects"),
    ],
)
async def test_update_firewall_rules_rejects_invalid_rule_lists(
    inbound: Any, outbound: Any, message: str
) -> None:
    """Test firewall rule update rejects invalid rule lists."""
    client = Client("https://api.linode.com/v4", "test-token")

    with pytest.raises(TypeError, match=message):
        await client.update_firewall_rules(12345, inbound=inbound, outbound=outbound)

    await client.close()


async def test_update_firewall_rules_wraps_http_errors() -> None:
    """Test firewall rule update wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="UpdateFirewallRules"):
            await client.update_firewall_rules(12345, inbound=[], outbound=[])

    await client.close()


async def test_retryable_update_firewall_rules_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall rule updates to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_firewall_rules", new_callable=AsyncMock
    ) as mock_update:
        mock_update.return_value = {"inbound": [], "outbound": []}
        result = await retryable.update_firewall_rules(12345, inbound=[], outbound=[])

    assert result == {"inbound": [], "outbound": []}
    mock_update.assert_awaited_once_with(12345, [], [])
    await retryable.close()


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


async def test_update_firewall_settings() -> None:
    """Test updating default firewalls."""
    client = Client("https://api.linode.com/v4", "test-token")
    payload = {
        "linode": 100,
        "nodebalancer": 101,
        "public_interface": 200,
        "vpc_interface": 201,
    }

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"default_firewall_ids": payload}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_firewall_settings(payload)

    assert result == {"default_firewall_ids": payload}
    mock_request.assert_awaited_once_with(
        "PUT", "/networking/firewalls/settings", {"default_firewall_ids": payload}
    )
    await client.close()


@pytest.mark.parametrize(
    ("default_firewall_ids", "message"),
    [
        ({}, "must contain at least one"),
        ({"unknown": 100}, "unsupported keys"),
        ({"linode": 0}, "linode must be a positive integer"),
        ({"linode": -1}, "linode must be a positive integer"),
        ({"linode": True}, "linode must be a positive integer"),
        ({"linode": "100"}, "linode must be a positive integer"),
    ],
)
async def test_update_firewall_settings_rejects_invalid_default_ids(
    default_firewall_ids: Any, message: str
) -> None:
    """Test default firewall update rejects invalid IDs."""
    client = Client("https://api.linode.com/v4", "test-token")

    with pytest.raises(ValueError, match=message):
        await client.update_firewall_settings(default_firewall_ids)

    await client.close()


async def test_update_firewall_settings_wraps_http_errors() -> None:
    """Test default firewall update wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError, match="UpdateFirewallSettings"):
            await client.update_firewall_settings({"linode": 100})

    await client.close()


async def test_retryable_update_firewall_settings_does_not_replay_put() -> None:
    """Test RetryableClient delegates default firewall update once."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_firewall_settings", new_callable=AsyncMock
    ) as mock_update:
        mock_update.side_effect = httpx.HTTPError("transient")

        with pytest.raises(httpx.HTTPError, match="transient"):
            await retryable.update_firewall_settings({"linode": 100})

    mock_update.assert_awaited_once_with({"linode": 100})
    await retryable.close()


async def test_list_firewalls() -> None:
    """Test listing firewalls."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "id": 1,
                "label": "web-fw",
                "status": "enabled",
                "rules": {
                    "inbound": [],
                    "outbound": [],
                    "inbound_policy": "DROP",
                    "outbound_policy": "ACCEPT",
                },
                "tags": [],
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

        firewalls = await client.list_firewalls()

        assert len(firewalls) == 1
        assert firewalls[0].id == 1
        assert firewalls[0].label == "web-fw"
        mock_request.assert_awaited_once_with("GET", "/networking/firewalls")

    await client.close()


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


async def test_get_firewall_rule_version() -> None:
    """Test getting a specific firewall rule version."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "action": "ACCEPT",
        "protocol": "TCP",
        "ports": "22",
        "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
        "label": "allow-ssh",
        "description": "Allow SSH traffic",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        rule = await client.get_firewall_rule_version(12345, "v1")

        assert rule.action == "ACCEPT"
        assert rule.protocol == "TCP"
        assert rule.ports == "22"
        assert rule.label == "allow-ssh"
        mock_request.assert_awaited_once()
        call_args = mock_request.call_args
        assert call_args[0][0] == "GET"
        assert "/history/rules/" in call_args[0][1]

    await client.close()


async def test_get_firewall_rule_version_encodes_path_params() -> None:
    """Test that version path param is URL-encoded."""
    from urllib.parse import quote

    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "action": "ACCEPT",
        "protocol": "TCP",
        "ports": "22",
        "addresses": {"ipv4": ["0.0.0.0/0"], "ipv6": ["::/0"]},
        "label": "allow-ssh",
        "description": "",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        # Test with a version containing path traversal characters
        await client.get_firewall_rule_version(12345, "v1/../../../etc/passwd")

        call_args = mock_request.call_args
        endpoint = call_args[0][1]
        expected_encoded = quote("v1/../../../etc/passwd", safe="")
        expected_path = f"/networking/firewalls/12345/history/rules/{expected_encoded}"
        assert endpoint == expected_path

    await client.close()


async def test_get_firewall_rule_version_wraps_http_errors() -> None:
    """Test get_firewall_rule_version wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=MagicMock(status_code=404)
        )

        with pytest.raises(NetworkError):
            await client.get_firewall_rule_version(12345, "v1")

    await client.close()


async def test_retryable_get_firewall_rule_version_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall rule version get to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    mock_rule = FirewallRule(
        action="ACCEPT",
        protocol="TCP",
        ports="22",
        addresses=FirewallAddresses(ipv4=["0.0.0.0/0"], ipv6=["::/0"]),
        label="allow-ssh",
        description="Allow SSH traffic",
    )

    with patch.object(
        retryable.client, "get_firewall_rule_version", new_callable=AsyncMock
    ) as mock_method:
        mock_method.return_value = mock_rule

        result = await retryable.get_firewall_rule_version(12345, "v1")

        assert result is mock_rule
        mock_method.assert_awaited_once_with(12345, "v1")

    await retryable.close()


async def test_list_object_storage_quotas_sends_exact_route() -> None:
    """Object Storage quotas list uses the documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "quota_id": "obj-buckets-us-sea-1.linodeobjects.com",
                "quota_limit": 1000,
            },
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        quotas = await client.list_object_storage_quotas()

        assert quotas == mock_response.json.return_value["data"]
        mock_request.assert_awaited_once_with("GET", "/object-storage/quotas")

    await client.close()


async def test_list_object_storage_endpoints_sends_exact_route() -> None:
    """Object Storage endpoints list uses the documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "endpoint_type": "E1",
                "region": "us-sea",
                "s3_endpoint": "us-sea-1.linodeobjects.com",
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        endpoints = await client.list_object_storage_endpoints()

        assert endpoints == mock_response.json.return_value["data"]
        mock_request.assert_awaited_once_with("GET", "/object-storage/endpoints")

    await client.close()


async def test_retryable_list_object_storage_endpoints_delegates_to_client() -> None:
    """Retryable client delegates endpoint list to the low-level client."""
    base_client = AsyncMock()
    base_client.list_object_storage_endpoints.return_value = [{"region": "us-sea"}]
    retryable = RetryableClient.__new__(RetryableClient)
    retryable.client = base_client

    with patch.object(
        RetryableClient, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:
        execute_with_retry.return_value = [{"region": "us-sea"}]

        result = await retryable.list_object_storage_endpoints()

        assert result == [{"region": "us-sea"}]
        execute_with_retry.assert_awaited_once_with(
            base_client.list_object_storage_endpoints
        )


async def test_get_network_transfer_prices_sends_exact_route() -> None:
    """Network transfer prices use the documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {
        "data": [
            {
                "id": "network_transfer",
                "label": "Network Transfer",
                "price": {"hourly": 0.005, "monthly": None},
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

        prices = await client.get_network_transfer_prices()

        assert prices == response_data
        mock_request.assert_awaited_once_with("GET", "/network-transfer/prices")

    await client.close()


async def test_retryable_get_network_transfer_prices_delegates_to_client() -> None:
    """Retryable client delegates network transfer price lookup through retry."""
    base_client = AsyncMock()
    base_client.get_network_transfer_prices.return_value = {"data": []}
    retryable = RetryableClient.__new__(RetryableClient)
    retryable.client = base_client

    with patch.object(
        RetryableClient, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:
        execute_with_retry.return_value = {"data": []}

        result = await retryable.get_network_transfer_prices()

        assert result == {"data": []}
        execute_with_retry.assert_awaited_once_with(
            base_client.get_network_transfer_prices
        )


async def test_retryable_list_object_storage_quotas_delegates_to_client() -> None:
    """Retryable client delegates quota list to the low-level client."""
    base_client = AsyncMock()
    base_client.list_object_storage_quotas.return_value = [{"quota_id": "obj-buckets"}]
    retryable = RetryableClient.__new__(RetryableClient)
    retryable.client = base_client

    with patch.object(
        RetryableClient, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:
        execute_with_retry.return_value = [{"quota_id": "obj-buckets"}]

        result = await retryable.list_object_storage_quotas()

        assert result == [{"quota_id": "obj-buckets"}]
        execute_with_retry.assert_awaited_once_with(
            base_client.list_object_storage_quotas
        )


async def test_list_object_storage_quotas_handles_empty_response_data() -> None:
    """Object Storage quotas list tolerates missing response data."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        quotas = await client.list_object_storage_quotas()

        assert quotas == []
        mock_request.assert_awaited_once_with("GET", "/object-storage/quotas")

    await client.close()


async def test_get_object_storage_quota_sends_exact_route() -> None:
    """Object Storage quota get uses the documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "description": "Current number of buckets per account, per endpoint",
        "endpoint_type": "E1",
        "has_usage": True,
        "quota_id": "obj-buckets-us-sea-1.linodeobjects.com",
        "quota_limit": 1000,
        "quota_name": "Number of Buckets",
        "quota_type": "obj-buckets",
        "resource_metric": "bucket",
        "s3_endpoint": "us-sea-1.linodeobjects.com",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        quota = await client.get_object_storage_quota(
            "obj-buckets-us-sea-1.linodeobjects.com"
        )

        assert quota == mock_response.json.return_value
        mock_request.assert_awaited_once_with(
            "GET", "/object-storage/quotas/obj-buckets-us-sea-1.linodeobjects.com"
        )

    await client.close()


async def test_get_object_storage_quota_encodes_path_param() -> None:
    """Quota ID is encoded at the low-level client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"quota_id": "quota/../?x=1"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_object_storage_quota("quota/../?x=1")

        mock_request.assert_awaited_once_with(
            "GET", "/object-storage/quotas/quota%2F..%2F%3Fx%3D1"
        )

    await client.close()


async def test_retryable_get_object_storage_quota_delegates_to_client() -> None:
    """Retryable client delegates quota get to the low-level client."""
    base_client = AsyncMock()
    base_client.get_object_storage_quota.return_value = {"quota_id": "obj-buckets"}
    retryable = RetryableClient.__new__(RetryableClient)
    retryable.client = base_client

    with patch.object(
        RetryableClient, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:
        execute_with_retry.return_value = {"quota_id": "obj-buckets"}

        result = await retryable.get_object_storage_quota("obj-buckets")

        assert result == {"quota_id": "obj-buckets"}
        execute_with_retry.assert_awaited_once_with(
            base_client.get_object_storage_quota, "obj-buckets"
        )


async def test_get_object_storage_quota_usage_sends_exact_route() -> None:
    """Object Storage quota usage uses the documented GET route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "quota_id": 123,
        "s3_endpoint": "us-east-1.linodeobjects.com",
        "usage": {
            "objects": 7,
            "size": 1048576,
        },
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        usage = await client.get_object_storage_quota_usage(123)

        assert usage == mock_response.json.return_value
        mock_request.assert_awaited_once_with("GET", "/object-storage/quotas/123/usage")

    await client.close()


async def test_get_object_storage_quota_usage_encodes_path_param() -> None:
    """Quota ID is encoded at the low-level client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"usage": {}}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_object_storage_quota_usage("quota/../?x=1")

        mock_request.assert_awaited_once_with(
            "GET", "/object-storage/quotas/quota%2F..%2F%3Fx%3D1/usage"
        )

    await client.close()


async def test_retryable_get_object_storage_quota_usage_delegates_to_client() -> None:
    """Retryable client delegates quota usage to the low-level client."""
    base_client = AsyncMock()
    base_client.get_object_storage_quota_usage.return_value = {"usage": {}}
    retryable = RetryableClient.__new__(RetryableClient)
    retryable.client = base_client

    with patch.object(
        RetryableClient, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:
        execute_with_retry.return_value = {"usage": {}}

        result = await retryable.get_object_storage_quota_usage(123)

        assert result == {"usage": {}}
        execute_with_retry.assert_awaited_once_with(
            base_client.get_object_storage_quota_usage, 123
        )


async def test_list_object_storage_buckets_for_region_sends_exact_route() -> None:
    """Region-scoped Object Storage bucket list sends the documented route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {
            "data": [
                {
                    "label": "app-data",
                    "region": "us-ord",
                    "hostname": "app-data.us-ord-1.linodeobjects.com",
                }
            ]
        }
        mock_request.return_value = mock_response

        result = await client.list_object_storage_buckets_for_region("us-ord")

        assert result == [
            {
                "label": "app-data",
                "region": "us-ord",
                "hostname": "app-data.us-ord-1.linodeobjects.com",
            }
        ]
        mock_request.assert_awaited_once_with("GET", "/object-storage/buckets/us-ord")

    await client.close()


async def test_list_object_storage_buckets_for_region_encodes_region_id() -> None:
    """Region ID path parameter is encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {"data": []}
        mock_request.return_value = mock_response

        result = await client.list_object_storage_buckets_for_region("us/../?x=1")

        assert result == []
        mock_request.assert_awaited_once_with(
            "GET", "/object-storage/buckets/us%2F..%2F%3Fx%3D1"
        )

    await client.close()


async def test_retryable_list_object_storage_buckets_for_region_delegates() -> None:
    """Retryable client delegates region-scoped bucket listing."""
    base_client = AsyncMock()
    base_client.list_object_storage_buckets_for_region.return_value = []
    retryable = RetryableClient.__new__(RetryableClient)
    retryable.client = base_client

    with patch.object(
        RetryableClient, "_execute_with_retry", new_callable=AsyncMock
    ) as execute_with_retry:
        execute_with_retry.return_value = []

        result = await retryable.list_object_storage_buckets_for_region("us-ord")

        assert result == []
        execute_with_retry.assert_awaited_once_with(
            base_client.list_object_storage_buckets_for_region, "us-ord"
        )


async def test_cancel_object_storage_sends_exact_route() -> None:
    """Test cancelling Object Storage sends the exact documented route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {"message": "scheduled"}
        mock_request.return_value = mock_response

        result = await client.cancel_object_storage()

        assert result == {"message": "scheduled"}
        mock_request.assert_awaited_once_with("POST", "/object-storage/cancel")

    await client.close()


async def test_retryable_cancel_object_storage_delegates_without_retry() -> None:
    """RetryableClient should not replay Object Storage cancellation."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with (
        patch.object(
            client.client, "cancel_object_storage", new_callable=AsyncMock
        ) as cancel_object_storage,
        patch.object(
            client, "_execute_with_retry", new_callable=AsyncMock
        ) as execute_with_retry,
    ):
        cancel_object_storage.return_value = {"message": "scheduled"}

        result = await client.cancel_object_storage()

        assert result == {"message": "scheduled"}
        cancel_object_storage.assert_awaited_once_with()
        execute_with_retry.assert_not_called()

    await client.close()


async def test_allow_object_storage_bucket_access() -> None:
    """Test allowing Object Storage bucket access."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_response = MagicMock()
        mock_response.json.return_value = {}
        mock_request.return_value = mock_response

        result = await client.allow_object_storage_bucket_access(
            "us-east-1",
            "app-bucket",
            acl="public-read",
            cors_enabled=True,
        )

        assert result == {}
        mock_request.assert_called_once_with(
            "POST",
            "/object-storage/buckets/us-east-1/app-bucket/access",
            {"acl": "public-read", "cors_enabled": True},
        )


async def test_upload_bucket_ssl() -> None:
    """Test uploading an Object Storage bucket SSL certificate."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"ssl": True}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.upload_bucket_ssl(
            "us-east-1",
            "app-bucket",
            "sample-certificate",
            "sample-private-key",
        )

        assert result == {"ssl": True}
        mock_request.assert_awaited_once_with(
            "POST",
            "/object-storage/buckets/us-east-1/app-bucket/ssl",
            {
                "certificate": "sample-certificate",
                "private_key": "sample-private-key",
            },
        )

    await client.close()


async def test_list_firewall_rule_versions() -> None:
    """Test listing firewall rule versions."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "id": 12345,
                "label": "my-firewall",
                "status": "enabled",
                "version": 1,
                "created": "2025-01-01T00:00:00",
                "updated": "2025-01-01T00:00:00",
                "tags": [],
                "rules": {
                    "inbound": [],
                    "outbound": [],
                    "inbound_policy": "ACCEPT",
                    "outbound_policy": "ACCEPT",
                },
            },
            {
                "id": 12345,
                "label": "my-firewall",
                "status": "enabled",
                "version": 2,
                "created": "2025-01-01T00:00:00",
                "updated": "2025-01-02T00:00:00",
                "tags": [],
                "rules": {
                    "inbound": [],
                    "outbound": [],
                    "inbound_policy": "ACCEPT",
                    "outbound_policy": "ACCEPT",
                },
            },
        ]
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        versions = await client.list_firewall_rule_versions(12345)

        assert len(versions) == 2
        assert versions[0].id == 12345
        assert versions[1].id == 12345
        mock_request.assert_awaited_once()
        call_args = mock_request.call_args
        assert call_args[0][0] == "GET"
        assert call_args[0][1] == "/networking/firewalls/12345/history"

    await client.close()


async def test_list_firewall_rule_versions_wraps_http_errors() -> None:
    """Test list_firewall_rule_versions wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=MagicMock(status_code=404)
        )

        with pytest.raises(NetworkError):
            await client.list_firewall_rule_versions(12345)

    await client.close()


async def test_retryable_list_firewall_rule_versions_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall rule versions list to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    mock_fw = Firewall(
        id=12345,
        label="my-firewall",
        status="enabled",
        created="2025-01-01T00:00:00",
        updated="2025-01-01T00:00:00",
        tags=[],
        rules=FirewallRules(
            inbound=[],
            outbound=[],
            inbound_policy="ACCEPT",
            outbound_policy="ACCEPT",
        ),
    )

    with patch.object(
        retryable.client, "list_firewall_rule_versions", new_callable=AsyncMock
    ) as mock_method:
        mock_method.return_value = [mock_fw]

        result = await retryable.list_firewall_rule_versions(12345)

        assert result == [mock_fw]
        mock_method.assert_awaited_once_with(12345)

    await retryable.close()


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


async def test_delete_vlan() -> None:
    """Test deleting a VLAN."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.delete_vlan("us-east", "app-vlan")

        mock_request.assert_awaited_once_with(
            "DELETE", "/networking/vlans/us-east/app-vlan"
        )

    await client.close()


async def test_list_nodebalancers() -> None:
    """Test listing nodebalancers."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
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
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        nbs = await client.list_nodebalancers()

        assert len(nbs) == 1
        assert nbs[0].id == 1
        assert nbs[0].label == "web-lb"

    await client.close()


async def test_list_nodebalancer_types_sends_get_to_nodebalancer_types_route() -> None:
    """Test listing nodebalancer types sends GET /nodebalancers/types."""
    client = Client("https://api.linode.com/v4", "test-token")

    response_data = {
        "data": [
            {
                "id": "nodebalancer-type-1",
                "label": "NodeBalancer Type 1",
                "price": {"hourly": 0.015, "monthly": 10.00},
            },
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

        types = await client.list_nodebalancer_types()

    assert types == response_data["data"]
    mock_request.assert_called_once_with("GET", "/nodebalancers/types")

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


async def test_list_nodebalancer_vpc_configs() -> None:
    """Test listing NodeBalancer VPC configurations."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "id": 6,
                "ipv4_range": "10.0.0.12/30",
                "ipv6_range": None,
                "nodebalancer_id": 8,
                "subnet_id": 1,
                "vpc_id": 1,
            }
        ],
        "page": 2,
        "pages": 3,
        "results": 6,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        configs = await client.list_nodebalancer_vpc_configs(8, page=2, page_size=25)

        assert configs["data"][0]["id"] == 6
        assert configs["results"] == 6
        mock_request.assert_called_once_with(
            "GET", "/nodebalancers/8/vpcs?page=2&page_size=25"
        )

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "encoded"),
    [
        ("1/2", "1%2F2"),
        ("1?x", "1%3Fx"),
        ("..", ".."),
    ],
)
async def test_list_nodebalancer_vpc_configs_encodes_path_params(
    nodebalancer_id: str, encoded: str
) -> None:
    """NodeBalancer VPC config list path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.list_nodebalancer_vpc_configs(nodebalancer_id)  # type: ignore[arg-type]

        mock_request.assert_called_once_with("GET", f"/nodebalancers/{encoded}/vpcs")

    await client.close()


async def test_list_nodebalancer_vpc_configs_wraps_http_errors() -> None:
    """Test listing NodeBalancer VPC configs wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_nodebalancer_vpc_configs(8)

    assert "ListNodeBalancerVPCConfigs" in str(excinfo.value)
    await client.close()


async def test_retryable_list_nodebalancer_vpc_configs_delegates_to_client() -> None:
    """Test RetryableClient delegates NodeBalancer VPC config listing."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_nodebalancer_vpc_configs", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [], "page": 1, "pages": 1, "results": 0}
        result = await retryable.list_nodebalancer_vpc_configs(8, page=1, page_size=100)

    assert result["data"] == []
    mock_list.assert_awaited_once_with(8, page=1, page_size=100)
    await retryable.close()


async def test_update_nodebalancer_firewalls() -> None:
    """Test updating NodeBalancer firewall assignments."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"id": 123, "label": "web-fw"}],
        "page": 2,
        "pages": 3,
        "results": 6,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_nodebalancer_firewalls(
            8, [123, 456], page=2, page_size=25
        )

        assert result["data"][0]["id"] == 123
        assert result["results"] == 6
        mock_request.assert_called_once_with(
            "PUT",
            "/nodebalancers/8/firewalls?page=2&page_size=25",
            {"firewall_ids": [123, 456]},
        )

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "encoded"),
    [
        ("1/2", "1%2F2"),
        ("1?x", "1%3Fx"),
        ("..", ".."),
    ],
)
async def test_update_nodebalancer_firewalls_encodes_path_params(
    nodebalancer_id: str, encoded: str
) -> None:
    """NodeBalancer firewall update path parameter is URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_nodebalancer_firewalls(nodebalancer_id, [])  # type: ignore[arg-type]

        mock_request.assert_called_once_with(
            "PUT", f"/nodebalancers/{encoded}/firewalls", {"firewall_ids": []}
        )

    await client.close()


async def test_update_nodebalancer_firewalls_wraps_http_errors() -> None:
    """Test updating NodeBalancer firewalls wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_nodebalancer_firewalls(8, [123])

    assert "UpdateNodeBalancerFirewalls" in str(excinfo.value)
    await client.close()


async def test_retryable_update_nodebalancer_firewalls_does_not_replay() -> None:
    """RetryableClient delegates firewall assignment updates once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_nodebalancer_firewalls", new_callable=AsyncMock
    ) as mock_update:
        mock_update.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.update_nodebalancer_firewalls(
                8, [123], page=1, page_size=100
            )

    mock_update.assert_awaited_once_with(8, [123], page=1, page_size=100)
    await retryable.close()


async def test_rebuild_nodebalancer_config() -> None:
    """Test rebuilding a NodeBalancer config."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"rebuilt": True}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.rebuild_nodebalancer_config(8, 6)

        assert result == {"rebuilt": True}
        mock_request.assert_called_once_with(
            "POST", "/nodebalancers/8/configs/6/rebuild", {}
        )

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "config_id", "encoded_nodebalancer_id", "encoded_config_id"),
    [
        ("1/2", "4?x", "1%2F2", "4%3Fx"),
        ("..", "..", "..", ".."),
    ],
)
async def test_rebuild_nodebalancer_config_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
) -> None:
    """NodeBalancer config rebuild path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.rebuild_nodebalancer_config(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
        )

        mock_request.assert_called_once_with(
            "POST",
            (
                f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
                f"{encoded_config_id}/rebuild"
            ),
            {},
        )

    await client.close()


async def test_rebuild_nodebalancer_config_wraps_http_errors() -> None:
    """Test rebuilding a NodeBalancer config wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.rebuild_nodebalancer_config(8, 6)

    assert "RebuildNodeBalancerConfig" in str(excinfo.value)
    await client.close()


async def test_retryable_rebuild_nodebalancer_config_does_not_replay() -> None:
    """RetryableClient delegates config rebuild once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "rebuild_nodebalancer_config", new_callable=AsyncMock
    ) as mock_rebuild:
        mock_rebuild.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.rebuild_nodebalancer_config(8, 6)

    mock_rebuild.assert_awaited_once_with(8, 6)
    await retryable.close()


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
        ("..", ".."),
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


async def test_create_nodebalancer_config_node() -> None:
    """Test creating a NodeBalancer config node."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 4,
        "label": "node-1",
        "address": "192.0.2.4:80",
        "mode": "accept",
        "weight": 50,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_nodebalancer_config_node(
            8,
            6,
            {
                "address": "192.0.2.4:80",
                "label": "node-1",
                "mode": "accept",
                "weight": 50,
            },
        )

        assert result == {
            "id": 4,
            "label": "node-1",
            "address": "192.0.2.4:80",
            "mode": "accept",
            "weight": 50,
        }
        mock_request.assert_called_once_with(
            "POST",
            "/nodebalancers/8/configs/6/nodes",
            {
                "address": "192.0.2.4:80",
                "label": "node-1",
                "mode": "accept",
                "weight": 50,
            },
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
        ("..", "../6", "..", "..%2F6"),
    ],
)
async def test_create_nodebalancer_config_node_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
) -> None:
    """NodeBalancer config node create path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.create_nodebalancer_config_node(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
            {"address": "192.0.2.4:80", "label": "node-1"},
        )

        mock_request.assert_called_once_with(
            "POST",
            (
                f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
                f"{encoded_config_id}/nodes"
            ),
            {"address": "192.0.2.4:80", "label": "node-1"},
        )

    await client.close()


async def test_create_nodebalancer_config_node_wraps_http_errors() -> None:
    """Test creating a NodeBalancer config node wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_nodebalancer_config_node(
                8, 6, {"address": "192.0.2.4:80", "label": "node-1"}
            )

    assert "CreateNodeBalancerConfigNode" in str(excinfo.value)
    await client.close()


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
        ("..", "../6", "..", "..%2F6"),
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


async def test_retryable_create_nodebalancer_config_node_does_not_replay() -> None:
    """RetryableClient delegates config node create once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    fields = {"address": "192.0.2.4:80", "label": "node-1"}

    with patch.object(
        retryable.client, "create_nodebalancer_config_node", new_callable=AsyncMock
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.create_nodebalancer_config_node(8, 6, fields)

    mock_create.assert_awaited_once_with(8, 6, fields)
    await retryable.close()


async def test_update_nodebalancer_config_node() -> None:
    """Test updating a NodeBalancer config node."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 4, "address": "192.0.2.4:80"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_nodebalancer_config_node(
            8, 6, 4, {"address": "192.0.2.4:80", "weight": 50}
        )

        assert result == {"id": 4, "address": "192.0.2.4:80"}
        mock_request.assert_called_once_with(
            "PUT",
            "/nodebalancers/8/configs/6/nodes/4",
            {"address": "192.0.2.4:80", "weight": 50},
        )

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
async def test_update_nodebalancer_config_node_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    node_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
    encoded_node_id: str,
) -> None:
    """NodeBalancer config node update path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_nodebalancer_config_node(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
            cast("Any", node_id),
            {"mode": "drain"},
        )

        mock_request.assert_called_once_with(
            "PUT",
            (
                f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
                f"{encoded_config_id}/nodes/{encoded_node_id}"
            ),
            {"mode": "drain"},
        )

    await client.close()


async def test_update_nodebalancer_config_node_wraps_http_errors() -> None:
    """Test updating a NodeBalancer config node wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_nodebalancer_config_node(8, 6, 4, {"mode": "reject"})

    assert "UpdateNodeBalancerConfigNode" in str(excinfo.value)
    await client.close()


async def test_retryable_update_nodebalancer_config_node_does_not_replay() -> None:
    """RetryableClient delegates config node update once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_nodebalancer_config_node", new_callable=AsyncMock
    ) as mock_update:
        mock_update.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.update_nodebalancer_config_node(8, 6, 4, {"mode": "reject"})

    mock_update.assert_awaited_once_with(8, 6, 4, {"mode": "reject"})
    await retryable.close()


async def test_delete_nodebalancer_config() -> None:
    """Test deleting a NodeBalancer config."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = MagicMock(status_code=200)

        await client.delete_nodebalancer_config(8, 6)

        mock_request.assert_called_once_with("DELETE", "/nodebalancers/8/configs/6")

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "config_id", "encoded_nodebalancer_id", "encoded_config_id"),
    [
        ("1/2", "6", "1%2F2", "6"),
        ("8", "6?x", "8", "6%3Fx"),
        ("8", "../6", "8", "..%2F6"),
    ],
)
async def test_delete_nodebalancer_config_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
) -> None:
    """NodeBalancer config delete path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = MagicMock(status_code=200)

        await client.delete_nodebalancer_config(
            cast("Any", nodebalancer_id), cast("Any", config_id)
        )

        mock_request.assert_called_once_with(
            "DELETE",
            (f"/nodebalancers/{encoded_nodebalancer_id}/configs/{encoded_config_id}"),
        )

    await client.close()


async def test_delete_nodebalancer_config_wraps_http_errors() -> None:
    """Test deleting a NodeBalancer config wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_nodebalancer_config(8, 6)

    assert "DeleteNodeBalancerConfig" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_nodebalancer_config_does_not_replay() -> None:
    """RetryableClient delegates config delete once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_nodebalancer_config", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.delete_nodebalancer_config(8, 6)

    mock_delete.assert_awaited_once_with(8, 6)
    await retryable.close()


async def test_delete_nodebalancer_config_node() -> None:
    """Test deleting a NodeBalancer config node."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = MagicMock(status_code=200)

        await client.delete_nodebalancer_config_node(8, 6, 4)

        mock_request.assert_called_once_with(
            "DELETE", "/nodebalancers/8/configs/6/nodes/4"
        )

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
async def test_delete_nodebalancer_config_node_encodes_path_params(
    nodebalancer_id: str,
    config_id: str,
    node_id: str,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
    encoded_node_id: str,
) -> None:
    """NodeBalancer config node delete path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = MagicMock(status_code=200)

        await client.delete_nodebalancer_config_node(
            cast("Any", nodebalancer_id),
            cast("Any", config_id),
            cast("Any", node_id),
        )

        mock_request.assert_called_once_with(
            "DELETE",
            (
                f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
                f"{encoded_config_id}/nodes/{encoded_node_id}"
            ),
        )

    await client.close()


async def test_delete_nodebalancer_config_node_wraps_http_errors() -> None:
    """Test deleting a NodeBalancer config node wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_nodebalancer_config_node(8, 6, 4)

    assert "DeleteNodeBalancerConfigNode" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_nodebalancer_config_node_does_not_replay() -> None:
    """RetryableClient delegates config node delete once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_nodebalancer_config_node", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.delete_nodebalancer_config_node(8, 6, 4)

    mock_delete.assert_awaited_once_with(8, 6, 4)
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
        ("..", ".."),
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


async def test_get_nodebalancer_vpc_config() -> None:
    """Test getting a NodeBalancer VPC configuration."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 456,
        "vpc_id": 789,
        "subnet_id": 101,
        "ipv4_range": "10.0.0.0/24",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        config = await client.get_nodebalancer_vpc_config(123, 456)

        assert config["id"] == 456
        assert config["vpc_id"] == 789
        mock_request.assert_called_once_with("GET", "/nodebalancers/123/vpcs/456")

    await client.close()


async def test_get_nodebalancer_vpc_config_encodes_path_params() -> None:
    """NodeBalancer VPC config path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 4}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_nodebalancer_vpc_config("1/2", "4?x")  # type: ignore[arg-type]

        mock_request.assert_called_once_with("GET", "/nodebalancers/1%2F2/vpcs/4%3Fx")

    await client.close()


async def test_get_nodebalancer_stats() -> None:
    """Test getting NodeBalancer statistics."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": {
            "connections": [[1526391300000, 0]],
            "traffic": {
                "in": [[1526391300000, 631.21]],
                "out": [[1526391300000, 103.44]],
            },
        },
        "title": "linode.com - balancer12345 (12345) - day (5 min avg)",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        stats = await client.get_nodebalancer_stats(1)

        assert stats["data"]["connections"] == [[1526391300000, 0]]
        mock_request.assert_called_once_with("GET", "/nodebalancers/1/stats")

    await client.close()


async def test_get_nodebalancer_stats_encodes_path_params() -> None:
    """NodeBalancer stats path parameter is URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": {}}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_nodebalancer_stats("1/2")  # type: ignore[arg-type]

        mock_request.assert_called_once_with("GET", "/nodebalancers/1%2F2/stats")

    await client.close()


async def test_get_nodebalancer_stats_wraps_http_errors() -> None:
    """NodeBalancer stats HTTP errors are wrapped in NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=MagicMock(status_code=404)
        )

        with pytest.raises(NetworkError, match="GetNodeBalancerStats"):
            await client.get_nodebalancer_stats(1)

    await client.close()


async def test_list_stackscripts() -> None:
    """Test listing stackscripts."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "id": 1,
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
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        scripts = await client.list_stackscripts()

        assert len(scripts) == 1
        assert scripts[0].id == 1
        assert scripts[0].label == "my-script"

    await client.close()


async def test_create_stackscript() -> None:
    """Test creating a StackScript."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 1,
        "username": "testuser",
        "user_gravatar_id": "abc123",
        "label": "my-script",
        "description": "Test script",
        "images": ["linode/ubuntu22.04"],
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

        stackscript = await client.create_stackscript(
            label="my-script",
            images=["linode/ubuntu22.04"],
            script="#!/bin/bash",
            description="Test script",
            is_public=False,
            rev_note="Initial revision",
        )

        mock_request.assert_called_once_with(
            "POST",
            "/linode/stackscripts",
            {
                "label": "my-script",
                "images": ["linode/ubuntu22.04"],
                "script": "#!/bin/bash",
                "description": "Test script",
                "is_public": False,
                "rev_note": "Initial revision",
            },
        )
        assert stackscript.id == 1
        assert stackscript.label == "my-script"

    await client.close()


async def test_delete_image_sharegroup_sends_delete_to_encoded_path() -> None:
    """Image share group delete should issue DELETE to the encoded share group path."""
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_image_sharegroup("11111111-1111-4111-8111-111111111111")

    mock_request.assert_awaited_once_with(
        "DELETE", "/images/sharegroups/11111111-1111-4111-8111-111111111111"
    )
    await client.close()


async def test_delete_image_sharegroup_encodes_path_segment() -> None:
    """Image share group delete should encode separator characters."""
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_image_sharegroup("12/../?x=1")

    mock_request.assert_awaited_once_with(
        "DELETE", "/images/sharegroups/12%2F..%2F%3Fx%3D1"
    )
    await client.close()


async def test_delete_image_sharegroup_wraps_http_errors() -> None:
    """Image share group delete should map HTTP failures to NetworkError."""
    client = Client("https://api.linode.test/v4", "token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.delete_image_sharegroup("11111111-1111-4111-8111-111111111111")

    assert "DeleteImageSharegroup" in str(exc_info.value)
    await client.close()


async def test_retryable_delete_image_sharegroup_delegates_once() -> None:
    """Retryable delete wrapper should not replay share group deletion."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=3, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "delete_image_sharegroup",
        new_callable=AsyncMock,
    ) as mock_delete:
        mock_delete.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await client.delete_image_sharegroup("11111111-1111-4111-8111-111111111111")

        mock_delete.assert_awaited_once_with("11111111-1111-4111-8111-111111111111")

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


async def test_update_image_sharegroup_sends_put_to_encoded_path() -> None:
    """Image share group update should PUT documented body fields."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": "11111111-1111-4111-8111-111111111111",
        "label": "partner-group",
        "description": "Shared images",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.update_image_sharegroup(
            "11111111-1111-4111-8111-111111111111",
            label="partner-group",
            description="Shared images",
        )

        assert result["label"] == "partner-group"
        mock_request.assert_awaited_once_with(
            "PUT",
            "/images/sharegroups/11111111-1111-4111-8111-111111111111",
            {"label": "partner-group", "description": "Shared images"},
        )

    await client.close()


async def test_update_image_sharegroup_omits_absent_optional_fields() -> None:
    """Image share group update should omit absent optional fields."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "sharegroup-record-1", "label": "partner-group"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.update_image_sharegroup(
            "11111111-1111-4111-8111-111111111111", label="partner-group"
        )

        mock_request.assert_awaited_once_with(
            "PUT",
            "/images/sharegroups/11111111-1111-4111-8111-111111111111",
            {"label": "partner-group"},
        )

    await client.close()


async def test_update_image_sharegroup_encodes_path_segment() -> None:
    """Client boundary should encode unsafe sharegroup_id path input."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": "sharegroup-record-1"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.update_image_sharegroup("12/../?x=1", label="partner-group")

        mock_request.assert_awaited_once_with(
            "PUT",
            "/images/sharegroups/12%2F..%2F%3Fx%3D1",
            {"label": "partner-group"},
        )

    await client.close()


async def test_update_image_sharegroup_rejects_empty_body() -> None:
    """Image share group update should reject no-op bodies before HTTP."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="at least one"),
    ):
        await client.update_image_sharegroup("11111111-1111-4111-8111-111111111111")

    mock_request.assert_not_called()
    await client.close()


async def test_retryable_update_image_sharegroup_rejects_empty_body() -> None:
    """Retryable update wrapper should reject no-op bodies before client calls."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=3, base_delay=0.01),
    )

    with (
        patch.object(
            client.client,
            "update_image_sharegroup",
            new_callable=AsyncMock,
        ) as mock_update,
        pytest.raises(ValueError, match="at least one"),
    ):
        await client.update_image_sharegroup("11111111-1111-4111-8111-111111111111")

    mock_update.assert_not_called()
    await client.close()


async def test_update_image_sharegroup_wraps_http_errors() -> None:
    """Image share group update should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.update_image_sharegroup(
                "11111111-1111-4111-8111-111111111111", label="partner-group"
            )

    assert "UpdateImageSharegroup" in str(exc_info.value)
    await client.close()


async def test_retryable_update_image_sharegroup_delegates_once() -> None:
    """Retryable update wrapper should not replay share group updates."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=3, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "update_image_sharegroup",
        new_callable=AsyncMock,
    ) as mock_update:
        mock_update.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await client.update_image_sharegroup(
                "11111111-1111-4111-8111-111111111111",
                label="partner-group",
                description="Shared images",
            )

        mock_update.assert_awaited_once_with(
            "11111111-1111-4111-8111-111111111111",
            label="partner-group",
            description="Shared images",
        )

    await client.close()


async def test_create_image_sharegroup_token_sends_post_body() -> None:
    """Image share group token create should POST the documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": "sharegroup-record-1",
        "label": "partner-token",
        "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        result = await client.create_image_sharegroup_token(
            valid_for_sharegroup_uuid="11111111-1111-4111-8111-111111111111",
            label="partner-token",
        )

        assert result["id"] == "sharegroup-record-1"
        mock_request.assert_awaited_once_with(
            "POST",
            "/images/sharegroups/tokens",
            {
                "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
                "label": "partner-token",
            },
        )

    await client.close()


async def test_create_image_sharegroup_token_omits_optional_label() -> None:
    """Image share group token create should not send absent optional fields."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": "sharegroup-record-1",
        "valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response

        await client.create_image_sharegroup_token(
            valid_for_sharegroup_uuid="11111111-1111-4111-8111-111111111111"
        )

        mock_request.assert_awaited_once_with(
            "POST",
            "/images/sharegroups/tokens",
            {"valid_for_sharegroup_uuid": "11111111-1111-4111-8111-111111111111"},
        )

    await client.close()


async def test_create_image_sharegroup_token_wraps_http_errors() -> None:
    """Image share group token create should wrap HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as exc_info:
            await client.create_image_sharegroup_token(
                valid_for_sharegroup_uuid="11111111-1111-4111-8111-111111111111"
            )

    assert "CreateImageSharegroupToken" in str(exc_info.value)
    await client.close()


async def test_retryable_create_image_sharegroup_token_delegates_once() -> None:
    """Retryable create wrapper should not replay token creation after errors."""
    client = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=3, base_delay=0.01),
    )

    with patch.object(
        client.client,
        "create_image_sharegroup_token",
        new_callable=AsyncMock,
    ) as mock_create:
        mock_create.side_effect = httpx.HTTPError("temporary")

        with pytest.raises(httpx.HTTPError):
            await client.create_image_sharegroup_token(
                valid_for_sharegroup_uuid="11111111-1111-4111-8111-111111111111",
                label="partner-token",
            )

        mock_create.assert_awaited_once_with(
            valid_for_sharegroup_uuid="11111111-1111-4111-8111-111111111111",
            label="partner-token",
        )

    await client.close()


async def test_create_image_sends_post_to_images_route() -> None:
    """Test creating an image sends POST /images."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": "private/12345",
        "label": "app-image",
        "description": "Application image",
        "type": "manual",
        "is_public": False,
        "deprecated": False,
        "size": 2048,
        "vendor": "",
        "status": "creating",
        "created": "2024-01-01T00:00:00",
        "created_by": "testuser",
        "updated": "2024-01-01T00:00:00",
        "expiry": None,
        "eol": None,
        "capabilities": ["cloud-init"],
        "regions": [],
        "tags": ["prod"],
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        image = await client.create_image(
            disk_id=123,
            label="app-image",
            description="Application image",
            cloud_init=True,
            tags=["prod"],
        )

        mock_request.assert_called_once_with(
            "POST",
            "/images",
            {
                "disk_id": 123,
                "label": "app-image",
                "description": "Application image",
                "cloud_init": True,
                "tags": ["prod"],
            },
        )
        assert image.id == "private/12345"
        assert image.label == "app-image"

    await client.close()


async def test_upload_image_sends_post_to_upload_route() -> None:
    """Image upload creation sends POST /images/upload with documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "image": {"id": "private/98765", "label": "upload-image"},
        "upload_to": "https://uploads.example.invalid/image",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.upload_image(
            label="upload-image",
            region="us-east",
            cloud_init=True,
            description="Uploaded image",
            tags=["prod"],
        )

    mock_request.assert_awaited_once_with(
        "POST",
        "/images/upload",
        {
            "label": "upload-image",
            "region": "us-east",
            "cloud_init": True,
            "description": "Uploaded image",
            "tags": ["prod"],
        },
    )
    assert result["image"]["id"] == "private/98765"
    assert result["upload_to"] == "https://uploads.example.invalid/image"
    await client.close()


async def test_upload_image_wraps_http_errors() -> None:
    """Image upload creation wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.upload_image(label="upload-image", region="us-east")

    assert "UploadImage" in str(excinfo.value)
    await client.close()


# Validation function tests


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


# ---------------------------------------------------------------------------
# Client.make_request HTTP mechanics
# ---------------------------------------------------------------------------


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

    async def test_update_instance_put_shape(
        self, sample_instance_data: dict[str, Any]
    ) -> None:
        """PUT to a Linode instance sends the update body to the instance path."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {**sample_instance_data, "label": "updated"}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.update_instance(
                123, label="updated", tags=["prod"], watchdog_enabled=False
            )

            assert result.label == "updated"
            assert mock_req.call_args[0][0] == "PUT"
            assert mock_req.call_args[0][1].endswith("/linode/instances/123")
            assert mock_req.call_args[1]["json"] == {
                "label": "updated",
                "tags": ["prod"],
                "watchdog_enabled": False,
            }

        await client.close()

    async def test_list_monitor_services_get_shape(self) -> None:
        """GET /monitor/services returns the paginated services payload."""
        client = Client("https://api.linode.com/v4", "test-token")
        response_data = {
            "data": [{"label": "Databases", "service_type": "dbaas"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
        mock_response = MagicMock()
        mock_response.json.return_value = response_data

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.return_value = mock_response
            result = await client.list_monitor_services()

        assert result == response_data
        mock_request.assert_awaited_once_with("GET", "/monitor/services")
        await client.close()

    async def test_list_monitor_services_wraps_http_errors(self) -> None:
        """HTTP errors while listing monitor services are wrapped."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("boom")
            with pytest.raises(NetworkError, match="ListMonitorServices"):
                await client.list_monitor_services()

        await client.close()

    async def test_list_monitor_dashboards_get_shape(self) -> None:
        """GET /monitor/dashboards returns the paginated dashboards payload."""
        client = Client("https://api.linode.com/v4", "test-token")
        response_data = {
            "data": [{"id": 1, "label": "Resource Usage"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
        mock_response = MagicMock()
        mock_response.json.return_value = response_data

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.return_value = mock_response
            result = await client.list_monitor_dashboards()

        assert result == response_data
        mock_request.assert_awaited_once_with("GET", "/monitor/dashboards")
        await client.close()

    async def test_list_monitor_alert_channels_get_shape(self) -> None:
        """GET /monitor/alert-channels returns alert channels."""
        client = Client("https://api.linode.com/v4", "test-token")
        response_data = {
            "data": [{"id": 10000, "label": "Email Ops", "type": "email"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
        mock_response = MagicMock()
        mock_response.json.return_value = response_data

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.return_value = mock_response
            result = await client.list_monitor_alert_channels()

        assert result == response_data
        mock_request.assert_awaited_once_with("GET", "/monitor/alert-channels")
        await client.close()

    async def test_list_monitor_alert_channels_wraps_http_errors(self) -> None:
        """HTTP errors while listing monitor alert channels are wrapped."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("boom")
            with pytest.raises(NetworkError, match="ListMonitorAlertChannels"):
                await client.list_monitor_alert_channels()

        await client.close()

    async def test_list_monitor_alert_definitions_get_shape(self) -> None:
        """GET /monitor/alert-definitions returns alert definitions."""
        client = Client("https://api.linode.com/v4", "test-token")
        response_data = {
            "data": [{"id": 1, "label": "CPU Usage"}],
            "page": 1,
            "pages": 1,
            "results": 1,
        }
        mock_response = MagicMock()
        mock_response.json.return_value = response_data

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.return_value = mock_response
            result = await client.list_monitor_alert_definitions()

        assert result == response_data
        mock_request.assert_awaited_once_with("GET", "/monitor/alert-definitions")
        await client.close()

    async def test_list_monitor_alert_definitions_wraps_http_errors(self) -> None:
        """HTTP errors while listing monitor alert definitions are wrapped."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("boom")
            with pytest.raises(NetworkError, match="ListMonitorAlertDefinitions"):
                await client.list_monitor_alert_definitions()

        await client.close()

    async def test_list_monitor_dashboards_wraps_http_errors(self) -> None:
        """HTTP errors while listing monitor dashboards are wrapped."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("boom")
            with pytest.raises(NetworkError, match="ListMonitorDashboards"):
                await client.list_monitor_dashboards()

        await client.close()

    async def test_get_monitor_service_get_shape(self) -> None:
        """GET monitor service endpoint URL-encodes the service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "label": "Databases",
            "service_type": "dbaas",
        }

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.get_monitor_service("weird/type with space?and=query")

            url_arg = mock_req.call_args[0][1]
            assert result == {"label": "Databases", "service_type": "dbaas"}
            assert mock_req.call_args[0][0] == "GET"
            assert url_arg.endswith(
                "/monitor/services/weird%2Ftype%20with%20space%3Fand%3Dquery"
            )
            assert "json" not in mock_req.call_args[1]

        await client.close()

    async def test_get_monitor_service_rejects_empty_service_type(self) -> None:
        """Client raises ValueError before issuing a request for empty service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.get_monitor_service("")
            mock_req.assert_not_called()

        await client.close()

    async def test_get_monitor_service_wraps_http_errors(self) -> None:
        """Client wraps HTTP errors with the get monitor service operation."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("timeout")
            with pytest.raises(NetworkError) as exc_info:
                await client.get_monitor_service("dbaas")

        assert exc_info.value.operation == "GetMonitorService"
        await client.close()

    async def test_create_monitor_service_token_post_shape(self) -> None:
        """POST to monitor token endpoint URL-encodes the service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "token": "jwt.payload.signature",
            "expiry": "2026-06-01T00:00:00Z",
        }

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.create_monitor_service_token(
                "weird/type with space", [10, 20]
            )

            # Path segment is URL-encoded so "/" and " " never escape the segment.
            url_arg = mock_req.call_args[0][1]
            assert url_arg.endswith(
                "/monitor/services/weird%2Ftype%20with%20space/token"
            )
            assert mock_req.call_args[1]["json"] == {"entity_ids": [10, 20]}
            assert result["token"] == "jwt.payload.signature"
            assert result["expiry"] == "2026-06-01T00:00:00Z"

        await client.close()

    async def test_create_monitor_service_token_rejects_empty_inputs(self) -> None:
        """Client raises ValueError before issuing a request for empty inputs."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.create_monitor_service_token("", [1])
            with pytest.raises(ValueError, match="entity_ids"):
                await client.create_monitor_service_token("dbaas", [])
            mock_req.assert_not_called()

        await client.close()

    async def test_create_monitor_service_alert_definition_post_shape(self) -> None:
        """POST alert definition endpoint URL-encodes path params and sends body."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"id": 67890, "label": "CPU high"}
        rule_criteria = {"rules": [{"metric": "cpu_usage", "operator": "gt"}]}
        trigger_conditions = {"criteria_condition": "ALL"}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.create_monitor_service_alert_definition(
                "weird/type with space?and=query",
                label="CPU high",
                severity=1,
                rule_criteria=rule_criteria,
                trigger_conditions=trigger_conditions,
                channel_ids=[10000],
                description="High CPU usage",
                entity_ids=[12345],
            )

            url_arg = mock_req.call_args[0][1]
            assert result == {"id": 67890, "label": "CPU high"}
            assert mock_req.call_args[0][0] == "POST"
            assert url_arg.endswith(
                "/monitor/services/"
                "weird%2Ftype%20with%20space%3Fand%3Dquery"
                "/alert-definitions"
            )
            assert mock_req.call_args[1]["json"] == {
                "label": "CPU high",
                "severity": 1,
                "rule_criteria": rule_criteria,
                "trigger_conditions": trigger_conditions,
                "channel_ids": [10000],
                "description": "High CPU usage",
                "entity_ids": [12345],
            }

        await client.close()

    async def test_create_monitor_service_alert_definition_rejects_invalid_inputs(
        self,
    ) -> None:
        """Client rejects invalid create inputs before issuing a request."""
        client = Client("https://api.linode.com/v4", "test-token")
        rule_criteria = {"rules": [{"metric": "cpu_usage"}]}
        trigger_conditions = {"criteria_condition": "ALL"}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.create_monitor_service_alert_definition(
                    "",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="label"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                )
            with pytest.raises(TypeError, match="severity"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=cast("Any", True),
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="severity"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=4,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="rule_criteria"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria={},
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="rule_criteria"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=cast("Any", ["bad"]),
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="trigger_conditions"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions={},
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="trigger_conditions"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=cast("Any", True),
                    channel_ids=[10000],
                )
            with pytest.raises(ValueError, match="channel_ids"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=cast("Any", "bad"),
                )
            with pytest.raises(ValueError, match="channel_ids"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[],
                )
            with pytest.raises(ValueError, match="channel_ids"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[cast("Any", True)],
                )
            with pytest.raises(ValueError, match="description"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                    description=cast("Any", 123),
                )
            with pytest.raises(ValueError, match="entity_ids"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                    entity_ids=[],
                )
            with pytest.raises(ValueError, match="entity_ids"):
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria=rule_criteria,
                    trigger_conditions=trigger_conditions,
                    channel_ids=[10000],
                    entity_ids=[1, cast("Any", "bad")],
                )
            mock_req.assert_not_called()

        await client.close()

    async def test_create_monitor_service_alert_definition_wraps_http_errors(
        self,
    ) -> None:
        """Client wraps HTTP errors with the create alert definition operation."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.ReadTimeout("timeout")
            with pytest.raises(NetworkError) as exc_info:
                await client.create_monitor_service_alert_definition(
                    "dbaas",
                    label="CPU high",
                    severity=1,
                    rule_criteria={"rules": [{"metric": "cpu_usage"}]},
                    trigger_conditions={"criteria_condition": "ALL"},
                    channel_ids=[10000],
                )

        assert exc_info.value.operation == "CreateMonitorServiceAlertDefinition"
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

    async def test_delete_monitor_service_alert_definition_delete_shape(self) -> None:
        """DELETE alert definition endpoint URL-encodes path params."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            await client.delete_monitor_service_alert_definition(
                "weird/type with space?and=query", 12345
            )

            url_arg = mock_req.call_args[0][1]
            assert mock_req.call_args[0][0] == "DELETE"
            assert url_arg.endswith(
                "/monitor/services/"
                "weird%2Ftype%20with%20space%3Fand%3Dquery"
                "/alert-definitions/12345"
            )
            assert "json" not in mock_req.call_args[1]

        await client.close()

    async def test_delete_monitor_service_alert_definition_rejects_invalid_inputs(
        self,
    ) -> None:
        """Client rejects invalid delete inputs before issuing a request."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.delete_monitor_service_alert_definition("", 12345)
            with pytest.raises(TypeError, match="alert_id"):
                await client.delete_monitor_service_alert_definition("dbaas", True)
            with pytest.raises(ValueError, match="positive"):
                await client.delete_monitor_service_alert_definition("dbaas", 0)
            with pytest.raises(ValueError, match="positive"):
                await client.delete_monitor_service_alert_definition("dbaas", -1)
            mock_req.assert_not_called()

        await client.close()

    async def test_get_monitor_dashboard_get_shape(self) -> None:
        """GET monitor dashboard endpoint URL-encodes the dashboard_id."""
        client = Client("https://api.linode.com/v4", "test-token")
        response = MagicMock()
        response.status_code = 200
        response.json.return_value = {"id": 12345, "label": "Resource Usage"}

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.return_value = response
            result = await client.get_monitor_dashboard(12345)

        assert result == {"id": 12345, "label": "Resource Usage"}
        mock_request.assert_awaited_once_with("GET", "/monitor/dashboards/12345")
        await client.close()

    @pytest.mark.parametrize(
        "bad_dashboard_id", [True, "12345", "1/2", "1?x", "..", 12.9]
    )
    async def test_get_monitor_dashboard_rejects_invalid_dashboard_id(
        self, bad_dashboard_id: object
    ) -> None:
        """Client rejects invalid dashboard IDs before making a request."""
        client = Client("https://api.linode.com/v4", "test-token")

        with (
            patch.object(
                client, "make_request", new_callable=AsyncMock
            ) as mock_request,
            pytest.raises(TypeError, match="dashboard_id must be a valid integer"),
        ):
            await client.get_monitor_dashboard(cast("int", bad_dashboard_id))

        mock_request.assert_not_called()
        await client.close()

    @pytest.mark.parametrize("bad_dashboard_id", [0, -1])
    async def test_get_monitor_dashboard_rejects_non_positive_dashboard_id(
        self, bad_dashboard_id: int
    ) -> None:
        """Client rejects non-positive dashboard IDs before making a request."""
        client = Client("https://api.linode.com/v4", "test-token")

        with (
            patch.object(
                client, "make_request", new_callable=AsyncMock
            ) as mock_request,
            pytest.raises(ValueError, match="dashboard_id must be a positive integer"),
        ):
            await client.get_monitor_dashboard(bad_dashboard_id)

        mock_request.assert_not_called()
        await client.close()

    async def test_retryable_list_monitor_dashboards_delegates_to_client(self) -> None:
        """Retryable monitor dashboards list delegates to the client."""
        retryable = RetryableClient("https://api.linode.com/v4", "test-token")
        payload = {"data": [{"id": 1, "label": "Resource Usage"}]}

        with patch.object(
            retryable,
            "_execute_with_retry",
            new_callable=AsyncMock,
        ) as mock_execute:
            mock_execute.return_value = payload
            result = await retryable.list_monitor_dashboards()

        assert result == payload
        mock_execute.assert_awaited_once_with(retryable.client.list_monitor_dashboards)
        await retryable.close()

    async def test_retryable_get_monitor_dashboard_delegates_to_client(self) -> None:
        """Retryable monitor dashboard get delegates to the client."""
        retryable = RetryableClient("https://api.linode.com/v4", "test-token")
        payload = {"id": 12345, "label": "Resource Usage"}

        with patch.object(
            retryable,
            "_execute_with_retry",
            new_callable=AsyncMock,
        ) as mock_execute:
            mock_execute.return_value = payload
            result = await retryable.get_monitor_dashboard(12345)

        assert result == payload
        mock_execute.assert_awaited_once_with(
            retryable.client.get_monitor_dashboard, 12345
        )
        await retryable.close()

    async def test_list_monitor_service_dashboards_get_shape(self) -> None:
        """GET dashboards endpoint URL-encodes the service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "data": [
                {
                    "id": 1,
                    "label": "Resource Usage",
                    "service_type": "dbaas",
                    "type": "standard",
                    "widgets": [],
                }
            ],
            "page": 1,
            "pages": 1,
            "results": 1,
        }

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.list_monitor_service_dashboards(
                "weird/type with space?and=query"
            )

            url_arg = mock_req.call_args[0][1]
            assert mock_req.call_args[0][0] == "GET"
            assert url_arg.endswith(
                "/monitor/services/weird%2Ftype%20with%20space%3Fand%3Dquery/dashboards"
            )
            assert "json" not in mock_req.call_args[1]
            assert result["data"][0]["label"] == "Resource Usage"

        await client.close()

    async def test_list_monitor_service_dashboards_rejects_empty_service_type(
        self,
    ) -> None:
        """Client raises ValueError before issuing a request for empty service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.list_monitor_service_dashboards("")
            mock_req.assert_not_called()

        await client.close()

    async def test_read_monitor_service_metrics_post_shape(self) -> None:
        """POST to monitor metrics endpoint URL-encodes the service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"data": [{"label": "cpu", "value": 1.0}]}

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.read_monitor_service_metrics("weird/type with space")

            url_arg = mock_req.call_args[0][1]
            assert url_arg.endswith(
                "/monitor/services/weird%2Ftype%20with%20space/metrics"
            )
            assert mock_req.call_args[1]["json"] == {}
            assert result["data"] == [{"label": "cpu", "value": 1.0}]

        await client.close()

    async def test_list_monitor_service_alert_definitions_get_shape(self) -> None:
        """GET alert definitions endpoint URL-encodes the service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "data": [
                {
                    "id": 123,
                    "label": "CPU Usage",
                    "severity": 2,
                }
            ],
            "page": 1,
            "pages": 1,
            "results": 1,
        }

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.list_monitor_service_alert_definitions(
                "weird/type with space?and=query"
            )

            url_arg = mock_req.call_args[0][1]
            assert mock_req.call_args[0][0] == "GET"
            assert url_arg.endswith(
                "/monitor/services/"
                "weird%2Ftype%20with%20space%3Fand%3Dquery/alert-definitions"
            )
            assert "json" not in mock_req.call_args[1]
            assert result["data"][0]["label"] == "CPU Usage"

        await client.close()

    async def test_list_monitor_service_alert_definitions_rejects_empty_service_type(
        self,
    ) -> None:
        """Client raises ValueError before issuing a request for empty service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.list_monitor_service_alert_definitions("")
            mock_req.assert_not_called()

        await client.close()

    async def test_list_monitor_service_alert_definitions_wraps_http_error(
        self,
    ) -> None:
        """Client wraps HTTP errors while listing alert definitions."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client, "make_request", new_callable=AsyncMock) as mock_req:
            mock_req.side_effect = httpx.HTTPError("boom")
            with pytest.raises(NetworkError) as exc_info:
                await client.list_monitor_service_alert_definitions("dbaas")

        assert exc_info.value.operation == "ListMonitorServiceAlertDefinitions"
        mock_req.assert_awaited_once_with(
            "GET", "/monitor/services/dbaas/alert-definitions"
        )
        await client.close()

    async def test_list_monitor_service_metric_definitions_get_shape(self) -> None:
        """GET metric definitions endpoint URL-encodes the service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {
            "data": [
                {
                    "label": "CPU Usage",
                    "metric": "cpu_usage",
                    "metric_type": "gauge",
                }
            ],
            "page": 1,
            "pages": 1,
            "results": 1,
        }

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response

            result = await client.list_monitor_service_metric_definitions(
                "weird/type with space"
            )

            url_arg = mock_req.call_args[0][1]
            assert mock_req.call_args[0][0] == "GET"
            assert url_arg.endswith(
                "/monitor/services/weird%2Ftype%20with%20space/metric-definitions"
            )
            assert "json" not in mock_req.call_args[1]
            assert result["data"][0]["metric"] == "cpu_usage"

        await client.close()

    async def test_list_monitor_service_metric_definitions_rejects_empty_service_type(
        self,
    ) -> None:
        """Client raises ValueError before issuing a request for empty service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.list_monitor_service_metric_definitions("")
            mock_req.assert_not_called()

        await client.close()

    async def test_read_monitor_service_metrics_rejects_empty_service_type(
        self,
    ) -> None:
        """Client raises ValueError before issuing a request for empty service_type."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.read_monitor_service_metrics("")
            mock_req.assert_not_called()

        await client.close()

    async def test_update_monitor_alert_definition_put_shape(self) -> None:
        """PUT to monitor alert endpoint URL-encodes both path parameters."""
        client = Client("https://api.linode.com/v4", "test-token")
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.json.return_value = {"id": 42, "label": "cpu high"}
        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            mock_req.return_value = mock_response
            result = await client.update_monitor_alert_definition(
                "weird/type with space",
                42,
                label="cpu high",
                status="enabled",
                description=None,
            )
            assert mock_req.call_args[0][0] == "PUT"
            assert mock_req.call_args[0][1].endswith(
                "/monitor/services/weird%2Ftype%20with%20space/alert-definitions/42"
            )
            assert mock_req.call_args[1]["json"] == {
                "label": "cpu high",
                "status": "enabled",
            }
            assert result == {"id": 42, "label": "cpu high"}
        await client.close()

    async def test_update_monitor_alert_definition_rejects_invalid_inputs(self) -> None:
        """Client validates required path params before issuing a request."""
        client = Client("https://api.linode.com/v4", "test-token")
        with patch.object(client.client, "request", new_callable=AsyncMock) as mock_req:
            with pytest.raises(ValueError, match="service_type"):
                await client.update_monitor_alert_definition("", 42, label="cpu high")
            with pytest.raises(TypeError, match="alert_id"):
                await client.update_monitor_alert_definition(
                    "linode", True, label="cpu high"
                )
            with pytest.raises(ValueError, match="positive"):
                await client.update_monitor_alert_definition(
                    "linode", 0, label="cpu high"
                )
            with pytest.raises(ValueError, match="positive"):
                await client.update_monitor_alert_definition(
                    "linode", -1, label="cpu high"
                )
            with pytest.raises(ValueError, match="update field"):
                await client.update_monitor_alert_definition("linode", 42)
            with pytest.raises(ValueError, match="update field"):
                await client.update_monitor_alert_definition(
                    "linode", 42, label=None, status=None
                )
            mock_req.assert_not_called()
        await client.close()

    async def test_update_monitor_alert_definition_wraps_http_errors(self) -> None:
        """Client wraps HTTP errors with the update monitor alert operation."""
        client = Client("https://api.linode.com/v4", "test-token")

        with patch.object(
            client, "make_request", new_callable=AsyncMock
        ) as mock_request:
            mock_request.side_effect = httpx.HTTPError("boom")
            with pytest.raises(NetworkError) as excinfo:
                await client.update_monitor_alert_definition(
                    "linode", 42, label="cpu high"
                )

        assert "UpdateMonitorAlertDefinition" in str(excinfo.value)
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


async def test_create_profile_tfa_secret_sends_post_to_enable_route() -> None:
    """Profile TFA secret creation sends POST with no body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "secret": "5FXX6KLACOC33GTC",
        "expiry": "2026-01-01T00:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.create_profile_tfa_secret()

    assert result == {
        "secret": "5FXX6KLACOC33GTC",
        "expiry": "2026-01-01T00:00:00",
    }
    mock_request.assert_called_once_with("POST", "/profile/tfa-enable")
    await client.close()


async def test_retryable_create_profile_tfa_secret_delegates_to_client() -> None:
    """Retryable profile TFA secret creation delegates to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "create_profile_tfa_secret", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"secret": "5FXX6KLACOC33GTC"}
        result = await client.create_profile_tfa_secret()

    assert result == {"secret": "5FXX6KLACOC33GTC"}
    mock_create.assert_awaited_once_with()
    await client.close()


async def test_create_profile_tfa_secret_wraps_http_errors() -> None:
    """Profile TFA secret creation maps HTTP errors to CreateProfileTFASecret."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.create_profile_tfa_secret()

    assert exc_info.value.operation == "CreateProfileTFASecret"
    await client.close()


async def test_confirm_profile_tfa_enable_sends_post_to_confirm_route() -> None:
    """Profile TFA enable confirm sends POST with documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "scratch": "setup-token",
        "expiry": "2026-01-01T00:00:00",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.confirm_profile_tfa_enable("123456")

    assert result == {
        "scratch": "setup-token",
        "expiry": "2026-01-01T00:00:00",
    }
    mock_request.assert_called_once_with(
        "POST", "/profile/tfa-enable-confirm", {"tfa_code": "123456"}
    )
    await client.close()


async def test_confirm_profile_tfa_enable_allows_empty_body_when_code_omitted() -> None:
    """Profile TFA enable confirm can omit the optional tfa_code body field."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.confirm_profile_tfa_enable()

    mock_request.assert_called_once_with("POST", "/profile/tfa-enable-confirm", {})
    await client.close()


async def test_retryable_confirm_profile_tfa_enable_delegates_to_client() -> None:
    """Retryable profile TFA enable confirm forwards the body field."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "confirm_profile_tfa_enable", new_callable=AsyncMock
    ) as mock_confirm:
        mock_confirm.return_value = {"scratch": "setup-token"}
        result = await client.confirm_profile_tfa_enable("123456")

    assert result == {"scratch": "setup-token"}
    mock_confirm.assert_awaited_once_with("123456")
    await client.close()


async def test_confirm_profile_tfa_enable_wraps_http_errors() -> None:
    """Profile TFA enable confirm maps HTTP errors to ConfirmProfileTFAEnable."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.confirm_profile_tfa_enable("123456")

    assert exc_info.value.operation == "ConfirmProfileTFAEnable"
    await client.close()


async def test_send_profile_phone_number_verification_sends_post_with_body() -> None:
    """Profile phone number send posts the documented verification body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.send_profile_phone_number_verification(
            "US", "+15551234567"
        )

    assert result == {}
    mock_request.assert_called_once_with(
        "POST",
        "/profile/phone-number",
        {"iso_code": "US", "phone_number": "+15551234567"},
    )
    await client.close()


async def test_retryable_send_profile_phone_number_verification_delegates() -> None:
    """Retryable profile phone send forwards the country code and number."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "send_profile_phone_number_verification", new_callable=AsyncMock
    ) as mock_send:
        mock_send.return_value = {}
        result = await client.send_profile_phone_number_verification(
            "US", "+15551234567"
        )

    assert result == {}
    mock_send.assert_awaited_once_with("US", "+15551234567")
    await client.close()


async def test_send_profile_phone_number_verification_wraps_http_errors() -> None:
    """Profile phone number send maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.send_profile_phone_number_verification("US", "+15551234567")

    assert exc_info.value.operation == "SendProfilePhoneNumberVerification"
    await client.close()


async def test_verify_profile_phone_number_sends_post_with_otp_code() -> None:
    """Profile phone number verification sends POST with documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.verify_profile_phone_number("123456")

    assert result == {}
    mock_request.assert_called_once_with(
        "POST", "/profile/phone-number/verify", {"otp_code": "123456"}
    )
    await client.close()


async def test_delete_profile_phone_number_sends_delete_to_phone_number_route() -> None:
    """Profile phone number delete sends DELETE with no body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.delete_profile_phone_number()

    assert result == {}
    mock_request.assert_called_once_with("DELETE", "/profile/phone-number")
    await client.close()


async def test_retryable_delete_profile_phone_number_delegates_to_client() -> None:
    """Retryable profile phone deletion forwards to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "delete_profile_phone_number", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.return_value = {}
        result = await client.delete_profile_phone_number()

    assert result == {}
    mock_delete.assert_awaited_once_with()
    await client.close()


async def test_delete_profile_phone_number_wraps_http_errors() -> None:
    """Profile phone number deletion maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.delete_profile_phone_number()

    assert exc_info.value.operation == "DeleteProfilePhoneNumber"
    await client.close()


async def test_retryable_verify_profile_phone_number_delegates_to_client() -> None:
    """Retryable profile phone verification forwards the one-time code."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "verify_profile_phone_number", new_callable=AsyncMock
    ) as mock_verify:
        mock_verify.return_value = {}
        result = await client.verify_profile_phone_number("123456")

    assert result == {}
    mock_verify.assert_awaited_once_with("123456")
    await client.close()


async def test_verify_profile_phone_number_wraps_http_errors() -> None:
    """Profile phone number verification maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.verify_profile_phone_number("123456")

    assert exc_info.value.operation == "VerifyProfilePhoneNumber"
    await client.close()


async def test_disable_profile_tfa_sends_post_to_disable_route() -> None:
    """Profile TFA disable sends POST with no body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.disable_profile_tfa()

    assert result == {}
    mock_request.assert_called_once_with("POST", "/profile/tfa-disable")
    await client.close()


async def test_retryable_disable_profile_tfa_delegates_to_client() -> None:
    """Retryable profile TFA disable delegates to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "disable_profile_tfa", new_callable=AsyncMock
    ) as mock_disable:
        mock_disable.return_value = {}
        result = await client.disable_profile_tfa()

    assert result == {}
    mock_disable.assert_awaited_once_with()
    await client.close()


async def test_disable_profile_tfa_wraps_http_errors() -> None:
    """Profile TFA disable maps HTTP errors to DisableProfileTFA."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.disable_profile_tfa()

    assert exc_info.value.operation == "DisableProfileTFA"
    await client.close()


async def test_list_profile_security_questions_sends_get_to_route() -> None:
    """Profile security questions list sends GET to the documented route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "security_questions": [
            {"id": 1, "question": "In what city were you born?"},
            {"id": 2, "question": "What was your first pet's name?"},
        ]
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.list_profile_security_questions()

    assert result == {
        "security_questions": [
            {"id": 1, "question": "In what city were you born?"},
            {"id": 2, "question": "What was your first pet's name?"},
        ]
    }
    mock_request.assert_called_once_with("GET", "/profile/security-questions")
    await client.close()


async def test_retryable_list_profile_security_questions_delegates_to_client() -> None:
    """Retryable profile security questions listing delegates to Client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_profile_security_questions", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"security_questions": []}
        result = await client.list_profile_security_questions()

    assert result == {"security_questions": []}
    mock_list.assert_awaited_once_with()
    await client.close()


async def test_list_profile_security_questions_wraps_http_errors() -> None:
    """Profile security questions list maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.list_profile_security_questions()

    assert exc_info.value.operation == "ListProfileSecurityQuestions"
    await client.close()


async def test_answer_profile_security_questions_sends_post_to_route() -> None:
    """Profile security questions sends POST with documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"security_questions": []}
    questions = [
        {"question_id": 1, "response": "Gotham City", "security_question": "ignored"},
        {"question_id": 2, "response": "Blue"},
        {"question_id": 3, "response": "Pizza"},
    ]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.answer_profile_security_questions(questions)

    assert result == {"security_questions": []}
    mock_request.assert_called_once_with(
        "POST",
        "/profile/security-questions",
        {
            "security_questions": [
                {"question_id": 1, "response": "Gotham City"},
                {"question_id": 2, "response": "Blue"},
                {"question_id": 3, "response": "Pizza"},
            ]
        },
    )
    await client.close()


async def test_answer_profile_security_questions_validates_before_request() -> None:
    """Profile security questions validates documented body fields first."""
    client = Client("https://api.linode.com/v4", "test-token")

    invalid_calls: tuple[object, ...] = (
        [],
        "not-a-list",
        [
            "not-an-object",
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": 0, "response": "Blue"},
            {"question_id": 2, "response": "Green"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": True, "response": "Blue"},
            {"question_id": 2, "response": "Green"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": 1, "response": "no"},
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": 1, "response": "x" * 18},
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": 1},
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": 1, "response": None},
            {"question_id": 2, "response": "Blue"},
            {"question_id": 3, "response": "Pizza"},
        ],
        [
            {"question_id": 1, "response": "Blue"},
            {"question_id": 1, "response": "Red"},
            {"question_id": 3, "response": "Pizza"},
        ],
    )
    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        for security_questions in invalid_calls:
            with pytest.raises((TypeError, ValueError)):
                await client.answer_profile_security_questions(security_questions)  # type: ignore[arg-type]

    mock_request.assert_not_called()
    await client.close()


async def test_retryable_answer_security_questions_delegates_to_client() -> None:
    """Retryable profile security questions forwards the body list."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    questions = [{"question_id": 1, "response": "Gotham City"}]

    with patch.object(
        client.client, "answer_profile_security_questions", new_callable=AsyncMock
    ) as mock_answer:
        mock_answer.return_value = {"security_questions": []}
        result = await client.answer_profile_security_questions(questions)

    assert result == {"security_questions": []}
    mock_answer.assert_awaited_once_with(questions)
    await client.close()


async def test_answer_profile_security_questions_wraps_http_errors() -> None:
    """Profile security questions maps HTTP errors to operation name."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.answer_profile_security_questions(
                [
                    {"question_id": 1, "response": "Gotham City"},
                    {"question_id": 2, "response": "Blue"},
                    {"question_id": 3, "response": "Pizza"},
                ]
            )

    assert exc_info.value.operation == "AnswerProfileSecurityQuestions"
    await client.close()


async def test_create_profile_token_sends_post_to_profile_tokens_route() -> None:
    """Profile token create sends POST /profile/tokens with documented body."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "id": 12345,
        "label": "api-token",
        "scopes": "linodes:read_only",
        "expiry": "2026-01-01T00:00:00",
        "token": "abcdefghijklmnop",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.create_profile_token(
            expiry="2026-01-01T00:00:00",
            label="api-token",
            scopes="linodes:read_only",
        )

    assert result == {
        "id": 12345,
        "label": "api-token",
        "scopes": "linodes:read_only",
        "expiry": "2026-01-01T00:00:00",
        "token": "abcdefghijklmnop",
    }
    mock_request.assert_called_once_with(
        "POST",
        "/profile/tokens",
        {
            "expiry": "2026-01-01T00:00:00",
            "label": "api-token",
            "scopes": "linodes:read_only",
        },
    )
    await client.close()


async def test_create_profile_token_omits_unspecified_body_fields() -> None:
    """Profile token create omits unset optional body fields."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "label": "api-token"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.create_profile_token(label="api-token")

    mock_request.assert_called_once_with(
        "POST", "/profile/tokens", {"label": "api-token"}
    )
    await client.close()


async def test_create_profile_token_validates_inputs_before_request() -> None:
    """Profile token create validates documented body fields before POST."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        invalid_calls = (
            {"label": ""},
            {"label": "   "},
            {"label": "x" * 101},
            {"scopes": ""},
            {"expiry": "not-a-date"},
            {"expiry": ""},
        )
        for kwargs in invalid_calls:
            with pytest.raises(ValueError, match="must be"):
                await client.create_profile_token(**kwargs)

    mock_request.assert_not_called()
    await client.close()


async def test_retryable_create_profile_token_preserves_optional_arguments() -> None:
    """Retryable profile token create forwards documented body fields."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "create_profile_token", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"id": 12345, "label": "api-token"}
        result = await client.create_profile_token(
            expiry="2026-01-01T00:00:00",
            label="api-token",
            scopes="linodes:read_only",
        )

    assert result == {"id": 12345, "label": "api-token"}
    mock_create.assert_awaited_once_with(
        expiry="2026-01-01T00:00:00",
        label="api-token",
        scopes="linodes:read_only",
    )
    await client.close()


async def test_create_profile_token_wraps_http_errors() -> None:
    """Profile token create maps HTTP errors to CreateProfileToken."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.create_profile_token(label="api-token")

    assert exc_info.value.operation == "CreateProfileToken"
    await client.close()


async def test_list_profile_tokens_sends_get_to_profile_tokens_route() -> None:
    """Profile token list sends GET /profile/tokens with no body or query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "data": [
            {"id": 12345, "label": "api-token"},
            {"id": 67890, "label": "ci-token"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.list_profile_tokens()

    assert result == [
        {"id": 12345, "label": "api-token"},
        {"id": 67890, "label": "ci-token"},
    ]
    mock_request.assert_called_once_with("GET", "/profile/tokens")
    await client.close()


async def test_list_profile_tokens_fetches_all_pages() -> None:
    """Profile token list fetches subsequent pages when present."""
    client = Client("https://api.linode.com/v4", "test-token")
    first_response = MagicMock()
    first_response.json.return_value = {
        "data": [{"id": 12345, "label": "api-token"}],
        "page": 1,
        "pages": 2,
        "results": 2,
    }
    second_response = MagicMock()
    second_response.json.return_value = {
        "data": [{"id": 67890, "label": "ci-token"}],
        "page": 2,
        "pages": 2,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = [first_response, second_response]
        result = await client.list_profile_tokens()

    assert result == [
        {"id": 12345, "label": "api-token"},
        {"id": 67890, "label": "ci-token"},
    ]
    assert [args.args for args in mock_request.await_args_list] == [
        ("GET", "/profile/tokens"),
        ("GET", "/profile/tokens?page=2"),
    ]
    await client.close()


async def test_list_profile_tokens_rejects_malformed_response() -> None:
    """Profile token list fails closed on malformed payloads."""
    client = Client("https://api.linode.com/v4", "test-token")

    malformed_payloads: tuple[Any, ...] = (
        None,
        [],
        {},
        {"data": "not-a-list"},
        {"data": ["not-an-object"]},
        {"data": [], "pages": "not-an-int"},
        {"data": [], "pages": True},
        {"data": [], "pages": 0},
    )
    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        for payload in malformed_payloads:
            response = MagicMock()
            response.json.return_value = payload
            mock_request.return_value = response
            with pytest.raises(
                (TypeError, ValueError), match="profile tokens response"
            ):
                await client.list_profile_tokens()

    assert mock_request.await_count == len(malformed_payloads)
    await client.close()


async def test_retryable_list_profile_tokens_delegates_to_client() -> None:
    """Retryable profile token list forwards to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_profile_tokens", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = [{"id": 12345, "label": "api-token"}]
        result = await client.list_profile_tokens()

    assert result == [{"id": 12345, "label": "api-token"}]
    mock_list.assert_awaited_once_with()
    await client.close()


async def test_list_profile_tokens_wraps_http_errors() -> None:
    """Profile token list maps HTTP errors to ListProfileTokens."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.list_profile_tokens()

    assert exc_info.value.operation == "ListProfileTokens"
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


async def test_list_profile_logins_sends_get_to_profile_logins_route() -> None:
    """Profile login list sends GET /profile/logins with no body or query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "data": [
            {"id": 12345, "ip": "192.0.2.10"},
            {"id": 67890, "ip": "192.0.2.11"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.list_profile_logins()

    assert result == [
        {"id": 12345, "ip": "192.0.2.10"},
        {"id": 67890, "ip": "192.0.2.11"},
    ]
    mock_request.assert_called_once_with("GET", "/profile/logins")
    await client.close()


async def test_list_profile_logins_fetches_all_pages() -> None:
    """Profile login list fetches subsequent pages when present."""
    client = Client("https://api.linode.com/v4", "test-token")
    first_response = MagicMock()
    first_response.json.return_value = {
        "data": [{"id": 12345, "ip": "192.0.2.10"}],
        "page": 1,
        "pages": 2,
        "results": 2,
    }
    second_response = MagicMock()
    second_response.json.return_value = {
        "data": [{"id": 67890, "ip": "192.0.2.11"}],
        "page": 2,
        "pages": 2,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = [first_response, second_response]
        result = await client.list_profile_logins()

    assert result == [
        {"id": 12345, "ip": "192.0.2.10"},
        {"id": 67890, "ip": "192.0.2.11"},
    ]
    assert [args.args for args in mock_request.await_args_list] == [
        ("GET", "/profile/logins"),
        ("GET", "/profile/logins?page=2"),
    ]
    await client.close()


async def test_list_profile_logins_rejects_malformed_response() -> None:
    """Profile login list fails closed on malformed payloads."""
    client = Client("https://api.linode.com/v4", "test-token")

    malformed_payloads: tuple[Any, ...] = (
        None,
        [],
        {},
        {"data": "not-a-list"},
        {"data": ["not-an-object"]},
        {"data": [], "pages": "not-an-int"},
        {"data": [], "pages": True},
        {"data": [], "pages": 0},
    )
    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        for payload in malformed_payloads:
            response = MagicMock()
            response.json.return_value = payload
            mock_request.return_value = response
            with pytest.raises(
                (TypeError, ValueError), match="profile logins response"
            ):
                await client.list_profile_logins()

    assert mock_request.await_count == len(malformed_payloads)
    await client.close()


async def test_list_profile_logins_wraps_http_errors() -> None:
    """Profile login list maps HTTP errors to ListProfileLogins."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.list_profile_logins()

    assert exc_info.value.operation == "ListProfileLogins"
    await client.close()


async def test_retryable_list_profile_logins_delegates_to_client() -> None:
    """Retryable profile login list forwards to the base client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_profile_logins", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = [{"id": 12345, "ip": "192.0.2.10"}]
        result = await client.list_profile_logins()

    assert result == [{"id": 12345, "ip": "192.0.2.10"}]
    mock_list.assert_awaited_once_with()
    await client.close()


async def test_get_profile_login_sends_get_to_profile_login_route() -> None:
    """Profile login get sends GET /profile/logins/{loginId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "ip": "192.0.2.10"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.get_profile_login(12345)

    assert result == {"id": 12345, "ip": "192.0.2.10"}
    mock_request.assert_called_once_with("GET", "/profile/logins/12345")
    await client.close()


async def test_get_profile_login_encodes_path_parameter() -> None:
    """Profile login get path segment is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "ip": "192.0.2.10"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.get_profile_login("12/../34?x=1")  # type: ignore[arg-type]

    mock_request.assert_called_once_with("GET", "/profile/logins/12%2F..%2F34%3Fx%3D1")
    await client.close()


async def test_get_profile_login_wraps_http_errors() -> None:
    """Profile login get maps HTTP errors to GetProfileLogin."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.get_profile_login(12345)

    assert exc_info.value.operation == "GetProfileLogin"
    await client.close()


async def test_retryable_get_profile_login_delegates_to_client() -> None:
    """Retryable profile login get delegates to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "get_profile_login", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = {"id": 12345, "ip": "192.0.2.10"}
        result = await client.get_profile_login(12345)

    assert result == {"id": 12345, "ip": "192.0.2.10"}
    mock_get.assert_awaited_once_with(12345)
    await client.close()


async def test_update_profile_token_sends_put_to_profile_token_route() -> None:
    """Profile token update sends PUT /profile/tokens/{tokenId}."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "label": "new-label"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.update_profile_token(12345, label="new-label")

    assert result == {"id": 12345, "label": "new-label"}
    mock_request.assert_called_once_with(
        "PUT", "/profile/tokens/12345", {"label": "new-label"}
    )
    await client.close()


async def test_update_profile_token_encodes_path_parameter() -> None:
    """Profile token update path segment is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 12345, "label": "new-label"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.update_profile_token("12/../34?x=1", label="new-label")  # type: ignore[arg-type]

    mock_request.assert_called_once_with(
        "PUT", "/profile/tokens/12%2F..%2F34%3Fx%3D1", {"label": "new-label"}
    )
    await client.close()


async def test_update_profile_token_wraps_http_errors() -> None:
    """Profile token update maps HTTP errors to UpdateProfileToken."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.update_profile_token(12345, label="new-label")

    assert exc_info.value.operation == "UpdateProfileToken"
    await client.close()


async def test_list_profile_apps_sends_get_to_profile_apps_route() -> None:
    """Profile apps list sends GET /profile/apps."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = httpx.Response(200, json={"data": [{"id": 123}], "page": 1, "pages": 1})

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.list_profile_apps()

    assert result == {"data": [{"id": 123}], "page": 1, "pages": 1}
    mock_request.assert_awaited_once_with("GET", "/profile/apps")
    await client.close()


async def test_list_profile_apps_includes_pagination_query_params() -> None:
    """Profile apps list includes page and page_size query params."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = httpx.Response(200, json={"data": [], "page": 2, "pages": 3})

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        await client.list_profile_apps(page=2, page_size=50)

    mock_request.assert_awaited_once_with("GET", "/profile/apps?page=2&page_size=50")
    await client.close()


async def test_list_profile_apps_wraps_http_errors() -> None:
    """Profile apps list maps HTTP errors to ListProfileApps."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError, match="ListProfileApps"):
            await client.list_profile_apps()
    await client.close()


async def test_retryable_client_list_profile_apps_delegates() -> None:
    """RetryableClient delegates profile apps listing to Client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_profile_apps", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": [{"id": 123}]}
        result = await client.list_profile_apps(page=2, page_size=50)

    assert result == {"data": [{"id": 123}]}
    mock_list.assert_awaited_once_with(page=2, page_size=50)
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


async def test_delete_profile_app_sends_delete_to_profile_app_route() -> None:
    """Profile app revoke sends DELETE /profile/apps/{appId}."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_profile_app(12345)

    mock_request.assert_awaited_once_with("DELETE", "/profile/apps/12345")
    await client.close()


async def test_delete_profile_app_encodes_path_parameter() -> None:
    """Profile app path segment is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_profile_app("12/../34?x=1")  # type: ignore[arg-type]

    mock_request.assert_awaited_once_with(
        "DELETE", "/profile/apps/12%2F..%2F34%3Fx%3D1"
    )
    await client.close()


async def test_delete_profile_app_wraps_http_errors() -> None:
    """Profile app revoke maps HTTP errors to DeleteProfileApp."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.delete_profile_app(12345)

    assert exc_info.value.operation == "DeleteProfileApp"
    await client.close()


async def test_retryable_client_delete_profile_app_delegates() -> None:
    """Retryable profile app revoke delegates to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "delete_profile_app", new_callable=AsyncMock
    ) as mock_delete:
        await client.delete_profile_app(12345)

    mock_delete.assert_awaited_once_with(12345)
    await client.close()


async def test_delete_profile_token_sends_delete_to_profile_token_route() -> None:
    """Profile token revoke sends DELETE /profile/tokens/{tokenId}."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_profile_token(12345)

    mock_request.assert_called_once_with("DELETE", "/profile/tokens/12345")
    await client.close()


async def test_delete_profile_token_encodes_path_parameter() -> None:
    """Profile token path segment is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_profile_token("12/../34?x=1")  # type: ignore[arg-type]

    mock_request.assert_called_once_with(
        "DELETE", "/profile/tokens/12%2F..%2F34%3Fx%3D1"
    )
    await client.close()


async def test_list_profile_devices_sends_get_to_profile_devices_route() -> None:
    """Profile trusted device list sends GET /profile/devices."""
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {
        "data": [
            {"id": 123, "user_agent": "Mozilla/5.0"},
            {"id": 456, "user_agent": "curl/8.0"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = response
        result = await client.list_profile_devices()

    assert result == [
        {"id": 123, "user_agent": "Mozilla/5.0"},
        {"id": 456, "user_agent": "curl/8.0"},
    ]
    mock_request.assert_awaited_once_with("GET", "/profile/devices")
    await client.close()


async def test_list_profile_devices_fetches_all_pages() -> None:
    """Profile trusted device list fetches subsequent pages when present."""
    client = Client("https://api.linode.com/v4", "test-token")
    first_response = MagicMock()
    first_response.json.return_value = {
        "data": [{"id": 123, "user_agent": "Mozilla/5.0"}],
        "page": 1,
        "pages": 2,
        "results": 2,
    }
    second_response = MagicMock()
    second_response.json.return_value = {
        "data": [{"id": 456, "user_agent": "curl/8.0"}],
        "page": 2,
        "pages": 2,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = [first_response, second_response]
        result = await client.list_profile_devices()

    assert result == [
        {"id": 123, "user_agent": "Mozilla/5.0"},
        {"id": 456, "user_agent": "curl/8.0"},
    ]
    assert [args.args for args in mock_request.await_args_list] == [
        ("GET", "/profile/devices"),
        ("GET", "/profile/devices?page=2"),
    ]
    await client.close()


async def test_list_profile_devices_rejects_malformed_response() -> None:
    """Profile trusted device list fails closed on malformed payloads."""
    client = Client("https://api.linode.com/v4", "test-token")
    malformed_payloads: tuple[Any, ...] = (
        None,
        [],
        {},
        {"data": "not-a-list"},
        {"data": ["not-an-object"]},
        {"data": [], "pages": "not-an-int"},
        {"data": [], "pages": True},
        {"data": [], "pages": 0},
    )

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        for payload in malformed_payloads:
            response = MagicMock()
            response.json.return_value = payload
            mock_request.return_value = response
            with pytest.raises(
                (TypeError, ValueError), match="profile devices response"
            ):
                await client.list_profile_devices()

    assert mock_request.await_count == len(malformed_payloads)
    await client.close()


async def test_retryable_client_list_profile_devices_delegates() -> None:
    """Retryable profile trusted device list delegates to the client."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "list_profile_devices", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = [{"id": 123}]
        result = await client.list_profile_devices()

    assert result == [{"id": 123}]
    mock_list.assert_awaited_once_with()
    await client.close()


async def test_list_profile_devices_wraps_http_errors() -> None:
    """Profile trusted device list maps HTTP errors to ListProfileDevices."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ReadTimeout("timeout")
        with pytest.raises(NetworkError) as exc_info:
            await client.list_profile_devices()

    assert exc_info.value.operation == "ListProfileDevices"
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


async def test_delete_profile_device_uses_delete_method_and_encoded_path() -> None:
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_profile_device(123)

    mock_request.assert_awaited_once_with("DELETE", "/profile/devices/123")
    await client.close()


async def test_delete_profile_device_encodes_path_parameter() -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    unsafe_device_id: Any = "12/../34?x=1"

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.delete_profile_device(unsafe_device_id)

    mock_request.assert_awaited_once_with(
        "DELETE", "/profile/devices/12%2F..%2F34%3Fx%3D1"
    )
    await client.close()


async def test_retryable_client_delete_profile_device_delegates() -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        client.client, "delete_profile_device", new_callable=AsyncMock
    ) as mock_delete:
        await client.delete_profile_device(123)

    mock_delete.assert_awaited_once_with(123)
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


async def test_delete_firewall_device() -> None:
    """Test deleting a firewall device."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.delete_firewall_device(12345, 456)

        mock_request.assert_awaited_once_with(
            "DELETE", "/networking/firewalls/12345/devices/456"
        )

    await client.close()


async def test_delete_firewall_device_encodes_path_params() -> None:
    """Test delete_firewall_device URL-encodes both path params."""
    from urllib.parse import quote

    unsafe_firewall_id: Any = "12345/../?x=1"
    unsafe_device_id: Any = "456/../../../etc/passwd"

    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.delete_firewall_device(unsafe_firewall_id, unsafe_device_id)

        expected = (
            f"/networking/firewalls/{quote(str(unsafe_firewall_id), safe='')}/"
            f"devices/{quote(str(unsafe_device_id), safe='')}"
        )
        mock_request.assert_awaited_once_with("DELETE", expected)

    await client.close()


async def test_delete_firewall_device_wraps_http_errors() -> None:
    """Test delete_firewall_device wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_firewall_device(12345, 456)

    assert "DeleteFirewallDevice" in str(excinfo.value)
    await client.close()


async def test_retryable_delete_firewall_device_delegates_once_without_retry() -> None:
    """Test destructive firewall device delete is not replayed by retry logic."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_firewall_device", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.side_effect = NetworkError(
            "DeleteFirewallDevice", httpx.HTTPError("boom")
        )

        with pytest.raises(NetworkError):
            await retryable.delete_firewall_device(12345, 456)

    mock_delete.assert_awaited_once_with(12345, 456)
    await retryable.close()


async def test_create_firewall_device() -> None:
    """Test creating a firewall device."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 456,
        "entity": {"id": 123, "type": "linode", "label": "linode-123"},
        "created": "2018-01-01T01:01:01",
        "updated": "2018-01-01T01:01:01",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.create_firewall_device(12345, 123, "linode")

        assert result["id"] == 456
        assert result["entity"]["id"] == 123
        mock_request.assert_called_once_with(
            "POST", "/networking/firewalls/12345/devices", {"id": 123, "type": "linode"}
        )

    await client.close()


async def test_create_firewall_device_encodes_firewall_id() -> None:
    """Test firewall_id path param is URL-encoded."""
    from urllib.parse import quote

    unsafe_firewall_id: Any = "12345/../etc/passwd"

    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"id": 456}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.create_firewall_device(unsafe_firewall_id, 123, "linode")

        call_args = mock_request.call_args
        assert quote(str(unsafe_firewall_id), safe="") in call_args[0][1]

    await client.close()


@pytest.mark.parametrize(
    ("firewall_id", "device_id", "device_type"),
    [
        (0, 123, "linode"),
        (-1, 123, "linode"),
        (12345, 0, "linode"),
        (12345, -1, "linode"),
        (12345, 123, ""),
        (12345, 123, " "),
    ],
)
async def test_create_firewall_device_rejects_invalid_params(
    firewall_id: int, device_id: int, device_type: str
) -> None:
    """Test create_firewall_device rejects invalid parameters."""
    client = Client("https://api.linode.com/v4", "test-token")

    with pytest.raises(ValueError, match=r"positive integer|non-empty string"):
        await client.create_firewall_device(firewall_id, device_id, device_type)

    await client.close()


@pytest.mark.parametrize(
    ("device_id", "device_type"),
    [
        (cast("Any", "123"), "linode"),
        (123, cast("Any", 123)),
    ],
)
async def test_create_firewall_device_rejects_invalid_param_types(
    device_id: Any, device_type: Any
) -> None:
    """Test create_firewall_device rejects invalid parameter types."""
    client = Client("https://api.linode.com/v4", "test-token")

    with pytest.raises(ValueError, match=r"positive integer|non-empty string"):
        await client.create_firewall_device(12345, device_id, device_type)

    await client.close()


async def test_create_firewall_device_wraps_http_errors() -> None:
    """Test create_firewall_device wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_firewall_device(12345, 123, "linode")

    assert "CreateFirewallDevice" in str(excinfo.value)
    await client.close()


async def test_retryable_create_firewall_device_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall device creation to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "create_firewall_device", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = {"id": 456}
        result = await retryable.create_firewall_device(12345, 123, "linode")

    assert result == {"id": 456}
    mock_create.assert_awaited_once_with(12345, 123, "linode")
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


async def test_list_firewall_templates() -> None:
    """Test listing firewall templates."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [
            {
                "slug": "allow-http",
                "label": "Allow HTTP",
                "description": "Allow HTTP traffic on port 80",
                "rules": {
                    "inbound": [],
                    "outbound": [],
                    "inbound_policy": "DROP",
                    "outbound_policy": "ACCEPT",
                },
            }
        ],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        result = await client.list_firewall_templates()

    assert result["results"] == 1
    assert result["data"][0]["slug"] == "allow-http"
    mock_request.assert_awaited_once_with("GET", "/networking/firewalls/templates")
    await client.close()


async def test_list_firewall_templates_with_pagination() -> None:
    """Test listing firewall templates with pagination parameters."""
    client = Client("https://api.linode.com/v4", "test-token")
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": [], "page": 2, "pages": 5, "results": 0}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response
        result = await client.list_firewall_templates(page=2, page_size=25)

    assert result["page"] == 2
    mock_request.assert_awaited_once_with(
        "GET", "/networking/firewalls/templates?page=2&page_size=25"
    )
    await client.close()


async def test_list_firewall_templates_wraps_http_errors() -> None:
    """Test list_firewall_templates wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.list_firewall_templates()

    assert "ListFirewallTemplates" in str(excinfo.value)
    await client.close()


async def test_retryable_list_firewall_templates_delegates_to_client() -> None:
    """Test RetryableClient delegates firewall template list to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_firewall_templates", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = {"data": []}
        result = await retryable.list_firewall_templates(page=2, page_size=25)

    assert result == {"data": []}
    mock_list.assert_awaited_once_with(2, 25)
    await retryable.close()


async def test_get_firewall_template() -> None:
    """Test getting a firewall template by slug."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "slug": "allow-http",
        "label": "Allow HTTP",
        "description": "Allow HTTP traffic on port 80",
        "rules": {
            "inbound": [],
            "outbound": [],
            "inbound_policy": "DROP",
            "outbound_policy": "ACCEPT",
        },
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        template = await client.get_firewall_template("allow-http")

        assert template.slug == "allow-http"
        assert template.label == "Allow HTTP"
        assert template.description == "Allow HTTP traffic on port 80"
        mock_request.assert_awaited_once_with(
            "GET", "/networking/firewalls/templates/allow-http"
        )

    await client.close()


async def test_get_firewall_template_encodes_slug() -> None:
    """Test that the slug is URL-encoded in the request path."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "slug": "my template/special?chars",
        "label": "Special",
        "description": "Test",
        "rules": {
            "inbound": [],
            "outbound": [],
            "inbound_policy": "DROP",
            "outbound_policy": "ACCEPT",
        },
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_firewall_template("my template/special?chars")

        call_args = mock_request.await_args
        assert call_args is not None
        # The slug should be URL-encoded
        assert "%2F" in call_args[0][1] or "my%20template" in call_args[0][1]

    await client.close()


async def test_get_firewall_template_with_pagination() -> None:
    """Test that page/page_size are passed as query params."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "slug": "allow-http",
        "label": "Allow HTTP",
        "description": "Allow HTTP traffic",
        "rules": {
            "inbound": [],
            "outbound": [],
            "inbound_policy": "DROP",
            "outbound_policy": "ACCEPT",
        },
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.get_firewall_template("allow-http", page=1, page_size=10)

        call_args = mock_request.await_args
        assert call_args is not None
        endpoint = call_args[0][1]
        assert "page=1" in endpoint
        assert "page_size=10" in endpoint

    await client.close()


async def test_get_firewall_template_wraps_http_errors() -> None:
    """Test that HTTP errors are wrapped as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Not Found", request=MagicMock(), response=MagicMock(status_code=404)
        )

        with pytest.raises(NetworkError, match="GetFirewallTemplate"):
            await client.get_firewall_template("nonexistent")

    await client.close()


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


async def test_update_nodebalancer_config() -> None:
    """Test updating a NodeBalancer config."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "id": 6,
        "nodebalancer_id": 8,
        "port": 443,
        "protocol": "https",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_nodebalancer_config(
            8, 6, {"port": 443, "protocol": "https"}
        )

        assert result == {
            "id": 6,
            "nodebalancer_id": 8,
            "port": 443,
            "protocol": "https",
        }
        mock_request.assert_called_once_with(
            "PUT", "/nodebalancers/8/configs/6", {"port": 443, "protocol": "https"}
        )

    await client.close()


@pytest.mark.parametrize(
    ("nodebalancer_id", "config_id", "encoded_nodebalancer_id", "encoded_config_id"),
    [
        (8, 6, "8", "6"),
        (999, 42, "999", "42"),
    ],
)
async def test_update_nodebalancer_config_encodes_path_params(
    nodebalancer_id: int,
    config_id: int,
    encoded_nodebalancer_id: str,
    encoded_config_id: str,
) -> None:
    """NodeBalancer config update path parameters are URL-encoded."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_nodebalancer_config(
            nodebalancer_id,
            config_id,
            {"port": 80},
        )

        mock_request.assert_called_once_with(
            "PUT",
            (f"/nodebalancers/{encoded_nodebalancer_id}/configs/{encoded_config_id}"),
            {"port": 80},
        )

    await client.close()


async def test_update_nodebalancer_config_rejects_invalid_ids() -> None:
    """NodeBalancer config update rejects non-positive-integer IDs."""
    client = Client("https://api.linode.com/v4", "test-token")

    for bad_nb, bad_cfg in [
        (0, 6),
        (-1, 6),
        (True, 6),
        ("1/2", 6),
        ("..", 6),
        (8, 0),
        (8, -1),
        (8, True),
        (8, "4/5"),
        (8, ".."),
    ]:
        with pytest.raises(ValueError, match="must be a positive integer"):
            await client.update_nodebalancer_config(
                cast("Any", bad_nb), cast("Any", bad_cfg), {}
            )

    await client.close()


async def test_update_nodebalancer_config_wraps_http_errors() -> None:
    """Test updating a NodeBalancer config wraps HTTP errors."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_nodebalancer_config(8, 6, {"port": 80})

    assert "UpdateNodeBalancerConfig" in str(excinfo.value)
    await client.close()


async def test_retryable_update_nodebalancer_config_does_not_replay() -> None:
    """RetryableClient delegates config update once without retry."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_nodebalancer_config", new_callable=AsyncMock
    ) as mock_update:
        mock_update.side_effect = httpx.HTTPError("transient")
        with pytest.raises(httpx.HTTPError):
            await retryable.update_nodebalancer_config(8, 6, {"port": 80})

    mock_update.assert_awaited_once_with(8, 6, {"port": 80})
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
    assert call_args[0][1] == "/networking/ips/2001%3Adb8%3A%3A1"

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


async def test_list_networking_ips_sends_get_to_networking_ips_route() -> None:
    """Client.list_networking_ips sends GET /networking/ips."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "data": [{"address": "198.51.100.5", "type": "ipv4"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_networking_ips()

    assert result == [{"address": "198.51.100.5", "type": "ipv4"}]
    mock_request.assert_awaited_once_with("GET", "/networking/ips")

    await client.close()


async def test_list_networking_ips_sends_skip_ipv6_rdns_query() -> None:
    """Client.list_networking_ips passes skip_ipv6_rdns query when true."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"data": []}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.list_networking_ips(skip_ipv6_rdns=True)

    assert result == []
    mock_request.assert_awaited_once_with("GET", "/networking/ips?skip_ipv6_rdns=true")

    await client.close()


async def test_list_networking_ips_fetches_all_pages() -> None:
    """Client.list_networking_ips follows pagination until all pages are read."""
    client = Client("https://api.linode.com/v4", "test-token")

    first_response = MagicMock()
    first_response.status_code = 200
    first_response.json.return_value = {
        "data": [{"address": "198.51.100.5", "type": "ipv4"}],
        "page": 1,
        "pages": 2,
        "results": 2,
    }
    second_response = MagicMock()
    second_response.status_code = 200
    second_response.json.return_value = {
        "data": [{"address": "2001:db8::1", "type": "ipv6"}],
        "page": 2,
        "pages": 2,
        "results": 2,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = [first_response, second_response]

        result = await client.list_networking_ips(skip_ipv6_rdns=True)

    assert result == [
        {"address": "198.51.100.5", "type": "ipv4"},
        {"address": "2001:db8::1", "type": "ipv6"},
    ]
    assert mock_request.await_args_list[0].args == (
        "GET",
        "/networking/ips?skip_ipv6_rdns=true",
    )
    assert mock_request.await_args_list[1].args == (
        "GET",
        "/networking/ips?skip_ipv6_rdns=true&page=2",
    )

    await client.close()


async def test_list_networking_ips_wraps_http_errors() -> None:
    """Listing networking IPs wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Server error",
            request=MagicMock(),
            response=MagicMock(status_code=500),
        )

        with pytest.raises(NetworkError) as exc_info:
            await client.list_networking_ips()

    assert "ListNetworkingIPs" in str(exc_info.value)

    await client.close()


async def test_retryable_list_networking_ips_delegates_to_client() -> None:
    """RetryableClient delegates list_networking_ips to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "list_networking_ips", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = [{"address": "198.51.100.5"}]
        result = await retryable.list_networking_ips(skip_ipv6_rdns=True)

    assert result == [{"address": "198.51.100.5"}]
    mock_list.assert_awaited_once_with(True)
    await retryable.close()


async def test_allocate_networking_ip_sends_post_to_networking_ips_route() -> None:
    """Allocating a networking IP sends POST to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "address": "198.51.100.10",
        "linode_id": 12345,
        "type": "ipv4",
        "public": True,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.allocate_networking_ip(
            12345,
            ip_type="ipv4",
            public=True,
        )

    assert result["address"] == "198.51.100.10"
    mock_request.assert_called_once_with(
        "POST",
        "/networking/ips",
        {"linode_id": 12345, "type": "ipv4", "public": True},
    )

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


async def test_update_networking_ip_sends_put_to_networking_ips_route() -> None:
    """Updating networking IP RDNS sends PUT to the exact route."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "address": "198.51.100.5",
        "rdns": "example.example.com",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_networking_ip(
            "198.51.100.5",
            "example.example.com",
        )

    assert result["rdns"] == "example.example.com"
    mock_request.assert_called_once_with(
        "PUT",
        "/networking/ips/198.51.100.5",
        {"rdns": "example.example.com"},
    )

    await client.close()


async def test_update_networking_ip_url_encodes_address() -> None:
    """Path param address is URL-encoded at the client boundary."""
    client = Client("https://api.linode.com/v4", "test-token")

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"address": "2001:db8::1", "rdns": None}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.update_networking_ip(
            "2001:db8::1",
            None,
        )

    # IPv6 colons should be percent-encoded
    call_args = mock_request.call_args
    assert call_args[0][1] == "/networking/ips/2001%3Adb8%3A%3A1"

    await client.close()


async def test_retryable_update_networking_ip_delegates_to_client() -> None:
    """RetryableClient delegates update_networking_ip to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "update_networking_ip", new_callable=AsyncMock
    ) as mock_update:
        mock_update.return_value = {"address": "10.0.0.1", "rdns": "host.example.com"}
        result = await retryable.update_networking_ip("10.0.0.1", "host.example.com")

    assert result["rdns"] == "host.example.com"
    mock_update.assert_awaited_once_with("10.0.0.1", "host.example.com")
    await retryable.close()


async def test_allocate_networking_ip_wraps_http_errors() -> None:
    """Allocating a networking IP wraps HTTP errors with operation context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPStatusError(
            "Server error",
            request=MagicMock(),
            response=MagicMock(status_code=500),
        )

        with pytest.raises(NetworkError) as exc_info:
            await client.allocate_networking_ip(12345, "ipv4")

    assert "AllocateNetworkingIP" in str(exc_info.value)

    await client.close()


async def test_retryable_allocate_networking_ip_delegates_to_client() -> None:
    """RetryableClient delegates allocate_networking_ip to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "allocate_networking_ip", new_callable=AsyncMock
    ) as mock_allocate:
        mock_allocate.return_value = {"address": "198.51.100.10", "linode_id": 12345}
        result = await retryable.allocate_networking_ip(12345, "ipv4", public=True)

    assert result["address"] == "198.51.100.10"
    mock_allocate.assert_awaited_once_with(12345, "ipv4", True)
    await retryable.close()


async def test_retryable_allocate_networking_ip_retries_transient_failure() -> None:
    """RetryableClient retries allocate_networking_ip after transient NetworkError."""
    retryable = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=2, base_delay=0.01),
    )

    success_response = {"address": "198.51.100.10", "linode_id": 12345}

    with patch.object(
        retryable.client, "allocate_networking_ip", new_callable=AsyncMock
    ) as mock_allocate:
        mock_allocate.side_effect = [
            NetworkError("AllocateNetworkingIP", httpx.TimeoutException("timeout")),
            success_response,
        ]
        result = await retryable.allocate_networking_ip(12345, "ipv4", public=True)

    assert result["address"] == "198.51.100.10"
    assert mock_allocate.call_count == 2
    await retryable.close()


async def test_retryable_list_monitor_services_delegates_to_client() -> None:
    """RetryableClient delegates monitor services listing to Client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    response_data = {
        "data": [{"label": "Databases", "service_type": "dbaas"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }

    with patch.object(
        retryable.client, "list_monitor_services", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = response_data
        result = await retryable.list_monitor_services()

    assert result == response_data
    mock_list.assert_awaited_once_with()
    await retryable.close()


async def test_retryable_create_monitor_service_alert_definition_does_not_retry() -> (
    None
):
    """Mutating monitor alert definition create is not replayed on failure."""
    retryable = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=2, base_delay=0.01),
    )

    with patch.object(
        retryable.client,
        "create_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_create:
        mock_create.side_effect = NetworkError(
            "CreateMonitorServiceAlertDefinition",
            httpx.TimeoutException("timeout"),
        )
        with pytest.raises(NetworkError):
            await retryable.create_monitor_service_alert_definition(
                "dbaas",
                label="CPU high",
                severity=1,
                rule_criteria={"rules": [{"metric": "cpu_usage"}]},
                trigger_conditions={"criteria_condition": "ALL"},
                channel_ids=[10000],
            )

    mock_create.assert_awaited_once_with(
        "dbaas",
        label="CPU high",
        severity=1,
        rule_criteria={"rules": [{"metric": "cpu_usage"}]},
        trigger_conditions={"criteria_condition": "ALL"},
        channel_ids=[10000],
        description=None,
        entity_ids=None,
    )
    await retryable.close()


async def test_retryable_list_monitor_alert_channels_delegates_to_client() -> None:
    """Retryable monitor alert channels list delegates to the client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    payload = {"data": [{"id": 10000, "label": "Email Ops", "type": "email"}]}

    with patch.object(
        retryable,
        "_execute_with_retry",
        new_callable=AsyncMock,
    ) as mock_execute:
        mock_execute.return_value = payload
        result = await retryable.list_monitor_alert_channels()

    assert result == payload
    mock_execute.assert_awaited_once_with(retryable.client.list_monitor_alert_channels)
    await retryable.close()


async def test_retryable_list_monitor_alert_definitions_delegates_to_client() -> None:
    """Retryable monitor alert definitions list delegates to the client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    payload = {"data": [{"id": 12345, "label": "CPU high"}]}

    with patch.object(
        retryable,
        "_execute_with_retry",
        new_callable=AsyncMock,
    ) as mock_execute:
        mock_execute.return_value = payload
        result = await retryable.list_monitor_alert_definitions()

    assert result == payload
    mock_execute.assert_awaited_once_with(
        retryable.client.list_monitor_alert_definitions
    )
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


async def test_retryable_delete_monitor_service_alert_definition_does_not_retry() -> (
    None
):
    """Destructive monitor alert definition delete is not replayed on failure."""
    retryable = RetryableClient(
        "https://api.linode.com/v4",
        "test-token",
        RetryConfig(max_retries=2, base_delay=0.01),
    )

    with patch.object(
        retryable.client,
        "delete_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_delete:
        mock_delete.side_effect = NetworkError(
            "DeleteMonitorServiceAlertDefinition",
            httpx.TimeoutException("timeout"),
        )
        with pytest.raises(NetworkError):
            await retryable.delete_monitor_service_alert_definition("dbaas", 12345)

    mock_delete.assert_awaited_once_with("dbaas", 12345)
    await retryable.close()


async def test_monitor_alert_definition_create_tool_schema_and_handler_success() -> (
    None
):
    """Monitor alert definition create requires confirm and returns output."""
    tool, capability = create_linode_monitor_service_alert_definition_create_tool()
    assert tool.name == "linode_monitor_service_alert_definition_create"
    assert capability == Capability.Write
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["service_type"]["pattern"] == (
        "^[A-Za-z0-9_-]+$"
    )
    assert tool.inputSchema["required"] == [
        "service_type",
        "label",
        "severity",
        "rule_criteria",
        "trigger_conditions",
        "channel_ids",
        "confirm",
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
    response_payload = {"id": 67890, "label": "CPU high"}
    rule_criteria = {"rules": [{"metric": "cpu_usage"}]}
    trigger_conditions = {"criteria_condition": "ALL"}

    with patch.object(
        RetryableClient,
        "create_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_create:
        mock_create.return_value = response_payload
        result = await handle_linode_monitor_service_alert_definition_create(
            {
                "service_type": "dbaas",
                "label": "CPU high",
                "severity": 1,
                "rule_criteria": rule_criteria,
                "trigger_conditions": trigger_conditions,
                "channel_ids": [10000],
                "description": "High CPU usage",
                "entity_ids": [12345],
                "confirm": True,
            },
            cfg,
        )

    mock_create.assert_awaited_once_with(
        "dbaas",
        label="CPU high",
        severity=1,
        rule_criteria=rule_criteria,
        trigger_conditions=trigger_conditions,
        channel_ids=[10000],
        description="High CPU usage",
        entity_ids=[12345],
    )
    assert "Monitor service alert definition created for 'dbaas'" in result[0].text
    assert "CPU high" in result[0].text


@pytest.mark.parametrize("bad_confirm", [None, False, "true", 1])
async def test_monitor_alert_definition_create_requires_boolean_confirm(
    bad_confirm: object,
) -> None:
    """Handler rejects missing/non-true confirm before client call."""
    cfg = Config()
    args: dict[str, object] = {
        "service_type": "dbaas",
        "label": "CPU high",
        "severity": 1,
        "rule_criteria": {"rules": [{"metric": "cpu_usage"}]},
        "trigger_conditions": {"criteria_condition": "ALL"},
        "channel_ids": [10000],
    }
    if bad_confirm is not None:
        args["confirm"] = bad_confirm

    with patch.object(
        RetryableClient,
        "create_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_create:
        result = await handle_linode_monitor_service_alert_definition_create(
            cast("dict[str, Any]", args), cfg
        )

    mock_create.assert_not_called()
    assert result[0].text == (
        "Error: This creates a Linode Metrics alert definition. "
        "Set confirm=true to proceed."
    )


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_alert_definition_create_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "create_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_create:
        result = await handle_linode_monitor_service_alert_definition_create(
            {
                "service_type": bad_service_type,
                "label": "CPU high",
                "severity": 1,
                "rule_criteria": {"rules": [{"metric": "cpu_usage"}]},
                "trigger_conditions": {"criteria_condition": "ALL"},
                "channel_ids": [10000],
                "confirm": True,
            },
            cfg,
        )

    mock_create.assert_not_called()
    assert result[0].text == (
        "Error: service_type is required and must contain only letters, "
        "numbers, '_' or '-'"
    )


@pytest.mark.parametrize(
    ("field", "value", "message"),
    [
        ("label", "", "label is required"),
        ("severity", True, "severity must be a valid integer"),
        ("severity", 4, "severity must be one of 0, 1, 2, or 3"),
        ("rule_criteria", {}, "rule_criteria must be a non-empty object"),
        (
            "trigger_conditions",
            {},
            "trigger_conditions must be a non-empty object",
        ),
        ("channel_ids", [], "channel_ids must be a non-empty list of integers"),
        ("channel_ids", [True], "channel_ids must be a non-empty list of integers"),
        ("entity_ids", ["bad"], "entity_ids must be a non-empty list of integers"),
        ("description", 123, "description must be a string"),
    ],
)
async def test_monitor_alert_definition_create_rejects_invalid_body_fields(
    field: str, value: object, message: str
) -> None:
    """Handler rejects malformed create body fields before client construction."""
    cfg = Config()
    args: dict[str, object] = {
        "service_type": "dbaas",
        "label": "CPU high",
        "severity": 1,
        "rule_criteria": {"rules": [{"metric": "cpu_usage"}]},
        "trigger_conditions": {"criteria_condition": "ALL"},
        "channel_ids": [10000],
        "confirm": True,
    }
    args[field] = value

    with patch.object(
        RetryableClient,
        "create_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_create:
        result = await handle_linode_monitor_service_alert_definition_create(
            cast("dict[str, Any]", args), cfg
        )

    mock_create.assert_not_called()
    assert result[0].text == f"Error: {message}"


async def test_monitor_alert_definition_get_tool_schema_and_handler_success() -> None:
    """Monitor alert definition get tool is read-only and returns output."""
    tool, capability = create_linode_monitor_service_alert_definition_get_tool()
    assert tool.name == "linode_monitor_service_alert_definition_get"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]
    assert tool.inputSchema["properties"]["service_type"]["pattern"] == (
        "^[A-Za-z0-9_-]+$"
    )
    assert tool.inputSchema["required"] == ["service_type", "alert_id"]
    assert tool.inputSchema["properties"]["alert_id"]["minimum"] == 1

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
    payload = {"id": 12345, "label": "CPU high"}

    with patch.object(
        RetryableClient,
        "get_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = payload
        result = await handle_linode_monitor_service_alert_definition_get(
            {"service_type": "dbaas", "alert_id": 12345}, cfg
        )

    mock_get.assert_awaited_once_with("dbaas", 12345)
    assert "Monitor service alert definition 12345 retrieved for 'dbaas'" in (
        result[0].text
    )
    assert "CPU high" in result[0].text


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
        "Error: service_type is required and must contain only letters, "
        "numbers, '_' or '-'"
    )


@pytest.mark.parametrize(
    "bad_alert_id", [None, True, "12345", "1/2", "1?x", "..", 12.9]
)
async def test_monitor_alert_definition_get_rejects_invalid_alert_id(
    bad_alert_id: object,
) -> None:
    """Handler rejects invalid alert IDs before client construction."""
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
    assert result[0].text == "Error: alert_id must be a valid integer"


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
    assert tool.inputSchema["properties"]["confirm"]["type"] == "boolean"
    assert tool.inputSchema["properties"]["service_type"]["pattern"] == (
        "^[A-Za-z0-9_-]+$"
    )
    assert tool.inputSchema["required"] == [
        "service_type",
        "alert_id",
        "confirm",
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
        "delete_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            {"service_type": "dbaas", "alert_id": 12345, "confirm": True}, cfg
        )

    mock_delete.assert_awaited_once_with("dbaas", 12345)
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

    with patch.object(
        RetryableClient,
        "delete_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            cast("dict[str, Any]", args), cfg
        )

    mock_delete.assert_not_called()
    assert result[0].text == (
        "Error: This deletes a Linode Metrics alert definition. "
        "Set confirm=true to proceed."
    )


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_alert_definition_delete_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "delete_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            {"service_type": bad_service_type, "alert_id": 12345, "confirm": True},
            cfg,
        )

    mock_delete.assert_not_called()
    assert result[0].text == (
        "Error: service_type is required and must contain only letters, "
        "numbers, '_' or '-'"
    )


@pytest.mark.parametrize("bad_alert_id", [None, True, "12345", "not-an-int", 12.9])
async def test_monitor_alert_definition_delete_rejects_invalid_alert_id(
    bad_alert_id: object,
) -> None:
    """Handler rejects invalid alert IDs before client construction."""
    cfg = Config()
    args: dict[str, object] = {"service_type": "dbaas", "confirm": True}
    if bad_alert_id is not None:
        args["alert_id"] = bad_alert_id

    with patch.object(
        RetryableClient,
        "delete_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            cast("dict[str, Any]", args), cfg
        )

    mock_delete.assert_not_called()
    assert result[0].text == "Error: alert_id must be a valid integer"


@pytest.mark.parametrize("bad_alert_id", [0, -1])
async def test_monitor_alert_definition_delete_rejects_non_positive_alert_id(
    bad_alert_id: int,
) -> None:
    """Handler rejects non-positive alert IDs before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "delete_monitor_service_alert_definition",
        new_callable=AsyncMock,
    ) as mock_delete:
        result = await handle_linode_monitor_service_alert_definition_delete(
            {"service_type": "dbaas", "alert_id": bad_alert_id, "confirm": True},
            cfg,
        )

    mock_delete.assert_not_called()
    assert result[0].text == "Error: alert_id must be a positive integer"


async def test_monitor_global_alert_definitions_list_tool_success() -> None:
    """Global monitor alert definitions list is read-only and returns output."""
    tool, capability = create_linode_monitor_alert_definitions_list_tool()
    assert tool.name == "linode_monitor_alert_definitions_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]
    assert "required" not in tool.inputSchema

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
        "list_monitor_alert_definitions",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_alert_definitions_list({}, cfg)

    mock_list.assert_awaited_once_with()
    assert "Monitor alert definitions listed" in result[0].text
    assert "CPU Usage" in result[0].text


async def test_monitor_alert_channels_list_tool_schema_and_handler_success() -> None:
    """Monitor alert channels list tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_alert_channels_list_tool()
    assert tool.name == "linode_monitor_alert_channels_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]

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
        "list_monitor_alert_channels",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_alert_channels_list({}, cfg)

    mock_list.assert_awaited_once_with()
    assert "Monitor alert channels listed" in result[0].text
    assert "Email Ops" in result[0].text


async def test_monitor_alert_definitions_list_tool_schema_and_handler_success() -> None:
    """Monitor alert definitions list tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_service_alert_definitions_list_tool()
    assert tool.name == "linode_monitor_service_alert_definitions_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["service_type"]

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
        "list_monitor_service_alert_definitions",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_service_alert_definitions_list(
            {"service_type": "dbaas"}, cfg
        )

    mock_list.assert_awaited_once_with("dbaas")
    assert "Monitor service alert definitions listed for 'dbaas'" in result[0].text
    assert "CPU Usage" in result[0].text


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_alert_definitions_list_handler_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "list_monitor_service_alert_definitions",
        new_callable=AsyncMock,
    ) as mock_list:
        result = await handle_linode_monitor_service_alert_definitions_list(
            {"service_type": bad_service_type}, cfg
        )

    mock_list.assert_not_called()
    assert result[0].text == (
        "Error: service_type is required and must contain only letters, "
        "numbers, '_' or '-'"
    )


async def test_monitor_dashboard_get_tool_schema_and_handler_success() -> None:
    """Monitor dashboard get tool is read-only and returns output."""
    tool, capability = create_linode_monitor_dashboard_get_tool()
    assert tool.name == "linode_monitor_dashboard_get"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["dashboard_id"]
    assert tool.inputSchema["properties"]["dashboard_id"]["minimum"] == 1

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
    payload = {"id": 12345, "label": "Resource Usage"}

    with patch.object(
        RetryableClient,
        "get_monitor_dashboard",
        new_callable=AsyncMock,
    ) as mock_get:
        mock_get.return_value = payload
        result = await handle_linode_monitor_dashboard_get({"dashboard_id": 12345}, cfg)

    mock_get.assert_awaited_once_with(12345)
    assert "Monitor dashboard 12345 retrieved" in result[0].text
    assert "Resource Usage" in result[0].text


@pytest.mark.parametrize(
    "bad_dashboard_id", [None, True, "12345", "1/2", "1?x", "..", 12.9]
)
async def test_monitor_dashboard_get_rejects_invalid_dashboard_id(
    bad_dashboard_id: object,
) -> None:
    """Handler rejects invalid dashboard IDs before client construction."""
    cfg = Config()
    args: dict[str, object] = {}
    if bad_dashboard_id is not None:
        args["dashboard_id"] = bad_dashboard_id

    with patch.object(
        RetryableClient,
        "get_monitor_dashboard",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_dashboard_get(
            cast("dict[str, Any]", args), cfg
        )

    mock_get.assert_not_called()
    assert result[0].text == "Error: dashboard_id must be a valid integer"


@pytest.mark.parametrize("bad_dashboard_id", [0, -1])
async def test_monitor_dashboard_get_rejects_non_positive_dashboard_id(
    bad_dashboard_id: int,
) -> None:
    """Handler rejects non-positive dashboard IDs before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "get_monitor_dashboard",
        new_callable=AsyncMock,
    ) as mock_get:
        result = await handle_linode_monitor_dashboard_get(
            {"dashboard_id": bad_dashboard_id}, cfg
        )

    mock_get.assert_not_called()
    assert result[0].text == "Error: dashboard_id must be a positive integer"


async def test_monitor_dashboards_list_tool_schema_and_handler_success() -> None:
    """Monitor dashboards list tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_dashboards_list_tool()
    assert tool.name == "linode_monitor_dashboards_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]
    assert "required" not in tool.inputSchema

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
        "data": [{"id": 1, "label": "Resource Usage"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "list_monitor_dashboards",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_dashboards_list({}, cfg)

    mock_list.assert_awaited_once_with()
    assert "Monitor dashboards listed" in result[0].text
    assert "Resource Usage" in result[0].text


async def test_monitor_service_dashboards_tool_schema_and_handler_success() -> None:
    """Monitor service dashboards tool is read-only and returns handler output."""
    tool, capability = create_linode_monitor_service_dashboards_list_tool()
    assert tool.name == "linode_monitor_service_dashboards_list"
    assert capability == Capability.Read
    assert "confirm" not in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["service_type"]

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
        "data": [{"id": 1, "label": "Resource Usage"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    with patch.object(
        RetryableClient,
        "list_monitor_service_dashboards",
        new_callable=AsyncMock,
    ) as mock_list:
        mock_list.return_value = response_payload

        result = await handle_linode_monitor_service_dashboards_list(
            {"service_type": "dbaas"}, cfg
        )

    mock_list.assert_awaited_once_with("dbaas")
    assert "Monitor service dashboards listed for 'dbaas'" in result[0].text
    assert "Resource Usage" in result[0].text


@pytest.mark.parametrize("bad_service_type", ["", "bad/type", "bad?type", ".."])
async def test_monitor_dashboards_handler_rejects_malformed_service_type(
    bad_service_type: str,
) -> None:
    """Handler rejects unsafe service type values before client construction."""
    cfg = Config()

    with patch.object(
        RetryableClient,
        "list_monitor_service_dashboards",
        new_callable=AsyncMock,
    ) as mock_list:
        result = await handle_linode_monitor_service_dashboards_list(
            {"service_type": bad_service_type}, cfg
        )

    mock_list.assert_not_called()
    assert result[0].text == (
        "Error: service_type is required and must contain only letters, "
        "numbers, '_' or '-'"
    )


async def test_update_account_user_sends_put_to_encoded_user_route() -> None:
    """Updating an account user sends PUT to the encoded user route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_body = {"username": "new-user", "email": "new@example.com"}
    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_body

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.update_account_user(
            "old/user",
            username="new-user",
            email="new@example.com",
            restricted=False,
            ssh_keys=["ssh-rsa AAA"],
        )

    assert result == response_body
    mock_request.assert_called_once_with(
        "PUT",
        "/account/users/old%2Fuser",
        {
            "username": "new-user",
            "email": "new@example.com",
            "restricted": False,
            "ssh_keys": ["ssh-rsa AAA"],
        },
    )
    await client.close()


async def test_update_account_user_rejects_empty_body() -> None:
    """Empty account user update bodies are rejected before request dispatch."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="At least one account user field"),
    ):
        await client.update_account_user("user")

    mock_request.assert_not_called()
    await client.close()


async def test_retryable_update_account_user_delegates_once() -> None:
    """Retryable account user update delegates once to avoid replaying mutation."""
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    response_body = {"username": "new-user"}

    with patch.object(
        client.client, "update_account_user", new_callable=AsyncMock
    ) as mock_update:
        mock_update.return_value = response_body

        result = await client.update_account_user("old-user", username="new-user")

    assert result == response_body
    mock_update.assert_awaited_once_with("old-user", username="new-user")
    await client.close()


async def test_update_account_user_wraps_http_errors() -> None:
    """HTTP errors from account user updates are wrapped."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_account_user("old-user", email="new@example.com")

    assert "UpdateAccountUser" in str(excinfo.value)
    await client.close()


async def test_get_database_mysql_instance_credentials_sends_encoded_get() -> None:
    """Getting MySQL database credentials sends the documented route."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"username": "linode", "password": "secret"}

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_database_mysql_instance_credentials(
            cast("Any", "123/../../../etc/passwd")
        )

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET",
        "/databases/mysql/instances/123%2F..%2F..%2F..%2Fetc%2Fpasswd/credentials",
    )

    await client.close()


async def test_get_database_mysql_instance_credentials_wraps_http_errors() -> None:
    """Database credentials client wraps HTTP errors with route context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ConnectError("temporary failure")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_database_mysql_instance_credentials(123)

    assert "GetDatabaseMySQLInstanceCredentials" in str(exc_info.value)
    await client.close()


async def test_retryable_get_database_mysql_instance_credentials_delegates() -> None:
    """Retryable database credentials get delegates through the retry wrapper."""
    retry_client = RetryableClient("https://api.linode.com/v4", "test-token")
    base_client = AsyncMock(spec=Client)
    base_client.get_database_mysql_instance_credentials.return_value = {
        "username": "linode"
    }
    retry_client.client = base_client

    result = await retry_client.get_database_mysql_instance_credentials(123)

    assert result == {"username": "linode"}
    base_client.get_database_mysql_instance_credentials.assert_awaited_once_with(123)
    await retry_client.close()


async def test_get_database_engine_sends_encoded_get_with_query() -> None:
    """Getting a database engine sends GET with an encoded engine ID and query."""
    client = Client("https://api.linode.com/v4", "test-token")
    response_data = {"id": "mysql/8.0.26", "engine": "mysql", "version": "8.0.26"}

    mock_response = MagicMock()
    mock_response.status_code = 200
    mock_response.json.return_value = response_data

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        result = await client.get_database_engine("mysql/8.0.26", page=2, page_size=25)

    assert result == response_data
    mock_request.assert_called_once_with(
        "GET", "/databases/engines/mysql%2F8.0.26?page=2&page_size=25"
    )

    await client.close()


async def test_get_database_engine_wraps_http_errors() -> None:
    """Database engine client wraps HTTP errors with route context."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ConnectError("temporary failure")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_database_engine("mysql/8.0.26")

    assert "GetDatabaseEngine" in str(exc_info.value)
    await client.close()


async def test_retryable_get_database_engine_delegates_with_retry() -> None:
    """Retryable database engine get delegates through the retry wrapper."""
    retry_client = RetryableClient("https://api.linode.com/v4", "test-token")
    base_client = AsyncMock(spec=Client)
    base_client.get_database_engine.return_value = {"id": "postgres/17"}
    retry_client.client = base_client

    result = await retry_client.get_database_engine("postgres/17", page=1, page_size=50)

    assert result == {"id": "postgres/17"}
    base_client.get_database_engine.assert_awaited_once_with(
        "postgres/17", page=1, page_size=50
    )
    await retry_client.close()
