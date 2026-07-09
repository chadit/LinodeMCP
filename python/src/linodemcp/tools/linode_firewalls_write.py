from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeGuard, cast

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    firewall_device_pb2,
    firewall_pb2,
    instance_pb2,
)
from linodemcp.linode import APIError, NetworkError
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.proto_enum import enum_choice_error, optional_enum_error
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _is_firewall_rule_list(value: Any) -> TypeGuard[list[dict[str, Any]]]:
    if not isinstance(value, list):
        return False
    rules = cast("list[object]", value)
    return all(isinstance(rule, dict) for rule in rules)


def _is_default_firewall_ids(value: Any) -> TypeGuard[dict[str, int]]:
    if not isinstance(value, dict):
        return False
    ids = cast("dict[str, object]", value)
    valid_keys = {"linode", "nodebalancer", "public_interface", "vpc_interface"}
    if not ids or set(ids) - valid_keys:
        return False
    return all(
        type(firewall_id) is int and firewall_id > 0 for firewall_id in ids.values()
    )


def _positive_int_argument(
    arguments: dict[str, Any], name: str
) -> tuple[int | None, str | None]:
    """Option-B id validation. Returns None (not "") on success so the existing
    ``if error is not None`` call sites stay correct; the text matches
    required_int_id."""
    if name not in arguments:
        return None, f"{name} is required"
    value = arguments[name]
    if isinstance(value, bool) or not isinstance(value, int) or value < 1:
        return None, f"{name} must be a positive integer"
    return value, None


def _firewall_policy_error(arguments: dict[str, Any]) -> str | None:
    """Validate inbound_policy/outbound_policy against the FirewallPolicy enum,
    mirroring Go so both languages reject the same values. A policy is validated
    only when present; the API defaults an absent policy to ACCEPT.
    """
    for key in ("inbound_policy", "outbound_policy"):
        policy_error = optional_enum_error(
            arguments, key, firewall_pb2.FirewallPolicy.Value
        )
        if policy_error is not None:
            return policy_error
    return None


