"""LKE (Kubernetes) WRITE tools for LinodeMCP."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

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


def create_linode_lke_cluster_create_tool() -> tuple[Tool, Capability]:
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "region", "k8s_version", "node_pools", "confirm"],
        },
    ), Capability.Write


def _lke_cluster_create_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate cluster_create args; return an error response or None.

    Extracted to keep handle_linode_lke_cluster_create under PLR0911's
    return-count threshold once the dry-run branch is added.
    """
    if not arguments.get("label", ""):
        return error_response("label is required")
    if not arguments.get("region", ""):
        return error_response("region is required")
    if not arguments.get("k8s_version", ""):
        return error_response("k8s_version is required")
    if not arguments.get("node_pools", []):
        return error_response("node_pools is required")
    return None


async def handle_linode_lke_cluster_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_create tool request."""
    fields_error = _lke_cluster_create_error(arguments)
    if fields_error is not None:
        return fields_error

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_lke_cluster_create",
            arguments.get("environment", ""),
            "POST",
            "/lke/clusters",
            None,
        )

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


def create_linode_lke_cluster_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_update tool."""
    return Tool(
        name="linode_lke_cluster_update",
        description=(
            "Updates an existing LKE cluster."
            " Pass dry_run=true to preview without modifying."
        ),
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_lke_cluster_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_update tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_cluster_update",
            "PUT",
            f"/lke/clusters/{cluster_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_cluster(
            cluster_id=cluster_id,
            label=arguments.get("label"),
            k8s_version=arguments.get("k8s_version"),
            tags=arguments.get("tags"),
            control_plane=arguments.get("control_plane"),
        )

    return await execute_tool(cfg, arguments, "update LKE cluster", _call)


def create_linode_lke_cluster_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_delete tool."""
    return Tool(
        name="linode_lke_cluster_delete",
        description=(
            "Deletes an LKE cluster and all associated resources."
            " Pass dry_run=true to preview without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_cluster_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_delete tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_cluster_delete",
            "DELETE",
            f"/lke/clusters/{cluster_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} deleted successfully",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE cluster", _call)


def create_linode_lke_cluster_recycle_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_recycle tool."""
    return Tool(
        name="linode_lke_cluster_recycle",
        description=(
            "Recycles all nodes in an LKE cluster."
            " Pass dry_run=true to preview without recycling."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_cluster_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_recycle tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_cluster_recycle",
            "POST",
            f"/lke/clusters/{cluster_id}/recycle",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} nodes recycled successfully",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE cluster", _call)


