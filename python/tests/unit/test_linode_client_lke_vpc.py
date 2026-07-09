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


# --- LKE clusters ----------------------------------------------------------


async def test_list_lke_clusters_unwraps_data_envelope() -> None:
    """list_lke_clusters GETs /lke/clusters and returns the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    clusters = [{"id": 1, "label": "prod"}, {"id": 2, "label": "staging"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": clusters, "page": 1})

        result = await client.list_lke_clusters()

    assert result == clusters
    mock_request.assert_awaited_once_with("GET", "/lke/clusters")
    await client.close()


async def test_list_lke_clusters_missing_data_is_empty() -> None:
    """A body without a data key yields an empty list, not a KeyError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        result = await client.list_lke_clusters()

    assert result == []
    await client.close()


async def test_list_lke_clusters_wraps_http_errors() -> None:
    """list_lke_clusters wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_lke_clusters()

    assert "ListLKEClusters" in str(excinfo.value)
    await client.close()


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


async def test_delete_lke_cluster_sends_delete() -> None:
    """delete_lke_cluster DELETEs the cluster route and returns None."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_lke_cluster(7)

    mock_request.assert_awaited_once_with("DELETE", "/lke/clusters/7")
    await client.close()


async def test_delete_lke_cluster_wraps_http_errors() -> None:
    """delete_lke_cluster wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_lke_cluster(7)

    assert "DeleteLKECluster" in str(excinfo.value)
    await client.close()


async def test_recycle_lke_cluster_posts_recycle() -> None:
    """recycle_lke_cluster POSTs to the recycle subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.recycle_lke_cluster(7)

    mock_request.assert_awaited_once_with("POST", "/lke/clusters/7/recycle")
    await client.close()


async def test_recycle_lke_cluster_wraps_http_errors() -> None:
    """recycle_lke_cluster wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.recycle_lke_cluster(7)

    assert "RecycleLKECluster" in str(excinfo.value)
    await client.close()


async def test_regenerate_lke_cluster_posts_regenerate() -> None:
    """regenerate_lke_cluster POSTs to the regenerate subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.regenerate_lke_cluster(7)

    mock_request.assert_awaited_once_with("POST", "/lke/clusters/7/regenerate")
    await client.close()


async def test_regenerate_lke_cluster_wraps_http_errors() -> None:
    """regenerate_lke_cluster wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.regenerate_lke_cluster(7)

    assert "RegenerateLKECluster" in str(excinfo.value)
    await client.close()


# --- LKE node pools --------------------------------------------------------


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


async def test_delete_lke_node_pool_sends_delete() -> None:
    """delete_lke_node_pool DELETEs the pool route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_lke_node_pool(7, 100)

    mock_request.assert_awaited_once_with("DELETE", "/lke/clusters/7/pools/100")
    await client.close()


async def test_delete_lke_node_pool_wraps_http_errors() -> None:
    """delete_lke_node_pool wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_lke_node_pool(7, 100)

    assert "DeleteLKENodePool" in str(excinfo.value)
    await client.close()


async def test_recycle_lke_node_pool_posts_recycle() -> None:
    """recycle_lke_node_pool POSTs to the pool recycle subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.recycle_lke_node_pool(7, 100)

    mock_request.assert_awaited_once_with("POST", "/lke/clusters/7/pools/100/recycle")
    await client.close()


async def test_recycle_lke_node_pool_wraps_http_errors() -> None:
    """recycle_lke_node_pool wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.recycle_lke_node_pool(7, 100)

    assert "RecycleLKENodePool" in str(excinfo.value)
    await client.close()


# --- LKE nodes -------------------------------------------------------------


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


async def test_delete_lke_node_sends_delete() -> None:
    """delete_lke_node DELETEs the node route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_lke_node(7, "12345-abc")

    mock_request.assert_awaited_once_with("DELETE", "/lke/clusters/7/nodes/12345-abc")
    await client.close()


async def test_delete_lke_node_wraps_http_errors() -> None:
    """delete_lke_node wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_lke_node(7, "12345-abc")

    assert "DeleteLKENode" in str(excinfo.value)
    await client.close()


async def test_recycle_lke_node_posts_recycle() -> None:
    """recycle_lke_node POSTs to the node recycle subroute."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.recycle_lke_node(7, "12345-abc")

    mock_request.assert_awaited_once_with(
        "POST", "/lke/clusters/7/nodes/12345-abc/recycle"
    )
    await client.close()


