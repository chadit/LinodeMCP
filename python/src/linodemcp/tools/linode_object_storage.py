"""Linode Object Storage read-only tools."""

from __future__ import annotations

from typing import Any, cast

# Deprecated Object Storage cluster listing and single-cluster lookup are
# intentionally not exposed. Use the regions API for supported region metadata.


def _obj_key_bucket_access_to_dict(access: dict[str, Any]) -> dict[str, Any]:
    """Shape a per-bucket access grant to proto-canonical form."""
    return {
        "bucket_name": access.get("bucket_name", ""),
        "region": access.get("region", ""),
        "permissions": access.get("permissions", ""),
    }


def _obj_key_region_to_dict(region: dict[str, Any]) -> dict[str, Any]:
    """Shape a per-region key entry to proto-canonical form."""
    return {
        "id": region.get("id", ""),
        "s3_endpoint": region.get("s3_endpoint", ""),
    }


def object_storage_key_to_response_dict(key: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw Object Storage key API dict to proto-canonical form.

    bucket_access and regions are always lists; secret_key coerces null to "".
    """
    return {
        "label": key.get("label", ""),
        "access_key": key.get("access_key", ""),
        "secret_key": key.get("secret_key") or "",
        "bucket_access": [
            _obj_key_bucket_access_to_dict(a)
            for a in cast("list[dict[str, Any]]", key.get("bucket_access") or [])
        ],
        "regions": [
            _obj_key_region_to_dict(r)
            for r in cast("list[dict[str, Any]]", key.get("regions") or [])
        ],
        "id": key.get("id", 0),
        "limited": key.get("limited", False),
    }
