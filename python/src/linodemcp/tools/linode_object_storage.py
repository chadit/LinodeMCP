"""Linode Object Storage read-only tools."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    bucket_access_pb2,
    object_storage_pb2,
    type_pb2,
)
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


# Lowercase alphanumeric slug with single internal hyphens (us-east-1). Matches
# Go's isSafeObjectStorageRegion, which is stricter than the previous pattern:
# it rejects uppercase and consecutive hyphens the live cluster route never uses
# (strictest-wins; the two languages now reject the same set).
_CLUSTER_ID_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
_BUCKET_LABEL_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]{1,2}$")
_MAX_BUCKET_LABEL_LENGTH = 63


def _valid_cluster_id(cluster_id: str) -> bool:
    """Return whether a cluster ID is safe for the legacy cluster route."""
    return bool(_CLUSTER_ID_RE.fullmatch(cluster_id))


def _valid_bucket_label(label: str) -> bool:
    """Return whether a bucket label is safe for Object Storage bucket routes."""
    return (
        bool(_BUCKET_LABEL_RE.fullmatch(label))
        and len(label) <= _MAX_BUCKET_LABEL_LENGTH
    )


def create_linode_object_storage_bucket_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_list tool."""
    return Tool(
        name="linode_object_storage_bucket_list",
        description="Lists all Object Storage buckets on your Linode account.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageBucketListInput"),
    ), Capability.Read


async def handle_linode_object_storage_bucket_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        buckets = await client.list_object_storage_buckets()
        return serialize_list_response(
            {"data": buckets},
            "buckets",
            object_storage_pb2.ObjectStorageBucketListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage buckets", _call)


def create_linode_object_storage_bucket_by_region_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_object_storage_bucket_by_region_list tool."""
    return Tool(
        name="linode_object_storage_bucket_by_region_list",
        description="Lists Object Storage buckets in a region.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageBucketByRegionListInput"),
    ), Capability.Read


async def handle_linode_object_storage_bucket_by_region_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_by_region_list tool request."""
    region = arguments.get("region", "")

    if not region:
        return _error_response("region is required")
    if not isinstance(region, str) or not _valid_cluster_id(region):
        return _error_response("region must be a valid region or cluster ID")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        buckets = await client.list_object_storage_buckets_for_region(region)
        return serialize_list_response(
            {"data": buckets},
            "buckets",
            object_storage_pb2.ObjectStorageBucketListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage buckets for region", _call
    )


def create_linode_object_storage_bucket_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_get tool."""
    return Tool(
        name="linode_object_storage_bucket_get",
        description="Gets details about a specific Object Storage bucket.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageBucketGetInput"),
    ), Capability.Read


async def handle_linode_object_storage_bucket_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not isinstance(region, str) or not _valid_cluster_id(region):
        return _error_response("region must be a valid region or cluster ID")
    if not label:
        return _error_response("label is required")
    if not isinstance(label, str) or not _valid_bucket_label(label):
        return _error_response("label must be a valid bucket label")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_object_storage_bucket(region, label),
            object_storage_pb2.ObjectStorageBucket(),
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage bucket", _call)


def create_linode_object_storage_bucket_object_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_object_list tool."""
    return Tool(
        name="linode_object_storage_bucket_object_list",
        description=(
            "Lists objects in an Object Storage bucket. "
            "Supports pagination and filtering by prefix/delimiter."
        ),
        inputSchema=schema("linode.mcp.v1.ObjectStorageBucketObjectListInput"),
    ), Capability.Read


def _build_bucket_params(
    prefix: str, delimiter: str, marker: str, page_size: str
) -> dict[str, str]:
    """Build parameters dictionary for bucket contents request."""
    params: dict[str, str] = {}
    if prefix:
        params["prefix"] = prefix
    if delimiter:
        params["delimiter"] = delimiter
    if marker:
        params["marker"] = marker
    if page_size:
        params["page_size"] = page_size
    return params


async def handle_linode_object_storage_bucket_object_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_object_list tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    prefix = arguments.get("prefix", "")
    delimiter = arguments.get("delimiter", "")
    marker = arguments.get("marker", "")
    page_size = arguments.get("page_size", "")

    if not region:
        return _error_response("region is required")
    if not isinstance(region, str) or not _valid_cluster_id(region):
        return _error_response("region must be a valid region or cluster ID")
    if not label:
        return _error_response("label is required")
    if not isinstance(label, str) or not _valid_bucket_label(label):
        return _error_response("label must be a valid bucket label")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        params = _build_bucket_params(prefix, delimiter, marker, page_size)
        result = await client.list_object_storage_bucket_contents(
            region, label, params or None
        )

        objects = result.get("data", [])
        wrapper: dict[str, Any] = {
            "count": len(objects),
            "is_truncated": result.get("is_truncated", False),
            "objects": objects,
        }

        next_marker = result.get("next_marker")
        if next_marker:
            wrapper["next_marker"] = next_marker

        # Go echoes only prefix/delimiter in the filter, never marker/page_size.
        filters: list[str] = []
        if prefix:
            filters.append(f"prefix={prefix}")
        if delimiter:
            filters.append(f"delimiter={delimiter}")
        if filters:
            wrapper["filter"] = ", ".join(filters)

        return serialize_api_response(
            wrapper,
            object_storage_pb2.ObjectStorageObjectListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage bucket contents", _call
    )


# Deprecated Object Storage cluster listing and single-cluster lookup are
# intentionally not exposed. Use the regions API for supported region metadata.


def create_linode_object_storage_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_type_list tool."""
    return Tool(
        name="linode_object_storage_type_list",
        description=(
            "Lists Object Storage pricing tiers and capabilities. Shows pricing, "
            "storage limits, and transfer allowances."
        ),
        inputSchema=schema("linode.mcp.v1.ObjectStorageTypeListInput"),
    ), Capability.Read


async def handle_linode_object_storage_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_object_storage_types()
        return serialize_list_response(
            {"data": types}, "types", type_pb2.ObjectStorageTypeListResponse()
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage types", _call)


def create_linode_object_storage_endpoint_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_endpoint_list tool."""
    return Tool(
        name="linode_object_storage_endpoint_list",
        description="Lists Object Storage endpoints available to your account.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageEndpointListInput"),
    ), Capability.Read


