from __future__ import annotations

from typing import TYPE_CHECKING, Any
from urllib.parse import quote, urlencode

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import firewall_device_pb2, firewall_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool, required_int_id
from linodemcp.tools.proto_enum import enum_choice_error
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_firewall_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_list tool."""
    return Tool(
        name="linode_firewall_list",
        description=(
            "Lists all Cloud Firewalls on your account. Can filter by status or label."
        ),
        inputSchema=schema("linode.mcp.v1.FirewallListInput"),
    ), Capability.Read


def create_linode_firewall_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_get tool."""
    return Tool(
        name="linode_firewall_get",
        description="Gets a Cloud Firewall by ID.",
        inputSchema=schema("linode.mcp.v1.FirewallGetInput"),
    ), Capability.Read


async def handle_linode_firewall_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_get tool request."""
    firewall_id, error = required_int_id(arguments, "firewall_id")
    if firewall_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_raw(f"/networking/firewalls/{int(firewall_id)}"),
            firewall_pb2.Firewall(),
        )

    return await execute_tool(cfg, arguments, "retrieve firewall", _call)


def create_linode_firewall_rules_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rules_get tool."""
    return Tool(
        name="linode_firewall_rules_get",
        description="Gets the rules for a Cloud Firewall by ID.",
        inputSchema=schema("linode.mcp.v1.FirewallRulesGetInput"),
    ), Capability.Read


async def handle_linode_firewall_rules_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rules_get tool request."""
    firewall_id, error = required_int_id(arguments, "firewall_id")
    if firewall_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_raw(f"/networking/firewalls/{int(firewall_id)}/rules"),
            firewall_pb2.FirewallRules(),
        )

    return await execute_tool(cfg, arguments, "retrieve firewall rules", _call)


async def handle_linode_firewall_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_list tool request."""
    status_filter = arguments.get("status", "")
    label_contains = arguments.get("label_contains", "")

    def _matches(firewall: dict[str, Any]) -> bool:
        status = str(firewall.get("status", ""))
        if status_filter and status.lower() != status_filter.lower():
            return False
        label = str(firewall.get("label", ""))
        return not (label_contains and label_contains.lower() not in label.lower())

    filters: list[str] = []
    if status_filter:
        filters.append(f"status={status_filter}")
    if label_contains:
        filters.append(f"label_contains={label_contains}")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw("/networking/firewalls")
        return serialize_list_response(
            raw,
            "firewalls",
            firewall_pb2.FirewallListResponse(),
            filter_value=", ".join(filters) if filters else None,
            item_filter=_matches,
        )

    return await execute_tool(cfg, arguments, "retrieve firewalls", _call)


