"""MCP tools for LinodeMCP."""

import json
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config, EnvironmentConfig, EnvironmentNotFoundError
from linodemcp.linode import RetryableClient, RetryConfig
from linodemcp.version import get_version_info

# Constants for truncation limits
SSH_KEY_TRUNCATE_LIMIT = 50
DESCRIPTION_TRUNCATE_LIMIT = 100


def _truncate_string(value: str, limit: int) -> str:
    """Truncate a string with ellipsis if it exceeds the limit."""
    if len(value) > limit:
        return value[:limit] + "..."
    return value


__all__ = [
    "create_hello_tool",
    "create_linode_account_tool",
    "create_linode_domain_create_tool",
    "create_linode_domain_delete_tool",
    "create_linode_domain_get_tool",
    "create_linode_domain_record_create_tool",
    "create_linode_domain_record_delete_tool",
    "create_linode_domain_record_update_tool",
    "create_linode_domain_records_list_tool",
    "create_linode_domain_update_tool",
    "create_linode_domains_list_tool",
    "create_linode_firewall_create_tool",
    "create_linode_firewall_delete_tool",
    "create_linode_firewall_update_tool",
    "create_linode_firewalls_list_tool",
    "create_linode_images_list_tool",
    "create_linode_instance_boot_tool",
    "create_linode_instance_create_tool",
    "create_linode_instance_delete_tool",
    "create_linode_instance_get_tool",
    "create_linode_instance_reboot_tool",
    "create_linode_instance_resize_tool",
    "create_linode_instance_shutdown_tool",
    "create_linode_instances_list_tool",
    "create_linode_nodebalancer_create_tool",
    "create_linode_nodebalancer_delete_tool",
    "create_linode_nodebalancer_get_tool",
    "create_linode_nodebalancer_update_tool",
    "create_linode_nodebalancers_list_tool",
    "create_linode_profile_tool",
    "create_linode_regions_list_tool",
    "create_linode_sshkey_create_tool",
    "create_linode_sshkey_delete_tool",
    "create_linode_sshkeys_list_tool",
    "create_linode_stackscripts_list_tool",
    "create_linode_types_list_tool",
    "create_linode_volume_attach_tool",
    "create_linode_volume_create_tool",
    "create_linode_volume_delete_tool",
    "create_linode_volume_detach_tool",
    "create_linode_volume_resize_tool",
    "create_linode_volumes_list_tool",
    "create_version_tool",
    "handle_hello",
    "handle_linode_account",
    "handle_linode_domain_create",
    "handle_linode_domain_delete",
    "handle_linode_domain_get",
    "handle_linode_domain_record_create",
    "handle_linode_domain_record_delete",
    "handle_linode_domain_record_update",
    "handle_linode_domain_records_list",
    "handle_linode_domain_update",
    "handle_linode_domains_list",
    "handle_linode_firewall_create",
    "handle_linode_firewall_delete",
    "handle_linode_firewall_update",
    "handle_linode_firewalls_list",
    "handle_linode_images_list",
    "handle_linode_instance_boot",
    "handle_linode_instance_create",
    "handle_linode_instance_delete",
    "handle_linode_instance_get",
    "handle_linode_instance_reboot",
    "handle_linode_instance_resize",
    "handle_linode_instance_shutdown",
    "handle_linode_instances_list",
    "handle_linode_nodebalancer_create",
    "handle_linode_nodebalancer_delete",
    "handle_linode_nodebalancer_get",
    "handle_linode_nodebalancer_update",
    "handle_linode_nodebalancers_list",
    "handle_linode_profile",
    "handle_linode_regions_list",
    "handle_linode_sshkey_create",
    "handle_linode_sshkey_delete",
    "handle_linode_sshkeys_list",
    "handle_linode_stackscripts_list",
    "handle_linode_types_list",
    "handle_linode_volume_attach",
    "handle_linode_volume_create",
    "handle_linode_volume_delete",
    "handle_linode_volume_detach",
    "handle_linode_volume_resize",
    "handle_linode_volumes_list",
    "handle_version",
]


def create_hello_tool() -> Tool:
    """Create the hello tool."""
    return Tool(
        name="hello",
        description="Responds with a friendly greeting from LinodeMCP",
        inputSchema={
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Name to include in the greeting (optional)",
                },
            },
        },
    )


async def handle_hello(arguments: dict[str, Any]) -> list[TextContent]:
    """Handle hello tool request."""
    name = arguments.get("name", "World")
    message = f"Hello, {name}! LinodeMCP server is running and ready."
    return [TextContent(type="text", text=message)]


def create_version_tool() -> Tool:
    """Create the version tool."""
    return Tool(
        name="version",
        description="Returns LinodeMCP server version and build information",
        inputSchema={
            "type": "object",
            "properties": {},
        },
    )


async def handle_version(_arguments: dict[str, Any]) -> list[TextContent]:
    """Handle version tool request."""
    version_info = get_version_info()
    json_response = json.dumps(version_info.to_dict(), indent=2)
    return [TextContent(type="text", text=json_response)]


def create_linode_profile_tool() -> Tool:
    """Create the linode_profile tool."""
    return Tool(
        name="linode_profile",
        description="Retrieves Linode user account profile information",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
            },
        },
    )


