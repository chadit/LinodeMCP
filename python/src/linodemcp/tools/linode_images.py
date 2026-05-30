"""Linode images list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    PARAM_DRY_RUN,
    build_dry_run_response,
    error_response,
    execute_tool,
    is_dry_run,
)

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def create_linode_images_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_list tool."""
    return Tool(
        name="linode_images_list",
        description=(
            "Lists all available Linode images (OS images and custom images) "
            "with optional filtering by type, public status, or deprecated status"
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
                "type": {
                    "type": "string",
                    "description": "Filter images by type (manual, automatic)",
                },
                "is_public": {
                    "type": "string",
                    "description": "Filter by public status (true, false)",
                },
                "deprecated": {
                    "type": "string",
                    "description": "Filter by deprecated status (true, false)",
                },
            },
        },
    ), Capability.Read


def create_linode_image_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_create tool."""
    return Tool(
        name="linode_image_create",
        description="Creates a private image from a Linode disk.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "disk_id": {
                    "type": "integer",
                    "description": "ID of the Linode disk to image (required)",
                },
                "label": {
                    "type": "string",
                    "description": "Short title for the image",
                },
                "description": {
                    "type": "string",
                    "description": "Detailed description for the image",
                },
                "cloud_init": {
                    "type": "boolean",
                    "description": "Whether the image supports cloud-init",
                },
                "tags": {
                    "type": "array",
                    "description": "Tags to apply to the image",
                    "items": {"type": "string"},
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
            "required": ["disk_id", "confirm"],
        },
    ), Capability.Write


def _image_create_disk_id_error(disk_id: Any) -> str | None:
    """Validate the disk_id arg; return an error message or None."""
    if not isinstance(disk_id, int) or isinstance(disk_id, bool) or disk_id <= 0:
        return "disk_id must be a positive integer"
    return None


def _image_create_tags(tags_arg: Any) -> tuple[list[str] | None, str | None]:
    """Parse+validate the tags arg; return (tags, error message)."""
    if tags_arg is None:
        return None, None
    if not isinstance(tags_arg, list):
        return None, "tags must be a list of strings"
    tag_values = cast("list[object]", tags_arg)
    tags: list[str] = []
    for tag in tag_values:
        if not isinstance(tag, str) or not tag:
            return None, "tags must contain non-empty strings"
        tags.append(tag)
    return tags, None


async def handle_linode_image_create(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_create tool request."""
    disk_id = arguments.get("disk_id")

    if is_dry_run(arguments):
        disk_err = _image_create_disk_id_error(disk_id)
        if disk_err is not None:
            return error_response(disk_err)
        return build_dry_run_response(
            "linode_image_create",
            arguments.get("environment", ""),
            "POST",
            "/images",
            None,
        )

    if not arguments.get("confirm"):
        return error_response("This creates an image. Set confirm=true to proceed.")

    disk_err = _image_create_disk_id_error(disk_id)
    if disk_err is not None:
        return error_response(disk_err)

    tags, tags_err = _image_create_tags(arguments.get("tags"))
    if tags_err is not None:
        return error_response(tags_err)

    disk_id_int = cast("int", disk_id)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        image = await client.create_image(
            disk_id=disk_id_int,
            label=arguments.get("label"),
            description=arguments.get("description"),
            cloud_init=arguments.get("cloud_init"),
            tags=tags,
        )
        return {
            "message": f"Image '{image.label}' ({image.id}) created successfully",
            "image": {
                "id": image.id,
                "label": image.label,
                "description": image.description,
                "type": image.type,
                "status": image.status,
                "size": image.size,
                "is_public": image.is_public,
                "created": image.created,
            },
        }

    return await execute_tool(cfg, arguments, "create Linode image", _call)


async def handle_linode_images_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_list tool request."""
    type_filter: str = arguments.get("type", "")
    is_public_filter: str | bool = arguments.get("is_public", "")
    deprecated_filter: str = arguments.get("deprecated", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        images = await client.list_images()

        if type_filter:
            images = [i for i in images if i.type.lower() == type_filter.lower()]

        if is_public_filter:
            want_public = (
                is_public_filter.lower() == "true"
                if isinstance(is_public_filter, str)
                else is_public_filter
            )
            images = [i for i in images if i.is_public == want_public]

        if deprecated_filter:
            want_deprecated = deprecated_filter.lower() == "true"
            images = [i for i in images if i.deprecated == want_deprecated]

        images_data = [
            {
                "id": i.id,
                "label": i.label,
                "type": i.type,
                "is_public": i.is_public,
                "deprecated": i.deprecated,
                "size": i.size,
                "vendor": i.vendor,
                "created": i.created,
            }
            for i in images
        ]

        response: dict[str, Any] = {
            "count": len(images),
            "images": images_data,
        }

        filters: list[str] = []
        if type_filter:
            filters.append(f"type={type_filter}")
        if is_public_filter:
            filters.append(f"is_public={is_public_filter}")
        if deprecated_filter:
            filters.append(f"deprecated={deprecated_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode images", _call)
