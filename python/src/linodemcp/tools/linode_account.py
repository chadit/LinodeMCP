"""Linode account tool - authenticated user account information."""

import re
from pathlib import Path
from typing import Any, cast

from mcp.types import TextContent, Tool

from linodemcp.config import Config
from linodemcp.linode import RetryableClient
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    DRY_RUN_PROP,
    ENV_PARAM_SCHEMA,
    PARAM_DRY_RUN,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
)

_CHILD_ACCOUNT_EUUID_PATTERN = re.compile(
    r"^[A-Za-z0-9]{8}-[A-Za-z0-9]{4}-[A-Za-z0-9]{4}-[A-Za-z0-9]{16}$"
)


def create_linode_account_tool() -> tuple[Tool, Capability]:
    """Create the linode_account tool."""
    return Tool(
        name="linode_account",
        description=(
            "Retrieves the authenticated user's Linode account information "
            "including billing details and capabilities"
        ),
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


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


def create_linode_account_agreements_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_agreements_list tool."""
    return Tool(
        name="linode_account_agreements_list",
        description="Lists agreements on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_account_agreements_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_agreements_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_agreements()

    return await execute_tool(cfg, arguments, "list Linode account agreements", _call)


def create_linode_account_events_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_events_list tool."""
    return Tool(
        name="linode_account_events_list",
        description="Lists events on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_events_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_events_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_events(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account events", _call)


def create_linode_account_event_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_get tool."""
    return Tool(
        name="linode_account_event_get",
        description="Gets a Linode account event by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "event_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Account event ID to retrieve",
                },
            },
            "required": ["event_id"],
        },
    ), Capability.Read


async def handle_linode_account_event_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_event_get tool request."""
    event_id = arguments.get("event_id")
    if not isinstance(event_id, int) or isinstance(event_id, bool) or event_id < 1:
        return error_response("event_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_event(event_id)

    return await execute_tool(cfg, arguments, "get Linode account event", _call)


def create_linode_account_event_seen_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_event_seen tool."""
    return Tool(
        name="linode_account_event_seen",
        description=(
            "Marks a Linode account event as seen. "
            "Pass dry_run=true to preview without marking the event seen."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "event_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Account event ID to mark as seen",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["event_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_event_seen(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_event_seen tool request."""
    event_id = arguments.get("event_id")
    if not isinstance(event_id, int) or isinstance(event_id, bool) or event_id < 1:
        return error_response("event_id must be a positive integer")

    if is_dry_run(arguments):
        # Dry-run previews the current event with a safe GET. The response
        # still reports the POST that would run; it must not mark the event
        # seen when dry_run=true.
        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_event_seen",
            "POST",
            f"/account/events/{event_id}/seen",
            lambda client: client.get_account_event(event_id),
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This marks an account event seen. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.mark_account_event_seen(event_id)

    return await execute_tool(cfg, arguments, "mark Linode account event seen", _call)


_ACCOUNT_AGREEMENT_FIELDS = (
    "billing_agreement",
    "eu_model",
    "master_service_agreement",
    "privacy_policy",
)


def create_linode_account_agreements_acknowledge_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_agreements_acknowledge tool."""
    return Tool(
        name="linode_account_agreements_acknowledge",
        description="Acknowledges agreements on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "billing_agreement": {
                    "type": "boolean",
                    "description": "Acknowledge the billing agreement",
                },
                "eu_model": {
                    "type": "boolean",
                    "description": "Acknowledge the EU model agreement",
                },
                "master_service_agreement": {
                    "type": "boolean",
                    "description": "Acknowledge the master service agreement",
                },
                "privacy_policy": {
                    "type": "boolean",
                    "description": "Acknowledge the privacy policy",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_account_agreements_acknowledge(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_agreements_acknowledge tool request."""
    agreements: dict[str, bool] = {}
    for field in _ACCOUNT_AGREEMENT_FIELDS:
        value = arguments.get(field)
        if value is None:
            continue
        if not isinstance(value, bool):
            return error_response(f"{field} must be a boolean")
        agreements[field] = value

    if not agreements:
        return error_response("At least one account agreement field is required")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_agreements_acknowledge",
            arguments.get("environment", ""),
            "POST",
            "/account/agreements",
            None,
            side_effects=[
                "The selected account agreements are acknowledged for this account."
            ],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This acknowledges account agreements. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.acknowledge_account_agreements(agreements)

    return await execute_tool(
        cfg, arguments, "acknowledge Linode account agreements", _call
    )


def create_linode_account_beta_enroll_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_beta_enroll tool."""
    return Tool(
        name="linode_account_beta_enroll",
        description=(
            "Enrolls the Linode account in a Beta program. "
            "Pass dry_run=true to preview without enrolling."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "id": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Beta program ID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to acknowledge this mutating operation. "
                        "Required even when dry_run=true; dry_run still "
                        "avoids the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_beta_enroll(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_enroll tool request."""
    raw_beta_id = arguments.get("id")
    if raw_beta_id is None:
        return error_response("id is required")
    if not isinstance(raw_beta_id, str):
        return error_response("id must be a string")

    beta_id = raw_beta_id.strip()
    if not beta_id:
        return error_response("id is required")

    if arguments.get("confirm") is not True:
        return error_response(
            "This enrolls the account in a beta program. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_beta_enroll",
            arguments.get("environment", ""),
            "POST",
            "/account/betas",
            None,
            request_body={"id": beta_id},
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.enroll_account_beta(beta_id)

    return await execute_tool(
        cfg, arguments, f"enroll Linode account in beta {beta_id}", _call
    )


def create_linode_account_cancel_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_cancel tool."""
    return Tool(
        name="linode_account_cancel",
        description=(
            "Cancels the Linode account. "
            "Pass dry_run=true to preview without canceling."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "comments": {
                    "type": "string",
                    "description": "Optional cancellation comments",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this destructive operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Destroy


async def handle_linode_account_cancel(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_cancel tool request."""
    comments = arguments.get("comments")
    if comments is not None and not isinstance(comments, str):
        return error_response("comments must be a string")

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_cancel",
            arguments.get("environment", ""),
            "POST",
            "/account/cancel",
            None,
            request_body={"comments": comments} if comments is not None else None,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This cancels the Linode account. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.cancel_account(comments=comments)

    return await execute_tool(cfg, arguments, "cancel Linode account", _call)


def create_linode_account_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_update tool."""
    return Tool(
        name="linode_account_update",
        description=(
            "Updates Linode account contact and billing-address information. "
            "Pass dry_run=true to preview without updating."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "first_name": {"type": "string", "description": "First name"},
                "last_name": {"type": "string", "description": "Last name"},
                "email": {"type": "string", "description": "Contact email"},
                "company": {"type": "string", "description": "Company name"},
                "address_1": {"type": "string", "description": "Address line 1"},
                "address_2": {"type": "string", "description": "Address line 2"},
                "city": {"type": "string", "description": "City"},
                "state": {"type": "string", "description": "State or province"},
                "zip": {"type": "string", "description": "Postal code"},
                "country": {"type": "string", "description": "Country code"},
                "phone": {"type": "string", "description": "Phone number"},
                "tax_id": {"type": "string", "description": "Tax ID"},
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this mutating operation. Ignored "
                        "when dry_run=true."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["confirm"],
        },
    ), Capability.Write


async def handle_linode_account_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_update tool request."""
    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_account()

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_update",
            "PUT",
            "/account",
            _fetch,
        )

    if not arguments.get("confirm"):
        return error_response(
            "This updates account information. Set confirm=true to proceed."
        )

    update_fields = {
        key: arguments.get(key)
        for key in (
            "first_name",
            "last_name",
            "email",
            "company",
            "address_1",
            "address_2",
            "city",
            "state",
            "zip",
            "country",
            "phone",
            "tax_id",
        )
        if arguments.get(key) is not None
    }
    if not update_fields:
        return error_response("At least one account field is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        account = await client.update_account(**update_fields)
        return {
            "message": "Account updated successfully",
            "account": {
                "first_name": account.first_name,
                "last_name": account.last_name,
                "email": account.email,
                "company": account.company,
                "address_1": account.address_1,
                "address_2": account.address_2,
                "city": account.city,
                "state": account.state,
                "zip": account.zip,
                "country": account.country,
                "phone": account.phone,
            },
        }

    return await execute_tool(cfg, arguments, "update Linode account", _call)


_MIN_REGION_ID_PARTS = 2


def _is_region_id(value: str) -> bool:
    """Return True when value looks like a Linode region ID slug."""
    parts = value.split("-")
    return len(parts) >= _MIN_REGION_ID_PARTS and all(
        part and all("0" <= c <= "9" or "a" <= c <= "z" for c in part) for part in parts
    )


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, int) or isinstance(value, bool):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def create_linode_account_beta_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_beta_get tool."""
    return Tool(
        name="linode_account_beta_get",
        description="Gets an enrolled Beta program on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "beta_id": {
                    "type": "string",
                    "description": "Beta program ID to retrieve",
                },
            },
            "required": ["beta_id"],
        },
    ), Capability.Read


async def handle_linode_account_beta_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_beta_get tool request."""
    raw_beta_id = arguments.get("beta_id")
    if raw_beta_id is None:
        return error_response("beta_id is required")
    if not isinstance(raw_beta_id, str):
        return error_response("beta_id must be a string")

    beta_id = raw_beta_id.strip()
    if not beta_id:
        return error_response("beta_id is required")
    if "/" in beta_id or "?" in beta_id or ".." in beta_id:
        return error_response("beta_id must not contain '/', '?', or '..'")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_beta(beta_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account beta {beta_id}", _call
    )


def create_linode_account_child_account_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_account_get tool."""
    return Tool(
        name="linode_account_child_account_get",
        description="Gets a child account by EUUID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "euuid": {
                    "type": "string",
                    "description": "Child account EUUID to retrieve",
                },
            },
            "required": ["euuid"],
        },
    ), Capability.Read


async def handle_linode_account_child_account_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_account_get tool request."""
    raw_euuid = arguments.get("euuid")
    if raw_euuid is None:
        return error_response("euuid is required")
    if not isinstance(raw_euuid, str):
        return error_response("euuid must be a string")

    euuid = raw_euuid.strip()
    if not euuid:
        return error_response("euuid is required")
    if "/" in euuid or "?" in euuid or ".." in euuid:
        return error_response("euuid must not contain '/', '?', or '..'")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_child_account(euuid)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode child account {euuid}", _call
    )


def create_linode_account_availability_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_availability_list tool."""
    return Tool(
        name="linode_account_availability_list",
        description="Lists available Linode services for the account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_availability_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_availability_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_availability(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account availability", _call)


def create_linode_account_availability_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_availability_get tool."""
    return Tool(
        name="linode_account_availability_get",
        description=(
            "Gets available Linode services for the account in a specific region."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "region_id": {
                    "type": "string",
                    "description": "Region ID to check (for example, 'us-east')",
                },
            },
            "required": ["region_id"],
        },
    ), Capability.Read


