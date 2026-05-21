from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_firewalls_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewalls_list tool."""
    return Tool(
        name="linode_firewalls_list",
        description=(
            "Lists all Cloud Firewalls on your account. Can filter by status or label."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "status": {
                    "type": "string",
                    "description": (
                        "Filter by firewall status (enabled, disabled, deleted)"
                    ),
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter firewalls by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    ), Capability.Read


def create_linode_firewall_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_get tool."""
    return Tool(
        name="linode_firewall_get",
        description="Gets a Cloud Firewall by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to retrieve (required)",
                },
            },
            "required": ["firewall_id"],
        },
    ), Capability.Read


async def handle_linode_firewall_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_get tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    if not firewall_id:
        return error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.get_firewall(int(firewall_id))
        return {
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "rules_inbound_count": len(firewall.rules.inbound),
                "rules_outbound_count": len(firewall.rules.outbound),
                "created": firewall.created,
                "updated": firewall.updated,
                "tags": firewall.tags,
            }
        }

    return await execute_tool(cfg, arguments, "retrieve firewall", _call)


def create_linode_firewall_rules_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rules_get tool."""
    return Tool(
        name="linode_firewall_rules_get",
        description="Gets the rules for a Cloud Firewall by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the firewall to retrieve rules for (required)"
                    ),
                },
            },
            "required": ["firewall_id"],
        },
    ), Capability.Read


async def handle_linode_firewall_rules_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rules_get tool request."""
    firewall_id = arguments.get("firewall_id", 0)
    if not firewall_id:
        return error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        rules = await client.get_firewall_rules(int(firewall_id))
        return {
            "inbound": [
                {
                    "action": r.action,
                    "protocol": r.protocol,
                    "ports": r.ports,
                    "addresses": {
                        "ipv4": r.addresses.ipv4,
                        "ipv6": r.addresses.ipv6,
                    },
                    "label": r.label,
                    "description": r.description,
                }
                for r in rules.inbound
            ],
            "inbound_policy": rules.inbound_policy,
            "outbound": [
                {
                    "action": r.action,
                    "protocol": r.protocol,
                    "ports": r.ports,
                    "addresses": {
                        "ipv4": r.addresses.ipv4,
                        "ipv6": r.addresses.ipv6,
                    },
                    "label": r.label,
                    "description": r.description,
                }
                for r in rules.outbound
            ],
            "outbound_policy": rules.outbound_policy,
        }

    return await execute_tool(cfg, arguments, "retrieve firewall rules", _call)


async def handle_linode_firewalls_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewalls_list tool request."""
    status_filter = arguments.get("status", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewalls = await client.list_firewalls()

        if status_filter:
            firewalls = [
                f for f in firewalls if f.status.lower() == status_filter.lower()
            ]

        if label_contains:
            firewalls = [
                f for f in firewalls if label_contains.lower() in f.label.lower()
            ]

        firewalls_data = [
            {
                "id": f.id,
                "label": f.label,
                "status": f.status,
                "rules_inbound_count": len(f.rules.inbound),
                "rules_outbound_count": len(f.rules.outbound),
                "created": f.created,
                "updated": f.updated,
            }
            for f in firewalls
        ]

        response: dict[str, Any] = {
            "count": len(firewalls),
            "firewalls": firewalls_data,
        }

        filters: list[str] = []
        if status_filter:
            filters.append(f"status={status_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve firewalls", _call)


def create_linode_firewall_rule_version_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rule_version_get tool."""
    return Tool(
        name="linode_firewall_rule_version_get",
        description=(
            "Gets a specific version of a Cloud Firewall rule by ID and version."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": ("The ID of the firewall (required)"),
                },
                "version": {
                    "type": "string",
                    "description": (
                        "The version identifier of the firewall rule (required)"
                    ),
                },
            },
            "required": ["firewall_id", "version"],
        },
    ), Capability.Read


async def handle_linode_firewall_rule_version_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_rule_version_get tool request."""
    firewall_id = arguments.get("firewall_id")
    version = arguments.get("version", "")
    if not firewall_id:
        return error_response("firewall_id is required")
    if not version:
        return error_response("version is required")
    if isinstance(firewall_id, bool):
        return error_response("firewall_id must be a valid integer")
    try:
        fw_id = int(firewall_id)
        if fw_id <= 0:
            return error_response("firewall_id must be a positive integer")
    except (ValueError, TypeError):
        return error_response("firewall_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        rule = await client.get_firewall_rule_version(fw_id, str(version))
        return {
            "action": rule.action,
            "protocol": rule.protocol,
            "ports": rule.ports,
            "addresses": {
                "ipv4": rule.addresses.ipv4,
                "ipv6": rule.addresses.ipv6,
            },
            "label": rule.label,
            "description": rule.description,
        }

    return await execute_tool(cfg, arguments, "retrieve firewall rule version", _call)
