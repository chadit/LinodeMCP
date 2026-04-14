"""Linode Object Storage write tools."""

from __future__ import annotations

import json
import re
from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


# Validation constants
_VALID_BUCKET_LABEL_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]{1,2}$")
_VALID_ACLS = {"private", "public-read", "authenticated-read", "public-read-write"}
_MIN_BUCKET_LABEL_LENGTH = 3
_MAX_BUCKET_LABEL_LENGTH = 63


def _validate_bucket_label(label: str) -> str | None:
    """Validate S3 bucket label. Returns error message or None."""
    if not label:
        return "label is required"
    if len(label) < _MIN_BUCKET_LABEL_LENGTH:
        return "bucket label must be at least 3 characters"
    if len(label) > _MAX_BUCKET_LABEL_LENGTH:
        return "bucket label must not exceed 63 characters"
    if not _VALID_BUCKET_LABEL_RE.match(label):
        return "bucket label must contain only lowercase letters, numbers, and hyphens"
    first, last = label[0], label[-1]
    if not (first.isalnum() and last.isalnum()):
        return "bucket label must start and end with a lowercase letter or number"
    return None


def _validate_bucket_acl(acl: str) -> str | None:
    """Validate bucket ACL. Returns error message or None."""
    if acl not in _VALID_ACLS:
        return f"acl must be one of: {', '.join(sorted(_VALID_ACLS))}"
    return None


