"""Longview tools for LinodeMCP."""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import longview_pb2
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    build_dry_run_response,
    error_response,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_longview_client_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_update tool."""
    return Tool(
        name="linode_longview_client_update",
        description=(
            "Updates a Longview client label. Pass dry_run=true to preview "
            "without updating."
        ),
        inputSchema=schema("linode.mcp.v1.LongviewClientUpdateInput"),
    ), Capability.Write


def create_linode_longview_subscription_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_subscription_get tool."""
    return Tool(
        name="linode_longview_subscription_get",
        description="Gets details for a single Longview subscription by ID.",
        inputSchema=schema("linode.mcp.v1.LongviewSubscriptionGetInput"),
    ), Capability.Read


def create_linode_longview_plan_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_plan_get tool."""
    return Tool(
        name="linode_longview_plan_get",
        description="Gets the account Longview plan.",
        inputSchema=schema("linode.mcp.v1.LongviewPlanGetInput"),
    ), Capability.Read


def create_linode_longview_type_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_type_list tool."""
    return Tool(
        name="linode_longview_type_list",
        description="Lists Longview types.",
        inputSchema=schema("linode.mcp.v1.LongviewTypeListInput"),
    ), Capability.Read


def create_linode_longview_client_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_list tool."""
    return Tool(
        name="linode_longview_client_list",
        description="Lists Longview clients on the account.",
        inputSchema=schema("linode.mcp.v1.LongviewClientListInput"),
    ), Capability.Read


def create_linode_longview_client_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_get tool."""
    return Tool(
        name="linode_longview_client_get",
        description="Gets a Longview client by ID.",
        inputSchema=schema("linode.mcp.v1.LongviewClientGetInput"),
    ), Capability.Read


def create_linode_longview_plan_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_plan_update tool."""
    return Tool(
        name="linode_longview_plan_update",
        description=(
            "Updates the account Longview plan."
            " Requires confirm=true and supports dry_run=true to preview."
        ),
        inputSchema=schema("linode.mcp.v1.LongviewPlanUpdateInput"),
    ), Capability.Write


def create_linode_longview_subscription_list_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_subscription_list tool."""
    return Tool(
        name="linode_longview_subscription_list",
        description="Lists Longview subscriptions on the account.",
        inputSchema=schema("linode.mcp.v1.LongviewSubscriptionListInput"),
    ), Capability.Read


