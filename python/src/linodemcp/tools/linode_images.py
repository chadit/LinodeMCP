"""Linode images list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast
from uuid import UUID

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


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    """Parse an optional integer argument with range checks."""
    if name not in arguments or arguments[name] is None:
        return None
    value = arguments[name]
    if type(value) is not int or value < minimum:
        if maximum is None:
            raise ValueError(f"{name} must be an integer at least {minimum}")
        raise ValueError(f"{name} must be an integer between {minimum} and {maximum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be an integer between {minimum} and {maximum}")
    return value


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


def create_linode_images_sharegroups_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroups_list tool."""
    return Tool(
        name="linode_images_sharegroups_list",
        description="Lists image share groups available to the account.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


def create_linode_images_sharegroups_tokens_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroups_tokens_list tool."""
    return Tool(
        name="linode_images_sharegroups_tokens_list",
        description="Lists image share group tokens for the user.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    ), Capability.Read


def create_linode_images_sharegroups_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroups_token_create tool."""
    return Tool(
        name="linode_images_sharegroups_token_create",
        description="Creates a token for sharing images with another share group.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "valid_for_sharegroup_uuid": {
                    "type": "string",
                    "description": "Share group UUID the token is valid for (required)",
                },
                "label": {
                    "type": "string",
                    "description": "Optional label for the share group token",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["valid_for_sharegroup_uuid", "confirm"],
        },
    ), Capability.Write


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
        img_label = arguments.get("label")
        effect = f"A new image will be captured from disk {disk_id}"
        if img_label:
            effect += f" and labeled {img_label!r}"
        return build_dry_run_response(
            "linode_image_create",
            arguments.get("environment", ""),
            "POST",
            "/images",
            None,
            side_effects=[f"{effect}."],
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


async def handle_linode_images_sharegroups_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_image_sharegroups(page=page, page_size=page_size)
        sharegroups = data.get("data", [])
        return {
            "message": "Image share groups listed",
            "count": len(sharegroups),
            "sharegroups": sharegroups,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list image share groups", _call)


async def handle_linode_images_sharegroups_tokens_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_tokens_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_image_sharegroup_tokens()
        tokens = data.get("data", [])
        return {
            "message": "Image share group tokens listed",
            "count": len(tokens),
            "tokens": tokens,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list image share group tokens", _call)


def _image_sharegroup_token_create_uuid_error(value: Any) -> str | None:
    """Validate the valid_for_sharegroup_uuid arg."""
    if not isinstance(value, str) or not value.strip():
        return "valid_for_sharegroup_uuid must be a non-empty string"
    try:
        UUID(value.strip())
    except ValueError:
        return "valid_for_sharegroup_uuid must be a valid UUID"
    return None


def _image_sharegroup_token_create_label_error(value: Any) -> str | None:
    """Validate the optional label arg."""
    if value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        return "label must be a non-empty string when provided"
    return None


async def handle_linode_images_sharegroups_token_create(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_token_create tool request."""
    uuid_value = arguments.get("valid_for_sharegroup_uuid")
    uuid_error = _image_sharegroup_token_create_uuid_error(uuid_value)
    if uuid_error is not None:
        return error_response(uuid_error)

    label = arguments.get("label")
    label_error = _image_sharegroup_token_create_label_error(label)
    if label_error is not None:
        return error_response(label_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an image share group token. Set confirm=true to proceed."
        )

    uuid_str = cast("str", uuid_value).strip()
    label_str = cast("str", label).strip() if label is not None else None

    if is_dry_run(arguments):
        request_body: dict[str, Any] = {"valid_for_sharegroup_uuid": uuid_str}
        if label_str is not None:
            request_body["label"] = label_str
        return build_dry_run_response(
            "linode_images_sharegroups_token_create",
            arguments.get("environment", ""),
            "POST",
            "/images/sharegroups/tokens",
            None,
            request_body=request_body,
            side_effects=[
                (
                    "A new image share group token will be created for "
                    "the target share group."
                )
            ],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.create_image_sharegroup_token(
            valid_for_sharegroup_uuid=uuid_str, label=label_str
        )
        return {
            "message": "Image share group token created",
            "token": token,
        }

    return await execute_tool(cfg, arguments, "create image share group token", _call)


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
