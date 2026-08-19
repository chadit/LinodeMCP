"""The declared previews a flag selects the wording of, and the one whose read
is addressed by a query parameter.

Go's twins live in go/internal/tools/generated_preview_choice_test.go and pin
the same prose. The object ACL read is pinned here rather than left to the
behavior fixtures alone: a fixture stub is matched on the path, so it cannot
see the query the read is addressed by, and without that query the fetch
answers about the bucket.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.gentools import (
    handle_linode_instance_backup_restore,
    handle_linode_lke_acl_update,
    handle_linode_object_storage_bucket_access_update,
    handle_linode_object_storage_object_acl_update,
)

if TYPE_CHECKING:
    from unittest.mock import AsyncMock

    from linodemcp.config import Config


def _preview(result: list[Any]) -> dict[str, Any]:
    """The dry-run envelope a generated handler answered with."""
    payload: dict[str, Any] = json.loads(result[0].text)
    return payload


RESTORE_REPLACED = (
    "All existing disks and configs on target instance 789 are destroyed and "
    "replaced by the backup."
)
RESTORE_BESIDE = (
    "The backup is restored onto target instance 789; the restore fails if its "
    "disks or configs collide."
)
RESTORE_LOST = (
    "overwrite=true: existing data on target instance 789 is permanently lost."
)


@pytest.mark.parametrize(
    ("overwrite", "effect", "warnings"),
    [
        (True, RESTORE_REPLACED, [RESTORE_LOST]),
        (False, RESTORE_BESIDE, []),
        (None, RESTORE_BESIDE, []),
    ],
)
async def test_backup_restore_preview_splits_on_overwrite(
    sample_config: Config,
    mock_linode_client: AsyncMock,
    overwrite: bool | None,
    effect: str,
    warnings: list[str],
) -> None:
    """The flag decides what the restore does to the target and whether the
    destructive warning is reported at all. An absent overwrite reads as false,
    which is what both hand walks did.
    """
    mock_linode_client.route_raw.return_value = {
        "id": 456,
        "label": "nightly",
        "status": "successful",
    }

    arguments: dict[str, Any] = {
        "linode_id": 123,
        "backup_id": 456,
        "target_linode_id": 789,
        "dry_run": True,
    }
    if overwrite is not None:
        arguments["overwrite"] = overwrite

    preview = _preview(
        await handle_linode_instance_backup_restore(arguments, sample_config)
    )

    assert preview["side_effects"] == [effect]
    assert preview.get("warnings", []) == warnings


@pytest.mark.parametrize(
    ("supplied", "effects"),
    [
        (
            {"acl": "private", "cors_enabled": True},
            [
                'Bucket access control is set to "private".',
                "CORS is enabled for the bucket.",
            ],
        ),
        ({"cors_enabled": False}, ["CORS is disabled for the bucket."]),
        ({"acl": "private"}, ['Bucket access control is set to "private".']),
    ],
)
async def test_bucket_access_update_preview_names_only_what_was_asked(
    sample_config: Config,
    mock_linode_client: AsyncMock,
    supplied: dict[str, Any],
    effects: list[str],
) -> None:
    """An omitted acl or cors_enabled leaves that setting as it stands, so its
    whole line is dropped rather than reported with a gap in it.
    """
    mock_linode_client.route_raw.return_value = {
        "acl": "public-read",
        "cors_enabled": True,
    }

    arguments: dict[str, Any] = {
        "region": "us-east-1",
        "label": "my-bucket",
        "dry_run": True,
    }
    arguments.update(supplied)

    preview = _preview(
        await handle_linode_object_storage_bucket_access_update(
            arguments, sample_config
        )
    )

    assert preview["side_effects"] == effects


ACL_ENABLED = (
    "The control-plane ACL is enabled; only the listed addresses may reach the "
    "Kubernetes API."
)
ACL_DISABLED = (
    "The control-plane ACL is disabled; the Kubernetes API becomes reachable "
    "from any address."
)


@pytest.mark.parametrize(
    ("acl", "sentence"),
    [
        ({"enabled": True}, ACL_ENABLED),
        ({"enabled": False}, ACL_DISABLED),
        (
            {"addresses": {"ipv4": ["203.0.113.1/32"]}},
            "The cluster control-plane ACL address list is updated.",
        ),
        (
            {"enabled": "true"},
            "The cluster control-plane ACL address list is updated.",
        ),
    ],
)
async def test_lke_acl_update_preview_names_what_the_edit_does(
    sample_config: Config,
    mock_linode_client: AsyncMock,
    acl: dict[str, Any],
    sentence: str,
) -> None:
    """The flag rides inside the object the call sends, and an absent enabled is
    neither an open nor a close, which is the third answer Go's hand walk once
    reported as a disable.
    """
    # The API wraps the ACL under a top-level acl key, which is what the
    # declaration reads it out of.
    mock_linode_client.route_raw.return_value = {
        "acl": {"enabled": False, "addresses": {"ipv4": [], "ipv6": []}},
    }

    preview = _preview(
        await handle_linode_lke_acl_update(
            {"cluster_id": 123, "acl": acl, "dry_run": True}, sample_config
        )
    )

    assert preview["side_effects"] == [sentence]


async def test_object_acl_update_preview_addresses_the_object(
    sample_config: Config, mock_linode_client: AsyncMock
) -> None:
    """The read is addressed by the object key in its query as well as by the
    bucket in its path. Without the query this fetch answers about the bucket.
    """
    mock_linode_client.route_raw.return_value = {
        "acl": "private",
        "acl_xml": "<AccessControlPolicy/>",
    }

    preview = _preview(
        await handle_linode_object_storage_object_acl_update(
            {
                "region": "us-east-1",
                "label": "my-bucket",
                "name": "photo.jpg",
                "acl": "public-read",
                "dry_run": True,
            },
            sample_config,
        )
    )

    assert preview["side_effects"] == ['Object access control is set to "public-read".']
    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_object_storage_object_acl_get",
        "us-east-1",
        "my-bucket",
        query="name=photo.jpg",
    )
