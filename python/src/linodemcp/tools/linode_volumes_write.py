from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import volume_pb2
from linodemcp.linode import validate_label, validate_volume_size
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    required_int_id,
)
from linodemcp.tools.proto_response import raw_int, raw_str, serialize_api_response
from linodemcp.tools.toolschemas import schema
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
        inputSchema=schema("linode.mcp.v1.VolumeCreateInput"),
    ), Capability.Write


def _volume_create_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate volume create fields; return an error response or None."""
    label = arguments.get("label", "")
    if not label:
        return error_response("label is required")
    # The API needs a region or an attach target to place the volume; Go
    # enforces this, so mirror it here for identical cross-language rejection.
    if not arguments.get("region") and not arguments.get("linode_id"):
        return error_response("either region or linode_id is required")
    try:
        validate_label(label)
        # Validate size only when the caller supplied it. An omitted size defers
        # to the API's documented default (20 GB), matching Go, which validates
        # only when size > 0.
        size = arguments.get("size", 0)
        if size:
            validate_volume_size(size)
    except ValueError as exc:
        return error_response(str(exc))
    return None


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
        return error_response(
            "This operation creates a billable resource. Set confirm=true to proceed."
        )

    fields_error = _volume_create_error(arguments)
    if fields_error is not None:
        return fields_error

    body: dict[str, Any] = {"label": label}
    # Send size only when the caller provided it; an omitted size lets the API
    # apply its documented 20 GB default, matching Go's omitempty on size.
    size = arguments.get("size", 0)
    if size:
        body["size"] = size
    if arguments.get("region"):
        body["region"] = arguments.get("region")
    if arguments.get("linode_id") is not None:
        body["linode_id"] = arguments.get("linode_id")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.post_raw("/volumes", body)
        vol_label = raw_str(raw, "label")
        vol_id = raw_int(raw, "id")
        vol_region = raw_str(raw, "region")
        return serialize_api_response(
            {
                "message": (
                    f"Volume '{vol_label}' (ID: {vol_id}) "
                    f"created successfully in {vol_region}"
                ),
                "volume": raw,
            },
            volume_pb2.VolumeWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.VolumeCloneInput"),
    ), Capability.Write


def _volume_clone_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate clone args; return an error response or None.

    volume_id runs through required_int_id so a non-positive id is rejected
    locally ("volume_id must be a positive integer") instead of being sent as
    /volumes/-1/clone, matching Go's requiredIDArgument (strictest-wins).
    """
    _, error = required_int_id(arguments, "volume_id")
    if error:
        return error_response(error)
    if not arguments.get("label"):
        return error_response("label is required")
    return None


