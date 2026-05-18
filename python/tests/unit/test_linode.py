"""Unit tests for Linode client."""

import asyncio
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import (
    Account,
    APIError,
    CircuitBreaker,
    CircuitOpenError,
    Client,
    Grant,
    Grants,
    NetworkError,
    Profile,
    RateLimiter,
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
