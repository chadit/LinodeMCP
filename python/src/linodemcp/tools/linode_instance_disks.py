from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, TypeGuard, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    firewall_pb2,
    instance_pb2,
    volume_pb2,
)
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
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


def _split_comma_separated(raw: object) -> list[str] | None:
    """Split a comma-separated string into trimmed, non-empty entries.

    Mirrors the Go disk-create handler so both languages accept the same
    comma-delimited input and post the same array body to the Linode API.
    Returns None when nothing is supplied so the client omits the key.
    """
    if not isinstance(raw, str):
        return None
    parts = [segment.strip() for segment in raw.split(",")]
    entries = [segment for segment in parts if segment]
    return entries or None


_VALID_CONFIG_INTERFACE_PURPOSES = frozenset({"public", "vlan", "vpc"})


def _is_dict(value: Any) -> bool:
    """Return True when value is a dict, isolating the isinstance narrowing."""
    return isinstance(value, dict)


def _is_list(value: Any) -> bool:
    """Return True when value is a list, isolating the isinstance narrowing."""
    return isinstance(value, list)


def _parse_config_helpers(raw: object) -> tuple[Any, str | None]:
    """Parse the helpers JSON-object string; return (value, error message).

    Mirrors the Go create handler, which takes helpers as a JSON-encoded object
    string and posts the decoded object verbatim so the Linode API can keep
    evolving the boot-helper toggles without the tool rejecting new fields.
    """
    if raw is None:
        return None, None
    if not isinstance(raw, str) or not raw:
        return None, None
    try:
        helpers: Any = json.loads(raw)
    except (json.JSONDecodeError, TypeError) as exc:
        return None, f"invalid helpers JSON: {exc}"
    if not _is_dict(helpers):
        return None, "helpers must be a JSON object"
    return helpers, None


def _validate_config_interfaces(interfaces: Any) -> str | None:
    """Validate each interface object's purpose; return an error or None."""
    for index, iface in enumerate(interfaces):
        if not _is_dict(iface):
            return "interfaces must be an array of objects"
        if iface.get("purpose") not in _VALID_CONFIG_INTERFACE_PURPOSES:
            return f"interfaces[{index}].purpose must be public, vlan, or vpc"
    return None


def _parse_config_interfaces(raw: object) -> tuple[Any, str | None]:
    """Parse the interfaces JSON-array string; return (value, error message).

    Mirrors the Go create handler: interfaces arrives as a JSON-encoded array of
    objects, each carrying a purpose of public, vlan, or vpc.
    """
    if raw is None:
        return None, None
    if not isinstance(raw, str) or not raw:
        return None, None
    try:
        interfaces: Any = json.loads(raw)
    except (json.JSONDecodeError, TypeError) as exc:
        return None, f"invalid interfaces JSON: {exc}"
    if not _is_list(interfaces):
        return None, "interfaces must be an array of objects"
    error = _validate_config_interfaces(interfaces)
    if error is not None:
        return None, error
    return interfaces, None


def _parse_config_json_options(
    arguments: dict[str, Any],
) -> tuple[Any, Any, str | None]:
    """Parse the helpers and interfaces options; return (helpers, interfaces, error)."""
    helpers, helpers_err = _parse_config_helpers(arguments.get("helpers"))
    if helpers_err is not None:
        return None, None, helpers_err
    interfaces, interfaces_err = _parse_config_interfaces(arguments.get("interfaces"))
    if interfaces_err is not None:
        return None, None, interfaces_err
    return helpers, interfaces, None


_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_LINODE_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The ID of the Linode instance (required)",
}

_DISK_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The ID of the disk (required)",
}

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}


