from __future__ import annotations

import ipaddress
from typing import TYPE_CHECKING, Any, cast

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
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_LINODE_ID_PROP: dict[str, Any] = {
    "type": "integer",
    "minimum": 1,
    "description": "The ID of the Linode instance (required)",
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
    if not raw:
        return _error_response("linode_id is required")
    try:
        return int(raw)
    except (ValueError, TypeError):
        return _error_response("linode_id must be a valid integer")


def create_linode_instance_ip_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_ip_list tool."""
    return Tool(
        name="linode_instance_ip_list",
        description=("Lists IP addresses for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
            },
            "required": ["linode_id"],
        },
    ), Capability.Read


async def handle_linode_instance_ip_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_list tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.list_instance_ips(iid)

    return await execute_tool(cfg, arguments, "list instance IPs", _call)


def create_linode_instance_ip_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_ip_get tool."""
    return Tool(
        name="linode_instance_ip_get",
        description=("Gets details of a specific IP for an instance"),
        inputSchema=schema("linode.mcp.v1.InstanceIPGetInput"),
    ), Capability.Read


async def handle_linode_instance_ip_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_get tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    address = _parse_ip_address_argument(arguments)
    if isinstance(address, list):
        return address

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return ip_address_to_response_dict(await client.get_instance_ip(iid, address))

    return await execute_tool(cfg, arguments, "get instance IP", _call)


def _parse_ip_address_argument(arguments: dict[str, Any]) -> str | list[TextContent]:
    """Parse and validate an IP address argument."""
    address = arguments.get("address")
    if not address:
        return _error_response("address is required")
    if not isinstance(address, str):
        return _error_response("address must be a string")
    try:
        ipaddress.ip_address(address)
    except ValueError:
        return _error_response("address must be a valid IP address")
    return address


def ip_address_to_response_dict(ip: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw networking IP API dict to proto-canonical IPAddress form.

    vpc_nat_1_1 is omitted when null; other null scalars coerce to the zero value
    to match the Go struct decoding.
    """
    body: dict[str, Any] = {
        "address": ip.get("address") or "",
        "gateway": ip.get("gateway") or "",
        "subnet_mask": ip.get("subnet_mask") or "",
        "prefix": ip.get("prefix") or 0,
        "type": ip.get("type") or "",
        "public": ip.get("public") or False,
        "rdns": ip.get("rdns") or "",
        "linode_id": ip.get("linode_id") or 0,
        "region": ip.get("region") or "",
    }
    if ip.get("vpc_nat_1_1") is not None:
        body["vpc_nat_1_1"] = ip["vpc_nat_1_1"]
    return body


def create_linode_networking_ip_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ip_get tool."""
    return Tool(
        name="linode_networking_ip_get",
        description=("Gets details of a networking-level IP address"),
        inputSchema=schema("linode.mcp.v1.IPAddressGetInput"),
    ), Capability.Read


async def handle_linode_networking_ip_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ip_get tool request."""
    address = _parse_ip_address_argument(arguments)
    if isinstance(address, list):
        return address

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return ip_address_to_response_dict(await client.get_networking_ip(address))

    return await execute_tool(cfg, arguments, "get networking IP", _call)


def create_linode_instance_ip_allocate_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_ip_allocate tool."""
    return Tool(
        name="linode_instance_ip_allocate",
        description=("Allocates a new IP address for a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "type": {
                    "type": "string",
                    "description": ("IP type: ipv4 or ipv6 (required)"),
                },
                "public": {
                    "type": "boolean",
                    "description": ("Whether the IP is public (default true)"),
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "type",
                "public",
                "confirm",
            ],
        },
    ), Capability.Write


async def handle_linode_instance_ip_allocate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_allocate tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    ip_type = arguments.get("type", "")
    if not ip_type:
        return _error_response("type is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_instance_ip_allocate",
            arguments.get("environment", ""),
            "POST",
            f"/linode/instances/{iid}/ips",
            None,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    public = arguments.get("public", True)

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.allocate_instance_ip(iid, ip_type=ip_type, public=public)

    return await execute_tool(cfg, arguments, "allocate instance IP", _call)


def create_linode_instance_ip_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_ip_update tool."""
    return Tool(
        name="linode_instance_ip_update",
        description=("Updates reverse DNS for an IP address on a Linode instance"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "address": {
                    "type": "string",
                    "description": ("The IP address to update (required)"),
                },
                "rdns": {
                    "type": "string",
                    "description": ("The reverse DNS value to assign (required)"),
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "address",
                "rdns",
                "confirm",
            ],
        },
    ), Capability.Write


def _parse_instance_ip_update(
    arguments: dict[str, Any],
) -> tuple[int, str, str | None] | list[TextContent]:
    """Parse linode_id, address, and rdns; return the triple or an error."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid
    address = _parse_ip_address_argument(arguments)
    if isinstance(address, list):
        return address
    if "rdns" not in arguments:
        return _error_response("rdns is required")
    rdns = arguments.get("rdns")
    if rdns is not None and not isinstance(rdns, str):
        return _error_response("rdns must be a string or null")
    return iid, address, rdns


async def handle_linode_instance_ip_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_update tool request."""
    if is_dry_run(arguments):
        parsed = _parse_instance_ip_update(arguments)
        if isinstance(parsed, list):
            return parsed
        iid, address, _ = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_ip(iid, address)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_ip_update",
            "PUT",
            f"/linode/instances/{iid}/ips/{address}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    parsed = _parse_instance_ip_update(arguments)
    if isinstance(parsed, list):
        return parsed
    iid, address, rdns = parsed

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.update_instance_ip(iid, address, rdns)

    return await execute_tool(cfg, arguments, "update instance IP", _call)


def create_linode_networking_ip_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ip_update tool."""
    return Tool(
        name="linode_networking_ip_update",
        description=("Updates reverse DNS for a networking-level IP address"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "address": {
                    "type": "string",
                    "description": ("The IP address to update (required)"),
                },
                "rdns": {
                    "type": "string",
                    "description": ("The reverse DNS value to assign (required)"),
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "address",
                "rdns",
                "confirm",
            ],
        },
    ), Capability.Write


def _parse_networking_ip_update(
    arguments: dict[str, Any],
) -> tuple[str, str | None] | list[TextContent]:
    """Parse address and rdns; return the pair or an error response."""
    address = _parse_ip_address_argument(arguments)
    if isinstance(address, list):
        return address
    if "rdns" not in arguments:
        return _error_response("rdns is required")
    rdns = arguments.get("rdns")
    if rdns is not None and not isinstance(rdns, str):
        return _error_response("rdns must be a string or null")
    return address, rdns


def _networking_ip_update_side_effects(new_rdns: Any) -> DryRunDetails:
    """Phase 2 Tier B walk for networking IP update. Reports the reverse-DNS
    (rDNS) change, or its removal when the rdns value is empty.
    """
    if new_rdns:
        return {"side_effects": [f"Reverse DNS (rDNS) is set to {new_rdns!r}."]}
    return {"side_effects": ["Reverse DNS (rDNS) is cleared."]}


async def handle_linode_networking_ip_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ip_update tool request."""
    if is_dry_run(arguments):
        parsed = _parse_networking_ip_update(arguments)
        if isinstance(parsed, list):
            return parsed
        address, rdns = parsed

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_networking_ip(address)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return _networking_ip_update_side_effects(rdns)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_networking_ip_update",
            "PUT",
            f"/networking/ips/{address}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    parsed = _parse_networking_ip_update(arguments)
    if isinstance(parsed, list):
        return parsed
    address, rdns = parsed

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        ip = await client.update_networking_ip(address, rdns)
        return {
            "message": f"Networking IP {address} RDNS updated",
            "ip": ip_address_to_response_dict(ip),
        }

    return await execute_tool(cfg, arguments, "update networking IP", _call)


def create_linode_networking_ip_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ip_list tool."""
    return Tool(
        name="linode_networking_ip_list",
        description="Lists public IP addresses on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "skip_ipv6_rdns": {
                    "type": "boolean",
                    "description": (
                        "When true, omit IPv6 reverse DNS data from the response"
                    ),
                },
            },
        },
    ), Capability.Read


