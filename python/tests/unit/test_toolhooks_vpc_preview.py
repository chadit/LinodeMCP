"""The four preview hooks the VPC write tools own.

The Go twin is ``vpc_preview_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

_VPC_PATH = "/vpcs/5"
_SUBNETS_PATH = "/vpcs/5/subnets"
_SUBNET_PATH = "/vpcs/5/subnets/7"
_VPC_STATE = {"id": 5, "label": "old-label", "description": "", "region": "us-east"}
_SUBNET_STATE = {"id": 7, "label": "subnet-a", "ipv4": "10.0.0.0/24"}
_LABEL_CHANGE = 'Label changes from "old-label" to "after".'
_DESCRIPTION_EFFECT = "The VPC description is updated."
_SUBNET_RANGE = "10.0.0.0/24"

pytestmark = pytest.mark.asyncio


def _client(**attrs: Any) -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    for name, value in attrs.items():
        setattr(client, name, value)
    return client


async def test_vpc_update_preview_reads_an_unlabeled_vpc_as_a_set(
    sample_config: Config,
) -> None:
    """A VPC carrying no label reads as a label being set, not replaced."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _client()
        mock_client.get_vpc.return_value = {"id": 5, "label": ""}
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_vpc_update_preview(
            sample_config,
            {"vpc_id": 5, "label": "after", "dry_run": True},
            "PUT",
            _VPC_PATH,
            {"label": "after"},
        )

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["side_effects"] == ['Label is set to "after".']


async def test_vpc_update_preview_reports_an_unreadable_vpc(
    sample_config: Config,
) -> None:
    """A failed read is a tool error, not a preview naming no starting point."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _client()
        mock_client.get_vpc.side_effect = RuntimeError("vpc not found")
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_vpc_update_preview(
            sample_config,
            {"vpc_id": 5, "label": "after", "dry_run": True},
            "PUT",
            _VPC_PATH,
            {"label": "after"},
        )

    assert "vpc not found" in result[0].text
