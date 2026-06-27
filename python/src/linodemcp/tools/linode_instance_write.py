from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

import httpx
from mcp.types import TextContent, Tool

from linodemcp.linode import APIError, NetworkError, instance_to_response_dict
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    MODE_PROP,
    PARAM_DRY_RUN,
    PARAM_MODE,
    PARAM_PLAN_ID,
    PLAN_ID_PROP,
    TWO_STAGE_NOTE,
    TWO_STAGE_OPT_IN_NOTE,
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


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


def _positive_int_argument(arguments: dict[str, Any], name: str) -> int | None:
    value = arguments.get(name)
    if isinstance(value, bool) or not isinstance(value, int) or value < 1:
        return None
    return value


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def _firewall_ids_argument(arguments: dict[str, Any]) -> list[int] | None:
    raw_value: object = arguments.get("firewall_ids")
    if not isinstance(raw_value, list):
        return None

    firewall_ids: list[int] = []
    for item in cast("list[object]", raw_value):
        if isinstance(item, bool) or not isinstance(item, int) or item < 1:
            return None
        firewall_ids.append(item)
    return firewall_ids


def create_linode_instance_boot_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_boot tool."""
    return Tool(
        name="linode_instance_boot",
        description="Boots a Linode instance that is currently offline.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to boot (required)",
                },
                "config_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the config profile to boot with (optional)"
                    ),
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
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_boot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_boot tool request."""
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_boot",
            "POST",
            f"/linode/instances/{int(instance_id)}/boot",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.boot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} boot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "boot instance", _call)


def create_linode_instance_reboot_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_reboot tool."""
    return Tool(
        name="linode_instance_reboot",
        description="Reboots a running Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to reboot (required)",
                },
                "config_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the config profile to reboot with (optional)"
                    ),
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
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_reboot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_reboot tool request."""
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_reboot",
            "POST",
            f"/linode/instances/{int(instance_id)}/reboot",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.reboot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} reboot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "reboot instance", _call)


def create_linode_instance_shutdown_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_shutdown tool."""
    return Tool(
        name="linode_instance_shutdown",
        description="Shuts down a running Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to shutdown (required)",
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
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_shutdown(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_shutdown tool request."""
    instance_id = arguments.get("instance_id", 0)

    if not instance_id:
        return _error_response("instance_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_shutdown",
            "POST",
            f"/linode/instances/{int(instance_id)}/shutdown",
            _fetch,
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.shutdown_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} shutdown initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "shutdown instance", _call)


def create_linode_instance_firewall_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_firewall_update tool."""
    return Tool(
        name="linode_instance_firewall_update",
        description=(
            "Replaces the firewall assignments for a Linode instance. "
            "Pass an empty firewall_ids list to remove all assignments."
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
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode instance (required)",
                },
                "firewall_ids": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": (
                        "Complete list of Firewall IDs to assign. Use [] to remove all."
                    ),
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of assigned Firewall results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of assigned Firewall results per page",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to replace Linode firewall assignments. "
                        "Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "firewall_ids", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_firewall_update tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return _error_response("linode_id must be a positive integer")

    firewall_ids = _firewall_ids_argument(arguments)
    if firewall_ids is None:
        return _error_response("firewall_ids must be a list of positive integers")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return _error_response(str(exc))

    request_body = {"firewall_ids": firewall_ids}
    dry_run_path = f"/linode/instances/{linode_id}/firewalls"
    dry_run_query: dict[str, int] = {}
    if page is not None:
        dry_run_query["page"] = page
    if page_size is not None:
        dry_run_query["page_size"] = page_size
    if dry_run_query:
        dry_run_path += "?" + "&".join(
            f"{name}={value}" for name, value in dry_run_query.items()
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_firewall_update",
            arguments.get("environment", ""),
            "PUT",
            dry_run_path,
            None,
            side_effects=[
                f"Firewall assignments for Linode {linode_id} will be replaced."
            ],
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return _error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_instance_firewalls(
            linode_id, firewall_ids, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "update Linode firewall assignments", _call
    )


def create_linode_instance_interface_settings_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_settings_update tool."""
    return Tool(
        name="linode_instance_interface_settings_update",
        description=(
            "Updates Network Helper and default route settings on a Linode. "
            "Power off the Linode before enabling or disabling Network Helper."
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
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode to update (required)",
                },
                "network_helper": {
                    "type": "boolean",
                    "description": "Enable or disable Network Helper (optional)",
                },
                "default_route": {
                    "type": "object",
                    "description": (
                        "Default route interface IDs. Supports ipv4_interface_id "
                        "and ipv6_interface_id values, each integer or null."
                    ),
                    "additionalProperties": False,
                    "minProperties": 1,
                    "properties": {
                        "ipv4_interface_id": {
                            "type": ["integer", "null"],
                            "minimum": 1,
                        },
                        "ipv6_interface_id": {
                            "type": ["integer", "null"],
                            "minimum": 1,
                        },
                    },
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to update interface settings. "
                        "Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "confirm"],
        },
    ), Capability.Write


