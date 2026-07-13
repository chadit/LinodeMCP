"""Behavioral tests for previously-untested Client write methods.

These cover the struct-returning write methods (create/update/resize/clone/boot)
that decode the API body through the `_parse_*` helpers and wrap httpx failures
in NetworkError. The proto `*_raw` siblings were already tested; the typed
methods here were whole untested branches.
"""

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import (
    Client,
    DomainRecord,
    NetworkError,
    RetryableClient,
    Volume,
)

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


# --- Firewalls -------------------------------------------------------------


async def test_update_firewall_raw_builds_partial_body() -> None:
    """update_firewall_raw only sends the fields the caller supplied."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "id": 5,
        "label": "renamed",
        "status": "disabled",
        "rules": {"inbound_policy": "ACCEPT", "outbound_policy": "DROP"},
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.update_firewall_raw(
            5, label="renamed", outbound_policy="DROP"
        )

    assert result["label"] == "renamed"
    assert result["rules"]["outbound_policy"] == "DROP"
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/networking/firewalls/5"
    assert sent == {"label": "renamed", "rules": {"outbound_policy": "DROP"}}
    assert "status" not in sent
    await client.close()


async def test_update_firewall_raw_wraps_http_errors() -> None:
    """update_firewall_raw wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_firewall_raw(5, label="renamed")

    assert "UpdateFirewall" in str(excinfo.value)
    await client.close()


# --- Domains ---------------------------------------------------------------


async def test_update_domain_wraps_http_errors() -> None:
    """update_domain wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.update_domain(42, description="updated")

    assert "UpdateDomain" in str(excinfo.value)
    await client.close()


async def test_create_domain_record_omits_unset_optionals() -> None:
    """create_domain_record drops None optionals from the request body."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "id": 9,
        "type": "A",
        "name": "www",
        "target": "8.8.8.8",
        "ttl_sec": 300,
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.create_domain_record(
            42, "A", name="www", target="8.8.8.8", ttl_sec=300
        )

    assert isinstance(result, DomainRecord)
    assert result.id == 9
    assert result.target == "8.8.8.8"
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/domains/42/records"
    assert sent == {"type": "A", "name": "www", "target": "8.8.8.8", "ttl_sec": 300}
    assert "priority" not in sent
    await client.close()


async def test_create_domain_record_rejects_private_a_target() -> None:
    """create_domain_record validates A record targets before any request."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="private IP"),
    ):
        await client.create_domain_record(42, "A", name="www", target="10.0.0.1")

    mock_request.assert_not_awaited()
    await client.close()


async def test_update_domain_record_sends_only_provided_fields() -> None:
    """update_domain_record sends a partial body keyed by the supplied args."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {"id": 9, "type": "A", "name": "www", "target": "203.0.113.9"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.update_domain_record(
            42, 9, target="203.0.113.9", ttl_sec=600
        )

    assert isinstance(result, DomainRecord)
    assert result.target == "203.0.113.9"
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "PUT"
    assert endpoint == "/domains/42/records/9"
    assert sent == {"target": "203.0.113.9", "ttl_sec": 600}
    await client.close()


async def test_delete_domain_record_targets_record_route() -> None:
    """delete_domain_record issues a DELETE to the record route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_domain_record(42, 9)

    method, endpoint = mock_request.await_args_list[0].args
    assert method == "DELETE"
    assert endpoint == "/domains/42/records/9"
    await client.close()


async def test_delete_domain_record_wraps_http_errors() -> None:
    """delete_domain_record wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_domain_record(42, 9)

    assert "DeleteDomainRecord" in str(excinfo.value)
    await client.close()


# --- Volumes ---------------------------------------------------------------


async def test_create_volume_decodes_response() -> None:
    """create_volume posts to /volumes and parses the returned volume."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "id": 100,
        "label": "data-vol",
        "status": "creating",
        "size": 40,
        "region": "us-east",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.create_volume(
            "data-vol", region="us-east", size=40, linode_id=7
        )

    assert isinstance(result, Volume)
    assert result.id == 100
    assert result.size == 40
    assert result.region == "us-east"
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/volumes"
    assert sent["linode_id"] == 7
    await client.close()


async def test_create_volume_rejects_undersized_volume() -> None:
    """create_volume validates size before issuing the request."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="at least 10 GB"),
    ):
        await client.create_volume("data-vol", size=5)

    mock_request.assert_not_awaited()
    await client.close()


