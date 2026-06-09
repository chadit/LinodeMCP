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
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_volume_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_create tool."""
    return Tool(
        name="linode_volume_create",
        description=(
            "Creates a new block storage volume. WARNING: Billing starts immediately."
            " Pass dry_run=true to preview without creating."
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


async def handle_linode_volume_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_create tool request."""
    label = arguments.get("label", "")

    if is_dry_run(arguments):
        if not label:
            return error_response("label is required")
        size = arguments.get("size", 20)
        region = arguments.get("region")
        attach_to = arguments.get("linode_id")
        effect = f"A new {size} GB volume {label!r} will be created"
        if region:
            effect += f" in region {region}"
        side_effects = [f"{effect}."]
        if attach_to:
            side_effects.append(
                f"The volume is attached to instance {attach_to} on creation."
            )
        return build_dry_run_response(
            "linode_volume_create",
            arguments.get("environment", ""),
            "POST",
            "/volumes",
            None,
            side_effects=side_effects,
            warnings=["Billing for the volume starts immediately on creation."],
        )

    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    if not label:
        return error_response("label is required")

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


def create_linode_volume_clone_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_clone tool."""
    return Tool(
        name="linode_volume_clone",
        description=(
            "Clones a block storage volume. WARNING: The cloned volume is a "
            "new billable resource."
            " Pass dry_run=true to preview without cloning."
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
                    "description": "The ID of the volume to clone (required)",
                },
                "label": {
                    "type": "string",
                    "description": "Label for the cloned volume (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm cloning. This incurs billing."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["volume_id", "label", "confirm"],
        },
    ), Capability.Write


def _volume_clone_error(volume_id: Any, label: str) -> list[TextContent] | None:
    """Validate clone args; return an error response or None."""
    if not volume_id:
        return error_response("volume_id is required")
    if not label:
        return error_response("label is required")
    return None


async def handle_linode_volume_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_clone tool request."""
    volume_id = arguments.get("volume_id", 0)
    label = arguments.get("label", "")

    if is_dry_run(arguments):
        fields_error = _volume_clone_error(volume_id, label)
        if fields_error is not None:
            return fields_error

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_volume(int(volume_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_volume_clone",
            "POST",
            f"/volumes/{int(volume_id)}/clone",
            _fetch,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    fields_error = _volume_clone_error(volume_id, label)
    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.clone_volume(int(volume_id), label)
        return {
            "message": (
                f"Volume {volume_id} cloned successfully as "
                f"'{volume.label}' (ID: {volume.id})"
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

    return await execute_tool(cfg, arguments, "clone volume", _call)


def create_linode_volume_attach_tool() -> tuple[Tool, Capability]:
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["volume_id", "linode_id", "confirm"],
        },
    ), Capability.Write


def _volume_attach_side_effects(volume_id: int, linode_id: int) -> DryRunDetails:
    """Phase 2 Tier B walk for volume attach. Describes the attachment the call
    would make; the instance is an argument, not read from the volume state.
    """
    return {
        "side_effects": [
            f"Volume {volume_id} attaches to instance {linode_id}; the volume "
            "must be in the same region as the instance."
        ]
    }


async def handle_linode_volume_attach(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_attach tool request."""
    volume_id = arguments.get("volume_id", 0)
    linode_id = arguments.get("linode_id", 0)

    if is_dry_run(arguments):
        if not volume_id:
            return error_response("volume_id is required")
        if not linode_id:
            return error_response("linode_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_volume(int(volume_id))

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _volume_attach_side_effects(int(volume_id), int(linode_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_volume_attach",
            "POST",
            f"/volumes/{int(volume_id)}/attach",
            _fetch,
            _walk,
        )

    if not volume_id:
        return error_response("volume_id is required")
    if not linode_id:
        return error_response("linode_id is required")

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


def create_linode_volume_detach_tool() -> tuple[Tool, Capability]:
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["volume_id", "confirm"],
        },
    ), Capability.Write


def _volume_detach_side_effects(state: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for volume detach. Reads the volume's current
    attachment from the fetched state and reports the detach (data preserved,
    billing continues). State-only, no extra API call.
    """
    attached = getattr(state, "linode_id", None)
    if not attached:
        return {
            "side_effects": [
                "Volume is not attached to any instance; detach is a no-op."
            ]
        }
    return {
        "side_effects": [
            f"Volume detaches from instance {attached}; its data is preserved "
            "and billing continues."
        ]
    }


async def handle_linode_volume_detach(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_detach tool request."""
    volume_id = arguments.get("volume_id", 0)

    if not volume_id:
        return error_response("volume_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_volume(int(volume_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _volume_detach_side_effects(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_volume_detach",
            "POST",
            f"/volumes/{int(volume_id)}/detach",
            _fetch,
            _walk,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.detach_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} detached successfully",
            "volume_id": volume_id,
        }

    return await execute_tool(cfg, arguments, "detach volume", _call)


def create_linode_volume_resize_tool() -> tuple[Tool, Capability]:
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["volume_id", "size", "confirm"],
        },
    ), Capability.Write


def _volume_resize_error(volume_id: Any, size: Any) -> list[TextContent] | None:
    """Validate resize args; return an error response or None."""
    if not volume_id:
        return error_response("volume_id is required")
    if not size:
        return error_response("size is required")
    return None


def _volume_resize_side_effects(state: Any, target_size: int) -> DryRunDetails:
    """Phase 2 Tier B walk for volume resize. Names the size change (from the
    fetched state to the requested size) and warns a volume can only grow.
    """
    from_size = getattr(state, "size", 0)
    if from_size:
        effect = f"Volume resizes from {from_size} GB to {target_size} GB."
    else:
        effect = f"Volume resizes to {target_size} GB."
    return {
        "side_effects": [effect],
        "warnings": [
            "A volume can only grow; the new size must be larger than the current size."
        ],
    }


async def handle_linode_volume_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_resize tool request."""
    volume_id = arguments.get("volume_id", 0)
    size = arguments.get("size", 0)

    if is_dry_run(arguments):
        fields_error = _volume_resize_error(volume_id, size)
        if fields_error is not None:
            return fields_error

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_volume(int(volume_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _volume_resize_side_effects(state, int(size))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_volume_resize",
            "POST",
            f"/volumes/{int(volume_id)}/resize",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This increases billing. Set confirm=true to proceed.",
            )
        ]

    fields_error = _volume_resize_error(volume_id, size)
    if fields_error is not None:
        return fields_error

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


def create_linode_volume_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_update tool."""
    return Tool(
        name="linode_volume_update",
        description="Updates a block storage volume label or tags.",
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
                    "description": "The ID of the volume to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New volume label (optional)",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Replacement tags for the volume (optional)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm update. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["volume_id", "confirm"],
        },
    ), Capability.Write


def _volume_update_error(
    volume_id: Any, label: Any, tags: Any
) -> list[TextContent] | None:
    """Validate update args; return an error response or None."""
    if not volume_id:
        return error_response("volume_id is required")
    if label is None and tags is None:
        return error_response("label or tags is required")
    return None


def _volume_update_side_effects(
    state: Any, new_label: Any, new_tags: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for volume update. Reports the label change (against
    the fetched state) and notes when the tag set is replaced.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f"Label changes from {from_label!r} to {new_label!r}.")
        else:
            side_effects.append(f"Label is set to {new_label!r}.")
    if new_tags is not None:
        side_effects.append("The volume's tag set is replaced with the provided tags.")
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_volume_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_update tool request."""
    volume_id = arguments.get("volume_id", 0)
    label = arguments.get("label")
    tags = arguments.get("tags")

    if is_dry_run(arguments):
        fields_error = _volume_update_error(volume_id, label, tags)
        if fields_error is not None:
            return fields_error

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_volume(int(volume_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _volume_update_side_effects(state, label, tags)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_volume_update",
            "PUT",
            f"/volumes/{int(volume_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This updates a volume. Set confirm=true to proceed.",
            )
        ]

    fields_error = _volume_update_error(volume_id, label, tags)
    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.update_volume(
            volume_id=int(volume_id),
            label=label,
            tags=tags,
        )
        return {
            "message": f"Volume {volume_id} updated successfully",
            "volume": {
                "id": volume.id,
                "label": volume.label,
                "tags": volume.tags,
            },
        }

    return await execute_tool(cfg, arguments, "update volume", _call)


def create_linode_volume_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_delete tool."""
    return Tool(
        name="linode_volume_delete",
        description=(
            "Deletes a block storage volume. WARNING: This is destructive "
            "and all data will be lost. Volume must be detached first. "
            "Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
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
                    "description": (
                        "Must be true to confirm deletion. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["volume_id", "confirm"],
        },
    ), Capability.Destroy


async def _volume_delete_dependency_walk(
    _client: RetryableClient, state: Any
) -> DryRunDetails:
    """Phase 2 Tier A walk for volume delete. The instance the volume is
    attached to detaches before the volume is destroyed. Read straight from
    the volume state (which carries linode_id/linode_label); no extra call.
    """
    details: DryRunDetails = {}
    linode_id = getattr(state, "linode_id", None)
    if not linode_id:
        return details

    label = getattr(state, "linode_label", None) or ""
    details["dependencies"] = [
        {
            "kind": "instance",
            "id": linode_id,
            "label": label,
            "action": "detached",
            "note": (
                "Volume is attached; it detaches from this instance before deletion."
            ),
        }
    ]
    details["warnings"] = [
        "Volume is currently attached to an instance; "
        "it will be detached as part of deletion."
    ]
    return details


async def _volume_delete_two_stage(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    volume_id = arguments.get("volume_id", 0)
    if not volume_id:
        return error_response("volume_id is required")

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_volume(int(volume_id))

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} deleted successfully",
            "volume_id": volume_id,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_volume_delete",
        method="DELETE",
        path=f"/volumes/{int(volume_id)}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Volume"),
        dependency_walk=_volume_delete_dependency_walk,
    )


async def handle_linode_volume_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_delete tool request."""
    volume_id = arguments.get("volume_id", 0)

    two_stage = await _volume_delete_two_stage(arguments, cfg)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        if not volume_id:
            return error_response("volume_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_volume(int(volume_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_volume_delete",
            "DELETE",
            f"/volumes/{int(volume_id)}",
            _fetch,
            _volume_delete_dependency_walk,
        )

    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not volume_id:
        return error_response("volume_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} deleted successfully",
            "volume_id": volume_id,
        }

    return await execute_tool(cfg, arguments, "delete volume", _call)
