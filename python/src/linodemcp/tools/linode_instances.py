"""Linode instances list tool."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, TypeGuard, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    firewall_pb2,
    instance_pb2,
    instance_stats_pb2,
    nodebalancer_pb2,
)
from linodemcp.linode import (
    LINODE_STATS_MAX_MONTH,
    LINODE_STATS_MAX_YEAR,
    LINODE_STATS_MIN_YEAR,
)
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    required_int_id,
)
from linodemcp.tools.linode_instance_disks import validate_device_slots
from linodemcp.tools.proto_enum import enum_value_names, optional_enum_error
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient


def _is_positive_int_list(value: object) -> TypeGuard[list[int]]:
    """Return whether value is a non-empty list of positive integers."""
    if not isinstance(value, list) or not value:
        return False
    items = cast("list[object]", value)
    return all(
        isinstance(item, int) and not isinstance(item, bool) and item >= 1
        for item in items
    )


def _optional_int_argument(
    arguments: dict[str, Any],
    name: str,
    minimum: int,
    maximum: int | None = None,
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def _add_optional_string_field(
    body: dict[str, Any], arguments: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if value is not None and not isinstance(value, str):
        return f"{name} must be a string or null"
    body[name] = value
    return None


def _add_optional_positive_int_field(
    body: dict[str, Any], arguments: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return f"{name} must be a positive integer"
    body[name] = value
    return None


def _add_optional_object_field(
    body: dict[str, Any], arguments: dict[str, Any], name: str
) -> str | None:
    if name not in arguments:
        return None
    value = arguments[name]
    if value is not None and not isinstance(value, dict):
        return f"{name} must be an object or null"
    body[name] = value
    return None


def _validate_config_interface_purpose(arguments: dict[str, Any]) -> str | None:
    purpose = arguments.get("purpose")
    if isinstance(purpose, str) and purpose in enum_value_names(
        instance_pb2.ConfigInterfacePurpose.Value
    ):
        return purpose
    return None


def _add_interface_purpose_fields(
    body: dict[str, Any], arguments: dict[str, Any]
) -> str | None:
    for field in ("label", "ipam_address"):
        error = _add_optional_string_field(body, arguments, field)
        if error is not None:
            return error

    purpose = body["purpose"]
    if purpose == "vlan" and not body.get("label"):
        return "label is required for vlan interfaces"

    error = _add_optional_positive_int_field(body, arguments, "subnet_id")
    if error is not None:
        return error
    if purpose == "vpc" and "subnet_id" not in body:
        return "subnet_id is required for vpc interfaces"
    return None


def _add_interface_misc_fields(
    body: dict[str, Any], arguments: dict[str, Any]
) -> str | None:
    if "primary" in arguments:
        primary = arguments["primary"]
        if not isinstance(primary, bool):
            return "primary must be a boolean"
        body["primary"] = primary

    if "ip_ranges" in arguments:
        ip_ranges = arguments["ip_ranges"]
        if ip_ranges is not None and not isinstance(ip_ranges, list):
            return "ip_ranges must be an array of strings or null"
        ip_range_values = cast("list[object] | None", ip_ranges)
        if ip_range_values is not None and any(
            not isinstance(item, str) for item in ip_range_values
        ):
            return "ip_ranges must be an array of strings or null"
        body["ip_ranges"] = ip_ranges

    for field in ("ipv4", "ipv6"):
        error = _add_optional_object_field(body, arguments, field)
        if error is not None:
            return error
    return None


def _config_interface_add_body(arguments: dict[str, Any]) -> dict[str, Any] | str:
    purpose = _validate_config_interface_purpose(arguments)
    if purpose is None:
        return "purpose must be one of: " + ", ".join(
            enum_value_names(instance_pb2.ConfigInterfacePurpose.Value)
        )

    body: dict[str, Any] = {"purpose": purpose}
    for add_fields in (_add_interface_purpose_fields, _add_interface_misc_fields):
        error = add_fields(body, arguments)
        if error is not None:
            return error
    return body


def create_linode_instance_config_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_get tool."""
    return Tool(
        name="linode_instance_config_get",
        description="Gets a configuration profile for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceConfigGetInput"),
    ), Capability.Read


