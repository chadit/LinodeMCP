"""Linode Object Storage write tools."""

from __future__ import annotations

import json
import re
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
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

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


def create_linode_object_storage_cancel_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_cancel tool."""
    return Tool(
        name="linode_object_storage_cancel",
        description=(
            "Cancels Object Storage service for the account. "
            "Requires confirm=true because this is a destructive "
            "account-level operation."
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm Object Storage cancellation"
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_object_storage_cancel(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_cancel tool request."""
    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_object_storage_cancel",
            arguments.get("environment", ""),
            "POST",
            "/object-storage/cancel",
            None,
        )

    if arguments.get("confirm") is not True:
        return [TextContent(type="text", text="confirm=true is required")]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.cancel_object_storage()
        if result:
            return result
        return {"message": "Object Storage cancellation requested"}

    return await execute_tool(cfg, arguments, "cancel Object Storage", _call)


def create_linode_object_storage_bucket_create_tool() -> tuple[Tool, Capability]:
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "region", "confirm"],
        },
    ), Capability.Write


def _bucket_create_error(label: str, region: str, acl: Any) -> str | None:
    """Validate bucket create args; return an error message or None."""
    label_err = _validate_bucket_label(label)
    if label_err:
        return label_err
    if not region:
        return "region is required"
    if acl is not None:
        return _validate_bucket_acl(acl)
    return None


async def handle_linode_object_storage_bucket_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_create tool request."""
    label = arguments.get("label", "")
    region = arguments.get("region", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    if is_dry_run(arguments):
        validation_err = _bucket_create_error(label, region, acl)
        if validation_err:
            return _error_response(validation_err)
        return build_dry_run_response(
            "linode_object_storage_bucket_create",
            arguments.get("environment", ""),
            "POST",
            "/object-storage/buckets",
            None,
            side_effects=[
                f"A new Object Storage bucket {label!r} will be created in {region}."
            ],
            warnings=["Billing for Object Storage starts immediately on creation."],
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = _bucket_create_error(label, region, acl)
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


def create_linode_object_storage_bucket_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_delete tool."""
    return Tool(
        name="linode_object_storage_bucket_delete",
        description=(
            "Deletes an Object Storage bucket."
            " WARNING: This is irreversible."
            " All objects must be removed first."
            " Pass dry_run=true to preview without deleting."
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["region", "label", "confirm"],
        },
    ), Capability.Destroy


async def _object_storage_bucket_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, region: str, label: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_object_storage_bucket(region, label)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_bucket(region, label)
        return {
            "message": f"Bucket '{label}' in {region} deleted successfully",
            "region": region,
            "label": label,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_object_storage_bucket_delete",
        method="DELETE",
        path=f"/object-storage/buckets/{region}/{label}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("ObjectStorageBucket"),
    )


async def handle_linode_object_storage_bucket_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_delete tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    # Both branches need region and label, and the spec is explicit that
    # dry-run errors out on missing required args the same way the real
    # call would.
    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    two_stage = await _object_storage_bucket_delete_two_stage(
        arguments, cfg, region, label
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_object_storage_bucket(region, label)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_bucket_delete",
            "DELETE",
            f"/object-storage/buckets/{region}/{label}",
            _fetch,
        )

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

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_bucket(region, label)
        return {
            "message": (f"Bucket '{label}' in {region} deleted successfully"),
            "region": region,
            "label": label,
        }

    return await execute_tool(cfg, arguments, "delete bucket", _call)


def create_linode_object_storage_bucket_access_update_tool() -> tuple[Tool, Capability]:
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
                    "description": (
                        "Must be true to confirm access update."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["region", "label", "confirm"],
        },
    ), Capability.Write


def _bucket_access_error(region: str, label: str, acl: Any) -> str | None:
    """Validate bucket access args; return an error message or None."""
    if not region:
        return "region is required"
    if not label:
        return "label is required"
    if acl is not None:
        return _validate_bucket_acl(acl)
    return None


