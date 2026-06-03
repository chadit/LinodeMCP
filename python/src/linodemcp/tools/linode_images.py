"""Linode images list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast
from urllib.parse import quote
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


def create_linode_images_sharegroup_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_delete tool."""
    return Tool(
        name="linode_images_sharegroup_delete",
        description="Deletes a single image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "description": "Image share group UUID (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["sharegroup_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_image_sharegroup_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_create tool."""
    return Tool(
        name="linode_image_sharegroup_create",
        description="Creates a share group for sharing images with other users.",
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
                    "description": "Label for the image share group (required)",
                },
                "description": {
                    "type": "string",
                    "description": "Detailed description for the image share group",
                },
                "images": {
                    "type": "array",
                    "description": "Images to add to the share group",
                    "items": {
                        "type": "object",
                        "properties": {
                            "id": {"type": "string"},
                            "label": {"type": "string"},
                            "description": {"type": "string"},
                        },
                        "required": ["id"],
                    },
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


def create_linode_images_sharegroup_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_get tool."""
    return Tool(
        name="linode_images_sharegroup_get",
        description="Gets a single image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "description": "Image share group UUID (required)",
                },
            },
            "required": ["sharegroup_id"],
        },
    ), Capability.Read


def create_linode_images_sharegroup_images_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_images_list tool."""
    return Tool(
        name="linode_images_sharegroup_images_list",
        description="Lists images available in an image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "pattern": (
                        "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-"
                        "[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-"
                        "[0-9a-fA-F]{12}$"
                    ),
                    "description": "Image share group UUID (required)",
                },
            },
            "required": ["sharegroup_id"],
        },
    ), Capability.Read


def create_linode_images_sharegroup_members_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_members_list tool."""
    return Tool(
        name="linode_images_sharegroup_members_list",
        description="Lists members of an image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "pattern": (
                        "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-"
                        "[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-"
                        "[0-9a-fA-F]{12}$"
                    ),
                    "description": "Image share group UUID (required)",
                },
            },
            "required": ["sharegroup_id"],
        },
    ), Capability.Read


def create_linode_images_sharegroup_member_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_member_token_get tool."""
    return Tool(
        name="linode_images_sharegroup_member_token_get",
        description="Gets a membership token from an image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "pattern": (
                        "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-"
                        "[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-"
                        "[0-9a-fA-F]{12}$"
                    ),
                    "description": "Image share group UUID (required)",
                },
                "token_uuid": {
                    "type": "string",
                    "pattern": (
                        "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-"
                        "[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-"
                        "[0-9a-fA-F]{12}$"
                    ),
                    "description": "Membership token UUID (required)",
                },
            },
            "required": ["sharegroup_id", "token_uuid"],
        },
    ), Capability.Read


def create_linode_images_sharegroup_members_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_members_add tool."""
    return Tool(
        name="linode_images_sharegroup_members_add",
        description="Adds members to an image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "pattern": (
                        "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-"
                        "[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-"
                        "[0-9a-fA-F]{12}$"
                    ),
                    "description": "Image share group UUID (required)",
                },
                "label": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Label for the member being added (required)",
                },
                "token": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Share group token for the member (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["sharegroup_id", "label", "token", "confirm"],
        },
    ), Capability.Write


def create_linode_images_sharegroup_image_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_image_delete tool."""
    # The shared-image route uses sharegroup-id-path.yaml, a numeric ID,
    # unlike neighboring membership/token routes that use UUIDs.
    return Tool(
        name="linode_images_sharegroup_image_delete",
        description="Revokes access to one shared image from an image share group.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Image share group numeric ID (required)",
                },
                "image_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "Shared image numeric ID, for example 1234 (required)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["sharegroup_id", "image_id", "confirm"],
        },
    ), Capability.Destroy


def create_linode_images_sharegroup_image_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_image_update tool."""
    # The shared-image route uses sharegroup-id-path.yaml, a numeric ID,
    # unlike neighboring membership/token routes that use UUIDs.
    return Tool(
        name="linode_images_sharegroup_image_update",
        description=(
            "Updates a shared image label or description by share group and image ID."
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
                "sharegroup_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Image share group numeric ID (required)",
                },
                "image_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "Shared image numeric ID, for example 1234 (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "minLength": 1,
                    "description": "New non-empty label for the shared image",
                },
                "description": {
                    "type": "string",
                    "minLength": 1,
                    "description": "New non-empty description for the shared image",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["sharegroup_id", "image_id", "confirm"],
            "anyOf": [{"required": ["label"]}, {"required": ["description"]}],
        },
    ), Capability.Write


def create_linode_images_sharegroup_images_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_images_add tool."""
    return Tool(
        name="linode_images_sharegroup_images_add",
        description="Adds images to an image share group by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "pattern": (
                        "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-"
                        "[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-"
                        "[0-9a-fA-F]{12}$"
                    ),
                    "description": "Image share group UUID (required)",
                },
                "images": {
                    "type": "array",
                    "minItems": 1,
                    "description": "Images to add to the share group",
                    "items": {
                        "type": "object",
                        "properties": {
                            "id": {"type": "string"},
                            "label": {"type": "string"},
                            "description": {"type": "string"},
                        },
                        "required": ["id"],
                    },
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["sharegroup_id", "images", "confirm"],
        },
    ), Capability.Write