async def test_attach_volume_posts_to_attach_route() -> None:
    """attach_volume posts the linode and config to the attach route."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {"id": 100, "label": "data-vol", "status": "active", "linode_id": 7}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.attach_volume(100, 7, config_id=3)

    assert isinstance(result, Volume)
    assert result.linode_id == 7
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/volumes/100/attach"
    assert sent["linode_id"] == 7
    assert sent["config_id"] == 3
    # persist_across_boots not supplied -> omitted so the API applies its default.
    assert "persist_across_boots" not in sent
    await client.close()


async def test_attach_volume_wraps_http_errors() -> None:
    """attach_volume wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.attach_volume(100, 7)

    assert "AttachVolume" in str(excinfo.value)
    await client.close()


async def test_resize_volume_posts_new_size() -> None:
    """resize_volume posts the new size to the resize route."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {"id": 100, "label": "data-vol", "status": "resizing", "size": 80}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.resize_volume(100, 80)

    assert isinstance(result, Volume)
    assert result.size == 80
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/volumes/100/resize"
    assert sent == {"size": 80}
    await client.close()


async def test_resize_volume_rejects_undersized() -> None:
    """resize_volume validates the requested size first."""
    client = Client("https://api.linode.com/v4", "test-token")

    with (
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
        pytest.raises(ValueError, match="at least 10 GB"),
    ):
        await client.resize_volume(100, 3)

    mock_request.assert_not_awaited()
    await client.close()


# --- NodeBalancers ---------------------------------------------------------


async def test_create_nodebalancer_raw_wraps_http_errors() -> None:
    """create_nodebalancer_raw wraps an HTTP status error as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")
    request = httpx.Request("POST", "https://api.linode.com/v4/nodebalancers")
    response = httpx.Response(400, request=request)
    status_error = httpx.HTTPStatusError("bad", request=request, response=response)

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = status_error

        with pytest.raises(NetworkError) as excinfo:
            await client.create_nodebalancer_raw("us-east", label="lb-1")

    assert "CreateNodeBalancer" in str(excinfo.value)
    await client.close()


async def test_delete_nodebalancer_targets_route() -> None:
    """delete_nodebalancer issues a DELETE to the nodebalancer route."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.delete_nodebalancer(200)

    method, endpoint = mock_request.await_args_list[0].args
    assert method == "DELETE"
    assert endpoint == "/nodebalancers/200"
    await client.close()


async def test_delete_nodebalancer_wraps_http_errors() -> None:
    """delete_nodebalancer wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.delete_nodebalancer(200)

    assert "DeleteNodeBalancer" in str(excinfo.value)
    await client.close()


# --- Instance lifecycle ----------------------------------------------------


async def test_boot_instance_sends_config_id() -> None:
    """boot_instance posts the config id when one is provided."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.boot_instance(7, config_id=3)

    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/7/boot"
    assert sent == {"config_id": 3}
    await client.close()


async def test_boot_instance_omits_empty_body() -> None:
    """boot_instance sends a None body when no config id is given."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.boot_instance(7)

    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/7/boot"
    assert sent is None
    await client.close()


