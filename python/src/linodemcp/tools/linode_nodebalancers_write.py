from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

import httpx
from mcp.types import TextContent, Tool

from linodemcp.genpb.linode.mcp.v1 import (
    firewall_pb2,
    nodebalancer_config_node_pb2,
    nodebalancer_config_pb2,
    nodebalancer_pb2,
)
from linodemcp.linode import APIError, NetworkError, validate_ipv4_address
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    TWO_STAGE_NOTE,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    required_int_id,
)
from linodemcp.tools.proto_enum import optional_enum_error
from linodemcp.tools.proto_response import (
    raw_int,
    raw_str,
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.toolschemas import schema
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _optional_int_argument(
    arguments: dict[str, Any], name: str, minimum: int, maximum: int | None = None
) -> int | None:
    value = arguments.get(name)
    if value is None:
        return None
    if isinstance(value, bool) or not isinstance(value, int):
        raise TypeError(f"{name} must be an integer")
    if value < minimum:
        raise ValueError(f"{name} must be at least {minimum}")
    if maximum is not None and value > maximum:
        raise ValueError(f"{name} must be at most {maximum}")
    return value


def _firewall_ids_argument(arguments: dict[str, Any]) -> list[int] | None:
    raw_value: object = arguments.get("firewall_ids")
    if not isinstance(raw_value, list):
        return None

    firewall_ids: list[int] = []
    for item in cast("list[object]", raw_value):
        if isinstance(item, bool) or not isinstance(item, int) or item < 1:
            return None
        firewall_ids.append(item)
    return firewall_ids


NODE_LABEL_MIN_LENGTH = 3
NODE_LABEL_MAX_LENGTH = 32


def _optional_non_empty_string(arguments: dict[str, Any], name: str) -> str | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, str) or not value.strip():
        raise TypeError(f"{name} must be a non-empty string")
    return value


def _node_create_fields(arguments: dict[str, Any]) -> tuple[dict[str, Any], str | None]:
    try:
        address = _optional_non_empty_string(arguments, "address")
        label = _optional_non_empty_string(arguments, "label")
        mode = _optional_non_empty_string(arguments, "mode")
        subnet_id = _optional_int_argument(arguments, "subnet_id", 1)
        weight = _optional_int_argument(arguments, "weight", 1, 255)
    except (TypeError, ValueError) as exc:
        return {}, str(exc)

    if address is None:
        return {}, "address is required"
    if label is None:
        return {}, "label is required"
    if not (NODE_LABEL_MIN_LENGTH <= len(label) <= NODE_LABEL_MAX_LENGTH):
        return {}, "label must be 3 to 32 characters"
    mode_error = optional_enum_error(
        arguments, "mode", nodebalancer_config_node_pb2.NodeBalancerNodeMode.Value
    )
    if mode_error is not None:
        return {}, mode_error

    return {
        key: value
        for key, value in {
            "address": address,
            "label": label,
            "mode": mode,
            "subnet_id": subnet_id,
            "weight": weight,
        }.items()
        if value is not None
    }, None


def _node_update_fields(arguments: dict[str, Any]) -> tuple[dict[str, Any], str | None]:
    try:
        address = _optional_non_empty_string(arguments, "address")
        label = _optional_non_empty_string(arguments, "label")
        mode = _optional_non_empty_string(arguments, "mode")
        subnet_id = _optional_int_argument(arguments, "subnet_id", 1)
        weight = _optional_int_argument(arguments, "weight", 1, 255)
    except (TypeError, ValueError) as exc:
        return {}, str(exc)

    if label is not None and not (
        NODE_LABEL_MIN_LENGTH <= len(label) <= NODE_LABEL_MAX_LENGTH
    ):
        return {}, "label must be 3 to 32 characters"
    mode_error = optional_enum_error(
        arguments, "mode", nodebalancer_config_node_pb2.NodeBalancerNodeMode.Value
    )
    if mode_error is not None:
        return {}, mode_error

    fields = {
        key: value
        for key, value in {
            "address": address,
            "label": label,
            "mode": mode,
            "subnet_id": subnet_id,
            "weight": weight,
        }.items()
        if value is not None
    }
    if not fields:
        return {}, "at least one update field is required"
    return fields, None


