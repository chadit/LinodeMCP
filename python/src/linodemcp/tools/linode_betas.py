"""Linode beta program tools."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from mcp.types import TextContent, Tool

from linodemcp.profiles import Capability
from linodemcp.tools.helpers import error_response, execute_tool
from linodemcp.tools.toolschemas import schema

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _is_beta_id(value: str) -> bool:
    """Return True when value looks like a Beta program ID slug."""
    return bool(value) and all(c.isalnum() or c == "-" for c in value)


def beta_program_to_response_dict(beta: dict[str, Any]) -> dict[str, Any]:
    """Shape a raw Beta program API dict to proto-canonical form.

    description and ended are nullable and omitted when null.
    """
    body: dict[str, Any] = {
        "class": beta.get("class") or "",
        "greenlight_only": bool(beta.get("greenlight_only")),
        "id": beta.get("id") or "",
        "label": beta.get("label") or "",
        "more_info": beta.get("more_info") or "",
        "started": beta.get("started") or "",
    }
    for key in ("description", "ended"):
        if beta.get(key) is not None:
            body[key] = beta[key]
    return body


def create_linode_beta_get_tool() -> tuple[Tool, Capability]:
    """Create the linode_beta_get tool."""
    return Tool(
        name="linode_beta_get",
        description="Gets details for an available Linode Beta program.",
        inputSchema=schema("linode.mcp.v1.BetaGetInput"),
    ), Capability.Read


async def handle_linode_beta_get(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_beta_get tool request."""
    raw_beta_id = arguments.get("beta_id")
    if raw_beta_id is None:
        return error_response("beta_id is required")
    if not isinstance(raw_beta_id, str):
        return error_response("beta_id must be a string")

    beta_id = raw_beta_id.strip()
    if not beta_id:
        return error_response("beta_id is required")
    if not _is_beta_id(beta_id):
        return error_response("beta_id must contain only letters, numbers, and hyphens")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        return beta_program_to_response_dict(await client.get_beta(beta_id))

    return await execute_tool(cfg, arguments, f"retrieve Linode beta {beta_id}", _call)
