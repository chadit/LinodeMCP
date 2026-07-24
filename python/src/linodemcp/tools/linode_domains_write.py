from __future__ import annotations

from typing import TYPE_CHECKING, Any
from urllib.parse import quote

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import domain_pb2
from linodemcp.linode import APIError, NetworkError, validate_label
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.proto_response import raw_int, raw_str, serialize_api_response
from linodemcp.tools.toolschemas import schema
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
        inputSchema=schema("linode.mcp.v1.DomainImportInput"),
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
                (
                    f"DNS domain {domain_name!r} will be imported from "
                    f"{remote_nameserver!r}."
                )
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This imports a DNS domain zone. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.post_raw("/domains/import", request_body)
        domain_id = raw_int(raw, "id")
        domain_label = raw_str(raw, "domain")
        return serialize_api_response(
            {
                "message": (
                    f"Domain '{domain_label}' (ID: {domain_id}) imported successfully"
                ),
                "domain": raw,
            },
            domain_pb2.DomainWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.DomainCloneInput"),
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
        raw = await client.post_raw(f"/domains/{encoded_domain_id}/clone", request_body)
        new_id = raw_int(raw, "id")
        new_label = raw_str(raw, "domain")
        return serialize_api_response(
            {
                "message": f"Domain {domain_id} cloned as '{new_label}' (ID: {new_id})",
                "domain": raw,
            },
            domain_pb2.DomainWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "clone domain", _call)


def create_linode_domain_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_create tool."""
    return Tool(
        name="linode_domain_create",
        description=(
            "Creates a new DNS domain. Pass dry_run=true to preview without creating."
        ),
        inputSchema=schema("linode.mcp.v1.DomainCreateInput"),
    ), Capability.Write


def _domain_create_field_error(arguments: dict[str, Any]) -> list[TextContent] | None:
    """Validate domain create fields; return an error response or None."""
    if not arguments.get("domain"):
        return error_response("domain is required")
    # type has no safe default; Go requires it, so require it here too rather
    # than silently defaulting to "master".
    if not arguments.get("type"):
        return error_response("type is required")
    return None


async def handle_linode_domain_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_create tool request."""
    domain_name = arguments.get("domain", "")

    if is_dry_run(arguments):
        fields_error = _domain_create_field_error(arguments)
        if fields_error is not None:
            return fields_error
        domain_type = arguments.get("type", "")
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

    fields_error = _domain_create_field_error(arguments)
    if fields_error is not None:
        return fields_error

    body: dict[str, Any] = {
        "domain": domain_name,
        "type": arguments.get("type", ""),
    }
    soa_email = arguments.get("soa_email")
    if soa_email:
        body["soa_email"] = soa_email
    description = arguments.get("description")
    if description:
        body["description"] = description
    ttl_sec = arguments.get("ttl_sec")
    if ttl_sec is not None:
        body["ttl_sec"] = ttl_sec

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.post_raw("/domains", body)
        new_id = raw_int(raw, "id")
        new_label = raw_str(raw, "domain")
        return serialize_api_response(
            {
                "message": f"Domain '{new_label}' (ID: {new_id}) created successfully",
                "domain": raw,
            },
            domain_pb2.DomainWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create domain", _call)


def create_linode_domain_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_update tool."""
    return Tool(
        name="linode_domain_update",
        description=(
            "Updates an existing DNS domain. Pass dry_run=true to preview "
            "without updating."
        ),
        inputSchema=schema("linode.mcp.v1.DomainUpdateInput"),
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


def _domain_update_body(arguments: dict[str, Any]) -> dict[str, Any]:
    """Build the domain update PUT body, mirroring the client's omit rules."""
    body: dict[str, Any] = {}
    if arguments.get("domain"):
        body["domain"] = arguments.get("domain")
    if arguments.get("soa_email"):
        body["soa_email"] = arguments.get("soa_email")
    if arguments.get("description") is not None:
        body["description"] = arguments.get("description")
    if arguments.get("status"):
        body["status"] = arguments.get("status")
    if arguments.get("ttl_sec") is not None:
        body["ttl_sec"] = arguments.get("ttl_sec")
    return body


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

    body = _domain_update_body(arguments)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.put_raw(f"/domains/{int(domain_id)}", body)
        return serialize_api_response(
            {
                "message": f"Domain {domain_id} modified successfully",
                "domain": raw,
            },
            domain_pb2.DomainWriteResponse(),
        )

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
        inputSchema=schema("linode.mcp.v1.DomainDeleteInput"),
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
            (
                f"Deleting this domain destroys {len(records)} DNS record(s), "
                f"including {ns_count} NS record(s)."
            )
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
        return serialize_api_response(
            {
                "message": (
                    f"Domain {domain_id} and all its records removed successfully"
                ),
                "domain_id": domain_id,
            },
            domain_pb2.DomainDeleteResponse(),
        )

    async def _ts_walk(client: RetryableClient, _state: Any) -> DryRunDetails:
        return await _domain_delete_dependency_walk(client, int(domain_id))

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_domain_delete",
        method="DELETE",
        path=f"/domains/{int(domain_id)}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("Domain"),
        dependency_walk=_ts_walk,
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
        return error_response(
            "This operation is destructive and deletes all DNS records. Set "
            "confirm=true to proceed."
        )

    if not domain_id:
        return error_response("domain_id is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_domain(int(domain_id))
        return serialize_api_response(
            {
                "message": (
                    f"Domain {domain_id} and all its records removed successfully"
                ),
                "domain_id": domain_id,
            },
            domain_pb2.DomainDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete domain", _call)