async def handle_linode_account_availability_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_availability_get tool request."""
    raw_region_id = arguments.get("region_id")
    if raw_region_id is None:
        return error_response("region_id is required")
    if not isinstance(raw_region_id, str):
        return error_response("region_id must be a string")

    region_id = raw_region_id.strip()
    if not region_id:
        return error_response("region_id is required")
    if not _is_region_id(region_id):
        return error_response(
            "region_id must be a lowercase region slug with letters, "
            "numbers, and hyphens"
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_account_availability(region_id)

    return await execute_tool(
        cfg, arguments, f"retrieve Linode account availability for {region_id}", _call
    )


def create_linode_account_betas_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_betas_list tool."""
    return Tool(
        name="linode_account_betas_list",
        description="Lists enrolled Beta programs for the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_betas_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_betas_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_betas(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account betas", _call)


def create_linode_account_child_accounts_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_accounts_list tool."""
    return Tool(
        name="linode_account_child_accounts_list",
        description="Lists child accounts for the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_child_accounts_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_accounts_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_account_child_accounts(page=page, page_size=page_size)

    return await execute_tool(
        cfg, arguments, "list Linode account child accounts", _call
    )


def create_linode_account_child_account_token_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_child_account_token_create tool."""
    return Tool(
        name="linode_account_child_account_token_create",
        description=(
            "Creates a proxy user token for a child account. "
            "Pass dry_run=true to preview without creating a token."
        ),
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "euuid": {
                    "type": "string",
                    "minLength": 1,
                    "description": "Child account EUUID",
                },
                "confirm": {
                    "type": "boolean",
                    "description": (
                        "Set true to confirm this credential-creating operation. "
                        "Required even when dry_run=true; dry_run still avoids "
                        "the client call."
                    ),
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["euuid", "confirm"],
        },
    ), Capability.Write


