from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from mcp.types import TextContent, Tool

from linodemcp.tools.helpers import error_response, execute_tool

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def create_linode_monitor_service_token_create_tool() -> Tool:
    """Create the linode_monitor_service_token_create tool."""
    return Tool(
        name="linode_monitor_service_token_create",
        description=(
            "Creates a JWT for the Linode Metrics service scoped to the given "
            "entities. The token is returned only once and cannot be retrieved "
            "later; capture both `token` and `expiry` from the response."
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
                "service_type": {
                    "type": "string",
                    "description": (
                        "Metrics service type, e.g. 'dbaas' or 'linode' (required)"
                    ),
                },
                "entity_ids": {
                    "type": "array",
                    "items": {"type": "integer"},
                    "minItems": 1,
                    "description": (
                        "Non-empty list of entity IDs the token will grant access "
                        "to (required)"
                    ),
                },
            },
            "required": ["service_type", "entity_ids"],
        },
    )


def _coerce_entity_ids(raw: object) -> list[int] | None:
    """Return raw as a list of ints, or None if any element is not an int.

    Typed `object` rather than `Any`; the inner cast satisfies pyright strict
    mode by giving each iterated item a known type to narrow from.
    """
    if not isinstance(raw, list) or not raw:
        return None
    # isinstance narrows raw to list[Unknown]; cast to list[object] gives
    # each iterated item a known type for pyright to narrow further.
    items = cast("list[object]", raw)
    result: list[int] = []
    for item in items:
        # bool is a subclass of int; reject it explicitly to avoid `True` -> 1.
        if isinstance(item, bool) or not isinstance(item, int):
            return None
        result.append(item)
    return result


async def handle_linode_monitor_service_token_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_monitor_service_token_create tool request."""
    service_type = arguments.get("service_type", "")
    if not service_type or not isinstance(service_type, str):
        return error_response("service_type is required")

    entity_ids = _coerce_entity_ids(arguments.get("entity_ids"))
    if entity_ids is None:
        return error_response("entity_ids must be a non-empty list of integers")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        data = await client.create_monitor_service_token(service_type, entity_ids)
        return {
            "message": f"Monitor service token created for '{service_type}'",
            "service_type": service_type,
            "token": data.get("token"),
            "expiry": data.get("expiry"),
        }

    return await execute_tool(cfg, arguments, "create monitor service token", _call)
