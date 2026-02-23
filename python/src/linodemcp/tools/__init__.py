"""MCP tools for LinodeMCP."""

import json
import re
from collections.abc import Awaitable, Callable
from typing import Any, NotRequired, TypedDict

from mcp.types import TextContent, Tool

from linodemcp.config import Config, EnvironmentConfig, EnvironmentNotFoundError
from linodemcp.linode import RetryableClient, RetryConfig
from linodemcp.version import get_version_info


class EnvironmentArgs(TypedDict):
    """Common arguments with environment field."""

    environment: NotRequired[str]


class HelloArgs(TypedDict):
    """Arguments for hello tool."""

    name: NotRequired[str]


class InstanceFilterArgs(EnvironmentArgs):
    """Arguments for instance listing with filters."""

    status: NotRequired[str]


class InstanceIDArgs(EnvironmentArgs):
    """Arguments requiring instance_id."""

    instance_id: int


class RegionFilterArgs(EnvironmentArgs):
    """Arguments for region listing with filters."""

    capabilities: NotRequired[str]


class TypeFilterArgs(EnvironmentArgs):
    """Arguments for type listing with filters."""

    type_class: NotRequired[str]


class VolumeFilterArgs(EnvironmentArgs):
    """Arguments for volume listing with filters."""

    region: NotRequired[str]
    label: NotRequired[str]


