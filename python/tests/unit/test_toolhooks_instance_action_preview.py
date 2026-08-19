"""The preview hooks the instance migrate, mutate, and backup-restore tools own.

The Go twin is ``instance_action_preview_test.go`` in ``go/internal/toolhooks``,
and every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks
from linodemcp.linode import instance_preview_state, parse_instance

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable

    from mcp.types import TextContent

    from linodemcp.config import Config

_INSTANCE_PATH = "/linode/instances/123"
_MIGRATE_PATH = f"{_INSTANCE_PATH}/migrate"
_MUTATE_PATH = f"{_INSTANCE_PATH}/mutate"
_FROM_REGION = "us-east"
_TARGET_REGION = "us-west"
_FROM_TYPE = "g6-standard-1"
pytestmark = pytest.mark.asyncio


def _client(**attrs: Any) -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    for name, value in attrs.items():
        setattr(client, name, value)
    return client


async def _run_instance_preview(
    hook: Callable[..., Awaitable[list[TextContent]]],
    cfg: Config,
    arguments: dict[str, Any],
    path: str,
    body: dict[str, Any] | None,
    instance: Any,
) -> dict[str, Any]:
    """Call one instance-reading preview against a mocked client."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _client()
        mock_client.get_instance.return_value = instance
        mock_cls.return_value = mock_client

        result = await hook(cfg, arguments, "POST", path, body)

        mock_client.get_instance.assert_awaited_once_with(123)

    envelope: dict[str, Any] = json.loads(result[0].text)
    return envelope


@pytest.mark.parametrize(
    ("region_state", "target", "want"),
    [
        (
            _FROM_REGION,
            _TARGET_REGION,
            (
                "Instance migrates from region us-east to us-west; it is"
                " unavailable during the migration."
            ),
        ),
        (
            "",
            _TARGET_REGION,
            (
                "Instance migrates to region us-west; it is unavailable during"
                " the migration."
            ),
        ),
        (
            _FROM_REGION,
            "",
            "Instance migrates; it is unavailable during the migration.",
        ),
    ],
)
async def test_migrate_preview_words_every_destination(
    sample_config: Config, region_state: str, target: str, want: str
) -> None:
    """Both regions known, only the target known, and neither, which is a caller
    letting Linode pick from an instance the read answered nothing about.
    """
    instance = parse_instance({"id": 123, "label": "web-1", "region": region_state})
    arguments: dict[str, Any] = {"linode_id": 123, "dry_run": True}
    if target:
        arguments["region"] = target

    preview = await _run_instance_preview(
        toolhooks.linode_instance_migrate_preview,
        sample_config,
        arguments,
        _MIGRATE_PATH,
        {"region": target} if target else {},
        instance,
    )

    assert preview["tool"] == "linode_instance_migrate"
    assert preview["current_state"] == instance_preview_state(instance)
    assert preview["side_effects"] == [want]
    assert preview.get("warnings", []) == []


@pytest.mark.parametrize(
    ("type_state", "want"),
    [
        (
            _FROM_TYPE,
            (
                "Instance type g6-standard-1 upgrades to the latest generation;"
                " it reboots during the upgrade."
            ),
        ),
        (
            "",
            (
                "Instance upgrades to the latest generation of its type; it"
                " reboots during the upgrade."
            ),
        ),
    ],
)
async def test_mutate_preview_carries_both_halves(
    sample_config: Config, type_state: str, want: str
) -> None:
    """Go's preview named the type and Python's named the downtime, so the
    warning stands whether or not the read answered with a type.
    """
    instance = parse_instance({"id": 123, "label": "web-1", "type": type_state})

    preview = await _run_instance_preview(
        toolhooks.linode_instance_mutate_preview,
        sample_config,
        {"linode_id": 123, "allow_auto_disk_resize": True, "dry_run": True},
        _MUTATE_PATH,
        {"allow_auto_disk_resize": True},
        instance,
    )

    assert preview["tool"] == "linode_instance_mutate"
    assert preview["current_state"] == instance_preview_state(instance)
    assert preview["would_execute"]["body"] == {"allow_auto_disk_resize": True}
    assert preview["side_effects"] == [want]
    assert preview["warnings"] == ["The Linode may be unavailable during the upgrade."]
