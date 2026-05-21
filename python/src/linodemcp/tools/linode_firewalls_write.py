from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeGuard, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _is_firewall_rule_list(value: Any) -> TypeGuard[list[dict[str, Any]]]:
    if not isinstance(value, list):
        return False
    rules = cast("list[object]", value)
    return all(isinstance(rule, dict) for rule in rules)


def _is_default_firewall_ids(value: Any) -> TypeGuard[dict[str, int]]:
    if not isinstance(value, dict):
        return False
    ids = cast("dict[str, object]", value)
    valid_keys = {"linode", "nodebalancer", "public_interface", "vpc_interface"}
    if not ids or set(ids) - valid_keys:
        return False
    return all(
        type(firewall_id) is int and firewall_id > 0 for firewall_id in ids.values()
    )


def create_linode_firewall_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_create tool."""
    return Tool(
        name="linode_firewall_create",
        description=(
            "Creates a new Cloud Firewall. The firewall is created with no rules."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {
                    "type": "string",
                    "description": "A label for the firewall (required)",
                },
                "inbound_policy": {
                    "type": "string",
                    "description": (
                        "Default inbound policy: 'ACCEPT' or 'DROP' (default: 'ACCEPT')"
                    ),
                },
                "outbound_policy": {
                    "type": "string",
                    "description": (
                        "Default outbound policy: 'ACCEPT' or 'DROP' "
                        "(default: 'ACCEPT')"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm this operation.",
                },
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


async def handle_linode_firewall_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_create tool request."""
    if not arguments.get("confirm"):
        return error_response(
            "This creates a Cloud Firewall. Set confirm=true to proceed."
        )

    label = arguments.get("label", "")
    inbound_policy = arguments.get("inbound_policy", "ACCEPT")
    outbound_policy = arguments.get("outbound_policy", "ACCEPT")

    if not label:
        return error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.create_firewall(
            label=label,
            inbound_policy=inbound_policy,
            outbound_policy=outbound_policy,
        )
        return {
            "message": (
                f"Firewall '{firewall.label}' (ID: {firewall.id}) created successfully"
            ),
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "created": firewall.created,
            },
        }

    return await execute_tool(cfg, arguments, "create firewall", _call)


def create_linode_firewall_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_update tool."""
    return Tool(
        name="linode_firewall_update",
        description="Updates an existing Cloud Firewall.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New label for the firewall (optional)",
                },
                "status": {
                    "type": "string",
                    "description": "New status: 'enabled' or 'disabled' (optional)",
                },
                "inbound_policy": {
                    "type": "string",
                    "description": "New inbound policy: 'ACCEPT' or 'DROP' (optional)",
                },
                "outbound_policy": {
                    "type": "string",
                    "description": "New outbound policy: 'ACCEPT' or 'DROP' (optional)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
            },
            "required": ["firewall_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_update tool request."""
    firewall_id = arguments.get("firewall_id", 0)

    if not firewall_id:
        return error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.update_firewall(
            firewall_id=int(firewall_id),
            label=arguments.get("label"),
            status=arguments.get("status"),
            inbound_policy=arguments.get("inbound_policy"),
            outbound_policy=arguments.get("outbound_policy"),
        )
        return {
            "message": f"Firewall {firewall_id} updated successfully",
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "updated": firewall.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update firewall", _call)


def create_linode_firewall_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_delete tool."""
    return Tool(
        name="linode_firewall_delete",
        description=(
            "Deletes a Cloud Firewall. WARNING: This removes all rules "
            "and unassigns all devices."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["firewall_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_firewall_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_delete tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not firewall_id:
        return error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall(int(firewall_id))
        return {
            "message": f"Firewall {firewall_id} deleted successfully",
            "firewall_id": firewall_id,
        }

    return await execute_tool(cfg, arguments, "delete firewall", _call)


def create_linode_firewall_rules_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rules_update tool."""
    return Tool(
        name="linode_firewall_rules_update",
        description=(
            "Replaces the inbound and outbound rules for a Cloud Firewall. "
            "WARNING: This overwrites all existing rules."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Firewall ID (required)",
                },
                "inbound": {
                    "type": "array",
                    "description": "List of inbound firewall rules to set (required)",
                    "items": {"type": "object"},
                },
                "outbound": {
                    "type": "array",
                    "description": "List of outbound firewall rules to set (required)",
                    "items": {"type": "object"},
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm.",
                },
            },
            "required": ["firewall_id", "inbound", "outbound", "confirm"],
        },
    ), Capability.Write