def _validate_child_account_euuid(value: Any) -> str | None:
    """Validate a child account EUUID tool argument."""
    if value is None:
        return None
    if not isinstance(value, str):
        return None

    euuid = value.strip()
    if not euuid:
        return None
    if _CHILD_ACCOUNT_EUUID_PATTERN.fullmatch(euuid) is None:
        return None
    return euuid


async def handle_linode_account_child_account_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_child_account_token_create tool request."""
    euuid = _validate_child_account_euuid(arguments.get("euuid"))
    if euuid is None:
        return error_response("euuid must match the child account EUUID format")

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a child account proxy token. Set confirm=true to proceed."
        )

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_account_child_account_token_create",
            arguments.get("environment", ""),
            "POST",
            f"/account/child-accounts/{euuid}/token",
            None,
            side_effects=[
                "A proxy user token is created for the selected child account."
            ],
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.create_account_child_account_token(euuid)

    return await execute_tool(
        cfg, arguments, "create Linode account child account proxy token", _call
    )


def create_linode_account_tags_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tags_list tool."""
    return Tool(
        name="linode_account_tags_list",
        description="Lists tags on the Linode account.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_tags_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tags_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_tags(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode account tags", _call)


def create_linode_account_tag_objects_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_objects_list tool."""
    return Tool(
        name="linode_account_tag_objects_list",
        description="Lists objects assigned to a Linode account tag.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tag_label": {
                    "type": "string",
                    "description": "Label of the tag to inspect",
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["tag_label"],
        },
    ), Capability.Read


async def handle_linode_account_tag_objects_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_objects_list tool request."""
    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_tagged_objects(
            tag_label, page=page, page_size=page_size
        )

    return await execute_tool(cfg, arguments, "list tagged objects", _call)


