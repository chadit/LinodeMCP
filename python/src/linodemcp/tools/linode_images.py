"""Linode images list tool."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast
from urllib.parse import quote
from uuid import UUID

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    image_pb2,
    image_sharegroup_member_pb2,
    image_sharegroup_pb2,
    image_sharegroup_token_pb2,
)
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    required_int_id,
)
from linodemcp.tools.proto_response import (
    raw_str,
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


_IMAGE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]+/[A-Za-z0-9._-]+$")
_SHARED_IMAGE_ID_PATTERN = re.compile(r"^shared/[1-9]\d*$")


def create_linode_image_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_list tool."""
    return Tool(
        name="linode_image_list",
        description=(
            "Lists all available Linode images (OS images and custom images) "
            "with optional filtering by type, public status, or deprecated status"
        ),
        inputSchema=schema("linode.mcp.v1.ImageListInput"),
    ), Capability.Read


def create_linode_image_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_get tool."""
    return Tool(
        name="linode_image_get",
        description="Gets a single Linode image by ID.",
        inputSchema=schema("linode.mcp.v1.ImageGetInput"),
    ), Capability.Read


def create_linode_image_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_delete tool."""
    return Tool(
        name="linode_image_delete",
        description="Deletes a private Linode image by ID." + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.ImageDeleteInput"),
    ), Capability.Destroy


def create_linode_image_sharegroup_by_image_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_by_image_list tool."""
    return Tool(
        name="linode_image_sharegroup_by_image_list",
        description="Lists share groups for a Linode image.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupByImageListInput"),
    ), Capability.Read


def create_linode_image_sharegroup_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_list tool."""
    return Tool(
        name="linode_image_sharegroup_list",
        description="Lists image share groups available to the account.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupListInput"),
    ), Capability.Read


def create_linode_image_sharegroup_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_delete tool."""
    return Tool(
        name="linode_image_sharegroup_delete",
        description="Deletes a single image share group by UUID." + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.ImageShareGroupDeleteInput"),
    ), Capability.Destroy


def create_linode_image_sharegroup_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_create tool."""
    return Tool(
        name="linode_image_sharegroup_create",
        description="Creates a share group for sharing images with other users.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupCreateInput"),
    ), Capability.Write


def create_linode_image_sharegroup_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_get tool."""
    return Tool(
        name="linode_image_sharegroup_get",
        description="Gets a single image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupGetInput"),
    ), Capability.Read


def create_linode_image_sharegroup_image_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_image_list tool."""
    return Tool(
        name="linode_image_sharegroup_image_list",
        description="Lists images available in an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupImageListInput"),
    ), Capability.Read


def create_linode_image_sharegroup_member_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_member_list tool."""
    return Tool(
        name="linode_image_sharegroup_member_list",
        description="Lists members of an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupMemberListInput"),
    ), Capability.Read


def create_linode_image_sharegroup_member_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_member_token_get tool."""
    return Tool(
        name="linode_image_sharegroup_member_token_get",
        description="Gets a membership token from an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupMemberTokenGetInput"),
    ), Capability.Read


def create_linode_image_sharegroup_member_token_delete_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_image_sharegroup_member_token_delete tool."""
    return Tool(
        name="linode_image_sharegroup_member_token_delete",
        description="Revokes a membership token from an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupMemberTokenDeleteInput"),
    ), Capability.Destroy


def create_linode_image_sharegroup_member_token_update_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_image_sharegroup_member_token_update tool."""
    return Tool(
        name="linode_image_sharegroup_member_token_update",
        description="Updates a membership token label in an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupMemberTokenUpdateInput"),
    ), Capability.Write


def create_linode_image_sharegroup_member_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_member_add tool."""
    return Tool(
        name="linode_image_sharegroup_member_add",
        description="Adds members to an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupMemberAddInput"),
    ), Capability.Write


def create_linode_image_sharegroup_image_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_image_delete tool."""
    # The shared-image route uses sharegroup-id-path.yaml, a numeric ID,
    # unlike neighboring membership/token routes that use UUIDs.
    return Tool(
        name="linode_image_sharegroup_image_delete",
        description="Revokes access to one shared image from an image share group.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupImageDeleteInput"),
    ), Capability.Destroy


def create_linode_image_sharegroup_image_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_image_update tool."""
    # The shared-image route uses sharegroup-id-path.yaml, a numeric ID,
    # unlike neighboring membership/token routes that use UUIDs.
    return Tool(
        name="linode_image_sharegroup_image_update",
        description=(
            "Updates a shared image label or description by share group and image ID."
        ),
        inputSchema=schema("linode.mcp.v1.ImageShareGroupImageUpdateInput"),
    ), Capability.Write


def create_linode_image_sharegroup_image_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_image_add tool."""
    return Tool(
        name="linode_image_sharegroup_image_add",
        description="Adds images to an image share group by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupImageAddInput"),
    ), Capability.Write


