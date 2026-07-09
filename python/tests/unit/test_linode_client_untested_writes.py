"""Behavioral tests for Client write methods whose success path was untested.

These cover object-storage, LKE, VPC, and instance write methods that build a
request body (often with conditional optional fields), send it through
``make_request``, decode the JSON body, and wrap ``httpx`` failures in
``NetworkError``. Each success test asserts the exact HTTP verb, endpoint, and
body so a dropped or renamed field is caught; each error test asserts the
failure maps to ``NetworkError`` carrying the operation name.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError

if TYPE_CHECKING:
    from collections.abc import AsyncIterator

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


@pytest.fixture
async def client() -> AsyncIterator[Client]:
    """Yield a Client with a real httpx session that gets closed on teardown."""
    instance = Client("https://api.linode.com/v4", "test-token")
    yield instance
    await instance.close()


# --- Object Storage --------------------------------------------------------


async def test_create_object_storage_bucket_includes_optionals(
    client: Client,
) -> None:
    """create_object_storage_bucket forwards acl and cors when supplied."""
    body = {"label": "assets", "region": "us-east", "cors_enabled": True}
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response(body)
        result = await client.create_object_storage_bucket(
            "assets", "us-east", acl="private", cors_enabled=True
        )

    assert result == body
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/object-storage/buckets"
    assert sent == {
        "label": "assets",
        "region": "us-east",
        "acl": "private",
        "cors_enabled": True,
    }


async def test_create_object_storage_bucket_omits_unset_optionals(
    client: Client,
) -> None:
    """create_object_storage_bucket sends only label and region by default."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"label": "logs", "region": "us-ord"})
        await client.create_object_storage_bucket("logs", "us-ord")

    _, _, sent = req.await_args_list[0].args
    assert sent == {"label": "logs", "region": "us-ord"}
    assert "acl" not in sent
    assert "cors_enabled" not in sent


async def test_create_object_storage_bucket_wraps_http_errors(client: Client) -> None:
    """create_object_storage_bucket maps httpx failures to NetworkError."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_object_storage_bucket("assets", "us-east")

    assert "CreateObjectStorageBucket" in str(excinfo.value)


async def test_update_object_storage_bucket_access_partial_body(
    client: Client,
) -> None:
    """update_object_storage_bucket_access PUTs only the supplied fields."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response(None)
        await client.update_object_storage_bucket_access(
            "us-east", "assets", acl="public-read"
        )

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/object-storage/buckets/us-east/assets/access"
    assert sent == {"acl": "public-read"}
    assert "cors_enabled" not in sent


async def test_update_object_storage_bucket_access_wraps_errors(
    client: Client,
) -> None:
    """update_object_storage_bucket_access wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_object_storage_bucket_access("us-east", "assets")

    assert "UpdateObjectStorageBucketAccess" in str(excinfo.value)


async def test_create_object_storage_key_with_bucket_access(client: Client) -> None:
    """create_object_storage_key forwards the bucket_access grant list."""
    grants = [{"bucket_name": "assets", "permissions": "read_only"}]
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 7, "label": "ci"})
        result = await client.create_object_storage_key("ci", bucket_access=grants)

    assert result["id"] == 7
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/object-storage/keys"
    assert sent == {"label": "ci", "bucket_access": grants}


async def test_create_object_storage_key_wraps_errors(client: Client) -> None:
    """create_object_storage_key wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_object_storage_key("ci")

    assert "CreateObjectStorageKey" in str(excinfo.value)


async def test_update_object_storage_key_partial_body(client: Client) -> None:
    """update_object_storage_key PUTs to the keyed route with only label set."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response(None)
        await client.update_object_storage_key(42, label="renamed")

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/object-storage/keys/42"
    assert sent == {"label": "renamed"}
    assert "bucket_access" not in sent


async def test_update_object_storage_key_wraps_errors(client: Client) -> None:
    """update_object_storage_key wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_object_storage_key(42, label="x")

    assert "UpdateObjectStorageKey" in str(excinfo.value)


async def test_create_presigned_url_builds_body(client: Client) -> None:
    """create_presigned_url posts method/name/expires_in to the object-url route."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"url": "https://signed"})
        result = await client.create_presigned_url(
            "us-east", "assets", "photo.jpg", "GET", expires_in=120
        )

    assert result == {"url": "https://signed"}
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/object-storage/buckets/us-east/assets/object-url"
    assert sent == {"method": "GET", "name": "photo.jpg", "expires_in": 120}


async def test_create_presigned_url_wraps_errors(client: Client) -> None:
    """create_presigned_url wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_presigned_url("us-east", "assets", "photo.jpg", "GET")

    assert "CreatePresignedURL" in str(excinfo.value)


