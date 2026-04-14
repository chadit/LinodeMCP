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

_DISK_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the disk (required)",
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


def _parse_instance_and_disk_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse instance_id and disk_id from arguments."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid
    raw = arguments.get("disk_id", "")
    if not raw:
        return _error_response("disk_id is required")
    try:
        disk_id = int(raw)
    except (ValueError, TypeError):
        return _error_response("disk_id must be a valid integer")
    return iid, disk_id


def create_linode_instance_disks_list_tool() -> Tool:
    """Create the linode_instance_disks_list tool."""
    return Tool(
        name="linode_instance_disks_list",
        description="Lists disks for a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_disks_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disks_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        disks = await client.list_instance_disks(iid)
        return {"count": len(disks), "disks": disks}

    return await execute_tool(cfg, arguments, "list instance disks", _call)


def create_linode_instance_disk_get_tool() -> Tool:
    """Create the linode_instance_disk_get tool."""
    return Tool(
        name="linode_instance_disk_get",
        description=("Gets details of a specific disk on an instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
            },
            "required": ["instance_id", "disk_id"],
        },
    )


async def handle_linode_instance_disk_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_get tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.get_instance_disk(instance_id, disk_id)

    return await execute_tool(cfg, arguments, "get instance disk", _call)


def create_linode_instance_disk_create_tool() -> Tool:
    """Create the linode_instance_disk_create tool."""
    return Tool(
        name="linode_instance_disk_create",
        description="Creates a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the disk",
                },
                "size": {
                    "type": "integer",
                    "description": "Disk size in MB",
                },
                "filesystem": {
                    "type": "string",
                    "description": ("Filesystem type (ext4, swap, raw, etc.)"),
                },
                "image": {
                    "type": "string",
                    "description": "Image to deploy",
                },
                "root_pass": {
                    "type": "string",
                    "description": ("Root password (required with image)"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "label",
                "size",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_disk_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    label = arguments.get("label", "")
    if not label:
        return _error_response("label is required")

    size = arguments.get("size")
    if not size:
        return _error_response("size is required")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.create_instance_disk(
            iid,
            label=label,
            size=int(size),
            filesystem=arguments.get("filesystem"),
            image=arguments.get("image"),
            root_pass=arguments.get("root_pass"),
        )

    return await execute_tool(cfg, arguments, "create instance disk", _call)


def create_linode_instance_disk_update_tool() -> Tool:
    """Create the linode_instance_disk_update tool."""
    return Tool(
        name="linode_instance_disk_update",
        description="Updates a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "New label for the disk",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_disk_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.update_instance_disk(
            instance_id,
            disk_id,
            label=arguments.get("label"),
        )

    return await execute_tool(cfg, arguments, "update instance disk", _call)


def create_linode_instance_disk_delete_tool() -> Tool:
    """Create the linode_instance_disk_delete tool."""
    return Tool(
        name="linode_instance_disk_delete",
        description="Deletes a disk from a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": [
                "instance_id",
                "disk_id",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_disk_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.delete_instance_disk(instance_id, disk_id)
        return {
            "message": (f"Disk {disk_id} deleted from instance {instance_id}"),
            "instance_id": instance_id,
            "disk_id": disk_id,
        }

    return await execute_tool(cfg, arguments, "delete instance disk", _call)


def create_linode_instance_disk_clone_tool() -> Tool:
    """Create the linode_instance_disk_clone tool."""
    return Tool(
        name="linode_instance_disk_clone",
        description="Clones a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_disk_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_clone tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.clone_instance_disk(instance_id, disk_id)

    return await execute_tool(cfg, arguments, "clone instance disk", _call)


def create_linode_instance_disk_resize_tool() -> Tool:
    """Create the linode_instance_disk_resize tool."""
    return Tool(
        name="linode_instance_disk_resize",
        description="Resizes a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "size": {
                    "type": "integer",
                    "description": "New size in MB",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "size",
                "confirm",
            ],
        },
    )


async def handle_linode_instance_disk_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_resize tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    size = arguments.get("size")
    if not size:
        return _error_response("size is required")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.resize_instance_disk(instance_id, disk_id, int(size))
        return {
            "message": (f"Disk {disk_id} resized to {size} MB"),
            "instance_id": instance_id,
            "disk_id": disk_id,
            "size": int(size),
        }

    return await execute_tool(cfg, arguments, "resize instance disk", _call)