def create_linode_image_sharegroup_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_update tool."""
    return Tool(
        name="linode_image_sharegroup_update",
        description="Updates an image share group label or description by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupUpdateInput"),
    ), Capability.Write


def create_linode_image_sharegroup_token_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_token_delete tool."""
    return Tool(
        name="linode_image_sharegroup_token_delete",
        description="Deletes an image share group token by UUID." + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.ImageShareGroupTokenDeleteInput"),
    ), Capability.Destroy


def create_linode_image_sharegroup_token_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_token_list tool."""
    return Tool(
        name="linode_image_sharegroup_token_list",
        description="Lists image share group tokens for the user.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupTokenListInput"),
    ), Capability.Read


def create_linode_image_sharegroup_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_token_get tool."""
    return Tool(
        name="linode_image_sharegroup_token_get",
        description="Gets a single image share group token by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupTokenGetInput"),
    ), Capability.Read


def create_linode_image_sharegroup_by_token_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_by_token_get tool."""
    return Tool(
        name="linode_image_sharegroup_by_token_get",
        description="Gets the image share group associated with a token UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupByTokenGetInput"),
    ), Capability.Read


def create_linode_image_sharegroup_token_image_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_token_image_list tool."""
    return Tool(
        name="linode_image_sharegroup_token_image_list",
        description="Lists images available through an image share group token UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupTokenImageListInput"),
    ), Capability.Read


def create_linode_image_sharegroup_token_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_token_update tool."""
    return Tool(
        name="linode_image_sharegroup_token_update",
        description="Updates an image share group token label by UUID.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupTokenUpdateInput"),
    ), Capability.Write


def create_linode_image_sharegroup_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_sharegroup_token_create tool."""
    return Tool(
        name="linode_image_sharegroup_token_create",
        description="Creates a token for sharing images with another share group.",
        inputSchema=schema("linode.mcp.v1.ImageShareGroupTokenCreateInput"),
    ), Capability.Write


def create_linode_image_upload_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_upload tool."""
    return Tool(
        name="linode_image_upload",
        description="Creates a pending private image upload for a region.",
        inputSchema=schema("linode.mcp.v1.ImageUploadInput"),
    ), Capability.Write


def create_linode_image_replicate_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_replicate tool."""
    return Tool(
        name="linode_image_replicate",
        description="Replicates a private or public image to one or more regions.",
        inputSchema=schema("linode.mcp.v1.ImageReplicateInput"),
    ), Capability.Write


