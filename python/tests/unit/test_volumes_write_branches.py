"""Validation, confirm, and dry-run branch coverage for volume write tools.

The documented-body happy paths live in ``test_tools.py``. This file drives
the guards those skip: the confirm gate, required-field checks on each verb,
label/size rejection, and the dry-run side-effect walks (attach note, resize
size change, update label/tag replacement).
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

from linodemcp.linode import Volume
from linodemcp.tools.linode_volumes_write import (
    handle_linode_volume_attach,
    handle_linode_volume_clone,
    handle_linode_volume_create,
    handle_linode_volume_delete,
    handle_linode_volume_detach,
    handle_linode_volume_resize,
    handle_linode_volume_update,
)

if TYPE_CHECKING:
    from linodemcp.config import Config


def _volume(**overrides: object) -> Volume:
    """Build a Volume read model with sensible defaults for walk tests."""
    base: dict[str, object] = {
        "id": 900,
        "label": "old-label",
        "status": "active",
        "size": 20,
        "region": "us-east",
        "linode_id": None,
        "linode_label": None,
        "filesystem_path": "/dev/disk/by-id/x",
        "tags": ["a"],
        "created": "2026-01-01T00:00:00",
        "updated": "2026-01-01T00:00:00",
        "hardware_type": "nvme",
    }
    base.update(overrides)
    return Volume(**base)  # type: ignore[arg-type]


def _mock_client_returning(method: str, value: object) -> AsyncMock:
    """Build a patched RetryableClient whose named coroutine returns value."""
    client = AsyncMock()
    getattr(client, method).return_value = value
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


# --- create --------------------------------------------------------------


async def test_create_dry_run_notes_attachment(sample_config: Config) -> None:
    """A dry-run create with linode_id adds the on-create attach note."""
    result = await handle_linode_volume_create(
        {"label": "vol", "size": 20, "linode_id": 42, "dry_run": True},
        sample_config,
    )
    body = json.loads(result[0].text)
    assert any("attached to instance 42" in s for s in body["side_effects"])


async def test_create_confirmed_requires_label(sample_config: Config) -> None:
    """confirm=true does not bypass the required label check."""
    result = await handle_linode_volume_create({"confirm": True}, sample_config)
    assert "label is required" in result[0].text


async def test_create_rejects_undersized_volume(sample_config: Config) -> None:
    """A size below the 10 GB floor aborts before the POST."""
    result = await handle_linode_volume_create(
        {"label": "vol", "region": "us-east", "size": 5, "confirm": True}, sample_config
    )
    assert "at least 10 GB" in result[0].text


async def test_create_threads_linode_id_into_body(sample_config: Config) -> None:
    """A confirmed create forwards linode_id in the POST body."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        client = _mock_client_returning(
            "post_raw", {"id": 1, "label": "vol", "region": "us-east"}
        )
        mock_cls.return_value = client

        await handle_linode_volume_create(
            {"label": "vol", "size": 20, "linode_id": 42, "confirm": True},
            sample_config,
        )

    client.post_raw.assert_awaited_once_with(
        "/volumes", {"label": "vol", "size": 20, "linode_id": 42}
    )


# --- clone ---------------------------------------------------------------


async def test_clone_dry_run_requires_volume_id(sample_config: Config) -> None:
    """A dry-run clone validates volume_id before fetching state."""
    result = await handle_linode_volume_clone(
        {"label": "copy", "dry_run": True}, sample_config
    )
    assert "volume_id is required" in result[0].text


async def test_clone_rejects_invalid_label(sample_config: Config) -> None:
    """A confirmed clone with an illegal label aborts before the POST."""
    result = await handle_linode_volume_clone(
        {"volume_id": 5, "label": "bad label!", "confirm": True}, sample_config
    )
    assert "invalid character" in result[0].text


# --- attach --------------------------------------------------------------


async def test_attach_dry_run_requires_linode_id(sample_config: Config) -> None:
    """A dry-run attach validates linode_id before fetching state."""
    result = await handle_linode_volume_attach(
        {"volume_id": 5, "dry_run": True}, sample_config
    )
    assert "linode_id is required" in result[0].text


async def test_attach_requires_volume_id(sample_config: Config) -> None:
    """A real attach with no volume_id errors before any call."""
    result = await handle_linode_volume_attach({"linode_id": 42}, sample_config)
    assert "volume_id is required" in result[0].text


async def test_attach_requires_linode_id(sample_config: Config) -> None:
    """A real attach with no linode_id errors before any call."""
    result = await handle_linode_volume_attach({"volume_id": 5}, sample_config)
    assert "linode_id is required" in result[0].text