def create_linode_longview_client_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_delete tool."""
    return Tool(
        name="linode_longview_client_delete",
        description=(
            "Deletes a Longview client."
            " Requires confirm=true and supports dry_run=true to preview."
        ),
        inputSchema=schema("linode.mcp.v1.LongviewClientDeleteInput"),
    ), Capability.Destroy


def _required_positive_int_argument(arguments: dict[str, Any], name: str) -> int:
    value = arguments.get(name)
    if value is None:
        msg = f"{name} is required"
        raise ValueError(msg)
    if type(value) is not int or value < 1:
        msg = f"{name} must be a positive integer"
        raise ValueError(msg)
    return value


def _required_subscription_id(arguments: dict[str, Any]) -> str:
    # Longview subscription IDs are opaque strings (for example "longview-10"),
    # not integers, so this mirrors the Go side: a non-empty string with no path
    # or query separators that could escape the URL path segment.
    if "subscription_id" not in arguments:
        msg = "subscription_id is required"
        raise ValueError(msg)
    raw = arguments["subscription_id"]
    if not isinstance(raw, str) or not raw.strip():
        msg = "subscription_id must be a non-empty string"
        raise ValueError(msg)
    if raw != raw.strip() or "/" in raw or "?" in raw or ".." in raw:
        msg = (
            "subscription_id must not contain path separators, query separators, "
            "or traversal segments"
        )
        raise ValueError(msg)
    return raw


def _longview_client_id_error(value: object) -> str | None:
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        return "client_id must be a positive integer"
    return None


def _longview_client_update_body(arguments: dict[str, Any]) -> dict[str, str]:
    label = arguments.get("label")
    if not isinstance(label, str):
        return {}
    return {"label": label}


def _longview_client_update_error(
    arguments: dict[str, Any],
) -> list[TextContent] | None:
    id_error = _longview_client_id_error(arguments.get("client_id"))
    if id_error is not None:
        return error_response(id_error)
    label = arguments.get("label")
    if not isinstance(label, str) or not label:
        return error_response("label must be a non-empty string")
    # Length + charset check ported from Go's validLongviewClientLabel (Python
    # update previously only checked non-empty); Go's single combined message.
    if (
        len(label) < LABEL_MIN_LENGTH
        or len(label) > LABEL_MAX_LENGTH
        or LABEL_PATTERN.fullmatch(label) is None
    ):
        return error_response(
            "label must be 3-32 characters and contain only letters, digits, "
            "hyphen, or underscore"
        )
    return None


async def handle_linode_longview_client_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_update tool request."""
    validation = _longview_client_update_error(arguments)
    if validation is not None:
        return validation

    client_id = cast("int", arguments["client_id"])
    body = _longview_client_update_body(arguments)

    if is_dry_run(arguments):
        return build_dry_run_response(
            "linode_longview_client_update",
            arguments.get("environment", ""),
            "PUT",
            f"/longview/clients/{client_id}",
            None,
            request_body=body,
            side_effects=["The Longview client is updated with the provided label."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a Longview client. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_longview_client(client_id, label=body["label"])
        return serialize_api_response(
            {
                "message": "Longview client updated successfully",
                "longview_client": result,
            },
            longview_pb2.LongviewClientWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Longview client", _call)


async def handle_linode_longview_type_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_type_list tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_longview_types()
        return serialize_list_response(
            raw, "types", longview_pb2.LongviewTypeListResponse()
        )

    return await execute_tool(cfg, arguments, "list Longview types", _call)


async def handle_linode_longview_client_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_longview_clients(page=page, page_size=page_size)
        return serialize_list_response(
            raw, "longview_clients", longview_pb2.LongviewClientListResponse()
        )

    return await execute_tool(cfg, arguments, "list Longview clients", _call)


LABEL_MIN_LENGTH = 3
LABEL_MAX_LENGTH = 32
LABEL_PATTERN = re.compile(r"^[A-Za-z0-9_-]+$")


def create_linode_longview_client_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_longview_client_create tool."""
    return Tool(
        name="linode_longview_client_create",
        description=(
            "Creates a Longview client. Pass dry_run=true to preview without "
            "creating the client."
        ),
        inputSchema=schema("linode.mcp.v1.LongviewClientCreateInput"),
    ), Capability.Write


def _validate_longview_client_label(value: object) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError("label is required")
    label = value.strip()
    if len(label) < LABEL_MIN_LENGTH or len(label) > LABEL_MAX_LENGTH:
        raise ValueError("label must be 3 to 32 characters")
    if LABEL_PATTERN.fullmatch(label) is None:
        raise ValueError(
            "label may only contain letters, numbers, hyphens, and underscores"
        )
    return label


async def handle_linode_longview_client_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_get tool request."""
    try:
        client_id = _required_positive_int_argument(arguments, "client_id")
    except ValueError as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        payload = await client.get_longview_client(client_id)
        return serialize_api_response(payload, longview_pb2.LongviewClient())

    return await execute_tool(cfg, arguments, "retrieve Longview client", _call)


async def handle_linode_longview_client_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_create tool request."""
    try:
        label = _validate_longview_client_label(arguments.get("label"))
    except ValueError as exc:
        return error_response(str(exc))

    confirm = arguments.get("confirm")
    if not isinstance(confirm, bool) or confirm is not True:
        return error_response(
            "This creates a Longview client and returns setup credentials. Set "
            "confirm=true to proceed."
        )

    if is_dry_run(arguments):
        environment = arguments.get("environment")
        if not isinstance(environment, str):
            environment = ""
        return build_dry_run_response(
            "linode_longview_client_create",
            environment,
            "POST",
            "/longview/clients",
            None,
            side_effects=[f"A Longview client labeled {label!r} will be created."],
            request_body={"label": label},
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        client_data = await client.create_longview_client(label)
        return serialize_api_response(
            {
                "message": "Longview client created successfully",
                "warning": (
                    "IMPORTANT: Save the API key and install code if they are "
                    "present; they are required to configure the Longview client "
                    "application."
                ),
                "longview_client": client_data,
            },
            longview_pb2.LongviewClientCreateWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create Longview client", _call)


async def handle_linode_longview_plan_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_plan_update tool request."""
    longview_subscription_value = arguments.get("longview_subscription")
    if not isinstance(longview_subscription_value, str):
        return error_response("longview_subscription is required and must be a string")
    longview_subscription = longview_subscription_value
    if not re.fullmatch(r"longview-[1-9][0-9]*", longview_subscription):
        return error_response(
            "longview_subscription must be a Longview plan ID like longview-10"
        )

    request_body: dict[str, Any] = {"longview_subscription": longview_subscription}

    if is_dry_run(arguments):
        environment = arguments.get("environment")
        if not isinstance(environment, str):
            environment = ""
        return build_dry_run_response(
            "linode_longview_plan_update",
            environment,
            "PUT",
            "/longview/plan",
            None,
            request_body=request_body,
            side_effects=["The account Longview plan would be updated."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates the Longview subscription plan. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        plan = await client.update_longview_plan(longview_subscription)
        return serialize_api_response(
            {
                "message": "Longview plan updated successfully",
                "plan": plan,
            },
            longview_pb2.LongviewSubscriptionWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update Longview plan", _call)


async def handle_linode_longview_subscription_list(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_subscription_list tool request."""
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.list_longview_subscriptions(page=page, page_size=page_size)
        return serialize_list_response(
            raw,
            "longview_subscriptions",
            longview_pb2.LongviewSubscriptionListResponse(),
        )

    return await execute_tool(cfg, arguments, "list Longview subscriptions", _call)


async def handle_linode_longview_client_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_client_delete tool request."""
    try:
        client_id = _required_positive_int_argument(arguments, "client_id")
    except ValueError as exc:
        return error_response(str(exc))

    if is_dry_run(arguments):
        environment = arguments.get("environment")
        if not isinstance(environment, str):
            environment = ""
        return build_dry_run_response(
            "linode_longview_client_delete",
            environment,
            "DELETE",
            f"/longview/clients/{client_id}",
            None,
            side_effects=[f"Longview client {client_id} would be deleted."],
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This deletes a Longview client. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_longview_client(client_id)
        return serialize_api_response(
            {
                "message": "Longview client deleted successfully",
                "client_id": client_id,
            },
            longview_pb2.LongviewClientIDResponse(),
        )

    return await execute_tool(cfg, arguments, "delete Longview client", _call)


async def handle_linode_longview_subscription_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_subscription_get tool request."""
    try:
        subscription_id = _required_subscription_id(arguments)
    except ValueError as exc:
        return error_response(str(exc))

    async def _call(client: RetryableClient) -> dict[str, Any]:
        subscription = await client.get_longview_subscription(subscription_id)
        return serialize_api_response(subscription, longview_pb2.LongviewSubscription())

    return await execute_tool(
        cfg,
        arguments,
        f"get Longview subscription {subscription_id}",
        _call,
    )


async def handle_linode_longview_plan_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_longview_plan_get tool request."""

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.get_longview_plan()
        return serialize_api_response(raw, longview_pb2.LongviewSubscription())

    return await execute_tool(cfg, arguments, "get Longview plan", _call)
