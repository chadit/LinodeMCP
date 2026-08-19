"""Parser and validation branch coverage for the VPC destroy tools.

The tool handlers get exercised in ``test_tools.py``. This file drives the
argument parsers and dependency walks the two destroys still own by hand,
including the delete walk's best-effort fallback when subnet listing fails.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

from linodemcp.gentools import handle_linode_vpc_delete
from linodemcp.linode import APIError

if TYPE_CHECKING:
    from linodemcp.config import Config


def _cm_client() -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def test_delete_dry_run_degrades_on_subnet_list_failure(
    sample_config: Config,
) -> None:
    """A failed subnet list becomes a warning in the delete preview instead of
    raising."""
    client = _cm_client()
    client.route_raw.return_value = {"id": 5}
    client.list_vpc_subnets.side_effect = APIError(503, "unavailable")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_vpc_delete(
            {"vpc_id": 5, "dry_run": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert any("Could not list VPC subnets" in w for w in payload["warnings"])