async def test_recycle_lke_node_wraps_http_errors() -> None:
    """recycle_lke_node wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.recycle_lke_node(7, "12345-abc")

    assert "RecycleLKENode" in str(excinfo.value)
    await client.close()


# --- LKE kubeconfig / dashboard / endpoints / tokens / acl -----------------


async def test_get_lke_kubeconfig_returns_body() -> None:
    """get_lke_kubeconfig GETs the kubeconfig route."""
    client = Client("https://api.linode.com/v4", "test-token")
    kubeconfig = {"kubeconfig": "base64-blob"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(kubeconfig)

        result = await client.get_lke_kubeconfig(7)

    assert result == kubeconfig
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7/kubeconfig")
    await client.close()


async def test_get_lke_kubeconfig_wraps_http_errors() -> None:
    """get_lke_kubeconfig wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_lke_kubeconfig(7)

    assert "GetLKEKubeconfig" in str(excinfo.value)
    await client.close()


async def test_delete_lke_kubeconfig_sends_delete() -> None:
    """delete_lke_kubeconfig DELETEs the kubeconfig route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_lke_kubeconfig(7)

    mock_request.assert_awaited_once_with("DELETE", "/lke/clusters/7/kubeconfig")
    await client.close()


async def test_delete_lke_kubeconfig_wraps_http_errors() -> None:
    """delete_lke_kubeconfig wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_lke_kubeconfig(7)

    assert "DeleteLKEKubeconfig" in str(excinfo.value)
    await client.close()