async def test_update_object_acl_builds_body(client: Client) -> None:
    """update_object_acl PUTs the acl and object name."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"acl": "public-read"})
        result = await client.update_object_acl(
            "us-east", "assets", "photo.jpg", "public-read"
        )

    assert result == {"acl": "public-read"}
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/object-storage/buckets/us-east/assets/object-acl"
    assert sent == {"acl": "public-read", "name": "photo.jpg"}


async def test_update_object_acl_wraps_errors(client: Client) -> None:
    """update_object_acl wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_object_acl("us-east", "assets", "p.jpg", "private")

    assert "UpdateObjectACL" in str(excinfo.value)


# --- LKE -------------------------------------------------------------------


async def test_create_lke_cluster_includes_optionals(client: Client) -> None:
    """create_lke_cluster forwards tags and control_plane when supplied."""
    pools = [{"type": "g6-standard-1", "count": 3}]
    control_plane = {"high_availability": True}
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 99, "label": "prod"})
        result = await client.create_lke_cluster(
            "prod",
            "us-east",
            "1.30",
            pools,
            tags=["team-a"],
            control_plane=control_plane,
        )

    assert result["id"] == 99
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/lke/clusters"
    assert sent == {
        "label": "prod",
        "region": "us-east",
        "k8s_version": "1.30",
        "node_pools": pools,
        "tags": ["team-a"],
        "control_plane": control_plane,
    }


async def test_create_lke_cluster_wraps_errors(client: Client) -> None:
    """create_lke_cluster wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_lke_cluster("prod", "us-east", "1.30", [])

    assert "CreateLKECluster" in str(excinfo.value)


async def test_update_lke_cluster_partial_body(client: Client) -> None:
    """update_lke_cluster PUTs only the fields the caller changes."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 99, "label": "renamed"})
        await client.update_lke_cluster(99, label="renamed", k8s_version="1.31")

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/lke/clusters/99"
    assert sent == {"label": "renamed", "k8s_version": "1.31"}
    assert "tags" not in sent
    assert "control_plane" not in sent


async def test_update_lke_cluster_wraps_errors(client: Client) -> None:
    """update_lke_cluster wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_lke_cluster(99, label="x")

    assert "UpdateLKECluster" in str(excinfo.value)


async def test_create_lke_node_pool_includes_optionals(client: Client) -> None:
    """create_lke_node_pool forwards autoscaler and tags to the pools route."""
    autoscaler = {"enabled": True, "min": 1, "max": 5}
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 3, "count": 2})
        result = await client.create_lke_node_pool(
            99, "g6-standard-2", 2, autoscaler=autoscaler, tags=["web"]
        )

    assert result["id"] == 3
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/lke/clusters/99/pools"
    assert sent == {
        "type": "g6-standard-2",
        "count": 2,
        "autoscaler": autoscaler,
        "tags": ["web"],
    }


async def test_create_lke_node_pool_wraps_errors(client: Client) -> None:
    """create_lke_node_pool wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_lke_node_pool(99, "g6-standard-2", 2)

    assert "CreateLKENodePool" in str(excinfo.value)


async def test_update_lke_node_pool_partial_body(client: Client) -> None:
    """update_lke_node_pool PUTs only the supplied count to the pool route."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 3, "count": 4})
        await client.update_lke_node_pool(99, 3, count=4)

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/lke/clusters/99/pools/3"
    assert sent == {"count": 4}
    assert "autoscaler" not in sent


async def test_update_lke_node_pool_wraps_errors(client: Client) -> None:
    """update_lke_node_pool wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_lke_node_pool(99, 3, count=4)

    assert "UpdateLKENodePool" in str(excinfo.value)


async def test_update_lke_control_plane_acl_wraps_acl(client: Client) -> None:
    """update_lke_control_plane_acl nests the caller's acl under an acl key."""
    acl = {"enabled": True, "addresses": {"ipv4": ["10.0.0.0/24"]}}
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"acl": acl})
        result = await client.update_lke_control_plane_acl(99, acl)

    assert result == {"acl": acl}
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/lke/clusters/99/control_plane_acl"
    assert sent == {"acl": acl}


async def test_update_lke_control_plane_acl_wraps_errors(client: Client) -> None:
    """update_lke_control_plane_acl wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_lke_control_plane_acl(99, {"enabled": False})

    assert "UpdateLKEControlPlaneACL" in str(excinfo.value)


# --- VPC -------------------------------------------------------------------


async def test_create_vpc_includes_optionals(client: Client) -> None:
    """create_vpc forwards description and inline subnets when supplied."""
    subnets = [{"label": "sub-a", "ipv4": "10.0.0.0/24"}]
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 5, "label": "net"})
        result = await client.create_vpc(
            "net", "us-east", description="primary", subnets=subnets
        )

    assert result["id"] == 5
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/vpcs"
    assert sent == {
        "label": "net",
        "region": "us-east",
        "description": "primary",
        "subnets": subnets,
    }


async def test_create_vpc_omits_unset_optionals(client: Client) -> None:
    """create_vpc sends only label and region when optionals are omitted."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 6, "label": "bare"})
        await client.create_vpc("bare", "us-ord")

    _, _, sent = req.await_args_list[0].args
    assert sent == {"label": "bare", "region": "us-ord"}


