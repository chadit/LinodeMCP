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
    parse_grants,
    parse_profile,
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
# The read the retry, breaker and limiter tests probe the client with: a real
# declared route with no path slot to fill.
PROFILE_GET_TOOL = "linode_profile_get"
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


def test_parse_profile_reads_the_pat_scope_string(
    sample_profile_data: dict[str, Any],
) -> None:
    """A PAT /profile body carries the scope string, and it lands on the dataclass.

    The scope validator reads this field instead of /profile/grants when it is
    non-empty.
    """
    profile = parse_profile({**sample_profile_data, "scopes": "linodes:read_write *"})

    assert isinstance(profile, Profile)
    assert profile.username == "testuser"
    assert profile.uid == 12345
    assert profile.scopes == "linodes:read_write *"


def test_parse_profile_leaves_scopes_empty_without_the_key(
    sample_profile_data: dict[str, Any],
) -> None:
    """An OAuth /profile body has no scopes key, and the validator relies on that
    reading as an empty string to fall back to /profile/grants.
    """
    assert parse_profile(sample_profile_data).scopes == ""


def test_parse_grants_reads_the_oauth_shape() -> None:
    """The global block, the per-resource lists and the permission strings all
    round-trip; categories the body omits default to empty lists.
    """
    grants = parse_grants(
        {
            "global": {
                "account_access": "read_write",
                "add_linodes": True,
                "add_domains": False,
                "cancel_account": False,
            },
            "linode": [{"id": 42, "label": "web-1", "permissions": "read_write"}],
            "domain": [{"id": 7, "label": "example.com", "permissions": "read_only"}],
        }
    )

    assert isinstance(grants, Grants)
    assert grants.global_.account_access == "read_write"
    assert grants.global_.add_linodes is True
    assert grants.global_.add_domains is False
    assert grants.linode == [Grant(id=42, label="web-1", permissions="read_write")]
    assert grants.domain[0].permissions == "read_only"
    assert grants.nodebalancer == []
    assert grants.image == []


def test_parse_grants_of_a_pat_payload_is_zero_valued() -> None:
    """A PAT answers /profile/grants with an empty object, or nothing usable at
    all, and either reads as a Grants with nothing granted rather than raising.
    """
    payloads: tuple[Any, ...] = ({}, None, [])
    for body in payloads:
        grants = parse_grants(body)

        assert grants.linode == []
        assert grants.global_.account_access == ""
        assert grants.global_.add_linodes is False


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


def test_deprecated_object_storage_cluster_client_methods_absent() -> None:
    """Deprecated Object Storage cluster client methods should be removed.

    The region read that replaced them is a generated tool now, so no typed
    replacement method is pinned here anymore.
    """
    assert not hasattr(Client, "list_object_storage_clusters")
    assert not hasattr(RetryableClient, "list_object_storage_clusters")
    assert not hasattr(Client, "get_object_storage_cluster")
    assert not hasattr(RetryableClient, "get_object_storage_cluster")


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

        profile = await client.route_raw(PROFILE_GET_TOOL)

        assert profile["username"] == "testuser"

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

        profile = await client.route_raw(PROFILE_GET_TOOL)

        assert profile["username"] == "testuser"
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
            await client.route_raw(PROFILE_GET_TOOL)

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


