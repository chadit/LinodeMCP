"""The previews the two Object Storage key mutators own.

The create mints a key nothing has captured yet, so its sentence and the
shown-once notice are all a caller gets. The update is addressed by a key id and
reads it first, so the caller sees the label the call would replace. The Go twin
is ``objectstorage_key_preview_test.go`` in ``go/internal/toolhooks``, and every
sentence below is one both languages answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks

if TYPE_CHECKING:
    from linodemcp.config import Config

_KEY_PATH = "/object-storage/keys/77"
_SCOPE = [{"bucket_name": "assets", "region": "us-east", "permissions": "read_only"}]
_SCOPE_REPLACED = "The key's bucket access scopes are replaced."


@pytest.mark.parametrize(
    ("arguments", "stored", "effects"),
    [
        (
            {"key_id": 77, "label": "renamed-key", "bucket_access": _SCOPE},
            "my-key",
            ['Label changes from "my-key" to "renamed-key".', _SCOPE_REPLACED],
        ),
        (
            {"key_id": 77, "label": "renamed-key"},
            "renamed-key",
            ['Label is set to "renamed-key".'],
        ),
        (
            {"key_id": 77, "bucket_access": _SCOPE},
            "my-key",
            [_SCOPE_REPLACED],
        ),
        ({"key_id": 77}, "my-key", []),
    ],
)
async def test_key_update_preview_reads_the_key_it_would_replace(
    sample_config: Config,
    arguments: dict[str, Any],
    stored: str,
    effects: list[str],
) -> None:
    """Each sentence is named only when the caller asked for that change."""
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_key.return_value = {"id": 77, "label": stored}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_object_storage_key_update_preview(
            sample_config,
            {**arguments, "dry_run": True},
            "PUT",
            _KEY_PATH,
            {},
        )

        mock_client.get_object_storage_key.assert_awaited_once_with(77)

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["dry_run"] is True
    assert preview["tool"] == "linode_object_storage_key_update"
    assert preview["would_execute"]["method"] == "PUT"
    assert preview["would_execute"]["path"] == _KEY_PATH
    assert preview["current_state"]["label"] == stored
    assert preview["side_effects"] == effects


async def test_key_update_preview_reads_a_state_that_is_not_an_object(
    sample_config: Config,
) -> None:
    """A body that decoded to something other than an object names no old label.

    The fetch answers whatever the route sent, so the walk reads the label only
    when there is a mapping to read it from.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_object_storage_key.return_value = ["not", "an", "object"]
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_object_storage_key_update_preview(
            sample_config,
            {"key_id": 77, "label": "renamed-key", "dry_run": True},
            "PUT",
            _KEY_PATH,
            {},
        )

    preview: dict[str, Any] = json.loads(result[0].text)

    assert preview["side_effects"] == ['Label is set to "renamed-key".']
