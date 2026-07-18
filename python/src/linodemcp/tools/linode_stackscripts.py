from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast
from urllib.parse import quote

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import stackscript_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    required_int_id,
)
from linodemcp.tools.proto_response import (
    raw_int,
    raw_str,
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_stackscript_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscript_list tool."""
    return Tool(
        name="linode_stackscript_list",
        description=(
            "Lists StackScripts. By default returns your own StackScripts. "
            "Can filter by public status, ownership, or label."
        ),
        inputSchema=schema("linode.mcp.v1.StackScriptListInput"),
    ), Capability.Read


async def handle_linode_stackscript_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscript_list tool request."""
    is_public_filter = arguments.get("is_public", "")
    mine_filter = arguments.get("mine", "")
    label_contains = arguments.get("label_contains", "")

    def _matches(script: dict[str, Any]) -> bool:
        if is_public_filter and bool(script.get("is_public", False)) != (
            is_public_filter.lower() == "true"
        ):
            return False
        if mine_filter and bool(script.get("mine", False)) != (
            mine_filter.lower() == "true"
        ):
            return False
        label = str(script.get("label", ""))
        return not (label_contains and label_contains.lower() not in label.lower())

    filters: list[str] = []
    if is_public_filter:
        filters.append(f"is_public={is_public_filter}")
    if mine_filter:
        filters.append(f"mine={mine_filter}")
    if label_contains:
        filters.append(f"label_contains={label_contains}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/linode/stackscripts")
        return serialize_list_response(
            raw,
            "stackscripts",
            stackscript_pb2.StackScriptListResponse(),
            filter_value=", ".join(filters) if filters else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve StackScripts", _call)


def create_linode_stackscript_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscript_get tool."""
    return Tool(
        name="linode_stackscript_get",
        description="Gets details for a specific StackScript.",
        inputSchema=schema("linode.mcp.v1.StackScriptGetInput"),
    ), Capability.Read


async def handle_linode_stackscript_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscript_get tool request."""
    stackscript_id, error = required_int_id(arguments, "stackscript_id")
    if stackscript_id is None:
        return error_response(error)

    encoded_stackscript_id = quote(str(stackscript_id), safe="")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_raw(f"/linode/stackscripts/{encoded_stackscript_id}"),
            stackscript_pb2.StackScript(),
        )

    return await execute_tool(
        cfg, arguments, f"retrieve StackScript {stackscript_id}", _call
    )


def create_linode_stackscript_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscript_delete tool."""
    return Tool(
        name="linode_stackscript_delete",
        description="Deletes a StackScript by ID." + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.StackScriptDeleteInput"),
    ), Capability.Destroy


async def _stackscript_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, stackscript_id: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_stackscript(stackscript_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_stackscript(stackscript_id)
        return serialize_api_response(
            {
                "message": f"StackScript {stackscript_id} deleted successfully",
                "stackscript_id": stackscript_id,
            },
            stackscript_pb2.StackScriptDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_stackscript_delete",
        method="DELETE",
        path=f"/linode/stackscripts/{stackscript_id}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("StackScript"),
    )


async def handle_linode_stackscript_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscript_delete tool request."""
    stackscript_id, error = required_int_id(arguments, "stackscript_id")
    if stackscript_id is None:
        return error_response(error)

    two_stage = await _stackscript_delete_two_stage(arguments, cfg, stackscript_id)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_stackscript(stackscript_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_stackscript_delete",
            "DELETE",
            f"/linode/stackscripts/{stackscript_id}",
            _fetch,
        )
    if arguments.get("confirm") is not True:
        return error_response(
            "This operation is destructive. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_stackscript(stackscript_id)
        return serialize_api_response(
            {
                "message": f"StackScript {stackscript_id} deleted successfully",
                "stackscript_id": stackscript_id,
            },
            stackscript_pb2.StackScriptDeleteResponse(),
        )

    return await execute_tool(
        cfg, arguments, f"delete StackScript {stackscript_id}", _call
    )


def create_linode_stackscript_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscript_create tool."""
    return Tool(
        name="linode_stackscript_create",
        description=(
            "Creates a StackScript for deploying configured Linodes."
            " Pass dry_run=true to preview without creating."
        ),
        inputSchema=schema("linode.mcp.v1.StackScriptCreateInput"),
    ), Capability.Write


def _stackscript_create_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate create fields; return an error response or None."""
    if not arguments.get("label", ""):
        return error_response("label is required")
    images_arg: object = arguments.get("images", [])
    if not isinstance(images_arg, list) or not images_arg:
        return error_response("images must be a non-empty list")
    for image in cast("list[object]", images_arg):
        if not isinstance(image, str) or not image:
            return error_response("images must contain non-empty strings")
    if not arguments.get("script", ""):
        return error_response("script is required")
    return None


async def handle_linode_stackscript_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscript_create tool request."""
    if is_dry_run(arguments):
        validation = _stackscript_create_error(arguments)
        if validation is not None:
            return validation
        return build_dry_run_response(
            "linode_stackscript_create",
            arguments.get("environment", ""),
            "POST",
            "/linode/stackscripts",
            None,
        )

    if not arguments.get("confirm"):
        return error_response(
            "This creates a new StackScript in your account. Set confirm=true to "
            "proceed."
        )

    validation = _stackscript_create_error(arguments)
    if validation is not None:
        return validation

    label = arguments.get("label", "")
    images = cast("list[str]", arguments.get("images", []))
    script = arguments.get("script", "")

    body: dict[str, Any] = {"label": label, "images": images, "script": script}
    if arguments.get("description") is not None:
        body["description"] = arguments["description"]
    if arguments.get("is_public") is not None:
        body["is_public"] = arguments["is_public"]
    if arguments.get("rev_note") is not None:
        body["rev_note"] = arguments["rev_note"]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.post_raw("/linode/stackscripts", body)
        return serialize_api_response(
            {
                "message": (
                    f"StackScript '{raw_str(raw, 'label')}' "
                    f"(ID: {raw_int(raw, 'id')}) created successfully"
                ),
                "stackscript": raw,
            },
            stackscript_pb2.StackScriptWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create StackScript", _call)


def create_linode_stackscript_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscript_update tool."""
    return Tool(
        name="linode_stackscript_update",
        description=(
            "Updates an existing StackScript. Pass dry_run=true to preview "
            "without updating."
        ),
        inputSchema=schema("linode.mcp.v1.StackScriptUpdateInput"),
    ), Capability.Write


def _stackscript_images_error(images_arg: object) -> str | None:
    """Validate the optional images list; return an error message or None."""
    if images_arg is None:
        return None
    if not isinstance(images_arg, list) or not images_arg:
        return "images must be a non-empty list"
    for image in cast("list[object]", images_arg):
        if not isinstance(image, str) or not image:
            return "images must contain non-empty strings"
    return None


def _stackscript_update_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate update fields; return an error response or None."""
    _, id_error = required_int_id(arguments, "stackscript_id")
    if id_error:
        return error_response(id_error)
    images_error = _stackscript_images_error(arguments.get("images"))
    if images_error is not None:
        return error_response(images_error)
    for field in ("label", "script", "description", "rev_note"):
        value = arguments.get(field)
        if value is not None and not isinstance(value, str):
            return error_response(f"{field} must be a string")
    is_public = arguments.get("is_public")
    if is_public is not None and not isinstance(is_public, bool):
        return error_response("is_public must be a boolean")
    editable = ("label", "images", "script", "description", "is_public", "rev_note")
    if not any(field in arguments for field in editable):
        return error_response("at least one editable field is required")
    return None


def _stackscript_update_body(arguments: dict[str, Any]) -> dict[str, Any]:
    body: dict[str, Any] = {}
    for field in ("label", "images", "script", "description", "is_public", "rev_note"):
        if field in arguments:
            body[field] = arguments[field]
    return body


async def handle_linode_stackscript_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscript_update tool request."""
    validation = _stackscript_update_error(arguments)
    if validation is not None:
        return validation

    stackscript_id = cast("int", arguments["stackscript_id"])

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_stackscript_update",
            arguments.get("environment", ""),
            "PUT",
            f"/linode/stackscripts/{stackscript_id}",
            None,
            request_body=_stackscript_update_body(arguments),
            side_effects=["The StackScript is updated with the provided fields."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a StackScript in your account. Set confirm=true to proceed."
        )

    body = _stackscript_update_body(arguments)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.put_raw(f"/linode/stackscripts/{stackscript_id}", body)
        return serialize_api_response(
            {
                "message": (
                    f"StackScript '{raw_str(raw, 'label')}' "
                    f"(ID: {raw_int(raw, 'id')}) updated successfully"
                ),
                "stackscript": raw,
            },
            stackscript_pb2.StackScriptWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update StackScript", _call)
