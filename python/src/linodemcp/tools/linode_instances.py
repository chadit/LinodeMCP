"""Linode instances list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def _positive_int_argument(arguments: dict[str, Any], name: str) -> int | None:
    value = arguments.get(name)
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return None
    return value


def _optional_int_argument(
    arguments: dict[str, Any],
    name: str,
    minimum: int,
    maximum: int | None = None,
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_instances_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instances_list tool."""
    return Tool(
        name="linode_instances_list",
        description="Lists Linode instances with optional filtering by status",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "status": {
                    "type": "string",
                    "description": (
                        "Filter instances by status (running, stopped, etc.)"
                    ),
                },
            },
        },
    ), Capability.Read


def create_linode_instance_configs_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_configs_list tool."""
    return Tool(
        name="linode_instance_configs_list",
        description="Lists configuration profiles for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
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
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_instances_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instances_list tool request.

    Args:
        arguments: InstanceFilterArgs - environment, status (optional)
        cfg: Configuration object
    """
    status_filter = arguments.get("status", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instances = await client.list_instances()

        if status_filter:
            instances = [
                inst
                for inst in instances
                if inst.status.lower() == status_filter.lower()
            ]

        instances_data = [
            {
                "id": inst.id,
                "label": inst.label,
                "status": inst.status,
                "type": inst.type,
                "region": inst.region,
                "image": inst.image,
                "ipv4": inst.ipv4,
                "ipv6": inst.ipv6,
                "created": inst.created,
                "updated": inst.updated,
                "tags": inst.tags,
            }
            for inst in instances
        ]

        response: dict[str, Any] = {
            "count": len(instances),
            "instances": instances_data,
        }

        if status_filter:
            response["filter"] = f"status={status_filter}"

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode instances", _call)


async def handle_linode_instance_configs_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_configs_list tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_instance_configs(
            linode_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance configuration profiles", _call
    )
