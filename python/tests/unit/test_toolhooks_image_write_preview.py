"""The preview linode_image_create owns.

It names a disk nothing has captured yet, so its sentence is all a caller gets.
The Go twin is ``image_write_preview_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config


@pytest.mark.parametrize(
    ("arguments", "effect"),
    [
        (
            {"disk_id": 456, "label": "nightly"},
            'A new image will be captured from disk 456 and labeled "nightly".',
        ),
        (
            {"disk_id": 456},
            "A new image will be captured from disk 456.",
        ),
    ],
)
async def test_image_create_preview_names_the_disk_and_label(
    sample_config: Config,
    arguments: dict[str, Any],
    effect: str,
) -> None:
    """The sentence carries the disk, and the label only when one was sent."""
    body = {key: value for key, value in arguments.items() if key != "disk_id"}
    body["disk_id"] = arguments["disk_id"]

    result = await toolhooks.linode_image_create_preview(
        sample_config,
        {**arguments, "dry_run": True},
        "POST",
        "/images",
        body,
    )

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["dry_run"] is True
    assert preview["tool"] == "linode_image_create"
    assert preview["would_execute"]["method"] == "POST"
    assert preview["would_execute"]["path"] == "/images"
    assert preview["would_execute"]["body"] == body
    assert preview["current_state"] is None
    assert preview["side_effects"] == [effect]
