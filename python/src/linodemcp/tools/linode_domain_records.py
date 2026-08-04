from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import domain_pb2
from linodemcp.linode import validate_dns_record_name, validate_dns_record_target
from linodemcp.profiles import Capability
from linodemcp.tools.drivers import run_destructive_tool
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)
from linodemcp.tools.proto_enum import optional_enum_error
from linodemcp.tools.proto_response import (
    raw_int,
    raw_str,
    serialize_api_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_domain_record_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_record_create tool."""
    return Tool(
        name="linode_domain_record_create",
        description="Creates a new DNS record for a domain.",
        input_schema=schema("linode.mcp.v1.DomainRecordCreateInput"),
    ), Capability.Write


def _record_create_error(
    domain_id: Any, record_type: str, arguments: dict[str, Any]
) -> list[TextContent] | None:
    """Validate record-create args; return an error response or None."""
    if not domain_id:
        return error_response("domain_id is required")
    if not record_type:
        return error_response("type is required")
    tag_error = _record_tag_error(arguments)
    if tag_error is not None:
        return error_response(tag_error)
    return None


def _validate_record_fields(
    record_type: str, name: Any, target: Any
) -> list[TextContent] | None:
    """Run DNS name/target validation; return an error response or None."""
    if name:
        try:
            validate_dns_record_name(name)
        except ValueError as exc:
            return error_response(str(exc))
    if target:
        try:
            validate_dns_record_target(record_type, target)
        except ValueError as exc:
            return error_response(str(exc))
    return None


def _record_tag_error(arguments: dict[str, Any]) -> str | None:
    """Validate the CAA tag argument with Go's exact message. Shared by record
    create and update so both tools reject the same values.
    """
    return optional_enum_error(arguments, "tag", domain_pb2.DomainRecordCAATag.Value)


def _domain_record_create_body(
    record_type: str, arguments: dict[str, Any]
) -> dict[str, Any]:
    """Build the record-create POST body, mirroring the client's omit rules."""
    body: dict[str, Any] = {"type": record_type}
    optional_fields: dict[str, Any] = {
        "name": arguments.get("name"),
        "target": arguments.get("target"),
        "priority": arguments.get("priority"),
        "weight": arguments.get("weight"),
        "port": arguments.get("port"),
        "ttl_sec": arguments.get("ttl_sec"),
        "service": arguments.get("service"),
        "protocol": arguments.get("protocol"),
        "tag": arguments.get("tag"),
    }
    body.update({k: v for k, v in optional_fields.items() if v is not None})
    return body


async def handle_linode_domain_record_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_create tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_type = arguments.get("type", "")

    if is_dry_run(arguments):
        fields_error = _record_create_error(domain_id, record_type, arguments)
        if fields_error is not None:
            return fields_error
        effect = (
            f"A new {record_type} record will be created in domain {int(domain_id)}"
        )
        name = arguments.get("name", "")
        target = arguments.get("target", "")
        if name:
            effect += f" for host {name!r}"
        if target:
            effect += f" targeting {target!r}"
        return build_dry_run_response(
            "linode_domain_record_create",
            arguments.get("environment", ""),
            "POST",
            f"/domains/{int(domain_id)}/records",
            None,
            side_effects=[f"{effect}."],
        )

    if not arguments.get("confirm"):
        return error_response("This creates a DNS record. Set confirm=true to proceed.")

    fields_error = _record_create_error(domain_id, record_type, arguments)
    if fields_error is not None:
        return fields_error

    validation_error = _validate_record_fields(
        record_type, arguments.get("name"), arguments.get("target")
    )
    if validation_error is not None:
        return validation_error

    body = _domain_record_create_body(record_type, arguments)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        # POST /domains/{id}/records is not idempotent: the zone accepts
        # identical records side by side, so a replay leaves a duplicate RR the
        # caller never learns about.
        raw = await client.route_raw(
            "linode_domain_record_create", int(domain_id), body=body, retry=False
        )
        rec_type = raw_str(raw, "type")
        rec_id = raw_int(raw, "id")
        return serialize_api_response(
            {
                "message": f"{rec_type} record (ID: {rec_id}) created successfully",
                "record": raw,
            },
            domain_pb2.DomainRecordWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create DNS record", _call)


def create_linode_domain_record_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_record_update tool."""
    return Tool(
        name="linode_domain_record_update",
        description="Updates an existing DNS record.",
        input_schema=schema("linode.mcp.v1.DomainRecordUpdateInput"),
    ), Capability.Write


def _record_update_error(
    domain_id: Any, record_id: Any, arguments: dict[str, Any]
) -> list[TextContent] | None:
    """Validate record-update args; return an error response or None."""
    if not domain_id:
        return error_response("domain_id is required")
    if not record_id:
        return error_response("record_id is required")
    tag_error = _record_tag_error(arguments)
    if tag_error is not None:
        return error_response(tag_error)
    return None


def _domain_record_update_body(arguments: dict[str, Any]) -> dict[str, Any]:
    """Build the record-update PUT body, mirroring the client's omit rules."""
    body: dict[str, Any] = {}
    for field in ("name", "target", "priority", "weight", "port", "ttl_sec"):
        value = arguments.get(field)
        if value is not None:
            body[field] = value
    # Falsy values are dropped, matching the omitempty tags on Go's
    # UpdateDomainRecordRequest so both clients send the same bytes.
    for field in ("service", "protocol", "tag"):
        value = arguments.get(field)
        if value:
            body[field] = value
    return body


def _domain_record_update_side_effects(
    state: Any, new_name: Any, new_target: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for domain record update. Reports the record name
    and target changes against the fetched state.
    """
    side_effects: list[str] = []
    if new_name is not None:
        from_name = getattr(state, "name", "")
        if new_name != from_name:
            side_effects.append(
                f"Record name changes from {from_name!r} to {new_name!r}."
            )
    if new_target:
        from_target = getattr(state, "target", "")
        if new_target != from_target:
            side_effects.append(
                f"Record target changes from {from_target!r} to {new_target!r}."
            )
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_domain_record_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_update tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    if is_dry_run(arguments):
        fields_error = _record_update_error(domain_id, record_id, arguments)
        if fields_error is not None:
            return fields_error

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_domain_record(int(domain_id), int(record_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _domain_record_update_side_effects(
                state, arguments.get("name"), arguments.get("target")
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_domain_record_update",
            "PUT",
            f"/domains/{int(domain_id)}/records/{int(record_id)}",
            _fetch,
            _walk,
        )

    if not arguments.get("confirm"):
        return error_response("This updates a DNS record. Set confirm=true to proceed.")

    fields_error = _record_update_error(domain_id, record_id, arguments)
    if fields_error is not None:
        return fields_error

    name = arguments.get("name")
    if name:
        try:
            validate_dns_record_name(name)
        except ValueError as exc:
            return error_response(str(exc))

    body = _domain_record_update_body(arguments)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.route_raw(
            "linode_domain_record_update", int(domain_id), int(record_id), body=body
        )
        return serialize_api_response(
            {
                "message": f"Record {record_id} modified successfully",
                "record": raw,
            },
            domain_pb2.DomainRecordWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update DNS record", _call)


def create_linode_domain_record_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_domain_record_delete tool."""
    return Tool(
        name="linode_domain_record_delete",
        description=(
            "Deletes a DNS record. Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        input_schema=schema("linode.mcp.v1.DomainRecordDeleteInput"),
    ), Capability.Destroy


async def handle_linode_domain_record_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_domain_record_delete tool request."""
    domain_id = arguments.get("domain_id", 0)
    record_id = arguments.get("record_id", 0)

    # ID validation runs before every branch, including the plan/apply one:
    # each of them needs both IDs, and the spec is explicit that a dry run
    # errors out on a missing required arg the same way the real call would.
    if not domain_id:
        return error_response("domain_id is required")
    if not record_id:
        return error_response("record_id is required")

    async def _fetch(client: RetryableClient) -> Any:
        return await client.get_domain_record(int(domain_id), int(record_id))

    async def _execute(client: RetryableClient) -> None:
        await client.delete_domain_record(int(domain_id), int(record_id))

    return await run_destructive_tool(
        cfg,
        arguments,
        tool="linode_domain_record_delete",
        error_action="delete DNS record",
        id_args={"domain_id": int(domain_id), "record_id": int(record_id)},
        fetch_state=_fetch,
        execute=_execute,
        # Undeclared in the contract, like every delete whose prose names two
        # resources; see the note on linode_domain_delete.
        success_message=(
            f"Record {record_id} removed successfully from domain {domain_id}"
        ),
    )
