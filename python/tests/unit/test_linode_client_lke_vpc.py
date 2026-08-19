"""Behavioral tests for previously-untested LKE and VPC Client methods.

These read/delete/action methods go straight through make_request and return
the decoded JSON (list reads unwrap the "data" envelope, single reads return
the body). Each pair here asserts the HTTP method + endpoint that goes out, the
decoded return shape, and that httpx failures wrap into NetworkError with the
operation name. The whole method bodies were untested branches.
"""

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


async def test_get_lke_cluster_returns_body() -> None:
    """get_lke_cluster GETs the cluster route and returns the raw body."""
    client = Client("https://api.linode.com/v4", "test-token")
    cluster = {"id": 7, "label": "prod", "region": "us-east"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(cluster)

        result = await client.get_lke_cluster(7)

    assert result == cluster
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7")
    await client.close()


async def test_get_lke_cluster_wraps_http_errors() -> None:
    """get_lke_cluster wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_lke_cluster(7)

    assert "GetLKECluster" in str(excinfo.value)
    await client.close()


async def test_list_lke_node_pools_unwraps_data() -> None:
    """list_lke_node_pools GETs the pools route and returns the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    pools = [{"id": 100, "type": "g6-standard-1", "count": 3}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": pools})

        result = await client.list_lke_node_pools(7)

    assert result == pools
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7/pools")
    await client.close()


async def test_list_lke_node_pools_wraps_http_errors() -> None:
    """list_lke_node_pools wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_lke_node_pools(7)

    assert "ListLKENodePools" in str(excinfo.value)
    await client.close()


async def test_get_lke_node_pool_returns_body() -> None:
    """get_lke_node_pool GETs the pool route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    pool = {"id": 100, "count": 3}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(pool)

        result = await client.get_lke_node_pool(7, 100)

    assert result == pool
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7/pools/100")
    await client.close()


async def test_get_lke_node_pool_wraps_http_errors() -> None:
    """get_lke_node_pool wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_lke_node_pool(7, 100)

    assert "GetLKENodePool" in str(excinfo.value)
    await client.close()


async def test_get_lke_node_returns_body() -> None:
    """get_lke_node GETs the node route with the string node id."""
    client = Client("https://api.linode.com/v4", "test-token")
    node = {"id": "12345-abc", "status": "ready"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(node)

        result = await client.get_lke_node(7, "12345-abc")

    assert result == node
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7/nodes/12345-abc")
    await client.close()


async def test_get_lke_node_wraps_http_errors() -> None:
    """get_lke_node wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_lke_node(7, "12345-abc")

    assert "GetLKENode" in str(excinfo.value)
    await client.close()


async def test_get_vpc_returns_body() -> None:
    """get_vpc GETs the vpc route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    vpc = {"id": 3, "label": "net-c"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(vpc)

        result = await client.get_vpc(3)

    assert result == vpc
    mock_request.assert_awaited_once_with("GET", "/vpcs/3")
    await client.close()


async def test_get_vpc_wraps_http_errors() -> None:
    """get_vpc wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_vpc(3)

    assert "GetVPC" in str(excinfo.value)
    await client.close()


async def test_list_vpc_subnets_unwraps_data() -> None:
    """list_vpc_subnets GETs the subnets route and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    subnets = [{"id": 50, "label": "sub-a"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": subnets})

        result = await client.list_vpc_subnets(3)

    assert result == subnets
    mock_request.assert_awaited_once_with("GET", "/vpcs/3/subnets")
    await client.close()


async def test_list_vpc_subnets_wraps_http_errors() -> None:
    """list_vpc_subnets wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_vpc_subnets(3)

    assert "ListVPCSubnets" in str(excinfo.value)
    await client.close()


async def test_get_vpc_subnet_returns_body() -> None:
    """get_vpc_subnet GETs the subnet route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    subnet = {"id": 50, "label": "sub-a", "ipv4": "10.0.1.0/24"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(subnet)

        result = await client.get_vpc_subnet(3, 50)

    assert result == subnet
    mock_request.assert_awaited_once_with("GET", "/vpcs/3/subnets/50")
    await client.close()


async def test_get_vpc_subnet_wraps_http_errors() -> None:
    """get_vpc_subnet wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_vpc_subnet(3, 50)

    assert "GetVPCSubnet" in str(excinfo.value)
    await client.close()
