from __future__ import annotations

from typing import TYPE_CHECKING, Any

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import instance_pb2
from linodemcp.linode import APIError, NetworkError, instance_preview_state
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    preview_state_str,
)
from linodemcp.tools.proto_response import raw_int, raw_str, serialize_api_response
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


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


def create_linode_instance_clone_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_clone tool."""
    return Tool(
        name="linode_instance_clone",
        description="Clones a Linode instance",
        inputSchema=schema("linode.mcp.v1.InstanceCloneInput"),
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
            return instance_preview_state(await client.get_instance(iid))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_clone",
            "POST",
            f"/linode/instances/{iid}/clone",
            _fetch,
        )

    if not arguments.get("confirm"):
        return _error_response(
            "This clones a Linode instance and creates a billable resource. Set "
            "confirm=true to proceed."
        )

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        raw = await client.clone_instance_raw(
            iid,
            region=arguments.get("region"),
            instance_type=arguments.get("type"),
            label=arguments.get("label"),
            backups_enabled=bool(arguments.get("backups_enabled", False)),
            disks=arguments.get("disks"),
            configs=arguments.get("configs"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Instance {iid} cloned as '{raw_str(raw, 'label')}' "
                    f"(ID: {raw_int(raw, 'id')}) in {raw_str(raw, 'region')}"
                ),
                "instance": raw,
            },
            instance_pb2.InstanceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "clone instance", _call)


def create_linode_instance_migrate_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_migrate tool."""
    return Tool(
        name="linode_instance_migrate",
        description=("Migrates a Linode instance to a new region"),
        inputSchema=schema("linode.mcp.v1.InstanceMigrateInput"),
    ), Capability.Write


def _instance_migrate_side_effects(state: Any, target_region: str) -> DryRunDetails:
    """Phase 2 Tier B walk for instance migrate. Names the region change (from
    the fetched state to the requested region) and notes the downtime.
    """
    from_region = preview_state_str(state, "region")
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
            return instance_preview_state(await client.get_instance(iid))

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
        return _error_response(
            "This migrates the instance and causes downtime during migration. Set "
            "confirm=true to proceed."
        )

    region = arguments.get("region", "")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.migrate_instance(iid, region=region or None)
        # Echo the target region only when the caller picked one; an omitted
        # region (Linode picks the destination) leaves the field unset so it
        # stays absent from the output, matching Go's InstanceMigrateWriteResponse.
        payload: dict[str, Any] = {
            "message": f"Migration initiated for instance {iid}",
            "linode_id": iid,
        }
        if region:
            payload["message"] = (
                f"Migration initiated for instance {iid} to region {region}"
            )
            payload["region"] = region
        return serialize_api_response(
            payload,
            instance_pb2.InstanceMigrateWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.InstanceRebuildInput"),
    ), Capability.Destroy


