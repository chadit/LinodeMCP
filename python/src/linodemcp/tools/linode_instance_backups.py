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

_BACKUP_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the backup (required)",
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


def _parse_instance_and_backup_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse instance_id and backup_id from arguments."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid
    raw = arguments.get("backup_id", "")
    if not raw:
        return _error_response("backup_id is required")
    try:
        backup_id = int(raw)
    except (ValueError, TypeError):
        return _error_response("backup_id must be a valid integer")
    return iid, backup_id


def create_linode_instance_backups_list_tool() -> Tool:
    """Create the linode_instance_backups_list tool."""
    return Tool(
        name="linode_instance_backups_list",
        description=("Lists backups for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_backups_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backups_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.list_instance_backups(iid)

    return await execute_tool(cfg, arguments, "list instance backups", _call)


def create_linode_instance_backup_get_tool() -> Tool:
    """Create the linode_instance_backup_get tool."""
    return Tool(
        name="linode_instance_backup_get",
        description=("Gets details of a specific backup for an instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "backup_id": _BACKUP_ID_PROP,
            },
            "required": ["instance_id", "backup_id"],
        },
    )


async def handle_linode_instance_backup_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_get tool request."""
    ids = _parse_instance_and_backup_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, backup_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.get_instance_backup(instance_id, backup_id)

    return await execute_tool(cfg, arguments, "get instance backup", _call)


def create_linode_instance_backup_create_tool() -> Tool:
    """Create the linode_instance_backup_create tool."""
    return Tool(
        name="linode_instance_backup_create",
        description=("Creates a snapshot backup of a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the snapshot",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_backup_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.create_instance_backup(iid, label=arguments.get("label"))

    return await execute_tool(cfg, arguments, "create instance backup", _call)


def create_linode_instance_backup_restore_tool() -> Tool:
    """Create the linode_instance_backup_restore tool."""
    return Tool(
        name="linode_instance_backup_restore",
        description="Restores a backup to a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "backup_id": _BACKUP_ID_PROP,
                "linode_id": {
                    "type": "integer",
                    "description": ("Target instance ID for restore"),
                },
                "overwrite": {
                    "type": "boolean",
                    "description": ("Overwrite existing data on target"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "backup_id",
                "linode_id",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_backup_restore(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_restore request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    ids = _parse_instance_and_backup_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, backup_id = ids

    linode_id = arguments.get("linode_id")
    if not linode_id:
        return _error_response("linode_id is required")
    overwrite = arguments.get("overwrite", False)

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.restore_instance_backup(
            instance_id,
            backup_id,
            int(linode_id),
            overwrite=overwrite,
        )
        return {
            "message": (f"Backup {backup_id} restored to instance {linode_id}"),
            "instance_id": instance_id,
            "backup_id": backup_id,
        }

    return await execute_tool(cfg, arguments, "restore instance backup", _call)


def create_linode_instance_backups_enable_tool() -> Tool:
    """Create the linode_instance_backups_enable tool."""
    return Tool(
        name="linode_instance_backups_enable",
        description=("Enables backups for a Linode instance (billing charges apply)"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_backups_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backups_enable request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.enable_instance_backups(iid)
        return {
            "message": (f"Backups enabled for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "enable instance backups", _call)


def create_linode_instance_backups_cancel_tool() -> Tool:
    """Create the linode_instance_backups_cancel tool."""
    return Tool(
        name="linode_instance_backups_cancel",
        description=(
            "Cancels backups for a Linode instance."
            " All existing backups will be deleted."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm. Existing backups will be deleted."
                    ),
                },
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_backups_cancel(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backups_cancel request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.cancel_instance_backups(iid)
        return {
            "message": (f"Backups cancelled for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "cancel instance backups", _call)
