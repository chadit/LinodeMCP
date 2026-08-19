"""The preview hooks the instance rescue and interface-settings tools own.

The Go twin is ``instance_rescue_settings_preview_test.go`` in
``go/internal/toolhooks``, and every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks
from linodemcp.linode import instance_preview_state, parse_instance

if TYPE_CHECKING:
    from linodemcp.config import Config

_RESCUE_PATH = "/linode/instances/123/rescue"
_SETTINGS_PATH = "/linode/instances/123/interfaces/settings"
_RESCUE_EFFECT = (
    "The instance reboots into rescue mode; its normal boot configuration is "
    "bypassed until you reboot out of rescue mode."
)
_RESCUE_RUNNING_WARNING = (
    "Instance is currently running; entering rescue mode reboots it, causing downtime."
)
_SETTINGS_EFFECT = "Interface settings for Linode 123 will be updated."
_SETTINGS_STATE = {"default_route": {"ipv4_interface_id": 4}, "network_helper": True}

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
    ("status", "want_warnings"),
    [
        ("running", [_RESCUE_RUNNING_WARNING]),
        ("offline", []),
    ],
)
async def test_rescue_preview_warns_only_when_running(
    sample_config: Config, status: str, want_warnings: list[str]
) -> None:
    """A Linode that is up loses service to the rescue boot, and one that is
    already down has nothing to warn about.
    """
    instance = parse_instance({"id": 123, "label": "web-1", "status": status})

    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _client()
        mock_client.get_instance.return_value = instance
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_instance_rescue_preview(
            sample_config,
            {"linode_id": 123, "dry_run": True},
            "POST",
            _RESCUE_PATH,
            {},
        )

        mock_client.get_instance.assert_awaited_once_with(123)

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["tool"] == "linode_instance_rescue"
    assert preview["current_state"] == instance_preview_state(instance)
    assert preview["side_effects"] == [_RESCUE_EFFECT]
    assert preview.get("warnings", []) == want_warnings
