from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _positive_int_argument(arguments: dict[str, Any], name: str) -> int | None:
    value = arguments.get(name)
    if isinstance(value, bool) or not isinstance(value, int) or value < 1:
        return None
    return value


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def _firewall_ids_argument(arguments: dict[str, Any]) -> list[int] | None:
    raw_value: object = arguments.get("firewall_ids")
    if not isinstance(raw_value, list):
        return None

    firewall_ids: list[int] = []
    for item in cast("list[object]", raw_value):
        if isinstance(item, bool) or not isinstance(item, int) or item < 1:
            return None
        firewall_ids.append(item)
    return firewall_ids


def create_linode_nodebalancer_firewalls_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_firewalls_update tool."""
    return Tool(
        name="linode_nodebalancer_firewalls_update",
        description=(
            "Replaces the firewall assignments for a NodeBalancer. "
            "Pass an empty firewall_ids list to remove all assignments."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer (required)",
                },
                "firewall_ids": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": (
                        "Complete list of Firewall IDs to assign. Use [] to remove all."
                    ),
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of assigned Firewall results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of assigned Firewall results per page",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to replace NodeBalancer firewall assignments."
                    ),
                },
            },
            "required": ["nodebalancer_id", "firewall_ids", "confirm"],
        },
    ), Capability.Write


async def handle_linode_nodebalancer_firewalls_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_firewalls_update tool request."""
    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    firewall_ids = _firewall_ids_argument(arguments)
    if firewall_ids is None:
        return error_response("firewall_ids must be a list of positive integers")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_nodebalancer_firewalls(
            nodebalancer_id, firewall_ids, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "update NodeBalancer firewall assignments", _call
    )


def create_linode_nodebalancer_config_rebuild_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_rebuild tool."""
    return Tool(
        name="linode_nodebalancer_config_rebuild",
        description=(
            "Rebuilds a NodeBalancer config. "
            "Requires confirm because active connections may be affected."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer config (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to rebuild the NodeBalancer config.",
                },
            },
            "required": ["nodebalancer_id", "config_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_nodebalancer_config_rebuild(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_rebuild tool request."""
    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.rebuild_nodebalancer_config(nodebalancer_id, config_id)
        if result:
            return result
        return {
            "message": (
                f"NodeBalancer config {config_id} rebuild requested "
                f"for NodeBalancer {nodebalancer_id}"
            ),
            "nodebalancer_id": nodebalancer_id,
            "config_id": config_id,
        }

    return await execute_tool(cfg, arguments, "rebuild NodeBalancer config", _call)


def create_linode_nodebalancer_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_create tool."""
    return Tool(
        name="linode_nodebalancer_create",
        description=(
            "Creates a new NodeBalancer (load balancer). "
            "WARNING: Billing starts immediately."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "region": {
                    "type": "string",
                    "description": "Region for the NodeBalancer (required)",
                },
                "label": {
                    "type": "string",
                    "description": "Label for the NodeBalancer (optional)",
                },
                "client_conn_throttle": {
                    "type": "integer",
                    "description": "Connections per second throttle (0-20, default: 0)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["region", "confirm"],
        },
    ), Capability.Write


async def handle_linode_nodebalancer_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_create tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    region = arguments.get("region", "")
    if not region:
        return error_response("region is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.create_nodebalancer(
            region=region,
            label=arguments.get("label"),
            client_conn_throttle=arguments.get("client_conn_throttle", 0),
        )
        return {
            "message": (
                f"NodeBalancer '{nb.label}' (ID: {nb.id}) "
                f"created successfully in {nb.region}"
            ),
            "nodebalancer": {
                "id": nb.id,
                "label": nb.label,
                "region": nb.region,
                "hostname": nb.hostname,
                "ipv4": nb.ipv4,
                "ipv6": nb.ipv6,
            },
        }

    return await execute_tool(cfg, arguments, "create NodeBalancer", _call)


def create_linode_nodebalancer_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_update tool."""
    return Tool(
        name="linode_nodebalancer_update",
        description="Updates an existing NodeBalancer.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "description": "The ID of the NodeBalancer to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New label (optional)",
                },
                "client_conn_throttle": {
                    "type": "integer",
                    "description": "New throttle limit (0-20, optional)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
            },
            "required": ["nodebalancer_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_nodebalancer_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_update tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return error_response("nodebalancer_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.update_nodebalancer(
            nodebalancer_id=int(nodebalancer_id),
            label=arguments.get("label"),
            client_conn_throttle=arguments.get("client_conn_throttle"),
        )
        return {
            "message": f"NodeBalancer {nodebalancer_id} updated successfully",
            "nodebalancer": {
                "id": nb.id,
                "label": nb.label,
                "client_conn_throttle": nb.client_conn_throttle,
                "updated": nb.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update NodeBalancer", _call)


def create_linode_nodebalancer_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_delete tool."""
    return Tool(
        name="linode_nodebalancer_delete",
        description=(
            "Deletes a NodeBalancer. WARNING: This removes the load balancer "
            "and all its configurations."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "description": "The ID of the NodeBalancer to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["nodebalancer_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_nodebalancer_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_delete tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not nodebalancer_id:
        return error_response("nodebalancer_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer(int(nodebalancer_id))
        return {
            "message": f"NodeBalancer {nodebalancer_id} deleted successfully",
            "nodebalancer_id": nodebalancer_id,
        }

    return await execute_tool(cfg, arguments, "delete NodeBalancer", _call)


def create_linode_nodebalancer_config_node_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_delete tool."""
    return Tool(
        name="linode_nodebalancer_config_node_delete",
        description=(
            "Deletes a node from a NodeBalancer config. "
            "WARNING: This removes the backend node from the load balancer."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer config (required)",
                },
                "node_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the node to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["nodebalancer_id", "config_id", "node_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_nodebalancer_config_node_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_delete tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)
    config_id = arguments.get("config_id", 0)
    node_id = arguments.get("node_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not nodebalancer_id:
        return error_response("nodebalancer_id is required")

    if not config_id:
        return error_response("config_id is required")

    if not node_id:
        return error_response("node_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer_config_node(
            int(nodebalancer_id), int(config_id), int(node_id)
        )
        return {
            "message": (
                f"Node {node_id} deleted from NodeBalancer {nodebalancer_id} "
                f"config {config_id}"
            ),
            "nodebalancer_id": nodebalancer_id,
            "config_id": config_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "delete NodeBalancer config node", _call)
