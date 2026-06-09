from __future__ import annotations

from typing import TYPE_CHECKING, Any
from urllib.parse import quote

import httpx
from mcp.types import TextContent, Tool

from linodemcp.linode import APIError, NetworkError, validate_label
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    MODE_PROP,
    PARAM_DRY_RUN,
    PARAM_MODE,
    PARAM_PLAN_ID,
    PLAN_ID_PROP,
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _required_string_argument(arguments: dict[str, Any], name: str) -> str | None:
    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip() or value != value.strip():
        return None
    return value


def create_linode_domain_import_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_import tool."""
    return Tool(
        name="linode_domain_import",
        description=(
            "Imports a DNS domain from a remote nameserver. Pass dry_run=true "
            "with confirm=true to preview without importing."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain": {
                    "type": "string",
                    "description": "The domain name to import (required)",
                },
                "remote_nameserver": {
                    "type": "string",
                    "description": "The remote nameserver to import from (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm this operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain", "remote_nameserver", "confirm"],
        },
    ), Capability.Write


async def handle_linode_domain_import(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_import tool request."""
    domain_name = _required_string_argument(arguments, "domain")
    remote_nameserver = _required_string_argument(arguments, "remote_nameserver")

    if domain_name is None:
        return error_response("domain is required")
    if remote_nameserver is None:
        return error_response("remote_nameserver is required")
    try:
        validate_label(domain_name)
    except ValueError as exc:
        return error_response(str(exc))
    request_body = {
        "domain": domain_name,
        "remote_nameserver": remote_nameserver,
    }

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_domain_import",
            arguments.get("environment", ""),
            "POST",
            "/domains/import",
            None,
            request_body=request_body,
            side_effects=[
                f"DNS domain {domain_name!r} will be imported from "
                f"{remote_nameserver!r}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("This imports a DNS domain. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.import_domain(
            domain=domain_name,
            remote_nameserver=remote_nameserver,
        )
        return {
            "message": (
                f"Domain '{domain.domain}' (ID: {domain.id}) imported successfully"
            ),
            "domain": {
                "id": domain.id,
                "domain": domain.domain,
                "type": domain.type,
                "status": domain.status,
                "created": domain.created,
            },
        }

    return await execute_tool(cfg, arguments, "import domain", _call)


def _domain_id_argument(arguments: dict[str, Any]) -> int | None:
    value = arguments.get("domain_id")
    if type(value) is not int or value <= 0:
        return None
    return value


def create_linode_domain_clone_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_clone tool."""
    return Tool(
        name="linode_domain_clone",
        description=(
            "Clones an existing DNS domain. Pass dry_run=true with confirm=true "
            "to preview without cloning."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the source domain to clone (required)",
                },
                "domain": {
                    "type": "string",
                    "description": (
                        "The new domain name for the cloned domain (required)"
                    ),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Must be true to confirm this operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain_id", "domain", "confirm"],
        },
    ), Capability.Write


async def handle_linode_domain_clone(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_clone tool request."""
    domain_id = _domain_id_argument(arguments)
    domain_name = _required_string_argument(arguments, "domain")

    if domain_id is None:
        return error_response("domain_id must be a positive integer")
    if domain_name is None:
        return error_response("domain is required")
    try:
        validate_label(domain_name)
    except ValueError as exc:
        return error_response(str(exc))
    request_body = {"domain": domain_name}
    encoded_domain_id = quote(str(domain_id), safe="")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_domain_clone",
            arguments.get("environment", ""),
            "POST",
            f"/domains/{encoded_domain_id}/clone",
            None,
            request_body=request_body,
            side_effects=[
                f"DNS domain ID {domain_id} will be cloned to {domain_name!r}."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response("This clones a DNS domain. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        domain = await client.clone_domain(domain_id=domain_id, domain=domain_name)
        return {
            "message": (
                f"Domain '{domain.domain}' (ID: {domain.id}) cloned successfully"
            ),
            "domain": {
                "id": domain.id,
                "domain": domain.domain,
                "type": domain.type,
                "status": domain.status,
                "created": domain.created,
            },
        }

    return await execute_tool(cfg, arguments, "clone domain", _call)


def create_linode_domain_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_create tool."""
    return Tool(
        name="linode_domain_create",
        description=(
            "Creates a new DNS domain. Pass dry_run=true to preview without creating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm this operation."
                        " Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain", "confirm"],
        },
    ), Capability.Write


async def handle_linode_domain_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_create tool request."""
    domain_name = arguments.get("domain", "")

    if is_dry_run(arguments):
        if not domain_name:
            return error_response("domain is required")
        domain_type = arguments.get("type", "master")
        return build_dry_run_response(
            "linode_domain_create",
            arguments.get("environment", ""),
            "POST",
            "/domains",
            None,
            side_effects=[
                f"A new {domain_type} DNS domain {domain_name!r} will be created."
            ],
        )

    if not arguments.get("confirm"):
        return error_response("This creates a DNS domain. Set confirm=true to proceed.")

    if not domain_name:
        return error_response("domain is required")

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


def create_linode_domain_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_update tool."""
    return Tool(
        name="linode_domain_update",
        description=(
            "Updates an existing DNS domain. Pass dry_run=true to preview "
            "without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
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
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm this operation. Ignored when "
                        "dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["domain_id", "confirm"],
        },
    ), Capability.Write


def _domain_update_side_effects(
    state: Any, new_domain: Any, new_soa: Any, new_description: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for domain update. Reports the domain-name and SOA
    email changes against the fetched state and notes a description change.
    """
    side_effects: list[str] = []
    if new_domain:
        from_domain = getattr(state, "domain", "")
        if from_domain and from_domain != new_domain:
            side_effects.append(
                f"Domain name changes from {from_domain!r} to {new_domain!r}."
            )
        else:
            side_effects.append(f"Domain name is set to {new_domain!r}.")
    if new_soa:
        from_soa = getattr(state, "soa_email", "")
        if new_soa != from_soa:
            side_effects.append(f"SOA email is set to {new_soa!r}.")
    if new_description:
        side_effects.append("The domain description is updated.")
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_domain_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_update tool request."""
    domain_id = arguments.get("domain_id", 0)

    if is_dry_run(arguments):
        if not domain_id:
            return error_response("domain_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain(int(domain_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _domain_update_side_effects(
                state,
                arguments.get("domain"),
                arguments.get("soa_email"),
                arguments.get("description"),
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_update",
            "PUT",
            f"/domains/{int(domain_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This updates a DNS domain. Set confirm=true to proceed.")

    if not domain_id:
        return error_response("domain_id is required")

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


def create_linode_domain_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_delete tool."""
    return Tool(
        name="linode_domain_delete",
        description=(
            "Deletes a DNS domain. WARNING: This also deletes all associated "
            "records. Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "domain_id": {
                    "type": "integer",
                    "description": "The ID of the domain to delete (required)",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Must be true to confirm deletion. Ignored when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
                PARAM_MODE: MODE_PROP,
                PARAM_PLAN_ID: PLAN_ID_PROP,
            },
            "required": ["domain_id", "confirm"],
        },
    ), Capability.Destroy


async def _domain_delete_dependency_walk(
    client: RetryableClient, domain_id: int
) -> DryRunDetails:
    """Phase 2 Tier A walk for domain delete. Deleting a domain destroys all
    its DNS records; the walk surfaces the NS records (the delegation that
    breaks) as cascade_deleted dependencies and warns with the total record
    count. Best-effort: a failed record list becomes a warning, not a hard
    error.
    """
    details: DryRunDetails = {}
    try:
        records = await client.list_domain_records(domain_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list domain records: {exc}"]
        return details

    dependencies: list[dict[str, Any]] = []
    ns_count = 0
    for record in records:
        if record.type.upper() != "NS":
            continue
        ns_count += 1
        dependencies.append(
            {
                "kind": "ns_record",
                "label": record.target,
                "action": "cascade_deleted",
                "note": f"NS record for {record.name}",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
    if records:
        details["warnings"] = [
            f"Deleting this domain destroys {len(records)} DNS record(s), "
            f"including {ns_count} NS record(s)."
        ]
    return details


async def _domain_delete_two_stage(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    domain_id = arguments.get("domain_id", 0)
    if not domain_id:
        return error_response("domain_id is required")

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_domain(int(domain_id))

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain(int(domain_id))
        return {
            "message": f"Domain {domain_id} deleted successfully",
            "domain_id": domain_id,
        }

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_domain_delete",
        method="DELETE",
        path=f"/domains/{int(domain_id)}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Domain"),
    )


async def handle_linode_domain_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_delete tool request."""
    domain_id = arguments.get("domain_id", 0)

    two_stage = await _domain_delete_two_stage(arguments, cfg)
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):
        if not domain_id:
            return error_response("domain_id is required")

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain(int(domain_id))

        async def _walk(client: RetryableClient, _state: Any) -> DryRunDetails:
            return await _domain_delete_dependency_walk(client, int(domain_id))

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_delete",
            "DELETE",
            f"/domains/{int(domain_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This is destructive. Set confirm=true to proceed.")

    if not domain_id:
        return error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain(int(domain_id))
        return {
            "message": f"Domain {domain_id} deleted successfully",
            "domain_id": domain_id,
        }

    return await execute_tool(cfg, arguments, "delete domain", _call)
