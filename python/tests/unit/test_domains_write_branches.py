"""Validation, confirm, and dry-run branch coverage for domain write tools.

Happy paths live in ``test_tools.py``. This file drives the guards those
skip: the confirm gates, required-field checks, label rejection, the domain
update side-effect walk (name/SOA/description), the delete dependency-walk
error fallback, and the body-building omit rules.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

from linodemcp.gentools import (
    handle_linode_domain_clone,
    handle_linode_domain_create,
    handle_linode_domain_delete,
    handle_linode_domain_update,
)
from linodemcp.linode import APIError

if TYPE_CHECKING:
    from linodemcp.config import Config


def _patch_client(**attrs: object) -> AsyncMock:
    """Build an AsyncMock RetryableClient with the given attrs configured."""
    client = AsyncMock()
    for name, value in attrs.items():
        if isinstance(value, Exception):
            getattr(client, name).side_effect = value
        else:
            getattr(client, name).return_value = value
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def test_clone_rejects_invalid_label(sample_config: Config) -> None:
    """A clone whose domain name has an illegal character aborts."""
    result = await handle_linode_domain_clone(
        {"domain_id": 5, "domain": "bad domain!", "confirm": True}, sample_config
    )
    assert "invalid character" in result[0].text


async def test_create_requires_confirm(sample_config: Config) -> None:
    """A real create without confirm asks for confirm."""
    result = await handle_linode_domain_create({"domain": "x.com"}, sample_config)
    assert "confirm=true" in result[0].text


async def test_create_confirmed_requires_domain(sample_config: Config) -> None:
    """confirm=true does not bypass the required domain check."""
    result = await handle_linode_domain_create({"confirm": True}, sample_config)
    assert "domain is required" in result[0].text


async def test_create_threads_description_into_body(sample_config: Config) -> None:
    """A confirmed create forwards a description in the POST body."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        client = _patch_client(route_raw={"id": 1, "domain": "x.com"})
        mock_cls.return_value = client

        await handle_linode_domain_create(
            {
                "domain": "x.com",
                "type": "master",
                "soa_email": "admin@example.com",
                "description": "my zone",
                "confirm": True,
            },
            sample_config,
        )

    client.route_raw.assert_awaited_once_with(
        "linode_domain_create",
        body={
            "domain": "x.com",
            "type": "master",
            "soa_email": "admin@example.com",
            "description": "my zone",
        },
        retry=False,
    )


async def test_update_dry_run_requires_domain_id(sample_config: Config) -> None:
    """A dry-run update validates domain_id before fetching state."""
    result = await handle_linode_domain_update({"dry_run": True}, sample_config)
    assert "domain_id must be a positive integer" in result[0].text


async def test_update_rejects_negative_domain_id(sample_config: Config) -> None:
    """A negative id decodes as an int, so only the positivity rule stops it."""
    result = await handle_linode_domain_update(
        {"domain_id": -5, "description": "updated desc", "confirm": True},
        sample_config,
    )
    assert "domain_id must be a positive integer" in result[0].text


async def test_update_requires_confirm(sample_config: Config) -> None:
    """A real update without confirm asks for confirm."""
    result = await handle_linode_domain_update({"domain_id": 5}, sample_config)
    assert "confirm=true" in result[0].text


async def test_update_confirmed_requires_domain_id(sample_config: Config) -> None:
    """confirm=true does not bypass the required domain_id check."""
    result = await handle_linode_domain_update({"confirm": True}, sample_config)
    assert "domain_id must be a positive integer" in result[0].text


async def test_update_body_omits_absent_and_keeps_present(
    sample_config: Config,
) -> None:
    """The PUT body carries domain and soa_email, omitting the rest."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        client = _patch_client(route_raw={"id": 5, "domain": "new.example.com"})
        mock_cls.return_value = client

        await handle_linode_domain_update(
            {
                "domain_id": 5,
                "domain": "new.example.com",
                "soa_email": "new@example.com",
                "confirm": True,
            },
            sample_config,
        )

    client.route_raw.assert_awaited_once_with(
        "linode_domain_update",
        5,
        body={"domain": "new.example.com", "soa_email": "new@example.com"},
    )


async def test_delete_plan_requires_domain_id(sample_config: Config) -> None:
    """A plan-mode delete validates domain_id in the two-stage entry."""
    result = await handle_linode_domain_delete({"mode": "plan"}, sample_config)
    assert "domain_id is required" in result[0].text


async def test_delete_dry_run_requires_domain_id(sample_config: Config) -> None:
    """A dry-run delete validates domain_id before fetching state."""
    result = await handle_linode_domain_delete({"dry_run": True}, sample_config)
    assert "domain_id is required" in result[0].text


async def test_delete_dry_run_walk_survives_record_list_failure(
    sample_config: Config,
) -> None:
    """When listing records fails, the walk degrades to a warning, not an error."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:

        def routed(tool: str, *_values: object, **_kwargs: object) -> dict[str, object]:
            if tool == "linode_domain_record_list":
                raise APIError(500, "boom")
            return {}

        client = _patch_client()
        client.route_raw.side_effect = routed
        mock_cls.return_value = client

        result = await handle_linode_domain_delete(
            {"domain_id": 5, "dry_run": True}, sample_config
        )

    warnings = json.loads(result[0].text)["warnings"]
    assert any("Could not list domain records" in w for w in warnings)


async def test_delete_requires_confirm(sample_config: Config) -> None:
    """A real delete without confirm asks for confirm."""
    result = await handle_linode_domain_delete({"domain_id": 5}, sample_config)
    assert "confirm=true" in result[0].text


async def test_delete_confirmed_requires_domain_id(sample_config: Config) -> None:
    """confirm=true does not bypass the required domain_id check."""
    result = await handle_linode_domain_delete({"confirm": True}, sample_config)
    assert "domain_id is required" in result[0].text