async def test_reboot_instance_wraps_http_errors() -> None:
    """reboot_instance wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.reboot_instance(7)

    assert "RebootInstance" in str(excinfo.value)
    await client.close()


async def test_resize_instance_posts_resize_body() -> None:
    """resize_instance posts the new type plus any explicitly set resize options,
    and omits options the caller does not set so the API applies its own defaults."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.resize_instance(
            7, "g6-standard-4", allow_auto_disk_resize=True, migration_type="cold"
        )

    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/7/resize"
    assert sent == {
        "type": "g6-standard-4",
        "allow_auto_disk_resize": True,
        "migration_type": "cold",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response({})

        await client.resize_instance(7, "g6-standard-4")

    _, _, sent = mock_request.await_args_list[0].args
    assert sent == {"type": "g6-standard-4"}
    await client.close()


async def test_resize_instance_wraps_http_errors() -> None:
    """resize_instance wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.resize_instance(7, "g6-standard-4")

    assert "ResizeInstance" in str(excinfo.value)
    await client.close()


async def test_clone_instance_raw_sends_clone_body() -> None:
    """clone_instance_raw posts the clone body including disks and configs."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {
        "id": 8,
        "label": "clone-1",
        "status": "provisioning",
        "type": "g6-standard-2",
        "region": "us-west",
    }

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.clone_instance_raw(
            7, region="us-west", label="clone-1", disks=[111], configs=[222]
        )

    assert result["id"] == 8
    assert result["label"] == "clone-1"
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/7/clone"
    assert sent["region"] == "us-west"
    assert sent["disks"] == [111]
    assert sent["configs"] == [222]
    await client.close()


async def test_clone_instance_raw_wraps_http_errors() -> None:
    """clone_instance_raw wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.clone_instance_raw(7, region="us-west")

    assert "CloneInstance" in str(excinfo.value)
    await client.close()


async def test_create_instance_disk_returns_raw_body() -> None:
    """create_instance_disk returns the raw API body for the disk."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {"id": 555, "label": "boot-disk", "size": 25600, "status": "ready"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = _ok_response(body)

        result = await client.create_instance_disk(
            7, "boot-disk", 25600, filesystem="ext4", image="linode/debian12"
        )

    assert result["id"] == 555
    assert result["label"] == "boot-disk"
    method, endpoint, sent = mock_request.await_args_list[0].args
    assert method == "POST"
    assert endpoint == "/linode/instances/7/disks"
    assert sent["label"] == "boot-disk"
    assert sent["filesystem"] == "ext4"
    assert sent["image"] == "linode/debian12"
    await client.close()


async def test_create_instance_disk_wraps_http_errors() -> None:
    """create_instance_disk wraps httpx failures as NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.HTTPError("boom")

        with pytest.raises(NetworkError) as excinfo:
            await client.create_instance_disk(7, "boot-disk", 25600)

    assert "CreateInstanceDisk" in str(excinfo.value)
    await client.close()


# --- RetryableClient delegation -------------------------------------------


async def test_retryable_create_volume_delegates() -> None:
    """RetryableClient.create_volume delegates to the base client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    expected = Volume.__new__(Volume)

    with patch.object(
        retryable.client, "create_volume", new_callable=AsyncMock
    ) as mock_create:
        mock_create.return_value = expected

        result = await retryable.create_volume("data-vol", region="us-east", size=40)

    assert result is expected
    mock_create.assert_awaited_once_with("data-vol", "us-east", None, 40, None)
    await retryable.close()


async def test_retryable_resize_volume_delegates() -> None:
    """RetryableClient.resize_volume delegates to the base client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")
    expected = Volume.__new__(Volume)

    with patch.object(
        retryable.client, "resize_volume", new_callable=AsyncMock
    ) as mock_resize:
        mock_resize.return_value = expected

        result = await retryable.resize_volume(100, 80)

    assert result is expected
    mock_resize.assert_awaited_once_with(100, 80)
    await retryable.close()


async def test_retryable_delete_nodebalancer_delegates() -> None:
    """RetryableClient.delete_nodebalancer delegates to the base client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "delete_nodebalancer", new_callable=AsyncMock
    ) as mock_delete:
        mock_delete.return_value = None

        await retryable.delete_nodebalancer(200)

    mock_delete.assert_awaited_once_with(200)
    await retryable.close()


async def test_retryable_boot_instance_delegates() -> None:
    """RetryableClient.boot_instance delegates to the base client."""
    retryable = RetryableClient("https://api.linode.com/v4", "test-token")

    with patch.object(
        retryable.client, "boot_instance", new_callable=AsyncMock
    ) as mock_boot:
        mock_boot.return_value = None

        await retryable.boot_instance(7, config_id=3)

    mock_boot.assert_awaited_once_with(7, 3)
    await retryable.close()