def create_linode_object_storage_bucket_create_tool() -> Tool:
    """Create the linode_object_storage_bucket_create tool."""
    return Tool(
        name="linode_object_storage_bucket_create",
        description=(
            "Creates a new Object Storage bucket. WARNING: Billing starts immediately."
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
                    "description": (
                        "Bucket label (3-63 chars, lowercase alphanumeric and hyphens)"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": ("Region for the bucket (e.g. us-east-1)"),
                },
                "acl": {
                    "type": "string",
                    "description": (
                        "Access control: private, public-read,"
                        " authenticated-read, or"
                        " public-read-write"
                    ),
                },
                "cors_enabled": {
                    "type": "boolean",
                    "description": ("Whether to enable CORS (default: true)"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["label", "region", "confirm"],
        },
    )


async def handle_linode_object_storage_bucket_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_create tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    label = arguments.get("label", "")
    region = arguments.get("region", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    label_err = _validate_bucket_label(label)
    if label_err:
        return _error_response(label_err)

    validation_err = None
    if not region:
        validation_err = "region is required"
    elif acl is not None:
        validation_err = _validate_bucket_acl(acl)
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        bucket = await client.create_object_storage_bucket(
            label=label,
            region=region,
            acl=acl,
            cors_enabled=cors_enabled,
        )
        return {
            "message": (f"Bucket '{label}' created successfully in {region}"),
            "bucket": bucket,
        }

    return await execute_tool(cfg, arguments, "create bucket", _call)


def create_linode_object_storage_bucket_delete_tool() -> Tool:
    """Create the linode_object_storage_bucket_delete tool."""
    return Tool(
        name="linode_object_storage_bucket_delete",
        description=(
            "Deletes an Object Storage bucket."
            " WARNING: This is irreversible."
            " All objects must be removed first."
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
                "region": {
                    "type": "string",
                    "description": "Region of the bucket",
                },
                "label": {
                    "type": "string",
                    "description": "Label of the bucket",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["region", "label", "confirm"],
        },
    )


async def handle_linode_object_storage_bucket_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_delete tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This is destructive and irreversible."
                    " All objects must be removed first."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_bucket(region, label)
        return {
            "message": (f"Bucket '{label}' in {region} deleted successfully"),
            "region": region,
            "label": label,
        }

    return await execute_tool(cfg, arguments, "delete bucket", _call)


def create_linode_object_storage_bucket_access_update_tool() -> Tool:
    """Create the linode_object_storage_bucket_access_update tool."""
    return Tool(
        name="linode_object_storage_bucket_access_update",
        description=("Updates access control settings for an Object Storage bucket."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": "Region of the bucket",
                },
                "label": {
                    "type": "string",
                    "description": "Label of the bucket",
                },
                "acl": {
                    "type": "string",
                    "description": (
                        "New access control: private,"
                        " public-read, authenticated-read,"
                        " or public-read-write"
                    ),
                },
                "cors_enabled": {
                    "type": "boolean",
                    "description": ("Whether to enable CORS on the bucket"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": ("Must be true to confirm access update."),
                },
            },
            "required": ["region", "label", "confirm"],
        },
    )


async def handle_linode_object_storage_bucket_access_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle bucket access update tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This changes bucket access controls."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    validation_err = None
    if not region:
        validation_err = "region is required"
    elif not label:
        validation_err = "label is required"
    elif acl is not None:
        validation_err = _validate_bucket_acl(acl)
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.update_object_storage_bucket_access(
            region=region,
            label=label,
            acl=acl,
            cors_enabled=cors_enabled,
        )
        response: dict[str, Any] = {
            "message": (
                f"Access settings for bucket '{label}' in {region} updated successfully"
            ),
            "region": region,
            "label": label,
        }
        if acl is not None:
            response["acl"] = acl
        return response

    return await execute_tool(cfg, arguments, "update bucket access settings", _call)


# Stage 5 Phase 4: Object Storage access key write operations

_MAX_KEY_LABEL_LENGTH = 50
_VALID_KEY_PERMISSIONS = {"read_only", "read_write"}


def _validate_key_label(label: str) -> str | None:
    """Validate access key label. Returns error message or None."""
    if not label:
        return "label is required"
    if len(label) > _MAX_KEY_LABEL_LENGTH:
        return "access key label must not exceed 50 characters"
    return None


def _validate_bucket_access_entries(
    entries: list[dict[str, str]],
) -> str | None:
    """Validate bucket_access entries. Returns error message or None."""
    for i, entry in enumerate(entries):
        if not entry.get("bucket_name", "").strip():
            return f"entry {i}: bucket_access entries must include bucket_name"
        if not entry.get("region", "").strip():
            return f"entry {i}: bucket_access entries must include region"
        perms = entry.get("permissions", "")
        if perms not in _VALID_KEY_PERMISSIONS:
            return (
                f"entry {i}: bucket_access permissions must be"
                f" 'read_only' or 'read_write', got '{perms}'"
            )
    return None


def create_linode_object_storage_key_create_tool() -> Tool:
    """Create the linode_object_storage_key_create tool."""
    return Tool(
        name="linode_object_storage_key_create",
        description=(
            "Creates a new Object Storage access key."
            " WARNING: The secret_key is only shown ONCE"
            " in the response and cannot be retrieved later."
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
                    "description": ("Label for the access key (max 50 characters)"),
                },
                "bucket_access": {
                    "type": "string",
                    "description": (
                        "JSON array of bucket permissions:"
                        ' [{"bucket_name": "name", "region":'
                        ' "region", "permissions":'
                        ' "read_only|read_write"}].'
                        " Omit for unrestricted access."
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be set to true. The secret_key is only shown ONCE."
                    ),
                },
            },
            "required": ["label", "confirm"],
        },
    )


def create_linode_object_storage_key_update_tool() -> Tool:
    """Create the linode_object_storage_key_update tool."""
    return Tool(
        name="linode_object_storage_key_update",
        description=(
            "Updates an Object Storage access key's label or bucket permissions."
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
                "key_id": {
                    "type": "number",
                    "description": "ID of the access key to update",
                },
                "label": {
                    "type": "string",
                    "description": ("New label for the access key (max 50 characters)"),
                },
                "bucket_access": {
                    "type": "string",
                    "description": (
                        "JSON array of bucket permissions:"
                        ' [{"bucket_name": "name", "region":'
                        ' "region", "permissions":'
                        ' "read_only|read_write"}]'
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": ("Must be set to true to confirm key update."),
                },
            },
            "required": ["key_id", "confirm"],
        },
    )


def create_linode_object_storage_key_delete_tool() -> Tool:
    """Create the linode_object_storage_key_delete tool."""
    return Tool(
        name="linode_object_storage_key_delete",
        description=("Revokes an Object Storage access key permanently."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "key_id": {
                    "type": "number",
                    "description": ("ID of the access key to revoke"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be set to true to confirm"
                        " key revocation. This action is permanent."
                    ),
                },
            },
            "required": ["key_id", "confirm"],
        },
    )