async def handle_linode_profile(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_profile tool request."""
    environment = arguments.get("environment", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            profile = await client.get_profile()

            profile_data = {
                "username": profile.username,
                "email": profile.email,
                "timezone": profile.timezone,
                "email_notifications": profile.email_notifications,
                "restricted": profile.restricted,
                "two_factor_auth": profile.two_factor_auth,
                "uid": profile.uid,
            }

            json_response = json.dumps(profile_data, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode profile: {e}")
        ]


def create_linode_instances_list_tool() -> Tool:
    """Create the linode_instances_list tool."""
    return Tool(
        name="linode_instances_list",
        description="Lists Linode instances with optional filtering by status",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "status": {
                    "type": "string",
                    "description": (
                        "Filter instances by status (running, stopped, etc.)"
                    ),
                },
            },
        },
    )


async def handle_linode_instances_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instances_list tool request."""
    environment = arguments.get("environment", "")
    status_filter = arguments.get("status", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            instances = await client.list_instances()

            if status_filter:
                instances = [
                    inst
                    for inst in instances
                    if inst.status.lower() == status_filter.lower()
                ]

            instances_data = []
            for inst in instances:
                instances_data.append(
                    {
                        "id": inst.id,
                        "label": inst.label,
                        "status": inst.status,
                        "type": inst.type,
                        "region": inst.region,
                        "image": inst.image,
                        "ipv4": inst.ipv4,
                        "ipv6": inst.ipv6,
                        "created": inst.created,
                        "updated": inst.updated,
                        "tags": inst.tags,
                    }
                )

            response = {
                "count": len(instances),
                "instances": instances_data,
            }

            if status_filter:
                response["filter"] = f"status={status_filter}"

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode instances: {e}")
        ]


def _select_environment(cfg: Config, environment: str) -> EnvironmentConfig:
    """Select an environment from configuration."""
    if environment:
        if environment in cfg.environments:
            return cfg.environments[environment]
        msg = f"environment not found: {environment}"
        raise EnvironmentNotFoundError(msg)

    return cfg.select_environment("default")


def _validate_linode_config(env: EnvironmentConfig) -> None:
    """Validate Linode configuration."""
    if not env.linode.api_url or not env.linode.token:
        msg = "linode configuration is incomplete: check your API URL and token"
        raise ValueError(msg)


def create_linode_instance_get_tool() -> Tool:
    """Create the linode_instance_get tool."""
    return Tool(
        name="linode_instance_get",
        description="Retrieves details of a single Linode instance by its ID",
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
                    "type": "string",
                    "description": (
                        "The ID of the Linode instance to retrieve (required)"
                    ),
                },
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_get tool request."""
    environment = arguments.get("environment", "")
    instance_id_str = arguments.get("instance_id", "")

    if not instance_id_str:
        return [TextContent(type="text", text="Error: instance_id is required")]

    try:
        instance_id = int(instance_id_str)
    except ValueError:
        return [
            TextContent(type="text", text="Error: instance_id must be a valid integer")
        ]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            instance = await client.get_instance(instance_id)

            instance_data = {
                "id": instance.id,
                "label": instance.label,
                "status": instance.status,
                "type": instance.type,
                "region": instance.region,
                "image": instance.image,
                "ipv4": instance.ipv4,
                "ipv6": instance.ipv6,
                "created": instance.created,
                "updated": instance.updated,
                "tags": instance.tags,
            }

            json_response = json.dumps(instance_data, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode instance: {e}")
        ]


def create_linode_account_tool() -> Tool:
    """Create the linode_account tool."""
    return Tool(
        name="linode_account",
        description=(
            "Retrieves the authenticated user's Linode account information "
            "including billing details and capabilities"
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
            },
        },
    )


async def handle_linode_account(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account tool request."""
    environment = arguments.get("environment", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            account = await client.get_account()

            account_data = {
                "first_name": account.first_name,
                "last_name": account.last_name,
                "email": account.email,
                "company": account.company,
                "balance": account.balance,
                "balance_uninvoiced": account.balance_uninvoiced,
                "capabilities": account.capabilities,
                "active_since": account.active_since,
            }

            json_response = json.dumps(account_data, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode account: {e}")
        ]


def create_linode_regions_list_tool() -> Tool:
    """Create the linode_regions_list tool."""
    return Tool(
        name="linode_regions_list",
        description=(
            "Lists all available Linode regions (datacenters) "
            "with optional filtering by country or capabilities"
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
                "country": {
                    "type": "string",
                    "description": "Filter regions by country code (e.g., 'us', 'de')",
                },
                "capability": {
                    "type": "string",
                    "description": (
                        "Filter regions by capability "
                        "(e.g., 'Linodes', 'Block Storage')"
                    ),
                },
            },
        },
    )


async def handle_linode_regions_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_regions_list tool request."""
    environment = arguments.get("environment", "")
    country_filter = arguments.get("country", "")
    capability_filter = arguments.get("capability", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            regions = await client.list_regions()

            if country_filter:
                regions = [
                    r for r in regions if r.country.lower() == country_filter.lower()
                ]

            if capability_filter:
                regions = [
                    r
                    for r in regions
                    if any(
                        cap.lower() == capability_filter.lower()
                        for cap in r.capabilities
                    )
                ]

            regions_data = [
                {
                    "id": r.id,
                    "label": r.label,
                    "country": r.country,
                    "capabilities": r.capabilities,
                    "status": r.status,
                }
                for r in regions
            ]

            response: dict[str, Any] = {
                "count": len(regions),
                "regions": regions_data,
            }

            filters = []
            if country_filter:
                filters.append(f"country={country_filter}")
            if capability_filter:
                filters.append(f"capability={capability_filter}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode regions: {e}")
        ]


def create_linode_types_list_tool() -> Tool:
    """Create the linode_types_list tool."""
    return Tool(
        name="linode_types_list",
        description=(
            "Lists all available Linode instance types (plans) with pricing. "
            "Can filter by class (standard, dedicated, gpu, highmem, premium)."
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
                "class": {
                    "type": "string",
                    "description": (
                        "Filter types by class (standard, dedicated, gpu, highmem)"
                    ),
                },
            },
        },
    )


async def handle_linode_types_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_types_list tool request."""
    environment = arguments.get("environment", "")
    class_filter = arguments.get("class", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            types = await client.list_types()

            if class_filter:
                types = [t for t in types if t.class_.lower() == class_filter.lower()]

            types_data = [
                {
                    "id": t.id,
                    "label": t.label,
                    "class": t.class_,
                    "disk": t.disk,
                    "memory": t.memory,
                    "vcpus": t.vcpus,
                    "price": {"hourly": t.price.hourly, "monthly": t.price.monthly},
                }
                for t in types
            ]

            response: dict[str, Any] = {
                "count": len(types),
                "types": types_data,
            }

            if class_filter:
                response["filter"] = f"class={class_filter}"

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve Linode types: {e}")]


def create_linode_volumes_list_tool() -> Tool:
    """Create the linode_volumes_list tool."""
    return Tool(
        name="linode_volumes_list",
        description=(
            "Lists all block storage volumes for the authenticated user "
            "with optional filtering by region or label"
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
                    "description": "Filter volumes by region (e.g., 'us-east')",
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter volumes where label contains this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_volumes_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volumes_list tool request."""
    environment = arguments.get("environment", "")
    region_filter = arguments.get("region", "")
    label_contains = arguments.get("label_contains", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            volumes = await client.list_volumes()

            if region_filter:
                volumes = [
                    v for v in volumes if v.region.lower() == region_filter.lower()
                ]

            if label_contains:
                volumes = [
                    v for v in volumes if label_contains.lower() in v.label.lower()
                ]

            volumes_data = [
                {
                    "id": v.id,
                    "label": v.label,
                    "status": v.status,
                    "size": v.size,
                    "region": v.region,
                    "linode_id": v.linode_id,
                    "created": v.created,
                    "updated": v.updated,
                }
                for v in volumes
            ]

            response: dict[str, Any] = {
                "count": len(volumes),
                "volumes": volumes_data,
            }

            filters = []
            if region_filter:
                filters.append(f"region={region_filter}")
            if label_contains:
                filters.append(f"label_contains={label_contains}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode volumes: {e}")
        ]


def create_linode_images_list_tool() -> Tool:
    """Create the linode_images_list tool."""
    return Tool(
        name="linode_images_list",
        description=(
            "Lists all available Linode images (OS images and custom images) "
            "with optional filtering by type, public status, or deprecated status"
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
                "type": {
                    "type": "string",
                    "description": "Filter images by type (manual, automatic)",
                },
                "is_public": {
                    "type": "string",
                    "description": "Filter by public status (true, false)",
                },
                "deprecated": {
                    "type": "string",
                    "description": "Filter by deprecated status (true, false)",
                },
            },
        },
    )


async def handle_linode_images_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_images_list tool request."""
    environment = arguments.get("environment", "")
    type_filter = arguments.get("type", "")
    is_public_filter = arguments.get("is_public", "")
    deprecated_filter = arguments.get("deprecated", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            images = await client.list_images()

            if type_filter:
                images = [i for i in images if i.type.lower() == type_filter.lower()]

            if is_public_filter:
                want_public = is_public_filter.lower() == "true"
                images = [i for i in images if i.is_public == want_public]

            if deprecated_filter:
                want_deprecated = deprecated_filter.lower() == "true"
                images = [i for i in images if i.deprecated == want_deprecated]

            images_data = [
                {
                    "id": i.id,
                    "label": i.label,
                    "type": i.type,
                    "is_public": i.is_public,
                    "deprecated": i.deprecated,
                    "size": i.size,
                    "vendor": i.vendor,
                    "created": i.created,
                }
                for i in images
            ]

            response: dict[str, Any] = {
                "count": len(images),
                "images": images_data,
            }

            filters = []
            if type_filter:
                filters.append(f"type={type_filter}")
            if is_public_filter:
                filters.append(f"is_public={is_public_filter}")
            if deprecated_filter:
                filters.append(f"deprecated={deprecated_filter}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve Linode images: {e}")]


# Stage 3: Extended read operations


def create_linode_sshkeys_list_tool() -> Tool:
    """Create the linode_sshkeys_list tool."""
    return Tool(
        name="linode_sshkeys_list",
        description=(
            "Lists all SSH keys associated with your Linode profile. "
            "Can filter by label."
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
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter SSH keys by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_sshkeys_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkeys_list tool request."""
    environment = arguments.get("environment", "")
    label_contains = arguments.get("label_contains", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            keys = await client.list_ssh_keys()

            if label_contains:
                keys = [k for k in keys if label_contains.lower() in k.label.lower()]

            keys_data = [
                {
                    "id": k.id,
                    "label": k.label,
                    "ssh_key": _truncate_string(k.ssh_key, SSH_KEY_TRUNCATE_LIMIT),
                    "created": k.created,
                }
                for k in keys
            ]

            response: dict[str, Any] = {
                "count": len(keys),
                "ssh_keys": keys_data,
            }

            if label_contains:
                response["filter"] = f"label_contains={label_contains}"

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve SSH keys: {e}")]


def create_linode_domains_list_tool() -> Tool:
    """Create the linode_domains_list tool."""
    return Tool(
        name="linode_domains_list",
        description=(
            "Lists all domains managed by your Linode account. "
            "Can filter by domain name or type (master/slave)."
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
                "domain_contains": {
                    "type": "string",
                    "description": (
                        "Filter domains by name containing this string "
                        "(case-insensitive)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": "Filter by domain type (master, slave)",
                },
            },
        },
    )


async def handle_linode_domains_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domains_list tool request."""
    environment = arguments.get("environment", "")
    domain_contains = arguments.get("domain_contains", "")
    type_filter = arguments.get("type", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            domains = await client.list_domains()

            if domain_contains:
                domains = [
                    d for d in domains if domain_contains.lower() in d.domain.lower()
                ]

            if type_filter:
                domains = [d for d in domains if d.type.lower() == type_filter.lower()]

            domains_data = [
                {
                    "id": d.id,
                    "domain": d.domain,
                    "type": d.type,
                    "status": d.status,
                    "soa_email": d.soa_email,
                    "created": d.created,
                    "updated": d.updated,
                }
                for d in domains
            ]

            response: dict[str, Any] = {
                "count": len(domains),
                "domains": domains_data,
            }

            filters = []
            if domain_contains:
                filters.append(f"domain_contains={domain_contains}")
            if type_filter:
                filters.append(f"type={type_filter}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve domains: {e}")]


def create_linode_domain_get_tool() -> Tool:
    """Create the linode_domain_get tool."""
    return Tool(
        name="linode_domain_get",
        description="Gets detailed information about a specific domain by its ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to retrieve (required)",
                },
            },
            "required": ["domain_id"],
        },
    )


async def handle_linode_domain_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_get tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            domain = await client.get_domain(int(domain_id))

            domain_data = {
                "id": domain.id,
                "domain": domain.domain,
                "type": domain.type,
                "status": domain.status,
                "soa_email": domain.soa_email,
                "description": domain.description,
                "tags": domain.tags,
                "created": domain.created,
                "updated": domain.updated,
            }

            json_response = json.dumps(domain_data, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve domain: {e}")]


def create_linode_domain_records_list_tool() -> Tool:
    """Create the linode_domain_records_list tool."""
    return Tool(
        name="linode_domain_records_list",
        description=(
            "Lists all DNS records for a specific domain. "
            "Can filter by record type or name."
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
                "domain_id": {
                    "type": "integer",
                    "description": (
                        "The ID of the domain to list records for (required)"
                    ),
                },
                "type": {
                    "type": "string",
                    "description": (
                        "Filter by record type (A, AAAA, NS, MX, CNAME, TXT, SRV, CAA)"
                    ),
                },
                "name_contains": {
                    "type": "string",
                    "description": (
                        "Filter records by name containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
            "required": ["domain_id"],
        },
    )


async def handle_linode_domain_records_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_records_list tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)
    type_filter = arguments.get("type", "")
    name_contains = arguments.get("name_contains", "")

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            records = await client.list_domain_records(int(domain_id))

            if type_filter:
                records = [r for r in records if r.type.upper() == type_filter.upper()]

            if name_contains:
                records = [
                    r for r in records if name_contains.lower() in r.name.lower()
                ]

            records_data = [
                {
                    "id": r.id,
                    "type": r.type,
                    "name": r.name,
                    "target": r.target,
                    "priority": r.priority,
                    "ttl_sec": r.ttl_sec,
                }
                for r in records
            ]

            response: dict[str, Any] = {
                "count": len(records),
                "domain_id": domain_id,
                "records": records_data,
            }

            filters = []
            if type_filter:
                filters.append(f"type={type_filter}")
            if name_contains:
                filters.append(f"name_contains={name_contains}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [
            TextContent(type="text", text=f"Failed to retrieve domain records: {e}")
        ]


def create_linode_firewalls_list_tool() -> Tool:
    """Create the linode_firewalls_list tool."""
    return Tool(
        name="linode_firewalls_list",
        description=(
            "Lists all Cloud Firewalls on your account. Can filter by status or label."
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
    )


async def handle_linode_firewalls_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewalls_list tool request."""
    environment = arguments.get("environment", "")
    status_filter = arguments.get("status", "")
    label_contains = arguments.get("label_contains", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
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

            filters = []
            if status_filter:
                filters.append(f"status={status_filter}")
            if label_contains:
                filters.append(f"label_contains={label_contains}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve firewalls: {e}")]


def create_linode_nodebalancers_list_tool() -> Tool:
    """Create the linode_nodebalancers_list tool."""
    return Tool(
        name="linode_nodebalancers_list",
        description=(
            "Lists all NodeBalancers on your account. Can filter by region or label."
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
                    "description": "Filter by region ID (e.g., us-east, eu-west)",
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter NodeBalancers by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_nodebalancers_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancers_list tool request."""
    environment = arguments.get("environment", "")
    region_filter = arguments.get("region", "")
    label_contains = arguments.get("label_contains", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            nodebalancers = await client.list_nodebalancers()

            if region_filter:
                nodebalancers = [
                    nb
                    for nb in nodebalancers
                    if nb.region.lower() == region_filter.lower()
                ]

            if label_contains:
                nodebalancers = [
                    nb
                    for nb in nodebalancers
                    if label_contains.lower() in nb.label.lower()
                ]

            nodebalancers_data = [
                {
                    "id": nb.id,
                    "label": nb.label,
                    "region": nb.region,
                    "hostname": nb.hostname,
                    "ipv4": nb.ipv4,
                    "created": nb.created,
                    "updated": nb.updated,
                }
                for nb in nodebalancers
            ]

            response: dict[str, Any] = {
                "count": len(nodebalancers),
                "nodebalancers": nodebalancers_data,
            }

            filters = []
            if region_filter:
                filters.append(f"region={region_filter}")
            if label_contains:
                filters.append(f"label_contains={label_contains}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve NodeBalancers: {e}")]


def create_linode_nodebalancer_get_tool() -> Tool:
    """Create the linode_nodebalancer_get tool."""
    return Tool(
        name="linode_nodebalancer_get",
        description=(
            "Gets detailed information about a specific NodeBalancer by its ID."
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
                "nodebalancer_id": {
                    "type": "integer",
                    "description": "The ID of the NodeBalancer to retrieve (required)",
                },
            },
            "required": ["nodebalancer_id"],
        },
    )


async def handle_linode_nodebalancer_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_get tool request."""
    environment = arguments.get("environment", "")
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return [TextContent(type="text", text="Error: nodebalancer_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            nb = await client.get_nodebalancer(int(nodebalancer_id))

            nb_data = {
                "id": nb.id,
                "label": nb.label,
                "region": nb.region,
                "hostname": nb.hostname,
                "ipv4": nb.ipv4,
                "ipv6": nb.ipv6,
                "client_conn_throttle": nb.client_conn_throttle,
                "transfer": {
                    "in": nb.transfer.in_,
                    "out": nb.transfer.out,
                    "total": nb.transfer.total,
                },
                "tags": nb.tags,
                "created": nb.created,
                "updated": nb.updated,
            }

            json_response = json.dumps(nb_data, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve NodeBalancer: {e}")]


def create_linode_stackscripts_list_tool() -> Tool:
    """Create the linode_stackscripts_list tool."""
    return Tool(
        name="linode_stackscripts_list",
        description=(
            "Lists StackScripts. By default returns your own StackScripts. "
            "Can filter by public status, ownership, or label."
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
                "is_public": {
                    "type": "string",
                    "description": "Filter by public status (true, false)",
                },
                "mine": {
                    "type": "string",
                    "description": (
                        "Filter by ownership - only your own StackScripts (true, false)"
                    ),
                },
                "label_contains": {
                    "type": "string",
                    "description": (
                        "Filter StackScripts by label containing this string "
                        "(case-insensitive)"
                    ),
                },
            },
        },
    )


async def handle_linode_stackscripts_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_stackscripts_list tool request."""
    environment = arguments.get("environment", "")
    is_public_filter = arguments.get("is_public", "")
    mine_filter = arguments.get("mine", "")
    label_contains = arguments.get("label_contains", "")

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            scripts = await client.list_stackscripts()

            if is_public_filter:
                want_public = is_public_filter.lower() == "true"
                scripts = [s for s in scripts if s.is_public == want_public]

            if mine_filter:
                want_mine = mine_filter.lower() == "true"
                scripts = [s for s in scripts if s.mine == want_mine]

            if label_contains:
                scripts = [
                    s for s in scripts if label_contains.lower() in s.label.lower()
                ]

            scripts_data = [
                {
                    "id": s.id,
                    "label": s.label,
                    "username": s.username,
                    "description": _truncate_string(
                        s.description, DESCRIPTION_TRUNCATE_LIMIT
                    ),
                    "is_public": s.is_public,
                    "mine": s.mine,
                    "deployments_total": s.deployments_total,
                    "deployments_active": s.deployments_active,
                    "created": s.created,
                    "updated": s.updated,
                }
                for s in scripts
            ]

            response: dict[str, Any] = {
                "count": len(scripts),
                "stackscripts": scripts_data,
            }

            filters = []
            if is_public_filter:
                filters.append(f"is_public={is_public_filter}")
            if mine_filter:
                filters.append(f"mine={mine_filter}")
            if label_contains:
                filters.append(f"label_contains={label_contains}")
            if filters:
                response["filter"] = ", ".join(filters)

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to retrieve StackScripts: {e}")]


# Stage 4: Write operations


def create_linode_sshkey_create_tool() -> Tool:
    """Create the linode_sshkey_create tool."""
    return Tool(
        name="linode_sshkey_create",
        description="Creates a new SSH key and adds it to your Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "A label for the SSH key (required)",
                },
                "ssh_key": {
                    "type": "string",
                    "description": "The public SSH key (required)",
                },
            },
            "required": ["label", "ssh_key"],
        },
    )


async def handle_linode_sshkey_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_create tool request."""
    environment = arguments.get("environment", "")
    label = arguments.get("label", "")
    ssh_key = arguments.get("ssh_key", "")

    if not label:
        return [TextContent(type="text", text="Error: label is required")]
    if not ssh_key:
        return [TextContent(type="text", text="Error: ssh_key is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            key = await client.create_ssh_key(label, ssh_key)

            response = {
                "message": f"SSH key '{key.label}' (ID: {key.id}) created successfully",
                "ssh_key": {
                    "id": key.id,
                    "label": key.label,
                    "created": key.created,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create SSH key: {e}")]


def create_linode_sshkey_delete_tool() -> Tool:
    """Create the linode_sshkey_delete tool."""
    return Tool(
        name="linode_sshkey_delete",
        description="Deletes an SSH key from your Linode profile.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "ssh_key_id": {
                    "type": "integer",
                    "description": "The ID of the SSH key to delete (required)",
                },
            },
            "required": ["ssh_key_id"],
        },
    )


async def handle_linode_sshkey_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_sshkey_delete tool request."""
    environment = arguments.get("environment", "")
    ssh_key_id = arguments.get("ssh_key_id", 0)

    if not ssh_key_id:
        return [TextContent(type="text", text="Error: ssh_key_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_ssh_key(int(ssh_key_id))

            response = {
                "message": f"SSH key {ssh_key_id} deleted successfully",
                "ssh_key_id": ssh_key_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete SSH key: {e}")]


def create_linode_instance_boot_tool() -> Tool:
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
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_boot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_boot tool request."""
    environment = arguments.get("environment", "")
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return [TextContent(type="text", text="Error: instance_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.boot_instance(int(instance_id), config_id)

            response = {
                "message": f"Instance {instance_id} boot initiated successfully",
                "instance_id": instance_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to boot instance: {e}")]


def create_linode_instance_reboot_tool() -> Tool:
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
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_reboot(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_reboot tool request."""
    environment = arguments.get("environment", "")
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return [TextContent(type="text", text="Error: instance_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.reboot_instance(int(instance_id), config_id)

            response = {
                "message": f"Instance {instance_id} reboot initiated successfully",
                "instance_id": instance_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to reboot instance: {e}")]


def create_linode_instance_shutdown_tool() -> Tool:
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
            },
            "required": ["instance_id"],
        },
    )


async def handle_linode_instance_shutdown(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_shutdown tool request."""
    environment = arguments.get("environment", "")
    instance_id = arguments.get("instance_id", 0)

    if not instance_id:
        return [TextContent(type="text", text="Error: instance_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.shutdown_instance(int(instance_id))

            response = {
                "message": f"Instance {instance_id} shutdown initiated successfully",
                "instance_id": instance_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to shutdown instance: {e}")]


def create_linode_instance_create_tool() -> Tool:
    """Create the linode_instance_create tool."""
    return Tool(
        name="linode_instance_create",
        description=(
            "Creates a new Linode instance. WARNING: Billing starts immediately. "
            "Use linode_regions_list and linode_types_list to find valid values."
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
                "private_ip": {
                    "type": "boolean",
                    "description": "Add private IP (default: false)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["region", "type", "confirm"],
        },
    )


async def handle_linode_instance_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_create tool request."""
    environment = arguments.get("environment", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    region = arguments.get("region", "")
    instance_type = arguments.get("type", "")

    if not region:
        return [TextContent(type="text", text="Error: region is required")]
    if not instance_type:
        return [TextContent(type="text", text="Error: type is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            instance = await client.create_instance(
                region=region,
                instance_type=instance_type,
                image=arguments.get("image"),
                label=arguments.get("label"),
                root_pass=arguments.get("root_pass"),
                authorized_keys=arguments.get("authorized_keys"),
                booted=arguments.get("booted", True),
                backups_enabled=arguments.get("backups_enabled", False),
                private_ip=arguments.get("private_ip", False),
            )

            response = {
                "message": (
                    f"Instance '{instance.label}' (ID: {instance.id}) "
                    f"created successfully in {instance.region}"
                ),
                "instance": {
                    "id": instance.id,
                    "label": instance.label,
                    "status": instance.status,
                    "type": instance.type,
                    "region": instance.region,
                    "ipv4": instance.ipv4,
                    "ipv6": instance.ipv6,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create instance: {e}")]


def create_linode_instance_delete_tool() -> Tool:
    """Create the linode_instance_delete tool."""
    return Tool(
        name="linode_instance_delete",
        description=(
            "Deletes a Linode instance. WARNING: This is destructive and cannot "
            "be undone. All data will be lost."
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
                    "description": "The ID of the instance to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["instance_id", "confirm"],
        },
    )


async def handle_linode_instance_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_delete tool request."""
    environment = arguments.get("environment", "")
    instance_id = arguments.get("instance_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not instance_id:
        return [TextContent(type="text", text="Error: instance_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_instance(int(instance_id))

            response = {
                "message": f"Instance {instance_id} deleted successfully",
                "instance_id": instance_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete instance: {e}")]


def create_linode_instance_resize_tool() -> Tool:
    """Create the linode_instance_resize tool."""
    return Tool(
        name="linode_instance_resize",
        description=(
            "Resizes a Linode instance to a different plan. "
            "WARNING: This may cause downtime and billing changes."
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
                    "description": "Must be true to confirm resize.",
                },
            },
            "required": ["instance_id", "type", "confirm"],
        },
    )


async def handle_linode_instance_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_instance_resize tool request."""
    environment = arguments.get("environment", "")
    instance_id = arguments.get("instance_id", 0)
    instance_type = arguments.get("type", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This may cause downtime. Set confirm=true to proceed.",
            )
        ]

    if not instance_id:
        return [TextContent(type="text", text="Error: instance_id is required")]
    if not instance_type:
        return [TextContent(type="text", text="Error: type is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.resize_instance(
                instance_id=int(instance_id),
                instance_type=instance_type,
                allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", True),
                migration_type=arguments.get("migration_type", "warm"),
            )

            response = {
                "message": (
                    f"Instance {instance_id} resize to {instance_type} initiated"
                ),
                "instance_id": instance_id,
                "new_type": instance_type,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to resize instance: {e}")]


def create_linode_firewall_create_tool() -> Tool:
    """Create the linode_firewall_create tool."""
    return Tool(
        name="linode_firewall_create",
        description=(
            "Creates a new Cloud Firewall. The firewall is created with no rules."
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
                "label": {
                    "type": "string",
                    "description": "A label for the firewall (required)",
                },
                "inbound_policy": {
                    "type": "string",
                    "description": (
                        "Default inbound policy: 'ACCEPT' or 'DROP' (default: 'ACCEPT')"
                    ),
                },
                "outbound_policy": {
                    "type": "string",
                    "description": (
                        "Default outbound policy: 'ACCEPT' or 'DROP' "
                        "(default: 'ACCEPT')"
                    ),
                },
            },
            "required": ["label"],
        },
    )


async def handle_linode_firewall_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_create tool request."""
    environment = arguments.get("environment", "")
    label = arguments.get("label", "")
    inbound_policy = arguments.get("inbound_policy", "ACCEPT")
    outbound_policy = arguments.get("outbound_policy", "ACCEPT")

    if not label:
        return [TextContent(type="text", text="Error: label is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            firewall = await client.create_firewall(
                label=label,
                inbound_policy=inbound_policy,
                outbound_policy=outbound_policy,
            )

            response = {
                "message": (
                    f"Firewall '{firewall.label}' (ID: {firewall.id}) "
                    "created successfully"
                ),
                "firewall": {
                    "id": firewall.id,
                    "label": firewall.label,
                    "status": firewall.status,
                    "created": firewall.created,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create firewall: {e}")]


def create_linode_firewall_update_tool() -> Tool:
    """Create the linode_firewall_update tool."""
    return Tool(
        name="linode_firewall_update",
        description="Updates an existing Cloud Firewall.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New label for the firewall (optional)",
                },
                "status": {
                    "type": "string",
                    "description": "New status: 'enabled' or 'disabled' (optional)",
                },
                "inbound_policy": {
                    "type": "string",
                    "description": "New inbound policy: 'ACCEPT' or 'DROP' (optional)",
                },
                "outbound_policy": {
                    "type": "string",
                    "description": "New outbound policy: 'ACCEPT' or 'DROP' (optional)",
                },
            },
            "required": ["firewall_id"],
        },
    )


async def handle_linode_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_update tool request."""
    environment = arguments.get("environment", "")
    firewall_id = arguments.get("firewall_id", 0)

    if not firewall_id:
        return [TextContent(type="text", text="Error: firewall_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            firewall = await client.update_firewall(
                firewall_id=int(firewall_id),
                label=arguments.get("label"),
                status=arguments.get("status"),
                inbound_policy=arguments.get("inbound_policy"),
                outbound_policy=arguments.get("outbound_policy"),
            )

            response = {
                "message": f"Firewall {firewall_id} updated successfully",
                "firewall": {
                    "id": firewall.id,
                    "label": firewall.label,
                    "status": firewall.status,
                    "updated": firewall.updated,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to update firewall: {e}")]


def create_linode_firewall_delete_tool() -> Tool:
    """Create the linode_firewall_delete tool."""
    return Tool(
        name="linode_firewall_delete",
        description=(
            "Deletes a Cloud Firewall. WARNING: This removes all rules "
            "and unassigns all devices."
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
                "firewall_id": {
                    "type": "integer",
                    "description": "The ID of the firewall to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["firewall_id", "confirm"],
        },
    )


async def handle_linode_firewall_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_firewall_delete tool request."""
    environment = arguments.get("environment", "")
    firewall_id = arguments.get("firewall_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not firewall_id:
        return [TextContent(type="text", text="Error: firewall_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_firewall(int(firewall_id))

            response = {
                "message": f"Firewall {firewall_id} deleted successfully",
                "firewall_id": firewall_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete firewall: {e}")]


def create_linode_domain_create_tool() -> Tool:
    """Create the linode_domain_create tool."""
    return Tool(
        name="linode_domain_create",
        description="Creates a new DNS domain.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "domain": {
                    "type": "string",
                    "description": "The domain name (required)",
                },
                "type": {
                    "type": "string",
                    "description": (
                        "Domain type: 'master' or 'slave' (default: 'master')"
                    ),
                },
                "soa_email": {
                    "type": "string",
                    "description": "SOA email address (required for master domains)",
                },
                "description": {
                    "type": "string",
                    "description": "Description for the domain (optional)",
                },
            },
            "required": ["domain"],
        },
    )


async def handle_linode_domain_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_create tool request."""
    environment = arguments.get("environment", "")
    domain_name = arguments.get("domain", "")

    if not domain_name:
        return [TextContent(type="text", text="Error: domain is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            domain = await client.create_domain(
                domain=domain_name,
                domain_type=arguments.get("type", "master"),
                soa_email=arguments.get("soa_email"),
                description=arguments.get("description"),
            )

            response = {
                "message": (
                    f"Domain '{domain.domain}' (ID: {domain.id}) created successfully"
                ),
                "domain": {
                    "id": domain.id,
                    "domain": domain.domain,
                    "type": domain.type,
                    "status": domain.status,
                    "created": domain.created,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create domain: {e}")]


def create_linode_domain_update_tool() -> Tool:
    """Create the linode_domain_update tool."""
    return Tool(
        name="linode_domain_update",
        description="Updates an existing DNS domain.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to update (required)",
                },
                "domain": {
                    "type": "string",
                    "description": "New domain name (optional)",
                },
                "soa_email": {
                    "type": "string",
                    "description": "New SOA email address (optional)",
                },
                "description": {
                    "type": "string",
                    "description": "New description (optional)",
                },
            },
            "required": ["domain_id"],
        },
    )


async def handle_linode_domain_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_update tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            domain = await client.update_domain(
                domain_id=int(domain_id),
                domain=arguments.get("domain"),
                soa_email=arguments.get("soa_email"),
                description=arguments.get("description"),
            )

            response = {
                "message": f"Domain {domain_id} updated successfully",
                "domain": {
                    "id": domain.id,
                    "domain": domain.domain,
                    "type": domain.type,
                    "status": domain.status,
                    "updated": domain.updated,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to update domain: {e}")]


def create_linode_domain_delete_tool() -> Tool:
    """Create the linode_domain_delete tool."""
    return Tool(
        name="linode_domain_delete",
        description=(
            "Deletes a DNS domain. WARNING: This also deletes all associated records."
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
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["domain_id", "confirm"],
        },
    )


async def handle_linode_domain_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_delete tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_domain(int(domain_id))

            response = {
                "message": f"Domain {domain_id} deleted successfully",
                "domain_id": domain_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete domain: {e}")]


def create_linode_domain_record_create_tool() -> Tool:
    """Create the linode_domain_record_create tool."""
    return Tool(
        name="linode_domain_record_create",
        description="Creates a new DNS record for a domain.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain (required)",
                },
                "type": {
                    "type": "string",
                    "description": (
                        "Record type: A, AAAA, NS, MX, CNAME, TXT, SRV, CAA (required)"
                    ),
                },
                "name": {
                    "type": "string",
                    "description": "Record name/subdomain (optional)",
                },
                "target": {
                    "type": "string",
                    "description": (
                        "Target value for the record (required for most types)"
                    ),
                },
                "priority": {
                    "type": "integer",
                    "description": "Priority (for MX and SRV records)",
                },
                "weight": {
                    "type": "integer",
                    "description": "Weight (for SRV records)",
                },
                "port": {
                    "type": "integer",
                    "description": "Port (for SRV records)",
                },
                "ttl_sec": {
                    "type": "integer",
                    "description": "TTL in seconds (optional)",
                },
            },
            "required": ["domain_id", "type"],
        },
    )


async def handle_linode_domain_record_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_create tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)
    record_type = arguments.get("type", "")

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]
    if not record_type:
        return [TextContent(type="text", text="Error: type is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            record = await client.create_domain_record(
                domain_id=int(domain_id),
                record_type=record_type,
                name=arguments.get("name"),
                target=arguments.get("target"),
                priority=arguments.get("priority"),
                weight=arguments.get("weight"),
                port=arguments.get("port"),
                ttl_sec=arguments.get("ttl_sec"),
            )

            response = {
                "message": (
                    f"DNS record (ID: {record.id}) created successfully "
                    f"for domain {domain_id}"
                ),
                "record": {
                    "id": record.id,
                    "type": record.type,
                    "name": record.name,
                    "target": record.target,
                    "ttl_sec": record.ttl_sec,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create DNS record: {e}")]


def create_linode_domain_record_update_tool() -> Tool:
    """Create the linode_domain_record_update tool."""
    return Tool(
        name="linode_domain_record_update",
        description="Updates an existing DNS record.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain (required)",
                },
                "record_id": {
                    "type": "integer",
                    "description": "The ID of the record to update (required)",
                },
                "name": {
                    "type": "string",
                    "description": "New record name (optional)",
                },
                "target": {
                    "type": "string",
                    "description": "New target value (optional)",
                },
                "priority": {
                    "type": "integer",
                    "description": "New priority (optional)",
                },
                "weight": {
                    "type": "integer",
                    "description": "New weight (optional)",
                },
                "port": {
                    "type": "integer",
                    "description": "New port (optional)",
                },
                "ttl_sec": {
                    "type": "integer",
                    "description": "New TTL in seconds (optional)",
                },
            },
            "required": ["domain_id", "record_id"],
        },
    )


async def handle_linode_domain_record_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_update tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]
    if not record_id:
        return [TextContent(type="text", text="Error: record_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            record = await client.update_domain_record(
                domain_id=int(domain_id),
                record_id=int(record_id),
                name=arguments.get("name"),
                target=arguments.get("target"),
                priority=arguments.get("priority"),
                weight=arguments.get("weight"),
                port=arguments.get("port"),
                ttl_sec=arguments.get("ttl_sec"),
            )

            response = {
                "message": f"DNS record {record_id} updated successfully",
                "record": {
                    "id": record.id,
                    "type": record.type,
                    "name": record.name,
                    "target": record.target,
                    "ttl_sec": record.ttl_sec,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to update DNS record: {e}")]


def create_linode_domain_record_delete_tool() -> Tool:
    """Create the linode_domain_record_delete tool."""
    return Tool(
        name="linode_domain_record_delete",
        description="Deletes a DNS record.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain (required)",
                },
                "record_id": {
                    "type": "integer",
                    "description": "The ID of the record to delete (required)",
                },
            },
            "required": ["domain_id", "record_id"],
        },
    )


async def handle_linode_domain_record_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_delete tool request."""
    environment = arguments.get("environment", "")
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return [TextContent(type="text", text="Error: domain_id is required")]
    if not record_id:
        return [TextContent(type="text", text="Error: record_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_domain_record(int(domain_id), int(record_id))

            response = {
                "message": f"DNS record {record_id} deleted successfully",
                "domain_id": domain_id,
                "record_id": record_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete DNS record: {e}")]


def create_linode_volume_create_tool() -> Tool:
    """Create the linode_volume_create tool."""
    return Tool(
        name="linode_volume_create",
        description=(
            "Creates a new block storage volume. WARNING: Billing starts immediately."
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
                "label": {
                    "type": "string",
                    "description": "Label for the volume (required)",
                },
                "region": {
                    "type": "string",
                    "description": "Region for the volume (required if not attaching)",
                },
                "size": {
                    "type": "integer",
                    "description": "Size in GB (default: 20, min: 10, max: 10240)",
                },
                "linode_id": {
                    "type": "integer",
                    "description": "Linode ID to attach to (optional)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["label", "confirm"],
        },
    )


async def handle_linode_volume_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_create tool request."""
    environment = arguments.get("environment", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    label = arguments.get("label", "")
    if not label:
        return [TextContent(type="text", text="Error: label is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            volume = await client.create_volume(
                label=label,
                region=arguments.get("region"),
                linode_id=arguments.get("linode_id"),
                size=arguments.get("size", 20),
            )

            response = {
                "message": (
                    f"Volume '{volume.label}' (ID: {volume.id}) "
                    f"created successfully in {volume.region}"
                ),
                "volume": {
                    "id": volume.id,
                    "label": volume.label,
                    "size": volume.size,
                    "region": volume.region,
                    "status": volume.status,
                    "filesystem_path": volume.filesystem_path,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create volume: {e}")]


def create_linode_volume_attach_tool() -> Tool:
    """Create the linode_volume_attach tool."""
    return Tool(
        name="linode_volume_attach",
        description="Attaches a block storage volume to a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to attach (required)",
                },
                "linode_id": {
                    "type": "integer",
                    "description": "The ID of the Linode to attach to (required)",
                },
                "config_id": {
                    "type": "integer",
                    "description": "Config profile ID (optional)",
                },
                "persist_across_boots": {
                    "type": "boolean",
                    "description": "Keep attached across reboots (default: false)",
                },
            },
            "required": ["volume_id", "linode_id"],
        },
    )


async def handle_linode_volume_attach(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_attach tool request."""
    environment = arguments.get("environment", "")
    volume_id = arguments.get("volume_id", 0)
    linode_id = arguments.get("linode_id", 0)

    if not volume_id:
        return [TextContent(type="text", text="Error: volume_id is required")]
    if not linode_id:
        return [TextContent(type="text", text="Error: linode_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            volume = await client.attach_volume(
                volume_id=int(volume_id),
                linode_id=int(linode_id),
                config_id=arguments.get("config_id"),
                persist_across_boots=arguments.get("persist_across_boots", False),
            )

            response = {
                "message": (
                    f"Volume {volume_id} attached to Linode {linode_id} successfully"
                ),
                "volume": {
                    "id": volume.id,
                    "label": volume.label,
                    "linode_id": volume.linode_id,
                    "filesystem_path": volume.filesystem_path,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to attach volume: {e}")]


def create_linode_volume_detach_tool() -> Tool:
    """Create the linode_volume_detach tool."""
    return Tool(
        name="linode_volume_detach",
        description="Detaches a block storage volume from a Linode instance.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to detach (required)",
                },
            },
            "required": ["volume_id"],
        },
    )


async def handle_linode_volume_detach(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_detach tool request."""
    environment = arguments.get("environment", "")
    volume_id = arguments.get("volume_id", 0)

    if not volume_id:
        return [TextContent(type="text", text="Error: volume_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.detach_volume(int(volume_id))

            response = {
                "message": f"Volume {volume_id} detached successfully",
                "volume_id": volume_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to detach volume: {e}")]


def create_linode_volume_resize_tool() -> Tool:
    """Create the linode_volume_resize tool."""
    return Tool(
        name="linode_volume_resize",
        description=(
            "Resizes a block storage volume. WARNING: Volumes can only be resized "
            "up, not down. This increases billing."
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
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to resize (required)",
                },
                "size": {
                    "type": "integer",
                    "description": "New size in GB (must be larger than current)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm resize. This increases billing."
                    ),
                },
            },
            "required": ["volume_id", "size", "confirm"],
        },
    )


async def handle_linode_volume_resize(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_resize tool request."""
    environment = arguments.get("environment", "")
    volume_id = arguments.get("volume_id", 0)
    size = arguments.get("size", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This increases billing. Set confirm=true to proceed.",
            )
        ]

    if not volume_id:
        return [TextContent(type="text", text="Error: volume_id is required")]
    if not size:
        return [TextContent(type="text", text="Error: size is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            volume = await client.resize_volume(int(volume_id), int(size))

            response = {
                "message": f"Volume {volume_id} resized to {size}GB successfully",
                "volume": {
                    "id": volume.id,
                    "label": volume.label,
                    "size": volume.size,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to resize volume: {e}")]


def create_linode_volume_delete_tool() -> Tool:
    """Create the linode_volume_delete tool."""
    return Tool(
        name="linode_volume_delete",
        description=(
            "Deletes a block storage volume. WARNING: This is destructive "
            "and all data will be lost. Volume must be detached first."
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
                "volume_id": {
                    "type": "integer",
                    "description": "The ID of the volume to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["volume_id", "confirm"],
        },
    )


async def handle_linode_volume_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_volume_delete tool request."""
    environment = arguments.get("environment", "")
    volume_id = arguments.get("volume_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not volume_id:
        return [TextContent(type="text", text="Error: volume_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_volume(int(volume_id))

            response = {
                "message": f"Volume {volume_id} deleted successfully",
                "volume_id": volume_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete volume: {e}")]


def create_linode_nodebalancer_create_tool() -> Tool:
    """Create the linode_nodebalancer_create tool."""
    return Tool(
        name="linode_nodebalancer_create",
        description=(
            "Creates a new NodeBalancer (load balancer). "
            "WARNING: Billing starts immediately."
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
                    "description": "Region for the NodeBalancer (required)",
                },
                "label": {
                    "type": "string",
                    "description": "Label for the NodeBalancer (optional)",
                },
                "client_conn_throttle": {
                    "type": "integer",
                    "description": "Connections per second throttle (0-20, default: 0)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["region", "confirm"],
        },
    )


async def handle_linode_nodebalancer_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_create tool request."""
    environment = arguments.get("environment", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This creates a billable resource. Set confirm=true.",
            )
        ]

    region = arguments.get("region", "")
    if not region:
        return [TextContent(type="text", text="Error: region is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            nb = await client.create_nodebalancer(
                region=region,
                label=arguments.get("label"),
                client_conn_throttle=arguments.get("client_conn_throttle", 0),
            )

            response = {
                "message": (
                    f"NodeBalancer '{nb.label}' (ID: {nb.id}) "
                    f"created successfully in {nb.region}"
                ),
                "nodebalancer": {
                    "id": nb.id,
                    "label": nb.label,
                    "region": nb.region,
                    "hostname": nb.hostname,
                    "ipv4": nb.ipv4,
                    "ipv6": nb.ipv6,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to create NodeBalancer: {e}")]


def create_linode_nodebalancer_update_tool() -> Tool:
    """Create the linode_nodebalancer_update tool."""
    return Tool(
        name="linode_nodebalancer_update",
        description="Updates an existing NodeBalancer.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "nodebalancer_id": {
                    "type": "integer",
                    "description": "The ID of the NodeBalancer to update (required)",
                },
                "label": {
                    "type": "string",
                    "description": "New label (optional)",
                },
                "client_conn_throttle": {
                    "type": "integer",
                    "description": "New throttle limit (0-20, optional)",
                },
            },
            "required": ["nodebalancer_id"],
        },
    )


async def handle_linode_nodebalancer_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_update tool request."""
    environment = arguments.get("environment", "")
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return [TextContent(type="text", text="Error: nodebalancer_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            nb = await client.update_nodebalancer(
                nodebalancer_id=int(nodebalancer_id),
                label=arguments.get("label"),
                client_conn_throttle=arguments.get("client_conn_throttle"),
            )

            response = {
                "message": f"NodeBalancer {nodebalancer_id} updated successfully",
                "nodebalancer": {
                    "id": nb.id,
                    "label": nb.label,
                    "client_conn_throttle": nb.client_conn_throttle,
                    "updated": nb.updated,
                },
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to update NodeBalancer: {e}")]


def create_linode_nodebalancer_delete_tool() -> Tool:
    """Create the linode_nodebalancer_delete tool."""
    return Tool(
        name="linode_nodebalancer_delete",
        description=(
            "Deletes a NodeBalancer. WARNING: This removes the load balancer "
            "and all its configurations."
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
                "nodebalancer_id": {
                    "type": "integer",
                    "description": "The ID of the NodeBalancer to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm deletion.",
                },
            },
            "required": ["nodebalancer_id", "confirm"],
        },
    )


async def handle_linode_nodebalancer_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_delete tool request."""
    environment = arguments.get("environment", "")
    nodebalancer_id = arguments.get("nodebalancer_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    if not nodebalancer_id:
        return [TextContent(type="text", text="Error: nodebalancer_id is required")]

    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)

        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            await client.delete_nodebalancer(int(nodebalancer_id))

            response = {
                "message": f"NodeBalancer {nodebalancer_id} deleted successfully",
                "nodebalancer_id": nodebalancer_id,
            }

            json_response = json.dumps(response, indent=2)
            return [TextContent(type="text", text=json_response)]

    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to delete NodeBalancer: {e}")]
