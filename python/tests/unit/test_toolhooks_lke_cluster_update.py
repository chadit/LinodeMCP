"""The preview hook linode_lke_cluster_update owns.

The Go twin is ``lke_cluster_update_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

_CLUSTER_PATH = "/lke/clusters/123"
_CURRENT = {"id": 123, "label": "before", "k8s_version": "1.30"}
_LABEL_CHANGE = 'Label changes from "before" to "after".'
_UPGRADE = (
    'Kubernetes version changes from "1.30" to "1.32"; '
    "the control plane and nodes upgrade."
)

pytestmark = pytest.mark.asyncio


async def test_cluster_update_preview_reports_an_unreadable_cluster(
    sample_config: Config,
) -> None:
    """A failed read is a tool error, not a preview naming no starting point."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_lke_cluster.side_effect = RuntimeError("cluster not found")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_lke_cluster_update_preview(
            sample_config,
            {"cluster_id": 123, "label": "after", "dry_run": True},
            "PUT",
            _CLUSTER_PATH,
            {"label": "after"},
        )

    assert "cluster not found" in result[0].text
