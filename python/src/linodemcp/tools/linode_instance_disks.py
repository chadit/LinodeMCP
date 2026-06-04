from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeGuard, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    DryRunDetails,
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


def _is_non_empty_dict(value: Any) -> TypeGuard[dict[str, Any]]:
    """Return whether a value is a non-empty dictionary."""
    if not isinstance(value, dict):
        return False
    candidate = cast("dict[str, Any]", value)
    return len(candidate) > 0


def create_linode_instance_disks_list_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


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


def create_linode_instance_disk_get_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


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


def create_linode_instance_config_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_create tool."""
    return Tool(
        name="linode_instance_config_create",
        description="Creates a configuration profile on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "instance_id": _INSTANCE_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the configuration profile",
                },
                "devices": {
                    "type": "object",
                    "description": "Config devices mapping, such as sda/sdb entries",
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "label", "devices", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_config_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_config_create tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    label = arguments.get("label", "")
    if not isinstance(label, str) or not label:
        return _error_response("label is required")

    devices = arguments.get("devices")
    if not _is_non_empty_dict(devices):
        return _error_response("devices must be a non-empty object")
    devices_payload = devices

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"A new config profile {label!r} will be created on instance {iid}."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_config_create",
            "POST",
            f"/linode/instances/{iid}/configs",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if confirm is not True:
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.create_instance_config(
            iid, label=label, devices=devices_payload
        )

    return await execute_tool(cfg, arguments, "create instance config", _call)


def create_linode_instance_disk_create_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "instance_id",
                "label",
                "size",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_instance_disk_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_create tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    label = arguments.get("label", "")
    if not label:
        return _error_response("label is required")

    size = arguments.get("size")
    if not size:
        return _error_response("size is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"A new {size} MB disk {label!r} will be created on instance {iid}."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_create",
            "POST",
            f"/linode/instances/{iid}/disks",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

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


def create_linode_instance_disk_update_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_instance_disk_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_update tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(instance_id, disk_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_update",
            "PUT",
            f"/linode/instances/{instance_id}/disks/{disk_id}",
            _fetch,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.update_instance_disk(
            instance_id,
            disk_id,
            label=arguments.get("label"),
        )

    return await execute_tool(cfg, arguments, "update instance disk", _call)


def create_linode_instance_disk_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_delete tool."""
    return Tool(
        name="linode_instance_disk_delete",
        description=(
            "Deletes a disk from a Linode instance."
            " Pass dry_run=true to preview without deleting."
        ),
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "confirm",
            ],
        },
    ), Capability.Destroy


async def handle_linode_instance_disk_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_delete tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(instance_id, disk_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_delete",
            "DELETE",
            f"/linode/instances/{instance_id}/disks/{disk_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

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


def create_linode_instance_disk_clone_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "confirm",
            ],
        },
    ), Capability.Write


def _instance_disk_clone_side_effects(state: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for instance disk clone. Reports the new disk a
    clone creates on the same instance and the storage it consumes.
    """
    if isinstance(state, dict):
        disk = cast("dict[str, Any]", state)
        label = disk.get("label", "")
        size = disk.get("size", 0)
        return {
            "side_effects": [
                f"Disk {label!r} ({size} MB) is cloned to a new disk on the "
                f"same instance, consuming {size} MB of additional storage."
            ]
        }
    return {
        "side_effects": [
            "A copy of the disk is created on the same instance, consuming "
            "additional storage."
        ]
    }


async def handle_linode_instance_disk_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_clone tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(instance_id, disk_id)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_disk_clone_side_effects(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_clone",
            "POST",
            f"/linode/instances/{instance_id}/disks/{disk_id}/clone",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.clone_instance_disk(instance_id, disk_id)

    return await execute_tool(cfg, arguments, "clone instance disk", _call)


def create_linode_instance_disk_resize_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "instance_id",
                "disk_id",
                "size",
                "confirm",
            ],
        },
    ), Capability.Write


def _instance_disk_resize_side_effects(state: Any, target_size: int) -> DryRunDetails:
    """Phase 2 Tier B walk for instance disk resize. Names the size change (in
    MB) and notes the instance must be powered off.
    """
    from_size = 0
    if isinstance(state, dict):
        from_size = cast("dict[str, Any]", state).get("size", 0)
    if from_size:
        effect = f"Disk resizes from {from_size} MB to {target_size} MB."
    else:
        effect = f"Disk resizes to {target_size} MB."
    return {
        "side_effects": [effect],
        "warnings": ["The instance must be powered off to resize a disk."],
    }


async def handle_linode_instance_disk_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_resize tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    instance_id, disk_id = ids

    size = arguments.get("size")
    if not size:
        return _error_response("size is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(instance_id, disk_id)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_disk_resize_side_effects(state, int(size))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_resize",
            "POST",
            f"/linode/instances/{instance_id}/disks/{disk_id}/resize",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

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
