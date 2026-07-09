"""Parser and validation branch coverage for VPC write tools.

The tool handlers get exercised in ``test_tools.py``. This file drives the
argument parsers and side-effect walks through the public handlers: integer
parsing failures, required-field errors, the update label/description walk,
and the delete dependency-walk's best-effort fallback when subnet listing
fails.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

from linodemcp.linode import APIError
from linodemcp.tools.linode_vpc_write import (
    handle_linode_ipv6_range_create,
    handle_linode_vpc_create,
    handle_linode_vpc_delete,
    handle_linode_vpc_subnet_create,
    handle_linode_vpc_subnet_update,
    handle_linode_vpc_update,
)

if TYPE_CHECKING:
    from linodemcp.config import Config


def _cm_client() -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def test_subnet_update_rejects_non_integer_vpc(sample_config: Config) -> None:
    """A non-numeric vpc_id yields an integer-parse error."""
    result = await handle_linode_vpc_subnet_update({"vpc_id": "abc"}, sample_config)
    assert "vpc_id must be a valid integer" in result[0].text


async def test_subnet_update_requires_subnet(sample_config: Config) -> None:
    """A valid vpc_id with no subnet_id reports the missing subnet."""
    result = await handle_linode_vpc_subnet_update({"vpc_id": "1"}, sample_config)
    assert "subnet_id is required" in result[0].text


async def test_subnet_update_rejects_non_integer_subnet(sample_config: Config) -> None:
    """A non-numeric subnet_id yields an integer-parse error."""
    result = await handle_linode_vpc_subnet_update(
        {"vpc_id": "1", "subnet_id": "xyz"}, sample_config
    )
    assert "subnet_id must be a valid integer" in result[0].text


async def test_subnet_update_valid_ids_reach_label_check(sample_config: Config) -> None:
    """Two valid ids parse, so the handler advances to the label check. That it
    reports the missing label proves the id parse returned the id tuple rather
    than an error."""
    result = await handle_linode_vpc_subnet_update(
        {"vpc_id": "1", "subnet_id": "2"}, sample_config
    )
    assert "label is required" in result[0].text


async def test_ipv6_range_create_rejects_non_integer_prefix(
    sample_config: Config,
) -> None:
    """A non-numeric prefix_length is rejected with the 56/64 hint."""
    result = await handle_linode_ipv6_range_create(
        {"prefix_length": "abc", "dry_run": True}, sample_config
    )
    assert "prefix_length must be 56 or 64" in result[0].text


async def test_vpc_create_requires_region(sample_config: Config) -> None:
    """A label with no region reports the missing region."""
    result = await handle_linode_vpc_create(
        {"label": "my-vpc", "dry_run": True}, sample_config
    )
    assert "region is required" in result[0].text


async def test_subnet_create_rejects_non_integer_vpc(sample_config: Config) -> None:
    """A non-numeric vpc_id in subnet-create is an integer-parse error."""
    result = await handle_linode_vpc_subnet_create(
        {"vpc_id": "abc", "label": "sub", "ipv4": "10.0.0.0/24", "dry_run": True},
        sample_config,
    )
    assert "vpc_id must be a valid integer" in result[0].text


async def test_subnet_create_requires_ipv4(sample_config: Config) -> None:
    """A valid vpc_id and label but no ipv4 reports the missing ipv4."""
    result = await handle_linode_vpc_subnet_create(
        {"vpc_id": "1", "label": "sub", "ipv4": "", "dry_run": True},
        sample_config,
    )
    assert "ipv4 is required" in result[0].text


async def test_update_dry_run_walk_sets_label_and_notes_description(
    sample_config: Config,
) -> None:
    """The update dry-run walk phrases an unchanged label as a set and notes a
    description edit in the preview's side_effects."""
    client = _cm_client()
    client.get_vpc.return_value = {"label": "same"}

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_vpc_update(
            {
                "vpc_id": "5",
                "label": "same",
                "description": "new desc",
                "dry_run": True,
            },
            sample_config,
        )

    payload = json.loads(result[0].text)
    effects = payload["side_effects"]
    assert any("Label is set to" in s for s in effects)
    assert any("description is updated" in s for s in effects)


async def test_update_handler_rejects_non_integer_vpc_id(
    sample_config: Config,
) -> None:
    """The update handler rejects a non-numeric vpc_id before dispatch."""
    result = await handle_linode_vpc_update({"vpc_id": "abc"}, sample_config)
    assert "vpc_id must be a valid integer" in result[0].text


async def test_delete_dry_run_degrades_on_subnet_list_failure(
    sample_config: Config,
) -> None:
    """A failed subnet list becomes a warning in the delete preview instead of
    raising."""
    client = _cm_client()
    client.get_vpc.return_value = {"id": 5}
    client.list_vpc_subnets.side_effect = APIError(503, "unavailable")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_vpc_delete(
            {"vpc_id": "5", "dry_run": True}, sample_config
        )

    payload = json.loads(result[0].text)
    assert any("Could not list VPC subnets" in w for w in payload["warnings"])