def _firewall_rules_update_validation_error(arguments: dict[str, Any]) -> str | None:
    firewall_id = arguments.get("firewall_id", 0)
    error: str | None = None

    if arguments.get("confirm") is not True:
        error = "This replaces all firewall rules. Set confirm=true to proceed."
    elif not firewall_id:
        error = "firewall_id is required"
    elif not isinstance(firewall_id, int) or isinstance(firewall_id, bool):
        error = "firewall_id must be an integer"
    elif firewall_id <= 0:
        error = "firewall_id must be a positive integer"
    else:
        for field in ("inbound", "outbound"):
            rules_raw: Any = arguments.get(field)
            if rules_raw is None:
                error = f"{field} is required"
                break
            if not _is_firewall_rule_list(rules_raw):
                error = f"{field} must be a list of rule objects"
                break

    return error


async def handle_linode_firewall_rules_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rules_update tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    validation_error = _firewall_rules_update_validation_error(arguments)
    if validation_error is not None:
        return error_response(validation_error)

    inbound = cast("list[dict[str, Any]]", arguments.get("inbound"))
    outbound = cast("list[dict[str, Any]]", arguments.get("outbound"))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_firewall_rules(
            firewall_id=firewall_id,
            inbound=inbound,
            outbound=outbound,
        )
        return {
            "message": f"Firewall {firewall_id} rules updated successfully",
            "firewall_id": firewall_id,
            "inbound_count": len(result.get("inbound", [])),
            "outbound_count": len(result.get("outbound", [])),
        }

    return await execute_tool(cfg, arguments, "update firewall rules", _call)


def create_linode_firewall_settings_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_settings_update tool."""
    return Tool(
        name="linode_firewall_settings_update",
        description=(
            "Updates the account default firewalls for Linodes, NodeBalancers, "
            "public interfaces, and VPC interfaces."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "default_firewall_ids": {
                    "type": "object",
                    "description": (
                        "Default firewall IDs keyed by linode, nodebalancer, "
                        "public_interface, or vpc_interface."
                    ),
                    "additionalProperties": False,
                    "minProperties": 1,
                    "properties": {
                        "linode": {"type": "integer", "minimum": 1},
                        "nodebalancer": {"type": "integer", "minimum": 1},
                        "public_interface": {"type": "integer", "minimum": 1},
                        "vpc_interface": {"type": "integer", "minimum": 1},
                    },
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to update default firewalls.",
                },
            },
            "required": ["default_firewall_ids", "confirm"],
        },
    ), Capability.Write


async def handle_linode_firewall_settings_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_settings_update tool request."""
    if arguments.get("confirm") is not True:
        return error_response("confirm must be true")

    default_firewall_ids_raw = arguments.get("default_firewall_ids")
    if not _is_default_firewall_ids(default_firewall_ids_raw):
        return error_response(
            "default_firewall_ids must be a non-empty object of positive "
            "integer firewall IDs keyed by linode, nodebalancer, "
            "public_interface, or vpc_interface"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_firewall_settings(default_firewall_ids_raw)
        updated = result.get("default_firewall_ids", default_firewall_ids_raw)
        return {
            "message": "Default firewall settings updated successfully",
            "default_firewall_ids": updated,
        }

    return await execute_tool(cfg, arguments, "update default firewalls", _call)


def create_linode_firewall_device_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_create tool."""
    return Tool(
        name="linode_firewall_device_create",
        description=(
            "Creates a new device for a Cloud Firewall. "
            "WARNING: This operation requires confirmation."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the firewall to attach the device to (required)"
                    ),
                },
                "id": {
                    "type": "integer",
                    "description": (
                        "The ID of the entity to attach as a device (required)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": "The type of entity to attach (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm this operation.",
                },
            },
            "required": ["firewall_id", "id", "type", "confirm"],
        },
    ), Capability.Write


async def handle_linode_firewall_device_create(  # noqa: PLR0911
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_device_create tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    device_id = arguments.get("id", 0)
    device_type = arguments.get("type", "")
    confirm = arguments.get("confirm", False)

    # Validation
    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This operation requires confirmation. "
                    "Set confirm=true to proceed."
                ),
            )
        ]

    if not firewall_id:
        return error_response("firewall_id is required")
    if not isinstance(firewall_id, int) or isinstance(firewall_id, bool):
        return error_response("firewall_id must be a valid integer")
    if firewall_id <= 0:
        return error_response("firewall_id must be a positive integer")

    if not device_id:
        return error_response("id is required")
    if not isinstance(device_id, int) or isinstance(device_id, bool):
        return error_response("id must be a valid integer")
    if device_id <= 0:
        return error_response("id must be a positive integer")

    if "type" not in arguments:
        return error_response("type is required")
    if not isinstance(device_type, str):
        return error_response("type must be a string")
    if not device_type.strip():
        return error_response("type must be a non-empty string")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        device = await client.create_firewall_device(
            firewall_id=int(firewall_id),
            device_id=int(device_id),
            device_type=str(device_type),
        )
        return {"message": "Firewall device created successfully", "device": device}

    return await execute_tool(cfg, arguments, "create firewall device", _call)
