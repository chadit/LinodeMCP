"""Reserved public IPv4 address tools."""

from __future__ import annotations

import ipaddress
import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import ip_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


_REGION_SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$")
_RESERVED_IP_FIELD_ORDER = (
    "address",
    "assigned_entity",
    "gateway",
    "interface_id",
    "linode_id",
    "prefix",
    "public",
    "rdns",
    "region",
    "reserved",
    "subnet_mask",
    "tags",
    "type",
    "vpc_nat_1_1",
)
_RESERVED_IP_NULLABLE_FIELDS = frozenset(
    {
        "assigned_entity",
        "gateway",
        "interface_id",
        "linode_id",
        "rdns",
        "vpc_nat_1_1",
    }
)


def _restore_reserved_ip_nulls(
    raw: dict[str, Any], serialized: dict[str, Any]
) -> dict[str, Any]:
    """Restore documented nulls after typed proto serialization.

    Proto presence preserves integer and object types but omits fields decoded
    from JSON null. Rebuild in proto field order so the raw tool response stays
    deterministic while retaining explicit API null values.
    """
    restored: dict[str, Any] = {}
    for field in _RESERVED_IP_FIELD_ORDER:
        if field in serialized:
            restored[field] = serialized[field]
        elif (
            field in _RESERVED_IP_NULLABLE_FIELDS
            and field in raw
            and raw[field] is None
        ):
            restored[field] = None
    return restored


def _restore_reserved_ip_list_nulls(
    raw: Any, serialized: dict[str, Any]
) -> dict[str, Any]:
    """Restore explicit nulls for each reserved IP in a list envelope."""
    raw_page = cast("dict[str, Any]", raw) if isinstance(raw, dict) else {}
    raw_data = raw_page.get("data", [])
    raw_items = (
        [
            cast("dict[str, Any]", item)
            for item in cast("list[object]", raw_data)
            if isinstance(item, dict)
        ]
        if isinstance(raw_data, list)
        else []
    )
    serialized_data = serialized.get("reserved_ips", [])
    serialized_items = (
        [
            cast("dict[str, Any]", item)
            for item in cast("list[object]", serialized_data)
            if isinstance(item, dict)
        ]
        if isinstance(serialized_data, list)
        else []
    )
    serialized["reserved_ips"] = [
        _restore_reserved_ip_nulls(raw_item, serialized_item)
        for raw_item, serialized_item in zip(raw_items, serialized_items, strict=True)
    ]
    return serialized


def _reserved_ipv4_argument(
    arguments: dict[str, Any],
) -> str | list[TextContent]:
    """Validate and return the reserved public IPv4 path argument."""
    address = arguments.get("address")
    if not isinstance(address, str) or not address:
        return error_response("address is required")
    try:
        parsed = ipaddress.ip_address(address)
    except ValueError:
        return error_response("address must be a valid IPv4 address")
    if not isinstance(parsed, ipaddress.IPv4Address):
        return error_response("address must be a valid IPv4 address")
    return address


def _tags_argument(
    arguments: dict[str, Any], *, required: bool
) -> tuple[list[str] | None, str | None]:
    """Validate an optional or required list of non-empty tag strings."""
    if "tags" not in arguments:
        if required:
            return None, "tags is required"
        return None, None
    tags = arguments["tags"]
    if not isinstance(tags, list):
        return None, "tags must be a list of strings"
    raw_tags = cast("list[object]", tags)
    if not all(isinstance(tag, str) and tag for tag in raw_tags):
        return None, "tags must contain only non-empty strings"
    return [tag for tag in raw_tags if isinstance(tag, str)], None


def _region_argument(arguments: dict[str, Any]) -> str | list[TextContent]:
    """Validate and return a documented lowercase region slug."""
    region = arguments.get("region")
    if not isinstance(region, str) or not region:
        return error_response("region is required")
    if not _REGION_SLUG_RE.fullmatch(region):
        return error_response("region must be a lowercase region slug")
    return region