async def test_create_vpc_wraps_errors(client: Client) -> None:
    """create_vpc wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_vpc("net", "us-east")

    assert "CreateVPC" in str(excinfo.value)


async def test_update_vpc_partial_body(client: Client) -> None:
    """update_vpc PUTs only the description when label is unchanged."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 5, "description": "updated"})
        await client.update_vpc(5, description="updated")

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/vpcs/5"
    assert sent == {"description": "updated"}
    assert "label" not in sent


async def test_update_vpc_wraps_errors(client: Client) -> None:
    """update_vpc wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_vpc(5, label="x")

    assert "UpdateVPC" in str(excinfo.value)


async def test_create_vpc_subnet_builds_body(client: Client) -> None:
    """create_vpc_subnet posts label and ipv4 to the subnets route."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 2, "label": "sub"})
        result = await client.create_vpc_subnet(5, "sub", "10.0.1.0/24")

    assert result["id"] == 2
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/vpcs/5/subnets"
    assert sent == {"label": "sub", "ipv4": "10.0.1.0/24"}


async def test_create_vpc_subnet_wraps_errors(client: Client) -> None:
    """create_vpc_subnet wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.create_vpc_subnet(5, "sub", "10.0.1.0/24")

    assert "CreateVPCSubnet" in str(excinfo.value)


async def test_update_vpc_subnet_builds_body(client: Client) -> None:
    """update_vpc_subnet PUTs the new label to the subnet route."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 2, "label": "renamed"})
        result = await client.update_vpc_subnet(5, 2, "renamed")

    assert result["label"] == "renamed"
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/vpcs/5/subnets/2"
    assert sent == {"label": "renamed"}


async def test_update_vpc_subnet_wraps_errors(client: Client) -> None:
    """update_vpc_subnet wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_vpc_subnet(5, 2, "renamed")

    assert "UpdateVPCSubnet" in str(excinfo.value)


# --- Instances -------------------------------------------------------------


async def test_get_instance_backup_targets_backup_route(client: Client) -> None:
    """get_instance_backup GETs the nested backup route and returns the body."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 8, "status": "successful"})
        result = await client.get_instance_backup(123, 8)

    assert result == {"id": 8, "status": "successful"}
    method, endpoint = req.await_args_list[0].args
    assert method == "GET"
    assert endpoint == "/linode/instances/123/backups/8"


async def test_get_instance_backup_wraps_errors(client: Client) -> None:
    """get_instance_backup wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.get_instance_backup(123, 8)

    assert "GetInstanceBackup" in str(excinfo.value)


async def test_restore_instance_backup_builds_body(client: Client) -> None:
    """restore_instance_backup posts target linode_id and overwrite flag."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({})
        await client.restore_instance_backup(123, 8, 456, overwrite=True)

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/backups/8/restore"
    assert sent == {"linode_id": 456, "overwrite": True}


async def test_restore_instance_backup_wraps_errors(client: Client) -> None:
    """restore_instance_backup wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.restore_instance_backup(123, 8, 456)

    assert "RestoreInstanceBackup" in str(excinfo.value)


async def test_update_instance_disk_partial_body(client: Client) -> None:
    """update_instance_disk PUTs only the new label to the disk route."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 77, "label": "root-renamed"})
        result = await client.update_instance_disk(123, 77, label="root-renamed")

    assert result["label"] == "root-renamed"
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/linode/instances/123/disks/77"
    assert sent == {"label": "root-renamed"}


async def test_update_instance_disk_wraps_errors(client: Client) -> None:
    """update_instance_disk wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.update_instance_disk(123, 77, label="x")

    assert "UpdateInstanceDisk" in str(excinfo.value)


async def test_clone_instance_disk_targets_clone_route(client: Client) -> None:
    """clone_instance_disk POSTs to the disk clone route with no body."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 78, "status": "ready"})
        result = await client.clone_instance_disk(123, 77)

    assert result["id"] == 78
    method, endpoint = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/disks/77/clone"


async def test_clone_instance_disk_wraps_errors(client: Client) -> None:
    """clone_instance_disk wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.clone_instance_disk(123, 77)

    assert "CloneInstanceDisk" in str(excinfo.value)


async def test_resize_instance_disk_builds_body(client: Client) -> None:
    """resize_instance_disk posts the new size to the disk resize route."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({})
        await client.resize_instance_disk(123, 77, 51200)

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/disks/77/resize"
    assert sent == {"size": 51200}


