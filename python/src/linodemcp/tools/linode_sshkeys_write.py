from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import sshkey_pb2
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
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_sshkey_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_create tool."""
    return Tool(
        name="linode_sshkey_create",
        description=(
            "Creates a new SSH key and adds it to your Linode profile."
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
                    "description": "A label for the SSH key (required)",
                },
                "ssh_key": {
                    "type": "string",
                    "description": "The public SSH key (required)",
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
            "required": ["label", "ssh_key", "confirm"],
        },
    ), Capability.Write


def _sshkey_create_fields_error(label: str, ssh_key: str) -> list[TextContent] | None:
    """Validate required create fields; return an error response or None."""
    if not label:
        return error_response("label is required")
    if not ssh_key:
        return error_response("ssh_key is required")
    return None


async def handle_linode_sshkey_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_create tool request."""
    label = arguments.get("label", "")
    ssh_key = arguments.get("ssh_key", "")

    if is_dry_run(arguments):
        fields_error = _sshkey_create_fields_error(label, ssh_key)
        if fields_error is not None:
            return fields_error
        return build_dry_run_response(
            "linode_sshkey_create",
            arguments.get("environment", ""),
            "POST",
            "/profile/sshkeys",
            None,
            side_effects=[f"A new SSH key {label!r} will be added to your profile."],
        )

    if not arguments.get("confirm"):
        return error_response(
            "This creates an SSH key on your profile. Set confirm=true to proceed."
        )

    fields_error = _sshkey_create_fields_error(label, ssh_key)
    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.create_ssh_key(label, ssh_key)
        # The ssh_key field carries the public key, which is public information
        # and kept in full.
        return serialize_api_response(
            {
                "message": (
                    f"SSH key '{key.label}' (ID: {key.id}) created successfully"
                ),
                "ssh_key": {
                    "id": key.id,
                    "label": key.label,
                    "ssh_key": key.ssh_key,
                    "created": key.created,
                },
            },
            sshkey_pb2.SSHKeyWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create SSH key", _call)


def create_linode_sshkey_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_update tool."""
    return Tool(
        name="linode_sshkey_update",
        description=(
            "Updates the label for an SSH key in your Linode profile."
            " Pass dry_run=true to preview without updating."
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
                "ssh_key_id": {
                    "type": "integer",
                    "description": "The ID of the SSH key to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "The new label for the SSH key (required)",
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
            "required": ["ssh_key_id", "label", "confirm"],
        },
    ), Capability.Write


def _sshkey_update_fields_error(
    ssh_key_id: Any, label: str
) -> list[TextContent] | None:
    """Validate required update fields; return an error response or None."""
    if not ssh_key_id:
        return error_response("ssh_key_id is required")
    if not label:
        return error_response("label is required")
    return None


def _sshkey_update_side_effects(state: Any, new_label: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for SSH key update. Reports the label change against
    the fetched state.
    """
    if not new_label:
        return {}
    from_label = getattr(state, "label", "")
    if from_label and from_label != new_label:
        return {
            "side_effects": [f"Label changes from {from_label!r} to {new_label!r}."]
        }
    return {"side_effects": [f"Label is set to {new_label!r}."]}


async def handle_linode_sshkey_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_update tool request."""
    ssh_key_id = arguments.get("ssh_key_id", 0)
    label = arguments.get("label", "")

    if is_dry_run(arguments):
        fields_error = _sshkey_update_fields_error(ssh_key_id, label)
        if fields_error is not None:
            return fields_error
        ssh_key_id_int = int(ssh_key_id)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_ssh_key(ssh_key_id_int)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _sshkey_update_side_effects(state, label)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_sshkey_update",
            "PUT",
            f"/profile/sshkeys/{ssh_key_id_int}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This updates an SSH key. Set confirm=true to proceed.")

    fields_error = _sshkey_update_fields_error(ssh_key_id, label)
    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.update_ssh_key(int(ssh_key_id), label)
        # The ssh_key field carries the public key, which is public information
        # and kept in full.
        return serialize_api_response(
            {
                "message": (
                    f"SSH key '{key.label}' (ID: {key.id}) updated successfully"
                ),
                "ssh_key": {
                    "id": key.id,
                    "label": key.label,
                    "ssh_key": key.ssh_key,
                    "created": key.created,
                },
            },
            sshkey_pb2.SSHKeyWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update SSH key", _call)


def create_linode_sshkey_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_sshkey_delete tool."""
    return Tool(
        name="linode_sshkey_delete",
        description=(
            "Deletes an SSH key from your Linode profile."
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
                "ssh_key_id": {
                    "type": "integer",
                    "description": "The ID of the SSH key to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["ssh_key_id", "confirm"],
        },
    ), Capability.Destroy


async def _sshkey_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, ssh_key_id_int: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_ssh_key(ssh_key_id_int)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ssh_key(ssh_key_id_int)
        return {
            "message": f"SSH key {ssh_key_id_int} removed successfully",
            "ssh_key_id": ssh_key_id_int,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_sshkey_delete",
        method="DELETE",
        path=f"/profile/sshkeys/{ssh_key_id_int}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("SSHKey"),
    )


async def handle_linode_sshkey_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_delete tool request."""
    ssh_key_id = arguments.get("ssh_key_id", 0)

    if not ssh_key_id:
        return error_response("ssh_key_id is required")

    ssh_key_id_int = int(ssh_key_id)

    two_stage = await _sshkey_delete_two_stage(arguments, cfg, ssh_key_id_int)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_ssh_key(ssh_key_id_int)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_sshkey_delete",
            "DELETE",
            f"/profile/sshkeys/{ssh_key_id_int}",
            _fetch,
        )

    if not arguments.get("confirm"):
        return error_response("This deletes an SSH key. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ssh_key(ssh_key_id_int)
        return {
            "message": f"SSH key {ssh_key_id_int} removed successfully",
            "ssh_key_id": ssh_key_id_int,
        }

    return await execute_tool(cfg, arguments, "delete SSH key", _call)