def create_linode_lke_cluster_regenerate_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_cluster_regenerate tool."""
    return Tool(
        name="linode_lke_cluster_regenerate",
        description=(
            "Regenerates the service token for an LKE cluster."
            " Pass dry_run=true to preview without regenerating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_cluster_regenerate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_regenerate tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):
        # Fetch the cluster (not the service token) so dry_run surfaces
        # cluster metadata without exposing the token credential.
        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_cluster_regenerate",
            "POST",
            f"/lke/clusters/{cluster_id}/regenerate",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.regenerate_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} service token regenerated",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "regenerate LKE cluster", _call)


def create_linode_lke_pool_create_tool() -> tuple[Tool, Capability]:
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "type", "count", "confirm"],
        },
    ), Capability.Write


def _parse_pool_create(
    arguments: dict[str, Any],
) -> tuple[int, str, int] | list[TextContent]:
    """Parse and validate cluster_id + type + count for pool creation.

    Extracted to keep handle_linode_lke_pool_create under PLR0911's
    return-count threshold once the dry-run branch is added.
    """
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    node_type = arguments.get("type", "")
    if not node_type:
        return error_response("type is required")

    count = arguments.get("count", 0)
    if not count:
        return error_response("count is required and must be > 0")

    return cluster_id, str(node_type), int(count)


async def handle_linode_lke_pool_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_create tool request."""
    parsed = _parse_pool_create(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, node_type, count = parsed

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_pool_create",
            "POST",
            f"/lke/clusters/{cluster_id}/pools",
            _fetch,
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_lke_node_pool(
            cluster_id=cluster_id,
            node_type=node_type,
            count=count,
            autoscaler=arguments.get("autoscaler"),
            tags=arguments.get("tags"),
        )

    return await execute_tool(cfg, arguments, "create LKE node pool", _call)


def create_linode_lke_pool_update_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_lke_pool_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_update tool request."""
    parsed = _parse_cluster_pool_ids(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, pool_id = parsed

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_node_pool(cluster_id, pool_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_pool_update",
            "PUT",
            f"/lke/clusters/{cluster_id}/pools/{pool_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_node_pool(
            cluster_id=cluster_id,
            pool_id=pool_id,
            count=arguments.get("count"),
            autoscaler=arguments.get("autoscaler"),
            tags=arguments.get("tags"),
        )

    return await execute_tool(cfg, arguments, "update LKE node pool", _call)


def create_linode_lke_pool_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_pool_delete tool."""
    return Tool(
        name="linode_lke_pool_delete",
        description=(
            "Deletes a node pool from an LKE cluster."
            " Pass dry_run=true to preview without deleting."
        ),
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    ), Capability.Destroy


def _parse_cluster_pool_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse and validate cluster_id + pool_id from arguments.

    Returns (cluster_id, pool_id) on success or an error response list
    on failure. Extracted to keep handle_linode_lke_pool_delete under
    PLR0911's return-count threshold.
    """
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    pool_id_str = arguments.get("pool_id", "")
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
    return cluster_id, pool_id


async def handle_linode_lke_pool_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_delete tool request."""
    parsed = _parse_cluster_pool_ids(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, pool_id = parsed

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_node_pool(cluster_id, pool_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_pool_delete",
            "DELETE",
            f"/lke/clusters/{cluster_id}/pools/{pool_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_node_pool(cluster_id, pool_id)
        return {
            "message": f"Node pool {pool_id} deleted from cluster {cluster_id}",
            "cluster_id": cluster_id,
            "pool_id": pool_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE node pool", _call)


def create_linode_lke_pool_recycle_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_pool_recycle tool."""
    return Tool(
        name="linode_lke_pool_recycle",
        description=(
            "Recycles all nodes in a node pool."
            " Pass dry_run=true to preview without recycling."
        ),
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_pool_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_recycle tool request."""
    parsed = _parse_cluster_pool_ids(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, pool_id = parsed

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_node_pool(cluster_id, pool_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_pool_recycle",
            "POST",
            f"/lke/clusters/{cluster_id}/pools/{pool_id}/recycle",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_node_pool(cluster_id, pool_id)
        return {
            "message": f"Node pool {pool_id} in cluster {cluster_id} recycled",
            "cluster_id": cluster_id,
            "pool_id": pool_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE node pool", _call)


def create_linode_lke_node_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_node_delete tool."""
    return Tool(
        name="linode_lke_node_delete",
        description=(
            "Deletes a specific node from an LKE cluster."
            " Pass dry_run=true to preview without deleting."
        ),
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "node_id", "confirm"],
        },
    ), Capability.Destroy


def _parse_cluster_node_ids(
    arguments: dict[str, Any],
) -> tuple[int, str] | list[TextContent]:
    """Parse and validate cluster_id (int) + node_id (string).

    Extracted to keep handle_linode_lke_node_delete under PLR0911's
    return-count threshold.
    """
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
    return cluster_id, str(node_id)


async def handle_linode_lke_node_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_delete tool request."""
    parsed = _parse_cluster_node_ids(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, node_id = parsed

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_node(cluster_id, node_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_node_delete",
            "DELETE",
            f"/lke/clusters/{cluster_id}/nodes/{node_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_node(cluster_id, node_id)
        return {
            "message": f"Node {node_id} deleted from cluster {cluster_id}",
            "cluster_id": cluster_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE node", _call)


def create_linode_lke_node_recycle_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_node_recycle tool."""
    return Tool(
        name="linode_lke_node_recycle",
        description=(
            "Recycles a specific node in an LKE cluster."
            " Pass dry_run=true to preview without recycling."
        ),
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "node_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_node_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_recycle tool request."""
    parsed = _parse_cluster_node_ids(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, node_id = parsed

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_node(cluster_id, node_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_node_recycle",
            "POST",
            f"/lke/clusters/{cluster_id}/nodes/{node_id}/recycle",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_node(cluster_id, node_id)
        return {
            "message": f"Node {node_id} in cluster {cluster_id} recycled",
            "cluster_id": cluster_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE node", _call)


def create_linode_lke_kubeconfig_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_kubeconfig_delete tool."""
    return Tool(
        name="linode_lke_kubeconfig_delete",
        description=(
            "Deletes and regenerates the kubeconfig for an LKE cluster."
            " Pass dry_run=true to preview without regenerating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_kubeconfig_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_kubeconfig_delete tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):
        # Fetch the cluster (not the kubeconfig contents) so dry_run
        # surfaces cluster metadata without exposing credential material.
        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_kubeconfig_delete",
            "DELETE",
            f"/lke/clusters/{cluster_id}/kubeconfig",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_kubeconfig(cluster_id)
        return {
            "message": f"Kubeconfig for cluster {cluster_id} regenerated",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE kubeconfig", _call)


def create_linode_lke_service_token_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_service_token_delete tool."""
    return Tool(
        name="linode_lke_service_token_delete",
        description=(
            "Deletes the service token for an LKE cluster."
            " Pass dry_run=true to preview without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_service_token_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_service_token_delete tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):
        # Fetch the cluster (not the service token) so dry_run surfaces
        # cluster metadata without exposing the token credential.
        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_cluster(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_service_token_delete",
            "DELETE",
            f"/lke/clusters/{cluster_id}/servicetoken",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_service_token(cluster_id)
        return {
            "message": f"Service token for cluster {cluster_id} deleted",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE service token", _call)


def create_linode_lke_acl_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_acl_update tool."""
    return Tool(
        name="linode_lke_acl_update",
        description=(
            "Updates the control plane ACL for an LKE cluster."
            " Pass dry_run=true to preview without modifying."
        ),
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "acl", "confirm"],
        },
    ), Capability.Write


def _parse_acl_update(
    arguments: dict[str, Any],
) -> tuple[int, dict[str, Any]] | list[TextContent]:
    """Parse and validate cluster_id + acl for an ACL update.

    Extracted so the dry-run and real paths share validation while the
    real path keeps its confirm-before-acl precedence, and to stay under
    PLR0911's return-count threshold.
    """
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    acl = arguments.get("acl")
    if not acl:
        return error_response("acl is required")

    return cluster_id, acl


async def handle_linode_lke_acl_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_update tool request."""
    if is_dry_run(arguments):
        parsed = _parse_acl_update(arguments)
        if isinstance(parsed, list):
            return parsed
        cluster_id, _acl = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_control_plane_acl(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_acl_update",
            "PUT",
            f"/lke/clusters/{cluster_id}/control_plane_acl",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response("Set confirm=true to proceed.")

    parsed = _parse_acl_update(arguments)
    if isinstance(parsed, list):
        return parsed
    cluster_id, acl = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_control_plane_acl(cluster_id, acl)

    return await execute_tool(cfg, arguments, "update LKE control plane ACL", _call)


def create_linode_lke_acl_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_lke_acl_delete tool."""
    return Tool(
        name="linode_lke_acl_delete",
        description=(
            "Deletes the control plane ACL for an LKE cluster."
            " Pass dry_run=true to preview without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_lke_acl_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_delete tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return error_response("cluster_id must be a valid integer")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_lke_control_plane_acl(cluster_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_lke_acl_delete",
            "DELETE",
            f"/lke/clusters/{cluster_id}/control_plane_acl",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_control_plane_acl(cluster_id)
        return {
            "message": f"Control plane ACL for cluster {cluster_id} deleted",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE control plane ACL", _call)