def _parse_instance_id(
    arguments: dict[str, Any],
) -> int | list[TextContent]:
    """Parse and validate linode_id from arguments."""
    raw = arguments.get("linode_id", "")
    if raw is None or raw == "":
        return _error_response("linode_id is required")
    if isinstance(raw, bool):
        return _error_response("linode_id must be a positive integer")
    try:
        linode_id = int(raw)
    except (ValueError, TypeError):
        return _error_response("linode_id must be a valid integer")
    if linode_id < 1:
        return _error_response("linode_id must be a positive integer")
    return linode_id


def _parse_optional_page_args(
    arguments: dict[str, Any],
) -> tuple[int | None, int | None] | list[TextContent]:
    """Parse optional page and page_size arguments."""
    values: dict[str, int | None] = {"page": None, "page_size": None}
    for name, minimum, maximum in (("page", 1, None), ("page_size", 25, 500)):
        raw = arguments.get(name)
        if raw is None:
            continue
        if isinstance(raw, bool):
            return _error_response(f"{name} must be an integer")
        try:
            value = int(raw)
        except (TypeError, ValueError):
            return _error_response(f"{name} must be an integer")
        if value < minimum:
            return _error_response(f"{name} must be at least {minimum}")
        if maximum is not None and value > maximum:
            return _error_response(f"{name} must be at most {maximum}")
        values[name] = value
    return values["page"], values["page_size"]


def _parse_instance_and_disk_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse linode_id and disk_id from arguments."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid
    raw = arguments.get("disk_id", "")
    if not raw:
        return _error_response("disk_id is required")
    try:
        disk_id = int(raw)
    except (ValueError, TypeError):
        return _error_response("disk_id must be a valid integer")
    return iid, disk_id


def _is_non_empty_dict(value: Any) -> TypeGuard[dict[str, Any]]:
    """Return whether a value is a non-empty dictionary."""
    if not isinstance(value, dict):
        return False
    candidate = cast("dict[str, Any]", value)
    return len(candidate) > 0


def create_linode_instance_disk_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_list tool."""
    return Tool(
        name="linode_instance_disk_list",
        description="Lists disks for a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_instance_disk_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        disks = await client.list_instance_disks(iid)
        return serialize_list_response(
            {"data": disks},
            "disks",
            instance_pb2.InstanceDiskListResponse(),
        )

    return await execute_tool(cfg, arguments, "list instance disks", _call)


def create_linode_instance_volume_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_volume_list tool."""
    return Tool(
        name="linode_instance_volume_list",
        description="Lists volumes attached to a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
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
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_instance_volume_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_volume_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid
    pagination = _parse_optional_page_args(arguments)
    if isinstance(pagination, list):
        return pagination
    page, page_size = pagination

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        raw = await client.list_instance_volumes(iid, page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "volumes",
            volume_pb2.VolumeListResponse(),
        )

    return await execute_tool(cfg, arguments, "list instance volumes", _call)


def create_linode_instance_firewall_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_firewall_list tool."""
    return Tool(
        name="linode_instance_firewall_list",
        description="Lists firewalls assigned to a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
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
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_instance_firewall_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_firewall_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid
    pagination = _parse_optional_page_args(arguments)
    if isinstance(pagination, list):
        return pagination
    page, page_size = pagination

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        raw = await client.list_instance_firewalls(iid, page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "firewalls",
            firewall_pb2.FirewallListResponse(),
        )

    return await execute_tool(cfg, arguments, "list instance firewalls", _call)


def create_linode_instance_disk_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_get tool."""
    return Tool(
        name="linode_instance_disk_get",
        description=("Gets details of a specific disk on an instance"),
        inputSchema=schema("linode.mcp.v1.InstanceDiskGetInput"),
    ), Capability.Read


async def handle_linode_instance_disk_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_get tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, disk_id = ids

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_disk(linode_id, disk_id),
            instance_pb2.InstanceDisk(),
        )

    return await execute_tool(cfg, arguments, "get instance disk", _call)


