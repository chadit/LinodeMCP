from __future__ import annotations

from typing import TYPE_CHECKING, Any
from urllib.parse import quote

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    firewall_pb2,
    nodebalancer_config_node_pb2,
    nodebalancer_config_pb2,
    nodebalancer_pb2,
    nodebalancer_stats_pb2,
    nodebalancer_vpc_config_pb2,
    type_pb2,
)
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    error_response,
    execute_tool,
    pagination_int_argument,
    required_int_id,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_nodebalancer_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_type_list tool."""
    return Tool(
        name="linode_nodebalancer_type_list",
        description="Lists all available NodeBalancer types.",
        inputSchema=schema("linode.mcp.v1.NodeBalancerTypeListInput"),
    ), Capability.Read


def create_linode_nodebalancer_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_list tool."""
    return Tool(
        name="linode_nodebalancer_list",
        description=(
            "Lists all NodeBalancers on your account. Can filter by region or label."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerListInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_nodebalancer_types()
        return serialize_list_response(
            {"data": types},
            "nodebalancer_types",
            type_pb2.NodeBalancerTypeListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer types", _call)


async def handle_linode_nodebalancer_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_list tool request."""
    region_filter = arguments.get("region", "")
    label_contains = arguments.get("label_contains", "")

    def _matches(nodebalancer: dict[str, Any]) -> bool:
        region = str(nodebalancer.get("region", ""))
        if region_filter and region.lower() != region_filter.lower():
            return False
        label = str(nodebalancer.get("label", ""))
        return not (label_contains and label_contains.lower() not in label.lower())

    filters: list[str] = []
    if region_filter:
        filters.append(f"region={region_filter}")
    if label_contains:
        filters.append(f"label_contains={label_contains}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/nodebalancers")
        return serialize_list_response(
            raw,
            "nodebalancers",
            nodebalancer_pb2.NodeBalancerListResponse(),
            filter_value=", ".join(filters) if filters else None,
            item_filter=_matches,
        )

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


async def handle_linode_nodebalancer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_get tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")

    if nodebalancer_id is None:
        return error_response(error)

    encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_raw(f"/nodebalancers/{encoded_nodebalancer_id}"),
            nodebalancer_pb2.NodeBalancer(),
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer", _call)


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
        inputSchema=schema("linode.mcp.v1.NodeBalancerVPCConfigListInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_vpc_config_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_vpc_config_list tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_nodebalancer_vpc_configs(
            nodebalancer_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "vpc_configs",
            nodebalancer_vpc_config_pb2.NodeBalancerVPCConfigListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer VPC configurations", _call
    )


async def handle_linode_nodebalancer_vpc_config_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_vpc_config_get tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    vpc_config_id, error = required_int_id(arguments, "vpc_config_id")
    if vpc_config_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_nodebalancer_vpc_config(nodebalancer_id, vpc_config_id),
            nodebalancer_vpc_config_pb2.NodeBalancerVPCConfig(),
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
        inputSchema=schema("linode.mcp.v1.NodeBalancerStatsGetInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_stats_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_stats_get tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")

    if nodebalancer_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_nodebalancer_stats(nodebalancer_id),
            nodebalancer_stats_pb2.NodeBalancerStats(),
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer statistics", _call)


def create_linode_nodebalancer_firewall_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_firewall_list tool."""
    return Tool(
        name="linode_nodebalancer_firewall_list",
        description=("Lists firewalls assigned to a specific NodeBalancer by its ID."),
        inputSchema=schema("linode.mcp.v1.NodeBalancerFirewallListInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_firewall_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_firewall_list tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_nodebalancer_firewalls(
            nodebalancer_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "firewalls",
            firewall_pb2.FirewallListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer firewalls", _call)


def create_linode_nodebalancer_config_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_list tool."""
    return Tool(
        name="linode_nodebalancer_config_list",
        description="Lists configs for a NodeBalancer.",
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigListInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_config_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_list tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_nodebalancer_configs(
            nodebalancer_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "configs",
            nodebalancer_config_pb2.NodeBalancerConfigListResponse(),
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer configs", _call)


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
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_nodebalancer_config(nodebalancer_id, config_id),
            nodebalancer_config_pb2.NodeBalancerConfig(),
        )

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer config", _call)


def create_linode_nodebalancer_config_node_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_list tool."""
    return Tool(
        name="linode_nodebalancer_config_node_list",
        description="Lists backend nodes in a NodeBalancer config.",
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigNodeListInput"),
    ), Capability.Read


async def handle_linode_nodebalancer_config_node_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_list tool request."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_nodebalancer_config_nodes(
            nodebalancer_id, config_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "nodes",
            nodebalancer_config_node_pb2.NodeBalancerConfigNodeListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer config nodes", _call
    )


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
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    node_id, error = required_int_id(arguments, "node_id")
    if node_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_nodebalancer_config_node(
                nodebalancer_id, config_id, node_id
            ),
            nodebalancer_config_node_pb2.NodeBalancerConfigNode(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve NodeBalancer config node", _call
    )
