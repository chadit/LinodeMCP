from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast
from urllib.parse import quote

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import domain_pb2
from linodemcp.linode import APIError, NetworkError, validate_label
from linodemcp.profiles import Capability
from linodemcp.tools.drivers import run_destructive_tool, run_write_tool
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_tool,
    is_dry_run,
    optional_tags_argument,
)
from linodemcp.tools.proto_enum import optional_enum_error
from linodemcp.tools.proto_response import raw_int, raw_str, serialize_api_response
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


# The domain-name pattern POST /domains documents. Pinning it verbatim rather
# than hand-rolling one keeps both languages rejecting the same names.
_DOMAIN_CREATE_NAME_PATTERN = re.compile(
    r"^(\*\.)?([a-zA-Z0-9-_]{1,63}\.)+"
    r"([a-zA-Z]{2,3}\.)?([a-zA-Z]{2,16}|xn--[a-zA-Z0-9]+)$"
)
_DOMAIN_CREATE_NAME_MAX_LENGTH = 253
_DOMAIN_CREATE_ARRAY_FIELDS = ("axfr_ips", "master_ips", "tags")
_DOMAIN_CREATE_INTEGER_FIELDS = (
    "expire_sec",
    "refresh_sec",
    "retry_sec",
    "ttl_sec",
)
_DOMAIN_CREATE_STRING_FIELDS = ("description", "group", "soa_email", "status")


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
        input_schema=schema("linode.mcp.v1.DomainImportInput"),
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
        # retry=False: POST /domains/import is not idempotent, so replaying
        # after a transient failure leaves a duplicate zone nobody hears about.
        raw = await client.route_raw(
            "linode_domain_import", body=request_body, retry=False
        )
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
        input_schema=schema("linode.mcp.v1.DomainCloneInput"),
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
        # retry=False: the clone POST mints a new API-assigned ID, so replaying
        # after a transient failure leaves a duplicate nobody hears about.
        raw = await client.route_raw(
            "linode_domain_clone", domain_id, body=request_body, retry=False
        )
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
        input_schema=schema("linode.mcp.v1.DomainCreateInput"),
    ), Capability.Write


def _domain_create_name_error(domain_name: Any) -> str | None:
    """Return the domain-name validation error, if any."""
    if not isinstance(domain_name, str) or not domain_name:
        return "domain is required"
    if len(domain_name) > _DOMAIN_CREATE_NAME_MAX_LENGTH:
        return "domain must be between 1 and 253 characters"
    if _DOMAIN_CREATE_NAME_PATTERN.fullmatch(domain_name) is None:
        return "domain must match the documented domain-name pattern"
    return None


def _domain_create_type_error(domain_type: Any) -> str | None:
    """Return the domain-type validation error, if any."""
    if not isinstance(domain_type, str) or not domain_type:
        return "type is required"
    if domain_type not in ("master", "slave"):
        return "type must be one of: master, slave"
    return None


def _domain_create_integer_fields_error(arguments: dict[str, Any]) -> str | None:
    """Return an integer-field type error, if any.

    ``type(value) is int`` rather than ``isinstance`` because ``bool`` subclasses
    ``int`` and ``true`` is not an interval the API accepts. A float passes only
    when exactly integral, which also rejects NaN and the infinities.
    """
    for name in _DOMAIN_CREATE_INTEGER_FIELDS:
        if name not in arguments:
            continue
        value = arguments[name]
        if type(value) is int:
            continue
        if not isinstance(value, float) or not value.is_integer():
            return f"{name} must be an integer"

    return None


def _domain_create_status_error(arguments: dict[str, Any]) -> str | None:
    """Return a status type or enum validation error, if any."""
    if "status" not in arguments:
        return None
    status = arguments["status"]
    if not isinstance(status, str):
        return "status must be a string"
    if status not in ("active", "disabled"):
        return "status must be one of: active, disabled"
    return None


def _domain_create_optional_fields_error(arguments: dict[str, Any]) -> str | None:
    """Return an optional-field type or enum validation error, if any."""
    status_error = _domain_create_status_error(arguments)
    if status_error is not None:
        return status_error

    for name in _DOMAIN_CREATE_ARRAY_FIELDS:
        if name not in arguments:
            continue
        value: object = arguments[name]
        if not isinstance(value, list) or not all(
            isinstance(item, str) for item in cast("list[object]", value)
        ):
            return f"{name} must be an array of strings"

    integer_error = _domain_create_integer_fields_error(arguments)
    if integer_error is not None:
        return integer_error

    for name in _DOMAIN_CREATE_STRING_FIELDS:
        if name in arguments and not isinstance(arguments[name], str):
            return f"{name} must be a string"

    return None


def _domain_create_request(
    arguments: dict[str, Any],
) -> tuple[dict[str, Any], str | None]:
    """Build the POST /domains body, or return the first validation failure.

    Optional fields are copied by presence, not truthiness, so an explicit ""
    or 0 still reaches the API instead of being dropped.
    """
    domain_name = arguments.get("domain")
    domain_type = arguments.get("type")

    for message in (
        _domain_create_name_error(domain_name),
        _domain_create_type_error(domain_type),
        _domain_create_optional_fields_error(arguments),
    ):
        if message is not None:
            return {}, message

    if domain_type == "master" and not arguments.get("soa_email"):
        return {}, "soa_email is required for master domains"
    if domain_type == "slave" and not arguments.get("master_ips"):
        return {}, "master_ips must include at least one value for slave domains"

    body: dict[str, Any] = {"domain": domain_name, "type": domain_type}
    for name in (
        *_DOMAIN_CREATE_ARRAY_FIELDS,
        *_DOMAIN_CREATE_INTEGER_FIELDS,
        *_DOMAIN_CREATE_STRING_FIELDS,
    ):
        if name in arguments:
            value = arguments[name]
            body[name] = (
                int(value)
                if name in _DOMAIN_CREATE_INTEGER_FIELDS and isinstance(value, float)
                else value
            )

    return body, None