async def handle_linode_networking_ip_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ip_list tool request."""
    skip_ipv6_rdns = arguments.get("skip_ipv6_rdns", False)
    if not isinstance(skip_ipv6_rdns, bool):
        return _error_response("skip_ipv6_rdns must be a boolean")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        ips = await client.list_networking_ips(skip_ipv6_rdns=skip_ipv6_rdns)
        return {"count": len(ips), "ips": ips}

    return await execute_tool(cfg, arguments, "list networking IPs", _call)


def create_linode_networking_ip_allocate_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_ip_allocate tool."""
    return Tool(
        name="linode_networking_ip_allocate",
        description=("Allocates a new IP address at the networking level for a Linode"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": {
                    "type": "integer",
                    "description": ("The Linode ID to allocate the IP for"),
                },
                "type": {
                    "type": "string",
                    "description": ("IP type: ipv4 or ipv6 (required)"),
                },
                "public": {
                    "type": "boolean",
                    "description": ("Whether the IP is public (default true)"),
                },
                "confirm": _CONFIRM_PROP,
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": [
                "linode_id",
                "type",
                "public",
                "confirm",
            ],
        },
    ), Capability.Write


def _networking_ip_allocate_error(arguments: dict[str, Any]) -> str | None:
    """Validate allocate args; return a joined error string or None."""
    errors: list[str] = []
    linode_id = arguments.get("linode_id")
    if (
        linode_id is None
        or not isinstance(linode_id, int)
        or isinstance(linode_id, bool)
    ):
        errors.append("linode_id must be an integer")

    ip_type = arguments.get("type", "")
    if not ip_type or not isinstance(ip_type, str):
        errors.append("type must be a non-empty string")
    elif ip_type not in ("ipv4", "ipv6"):
        errors.append("type must be ipv4 or ipv6")

    public = arguments.get("public", True)
    if not isinstance(public, bool):
        errors.append("public must be a boolean")

    if errors:
        return "; ".join(errors)
    return None


