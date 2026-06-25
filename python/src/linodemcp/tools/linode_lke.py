"""LKE (Kubernetes) READ tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool


def _validate_lke_path_segment(value: Any) -> str:
    """Validate an LKE string path segment before client dispatch."""
    if not isinstance(value, str) or not value:
        return ""
    if "/" in value or "?" in value or ".." in value:
        return ""
    return value


if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_CLUSTER_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the LKE cluster (required)",
}

_LKE_TIER_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The LKE tier ID, such as 'standard' or 'enterprise' (required)",
}
_LKE_TIER_PATTERN = re.compile(r"^[A-Za-z0-9][A-Za-z0-9_-]*$")


def _validate_lke_tier(value: object) -> str | None:
    """Return a safe LKE tier path segment or None when invalid."""
    if not isinstance(value, str) or not _LKE_TIER_PATTERN.fullmatch(value):
        return None
    return value


def create_linode_lke_cluster_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_list tool."""
    return Tool(
        name="linode_lke_cluster_list",
        description="Lists all LKE (Kubernetes) clusters on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_lke_cluster_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        clusters = await client.list_lke_clusters()
        return {"count": len(clusters), "clusters": clusters}

    return await execute_tool(cfg, arguments, "list LKE clusters", _call)


def create_linode_lke_cluster_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_get tool."""
    return Tool(
        name="linode_lke_cluster_get",
        description="Gets details of a specific LKE cluster by ID",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    ), Capability.Read


async def handle_linode_lke_cluster_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_cluster(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE cluster", _call)


def create_linode_lke_pool_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_pool_list tool."""
    return Tool(
        name="linode_lke_pool_list",
        description="Lists node pools for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    ), Capability.Read


async def handle_linode_lke_pool_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_list tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        pools = await client.list_lke_node_pools(cluster_id)
        return {"count": len(pools), "pools": pools}

    return await execute_tool(cfg, arguments, "list LKE node pools", _call)


def create_linode_lke_pool_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_pool_get tool."""
    return Tool(
        name="linode_lke_pool_get",
        description="Gets details of a specific node pool in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
            },
            "required": ["cluster_id", "pool_id"],
        },
    ), Capability.Read


async def handle_linode_lke_pool_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    if not pool_id_str:
        return error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_node_pool(cluster_id, pool_id)

    return await execute_tool(cfg, arguments, "get LKE node pool", _call)


def create_linode_lke_node_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_node_get tool."""
    return Tool(
        name="linode_lke_node_get",
        description="Gets details of a specific node in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "node_id": {
                    "type": "string",
                    "description": "The ID of the node (required, string)",
                },
            },
            "required": ["cluster_id", "node_id"],
        },
    ), Capability.Read


async def handle_linode_lke_node_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    node_id = arguments.get("node_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    if not node_id:
        return error_response("node_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_node(cluster_id, str(node_id))

    return await execute_tool(cfg, arguments, "get LKE node", _call)


def create_linode_lke_kubeconfig_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_kubeconfig_get tool."""
    return Tool(
        name="linode_lke_kubeconfig_get",
        description="Gets the kubeconfig for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    ), Capability.Read


async def handle_linode_lke_kubeconfig_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_kubeconfig_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_kubeconfig(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE kubeconfig", _call)


def create_linode_lke_dashboard_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_dashboard_get tool."""
    return Tool(
        name="linode_lke_dashboard_get",
        description="Gets the dashboard URL for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    ), Capability.Read


async def handle_linode_lke_dashboard_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_dashboard_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_dashboard(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE dashboard", _call)


def create_linode_lke_api_endpoint_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_api_endpoint_list tool."""
    return Tool(
        name="linode_lke_api_endpoint_list",
        description="Lists API endpoints for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    ), Capability.Read


async def handle_linode_lke_api_endpoint_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_api_endpoint_list tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoints = await client.list_lke_api_endpoints(cluster_id)
        return {"count": len(endpoints), "endpoints": endpoints}

    return await execute_tool(cfg, arguments, "list LKE API endpoints", _call)


def create_linode_lke_acl_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_acl_get tool."""
    return Tool(
        name="linode_lke_acl_get",
        description="Gets the control plane ACL configuration for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    ), Capability.Read


async def handle_linode_lke_acl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_control_plane_acl(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE control plane ACL", _call)


def create_linode_lke_version_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_version_list tool."""
    return Tool(
        name="linode_lke_version_list",
        description="Lists available Kubernetes versions for LKE",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_lke_version_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_version_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        versions = await client.list_lke_versions()
        return {"count": len(versions), "versions": versions}

    return await execute_tool(cfg, arguments, "list LKE versions", _call)


def create_linode_lke_version_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_version_get tool."""
    return Tool(
        name="linode_lke_version_get",
        description="Gets details of a specific LKE Kubernetes version",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "version": {
                    "type": "string",
                    "description": "The Kubernetes version (e.g. '1.29') (required)",
                },
            },
            "required": ["version"],
        },
    ), Capability.Read


async def handle_linode_lke_version_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_version_get tool request."""
    version = arguments.get("version", "")
    if not version:
        return error_response("version is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_version(str(version))

    return await execute_tool(cfg, arguments, "get LKE version", _call)


def create_linode_lke_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_type_list tool."""
    return Tool(
        name="linode_lke_type_list",
        description="Lists available node types for LKE clusters",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_lke_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_lke_types()
        return {"count": len(types), "types": types}

    return await execute_tool(cfg, arguments, "list LKE types", _call)


def create_linode_lke_tier_version_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_tier_version_list tool."""
    return Tool(
        name="linode_lke_tier_version_list",
        description="Lists LKE Kubernetes versions for a tier",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "tier": _LKE_TIER_ID_PROP,
            },
            "required": ["tier"],
        },
    ), Capability.Read


async def handle_linode_lke_tier_version_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_tier_version_list tool request."""
    tier = _validate_lke_tier(arguments.get("tier"))
    if tier is None:
        return error_response("tier must be a non-empty path segment")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        versions = await client.list_lke_tier_versions(tier)
        return {"count": len(versions), "tier_versions": versions}

    return await execute_tool(cfg, arguments, "list LKE tier versions", _call)


def create_linode_lke_tier_version_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_tier_version_get tool."""
    return Tool(
        name="linode_lke_tier_version_get",
        description="Gets details of a specific LKE Kubernetes version for any tier",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "tier": {
                    "type": "string",
                    "description": "The LKE tier (required)",
                },
                "version": {
                    "type": "string",
                    "description": "The Kubernetes version (required)",
                },
            },
            "required": ["tier", "version"],
        },
    ), Capability.Read


async def handle_linode_lke_tier_version_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_tier_version_get tool request."""
    tier = _validate_lke_path_segment(arguments.get("tier", ""))
    if not tier:
        return error_response(
            "tier must be a non-empty string without '/', '?', or '..'"
        )
    version = _validate_lke_path_segment(arguments.get("version", ""))
    if not version:
        return error_response(
            "version must be a non-empty string without '/', '?', or '..'"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_tier_version(tier, version)

    return await execute_tool(cfg, arguments, "get LKE tier version", _call)
