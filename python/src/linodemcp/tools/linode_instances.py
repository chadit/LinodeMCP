"""Linode instances list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeGuard, cast

from mcp.types import TextContent, Tool

from linodemcp.linode import (
    LINODE_STATS_MAX_MONTH,
    LINODE_STATS_MAX_YEAR,
    LINODE_STATS_MIN_YEAR,
    instance_to_response_dict,
)
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


def _add_optional_string_field(
    body: dict[str, Any], arguments: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if value is not None and not isinstance(value, str):
        return f"{name} must be a string or null"
    body[name] = value
    return None


def _add_optional_positive_int_field(
    body: dict[str, Any], arguments: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return f"{name} must be a positive integer"
    body[name] = value
    return None


def _add_optional_object_field(
    body: dict[str, Any], arguments: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if value is not None and not isinstance(value, dict):
        return f"{name} must be an object or null"
    body[name] = value
    return None


def _validate_config_interface_purpose(arguments: dict[str, Any]) -> str | None:
    purpose = arguments.get("purpose")
    if isinstance(purpose, str) and purpose in {"public", "vlan", "vpc"}:
        return purpose
    return None


def _add_interface_purpose_fields(
    body: dict[str, Any], arguments: dict[str, Any]
) -> str | None:
    for field in ("label", "ipam_address"):
        error = _add_optional_string_field(body, arguments, field)
        if error is not None:
            return error

    purpose = body["purpose"]
    if purpose == "vlan" and not body.get("label"):
        return "label is required for vlan interfaces"

    error = _add_optional_positive_int_field(body, arguments, "subnet_id")
    if error is not None:
        return error
    if purpose == "vpc" and "subnet_id" not in body:
        return "subnet_id is required for vpc interfaces"
    return None


def _add_interface_misc_fields(
    body: dict[str, Any], arguments: dict[str, Any]
) -> str | None:
    if "primary" in arguments:
        primary = arguments["primary"]
        if not isinstance(primary, bool):
            return "primary must be a boolean"
        body["primary"] = primary

    if "ip_ranges" in arguments:
        ip_ranges = arguments["ip_ranges"]
        if ip_ranges is not None and not isinstance(ip_ranges, list):
            return "ip_ranges must be an array of strings or null"
        ip_range_values = cast("list[object] | None", ip_ranges)
        if ip_range_values is not None and any(
            not isinstance(item, str) for item in ip_range_values
        ):
            return "ip_ranges must be an array of strings or null"
        body["ip_ranges"] = ip_ranges

    for field in ("ipv4", "ipv6"):
        error = _add_optional_object_field(body, arguments, field)
        if error is not None:
            return error
    return None


def _config_interface_add_body(arguments: dict[str, Any]) -> dict[str, Any] | str:
    purpose = _validate_config_interface_purpose(arguments)
    if purpose is None:
        return "purpose must be one of: public, vlan, vpc"

    body: dict[str, Any] = {"purpose": purpose}
    for add_fields in (_add_interface_purpose_fields, _add_interface_misc_fields):
        error = add_fields(body, arguments)
        if error is not None:
            return error
    return body


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


def create_linode_instance_config_interface_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_delete tool."""
    return Tool(
        name="linode_instance_config_interface_delete",
        description=(
            "Deletes an interface from a Linode instance configuration profile. "
            "Requires confirm because the interface is removed from the profile."
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
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "The ID of the configuration profile interface (required)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to delete the configuration interface."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "config_id", "interface_id", "confirm"],
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


def create_linode_instance_interface_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_delete tool."""
    return Tool(
        name="linode_instance_interface_delete",
        description=(
            "Deletes an interface from a Linode instance. "
            "Requires confirm because the interface is removed from the Linode."
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
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the instance interface (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to delete the instance interface.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "interface_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_instance_interface_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_list tool."""
    return Tool(
        name="linode_instance_interface_list",
        description="Lists interfaces for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


def create_linode_instance_interface_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_settings_get tool."""
    return Tool(
        name="linode_instance_interface_settings_get",
        description="Lists interface settings for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


def create_linode_instance_transfer_month_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_transfer_month_get tool."""
    return Tool(
        name="linode_instance_transfer_month_get",
        description="Gets monthly network transfer stats for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "year": {
                    "type": "integer",
                    "minimum": LINODE_STATS_MIN_YEAR,
                    "maximum": LINODE_STATS_MAX_YEAR,
                    "description": (
                        "The four-digit year for the transfer stats (required)"
                    ),
                },
                "month": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": LINODE_STATS_MAX_MONTH,
                    "description": (
                        "The month for the transfer stats, 1 through 12 (required)"
                    ),
                },
            },
            "required": ["linode_id", "year", "month"],
        },
    ), Capability.Read