def create_linode_instance_config_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_delete tool."""
    return Tool(
        name="linode_instance_config_delete",
        description="Deletes a configuration profile from a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceConfigDeleteInput"),
    ), Capability.Destroy


def create_linode_instance_config_interface_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_delete tool."""
    return Tool(
        name="linode_instance_config_interface_delete",
        description=(
            "Deletes an interface from a Linode instance configuration profile. "
            "Requires confirm because the interface is removed from the profile."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceConfigInterfaceDeleteInput"),
    ), Capability.Destroy


def create_linode_instance_config_interface_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_get tool."""
    return Tool(
        name="linode_instance_config_interface_get",
        description="Gets an interface for a Linode instance configuration profile.",
        inputSchema=schema("linode.mcp.v1.InstanceConfigInterfaceGetInput"),
    ), Capability.Read


def create_linode_instance_interface_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_delete tool."""
    return Tool(
        name="linode_instance_interface_delete",
        description=(
            "Deletes an interface from a Linode instance. "
            "Requires confirm because the interface is removed from the Linode."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceDeleteInput"),
    ), Capability.Destroy


def create_linode_instance_interface_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_list tool."""
    return Tool(
        name="linode_instance_interface_list",
        description="Lists interfaces for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceListInput"),
    ), Capability.Read


def create_linode_instance_interface_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_settings_get tool."""
    return Tool(
        name="linode_instance_interface_settings_get",
        description="Lists interface settings for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceSettingsGetInput"),
    ), Capability.Read


def create_linode_instance_transfer_month_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_transfer_month_get tool."""
    return Tool(
        name="linode_instance_transfer_month_get",
        description="Gets monthly network transfer stats for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceTransferMonthGetInput"),
    ), Capability.Read


def create_linode_instance_interface_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_get tool."""
    return Tool(
        name="linode_instance_interface_get",
        description="Gets an interface for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceGetInput"),
    ), Capability.Read


def create_linode_instance_interface_firewall_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_firewall_list tool."""
    return Tool(
        name="linode_instance_interface_firewall_list",
        description="Lists firewalls assigned to a Linode instance interface.",
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceFirewallListInput"),
    ), Capability.Read


async def handle_linode_instance_interface_firewall_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_firewall_list tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    interface_id, error = required_int_id(arguments, "interface_id")
    if interface_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_instance_interface_firewalls(linode_id, interface_id)
        return serialize_list_response(
            raw,
            "firewalls",
            firewall_pb2.FirewallListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interface firewalls", _call
    )


def create_linode_instance_config_interface_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_list tool."""
    return Tool(
        name="linode_instance_config_interface_list",
        description="Lists interfaces for a Linode instance configuration profile.",
        inputSchema=schema("linode.mcp.v1.InstanceConfigInterfaceListInput"),
    ), Capability.Read


def create_linode_instance_interface_history_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_history_list tool."""
    return Tool(
        name="linode_instance_interface_history_list",
        description="Lists network interface history for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceHistoryListInput"),
    ), Capability.Read


def create_linode_instance_config_interface_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_update tool."""
    return Tool(
        name="linode_instance_config_interface_update",
        description=(
            "Updates an interface for a Linode instance configuration profile. "
            "Requires confirm because interface networking can change."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceConfigInterfaceUpdateInput"),
    ), Capability.Write


def create_linode_instance_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_list tool."""
    return Tool(
        name="linode_instance_list",
        description="Lists Linode instances with optional filtering by status",
        inputSchema=schema("linode.mcp.v1.InstanceListInput"),
    ), Capability.Read


def create_linode_instance_stats_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_stats_get tool."""
    return Tool(
        name="linode_instance_stats_get",
        description="Gets daily statistics for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceStatsGetInput"),
    ), Capability.Read


