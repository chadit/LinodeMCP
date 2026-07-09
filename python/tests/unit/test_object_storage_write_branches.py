"""Validator and walk branch coverage for object storage write tools.

Drives the bucket-label validator, the access-update argument check, and the
access-update side-effect walk through the public handlers, covering the
length/charset rejections and the ACL/CORS effect strings the happy-path
handler tests skip.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING
from unittest.mock import AsyncMock, patch

from linodemcp.tools.linode_object_storage_write import (
    handle_linode_object_storage_bucket_access_update,
    handle_linode_object_storage_bucket_create,
)

if TYPE_CHECKING:
    from linodemcp.config import Config


def _cm_client() -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


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


async def test_bucket_access_update_requires_region(sample_config: Config) -> None:
    """A missing region is reported first."""
    result = await handle_linode_object_storage_bucket_access_update(
        {"label": "my-bucket", "dry_run": True}, sample_config
    )
    assert "region is required" in result[0].text


async def test_bucket_access_update_requires_label(sample_config: Config) -> None:
    """A present region but missing label reports the label."""
    result = await handle_linode_object_storage_bucket_access_update(
        {"region": "us-east", "dry_run": True}, sample_config
    )
    assert "label is required" in result[0].text


async def test_bucket_access_update_walk_reports_acl_and_cors_toggle(
    sample_config: Config,
) -> None:
    """The dry-run walk names the ACL change and the CORS disable toggle."""
    client = _cm_client()
    client.get_object_storage_bucket_access.return_value = {"acl": "public-read"}

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await handle_linode_object_storage_bucket_access_update(
            {
                "region": "us-east",
                "label": "my-bucket",
                "acl": "private",
                "cors_enabled": False,
                "dry_run": True,
            },
            sample_config,
        )

    payload = json.loads(result[0].text)
    effects = payload["side_effects"]
    assert any("access control is set to 'private'" in s for s in effects)
    assert any("CORS is disabled" in s for s in effects)