def create_linode_instance_interface_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_get tool."""
    return Tool(
        name="linode_instance_interface_get",
        description="Gets an interface for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance interface (required)",
                },
            },
            "required": ["linode_id", "interface_id"],
        },
    ), Capability.Read


def create_linode_instance_interface_firewall_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_firewall_list tool."""
    return Tool(
        name="linode_instance_interface_firewall_list",
        description="Lists firewalls assigned to a Linode instance interface.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance interface (required)",
                },
            },
            "required": ["linode_id", "interface_id"],
        },
    ), Capability.Read


async def handle_linode_instance_interface_firewall_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_firewall_list tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    interface_id = _positive_int_argument(arguments, "interface_id")
    if interface_id is None:
        return error_response("interface_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_instance_interface_firewalls(linode_id, interface_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interface firewalls", _call
    )


def create_linode_instance_config_interface_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_list tool."""
    return Tool(
        name="linode_instance_config_interface_list",
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


def create_linode_instance_interface_history_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_history_list tool."""
    return Tool(
        name="linode_instance_interface_history_list",
        description="Lists network interface history for a Linode instance.",
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


def create_linode_instance_config_interface_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_update tool."""
    return Tool(
        name="linode_instance_config_interface_update",
        description=(
            "Updates an interface for a Linode instance configuration profile. "
            "Requires confirm because interface networking can change."
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
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "The ID of the configuration profile interface (required)"
                    ),
                },
                "ip_ranges": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "IPv4 ranges routed to this interface.",
                },
                "ipv4": {
                    "type": "object",
                    "description": "IPv4 configuration for this interface.",
                },
                "primary": {
                    "type": "boolean",
                    "description": "Whether this is the primary interface.",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to update the configuration interface."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "config_id", "interface_id", "confirm"],
            "anyOf": [
                {"required": [field]} for field in ("ip_ranges", "ipv4", "primary")
            ],
        },
    ), Capability.Write


def create_linode_instance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_list tool."""
    return Tool(
        name="linode_instance_list",
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


def create_linode_instance_stats_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_stats_get tool."""
    return Tool(
        name="linode_instance_stats_get",
        description="Gets daily statistics for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


def create_linode_instance_nodebalancer_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_nodebalancer_list tool."""
    return Tool(
        name="linode_instance_nodebalancer_list",
        description="Lists NodeBalancers assigned to a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


def create_linode_instance_stats_month_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_stats_month_get tool."""
    return Tool(
        name="linode_instance_stats_month_get",
        description=(
            "Gets a month of statistics for a Linode instance by year and month."
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
                "year": {
                    "type": "integer",
                    "minimum": 1970,
                    "maximum": 9999,
                    "description": "The four-digit year to retrieve statistics for.",
                },
                "month": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 12,
                    "description": "The month number to retrieve statistics for.",
                },
            },
            "required": ["linode_id", "year", "month"],
        },
    ), Capability.Read


def create_linode_instance_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_transfer_get tool."""
    return Tool(
        name="linode_instance_transfer_get",
        description="Gets this month's network transfer stats for a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


def create_linode_instance_config_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_list tool."""
    return Tool(
        name="linode_instance_config_list",
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


def create_linode_instance_config_interface_reorder_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_reorder tool."""
    return Tool(
        name="linode_instance_config_interface_reorder",
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


def create_linode_instance_config_interface_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_add tool."""
    return Tool(
        name="linode_instance_config_interface_add",
        description=(
            "Adds an interface to a Linode instance configuration profile. "
            "Requires confirm because the instance network configuration changes."
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
                "purpose": {
                    "type": "string",
                    "enum": ["public", "vlan", "vpc"],
                    "description": "The interface purpose (required).",
                },
                "label": {
                    "type": "string",
                    "description": "Interface label. Required for vlan interfaces.",
                },
                "ipam_address": {
                    "type": "string",
                    "description": "Private CIDR address for vlan interfaces.",
                },
                "primary": {
                    "type": "boolean",
                    "description": "Whether this is the primary non-vlan interface.",
                },
                "subnet_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The VPC subnet ID. Required for vpc interfaces.",
                },
                "ip_ranges": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": (
                        "IPv4 CIDR VPC subnet ranges routed to this interface."
                    ),
                },
                "ipv4": {
                    "type": "object",
                    "description": "VPC IPv4 configuration for this interface.",
                },
                "ipv6": {
                    "type": "object",
                    "description": "VPC IPv6 configuration for this interface.",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to add the configuration profile interface."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "config_id", "purpose", "confirm"],
        },
    ), Capability.Write


def create_linode_instance_interface_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_add tool."""
    return Tool(
        name="linode_instance_interface_add",
        description=(
            "Adds an interface to a Linode instance using the current Linode "
            "Interfaces API. Requires confirm because instance networking changes."
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
                "interface": {
                    "type": "object",
                    "description": (
                        "Interface payload matching the Linode API public, VPC, "
                        "or VLAN interface request body."
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to add the instance interface.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "interface", "confirm"],
        },
    ), Capability.Write


def create_linode_instance_interface_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_update tool."""
    section_schema = {
        "type": "object",
        "description": "Documented Linode interface update section.",
    }
    return Tool(
        name="linode_instance_interface_update",
        description=(
            "Updates a Linode interface using explicit documented body sections. "
            "Requires confirm because instance networking changes."
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
                "interface_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode interface (required)",
                },
                "default_route": section_schema,
                "public": section_schema,
                "vlan": section_schema,
                "vpc": section_schema,
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to update the instance interface.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "interface_id", "confirm"],
            "anyOf": [
                {"required": ["default_route"]},
                {"required": ["public"]},
                {"required": ["vlan"]},
                {"required": ["vpc"]},
            ],
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


async def handle_linode_instance_interface_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_add tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    interface = arguments.get("interface")
    if not isinstance(interface, dict):
        return error_response("interface must be an object")
    interface_body = cast("dict[str, Any]", interface)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_add",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/interfaces",
            None,
            request_body=interface_body,
            side_effects=[f"An interface will be added to Linode {linode_id}."],
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        interface_result = await client.add_instance_interface(
            linode_id, interface_body
        )
        return {"interface": interface_result}

    return await execute_tool(cfg, arguments, "add Linode instance interface", _call)


def _instance_interface_update_fields(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    fields: dict[str, Any] = {}
    for name in ("default_route", "public", "vlan", "vpc"):
        if name not in arguments:
            continue
        value = arguments[name]
        if value is not None and not isinstance(value, dict):
            return None, f"{name} must be an object or null"
        fields[name] = value

    if not fields:
        return None, "at least one update field is required"
    return fields, None


async def handle_linode_instance_interface_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_update tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    interface_id = _positive_int_argument(arguments, "interface_id")
    if interface_id is None:
        return error_response("interface_id must be a positive integer")

    fields, fields_error = _instance_interface_update_fields(arguments)
    if fields is None:
        return error_response(fields_error or "at least one update field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_update",
            arguments.get("environment", ""),
            "PUT",
            f"/linode/instances/{linode_id}/interfaces/{interface_id}",
            None,
            side_effects=[
                f"Interface {interface_id} on Linode {linode_id} will be updated."
            ],
            request_body=fields,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_instance_interface(linode_id, interface_id, fields)
        if result:
            return result
        return {
            "message": (
                f"Linode instance interface {interface_id} update requested "
                f"for Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "interface_id": interface_id,
        }

    return await execute_tool(cfg, arguments, "update Linode instance interface", _call)


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


async def handle_linode_instance_config_interface_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_delete tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")
    interface_id = _positive_int_argument(arguments, "interface_id")
    if interface_id is None:
        return error_response("interface_id must be a positive integer")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interface_delete",
            arguments.get("environment", ""),
            "DELETE",
            (
                f"/linode/instances/{linode_id}/configs/{config_id}/"
                f"interfaces/{interface_id}"
            ),
            None,
            side_effects=[
                f"Interface {interface_id} will be deleted from configuration "
                f"profile {config_id} for Linode {linode_id}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_config_interface(
            linode_id, config_id, interface_id
        )
        return {
            "message": (
                f"Linode instance config {config_id} interface {interface_id} "
                f"deleted from Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "config_id": config_id,
            "interface_id": interface_id,
        }

    return await execute_tool(
        cfg, arguments, "delete Linode instance config interface", _call
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


async def handle_linode_instance_interface_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_delete tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    interface_id = _positive_int_argument(arguments, "interface_id")
    if interface_id is None:
        return error_response("interface_id must be a positive integer")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/linode/instances/{linode_id}/interfaces/{interface_id}",
            None,
            side_effects=[
                f"Interface {interface_id} will be deleted from Linode {linode_id}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_interface(linode_id, interface_id)
        return {
            "message": (
                f"Linode instance interface {interface_id} deleted from "
                f"Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "interface_id": interface_id,
        }

    return await execute_tool(cfg, arguments, "delete Linode instance interface", _call)


async def handle_linode_instance_interface_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_list tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_instance_interfaces(linode_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interfaces", _call
    )


async def handle_linode_instance_interface_settings_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_settings_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_interface_settings(linode_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interface settings", _call
    )


async def handle_linode_instance_transfer_month_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_transfer_month_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    try:
        year = _optional_int_argument(
            arguments, "year", LINODE_STATS_MIN_YEAR, LINODE_STATS_MAX_YEAR
        )
        month = _optional_int_argument(arguments, "month", 1, LINODE_STATS_MAX_MONTH)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if year is None:
        return error_response("year must be an integer")
    if month is None:
        return error_response("month must be an integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_transfer_by_year_month(linode_id, year, month)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance monthly transfer stats", _call
    )


async def handle_linode_instance_interface_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")
    interface_id = _positive_int_argument(arguments, "interface_id")
    if interface_id is None:
        return error_response("interface_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_interface(linode_id, interface_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interface", _call
    )


async def handle_linode_instance_config_interface_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_list tool request."""
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


async def handle_linode_instance_interface_history_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_history_list tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_instance_interface_history(
            linode_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg,
        arguments,
        "retrieve Linode instance network interface history",
        _call,
    )


def _instance_config_interface_update_fields(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    fields: dict[str, Any] = {}
    if "ip_ranges" in arguments:
        value = arguments["ip_ranges"]
        if not isinstance(value, list) or not all(
            isinstance(item, str) for item in cast("list[object]", value)
        ):
            return None, "ip_ranges must be an array of strings"
        fields["ip_ranges"] = value
    if "ipv4" in arguments:
        value = arguments["ipv4"]
        if not isinstance(value, dict):
            return None, "ipv4 must be an object"
        fields["ipv4"] = value
    if "primary" in arguments:
        value = arguments["primary"]
        if not isinstance(value, bool):
            return None, "primary must be a boolean"
        fields["primary"] = value

    if not fields:
        return None, "at least one update field is required"
    return fields, None


async def handle_linode_instance_config_interface_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_update tool request."""
    ids: dict[str, int] = {}
    for key in ("linode_id", "config_id", "interface_id"):
        value = _positive_int_argument(arguments, key)
        if value is None:
            return error_response(f"{key} must be a positive integer")
        ids[key] = value
    linode_id = ids["linode_id"]
    config_id = ids["config_id"]
    interface_id = ids["interface_id"]

    fields, fields_error = _instance_config_interface_update_fields(arguments)
    if fields is None:
        return error_response(fields_error or "at least one update field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interface_update",
            arguments.get("environment", ""),
            "PUT",
            (
                f"/linode/instances/{linode_id}/configs/{config_id}"
                f"/interfaces/{interface_id}"
            ),
            None,
            side_effects=[
                f"Interface {interface_id} on configuration profile {config_id} "
                f"for Linode {linode_id} will be updated."
            ],
            request_body=fields,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_instance_config_interface(
            linode_id, config_id, interface_id, fields
        )
        if result:
            return result
        return {
            "message": (
                f"Linode instance config interface {interface_id} update requested "
                f"for config {config_id} on Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "config_id": config_id,
            "interface_id": interface_id,
        }

    return await execute_tool(
        cfg, arguments, "update Linode instance config interface", _call
    )


async def handle_linode_instance_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_list tool request.

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

        instances_data = [instance_to_response_dict(inst) for inst in instances]

        response: dict[str, Any] = {"count": len(instances)}
        if status_filter:
            response["filter"] = f"status={status_filter}"
        response["instances"] = instances_data

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode instances", _call)


async def handle_linode_instance_stats_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_stats_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_stats(linode_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance statistics", _call
    )


async def handle_linode_instance_nodebalancer_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_nodebalancer_list tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_instance_nodebalancers(linode_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance NodeBalancers", _call
    )


async def handle_linode_instance_stats_month_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_stats_month_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    try:
        year = _optional_int_argument(arguments, "year", 1970, 9999)
        month = _optional_int_argument(arguments, "month", 1, 12)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    if year is None:
        return error_response("year must be an integer")
    if month is None:
        return error_response("month must be an integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_stats_by_year_month(linode_id, year, month)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance monthly statistics", _call
    )


async def handle_linode_instance_transfer_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_transfer_get tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_instance_transfer(linode_id)

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance network transfer stats", _call
    )


async def handle_linode_instance_config_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_list tool request."""
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


async def handle_linode_instance_config_interface_reorder(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_reorder tool request."""
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
            "linode_instance_config_interface_reorder",
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

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

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


async def handle_linode_instance_config_interface_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_add tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return error_response("linode_id must be a positive integer")

    config_id = _positive_int_argument(arguments, "config_id")
    if config_id is None:
        return error_response("config_id must be a positive integer")

    body = _config_interface_add_body(arguments)
    if isinstance(body, str):
        return error_response(body)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interface_add",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/configs/{config_id}/interfaces",
            None,
            side_effects=[
                f"An interface will be added to configuration profile {config_id} "
                f"on Linode {linode_id}."
            ],
            request_body=body,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.add_instance_config_interface(linode_id, config_id, body)
        if result:
            return result
        return {
            "message": (
                f"Linode instance config interface add requested for "
                f"config {config_id} on Linode {linode_id}"
            ),
            "linode_id": linode_id,
            "config_id": config_id,
        }

    return await execute_tool(
        cfg, arguments, "add Linode instance config interface", _call
    )


async def handle_linode_instance_config_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_update tool request."""
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

    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

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
