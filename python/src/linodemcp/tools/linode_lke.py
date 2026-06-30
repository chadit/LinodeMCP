"""LKE (Kubernetes) READ tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    lke_api_endpoint_pb2,
    lke_dashboard_pb2,
    lke_kubeconfig_pb2,
    lke_node_pb2,
    lke_pb2,
    lke_pool_pb2,
    lke_tier_version_pb2,
    lke_version_pb2,
    type_pb2,
)
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema


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


_LKE_LABEL_FILTER_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Filter clusters by label containing this string (case-insensitive)",
}


def create_linode_lke_cluster_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_list tool."""
    return Tool(
        name="linode_lke_cluster_list",
        description=(
            "Lists all Linode Kubernetes Engine (LKE) clusters. Can filter by label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "label": _LKE_LABEL_FILTER_PROP,
            },
        },
    ), Capability.Read


async def handle_linode_lke_cluster_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_list tool request."""
    label_filter = arguments.get("label", "")

    def _matches(cluster: dict[str, Any]) -> bool:
        if not label_filter:
            return True
        return label_filter.lower() in str(cluster.get("label", "")).lower()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/lke/clusters")
        return serialize_list_response(
            raw,
            "clusters",
            lke_pb2.LKEClusterListResponse(),
            filter_value=f"label={label_filter}" if label_filter else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "list LKE clusters", _call)


def lke_cluster_to_response_dict(cluster: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw LKE cluster API dict to proto-canonical LKECluster form.

    Retained for the LKE cluster create/update write tools, which embed the
    shaped cluster in their {message, cluster} envelope. The read tool itself
    now serializes through serialize_api_response.
    """
    control_plane: dict[str, Any] = cluster.get("control_plane") or {}
    return {
        "id": cluster.get("id", 0),
        "label": cluster.get("label", ""),
        "region": cluster.get("region", ""),
        "k8s_version": cluster.get("k8s_version", ""),
        "status": cluster.get("status", ""),
        "tags": cluster.get("tags") or [],
        "created": cluster.get("created", ""),
        "updated": cluster.get("updated", ""),
        "control_plane": {
            "high_availability": control_plane.get("high_availability", False),
        },
    }


def create_linode_lke_cluster_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_get tool."""
    return Tool(
        name="linode_lke_cluster_get",
        description="Gets details of a specific LKE cluster by ID",
        inputSchema=schema("linode.mcp.v1.LKEClusterGetInput"),
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
        return serialize_api_response(
            await client.get_lke_cluster(cluster_id), lke_pb2.LKECluster()
        )

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
        return serialize_list_response(
            {"data": pools},
            "pools",
            lke_pool_pb2.LKENodePoolListResponse(),
        )

    return await execute_tool(cfg, arguments, "list LKE node pools", _call)


def create_linode_lke_pool_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_pool_get tool."""
    return Tool(
        name="linode_lke_pool_get",
        description="Gets details of a specific node pool in an LKE cluster",
        inputSchema=schema("linode.mcp.v1.LKENodePoolGetInput"),
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
        return serialize_api_response(
            await client.get_lke_node_pool(cluster_id, pool_id),
            lke_pool_pb2.LKENodePool(),
        )

    return await execute_tool(cfg, arguments, "get LKE node pool", _call)


def create_linode_lke_node_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_node_get tool."""
    return Tool(
        name="linode_lke_node_get",
        description="Gets details of a specific node in an LKE cluster",
        inputSchema=schema("linode.mcp.v1.LKENodeGetInput"),
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
        return serialize_api_response(
            await client.get_lke_node(cluster_id, str(node_id)),
            lke_node_pb2.LKENode(),
        )

    return await execute_tool(cfg, arguments, "get LKE node", _call)


def create_linode_lke_kubeconfig_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_kubeconfig_get tool."""
    return Tool(
        name="linode_lke_kubeconfig_get",
        description="Gets the kubeconfig for an LKE cluster",
        inputSchema=schema("linode.mcp.v1.LKEKubeconfigGetInput"),
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
        return serialize_api_response(
            await client.get_lke_kubeconfig(cluster_id),
            lke_kubeconfig_pb2.LKEKubeconfig(),
        )

    return await execute_tool(cfg, arguments, "get LKE kubeconfig", _call)


def create_linode_lke_dashboard_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_dashboard_get tool."""
    return Tool(
        name="linode_lke_dashboard_get",
        description="Gets the dashboard URL for an LKE cluster",
        inputSchema=schema("linode.mcp.v1.LKEDashboardGetInput"),
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
        return serialize_api_response(
            await client.get_lke_dashboard(cluster_id),
            lke_dashboard_pb2.LKEDashboard(),
        )

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
        return serialize_list_response(
            {"data": endpoints},
            "endpoints",
            lke_api_endpoint_pb2.LKEAPIEndpointListResponse(),
        )

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
        return serialize_list_response(
            {"data": versions},
            "versions",
            lke_version_pb2.LKEVersionListResponse(),
        )

    return await execute_tool(cfg, arguments, "list LKE versions", _call)


def create_linode_lke_version_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_version_get tool."""
    return Tool(
        name="linode_lke_version_get",
        description="Gets details of a specific LKE Kubernetes version",
        inputSchema=schema("linode.mcp.v1.LKEVersionGetInput"),
    ), Capability.Read


async def handle_linode_lke_version_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_version_get tool request."""
    version = arguments.get("version", "")
    if not version:
        return error_response("version is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_lke_version(str(version)),
            lke_version_pb2.LKEVersion(),
        )

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
        return serialize_list_response(
            {"data": types},
            "lke_types",
            type_pb2.LKETypeListResponse(),
        )

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
        return serialize_list_response(
            {"data": versions},
            "tier_versions",
            lke_tier_version_pb2.LKETierVersionListResponse(),
        )

    return await execute_tool(cfg, arguments, "list LKE tier versions", _call)


def create_linode_lke_tier_version_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_tier_version_get tool."""
    return Tool(
        name="linode_lke_tier_version_get",
        description="Gets details of a specific LKE Kubernetes version for any tier",
        inputSchema=schema("linode.mcp.v1.LKETierVersionGetInput"),
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
        return serialize_api_response(
            await client.get_lke_tier_version(tier, version),
            lke_tier_version_pb2.LKETierVersion(),
        )

    return await execute_tool(cfg, arguments, "get LKE tier version", _call)
