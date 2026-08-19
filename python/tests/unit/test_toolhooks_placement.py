"""The sentences and prose the placement group write hooks answer with.

The Go twin is ``placement_test.go`` in ``go/internal/toolhooks``. Several
answers below differ from it on purpose: the two handlers these hooks were
lifted from never agreed on what an absent argument is called, and neither
language's callers should start seeing the other's sentence.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

LABEL_ERROR = (
    "label must start and end with an alphanumeric character and contain only "
    "alphanumeric characters, hyphens, underscores, or periods"
)
LINODES_ERROR = "linodes must be a non-empty array of positive integers"


def _create_args(**overrides: Any) -> dict[str, Any]:
    """The create request every row below starts from."""
    args: dict[str, Any] = {
        "label": "pg-test",
        "region": "us-east",
        "placement_group_type": "anti_affinity:local",
        "placement_group_policy": "strict",
    }
    args.update(overrides)
    return args


def test_create_normalize_trims_only_the_label() -> None:
    """Go trims all four; this language has only ever trimmed the label."""
    args = _create_args(
        label="  pg-test  ",
        region=" us-east ",
        placement_group_type=" anti_affinity:local ",
    )

    toolhooks.linode_placement_group_create_normalize(args)

    assert args["label"] == "pg-test"
    assert args["region"] == " us-east "
    assert args["placement_group_type"] == " anti_affinity:local "


def test_create_normalize_leaves_non_text_alone() -> None:
    """A value the trim cannot apply to is left for the check to refuse."""
    args = _create_args(label=123)

    toolhooks.linode_placement_group_create_normalize(args)

    assert args["label"] == 123


async def test_unassign_preview_names_every_linode(sample_config: Config) -> None:
    """The preview reads the group for current_state and names each departure."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = {"id": 7}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await toolhooks.linode_placement_group_unassign_preview(
            sample_config,
            {"group_id": 7, "linodes": [123, 456], "dry_run": True},
            "POST",
            "/placement/groups/7/unassign",
            {},
        )

    body = json.loads(result[0].text)
    assert body["side_effects"] == [
        "Linode 123 will be removed from placement group 7.",
        "Linode 456 will be removed from placement group 7.",
    ]
    mock_client.get_placement_group.assert_awaited_once_with(7)


async def test_assign_preview_names_every_linode(sample_config: Config) -> None:
    """The preview reads the group for current_state and names each arrival."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_client_class:
        mock_client = AsyncMock()
        mock_client.get_placement_group.return_value = {"id": 7}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_client_class.return_value = mock_client

        result = await toolhooks.linode_placement_group_assign_preview(
            sample_config,
            {"group_id": 7, "linodes": [123, 456], "dry_run": True},
            "POST",
            "/placement/groups/7/assign",
            {},
        )

    body = json.loads(result[0].text)
    assert body["side_effects"] == [
        "Linode 123 will be assigned to placement group 7.",
        "Linode 456 will be assigned to placement group 7.",
    ]
    mock_client.get_placement_group.assert_awaited_once_with(7)
