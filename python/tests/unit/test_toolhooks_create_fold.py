"""The hooks the two folding create tools own.

The Go twin is ``create_fold_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

_FIREWALLS_PATH = "/networking/firewalls"
_INSTANCES_PATH = "/linode/instances"
_BILLING_WARNING = "Billing for the instance starts immediately on creation."


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        (
            {"label": "web-fw", "dry_run": True},
            (
                'A new Cloud Firewall "web-fw" will be created with inbound '
                "policy ACCEPT and outbound policy ACCEPT."
            ),
        ),
        (
            {"label": "web-fw", "inbound_policy": "DROP", "dry_run": True},
            (
                'A new Cloud Firewall "web-fw" will be created with inbound '
                "policy DROP and outbound policy ACCEPT."
            ),
        ),
        (
            {
                "label": "edge",
                "inbound_policy": "DROP",
                "outbound_policy": "DROP",
                "dry_run": True,
            },
            (
                'A new Cloud Firewall "edge" will be created with inbound '
                "policy DROP and outbound policy DROP."
            ),
        ),
    ],
)
async def test_firewall_create_preview_names_the_policies_the_fold_sends(
    sample_config: Config, arguments: dict[str, Any], want: str
) -> None:
    """The sentence reports the same defaults the body folds into rules, so a
    caller who named no policy is told which one the create would carry.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = await toolhooks.linode_firewall_create_preview(
            sample_config,
            arguments,
            "POST",
            _FIREWALLS_PATH,
            {"label": arguments["label"]},
        )

        mock_cls.assert_not_called()

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["would_execute"]["path"] == _FIREWALLS_PATH
    assert preview["current_state"] is None
    assert preview["side_effects"] == [want]


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        (
            {
                "region": "us-east",
                "type": "g6-nanode-1",
                "firewall_id": 123,
                "dry_run": True,
            },
            "A new g6-nanode-1 instance will be created in region us-east.",
        ),
        (
            {
                "region": "us-east",
                "type": "g6-nanode-1",
                "firewall_id": 123,
                "image": "linode/debian11",
                "dry_run": True,
            },
            (
                "A new g6-nanode-1 instance will be created in region us-east "
                "from image linode/debian11."
            ),
        ),
    ],
)
async def test_instance_create_preview_describes_the_instance_and_its_billing(
    sample_config: Config, arguments: dict[str, Any], want: str
) -> None:
    """The image is named only when the caller chose one, and the billing
    warning rides on every answer.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        result = await toolhooks.linode_instance_create_preview(
            sample_config,
            arguments,
            "POST",
            _INSTANCES_PATH,
            {"region": arguments["region"], "type": arguments["type"]},
        )

        mock_cls.assert_not_called()

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["would_execute"]["path"] == _INSTANCES_PATH
    assert preview["current_state"] is None
    assert preview["side_effects"] == [want]
    assert preview["warnings"] == [_BILLING_WARNING]