async def test_resize_instance_disk_wraps_errors(client: Client) -> None:
    """resize_instance_disk wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.resize_instance_disk(123, 77, 51200)

    assert "ResizeInstanceDisk" in str(excinfo.value)


async def test_allocate_instance_ip_builds_body(client: Client) -> None:
    """allocate_instance_ip posts the requested ip type and public flag."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"address": "192.0.2.5", "public": False})
        result = await client.allocate_instance_ip(123, "ipv4", public=False)

    assert result["public"] is False
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/ips"
    assert sent == {"type": "ipv4", "public": False}


async def test_allocate_instance_ip_wraps_errors(client: Client) -> None:
    """allocate_instance_ip wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.allocate_instance_ip(123, "ipv4")

    assert "AllocateInstanceIP" in str(excinfo.value)


async def test_migrate_instance_includes_region(client: Client) -> None:
    """migrate_instance forwards a target region when supplied."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({})
        await client.migrate_instance(123, region="us-central")

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/migrate"
    assert sent == {"region": "us-central"}


async def test_migrate_instance_omits_region_when_unset(client: Client) -> None:
    """migrate_instance sends an empty body when no region is given."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({})
        await client.migrate_instance(123)

    _, _, sent = req.await_args_list[0].args
    assert sent == {}


async def test_migrate_instance_wraps_errors(client: Client) -> None:
    """migrate_instance wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.migrate_instance(123)

    assert "MigrateInstance" in str(excinfo.value)


async def test_rebuild_instance_includes_optionals(client: Client) -> None:
    """rebuild_instance forwards authorized keys/users and the booted flag."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 123, "status": "rebuilding"})
        result = await client.rebuild_instance(
            123,
            "linode/ubuntu24.04",
            "s3cret-pass",
            authorized_keys=["ssh-ed25519 AAAA"],
            authorized_users=["alice"],
            booted=False,
        )

    assert result["status"] == "rebuilding"
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/rebuild"
    assert sent == {
        "image": "linode/ubuntu24.04",
        "root_pass": "s3cret-pass",
        "authorized_keys": ["ssh-ed25519 AAAA"],
        "authorized_users": ["alice"],
        "booted": False,
    }


async def test_rebuild_instance_omits_unset_optionals(client: Client) -> None:
    """rebuild_instance sends only image and root_pass by default."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 123})
        await client.rebuild_instance(123, "linode/debian12", "pw")

    _, _, sent = req.await_args_list[0].args
    assert sent == {"image": "linode/debian12", "root_pass": "pw"}
    assert "booted" not in sent


async def test_rebuild_instance_wraps_errors(client: Client) -> None:
    """rebuild_instance wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.rebuild_instance(123, "linode/debian12", "pw")

    assert "RebuildInstance" in str(excinfo.value)


async def test_rescue_instance_includes_devices(client: Client) -> None:
    """rescue_instance forwards a device map to the rescue route."""
    devices = {"sda": {"disk_id": 77}}
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({})
        await client.rescue_instance(123, devices=devices)

    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/123/rescue"
    assert sent == {"devices": devices}


async def test_rescue_instance_wraps_errors(client: Client) -> None:
    """rescue_instance wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.rescue_instance(123)

    assert "RescueInstance" in str(excinfo.value)


# --- Raw passthrough helpers ----------------------------------------------


async def test_get_raw_returns_decoded_json(client: Client) -> None:
    """get_raw GETs the endpoint and returns the undecorated JSON body."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 1, "extra": "unmodeled"})
        result = await client.get_raw("/linode/instances/1")

    assert result == {"id": 1, "extra": "unmodeled"}
    method, endpoint = req.await_args_list[0].args
    assert method == "GET"
    assert endpoint == "/linode/instances/1"


async def test_post_raw_forwards_body(client: Client) -> None:
    """post_raw POSTs the caller's body and returns the decoded JSON."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 2})
        result = await client.post_raw("/volumes", {"label": "v"})

    assert result == {"id": 2}
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/volumes"
    assert sent == {"label": "v"}


async def test_post_raw_defaults_missing_body_to_empty(client: Client) -> None:
    """post_raw sends an empty object rather than None when no body is given."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({})
        await client.post_raw("/linode/instances/1/reboot")

    _, _, sent = req.await_args_list[0].args
    assert sent == {}


async def test_put_raw_defaults_missing_body_to_empty(client: Client) -> None:
    """put_raw sends an empty object rather than None when no body is given."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"updated": True})
        result = await client.put_raw("/vpcs/5")

    assert result == {"updated": True}
    method, endpoint, sent = req.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/vpcs/5"
    assert sent == {}
