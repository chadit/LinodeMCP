"""The normalize and preview hooks linode_image_replicate owns.

Its preview reads the image and lists the regions the copy lands in, and its
normalize trims each region slug before the contract's slug rule reads it. The
Go twin is ``image_replicate_update_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from mcp.types import TextContent

    from linodemcp.config import Config

_IMAGE_ID = "private/123"
_IMAGE = {"id": _IMAGE_ID, "label": "golden"}
_REPLICATE_PATH = "/images/private%2F123/regions"


def _patched_client() -> Any:
    """Answer the image read both previews make."""
    mock_client = AsyncMock()
    mock_client.get_image.return_value = _IMAGE
    mock_client.__aenter__.return_value = mock_client
    mock_client.__aexit__.return_value = None

    return mock_client


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"regions": [" us-east ", "us-west"]}, {"regions": ["us-east", "us-west"]}),
        ({"regions": ["\tus-east\n"]}, {"regions": ["us-east"]}),
        ({"regions": [7]}, {"regions": [7]}),
        ({"regions": "us-east"}, {"regions": "us-east"}),
        ({"image_id": _IMAGE_ID}, {"image_id": _IMAGE_ID}),
    ],
)
def test_image_replicate_normalize_trims_each_slug(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    """A padded slug is trimmed; anything that is not text is left for the rule."""
    supplied = dict(arguments)
    toolhooks.linode_image_replicate_normalize(supplied)

    assert supplied == expected


@pytest.mark.parametrize(
    ("regions", "effect"),
    [
        (["us-east", "us-west"], "us-east, us-west"),
        (["us-east", 7], "us-east"),
        ("us-east", ""),
    ],
)
async def test_image_replicate_preview_lists_the_regions(
    sample_config: Config, regions: Any, effect: str
) -> None:
    """The preview fetches the image and names where the copies would land."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = _patched_client()
        mock_cls.return_value = mock_client

        result: list[TextContent] = await toolhooks.linode_image_replicate_preview(
            sample_config,
            {"image_id": _IMAGE_ID, "regions": regions, "dry_run": True},
            "POST",
            _REPLICATE_PATH,
            {"regions": regions},
        )

        mock_client.get_image.assert_awaited_once_with(_IMAGE_ID)

    body: dict[str, Any] = json.loads(result[0].text)

    assert body["tool"] == "linode_image_replicate"
    assert body["would_execute"]["method"] == "POST"
    assert body["would_execute"]["path"] == _REPLICATE_PATH
    assert body["current_state"] == _IMAGE
    assert body["side_effects"] == [
        f"Image '{_IMAGE_ID}' will be replicated to {effect}."
    ]