def create_linode_instance_nodebalancer_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_nodebalancer_list tool."""
    return Tool(
        name="linode_instance_nodebalancer_list",
        description="Lists NodeBalancers assigned to a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceNodeBalancerListInput"),
    ), Capability.Read


def create_linode_instance_stats_month_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_stats_month_get tool."""
    return Tool(
        name="linode_instance_stats_month_get",
        description=(
            "Gets a month of statistics for a Linode instance by year and month."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceStatsMonthGetInput"),
    ), Capability.Read


def create_linode_instance_transfer_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_transfer_get tool."""
    return Tool(
        name="linode_instance_transfer_get",
        description="Gets this month's network transfer stats for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceTransferGetInput"),
    ), Capability.Read


def create_linode_instance_config_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_list tool."""
    return Tool(
        name="linode_instance_config_list",
        description="Lists configuration profiles for a Linode instance.",
        inputSchema=schema("linode.mcp.v1.InstanceConfigListInput"),
    ), Capability.Read


def create_linode_instance_config_interface_reorder_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_reorder tool."""
    return Tool(
        name="linode_instance_config_interface_reorder",
        description=(
            "Reorders interfaces on a Linode instance configuration profile. "
            "Requires confirm because the active interface order can change."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceConfigInterfaceReorderInput"),
    ), Capability.Write


def create_linode_instance_config_interface_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_interface_add tool."""
    return Tool(
        name="linode_instance_config_interface_add",
        description=(
            "Adds an interface to a Linode instance configuration profile. "
            "Requires confirm because the instance network configuration changes."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceConfigInterfaceAddInput"),
    ), Capability.Write


def create_linode_instance_interface_add_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_add tool."""
    return Tool(
        name="linode_instance_interface_add",
        description=(
            "Adds an interface to a Linode instance using the current Linode "
            "Interfaces API. Requires confirm because instance networking changes."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceAddInput"),
    ), Capability.Write


def create_linode_instance_interface_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_interface_update tool."""
    return Tool(
        name="linode_instance_interface_update",
        description=(
            "Updates a Linode interface using explicit documented body sections. "
            "Requires confirm because instance networking changes."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceInterfaceUpdateInput"),
    ), Capability.Write


def create_linode_instance_config_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_config_update tool."""
    return Tool(
        name="linode_instance_config_update",
        description=(
            "Updates a configuration profile for a Linode instance. "
            "Requires confirm because the instance boot profile can change."
        ),
        inputSchema=schema("linode.mcp.v1.InstanceConfigUpdateInput"),
    ), Capability.Write


def _instance_interface_add_shape_error(interface: dict[str, Any]) -> str | None:
    """Validate the add-interface shape (mirrors Go's add-interface validator).

    Exactly one of public/vpc/vlan must be defined; a vpc needs a positive
    subnet_id and a vlan needs a non-empty vlan_label. Ported so Python rejects a
    malformed interface locally instead of forwarding it (strictest-wins).
    """
    type_count = 0
    if interface.get("public") is not None:
        type_count += 1
    vpc = interface.get("vpc")
    if vpc is not None:
        type_count += 1
        subnet_id: object = None
        if isinstance(vpc, dict):
            subnet_id = cast("dict[str, Any]", vpc).get("subnet_id")
        if (
            not isinstance(subnet_id, int)
            or isinstance(subnet_id, bool)
            or subnet_id <= 0
        ):
            return "interface.vpc.subnet_id must be a positive integer"
    vlan = interface.get("vlan")
    if vlan is not None:
        type_count += 1
        label: object = None
        if isinstance(vlan, dict):
            label = cast("dict[str, Any]", vlan).get("vlan_label")
        if not isinstance(label, str) or not label.strip():
            return "interface.vlan.vlan_label is required"
    if type_count != 1:
        return "interface must define exactly one of public, vpc, or vlan"
    return None


async def handle_linode_instance_interface_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_add tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    interface = arguments.get("interface")
    if not isinstance(interface, dict):
        return error_response("interface must be an object")
    interface_body = cast("dict[str, Any]", interface)

    shape_error = _instance_interface_add_shape_error(interface_body)
    if shape_error is not None:
        return error_response(shape_error)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_add",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/interfaces",
            None,
            request_body=interface_body,
            side_effects=[f"An interface will be added to Linode {linode_id}."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This adds a network interface to the Linode instance. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        interface_result = await client.add_instance_interface(
            linode_id, interface_body
        )
        return serialize_api_response(
            {
                "message": f"Interface added to instance {linode_id} successfully",
                "interface": interface_result,
            },
            instance_pb2.InstanceInterfaceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "add Linode instance interface", _call)


def _instance_interface_update_fields(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    fields: dict[str, Any] = {}
    for name in ("default_route", "public", "vlan", "vpc"):
        if name not in arguments:
            continue
        value = arguments[name]
        if value is not None and not isinstance(value, dict):
            return None, f"{name} must be an object or null"
        fields[name] = value

    if not fields:
        return None, "at least one update field is required"
    return fields, None


async def handle_linode_instance_interface_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_update tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    interface_id, error = required_int_id(arguments, "interface_id")
    if interface_id is None:
        return error_response(error)

    fields, fields_error = _instance_interface_update_fields(arguments)
    if fields is None:
        return error_response(fields_error or "at least one update field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_interface_update",
            arguments.get("environment", ""),
            "PUT",
            f"/linode/instances/{linode_id}/interfaces/{interface_id}",
            None,
            side_effects=[
                f"Interface {interface_id} on Linode {linode_id} will be updated."
            ],
            request_body=fields,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a network interface on the Linode instance. Set confirm=true "
            "to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_instance_interface(linode_id, interface_id, fields)
        return serialize_api_response(
            {
                "message": (
                    f"Interface {interface_id} updated on instance "
                    f"{linode_id} successfully"
                ),
                "interface": result,
            },
            instance_pb2.InstanceInterfaceWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode instance interface", _call)


async def handle_linode_instance_config_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_delete tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    # Dry-run is checked before confirm so a preview never demands the
    # confirmation flag, matching the Go destroy flow's branch order.
    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> dict[str, Any]:
            return await client.get_instance_config(linode_id, config_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_config_delete",
            "DELETE",
            f"/linode/instances/{linode_id}/configs/{config_id}",
            _fetch,
        )

    confirm = arguments.get("confirm")
    if not isinstance(confirm, bool) or not confirm:
        return error_response(
            "This is irreversible. The configuration profile will be permanently "
            "deleted. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_config(linode_id, config_id)
        return serialize_api_response(
            {
                "message": (
                    f"Configuration profile {config_id} deleted from instance "
                    f"{linode_id} successfully"
                ),
                "linode_id": linode_id,
                "config_id": config_id,
            },
            instance_pb2.InstanceConfigDeleteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "delete Linode instance configuration profile", _call
    )


async def handle_linode_instance_config_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_config(linode_id, config_id),
            instance_pb2.InstanceConfig(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance configuration profile", _call
    )


async def handle_linode_instance_config_interface_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_delete tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)
    interface_id, error = required_int_id(arguments, "interface_id")
    if interface_id is None:
        return error_response(error)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_config_interface(
                linode_id, config_id, interface_id
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_config_interface_delete",
            "DELETE",
            (
                f"/linode/instances/{linode_id}/configs/{config_id}/"
                f"interfaces/{interface_id}"
            ),
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This removes a network interface from the configuration profile. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_config_interface(
            linode_id, config_id, interface_id
        )
        return serialize_api_response(
            {
                "message": (
                    f"Configuration profile interface {interface_id} removed from "
                    f"config {config_id} on instance {linode_id}"
                ),
                "linode_id": linode_id,
                "config_id": config_id,
                "interface_id": interface_id,
            },
            instance_pb2.InstanceConfigInterfaceDeleteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "delete Linode instance config interface", _call
    )


async def handle_linode_instance_config_interface_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)
    interface_id, error = required_int_id(arguments, "interface_id")
    if interface_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_config_interface(
                linode_id, config_id, interface_id
            ),
            instance_pb2.ConfigInterfaceResponse(),
        )

    return await execute_tool(
        cfg,
        arguments,
        "retrieve Linode instance configuration profile interface",
        _call,
    )


async def handle_linode_instance_interface_delete(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_delete tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    interface_id, error = required_int_id(arguments, "interface_id")
    if interface_id is None:
        return error_response(error)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_interface(linode_id, interface_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_interface_delete",
            "DELETE",
            f"/linode/instances/{linode_id}/interfaces/{interface_id}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Linode interface and changes instance networking. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_interface(linode_id, interface_id)
        return serialize_api_response(
            {
                "message": (
                    f"Interface {interface_id} deleted from instance "
                    f"{linode_id} successfully"
                ),
                "linode_id": linode_id,
                "interface_id": interface_id,
            },
            instance_pb2.InstanceInterfaceDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete Linode instance interface", _call)


async def handle_linode_instance_interface_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_list tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # The current-generation interfaces endpoint wraps the list under
        # "interfaces", not a {data} page envelope; rewrap it for the helper.
        raw: Any = await client.list_instance_interfaces(linode_id)
        if not isinstance(raw, dict):
            raise TypeError("list response must be an object")
        envelope = cast("dict[str, Any]", raw)
        items: Any = envelope.get("interfaces", [])
        return serialize_list_response(
            {"data": items},
            "interfaces",
            instance_pb2.InstanceInterfaceListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interfaces", _call
    )


async def handle_linode_instance_interface_settings_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_settings_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_interface_settings(linode_id),
            instance_pb2.InstanceInterfaceSettings(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interface settings", _call
    )


async def handle_linode_instance_transfer_month_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_transfer_month_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    try:
        year = _optional_int_argument(
            arguments, "year", LINODE_STATS_MIN_YEAR, LINODE_STATS_MAX_YEAR
        )
        month = _optional_int_argument(arguments, "month", 1, LINODE_STATS_MAX_MONTH)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    if year is None:
        return error_response("year must be an integer")
    if month is None:
        return error_response("month must be an integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_transfer_by_year_month(linode_id, year, month),
            instance_pb2.InstanceTransferMonth(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance monthly transfer stats", _call
    )


async def handle_linode_instance_interface_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    interface_id, error = required_int_id(arguments, "interface_id")
    if interface_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_interface(linode_id, interface_id),
            instance_pb2.InstanceInterface(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance interface", _call
    )


async def handle_linode_instance_config_interface_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_list tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)
    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        items = await client.list_instance_config_interfaces(linode_id, config_id)
        return serialize_list_response(
            {"data": items},
            "interfaces",
            instance_pb2.ConfigInterfaceListResponse(),
        )

    return await execute_tool(
        cfg,
        arguments,
        "retrieve Linode instance configuration profile interfaces",
        _call,
    )


async def handle_linode_instance_interface_history_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_interface_history_list tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_instance_interface_history(
            linode_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "interface_history",
            instance_pb2.InstanceInterfaceHistoryListResponse(),
        )

    return await execute_tool(
        cfg,
        arguments,
        "retrieve Linode instance network interface history",
        _call,
    )


def _instance_config_interface_update_fields(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any] | None, str | None]:
    fields: dict[str, Any] = {}
    if "ip_ranges" in arguments:
        value = arguments["ip_ranges"]
        if not isinstance(value, list) or not all(
            isinstance(item, str) for item in cast("list[object]", value)
        ):
            return None, "ip_ranges must be an array of strings"
        fields["ip_ranges"] = value
    if "ipv4" in arguments:
        value = arguments["ipv4"]
        if not isinstance(value, dict):
            return None, "ipv4 must be an object"
        fields["ipv4"] = value
    if "primary" in arguments:
        value = arguments["primary"]
        if not isinstance(value, bool):
            return None, "primary must be a boolean"
        fields["primary"] = value

    if not fields:
        return None, "at least one update field is required"
    return fields, None


async def handle_linode_instance_config_interface_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_update tool request."""
    ids: dict[str, int] = {}
    for key in ("linode_id", "config_id", "interface_id"):
        value, error = required_int_id(arguments, key)
        if value is None:
            return error_response(error)
        ids[key] = value
    linode_id = ids["linode_id"]
    config_id = ids["config_id"]
    interface_id = ids["interface_id"]

    fields, fields_error = _instance_config_interface_update_fields(arguments)
    if fields is None:
        return error_response(fields_error or "at least one update field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interface_update",
            arguments.get("environment", ""),
            "PUT",
            (
                f"/linode/instances/{linode_id}/configs/{config_id}"
                f"/interfaces/{interface_id}"
            ),
            None,
            side_effects=[
                (
                    f"Interface {interface_id} on configuration profile {config_id} "
                    f"for Linode {linode_id} will be updated."
                )
            ],
            request_body=fields,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a network interface on the configuration profile. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_instance_config_interface(
            linode_id, config_id, interface_id, fields
        )
        return serialize_api_response(
            {
                "message": (
                    f"Configuration profile interface {interface_id} updated"
                    f" on config {config_id} for instance {linode_id}"
                ),
                "interface": result,
            },
            instance_pb2.ConfigInterfaceWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "update Linode instance config interface", _call
    )


async def handle_linode_instance_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_list tool request.

    Args:
        arguments: InstanceFilterArgs - environment, status (optional)
        cfg: Configuration object
    """
    status_filter = arguments.get("status", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/linode/instances")
        if not status_filter:
            return serialize_list_response(
                raw, "instances", instance_pb2.InstanceListResponse()
            )
        return serialize_list_response(
            raw,
            "instances",
            instance_pb2.InstanceListResponse(),
            filter_value=f"status={status_filter}",
            item_filter=lambda inst: (
                str(inst.get("status", "")).lower() == status_filter.lower()
            ),
        )

    return await execute_tool(cfg, arguments, "retrieve Linode instances", _call)


async def handle_linode_instance_stats_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_stats_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_stats(linode_id),
            instance_stats_pb2.InstanceStats(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance statistics", _call
    )


async def handle_linode_instance_nodebalancer_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_nodebalancer_list tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_instance_nodebalancers(linode_id)
        return serialize_list_response(
            raw,
            "nodebalancers",
            nodebalancer_pb2.NodeBalancerListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance NodeBalancers", _call
    )


async def handle_linode_instance_stats_month_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_stats_month_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    try:
        year = _optional_int_argument(arguments, "year", 1970, 9999)
        month = _optional_int_argument(arguments, "month", 1, 12)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    if year is None:
        return error_response("year must be an integer")
    if month is None:
        return error_response("month must be an integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_stats_by_year_month(linode_id, year, month),
            instance_stats_pb2.InstanceStats(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance monthly statistics", _call
    )


async def handle_linode_instance_transfer_get(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_transfer_get tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_instance_transfer(linode_id),
            instance_pb2.InstanceTransfer(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance network transfer stats", _call
    )


async def handle_linode_instance_config_list(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_list tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_instance_configs(
            linode_id, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw,
            "configs",
            instance_pb2.InstanceConfigListResponse(),
        )

    return await execute_tool(
        cfg, arguments, "retrieve Linode instance configuration profiles", _call
    )


def _reorder_ids_error(ids: object) -> str | None:
    """Validate the reorder ids: a non-empty positive-int list with no duplicates.

    Mirrors Go's buildReorderConfigInterfacesRequest so both reject a duplicate
    interface id locally instead of forwarding it (strictest-wins).
    """
    if not _is_positive_int_list(ids):
        return "ids must be a non-empty list of positive integers"
    if len(set(ids)) != len(ids):
        return "ids must not contain duplicate interface IDs"
    return None


async def handle_linode_instance_config_interface_reorder(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_reorder tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    ids = arguments.get("ids")
    ids_error = _reorder_ids_error(ids)
    if ids_error is not None:
        return error_response(ids_error)
    ids = cast("list[int]", ids)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interface_reorder",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/configs/{config_id}/interfaces/order",
            None,
            side_effects=[
                (
                    f"Interfaces on configuration profile {config_id} for Linode "
                    f"{linode_id} will be reordered."
                )
            ],
            request_body={"ids": ids},
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This reorders network interfaces on the configuration profile. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.reorder_instance_config_interfaces(linode_id, config_id, ids)
        return serialize_api_response(
            {
                "message": (
                    f"Configuration profile {config_id} interfaces reordered on "
                    f"instance {linode_id}"
                ),
                "linode_id": linode_id,
                "config_id": config_id,
                "ids": ids,
            },
            instance_pb2.InstanceConfigInterfaceReorderResponse(),
        )

    return await execute_tool(
        cfg, arguments, "reorder Linode instance config interfaces", _call
    )


async def handle_linode_instance_config_interface_add(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_interface_add tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    body = _config_interface_add_body(arguments)
    if isinstance(body, str):
        return error_response(body)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_interface_add",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{linode_id}/configs/{config_id}/interfaces",
            None,
            side_effects=[
                (
                    f"An interface will be added to configuration profile {config_id} "
                    f"on Linode {linode_id}."
                )
            ],
            request_body=body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This adds a network interface to the configuration profile. Set "
            "confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.add_instance_config_interface(linode_id, config_id, body)
        return serialize_api_response(
            {
                "message": (
                    f"Configuration profile interface added to"
                    f" config {config_id} on instance {linode_id}"
                ),
                "interface": result,
            },
            instance_pb2.ConfigInterfaceWriteResponse(),
        )

    return await execute_tool(
        cfg, arguments, "add Linode instance config interface", _call
    )


def _config_update_field_error(
    arguments: dict[str, Any], fields: dict[str, Any]
) -> str | None:
    """Return the first config-update field error, or None when valid.

    Follows the Go update handler's order (the run_level/virt_mode enums, then
    the device slot names, then the at-least-one-field requirement) so both
    languages surface the same error first for a given payload.
    """
    for enum_key, enum in (
        ("run_level", instance_pb2.ConfigRunLevel.Value),
        ("virt_mode", instance_pb2.ConfigVirtMode.Value),
    ):
        enum_error = optional_enum_error(arguments, enum_key, enum)
        if enum_error is not None:
            return enum_error

    devices = arguments.get("devices")
    if isinstance(devices, dict):
        slot_error = validate_device_slots(cast("dict[str, Any]", devices))
        if slot_error is not None:
            return slot_error

    if not fields:
        return "at least one update field is required"
    return None


async def handle_linode_instance_config_update(
    arguments: dict[str, Any], cfg: Any
) -> list[TextContent]:
    """Handle linode_instance_config_update tool request."""
    linode_id, error = required_int_id(arguments, "linode_id")
    if linode_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    fields: dict[str, Any] = {}
    for key in (
        "comments",
        "devices",
        "helpers",
        "interfaces",
        "kernel",
        "label",
        "memory_limit",
        "root_device",
        "run_level",
        "virt_mode",
    ):
        value = arguments.get(key)
        if value is not None:
            fields[key] = value

    field_error = _config_update_field_error(arguments, fields)
    if field_error is not None:
        return error_response(field_error)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_config_update",
            arguments.get("environment", ""),
            "PUT",
            f"/linode/instances/{linode_id}/configs/{config_id}",
            None,
            side_effects=[
                (
                    f"Configuration profile {config_id} on Linode {linode_id} "
                    "will be updated."
                )
            ],
            request_body=fields,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a configuration profile on the instance. Set confirm=true to "
            "proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_instance_config(linode_id, config_id, fields)
        return serialize_api_response(
            {
                # Match Go's zero-value getters on an empty API body: label "",
                # id 0, so both languages emit the same message string.
                "message": (
                    f"Configuration profile '{result.get('label', '')}'"
                    f" (ID: {result.get('id', 0)}) updated on instance {linode_id}"
                ),
                "config": result,
            },
            instance_pb2.InstanceConfigWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Linode instance config", _call)