def create_linode_firewall_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_create tool."""
    return Tool(
        name="linode_firewall_create",
        description=(
            "Creates a new Cloud Firewall. The firewall is created with no rules."
        ),
        inputSchema=schema("linode.mcp.v1.FirewallCreateInput"),
    ), Capability.Write


async def handle_linode_firewall_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_create tool request."""
    label = arguments.get("label", "")
    inbound_policy = arguments.get("inbound_policy", "ACCEPT")
    outbound_policy = arguments.get("outbound_policy", "ACCEPT")

    policy_error = _firewall_policy_error(arguments)
    if policy_error is not None:
        return error_response(policy_error)

    if is_dry_run(arguments):
        if not label:
            return error_response("label is required")
        return build_dry_run_response(
            "linode_firewall_create",
            arguments.get("environment", ""),
            "POST",
            "/networking/firewalls",
            None,
            side_effects=[
                f"A new Cloud Firewall {label!r} will be created with inbound "
                f"policy {inbound_policy} and outbound policy {outbound_policy}."
            ],
        )

    if not arguments.get("confirm"):
        return error_response(
            "This creates a Cloud Firewall. Set confirm=true to proceed."
        )

    if not label:
        return error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.create_firewall_raw(
            label=label,
            inbound_policy=inbound_policy,
            outbound_policy=outbound_policy,
        )
        return serialize_api_response(
            {
                "message": (
                    f"Firewall '{firewall.get('label', '')}' "
                    f"(ID: {firewall.get('id', 0)}) created successfully"
                ),
                "firewall": firewall,
            },
            firewall_pb2.FirewallWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create firewall", _call)


def create_linode_firewall_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_update tool."""
    return Tool(
        name="linode_firewall_update",
        description="Updates an existing Cloud Firewall.",
        inputSchema=schema("linode.mcp.v1.FirewallUpdateInput"),
    ), Capability.Write


def _firewall_update_side_effects(
    state: Any, new_label: Any, new_status: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for firewall update. Reports the label change and a
    status change (enabled/disabled) against the fetched state.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f"Label changes from {from_label!r} to {new_label!r}.")
        else:
            side_effects.append(f"Label is set to {new_label!r}.")
    if new_status:
        from_status = getattr(state, "status", "")
        if new_status != from_status:
            verb = "stops enforcing" if new_status == "disabled" else "starts enforcing"
            side_effects.append(
                f"Firewall status changes to {new_status!r}; this immediately "
                f"{verb} its rules."
            )
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_update tool request."""
    firewall_id = arguments.get("firewall_id", 0)

    if not firewall_id:
        return error_response("firewall_id is required")

    policy_error = _firewall_policy_error(arguments)
    if policy_error is not None:
        return error_response(policy_error)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_firewall(int(firewall_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _firewall_update_side_effects(
                state, arguments.get("label"), arguments.get("status")
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_firewall_update",
            "PUT",
            f"/networking/firewalls/{int(firewall_id)}",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Cloud Firewall. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.update_firewall_raw(
            firewall_id=int(firewall_id),
            label=arguments.get("label"),
            status=arguments.get("status"),
            inbound_policy=arguments.get("inbound_policy"),
            outbound_policy=arguments.get("outbound_policy"),
        )
        return serialize_api_response(
            {
                "message": f"Firewall {int(firewall_id)} modified successfully",
                "firewall": firewall,
            },
            firewall_pb2.FirewallWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update firewall", _call)


def create_linode_firewall_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_delete tool."""
    return Tool(
        name="linode_firewall_delete",
        description=(
            "Deletes a Cloud Firewall. WARNING: This removes all rules "
            "and unassigns all devices."
            " Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.FirewallDeleteInput"),
    ), Capability.Destroy


async def _firewall_delete_dependency_walk(
    client: RetryableClient, firewall_id: int
) -> DryRunDetails:
    """Phase 2 Tier A walk for firewall delete. The Linodes and NodeBalancers
    attached to a firewall survive the delete but lose its rules, so each
    attached device is surfaced as a removed dependency. Best-effort: a failed
    device list becomes a warning, not a hard error.
    """
    details: DryRunDetails = {}
    try:
        response = await client.list_firewall_devices(firewall_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list firewall devices: {exc}"]
        return details

    devices = cast("list[dict[str, Any]]", response.get("data", []))
    dependencies: list[dict[str, Any]] = []
    for device in devices:
        entity = cast("dict[str, Any]", device.get("entity") or {})
        dependencies.append(
            {
                "kind": entity.get("type", ""),
                "id": entity.get("id"),
                "label": entity.get("label", ""),
                "action": "removed",
                "note": "Loses this firewall's rules when the firewall is deleted.",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            f"{len(dependencies)} resource(s) currently use this firewall "
            "and will lose its rules."
        ]
    return details


async def _firewall_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, firewall_id_int: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_firewall(firewall_id_int)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall(firewall_id_int)
        return serialize_api_response(
            {
                "message": f"Firewall {firewall_id_int} removed successfully",
                "firewall_id": firewall_id_int,
            },
            firewall_pb2.FirewallDeleteResponse(),
        )

    async def _ts_walk(client: RetryableClient, _state: Any) -> DryRunDetails:
        return await _firewall_delete_dependency_walk(client, firewall_id_int)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_firewall_delete",
        method="DELETE",
        path=f"/networking/firewalls/{firewall_id_int}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Firewall"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_firewall_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_delete tool request."""
    firewall_id = arguments.get("firewall_id", 0)

    # Both branches need a non-zero firewall_id, and the spec says
    # dry-run errors on missing required args the same way the real
    # call would.
    if not firewall_id:
        return error_response("firewall_id is required")

    firewall_id_int = int(firewall_id)

    two_stage = await _firewall_delete_two_stage(arguments, cfg, firewall_id_int)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_firewall(firewall_id_int)

        async def _walk(client: RetryableClient, _state: Any) -> DryRunDetails:
            return await _firewall_delete_dependency_walk(client, firewall_id_int)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_firewall_delete",
            "DELETE",
            f"/networking/firewalls/{firewall_id_int}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)

    if not confirm:
        return error_response(
            "This operation is destructive. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall(firewall_id_int)
        return serialize_api_response(
            {
                "message": f"Firewall {firewall_id_int} removed successfully",
                "firewall_id": firewall_id_int,
            },
            firewall_pb2.FirewallDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete firewall", _call)


def create_linode_firewall_device_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_delete tool."""
    return Tool(
        name="linode_firewall_device_delete",
        description=(
            "Deletes a device assignment from a Cloud Firewall. "
            "WARNING: This operation requires confirmation."
            " Pass dry_run=true to preview without removing."
        )
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.FirewallDeviceDeleteInput"),
    ), Capability.Destroy


async def _firewall_device_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, firewall_id: int, device_id: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_firewall_device(firewall_id, device_id)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall_device(firewall_id, device_id)
        return serialize_api_response(
            {
                "message": "Firewall device removed successfully",
                "firewall_id": firewall_id,
                "device_id": device_id,
            },
            firewall_device_pb2.FirewallDeviceDeleteResponse(),
        )

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_firewall_device_delete",
        method="DELETE",
        path=f"/networking/firewalls/{firewall_id}/devices/{device_id}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("FirewallDevice"),
    )


async def handle_linode_firewall_device_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_device_delete tool request."""
    # Both branches need valid positive IDs, and the spec says dry-run
    # errors on missing/invalid required args the same way the real
    # call would.
    firewall_id, error = _positive_int_argument(arguments, "firewall_id")
    if error is not None:
        return error_response(error)
    device_id, error = _positive_int_argument(arguments, "device_id")
    if error is not None:
        return error_response(error)

    firewall_id_value = cast("int", firewall_id)
    device_id_value = cast("int", device_id)

    two_stage = await _firewall_device_delete_two_stage(
        arguments, cfg, firewall_id_value, device_id_value
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_firewall_device(firewall_id_value, device_id_value)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_firewall_device_delete",
            "DELETE",
            f"/networking/firewalls/{firewall_id_value}/devices/{device_id_value}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This removes a device assignment from a Cloud Firewall. Set confirm=true "
            "to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall_device(firewall_id_value, device_id_value)
        return serialize_api_response(
            {
                "message": "Firewall device removed successfully",
                "firewall_id": firewall_id_value,
                "device_id": device_id_value,
            },
            firewall_device_pb2.FirewallDeviceDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete firewall device", _call)


def create_linode_firewall_rules_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rules_update tool."""
    return Tool(
        name="linode_firewall_rules_update",
        description=(
            "Replaces the inbound and outbound rules for a Cloud Firewall. "
            "WARNING: This overwrites all existing rules."
        ),
        inputSchema=schema("linode.mcp.v1.FirewallRulesUpdateInput"),
    ), Capability.Write


def _firewall_rules_fields_error(arguments: dict[str, Any]) -> str | None:
    firewall_id = arguments.get("firewall_id", 0)

    if not firewall_id:
        return "firewall_id is required"
    if not isinstance(firewall_id, int) or isinstance(firewall_id, bool):
        return "firewall_id must be an integer"
    if firewall_id <= 0:
        return "firewall_id must be a positive integer"

    for field in ("inbound", "outbound"):
        rules_raw: Any = arguments.get(field)
        if rules_raw is None:
            return f"{field} is required"
        if not _is_firewall_rule_list(rules_raw):
            return f"{field} must be a list of rule objects"

    return None


def _firewall_rules_update_validation_error(arguments: dict[str, Any]) -> str | None:
    if arguments.get("confirm") is not True:
        return "This replaces Cloud Firewall rules. Set confirm=true to proceed."

    return _firewall_rules_fields_error(arguments)


async def handle_linode_firewall_rules_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rules_update tool request."""
    firewall_id = arguments.get("firewall_id", 0)

    if is_dry_run(arguments):
        fields_error = _firewall_rules_fields_error(arguments)
        if fields_error is not None:
            return error_response(fields_error)

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_firewall_rules(int(firewall_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_firewall_rules_update",
            "PUT",
            f"/networking/firewalls/{int(firewall_id)}/rules",
            _fetch,
        )

    validation_error = _firewall_rules_update_validation_error(arguments)
    if validation_error is not None:
        return error_response(validation_error)

    inbound = cast("list[dict[str, Any]]", arguments.get("inbound"))
    outbound = cast("list[dict[str, Any]]", arguments.get("outbound"))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_firewall_rules_raw(
            firewall_id=firewall_id,
            inbound=inbound,
            outbound=outbound,
        )
        return serialize_api_response(
            {
                "message": f"Firewall {firewall_id} rules updated successfully",
                "firewall_id": firewall_id,
                "rules": result,
            },
            firewall_pb2.FirewallRulesWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update firewall rules", _call)


def create_linode_instance_firewall_apply_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_firewall_apply tool."""
    return Tool(
        name="linode_instance_firewall_apply",
        description=(
            "Applies the currently assigned Cloud Firewalls to a Linode instance."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceFirewallApplyInput"),
    ), Capability.Write


async def handle_linode_instance_firewall_apply(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_firewall_apply tool request."""
    linode_id, error = _positive_int_argument(arguments, "linode_id")
    if error is not None:
        return error_response(error)

    linode_id_value = cast("int", linode_id)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_firewall_apply",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id_value}/firewalls/apply",
            None,
            side_effects=[
                "Cloud Firewall assignments will be applied to "
                f"Linode {linode_id_value}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This reapplies assigned firewalls to the Linode. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.apply_linode_firewalls(linode_id_value)
        return serialize_api_response(
            {
                "message": f"Firewall apply initiated for instance {linode_id_value}",
                "linode_id": linode_id_value,
            },
            instance_pb2.InstanceActionWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "apply Linode firewalls", _call)


def create_linode_firewall_settings_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_settings_update tool."""
    return Tool(
        name="linode_firewall_settings_update",
        description=(
            "Updates the account default firewalls for Linodes, NodeBalancers, "
            "public interfaces, and VPC interfaces."
        ),
        inputSchema=schema("linode.mcp.v1.FirewallSettingsUpdateInput"),
    ), Capability.Write


def _firewall_settings_ids_error(raw: Any) -> list[TextContent] | None:
    if not _is_default_firewall_ids(raw):
        return error_response(
            "default_firewall_ids must be a non-empty object of positive "
            "integer firewall IDs keyed by linode, nodebalancer, "
            "public_interface, or vpc_interface"
        )
    return None


async def handle_linode_firewall_settings_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_settings_update tool request."""
    default_firewall_ids_raw = arguments.get("default_firewall_ids")

    if is_dry_run(arguments):
        ids_error = _firewall_settings_ids_error(default_firewall_ids_raw)
        if ids_error is not None:
            return ids_error

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_firewall_settings()

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_firewall_settings_update",
            "PUT",
            "/networking/firewalls/settings",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates default Cloud Firewall assignments. Set confirm=true to "
            "proceed."
        )

    ids_error = _firewall_settings_ids_error(default_firewall_ids_raw)
    if ids_error is not None:
        return ids_error

    default_firewall_ids = cast("dict[str, int]", default_firewall_ids_raw)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.update_firewall_settings(default_firewall_ids)
        return serialize_api_response(
            {
                "message": "Default firewall settings updated successfully",
                "settings": raw,
            },
            firewall_pb2.FirewallSettingsWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update default firewalls", _call)


def create_linode_firewall_device_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_create tool."""
    return Tool(
        name="linode_firewall_device_create",
        description=(
            "Creates a new device for a Cloud Firewall. "
            "WARNING: This operation requires confirmation."
        ),
        inputSchema=schema("linode.mcp.v1.FirewallDeviceCreateInput"),
    ), Capability.Write


def _firewall_device_type_error(arguments: dict[str, Any]) -> str | None:
    """Validate the device type: present, a non-empty string, and one of the
    FirewallDeviceType enum values (linode, nodebalancer, linode_interface). The
    closed-set check mirrors Go so both languages reject the same values.
    """
    if "type" not in arguments:
        return "type is required"

    device_type = arguments.get("type", "")
    if not isinstance(device_type, str):
        return "type must be a string"
    if not device_type.strip():
        return "type must be a non-empty string"

    return enum_choice_error(
        device_type, "type", firewall_device_pb2.FirewallDeviceType.Value
    )


def _firewall_device_create_fields_error(
    arguments: dict[str, Any],
) -> list[TextContent] | None:
    """Validate device-create fields; return an error response or None."""
    _, error = _positive_int_argument(arguments, "firewall_id")
    if error is not None:
        return error_response(error)

    _, error = _positive_int_argument(arguments, "id")
    if error is not None:
        return error_response(error)

    type_error = _firewall_device_type_error(arguments)
    if type_error is not None:
        return error_response(type_error)

    return None


async def handle_linode_firewall_device_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_device_create tool request."""
    if is_dry_run(arguments):
        fields_error = _firewall_device_create_fields_error(arguments)
        if fields_error is not None:
            return fields_error

        firewall_id = int(arguments["firewall_id"])
        device_type = arguments.get("type", "")
        device_id = int(arguments["id"])
        return build_dry_run_response(
            "linode_firewall_device_create",
            arguments.get("environment", ""),
            "POST",
            f"/networking/firewalls/{firewall_id}/devices",
            None,
            side_effects=[
                f"The {device_type} {device_id} will be attached to "
                f"firewall {firewall_id}."
            ],
        )

    if not arguments.get("confirm"):
        return error_response(
            "This assigns a device to a Cloud Firewall. Set confirm=true to proceed."
        )

    fields_error = _firewall_device_create_fields_error(arguments)
    if fields_error is not None:
        return fields_error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.create_firewall_device(
            firewall_id=int(arguments["firewall_id"]),
            device_id=int(arguments["id"]),
            device_type=str(arguments["type"]),
        )
        return serialize_api_response(
            {"message": "Firewall device assigned successfully", "device": raw},
            firewall_device_pb2.FirewallDeviceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create firewall device", _call)