@pytest.mark.parametrize(
    "transport_error",
    [
        httpx.ReadTimeout("read timed out"),
        httpx.ConnectTimeout("connect timed out"),
        httpx.ConnectError("connection refused"),
    ],
)
def test_is_retryable_rejects_unwrapped_transport_error(
    transport_error: httpx.TransportError,
) -> None:
    """A bare httpx transport error is not retryable on its own.

    The client wraps every httpx.TransportError into NetworkError before a
    retry decision sees it, so an unwrapped one means the wrap was skipped.
    Answering True there would replay a call the client never classified.
    Go's isRetryable refuses the same way, requiring its requestError wrapper.
    """
    assert not is_retryable(transport_error)
    assert is_retryable(NetworkError("GetProfile", transport_error))


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

            profile = await client.route_raw(PROFILE_GET_TOOL)

            assert profile["username"] == "retryuser"
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
                await client.route_raw(PROFILE_GET_TOOL)

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
                await client.route_raw(PROFILE_GET_TOOL)

            assert exc_info.value.status_code == 403
            assert mock_req.call_count == 1

        await client.close()

    async def test_retry_on_network_error(self) -> None:
        """A connection failure then success retries once and succeeds.

        The transport raises httpx's own ConnectError, so the case proves the
        routed call wraps a socket failure into the class the loop replays.
        """
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

            profile = await client.route_raw(PROFILE_GET_TOOL)

            assert profile["username"] == "retryuser"
            assert call_count == 2

        await client.close()

    async def test_retry_on_read_timeout(self) -> None:
        """A read timeout then success retries once and succeeds.

        The timeout twin of the connection-failure case: httpx.ReadTimeout is
        a TransportError, so the routed call wraps it into NetworkError and the
        loop replays it. This is why the retry decision never has to name a
        timeout type of its own.
        """
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=2, base_delay=0.01),
        )

        mock_success_response = MagicMock()
        mock_success_response.status_code = 200
        mock_success_response.json.return_value = {
            "username": "timeoutuser",
            "email": "timeout@test.com",
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
                raise httpx.ReadTimeout("read timed out")
            return mock_success_response

        with patch.object(
            client.client.client, "request", new_callable=AsyncMock
        ) as mock_req:
            mock_req.side_effect = mock_request

            profile = await client.route_raw(PROFILE_GET_TOOL)

            assert profile["username"] == "timeoutuser"
            assert call_count == 2

        await client.close()

    async def test_no_retry_on_unwrapped_transport_error(self) -> None:
        """An unwrapped transport error escaping a client method is not replayed.

        Every routed client method wraps httpx.TransportError into
        NetworkError, so a bare one arriving at the retry loop means that wrap
        was skipped. The loop surfaces it on the first attempt instead of
        replaying a call nothing classified.
        """
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(max_retries=3, base_delay=0.01),
        )

        with patch.object(
            client.client, "route_raw", new_callable=AsyncMock
        ) as mock_route:
            mock_route.side_effect = httpx.ReadTimeout("read timed out")

            with pytest.raises(httpx.ReadTimeout):
                await client.route_raw(PROFILE_GET_TOOL)

            assert mock_route.call_count == 1

        await client.close()

    async def test_unwrapped_transport_error_does_not_trip_breaker(self) -> None:
        """The breaker counts NetworkError, not a bare transport exception.

        The single-attempt path feeds the same retry decision to the breaker,
        so an error the client never classified must not spend a failure slot
        that a real network failure has earned.
        """
        client = RetryableClient(
            "https://api.linode.com/v4",
            "test-token",
            RetryConfig(circuit_breaker_threshold=1, circuit_breaker_timeout=60.0),
        )

        with patch.object(
            client.client, "route_raw", new_callable=AsyncMock
        ) as mock_route:
            mock_route.side_effect = httpx.ReadTimeout("read timed out")

            with pytest.raises(httpx.ReadTimeout):
                await client.route_raw(PROFILE_GET_TOOL, retry=False)

            # A tripped breaker would answer CircuitOpenError here instead.
            mock_route.side_effect = NetworkError(
                PROFILE_GET_TOOL, httpx.ReadTimeout("read timed out")
            )
            with pytest.raises(NetworkError):
                await client.route_raw(PROFILE_GET_TOOL, retry=False)

            # The NetworkError above is the failure the breaker does count.
            with pytest.raises(CircuitOpenError):
                await client.route_raw(PROFILE_GET_TOOL, retry=False)

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
                    await client.route_raw(PROFILE_GET_TOOL)

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
                await client.route_raw(PROFILE_GET_TOOL)

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
                await client.route_raw(PROFILE_GET_TOOL)
            assert mock_req.call_count == 2

            # Second exhaustion: another 2 calls. Breaker trips after.
            with pytest.raises(APIError):
                await client.route_raw(PROFILE_GET_TOOL)
            assert mock_req.call_count == 4

            # Third call: breaker open. Upstream must NOT be touched.
            with pytest.raises(CircuitOpenError):
                await client.route_raw(PROFILE_GET_TOOL)
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
                    await client.route_raw(PROFILE_GET_TOOL)

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
                    await client.route_raw(PROFILE_GET_TOOL)

            # Success in between resets counter.
            profile = await client.route_raw(PROFILE_GET_TOOL)
            assert profile["username"] == "ok"

            # 2 more failures: still below threshold (3) thanks to reset.
            for _ in range(2):
                with pytest.raises(APIError):
                    await client.route_raw(PROFILE_GET_TOOL)

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
                await client.route_raw(PROFILE_GET_TOOL)

            assert mock_req.call_count == 60
            sleeps.clear()  # ignore any sleeps during burst

            # 61st call must wait for the limiter (~1s synthetic).
            await client.route_raw(PROFILE_GET_TOOL)
            assert mock_req.call_count == 61
            assert sleeps, "limiter must have parked the 61st call on sleep"

        await client.close()


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