def _bucket_access_update_side_effects(new_acl: Any, new_cors: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for bucket access update. Reports the ACL change and
    a CORS enable/disable toggle.
    """
    side_effects: list[str] = []
    if new_acl:
        side_effects.append(f"Bucket access control is set to {new_acl!r}.")
    if new_cors is not None:
        side_effects.append(
            f"CORS is {'enabled' if new_cors else 'disabled'} for the bucket."
        )
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_object_storage_bucket_access_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle bucket access update tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    if is_dry_run(arguments):
        validation_err = _bucket_access_error(region, label, acl)
        if validation_err:
            return _error_response(validation_err)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_object_storage_bucket_access(region, label)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _bucket_access_update_side_effects(acl, cors_enabled)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_bucket_access_update",
            "PUT",
            f"/object-storage/buckets/{region}/{label}/access",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This changes bucket access controls."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = _bucket_access_error(region, label, acl)
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


def create_linode_object_storage_bucket_access_allow_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_bucket_access_allow tool."""
    return Tool(
        name="linode_object_storage_bucket_access_allow",
        description=("Allows access to an Object Storage bucket."),
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
                        "Access control: private, public-read,"
                        " authenticated-read, or public-read-write"
                    ),
                },
                "cors_enabled": {
                    "type": "boolean",
                    "description": ("Whether to enable CORS on the bucket"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm access changes."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["region", "label", "confirm"],
        },
    ), Capability.Write


async def handle_linode_object_storage_bucket_access_allow(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle bucket access allow tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    if is_dry_run(arguments):
        validation_err = _bucket_access_error(region, label, acl)
        if validation_err:
            return _error_response(validation_err)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_object_storage_bucket_access(region, label)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_bucket_access_allow",
            "POST",
            f"/object-storage/buckets/{region}/{label}/access",
            _fetch,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This changes bucket access controls."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = _bucket_access_error(region, label, acl)
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        access = await client.allow_object_storage_bucket_access(
            region=region,
            label=label,
            acl=acl,
            cors_enabled=cors_enabled,
        )
        return {
            "message": (
                f"Access allowed for bucket '{label}' in {region} successfully"
            ),
            "region": region,
            "label": label,
            "access": access,
        }

    return await execute_tool(cfg, arguments, "allow bucket access", _call)


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


def create_linode_object_storage_key_create_tool() -> tuple[Tool, Capability]:
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


def create_linode_object_storage_key_update_tool() -> tuple[Tool, Capability]:
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
                    "description": (
                        "Must be set to true to confirm key update."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["key_id", "confirm"],
        },
    ), Capability.Write


def create_linode_object_storage_key_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_key_delete tool."""
    return Tool(
        name="linode_object_storage_key_delete",
        description=(
            "Revokes an Object Storage access key permanently. Pass "
            "dry_run=true to preview without revoking."
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
                "key_id": {
                    "type": "number",
                    "description": ("ID of the access key to revoke"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be set to true to confirm key revocation. This "
                        "action is permanent. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["key_id", "confirm"],
        },
    ), Capability.Destroy


def _parse_key_bucket_access(bucket_access_json: str) -> tuple[Any, str | None]:
    """Parse+validate bucket_access JSON; return (value, error message)."""
    if not bucket_access_json:
        return None, None
    try:
        bucket_access = json.loads(bucket_access_json)
    except (json.JSONDecodeError, TypeError) as e:
        return None, (
            f"Invalid bucket_access JSON: {e}."
            " Expected format:"
            ' [{"bucket_name": "name",'
            ' "region": "region",'
            ' "permissions": "read_only"}]'
        )
    return bucket_access, _validate_bucket_access_entries(bucket_access)


def _key_create_error(label: str, bucket_access_json: str) -> str | None:
    """Validate key create args; return an error message or None."""
    label_err = _validate_key_label(label)
    if label_err:
        return label_err
    _, access_err = _parse_key_bucket_access(bucket_access_json)
    return access_err


async def handle_linode_object_storage_key_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_create tool."""
    label = arguments.get("label", "")
    bucket_access_json = arguments.get("bucket_access", "")

    if is_dry_run(arguments):
        validation_err = _key_create_error(label, bucket_access_json)
        if validation_err:
            return _error_response(validation_err)
        return build_dry_run_response(
            "linode_object_storage_key_create",
            arguments.get("environment", ""),
            "POST",
            "/object-storage/keys",
            None,
            side_effects=[
                f"A new Object Storage access key {label!r} will be created."
            ],
            warnings=["The secret key is returned only once, at creation time."],
        )

    if not arguments.get("confirm"):
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

    validation_err = _key_create_error(label, bucket_access_json)
    if validation_err:
        return _error_response(validation_err)

    bucket_access, _ = _parse_key_bucket_access(bucket_access_json)

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


