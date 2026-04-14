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


_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_INSTANCE_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the Linode instance (required)",
}

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}


def _parse_instance_id(
    arguments: dict[str, Any],
) -> int | list[TextContent]:
    """Parse and validate instance_id from arguments."""
    raw = arguments.get("instance_id", "")
    if not raw:
        return _error_response("instance_id is required")
    try:
        return int(raw)
    except (ValueError, TypeError):
        return _error_response("instance_id must be a valid integer")


def create_linode_instance_clone_tool() -> Tool:
    """Create the linode_instance_clone tool."""
    return Tool(
        name="linode_instance_clone",
        description="Clones a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "region": {
                    "type": "string",
                    "description": "Target region for clone",
                },
                "type": {
                    "type": "string",
                    "description": ("Instance type for the clone"),
                },
                "label": {
                    "type": "string",
                    "description": "Label for cloned instance",
                },
                "disks": {
                    "type": "array",
                    "description": "Disk IDs to include",
                    "items": {"type": "integer"},
                },
                "configs": {
                    "type": "array",
                    "description": "Config IDs to include",
                    "items": {"type": "integer"},
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_clone tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.clone_instance(
            iid,
            region=arguments.get("region"),
            instance_type=arguments.get("type"),
            label=arguments.get("label"),
            disks=arguments.get("disks"),
            configs=arguments.get("configs"),
        )

    return await execute_tool(cfg, arguments, "clone instance", _call)


def create_linode_instance_migrate_tool() -> Tool:
    """Create the linode_instance_migrate tool."""
    return Tool(
        name="linode_instance_migrate",
        description=("Migrates a Linode instance to a new region"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "region": {
                    "type": "string",
                    "description": ("Target region for migration"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_migrate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_migrate tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.migrate_instance(iid, region=arguments.get("region"))
        return {
            "message": (f"Migration initiated for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "migrate instance", _call)


def create_linode_instance_rebuild_tool() -> Tool:
    """Create the linode_instance_rebuild tool."""
    return Tool(
        name="linode_instance_rebuild",
        description=(
            "Rebuilds a Linode instance with a new image."
            " All data on existing disks will be destroyed."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "image": {
                    "type": "string",
                    "description": ("Image ID to rebuild with (required)"),
                },
                "root_pass": {
                    "type": "string",
                    "description": (
                        "Root password for the rebuilt instance (required)"
                    ),
                },
                "authorized_keys": {
                    "type": "array",
                    "description": "SSH public keys",
                    "items": {"type": "string"},
                },
                "authorized_users": {
                    "type": "array",
                    "description": ("Usernames with SSH keys on profile"),
                    "items": {"type": "string"},
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm rebuild."
                        " Destroys all existing disk data."
                    ),
                },
            },
            "required": [
                "instance_id",
                "image",
                "root_pass",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_rebuild(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_rebuild tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    image = arguments.get("image", "")
    if not image:
        return _error_response("image is required")

    root_pass = arguments.get("root_pass", "")
    if not root_pass:
        return _error_response("root_pass is required")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.rebuild_instance(
            iid,
            image=image,
            root_pass=root_pass,
            authorized_keys=arguments.get("authorized_keys"),
            authorized_users=arguments.get("authorized_users"),
        )

    return await execute_tool(cfg, arguments, "rebuild instance", _call)


def create_linode_instance_rescue_tool() -> Tool:
    """Create the linode_instance_rescue tool."""
    return Tool(
        name="linode_instance_rescue",
        description=("Boots a Linode instance into rescue mode"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "devices": {
                    "type": "object",
                    "description": ("Device mappings for rescue mode"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_rescue(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_rescue tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.rescue_instance(iid, devices=arguments.get("devices"))
        return {
            "message": (f"Rescue mode initiated for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "rescue instance", _call)


def create_linode_instance_password_reset_tool() -> Tool:
    """Create the linode_instance_password_reset tool."""
    return Tool(
        name="linode_instance_password_reset",
        description=("Resets the root password for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "root_pass": {
                    "type": "string",
                    "description": ("New root password (required)"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "root_pass",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_password_reset(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_password_reset request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    root_pass = arguments.get("root_pass", "")
    if not root_pass:
        return _error_response("root_pass is required")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.reset_instance_password(iid, root_pass)
        return {
            "message": (f"Password reset for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "reset instance password", _call)