async def test_attach_threads_config_id_into_body(sample_config: Config) -> None:
    """A real attach forwards config_id in the POST body when given."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        client = _mock_client_returning("post_raw", {"id": 5, "label": "vol"})
        mock_cls.return_value = client

        await handle_linode_volume_attach(
            {"volume_id": 5, "linode_id": 42, "config_id": 7, "confirm": True},
            sample_config,
        )

    client.post_raw.assert_awaited_once_with(
        "/volumes/5/attach",
        {"linode_id": 42, "config_id": 7},
    )


# --- detach --------------------------------------------------------------


async def test_detach_requires_volume_id(sample_config: Config) -> None:
    """A detach with no volume_id errors before any branch."""
    result = await handle_linode_volume_detach({}, sample_config)
    assert "volume_id is required" in result[0].text


# --- resize --------------------------------------------------------------


async def test_resize_dry_run_requires_volume_id(sample_config: Config) -> None:
    """A dry-run resize validates volume_id first."""
    result = await handle_linode_volume_resize(
        {"size": 40, "dry_run": True}, sample_config
    )
    assert "volume_id is required" in result[0].text


async def test_resize_dry_run_requires_size(sample_config: Config) -> None:
    """A dry-run resize validates size once volume_id is present."""
    result = await handle_linode_volume_resize(
        {"volume_id": 5, "dry_run": True}, sample_config
    )
    assert "size is required" in result[0].text


async def test_resize_confirmed_requires_size(sample_config: Config) -> None:
    """A confirmed resize still requires size."""
    result = await handle_linode_volume_resize(
        {"volume_id": 5, "confirm": True}, sample_config
    )
    assert "size is required" in result[0].text


async def test_resize_rejects_undersized_target(sample_config: Config) -> None:
    """A confirmed resize below the floor aborts before the POST."""
    result = await handle_linode_volume_resize(
        {"volume_id": 5, "size": 5, "confirm": True}, sample_config
    )
    assert "at least 10 GB" in result[0].text


async def test_resize_dry_run_reports_size_change(sample_config: Config) -> None:
    """The resize walk names the size change against fetched state."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        client = _mock_client_returning("get_volume", _volume(size=20))
        mock_cls.return_value = client

        result = await handle_linode_volume_resize(
            {"volume_id": 900, "size": 40, "dry_run": True}, sample_config
        )

    body = json.loads(result[0].text)
    assert any("from 20 GB to 40 GB" in s for s in body["side_effects"])


# --- update --------------------------------------------------------------


async def test_update_dry_run_requires_volume_id(sample_config: Config) -> None:
    """A dry-run update validates volume_id first."""
    result = await handle_linode_volume_update(
        {"label": "x", "dry_run": True}, sample_config
    )
    assert "volume_id is required" in result[0].text


async def test_update_dry_run_reports_label_and_tag_changes(
    sample_config: Config,
) -> None:
    """The update walk reports a label change and a tag-set replacement."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        client = _mock_client_returning("get_volume", _volume(label="old-label"))
        mock_cls.return_value = client

        result = await handle_linode_volume_update(
            {
                "volume_id": 900,
                "label": "new-label",
                "tags": ["prod"],
                "dry_run": True,
            },
            sample_config,
        )

    body = json.loads(result[0].text)
    assert any("Label changes" in s for s in body["side_effects"])
    assert any("tag set is replaced" in s for s in body["side_effects"])


async def test_update_rejects_invalid_label(sample_config: Config) -> None:
    """A confirmed update with an illegal label aborts before the PUT."""
    result = await handle_linode_volume_update(
        {"volume_id": 5, "label": "bad label!", "confirm": True}, sample_config
    )
    assert "invalid character" in result[0].text


# --- delete --------------------------------------------------------------


async def test_delete_plan_requires_volume_id(sample_config: Config) -> None:
    """A plan-mode delete validates volume_id in the two-stage entry."""
    result = await handle_linode_volume_delete({"mode": "plan"}, sample_config)
    assert "volume_id is required" in result[0].text


async def test_delete_dry_run_requires_volume_id(sample_config: Config) -> None:
    """A dry-run delete validates volume_id before fetching state."""
    result = await handle_linode_volume_delete({"dry_run": True}, sample_config)
    assert "volume_id is required" in result[0].text


async def test_delete_confirmed_requires_volume_id(sample_config: Config) -> None:
    """A confirmed delete still requires volume_id."""
    result = await handle_linode_volume_delete({"confirm": True}, sample_config)
    assert "volume_id is required" in result[0].text