async def handle_linode_domain_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_create tool request."""
    body, validation_message = _domain_create_request(arguments)

    # The prose names body fields, so build it only once validation passed.
    side_effects: tuple[str, ...] = ()
    if validation_message is None:
        side_effects = (
            f'A new {body["type"]} DNS domain "{body["domain"]}" will be created.',
        )

    return await run_write_tool(
        cfg,
        arguments,
        tool="linode_domain_create",
        error_action="create domain",
        body=body,
        error=validation_message,
        preview_error=validation_message,
        side_effects=side_effects,
        invalid_response_subject="domain create",
    )


def create_linode_domain_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_update tool."""
    return Tool(
        name="linode_domain_update",
        description=(
            "Updates an existing DNS domain. Pass dry_run=true to preview "
            "without updating."
        ),
        input_schema=schema("linode.mcp.v1.DomainUpdateInput"),
    ), Capability.Write


def _domain_update_side_effects(
    state: Any, new_domain: Any, new_soa: Any, new_description: Any
) -> DryRunDetails:
    """Tier B dry-run walk for domain update, diffed against the fetched state."""
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


def _domain_update_body(
    arguments: dict[str, Any], tags: list[str] | None
) -> dict[str, Any]:
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
    if tags:
        body["tags"] = tags
    # Falsy values are dropped rather than sent, matching the omitempty tags on
    # Go's UpdateDomainRequest so both clients put the same bytes on the wire.
    for field in (
        "axfr_ips",
        "master_ips",
        "expire_sec",
        "refresh_sec",
        "retry_sec",
        "type",
    ):
        value = arguments.get(field)
        if value:
            body[field] = value
    return body


def _domain_update_gate_error(domain_id: Any, arguments: dict[str, Any]) -> str | None:
    """Check the required domain_id, then validate the update body."""
    if not domain_id:
        return "domain_id is required"
    return _domain_update_body_error(arguments)


def _domain_update_body_error(arguments: dict[str, Any]) -> str | None:
    """Validate the zone-transfer lists, type enum, and SOA timers.

    Check order and message text match Go's populateDomainUpdateOptionals so
    both languages report the same first problem for the same input.
    """
    for field in ("axfr_ips", "master_ips"):
        value: Any = arguments.get(field)
        if value is None:
            continue
        if not isinstance(value, list) or not all(
            isinstance(entry, str) for entry in cast("list[object]", value)
        ):
            return f"{field} must be an array of strings"

    type_error = optional_enum_error(arguments, "type", domain_pb2.DomainType.Value)
    if type_error is not None:
        return type_error

    for field in ("expire_sec", "refresh_sec", "retry_sec"):
        timer: Any = arguments.get(field)
        if timer is None:
            continue
        if isinstance(timer, bool) or not isinstance(timer, int):
            return f"{field} must be an integer"

    return None


async def handle_linode_domain_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_update tool request."""
    domain_id = arguments.get("domain_id", 0)

    tags, tags_error = optional_tags_argument(arguments)
    if tags_error:
        return error_response(tags_error)

    async def _fetch(client: RetryableClient) -> Any:
        return await client.get_domain(int(domain_id))

    async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _domain_update_side_effects(
            state,
            arguments.get("domain"),
            arguments.get("soa_email"),
            arguments.get("description"),
        )

    return await run_write_tool(
        cfg,
        arguments,
        tool="linode_domain_update",
        error_action="update domain",
        body=_domain_update_body(arguments, tags),
        path_values={"domain_id": int(domain_id or 0)},
        # One gate for the id check and the body validation so both languages
        # report the same first problem. A preview checks only the id, which is
        # all Go's update preview checks.
        error=_domain_update_gate_error(domain_id, arguments),
        preview_error=None if domain_id else "domain_id is required",
        state_fetch=_fetch,
        walk=_walk,
        preview_request_body=False,
    )


def create_linode_domain_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_delete tool."""
    return Tool(
        name="linode_domain_delete",
        description=(
            "Deletes a DNS domain. WARNING: This also deletes all associated "
            "records. Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        input_schema=schema("linode.mcp.v1.DomainDeleteInput"),
    ), Capability.Destroy


async def _domain_delete_dependency_walk(
    client: RetryableClient, domain_id: int
) -> DryRunDetails:
    """Tier A dry-run walk for domain delete.

    Deleting a domain destroys every record, so NS records (the delegation that
    breaks) surface as cascade_deleted dependencies alongside a total-count
    warning. Best effort: a failed record list degrades to a warning.
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


async def handle_linode_domain_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_delete tool request."""
    domain_id = arguments.get("domain_id", 0)

    async def _fetch(client: RetryableClient) -> Any:
        return await client.get_domain(int(domain_id))

    async def _execute(client: RetryableClient) -> None:
        await client.delete_domain(int(domain_id))

    async def _walk(client: RetryableClient, _state: Any) -> DryRunDetails:
        return await _domain_delete_dependency_walk(client, int(domain_id))

    return await run_destructive_tool(
        cfg,
        arguments,
        tool="linode_domain_delete",
        error_action="delete domain",
        id_args={"domain_id": int(domain_id or 0)},
        fetch_state=_fetch,
        execute=_execute,
        dependency_walk=_walk,
        error=None if domain_id else "domain_id is required",
        # The contract leaves this tool's success_message unset, so the prose
        # stays here. `make tool-response` lists every tool in that position.
        success_message=(
            f"Domain {domain_id} and all its records removed successfully"
        ),
    )