def create_linode_images_sharegroup_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroup_update tool."""
    return Tool(
        name="linode_images_sharegroup_update",
        description="Updates an image share group label or description by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "sharegroup_id": {
                    "type": "string",
                    "description": "Image share group UUID (required)",
                },
                "label": {
                    "type": "string",
                    "minLength": 1,
                    "description": "New non-empty label for the share group",
                },
                "description": {
                    "type": "string",
                    "minLength": 1,
                    "description": "New non-empty description for the share group",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["sharegroup_id", "confirm"],
            "anyOf": [{"required": ["label"]}, {"required": ["description"]}],
        },
    ), Capability.Write


def create_linode_images_sharegroups_token_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroups_token_delete tool."""
    return Tool(
        name="linode_images_sharegroups_token_delete",
        description="Deletes an image share group token by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "token_uuid": {
                    "type": "string",
                    "description": "Image share group token UUID (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["token_uuid", "confirm"],
        },
    ), Capability.Destroy


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


def create_linode_images_sharegroups_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroups_token_get tool."""
    return Tool(
        name="linode_images_sharegroups_token_get",
        description="Gets a single image share group token by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "token_uuid": {
                    "type": "string",
                    "description": "Image share group token UUID (required)",
                },
            },
            "required": ["token_uuid"],
        },
    ), Capability.Read


def create_linode_images_sharegroups_token_sharegroup_get_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_images_sharegroups_token_sharegroup_get tool."""
    return Tool(
        name="linode_images_sharegroups_token_sharegroup_get",
        description="Gets the image share group associated with a token UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "token_uuid": {
                    "type": "string",
                    "description": "Image share group token UUID (required)",
                },
            },
            "required": ["token_uuid"],
        },
    ), Capability.Read


def create_linode_images_sharegroups_token_sharegroup_images_list_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_images_sharegroups_token_sharegroup_images_list tool."""
    return Tool(
        name="linode_images_sharegroups_token_sharegroup_images_list",
        description="Lists images available through an image share group token UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "token_uuid": {
                    "type": "string",
                    "description": "Image share group token UUID (required)",
                },
            },
            "required": ["token_uuid"],
        },
    ), Capability.Read


def create_linode_images_sharegroups_token_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_images_sharegroups_token_update tool."""
    return Tool(
        name="linode_images_sharegroups_token_update",
        description="Updates an image share group token label by UUID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "token_uuid": {
                    "type": "string",
                    "description": "Image share group token UUID (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New non-empty label for the token (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["token_uuid", "label", "confirm"],
        },
    ), Capability.Write


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


def _image_sharegroup_image_payload(
    image_arg: object,
) -> tuple[dict[str, str] | None, str | None]:
    """Build and validate one image object for a share group create body."""
    if not isinstance(image_arg, dict):
        return None, "images must contain objects"

    raw_image = cast("dict[str, object]", image_arg)
    image_id = raw_image.get("id")
    if not isinstance(image_id, str) or not image_id.strip():
        return None, "images[].id must be a non-empty string"

    image: dict[str, str] = {"id": image_id}
    for field in ("label", "description"):
        value = raw_image.get(field)
        if value is not None:
            if not isinstance(value, str):
                return None, f"images[].{field} must be a string"
            image[field] = value
    return image, None


def _image_sharegroup_images_add_payload(
    images_arg: Any,
) -> tuple[list[dict[str, str]] | None, str | None]:
    """Build and validate the add-images request body."""
    if not isinstance(images_arg, list) or not images_arg:
        return None, "images must be a non-empty list of image objects"

    images: list[dict[str, str]] = []
    for image_arg in cast("list[object]", images_arg):
        image, image_err = _image_sharegroup_image_payload(image_arg)
        if image_err is not None:
            return None, image_err
        images.append(cast("dict[str, str]", image))
    return images, None