async def handle_linode_object_storage_endpoint_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_endpoint_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoints = await client.list_object_storage_endpoints()
        return serialize_list_response(
            {"data": endpoints},
            "endpoints",
            object_storage_pb2.ObjectStorageEndpointListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage endpoints", _call
    )


# Phase 2: Read-Only Access Key & Transfer Tools


def create_linode_object_storage_key_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_key_list tool."""
    return Tool(
        name="linode_object_storage_key_list",
        description="Lists all Object Storage access keys for the authenticated user.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageKeyListInput"),
    ), Capability.Read


async def handle_linode_object_storage_key_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_key_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        keys = await client.list_object_storage_keys()
        return serialize_list_response(
            {"data": keys},
            "keys",
            object_storage_pb2.ObjectStorageKeyListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage keys", _call)


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


def create_linode_object_storage_key_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_key_get tool."""
    return Tool(
        name="linode_object_storage_key_get",
        description="Gets details about a specific Object Storage access key by ID.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageKeyGetInput"),
    ), Capability.Read


async def handle_linode_object_storage_key_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_key_get tool request."""
    key_id = arguments.get("key_id", 0)

    if not key_id:
        return _error_response("key_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_object_storage_key(int(key_id)),
            object_storage_pb2.ObjectStorageKey(),
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage key", _call)


def create_linode_object_storage_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_transfer_get tool."""
    return Tool(
        name="linode_object_storage_transfer_get",
        description=(
            "Gets Object Storage outbound data transfer usage for the current month."
        ),
        inputSchema=schema("linode.mcp.v1.ObjectStorageTransferGetInput"),
    ), Capability.Read


async def handle_linode_object_storage_transfer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_transfer_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_object_storage_transfer(),
            object_storage_pb2.ObjectStorageTransfer(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage transfer usage", _call
    )


def create_linode_object_storage_quota_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_quota_list tool."""
    return Tool(
        name="linode_object_storage_quota_list",
        description="Lists Object Storage quotas on your Linode account.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageQuotaListInput"),
    ), Capability.Read


async def handle_linode_object_storage_quota_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_quota_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        quotas = await client.list_object_storage_quotas()
        return serialize_list_response(
            {"data": quotas},
            "quotas",
            object_storage_pb2.ObjectStorageQuotaListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage quotas", _call)


def create_linode_object_storage_quota_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_quota_get tool."""
    return Tool(
        name="linode_object_storage_quota_get",
        description="Gets a single Object Storage quota by quota ID.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageQuotaGetInput"),
    ), Capability.Read


def _parse_object_storage_quota_id(value: Any) -> str | None:
    """Parse an Object Storage quota ID tool argument."""
    if not isinstance(value, str):
        return None
    parsed = value.strip()
    if not parsed:
        return None
    if any(separator in parsed for separator in ("/", "?", "#")):
        return None
    if ".." in parsed:
        return None
    return parsed


async def handle_linode_object_storage_quota_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_quota_get tool request."""
    obj_quota_id = _parse_object_storage_quota_id(arguments.get("obj_quota_id"))
    if obj_quota_id is None:
        return _error_response("obj_quota_id must be a valid Object Storage quota ID")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_object_storage_quota(obj_quota_id),
            object_storage_pb2.ObjectStorageQuota(),
        )

    return await execute_tool(cfg, arguments, "retrieve Object Storage quota", _call)


def create_linode_object_storage_quota_usage_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_quota_usage_get tool."""
    return Tool(
        name="linode_object_storage_quota_usage_get",
        description="Gets Object Storage quota usage data by quota ID.",
        inputSchema=schema("linode.mcp.v1.ObjectStorageQuotaUsageGetInput"),
    ), Capability.Read


async def handle_linode_object_storage_quota_usage_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_quota_usage_get tool request."""
    obj_quota_id = _parse_object_storage_quota_id(arguments.get("obj_quota_id"))
    if obj_quota_id is None:
        return _error_response("obj_quota_id must be a valid Object Storage quota ID")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_object_storage_quota_usage(obj_quota_id),
            object_storage_pb2.ObjectStorageQuotaUsage(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage quota usage", _call
    )


def create_linode_object_storage_bucket_access_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_access_get tool."""
    return Tool(
        name="linode_object_storage_bucket_access_get",
        description=(
            "Gets the ACL and CORS settings for a specific Object Storage bucket."
        ),
        inputSchema=schema("linode.mcp.v1.ObjectStorageBucketAccessGetInput"),
    ), Capability.Read


async def handle_linode_object_storage_bucket_access_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_access_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_object_storage_bucket_access(region, label),
            bucket_access_pb2.ObjectStorageBucketAccess(),
        )

    return await execute_tool(cfg, arguments, "retrieve bucket access settings", _call)


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]
