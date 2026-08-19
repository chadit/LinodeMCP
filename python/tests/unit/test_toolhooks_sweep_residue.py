"""Preview branches whose earlier tests rode the removed typed client methods.

Each test drives the public hook with the live route-era seams, pinning the
sentence the preview reports rather than the client method it once mocked.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks
from linodemcp.linode import Volume

if TYPE_CHECKING:
    from linodemcp.config import Config

pytestmark = pytest.mark.asyncio


def _previewing_client(reader: str, value: Any) -> AsyncMock:
    client = AsyncMock()
    getattr(client, reader).return_value = value
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def _preview(
    hook: str,
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
    client: AsyncMock,
) -> str:
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_cls.return_value = client
        result = await getattr(toolhooks, hook)(cfg, arguments, method, path, body)
    return str(result[0].text)


async def test_lke_cluster_update_preview_names_a_first_label(
    sample_config: Config,
) -> None:
    """A cluster with no label yet reads as the label being set, not changed."""
    client = _previewing_client("get_lke_cluster", {"id": 123, "k8s_version": "1.30"})

    text = await _preview(
        "linode_lke_cluster_update_preview",
        sample_config,
        {"cluster_id": 123, "label": "after", "dry_run": True},
        "PUT",
        "/lke/clusters/123",
        {"label": "after"},
        client,
    )

    assert 'Label is set to \\"after\\".' in text or 'Label is set to "after".' in text


async def test_vpc_update_preview_names_the_description_change(
    sample_config: Config,
) -> None:
    """A new description reads as replaced alongside the label being set."""
    client = _previewing_client("get_vpc", {"id": 7})

    text = await _preview(
        "linode_vpc_update_preview",
        sample_config,
        {"vpc_id": 7, "label": "net-a", "description": "edge", "dry_run": True},
        "PUT",
        "/vpcs/7",
        {"label": "net-a", "description": "edge"},
        client,
    )

    assert "The VPC description is updated." in text
    assert "net-a" in text


async def test_sshkey_update_preview_names_the_label_change(
    sample_config: Config,
) -> None:
    """The preview reads the key and names the label being replaced."""
    client = _previewing_client("get_ssh_key", {"id": 9, "label": "old-key"})

    text = await _preview(
        "linode_sshkey_update_preview",
        sample_config,
        {"ssh_key_id": 9, "label": "new-key", "dry_run": True},
        "PUT",
        "/profile/sshkeys/9",
        {"label": "new-key"},
        client,
    )

    client.get_ssh_key.assert_awaited_once_with(9)
    assert "new-key" in text


async def test_volume_resize_preview_names_the_starting_size(
    sample_config: Config,
) -> None:
    """A volume with a known size reads as growing from it, not just to."""
    volume = Volume(
        id=12,
        label="data",
        status="active",
        size=20,
        region="us-east",
        linode_id=None,
        linode_label=None,
        filesystem_path="/dev/disk/by-id/scsi-0Linode_Volume_data",
        tags=[],
        created="2026-07-01T00:00:00",
        updated="2026-07-01T00:00:00",
        hardware_type="nvme",
    )
    client = _previewing_client("get_volume", volume)

    text = await _preview(
        "linode_volume_resize_preview",
        sample_config,
        {"volume_id": 12, "size": 40, "dry_run": True},
        "POST",
        "/volumes/12/resize",
        {"size": 40},
        client,
    )

    assert "Volume resizes from 20 GB to 40 GB." in text


async def test_volume_resize_preview_without_a_readable_size(
    sample_config: Config,
) -> None:
    """A state carrying no size still previews the target it grows to."""
    client = _previewing_client("get_volume", {"id": 12, "label": "data"})

    text = await _preview(
        "linode_volume_resize_preview",
        sample_config,
        {"volume_id": 12, "size": 40, "dry_run": True},
        "POST",
        "/volumes/12/resize",
        {"size": 40},
        client,
    )

    assert "Volume resizes to 40 GB." in text
