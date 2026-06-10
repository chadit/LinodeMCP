from __future__ import annotations

from typing import TYPE_CHECKING, Any

import httpx
from mcp.types import TextContent, Tool

from linodemcp.linode import APIError, NetworkError
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


def create_linode_instance_clone_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_clone tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_clone",
            "POST",
            f"/linode/instances/{iid}/clone",
            _fetch,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

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


def create_linode_instance_migrate_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def _instance_migrate_side_effects(state: Any, target_region: str) -> DryRunDetails:
    """Phase 2 Tier B walk for instance migrate. Names the region change (from
    the fetched state to the requested region) and notes the downtime.
    """
    from_region = getattr(state, "region", "")
    if target_region and from_region:
        effect = (
            f"Instance migrates from region {from_region} to {target_region}; "
            "it is unavailable during the migration."
        )
    elif target_region:
        effect = (
            f"Instance migrates to region {target_region}; it is unavailable "
            "during the migration."
        )
    else:
        effect = "Instance migrates; it is unavailable during the migration."
    return {"side_effects": [effect]}


async def handle_linode_instance_migrate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_migrate tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    if is_dry_run(arguments):
        region = arguments.get("region", "")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_migrate_side_effects(state, region)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_migrate",
            "POST",
            f"/linode/instances/{iid}/migrate",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.migrate_instance(iid, region=arguments.get("region"))
        return {
            "message": (f"Migration initiated for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "migrate instance", _call)


def create_linode_instance_rebuild_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_rebuild tool."""
    return Tool(
        name="linode_instance_rebuild",
        description=(
            "Rebuilds a Linode instance with a new image."
            " All data on existing disks will be destroyed."
            " Pass dry_run=true to preview without rebuilding." + TWO_STAGE_NOTE
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": [
                "instance_id",
                "image",
                "root_pass",
                "confirm",
            ],
        },
    ), Capability.Destroy


async def _instance_rebuild_side_effects_walk(
    client: RetryableClient, instance_id: int, state: Any
) -> DryRunDetails:
    """Phase 2 Tier A walk for instance rebuild. A rebuild erases every disk
    and recreates the boot disk from the new image, so each existing disk is a
    side effect and the current image is named in a warning. Best-effort: a
    failed disk list becomes a warning.
    """
    warnings: list[str] = []
    try:
        disks = await client.list_instance_disks(instance_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        warnings.append(f"Could not list instance disks: {exc}")
        disks = []

    side_effects = [
        f"Disk {disk.get('label', '')!r} ({disk.get('size', 0)} MB, "
        f"{disk.get('filesystem', '')}) is erased and recreated from the new image."
        for disk in disks
    ]

    image = getattr(state, "image", "")
    if image:
        warnings.append(
            f"Rebuild replaces the current image {image!r}, destroys all data, "
            "and resets the root password."
        )
    else:
        warnings.append(
            "Rebuild destroys all data on the instance and resets the root password."
        )

    details: DryRunDetails = {"warnings": warnings}
    if side_effects:
        details["side_effects"] = side_effects
    return details


async def _instance_rebuild_two_stage(
    arguments: dict[str, Any], cfg: Config, instance_id: int, image: str, root_pass: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_instance(instance_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        return await client.rebuild_instance(
            instance_id,
            image=image,
            root_pass=root_pass,
            authorized_keys=arguments.get("authorized_keys"),
            authorized_users=arguments.get("authorized_users"),
        )

    async def _ts_walk(client: RetryableClient, state: Any) -> DryRunDetails:
        return await _instance_rebuild_side_effects_walk(client, instance_id, state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_rebuild",
        method="POST",
        path=f"/linode/instances/{instance_id}/rebuild",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Instance"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_instance_rebuild(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_rebuild tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    image = arguments.get("image", "")
    root_pass = arguments.get("root_pass", "")
    for field_name, value in (("image", image), ("root_pass", root_pass)):
        if not value:
            return _error_response(f"{field_name} is required")

    two_stage = await _instance_rebuild_two_stage(arguments, cfg, iid, image, root_pass)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(client: RetryableClient, state: Any) -> DryRunDetails:
            return await _instance_rebuild_side_effects_walk(client, iid, state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_rebuild",
            "POST",
            f"/linode/instances/{iid}/rebuild",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

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


def create_linode_instance_rescue_tool() -> tuple[Tool, Capability]:
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


def _instance_rescue_side_effects_walk(state: Any) -> DryRunDetails:
    """Phase 2 Tier A walk for instance rescue. Rescue mode reboots the
    instance and bypasses its normal boot configuration until the operator
    reboots out of it; reported as a side effect, with a downtime warning when
    the instance is running. State-only, no extra API call.
    """
    details: DryRunDetails = {
        "side_effects": [
            "The instance reboots into rescue mode; its normal boot "
            "configuration is bypassed until you reboot out of rescue mode."
        ]
    }
    if getattr(state, "status", "") == "running":
        details["warnings"] = [
            "Instance is currently running; entering rescue mode reboots it, "
            "causing downtime."
        ]
    return details


async def handle_linode_instance_rescue(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_rescue tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_rescue_side_effects_walk(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_rescue",
            "POST",
            f"/linode/instances/{iid}/rescue",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.rescue_instance(iid, devices=arguments.get("devices"))
        return {
            "message": (f"Rescue mode initiated for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "rescue instance", _call)


def create_linode_instance_password_reset_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_password_reset tool."""
    return Tool(
        name="linode_instance_password_reset",
        description=(
            "Resets the root password for a Linode instance."
            " Pass dry_run=true to preview without resetting."
        )
        + TWO_STAGE_NOTE,
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
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": [
                "instance_id",
                "root_pass",
                "confirm",
            ],
        },
    ), Capability.Destroy


def _instance_password_reset_side_effects_walk(state: Any) -> DryRunDetails:
    """Phase 2 Tier A walk for instance password reset. The reset powers the
    instance down and reboots it; that downtime is the only side effect, with
    an extra warning when the instance is currently running. State-only.
    """
    details: DryRunDetails = {
        "side_effects": [
            "The instance is powered down and rebooted to apply the new root password."
        ]
    }
    if getattr(state, "status", "") == "running":
        details["warnings"] = [
            "Instance is currently running; the reset shuts it down and "
            "reboots it, causing downtime."
        ]
    return details


async def _instance_password_reset_two_stage(
    arguments: dict[str, Any], cfg: Config, iid: int, root_pass: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_instance(iid)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.reset_instance_password(iid, root_pass)
        return {
            "message": f"Password reset for instance {iid}",
            "instance_id": iid,
        }

    async def _ts_walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _instance_password_reset_side_effects_walk(state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_password_reset",
        method="POST",
        path=f"/linode/instances/{iid}/password",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Instance"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_instance_password_reset(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_password_reset request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    root_pass = arguments.get("root_pass", "")
    if not root_pass:
        return _error_response("root_pass is required")

    two_stage = await _instance_password_reset_two_stage(arguments, cfg, iid, root_pass)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_password_reset_side_effects_walk(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_password_reset",
            "POST",
            f"/linode/instances/{iid}/password",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.reset_instance_password(iid, root_pass)
        return {
            "message": (f"Password reset for instance {iid}"),
            "instance_id": iid,
        }

    return await execute_tool(cfg, arguments, "reset instance password", _call)
