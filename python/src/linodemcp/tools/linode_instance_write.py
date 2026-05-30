from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    build_dry_run_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


def create_linode_instance_boot_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_boot tool."""
    return Tool(
        name="linode_instance_boot",
        description="Boots a Linode instance that is currently offline.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to boot (required)",
                },
                "config_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the config profile to boot with (optional)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_boot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_boot tool request."""
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_boot",
            "POST",
            f"/linode/instances/{int(instance_id)}/boot",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.boot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} boot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "boot instance", _call)


def create_linode_instance_reboot_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_reboot tool."""
    return Tool(
        name="linode_instance_reboot",
        description="Reboots a running Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to reboot (required)",
                },
                "config_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the config profile to reboot with (optional)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_reboot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_reboot tool request."""
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_reboot",
            "POST",
            f"/linode/instances/{int(instance_id)}/reboot",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.reboot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} reboot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "reboot instance", _call)


def create_linode_instance_shutdown_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_shutdown tool."""
    return Tool(
        name="linode_instance_shutdown",
        description="Shuts down a running Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to shutdown (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_shutdown(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_shutdown tool request."""
    instance_id = arguments.get("instance_id", 0)

    if not instance_id:
        return _error_response("instance_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_shutdown",
            "POST",
            f"/linode/instances/{int(instance_id)}/shutdown",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.shutdown_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} shutdown initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "shutdown instance", _call)


