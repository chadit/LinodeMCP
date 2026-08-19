"""The three hooks linode_instance_resize owns: its preview, the state a plan
hashes, and the walk that words the change.

The Go twin is ``instance_resize_hooks_test.go`` in ``go/internal/toolhooks``,
and every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks
from linodemcp.linode import APIError, parse_instance

if TYPE_CHECKING:
    from linodemcp.config import Config

_RESIZE_PATH = "/linode/instances/123/resize"
_FROM_TYPE = "g6-standard-1"
_TARGET_TYPE = "g6-standard-2"
_BILLING_NOTE = "Resizing changes the monthly price to match the new type."
_BOTH_TYPES = (
    "Instance resizes from type g6-standard-1 to g6-standard-2; it reboots and"
    " is unavailable during the resize."
)
_TARGET_ONLY = (
    "Instance resizes to type g6-standard-2; it reboots and is unavailable"
    " during the resize."
)

pytestmark = pytest.mark.asyncio


def _client(**attrs: Any) -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    for name, value in attrs.items():
        setattr(client, name, value)
    return client


@pytest.mark.parametrize(
    ("from_type", "want"),
    [(_FROM_TYPE, _BOTH_TYPES), ("", _TARGET_ONLY)],
    ids=["current type known", "current type unknown"],
)
async def test_preview_words_both_types(
    sample_config: Config, from_type: str, want: str
) -> None:
    """The two sentences the change can be described by: the type it leaves
    known, and a read that answered without one."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _client()
        mock_client.get_instance.return_value = parse_instance(
            {"id": 123, "label": "web-1", "type": from_type}
        )
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_instance_resize_preview(
            sample_config,
            {"instance_id": 123, "type": _TARGET_TYPE, "dry_run": True},
            "POST",
            _RESIZE_PATH,
            {"type": _TARGET_TYPE},
        )

        mock_client.get_instance.assert_awaited_once_with(123)

    envelope: dict[str, Any] = json.loads(result[0].text)

    assert envelope["side_effects"] == [want]
    assert envelope["warnings"] == [_BILLING_NOTE]
    # The preview reports the Linode the resize is about, not the projection a
    # plan hashes.
    assert envelope["current_state"]["label"] == "web-1"


async def test_fetch_state_carries_the_disks() -> None:
    """A plan hashing the instance alone would apply over a disk layout that
    changed under it, which allow_auto_disk_resize moves as part of the plan."""
    client = _client()
    client.get_instance.return_value = parse_instance({"id": 123, "type": _FROM_TYPE})
    client.list_instance_disks.return_value = [
        {"id": 456, "size": 25600, "filesystem": "ext4"}
    ]

    state = await toolhooks.linode_instance_resize_fetch_state(client, 123)

    assert state == {
        "type": _FROM_TYPE,
        "disks": [{"id": 456, "size": 25600, "filesystem": "ext4"}],
    }


async def test_fetch_state_reports_a_failed_disk_list() -> None:
    """A plan that hashed a partial projection would compare it against a full
    one at apply time and refuse a resource nothing changed."""
    client = _client()
    client.get_instance.return_value = parse_instance({"id": 123, "type": _FROM_TYPE})
    client.list_instance_disks.side_effect = APIError(500, "disks unavailable")

    with pytest.raises(ValueError, match="list disks for resize plan"):
        await toolhooks.linode_instance_resize_fetch_state(client, 123)


@pytest.mark.parametrize(
    ("state", "want"),
    [
        ({"type": _FROM_TYPE, "disks": []}, _BOTH_TYPES),
        (parse_instance({"id": 123, "type": _FROM_TYPE}), _BOTH_TYPES),
        (None, _TARGET_ONLY),
    ],
    ids=["the projection a plan hashes", "the instance a preview reports", "no type"],
)
async def test_dependency_walk_reads_both_state_shapes(state: Any, want: str) -> None:
    """The walk words the change from the projection a plan hashes as readily as
    from the instance a preview reports."""
    details = await toolhooks.linode_instance_resize_dependency_walk(
        _client(), {"instance_id": 123, "type": _TARGET_TYPE}, state
    )

    assert details == {"side_effects": [want], "warnings": [_BILLING_NOTE]}
