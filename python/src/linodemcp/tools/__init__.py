"""MCP tools for LinodeMCP."""

import json
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config, EnvironmentConfig, EnvironmentNotFoundError
from linodemcp.linode import RetryableClient, RetryConfig
from linodemcp.version import get_version_info

__all__ = [
    "create_hello_tool",
    "create_linode_account_tool",
    "create_linode_images_list_tool",
    "create_linode_instance_get_tool",
    "create_linode_instances_list_tool",
    "create_linode_profile_tool",
    "create_linode_regions_list_tool",
    "create_linode_types_list_tool",
    "create_linode_volumes_list_tool",
    "create_version_tool",
    "handle_hello",
    "handle_linode_account",
    "handle_linode_images_list",
    "handle_linode_instance_get",
    "handle_linode_instances_list",
    "handle_linode_profile",
    "handle_linode_regions_list",
    "handle_linode_types_list",
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
                types = [
                    t for t in types if t.class_.lower() == class_filter.lower()
                ]

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
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode types: {e}")
        ]


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
                    v
                    for v in volumes
                    if label_contains.lower() in v.label.lower()
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
        return [
            TextContent(type="text", text=f"Failed to retrieve Linode images: {e}")
        ]