def _image_sharegroup_member_add_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, str] | None, str | None]:
    """Build and validate the add-members request body."""
    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return None, "label must be a non-empty string"

    token = arguments.get("token")
    if not isinstance(token, str) or not token.strip():
        return None, "token must be a non-empty string"

    return {"label": label, "token": token}, None


def _image_sharegroup_create_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    """Build and validate the image share group create request body."""
    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return None, "label must be a non-empty string"

    payload: dict[str, Any] = {"label": label}

    description = arguments.get("description")
    if description is not None:
        if not isinstance(description, str):
            return None, "description must be a string"
        payload["description"] = description

    images_arg = arguments.get("images")
    if images_arg is None:
        return payload, None
    if not isinstance(images_arg, list):
        return None, "images must be a list of image objects"

    images: list[dict[str, str]] = []
    for image_arg in cast("list[object]", images_arg):
        image, image_err = _image_sharegroup_image_payload(image_arg)
        if image_err is not None:
            return None, image_err
        images.append(cast("dict[str, str]", image))

    payload["images"] = images
    return payload, None


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


async def handle_linode_image_sharegroup_create(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_create tool request."""
    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an image share group. Set confirm=true to proceed."
        )

    payload, payload_err = _image_sharegroup_create_payload(arguments)
    if payload_err is not None:
        return error_response(payload_err)
    payload = cast("dict[str, Any]", payload)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_create",
            arguments.get("environment", ""),
            "POST",
            "/images/sharegroups",
            None,
            side_effects=[
                f"A new image share group labeled {payload['label']!r} will be created."
            ],
            request_body=payload,
        )

    images = cast("list[dict[str, str]] | None", payload.get("images"))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.create_image_sharegroup(
            label=cast("str", payload["label"]),
            description=cast("str | None", payload.get("description")),
            images=images,
        )
        return {
            "message": f"Image share group '{sharegroup.get('label')}' created",
            "sharegroup": sharegroup,
        }

    return await execute_tool(cfg, arguments, "create image share group", _call)


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


async def handle_linode_images_sharegroups_token_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_token_delete tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes an image share group token. Set confirm=true to proceed."
        )

    token_uuid_str = cast("str", token_uuid).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroups_token_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/images/sharegroups/tokens/{quote(token_uuid_str, safe='')}",
            None,
            side_effects=["The image share group token will be deleted."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup_token(token_uuid=token_uuid_str)
        return {"message": "Image share group token deleted"}

    return await execute_tool(cfg, arguments, "delete image share group token", _call)


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


def _image_sharegroup_token_uuid_error(value: Any, name: str) -> str | None:
    """Validate an image share group UUID argument."""
    if not isinstance(value, str) or not value.strip():
        return f"{name} must be a non-empty string"
    try:
        UUID(value.strip())
    except ValueError:
        return f"{name} must be a valid UUID"
    return None


def _image_sharegroup_id_error(value: Any) -> str | None:
    """Validate a sharegroup_id arg."""
    return _image_sharegroup_token_uuid_error(value, "sharegroup_id")


def _image_sharegroup_numeric_id_error(value: Any) -> str | None:
    """Validate the numeric sharegroup_id arg for shared image routes.

    OpenAPI sharegroup-id-path.yaml documents this path parameter as the
    share group's numeric ID, not the UUID used by token-management routes.
    """
    if type(value) is not int or value <= 0:
        return "sharegroup_id must be a positive integer"
    return None


def _image_sharegroup_image_id_error(value: Any) -> str | None:
    """Validate the shared image numeric ID path arg."""
    if type(value) is not int or value <= 0:
        return "image_id must be a positive integer"
    return None


def _image_sharegroup_update_body(
    label: Any, description: Any
) -> tuple[dict[str, str], str | None]:
    """Validate update body fields."""
    body: dict[str, str] = {}
    if label is not None:
        if not isinstance(label, str) or not label.strip():
            return {}, "label must be a non-empty string when provided"
        body["label"] = label.strip()
    if description is not None:
        if not isinstance(description, str) or not description.strip():
            return {}, "description must be a non-empty string when provided"
        body["description"] = description.strip()
    if not body:
        return {}, "at least one of label or description must be provided"
    return body, None


def _image_sharegroup_token_create_uuid_error(value: Any) -> str | None:
    """Validate the valid_for_sharegroup_uuid arg."""
    return _image_sharegroup_token_uuid_error(value, "valid_for_sharegroup_uuid")


def _image_sharegroup_token_create_label_error(value: Any) -> str | None:
    """Validate the optional label arg."""
    if value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        return "label must be a non-empty string when provided"
    return None


def _image_sharegroup_token_update_label_error(value: Any) -> str | None:
    """Validate the required label arg."""
    if not isinstance(value, str) or not value.strip():
        return "label must be a non-empty string"
    return None


async def handle_linode_images_sharegroup_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_delete tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes an image share group. Set confirm=true to proceed."
        )

    sharegroup_id_str = cast("str", sharegroup_id).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroup_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}",
            None,
            side_effects=["The image share group will be deleted."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup(sharegroup_id_str)
        return {"message": "Image share group deleted"}

    return await execute_tool(cfg, arguments, "delete image share group", _call)


async def handle_linode_images_sharegroup_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_get tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    sharegroup_id_str = cast("str", sharegroup_id).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.get_image_sharegroup(sharegroup_id_str)
        return {
            "message": "Image share group retrieved",
            "sharegroup": sharegroup,
        }

    return await execute_tool(cfg, arguments, "get image share group", _call)


async def handle_linode_images_sharegroup_images_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_images_list request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    sharegroup_id_str = cast("str", sharegroup_id).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_image_sharegroup_images(sharegroup_id_str)
        images = data.get("data", [])
        return {
            "message": "Image share group images retrieved",
            "count": len(images),
            "images": images,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(cfg, arguments, "list image share group images", _call)


async def handle_linode_images_sharegroup_members_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_members_list request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    sharegroup_id_str = cast("str", sharegroup_id).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_image_sharegroup_members(sharegroup_id_str)
        members = data.get("data", [])
        return {
            "message": "Image share group members retrieved",
            "count": len(members),
            "members": members,
            "page": data.get("page", 1),
            "pages": data.get("pages", 1),
            "results": data.get("results", len(members)),
        }

    return await execute_tool(cfg, arguments, "list image share group members", _call)


async def handle_linode_images_sharegroup_member_token_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_member_token_get request."""
    sharegroup_id = arguments.get("sharegroup_id")
    sharegroup_error = _image_sharegroup_id_error(sharegroup_id)
    if sharegroup_error is not None:
        return error_response(sharegroup_error)

    token_uuid = arguments.get("token_uuid")
    token_error = _image_sharegroup_token_uuid_error(token_uuid, "token_uuid")
    if token_error is not None:
        return error_response(token_error)

    sharegroup_id_str = cast("str", sharegroup_id).strip()
    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.get_image_sharegroup_member_token(
            sharegroup_id_str, token_uuid_str
        )
        return {
            "message": "Image share group member token retrieved",
            "token": token,
        }

    return await execute_tool(
        cfg, arguments, "get image share group member token", _call
    )


async def handle_linode_images_sharegroup_image_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_image_delete tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    sharegroup_error = _image_sharegroup_numeric_id_error(sharegroup_id)
    if sharegroup_error is not None:
        return error_response(sharegroup_error)

    image_id = arguments.get("image_id")
    image_error = _image_sharegroup_image_id_error(image_id)
    if image_error is not None:
        return error_response(image_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This revokes shared image access. Set confirm=true to proceed."
        )

    sharegroup_id_str = str(cast("int", sharegroup_id))
    image_id_str = str(cast("int", image_id))

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroup_image_delete",
            arguments.get("environment", ""),
            "DELETE",
            (
                f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}"
                f"/images/{quote(image_id_str, safe='')}"
            ),
            current_state=None,
            side_effects=[
                "The shared image will be removed from the image share group."
            ],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup_image(sharegroup_id_str, image_id_str)
        return {"message": "Shared image access revoked"}

    return await execute_tool(
        cfg, arguments, "revoke image share group image access", _call
    )