async def handle_linode_networking_ip_allocate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_ip_allocate tool request."""
    if is_dry_run(arguments):
        validation_error = _networking_ip_allocate_error(arguments)
        if validation_error is not None:
            return _error_response(validation_error)
        return build_dry_run_response(
            "linode_networking_ip_allocate",
            arguments.get("environment", ""),
            "POST",
            "/networking/ips",
            None,
        )

    if not arguments.get("confirm"):
        return _error_response("Set confirm=true to proceed.")

    validation_error = _networking_ip_allocate_error(arguments)
    if validation_error is not None:
        return _error_response(validation_error)

    linode_id = arguments.get("linode_id")
    ip_type = arguments.get("type", "")
    public = arguments.get("public", True)

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        return await client.allocate_networking_ip(
            cast("int", linode_id), ip_type=cast("str", ip_type), public=public
        )

    return await execute_tool(cfg, arguments, "allocate networking IP", _call)


def create_linode_instance_ip_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_instance_ip_delete tool."""
    return Tool(
        name="linode_instance_ip_delete",
        description=(
            "Deletes an IP address from a Linode instance."
            " Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "linode_id": _LINODE_ID_PROP,
                "address": {
                    "type": "string",
                    "description": ("The IP address to delete (required)"),
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
            "required": [
                "linode_id",
                "address",
                "confirm",
            ],
        },
    ), Capability.Destroy


async def _instance_ip_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, iid: int, address: str
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_instance_ip(iid, address)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance_ip(iid, address)
        return {
            "message": f"IP {address} deleted from instance {iid}",
            "linode_id": iid,
            "address": address,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_instance_ip_delete",
        method="DELETE",
        path=f"/linode/instances/{iid}/ips/{address}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("InstanceIP"),
    )


async def handle_linode_instance_ip_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_ip_delete tool request."""
    iid = _parse_instance_id(arguments)
    if isinstance(iid, list):
        return iid

    address = _parse_ip_address_argument(arguments)
    if isinstance(address, list):
        return address

    two_stage = await _instance_ip_delete_two_stage(arguments, cfg, iid, address)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_instance_ip(iid, address)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_instance_ip_delete",
            "DELETE",
            f"/linode/instances/{iid}/ips/{address}",
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("This is destructive. Set confirm=true to proceed.")

    async def _call(
        client: RetryableClient,
    ) -> dict[str, Any]:
        await client.delete_instance_ip(iid, address)
        return {
            "message": (f"IP {address} deleted from instance {iid}"),
            "linode_id": iid,
            "address": address,
        }

    return await execute_tool(cfg, arguments, "delete instance IP", _call)
