"""Linode Object Storage read-only tools."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


_CLUSTER_ID_RE = re.compile(r"^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$")
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
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_object_storage_bucket_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        buckets = await client.list_object_storage_buckets()
        return {
            "count": len(buckets),
            "buckets": buckets,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage buckets", _call)


def create_linode_object_storage_bucket_by_region_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_object_storage_bucket_by_region_list tool."""
    return Tool(
        name="linode_object_storage_bucket_by_region_list",
        description="Lists Object Storage buckets in a region.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": (
                        "The region or legacy cluster ID to list buckets for"
                    ),
                },
            },
            "required": ["region"],
        },
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
        return {
            "count": len(buckets),
            "region": region,
            "buckets": buckets,
        }

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage buckets for region", _call
    )


def create_linode_object_storage_bucket_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_get tool."""
    return Tool(
        name="linode_object_storage_bucket_get",
        description="Gets details about a specific Object Storage bucket.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": (
                        "The region/cluster ID where the bucket exists (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "The label/name of the bucket (required)",
                },
            },
            "required": ["region", "label"],
        },
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
        return await client.get_object_storage_bucket(region, label)

    return await execute_tool(cfg, arguments, "retrieve Object Storage bucket", _call)


def create_linode_object_storage_bucket_object_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_object_list tool."""
    return Tool(
        name="linode_object_storage_bucket_object_list",
        description=(
            "Lists objects in an Object Storage bucket. "
            "Supports pagination and filtering by prefix/delimiter."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": (
                        "The region/cluster ID where the bucket exists (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "The label/name of the bucket (required)",
                },
                "prefix": {
                    "type": "string",
                    "description": (
                        "Limits results to object keys that begin with this prefix"
                    ),
                },
                "delimiter": {
                    "type": "string",
                    "description": (
                        "Character used to group keys "
                        "(e.g., '/' for directory-like listing)"
                    ),
                },
                "marker": {
                    "type": "string",
                    "description": "Object key to start listing from (for pagination)",
                },
                "page_size": {
                    "type": "string",
                    "description": "Number of objects to return per page (1-1000)",
                },
            },
            "required": ["region", "label"],
        },
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


def _build_bucket_filter_string(
    prefix: str, delimiter: str, marker: str, page_size: str
) -> str:
    """Build filter string for bucket contents response."""
    filters: list[str] = []
    if prefix:
        filters.append(f"prefix={prefix}")
    if delimiter:
        filters.append(f"delimiter={delimiter}")
    if marker:
        filters.append(f"marker={marker}")
    if page_size:
        filters.append(f"page_size={page_size}")
    return ", ".join(filters) if filters else ""


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
        response: dict[str, Any] = {
            "count": len(objects),
            "objects": objects,
            "is_truncated": result.get("is_truncated", False),
        }

        if result.get("next_marker"):
            response["next_marker"] = result["next_marker"]

        filter_str = _build_bucket_filter_string(prefix, delimiter, marker, page_size)
        if filter_str:
            response["filter"] = filter_str

        return response

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
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_object_storage_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_object_storage_types()
        return {
            "count": len(types),
            "types": types,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage types", _call)


def create_linode_object_storage_endpoint_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_endpoint_list tool."""
    return Tool(
        name="linode_object_storage_endpoint_list",
        description="Lists Object Storage endpoints available to your account.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_object_storage_endpoint_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_endpoint_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoints = await client.list_object_storage_endpoints()
        return {
            "count": len(endpoints),
            "endpoints": endpoints,
        }

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage endpoints", _call
    )


# Phase 2: Read-Only Access Key & Transfer Tools


def create_linode_object_storage_key_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_key_list tool."""
    return Tool(
        name="linode_object_storage_key_list",
        description="Lists all Object Storage access keys for the authenticated user.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_object_storage_key_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_key_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        keys = await client.list_object_storage_keys()
        return {
            "count": len(keys),
            "keys": keys,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage keys", _call)


def create_linode_object_storage_key_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_key_get tool."""
    return Tool(
        name="linode_object_storage_key_get",
        description="Gets details about a specific Object Storage access key by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "key_id": {
                    "type": "integer",
                    "description": "The ID of the access key to retrieve (required)",
                },
            },
            "required": ["key_id"],
        },
    ), Capability.Read


async def handle_linode_object_storage_key_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_key_get tool request."""
    key_id = arguments.get("key_id", 0)

    if not key_id:
        return _error_response("key_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_key(int(key_id))

    return await execute_tool(cfg, arguments, "retrieve Object Storage key", _call)


def create_linode_object_storage_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_transfer_get tool."""
    return Tool(
        name="linode_object_storage_transfer_get",
        description=(
            "Gets Object Storage outbound data transfer usage for the current month."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_object_storage_transfer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_transfer_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_transfer()

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage transfer usage", _call
    )


def create_linode_object_storage_quota_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_quota_list tool."""
    return Tool(
        name="linode_object_storage_quota_list",
        description="Lists Object Storage quotas on your Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_object_storage_quota_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_quota_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        quotas = await client.list_object_storage_quotas()
        return {
            "count": len(quotas),
            "quotas": quotas,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage quotas", _call)


def create_linode_object_storage_quota_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_quota_get tool."""
    return Tool(
        name="linode_object_storage_quota_get",
        description="Gets a single Object Storage quota by quota ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "obj_quota_id": {
                    "type": "string",
                    "description": (
                        "The Object Storage quota ID, formatted as "
                        "<quota_type>-<s3_endpoint>."
                    ),
                },
            },
            "required": ["obj_quota_id"],
        },
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
        return await client.get_object_storage_quota(obj_quota_id)

    return await execute_tool(cfg, arguments, "retrieve Object Storage quota", _call)


def create_linode_object_storage_quota_usage_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_quota_usage_get tool."""
    return Tool(
        name="linode_object_storage_quota_usage_get",
        description="Gets Object Storage quota usage data by quota ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "obj_quota_id": {
                    "type": "integer",
                    "description": "The Object Storage quota ID to retrieve usage for.",
                },
            },
            "required": ["obj_quota_id"],
        },
    ), Capability.Read


def _parse_positive_int_argument(value: Any) -> int | None:
    """Parse a positive integer tool argument."""
    if isinstance(value, (bool, float)):
        return None
    try:
        parsed = int(value)
    except (TypeError, ValueError):
        return None
    if parsed <= 0:
        return None
    return parsed


async def handle_linode_object_storage_quota_usage_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_quota_usage_get tool request."""
    obj_quota_id = _parse_positive_int_argument(arguments.get("obj_quota_id"))
    if obj_quota_id is None:
        return _error_response("obj_quota_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_quota_usage(obj_quota_id)

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
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": (
                        "The region/cluster ID where the bucket exists (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "The label/name of the bucket (required)",
                },
            },
            "required": ["region", "label"],
        },
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
        return await client.get_object_storage_bucket_access(region, label)

    return await execute_tool(cfg, arguments, "retrieve bucket access settings", _call)


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]
