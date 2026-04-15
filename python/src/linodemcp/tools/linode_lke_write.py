"""LKE (Kubernetes) WRITE tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import _error_response, execute_tool

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

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}


def create_linode_lke_cluster_create_tool() -> Tool:
    """Create the linode_lke_cluster_create tool."""
    return Tool(
        name="linode_lke_cluster_create",
        description="Creates a new LKE (Kubernetes) cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the cluster (required)",
                },
                "region": {
                    "type": "string",
                    "description": "Region for the cluster (required)",
                },
                "k8s_version": {
                    "type": "string",
                    "description": "Kubernetes version (required)",
                },
                "node_pools": {
                    "type": "array",
                    "description": (
                        "Node pools: [{type, count, autoscaler?, tags?}] (required)"
                    ),
                    "items": {"type": "object"},
                },
                "tags": {
                    "type": "array",
                    "description": "Tags for the cluster",
                    "items": {"type": "string"},
                },
                "control_plane": {
                    "type": "object",
                    "description": "Control plane config: {high_availability: bool}",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["label", "region", "k8s_version", "node_pools", "confirm"],
        },
    )


async def handle_linode_lke_cluster_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource. "
                    "Set confirm=true to proceed."
                ),
            )
        ]

    label = arguments.get("label", "")
    region = arguments.get("region", "")
    k8s_version = arguments.get("k8s_version", "")
    node_pools = arguments.get("node_pools", [])
    tags = arguments.get("tags")
    control_plane = arguments.get("control_plane")

    if not label:
        return _error_response("label is required")
    if not region:
        return _error_response("region is required")
    if not k8s_version:
        return _error_response("k8s_version is required")
    if not node_pools:
        return _error_response("node_pools is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_lke_cluster(
            label=label,
            region=region,
            k8s_version=k8s_version,
            node_pools=node_pools,
            tags=tags,
            control_plane=control_plane,
        )

    return await execute_tool(cfg, arguments, "create LKE cluster", _call)


def create_linode_lke_cluster_update_tool() -> Tool:
    """Create the linode_lke_cluster_update tool."""
    return Tool(
        name="linode_lke_cluster_update",
        description="Updates an existing LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "New label for the cluster",
                },
                "k8s_version": {
                    "type": "string",
                    "description": "New Kubernetes version",
                },
                "tags": {
                    "type": "array",
                    "description": "New tags for the cluster",
                    "items": {"type": "string"},
                },
                "control_plane": {
                    "type": "object",
                    "description": "Control plane config: {high_availability: bool}",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_cluster(
            cluster_id=cluster_id,
            label=arguments.get("label"),
            k8s_version=arguments.get("k8s_version"),
            tags=arguments.get("tags"),
            control_plane=arguments.get("control_plane"),
        )

    return await execute_tool(cfg, arguments, "update LKE cluster", _call)


def create_linode_lke_cluster_delete_tool() -> Tool:
    """Create the linode_lke_cluster_delete tool."""
    return Tool(
        name="linode_lke_cluster_delete",
        description="Deletes an LKE cluster and all associated resources",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} deleted successfully",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE cluster", _call)


def create_linode_lke_cluster_recycle_tool() -> Tool:
    """Create the linode_lke_cluster_recycle tool."""
    return Tool(
        name="linode_lke_cluster_recycle",
        description="Recycles all nodes in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_recycle tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} nodes recycled successfully",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE cluster", _call)


def create_linode_lke_cluster_regenerate_tool() -> Tool:
    """Create the linode_lke_cluster_regenerate tool."""
    return Tool(
        name="linode_lke_cluster_regenerate",
        description="Regenerates the service token for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_regenerate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_regenerate tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.regenerate_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} service token regenerated",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "regenerate LKE cluster", _call)


def create_linode_lke_pool_create_tool() -> Tool:
    """Create the linode_lke_pool_create tool."""
    return Tool(
        name="linode_lke_pool_create",
        description="Creates a new node pool in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "type": {
                    "type": "string",
                    "description": "Linode type for pool nodes (required)",
                },
                "count": {
                    "type": "integer",
                    "description": "Number of nodes in the pool (required)",
                },
                "autoscaler": {
                    "type": "object",
                    "description": "Autoscaler config: {enabled, min, max}",
                },
                "tags": {
                    "type": "array",
                    "description": "Tags for the node pool",
                    "items": {"type": "string"},
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["cluster_id", "type", "count", "confirm"],
        },
    )


async def handle_linode_lke_pool_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource. "
                    "Set confirm=true to proceed."
                ),
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    node_type = arguments.get("type", "")
    if not node_type:
        return _error_response("type is required")

    count = arguments.get("count", 0)
    if not count:
        return _error_response("count is required and must be > 0")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_lke_node_pool(
            cluster_id=cluster_id,
            node_type=node_type,
            count=int(count),
            autoscaler=arguments.get("autoscaler"),
            tags=arguments.get("tags"),
        )

    return await execute_tool(cfg, arguments, "create LKE node pool", _call)


def create_linode_lke_pool_update_tool() -> Tool:
    """Create the linode_lke_pool_update tool."""
    return Tool(
        name="linode_lke_pool_update",
        description="Updates a node pool in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
                "count": {
                    "type": "integer",
                    "description": "New number of nodes in the pool",
                },
                "autoscaler": {
                    "type": "object",
                    "description": "Autoscaler config: {enabled, min, max}",
                },
                "tags": {
                    "type": "array",
                    "description": "New tags for the node pool",
                    "items": {"type": "string"},
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    )


async def handle_linode_lke_pool_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_node_pool(
            cluster_id=cluster_id,
            pool_id=pool_id,
            count=arguments.get("count"),
            autoscaler=arguments.get("autoscaler"),
            tags=arguments.get("tags"),
        )

    return await execute_tool(cfg, arguments, "update LKE node pool", _call)


def create_linode_lke_pool_delete_tool() -> Tool:
    """Create the linode_lke_pool_delete tool."""
    return Tool(
        name="linode_lke_pool_delete",
        description="Deletes a node pool from an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    )


async def handle_linode_lke_pool_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_node_pool(cluster_id, pool_id)
        return {
            "message": f"Node pool {pool_id} deleted from cluster {cluster_id}",
            "cluster_id": cluster_id,
            "pool_id": pool_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE node pool", _call)


def create_linode_lke_pool_recycle_tool() -> Tool:
    """Create the linode_lke_pool_recycle tool."""
    return Tool(
        name="linode_lke_pool_recycle",
        description="Recycles all nodes in a node pool",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    )


async def handle_linode_lke_pool_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_recycle tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_node_pool(cluster_id, pool_id)
        return {
            "message": f"Node pool {pool_id} in cluster {cluster_id} recycled",
            "cluster_id": cluster_id,
            "pool_id": pool_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE node pool", _call)


def create_linode_lke_node_delete_tool() -> Tool:
    """Create the linode_lke_node_delete tool."""
    return Tool(
        name="linode_lke_node_delete",
        description="Deletes a specific node from an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "node_id": {
                    "type": "string",
                    "description": "The ID of the node (required, string)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "node_id", "confirm"],
        },
    )


async def handle_linode_lke_node_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    node_id = arguments.get("node_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not node_id:
        return _error_response("node_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_node(cluster_id, str(node_id))
        return {
            "message": f"Node {node_id} deleted from cluster {cluster_id}",
            "cluster_id": cluster_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE node", _call)


def create_linode_lke_node_recycle_tool() -> Tool:
    """Create the linode_lke_node_recycle tool."""
    return Tool(
        name="linode_lke_node_recycle",
        description="Recycles a specific node in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "node_id": {
                    "type": "string",
                    "description": "The ID of the node (required, string)",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "node_id", "confirm"],
        },
    )


async def handle_linode_lke_node_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_recycle tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    node_id = arguments.get("node_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not node_id:
        return _error_response("node_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_node(cluster_id, str(node_id))
        return {
            "message": f"Node {node_id} in cluster {cluster_id} recycled",
            "cluster_id": cluster_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE node", _call)


def create_linode_lke_kubeconfig_delete_tool() -> Tool:
    """Create the linode_lke_kubeconfig_delete tool."""
    return Tool(
        name="linode_lke_kubeconfig_delete",
        description="Deletes and regenerates the kubeconfig for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_kubeconfig_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_kubeconfig_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_kubeconfig(cluster_id)
        return {
            "message": f"Kubeconfig for cluster {cluster_id} regenerated",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE kubeconfig", _call)


def create_linode_lke_service_token_delete_tool() -> Tool:
    """Create the linode_lke_service_token_delete tool."""
    return Tool(
        name="linode_lke_service_token_delete",
        description="Deletes the service token for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_service_token_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_service_token_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_service_token(cluster_id)
        return {
            "message": f"Service token for cluster {cluster_id} deleted",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE service token", _call)


def create_linode_lke_acl_update_tool() -> Tool:
    """Create the linode_lke_acl_update tool."""
    return Tool(
        name="linode_lke_acl_update",
        description="Updates the control plane ACL for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "acl": {
                    "type": "object",
                    "description": (
                        "ACL config: {enabled: bool, addresses: "
                        "{ipv4: [...], ipv6: [...]}}"
                    ),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "acl", "confirm"],
        },
    )


async def handle_linode_lke_acl_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    acl = arguments.get("acl")
    if not acl:
        return _error_response("acl is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_control_plane_acl(cluster_id, acl)

    return await execute_tool(cfg, arguments, "update LKE control plane ACL", _call)


def create_linode_lke_acl_delete_tool() -> Tool:
    """Create the linode_lke_acl_delete tool."""
    return Tool(
        name="linode_lke_acl_delete",
        description="Deletes the control plane ACL for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_acl_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_control_plane_acl(cluster_id)
        return {
            "message": f"Control plane ACL for cluster {cluster_id} deleted",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE control plane ACL", _call)