def create_linode_firewall_rule_version_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rule_version_list tool."""
    return Tool(
        name="linode_firewall_rule_version_list",
        description=("Lists all rule versions (history) for a Cloud Firewall by ID."),
        inputSchema=schema("linode.mcp.v1.FirewallRuleVersionListInput"),
    ), Capability.Read


async def handle_linode_firewall_rule_version_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rule_version_list tool request."""
    firewall_id = arguments.get("firewall_id")
    if not firewall_id:
        return error_response("firewall_id is required")
    if isinstance(firewall_id, bool):
        return error_response("firewall_id must be a valid integer")
    try:
        fw_id = int(firewall_id)
        if fw_id <= 0:
            return error_response("firewall_id must be a positive integer")
    except (ValueError, TypeError):
        return error_response("firewall_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw(f"/networking/firewalls/{fw_id}/history")
        return serialize_list_response(
            raw,
            "firewall_rule_versions",
            firewall_pb2.FirewallRuleVersionListResponse(),
        )

    return await execute_tool(cfg, arguments, "list firewall rule versions", _call)


def create_linode_firewall_rule_version_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rule_version_get tool."""
    return Tool(
        name="linode_firewall_rule_version_get",
        description=(
            "Gets a specific version of a Cloud Firewall rule by ID and version."
        ),
        inputSchema=schema("linode.mcp.v1.FirewallRuleVersionGetInput"),
    ), Capability.Read


def create_linode_firewall_device_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_get tool."""
    return Tool(
        name="linode_firewall_device_get",
        description="Gets a specific device attached to a Cloud Firewall by ID.",
        inputSchema=schema("linode.mcp.v1.FirewallDeviceGetInput"),
    ), Capability.Read


def create_linode_firewall_device_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_list tool."""
    return Tool(
        name="linode_firewall_device_list",
        description="Lists devices attached to a Cloud Firewall by ID.",
        inputSchema=schema("linode.mcp.v1.FirewallDeviceListInput"),
    ), Capability.Read


async def handle_linode_firewall_device_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_device_get tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    device_id = arguments.get("device_id", 0)
    for _name, value, label in [
        ("firewall_id", firewall_id, "firewall_id"),
        ("device_id", device_id, "device_id"),
    ]:
        if not value:
            return error_response(f"{label} is required")
        if isinstance(value, bool):
            return error_response(f"{label} must be a valid integer")
        try:
            parsed = int(value)
            if parsed <= 0:
                return error_response(f"{label} must be a positive integer")
        except (ValueError, TypeError):
            return error_response(f"{label} must be a valid integer")

    fw_id = int(firewall_id)
    dev_id = int(device_id)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_firewall_device(fw_id, dev_id),
            firewall_device_pb2.FirewallDevice(),
        )

    return await execute_tool(cfg, arguments, "retrieve firewall device", _call)


def _parse_positive_integer_arg(
    arguments: dict[str, Any],
    name: str,
    *,
    required: bool,
) -> tuple[int | None, list[TextContent] | None]:
    """Parse an optional or required positive integer tool argument."""
    value = arguments.get(name)
    if value is None or value == "":
        if required:
            return None, error_response(f"{name} is required")
        return None, None
    if isinstance(value, bool):
        return None, error_response(f"{name} must be a valid integer")
    try:
        parsed = int(value)
    except (ValueError, TypeError):
        return None, error_response(f"{name} must be a valid integer")
    if parsed <= 0:
        return None, error_response(f"{name} must be a positive integer")
    return parsed, None


async def handle_linode_firewall_device_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_device_list tool request."""
    fw_id, error = _parse_positive_integer_arg(arguments, "firewall_id", required=True)
    if error is not None:
        return error
    if fw_id is None:
        return error_response("firewall_id is required")
    page, error = _parse_positive_integer_arg(arguments, "page", required=False)
    if error is not None:
        return error
    page_size, error = _parse_positive_integer_arg(
        arguments, "page_size", required=False
    )
    if error is not None:
        return error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_firewall_devices(fw_id, page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "devices",
            firewall_device_pb2.FirewallDeviceListResponse(),
        )

    return await execute_tool(cfg, arguments, "list firewall devices", _call)


async def handle_linode_firewall_rule_version_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rule_version_get tool request."""
    fw_id, error = _parse_positive_integer_arg(arguments, "firewall_id", required=True)
    if error is not None:
        return error
    if fw_id is None:
        return error_response("firewall_id is required")
    version_int, error = _parse_positive_integer_arg(
        arguments, "version", required=True
    )
    if error is not None:
        return error
    if version_int is None:
        return error_response("version is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw(
            f"/networking/firewalls/{fw_id}/history/rules/{version_int}"
        )
        return serialize_api_response(raw, firewall_pb2.FirewallRuleVersion())

    return await execute_tool(cfg, arguments, "retrieve firewall rule version", _call)


def create_linode_firewall_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_settings_get tool."""
    return Tool(
        name="linode_firewall_settings_get",
        description="Lists account default firewall settings.",
        inputSchema=schema("linode.mcp.v1.FirewallSettingsGetInput"),
    ), Capability.Read


async def handle_linode_firewall_settings_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_settings_get tool request."""
    page, error = _parse_positive_integer_arg(arguments, "page", required=False)
    if error is not None:
        return error
    page_size, error = _parse_positive_integer_arg(
        arguments, "page_size", required=False
    )
    if error is not None:
        return error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return serialize_api_response(
            await client.get_firewall_settings(page=page, page_size=page_size),
            firewall_pb2.FirewallSettings(),
        )

    return await execute_tool(cfg, arguments, "list default firewall settings", _call)


def create_linode_firewall_template_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_template_list tool."""
    return Tool(
        name="linode_firewall_template_list",
        description="Lists Cloud Firewall Templates.",
        inputSchema=schema("linode.mcp.v1.FirewallTemplateListInput"),
    ), Capability.Read


async def handle_linode_firewall_template_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_template_list tool request."""
    page, error = _parse_positive_integer_arg(arguments, "page", required=False)
    if error is not None:
        return error
    page_size, error = _parse_positive_integer_arg(
        arguments, "page_size", required=False
    )
    if error is not None:
        return error

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_firewall_templates(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "firewall_templates",
            firewall_pb2.FirewallTemplateListResponse(),
        )

    return await execute_tool(cfg, arguments, "list firewall templates", _call)


def create_linode_firewall_template_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_template_get tool."""
    return Tool(
        name="linode_firewall_template_get",
        description="Gets a Cloud Firewall Template by slug.",
        inputSchema=schema("linode.mcp.v1.FirewallTemplateGetInput"),
    ), Capability.Read


async def handle_linode_firewall_template_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_template_get tool request."""
    slug = arguments.get("slug", "")
    if not isinstance(slug, str) or not slug:
        return error_response("slug is required and must be a string")

    slug_error = enum_choice_error(
        slug, "slug", firewall_pb2.FirewallTemplateSlug.Value
    )
    if slug_error is not None:
        return error_response(slug_error)

    # Validate pagination parameters
    page = arguments.get("page")
    page_size = arguments.get("page_size")

    if page is not None and (not isinstance(page, int) or page < 1):
        return error_response("page must be a positive integer")

    if page_size is not None and (not isinstance(page_size, int) or page_size < 1):
        return error_response("page_size must be a positive integer")

    params: dict[str, Any] = {}
    if page is not None:
        params["page"] = page
    if page_size is not None:
        params["page_size"] = page_size
    query = f"?{urlencode(params)}" if params else ""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_raw(
            f"/networking/firewalls/templates/{quote(slug, safe='')}{query}"
        )
        return serialize_api_response(raw, firewall_pb2.FirewallTemplate())

    return await execute_tool(cfg, arguments, "retrieve firewall template", _call)
