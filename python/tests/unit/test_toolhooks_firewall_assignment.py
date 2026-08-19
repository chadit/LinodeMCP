"""The hooks the two Cloud Firewall assignment replacements own.

The Go twin is ``firewall_assignment_test.go`` in ``go/internal/toolhooks``,
and every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

_INSTANCE_PATH = "/linode/instances/123/firewalls"
_BALANCER_PATH = "/nodebalancers/5/firewalls"
_PAGE: dict[str, Any] = {
    "data": [{"id": 9, "label": "edge", "status": "enabled"}],
    "page": 2,
    "pages": 2,
    "results": 1,
}
_CANONICAL: list[dict[str, Any]] = [
    {
        "id": 9,
        "label": "edge",
        "status": "enabled",
        "tags": [],
        "created": "",
        "updated": "",
    }
]


def _stub_client() -> Any:
    """Build the async client stub every preview below fetches through."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def test_nodebalancer_firewall_update_preview_reads_its_own_firewalls(
    sample_config: Config,
) -> None:
    """The two convergences the twin carries: this language read the
    NodeBalancer where the state is its firewalls, and neither said anything
    about the change.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.list_nodebalancer_firewalls.return_value = _PAGE
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_nodebalancer_firewall_update_preview(
            sample_config,
            {"nodebalancer_id": 5, "firewall_ids": [1], "dry_run": True},
            "PUT",
            _BALANCER_PATH,
            {"firewall_ids": [1]},
        )

        mock_client.list_nodebalancer_firewalls.assert_awaited_once_with(
            5, page=None, page_size=None
        )
        mock_client.get_nodebalancer.assert_not_called()

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["would_execute"]["path"] == _BALANCER_PATH
    assert preview["current_state"] == _CANONICAL
    assert preview["side_effects"] == []


@pytest.mark.parametrize(
    "answer",
    [
        pytest.param([], id="not an envelope"),
        pytest.param({"page": 1}, id="no data member"),
        pytest.param({"data": "unexpected"}, id="data is not a page"),
        pytest.param({"data": [None]}, id="element is not an object"),
    ],
)
async def test_instance_firewall_update_preview_reads_only_a_page(
    sample_config: Config, answer: Any
) -> None:
    """A read that did not answer with a page of objects has no assignments to
    report, which is an empty state rather than a fabricated one.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.list_instance_firewalls.return_value = answer
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_instance_firewall_update_preview(
            sample_config,
            {"linode_id": 123, "firewall_ids": [1], "dry_run": True},
            "PUT",
            _INSTANCE_PATH,
            {"firewall_ids": [1]},
        )

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["current_state"] == []


async def test_instance_firewall_update_preview_ignores_an_unusable_page_bound(
    sample_config: Config,
) -> None:
    """A page control the caller sent as something other than an integer never
    reaches the fetch: the generated handler has already answered it, so the
    hook reads the collection under the route's own default.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _stub_client()
        mock_client.list_instance_firewalls.return_value = _PAGE
        mock_cls.return_value = mock_client

        await toolhooks.linode_instance_firewall_update_preview(
            sample_config,
            {
                "linode_id": 123,
                "firewall_ids": [1],
                "page": "two",
                "dry_run": True,
            },
            "PUT",
            _INSTANCE_PATH,
            {"firewall_ids": [1]},
        )

        mock_client.list_instance_firewalls.assert_awaited_once_with(
            123, page=None, page_size=None
        )
