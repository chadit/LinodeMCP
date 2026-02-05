"""Unit tests for Linode client."""

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import (
    APIError,
    Client,
    NetworkError,
    Profile,
    RetryableClient,
    RetryConfig,
    is_retryable,
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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.get_profile()

        assert isinstance(profile, Profile)
        assert profile.username == "testuser"
        assert profile.email == "test@example.com"
        assert profile.uid == 12345

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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        instance = await client.get_instance(123456)

        assert instance.id == 123456
        assert instance.label == "test-instance"

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
            await client._make_request("GET", "/profile")

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
            await client._make_request("GET", "/profile")

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
            await client._make_request("GET", "/profile")

        assert exc_info.value.status_code == 500
        assert exc_info.value.is_server_error()

    await client.close()


async def test_network_error() -> None:
    """Test network error handling."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ConnectError("Connection failed")

        with pytest.raises(NetworkError) as exc_info:
            await client.get_profile()

        assert "GetProfile" in str(exc_info.value)

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
        client.client, "_make_request", new_callable=AsyncMock
    ) as mock_request:
        mock_request.return_value = mock_response

        profile = await client.get_profile()

        assert profile.username == "testuser"

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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        keys = await client.list_ssh_keys()

        assert len(keys) == 1
        assert keys[0].id == 1
        assert keys[0].label == "work-key"

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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        records = await client.list_domain_records(1)

        assert len(records) == 1
        assert records[0].id == 1
        assert records[0].type == "A"

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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        firewalls = await client.list_firewalls()

        assert len(firewalls) == 1
        assert firewalls[0].id == 1
        assert firewalls[0].label == "web-fw"

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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
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

    with patch.object(client, "_make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        scripts = await client.list_stackscripts()

        assert len(scripts) == 1
        assert scripts[0].id == 1
        assert scripts[0].label == "my-script"

    await client.close()