async def _instance_rebuild_side_effects_walk(
    client: RetryableClient, linode_id: int, state: Any
) -> DryRunDetails:
    """Phase 2 Tier A walk for instance rebuild. A rebuild erases every disk
    and recreates the boot disk from the new image, so each existing disk is a
    side effect and the current image is named in a warning. Best-effort: a
    failed disk list becomes a warning.
    """
    warnings: list[str] = []
    try:
        disks = await client.list_instance_disks(linode_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        warnings.append(f"Could not list instance disks: {exc}")
        disks = []

    # Double quotes match the Go walk's %q formatting, so the fixture-pinned
    # side-effect text is identical across languages.
    side_effects = [
        f'Disk "{disk.get("label", "")}" ({disk.get("size", 0)} MB, '
        f"{disk.get('filesystem', '')}) is erased and recreated from the new image."
        for disk in disks
    ]

    image = preview_state_str(state, "image")
    if image:
        warnings.append(
            f'Rebuild replaces the current image "{image}", destroys all data, '
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
    arguments: dict[str, Any], cfg: Config, linode_id: int, image: str, root_pass: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return instance_preview_state(await client.get_instance(linode_id))

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.rebuild_instance(
            linode_id,
            image=image,
            root_pass=root_pass,
            authorized_keys=arguments.get("authorized_keys"),
            authorized_users=arguments.get("authorized_users"),
        )
        return serialize_api_response(
            {
                "message": f"Instance {linode_id} rebuilt with image {image}",
                "instance": instance,
            },
            instance_pb2.InstanceWriteResponse(),
        )

    async def _ts_walk(client: RetryableClient, state: Any) -> DryRunDetails:
        return await _instance_rebuild_side_effects_walk(client, linode_id, state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_rebuild",
        method="POST",
        path=f"/linode/instances/{linode_id}/rebuild",
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
            return instance_preview_state(await client.get_instance(iid))

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
        return _error_response(
            "This DESTROYS ALL DATA on the instance and rebuilds it. Set confirm=true "
            "to proceed."
        )

    # Mirror Go's explicit-only semantics: pass booted only when the caller
    # supplied the key, so an omitted value leaves the API default (true).
    booted = bool(arguments["booted"]) if "booted" in arguments else None

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        instance = await client.rebuild_instance(
            iid,
            image=image,
            root_pass=root_pass,
            authorized_keys=arguments.get("authorized_keys"),
            authorized_users=arguments.get("authorized_users"),
            booted=booted,
        )
        return serialize_api_response(
            {
                "message": f"Instance {iid} rebuilt with image {image}",
                "instance": instance,
            },
            instance_pb2.InstanceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "rebuild instance", _call)


def create_linode_instance_rescue_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_rescue tool."""
    return Tool(
        name="linode_instance_rescue",
        description=("Boots a Linode instance into rescue mode"),
        inputSchema=schema("linode.mcp.v1.InstanceRescueInput"),
    ), Capability.Write


def _instance_rescue_side_effects_walk(state: Any) -> DryRunDetails:
    """Phase 2 Tier A walk for instance rescue. Rescue mode reboots the
    instance and bypasses its normal boot configuration until the operator
    reboots out of it; reported as a side effect, with a downtime warning when
    the instance is running. State-only, no extra API call.
    """
    details: DryRunDetails = {
        "side_effects": [
            (
                "The instance reboots into rescue mode; its normal boot "
                "configuration is bypassed until you reboot out of rescue mode."
            )
        ]
    }
    if preview_state_str(state, "status") == "running":
        details["warnings"] = [
            (
                "Instance is currently running; entering rescue mode reboots it, "
                "causing downtime."
            )
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
            return instance_preview_state(await client.get_instance(iid))

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
        return _error_response(
            "This reboots the instance into rescue mode. Set confirm=true to proceed."
        )

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.rescue_instance(iid, devices=arguments.get("devices"))
        return serialize_api_response(
            {
                "message": f"Instance {iid} is booting into rescue mode",
                "linode_id": iid,
            },
            instance_pb2.InstanceActionWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.InstancePasswordResetInput"),
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
    if preview_state_str(state, "status") == "running":
        details["warnings"] = [
            (
                "Instance is currently running; the reset shuts it down and "
                "reboots it, causing downtime."
            )
        ]
    return details


async def _instance_password_reset_two_stage(
    arguments: dict[str, Any], cfg: Config, iid: int, root_pass: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return instance_preview_state(await client.get_instance(iid))

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.reset_instance_password(iid, root_pass)
        return serialize_api_response(
            {
                "message": f"Root password reset for instance {iid}",
                "linode_id": iid,
            },
            instance_pb2.InstanceActionWriteResponse(),
        )

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
            return instance_preview_state(await client.get_instance(iid))

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
        return _error_response(
            "This resets the root password on the instance. Set confirm=true to "
            "proceed."
        )

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.reset_instance_password(iid, root_pass)
        return serialize_api_response(
            {
                "message": f"Root password reset for instance {iid}",
                "linode_id": iid,
            },
            instance_pb2.InstanceActionWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "reset instance password", _call)