def create_linode_account_tag_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_create tool."""
    return Tool(
        name="linode_account_tag_create",
        description="Creates a Linode account tag and optionally assigns resources.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "label": {"type": "string", "description": "Tag label to create"},
                "domains": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Domain IDs to assign to the tag",
                },
                "linodes": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Linode IDs to assign to the tag",
                },
                "nodebalancers": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "NodeBalancer IDs to assign to the tag",
                },
                "volumes": {
                    "type": "array",
                    "items": {"type": "integer", "minimum": 1},
                    "description": "Volume IDs to assign to the tag",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["label", "confirm"],
        },
    ), Capability.Write


def _optional_int_list_argument(
    arguments: dict[str, Any], name: str
) -> list[int] | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, list):
        raise TypeError(f"{name} must be a list of integers")
    values: list[int] = []
    for item in cast("list[object]", value):
        if not isinstance(item, int) or isinstance(item, bool):
            raise TypeError(f"{name} must be a list of integers")
        if item < 1:
            raise ValueError(f"{name} must contain positive integers")
        values.append(item)
    if not values:
        return None
    return values


async def handle_linode_account_tag_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_create tool request."""
    if is_dry_run(arguments):
        label = arguments.get("label")
        if not isinstance(label, str) or not label.strip():
            return error_response("label is required")
        return build_dry_run_response(
            "linode_account_tag_create",
            arguments.get("environment", ""),
            "POST",
            "/tags",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response("This creates a tag. Set confirm=true to proceed.")

    label = arguments.get("label")
    if not isinstance(label, str) or not label.strip():
        return error_response("label is required")

    try:
        resource_ids = {
            name: _optional_int_list_argument(arguments, name)
            for name in ("domains", "linodes", "nodebalancers", "volumes")
        }
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        tag = await client.create_tag(label.strip(), **resource_ids)
        return {"message": f"Tag '{label.strip()}' created successfully", "tag": tag}

    return await execute_tool(cfg, arguments, "create Linode tag", _call)


def create_linode_account_tag_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_tag_delete tool."""
    return Tool(
        name="linode_account_tag_delete",
        description="Deletes a Linode account tag by label.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "tag_label": {
                    "type": "string",
                    "description": "Label of the tag to delete",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this destructive operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["tag_label", "confirm"],
        },
    ), Capability.Destroy


async def handle_linode_account_tag_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_tag_delete tool request."""
    if is_dry_run(arguments):
        tag_label = arguments.get("tag_label")
        if not isinstance(tag_label, str) or not tag_label.strip():
            return error_response("tag_label is required")
        return build_dry_run_response(
            "linode_account_tag_delete",
            arguments.get("environment", ""),
            "DELETE",
            f"/tags/{tag_label}",
            None,
        )

    tag_label = arguments.get("tag_label")
    if not isinstance(tag_label, str) or not tag_label.strip():
        return error_response("tag_label is required")
    if not arguments.get("confirm"):
        return error_response("This deletes a tag. Set confirm=true to proceed.")

    async def _call(client: RetryableClient) -> dict[str, str]:
        await client.delete_tag(tag_label)
        return {"message": f"Tag '{tag_label}' deleted successfully"}

    return await execute_tool(cfg, arguments, "delete Linode tag", _call)


def create_linode_account_support_ticket_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_create tool."""
    return Tool(
        name="linode_account_support_ticket_create",
        description="Opens a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "summary": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 64,
                    "description": "Support ticket summary or title",
                },
                "description": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 65000,
                    "description": "Full details of the issue or question",
                },
                "bucket": {
                    "type": "string",
                    "description": "Object Storage bucket name",
                },
                "database_id": {"type": "integer", "minimum": 1},
                "domain_id": {"type": "integer", "minimum": 1},
                "firewall_id": {"type": "integer", "minimum": 1},
                "linode_id": {"type": "integer", "minimum": 1},
                "lkecluster_id": {"type": "integer", "minimum": 1},
                "longviewclient_id": {"type": "integer", "minimum": 1},
                "managed_issue": {"type": "boolean"},
                "nodebalancer_id": {"type": "integer", "minimum": 1},
                "region": {"type": "string", "description": "Region ID"},
                "severity": {
                    "type": "integer",
                    "minimum": 1,
                    "maximum": 3,
                    "description": "Ticket severity: 1 major, 2 moderate, 3 low",
                },
                "vlan": {"type": "string", "description": "VLAN label"},
                "volume_id": {"type": "integer", "minimum": 1},
                "vpc_id": {"type": "integer", "minimum": 1},
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["summary", "description", "confirm"],
        },
    ), Capability.Write


def _required_string_argument(arguments: dict[str, Any], name: str) -> str:
    value = arguments.get(name)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} is required")
    return value.strip()


def _optional_string_argument(arguments: dict[str, Any], name: str) -> str | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"{name} must be a non-empty string")
    return value.strip()


def _optional_bool_argument(arguments: dict[str, Any], name: str) -> bool | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, bool):
        raise TypeError(f"{name} must be a boolean")
    return value


def _support_ticket_id(arguments: dict[str, Any]) -> int | list[TextContent]:
    """Parse a positive integer ticket_id, or return an error response."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")
    return ticket_id