async def handle_linode_object_storage_key_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_create tool."""
    label = arguments.get("label", "")
    bucket_access_json = arguments.get("bucket_access", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates an access key."
                    " The secret_key is only shown ONCE"
                    " in the response."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = _validate_key_label(label)
    bucket_access = None
    if not validation_err and bucket_access_json:
        try:
            bucket_access = json.loads(bucket_access_json)
            validation_err = _validate_bucket_access_entries(bucket_access)
        except (json.JSONDecodeError, TypeError) as e:
            validation_err = (
                f"Invalid bucket_access JSON: {e}."
                " Expected format:"
                ' [{"bucket_name": "name",'
                ' "region": "region",'
                ' "permissions": "read_only"}]'
            )
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.create_object_storage_key(
            label=label,
            bucket_access=bucket_access,
        )
        return {
            "warning": (
                "IMPORTANT: The secret_key below is shown"
                " ONLY ONCE. Save it now - it cannot be"
                " retrieved later."
            ),
            "message": (
                f"Access key '{key.get('label', label)}'"
                " created successfully"
                f" (ID: {key.get('id', 'unknown')})"
            ),
            "key": key,
        }

    return await execute_tool(cfg, arguments, "create access key", _call)


async def handle_linode_object_storage_key_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_update tool."""
    key_id = arguments.get("key_id", 0)
    label = arguments.get("label", "")
    bucket_access_json = arguments.get("bucket_access", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This modifies access key permissions."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = None
    if not key_id or int(key_id) <= 0:
        validation_err = "key_id is required and must be a positive integer"
    elif label:
        validation_err = _validate_key_label(label)

    key_id = int(key_id) if not validation_err else 0
    bucket_access = None
    if not validation_err and bucket_access_json:
        try:
            bucket_access = json.loads(bucket_access_json)
            validation_err = _validate_bucket_access_entries(bucket_access)
        except (json.JSONDecodeError, TypeError) as e:
            validation_err = (
                f"Invalid bucket_access JSON: {e}."
                " Expected format:"
                ' [{"bucket_name": "name",'
                ' "region": "region",'
                ' "permissions": "read_only"}]'
            )
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.update_object_storage_key(
            key_id=key_id,
            label=label or None,
            bucket_access=bucket_access,
        )
        return {
            "message": (f"Access key {key_id} updated successfully"),
            "key_id": key_id,
        }

    return await execute_tool(cfg, arguments, f"update access key {key_id}", _call)


async def handle_linode_object_storage_key_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_delete tool."""
    key_id = arguments.get("key_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This revokes the access key"
                    " permanently."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    if not key_id or int(key_id) <= 0:
        return _error_response("key_id is required and must be a positive integer")

    key_id = int(key_id)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_key(key_id=key_id)
        return {
            "message": (f"Access key {key_id} revoked successfully"),
            "key_id": key_id,
        }

    return await execute_tool(cfg, arguments, f"revoke access key {key_id}", _call)


# Stage 5 Phase 5: Presigned URLs, Object ACL, and SSL

_VALID_PRESIGNED_METHODS = {"GET", "PUT"}
_MIN_EXPIRES_IN = 1
_MAX_EXPIRES_IN = 604800
_DEFAULT_EXPIRES_IN = 3600


def _validate_presigned_method(method: str) -> str | None:
    """Validate presigned URL method. Returns error message or None."""
    if method.upper() not in _VALID_PRESIGNED_METHODS:
        return f"method must be 'GET' or 'PUT', got '{method}'"
    return None


def _validate_expires_in(expires_in: int) -> str | None:
    """Validate expires_in value. Returns error message or None."""
    if expires_in < _MIN_EXPIRES_IN or expires_in > _MAX_EXPIRES_IN:
        return (
            f"expires_in must be between {_MIN_EXPIRES_IN} and"
            f" {_MAX_EXPIRES_IN} seconds (7 days),"
            f" got {expires_in}"
        )
    return None


def create_linode_object_storage_presigned_url_tool() -> Tool:
    """Create the linode_object_storage_presigned_url tool."""
    return Tool(
        name="linode_object_storage_presigned_url",
        description=(
            "Generates a presigned URL for accessing an object"
            " in Object Storage. Use method=GET to create a"
            " download URL, method=PUT to create an upload URL."
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
                "region": {
                    "type": "string",
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "name": {
                    "type": "string",
                    "description": ("The object key (path/filename within the bucket)"),
                },
                "method": {
                    "type": "string",
                    "description": (
                        "HTTP method: 'GET' for download URL, 'PUT' for upload URL"
                    ),
                },
                "expires_in": {
                    "type": "number",
                    "description": (
                        "URL expiration in seconds (1-604800, default 3600 = 1 hour)"
                    ),
                },
            },
            "required": ["region", "label", "name", "method"],
        },
    )


async def handle_linode_object_storage_presigned_url(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_presigned_url tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")
    method = arguments.get("method", "")
    expires_in = int(arguments.get("expires_in", _DEFAULT_EXPIRES_IN))

    missing = (
        "region is required"
        if not region
        else "label is required"
        if not label
        else "name (object key) is required"
        if not name
        else None
    )
    if missing is not None:
        return _error_response(missing)

    validation_err = _validate_presigned_method(method)
    if validation_err is None:
        validation_err = _validate_expires_in(expires_in)
    if validation_err is not None:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_presigned_url(
            region, label, name, method.upper(), expires_in
        )

    return await execute_tool(cfg, arguments, "generate presigned URL", _call)


def create_linode_object_storage_object_acl_get_tool() -> Tool:
    """Create the linode_object_storage_object_acl_get tool."""
    return Tool(
        name="linode_object_storage_object_acl_get",
        description=(
            "Gets the Access Control List (ACL) for a specific"
            " object in an Object Storage bucket"
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
                "region": {
                    "type": "string",
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "name": {
                    "type": "string",
                    "description": ("The object key (path/filename within the bucket)"),
                },
            },
            "required": ["region", "label", "name"],
        },
    )


async def handle_linode_object_storage_object_acl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_object_acl_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")
    if not name:
        return _error_response("name (object key) is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_acl(region, label, name)

    return await execute_tool(cfg, arguments, "retrieve object ACL", _call)


def create_linode_object_storage_object_acl_update_tool() -> Tool:
    """Create the linode_object_storage_object_acl_update tool."""
    return Tool(
        name="linode_object_storage_object_acl_update",
        description=(
            "Updates the Access Control List (ACL) for a specific"
            " object in an Object Storage bucket."
            " Requires confirm=true to proceed."
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
                "region": {
                    "type": "string",
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "name": {
                    "type": "string",
                    "description": ("The object key (path/filename within the bucket)"),
                },
                "acl": {
                    "type": "string",
                    "description": (
                        "ACL to set: private, public-read,"
                        " authenticated-read,"
                        " or public-read-write"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to proceed."
                        " This modifies the object's"
                        " access permissions."
                    ),
                },
            },
            "required": [
                "region",
                "label",
                "name",
                "acl",
                "confirm",
            ],
        },
    )


async def handle_linode_object_storage_object_acl_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_object_acl_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="This modifies the object's access permissions."
                " Set confirm=true to proceed.",
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")
    acl = arguments.get("acl", "")

    missing = (
        "region is required"
        if not region
        else "label is required"
        if not label
        else "name (object key) is required"
        if not name
        else None
    )
    if missing is not None:
        return _error_response(missing)

    acl_err = _validate_bucket_acl(acl)
    if acl_err is not None:
        return _error_response(acl_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_object_acl(region, label, name, acl)

    return await execute_tool(cfg, arguments, "update object ACL", _call)


def create_linode_object_storage_ssl_get_tool() -> Tool:
    """Create the linode_object_storage_ssl_get tool."""
    return Tool(
        name="linode_object_storage_ssl_get",
        description=(
            "Checks whether an Object Storage bucket has an"
            " SSL/TLS certificate installed"
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
                "region": {
                    "type": "string",
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
            },
            "required": ["region", "label"],
        },
    )


async def handle_linode_object_storage_ssl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_ssl_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_bucket_ssl(region, label)

    return await execute_tool(cfg, arguments, "retrieve SSL status", _call)


def create_linode_object_storage_ssl_delete_tool() -> Tool:
    """Create the linode_object_storage_ssl_delete tool."""
    return Tool(
        name="linode_object_storage_ssl_delete",
        description=(
            "Deletes the SSL/TLS certificate from an Object"
            " Storage bucket."
            " Requires confirm=true to proceed."
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
                "region": {
                    "type": "string",
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to proceed."
                        " This removes the SSL certificate"
                        " from the bucket."
                    ),
                },
            },
            "required": ["region", "label", "confirm"],
        },
    )


async def handle_linode_object_storage_ssl_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_ssl_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="This removes the SSL certificate"
                " from the bucket."
                " Set confirm=true to proceed.",
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_bucket_ssl(region, label)
        return {
            "message": (
                f"SSL certificate deleted from bucket '{label}' in region '{region}'"
            ),
            "region": region,
            "bucket": label,
        }

    return await execute_tool(cfg, arguments, "delete SSL certificate", _call)


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]