def _key_update_error(key_id: Any, label: str, bucket_access_json: str) -> str | None:
    """Validate key update args; return an error message or None."""
    if not key_id or int(key_id) <= 0:
        return "key_id is required and must be a positive integer"
    if label:
        label_err = _validate_key_label(label)
        if label_err:
            return label_err
    _, access_err = _parse_key_bucket_access(bucket_access_json)
    return access_err


def _key_update_side_effects(
    state: Any, new_label: Any, new_bucket_access: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for object-storage key update. Reports the label
    change against the fetched key (credential-safe: the GET never returns the
    secret) and notes when bucket access scopes are replaced.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f"Label changes from {from_label!r} to {new_label!r}.")
        else:
            side_effects.append(f"Label is set to {new_label!r}.")
    if new_bucket_access:
        side_effects.append("The key's bucket access scopes are replaced.")
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_object_storage_key_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_update tool."""
    key_id_raw = arguments.get("key_id", 0)
    label = arguments.get("label", "")
    bucket_access_json = arguments.get("bucket_access", "")

    validation_err = _key_update_error(key_id_raw, label, bucket_access_json)
    if validation_err:
        return _error_response(validation_err)

    key_id = int(key_id_raw)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_object_storage_key(key_id=key_id)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _key_update_side_effects(state, label, bucket_access_json)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_key_update",
            "PUT",
            f"/object-storage/keys/{key_id}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This modifies access key permissions."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    bucket_access, _ = _parse_key_bucket_access(bucket_access_json)

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


async def _object_storage_key_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, key_id: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_object_storage_key(key_id=key_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_key(key_id=key_id)
        return {
            "message": f"Access key {key_id} revoked successfully",
            "key_id": key_id,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_object_storage_key_delete",
        method="DELETE",
        path=f"/object-storage/keys/{key_id}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("ObjectStorageKey"),
    )


async def handle_linode_object_storage_key_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_delete tool."""
    key_id_raw = arguments.get("key_id", 0)

    # ID validation runs before both branches: dry-run and the real call
    # both need a positive integer, and the spec is explicit that dry-run
    # errors out on missing required args the same way the real call would.
    if not key_id_raw or int(key_id_raw) <= 0:
        return _error_response("key_id is required and must be a positive integer")

    key_id = int(key_id_raw)

    two_stage = await _object_storage_key_delete_two_stage(arguments, cfg, key_id)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_object_storage_key(key_id=key_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_key_delete",
            "DELETE",
            f"/object-storage/keys/{key_id}",
            _fetch,
        )

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


def create_linode_object_storage_presigned_url_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_presigned_url_create tool."""
    return Tool(
        name="linode_object_storage_presigned_url_create",
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
    ), Capability.Read


async def handle_linode_object_storage_presigned_url_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_presigned_url_create tool request."""
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


def create_linode_object_storage_object_acl_get_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


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


def create_linode_object_storage_object_acl_update_tool() -> tuple[Tool, Capability]:
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "region",
                "label",
                "name",
                "acl",
                "confirm",
            ],
        },
    ), Capability.Write


def _object_acl_update_error(
    region: str, label: str, name: str, acl: str
) -> str | None:
    """Validate object ACL update args; return an error message or None."""
    if not region:
        return "region is required"
    if not label:
        return "label is required"
    if not name:
        return "name (object key) is required"
    return _validate_bucket_acl(acl)


