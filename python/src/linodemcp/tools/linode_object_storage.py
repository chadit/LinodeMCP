"""Linode Object Storage read-only tools."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_object_storage_buckets_list_tool() -> Tool:
    """Create the linode_object_storage_buckets_list tool."""
    return Tool(
        name="linode_object_storage_buckets_list",
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
    )


async def handle_linode_object_storage_buckets_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_buckets_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        buckets = await client.list_object_storage_buckets()
        return {
            "count": len(buckets),
            "buckets": buckets,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage buckets", _call)


def create_linode_object_storage_bucket_get_tool() -> Tool:
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
    )


async def handle_linode_object_storage_bucket_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_bucket(region, label)

    return await execute_tool(cfg, arguments, "retrieve Object Storage bucket", _call)


def create_linode_object_storage_bucket_contents_tool() -> Tool:
    """Create the linode_object_storage_bucket_contents tool."""
    return Tool(
        name="linode_object_storage_bucket_contents",
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
    )


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


async def handle_linode_object_storage_bucket_contents(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_contents tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    prefix = arguments.get("prefix", "")
    delimiter = arguments.get("delimiter", "")
    marker = arguments.get("marker", "")
    page_size = arguments.get("page_size", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

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


def create_linode_object_storage_clusters_list_tool() -> Tool:
    """Create the linode_object_storage_clusters_list tool."""
    return Tool(
        name="linode_object_storage_clusters_list",
        description=(
            "Lists available Object Storage clusters/regions. "
            "Shows which regions support Object Storage and their endpoints."
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
    )


async def handle_linode_object_storage_clusters_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_clusters_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        clusters = await client.list_object_storage_clusters()
        return {
            "count": len(clusters),
            "clusters": clusters,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage clusters", _call)


def create_linode_object_storage_types_list_tool() -> Tool:
    """Create the linode_object_storage_types_list tool."""
    return Tool(
        name="linode_object_storage_types_list",
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
    )


async def handle_linode_object_storage_types_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_types_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_object_storage_types()
        return {
            "count": len(types),
            "types": types,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage types", _call)


# Phase 2: Read-Only Access Key & Transfer Tools


def create_linode_object_storage_keys_list_tool() -> Tool:
    """Create the linode_object_storage_keys_list tool."""
    return Tool(
        name="linode_object_storage_keys_list",
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
    )


async def handle_linode_object_storage_keys_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_keys_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        keys = await client.list_object_storage_keys()
        return {
            "count": len(keys),
            "keys": keys,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage keys", _call)


def create_linode_object_storage_key_get_tool() -> Tool:
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
    )


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


def create_linode_object_storage_transfer_tool() -> Tool:
    """Create the linode_object_storage_transfer tool."""
    return Tool(
        name="linode_object_storage_transfer",
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
    )


async def handle_linode_object_storage_transfer(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_transfer tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_transfer()

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage transfer usage", _call
    )


def create_linode_object_storage_bucket_access_get_tool() -> Tool:
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
    )


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