def _nb_config_ids(
    arguments: dict[str, Any],
) -> tuple[int, int] | list[TextContent]:
    """Parse nodebalancer_id and config_id; return the pair or an error."""
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)
    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)
    return nodebalancer_id, config_id


def _nb_config_node_ids(
    arguments: dict[str, Any],
) -> tuple[int, int, int] | list[TextContent]:
    """Parse nodebalancer_id, config_id, and node_id, or return an error."""
    ids = _nb_config_ids(arguments)
    if isinstance(ids, list):
        return ids
    node_id, error = required_int_id(arguments, "node_id")
    if node_id is None:
        return error_response(error)
    return ids[0], ids[1], node_id


def _firewalls_update_fields(
    arguments: dict[str, Any],
) -> tuple[list[int], int | None, int | None] | list[TextContent]:
    """Parse firewall_ids plus pagination, or return an error response."""
    firewall_ids = _firewall_ids_argument(arguments)
    if firewall_ids is None:
        return error_response("firewall_ids must be a list of positive integers")
    try:
        page = pagination_int_argument(arguments, "page", 1)
        page_size = pagination_int_argument(arguments, "page_size", 25, 500)
    except (TypeError, ValueError) as exc:
        return error_response(str(exc))
    return firewall_ids, page, page_size


def create_linode_nodebalancer_firewall_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_firewall_update tool."""
    return Tool(
        name="linode_nodebalancer_firewall_update",
        description=(
            "Replaces the firewall assignments for a NodeBalancer. "
            "Pass an empty firewall_ids list to remove all assignments."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerFirewallUpdateInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_firewall_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_firewall_update tool request."""
    if is_dry_run(arguments):
        nb_id, error = required_int_id(arguments, "nodebalancer_id")
        if nb_id is None:
            return error_response(error)
        nb = nb_id

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer(nb)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_firewall_update",
            "PUT",
            f"/nodebalancers/{nb}/firewalls",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This replaces firewall assignments for a NodeBalancer. Set confirm=true "
            "to proceed."
        )

    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    parsed = _firewalls_update_fields(arguments)
    if isinstance(parsed, list):
        return parsed
    firewall_ids, page, page_size = parsed

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.update_nodebalancer_firewalls(
            nodebalancer_id, firewall_ids, page=page, page_size=page_size
        )
        return serialize_list_response(
            raw, "firewalls", firewall_pb2.FirewallListResponse()
        )

    return await execute_tool(
        cfg, arguments, "update NodeBalancer firewall assignments", _call
    )


def create_linode_nodebalancer_config_rebuild_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_rebuild tool."""
    return Tool(
        name="linode_nodebalancer_config_rebuild",
        description=(
            "Rebuilds a NodeBalancer config. "
            "Requires confirm because active connections may be affected."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigRebuildInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_config_rebuild(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_rebuild tool request."""
    if is_dry_run(arguments):
        ids = _nb_config_ids(arguments)
        if isinstance(ids, list):
            return ids
        nb, config = ids

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer_config(nb, config)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_config_rebuild",
            "POST",
            f"/nodebalancers/{nb}/configs/{config}/rebuild",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This rebuilds a NodeBalancer config. Set confirm=true to proceed."
        )

    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.rebuild_nodebalancer_config(nodebalancer_id, config_id)
        return serialize_api_response(
            {
                "message": (
                    f"Rebuilt config {config_id} for "
                    f"NodeBalancer {nodebalancer_id} successfully"
                ),
                "config": result,
            },
            nodebalancer_config_pb2.NodeBalancerConfigWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "rebuild NodeBalancer config", _call)


