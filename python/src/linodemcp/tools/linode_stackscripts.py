from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DESCRIPTION_TRUNCATE_LIMIT,
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_stackscripts_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscripts_list tool."""
    return Tool(
        name="linode_stackscripts_list",
        description=(
            "Lists StackScripts. By default returns your own StackScripts. "
            "Can filter by public status, ownership, or label."
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
                "is_public": {
                    "type": "string",
                    "description": "Filter by public status (true, false)",
                },
                "mine": {
                    "type": "string",
                    "description": (
                        "Filter by ownership - only your own StackScripts (true, false)"
                    ),
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter StackScripts by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_stackscripts_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscripts_list tool request."""
    is_public_filter = arguments.get("is_public", "")
    mine_filter = arguments.get("mine", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        scripts = await client.list_stackscripts()

        if is_public_filter:
            want_public = is_public_filter.lower() == "true"
            scripts = [s for s in scripts if s.is_public == want_public]

        if mine_filter:
            want_mine = mine_filter.lower() == "true"
            scripts = [s for s in scripts if s.mine == want_mine]

        if label_contains:
            scripts = [s for s in scripts if label_contains.lower() in s.label.lower()]

        scripts_data = [
            {
                "id": s.id,
                "label": s.label,
                "username": s.username,
                "description": truncate_string(
                    s.description, DESCRIPTION_TRUNCATE_LIMIT
                ),
                "is_public": s.is_public,
                "mine": s.mine,
                "deployments_total": s.deployments_total,
                "deployments_active": s.deployments_active,
                "created": s.created,
                "updated": s.updated,
            }
            for s in scripts
        ]

        response: dict[str, Any] = {
            "count": len(scripts),
            "stackscripts": scripts_data,
        }

        filters: list[str] = []
        if is_public_filter:
            filters.append(f"is_public={is_public_filter}")
        if mine_filter:
            filters.append(f"mine={mine_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve StackScripts", _call)


def create_linode_stackscript_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_stackscript_create tool."""
    return Tool(
        name="linode_stackscript_create",
        description=(
            "Creates a StackScript for deploying configured Linodes."
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
                    "description": "Display label for the StackScript (required)",
                },
                "images": {
                    "type": "array",
                    "description": (
                        "Image IDs deployable with this StackScript (required)"
                    ),
                    "items": {"type": "string"},
                },
                "script": {
                    "type": "string",
                    "description": "Script executed during provisioning (required)",
                },
                "description": {
                    "type": "string",
                    "description": "Description for the StackScript",
                },
                "is_public": {
                    "type": "boolean",
                    "description": "Whether other users can use this StackScript",
                },
                "rev_note": {
                    "type": "string",
                    "description": "Notes for this StackScript revision",
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
            "required": ["label", "images", "script", "confirm"],
        },
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
            "This creates a StackScript. Set confirm=true to proceed."
        )

    validation = _stackscript_create_error(arguments)
    if validation is not None:
        return validation

    label = arguments.get("label", "")
    images = cast("list[str]", arguments.get("images", []))
    script = arguments.get("script", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        stackscript = await client.create_stackscript(
            label=label,
            images=images,
            script=script,
            description=arguments.get("description"),
            is_public=arguments.get("is_public"),
            rev_note=arguments.get("rev_note"),
        )
        return {
            "message": (
                f"StackScript '{stackscript.label}' "
                f"(ID: {stackscript.id}) created successfully"
            ),
            "stackscript": {
                "id": stackscript.id,
                "label": stackscript.label,
                "username": stackscript.username,
                "description": truncate_string(
                    stackscript.description, DESCRIPTION_TRUNCATE_LIMIT
                ),
                "images": stackscript.images,
                "is_public": stackscript.is_public,
                "mine": stackscript.mine,
                "created": stackscript.created,
                "updated": stackscript.updated,
            },
        }

    return await execute_tool(cfg, arguments, "create StackScript", _call)


def truncate_string(value: str, limit: int) -> str:
    """Truncate a string with ellipsis if it exceeds the limit."""
    if len(value) > limit:
        return value[:limit] + "..."
    return value
