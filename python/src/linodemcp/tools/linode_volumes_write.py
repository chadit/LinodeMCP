from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import _error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_volume_create_tool() -> Tool:
    """Create the linode_volume_create tool."""
    return Tool(
        name="linode_volume_create",
        description=(
            "Creates a new block storage volume. WARNING: Billing starts immediately."
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
                "label": {
                    "type": "string",
                    "description": "Label for the volume (required)",
                },
                "region": {
                    "type": "string",
                    "description": "Region for the volume (required if not attaching)",
                },
                "size": {
                    "type": "integer",
                    "description": "Size in GB (default: 20, min: 10, max: 10240)",
                },
                "linode_id": {
                    "type": "integer",
                    "description": "Linode ID to attach to (optional)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["label", "confirm"],
        },
    )


async def handle_linode_volume_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_create tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    label = arguments.get("label", "")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.create_volume(
            label=label,
            region=arguments.get("region"),
            linode_id=arguments.get("linode_id"),
            size=arguments.get("size", 20),
        )
        return {
            "message": (
                f"Volume '{volume.label}' (ID: {volume.id}) "
                f"created successfully in {volume.region}"
            ),
            "volume": {
                "id": volume.id,
                "label": volume.label,
                "size": volume.size,
                "region": volume.region,
                "status": volume.status,
                "filesystem_path": volume.filesystem_path,
            },
        }

    return await execute_tool(cfg, arguments, "create volume", _call)


def create_linode_volume_attach_tool() -> Tool:
    """Create the linode_volume_attach tool."""
    return Tool(
        name="linode_volume_attach",
        description="Attaches a block storage volume to a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to attach (required)",
                },
                "linode_id": {
                    "type": "integer",
                    "description": "The ID of the Linode to attach to (required)",
                },
                "config_id": {
                    "type": "integer",
                    "description": "Config profile ID (optional)",
                },
                "persist_across_boots": {
                    "type": "boolean",
                    "description": "Keep attached across reboots (default: false)",
                },
            },
            "required": ["volume_id", "linode_id"],
        },
    )


async def handle_linode_volume_attach(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_attach tool request."""
    volume_id = arguments.get("volume_id", 0)
    linode_id = arguments.get("linode_id", 0)

    if not volume_id:
        return _error_response("volume_id is required")
    if not linode_id:
        return _error_response("linode_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.attach_volume(
            volume_id=int(volume_id),
            linode_id=int(linode_id),
            config_id=arguments.get("config_id"),
            persist_across_boots=arguments.get("persist_across_boots", False),
        )
        return {
            "message": (
                f"Volume {volume_id} attached to Linode {linode_id} successfully"
            ),
            "volume": {
                "id": volume.id,
                "label": volume.label,
                "linode_id": volume.linode_id,
                "filesystem_path": volume.filesystem_path,
            },
        }

    return await execute_tool(cfg, arguments, "attach volume", _call)


def create_linode_volume_detach_tool() -> Tool:
    """Create the linode_volume_detach tool."""
    return Tool(
        name="linode_volume_detach",
        description="Detaches a block storage volume from a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to detach (required)",
                },
            },
            "required": ["volume_id"],
        },
    )


async def handle_linode_volume_detach(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_detach tool request."""
    volume_id = arguments.get("volume_id", 0)

    if not volume_id:
        return _error_response("volume_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.detach_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} detached successfully",
            "volume_id": volume_id,
        }

    return await execute_tool(cfg, arguments, "detach volume", _call)


def create_linode_volume_resize_tool() -> Tool:
    """Create the linode_volume_resize tool."""
    return Tool(
        name="linode_volume_resize",
        description=(
            "Resizes a block storage volume. WARNING: Volumes can only be resized "
            "up, not down. This increases billing."
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
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to resize (required)",
                },
                "size": {
                    "type": "integer",
                    "description": "New size in GB (must be larger than current)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm resize. This increases billing."
                    ),
                },
            },
            "required": ["volume_id", "size", "confirm"],
        },
    )


async def handle_linode_volume_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_resize tool request."""
    volume_id = arguments.get("volume_id", 0)
    size = arguments.get("size", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This increases billing. Set confirm=true to proceed.",
            )
        ]

    if not volume_id:
        return _error_response("volume_id is required")
    if not size:
        return _error_response("size is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.resize_volume(int(volume_id), int(size))
        return {
            "message": f"Volume {volume_id} resized to {size}GB successfully",
            "volume": {
                "id": volume.id,
                "label": volume.label,
                "size": volume.size,
            },
        }

    return await execute_tool(cfg, arguments, "resize volume", _call)


def create_linode_volume_delete_tool() -> Tool:
    """Create the linode_volume_delete tool."""
    return Tool(
        name="linode_volume_delete",
        description=(
            "Deletes a block storage volume. WARNING: This is destructive "
            "and all data will be lost. Volume must be detached first."
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
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["volume_id", "confirm"],
        },
    )


async def handle_linode_volume_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_delete tool request."""
    volume_id = arguments.get("volume_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not volume_id:
        return _error_response("volume_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} deleted successfully",
            "volume_id": volume_id,
        }

    return await execute_tool(cfg, arguments, "delete volume", _call)