def _interface_settings_update_error(
    arguments: dict[str, Any],
    linode_id: int | None,
    network_helper: object,
    default_route: dict[str, int | None] | None,
) -> str | None:
    """Validate interface settings update arguments."""
    if linode_id is None:
        return "linode_id must be a positive integer"
    if "network_helper" in arguments and not isinstance(network_helper, bool):
        return "network_helper must be a boolean"
    if "network_helper" not in arguments and default_route is None:
        return "network_helper or default_route is required"
    return None


def _interface_settings_default_route_argument(
    arguments: dict[str, Any],
) -> dict[str, int | None] | None:
    if "default_route" not in arguments:
        return None
    raw_default_route = arguments["default_route"]
    if not isinstance(raw_default_route, dict):
        raise TypeError("default_route must be an object")

    default_route_input = cast("dict[str, Any]", raw_default_route)
    allowed_fields = {"ipv4_interface_id", "ipv6_interface_id"}
    unknown_fields = set(default_route_input) - allowed_fields
    if unknown_fields:
        raise ValueError(
            "default_route supports only ipv4_interface_id and ipv6_interface_id"
        )

    default_route: dict[str, int | None] = {}
    for key in sorted(allowed_fields):
        if key not in default_route_input:
            continue
        value = default_route_input[key]
        if value is not None and (
            isinstance(value, bool) or not isinstance(value, int) or value < 1
        ):
            raise ValueError(f"default_route.{key} must be a positive integer or null")
        default_route[key] = value
    if not default_route:
        raise ValueError(
            "default_route must include ipv4_interface_id or ipv6_interface_id"
        )
    return default_route


async def handle_linode_instance_interface_settings_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_interface_settings_update tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    network_helper = arguments.get("network_helper")

    try:
        default_route = _interface_settings_default_route_argument(arguments)
    except (TypeError, ValueError) as exc:
        return _error_response(str(exc))

    validation_error = _interface_settings_update_error(
        arguments, linode_id, network_helper, default_route
    )
    if validation_error is not None:
        return _error_response(validation_error)
    linode_id_int = cast("int", linode_id)

    request_body: dict[str, Any] = {}
    if default_route is not None:
        request_body["default_route"] = default_route
    if "network_helper" in arguments:
        request_body["network_helper"] = network_helper

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_settings_update",
            arguments.get("environment", ""),
            "PUT",
            f"/linode/instances/{linode_id_int}/interfaces/settings",
            None,
            side_effects=[
                f"Interface settings for Linode {linode_id_int} will be updated."
            ],
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return _error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_instance_interface_settings(
            linode_id_int,
            default_route=default_route,
            network_helper=network_helper if "network_helper" in arguments else None,
        )

    return await execute_tool(cfg, arguments, "update Linode interface settings", _call)


