from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import ENV_PARAM_SCHEMA, error_response, execute_tool

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


async def handle_linode_firewall_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_list tool request."""
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


def create_linode_firewall_rule_version_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_rule_version_list tool."""
    return Tool(
        name="linode_firewall_rule_version_list",
        description=("Lists all rule versions (history) for a Cloud Firewall by ID."),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": ("The ID of the firewall (required)"),
                },
            },
            "required": ["firewall_id"],
        },
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
        versions = await client.list_firewall_rule_versions(fw_id)
        return {
            "count": len(versions),
            "versions": [
                {
                    "id": v.id,
                    "label": v.label,
                    "status": v.status,
                    "created": v.created,
                    "updated": v.updated,
                    "tags": v.tags,
                }
                for v in versions
            ],
        }

    return await execute_tool(cfg, arguments, "list firewall rule versions", _call)


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
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "The version identifier of the firewall rule (required)"
                    ),
                },
            },
            "required": ["firewall_id", "version"],
        },
    ), Capability.Read


def create_linode_firewall_device_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_get tool."""
    return Tool(
        name="linode_firewall_device_get",
        description="Gets a specific device attached to a Cloud Firewall by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": ("The ID of the firewall (required)"),
                },
                "device_id": {
                    "type": "integer",
                    "description": ("The ID of the device (required)"),
                },
            },
            "required": ["firewall_id", "device_id"],
        },
    ), Capability.Read


def create_linode_firewall_device_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_device_list tool."""
    return Tool(
        name="linode_firewall_device_list",
        description="Lists devices attached to a Cloud Firewall by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "firewall_id": {
                    "type": "integer",
                    "description": ("The ID of the firewall (required)"),
                },
                "page": {
                    "type": "integer",
                    "description": "Page number for pagination",
                },
                "page_size": {
                    "type": "integer",
                    "description": "Number of items per page",
                },
            },
            "required": ["firewall_id"],
        },
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
        return await client.get_firewall_device(fw_id, dev_id)

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
        return await client.list_firewall_devices(fw_id, page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list firewall devices", _call)


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


def create_linode_firewall_settings_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_settings_get tool."""
    return Tool(
        name="linode_firewall_settings_get",
        description="Lists account default firewall settings.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "description": "Page number for pagination",
                },
                "page_size": {
                    "type": "integer",
                    "description": "Page size for pagination",
                },
            },
        },
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
        return await client.get_firewall_settings(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list default firewall settings", _call)


def create_linode_firewall_template_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_template_list tool."""
    return Tool(
        name="linode_firewall_template_list",
        description="Lists Cloud Firewall Templates.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "description": "Page number for pagination",
                },
                "page_size": {
                    "type": "integer",
                    "description": "Page size for pagination",
                },
            },
        },
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
        return await client.list_firewall_templates(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list firewall templates", _call)


def create_linode_firewall_template_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_firewall_template_get tool."""
    return Tool(
        name="linode_firewall_template_get",
        description="Gets a Cloud Firewall Template by slug.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "slug": {
                    "type": "string",
                    "description": (
                        "The slug of the firewall template to retrieve (required)"
                    ),
                },
                "page": {
                    "type": "integer",
                    "description": "Page number for pagination",
                },
                "page_size": {
                    "type": "integer",
                    "description": "Page size for pagination",
                },
            },
            "required": ["slug"],
        },
    ), Capability.Read


async def handle_linode_firewall_template_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_template_get tool request."""
    slug = arguments.get("slug", "")
    if not isinstance(slug, str) or not slug:
        return error_response("slug is required and must be a string")

    # Reject path traversal characters in slug
    if any(c in slug for c in ("/", "?", "..")):
        return error_response(
            "slug must not contain path separators or traversal characters"
        )

    # Validate pagination parameters
    page = arguments.get("page")
    page_size = arguments.get("page_size")

    if page is not None and (not isinstance(page, int) or page < 1):
        return error_response("page must be a positive integer")

    if page_size is not None and (not isinstance(page_size, int) or page_size < 1):
        return error_response("page_size must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        template = await client.get_firewall_template(slug, page, page_size)
        return {
            "slug": template.slug,
            "label": template.label,
            "description": template.description,
            "rules": {
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
                    for r in template.rules.inbound
                ],
                "inbound_policy": template.rules.inbound_policy,
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
                    for r in template.rules.outbound
                ],
                "outbound_policy": template.rules.outbound_policy,
            },
        }

    return await execute_tool(cfg, arguments, "retrieve firewall template", _call)