class ImageFilterArgs(EnvironmentArgs):
    """Arguments for image listing with filters."""

    is_public: NotRequired[bool]
    include_deprecated: NotRequired[bool]


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
    "create_linode_lke_acl_delete_tool",
    "create_linode_lke_acl_get_tool",
    "create_linode_lke_acl_update_tool",
    "create_linode_lke_api_endpoints_list_tool",
    "create_linode_lke_cluster_create_tool",
    "create_linode_lke_cluster_delete_tool",
    "create_linode_lke_cluster_get_tool",
    "create_linode_lke_cluster_recycle_tool",
    "create_linode_lke_cluster_regenerate_tool",
    "create_linode_lke_cluster_update_tool",
    "create_linode_lke_clusters_list_tool",
    "create_linode_lke_dashboard_get_tool",
    "create_linode_lke_kubeconfig_delete_tool",
    "create_linode_lke_kubeconfig_get_tool",
    "create_linode_lke_node_delete_tool",
    "create_linode_lke_node_get_tool",
    "create_linode_lke_node_recycle_tool",
    "create_linode_lke_pool_create_tool",
    "create_linode_lke_pool_delete_tool",
    "create_linode_lke_pool_get_tool",
    "create_linode_lke_pool_recycle_tool",
    "create_linode_lke_pool_update_tool",
    "create_linode_lke_pools_list_tool",
    "create_linode_lke_service_token_delete_tool",
    "create_linode_lke_tier_versions_list_tool",
    "create_linode_lke_types_list_tool",
    "create_linode_lke_version_get_tool",
    "create_linode_lke_versions_list_tool",
    "create_linode_nodebalancer_create_tool",
    "create_linode_nodebalancer_delete_tool",
    "create_linode_nodebalancer_get_tool",
    "create_linode_nodebalancer_update_tool",
    "create_linode_nodebalancers_list_tool",
    "create_linode_object_storage_bucket_access_get_tool",
    "create_linode_object_storage_bucket_access_update_tool",
    "create_linode_object_storage_bucket_contents_tool",
    "create_linode_object_storage_bucket_create_tool",
    "create_linode_object_storage_bucket_delete_tool",
    "create_linode_object_storage_bucket_get_tool",
    "create_linode_object_storage_buckets_list_tool",
    "create_linode_object_storage_clusters_list_tool",
    "create_linode_object_storage_key_create_tool",
    "create_linode_object_storage_key_delete_tool",
    "create_linode_object_storage_key_get_tool",
    "create_linode_object_storage_key_update_tool",
    "create_linode_object_storage_keys_list_tool",
    "create_linode_object_storage_object_acl_get_tool",
    "create_linode_object_storage_object_acl_update_tool",
    "create_linode_object_storage_presigned_url_tool",
    "create_linode_object_storage_ssl_delete_tool",
    "create_linode_object_storage_ssl_get_tool",
    "create_linode_object_storage_transfer_tool",
    "create_linode_object_storage_types_list_tool",
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
    "handle_linode_lke_acl_delete",
    "handle_linode_lke_acl_get",
    "handle_linode_lke_acl_update",
    "handle_linode_lke_api_endpoints_list",
    "handle_linode_lke_cluster_create",
    "handle_linode_lke_cluster_delete",
    "handle_linode_lke_cluster_get",
    "handle_linode_lke_cluster_recycle",
    "handle_linode_lke_cluster_regenerate",
    "handle_linode_lke_cluster_update",
    "handle_linode_lke_clusters_list",
    "handle_linode_lke_dashboard_get",
    "handle_linode_lke_kubeconfig_delete",
    "handle_linode_lke_kubeconfig_get",
    "handle_linode_lke_node_delete",
    "handle_linode_lke_node_get",
    "handle_linode_lke_node_recycle",
    "handle_linode_lke_pool_create",
    "handle_linode_lke_pool_delete",
    "handle_linode_lke_pool_get",
    "handle_linode_lke_pool_recycle",
    "handle_linode_lke_pool_update",
    "handle_linode_lke_pools_list",
    "handle_linode_lke_service_token_delete",
    "handle_linode_lke_tier_versions_list",
    "handle_linode_lke_types_list",
    "handle_linode_lke_version_get",
    "handle_linode_lke_versions_list",
    "handle_linode_nodebalancer_create",
    "handle_linode_nodebalancer_delete",
    "handle_linode_nodebalancer_get",
    "handle_linode_nodebalancer_update",
    "handle_linode_nodebalancers_list",
    "handle_linode_object_storage_bucket_access_get",
    "handle_linode_object_storage_bucket_access_update",
    "handle_linode_object_storage_bucket_contents",
    "handle_linode_object_storage_bucket_create",
    "handle_linode_object_storage_bucket_delete",
    "handle_linode_object_storage_bucket_get",
    "handle_linode_object_storage_buckets_list",
    "handle_linode_object_storage_clusters_list",
    "handle_linode_object_storage_key_create",
    "handle_linode_object_storage_key_delete",
    "handle_linode_object_storage_key_get",
    "handle_linode_object_storage_key_update",
    "handle_linode_object_storage_keys_list",
    "handle_linode_object_storage_object_acl_get",
    "handle_linode_object_storage_object_acl_update",
    "handle_linode_object_storage_presigned_url",
    "handle_linode_object_storage_ssl_delete",
    "handle_linode_object_storage_ssl_get",
    "handle_linode_object_storage_transfer",
    "handle_linode_object_storage_types_list",
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
    """Handle hello tool request.

    Args:
        arguments: HelloArgs - name (optional)
    """
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
    """Handle linode_profile tool request.

    Args:
        arguments: EnvironmentArgs - environment (optional)
        cfg: Configuration object
    """

    async def _call(client: RetryableClient) -> dict[str, Any]:
        profile = await client.get_profile()
        return {
            "username": profile.username,
            "email": profile.email,
            "timezone": profile.timezone,
            "email_notifications": profile.email_notifications,
            "restricted": profile.restricted,
            "two_factor_auth": profile.two_factor_auth,
            "uid": profile.uid,
        }

    return await execute_tool(cfg, arguments, "retrieve Linode profile", _call)


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
    """Handle linode_instances_list tool request.

    Args:
        arguments: InstanceFilterArgs - environment, status (optional)
        cfg: Configuration object
    """
    status_filter = arguments.get("status", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instances = await client.list_instances()

        if status_filter:
            instances = [
                inst
                for inst in instances
                if inst.status.lower() == status_filter.lower()
            ]

        instances_data = [
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
            for inst in instances
        ]

        response: dict[str, Any] = {
            "count": len(instances),
            "instances": instances_data,
        }

        if status_filter:
            response["filter"] = f"status={status_filter}"

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode instances", _call)


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


ToolCallback = Callable[[RetryableClient], Awaitable[dict[str, Any]]]


async def execute_tool(
    cfg: Config,
    arguments: dict[str, Any],
    error_action: str,
    callback: ToolCallback,
) -> list[TextContent]:
    """Run a tool handler with standard environment/client/error boilerplate."""
    environment = arguments.get("environment", "")
    try:
        selected_env = _select_environment(cfg, environment)
        _validate_linode_config(selected_env)
        async with RetryableClient(
            selected_env.linode.api_url,
            selected_env.linode.token,
            RetryConfig(),
        ) as client:
            response = await callback(client)
            return [TextContent(type="text", text=json.dumps(response, indent=2))]
    except (EnvironmentNotFoundError, ValueError) as e:
        return [TextContent(type="text", text=f"Error: {e}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Failed to {error_action}: {e}")]


def _error_response(message: str) -> list[TextContent]:
    """Return a single-element TextContent error list."""
    return [TextContent(type="text", text=f"Error: {message}")]


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
    """Handle linode_instance_get tool request.

    Args:
        arguments: InstanceIDArgs - instance_id, environment (optional)
        cfg: Configuration object
    """
    instance_id_str = arguments.get("instance_id", "")

    if not instance_id_str:
        return _error_response("instance_id is required")

    try:
        instance_id = int(instance_id_str)
    except ValueError:
        return _error_response("instance_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        instance = await client.get_instance(instance_id)
        return {
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

    return await execute_tool(cfg, arguments, "retrieve Linode instance", _call)


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
    """Handle linode_account tool request.

    Args:
        arguments: EnvironmentArgs - environment (optional)
        cfg: Configuration object
    """

    async def _call(client: RetryableClient) -> dict[str, Any]:
        account = await client.get_account()
        return {
            "first_name": account.first_name,
            "last_name": account.last_name,
            "email": account.email,
            "company": account.company,
            "balance": account.balance,
            "balance_uninvoiced": account.balance_uninvoiced,
            "capabilities": account.capabilities,
            "active_since": account.active_since,
        }

    return await execute_tool(cfg, arguments, "retrieve Linode account", _call)


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
    country_filter: str = arguments.get("country", "")
    capability_filter: str = arguments.get("capability", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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
                    cap.lower() == capability_filter.lower() for cap in r.capabilities
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

        filters: list[str] = []
        if country_filter:
            filters.append(f"country={country_filter}")
        if capability_filter:
            filters.append(f"capability={capability_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode regions", _call)


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
    class_filter: str = arguments.get("class", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode types", _call)


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
    region_filter: str = arguments.get("region", "")
    label_contains: str = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volumes = await client.list_volumes()

        if region_filter:
            volumes = [v for v in volumes if v.region.lower() == region_filter.lower()]

        if label_contains:
            volumes = [v for v in volumes if label_contains.lower() in v.label.lower()]

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

        filters: list[str] = []
        if region_filter:
            filters.append(f"region={region_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode volumes", _call)


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
    type_filter: str = arguments.get("type", "")
    is_public_filter: str | bool = arguments.get("is_public", "")
    deprecated_filter: str = arguments.get("deprecated", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        images = await client.list_images()

        if type_filter:
            images = [i for i in images if i.type.lower() == type_filter.lower()]

        if is_public_filter:
            want_public = (
                is_public_filter.lower() == "true"
                if isinstance(is_public_filter, str)
                else is_public_filter
            )
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

        filters: list[str] = []
        if type_filter:
            filters.append(f"type={type_filter}")
        if is_public_filter:
            filters.append(f"is_public={is_public_filter}")
        if deprecated_filter:
            filters.append(f"deprecated={deprecated_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve Linode images", _call)


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
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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

        return response

    return await execute_tool(cfg, arguments, "retrieve SSH keys", _call)


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
    domain_contains = arguments.get("domain_contains", "")
    type_filter = arguments.get("type", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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

        filters: list[str] = []
        if domain_contains:
            filters.append(f"domain_contains={domain_contains}")
        if type_filter:
            filters.append(f"type={type_filter}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve domains", _call)


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
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.get_domain(int(domain_id))
        return {
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

    return await execute_tool(cfg, arguments, "retrieve domain", _call)


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
    domain_id = arguments.get("domain_id", 0)
    type_filter = arguments.get("type", "")
    name_contains = arguments.get("name_contains", "")

    if not domain_id:
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        records = await client.list_domain_records(int(domain_id))

        if type_filter:
            records = [r for r in records if r.type.upper() == type_filter.upper()]

        if name_contains:
            records = [r for r in records if name_contains.lower() in r.name.lower()]

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

        filters: list[str] = []
        if type_filter:
            filters.append(f"type={type_filter}")
        if name_contains:
            filters.append(f"name_contains={name_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve domain records", _call)


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
    region_filter = arguments.get("region", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nodebalancers = await client.list_nodebalancers()

        if region_filter:
            nodebalancers = [
                nb for nb in nodebalancers if nb.region.lower() == region_filter.lower()
            ]

        if label_contains:
            nodebalancers = [
                nb for nb in nodebalancers if label_contains.lower() in nb.label.lower()
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

        filters: list[str] = []
        if region_filter:
            filters.append(f"region={region_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve NodeBalancers", _call)


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
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return _error_response("nodebalancer_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.get_nodebalancer(int(nodebalancer_id))
        return {
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

    return await execute_tool(cfg, arguments, "retrieve NodeBalancer", _call)


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
    is_public_filter = arguments.get("is_public", "")
    mine_filter = arguments.get("mine", "")
    label_contains = arguments.get("label_contains", "")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        scripts = await client.list_stackscripts()

        if is_public_filter:
            want_public = is_public_filter.lower() == "true"
            scripts = [s for s in scripts if s.is_public == want_public]

        if mine_filter:
            want_mine = mine_filter.lower() == "true"
            scripts = [s for s in scripts if s.mine == want_mine]

        if label_contains:
            scripts = [s for s in scripts if label_contains.lower() in s.label.lower()]

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

        filters: list[str] = []
        if is_public_filter:
            filters.append(f"is_public={is_public_filter}")
        if mine_filter:
            filters.append(f"mine={mine_filter}")
        if label_contains:
            filters.append(f"label_contains={label_contains}")
        if filters:
            response["filter"] = ", ".join(filters)

        return response

    return await execute_tool(cfg, arguments, "retrieve StackScripts", _call)


# Phase 1: Object Storage read operations


def create_linode_object_storage_buckets_list_tool() -> Tool:
    """Create the linode_object_storage_buckets_list tool."""
    return Tool(
        name="linode_object_storage_buckets_list",
        description="Lists all Object Storage buckets on your Linode account.",
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


async def handle_linode_object_storage_buckets_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_buckets_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        buckets = await client.list_object_storage_buckets()
        return {
            "count": len(buckets),
            "buckets": buckets,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage buckets", _call)


def create_linode_object_storage_bucket_get_tool() -> Tool:
    """Create the linode_object_storage_bucket_get tool."""
    return Tool(
        name="linode_object_storage_bucket_get",
        description="Gets details about a specific Object Storage bucket.",
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
                        "The region/cluster ID where the bucket exists (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "The label/name of the bucket (required)",
                },
            },
            "required": ["region", "label"],
        },
    )


async def handle_linode_object_storage_bucket_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_bucket(region, label)

    return await execute_tool(cfg, arguments, "retrieve Object Storage bucket", _call)


def create_linode_object_storage_bucket_contents_tool() -> Tool:
    """Create the linode_object_storage_bucket_contents tool."""
    return Tool(
        name="linode_object_storage_bucket_contents",
        description=(
            "Lists objects in an Object Storage bucket. "
            "Supports pagination and filtering by prefix/delimiter."
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
                        "The region/cluster ID where the bucket exists (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "The label/name of the bucket (required)",
                },
                "prefix": {
                    "type": "string",
                    "description": (
                        "Limits results to object keys that begin with this prefix"
                    ),
                },
                "delimiter": {
                    "type": "string",
                    "description": (
                        "Character used to group keys "
                        "(e.g., '/' for directory-like listing)"
                    ),
                },
                "marker": {
                    "type": "string",
                    "description": "Object key to start listing from (for pagination)",
                },
                "page_size": {
                    "type": "string",
                    "description": "Number of objects to return per page (1-1000)",
                },
            },
            "required": ["region", "label"],
        },
    )


def _build_bucket_params(
    prefix: str, delimiter: str, marker: str, page_size: str
) -> dict[str, str]:
    """Build parameters dictionary for bucket contents request."""
    params: dict[str, str] = {}
    if prefix:
        params["prefix"] = prefix
    if delimiter:
        params["delimiter"] = delimiter
    if marker:
        params["marker"] = marker
    if page_size:
        params["page_size"] = page_size
    return params


def _build_bucket_filter_string(
    prefix: str, delimiter: str, marker: str, page_size: str
) -> str:
    """Build filter string for bucket contents response."""
    filters: list[str] = []
    if prefix:
        filters.append(f"prefix={prefix}")
    if delimiter:
        filters.append(f"delimiter={delimiter}")
    if marker:
        filters.append(f"marker={marker}")
    if page_size:
        filters.append(f"page_size={page_size}")
    return ", ".join(filters) if filters else ""


async def handle_linode_object_storage_bucket_contents(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_contents tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    prefix = arguments.get("prefix", "")
    delimiter = arguments.get("delimiter", "")
    marker = arguments.get("marker", "")
    page_size = arguments.get("page_size", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        params = _build_bucket_params(prefix, delimiter, marker, page_size)
        result = await client.list_object_storage_bucket_contents(
            region, label, params or None
        )

        objects = result.get("data", [])
        response: dict[str, Any] = {
            "count": len(objects),
            "objects": objects,
            "is_truncated": result.get("is_truncated", False),
        }

        if result.get("next_marker"):
            response["next_marker"] = result["next_marker"]

        filter_str = _build_bucket_filter_string(prefix, delimiter, marker, page_size)
        if filter_str:
            response["filter"] = filter_str

        return response

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage bucket contents", _call
    )


def create_linode_object_storage_clusters_list_tool() -> Tool:
    """Create the linode_object_storage_clusters_list tool."""
    return Tool(
        name="linode_object_storage_clusters_list",
        description=(
            "Lists available Object Storage clusters/regions. "
            "Shows which regions support Object Storage and their endpoints."
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


async def handle_linode_object_storage_clusters_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_clusters_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        clusters = await client.list_object_storage_clusters()
        return {
            "count": len(clusters),
            "clusters": clusters,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage clusters", _call)


def create_linode_object_storage_types_list_tool() -> Tool:
    """Create the linode_object_storage_types_list tool."""
    return Tool(
        name="linode_object_storage_types_list",
        description=(
            "Lists Object Storage pricing tiers and capabilities. Shows pricing, "
            "storage limits, and transfer allowances."
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


async def handle_linode_object_storage_types_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_types_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_object_storage_types()
        return {
            "count": len(types),
            "types": types,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage types", _call)


# Phase 2: Read-Only Access Key & Transfer Tools


def create_linode_object_storage_keys_list_tool() -> Tool:
    """Create the linode_object_storage_keys_list tool."""
    return Tool(
        name="linode_object_storage_keys_list",
        description="Lists all Object Storage access keys for the authenticated user.",
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


async def handle_linode_object_storage_keys_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_keys_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        keys = await client.list_object_storage_keys()
        return {
            "count": len(keys),
            "keys": keys,
        }

    return await execute_tool(cfg, arguments, "retrieve Object Storage keys", _call)


def create_linode_object_storage_key_get_tool() -> Tool:
    """Create the linode_object_storage_key_get tool."""
    return Tool(
        name="linode_object_storage_key_get",
        description="Gets details about a specific Object Storage access key by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "key_id": {
                    "type": "integer",
                    "description": "The ID of the access key to retrieve (required)",
                },
            },
            "required": ["key_id"],
        },
    )


async def handle_linode_object_storage_key_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_key_get tool request."""
    key_id = arguments.get("key_id", 0)

    if not key_id:
        return _error_response("key_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_key(int(key_id))

    return await execute_tool(cfg, arguments, "retrieve Object Storage key", _call)


def create_linode_object_storage_transfer_tool() -> Tool:
    """Create the linode_object_storage_transfer tool."""
    return Tool(
        name="linode_object_storage_transfer",
        description=(
            "Gets Object Storage outbound data transfer usage for the current month."
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


async def handle_linode_object_storage_transfer(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_transfer tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_transfer()

    return await execute_tool(
        cfg, arguments, "retrieve Object Storage transfer usage", _call
    )


def create_linode_object_storage_bucket_access_get_tool() -> Tool:
    """Create the linode_object_storage_bucket_access_get tool."""
    return Tool(
        name="linode_object_storage_bucket_access_get",
        description=(
            "Gets the ACL and CORS settings for a specific Object Storage bucket."
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
                        "The region/cluster ID where the bucket exists (required)"
                    ),
                },
                "label": {
                    "type": "string",
                    "description": "The label/name of the bucket (required)",
                },
            },
            "required": ["region", "label"],
        },
    )


async def handle_linode_object_storage_bucket_access_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_access_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_storage_bucket_access(region, label)

    return await execute_tool(cfg, arguments, "retrieve bucket access settings", _call)


# Stage 5 Phase 3: Object Storage write operations

_VALID_BUCKET_LABEL_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]{1,2}$")
_VALID_ACLS = {"private", "public-read", "authenticated-read", "public-read-write"}
_MIN_BUCKET_LABEL_LENGTH = 3
_MAX_BUCKET_LABEL_LENGTH = 63


def _validate_bucket_label(label: str) -> str | None:
    """Validate S3 bucket label. Returns error message or None."""
    if not label:
        return "label is required"
    if len(label) < _MIN_BUCKET_LABEL_LENGTH:
        return "bucket label must be at least 3 characters"
    if len(label) > _MAX_BUCKET_LABEL_LENGTH:
        return "bucket label must not exceed 63 characters"
    if not _VALID_BUCKET_LABEL_RE.match(label):
        return "bucket label must contain only lowercase letters, numbers, and hyphens"
    first, last = label[0], label[-1]
    if not (first.isalnum() and last.isalnum()):
        return "bucket label must start and end with a lowercase letter or number"
    return None


def _validate_bucket_acl(acl: str) -> str | None:
    """Validate bucket ACL. Returns error message or None."""
    if acl not in _VALID_ACLS:
        return f"acl must be one of: {', '.join(sorted(_VALID_ACLS))}"
    return None


def create_linode_object_storage_bucket_create_tool() -> Tool:
    """Create the linode_object_storage_bucket_create tool."""
    return Tool(
        name="linode_object_storage_bucket_create",
        description=(
            "Creates a new Object Storage bucket. WARNING: Billing starts immediately."
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
                    "description": (
                        "Bucket label (3-63 chars, lowercase alphanumeric and hyphens)"
                    ),
                },
                "region": {
                    "type": "string",
                    "description": ("Region for the bucket (e.g. us-east-1)"),
                },
                "acl": {
                    "type": "string",
                    "description": (
                        "Access control: private, public-read,"
                        " authenticated-read, or"
                        " public-read-write"
                    ),
                },
                "cors_enabled": {
                    "type": "boolean",
                    "description": ("Whether to enable CORS (default: true)"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["label", "region", "confirm"],
        },
    )


async def handle_linode_object_storage_bucket_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_create tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    label = arguments.get("label", "")
    region = arguments.get("region", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    label_err = _validate_bucket_label(label)
    if label_err:
        return _error_response(label_err)

    validation_err = None
    if not region:
        validation_err = "region is required"
    elif acl is not None:
        validation_err = _validate_bucket_acl(acl)
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        bucket = await client.create_object_storage_bucket(
            label=label,
            region=region,
            acl=acl,
            cors_enabled=cors_enabled,
        )
        return {
            "message": (f"Bucket '{label}' created successfully in {region}"),
            "bucket": bucket,
        }

    return await execute_tool(cfg, arguments, "create bucket", _call)


def create_linode_object_storage_bucket_delete_tool() -> Tool:
    """Create the linode_object_storage_bucket_delete tool."""
    return Tool(
        name="linode_object_storage_bucket_delete",
        description=(
            "Deletes an Object Storage bucket."
            " WARNING: This is irreversible."
            " All objects must be removed first."
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
                    "description": "Region of the bucket",
                },
                "label": {
                    "type": "string",
                    "description": "Label of the bucket",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["region", "label", "confirm"],
        },
    )


async def handle_linode_object_storage_bucket_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_bucket_delete tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This is destructive and irreversible."
                    " All objects must be removed first."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_bucket(region, label)
        return {
            "message": (f"Bucket '{label}' in {region} deleted successfully"),
            "region": region,
            "label": label,
        }

    return await execute_tool(cfg, arguments, "delete bucket", _call)


def create_linode_object_storage_bucket_access_update_tool() -> Tool:
    """Create the linode_object_storage_bucket_access_update tool."""
    return Tool(
        name="linode_object_storage_bucket_access_update",
        description=("Updates access control settings for an Object Storage bucket."),
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
                    "description": "Region of the bucket",
                },
                "label": {
                    "type": "string",
                    "description": "Label of the bucket",
                },
                "acl": {
                    "type": "string",
                    "description": (
                        "New access control: private,"
                        " public-read, authenticated-read,"
                        " or public-read-write"
                    ),
                },
                "cors_enabled": {
                    "type": "boolean",
                    "description": ("Whether to enable CORS on the bucket"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": ("Must be true to confirm access update."),
                },
            },
            "required": ["region", "label", "confirm"],
        },
    )


async def handle_linode_object_storage_bucket_access_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle bucket access update tool request."""
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This changes bucket access controls."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")
    acl = arguments.get("acl")
    cors_enabled = arguments.get("cors_enabled")

    validation_err = None
    if not region:
        validation_err = "region is required"
    elif not label:
        validation_err = "label is required"
    elif acl is not None:
        validation_err = _validate_bucket_acl(acl)
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.update_object_storage_bucket_access(
            region=region,
            label=label,
            acl=acl,
            cors_enabled=cors_enabled,
        )
        response: dict[str, Any] = {
            "message": (
                f"Access settings for bucket '{label}' in {region} updated successfully"
            ),
            "region": region,
            "label": label,
        }
        if acl is not None:
            response["acl"] = acl
        return response

    return await execute_tool(cfg, arguments, "update bucket access settings", _call)


# Stage 5 Phase 4: Object Storage access key write operations

_MAX_KEY_LABEL_LENGTH = 50
_VALID_KEY_PERMISSIONS = {"read_only", "read_write"}


def _validate_key_label(label: str) -> str | None:
    """Validate access key label. Returns error message or None."""
    if not label:
        return "label is required"
    if len(label) > _MAX_KEY_LABEL_LENGTH:
        return "access key label must not exceed 50 characters"
    return None


def _validate_bucket_access_entries(
    entries: list[dict[str, str]],
) -> str | None:
    """Validate bucket_access entries. Returns error message or None."""
    for i, entry in enumerate(entries):
        if not entry.get("bucket_name", "").strip():
            return f"entry {i}: bucket_access entries must include bucket_name"
        if not entry.get("region", "").strip():
            return f"entry {i}: bucket_access entries must include region"
        perms = entry.get("permissions", "")
        if perms not in _VALID_KEY_PERMISSIONS:
            return (
                f"entry {i}: bucket_access permissions must be"
                f" 'read_only' or 'read_write', got '{perms}'"
            )
    return None


def create_linode_object_storage_key_create_tool() -> Tool:
    """Create the linode_object_storage_key_create tool."""
    return Tool(
        name="linode_object_storage_key_create",
        description=(
            "Creates a new Object Storage access key."
            " WARNING: The secret_key is only shown ONCE"
            " in the response and cannot be retrieved later."
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
                    "description": ("Label for the access key (max 50 characters)"),
                },
                "bucket_access": {
                    "type": "string",
                    "description": (
                        "JSON array of bucket permissions:"
                        ' [{"bucket_name": "name", "region":'
                        ' "region", "permissions":'
                        ' "read_only|read_write"}].'
                        " Omit for unrestricted access."
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be set to true. The secret_key is only shown ONCE."
                    ),
                },
            },
            "required": ["label", "confirm"],
        },
    )


def create_linode_object_storage_key_update_tool() -> Tool:
    """Create the linode_object_storage_key_update tool."""
    return Tool(
        name="linode_object_storage_key_update",
        description=(
            "Updates an Object Storage access key's label or bucket permissions."
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
                "key_id": {
                    "type": "number",
                    "description": "ID of the access key to update",
                },
                "label": {
                    "type": "string",
                    "description": ("New label for the access key (max 50 characters)"),
                },
                "bucket_access": {
                    "type": "string",
                    "description": (
                        "JSON array of bucket permissions:"
                        ' [{"bucket_name": "name", "region":'
                        ' "region", "permissions":'
                        ' "read_only|read_write"}]'
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": ("Must be set to true to confirm key update."),
                },
            },
            "required": ["key_id", "confirm"],
        },
    )


def create_linode_object_storage_key_delete_tool() -> Tool:
    """Create the linode_object_storage_key_delete tool."""
    return Tool(
        name="linode_object_storage_key_delete",
        description=("Revokes an Object Storage access key permanently."),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": {
                    "type": "string",
                    "description": (
                        "Linode environment to use (optional, defaults to 'default')"
                    ),
                },
                "key_id": {
                    "type": "number",
                    "description": ("ID of the access key to revoke"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be set to true to confirm"
                        " key revocation. This action is permanent."
                    ),
                },
            },
            "required": ["key_id", "confirm"],
        },
    )


async def handle_linode_object_storage_key_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_create tool."""
    label = arguments.get("label", "")
    bucket_access_json = arguments.get("bucket_access", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates an access key."
                    " The secret_key is only shown ONCE"
                    " in the response."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = _validate_key_label(label)
    bucket_access = None
    if not validation_err and bucket_access_json:
        try:
            bucket_access = json.loads(bucket_access_json)
            validation_err = _validate_bucket_access_entries(bucket_access)
        except (json.JSONDecodeError, TypeError) as e:
            validation_err = (
                f"Invalid bucket_access JSON: {e}."
                " Expected format:"
                ' [{"bucket_name": "name",'
                ' "region": "region",'
                ' "permissions": "read_only"}]'
            )
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.create_object_storage_key(
            label=label,
            bucket_access=bucket_access,
        )
        return {
            "warning": (
                "IMPORTANT: The secret_key below is shown"
                " ONLY ONCE. Save it now - it cannot be"
                " retrieved later."
            ),
            "message": (
                f"Access key '{key.get('label', label)}'"
                " created successfully"
                f" (ID: {key.get('id', 'unknown')})"
            ),
            "key": key,
        }

    return await execute_tool(cfg, arguments, "create access key", _call)


async def handle_linode_object_storage_key_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_update tool."""
    key_id = arguments.get("key_id", 0)
    label = arguments.get("label", "")
    bucket_access_json = arguments.get("bucket_access", "")
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This modifies access key permissions."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    validation_err = None
    if not key_id or int(key_id) <= 0:
        validation_err = "key_id is required and must be a positive integer"
    elif label:
        validation_err = _validate_key_label(label)

    key_id = int(key_id) if not validation_err else 0
    bucket_access = None
    if not validation_err and bucket_access_json:
        try:
            bucket_access = json.loads(bucket_access_json)
            validation_err = _validate_bucket_access_entries(bucket_access)
        except (json.JSONDecodeError, TypeError) as e:
            validation_err = (
                f"Invalid bucket_access JSON: {e}."
                " Expected format:"
                ' [{"bucket_name": "name",'
                ' "region": "region",'
                ' "permissions": "read_only"}]'
            )
    if validation_err:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.update_object_storage_key(
            key_id=key_id,
            label=label or None,
            bucket_access=bucket_access,
        )
        return {
            "message": (f"Access key {key_id} updated successfully"),
            "key_id": key_id,
        }

    return await execute_tool(cfg, arguments, f"update access key {key_id}", _call)


async def handle_linode_object_storage_key_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle the linode_object_storage_key_delete tool."""
    key_id = arguments.get("key_id", 0)
    confirm = arguments.get("confirm", False)

    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This revokes the access key"
                    " permanently."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    if not key_id or int(key_id) <= 0:
        return _error_response("key_id is required and must be a positive integer")

    key_id = int(key_id)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_object_storage_key(key_id=key_id)
        return {
            "message": (f"Access key {key_id} revoked successfully"),
            "key_id": key_id,
        }

    return await execute_tool(cfg, arguments, f"revoke access key {key_id}", _call)


# Stage 5 Phase 5: Presigned URLs, Object ACL, and SSL

_VALID_PRESIGNED_METHODS = {"GET", "PUT"}
_MIN_EXPIRES_IN = 1
_MAX_EXPIRES_IN = 604800
_DEFAULT_EXPIRES_IN = 3600


def _validate_presigned_method(method: str) -> str | None:
    """Validate presigned URL method. Returns error message or None."""
    if method.upper() not in _VALID_PRESIGNED_METHODS:
        return f"method must be 'GET' or 'PUT', got '{method}'"
    return None


def _validate_expires_in(expires_in: int) -> str | None:
    """Validate expires_in value. Returns error message or None."""
    if expires_in < _MIN_EXPIRES_IN or expires_in > _MAX_EXPIRES_IN:
        return (
            f"expires_in must be between {_MIN_EXPIRES_IN} and"
            f" {_MAX_EXPIRES_IN} seconds (7 days),"
            f" got {expires_in}"
        )
    return None


def create_linode_object_storage_presigned_url_tool() -> Tool:
    """Create the linode_object_storage_presigned_url tool."""
    return Tool(
        name="linode_object_storage_presigned_url",
        description=(
            "Generates a presigned URL for accessing an object"
            " in Object Storage. Use method=GET to create a"
            " download URL, method=PUT to create an upload URL."
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
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "name": {
                    "type": "string",
                    "description": ("The object key (path/filename within the bucket)"),
                },
                "method": {
                    "type": "string",
                    "description": (
                        "HTTP method: 'GET' for download URL, 'PUT' for upload URL"
                    ),
                },
                "expires_in": {
                    "type": "number",
                    "description": (
                        "URL expiration in seconds (1-604800, default 3600 = 1 hour)"
                    ),
                },
            },
            "required": ["region", "label", "name", "method"],
        },
    )


async def handle_linode_object_storage_presigned_url(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_presigned_url tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")
    method = arguments.get("method", "")
    expires_in = int(arguments.get("expires_in", _DEFAULT_EXPIRES_IN))

    missing = (
        "region is required"
        if not region
        else "label is required"
        if not label
        else "name (object key) is required"
        if not name
        else None
    )
    if missing is not None:
        return _error_response(missing)

    validation_err = _validate_presigned_method(method)
    if validation_err is None:
        validation_err = _validate_expires_in(expires_in)
    if validation_err is not None:
        return _error_response(validation_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_presigned_url(
            region, label, name, method.upper(), expires_in
        )

    return await execute_tool(cfg, arguments, "generate presigned URL", _call)


def create_linode_object_storage_object_acl_get_tool() -> Tool:
    """Create the linode_object_storage_object_acl_get tool."""
    return Tool(
        name="linode_object_storage_object_acl_get",
        description=(
            "Gets the Access Control List (ACL) for a specific"
            " object in an Object Storage bucket"
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
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "name": {
                    "type": "string",
                    "description": ("The object key (path/filename within the bucket)"),
                },
            },
            "required": ["region", "label", "name"],
        },
    )


async def handle_linode_object_storage_object_acl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_object_acl_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")
    if not name:
        return _error_response("name (object key) is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_object_acl(region, label, name)

    return await execute_tool(cfg, arguments, "retrieve object ACL", _call)


def create_linode_object_storage_object_acl_update_tool() -> Tool:
    """Create the linode_object_storage_object_acl_update tool."""
    return Tool(
        name="linode_object_storage_object_acl_update",
        description=(
            "Updates the Access Control List (ACL) for a specific"
            " object in an Object Storage bucket."
            " Requires confirm=true to proceed."
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
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "name": {
                    "type": "string",
                    "description": ("The object key (path/filename within the bucket)"),
                },
                "acl": {
                    "type": "string",
                    "description": (
                        "ACL to set: private, public-read,"
                        " authenticated-read,"
                        " or public-read-write"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to proceed."
                        " This modifies the object's"
                        " access permissions."
                    ),
                },
            },
            "required": [
                "region",
                "label",
                "name",
                "acl",
                "confirm",
            ],
        },
    )


async def handle_linode_object_storage_object_acl_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_object_acl_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="This modifies the object's access permissions."
                " Set confirm=true to proceed.",
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")
    name = arguments.get("name", "")
    acl = arguments.get("acl", "")

    missing = (
        "region is required"
        if not region
        else "label is required"
        if not label
        else "name (object key) is required"
        if not name
        else None
    )
    if missing is not None:
        return _error_response(missing)

    acl_err = _validate_bucket_acl(acl)
    if acl_err is not None:
        return _error_response(acl_err)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_object_acl(region, label, name, acl)

    return await execute_tool(cfg, arguments, "update object ACL", _call)


def create_linode_object_storage_ssl_get_tool() -> Tool:
    """Create the linode_object_storage_ssl_get tool."""
    return Tool(
        name="linode_object_storage_ssl_get",
        description=(
            "Checks whether an Object Storage bucket has an"
            " SSL/TLS certificate installed"
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
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
            },
            "required": ["region", "label"],
        },
    )


async def handle_linode_object_storage_ssl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_ssl_get tool request."""
    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_bucket_ssl(region, label)

    return await execute_tool(cfg, arguments, "retrieve SSL status", _call)


def create_linode_object_storage_ssl_delete_tool() -> Tool:
    """Create the linode_object_storage_ssl_delete tool."""
    return Tool(
        name="linode_object_storage_ssl_delete",
        description=(
            "Deletes the SSL/TLS certificate from an Object"
            " Storage bucket."
            " Requires confirm=true to proceed."
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
                    "description": ("Region where the bucket is located"),
                },
                "label": {
                    "type": "string",
                    "description": "The bucket label (name)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to proceed."
                        " This removes the SSL certificate"
                        " from the bucket."
                    ),
                },
            },
            "required": ["region", "label", "confirm"],
        },
    )


async def handle_linode_object_storage_ssl_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_object_storage_ssl_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="This removes the SSL certificate"
                " from the bucket."
                " Set confirm=true to proceed.",
            )
        ]

    region = arguments.get("region", "")
    label = arguments.get("label", "")

    if not region:
        return _error_response("region is required")
    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_bucket_ssl(region, label)
        return {
            "message": (
                f"SSL certificate deleted from bucket '{label}' in region '{region}'"
            ),
            "region": region,
            "bucket": label,
        }

    return await execute_tool(cfg, arguments, "delete SSL certificate", _call)


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
    label = arguments.get("label", "")
    ssh_key = arguments.get("ssh_key", "")

    if not label:
        return _error_response("label is required")
    if not ssh_key:
        return _error_response("ssh_key is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        key = await client.create_ssh_key(label, ssh_key)
        return {
            "message": f"SSH key '{key.label}' (ID: {key.id}) created successfully",
            "ssh_key": {
                "id": key.id,
                "label": key.label,
                "created": key.created,
            },
        }

    return await execute_tool(cfg, arguments, "create SSH key", _call)


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
    ssh_key_id = arguments.get("ssh_key_id", 0)

    if not ssh_key_id:
        return _error_response("ssh_key_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_ssh_key(int(ssh_key_id))
        return {
            "message": f"SSH key {ssh_key_id} deleted successfully",
            "ssh_key_id": ssh_key_id,
        }

    return await execute_tool(cfg, arguments, "delete SSH key", _call)


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
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.boot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} boot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "boot instance", _call)


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
    instance_id = arguments.get("instance_id", 0)
    config_id = arguments.get("config_id")

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.reboot_instance(int(instance_id), config_id)
        return {
            "message": f"Instance {instance_id} reboot initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "reboot instance", _call)


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
    instance_id = arguments.get("instance_id", 0)

    if not instance_id:
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.shutdown_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} shutdown initiated successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "shutdown instance", _call)


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
        return _error_response("region is required")
    if not instance_type:
        return _error_response("type is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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
        return {
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

    return await execute_tool(cfg, arguments, "create instance", _call)


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
        return _error_response("instance_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_instance(int(instance_id))
        return {
            "message": f"Instance {instance_id} deleted successfully",
            "instance_id": instance_id,
        }

    return await execute_tool(cfg, arguments, "delete instance", _call)


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
        return _error_response("instance_id is required")
    if not instance_type:
        return _error_response("type is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.resize_instance(
            instance_id=int(instance_id),
            instance_type=instance_type,
            allow_auto_disk_resize=arguments.get("allow_auto_disk_resize", True),
            migration_type=arguments.get("migration_type", "warm"),
        )
        return {
            "message": (f"Instance {instance_id} resize to {instance_type} initiated"),
            "instance_id": instance_id,
            "new_type": instance_type,
        }

    return await execute_tool(cfg, arguments, "resize instance", _call)


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
    label = arguments.get("label", "")
    inbound_policy = arguments.get("inbound_policy", "ACCEPT")
    outbound_policy = arguments.get("outbound_policy", "ACCEPT")

    if not label:
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.create_firewall(
            label=label,
            inbound_policy=inbound_policy,
            outbound_policy=outbound_policy,
        )
        return {
            "message": (
                f"Firewall '{firewall.label}' (ID: {firewall.id}) created successfully"
            ),
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "created": firewall.created,
            },
        }

    return await execute_tool(cfg, arguments, "create firewall", _call)


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
    firewall_id = arguments.get("firewall_id", 0)

    if not firewall_id:
        return _error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        firewall = await client.update_firewall(
            firewall_id=int(firewall_id),
            label=arguments.get("label"),
            status=arguments.get("status"),
            inbound_policy=arguments.get("inbound_policy"),
            outbound_policy=arguments.get("outbound_policy"),
        )
        return {
            "message": f"Firewall {firewall_id} updated successfully",
            "firewall": {
                "id": firewall.id,
                "label": firewall.label,
                "status": firewall.status,
                "updated": firewall.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update firewall", _call)


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
        return _error_response("firewall_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_firewall(int(firewall_id))
        return {
            "message": f"Firewall {firewall_id} deleted successfully",
            "firewall_id": firewall_id,
        }

    return await execute_tool(cfg, arguments, "delete firewall", _call)


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
    domain_name = arguments.get("domain", "")

    if not domain_name:
        return _error_response("domain is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.create_domain(
            domain=domain_name,
            domain_type=arguments.get("type", "master"),
            soa_email=arguments.get("soa_email"),
            description=arguments.get("description"),
        )
        return {
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

    return await execute_tool(cfg, arguments, "create domain", _call)


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
    domain_id = arguments.get("domain_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.update_domain(
            domain_id=int(domain_id),
            domain=arguments.get("domain"),
            soa_email=arguments.get("soa_email"),
            description=arguments.get("description"),
        )
        return {
            "message": f"Domain {domain_id} updated successfully",
            "domain": {
                "id": domain.id,
                "domain": domain.domain,
                "type": domain.type,
                "status": domain.status,
                "updated": domain.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update domain", _call)


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
        return _error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain(int(domain_id))
        return {
            "message": f"Domain {domain_id} deleted successfully",
            "domain_id": domain_id,
        }

    return await execute_tool(cfg, arguments, "delete domain", _call)


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
    domain_id = arguments.get("domain_id", 0)
    record_type = arguments.get("type", "")

    if not domain_id:
        return _error_response("domain_id is required")
    if not record_type:
        return _error_response("type is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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
        return {
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

    return await execute_tool(cfg, arguments, "create DNS record", _call)


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
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")
    if not record_id:
        return _error_response("record_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
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
        return {
            "message": f"DNS record {record_id} updated successfully",
            "record": {
                "id": record.id,
                "type": record.type,
                "name": record.name,
                "target": record.target,
                "ttl_sec": record.ttl_sec,
            },
        }

    return await execute_tool(cfg, arguments, "update DNS record", _call)


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
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if not domain_id:
        return _error_response("domain_id is required")
    if not record_id:
        return _error_response("record_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain_record(int(domain_id), int(record_id))
        return {
            "message": f"DNS record {record_id} deleted successfully",
            "domain_id": domain_id,
            "record_id": record_id,
        }

    return await execute_tool(cfg, arguments, "delete DNS record", _call)


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
        return _error_response("label is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.create_volume(
            label=label,
            region=arguments.get("region"),
            linode_id=arguments.get("linode_id"),
            size=arguments.get("size", 20),
        )
        return {
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

    return await execute_tool(cfg, arguments, "create volume", _call)


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
    volume_id = arguments.get("volume_id", 0)
    linode_id = arguments.get("linode_id", 0)

    if not volume_id:
        return _error_response("volume_id is required")
    if not linode_id:
        return _error_response("linode_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.attach_volume(
            volume_id=int(volume_id),
            linode_id=int(linode_id),
            config_id=arguments.get("config_id"),
            persist_across_boots=arguments.get("persist_across_boots", False),
        )
        return {
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

    return await execute_tool(cfg, arguments, "attach volume", _call)


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
    volume_id = arguments.get("volume_id", 0)

    if not volume_id:
        return _error_response("volume_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.detach_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} detached successfully",
            "volume_id": volume_id,
        }

    return await execute_tool(cfg, arguments, "detach volume", _call)


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
        return _error_response("volume_id is required")
    if not size:
        return _error_response("size is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        volume = await client.resize_volume(int(volume_id), int(size))
        return {
            "message": f"Volume {volume_id} resized to {size}GB successfully",
            "volume": {
                "id": volume.id,
                "label": volume.label,
                "size": volume.size,
            },
        }

    return await execute_tool(cfg, arguments, "resize volume", _call)


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
        return _error_response("volume_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_volume(int(volume_id))
        return {
            "message": f"Volume {volume_id} deleted successfully",
            "volume_id": volume_id,
        }

    return await execute_tool(cfg, arguments, "delete volume", _call)


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
        return _error_response("region is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.create_nodebalancer(
            region=region,
            label=arguments.get("label"),
            client_conn_throttle=arguments.get("client_conn_throttle", 0),
        )
        return {
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

    return await execute_tool(cfg, arguments, "create NodeBalancer", _call)


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
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return _error_response("nodebalancer_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        nb = await client.update_nodebalancer(
            nodebalancer_id=int(nodebalancer_id),
            label=arguments.get("label"),
            client_conn_throttle=arguments.get("client_conn_throttle"),
        )
        return {
            "message": f"NodeBalancer {nodebalancer_id} updated successfully",
            "nodebalancer": {
                "id": nb.id,
                "label": nb.label,
                "client_conn_throttle": nb.client_conn_throttle,
                "updated": nb.updated,
            },
        }

    return await execute_tool(cfg, arguments, "update NodeBalancer", _call)


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
        return _error_response("nodebalancer_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer(int(nodebalancer_id))
        return {
            "message": f"NodeBalancer {nodebalancer_id} deleted successfully",
            "nodebalancer_id": nodebalancer_id,
        }

    return await execute_tool(cfg, arguments, "delete NodeBalancer", _call)


# LKE (Linode Kubernetes Engine) tools

_ENV_PROP: dict[str, Any] = {
    "type": "string",
    "description": "Linode environment to use (optional, defaults to 'default')",
}

_CLUSTER_ID_PROP: dict[str, Any] = {
    "type": "string",
    "description": "The ID of the LKE cluster (required)",
}

_CONFIRM_PROP: dict[str, Any] = {
    "type": "boolean",
    "description": "Must be true to confirm this operation.",
}


def create_linode_lke_clusters_list_tool() -> Tool:
    """Create the linode_lke_clusters_list tool."""
    return Tool(
        name="linode_lke_clusters_list",
        description="Lists all LKE (Kubernetes) clusters on the account",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_lke_clusters_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_clusters_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        clusters = await client.list_lke_clusters()
        return {"count": len(clusters), "clusters": clusters}

    return await execute_tool(cfg, arguments, "list LKE clusters", _call)


def create_linode_lke_cluster_get_tool() -> Tool:
    """Create the linode_lke_cluster_get tool."""
    return Tool(
        name="linode_lke_cluster_get",
        description="Gets details of a specific LKE cluster by ID",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    )


async def handle_linode_lke_cluster_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_cluster(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE cluster", _call)


def create_linode_lke_cluster_create_tool() -> Tool:
    """Create the linode_lke_cluster_create tool."""
    return Tool(
        name="linode_lke_cluster_create",
        description="Creates a new LKE (Kubernetes) cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "label": {
                    "type": "string",
                    "description": "Label for the cluster (required)",
                },
                "region": {
                    "type": "string",
                    "description": "Region for the cluster (required)",
                },
                "k8s_version": {
                    "type": "string",
                    "description": "Kubernetes version (required)",
                },
                "node_pools": {
                    "type": "array",
                    "description": (
                        "Node pools: [{type, count, autoscaler?, tags?}] (required)"
                    ),
                    "items": {"type": "object"},
                },
                "tags": {
                    "type": "array",
                    "description": "Tags for the cluster",
                    "items": {"type": "string"},
                },
                "control_plane": {
                    "type": "object",
                    "description": ("Control plane config: {high_availability: bool}"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["label", "region", "k8s_version", "node_pools", "confirm"],
        },
    )


async def handle_linode_lke_cluster_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    label = arguments.get("label", "")
    region = arguments.get("region", "")
    k8s_version = arguments.get("k8s_version", "")
    node_pools = arguments.get("node_pools", [])
    tags = arguments.get("tags")
    control_plane = arguments.get("control_plane")

    if not label:
        return _error_response("label is required")
    if not region:
        return _error_response("region is required")
    if not k8s_version:
        return _error_response("k8s_version is required")
    if not node_pools:
        return _error_response("node_pools is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_lke_cluster(
            label=label,
            region=region,
            k8s_version=k8s_version,
            node_pools=node_pools,
            tags=tags,
            control_plane=control_plane,
        )

    return await execute_tool(cfg, arguments, "create LKE cluster", _call)


def create_linode_lke_cluster_update_tool() -> Tool:
    """Create the linode_lke_cluster_update tool."""
    return Tool(
        name="linode_lke_cluster_update",
        description="Updates an existing LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "label": {
                    "type": "string",
                    "description": "New label for the cluster",
                },
                "k8s_version": {
                    "type": "string",
                    "description": "New Kubernetes version",
                },
                "tags": {
                    "type": "array",
                    "description": "New tags for the cluster",
                    "items": {"type": "string"},
                },
                "control_plane": {
                    "type": "object",
                    "description": ("Control plane config: {high_availability: bool}"),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_cluster(
            cluster_id=cluster_id,
            label=arguments.get("label"),
            k8s_version=arguments.get("k8s_version"),
            tags=arguments.get("tags"),
            control_plane=arguments.get("control_plane"),
        )

    return await execute_tool(cfg, arguments, "update LKE cluster", _call)


def create_linode_lke_cluster_delete_tool() -> Tool:
    """Create the linode_lke_cluster_delete tool."""
    return Tool(
        name="linode_lke_cluster_delete",
        description="Deletes an LKE cluster and all associated resources",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} deleted successfully",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE cluster", _call)


def create_linode_lke_cluster_recycle_tool() -> Tool:
    """Create the linode_lke_cluster_recycle tool."""
    return Tool(
        name="linode_lke_cluster_recycle",
        description="Recycles all nodes in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_recycle tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_cluster(cluster_id)
        return {
            "message": f"LKE cluster {cluster_id} nodes recycled successfully",
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE cluster", _call)


def create_linode_lke_cluster_regenerate_tool() -> Tool:
    """Create the linode_lke_cluster_regenerate tool."""
    return Tool(
        name="linode_lke_cluster_regenerate",
        description=("Regenerates the service token for an LKE cluster"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_cluster_regenerate(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_cluster_regenerate tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.regenerate_lke_cluster(cluster_id)
        return {
            "message": (f"LKE cluster {cluster_id} service token regenerated"),
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "regenerate LKE cluster", _call)


def create_linode_lke_pools_list_tool() -> Tool:
    """Create the linode_lke_pools_list tool."""
    return Tool(
        name="linode_lke_pools_list",
        description="Lists node pools for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    )


async def handle_linode_lke_pools_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pools_list tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        pools = await client.list_lke_node_pools(cluster_id)
        return {"count": len(pools), "pools": pools}

    return await execute_tool(cfg, arguments, "list LKE node pools", _call)


def create_linode_lke_pool_get_tool() -> Tool:
    """Create the linode_lke_pool_get tool."""
    return Tool(
        name="linode_lke_pool_get",
        description="Gets details of a specific node pool in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
            },
            "required": ["cluster_id", "pool_id"],
        },
    )


async def handle_linode_lke_pool_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_node_pool(cluster_id, pool_id)

    return await execute_tool(cfg, arguments, "get LKE node pool", _call)


def create_linode_lke_pool_create_tool() -> Tool:
    """Create the linode_lke_pool_create tool."""
    return Tool(
        name="linode_lke_pool_create",
        description="Creates a new node pool in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "type": {
                    "type": "string",
                    "description": "Linode type for pool nodes (required)",
                },
                "count": {
                    "type": "integer",
                    "description": "Number of nodes in the pool (required)",
                },
                "autoscaler": {
                    "type": "object",
                    "description": ("Autoscaler config: {enabled, min, max}"),
                },
                "tags": {
                    "type": "array",
                    "description": "Tags for the node pool",
                    "items": {"type": "string"},
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm creation. This incurs billing."
                    ),
                },
            },
            "required": ["cluster_id", "type", "count", "confirm"],
        },
    )


async def handle_linode_lke_pool_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_create tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text=(
                    "Error: This creates a billable resource."
                    " Set confirm=true to proceed."
                ),
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    node_type = arguments.get("type", "")
    if not node_type:
        return _error_response("type is required")

    count = arguments.get("count", 0)
    if not count:
        return _error_response("count is required and must be > 0")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_lke_node_pool(
            cluster_id=cluster_id,
            node_type=node_type,
            count=int(count),
            autoscaler=arguments.get("autoscaler"),
            tags=arguments.get("tags"),
        )

    return await execute_tool(cfg, arguments, "create LKE node pool", _call)


def create_linode_lke_pool_update_tool() -> Tool:
    """Create the linode_lke_pool_update tool."""
    return Tool(
        name="linode_lke_pool_update",
        description="Updates a node pool in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
                "count": {
                    "type": "integer",
                    "description": "New number of nodes in the pool",
                },
                "autoscaler": {
                    "type": "object",
                    "description": ("Autoscaler config: {enabled, min, max}"),
                },
                "tags": {
                    "type": "array",
                    "description": "New tags for the node pool",
                    "items": {"type": "string"},
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    )


async def handle_linode_lke_pool_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_node_pool(
            cluster_id=cluster_id,
            pool_id=pool_id,
            count=arguments.get("count"),
            autoscaler=arguments.get("autoscaler"),
            tags=arguments.get("tags"),
        )

    return await execute_tool(cfg, arguments, "update LKE node pool", _call)


def create_linode_lke_pool_delete_tool() -> Tool:
    """Create the linode_lke_pool_delete tool."""
    return Tool(
        name="linode_lke_pool_delete",
        description="Deletes a node pool from an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    )


async def handle_linode_lke_pool_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_node_pool(cluster_id, pool_id)
        return {
            "message": (f"Node pool {pool_id} deleted from cluster {cluster_id}"),
            "cluster_id": cluster_id,
            "pool_id": pool_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE node pool", _call)


def create_linode_lke_pool_recycle_tool() -> Tool:
    """Create the linode_lke_pool_recycle tool."""
    return Tool(
        name="linode_lke_pool_recycle",
        description="Recycles all nodes in a node pool",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "pool_id": {
                    "type": "string",
                    "description": "The ID of the node pool (required)",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "pool_id", "confirm"],
        },
    )


async def handle_linode_lke_pool_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_pool_recycle tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    pool_id_str = arguments.get("pool_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not pool_id_str:
        return _error_response("pool_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")
    try:
        pool_id = int(pool_id_str)
    except ValueError:
        return _error_response("pool_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_node_pool(cluster_id, pool_id)
        return {
            "message": (f"Node pool {pool_id} in cluster {cluster_id} recycled"),
            "cluster_id": cluster_id,
            "pool_id": pool_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE node pool", _call)


def create_linode_lke_node_get_tool() -> Tool:
    """Create the linode_lke_node_get tool."""
    return Tool(
        name="linode_lke_node_get",
        description="Gets details of a specific node in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "node_id": {
                    "type": "string",
                    "description": "The ID of the node (required, string)",
                },
            },
            "required": ["cluster_id", "node_id"],
        },
    )


async def handle_linode_lke_node_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    node_id = arguments.get("node_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not node_id:
        return _error_response("node_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_node(cluster_id, str(node_id))

    return await execute_tool(cfg, arguments, "get LKE node", _call)


def create_linode_lke_node_delete_tool() -> Tool:
    """Create the linode_lke_node_delete tool."""
    return Tool(
        name="linode_lke_node_delete",
        description="Deletes a specific node from an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "node_id": {
                    "type": "string",
                    "description": "The ID of the node (required, string)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "node_id", "confirm"],
        },
    )


async def handle_linode_lke_node_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    node_id = arguments.get("node_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not node_id:
        return _error_response("node_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_node(cluster_id, str(node_id))
        return {
            "message": (f"Node {node_id} deleted from cluster {cluster_id}"),
            "cluster_id": cluster_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE node", _call)


def create_linode_lke_node_recycle_tool() -> Tool:
    """Create the linode_lke_node_recycle tool."""
    return Tool(
        name="linode_lke_node_recycle",
        description="Recycles a specific node in an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "node_id": {
                    "type": "string",
                    "description": "The ID of the node (required, string)",
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "node_id", "confirm"],
        },
    )


async def handle_linode_lke_node_recycle(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_node_recycle tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    node_id = arguments.get("node_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    if not node_id:
        return _error_response("node_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.recycle_lke_node(cluster_id, str(node_id))
        return {
            "message": (f"Node {node_id} in cluster {cluster_id} recycled"),
            "cluster_id": cluster_id,
            "node_id": node_id,
        }

    return await execute_tool(cfg, arguments, "recycle LKE node", _call)


def create_linode_lke_kubeconfig_get_tool() -> Tool:
    """Create the linode_lke_kubeconfig_get tool."""
    return Tool(
        name="linode_lke_kubeconfig_get",
        description="Gets the kubeconfig for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    )


async def handle_linode_lke_kubeconfig_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_kubeconfig_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_kubeconfig(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE kubeconfig", _call)


def create_linode_lke_kubeconfig_delete_tool() -> Tool:
    """Create the linode_lke_kubeconfig_delete tool."""
    return Tool(
        name="linode_lke_kubeconfig_delete",
        description=("Deletes and regenerates the kubeconfig for an LKE cluster"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_kubeconfig_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_kubeconfig_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_kubeconfig(cluster_id)
        return {
            "message": (f"Kubeconfig for cluster {cluster_id} regenerated"),
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE kubeconfig", _call)


def create_linode_lke_dashboard_get_tool() -> Tool:
    """Create the linode_lke_dashboard_get tool."""
    return Tool(
        name="linode_lke_dashboard_get",
        description="Gets the dashboard URL for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    )


async def handle_linode_lke_dashboard_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_dashboard_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_dashboard(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE dashboard", _call)


def create_linode_lke_api_endpoints_list_tool() -> Tool:
    """Create the linode_lke_api_endpoints_list tool."""
    return Tool(
        name="linode_lke_api_endpoints_list",
        description="Lists API endpoints for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    )


async def handle_linode_lke_api_endpoints_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_api_endpoints_list tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        endpoints = await client.list_lke_api_endpoints(cluster_id)
        return {"count": len(endpoints), "endpoints": endpoints}

    return await execute_tool(cfg, arguments, "list LKE API endpoints", _call)


def create_linode_lke_service_token_delete_tool() -> Tool:
    """Create the linode_lke_service_token_delete tool."""
    return Tool(
        name="linode_lke_service_token_delete",
        description="Deletes the service token for an LKE cluster",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_service_token_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_service_token_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_service_token(cluster_id)
        return {
            "message": (f"Service token for cluster {cluster_id} deleted"),
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE service token", _call)


def create_linode_lke_acl_get_tool() -> Tool:
    """Create the linode_lke_acl_get tool."""
    return Tool(
        name="linode_lke_acl_get",
        description=("Gets the control plane ACL configuration for an LKE cluster"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
            },
            "required": ["cluster_id"],
        },
    )


async def handle_linode_lke_acl_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_get tool request."""
    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_control_plane_acl(cluster_id)

    return await execute_tool(cfg, arguments, "get LKE control plane ACL", _call)


def create_linode_lke_acl_update_tool() -> Tool:
    """Create the linode_lke_acl_update tool."""
    return Tool(
        name="linode_lke_acl_update",
        description=("Updates the control plane ACL for an LKE cluster"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "acl": {
                    "type": "object",
                    "description": (
                        "ACL config: {enabled: bool,"
                        " addresses: {ipv4: [...], ipv6: [...]}}"
                    ),
                },
                "confirm": _CONFIRM_PROP,
            },
            "required": ["cluster_id", "acl", "confirm"],
        },
    )


async def handle_linode_lke_acl_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_update tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return _error_response("Set confirm=true to proceed.")

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    acl = arguments.get("acl")
    if not acl:
        return _error_response("acl is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.update_lke_control_plane_acl(cluster_id, acl)

    return await execute_tool(cfg, arguments, "update LKE control plane ACL", _call)


def create_linode_lke_acl_delete_tool() -> Tool:
    """Create the linode_lke_acl_delete tool."""
    return Tool(
        name="linode_lke_acl_delete",
        description=("Deletes the control plane ACL for an LKE cluster"),
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "cluster_id": _CLUSTER_ID_PROP,
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. This is irreversible."
                    ),
                },
            },
            "required": ["cluster_id", "confirm"],
        },
    )


async def handle_linode_lke_acl_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_acl_delete tool request."""
    confirm = arguments.get("confirm", False)
    if not confirm:
        return [
            TextContent(
                type="text",
                text="Error: This is destructive. Set confirm=true to proceed.",
            )
        ]

    cluster_id_str = arguments.get("cluster_id", "")
    if not cluster_id_str:
        return _error_response("cluster_id is required")
    try:
        cluster_id = int(cluster_id_str)
    except ValueError:
        return _error_response("cluster_id must be a valid integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_lke_control_plane_acl(cluster_id)
        return {
            "message": (f"Control plane ACL for cluster {cluster_id} deleted"),
            "cluster_id": cluster_id,
        }

    return await execute_tool(cfg, arguments, "delete LKE control plane ACL", _call)


def create_linode_lke_versions_list_tool() -> Tool:
    """Create the linode_lke_versions_list tool."""
    return Tool(
        name="linode_lke_versions_list",
        description="Lists available Kubernetes versions for LKE",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_lke_versions_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_versions_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        versions = await client.list_lke_versions()
        return {"count": len(versions), "versions": versions}

    return await execute_tool(cfg, arguments, "list LKE versions", _call)


def create_linode_lke_version_get_tool() -> Tool:
    """Create the linode_lke_version_get tool."""
    return Tool(
        name="linode_lke_version_get",
        description="Gets details of a specific LKE Kubernetes version",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
                "version_id": {
                    "type": "string",
                    "description": ("The version ID (e.g. '1.29') (required)"),
                },
            },
            "required": ["version_id"],
        },
    )


async def handle_linode_lke_version_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_version_get tool request."""
    version_id = arguments.get("version_id", "")
    if not version_id:
        return _error_response("version_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_lke_version(str(version_id))

    return await execute_tool(cfg, arguments, "get LKE version", _call)


def create_linode_lke_types_list_tool() -> Tool:
    """Create the linode_lke_types_list tool."""
    return Tool(
        name="linode_lke_types_list",
        description="Lists available node types for LKE clusters",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_lke_types_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_types_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        types = await client.list_lke_types()
        return {"count": len(types), "types": types}

    return await execute_tool(cfg, arguments, "list LKE types", _call)


def create_linode_lke_tier_versions_list_tool() -> Tool:
    """Create the linode_lke_tier_versions_list tool."""
    return Tool(
        name="linode_lke_tier_versions_list",
        description="Lists LKE tier versions",
        inputSchema={
            "type": "object",
            "properties": {
                "environment": _ENV_PROP,
            },
        },
    )


async def handle_linode_lke_tier_versions_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_lke_tier_versions_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        versions = await client.list_lke_tier_versions()
        return {"count": len(versions), "tier_versions": versions}

    return await execute_tool(cfg, arguments, "list LKE tier versions", _call)
