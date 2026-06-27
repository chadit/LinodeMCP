from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def nodebalancer_to_response_dict(nodebalancer: Any) -> dict[str, Any]:
    """Shape a NodeBalancer dataclass to proto-canonical NodeBalancer form."""
    transfer = nodebalancer.transfer
    return {
        "id": nodebalancer.id,
        "label": nodebalancer.label,
        "region": nodebalancer.region,
        "hostname": nodebalancer.hostname,
        "ipv4": nodebalancer.ipv4,
        "ipv6": nodebalancer.ipv6,
        "client_conn_throttle": nodebalancer.client_conn_throttle,
        "transfer": {
            "in": transfer.in_,
            "out": transfer.out,
            "total": transfer.total,
        },
        "tags": nodebalancer.tags or [],
        "created": nodebalancer.created,
        "updated": nodebalancer.updated,
    }


def create_linode_nodebalancer_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_type_list tool."""
    return Tool(
        name="linode_nodebalancer_type_list",
        description="Lists all available NodeBalancer types.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
            },
        },
    ), Capability.Read


def create_linode_nodebalancer_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_list tool."""
    return Tool(
        name="linode_nodebalancer_list",
        description=(
            "Lists all NodeBalancers on your account. Can filter by region or label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "region": {
                    "type": "string",
                    "description": "Filter by region ID (e.g., us-east, eu-west)",
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter NodeBalancers by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_nodebalancer_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_nodebalancer_types()

        types_data = [
            {
                "id": t.get("id", ""),
                "label": t.get("label", ""),
                "price": t.get("price", {}),
            }
            for t in types
        ]

        return {
            "count": len(types),
            "types": types_data,
        }

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer types", _call)


async def handle_linode_nodebalancer_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_list tool request."""
    region_filter = arguments.get("region", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nodebalancers = await client.list_nodebalancers()

        if region_filter:
            nodebalancers = [
                nb for nb in nodebalancers if nb.region.lower() == region_filter.lower()
            ]

        if label_contains:
            nodebalancers = [
                nb for nb in nodebalancers if label_contains.lower() in nb.label.lower()
            ]

        nodebalancers_data = [
            {
                "id": nb.id,
                "label": nb.label,
                "region": nb.region,
                "hostname": nb.hostname,
                "ipv4": nb.ipv4,
                "created": nb.created,
                "updated": nb.updated,
            }
            for nb in nodebalancers
        ]

        response: dict[str, Any] = {
            "count": len(nodebalancers),
            "nodebalancers": nodebalancers_data,
        }

        filters: list[str] = []
        if region_filter:
            filters.append(f"region={region_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve NodeBalancers", _call)


def create_linode_nodebalancer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_get tool."""
    return Tool(
        name="linode_nodebalancer_get",
        description=(
            "Gets detailed information about a specific NodeBalancer by its ID."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerGetInput"),
    ), Capability.Read


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


async def handle_linode_nodebalancer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_get tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")

    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.get_nodebalancer(nodebalancer_id)
        return nodebalancer_to_response_dict(nb)

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer", _call)


def nodebalancer_vpc_config_to_response_dict(
    config: dict[str, Any],
) -> dict[str, Any]:
    """Shape a raw NodeBalancer VPC config API dict to proto-canonical form.

    vpc_id, ipv4_range_id, ipv6_range_id, ipv4_range_auto_assign are nullable and
    omitted when null, matching the proto optional fields.
    """
    body: dict[str, Any] = {
        "id": config.get("id", 0),
        "subnet_id": config.get("subnet_id", 0),
        "ipv4_range": config.get("ipv4_range", ""),
        "ipv6_range": config.get("ipv6_range", ""),
        "nodebalancer_id": config.get("nodebalancer_id", 0),
    }
    if config.get("vpc_id") is not None:
        body["vpc_id"] = config["vpc_id"]
    if config.get("ipv4_range_id") is not None:
        body["ipv4_range_id"] = config["ipv4_range_id"]
    if config.get("ipv6_range_id") is not None:
        body["ipv6_range_id"] = config["ipv6_range_id"]
    if config.get("ipv4_range_auto_assign") is not None:
        body["ipv4_range_auto_assign"] = config["ipv4_range_auto_assign"]
    return body


def create_linode_nodebalancer_vpc_config_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_vpc_config_get tool."""
    return Tool(
        name="linode_nodebalancer_vpc_config_get",
        description="Gets a VPC configuration for a NodeBalancer.",
        inputSchema=schema("linode.mcp.v1.NodeBalancerVPCConfigGetInput"),
    ), Capability.Read


def create_linode_nodebalancer_vpc_config_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_vpc_config_list tool."""
    return Tool(
        name="linode_nodebalancer_vpc_config_list",
        description="Lists VPC configurations for a NodeBalancer.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer (required)",
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["nodebalancer_id"],
        },
    ), Capability.Read


async def handle_linode_nodebalancer_vpc_config_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_vpc_config_list tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_nodebalancer_vpc_configs(
            nodebalancer_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer VPC configurations", _call
    )


async def handle_linode_nodebalancer_vpc_config_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_vpc_config_get tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    vpc_config_id = _positive_int_argument(arguments, "vpc_config_id")
    if vpc_config_id is None:
        return error_response("vpc_config_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return nodebalancer_vpc_config_to_response_dict(
            await client.get_nodebalancer_vpc_config(nodebalancer_id, vpc_config_id)
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer VPC configuration", _call
    )


def create_linode_nodebalancer_stats_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_stats_get tool."""
    return Tool(
        name="linode_nodebalancer_stats_get",
        description=(
            "Gets detailed statistics about a specific NodeBalancer by its ID, "
            "including connections and traffic data."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "NodeBalancer ID for stats (required)",
                },
            },
            "required": ["nodebalancer_id"],
        },
    ), Capability.Read


async def handle_linode_nodebalancer_stats_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_stats_get tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")

    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_nodebalancer_stats(nodebalancer_id)

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer statistics", _call)


def create_linode_nodebalancer_firewall_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_firewall_list tool."""
    return Tool(
        name="linode_nodebalancer_firewall_list",
        description=("Lists firewalls assigned to a specific NodeBalancer by its ID."),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer (required)",
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["nodebalancer_id"],
        },
    ), Capability.Read


async def handle_linode_nodebalancer_firewall_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_firewall_list tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_nodebalancer_firewalls(
            nodebalancer_id, page=page, page_size=page_size
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer firewalls", _call)


def create_linode_nodebalancer_config_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_list tool."""
    return Tool(
        name="linode_nodebalancer_config_list",
        description="Lists configs for a NodeBalancer.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "nodebalancer_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the NodeBalancer (required)",
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["nodebalancer_id"],
        },
    ), Capability.Read


async def handle_linode_nodebalancer_config_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_list tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_nodebalancer_configs(
            nodebalancer_id, page=page, page_size=page_size
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer configs", _call)


def nodebalancer_config_to_response_dict(config: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw NodeBalancer config API dict to proto-canonical form."""
    nodes_status: dict[str, Any] = config.get("nodes_status") or {}
    return {
        "id": config.get("id", 0),
        "port": config.get("port", 0),
        "protocol": config.get("protocol", ""),
        "algorithm": config.get("algorithm", ""),
        "stickiness": config.get("stickiness", ""),
        "check": config.get("check", ""),
        "check_interval": config.get("check_interval", 0),
        "check_timeout": config.get("check_timeout", 0),
        "check_attempts": config.get("check_attempts", 0),
        "check_path": config.get("check_path", ""),
        "check_body": config.get("check_body", ""),
        "check_passive": config.get("check_passive", False),
        "cipher_suite": config.get("cipher_suite", ""),
        "ssl_commonname": config.get("ssl_commonname", ""),
        "ssl_fingerprint": config.get("ssl_fingerprint", ""),
        "nodebalancer_id": config.get("nodebalancer_id", 0),
        "nodes_status": {
            "up": nodes_status.get("up", 0),
            "down": nodes_status.get("down", 0),
        },
    }


def create_linode_nodebalancer_config_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_get tool."""
    return Tool(
        name="linode_nodebalancer_config_get",
        description="Gets a specific NodeBalancer config.",
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigGetInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_config_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_get tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return nodebalancer_config_to_response_dict(
            await client.get_nodebalancer_config(nodebalancer_id, config_id)
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer config", _call)


def create_linode_nodebalancer_config_node_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_list tool."""
    return Tool(
        name="linode_nodebalancer_config_node_list",
        description="Lists backend nodes in a NodeBalancer config.",
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
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["nodebalancer_id", "config_id"],
        },
    ), Capability.Read


async def handle_linode_nodebalancer_config_node_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_list tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_nodebalancer_config_nodes(
            nodebalancer_id, config_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer config nodes", _call
    )


def nodebalancer_config_node_to_response_dict(node: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw NodeBalancer config node API dict to proto-canonical form."""
    return {
        "id": node.get("id", 0),
        "address": node.get("address", ""),
        "label": node.get("label", ""),
        "status": node.get("status", ""),
        "weight": node.get("weight", 0),
        "mode": node.get("mode", ""),
        "nodebalancer_id": node.get("nodebalancer_id", 0),
        "config_id": node.get("config_id", 0),
    }


def create_linode_nodebalancer_config_node_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_get tool."""
    return Tool(
        name="linode_nodebalancer_config_node_get",
        description=(
            "Gets detailed information about a specific node in a NodeBalancer config."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigNodeGetInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_config_node_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_get tool request."""
    nodebalancer_id = _positive_int_argument(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response("nodebalancer_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    node_id = _positive_int_argument(arguments, "node_id")
    if node_id is None:
        return error_response("node_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return nodebalancer_config_node_to_response_dict(
            await client.get_nodebalancer_config_node(
                nodebalancer_id, config_id, node_id
            )
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer config node", _call
    )
