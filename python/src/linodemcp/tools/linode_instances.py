"""Linode instances list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeGuard, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def _positive_int_argument(arguments: dict[str, Any], name: str) -> int | None:
    value = arguments.get(name)
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return None
    return value


def _is_positive_int_list(value: object) -> TypeGuard[list[int]]:
    """Return whether value is a non-empty list of positive integers."""
    if not isinstance(value, list) or not value:
        return False
    items = cast("list[object]", value)
    return all(
        isinstance(item, int) and not isinstance(item, bool) and item >= 1
        for item in items
    )


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


def create_linode_instance_config_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_get tool."""
    return Tool(
        name="linode_instance_config_get",
        description="Gets a configuration profile for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the configuration profile (required)",
                },
            },
            "required": ["linode_id", "config_id"],
        },
    ), Capability.Read


def create_linode_instance_config_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_delete tool."""
    return Tool(
        name="linode_instance_config_delete",
        description="Deletes a configuration profile from a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the configuration profile (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "config_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_instance_config_interface_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_get tool."""
    return Tool(
        name="linode_instance_config_interface_get",
        description="Gets an interface for a Linode instance configuration profile.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the configuration profile (required)",
                },
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "The ID of the configuration profile interface (required)"
                    ),
                },
            },
            "required": ["linode_id", "config_id", "interface_id"],
        },
    ), Capability.Read


def create_linode_instance_config_interfaces_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interfaces_list tool."""
    return Tool(
        name="linode_instance_config_interfaces_list",
        description="Lists interfaces for a Linode instance configuration profile.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the configuration profile (required)",
                },
            },
            "required": ["linode_id", "config_id"],
        },
    ), Capability.Read


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


def create_linode_instance_config_interfaces_order_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interfaces_order tool."""
    return Tool(
        name="linode_instance_config_interfaces_order",
        description=(
            "Reorders interfaces on a Linode instance configuration profile. "
            "Requires confirm because the active interface order can change."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the configuration profile (required)",
                },
                "ids": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "minItems": 1,
                    "description": "Interface IDs in the desired order.",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to reorder configuration interfaces.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "config_id", "ids", "confirm"],
        },
    ), Capability.Write


def create_linode_instance_config_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_update tool."""
    return Tool(
        name="linode_instance_config_update",
        description=(
            "Updates a configuration profile for a Linode instance. "
            "Requires confirm because the instance boot profile can change."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the configuration profile (required)",
                },
                "comments": {"type": "string", "description": "Config comments."},
                "devices": {"type": "object", "description": "Block device mapping."},
                "helpers": {"type": "object", "description": "Helper settings."},
                "interfaces": {"type": "array", "description": "Network interfaces."},
                "kernel": {"type": "string", "description": "Kernel ID."},
                "label": {"type": "string", "description": "Config label."},
                "memory_limit": {
                    "type": "integer",
                    "description": "Memory limit in MB.",
                },
                "root_device": {"type": "string", "description": "Root device path."},
                "run_level": {"type": "string", "description": "Boot run level."},
                "virt_mode": {"type": "string", "description": "Virtualization mode."},
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to update the configuration profile.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "config_id", "confirm"],
            "anyOf": [
                {"required": [field]}
                for field in (
                    "comments",
                    "devices",
                    "helpers",
                    "interfaces",
                    "kernel",
                    "label",
                    "memory_limit",
                    "root_device",
                    "run_level",
                    "virt_mode",
                )
            ],
        },
    ), Capability.Write


async def handle_linode_instance_config_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_delete tool request."""
    confirm = arguments.get("confirm")
    if not isinstance(confirm, bool) or not confirm:
        return error_response("This is destructive. Set confirm=true to proceed.")

    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> dict[str, Any]:
            return await client.get_instance_config(linode_id, config_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_config_delete",
            "DELETE",
            f"/linode/instances/{linode_id}/configs/{config_id}",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_config(linode_id, config_id)
        return {
            "message": (
                f"Configuration profile {config_id} deleted from Linode instance "
                f"{linode_id}"
            ),
            "linode_id": linode_id,
            "config_id": config_id,
        }

    return await execute_tool(
        cfg, arguments, "delete Linode instance configuration profile", _call
    )


async def handle_linode_instance_config_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_config(linode_id, config_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance configuration profile", _call
    )


async def handle_linode_instance_config_interface_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")
    interface_id = _positive_int_argument(arguments, "interface_id")
    if interface_id is None:
        return error_response("interface_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_config_interface(
            linode_id, config_id, interface_id
        )

    return await execute_tool(
        cfg,
        arguments,
        "retrieve Linode instance configuration profile interface",
        _call,
    )


async def handle_linode_instance_config_interfaces_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interfaces_list tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_instance_config_interfaces(linode_id, config_id)

    return await execute_tool(
        cfg,
        arguments,
        "retrieve Linode instance configuration profile interfaces",
        _call,
    )


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


async def handle_linode_instance_config_interfaces_order(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interfaces_order tool request."""
    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    ids = arguments.get("ids")
    if not _is_positive_int_list(ids):
        return error_response("ids must be a non-empty list of positive integers")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interfaces_order",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/configs/{config_id}/interfaces/order",
            None,
            side_effects=[
                f"Interfaces on configuration profile {config_id} for Linode "
                f"{linode_id} will be reordered."
            ],
            request_body={"ids": ids},
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.reorder_instance_config_interfaces(
            linode_id, config_id, ids
        )
        if result:
            return result
        return {
            "message": (
                f"Linode instance config {config_id} interface reorder requested "
                f"for Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "config_id": config_id,
            "ids": ids,
        }

    return await execute_tool(
        cfg, arguments, "reorder Linode instance config interfaces", _call
    )


async def handle_linode_instance_config_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_update tool request."""
    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    fields: dict[str, Any] = {}
    for key in (
        "comments",
        "devices",
        "helpers",
        "interfaces",
        "kernel",
        "label",
        "memory_limit",
        "root_device",
        "run_level",
        "virt_mode",
    ):
        value = arguments.get(key)
        if value is not None:
            fields[key] = value

    if not fields:
        return error_response("at least one update field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_update",
            arguments.get("environment", ""),
            "PUT",
            f"/linode/instances/{linode_id}/configs/{config_id}",
            None,
            side_effects=[
                f"Configuration profile {config_id} on Linode {linode_id} "
                "will be updated."
            ],
            request_body=fields,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_instance_config(linode_id, config_id, fields)
        if result:
            return result
        return {
            "message": (
                f"Linode instance config {config_id} update requested "
                f"for Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "config_id": config_id,
        }

    return await execute_tool(cfg, arguments, "update Linode instance config", _call)
