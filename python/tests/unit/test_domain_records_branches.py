"""Validation, confirm, and dry-run branch coverage for domain record tools.

The happy paths live in ``test_tools.py``; this file drives the error and
preview branches the main suite skips: missing IDs, the confirm gate, DNS
name/target rejection, and the dry-run side-effect walks for update/delete.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

from linodemcp.tools.linode_domain_records import (
    handle_linode_domain_record_create,
    handle_linode_domain_record_delete,
    handle_linode_domain_record_get,
    handle_linode_domain_record_update,
)

if TYPE_CHECKING:
    from linodemcp.config import Config


async def test_get_missing_domain_id(sample_config: Config) -> None:
    """domain_record_get with no domain_id errors before any API call."""
    result = await handle_linode_domain_record_get({"record_id": 5}, sample_config)
    assert "domain_id is required" in result[0].text


async def test_create_dry_run_requires_type(sample_config: Config) -> None:
    """A dry-run create validates the type the real call would need."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "dry_run": True}, sample_config
    )
    assert "type is required" in result[0].text


async def test_create_dry_run_names_the_host(sample_config: Config) -> None:
    """A dry-run create with a name echoes that host in the side effect."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "type": "A", "name": "www", "dry_run": True},
        sample_config,
    )
    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert any("'www'" in effect for effect in body["side_effects"])


async def test_create_requires_confirm(sample_config: Config) -> None:
    """Without confirm or dry_run, create refuses and asks for confirm."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "type": "A"}, sample_config
    )
    assert "confirm=true" in result[0].text


async def test_create_confirmed_still_requires_type(sample_config: Config) -> None:
    """confirm=true does not bypass the required-field check for type."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "confirm": True}, sample_config
    )
    assert "type is required" in result[0].text


async def test_create_rejects_invalid_name(sample_config: Config) -> None:
    """An illegal DNS name aborts the create with a validation error."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "type": "A", "name": "bad name!", "confirm": True},
        sample_config,
    )
    assert "invalid DNS record name" in result[0].text


async def test_create_rejects_invalid_a_target(sample_config: Config) -> None:
    """An A record with a non-IPv4 target aborts before the POST."""
    result = await handle_linode_domain_record_create(
        {"domain_id": 333, "type": "A", "target": "not-an-ip", "confirm": True},
        sample_config,
    )
    assert "A record target must be a valid IPv4 address" in result[0].text


async def test_update_dry_run_missing_domain_id(sample_config: Config) -> None:
    """A dry-run update validates domain_id before fetching state."""
    result = await handle_linode_domain_record_update(
        {"record_id": 555, "dry_run": True}, sample_config
    )
    assert "domain_id is required" in result[0].text


async def test_update_dry_run_missing_record_id(sample_config: Config) -> None:
    """A dry-run update validates record_id before fetching state."""
    result = await handle_linode_domain_record_update(
        {"domain_id": 333, "dry_run": True}, sample_config
    )
    assert "record_id is required" in result[0].text


async def test_update_dry_run_reports_name_change(sample_config: Config) -> None:
    """The dry-run walk reports a record name change against fetched state."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_domain_record.return_value = {"id": 555, "name": "old"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_domain_record_update(
            {
                "domain_id": 333,
                "record_id": 555,
                "name": "new",
                "dry_run": True,
            },
            sample_config,
        )

    body = json.loads(result[0].text)
    assert any("name changes" in effect for effect in body["side_effects"])
    mock_client.get_domain_record.assert_awaited_once_with(333, 555)


async def test_update_requires_confirm(sample_config: Config) -> None:
    """Without confirm or dry_run, update refuses and asks for confirm."""
    result = await handle_linode_domain_record_update(
        {"domain_id": 333, "record_id": 555}, sample_config
    )
    assert "confirm=true" in result[0].text


async def test_update_confirmed_still_requires_record_id(
    sample_config: Config,
) -> None:
    """confirm=true does not bypass the required record_id check."""
    result = await handle_linode_domain_record_update(
        {"domain_id": 333, "confirm": True}, sample_config
    )
    assert "record_id is required" in result[0].text


async def test_update_rejects_invalid_name(sample_config: Config) -> None:
    """A confirmed update with an illegal DNS name aborts before the PUT."""
    result = await handle_linode_domain_record_update(
        {
            "domain_id": 333,
            "record_id": 555,
            "name": "bad name!",
            "confirm": True,
        },
        sample_config,
    )
    assert "invalid DNS record name" in result[0].text


async def test_delete_missing_domain_id(sample_config: Config) -> None:
    """delete errors on a missing domain_id before any branch."""
    result = await handle_linode_domain_record_delete({}, sample_config)
    assert "domain_id is required" in result[0].text


async def test_delete_missing_record_id(sample_config: Config) -> None:
    """delete errors on a missing record_id before any branch."""
    result = await handle_linode_domain_record_delete({"domain_id": 333}, sample_config)
    assert "record_id is required" in result[0].text


async def test_delete_dry_run_previews_via_get(sample_config: Config) -> None:
    """A dry-run delete fetches state via GET and does not delete."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_domain_record.return_value = {"id": 555, "type": "A"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await handle_linode_domain_record_delete(
            {"domain_id": 333, "record_id": 555, "dry_run": True},
            sample_config,
        )

    body = json.loads(result[0].text)
    assert body["dry_run"] is True
    assert body["would_execute"]["method"] == "DELETE"
    assert body["would_execute"]["path"] == "/domains/333/records/555"
    mock_client.get_domain_record.assert_awaited_once_with(333, 555)
    mock_client.delete_domain_record.assert_not_called()


async def test_delete_requires_confirm(sample_config: Config) -> None:
    """Without confirm or dry_run, delete refuses and asks for confirm."""
    result = await handle_linode_domain_record_delete(
        {"domain_id": 333, "record_id": 555}, sample_config
    )
    assert "confirm=true" in result[0].text
