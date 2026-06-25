from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    MODE_PROP,
    PARAM_DRY_RUN,
    PARAM_MODE,
    PARAM_PLAN_ID,
    PLAN_ID_PROP,
    TWO_STAGE_NOTE,
    DryRunDetails,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

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

_LINODE_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The ID of the Linode instance (required)",
}

_BACKUP_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The ID of the backup (required)",
}

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}


def _parse_instance_id(
    arguments: dict[str, Any],
) -> int | list[TextContent]:
    """Parse and validate linode_id from arguments."""
    raw = arguments.get("linode_id", "")
    if not raw:
        return _error_response("linode_id is required")
    try:
        return int(raw)
    except (ValueError, TypeError):
        return _error_response("linode_id must be a valid integer")


def _parse_instance_and_backup_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse linode_id and backup_id from arguments."""
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


def create_linode_instance_backup_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_backup_list tool."""
    return Tool(
        name="linode_instance_backup_list",
        description=("Lists backups for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_instance_backup_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.list_instance_backups(iid)

    return await execute_tool(cfg, arguments, "list instance backups", _call)


def create_linode_instance_backup_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_backup_get tool."""
    return Tool(
        name="linode_instance_backup_get",
        description=("Gets details of a specific backup for an instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "backup_id": _BACKUP_ID_PROP,
            },
            "required": ["linode_id", "backup_id"],
        },
    ), Capability.Read


async def handle_linode_instance_backup_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_get tool request."""
    ids = _parse_instance_and_backup_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, backup_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.get_instance_backup(linode_id, backup_id)

    return await execute_tool(cfg, arguments, "get instance backup", _call)


def create_linode_instance_backup_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_backup_create tool."""
    return Tool(
        name="linode_instance_backup_create",
        description=("Creates a snapshot backup of a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the snapshot",
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_backup_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_create tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_backup_create",
            "POST",
            f"/linode/instances/{iid}/backups",
            _fetch,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.create_instance_backup(iid, label=arguments.get("label"))

    return await execute_tool(cfg, arguments, "create instance backup", _call)


def create_linode_instance_backup_restore_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_backup_restore tool."""
    return Tool(
        name="linode_instance_backup_restore",
        description="Restores a backup to a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "backup_id": _BACKUP_ID_PROP,
                "target_linode_id": {
                    "type": "integer",
                    "description": ("Target instance ID for restore"),
                },
                "overwrite": {
                    "type": "boolean",
                    "description": ("Overwrite existing data on target"),
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "backup_id",
                "target_linode_id",
                "confirm",
            ],
        },
    ), Capability.Write


def _backup_restore_side_effects(target_id: int, overwrite: bool) -> DryRunDetails:
    """Phase 2 Tier A walk for backup restore. The side effect depends on the
    overwrite flag: with overwrite the target instance's existing disks and
    configs are destroyed and replaced, otherwise the backup is restored
    alongside what is already there. Args-based, no API call.
    """
    if overwrite:
        return {
            "side_effects": [
                f"All existing disks and configs on target instance {target_id} "
                "are destroyed and replaced by the backup."
            ],
            "warnings": [
                f"overwrite=true: existing data on target instance {target_id} "
                "is permanently lost."
            ],
        }
    return {
        "side_effects": [
            f"The backup is restored onto target instance {target_id}; the "
            "restore fails if its disks or configs collide."
        ]
    }


async def handle_linode_instance_backup_restore(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backup_restore request."""
    ids = _parse_instance_and_backup_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, backup_id = ids

    raw_target = arguments.get("target_linode_id")
    if not raw_target:
        return _error_response("target_linode_id is required")
    target_linode_id = int(raw_target)

    if is_dry_run(arguments):
        overwrite = arguments.get("overwrite", False)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_backup(linode_id, backup_id)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _backup_restore_side_effects(target_linode_id, overwrite)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_backup_restore",
            "POST",
            f"/linode/instances/{linode_id}/backups/{backup_id}/restore",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    overwrite = arguments.get("overwrite", False)

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.restore_instance_backup(
            linode_id,
            backup_id,
            target_linode_id,
            overwrite=overwrite,
        )
        return {
            "message": (f"Backup {backup_id} restored to instance {target_linode_id}"),
            "linode_id": linode_id,
            "backup_id": backup_id,
        }

    return await execute_tool(cfg, arguments, "restore instance backup", _call)


def create_linode_instance_backups_enable_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_backups_enable tool."""
    return Tool(
        name="linode_instance_backups_enable",
        description=("Enables backups for a Linode instance (billing charges apply)"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_backups_enable(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backups_enable request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_backups_enable",
            "POST",
            f"/linode/instances/{iid}/backups/enable",
            _fetch,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.enable_instance_backups(iid)
        return {
            "message": (f"Backups enabled for instance {iid}"),
            "linode_id": iid,
        }

    return await execute_tool(cfg, arguments, "enable instance backups", _call)


def create_linode_instance_backups_cancel_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_backups_cancel tool."""
    return Tool(
        name="linode_instance_backups_cancel",
        description=(
            "Cancels backups for a Linode instance."
            " All existing backups will be deleted."
            " Pass dry_run=true to preview without canceling."
        )
        + TWO_STAGE_NOTE,
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm. Existing backups will be deleted."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["linode_id", "confirm"],
        },
    ), Capability.Destroy


async def _instance_backups_cancel_two_stage(
    arguments: dict[str, Any], cfg: Config, iid: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_instance(iid)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.cancel_instance_backups(iid)
        return {
            "message": f"Backups cancelled for instance {iid}",
            "linode_id": iid,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_backups_cancel",
        method="POST",
        path=f"/linode/instances/{iid}/backups/cancel",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Instance"),
    )


async def handle_linode_instance_backups_cancel(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_backups_cancel request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    two_stage = await _instance_backups_cancel_two_stage(arguments, cfg, iid)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_backups_cancel",
            "POST",
            f"/linode/instances/{iid}/backups/cancel",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.cancel_instance_backups(iid)
        return {
            "message": (f"Backups cancelled for instance {iid}"),
            "linode_id": iid,
        }

    return await execute_tool(cfg, arguments, "cancel instance backups", _call)