def _attachment_file(arguments: dict[str, Any]) -> str | list[TextContent]:
    """Parse the attachment file path, or return an error response."""
    file = arguments.get("file")
    if not isinstance(file, str) or not file.strip():
        return error_response("file is required")
    file_path = file.strip()
    if not Path(file_path).is_absolute():
        return error_response("file must be a local, absolute path")
    return file_path


async def handle_linode_account_support_ticket_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_create tool request."""
    if is_dry_run(arguments):
        try:
            _required_string_argument(arguments, "summary")
            _required_string_argument(arguments, "description")
        except (TypeError, ValueError) as exc:
            return error_response(str(exc))
        summary = arguments.get("summary")
        effect = (
            f"A new support ticket {summary!r} will be opened."
            if summary
            else "A new support ticket will be opened."
        )
        return build_dry_run_response(
            "linode_account_support_ticket_create",
            arguments.get("environment", ""),
            "POST",
            "/support/tickets",
            None,
            side_effects=[effect],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This opens a support ticket. Set confirm=true to proceed."
        )

    try:
        summary = _required_string_argument(arguments, "summary")
        description = _required_string_argument(arguments, "description")
        ticket_fields: dict[str, Any] = {
            "bucket": _optional_string_argument(arguments, "bucket"),
            "database_id": _optional_int_argument(arguments, "database_id", 1),
            "domain_id": _optional_int_argument(arguments, "domain_id", 1),
            "firewall_id": _optional_int_argument(arguments, "firewall_id", 1),
            "linode_id": _optional_int_argument(arguments, "linode_id", 1),
            "lkecluster_id": _optional_int_argument(arguments, "lkecluster_id", 1),
            "longviewclient_id": _optional_int_argument(
                arguments, "longviewclient_id", 1
            ),
            "managed_issue": _optional_bool_argument(arguments, "managed_issue"),
            "nodebalancer_id": _optional_int_argument(arguments, "nodebalancer_id", 1),
            "region": _optional_string_argument(arguments, "region"),
            "severity": _optional_int_argument(arguments, "severity", 1, 3),
            "vlan": _optional_string_argument(arguments, "vlan"),
            "volume_id": _optional_int_argument(arguments, "volume_id", 1),
            "vpc_id": _optional_int_argument(arguments, "vpc_id", 1),
        }
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        ticket = await client.create_support_ticket(
            summary, description, **ticket_fields
        )
        return {"message": "Support ticket opened successfully", "ticket": ticket}

    return await execute_tool(cfg, arguments, "open Linode support ticket", _call)


def create_linode_managed_stats_tool() -> tuple[Tool, Capability]:
    """Create the linode_managed_stats tool."""
    return Tool(
        name="linode_managed_stats",
        description="Lists Managed statistics from the last 24 hours.",
        inputSchema={
            "type": "object",
            "properties": ENV_PARAM_SCHEMA,
        },
    ), Capability.Read


async def handle_linode_managed_stats(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_managed_stats tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_managed_stats()

    return await execute_tool(cfg, arguments, "list Linode Managed statistics", _call)


def create_linode_account_support_tickets_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_tickets_list tool."""
    return Tool(
        name="linode_account_support_tickets_list",
        description="Lists Linode support tickets.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
        },
    ), Capability.Read


