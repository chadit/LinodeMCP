from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import firewall_pb2, instance_pb2
from linodemcp.linode import APIError, NetworkError, instance_preview_state
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    TWO_STAGE_OPT_IN_NOTE,
    WALK_PAGE_SIZE,
    DryRunDetails,
    build_dry_run_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    preview_state_str,
    required_int_id,
    walk_page_items,
)
from linodemcp.tools.proto_response import (
    raw_int,
    raw_str,
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
        inputSchema=schema("linode.mcp.v1.InstanceBootInput"),
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
            return instance_preview_state(await client.get_instance(int(instance_id)))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_boot",
            "POST",
            f"/linode/instances/{int(instance_id)}/boot",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return _error_response(
            "This boots a Linode instance. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.boot_instance(int(instance_id), config_id)
        return serialize_api_response(
            {
                "message": f"Instance {instance_id} boot initiated successfully",
                "instance_id": int(instance_id),
            },
            instance_pb2.InstancePowerActionResponse(),
        )

    return await execute_tool(cfg, arguments, "boot instance", _call)


def create_linode_instance_reboot_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_reboot tool."""
    return Tool(
        name="linode_instance_reboot",
        description="Reboots a running Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceRebootInput"),
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
            return instance_preview_state(await client.get_instance(int(instance_id)))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_reboot",
            "POST",
            f"/linode/instances/{int(instance_id)}/reboot",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return _error_response(
            "This reboots a Linode instance and causes a brief outage. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.reboot_instance(int(instance_id), config_id)
        return serialize_api_response(
            {
                "message": f"Instance {instance_id} reboot initiated successfully",
                "instance_id": int(instance_id),
            },
            instance_pb2.InstancePowerActionResponse(),
        )

    return await execute_tool(cfg, arguments, "reboot instance", _call)


def create_linode_instance_shutdown_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_shutdown tool."""
    return Tool(
        name="linode_instance_shutdown",
        description="Shuts down a running Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceShutdownInput"),
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
            return instance_preview_state(await client.get_instance(int(instance_id)))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_shutdown",
            "POST",
            f"/linode/instances/{int(instance_id)}/shutdown",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return _error_response(
            "This shuts down a Linode instance. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.shutdown_instance(int(instance_id))
        return serialize_api_response(
            {
                "message": f"Instance {instance_id} shutdown initiated successfully",
                "instance_id": int(instance_id),
            },
            instance_pb2.InstancePowerActionResponse(),
        )

    return await execute_tool(cfg, arguments, "shutdown instance", _call)


def create_linode_instance_firewall_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_firewall_update tool."""
    return Tool(
        name="linode_instance_firewall_update",
        description=(
            "Replaces the firewall assignments for a Linode instance. "
            "Pass an empty firewall_ids list to remove all assignments."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceFirewallUpdateInput"),
    ), Capability.Write


async def handle_linode_instance_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_firewall_update tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return _error_response(error)

    firewall_ids = _firewall_ids_argument(arguments)
    if firewall_ids is None:
        return _error_response("firewall_ids must be a list of positive integers")

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
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
        return _error_response(
            "This replaces firewall assignments for a Linode instance. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.update_instance_firewalls(
            linode_id, firewall_ids, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw, "firewalls", firewall_pb2.FirewallListResponse()
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
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceSettingsUpdateInput"),
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
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return _error_response(error)
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
    linode_id_int = linode_id

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
        return _error_response(
            "This updates interface settings for the Linode instance. Set confirm=true "
            "to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        settings_result = await client.update_instance_interface_settings(
            linode_id_int,
            default_route=default_route,
            network_helper=network_helper if "network_helper" in arguments else None,
        )
        return serialize_api_response(
            {
                "message": (
                    f"Interface settings for instance "
                    f"{linode_id_int} updated successfully"
                ),
                "settings": settings_result,
            },
            instance_pb2.InstanceInterfaceSettingsWriteResponse(),
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
        inputSchema=schema("linode.mcp.v1.InstanceCreateInput"),
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
        return _error_response(
            "This operation creates a billable resource. Set confirm=true to proceed."
        )

    fields_error = _instance_create_error(region, instance_type, firewall_id)
    if fields_error is not None:
        return _error_response(fields_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.create_instance_raw(
            region=region,
            instance_type=instance_type,
            firewall_id=firewall_id,
            image=arguments.get("image"),
            label=arguments.get("label"),
            root_pass=arguments.get("root_pass"),
            authorized_keys=arguments.get("authorized_keys"),
            booted=arguments.get("booted"),
            backups_enabled=arguments.get("backups_enabled", False),
            route_ipv4=arguments.get("route_ipv4", True),
            route_ipv6=arguments.get("route_ipv6", True),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Instance '{raw_str(raw, 'label')}' "
                    f"(ID: {raw_int(raw, 'id')}) "
                    f"created successfully in {raw_str(raw, 'region')}"
                ),
                "instance": raw,
            },
            instance_pb2.InstanceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create instance", _call)


def create_linode_instance_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_update tool."""
    return Tool(
        name="linode_instance_update",
        description="Updates editable fields on a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceUpdateInput"),
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
            return instance_preview_state(await client.get_instance(iid))

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
        return _error_response(
            "This updates a Linode instance. Set confirm=true to proceed."
        )

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
        raw = await client.update_instance_raw(int(instance_id), **update_fields)
        return serialize_api_response(
            {
                "message": f"Instance {raw_int(raw, 'id')} updated successfully",
                "instance": raw,
            },
            instance_pb2.InstanceWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.InstanceDeleteInput"),
    ), Capability.Destroy


async def _instance_volume_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Volumes attached to the instance detach (not destroy) on delete."""
    try:
        page = await client.list_instance_volumes(instance_id, 1, WALK_PAGE_SIZE)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list attached volumes: {exc}"]

    deps = [
        {
            "kind": "volume",
            "id": volume.get("id"),
            "label": str(volume.get("label", "")),
            "action": "detached",
            "note": f"{volume.get('size', 0)}GB volume stays; billing continues.",
        }
        for volume in walk_page_items(page)
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


async def _instance_firewall_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Firewalls survive the delete; the instance drops from their devices."""
    try:
        page = await client.list_instance_firewalls(instance_id, 1, WALK_PAGE_SIZE)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list firewalls: {exc}"]

    deps = [
        {
            "kind": "firewall",
            "id": firewall.get("id"),
            "label": str(firewall.get("label", "")),
            "action": "removed",
            "note": "Firewall stays; this instance is removed from its device list.",
        }
        for firewall in walk_page_items(page)
    ]

    return deps, []


async def _instance_billing_delta(
    client: RetryableClient, type_id: str
) -> dict[str, Any]:
    """Best-effort monthly cost change from deleting an instance of type_id.

    Mirrors the Go instanceBillingDelta: an empty type or a failed pricing
    fetch degrades to the "unknown" sentinel instead of failing the preview.
    """
    if not type_id:
        return {"monthly_change_usd": "unknown"}

    try:
        instance_type = await client.get_type(type_id)
    except (APIError, NetworkError, httpx.HTTPError):
        return {
            "monthly_change_usd": "unknown",
            "note": "Could not fetch type pricing for the estimate.",
        }

    return {
        "monthly_change_usd": f"-{instance_type.price.monthly:.2f}",
        "note": "Instance billing stops. Attached volume billing continues.",
    }


async def _instance_delete_dependency_walk(
    client: RetryableClient, instance_id: int, state: Any
) -> DryRunDetails:
    """Phase 2 Tier A walk for instance delete. Mirrors the Go
    instanceDeleteDependencyWalk: attached volumes detach, public IPv4
    addresses release, firewall attachments drop, the monthly billing change
    is estimated, and a running instance adds a shutdown warning.
    Best-effort: a failed sub-fetch becomes a warning, not an error.
    """
    dependencies: list[dict[str, Any]] = []
    warnings: list[str] = []

    for collect in (_instance_volume_deps, _instance_ip_deps, _instance_firewall_deps):
        deps, deps_warnings = await collect(client, instance_id)
        dependencies.extend(deps)
        warnings.extend(deps_warnings)

    billing = await _instance_billing_delta(client, preview_state_str(state, "type"))

    if preview_state_str(state, "status") == "running":
        warnings.append(
            "Instance is currently running. Delete will not pause for a "
            "graceful shutdown."
        )

    details: DryRunDetails = {"billing_delta": billing}
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
        return instance_preview_state(await client.get_instance(int(instance_id)))

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance(int(instance_id))
        return serialize_api_response(
            {
                "message": f"Instance {instance_id} removed successfully",
                "instance_id": instance_id,
            },
            instance_pb2.InstanceDeleteResponse(),
        )

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
            return instance_preview_state(await client.get_instance(int(instance_id)))

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
        return _error_response(
            "This operation is destructive and irreversible. "
            "Set confirm=true to proceed."
        )

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance(int(instance_id))
        return serialize_api_response(
            {
                "message": f"Instance {instance_id} removed successfully",
                "instance_id": instance_id,
            },
            instance_pb2.InstanceDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete instance", _call)


def create_linode_instance_mutate_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_mutate tool."""
    return Tool(
        name="linode_instance_mutate",
        description=(
            "Upgrades a Linode using the mutate endpoint. "
            "WARNING: This changes instance state and may resize disks."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceMutateInput"),
    ), Capability.Write


async def handle_linode_instance_mutate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_mutate tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return _error_response(error)

    # Pass allow_auto_disk_resize only when the caller set it explicitly; an
    # omitted value defers to the API default (true), matching Go, which only
    # sets the field when the argument is present.
    allow_auto_disk_resize: bool | None = None
    if "allow_auto_disk_resize" in arguments:
        raw_allow = arguments["allow_auto_disk_resize"]
        if not isinstance(raw_allow, bool):
            return _error_response("allow_auto_disk_resize must be a boolean")
        allow_auto_disk_resize = raw_allow

    if is_dry_run(arguments):
        request_body = (
            {"allow_auto_disk_resize": allow_auto_disk_resize}
            if allow_auto_disk_resize is not None
            else None
        )
        return build_dry_run_response(
            "linode_instance_mutate",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/mutate",
            None,
            side_effects=[f"Linode {linode_id} will be upgraded."],
            warnings=["The Linode may be unavailable during the upgrade."],
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return _error_response(
            "This upgrades the instance and may cause downtime. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.mutate_instance(
            linode_id, allow_auto_disk_resize=allow_auto_disk_resize
        )
        return serialize_api_response(
            {
                "message": f"Upgrade initiated for instance {linode_id}",
                "linode_id": linode_id,
            },
            instance_pb2.InstanceActionWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "mutate Linode instance", _call)


def create_linode_instance_interface_upgrade_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_upgrade tool."""
    return Tool(
        name="linode_instance_interface_upgrade",
        description="Upgrades a Linode to Linode Interfaces.",
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceUpgradeInput"),
    ), Capability.Write


async def handle_linode_instance_interface_upgrade(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_interface_upgrade tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return _error_response(error)

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
        return _error_response(
            "This irreversibly upgrades Linode network interfaces. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.upgrade_instance_interfaces(
            linode_id, config_id=config_id, dry_run=api_dry_run
        )
        return serialize_api_response(
            {
                "message": f"Linode {linode_id} interface upgrade initiated",
                **result,
            },
            instance_pb2.InstanceInterfaceUpgradeWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.InstanceResizeInput"),
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
            allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", False),
            migration_type=arguments.get("migration_type", ""),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Instance {instance_id} resize to {instance_type} "
                    "initiated successfully"
                ),
                "instance_id": instance_id,
                "new_type": instance_type,
            },
            instance_pb2.InstanceResizeWriteResponse(),
        )

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
            return instance_preview_state(await client.get_instance(int(instance_id)))

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
        return _error_response(
            "This operation causes downtime and may affect billing. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.resize_instance(
            instance_id=int(instance_id),
            instance_type=instance_type,
            allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", False),
            migration_type=arguments.get("migration_type", ""),
        )
        return serialize_api_response(
            {
                "message": (
                    f"Instance {instance_id} resize to {instance_type} "
                    "initiated successfully"
                ),
                "instance_id": instance_id,
                "new_type": instance_type,
            },
            instance_pb2.InstanceResizeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "resize instance", _call)