def create_linode_instance_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_create tool."""
    return Tool(
        name="linode_instance_create",
        description=(
            "Creates a new Linode instance under the current Linode Interfaces "
            "generation. WARNING: Billing starts immediately. Requires "
            "firewall_id (get one from linode_firewall_list or create with "
            "linode_firewall_create). Note: VPC attachment via the current "
            "interface model is not yet supported by this tool; use "
            "linode_vpc_* tools after create."
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
                    "description": (
                        "Region where the instance will be created (required)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": "Instance type/plan (required)",
                },
                "image": {
                    "type": "string",
                    "description": "Image ID to deploy (optional)",
                },
                "label": {
                    "type": "string",
                    "description": "Label for the instance (optional)",
                },
                "root_pass": {
                    "type": "string",
                    "description": "Root password (required if image is provided)",
                },
                "authorized_keys": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "SSH public keys to add (optional)",
                },
                "booted": {
                    "type": "boolean",
                    "description": "Whether to boot the instance (default: true)",
                },
                "backups_enabled": {
                    "type": "boolean",
                    "description": "Enable backups (default: false)",
                },
                "firewall_id": {
                    "type": "integer",
                    "description": (
                        "Cloud Firewall ID to attach to the public interface. "
                        "Required under the current Linode Interfaces generation."
                    ),
                },
                "route_ipv4": {
                    "type": "boolean",
                    "description": (
                        "Whether the public interface owns the IPv4 default "
                        "route (optional, default: true)"
                    ),
                },
                "route_ipv6": {
                    "type": "boolean",
                    "description": (
                        "Whether the public interface owns the IPv6 default "
                        "route (optional, default: true)"
                    ),
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
            "required": ["region", "type", "firewall_id", "confirm"],
        },
    ), Capability.Write


def _instance_create_error(
    region: str, instance_type: str, firewall_id: Any
) -> str | None:
    """Validate instance create args; return an error message or None."""
    if not region:
        return "region is required"
    if not instance_type:
        return "type is required"
    if not firewall_id or firewall_id <= 0:
        return (
            "firewall_id is required for instance creation. Get a firewall ID "
            "from linode_firewall_list, or create one with linode_firewall_create."
        )
    return None


async def handle_linode_instance_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_create tool request."""
    region = arguments.get("region", "")
    instance_type = arguments.get("type", "")
    firewall_id = arguments.get("firewall_id", 0)

    if is_dry_run(arguments):
        fields_error = _instance_create_error(region, instance_type, firewall_id)
        if fields_error is not None:
            return _error_response(fields_error)
        image = arguments.get("image")
        effect = f"A new {instance_type} instance will be created in region {region}"
        if image:
            effect += f" from image {image}"
        return build_dry_run_response(
            "linode_instance_create",
            arguments.get("environment", ""),
            "POST",
            "/linode/instances",
            None,
            side_effects=[f"{effect}."],
            warnings=["Billing for the instance starts immediately on creation."],
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    fields_error = _instance_create_error(region, instance_type, firewall_id)
    if fields_error is not None:
        return _error_response(fields_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.create_instance(
            region=region,
            instance_type=instance_type,
            firewall_id=firewall_id,
            image=arguments.get("image"),
            label=arguments.get("label"),
            root_pass=arguments.get("root_pass"),
            authorized_keys=arguments.get("authorized_keys"),
            booted=arguments.get("booted", True),
            backups_enabled=arguments.get("backups_enabled", False),
            route_ipv4=arguments.get("route_ipv4", True),
            route_ipv6=arguments.get("route_ipv6", True),
        )
        return {
            "message": (
                f"Instance '{instance.label}' (ID: {instance.id}) "
                f"created successfully in {instance.region}"
            ),
            "instance": instance_to_response_dict(instance),
        }

    return await execute_tool(cfg, arguments, "create instance", _call)


def create_linode_instance_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_update tool."""
    return Tool(
        name="linode_instance_update",
        description="Updates editable fields on a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New Linode label (optional)",
                },
                "group": {
                    "type": "string",
                    "description": "Deprecated group label (optional)",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "Tags to assign to the Linode (optional)",
                },
                "alerts": {
                    "type": "object",
                    "description": "Alert threshold settings (optional)",
                },
                "maintenance_policy": {
                    "type": "string",
                    "description": (
                        "Maintenance policy, such as linode/migrate (optional)"
                    ),
                },
                "watchdog_enabled": {
                    "type": "boolean",
                    "description": (
                        "Whether Lassie shutdown watchdog is enabled (optional)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm update.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_update tool request."""
    instance_id = arguments.get("instance_id", 0)

    if is_dry_run(arguments):
        if not instance_id:
            return _error_response("instance_id is required")
        iid = int(instance_id)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(iid)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_update",
            "PUT",
            f"/linode/instances/{iid}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)

    if not confirm:
        return _error_response("Set confirm=true to update the instance.")

    if not instance_id:
        return _error_response("instance_id is required")

    update_fields = {
        key: arguments[key]
        for key in (
            "label",
            "group",
            "tags",
            "alerts",
            "maintenance_policy",
            "watchdog_enabled",
        )
        if key in arguments
    }

    if not update_fields:
        return _error_response(
            "at least one update field is required: label, group, tags, alerts, "
            "maintenance_policy, or watchdog_enabled"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.update_instance(int(instance_id), **update_fields)
        return {
            "message": f"Instance {instance.id} updated successfully",
            "instance": instance_to_response_dict(instance),
        }

    return await execute_tool(cfg, arguments, "update instance", _call)


def create_linode_instance_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_delete tool."""
    return Tool(
        name="linode_instance_delete",
        description=(
            "Deletes a Linode instance. WARNING: This is destructive and cannot "
            "be undone. All data will be lost. Pass dry_run=true to preview "
            "without deleting."
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
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["instance_id", "confirm"],
        },
    ), Capability.Destroy


async def _instance_volume_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Volumes attached to the instance detach (not destroy) on delete."""
    try:
        volumes = await client.list_volumes()
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list volumes: {exc}"]

    deps = [
        {
            "kind": "volume",
            "id": volume.id,
            "label": volume.label,
            "action": "detached",
            "note": f"{volume.size}GB volume stays; billing continues.",
        }
        for volume in volumes
        if volume.linode_id == instance_id
    ]

    return deps, []


async def _instance_ip_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Public IPv4 addresses are released back to the pool on delete."""
    try:
        ips = await client.list_instance_ips(instance_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list IP addresses: {exc}"]

    ipv4 = cast("dict[str, Any]", ips.get("ipv4", {}))
    public = cast("list[dict[str, Any]]", ipv4.get("public", []))
    deps: list[dict[str, Any]] = [
        {
            "kind": "public_ip",
            "label": str(addr.get("address", "")),
            "action": "released",
        }
        for addr in public
    ]

    return deps, []


async def _instance_delete_dependency_walk(
    client: RetryableClient, instance_id: int, state: Any
) -> DryRunDetails:
    """Phase 2 Tier A walk for instance delete. Best-effort: a failed
    sub-fetch becomes a warning, not an error. Firewall attachments and the
    billing estimate are omitted from this preview because the Python client
    lacks the per-instance firewall list and type-pricing lookups; that
    coverage is tracked for a later pass.
    """
    dependencies: list[dict[str, Any]] = []
    warnings: list[str] = []

    for collect in (_instance_volume_deps, _instance_ip_deps):
        deps, deps_warnings = await collect(client, instance_id)
        dependencies.extend(deps)
        warnings.extend(deps_warnings)

    warnings.append(
        "Firewall attachments and the billing estimate are not included in "
        "this preview."
    )

    if getattr(state, "status", "") == "running":
        warnings.append(
            "Instance is currently running. Delete will not pause for a "
            "graceful shutdown."
        )

    details: DryRunDetails = {}
    if dependencies:
        details["dependencies"] = dependencies
    if warnings:
        details["warnings"] = warnings

    return details


async def _instance_delete_two_stage(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    instance_id = arguments.get("instance_id", 0)
    if not instance_id:
        return _error_response("instance_id is required")

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_instance(int(instance_id))

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} deleted successfully",
            "instance_id": instance_id,
        }

    async def _ts_walk(client: RetryableClient, state: Any) -> DryRunDetails:
        return await _instance_delete_dependency_walk(client, int(instance_id), state)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_delete",
        method="DELETE",
        path=f"/linode/instances/{int(instance_id)}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Instance"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_instance_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_delete tool request."""
    instance_id = arguments.get("instance_id", 0)

    two_stage = await _instance_delete_two_stage(arguments, cfg)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        if not instance_id:
            return _error_response("instance_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        async def _walk(client: RetryableClient, state: Any) -> DryRunDetails:
            return await _instance_delete_dependency_walk(
                client, int(instance_id), state
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_delete",
            "DELETE",
            f"/linode/instances/{int(instance_id)}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} deleted successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "delete instance", _call)


def create_linode_instance_mutate_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_mutate tool."""
    return Tool(
        name="linode_instance_mutate",
        description=(
            "Upgrades a Linode using the mutate endpoint. "
            "WARNING: This changes instance state and may resize disks."
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
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode to mutate (required)",
                },
                "allow_auto_disk_resize": {
                    "type": "boolean",
                    "description": (
                        "Automatically resize disks when resizing a Linode "
                        "(optional, default: true)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm upgrade. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_mutate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_mutate tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return _error_response("linode_id must be a positive integer")

    allow_auto_disk_resize = arguments.get("allow_auto_disk_resize", True)
    if not isinstance(allow_auto_disk_resize, bool):
        return _error_response("allow_auto_disk_resize must be a boolean")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_mutate",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/mutate",
            None,
            side_effects=[f"Linode {linode_id} will be upgraded."],
            warnings=["The Linode may be unavailable during the upgrade."],
            request_body={"allow_auto_disk_resize": allow_auto_disk_resize},
        )

    if arguments.get("confirm") is not True:
        return _error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.mutate_instance(
            linode_id, allow_auto_disk_resize=allow_auto_disk_resize
        )
        return {
            "message": f"Linode {linode_id} upgrade initiated",
            "linode_id": linode_id,
            "allow_auto_disk_resize": allow_auto_disk_resize,
            "response": result,
        }

    return await execute_tool(cfg, arguments, "mutate Linode instance", _call)


def create_linode_instance_interface_upgrade_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_upgrade tool."""
    return Tool(
        name="linode_instance_interface_upgrade",
        description="Upgrades a Linode to Linode Interfaces.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "linode_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "The ID of the Linode to upgrade (required)",
                },
                "config_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Config profile ID to upgrade (optional)",
                },
                "api_dry_run": {
                    "type": "boolean",
                    "description": (
                        "Pass dry_run to the Linode API to validate the upgrade "
                        "without applying it (optional)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to upgrade instance interfaces. "
                        "Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["linode_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_instance_interface_upgrade(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_interface_upgrade tool request."""
    linode_id = _positive_int_argument(arguments, "linode_id")
    if linode_id is None:
        return _error_response("linode_id must be a positive integer")

    try:
        config_id = _optional_int_argument(arguments, "config_id", 1)
    except (TypeError, ValueError) as exc:
        return _error_response(str(exc))

    api_dry_run = arguments.get("api_dry_run")
    if "api_dry_run" in arguments and not isinstance(api_dry_run, bool):
        return _error_response("api_dry_run must be a boolean")

    request_body: dict[str, Any] = {}
    if config_id is not None:
        request_body["config_id"] = config_id
    if "api_dry_run" in arguments:
        request_body["dry_run"] = api_dry_run

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_upgrade",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/upgrade-interfaces",
            None,
            side_effects=[f"Linode {linode_id} will be upgraded to Linode Interfaces."],
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return _error_response("confirm must be true")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.upgrade_instance_interfaces(
            linode_id, config_id=config_id, dry_run=api_dry_run
        )
        return {
            "message": f"Linode {linode_id} interface upgrade initiated",
            "linode_id": linode_id,
            "response": result,
        }

    return await execute_tool(cfg, arguments, "upgrade Linode interfaces", _call)


def create_linode_instance_resize_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_resize tool."""
    return Tool(
        name="linode_instance_resize",
        description=(
            "Resizes a Linode instance to a different plan. "
            "WARNING: This may cause downtime and billing changes."
            + TWO_STAGE_OPT_IN_NOTE
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
                "instance_id": {
                    "type": "integer",
                    "description": "The ID of the instance to resize (required)",
                },
                "type": {
                    "type": "string",
                    "description": "The new instance type/plan (required)",
                },
                "allow_auto_disk_resize": {
                    "type": "boolean",
                    "description": (
                        "Auto-resize disks to fit new plan (default: true)"
                    ),
                },
                "migration_type": {
                    "type": "string",
                    "description": "Migration type: 'warm' or 'cold' (default: 'warm')",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm resize. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["instance_id", "type", "confirm"],
        },
    ), Capability.Write


def _resize_from_type(state: Any) -> str:
    """Read the current instance type from whichever state shape the resize walk
    receives: the bare instance on the dry-run path (attribute access) or the
    composite projection dict on the two-stage path.
    """
    if isinstance(state, dict):
        state_map = cast("dict[str, Any]", state)
        return str(state_map.get("type", ""))
    return str(getattr(state, "type", ""))


def _instance_resize_side_effects(from_type: str, target_type: str) -> DryRunDetails:
    """Phase 2 Tier B walk for instance resize. Names the type change (from the
    fetched state to the requested type) and warns about reboot and billing.
    """
    if from_type:
        effect = (
            f"Instance resizes from type {from_type} to {target_type}; it "
            "reboots and is unavailable during the resize."
        )
    else:
        effect = (
            f"Instance resizes to type {target_type}; it reboots and is "
            "unavailable during the resize."
        )
    return {
        "side_effects": [effect],
        "warnings": ["Resizing changes the monthly price to match the new type."],
    }


async def _fetch_instance_resize_state(
    client: RetryableClient, instance_id: int
) -> Any:
    """Build the composite resize projection: the instance type plus each disk's
    id, size, and filesystem. Resize affects both the plan and the disks, so the
    drift hash must cover both. The projection holds only drift-relevant fields,
    so cosmetic instance changes never refuse an apply and no hash-ignore list is
    needed. Mirrors the Go fetchInstanceResizeState.
    """
    instance = await client.get_instance(instance_id)
    try:
        disks: list[dict[str, Any]] = await client.list_instance_disks(instance_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        msg = f"list disks for resize plan: {exc}"
        raise ValueError(msg) from exc

    disk_snapshot = [
        {
            "id": disk.get("id"),
            "size": disk.get("size"),
            "filesystem": disk.get("filesystem"),
        }
        for disk in disks
    ]
    return {"type": _resize_from_type(instance), "disks": disk_snapshot}


async def _instance_resize_two_stage(
    arguments: dict[str, Any], cfg: Config, instance_id: int, instance_type: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through.

    Capability is Write, so resize stays opt-in: a plan/apply call resizes only
    when an operator enables linode_instance_resize in the two_stage config.
    """
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await _fetch_instance_resize_state(client, instance_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.resize_instance(
            instance_id=instance_id,
            instance_type=instance_type,
            allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", True),
            migration_type=arguments.get("migration_type", "warm"),
        )
        return {
            "message": f"Instance {instance_id} resize to {instance_type} initiated",
            "instance_id": instance_id,
            "new_type": instance_type,
        }

    async def _ts_walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _instance_resize_side_effects(_resize_from_type(state), instance_type)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_resize",
        method="POST",
        path=f"/linode/instances/{instance_id}/resize",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        dependency_walk=_ts_walk,
        capability=Capability.Write,
    )


async def handle_linode_instance_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_resize tool request."""
    instance_id = arguments.get("instance_id", 0)
    instance_type = arguments.get("type", "")

    if not instance_id:
        return _error_response("instance_id is required")
    if not instance_type:
        return _error_response("type is required")

    two_stage = await _instance_resize_two_stage(
        arguments, cfg, int(instance_id), str(instance_type)
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance(int(instance_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _instance_resize_side_effects(
                _resize_from_type(state), str(instance_type)
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_resize",
            "POST",
            f"/linode/instances/{int(instance_id)}/resize",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return [
            TextContent(
                type="text",
                text="Error: This may cause downtime. Set confirm=true to proceed.",
            )
        ]

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.resize_instance(
            instance_id=int(instance_id),
            instance_type=instance_type,
            allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", True),
            migration_type=arguments.get("migration_type", "warm"),
        )
        return {
            "message": (f"Instance {instance_id} resize to {instance_type} initiated"),
            "instance_id": instance_id,
            "new_type": instance_type,
        }

    return await execute_tool(cfg, arguments, "resize instance", _call)
