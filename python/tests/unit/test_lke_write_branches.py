"""Behavioral tests for untested branches in tools/linode_lke_write.py.

Drives the tool handlers (never the private preview builders) so the tests
match the repo convention and exercise the real dry-run integration. Covers
the cluster-update preview arms that existing tests miss (label set from
absent, k8s version upgrade), the ACL-update disabled and reconfigure
previews, and the per-required-field guards on cluster create.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.tools.linode_lke_write import (
    handle_linode_lke_acl_update,
    handle_linode_lke_cluster_create,
    handle_linode_lke_cluster_update,
)

if TYPE_CHECKING:
    from linodemcp.config import Config

pytestmark = pytest.mark.asyncio


def _mock_client(**returns: Any) -> AsyncMock:
    """Build an async-context-manager client mock with canned return values."""
    client = AsyncMock()
    for name, value in returns.items():
        getattr(client, name).return_value = value
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


# --- Cluster-update preview arms -------------------------------------------


async def test_cluster_update_dry_run_previews_label_set_and_upgrade(
    sample_config: Config,
) -> None:
    """A cluster with no prior label/version previews a label set and upgrade.

    Existing coverage only hits the label-change arm; this drives the
    label-set-from-absent and version-upgrade arms of the preview walk.
    """
    client = _mock_client(get_lke_cluster={"id": 123})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_lke_cluster_update(
            {
                "cluster_id": "123",
                "label": "fresh",
                "k8s_version": "1.30",
                "dry_run": True,
            },
            sample_config,
        )

    body = json.loads(result[0].text)
    effects = body["side_effects"]
    assert "Label is set to 'fresh'." in effects
    assert any("Kubernetes version changes to '1.30'" in s for s in effects)
    client.update_lke_cluster.assert_not_called()


async def test_cluster_update_dry_run_skips_unchanged_version(
    sample_config: Config,
) -> None:
    """A matching k8s version produces no upgrade line in the preview."""
    client = _mock_client(get_lke_cluster={"id": 123, "k8s_version": "1.30"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_lke_cluster_update(
            {"cluster_id": "123", "k8s_version": "1.30", "dry_run": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert not any("Kubernetes version" in s for s in body.get("side_effects", []))


# --- ACL-update preview arms -----------------------------------------------


async def test_acl_update_dry_run_previews_disabled(sample_config: Config) -> None:
    """Disabling the ACL previews that the API becomes reachable from anywhere."""
    client = _mock_client(get_lke_control_plane_acl={"enabled": True})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_lke_acl_update(
            {"cluster_id": "123", "acl": {"enabled": False}, "dry_run": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert any("reachable from any address" in s for s in body["side_effects"])
    client.update_lke_control_plane_acl.assert_not_called()


async def test_acl_update_dry_run_previews_reconfigure(sample_config: Config) -> None:
    """An ACL without an enabled flag previews a plain address-list update."""
    client = _mock_client(get_lke_control_plane_acl={"enabled": True})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_lke_acl_update(
            {
                "cluster_id": "123",
                "acl": {"addresses": {"ipv4": ["1.2.3.4/32"]}},
                "dry_run": True,
            },
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["side_effects"] == [
        "The cluster control-plane ACL address list is updated."
    ]


# --- Cluster-create required-field guards ----------------------------------


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"label": "c"}, "region is required"),
        ({"label": "c", "region": "us-east"}, "k8s_version is required"),
        (
            {"label": "c", "region": "us-east", "k8s_version": "1.30"},
            "node_pools is required",
        ),
    ],
)
async def test_cluster_create_rejects_missing_required_field(
    arguments: dict[str, Any],
    expected: str,
    sample_config: Config,
) -> None:
    """Each required field has its own guard that fires before any API call.

    The handler validates label, then region, then k8s_version, then
    node_pools; supplying all-but-one surfaces exactly that field's message.
    """
    result = await handle_linode_lke_cluster_create(arguments, sample_config)
    assert len(result) == 1
    assert result[0].text == f"Error: {expected}"
