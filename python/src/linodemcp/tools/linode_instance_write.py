from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


def create_linode_instance_boot_tool() -> Tool:
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
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_boot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_boot tool request."""
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.boot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} boot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "boot instance", _call)


def create_linode_instance_reboot_tool() -> Tool:
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
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_reboot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_reboot tool request."""
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.reboot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} reboot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "reboot instance", _call)


def create_linode_instance_shutdown_tool() -> Tool:
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
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_shutdown(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_shutdown tool request."""
    instance_id = arguments.get("instance_id", 0)

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.shutdown_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} shutdown initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "shutdown instance", _call)


def create_linode_instance_create_tool() -> Tool:
    """Create the linode_instance_create tool."""
    return Tool(
        name="linode_instance_create",
        description=(
            "Creates a new Linode instance. WARNING: Billing starts immediately. "
            "Use linode_regions_list and linode_types_list to find valid values."
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
                "private_ip": {
                    "type": "boolean",
                    "description": "Add private IP (default: false)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["region", "type", "confirm"],
        },
    )


async def handle_linode_instance_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_create tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    region = arguments.get("region", "")
    instance_type = arguments.get("type", "")

    if not region:
        return _error_response("region is required")
    if not instance_type:
        return _error_response("type is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.create_instance(
            region=region,
            instance_type=instance_type,
            image=arguments.get("image"),
            label=arguments.get("label"),
            root_pass=arguments.get("root_pass"),
            authorized_keys=arguments.get("authorized_keys"),
            booted=arguments.get("booted", True),
            backups_enabled=arguments.get("backups_enabled", False),
            private_ip=arguments.get("private_ip", False),
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


def create_linode_instance_delete_tool() -> Tool:
    """Create the linode_instance_delete tool."""
    return Tool(
        name="linode_instance_delete",
        description=(
            "Deletes a Linode instance. WARNING: This is destructive and cannot "
            "be undone. All data will be lost."
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
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_delete tool request."""
    instance_id = arguments.get("instance_id", 0)
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


def create_linode_instance_resize_tool() -> Tool:
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
                    "description": "Must be true to confirm resize.",
                },
            },
            "required": ["instance_id", "type", "confirm"],
        },
    )


async def handle_linode_instance_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_resize tool request."""
    instance_id = arguments.get("instance_id", 0)
    instance_type = arguments.get("type", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This may cause downtime. Set confirm=true to proceed.",
            )
        ]

    if not instance_id:
        return _error_response("instance_id is required")
    if not instance_type:
        return _error_response("type is required")

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