async def handle_linode_account_support_tickets_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_tickets_list tool request."""
    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_support_tickets(page=page, page_size=page_size)

    return await execute_tool(cfg, arguments, "list Linode support tickets", _call)


def create_linode_account_support_ticket_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_get tool."""
    return Tool(
        name="linode_account_support_ticket_get",
        description="Gets a Linode support ticket by ID.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to retrieve",
                },
            },
            "required": ["ticket_id"],
        },
    ), Capability.Read


async def handle_linode_account_support_ticket_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_get tool request."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.get_support_ticket(ticket_id)

    return await execute_tool(cfg, arguments, "get Linode support ticket", _call)


def create_linode_account_support_ticket_replies_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_replies_list tool."""
    return Tool(
        name="linode_account_support_ticket_replies_list",
        description="Lists replies on a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID whose replies to list",
                },
                "page": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Page of results to return",
                },
                "page_size": {
                    "type": "integer",
                    "minimum": 25,
                    "maximum": 500,
                    "description": "Number of results per page",
                },
            },
            "required": ["ticket_id"],
        },
    ), Capability.Read


async def handle_linode_account_support_ticket_replies_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_replies_list tool request."""
    ticket_id = arguments.get("ticket_id")
    if not isinstance(ticket_id, int) or isinstance(ticket_id, bool) or ticket_id < 1:
        return error_response("ticket_id must be a positive integer")

    try:
        page = _optional_int_argument(arguments, "page", 1)
        page_size = _optional_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return await client.list_support_ticket_replies(
            ticket_id, page=page, page_size=page_size
        )

    return await execute_tool(
        cfg, arguments, "list Linode support ticket replies", _call
    )


