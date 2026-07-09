"""LKE (Kubernetes) READ tools for LinodeMCP."""

from __future__ import annotations

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
from linodemcp.tools.proto_enum import enum_choice_error
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


def create_linode_lke_cluster_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_list tool."""
    return Tool(
        name="linode_lke_cluster_list",
        description=(
            "Lists all Linode Kubernetes Engine (LKE) clusters. Can filter by label."
        ),
        inputSchema=schema("linode.mcp.v1.LKEClusterListInput"),
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
        inputSchema=schema("linode.mcp.v1.LKENodePoolListInput"),
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
        inputSchema=schema("linode.mcp.v1.LKEAPIEndpointListInput"),
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
        inputSchema=schema("linode.mcp.v1.LKEACLGetInput"),
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
        acl = await client.get_lke_control_plane_acl(cluster_id)
        return serialize_api_response(acl, lke_pb2.LKEControlPlaneACL())

    return await execute_tool(cfg, arguments, "get LKE control plane ACL", _call)


def create_linode_lke_version_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_version_list tool."""
    return Tool(
        name="linode_lke_version_list",
        description="Lists available Kubernetes versions for LKE",
        inputSchema=schema("linode.mcp.v1.LKEVersionListInput"),
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
    if not isinstance(version, str) or not version:
        return error_response("version is required")
    # Reject path-unsafe values locally (mirrors Go validateLKEVersionID) so a
    # traversal or query segment cannot ride into the version path segment.
    if "/" in version or "?" in version or ".." in version:
        return error_response("version must be a Kubernetes version ID")

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
        inputSchema=schema("linode.mcp.v1.LKETypeListInput"),
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
        inputSchema=schema("linode.mcp.v1.LKETierVersionListInput"),
    ), Capability.Read


async def handle_linode_lke_tier_version_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_tier_version_list tool request."""
    tier = arguments.get("tier", "")
    if not isinstance(tier, str) or not tier:
        return error_response("tier is required")
    tier_error = enum_choice_error(tier, "tier", lke_tier_version_pb2.LKETier.Value)
    if tier_error is not None:
        return error_response(tier_error)

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
    tier = arguments.get("tier", "")
    if not isinstance(tier, str) or not tier:
        return error_response("tier is required")
    tier_error = enum_choice_error(tier, "tier", lke_tier_version_pb2.LKETier.Value)
    if tier_error is not None:
        return error_response(tier_error)
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