def create_linode_instance_config_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_create tool."""
    return Tool(
        name="linode_instance_config_create",
        description="Creates a configuration profile on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the configuration profile",
                },
                "devices": {
                    "type": "object",
                    "description": "Config devices mapping, such as sda/sdb entries",
                },
                "comments": {
                    "type": "string",
                    "description": "Optional comments for the configuration profile",
                },
                "kernel": {
                    "type": "string",
                    "description": "Kernel ID to boot, e.g. linode/latest-64bit",
                },
                "memory_limit": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Optional memory limit in MB",
                },
                "root_device": {
                    "type": "string",
                    "description": "Root device to boot, e.g. /dev/sda",
                },
                "run_level": {
                    "type": "string",
                    "enum": ["default", "single", "binbash"],
                    "description": "Run level: default, single, or binbash",
                },
                "virt_mode": {
                    "type": "string",
                    "enum": ["paravirt", "fullvirt"],
                    "description": "Virtualization mode: paravirt or fullvirt",
                },
                "helpers": {
                    "type": "string",
                    "description": "Optional helpers JSON object",
                },
                "interfaces": {
                    "type": "string",
                    "description": "Optional interfaces JSON array",
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "label", "devices", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_config_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_config_create tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    label = arguments.get("label", "")
    if not isinstance(label, str) or not label:
        return _error_response("label is required")

    devices = arguments.get("devices")
    if not _is_non_empty_dict(devices):
        return _error_response("devices must be a non-empty object")
    devices_payload = devices

    helpers_payload, interfaces_payload, json_err = _parse_config_json_options(
        arguments
    )
    if json_err is not None:
        return _error_response(json_err)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"A new config profile {label!r} will be created on instance {iid}."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_config_create",
            "POST",
            f"/linode/instances/{iid}/configs",
            _fetch,
            _walk,
        )

    return await _create_instance_config_live(
        arguments,
        cfg,
        iid,
        label,
        devices_payload,
        helpers_payload,
        interfaces_payload,
    )


async def _create_instance_config_live(
    arguments: dict[str, Any],
    cfg: Config,
    iid: int,
    label: str,
    devices_payload: dict[str, Any],
    helpers_payload: Any,
    interfaces_payload: Any,
) -> list[TextContent]:
    """Run the confirmed config-create after dry-run and validation passed."""
    confirm = arguments.get("confirm", False)
    if confirm is not True:
        return _error_response("Set confirm=true to proceed.")

    memory_limit_arg = arguments.get("memory_limit")
    memory_limit = int(memory_limit_arg) if memory_limit_arg is not None else None

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        config = await client.create_instance_config(
            iid,
            label=label,
            devices=devices_payload,
            comments=arguments.get("comments"),
            kernel=arguments.get("kernel"),
            memory_limit=memory_limit,
            root_device=arguments.get("root_device"),
            run_level=arguments.get("run_level"),
            virt_mode=arguments.get("virt_mode"),
            helpers=helpers_payload,
            interfaces=interfaces_payload,
        )
        return serialize_api_response(
            {
                # Match Go's zero-value getters on an empty API body: label "",
                # id 0, so both languages emit the same message string.
                "message": (
                    f"Configuration profile '{config.get('label', '')}'"
                    f" (ID: {config.get('id', 0)}) created on instance {iid}"
                ),
                "config": config,
            },
            instance_pb2.InstanceConfigWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create instance config", _call)


def create_linode_instance_disk_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_create tool."""
    return Tool(
        name="linode_instance_disk_create",
        description="Creates a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the disk",
                },
                "size": {
                    "type": "integer",
                    "description": "Disk size in MB",
                },
                "filesystem": {
                    "type": "string",
                    "description": ("Filesystem type (ext4, swap, raw, etc.)"),
                },
                "image": {
                    "type": "string",
                    "description": "Image to deploy",
                },
                "root_pass": {
                    "type": "string",
                    "description": ("Root password (required with image)"),
                },
                "authorized_keys": {
                    "type": "string",
                    "description": "Comma-separated list of SSH public keys to install",
                },
                "authorized_users": {
                    "type": "string",
                    "description": (
                        "Comma-separated list of Linode usernames whose SSH keys"
                        " to install"
                    ),
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "label",
                "size",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_instance_disk_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_create tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    label = arguments.get("label", "")
    if not label:
        return _error_response("label is required")

    size = arguments.get("size")
    if not size:
        return _error_response("size is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"A new {size} MB disk {label!r} will be created on instance {iid}."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_create",
            "POST",
            f"/linode/instances/{iid}/disks",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        disk = await client.create_instance_disk(
            iid,
            label=label,
            size=int(size),
            filesystem=arguments.get("filesystem"),
            image=arguments.get("image"),
            root_pass=arguments.get("root_pass"),
            authorized_keys=_split_comma_separated(arguments.get("authorized_keys")),
            authorized_users=_split_comma_separated(arguments.get("authorized_users")),
        )
        return serialize_api_response(
            {
                # Match Go's zero-value getters on an empty API body.
                "message": (
                    f"Disk '{disk.get('label', '')}' (ID: {disk.get('id', 0)})"
                    f" created on instance {iid}"
                ),
                "disk": disk,
            },
            instance_pb2.InstanceDiskWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create instance disk", _call)


def create_linode_instance_disk_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_update tool."""
    return Tool(
        name="linode_instance_disk_update",
        description="Updates a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "New label for the disk",
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "disk_id",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_instance_disk_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_update tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, disk_id = ids

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(linode_id, disk_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_update",
            "PUT",
            f"/linode/instances/{linode_id}/disks/{disk_id}",
            _fetch,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        disk = await client.update_instance_disk(
            linode_id,
            disk_id,
            label=arguments.get("label"),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Disk {disk_id} on instance {linode_id} modified successfully"
                ),
                "disk": disk,
            },
            instance_pb2.InstanceDiskWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update instance disk", _call)


def create_linode_instance_disk_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_delete tool."""
    return Tool(
        name="linode_instance_disk_delete",
        description=(
            "Deletes a disk from a Linode instance."
            " Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
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
            "required": [
                "linode_id",
                "disk_id",
                "confirm",
            ],
        },
    ), Capability.Destroy


async def _instance_disk_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, linode_id: int, disk_id: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_instance_disk(linode_id, disk_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_disk(linode_id, disk_id)
        return {
            "message": f"Disk {disk_id} deleted from instance {linode_id} successfully",
            "linode_id": linode_id,
            "disk_id": disk_id,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_disk_delete",
        method="DELETE",
        path=f"/linode/instances/{linode_id}/disks/{disk_id}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Disk"),
    )


async def handle_linode_instance_disk_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_delete tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, disk_id = ids

    two_stage = await _instance_disk_delete_two_stage(
        arguments, cfg, linode_id, disk_id
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(linode_id, disk_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_delete",
            "DELETE",
            f"/linode/instances/{linode_id}/disks/{disk_id}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.delete_instance_disk(linode_id, disk_id)
        return {
            "message": (
                f"Disk {disk_id} deleted from instance {linode_id} successfully"
            ),
            "linode_id": linode_id,
            "disk_id": disk_id,
        }

    return await execute_tool(cfg, arguments, "delete instance disk", _call)


def create_linode_instance_disk_clone_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_clone tool."""
    return Tool(
        name="linode_instance_disk_clone",
        description="Clones a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "disk_id",
                "confirm",
            ],
        },
    ), Capability.Write


def _instance_disk_clone_side_effects(state: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for instance disk clone. Reports the new disk a
    clone creates on the same instance and the storage it consumes.
    """
    if isinstance(state, dict):
        disk = cast("dict[str, Any]", state)
        label = disk.get("label", "")
        size = disk.get("size", 0)
        return {
            "side_effects": [
                f"Disk {label!r} ({size} MB) is cloned to a new disk on the "
                f"same instance, consuming {size} MB of additional storage."
            ]
        }
    return {
        "side_effects": [
            "A copy of the disk is created on the same instance, consuming "
            "additional storage."
        ]
    }


async def handle_linode_instance_disk_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_clone tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, disk_id = ids

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(linode_id, disk_id)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_disk_clone_side_effects(state)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_clone",
            "POST",
            f"/linode/instances/{linode_id}/disks/{disk_id}/clone",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        disk = await client.clone_instance_disk(linode_id, disk_id)
        return serialize_api_response(
            {
                # Match Go's zero-value getter on the new disk id.
                "message": (
                    f"Disk {disk_id} cloned to new disk {disk.get('id', 0)}"
                    f" on instance {linode_id}"
                ),
                "disk": disk,
            },
            instance_pb2.InstanceDiskWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "clone instance disk", _call)


def create_linode_instance_disk_resize_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_resize tool."""
    return Tool(
        name="linode_instance_disk_resize",
        description="Resizes a disk on a Linode instance",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "size": {
                    "type": "integer",
                    "description": "New size in MB",
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "disk_id",
                "size",
                "confirm",
            ],
        },
    ), Capability.Write


def _instance_disk_resize_side_effects(state: Any, target_size: int) -> DryRunDetails:
    """Phase 2 Tier B walk for instance disk resize. Names the size change (in
    MB) and notes the instance must be powered off.
    """
    from_size = 0
    if isinstance(state, dict):
        from_size = cast("dict[str, Any]", state).get("size", 0)
    if from_size:
        effect = f"Disk resizes from {from_size} MB to {target_size} MB."
    else:
        effect = f"Disk resizes to {target_size} MB."
    return {
        "side_effects": [effect],
        "warnings": ["The instance must be powered off to resize a disk."],
    }


async def handle_linode_instance_disk_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_resize tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, disk_id = ids

    size = arguments.get("size")
    if not size:
        return _error_response("size is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(linode_id, disk_id)

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_disk_resize_side_effects(state, int(size))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_resize",
            "POST",
            f"/linode/instances/{linode_id}/disks/{disk_id}/resize",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.resize_instance_disk(linode_id, disk_id, int(size))
        return serialize_api_response(
            {
                "message": (
                    f"Disk {disk_id} on instance {linode_id}"
                    f" resize initiated to {size} MB"
                ),
                "linode_id": linode_id,
                "disk_id": disk_id,
                "new_size_mb": int(size),
            },
            instance_pb2.InstanceDiskResizeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "resize instance disk", _call)


def create_linode_instance_disk_password_reset_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_disk_password_reset tool."""
    return Tool(
        name="linode_instance_disk_password_reset",
        description="Resets the root password for a Linode instance disk",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "disk_id": _DISK_ID_PROP,
                "password": {
                    "type": "string",
                    "description": "New root password for the disk",
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "disk_id",
                "password",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_instance_disk_password_reset(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_disk_password_reset tool request."""
    ids = _parse_instance_and_disk_ids(arguments)
    if isinstance(ids, list):
        return ids
    linode_id, disk_id = ids

    password = arguments.get("password", "")
    if not isinstance(password, str) or not password:
        return _error_response("password is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_disk(linode_id, disk_id)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"The root password for disk {disk_id} on instance "
                    f"{linode_id} will be reset."
                ],
                "warnings": ["Existing disk root password access will be replaced."],
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_disk_password_reset",
            "POST",
            f"/linode/instances/{linode_id}/disks/{disk_id}/password",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if confirm is not True:
        return _error_response("Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.reset_instance_disk_password(linode_id, disk_id, password)
        return {
            "message": (f"Password reset for disk {disk_id} on instance {linode_id}"),
            "linode_id": linode_id,
            "disk_id": disk_id,
        }

    return await execute_tool(cfg, arguments, "reset instance disk password", _call)