async def handle_linode_images_sharegroup_image_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_image_update tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_numeric_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    image_id = arguments.get("image_id")
    image_id_error = _image_sharegroup_image_id_error(image_id)
    if image_id_error is not None:
        return error_response(image_id_error)

    body, body_error = _image_sharegroup_update_body(
        arguments.get("label"), arguments.get("description")
    )
    if body_error is not None:
        return error_response(body_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a shared image. Set confirm=true to proceed."
        )

    sharegroup_id_str = str(cast("int", sharegroup_id))
    image_id_str = str(cast("int", image_id))

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroup_image_update",
            arguments.get("environment", ""),
            "PUT",
            (
                f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}"
                f"/images/{quote(image_id_str, safe='')}"
            ),
            current_state=None,
            request_body=body,
            side_effects=["The shared image will be updated."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        image = await client.update_image_sharegroup_image(
            sharegroup_id_str,
            image_id_str,
            label=body.get("label"),
            description=body.get("description"),
        )
        return {
            "message": "Shared image updated",
            "image": image,
        }

    return await execute_tool(cfg, arguments, "update shared image", _call)


async def handle_linode_images_sharegroup_members_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_members_add tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    body, body_error = _image_sharegroup_member_add_payload(arguments)
    if body_error is not None:
        return error_response(body_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This adds members to an image share group. Set confirm=true to proceed."
        )

    sharegroup_id_str = cast("str", sharegroup_id).strip()
    body = cast("dict[str, str]", body)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroup_members_add",
            arguments.get("environment", ""),
            "POST",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}/members",
            current_state=None,
            request_body=body,
            side_effects=["Members will be added to the image share group."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.add_members_to_image_sharegroup(
            sharegroup_id_str, label=body["label"], token=body["token"]
        )
        return {
            "message": "Members added to image share group",
            "result": result,
        }

    return await execute_tool(cfg, arguments, "add members to image share group", _call)


async def handle_linode_images_sharegroup_images_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_images_add tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    images, images_error = _image_sharegroup_images_add_payload(arguments.get("images"))
    if images_error is not None:
        return error_response(images_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This adds images to an image share group. Set confirm=true to proceed."
        )

    sharegroup_id_str = cast("str", sharegroup_id).strip()
    images = cast("list[dict[str, str]]", images)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroup_images_add",
            arguments.get("environment", ""),
            "POST",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}/images",
            current_state=None,
            request_body={"images": images},
            side_effects=["Images will be added to the image share group."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.add_image_sharegroup_images(sharegroup_id_str, images)
        return {
            "message": "Images added to image share group",
            "result": result,
        }

    return await execute_tool(cfg, arguments, "add images to image share group", _call)


async def handle_linode_images_sharegroup_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroup_update tool request."""
    sharegroup_id = arguments.get("sharegroup_id")
    id_error = _image_sharegroup_id_error(sharegroup_id)
    if id_error is not None:
        return error_response(id_error)

    body, body_error = _image_sharegroup_update_body(
        arguments.get("label"), arguments.get("description")
    )
    if body_error is not None:
        return error_response(body_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an image share group. Set confirm=true to proceed."
        )

    if not isinstance(sharegroup_id, str):
        return error_response("sharegroup_id must be a non-empty string")
    sharegroup_id_str = sharegroup_id.strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroup_update",
            arguments.get("environment", ""),
            "PUT",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}",
            current_state=None,
            request_body=body,
            side_effects=["The image share group will be updated."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.update_image_sharegroup(
            sharegroup_id_str,
            label=body.get("label"),
            description=body.get("description"),
        )
        return {
            "message": "Image share group updated",
            "sharegroup": sharegroup,
        }

    return await execute_tool(cfg, arguments, "update image share group", _call)


async def handle_linode_images_sharegroups_token_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_token_get tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.get_image_sharegroup_token(token_uuid_str)
        return {
            "message": "Image share group token retrieved",
            "token": token,
        }

    return await execute_tool(cfg, arguments, "get image share group token", _call)


async def handle_linode_images_sharegroups_token_sharegroup_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_token_sharegroup_get tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.get_image_sharegroup_by_token(token_uuid_str)
        return {
            "message": "Image share group retrieved",
            "sharegroup": sharegroup,
        }

    return await execute_tool(cfg, arguments, "get image share group by token", _call)


async def handle_linode_images_sharegroups_token_sharegroup_images_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_token_sharegroup_images_list request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.list_image_sharegroup_images_by_token(token_uuid_str)
        images = data.get("data", [])
        return {
            "message": "Image share group images retrieved",
            "count": len(images),
            "images": images,
            "page": data.get("page"),
            "pages": data.get("pages"),
            "results": data.get("results"),
        }

    return await execute_tool(
        cfg, arguments, "list image share group images by token", _call
    )


async def handle_linode_images_sharegroups_token_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_images_sharegroups_token_update tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    label = arguments.get("label")
    label_error = _image_sharegroup_token_update_label_error(label)
    if label_error is not None:
        return error_response(label_error)

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an image share group token. Set confirm=true to proceed."
        )

    token_uuid_str = cast("str", token_uuid).strip()
    label_str = cast("str", label).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_images_sharegroups_token_update",
            arguments.get("environment", ""),
            "PUT",
            f"/images/sharegroups/tokens/{quote(token_uuid_str, safe='')}",
            None,
            request_body={"label": label_str},
            side_effects=["The image share group token label will be updated."],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.update_image_sharegroup_token(
            token_uuid=token_uuid_str, label=label_str
        )
        return {
            "message": "Image share group token updated",
            "token": token,
        }

    return await execute_tool(cfg, arguments, "update image share group token", _call)


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