async def handle_linode_volume_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_clone tool request."""
    volume_id = arguments.get("volume_id", 0)
    label = arguments.get("label", "")

    if is_dry_run(arguments):
        fields_error = _volume_clone_error(arguments)
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
        return error_response(
            "This operation creates a billable cloned volume. Set confirm=true to "
            "proceed."
        )

    fields_error = _volume_clone_error(arguments)
    if fields_error is not None:
        return fields_error

    try:
        validate_label(label)
    except ValueError as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoint = f"/volumes/{int(volume_id)}/clone"
        raw = await client.post_raw(endpoint, {"label": label})
        vol_label = raw_str(raw, "label")
        return serialize_api_response(
            {
                "message": f'Volume {volume_id} cloned successfully as "{vol_label}"',
                "volume": raw,
            },
            volume_pb2.VolumeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "clone volume", _call)


def create_linode_volume_attach_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_attach tool."""
    return Tool(
        name="linode_volume_attach",
        description="Attaches a block storage volume to a Linode instance.",
        inputSchema=schema("linode.mcp.v1.VolumeAttachInput"),
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

    if not volume_id:
        return error_response("volume_id is required")
    if not linode_id:
        return error_response("linode_id is required")

    if is_dry_run(arguments):

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

    if arguments.get("confirm") is not True:
        return error_response(
            "This attaches a block storage volume to an instance. "
            "Set confirm=true to proceed."
        )

    body: dict[str, Any] = {"linode_id": int(linode_id)}
    # Send persist_across_boots only when the caller set it true; an omitted or
    # false value defers to the API default, matching Go's omitempty on the bool.
    if arguments.get("persist_across_boots"):
        body["persist_across_boots"] = True
    if arguments.get("config_id") is not None:
        body["config_id"] = arguments.get("config_id")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoint = f"/volumes/{int(volume_id)}/attach"
        raw = await client.post_raw(endpoint, body)
        return serialize_api_response(
            {
                "message": (
                    f"Volume {volume_id} attached to Linode {linode_id} successfully"
                ),
                "volume": raw,
            },
            volume_pb2.VolumeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "attach volume", _call)


def create_linode_volume_detach_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_detach tool."""
    return Tool(
        name="linode_volume_detach",
        description="Detaches a block storage volume from a Linode instance.",
        inputSchema=schema("linode.mcp.v1.VolumeDetachInput"),
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

    if arguments.get("confirm") is not True:
        return error_response(
            "This detaches a block storage volume from an instance. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.detach_volume(int(volume_id))
        return serialize_api_response(
            {
                "message": f"Volume {volume_id} detached successfully",
                "volume_id": volume_id,
            },
            volume_pb2.VolumeDetachResponse(),
        )

    return await execute_tool(cfg, arguments, "detach volume", _call)


def create_linode_volume_resize_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_resize tool."""
    return Tool(
        name="linode_volume_resize",
        description=(
            "Resizes a block storage volume. WARNING: Volumes can only be resized "
            "up, not down. This increases billing."
        ),
        inputSchema=schema("linode.mcp.v1.VolumeResizeInput"),
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
        return error_response(
            "This operation may increase billing. Volumes cannot be downsized. Set "
            "confirm=true to proceed."
        )

    fields_error = _volume_resize_error(volume_id, size)
    if fields_error is not None:
        return fields_error

    try:
        validate_volume_size(int(size))
    except ValueError as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoint = f"/volumes/{int(volume_id)}/resize"
        raw = await client.post_raw(endpoint, {"size": int(size)})
        return serialize_api_response(
            {
                "message": (
                    f"Volume {volume_id} resize to {size} GB initiated successfully"
                ),
                "volume": raw,
            },
            volume_pb2.VolumeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "resize volume", _call)


def create_linode_volume_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_volume_update tool."""
    return Tool(
        name="linode_volume_update",
        description="Updates a block storage volume label or tags.",
        inputSchema=schema("linode.mcp.v1.VolumeUpdateInput"),
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
        return error_response(
            "This updates a block storage volume. Set confirm=true to proceed."
        )

    fields_error = _volume_update_error(volume_id, label, tags)
    if fields_error is not None:
        return fields_error

    if label is not None:
        try:
            validate_label(label)
        except ValueError as exc:
            return error_response(str(exc))

    body: dict[str, Any] = {}
    if label is not None:
        body["label"] = label
    if tags is not None:
        body["tags"] = tags

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.put_raw(f"/volumes/{int(volume_id)}", body)
        return serialize_api_response(
            {
                "message": f"Volume {volume_id} updated successfully",
                "volume": raw,
            },
            volume_pb2.VolumeWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.VolumeDeleteInput"),
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
        return serialize_api_response(
            {
                "message": f"Volume {volume_id} removed successfully",
                "volume_id": volume_id,
            },
            volume_pb2.VolumeDeleteResponse(),
        )

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
        return error_response(
            "This operation is destructive and irreversible. Set confirm=true to "
            "proceed."
        )

    if not volume_id:
        return error_response("volume_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_volume(int(volume_id))
        return serialize_api_response(
            {
                "message": f"Volume {volume_id} removed successfully",
                "volume_id": volume_id,
            },
            volume_pb2.VolumeDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete volume", _call)
