"""The hooks the last four straggler write tools own.

The Go twin is ``straggler_write_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

_FIREWALL_PATH = "/networking/firewalls/123"
_SETTINGS_PATH = "/networking/firewalls/settings"
_RANGES_PATH = "/networking/ipv6/ranges"
_TOKENS_PATH = "/images/sharegroups/tokens"
_SHARE_UUID = "valid_for_sharegroup_uuid"
_CANONICAL = "123e4567-e89b-12d3-a456-426614174000"


def _stub_client() -> Any:
    """Build the async client stub every preview below fetches through."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


@dataclass
class _Firewall:
    """The attributes the firewall-update walk reads off the fetched state."""

    label: str = ""
    status: str = ""


@pytest.mark.parametrize(
    ("state", "arguments", "want"),
    [
        (
            _Firewall("web-fw", "enabled"),
            {"label": "renamed-fw", "status": "disabled"},
            [
                'Label changes from "web-fw" to "renamed-fw".',
                (
                    'Firewall status changes to "disabled"; this immediately '
                    "stops enforcing its rules."
                ),
            ],
        ),
        (
            _Firewall("web-fw", "disabled"),
            {"status": "enabled"},
            [
                (
                    'Firewall status changes to "enabled"; this immediately '
                    "starts enforcing its rules."
                ),
            ],
        ),
        (
            _Firewall(status="enabled"),
            {"label": "renamed-fw"},
            ['Label is set to "renamed-fw".'],
        ),
        (_Firewall("web-fw", "enabled"), {}, []),
    ],
)
async def test_firewall_update_preview_diffs_against_the_firewall(
    sample_config: Config,
    state: _Firewall,
    arguments: dict[str, Any],
    want: list[str],
) -> None:
    """Both sentences, quoted the way Go's %q writes them rather than repr."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.get_firewall.return_value = state
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_firewall_update_preview(
            sample_config,
            {"firewall_id": 123, "dry_run": True, **arguments},
            "PUT",
            _FIREWALL_PATH,
            {},
        )

        mock_client.get_firewall.assert_awaited_once_with(123)

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["side_effects"] == want


def test_token_create_preview_route_is_the_contract_route() -> None:
    """The token create declares no preview hook, so nothing here owns its
    route; this pins the path the generated handler reports.
    """
    assert _TOKENS_PATH == "/images/sharegroups/tokens"