async def test_get_lke_dashboard_returns_body() -> None:
    """get_lke_dashboard GETs the dashboard route."""
    client = Client("https://api.linode.com/v4", "test-token")
    dashboard = {"url": "https://dashboard.example"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(dashboard)

        result = await client.get_lke_dashboard(7)

    assert result == dashboard
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7/dashboard")
    await client.close()


async def test_get_lke_dashboard_wraps_http_errors() -> None:
    """get_lke_dashboard wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_lke_dashboard(7)

    assert "GetLKEDashboard" in str(excinfo.value)
    await client.close()


async def test_list_lke_api_endpoints_unwraps_data() -> None:
    """list_lke_api_endpoints GETs the api-endpoints route and unwraps data."""
    client = Client("https://api.linode.com/v4", "test-token")
    endpoints = [{"endpoint": "https://a.example:443"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": endpoints})

        result = await client.list_lke_api_endpoints(7)

    assert result == endpoints
    mock_request.assert_awaited_once_with("GET", "/lke/clusters/7/api-endpoints")
    await client.close()


async def test_list_lke_api_endpoints_wraps_http_errors() -> None:
    """list_lke_api_endpoints wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_lke_api_endpoints(7)

    assert "ListLKEAPIEndpoints" in str(excinfo.value)
    await client.close()


async def test_delete_lke_service_token_sends_delete() -> None:
    """delete_lke_service_token DELETEs the servicetoken route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_lke_service_token(7)

    mock_request.assert_awaited_once_with("DELETE", "/lke/clusters/7/servicetoken")
    await client.close()


async def test_delete_lke_service_token_wraps_http_errors() -> None:
    """delete_lke_service_token wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_lke_service_token(7)

    assert "DeleteLKEServiceToken" in str(excinfo.value)
    await client.close()


async def test_delete_lke_control_plane_acl_sends_delete() -> None:
    """delete_lke_control_plane_acl DELETEs the control_plane_acl route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_lke_control_plane_acl(7)

    mock_request.assert_awaited_once_with("DELETE", "/lke/clusters/7/control_plane_acl")
    await client.close()


async def test_delete_lke_control_plane_acl_wraps_http_errors() -> None:
    """delete_lke_control_plane_acl wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_lke_control_plane_acl(7)

    assert "DeleteLKEControlPlaneACL" in str(excinfo.value)
    await client.close()


# --- LKE versions / types --------------------------------------------------


async def test_list_lke_versions_unwraps_data() -> None:
    """list_lke_versions GETs /lke/versions and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    versions = [{"id": "1.31"}, {"id": "1.30"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": versions})

        result = await client.list_lke_versions()

    assert result == versions
    mock_request.assert_awaited_once_with("GET", "/lke/versions")
    await client.close()


async def test_list_lke_versions_wraps_http_errors() -> None:
    """list_lke_versions wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_lke_versions()

    assert "ListLKEVersions" in str(excinfo.value)
    await client.close()


async def test_get_lke_version_returns_body() -> None:
    """get_lke_version GETs the version route and returns the body."""
    client = Client("https://api.linode.com/v4", "test-token")
    version = {"id": "1.31"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(version)

        result = await client.get_lke_version("1.31")

    assert result == version
    mock_request.assert_awaited_once_with("GET", "/lke/versions/1.31")
    await client.close()


async def test_get_lke_version_wraps_http_errors() -> None:
    """get_lke_version wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.get_lke_version("1.31")

    assert "GetLKEVersion" in str(excinfo.value)
    await client.close()


async def test_list_lke_types_unwraps_data() -> None:
    """list_lke_types GETs /lke/types and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    types = [{"id": "lke-ha", "price": {"monthly": 60}}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": types})

        result = await client.list_lke_types()

    assert result == types
    mock_request.assert_awaited_once_with("GET", "/lke/types")
    await client.close()


async def test_list_lke_types_wraps_http_errors() -> None:
    """list_lke_types wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_lke_types()

    assert "ListLKETypes" in str(excinfo.value)
    await client.close()


# --- VPCs ------------------------------------------------------------------


async def test_list_vpcs_unwraps_data() -> None:
    """list_vpcs GETs /vpcs and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    vpcs = [{"id": 1, "label": "net-a"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": vpcs})

        result = await client.list_vpcs()

    assert result == vpcs
    mock_request.assert_awaited_once_with("GET", "/vpcs")
    await client.close()


async def test_list_vpcs_wraps_http_errors() -> None:
    """list_vpcs wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_vpcs()

    assert "ListVPCs" in str(excinfo.value)
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


async def test_delete_vpc_sends_delete() -> None:
    """delete_vpc DELETEs the vpc route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_vpc(3)

    mock_request.assert_awaited_once_with("DELETE", "/vpcs/3")
    await client.close()


async def test_delete_vpc_wraps_http_errors() -> None:
    """delete_vpc wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_vpc(3)

    assert "DeleteVPC" in str(excinfo.value)
    await client.close()


async def test_list_vpc_ips_unwraps_data() -> None:
    """list_vpc_ips GETs /vpcs/ips and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    ips = [{"address": "10.0.0.5"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": ips})

        result = await client.list_vpc_ips()

    assert result == ips
    mock_request.assert_awaited_once_with("GET", "/vpcs/ips")
    await client.close()


async def test_list_vpc_ips_wraps_http_errors() -> None:
    """list_vpc_ips wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_vpc_ips()

    assert "ListVPCIPs" in str(excinfo.value)
    await client.close()


async def test_list_vpc_ip_unwraps_data() -> None:
    """list_vpc_ip GETs the per-vpc ips route and unwraps the data list."""
    client = Client("https://api.linode.com/v4", "test-token")
    ips = [{"address": "10.0.1.9"}]

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({"data": ips})

        result = await client.list_vpc_ip(3)

    assert result == ips
    mock_request.assert_awaited_once_with("GET", "/vpcs/3/ips")
    await client.close()


async def test_list_vpc_ip_wraps_http_errors() -> None:
    """list_vpc_ip wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.list_vpc_ip(3)

    assert "ListVPCIP" in str(excinfo.value)
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


async def test_delete_vpc_subnet_sends_delete() -> None:
    """delete_vpc_subnet DELETEs the subnet route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_vpc_subnet(3, 50)

    mock_request.assert_awaited_once_with("DELETE", "/vpcs/3/subnets/50")
    await client.close()


async def test_delete_vpc_subnet_wraps_http_errors() -> None:
    """delete_vpc_subnet wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_vpc_subnet(3, 50)

    assert "DeleteVPCSubnet" in str(excinfo.value)
    await client.close()