def create_linode_networking_reserved_ip_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_reserved_ip_create tool."""
    return Tool(
        name="linode_networking_reserved_ip_create",
        description=(
            "Reserves a public IPv4 address in a region. Pass dry_run=true to "
            "preview without reserving or starting billing."
        ),
        inputSchema=schema("linode.mcp.v1.ReservedIPCreateInput"),
    ), Capability.Write


async def handle_linode_networking_reserved_ip_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_reserved_ip_create."""
    region = _region_argument(arguments)
    if isinstance(region, list):
        return region
    tags, tags_error = _tags_argument(arguments, required=False)
    if tags_error is not None:
        return error_response(tags_error)
    request_body: dict[str, Any] = {"region": region}
    if tags is not None:
        request_body["tags"] = tags

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_networking_reserved_ip_create",
            arguments.get("environment", ""),
            "POST",
            "/networking/reserved/ips",
            None,
            request_body=request_body,
            billing_delta={"reserved_ipv4": 1},
            warnings=["Reserved IP billing begins when the address is created."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This reserves a public IPv4 address and starts billing. "
            "Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        reserved_ip = await client.create_reserved_ip(region, tags)
        serialized = serialize_api_response(reserved_ip, ip_pb2.ReservedIPAddress())
        return _restore_reserved_ip_nulls(reserved_ip, serialized)

    return await execute_tool(cfg, arguments, "reserve public IPv4 address", _call)


def create_linode_networking_reserved_ip_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_reserved_ip_list tool."""
    return Tool(
        name="linode_networking_reserved_ip_list",
        description="Lists reserved public IPv4 addresses on the account",
        inputSchema=schema("linode.mcp.v1.ReservedIPListInput"),
    ), Capability.Read


async def handle_linode_networking_reserved_ip_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_reserved_ip_list."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        response = await client.list_reserved_ips(page=page, page_size=page_size)
        serialized = serialize_list_response(
            response,
            "reserved_ips",
            ip_pb2.ReservedIPListResponse(),
        )
        return _restore_reserved_ip_list_nulls(response, serialized)

    return await execute_tool(cfg, arguments, "list reserved IPv4 addresses", _call)


def create_linode_networking_reserved_ip_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_reserved_ip_get tool."""
    return Tool(
        name="linode_networking_reserved_ip_get",
        description="Gets a reserved public IPv4 address",
        inputSchema=schema("linode.mcp.v1.ReservedIPGetInput"),
    ), Capability.Read


async def handle_linode_networking_reserved_ip_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_reserved_ip_get."""
    address = _reserved_ipv4_argument(arguments)
    if isinstance(address, list):
        return address

    async def _call(client: RetryableClient) -> dict[str, Any]:
        reserved_ip = await client.get_reserved_ip(address)
        serialized = serialize_api_response(reserved_ip, ip_pb2.ReservedIPAddress())
        return _restore_reserved_ip_nulls(reserved_ip, serialized)

    return await execute_tool(cfg, arguments, "get reserved IPv4 address", _call)


def create_linode_networking_reserved_ip_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_reserved_ip_update tool."""
    return Tool(
        name="linode_networking_reserved_ip_update",
        description=(
            "Replaces all tags on a reserved public IPv4 address. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema=schema("linode.mcp.v1.ReservedIPUpdateInput"),
    ), Capability.Write


async def handle_linode_networking_reserved_ip_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_reserved_ip_update."""
    address = _reserved_ipv4_argument(arguments)
    if isinstance(address, list):
        return address
    tags, tags_error = _tags_argument(arguments, required=True)
    if tags_error is not None:
        return error_response(tags_error)
    typed_tags = cast("list[str]", tags)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> dict[str, Any]:
            return await client.get_reserved_ip(address)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_networking_reserved_ip_update",
            "PUT",
            f"/networking/reserved/ips/{address}",
            _fetch,
            request_body={"tags": typed_tags},
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This replaces all tags on the reserved IP. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        reserved_ip = await client.update_reserved_ip(address, typed_tags)
        serialized = serialize_api_response(reserved_ip, ip_pb2.ReservedIPAddress())
        return _restore_reserved_ip_nulls(reserved_ip, serialized)

    return await execute_tool(cfg, arguments, "replace reserved IPv4 tags", _call)


def create_linode_networking_reserved_ip_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_reserved_ip_type_list tool."""
    return Tool(
        name="linode_networking_reserved_ip_type_list",
        description="Lists reserved public IPv4 pricing information",
        inputSchema=schema("linode.mcp.v1.ReservedIPTypeListInput"),
    ), Capability.Read


async def handle_linode_networking_reserved_ip_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_reserved_ip_type_list."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        response = await client.list_reserved_ip_types()
        return serialize_list_response(
            response,
            "reserved_ip_types",
            ip_pb2.ReservedIPTypeListResponse(),
        )

    return await execute_tool(cfg, arguments, "list reserved IPv4 pricing", _call)


def create_linode_networking_reserved_ip_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_networking_reserved_ip_delete tool."""
    return Tool(
        name="linode_networking_reserved_ip_delete",
        description=(
            "Permanently unreserves a public IPv4 address and stops billing. "
            "Pass dry_run=true to preview without deleting." + TWO_STAGE_NOTE
        ),
        inputSchema=schema("linode.mcp.v1.ReservedIPDeleteInput"),
    ), Capability.Destroy


async def handle_linode_networking_reserved_ip_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_networking_reserved_ip_delete."""
    address = _reserved_ipv4_argument(arguments)
    if isinstance(address, list):
        return address

    async def _fetch(client: RetryableClient) -> dict[str, Any]:
        return await client.get_reserved_ip(address)

    async def _delete(client: RetryableClient) -> dict[str, Any]:
        await client.delete_reserved_ip(address)
        return serialize_api_response(
            {
                "message": f"Reserved IP {address} unreserved successfully",
                "address": address,
            },
            ip_pb2.ReservedIPDeleteResponse(),
        )

    two_stage = await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_networking_reserved_ip_delete",
        method="DELETE",
        path=f"/networking/reserved/ips/{address}",
        fetch_state=_fetch,
        execute=_delete,
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        return await execute_dry_run(
            cfg,
            arguments,
            "linode_networking_reserved_ip_delete",
            "DELETE",
            f"/networking/reserved/ips/{address}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This permanently unreserves the IP and it cannot be recovered. "
            "Set confirm=true to proceed."
        )

    return await execute_tool(cfg, arguments, "unreserve public IPv4 address", _delete)
