"""MCP tools for LinodeMCP."""

import json
from typing import Any

from mcp.types import TextContent, Tool

from linodemcp.config import Config, EnvironmentConfig, EnvironmentNotFoundError
from linodemcp.linode import RetryableClient, RetryConfig
from linodemcp.version import get_version_info

__all__ = [
    "create_hello_tool",
    "create_linode_instances_list_tool",
    "create_linode_profile_tool",
    "create_version_tool",
    "handle_hello",
    "handle_linode_instances_list",
    "handle_linode_profile",
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


async def handle_version(arguments: dict[str, Any]) -> list[TextContent]:
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
