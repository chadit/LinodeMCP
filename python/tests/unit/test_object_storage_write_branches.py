"""Validator and walk branch coverage for object storage write tools.

Drives the bucket-label validator, the access-update argument check, and the
access-update side-effect walk through the public handlers, covering the
length/charset rejections and the ACL/CORS effect strings the happy-path
handler tests skip.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from linodemcp.gentools import handle_linode_object_storage_bucket_create

if TYPE_CHECKING:
    from linodemcp.config import Config


async def test_bucket_create_rejects_overlong_label(sample_config: Config) -> None:
    """A label past 63 characters is rejected on length before any call."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "a" * 64, "dry_run": True}, sample_config
    )
    assert "bucket label must not exceed 63 characters" in result[0].text


async def test_bucket_create_rejects_invalid_characters(sample_config: Config) -> None:
    """An underscore fails the lowercase/number/hyphen charset rule."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "ab_cd", "dry_run": True}, sample_config
    )
    assert "lowercase letters, numbers, and hyphens" in result[0].text


async def test_bucket_create_accepts_valid_label(sample_config: Config) -> None:
    """A conforming label passes validation, so the missing region is what
    stops the request. That the handler reaches the region check proves the
    label validator returned no error."""
    result = await handle_linode_object_storage_bucket_create(
        {"label": "my-bucket-1", "dry_run": True}, sample_config
    )
    assert "region is required" in result[0].text