def create_linode_image_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_create tool."""
    return Tool(
        name="linode_image_create",
        description="Creates a private image from a Linode disk.",
        inputSchema=schema("linode.mcp.v1.ImageCreateInput"),
    ), Capability.Write


def create_linode_image_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_image_update tool."""
    return Tool(
        name="linode_image_update",
        description="Updates a private image label, description, or tags.",
        inputSchema=schema("linode.mcp.v1.ImageUpdateInput"),
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


def _replicate_image_id_error(value: object) -> str | None:
    """Validate image IDs before using them as path parameters."""
    if not isinstance(value, str) or not value.strip():
        return "image_id must be a non-empty string"
    image_id = value.strip()
    if ".." in image_id or "?" in image_id:
        return "image_id must not contain traversal or query separators"
    if not _IMAGE_ID_PATTERN.fullmatch(image_id):
        return "image_id must look like private/123 or linode/debian12"
    return None


def _image_regions_payload(value: object) -> tuple[list[str] | None, str | None]:
    """Validate replicate-image regions."""
    if not isinstance(value, list) or not value:
        return None, "regions must be a non-empty list of region slugs"
    region_values = cast("list[object]", value)
    regions: list[str] = []
    for region in region_values:
        if not isinstance(region, str) or not region.strip():
            return None, "regions must contain non-empty strings"
        if "/" in region or "?" in region or ".." in region:
            return None, "regions must not contain path or query separators"
        regions.append(region.strip())
    return regions, None


def _required_string_argument(
    arguments: dict[str, Any], name: str
) -> tuple[str | None, str | None]:
    """Parse a required non-empty string argument."""
    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip():
        return None, f"{name} must be a non-empty string"
    return value, None


def _optional_bool_argument(
    arguments: dict[str, Any], name: str
) -> tuple[bool | None, str | None]:
    """Parse an optional boolean argument."""
    if name not in arguments or arguments[name] is None:
        return None, None
    value = arguments[name]
    if type(value) is not bool:
        return None, f"{name} must be a boolean"
    return value, None


def _image_upload_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    """Build and validate the image upload request body."""
    label, label_err = _required_string_argument(arguments, "label")
    if label_err is not None:
        return None, label_err
    region, region_err = _required_string_argument(arguments, "region")
    if region_err is not None:
        return None, region_err

    payload: dict[str, Any] = {"label": label, "region": region}

    cloud_init, cloud_init_err = _optional_bool_argument(arguments, "cloud_init")
    if cloud_init_err is not None:
        return None, cloud_init_err
    if cloud_init is not None:
        payload["cloud_init"] = cloud_init

    description = arguments.get("description")
    if description is not None:
        if not isinstance(description, str):
            return None, "description must be a string"
        payload["description"] = description

    tags, tags_err = _image_create_tags(arguments.get("tags"))
    if tags_err is not None:
        return None, tags_err
    if tags is not None:
        payload["tags"] = tags

    return payload, None


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


def _image_update_id_error(image_id: Any) -> str | None:
    """Validate the image update path parameter."""
    if not isinstance(image_id, str) or not image_id.strip():
        return "image_id must be a non-empty string"
    if "?" in image_id or ".." in image_id:
        return "image_id must not contain query or traversal segments"
    if (
        not image_id.startswith("private/")
        or not image_id.removeprefix("private/").isdigit()
    ):
        return "image_id must match private/<numeric_id>"
    return None


def _image_update_payload(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    """Build and validate the image update request body."""
    payload: dict[str, Any] = {}

    label = arguments.get("label")
    if label is not None:
        if not isinstance(label, str) or not label.strip():
            return None, "label must be a non-empty string when provided"
        payload["label"] = label

    description = arguments.get("description")
    if description is not None:
        if not isinstance(description, str):
            return None, "description must be a string"
        payload["description"] = description

    tags, tags_err = _image_create_tags(arguments.get("tags"))
    if tags_err is not None:
        return None, tags_err
    if tags is not None:
        payload["tags"] = tags

    if not payload:
        return None, "at least one of label, description, or tags must be provided"
    return payload, None


async def handle_linode_image_sharegroup_create(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_create tool request."""
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

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an image share group. Set confirm=true to proceed."
        )

    images = cast("list[dict[str, str]] | None", payload.get("images"))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.create_image_sharegroup(
            label=cast("str", payload["label"]),
            description=cast("str | None", payload.get("description")),
            images=images,
        )
        return serialize_api_response(
            {
                "message": (
                    f"Image share group '{sharegroup.get('label', '')}' "
                    f"({sharegroup.get('id', 0)}) created successfully"
                ),
                "sharegroup": sharegroup,
            },
            image_sharegroup_pb2.ImageShareGroupWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create image share group", _call)


async def handle_linode_image_upload(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_upload tool request."""
    payload, payload_err = _image_upload_payload(arguments)
    if payload_err is not None:
        return error_response(payload_err)
    payload = cast("dict[str, Any]", payload)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_upload",
            arguments.get("environment", ""),
            "POST",
            "/images/upload",
            None,
            side_effects=[
                (
                    f"A pending image upload labeled {payload['label']!r} "
                    f"will be created in {payload['region']!r}."
                )
            ],
            request_body=payload,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates an image upload. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        upload = await client.upload_image(
            label=cast("str", payload["label"]),
            region=cast("str", payload["region"]),
            cloud_init=cast("bool | None", payload.get("cloud_init")),
            description=cast("str | None", payload.get("description")),
            tags=cast("list[str] | None", payload.get("tags")),
        )
        image = cast("dict[str, Any]", upload.get("image", {}))
        return serialize_api_response(
            {
                "message": (
                    f"Image upload '{image.get('label')}' "
                    f"({image.get('id')}) created successfully"
                ),
                "upload_to": upload.get("upload_to", ""),
                "image": image,
            },
            image_pb2.ImageUploadWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "upload Linode image", _call)


async def handle_linode_image_replicate(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_replicate tool request."""
    image_id = arguments.get("image_id")
    image_err = _replicate_image_id_error(image_id)
    if image_err is not None:
        return error_response(image_err)

    regions, regions_err = _image_regions_payload(arguments.get("regions"))
    if regions_err is not None:
        return error_response(regions_err)
    regions = cast("list[str]", regions)
    image_id = cast("str", image_id).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_replicate",
            arguments.get("environment", ""),
            "POST",
            f"/images/{quote(image_id, safe='')}/regions",
            None,
            side_effects=[
                f"Image {image_id!r} will be replicated to {', '.join(regions)}."
            ],
            request_body={"regions": regions},
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This replicates an image to the requested regions. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        image = await client.replicate_image(image_id, regions)
        replicated_id = image.get("id", "")
        return serialize_api_response(
            {
                "message": f"Image '{replicated_id}' replicated successfully",
                "image": image,
            },
            image_pb2.ImageWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "replicate Linode image", _call)


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
        return error_response(
            "This creates a private image from a Linode disk. Set confirm=true to "
            "proceed."
        )

    disk_err = _image_create_disk_id_error(disk_id)
    if disk_err is not None:
        return error_response(disk_err)

    tags, tags_err = _image_create_tags(arguments.get("tags"))
    if tags_err is not None:
        return error_response(tags_err)

    disk_id_int = cast("int", disk_id)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.create_image_raw(
            disk_id=disk_id_int,
            label=arguments.get("label"),
            description=arguments.get("description"),
            cloud_init=arguments.get("cloud_init"),
            tags=tags,
        )
        return serialize_api_response(
            {
                "message": (
                    f"Image '{raw_str(raw, 'label')}' "
                    f"({raw_str(raw, 'id')}) created successfully"
                ),
                "image": raw,
            },
            image_pb2.ImageWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Linode image", _call)


async def handle_linode_image_sharegroup_by_image_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_by_image_list tool request."""
    image_id = arguments.get("image_id")
    image_err = _image_id_error(image_id)
    if image_err is not None:
        return error_response(image_err)

    image_id_str = cast("str", image_id).strip()

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_image_sharegroups_by_image(
            image_id_str, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "image_sharegroups",
            image_sharegroup_pb2.ImageShareGroupListResponse(),
        )

    return await execute_tool(cfg, arguments, "list image share groups by image", _call)


async def handle_linode_image_sharegroup_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_image_sharegroups(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "image_sharegroups",
            image_sharegroup_pb2.ImageShareGroupListResponse(),
        )

    return await execute_tool(cfg, arguments, "list image share groups", _call)


async def _images_sharegroups_token_delete_two_stage(
    arguments: dict[str, Any], cfg: Any, token_uuid_str: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        # Resolve the token to its parent share group rather than the token
        # entity, so plan/apply never surface the token secret to the model.
        return await client.get_image_sharegroup_by_token(token_uuid_str)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup_token(token_uuid=token_uuid_str)
        return serialize_api_response(
            {"message": "Image share group token removed successfully"},
            image_sharegroup_token_pb2.ImageShareGroupTokenDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_image_sharegroup_token_delete",
        method="DELETE",
        path=f"/images/sharegroups/tokens/{quote(token_uuid_str, safe='')}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("ImageShareGroupToken"),
    )


async def handle_linode_image_sharegroup_token_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_token_delete tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    two_stage = await _images_sharegroups_token_delete_two_stage(
        arguments, cfg, token_uuid_str
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_image_sharegroup_by_token(token_uuid_str)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_image_sharegroup_token_delete",
            "DELETE",
            f"/images/sharegroups/tokens/{quote(token_uuid_str, safe='')}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "confirm=true is required to remove the share group token"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup_token(token_uuid=token_uuid_str)
        return serialize_api_response(
            {"message": "Image share group token removed successfully"},
            image_sharegroup_token_pb2.ImageShareGroupTokenDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete image share group token", _call)


async def handle_linode_image_sharegroup_token_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_token_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_image_sharegroup_tokens(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "image_sharegroup_tokens",
            image_sharegroup_token_pb2.ImageShareGroupTokenListResponse(),
        )

    return await execute_tool(cfg, arguments, "list image share group tokens", _call)


def _private_image_id_error(value: Any) -> str | None:
    """Validate a private image ID path argument."""
    if not isinstance(value, str) or not value.strip():
        return "image_id must be a non-empty string"
    image_id = value.strip()
    if not image_id.startswith("private/"):
        return "image_id must be a private image ID like private/<id>"
    suffix = image_id.removeprefix("private/")
    if not suffix or "/" in suffix or "?" in suffix or ".." in suffix:
        return "image_id must be a private image ID like private/<id>"
    return None


def _image_sharegroup_token_uuid_error(value: Any, name: str) -> str | None:
    """Validate an image share group UUID argument (lenient, stdlib UUID form).

    Used for valid_for_sharegroup_uuid, which Go validates leniently too
    (isUUIDFormat mirrors stdlib UUID()), so this stays as-is.
    """
    if not isinstance(value, str) or not value.strip():
        return f"{name} must be a non-empty string"
    try:
        UUID(value.strip())
    except ValueError:
        return f"{name} must be a valid UUID"
    return None


_TOKEN_UUID_PATTERN = re.compile(
    r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$"
)


def _image_sharegroup_token_uuid_strict_error(value: Any, name: str) -> str | None:
    """Validate a token_uuid path arg against Go's strict canonical-UUID check.

    Go's imageShareGroupTokenUUIDFromTool rejects path-unsafe values and any
    non-canonical UUID form (braces/urn/no-dash), unlike stdlib UUID(). Ported so
    token_uuid rejects the same set in both languages (strictest-wins).
    """
    if not isinstance(value, str) or not value.strip():
        return f"{name} must be a non-empty string"
    token = value.strip()
    if "/" in token or "?" in token or "#" in token or ".." in token:
        return (
            f"{name} must not contain path separators, query separators, "
            "fragments, or traversal segments"
        )
    if not _TOKEN_UUID_PATTERN.fullmatch(token):
        return f"{name} must be a UUID"
    return None


def _image_sharegroup_image_id_error(value: Any) -> str | None:
    """Validate the shared image numeric ID path arg."""
    if type(value) is not int or value <= 0:
        return "image_id must be a positive integer"
    return None


def _image_sharegroup_shared_image_id_error(value: Any) -> str | None:
    """Validate the shared image string ID path arg (for example shared/1)."""
    if not isinstance(value, str) or not _SHARED_IMAGE_ID_PATTERN.fullmatch(value):
        return "image_id must match shared/<positive integer>"
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


async def _images_sharegroup_delete_two_stage(
    arguments: dict[str, Any], cfg: Any, sharegroup_id_str: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_image_sharegroup(sharegroup_id_str)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup(sharegroup_id_str)
        return serialize_api_response(
            {"message": f"Image share group {sharegroup_id_str} removed successfully"},
            image_sharegroup_pb2.ImageShareGroupDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_image_sharegroup_delete",
        method="DELETE",
        path=f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("ImageShareGroup"),
    )


async def handle_linode_image_sharegroup_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_delete tool request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    sharegroup_id_str = str(sharegroup_id)

    two_stage = await _images_sharegroup_delete_two_stage(
        arguments, cfg, sharegroup_id_str
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_image_sharegroup(sharegroup_id_str)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_image_sharegroup_delete",
            "DELETE",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "confirm=true is required to delete the image share group"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup(sharegroup_id_str)
        return serialize_api_response(
            {"message": f"Image share group {sharegroup_id_str} removed successfully"},
            image_sharegroup_pb2.ImageShareGroupDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete image share group", _call)


async def handle_linode_image_sharegroup_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_get tool request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    sharegroup_id_str = str(sharegroup_id)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_image_sharegroup(sharegroup_id_str),
            image_sharegroup_pb2.ImageShareGroup(),
        )

    return await execute_tool(cfg, arguments, "get image share group", _call)


async def handle_linode_image_sharegroup_image_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_image_list request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    sharegroup_id_str = str(sharegroup_id)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_image_sharegroup_images(
            sharegroup_id_str, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "images",
            image_pb2.ImageListResponse(),
        )

    return await execute_tool(cfg, arguments, "list image share group images", _call)


async def handle_linode_image_sharegroup_member_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_member_list request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    sharegroup_id_str = str(sharegroup_id)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_image_sharegroup_members(
            sharegroup_id_str, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "image_sharegroup_members",
            image_sharegroup_member_pb2.ImageShareGroupMemberListResponse(),
        )

    return await execute_tool(cfg, arguments, "list image share group members", _call)


async def handle_linode_image_sharegroup_member_token_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_member_token_get request."""
    sharegroup_id, sharegroup_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(sharegroup_error)

    token_uuid = arguments.get("token_uuid")
    token_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if token_error is not None:
        return error_response(token_error)

    sharegroup_id_str = str(sharegroup_id)
    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_image_sharegroup_member_token(
                sharegroup_id_str, token_uuid_str
            ),
            image_sharegroup_member_pb2.ImageShareGroupMember(),
        )

    return await execute_tool(
        cfg, arguments, "get image share group member token", _call
    )


async def handle_linode_image_sharegroup_member_token_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_member_token_delete tool request."""
    sharegroup_id, sharegroup_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(sharegroup_error)

    token_uuid = arguments.get("token_uuid")
    token_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if token_error is not None:
        return error_response(token_error)

    sharegroup_id_str = str(sharegroup_id)
    token_uuid_str = cast("str", token_uuid).strip()

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_image_sharegroup(sharegroup_id_str)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_image_sharegroup_member_token_delete",
            "DELETE",
            (
                f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}"
                f"/members/{quote(token_uuid_str, safe='')}"
            ),
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm=true is required to revoke the member token")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup_member_token(
            sharegroup_id_str, token_uuid_str
        )
        return serialize_api_response(
            {
                "message": (
                    f"Image share group member token {token_uuid_str} "
                    f"revoked from share group {sharegroup_id_str} successfully"
                )
            },
            image_sharegroup_member_pb2.ImageShareGroupMemberTokenDeleteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "revoke image share group member token", _call
    )


async def handle_linode_image_sharegroup_member_token_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_member_token_update tool request."""
    sharegroup_id, sharegroup_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(sharegroup_error)

    token_uuid = arguments.get("token_uuid")
    token_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if token_error is not None:
        return error_response(token_error)

    label_error = _image_sharegroup_token_update_label_error(arguments.get("label"))
    if label_error is not None:
        return error_response(label_error)

    sharegroup_id_str = str(sharegroup_id)
    token_uuid_str = cast("str", token_uuid).strip()
    label = cast("str", arguments["label"]).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_member_token_update",
            arguments.get("environment", ""),
            "PUT",
            (
                f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}"
                f"/members/{quote(token_uuid_str, safe='')}"
            ),
            current_state=None,
            request_body={"label": label},
            side_effects=["The image share group member token will be updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an image share group member token. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        member = await client.update_image_sharegroup_member_token(
            sharegroup_id_str, token_uuid_str, label=label
        )
        return serialize_api_response(
            {
                "message": (
                    f"Image share group member token "
                    f"'{member.get('token_uuid')}' updated successfully"
                ),
                "member": member,
            },
            image_sharegroup_member_pb2.ImageShareGroupMemberWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "update image share group member token", _call
    )


async def handle_linode_image_sharegroup_image_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_image_delete tool request."""
    sharegroup_id, sharegroup_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(sharegroup_error)

    image_id = arguments.get("image_id")
    image_error = _image_sharegroup_image_id_error(image_id)
    if image_error is not None:
        return error_response(image_error)

    sharegroup_id_str = str(sharegroup_id)
    image_id_str = str(cast("int", image_id))

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_image_sharegroup(sharegroup_id_str)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_image_sharegroup_image_delete",
            "DELETE",
            (
                f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}"
                f"/images/{quote(image_id_str, safe='')}"
            ),
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm=true is required to remove the shared image")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image_sharegroup_image(sharegroup_id_str, image_id_str)
        return serialize_api_response(
            {
                "message": (
                    f"Shared image {image_id_str} removed from "
                    f"image share group {sharegroup_id_str} successfully"
                )
            },
            image_sharegroup_pb2.ImageShareGroupImageDeleteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "revoke image share group image access", _call
    )


async def handle_linode_image_sharegroup_image_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_image_update tool request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    image_id = arguments.get("image_id")
    image_id_error = _image_sharegroup_shared_image_id_error(image_id)
    if image_id_error is not None:
        return error_response(image_id_error)

    body, body_error = _image_sharegroup_update_body(
        arguments.get("label"), arguments.get("description")
    )
    if body_error is not None:
        return error_response(body_error)

    sharegroup_id_str = str(sharegroup_id)
    image_id_str = cast("str", image_id).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_image_update",
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

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a shared image. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        image = await client.update_image_sharegroup_image(
            sharegroup_id_str,
            image_id_str,
            label=body.get("label"),
            description=body.get("description"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Shared image '{image.get('id')}' in image share group "
                    f"{sharegroup_id_str} updated successfully"
                ),
                "image": image,
            },
            image_pb2.ImageWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update shared image", _call)


async def handle_linode_image_sharegroup_member_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_member_add tool request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    body, body_error = _image_sharegroup_member_add_payload(arguments)
    if body_error is not None:
        return error_response(body_error)

    sharegroup_id_str = str(sharegroup_id)
    body = cast("dict[str, str]", body)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_member_add",
            arguments.get("environment", ""),
            "POST",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}/members",
            current_state=None,
            request_body=body,
            side_effects=["Members will be added to the image share group."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This adds members to an image share group. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.add_members_to_image_sharegroup(
            sharegroup_id_str, label=body["label"], token=body["token"]
        )
        return serialize_api_response(
            {
                "message": (f"Added members to image share group {sharegroup_id_str}"),
                "sharegroup": sharegroup,
            },
            image_sharegroup_pb2.ImageShareGroupWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "add members to image share group", _call)


async def handle_linode_image_sharegroup_image_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_image_add tool request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    images, images_error = _image_sharegroup_images_add_payload(arguments.get("images"))
    if images_error is not None:
        return error_response(images_error)

    sharegroup_id_str = str(sharegroup_id)
    images = cast("list[dict[str, str]]", images)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_image_add",
            arguments.get("environment", ""),
            "POST",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}/images",
            current_state=None,
            request_body={"images": images},
            side_effects=["Images will be added to the image share group."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This adds images to an image share group. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        image = await client.add_image_sharegroup_images(sharegroup_id_str, images)
        return serialize_api_response(
            {
                "message": (
                    f"Added image set to image share group {sharegroup_id_str}; "
                    f"last returned image: '{image.get('id')}'"
                ),
                "image": image,
            },
            image_pb2.ImageWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "add images to image share group", _call)


async def handle_linode_image_sharegroup_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_update tool request."""
    sharegroup_id, id_error = required_int_id(arguments, "sharegroup_id")
    if sharegroup_id is None:
        return error_response(id_error)

    body, body_error = _image_sharegroup_update_body(
        arguments.get("label"), arguments.get("description")
    )
    if body_error is not None:
        return error_response(body_error)

    sharegroup_id_str = str(sharegroup_id)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_update",
            arguments.get("environment", ""),
            "PUT",
            f"/images/sharegroups/{quote(sharegroup_id_str, safe='')}",
            current_state=None,
            request_body=body,
            side_effects=["The image share group will be updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an image share group. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        sharegroup = await client.update_image_sharegroup(
            sharegroup_id_str,
            label=body.get("label"),
            description=body.get("description"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Image share group '{sharegroup.get('label', '')}' "
                    f"({sharegroup.get('id', 0)}) updated successfully"
                ),
                "sharegroup": sharegroup,
            },
            image_sharegroup_pb2.ImageShareGroupWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update image share group", _call)


async def handle_linode_image_sharegroup_token_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_token_get tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_image_sharegroup_token(token_uuid_str),
            image_sharegroup_token_pb2.ImageShareGroupToken(),
        )

    return await execute_tool(cfg, arguments, "get image share group token", _call)


async def handle_linode_image_sharegroup_by_token_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_by_token_get tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_image_sharegroup_by_token(token_uuid_str),
            image_sharegroup_pb2.ImageShareGroup(),
        )

    return await execute_tool(cfg, arguments, "get image share group by token", _call)


async def handle_linode_image_sharegroup_token_image_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_token_image_list request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    token_uuid_str = cast("str", token_uuid).strip()

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_image_sharegroup_images_by_token(
            token_uuid_str, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "images",
            image_pb2.ImageListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "list image share group images by token", _call
    )


async def handle_linode_image_sharegroup_token_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_token_update tool request."""
    token_uuid = arguments.get("token_uuid")
    uuid_error = _image_sharegroup_token_uuid_strict_error(token_uuid, "token_uuid")
    if uuid_error is not None:
        return error_response(uuid_error)

    label = arguments.get("label")
    label_error = _image_sharegroup_token_update_label_error(label)
    if label_error is not None:
        return error_response(label_error)

    token_uuid_str = cast("str", token_uuid).strip()
    label_str = cast("str", label).strip()

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_image_sharegroup_token_update",
            arguments.get("environment", ""),
            "PUT",
            f"/images/sharegroups/tokens/{quote(token_uuid_str, safe='')}",
            None,
            request_body={"label": label_str},
            side_effects=["The image share group token label will be updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates an image share group token. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.update_image_sharegroup_token(
            token_uuid=token_uuid_str, label=label_str
        )
        token_id = token.get("token_uuid", "")
        return serialize_api_response(
            {
                "message": (
                    f"Image share group token '{token_id}' updated successfully"
                ),
                "token": token,
            },
            image_sharegroup_token_pb2.ImageShareGroupTokenWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update image share group token", _call)


async def handle_linode_image_sharegroup_token_create(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_sharegroup_token_create tool request."""
    uuid_value = arguments.get("valid_for_sharegroup_uuid")
    uuid_error = _image_sharegroup_token_create_uuid_error(uuid_value)
    if uuid_error is not None:
        return error_response(uuid_error)

    label = arguments.get("label")
    label_error = _image_sharegroup_token_create_label_error(label)
    if label_error is not None:
        return error_response(label_error)

    uuid_str = cast("str", uuid_value).strip()
    label_str = cast("str", label).strip() if label is not None else None

    if is_dry_run(arguments):
        request_body: dict[str, Any] = {"valid_for_sharegroup_uuid": uuid_str}
        if label_str is not None:
            request_body["label"] = label_str
        return build_dry_run_response(
            "linode_image_sharegroup_token_create",
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

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates single-use image share group token material. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        token = await client.create_image_sharegroup_token(
            valid_for_sharegroup_uuid=uuid_str, label=label_str
        )
        token_id = token.get("token_uuid", "")
        return serialize_api_response(
            {
                "message": (
                    f"Image share group token '{token_id}' created successfully"
                ),
                "token": token,
            },
            image_sharegroup_token_pb2.ImageShareGroupTokenWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create image share group token", _call)


async def handle_linode_image_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_update tool request."""
    image_id = arguments.get("image_id")
    image_id_err = _image_update_id_error(image_id)
    if image_id_err is not None:
        return error_response(image_id_err)
    image_id = cast("str", image_id)

    payload, payload_err = _image_update_payload(arguments)
    if payload_err is not None:
        return error_response(payload_err)
    payload = cast("dict[str, Any]", payload)

    if is_dry_run(arguments):
        image_id_path = quote(image_id, safe="")
        return build_dry_run_response(
            "linode_image_update",
            arguments.get("environment", ""),
            "PUT",
            f"/images/{image_id_path}",
            None,
            side_effects=[f"Image {image_id!r} will be updated."],
            request_body=payload,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates image metadata. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.update_image_raw(
            image_id=image_id,
            label=cast("str | None", payload.get("label")),
            description=cast("str | None", payload.get("description")),
            tags=cast("list[str] | None", payload.get("tags")),
        )
        return serialize_api_response(
            {
                "message": f"Image '{raw_str(raw, 'id')}' updated successfully",
                "image": raw,
            },
            image_pb2.ImageWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode image", _call)


IMAGE_ID_PARTS = 2
IMAGE_ID_NAME_CHARS = frozenset(
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"
)


def _image_id_error(value: object) -> str | None:
    """Validate an image ID path parameter."""
    if not isinstance(value, str) or not value.strip():
        return "image_id is required"
    image_id = value.strip()
    if "?" in image_id or ".." in image_id:
        return "image_id must be a valid Linode image ID"
    parts = image_id.split("/")
    if (
        len(parts) != IMAGE_ID_PARTS
        or parts[0] not in {"linode", "private"}
        or not parts[1]
    ):
        return "image_id must be a valid Linode image ID"
    if any(ch not in IMAGE_ID_NAME_CHARS for ch in parts[1]):
        return "image_id must be a valid Linode image ID"
    return None


async def handle_linode_image_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_get tool request."""
    image_id = arguments.get("image_id")
    image_id_err = _image_id_error(image_id)
    if image_id_err is not None:
        return error_response(image_id_err)
    image_id_str = cast("str", image_id).strip()

    encoded_image_id = quote(image_id_str, safe="")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_raw(f"/images/{encoded_image_id}"),
            image_pb2.Image(),
        )

    return await execute_tool(cfg, arguments, "retrieve Linode image", _call)


async def _image_delete_two_stage(
    arguments: dict[str, Any], cfg: Any, image_id_str: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_image(image_id_str)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image(image_id_str)
        return serialize_api_response(
            {"message": f"Image {image_id_str} deleted successfully"},
            image_pb2.ImageDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_image_delete",
        method="DELETE",
        path=f"/images/{image_id_str}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Image"),
    )


async def handle_linode_image_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_delete tool request."""
    image_id = arguments.get("image_id")
    image_id_error = _private_image_id_error(image_id)
    if image_id_error is not None:
        return error_response(image_id_error)

    image_id_str = cast("str", image_id).strip()

    two_stage = await _image_delete_two_stage(arguments, cfg, image_id_str)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_image(image_id_str)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_image_delete",
            "DELETE",
            f"/images/{image_id_str}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response("confirm=true is required to delete the image")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_image(image_id_str)
        return serialize_api_response(
            {"message": f"Image {image_id_str} deleted successfully"},
            image_pb2.ImageDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete private image", _call)


async def handle_linode_image_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_image_list tool request."""
    type_filter = str(arguments.get("type", ""))
    is_public_filter = str(arguments.get("is_public", ""))
    deprecated_filter = str(arguments.get("deprecated", ""))

    def _matches(image: dict[str, Any]) -> bool:
        image_type = str(image.get("type", ""))
        if type_filter and image_type.lower() != type_filter.lower():
            return False
        if is_public_filter and bool(image.get("is_public", False)) != (
            is_public_filter.lower() == "true"
        ):
            return False
        return not (
            deprecated_filter
            and bool(image.get("deprecated", False))
            != (deprecated_filter.lower() == "true")
        )

    filters: list[str] = []
    if type_filter:
        filters.append(f"type={type_filter}")
    if is_public_filter:
        filters.append(f"is_public={is_public_filter}")
    if deprecated_filter:
        filters.append(f"deprecated={deprecated_filter}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/images")
        return serialize_list_response(
            raw,
            "images",
            image_pb2.ImageListResponse(),
            filter_value=", ".join(filters) if filters else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve Linode images", _call)
