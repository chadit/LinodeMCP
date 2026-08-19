"""Unit tests for the Object Storage upload hooks.

Go's twins live in ``go/internal/toolhooks/objectstorage_upload_test.go``.
Everything is driven through the public hooks: the preview is where the presign
body defaults and the side-effect prose are observable.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp import toolhooks
from linodemcp.config import Config, ObjectStorageConfig

if TYPE_CHECKING:
    from pathlib import Path

_PATH = "/object-storage/buckets/us-east/artifacts/object-url"


async def _preview(
    arguments: dict[str, Any],
    body: dict[str, Any] | None,
    object_storage: ObjectStorageConfig | None = None,
) -> dict[str, Any]:
    """Run the preview hook and hand back its decoded answer."""
    cfg = Config(object_storage=object_storage or ObjectStorageConfig())
    result = await toolhooks.linode_object_storage_object_upload_preview(
        cfg, arguments, "POST", _PATH, body
    )
    decoded: dict[str, Any] = json.loads(result[0].text)
    return decoded


@pytest.mark.asyncio
async def test_preview_defaults_the_signed_content_type(tmp_path: Path) -> None:
    """The signature covers Content-Type, so the presign body has to carry it.

    Setting it only on the PUT would sign one request and send another, which
    the endpoint answers with a 403.
    """
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")

    answer = await _preview(
        {"source_path": str(source), "name": "k", "label": "artifacts"},
        {"name": "k"},
    )

    assert answer["would_execute"]["body"] == {
        "name": "k",
        "content_type": "application/octet-stream",
        "expires_in": 3600,
    }


@pytest.mark.asyncio
async def test_preview_keeps_a_caller_supplied_content_type(tmp_path: Path) -> None:
    """A default fills a gap; it never overrides what the caller asked for."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")

    answer = await _preview(
        {"source_path": str(source), "name": "k", "label": "artifacts"},
        {"name": "k", "content_type": "text/plain", "expires_in": 60},
    )

    assert answer["would_execute"]["body"]["content_type"] == "text/plain"
    assert answer["would_execute"]["body"]["expires_in"] == 60


@pytest.mark.asyncio
async def test_preview_survives_a_tool_with_no_body(tmp_path: Path) -> None:
    """A missing body has nothing to default into and must not crash."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")

    answer = await _preview(
        {"source_path": str(source), "name": "k", "label": "artifacts"}, None
    )

    assert "body" not in answer["would_execute"]


@pytest.mark.asyncio
async def test_preview_describes_the_transfer(tmp_path: Path) -> None:
    """The sentence names the byte count, the key, the bucket, and the mode."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")

    answer = await _preview(
        {
            "source_path": str(source),
            "name": "releases/app.tar.gz",
            "label": "artifacts",
        },
        {"name": "releases/app.tar.gz"},
    )

    expected = (
        "20 bytes will be uploaded to 'releases/app.tar.gz' in bucket"
        " 'artifacts' as a single part."
    )
    assert answer["side_effects"] == [expected]


@pytest.mark.asyncio
async def test_preview_warns_when_the_source_cannot_be_read(tmp_path: Path) -> None:
    """A preview that cannot stat the file says so instead of describing one."""
    answer = await _preview(
        {
            "source_path": str(tmp_path / "absent.bin"),
            "name": "k",
            "label": "artifacts",
        },
        {"name": "k"},
    )

    assert "no readable file" in answer["warnings"][0]


@pytest.mark.asyncio
async def test_preview_warns_when_the_file_is_over_the_ceiling(tmp_path: Path) -> None:
    """The preview names the refusal before a live call is spent discovering it."""
    source = tmp_path / "big.bin"
    source.write_bytes(b"a" * 2048)

    answer = await _preview(
        {"source_path": str(source), "name": "k", "label": "artifacts"},
        {"name": "k"},
        ObjectStorageConfig(max_single_part_bytes=1024),
    )

    assert "single-part upload ceiling" in answer["warnings"][0]