def _object_acl_update_side_effects(new_acl: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for object ACL update. Reports the new access-control
    level the object is set to.
    """
    if not new_acl:
        return {}
    return {"side_effects": [f"Object access control is set to {new_acl!r}."]}


async def handle_linode_object_storage_object_acl_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_object_acl_update tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")
    acl = arguments.get("acl", "")

    if is_dry_run(arguments):
        validation_err = _object_acl_update_error(region, label, name, acl)
        if validation_err is not None:
            return _error_response(validation_err)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_object_acl(region, label, name)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _object_acl_update_side_effects(acl)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_object_acl_update",
            "PUT",
            f"/object-storage/buckets/{region}/{label}/object-acl",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="This modifies the object's access permissions."
                " Set confirm=true to proceed.",
            )
        ]

    validation_err = _object_acl_update_error(region, label, name, acl)
    if validation_err is not None:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_object_acl(region, label, name, acl)

    return await execute_tool(cfg, arguments, "update object ACL", _call)


def create_linode_object_storage_ssl_get_tool() -> tuple[Tool, Capability]:
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
    ), Capability.Read


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


def create_linode_object_storage_ssl_upload_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_ssl_upload tool."""
    return Tool(
        name="linode_object_storage_ssl_upload",
        description=(
            "Uploads an SSL/TLS certificate and private key"
            " for an Object Storage bucket."
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
                "certificate": {
                    "type": "string",
                    "description": "Base64 encoded and PEM formatted SSL certificate",
                },
                "private_key": {
                    "type": "string",
                    "description": "Private key associated with the SSL certificate",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to proceed."
                        " This uploads SSL certificate material"
                        " for the bucket."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "region",
                "label",
                "certificate",
                "private_key",
                "confirm",
            ],
        },
    ), Capability.Write


def _ssl_upload_error(
    region: str, label: str, certificate: str, private_key: str
) -> str | None:
    """Validate SSL upload args; return an error message or None."""
    if not region:
        return "region is required"
    if not label:
        return "label is required"
    if not certificate:
        return "certificate is required"
    if not private_key:
        return "private_key is required"
    return None


async def handle_linode_object_storage_ssl_upload(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_ssl_upload tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    certificate = arguments.get("certificate", "")
    private_key = arguments.get("private_key", "")

    if is_dry_run(arguments):
        validation_err = _ssl_upload_error(region, label, certificate, private_key)
        if validation_err is not None:
            return _error_response(validation_err)
        # current_state null; the request body (cert + private_key) is never
        # echoed in the v0 preview, so no key material leaks.
        return build_dry_run_response(
            "linode_object_storage_ssl_upload",
            arguments.get("environment", ""),
            "POST",
            f"/object-storage/buckets/{region}/{label}/ssl",
            None,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="This uploads SSL certificate material"
                " for the bucket."
                " Set confirm=true to proceed.",
            )
        ]

    validation_err = _ssl_upload_error(region, label, certificate, private_key)
    if validation_err is not None:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.upload_bucket_ssl(region, label, certificate, private_key)
        return {
            "message": (
                f"SSL certificate uploaded for bucket '{label}' in region '{region}'"
            ),
            "region": region,
            "bucket": label,
            "ssl": result.get("ssl"),
        }

    return await execute_tool(cfg, arguments, "upload SSL certificate", _call)


def create_linode_object_storage_ssl_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_object_storage_ssl_delete tool."""
    return Tool(
        name="linode_object_storage_ssl_delete",
        description=(
            "Deletes the SSL/TLS certificate from an Object"
            " Storage bucket."
            " Requires confirm=true to proceed."
            " Pass dry_run=true to preview without deleting."
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
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["region", "label", "confirm"],
        },
    ), Capability.Destroy


async def _object_storage_ssl_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, region: str, label: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_bucket_ssl(region, label)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_bucket_ssl(region, label)
        return {
            "message": (
                f"SSL certificate deleted from bucket '{label}' in region '{region}'"
            ),
            "region": region,
            "bucket": label,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_object_storage_ssl_delete",
        method="DELETE",
        path=f"/object-storage/buckets/{region}/{label}/ssl",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("ObjectStorageSSL"),
    )


async def handle_linode_object_storage_ssl_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_ssl_delete tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    # Both branches need region and label, and the spec is explicit that
    # dry-run errors out on missing required args the same way the real
    # call would.
    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    two_stage = await _object_storage_ssl_delete_two_stage(
        arguments, cfg, region, label
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_bucket_ssl(region, label)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_object_storage_ssl_delete",
            "DELETE",
            f"/object-storage/buckets/{region}/{label}/ssl",
            _fetch,
        )

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