def create_linode_instance_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_create tool."""
    return Tool(
        name="linode_instance_create",
        description=(
            "Creates a new Linode instance under the current Linode Interfaces "
            "generation. WARNING: Billing starts immediately. Requires "
            "firewall_id (get one from linode_firewalls_list or create with "
            "linode_firewall_create). Note: VPC attachment via the current "
            "interface model is not yet supported by this tool; use "
            "linode_vpc_* tools after create."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": (
                        "Region where the instance will be created (required)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": "Instance type/plan (required)",
                },
                "image": {
                    "type": "string",
                    "description": "Image ID to deploy (optional)",
                },
                "label": {
                    "type": "string",
                    "description": "Label for the instance (optional)",
                },
                "root_pass": {
                    "type": "string",
                    "description": "Root password (required if image is provided)",
                },
                "authorized_keys": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "SSH public keys to add (optional)",
                },
                "booted": {
                    "type": "boolean",
                    "description": "Whether to boot the instance (default: true)",
                },
                "backups_enabled": {
                    "type": "boolean",
                    "description": "Enable backups (default: false)",
                },
                "firewall_id": {
                    "type": "integer",
                    "description": (
                        "Cloud Firewall ID to attach to the public interface. "
                        "Required under the current Linode Interfaces generation."
                    ),
                },
                "route_ipv4": {
                    "type": "boolean",
                    "description": (
                        "Whether the public interface owns the IPv4 default "
                        "route (optional, default: true)"
                    ),
                },
                "route_ipv6": {
                    "type": "boolean",
                    "description": (
                        "Whether the public interface owns the IPv6 default "
                        "route (optional, default: true)"
                    ),
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
            "required": ["region", "type", "firewall_id", "confirm"],
        },
    ), Capability.Write


def _instance_create_error(
    region: str, instance_type: str, firewall_id: Any
) -> str | None:
    """Validate instance create args; return an error message or None."""
    if not region:
        return "region is required"
    if not instance_type:
        return "type is required"
    if not firewall_id or firewall_id <= 0:
        return (
            "firewall_id is required for instance creation. Get a firewall ID "
            "from linode_firewalls_list, or create one with linode_firewall_create."
        )
    return None


async def handle_linode_instance_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_create tool request."""
    region = arguments.get("region", "")
    instance_type = arguments.get("type", "")
    firewall_id = arguments.get("firewall_id", 0)

    if is_dry_run(arguments):
        fields_error = _instance_create_error(region, instance_type, firewall_id)
        if fields_error is not None:
            return _error_response(fields_error)
        return build_dry_run_response(
            "linode_instance_create",
            arguments.get("environment", ""),
            "POST",
            "/linode/instances",
            None,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    fields_error = _instance_create_error(region, instance_type, firewall_id)
    if fields_error is not None:
        return _error_response(fields_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.create_instance(
            region=region,
            instance_type=instance_type,
            firewall_id=firewall_id,
            image=arguments.get("image"),
            label=arguments.get("label"),
            root_pass=arguments.get("root_pass"),
            authorized_keys=arguments.get("authorized_keys"),
            booted=arguments.get("booted", True),
            backups_enabled=arguments.get("backups_enabled", False),
            route_ipv4=arguments.get("route_ipv4", True),
            route_ipv6=arguments.get("route_ipv6", True),
        )
        return {
            "message": (
                f"Instance '{instance.label}' (ID: {instance.id}) "
                f"created successfully in {instance.region}"
            ),
            "instance": {
                "id": instance.id,
                "label": instance.label,
                "status": instance.status,
                "type": instance.type,
                "region": instance.region,
                "ipv4": instance.ipv4,
                "ipv6": instance.ipv6,
            },
        }

    return await execute_tool(cfg, arguments, "create instance", _call)


def create_linode_instance_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_update tool."""
    return Tool(
        name="linode_instance_update",
        description="Updates editable fields on a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New Linode label (optional)",
                },
                "group": {
                    "type": "string",
                    "description": "Deprecated group label (optional)",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Tags to assign to the Linode (optional)",
                },
                "alerts": {
                    "type": "object",
                    "description": "Alert threshold settings (optional)",
                },
                "maintenance_policy": {
                    "type": "string",
                    "description": (
                        "Maintenance policy, such as linode/migrate (optional)"
                    ),
                },
                "watchdog_enabled": {
                    "type": "boolean",
                    "description": (
                        "Whether Lassie shutdown watchdog is enabled (optional)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_update tool request."""
    instance_id = arguments.get("instance_id", 0)

    if is_dry_run(arguments):
        if not instance_id:
            return _error_response("instance_id is required")
        iid = int(instance_id)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_update",
            "PUT",
            f"/linode/instances/{iid}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)

    if not confirm:
        return _error_response("Set confirm=true to update the instance.")

    if not instance_id:
        return _error_response("instance_id is required")

    update_fields = {
        key: arguments[key]
        for key in (
            "label",
            "group",
            "tags",
            "alerts",
            "maintenance_policy",
            "watchdog_enabled",
        )
        if key in arguments
    }

    if not update_fields:
        return _error_response(
            "at least one update field is required: label, group, tags, alerts, "
            "maintenance_policy, or watchdog_enabled"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.update_instance(int(instance_id), **update_fields)
        return {
            "message": f"Instance {instance.id} updated successfully",
            "instance": {
                "id": instance.id,
                "label": instance.label,
                "status": instance.status,
                "type": instance.type,
                "region": instance.region,
                "tags": instance.tags,
                "watchdog_enabled": instance.watchdog_enabled,
            },
        }

    return await execute_tool(cfg, arguments, "update instance", _call)


def create_linode_instance_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_delete tool."""
    return Tool(
        name="linode_instance_delete",
        description=(
            "Deletes a Linode instance. WARNING: This is destructive and cannot "
            "be undone. All data will be lost. Pass dry_run=true to preview "
            "without deleting."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_instance_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_delete tool request."""
    instance_id = arguments.get("instance_id", 0)

    if is_dry_run(arguments):
        if not instance_id:
            return _error_response("instance_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_delete",
            "DELETE",
            f"/linode/instances/{int(instance_id)}",
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

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} deleted successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "delete instance", _call)


def create_linode_instance_resize_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_resize tool."""
    return Tool(
        name="linode_instance_resize",
        description=(
            "Resizes a Linode instance to a different plan. "
            "WARNING: This may cause downtime and billing changes."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to resize (required)",
                },
                "type": {
                    "type": "string",
                    "description": "The new instance type/plan (required)",
                },
                "allow_auto_disk_resize": {
                    "type": "boolean",
                    "description": (
                        "Auto-resize disks to fit new plan (default: true)"
                    ),
                },
                "migration_type": {
                    "type": "string",
                    "description": "Migration type: 'warm' or 'cold' (default: 'warm')",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm resize. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "type", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_resize tool request."""
    instance_id = arguments.get("instance_id", 0)
    instance_type = arguments.get("type", "")

    if not instance_id:
        return _error_response("instance_id is required")
    if not instance_type:
        return _error_response("type is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_resize",
            "POST",
            f"/linode/instances/{int(instance_id)}/resize",
            _fetch,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This may cause downtime. Set confirm=true to proceed.",
            )
        ]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.resize_instance(
            instance_id=int(instance_id),
            instance_type=instance_type,
            allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", True),
            migration_type=arguments.get("migration_type", "warm"),
        )
        return {
            "message": (f"Instance {instance_id} resize to {instance_type} initiated"),
            "instance_id": instance_id,
            "new_type": instance_type,
        }

    return await execute_tool(cfg, arguments, "resize instance", _call)