def create_linode_account_support_ticket_close_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_close tool."""
    return Tool(
        name="linode_account_support_ticket_close",
        description="Closes a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to close",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["ticket_id", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_support_ticket_close(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_close tool request."""
    if is_dry_run(arguments):
        ticket = _support_ticket_id(arguments)
        if isinstance(ticket, list):
            return ticket
        tid = ticket

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_support_ticket(tid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {"side_effects": [f"Support ticket {tid} will be closed."]}

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_support_ticket_close",
            "POST",
            f"/support/tickets/{tid}/close",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This closes a support ticket. Set confirm=true to proceed."
        )

    ticket = _support_ticket_id(arguments)
    if isinstance(ticket, list):
        return ticket
    ticket_id = ticket

    async def _call(client: RetryableClient) -> dict[str, Any]:
        closed_ticket = await client.close_support_ticket(ticket_id)
        return {
            "message": "Support ticket closed successfully",
            "ticket": closed_ticket,
        }

    return await execute_tool(cfg, arguments, "close Linode support ticket", _call)


def create_linode_account_support_ticket_reply_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_account_support_ticket_reply_create tool."""
    return Tool(
        name="linode_account_support_ticket_reply_create",
        description="Creates a reply on a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to reply to",
                },
                "description": {
                    "type": "string",
                    "description": "Reply body to add to the support ticket",
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["ticket_id", "description", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_support_ticket_reply_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_reply_create tool request."""
    if is_dry_run(arguments):
        ticket = _support_ticket_id(arguments)
        if isinstance(ticket, list):
            return ticket
        tid = ticket

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_support_ticket(tid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [f"A reply will be posted to support ticket {tid}."]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_support_ticket_reply_create",
            "POST",
            f"/support/tickets/{tid}/replies",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a support ticket reply. Set confirm=true to proceed."
        )

    ticket = _support_ticket_id(arguments)
    if isinstance(ticket, list):
        return ticket
    ticket_id = ticket

    description = arguments.get("description")
    if not isinstance(description, str) or not description.strip():
        return error_response("description is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        reply = await client.create_support_ticket_reply(ticket_id, description.strip())
        return {"message": "Support ticket reply created successfully", "reply": reply}

    return await execute_tool(
        cfg, arguments, "create Linode support ticket reply", _call
    )


def create_linode_account_support_ticket_attachment_create_tool() -> tuple[
    Tool, Capability
]:
    """Create the linode_account_support_ticket_attachment_create tool."""
    return Tool(
        name="linode_account_support_ticket_attachment_create",
        description="Creates an attachment on a Linode support ticket.",
        inputSchema={
            "type": "object",
            "properties": {
                **ENV_PARAM_SCHEMA,
                "ticket_id": {
                    "type": "integer",
                    "minimum": 1,
                    "description": "Support ticket ID to attach the file to",
                },
                "file": {
                    "type": "string",
                    "description": ("Local, absolute path to the file to attach"),
                },
                "confirm": {
                    "type": "boolean",
                    "description": "Set true to confirm this mutating operation.",
                },
                PARAM_DRY_RUN: DRY_RUN_PROP,
            },
            "required": ["ticket_id", "file", "confirm"],
        },
    ), Capability.Write


async def handle_linode_account_support_ticket_attachment_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_account_support_ticket_attachment_create tool request."""
    if is_dry_run(arguments):
        ticket = _support_ticket_id(arguments)
        if isinstance(ticket, list):
            return ticket
        tid = ticket

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_support_ticket(tid)

        async def _walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
            return {
                "side_effects": [
                    f"A file attachment will be uploaded to support ticket {tid}."
                ]
            }

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_account_support_ticket_attachment_create",
            "POST",
            f"/support/tickets/{tid}/attachments",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a support ticket attachment. Set confirm=true to proceed."
        )

    ticket = _support_ticket_id(arguments)
    if isinstance(ticket, list):
        return ticket
    ticket_id = ticket

    parsed_file = _attachment_file(arguments)
    if isinstance(parsed_file, list):
        return parsed_file
    file_path = parsed_file

    async def _call(client: RetryableClient) -> dict[str, Any]:
        attachment = await client.create_support_ticket_attachment(ticket_id, file_path)
        return {
            "message": "Support ticket attachment created successfully",
            "attachment": attachment,
        }

    return await execute_tool(
        cfg, arguments, "create Linode support ticket attachment", _call
    )