def create_linode_nodebalancer_config_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_delete tool."""
    return Tool(
        name="linode_nodebalancer_config_delete",
        description=(
            "Deletes a NodeBalancer config. "
            "WARNING: This removes the config and its backend nodes."
            " Pass dry_run=true to preview without deleting."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigDeleteInput"),
    ), Capability.Destroy


async def handle_linode_nodebalancer_config_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_delete tool request."""
    # Both branches need valid positive IDs, and the spec says dry-run
    # errors on missing required args the same way the real call would.
    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    config_id, error = required_int_id(arguments, "config_id")
    if config_id is None:
        return error_response(error)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer_config(nodebalancer_id, config_id)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_config_delete",
            "DELETE",
            f"/nodebalancers/{nodebalancer_id}/configs/{config_id}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This operation is destructive. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer_config(nodebalancer_id, config_id)
        return serialize_api_response(
            {
                "message": (
                    f"Config {config_id} removed from "
                    f"NodeBalancer {nodebalancer_id} successfully"
                ),
                "nodebalancer_id": nodebalancer_id,
                "config_id": config_id,
            },
            nodebalancer_config_pb2.NodeBalancerConfigDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete NodeBalancer config", _call)


def create_linode_nodebalancer_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_create tool."""
    return Tool(
        name="linode_nodebalancer_create",
        description=(
            "Creates a new NodeBalancer (load balancer). "
            "WARNING: Billing starts immediately."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerCreateInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_create tool request."""
    region = arguments.get("region", "")
    ipv4 = arguments.get("ipv4")

    if ipv4 is not None:
        try:
            validate_ipv4_address(ipv4)
        except (TypeError, ValueError) as exc:
            return error_response(str(exc))

    if is_dry_run(arguments):
        if not region:
            return error_response("region is required")
        nb_label = arguments.get("label")
        request_body: dict[str, Any] = {
            "region": region,
            "client_conn_throttle": arguments.get("client_conn_throttle", 0),
        }
        if nb_label:
            request_body["label"] = nb_label
        if ipv4 is not None:
            request_body["ipv4"] = ipv4
        effect = (
            f"A new NodeBalancer {nb_label!r} will be created in region {region}."
            if nb_label
            else f"A new NodeBalancer will be created in region {region}."
        )
        return build_dry_run_response(
            "linode_nodebalancer_create",
            arguments.get("environment", ""),
            "POST",
            "/nodebalancers",
            None,
            side_effects=[effect],
            warnings=["Billing for the NodeBalancer starts immediately on creation."],
            request_body=request_body,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This operation creates a billable resource. Set confirm=true to proceed."
        )

    if not region:
        return error_response("region is required")

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.create_nodebalancer_raw(
            region=region,
            label=arguments.get("label"),
            client_conn_throttle=arguments.get("client_conn_throttle", 0),
            ipv4=ipv4,
        )
        return serialize_api_response(
            {
                "message": (
                    f"NodeBalancer '{raw_str(raw, 'label')}' "
                    f"(ID: {raw_int(raw, 'id')}) "
                    f"created successfully in {raw_str(raw, 'region')}"
                ),
                "nodebalancer": raw,
            },
            nodebalancer_pb2.NodeBalancerWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create NodeBalancer", _call)


def create_linode_nodebalancer_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_update tool."""
    return Tool(
        name="linode_nodebalancer_update",
        description="Updates an existing NodeBalancer.",
        inputSchema=schema("linode.mcp.v1.NodeBalancerUpdateInput"),
    ), Capability.Write


def _nodebalancer_update_side_effects(
    state: Any, new_label: Any, new_throttle: Any
) -> DryRunDetails:
    """Phase 2 Tier B walk for NodeBalancer update. Reports the label change
    and a connection-throttle change against the fetched state.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f"Label changes from {from_label!r} to {new_label!r}.")
        else:
            side_effects.append(f"Label is set to {new_label!r}.")
    if new_throttle is not None:
        side_effects.append(
            f"Connection throttle is set to {new_throttle} connections per "
            "second per client IP."
        )
    return {"side_effects": side_effects} if side_effects else {}


async def handle_linode_nodebalancer_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_update tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    if not nodebalancer_id:
        return error_response("nodebalancer_id is required")

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer(int(nodebalancer_id))

        async def _walk(_client: RetryableClient, state: Any) -> DryRunDetails:
            return _nodebalancer_update_side_effects(
                state, arguments.get("label"), arguments.get("client_conn_throttle")
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_update",
            "PUT",
            f"/nodebalancers/{int(nodebalancer_id)}",
            _fetch,
            _walk,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a NodeBalancer. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.update_nodebalancer_raw(
            nodebalancer_id=int(nodebalancer_id),
            label=arguments.get("label"),
            client_conn_throttle=arguments.get("client_conn_throttle"),
        )
        return serialize_api_response(
            {
                "message": f"NodeBalancer {nodebalancer_id} modified successfully",
                "nodebalancer": raw,
            },
            nodebalancer_pb2.NodeBalancerWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update NodeBalancer", _call)


def create_linode_nodebalancer_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_delete tool."""
    return Tool(
        name="linode_nodebalancer_delete",
        description=(
            "Deletes a NodeBalancer. WARNING: This removes the load balancer "
            "and all its configurations."
            " Pass dry_run=true to preview without deleting."
        )
        + TWO_STAGE_NOTE,
        inputSchema=schema("linode.mcp.v1.NodeBalancerDeleteInput"),
    ), Capability.Destroy


async def _nodebalancer_delete_dependency_walk(
    client: RetryableClient, nodebalancer_id: int
) -> DryRunDetails:
    """Phase 2 Tier A walk for NodeBalancer delete. Each config (and its
    backend node list) is destroyed with the NodeBalancer, so configs are
    surfaced as cascade_deleted dependencies. Best-effort: a failed config
    list becomes a warning, not a hard error.
    """
    details: DryRunDetails = {}
    try:
        response = await client.list_nodebalancer_configs(nodebalancer_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list NodeBalancer configs: {exc}"]
        return details

    configs = cast("list[dict[str, Any]]", response.get("data", []))
    dependencies: list[dict[str, Any]] = [
        {
            "kind": "nodebalancer_config",
            "id": config.get("id"),
            "action": "cascade_deleted",
            "note": (
                f"{config.get('protocol', '')} config on port {config.get('port', '')}"
            ),
        }
        for config in configs
    ]

    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            f"Deleting this NodeBalancer destroys {len(dependencies)} config(s) "
            "and their backend node lists."
        ]
    return details


async def _nodebalancer_delete_two_stage(
    arguments: dict[str, Any], cfg: Config, nodebalancer_id_int: int
) -> list[TextContent] | None:
    """Run the plan/apply flow when mode is plan/apply, else None to fall through."""
    if arguments.get("mode") not in ("plan", "apply"):
        return None

    async def _ts_fetch(client: RetryableClient) -> Any:
        return await client.get_nodebalancer(nodebalancer_id_int)

    async def _ts_call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer(nodebalancer_id_int)
        return serialize_api_response(
            {
                "message": f"NodeBalancer {nodebalancer_id_int} removed successfully",
                "nodebalancer_id": nodebalancer_id_int,
            },
            nodebalancer_pb2.NodeBalancerDeleteResponse(),
        )

    async def _ts_walk(client: RetryableClient, _state: Any) -> DryRunDetails:
        return await _nodebalancer_delete_dependency_walk(client, nodebalancer_id_int)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name="linode_nodebalancer_delete",
        method="DELETE",
        path=f"/nodebalancers/{nodebalancer_id_int}",
        fetch_state=_ts_fetch,
        execute=_ts_call,
        hash_ignore=hash_ignore_fields("NodeBalancer"),
        dependency_walk=_ts_walk,
    )


async def handle_linode_nodebalancer_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_delete tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)

    # Both branches need a non-zero nodebalancer_id, and the spec says
    # dry-run errors on missing required args the same way the real
    # call would.
    if not nodebalancer_id:
        return error_response("nodebalancer_id is required")

    nodebalancer_id_int = int(nodebalancer_id)

    two_stage = await _nodebalancer_delete_two_stage(
        arguments, cfg, nodebalancer_id_int
    )
    if two_stage is not None:
        return two_stage

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer(nodebalancer_id_int)

        async def _walk(client: RetryableClient, _state: Any) -> DryRunDetails:
            return await _nodebalancer_delete_dependency_walk(
                client, nodebalancer_id_int
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_delete",
            "DELETE",
            f"/nodebalancers/{nodebalancer_id_int}",
            _fetch,
            _walk,
        )

    confirm = arguments.get("confirm", False)

    if not confirm:
        return error_response(
            "This operation is destructive. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer(nodebalancer_id_int)
        return serialize_api_response(
            {
                "message": f"NodeBalancer {nodebalancer_id_int} removed successfully",
                "nodebalancer_id": nodebalancer_id_int,
            },
            nodebalancer_pb2.NodeBalancerDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete NodeBalancer", _call)


def create_linode_nodebalancer_config_node_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_create tool."""
    return Tool(
        name="linode_nodebalancer_config_node_create",
        description=(
            "Creates a backend node in a NodeBalancer config. "
            "Requires confirm because live backend routing may change."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigNodeCreateInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_config_node_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_create tool request."""
    if is_dry_run(arguments):
        ids = _nb_config_ids(arguments)
        if isinstance(ids, list):
            return ids
        nb, config = ids
        return build_dry_run_response(
            "linode_nodebalancer_config_node_create",
            arguments.get("environment", ""),
            "POST",
            f"/nodebalancers/{nb}/configs/{config}/nodes",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a NodeBalancer node. Set confirm=true to proceed."
        )

    ids = _nb_config_ids(arguments)
    if isinstance(ids, list):
        return ids
    nodebalancer_id, config_id = ids

    fields, field_error = _node_create_fields(arguments)
    if field_error is not None:
        return error_response(field_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.create_nodebalancer_config_node(
            nodebalancer_id, config_id, fields
        )
        return serialize_api_response(
            {
                "message": (
                    f"NodeBalancer node {result.get('id')} created successfully "
                    f"for NodeBalancer {nodebalancer_id} config {config_id}"
                ),
                "node": result,
            },
            nodebalancer_config_node_pb2.NodeBalancerConfigNodeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create NodeBalancer config node", _call)


def create_linode_nodebalancer_config_node_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_update tool."""
    return Tool(
        name="linode_nodebalancer_config_node_update",
        description=(
            "Updates a node in a NodeBalancer config. "
            "Requires confirm because live backend routing may change."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigNodeUpdateInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_config_node_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_update tool request."""
    if is_dry_run(arguments):
        ids = _nb_config_node_ids(arguments)
        if isinstance(ids, list):
            return ids
        nb, config, node = ids

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer_config_node(nb, config, node)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_config_node_update",
            "PUT",
            f"/nodebalancers/{nb}/configs/{config}/nodes/{node}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a NodeBalancer node. Set confirm=true to proceed."
        )

    ids = _nb_config_node_ids(arguments)
    if isinstance(ids, list):
        return ids
    nodebalancer_id, config_id, node_id = ids

    fields, field_error = _node_update_fields(arguments)
    if field_error is not None:
        return error_response(field_error)

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_nodebalancer_config_node(
            nodebalancer_id, config_id, node_id, fields
        )
        return serialize_api_response(
            {
                "message": (
                    f"NodeBalancer node {result.get('id')} updated successfully "
                    f"for NodeBalancer {nodebalancer_id} config {config_id}"
                ),
                "node": result,
            },
            nodebalancer_config_node_pb2.NodeBalancerConfigNodeWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update NodeBalancer config node", _call)


def create_linode_nodebalancer_config_node_delete_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_node_delete tool."""
    return Tool(
        name="linode_nodebalancer_config_node_delete",
        description=(
            "Deletes a node from a NodeBalancer config. "
            "WARNING: This removes the backend node from the load balancer."
            " Pass dry_run=true to preview without deleting."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigNodeDeleteInput"),
    ), Capability.Destroy


async def handle_linode_nodebalancer_config_node_delete(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_node_delete tool request."""
    nodebalancer_id = arguments.get("nodebalancer_id", 0)
    config_id = arguments.get("config_id", 0)
    node_id = arguments.get("node_id", 0)

    # All three IDs must be present in both branches; the spec says
    # dry-run errors on missing required args the same way the real
    # call would.
    if not nodebalancer_id:
        return error_response("nodebalancer_id is required")

    if not config_id:
        return error_response("config_id is required")

    if not node_id:
        return error_response("node_id is required")

    nodebalancer_id_int = int(nodebalancer_id)
    config_id_int = int(config_id)
    node_id_int = int(node_id)

    if is_dry_run(arguments):

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer_config_node(
                nodebalancer_id_int, config_id_int, node_id_int
            )

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_config_node_delete",
            "DELETE",
            (
                f"/nodebalancers/{nodebalancer_id_int}/configs/{config_id_int}"
                f"/nodes/{node_id_int}"
            ),
            _fetch,
        )

    confirm = arguments.get("confirm", False)
    if not confirm:
        return error_response(
            "This deletes a NodeBalancer node. Set confirm=true to proceed."
        )

    async def _call(client: RetryableClient) -> dict[str, Any]:
        await client.delete_nodebalancer_config_node(
            nodebalancer_id_int, config_id_int, node_id_int
        )
        return serialize_api_response(
            {
                "message": (
                    f"NodeBalancer node {node_id_int} removed successfully "
                    f"from NodeBalancer {nodebalancer_id_int} config {config_id_int}"
                ),
                "nodebalancer_id": nodebalancer_id_int,
                "config_id": config_id_int,
                "node_id": node_id_int,
            },
            nodebalancer_config_node_pb2.NodeBalancerConfigNodeDeleteResponse(),
        )

    return await execute_tool(cfg, arguments, "delete NodeBalancer config node", _call)


def create_linode_nodebalancer_config_update_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_update tool."""
    return Tool(
        name="linode_nodebalancer_config_update",
        description=(
            "Updates an existing NodeBalancer config. "
            "Requires confirm because live routing may be affected."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigUpdateInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_config_update(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_update tool request."""
    if is_dry_run(arguments):
        ids = _nb_config_ids(arguments)
        if isinstance(ids, list):
            return ids
        nb, config = ids

        async def _fetch(client: RetryableClient) -> Any:
            return await client.get_nodebalancer_config(nb, config)

        return await execute_dry_run(
            cfg,
            arguments,
            "linode_nodebalancer_config_update",
            "PUT",
            f"/nodebalancers/{nb}/configs/{config}",
            _fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This updates a NodeBalancer config. Set confirm=true to proceed."
        )

    ids = _nb_config_ids(arguments)
    if isinstance(ids, list):
        return ids
    nodebalancer_id, config_id = ids

    body_error = _config_body_error(arguments, require_port=False, require_field=True)
    if body_error is not None:
        return error_response(body_error)

    fields: dict[str, Any] = {
        key: arguments[key]
        for key in NODE_CONFIG_UPDATE_FIELDS
        if arguments.get(key) is not None
    }

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.update_nodebalancer_config(
            nodebalancer_id, config_id, fields
        )
        return serialize_api_response(
            {
                "message": (
                    f"NodeBalancer config {result.get('id')} updated successfully "
                    f"for NodeBalancer {nodebalancer_id}"
                ),
                "config": result,
            },
            nodebalancer_config_pb2.NodeBalancerConfigWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "update NodeBalancer config", _call)


# The config choice enums, sourced from the generated proto enums so the value
# sets match the live API and the Go side exactly (no hand-maintained lists).
# Order matches Go's validation order in nodeBalancerConfig*RequestFromTool for
# identical error messages when more than one field is invalid.
_CONFIG_CHOICE_ENUMS = (
    ("protocol", nodebalancer_config_pb2.NodeBalancerProtocol.Value),
    ("algorithm", nodebalancer_config_pb2.NodeBalancerAlgorithm.Value),
    ("stickiness", nodebalancer_config_pb2.NodeBalancerStickiness.Value),
    ("check", nodebalancer_config_pb2.NodeBalancerCheck.Value),
    ("cipher_suite", nodebalancer_config_pb2.NodeBalancerCipherSuite.Value),
    ("proxy_protocol", nodebalancer_config_pb2.NodeBalancerProxyProtocol.Value),
)
NODE_CONFIG_UPDATE_FIELDS = (
    "port",
    "protocol",
    "algorithm",
    "check",
    "check_passive",
    "check_attempts",
    "check_body",
    "check_interval",
    "check_path",
    "check_timeout",
    "stickiness",
    "proxy_protocol",
    "cipher_suite",
    "ssl_cert",
    "ssl_key",
    "udp_check_port",
)


def _config_body_error(
    arguments: dict[str, Any], *, require_port: bool, require_field: bool = False
) -> str | None:
    """Port of Go's shared NodeBalancer config body validation (create + update).

    Mirrors the checks Go performs in nodeBalancerConfig{Create,Update}RequestFromTool:
    port-required (create only), port range 1-65535, the protocol/algorithm/
    stickiness/check/cipher_suite/proxy_protocol choice enums, the
    ssl-cert/key-when-https requirement, and update's at-least-one-field rule
    (require_field). The choice-enum value sets come from the generated proto
    enums (the same source the JSON Schema and the Go handler use), so they match
    the live API exactly. This replaces an earlier state where only `check` was
    validated because Go's hand-maintained protocol/algorithm/stickiness
    allowlists were stale (rejecting udp, ring_hash, source_ip, session); the
    proto enums carry the full, correct sets on both sides now. The
    at-least-one check runs last to match Go's order. Returns Go's exact text.
    """
    if require_port and "port" not in arguments:
        return "port is required"
    if "port" in arguments:
        try:
            pagination_int_argument(arguments, "port", 1, 65535)
        except (TypeError, ValueError) as exc:
            return str(exc)
    for key, enum in _CONFIG_CHOICE_ENUMS:
        enum_error = optional_enum_error(arguments, key, enum)
        if enum_error is not None:
            return enum_error
    if arguments.get("protocol") == "https" and (
        not arguments.get("ssl_cert") or not arguments.get("ssl_key")
    ):
        return "ssl_cert and ssl_key are required when protocol is https"
    if require_field and not any(
        arguments.get(key) is not None for key in NODE_CONFIG_UPDATE_FIELDS
    ):
        return "at least one update field is required"
    return None


def create_linode_nodebalancer_config_create_tool() -> tuple[Tool, Capability]:
    """Create the linode_nodebalancer_config_create tool."""
    return Tool(
        name="linode_nodebalancer_config_create",
        description=(
            "Creates a NodeBalancer configuration. "
            "WARNING: This creates a new config on an existing NodeBalancer."
        ),
        inputSchema=schema("linode.mcp.v1.NodeBalancerConfigCreateInput"),
    ), Capability.Write


async def handle_linode_nodebalancer_config_create(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Handle linode_nodebalancer_config_create tool request."""
    if is_dry_run(arguments):
        nb_id, error = required_int_id(arguments, "nodebalancer_id")
        if nb_id is None:
            return error_response(error)
        return build_dry_run_response(
            "linode_nodebalancer_config_create",
            arguments.get("environment", ""),
            "POST",
            f"/nodebalancers/{nb_id}/configs",
            None,
        )

    if arguments.get("confirm") is not True:
        return error_response(
            "This creates a NodeBalancer config. Set confirm=true to proceed."
        )

    nodebalancer_id, error = required_int_id(arguments, "nodebalancer_id")
    if nodebalancer_id is None:
        return error_response(error)

    body_error = _config_body_error(arguments, require_port=True)
    if body_error is not None:
        return error_response(body_error)

    fields: dict[str, Any] = {}
    for key in (
        "port",
        "protocol",
        "algorithm",
        "stickiness",
        "check",
        "check_interval",
        "check_timeout",
        "check_attempts",
        "check_path",
        "check_body",
        "check_passive",
        "proxy_protocol",
        "udp_check_port",
        "cipher_suite",
        "ssl_cert",
        "ssl_key",
        "nodes",
    ):
        value = arguments.get(key)
        if value is not None:
            fields[key] = value

    async def _call(client: RetryableClient) -> dict[str, Any]:
        result = await client.create_nodebalancer_config(nodebalancer_id, fields)
        return serialize_api_response(
            {
                "message": (
                    f"NodeBalancer config {result.get('id')} created successfully "
                    f"for NodeBalancer {nodebalancer_id}"
                ),
                "config": result,
            },
            nodebalancer_config_pb2.NodeBalancerConfigWriteResponse(),
        )

    return await execute_tool(cfg, arguments, "create NodeBalancer config", _call)
