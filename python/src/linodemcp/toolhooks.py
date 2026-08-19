"""Per-tool logic the proto contract cannot express.

A tool declares the KINDS it needs through the ``tool_hooks`` option on its
*Input message, and the generated handler calls a function named from the tool
and the kind. A declared kind with no implementation here fails
the generated module's import, which ``test_toolhooks.py`` reaches before the
first tool call, and the reverse, a function nothing declares, fails
``test_toolhooks.py::test_contract_names_every_implemented_hook``.

A normalize hook rewrites the arguments in place before anything reads them,
which is where a family's own reading of a value lives. It runs ahead of
validate so a check sees the value the request will actually carry, and it
rewrites the dict it was given rather than answering a new one so Go's hook for
the same tool can do the same thing to the same request.

A validate hook checks the arguments before anything else runs and answers with
the message the tool reports, or ``""`` to continue. The generated handler wraps
that message in ``error_response``, so a bad argument reads as a result the model
can correct rather than a transport failure.

A preview hook answers a mutating tool's dry run. It receives the call the tool
would have made, resolved from the contract, and the body the generated builder
assembled, so it never re-spells a route or rebuilds a body: what it owns is the
side-effect prose, and for an update the fetch that prose is diffed against.

A fetch_state hook reads the resource a destroy would remove, and every destroy
declares one: the shape a preview reports and a two-stage plan hashes is what a
family's own typed client method answers with, and no descriptor says which
method that is. A dependency_walk hook says what else the removal takes with
it, reading the state fetch_state already read rather than fetching again.

An execute hook makes the live call itself, for a route whose request is not
the JSON body every generated mutation sends: the support-ticket attachment
posts multipart/form-data built from a local file. It is the one kind that
replaces the call rather than something around it, and the dry-run branch
never reaches it.

An answer hook produces a meta tool's whole result from local state: the audit
stores on disk, the profile builder's registry. It takes ``(arguments, cfg)``
and returns the ``list[TextContent]`` the handler answers with, because nothing
derived from the contract stands between it and the caller. A meta tool
declaring one declares no success_message, since the two would be separate
sources for one answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast

import httpx
from mcp.types import TextContent

from linodemcp import objectdata
from linodemcp.config import ObjectStorageConfig
from linodemcp.genpb.linode.mcp.v1 import (
    firewall_pb2,
)
from linodemcp.linode import (
    APIError,
    NetworkError,
    instance_preview_state,
)
from linodemcp.tools.helpers import (
    WALK_PAGE_SIZE,
    build_dry_run_response,
    execute_dry_run,
    preview_state_str,
    required_int_id,
    walk_page_items,
)
from linodemcp.tools.linode_account import (
    oauth_client_thumbnail_png,
)
from linodemcp.tools.linode_audit_export import audit_export_result
from linodemcp.tools.linode_audit_health import audit_health_result
from linodemcp.tools.linode_audit_recent import audit_recent_result
from linodemcp.tools.linode_audit_report import audit_report_result
from linodemcp.tools.linode_audit_summary import audit_summary_result
from linodemcp.tools.linode_profile_builder import (
    profile_list_categories_result,
    profile_list_tools_result,
)
from linodemcp.tools.linode_profile_can_run import profile_can_run_result
from linodemcp.tools.linode_profile_draft import (
    profile_draft_discard_result,
    profile_draft_new_result,
    profile_draft_show_result,
)
from linodemcp.tools.linode_profile_draft_mutate import (
    profile_draft_add_tools_result,
    profile_draft_remove_tools_result,
    profile_draft_set_result,
)
from linodemcp.tools.linode_profile_draft_save import profile_draft_save_result
from linodemcp.tools.proto_response import serialize_api_response
from linodemcp.tools.version import version_response_dict

if TYPE_CHECKING:
    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient
    from linodemcp.tools.declared_state import DeclaredState
    from linodemcp.tools.helpers import DryRunDetails


# The status a Linode reports while it is up, which is what turns a reboot into
# downtime worth warning about.
_INSTANCE_STATUS_RUNNING = "running"

# The only upload mode Tier A answers with: a file over the ceiling is refused
# rather than split, because multipart is not built yet.
_UPLOAD_MODE_SINGLE = "single"

# The largest page the VLAN list endpoint serves. VLANs have no single-resource
# GET, so a lookup that took the API default page would read a VLAN past
# position 100 as not found.
_MAX_VLAN_PAGE_SIZE = 500


# The boot-helper toggles the config routes publish. Go holds helpers to the
# same members through its typed shape.
_DEVICE_DISK_ID = "disk_id"
_DEVICE_VOLUME_ID = "volume_id"
_PLACEMENT_GROUP_LINODES_ERROR = (
    "linodes must be a non-empty array of positive integers"
)


async def linode_domain_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Fetch the zone as it stands and describe what the update would change.

    The request body is deliberately left out: this family's preview predates
    the body echo, and its shape is what clients read today.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_domain(int(arguments.get("domain_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _domain_update_side_effects(
            state,
            arguments.get("domain"),
            arguments.get("soa_email"),
            arguments.get("description"),
        )

    return await execute_dry_run(
        cfg, arguments, "linode_domain_update", method, path, fetch, walk
    )


def _domain_update_side_effects(
    state: Any, new_domain: Any, new_soa: Any, new_description: Any
) -> DryRunDetails:
    """Tier B dry-run walk for a zone update, diffed against the fetched state."""
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


async def linode_domain_delete_dependency_walk(
    client: RetryableClient, domain_id: int, _state: Any
) -> DryRunDetails:
    """Name what a zone delete takes with it.

    Every record in the zone goes, so the NS records (the delegation that
    breaks, which is the part a caller cannot put back from memory) are
    reported one by one and the rest are counted in a warning.

    A failed record list is a warning rather than an error: the delete is still
    previewable without the record picture, and refusing the preview would
    leave a caller with nothing to decide on.
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


async def linode_domain_record_create_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any],
) -> list[TextContent]:
    """Name the record the call would add to the zone.

    A create has no existing record to read, so it reports the request alone.
    """
    domain_id = int(arguments.get("domain_id", 0))
    record_type = arguments.get("type", "")
    name = arguments.get("name", "")
    target = arguments.get("target", "")

    effect = f"A new {record_type} record will be created in domain {domain_id}"
    if name:
        effect += f" for host {name!r}"
    if target:
        effect += f" targeting {target!r}"

    return build_dry_run_response(
        "linode_domain_record_create",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=body,
        side_effects=[effect + "."],
    )


async def linode_domain_record_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the record as it stands and report what the update changes."""
    domain_id = int(arguments.get("domain_id", 0))
    record_id = int(arguments.get("record_id", 0))

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_domain_record(domain_id, record_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _domain_record_update_side_effects(
            state, arguments.get("name", ""), arguments.get("target", "")
        )

    return await execute_dry_run(
        cfg, arguments, "linode_domain_record_update", method, path, fetch, walk
    )


def _domain_record_update_side_effects(
    state: Any, new_name: Any, new_target: Any
) -> DryRunDetails:
    """Tier B walk for a record update, diffed against the fetched state so the
    preview says what changes rather than what was asked for.
    """
    side_effects: list[str] = []
    from_name = getattr(state, "name", "")
    from_target = getattr(state, "target", "")
    if new_name and new_name != from_name:
        side_effects.append(f"Record name changes from {from_name!r} to {new_name!r}.")
    if new_target and new_target != from_target:
        side_effects.append(
            f"Record target changes from {from_target!r} to {new_target!r}."
        )
    return {"side_effects": side_effects} if side_effects else {}


async def linode_stackscript_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any],
) -> list[TextContent]:
    """Read the StackScript as it stands and report what the update changes."""
    stackscript_id = int(arguments.get("stackscript_id", 0))

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_stackscript(stackscript_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _stackscript_update_side_effects(
            state,
            arguments.get("label", ""),
            arguments.get("script", ""),
            arguments.get("description", ""),
        )

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_stackscript_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _stackscript_update_side_effects(
    state: Any, new_label: Any, new_script: Any, new_description: Any
) -> DryRunDetails:
    """Tier B walk for a StackScript update, diffed against the fetched state so
    the preview says what changes rather than what was asked for.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f"Label changes from {from_label!r} to {new_label!r}.")
        else:
            side_effects.append(f"Label is set to {new_label!r}.")
    if new_script:
        side_effects.append("The StackScript body is replaced.")
    if new_description:
        side_effects.append("The StackScript description is updated.")
    return {"side_effects": side_effects} if side_effects else {}


def linode_placement_group_create_normalize(arguments: dict[str, Any]) -> None:
    """Trim the label, the one create argument this language sends trimmed.

    Go trims the region, type, and policy too, so leaving them alone here is
    what keeps each language's bytes on the wire the ones it has always sent.
    """
    label = arguments.get("label")
    if isinstance(label, str):
        arguments["label"] = label.strip()


async def linode_placement_group_unassign_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the group so the preview reports it as current_state, and name each
    Linode the call takes out of it.
    """
    return await _placement_group_membership_preview(
        cfg, arguments, "linode_placement_group_unassign", method, path, "removed from"
    )


async def linode_placement_group_assign_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the group so the preview reports it as current_state, and name each
    Linode the call puts into it.
    """
    return await _placement_group_membership_preview(
        cfg, arguments, "linode_placement_group_assign", method, path, "assigned to"
    )


async def _placement_group_membership_preview(
    cfg: Config,
    arguments: dict[str, Any],
    tool: str,
    method: str,
    path: str,
    verb: str,
) -> list[TextContent]:
    """Preview a membership change, naming each Linode it moves.

    The verb is what separates the two routes: one adds the Linodes, the other
    takes them out.
    """
    group_id = int(cast("int", arguments.get("group_id", 0)))
    linodes = cast("list[int]", arguments.get("linodes", []))

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_placement_group(group_id)

    async def walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
        return {
            "side_effects": [
                f"Linode {linode_id} will be {verb} placement group {group_id}."
                for linode_id in linodes
            ]
        }

    return await execute_dry_run(cfg, arguments, tool, method, path, fetch, walk)


async def linode_lke_cluster_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the cluster so the label and Kubernetes version a change starts
    from can be named; the request carries only the new values.
    """
    cluster_id = required_int_id(arguments, "cluster_id")[0] or 0
    new_label = arguments.get("label")
    new_k8s_version = arguments.get("k8s_version")

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_lke_cluster(cluster_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _lke_cluster_update_effects(state, new_label, new_k8s_version)

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_lke_cluster_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _lke_cluster_update_effects(
    state: Any, new_label: Any, new_k8s_version: Any
) -> DryRunDetails:
    """Name the label change and the Kubernetes version change, which upgrades
    the control plane and every node.

    The sentences are quoted the way Go's %q writes them so both languages
    answer one preview.
    """
    cluster = cast("dict[str, Any]", state) if isinstance(state, dict) else {}
    side_effects: list[str] = []
    if new_label:
        from_label = cluster.get("label", "")
        if from_label and from_label != new_label:
            side_effects.append(f'Label changes from "{from_label}" to "{new_label}".')
        else:
            side_effects.append(f'Label is set to "{new_label}".')
    if new_k8s_version:
        from_version = cluster.get("k8s_version", "")
        if new_k8s_version != from_version:
            side_effects.append(
                f'Kubernetes version changes from "{from_version}" to '
                f'"{new_k8s_version}"; the control plane and nodes upgrade.'
            )
    return {"side_effects": side_effects} if side_effects else {}


async def linode_lke_pool_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the node pool so a caller sees the count the resize starts from.

    That count is the one number the arguments cannot supply.
    """
    cluster_id = int(arguments.get("cluster_id", 0))
    pool_id = int(arguments.get("pool_id", 0))
    count_supplied = "count" in arguments
    autoscaler_supplied = "autoscaler" in arguments
    new_count = _lke_pool_count(arguments.get("count"))

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_lke_node_pool(cluster_id, pool_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _lke_pool_update_effects(
            state,
            new_count,
            count_supplied=count_supplied,
            autoscaler_supplied=autoscaler_supplied,
        )

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_lke_pool_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


async def linode_lke_cluster_delete_dependency_walk(
    client: RetryableClient, cluster_id: int, _state: DeclaredState
) -> DryRunDetails:
    """Name what a cluster delete takes with it.

    Every node pool cascades, so each is reported one by one and the nodes they
    carry are counted in a warning, since the running workloads are the part a
    caller cannot put back.

    A failed pool list is a warning rather than an error: the delete is still
    previewable without the pool picture, and refusing the preview would leave
    a caller with nothing to decide on.
    """
    details: DryRunDetails = {}
    try:
        pools = await client.list_lke_node_pools(cluster_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list node pools: {exc}"]
        return details

    dependencies: list[dict[str, Any]] = []
    total_nodes = 0
    for pool in pools:
        count = int(pool.get("count", 0))
        total_nodes += count
        dependencies.append(
            {
                "kind": "node_pool",
                "id": pool.get("id"),
                "action": "cascade_deleted",
                "note": f"{count} node(s) of type {pool.get('type', '')}",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
    if total_nodes > 0:
        details["warnings"] = [
            (
                f"Deleting this cluster destroys {len(pools)} node pool(s) "
                f"and {total_nodes} node(s); running workloads are lost."
            )
        ]
    return details


def _lke_pool_count(raw: Any) -> int:
    """The node count a sentence names, 0 for anything the body sends as 0."""
    if isinstance(raw, bool) or not isinstance(raw, (int, float)):
        return 0
    return int(raw)


def _lke_pool_update_effects(
    state: Any,
    new_count: int,
    *,
    count_supplied: bool,
    autoscaler_supplied: bool,
) -> DryRunDetails:
    """Name the count change and the autoscaler change a pool update performs.

    A fetched count of zero is left out of the resize sentence: the read that
    failed and the pool that really holds no nodes are the same value here, so
    the sentence that names a starting count is reserved for a count that was
    read.
    """
    side_effects: list[str] = []

    if count_supplied:
        pool = cast("dict[str, Any]", state) if isinstance(state, dict) else {}
        from_count = _lke_pool_count(pool.get("count"))
        effect = f"Node pool is set to {new_count} node(s)."
        if from_count and from_count != new_count:
            effect = f"Node pool resizes from {from_count} to {new_count} node(s)."
        side_effects.append(effect)

    if autoscaler_supplied:
        side_effects.append("The pool autoscaler configuration is updated.")

    return {"side_effects": side_effects} if side_effects else {}


async def linode_image_create_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Name the disk the capture reads and the label it lands under."""
    disk_id = arguments.get("disk_id")
    label = arguments.get("label")
    effect = f"A new image will be captured from disk {disk_id}"
    if label:
        # Double quotes so the prose spells the label the echoed body spells it.
        effect += f' and labeled "{label}"'

    return build_dry_run_response(
        "linode_image_create",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=body,
        side_effects=[f"{effect}."],
    )


def linode_image_replicate_normalize(arguments: dict[str, Any]) -> None:
    """Trim each region slug so the slug rule reads the value that is sent.

    This narrows Python nowhere and widens Go, which used to refuse a padded
    slug outright.
    """
    regions = arguments.get("regions")
    if not isinstance(regions, list):
        return

    entries = cast("list[object]", regions)
    arguments["regions"] = [
        entry.strip() if isinstance(entry, str) else entry for entry in entries
    ]


async def linode_image_replicate_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the image so a caller sees what is being replicated.

    Each language carried one half of this before, the read or the sentence.
    """
    image_id = str(arguments.get("image_id", ""))
    slugs = ", ".join(_replication_regions(arguments))

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_image(image_id)

    async def walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
        return {"side_effects": [f"Image '{image_id}' will be replicated to {slugs}."]}

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_image_replicate",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _replication_regions(arguments: dict[str, Any]) -> list[str]:
    """Render the slugs the replication sentence lists.

    Entries the rules already refused cannot reach here, so anything non-text
    is dropped rather than spelled out in Python's own repr of it.
    """
    regions = arguments.get("regions")
    if not isinstance(regions, list):
        return []

    entries = cast("list[object]", regions)

    return [entry for entry in entries if isinstance(entry, str)]


async def linode_nodebalancer_config_create_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the NodeBalancer's existing configs, so a caller sees which ports
    are already taken before adding one.

    Go carried the fetch and Python carried none; the fetch is the union.
    """
    nodebalancer_id = int(arguments.get("nodebalancer_id", 0) or 0)

    async def fetch(client: RetryableClient) -> Any:
        page = await client.list_nodebalancer_configs(nodebalancer_id)
        return page.get("data", [])

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_nodebalancer_config_create",
        method,
        path,
        fetch,
        request_body=body,
    )


async def linode_nodebalancer_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the NodeBalancer so the label a change starts from can be named;
    the request carries only the new values.
    """
    nodebalancer_id = int(arguments.get("nodebalancer_id", 0) or 0)
    new_label = arguments.get("label")
    new_throttle = arguments.get("client_conn_throttle")

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_nodebalancer(nodebalancer_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _nodebalancer_update_effects(state, new_label, new_throttle)

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_nodebalancer_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _nodebalancer_update_effects(
    state: Any, new_label: Any, new_throttle: Any
) -> DryRunDetails:
    """Name the label change and the throttle the update sets, read against the
    NodeBalancer as it stands.

    The label is quoted the way Go's %q writes it so both languages answer one
    preview.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f'Label changes from "{from_label}" to "{new_label}".')
        else:
            side_effects.append(f'Label is set to "{new_label}".')
    if new_throttle is not None:
        side_effects.append(
            f"Connection throttle is set to {new_throttle} connections per "
            "second per client IP."
        )
    return {"side_effects": side_effects} if side_effects else {}


async def linode_vpc_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the VPC so the label a change starts from can be named; the request
    carries only the new values.
    """
    vpc_id = required_int_id(arguments, "vpc_id")[0] or 0
    new_label = arguments.get("label")
    new_description = arguments.get("description")

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_vpc(vpc_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _vpc_update_effects(state, new_label, new_description)

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_vpc_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _vpc_update_effects(
    state: Any, new_label: Any, new_description: Any
) -> DryRunDetails:
    """Name the label change and, when a new description travels, that the
    description is replaced.

    The sentences are quoted the way Go's %q writes them so both languages
    answer one preview.
    """
    vpc = cast("dict[str, Any]", state) if isinstance(state, dict) else {}
    side_effects: list[str] = []
    if new_label:
        from_label = vpc.get("label", "")
        if from_label and from_label != new_label:
            side_effects.append(f'Label changes from "{from_label}" to "{new_label}".')
        else:
            side_effects.append(f'Label is set to "{new_label}".')
    if new_description:
        side_effects.append("The VPC description is updated.")
    return {"side_effects": side_effects} if side_effects else {}


def _requested_page(arguments: dict[str, Any]) -> tuple[int | None, int | None]:
    """The page controls a replacement's preview fetches its state under.

    The generated handler has already refused an out-of-range control by the
    time a preview hook runs, so the values are read straight through.
    """
    page = arguments.get("page")
    page_size = arguments.get("page_size")

    return (
        page if isinstance(page, int) else None,
        page_size if isinstance(page_size, int) else None,
    )


def _firewall_state(raw: Any) -> list[dict[str, Any]]:
    """Normalize a fetched page of assignments through the Firewall descriptor.

    The state a replacement previews reads the way the elements in its own
    answer do rather than the way the API happened to spell them, which is what
    lets the other language build the same array from the same descriptor.
    """
    if not isinstance(raw, dict):
        return []

    data: object = cast("dict[str, Any]", raw).get("data")
    if not isinstance(data, list):
        return []

    return [
        serialize_api_response(cast("dict[str, Any]", item), firewall_pb2.Firewall())
        for item in cast("list[object]", data)
        if isinstance(item, dict)
    ]


async def linode_instance_firewall_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the assignments the replacement overwrites, on the call's own page.

    Go read them and said nothing; Python said the sentence and read nothing.
    """
    linode_id = required_int_id(arguments, "linode_id")[0] or 0
    page, page_size = _requested_page(arguments)

    async def fetch(client: RetryableClient) -> Any:
        return _firewall_state(
            await client.list_instance_firewalls(
                linode_id, page=page, page_size=page_size
            )
        )

    async def walk(_client: RetryableClient, _state: Any) -> DryRunDetails:
        return {
            "side_effects": [
                f"Firewall assignments for Linode {linode_id} will be replaced."
            ]
        }

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_firewall_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


async def linode_nodebalancer_firewall_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the assignments the replacement overwrites.

    Go read them off the collection's first page whatever the caller asked for,
    and Python read the NodeBalancer instead of its firewalls.
    """
    nodebalancer_id = required_int_id(arguments, "nodebalancer_id")[0] or 0
    page, page_size = _requested_page(arguments)

    async def fetch(client: RetryableClient) -> Any:
        return _firewall_state(
            await client.list_nodebalancer_firewalls(
                nodebalancer_id, page=page, page_size=page_size
            )
        )

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_nodebalancer_firewall_update",
        method,
        path,
        fetch,
        request_body=body,
    )


async def linode_firewall_create_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Name the firewall being created and the policies it starts under.

    The label is quoted the way Go's %q writes it so both languages answer one
    preview, and the policies fall back to what the body folds in.
    """
    label = arguments.get("label", "")
    inbound_policy = arguments.get("inbound_policy", _DEFAULT_FIREWALL_POLICY)
    outbound_policy = arguments.get("outbound_policy", _DEFAULT_FIREWALL_POLICY)
    sentence = (
        f'A new Cloud Firewall "{label}" will be created with inbound '
        f"policy {inbound_policy} and outbound policy {outbound_policy}."
    )

    return build_dry_run_response(
        "linode_firewall_create",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=body,
        side_effects=[sentence],
    )


# The policy a firewall carries when the caller names none, which is the value
# the create body folds into its rules object.
_DEFAULT_FIREWALL_POLICY = "ACCEPT"


# Stands in for a monthly change the preview cannot estimate, which reserved
# IPv4 pricing cannot be without a region price.
_RESERVED_IP_BILLING_UNKNOWN = "unknown"


async def linode_networking_reserved_ip_create_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any],
) -> list[TextContent]:
    """Report the reservation a create would make.

    It reads no state, since the address does not exist yet, and carries the
    billing a reservation starts. The wire contract is
    DryRunBillingDelta{monthly_change_usd, note}, so a differently shaped dict
    is dropped by the proto and leaves an empty delta behind.
    """
    return build_dry_run_response(
        "linode_networking_reserved_ip_create",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=body,
        billing_delta={
            "monthly_change_usd": _RESERVED_IP_BILLING_UNKNOWN,
            "note": (
                "Reserved IPv4 pricing varies by region; "
                "see linode_networking_reserved_ip_type_list."
            ),
        },
        warnings=["Reserved IP billing begins when the address is created."],
    )


def linode_object_storage_presigned_url_create_normalize(
    arguments: dict[str, Any],
) -> None:
    """Fold the method to the canonical S3 verb.

    A caller sending "get" reaches the enum rule as GET.
    """
    method = arguments.get("method")
    if isinstance(method, str):
        arguments["method"] = method.upper()


async def linode_object_storage_key_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the key so a caller sees the label the update replaces.

    The read is credential-safe: the key GET never carries the secret.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_object_storage_key(int(arguments.get("key_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        side_effects: list[str] = []
        new_label = arguments.get("label", "")
        if new_label:
            from_label = ""
            if isinstance(state, dict):
                from_label = cast("dict[str, Any]", state).get("label", "")
            if from_label and from_label != new_label:
                side_effects.append(
                    f'Label changes from "{from_label}" to "{new_label}".'
                )
            else:
                side_effects.append(f'Label is set to "{new_label}".')
        if "bucket_access" in arguments:
            side_effects.append("The key's bucket access scopes are replaced.")
        return {"side_effects": side_effects} if side_effects else {}

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_object_storage_key_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


async def linode_volume_detach_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the volume and say which instance it comes off.

    The attachment is the part a caller cannot see from the request, since the
    call names only the volume.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_volume(int(arguments.get("volume_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _volume_detach_side_effects(state)

    return await execute_dry_run(
        cfg, arguments, "linode_volume_detach", method, path, fetch, walk
    )


def _volume_detach_side_effects(state: Any) -> DryRunDetails:
    """Tier B walk for a detach, read off the fetched volume.

    A volume attached to nothing detaches to no effect, which is worth saying
    before the call rather than after.
    """
    attached = getattr(state, "linode_id", None)
    if not attached:
        return {
            "side_effects": [
                "Volume is not attached to any instance; detach is a no-op."
            ]
        }
    volume_id = getattr(state, "id", 0)
    return {
        "side_effects": [
            (
                f"Volume {volume_id} detaches from instance {attached}; its data"
                " is preserved and billing continues."
            )
        ]
    }


async def linode_account_oauth_client_thumbnail_get_execute(
    client: RetryableClient,
    values: tuple[object, ...],
) -> dict[str, Any]:
    """Fetch the image and answer with it as text.

    The route sends raw PNG bytes under image/png, so there is no JSON body for
    a generated read to decode. The client method that reads those bytes already
    base64-encodes them, which is the value the response member carries.

    Only the members the contract declares assembled belong here; the client_id
    beside them is the driver's, from the call.
    """
    raw = await client.get_account_oauth_client_thumbnail(str(values[0]))

    return {"thumbnail_png_base64": raw["thumbnail_png_base64"]}


async def linode_account_oauth_client_thumbnail_update_execute(
    client: RetryableClient,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    _body: dict[str, Any] | None,
) -> None:
    """Upload the decoded image.

    The route consumes the raw PNG bytes under image/png, so the JSON body the
    generated handler built is not the request. The id is read off the resolved
    path values, and the bytes come from the same decode the validate hook has
    already accepted.
    """
    thumbnail_png, _ = oauth_client_thumbnail_png(arguments)

    await client.update_account_oauth_client_thumbnail(
        str(values[0]), thumbnail_png or b""
    )


def linode_account_payment_create_normalize(arguments: dict[str, Any]) -> None:
    """Trim the amount, which is the value this tool has always put on the wire.

    A null payment method is dropped rather than trimmed: this tool has always
    read it as "charge the default method", and the body builder would send it
    as zero.
    """
    usd = arguments.get("usd")
    if isinstance(usd, str):
        arguments["usd"] = usd.strip()

    if "payment_method_id" in arguments and arguments["payment_method_id"] is None:
        del arguments["payment_method_id"]


async def linode_account_user_create_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Name the user being added and the restriction it starts with."""
    username = arguments.get("username", "")
    restricted = "true" if arguments.get("restricted") is True else "false"
    # The booleans and quotes render the way the echoed body renders them, so
    # the prose and the request a caller reads together agree.
    sentence = (
        f'A new account user "{username}" will be created with restricted={restricted}.'
    )

    return build_dry_run_response(
        "linode_account_user_create",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=body,
        side_effects=[sentence],
    )


async def linode_sshkey_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the key so the preview can name the label being replaced.

    The request body is left out: this family's preview predates the body echo,
    and its shape is what clients read today.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_ssh_key(int(arguments.get("ssh_key_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _label_change_details(state, arguments.get("label"))

    return await execute_dry_run(
        cfg, arguments, "linode_sshkey_update", method, path, fetch, walk
    )


def _label_change_details(state: Any, new_label: Any) -> DryRunDetails:
    """The "label changes from X to Y" prose an update reports, or nothing.

    A label the fetched state does not carry reads as a label being set rather
    than replaced, which is the same sentence the other language answers with.
    """
    if not new_label:
        return {}
    from_label = getattr(state, "label", "")
    if from_label and from_label != new_label:
        return {
            "side_effects": [f"Label changes from {from_label!r} to {new_label!r}."]
        }
    return {"side_effects": [f"Label is set to {new_label!r}."]}


async def linode_volume_clone_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the source volume so the preview can name what the copy is made
    from, which the request carries only as an id.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_volume(int(arguments.get("volume_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _volume_clone_details(state, arguments.get("label", ""))

    return await execute_dry_run(
        cfg, arguments, "linode_volume_clone", method, path, fetch, walk
    )


def _volume_clone_details(state: Any, label: Any) -> DryRunDetails:
    """The prose a clone preview reports.

    A state the fetch could not name still describes the copy, since the label
    the caller asked for is the part they are about to be billed for.
    """
    warnings = ["Billing for the cloned volume starts immediately on creation."]
    volume_id = getattr(state, "id", None)
    if not volume_id:
        return {
            "side_effects": [
                (
                    f"A new volume labeled {label!r} will be created from the"
                    " source volume."
                )
            ],
            "warnings": warnings,
        }
    from_label = getattr(state, "label", "")
    return {
        "side_effects": [
            (
                f"Volume {volume_id} ({from_label!r}) will be cloned to a new"
                f" volume labeled {label!r}."
            )
        ],
        "warnings": warnings,
    }


async def linode_volume_resize_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the volume so the preview can name the size the resize starts from,
    which is what says whether it grows at all.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_volume(int(arguments.get("volume_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _volume_resize_details(state, int(arguments.get("size", 0)))

    return await execute_dry_run(
        cfg, arguments, "linode_volume_resize", method, path, fetch, walk
    )


def _volume_resize_details(state: Any, target_size: int) -> DryRunDetails:
    """The prose a resize preview reports, read against the size the volume
    carries now.
    """
    from_size = getattr(state, "size", 0)
    if from_size:
        effect = f"Volume resizes from {from_size} GB to {target_size} GB."
    else:
        effect = f"Volume resizes to {target_size} GB."
    return {
        "side_effects": [effect],
        "warnings": [
            "A volume can only grow; the new size must be larger than the current size."
        ],
    }


async def linode_volume_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    _body: dict[str, Any],
) -> list[TextContent]:
    """Read the volume so the preview can name the label being replaced rather
    than only the one asked for.
    """

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_volume(int(arguments.get("volume_id", 0)))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        details = _label_change_details(state, arguments.get("label"))
        if arguments.get("tags") is None:
            return details
        effects = [*details.get("side_effects", [])]
        effects.append("The volume's tag set is replaced with the provided tags.")
        return {**details, "side_effects": effects}

    return await execute_dry_run(
        cfg, arguments, "linode_volume_update", method, path, fetch, walk
    )


async def linode_volume_delete_dependency_walk(
    _client: RetryableClient, _volume_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name the instance a delete takes the volume off.

    The attachment is already on the fetched state, which carries both the
    instance id and its label, so the walk makes no call of its own.
    """
    linode_id = state.number("linode_id")
    if not linode_id:
        return {}
    return {
        "dependencies": [
            {
                "kind": "instance",
                "id": linode_id,
                "label": state.text("linode_label"),
                "action": "detached",
                "note": (
                    "Volume is attached; it detaches from this instance before"
                    " deletion."
                ),
            }
        ],
        "warnings": [
            (
                "Volume is currently attached to an instance; it will be"
                " detached as part of deletion."
            )
        ],
    }


def linode_account_service_transfer_create_normalize(
    arguments: dict[str, Any],
) -> None:
    """Fold the linode_ids convenience form into entities.

    entities is the only shape either language ever put on the wire. A
    caller-supplied entities object wins outright, since it can name entity
    types linode_ids cannot express. An unusable linode_ids value is left alone
    so validate can name it.
    """
    entities = arguments.get("entities")
    if entities:
        return

    linode_ids, message = _account_service_transfer_linode_ids(arguments)
    if linode_ids is None or message is not None:
        return

    arguments["entities"] = {"linodes": linode_ids}
    del arguments["linode_ids"]


def _account_service_transfer_linode_ids(
    arguments: dict[str, Any],
) -> tuple[list[int] | None, str | None]:
    """Read linode_ids as a list of positive integers, or say why it is not."""
    raw_linode_ids = arguments.get("linode_ids")
    if raw_linode_ids is None:
        return None, "linode_ids is required"
    if not isinstance(raw_linode_ids, list) or not raw_linode_ids:
        return None, "linode_ids must be a non-empty list of positive integers"

    raw_linode_id_list = cast("list[object]", raw_linode_ids)
    linode_ids: list[int] = []
    for raw_linode_id in raw_linode_id_list:
        if (
            isinstance(raw_linode_id, bool)
            or not isinstance(raw_linode_id, int)
            or raw_linode_id < 1
        ):
            return None, "linode_ids must be a non-empty list of positive integers"
        linode_ids.append(raw_linode_id)
    return linode_ids, None


# The Managed Database create and update routes owned their whole argument
# check by hand before, and two of those rules survive nothing else: the
# unsupported-argument rejection, which no derived check performs, and update's
# at-least-one-field rule, which no single field can state. The PostgreSQL
# engine-id form is checked here and nowhere in Go, which is where it has always
# been.


# What stands in for an answer in the preview. The field-wide redaction marker
# would take the question ids with it, and those are the half of the call a
# caller checks before confirming.
_REDACTED_ANSWER = "[redacted]"


async def linode_profile_security_question_answer_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Report the call with every answer replaced and the question ids left alone.

    That is the shape Go's hand-written preview reported and the shape Python
    reported nothing at all in. The reported body is rebuilt rather than passed
    through, because the built body is the one the live call sends.
    """
    answers = cast("list[Any]", body.get("security_questions", [])) if body else []
    redacted: list[dict[str, Any]] = [
        {
            "question_id": cast("dict[str, Any]", answer).get("question_id"),
            "response": _REDACTED_ANSWER,
        }
        for answer in answers
        if isinstance(answer, dict)
    ]
    reported: dict[str, Any] = {"security_questions": redacted}

    return build_dry_run_response(
        "linode_profile_security_question_answer",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=reported,
        side_effects=["The profile's security question answers are saved."],
    )


async def linode_support_ticket_attachment_create_execute(
    client: RetryableClient,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    _body: dict[str, Any] | None,
) -> None:
    """Upload the named file.

    The route consumes multipart/form-data assembled from the file's CONTENTS,
    so the JSON body the generated handler built is not the request. The path is
    read off the call rather than out of that body because the body is not what
    travels: it carries the path so the schema can advertise it and the rules can
    check it, and what reaches the API is the bytes it names.

    The call goes through the plain client rather than the retrying one, which is
    what the tool's declared retry_disabled says and what Go's client method has
    always done.
    """
    await client.client.create_support_ticket_attachment(
        int(cast("int", values[0])), str(arguments.get("file", ""))
    )


# --- meta answers -------------------------------------------------------
#
# Each takes the tier's (arguments, cfg) and answers the result itself: a meta
# tool reaches no route, so nothing derived from the contract stands between
# the local state it reads and the caller. The audit stores are reached through
# the module bridges main installs, which is why cfg goes unread here.


async def version_answer(_arguments: dict[str, Any], _cfg: Config) -> list[TextContent]:
    """Report the build metadata this process carries.

    It serializes the same payload the CLI's version verb does, so the two
    surfaces cannot drift a field apart.
    """
    return [
        TextContent(type="text", text=json.dumps(version_response_dict(), indent=2))
    ]


async def linode_audit_health_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Report the audit subsystem's own status."""
    return audit_health_result(arguments)


async def linode_audit_recent_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Read the most recent audit events, newest first."""
    return audit_recent_result(arguments)


async def linode_audit_summary_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Count audit events bucketed by the requested columns."""
    return audit_summary_result(arguments)


async def linode_audit_export_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Dump a filtered window of audit events to a temp file."""
    return audit_export_result(arguments)


async def linode_audit_report_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Run a named report from the installed catalog."""
    return audit_report_result(arguments)


async def linode_profile_list_tools_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Enumerate every registerable tool, name-sorted, with the call's filters."""
    return profile_list_tools_result(arguments)


async def linode_profile_list_categories_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Reduce the catalog to its category list with per-category tool counts."""
    return profile_list_categories_result(arguments)


async def linode_profile_can_run_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Pre-check a sequence of calls against the active profile."""
    return profile_can_run_result(arguments)


async def linode_profile_draft_new_answer(
    arguments: dict[str, Any], cfg: Config
) -> list[TextContent]:
    """Start a draft, optionally seeded from the profile clone_from names.

    The one builder hook that reads the configuration, because that is where
    the clone source is resolved from.
    """
    return profile_draft_new_result(arguments, cfg)


async def linode_profile_draft_show_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Read one draft's current state."""
    return profile_draft_show_result(arguments)


async def linode_profile_draft_discard_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Remove one draft, answering whether it was there to remove."""
    return profile_draft_discard_result(arguments)


async def linode_profile_draft_add_tools_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Expand the call's patterns against the live catalog and merge them in."""
    return profile_draft_add_tools_result(arguments)


async def linode_profile_draft_remove_tools_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Strip the tools the call's patterns match on the draft's own list."""
    return profile_draft_remove_tools_result(arguments)


async def linode_profile_draft_set_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Set the draft's optional fields and answer which ones changed."""
    return profile_draft_set_result(arguments)


async def linode_profile_draft_save_answer(
    arguments: dict[str, Any], _cfg: Config
) -> list[TextContent]:
    """Merge the draft into the config file and answer the diff.

    The generated handler runs the confirm gate ahead of this.
    """
    return profile_draft_save_result(arguments)


async def linode_instance_create_preview(
    _cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Describe the instance the call would create and the billing it starts,
    neither of which the request body says.
    """
    instance_type = arguments.get("type", "")
    region = arguments.get("region", "")
    effect = f"A new {instance_type} instance will be created in region {region}"

    image = arguments.get("image")
    if image:
        effect += f" from image {image}"

    return build_dry_run_response(
        "linode_instance_create",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=body,
        side_effects=[f"{effect}."],
        warnings=["Billing for the instance starts immediately on creation."],
    )


async def linode_instance_rescue_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the instance the reboot would interrupt. The walk takes the state
    rather than the id because a running Linode goes down for the rescue boot
    and one that is already off does not.
    """
    linode_id = required_int_id(arguments, "linode_id")[0] or 0

    async def fetch(client: RetryableClient) -> Any:
        return instance_preview_state(await client.get_instance(linode_id))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        details: DryRunDetails = {
            "side_effects": [
                (
                    "The instance reboots into rescue mode; its normal boot "
                    "configuration is bypassed until you reboot out of rescue mode."
                )
            ]
        }
        if preview_state_str(state, "status") == _INSTANCE_STATUS_RUNNING:
            details["warnings"] = [
                (
                    "Instance is currently running; entering rescue mode reboots it, "
                    "causing downtime."
                )
            ]
        return details

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_rescue",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _instance_migrate_effect(from_region: str, target_region: str) -> str:
    """Word the move. An omitted region is Linode picking the destination, and a
    read that answered with nothing leaves the origin out rather than naming an
    empty one.
    """
    if not target_region:
        return "Instance migrates; it is unavailable during the migration."
    if not from_region:
        return (
            f"Instance migrates to region {target_region}; it is unavailable "
            "during the migration."
        )
    return (
        f"Instance migrates from region {from_region} to {target_region}; it is "
        "unavailable during the migration."
    )


async def linode_instance_migrate_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the Linode the migration would move, so the region it leaves is
    reported beside the one it is headed for.
    """
    linode_id = required_int_id(arguments, "linode_id")[0] or 0
    target_region = str(arguments.get("region", "") or "")

    async def fetch(client: RetryableClient) -> Any:
        return instance_preview_state(await client.get_instance(linode_id))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return {
            "side_effects": [
                _instance_migrate_effect(
                    preview_state_str(state, "region"), target_region
                )
            ]
        }

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_migrate",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _instance_mutate_effect(from_type: str) -> str:
    """Word the upgrade, naming the type it starts from when the read answered
    with one.
    """
    if not from_type:
        return (
            "Instance upgrades to the latest generation of its type; it reboots "
            "during the upgrade."
        )
    return (
        f"Instance type {from_type} upgrades to the latest generation; it "
        "reboots during the upgrade."
    )


async def linode_instance_mutate_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the Linode whose type the upgrade replaces. Go's preview named the
    type and Python's named the downtime, so this one carries both.
    """
    linode_id = required_int_id(arguments, "linode_id")[0] or 0

    async def fetch(client: RetryableClient) -> Any:
        return instance_preview_state(await client.get_instance(linode_id))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return {
            "side_effects": [_instance_mutate_effect(preview_state_str(state, "type"))],
            "warnings": ["The Linode may be unavailable during the upgrade."],
        }

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_mutate",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _instance_disk_clone_effects(state: Any) -> DryRunDetails:
    """Name the copy a clone leaves behind and the storage it takes, read off
    the disk fetched beside it.
    """
    label = preview_state_str(state, "label")
    raw_size: Any = 0
    if isinstance(state, dict):
        raw_size = cast("dict[str, Any]", state).get("size", 0)
    size = raw_size if isinstance(raw_size, int) else 0

    return {
        "side_effects": [
            (
                f'Disk "{label}" ({size} MB) is cloned to a new disk on the '
                f"same instance, consuming {size} MB of additional storage."
            )
        ]
    }


async def linode_instance_disk_clone_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the disk being copied, since its size is the storage the clone
    consumes.
    """
    linode_id = required_int_id(arguments, "linode_id")[0] or 0
    disk_id = required_int_id(arguments, "disk_id")[0] or 0

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_instance_disk(linode_id, disk_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _instance_disk_clone_effects(state)

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_disk_clone",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _instance_disk_resize_effect(state: Any, target_size: int) -> str:
    """Word the change. A read that answered with nothing leaves the starting
    size out rather than naming a zero.
    """
    raw_size: Any = 0
    if isinstance(state, dict):
        raw_size = cast("dict[str, Any]", state).get("size", 0)
    from_size = raw_size if isinstance(raw_size, int) else 0

    if not from_size:
        return f"Disk resizes to {target_size} MB."

    return f"Disk resizes from {from_size} MB to {target_size} MB."


async def linode_instance_disk_resize_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the disk so the size it grows or shrinks from is reported beside
    the size it is headed for.
    """
    linode_id = required_int_id(arguments, "linode_id")[0] or 0
    disk_id = required_int_id(arguments, "disk_id")[0] or 0
    size = int(cast("int", arguments.get("size", 0)))

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_instance_disk(linode_id, disk_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return {
            "side_effects": [_instance_disk_resize_effect(state, size)],
            "warnings": ["The instance must be powered off to resize a disk."],
        }

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_disk_resize",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _instance_resize_details(from_type: str, target_type: str) -> DryRunDetails:
    """Word the type change and the two costs it carries: the reboot the move
    needs, and the price the new plan bills at.
    """
    if from_type:
        effect = (
            f"Instance resizes from type {from_type} to {target_type}; it "
            "reboots and is unavailable during the resize."
        )
    else:
        effect = (
            f"Instance resizes to type {target_type}; it reboots and is "
            "unavailable during the resize."
        )
    return {
        "side_effects": [effect],
        "warnings": ["Resizing changes the monthly price to match the new type."],
    }


async def linode_instance_resize_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the Linode whose plan the resize replaces, so the type it leaves is
    reported beside the one it is headed for.
    """
    instance_id = required_int_id(arguments, "instance_id")[0] or 0
    target_type = str(arguments.get("type", ""))

    async def fetch(client: RetryableClient) -> Any:
        return instance_preview_state(await client.get_instance(instance_id))

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _instance_resize_details(preview_state_str(state, "type"), target_type)

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_instance_resize",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


async def linode_instance_resize_fetch_state(
    client: RetryableClient, instance_id: int
) -> Any:
    """Build the projection a resize plan hashes: the instance's current type
    plus each disk's id, size, and filesystem.

    It is not the instance the preview reports, because a resize moves the disks
    too and allow_auto_disk_resize resizes them as part of the plan change. The
    projection holds only what a real change would move, so a cosmetic instance
    field cannot refuse an apply and the tool needs no hash-ignore list.
    """
    instance = await client.get_instance(instance_id)
    try:
        disks: list[dict[str, Any]] = await client.list_instance_disks(instance_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        msg = f"list disks for resize plan: {exc}"
        raise ValueError(msg) from exc

    snapshot = [
        {
            "id": disk.get("id"),
            "size": disk.get("size"),
            "filesystem": disk.get("filesystem"),
        }
        for disk in disks
    ]
    return {"type": preview_state_str(instance, "type"), "disks": snapshot}


async def linode_instance_resize_dependency_walk(
    _client: RetryableClient, arguments: dict[str, Any], state: Any
) -> DryRunDetails:
    """Name the plan change over the state the fetch already read, so a plan
    reads like the preview beside it.
    """
    return _instance_resize_details(
        preview_state_str(state, "type"), str(arguments.get("type", ""))
    )


async def _instance_volume_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Volumes attached to the instance detach (not destroy) on delete."""
    try:
        page = await client.list_instance_volumes(instance_id, 1, WALK_PAGE_SIZE)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list attached volumes: {exc}"]

    deps = [
        {
            "kind": "volume",
            "id": volume.get("id"),
            "label": str(volume.get("label", "")),
            "action": "detached",
            "note": f"{volume.get('size', 0)}GB volume stays; billing continues.",
        }
        for volume in walk_page_items(page)
    ]

    return deps, []


async def _instance_ip_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Public IPv4 addresses are released back to the pool on delete."""
    try:
        ips = await client.list_instance_ips(instance_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list IP addresses: {exc}"]

    ipv4 = cast("dict[str, Any]", ips.get("ipv4", {}))
    public = cast("list[dict[str, Any]]", ipv4.get("public", []))
    deps: list[dict[str, Any]] = [
        {
            "kind": "public_ip",
            "label": str(addr.get("address", "")),
            "action": "released",
        }
        for addr in public
    ]

    return deps, []


async def _instance_firewall_deps(
    client: RetryableClient, instance_id: int
) -> tuple[list[dict[str, Any]], list[str]]:
    """Firewalls survive the delete; the instance drops from their devices."""
    try:
        page = await client.list_instance_firewalls(instance_id, 1, WALK_PAGE_SIZE)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return [], [f"Could not list firewalls: {exc}"]

    deps = [
        {
            "kind": "firewall",
            "id": firewall.get("id"),
            "label": str(firewall.get("label", "")),
            "action": "removed",
            "note": "Firewall stays; this instance is removed from its device list.",
        }
        for firewall in walk_page_items(page)
    ]

    return deps, []


async def _instance_billing_delta(
    client: RetryableClient, type_id: str
) -> dict[str, Any]:
    """Best-effort monthly cost change from deleting an instance of type_id.

    An empty type or a failed pricing fetch degrades to the "unknown" sentinel
    instead of failing the preview.
    """
    if not type_id:
        return {"monthly_change_usd": "unknown"}

    try:
        instance_type = await client.get_type(type_id)
    except (APIError, NetworkError, httpx.HTTPError):
        return {
            "monthly_change_usd": "unknown",
            "note": "Could not fetch type pricing for the estimate.",
        }

    return {
        "monthly_change_usd": f"-{instance_type.price.monthly:.2f}",
        "note": "Instance billing stops. Attached volume billing continues.",
    }


async def linode_instance_delete_dependency_walk(
    client: RetryableClient, instance_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name what a delete takes with it.

    Attached volumes detach, public IPv4 addresses release, firewall
    attachments drop, the monthly billing change is estimated, and a running
    instance adds a shutdown warning. Best-effort: a failed sub-fetch becomes a
    warning rather than failing the preview.
    """
    dependencies: list[dict[str, Any]] = []
    warnings: list[str] = []

    for collect in (_instance_volume_deps, _instance_ip_deps, _instance_firewall_deps):
        deps, deps_warnings = await collect(client, instance_id)
        dependencies.extend(deps)
        warnings.extend(deps_warnings)

    billing = await _instance_billing_delta(client, state.text("type"))

    if state.text("status") == _INSTANCE_STATUS_RUNNING:
        warnings.append(
            "Instance is currently running. Delete will not pause for a "
            "graceful shutdown."
        )

    details: DryRunDetails = {"billing_delta": billing}
    if dependencies:
        details["dependencies"] = dependencies
    if warnings:
        details["warnings"] = warnings

    return details


async def linode_firewall_delete_dependency_walk(
    client: RetryableClient, firewall_id: int, _state: Any
) -> DryRunDetails:
    """Name the devices that lose this firewall's rules.

    The Linodes and NodeBalancers attached to a firewall survive the delete but
    stop being protected. Best-effort: a failed device list becomes a warning
    rather than failing the preview.
    """
    details: DryRunDetails = {}
    try:
        response = await client.list_firewall_devices(firewall_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list firewall devices: {exc}"]
        return details

    devices = cast("list[dict[str, Any]]", response.get("data", []))
    dependencies: list[dict[str, Any]] = []
    for device in devices:
        entity = cast("dict[str, Any]", device.get("entity") or {})
        dependencies.append(
            {
                "kind": entity.get("type", ""),
                "id": entity.get("id"),
                "label": entity.get("label", ""),
                "action": "removed",
                "note": "Loses this firewall's rules when the firewall is deleted.",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            (
                f"{len(dependencies)} resource(s) currently use this firewall"
                " and will lose its rules."
            )
        ]

    return details


async def linode_vpc_delete_dependency_walk(
    client: RetryableClient, vpc_id: int, _state: DeclaredState
) -> DryRunDetails:
    """Name the subnets destroyed with the VPC and the interfaces they detach.

    Best-effort: a failed subnet list becomes a warning rather than failing the
    preview.
    """
    details: DryRunDetails = {}
    try:
        subnets = await client.list_vpc_subnets(vpc_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        details["warnings"] = [f"Could not list VPC subnets: {exc}"]
        return details

    attached_interfaces = 0
    dependencies: list[dict[str, Any]] = []
    for subnet in subnets:
        linodes = cast("list[Any]", subnet.get("linodes", []))
        attached_interfaces += len(linodes)
        dependencies.append(
            {
                "kind": "vpc_subnet",
                "id": subnet.get("id"),
                "label": subnet.get("label", ""),
                "action": "cascade_deleted",
                "note": f"{len(linodes)} attached Linode interface(s)",
            }
        )

    if dependencies:
        details["dependencies"] = dependencies
    if attached_interfaces > 0:
        details["warnings"] = [
            (
                f"{attached_interfaces} Linode interface(s) across"
                f" {len(dependencies)} subnet(s) will be detached."
            )
        ]

    return details


async def linode_nodebalancer_delete_dependency_walk(
    client: RetryableClient, nodebalancer_id: int, _state: Any
) -> DryRunDetails:
    """Name the configs destroyed with the NodeBalancer.

    Each config takes its backend node list with it. Best-effort: a failed
    config list becomes a warning rather than failing the preview.
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
            (
                f"Deleting this NodeBalancer destroys {len(dependencies)} config(s) "
                "and their backend node lists."
            )
        ]

    return details


async def linode_placement_group_delete_dependency_walk(
    client: RetryableClient, group_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name what a placement-group delete takes with it.

    Each member Linode is detached; the instances themselves are not deleted.
    The members come from the declared fetch's state, so no extra call is made.
    """
    del client, group_id
    details: DryRunDetails = {}
    members = state.objects("members")
    dependencies: list[dict[str, Any]] = [
        {
            "kind": "instance",
            "id": member.number("linode_id"),
            "action": "detached",
            "note": "Linode is removed from the placement group; "
            "the instance is not deleted.",
        }
        for member in members
    ]
    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            (
                f"Deleting this placement group detaches {len(dependencies)} "
                "Linode(s); the instances are not deleted."
            )
        ]
    return details


async def linode_lke_pool_delete_dependency_walk(
    client: RetryableClient, cluster_id: int, pool_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name the backing Linodes a pool delete destroys.

    The fetched pool carries its nodes, so each node's Linode is reported one
    by one and a warning counts them: the running workloads are the part a
    caller cannot put back.
    """
    del client, cluster_id, pool_id
    details: DryRunDetails = {}
    dependencies: list[dict[str, Any]] = [
        {
            "kind": "instance",
            "id": node.number("instance_id"),
            "label": node.text("id"),
            "action": "cascade_deleted",
            "note": "Backing Linode for this pool node.",
        }
        for node in state.objects("nodes")
    ]
    count = state.number("count") or 0
    if dependencies:
        details["dependencies"] = dependencies
    if count > 0:
        details["warnings"] = [
            (
                f"Deleting this pool destroys {count} node(s) and their backing "
                "Linodes; running workloads are lost."
            )
        ]
    return details


async def linode_lke_node_delete_dependency_walk(
    client: RetryableClient, cluster_id: int, node_id: str, state: DeclaredState
) -> DryRunDetails:
    """Name the backing Linode a node delete destroys, plus the pool effect.

    The fetched node names its Linode, and the pool-level consequence is the
    part a caller cannot see from the request.
    """
    del client, cluster_id, node_id
    details: DryRunDetails = {}
    instance_id = state.number("instance_id")
    if instance_id:
        details["dependencies"] = [
            {
                "kind": "instance",
                "id": instance_id,
                "label": state.text("id"),
                "action": "cascade_deleted",
                "note": "Backing Linode for this node.",
            }
        ]
    details["warnings"] = [
        (
            "Deleting this node removes it from its pool; the pool node count "
            "drops by one and scheduled workloads reschedule."
        )
    ]
    return details


async def linode_tag_delete_fetch_state(client: RetryableClient, tag_label: str) -> Any:
    """List the objects carrying the tag a delete would remove.

    The page is both what the preview reports and what the walk names one by
    one, so the tagged objects are read once for the two of them.
    """
    return await client.list_tagged_objects(tag_label)


async def linode_tag_delete_dependency_walk(
    client: RetryableClient, tag_label: str, state: Any
) -> DryRunDetails:
    """Name each object that loses the tag; none of them is deleted.

    The itemized list is the fetched page, while the count comes from the
    envelope's total, so a truncated first page cannot understate the blast
    radius. A second warning says how many were itemized.
    """
    del client, tag_label
    page = cast("dict[str, Any]", state) if isinstance(state, dict) else {}
    raw_objects = page.get("data", [])
    tagged_items = (
        cast("list[object]", raw_objects) if isinstance(raw_objects, list) else []
    )
    objects = [
        cast("dict[str, Any]", obj) for obj in tagged_items if isinstance(obj, dict)
    ]

    dependencies: list[dict[str, Any]] = []
    for tagged in objects:
        dependency: dict[str, Any] = {
            "kind": str(tagged.get("type") or "resource"),
            "action": "removed",
            "note": "Loses this tag; the resource itself is not deleted.",
        }
        data = tagged.get("data")
        if isinstance(data, dict) and "id" in data:
            dependency["id"] = cast("dict[str, Any]", data)["id"]
        dependencies.append(dependency)

    details: DryRunDetails = {}
    if not dependencies:
        return details

    total = len(dependencies)
    raw_results = page.get("results")
    if isinstance(raw_results, int) and not isinstance(raw_results, bool):
        total = max(total, raw_results)

    warnings = [
        (
            f"Deleting this tag removes it from {total} tagged "
            "object(s); the objects are not deleted."
        )
    ]
    if total > len(dependencies):
        warnings.append(
            f"Only the first {len(dependencies)} tagged object(s) are "
            "itemized in this preview."
        )

    details["dependencies"] = dependencies
    details["warnings"] = warnings
    return details


def _config_references_disk(config: dict[str, Any], disk_id: int) -> bool:
    """Report whether any device slot of the config points at the disk."""
    devices = config.get("devices")
    if not isinstance(devices, dict):
        return False
    slots = cast("dict[str, Any]", devices).values()
    return any(
        isinstance(slot, dict)
        and cast("dict[str, Any]", slot).get("disk_id") == disk_id
        for slot in slots
    )


async def linode_instance_disk_delete_dependency_walk(
    client: RetryableClient, linode_id: int, disk_id: int, state: Any
) -> DryRunDetails:
    """Name the configuration profiles whose device slots point at the disk.

    Deleting it leaves those slots empty. Best-effort: a failed config list
    becomes a warning rather than refusing the preview.
    """
    del state
    try:
        page = await client.list_instance_configs(linode_id, 1, WALK_PAGE_SIZE)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return {"warnings": [f"Could not list instance configs: {exc}"]}

    dependencies = [
        {
            "kind": "instance_config",
            "id": config.get("id"),
            "label": str(config.get("label", "")),
            "action": "removed",
            "note": (
                "References this disk; its device slot is cleared when the "
                "disk is deleted."
            ),
        }
        for config in walk_page_items(page)
        if _config_references_disk(config, disk_id)
    ]

    details: DryRunDetails = {}
    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            "Config profiles reference this disk; deleting it leaves those slots empty."
        ]
    return details


async def linode_nodebalancer_config_delete_dependency_walk(
    client: RetryableClient, nodebalancer_id: int, config_id: int, state: Any
) -> DryRunDetails:
    """Name the backend nodes the delete takes out of rotation.

    The config owns them and they go with it. Best-effort: a failed node list
    becomes a warning rather than refusing the preview.
    """
    del state
    try:
        page = await client.list_nodebalancer_config_nodes(
            nodebalancer_id, config_id, 1, WALK_PAGE_SIZE
        )
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        return {"warnings": [f"Could not list config backend nodes: {exc}"]}

    dependencies = [
        {
            "kind": "nodebalancer_node",
            "id": node.get("id"),
            "label": str(node.get("label", "")),
            "action": "cascade_deleted",
            "note": f"backend {node.get('address', '')} ({node.get('mode', '')})",
        }
        for node in walk_page_items(page)
    ]

    details: DryRunDetails = {}
    if dependencies:
        details["dependencies"] = dependencies
        details["warnings"] = [
            (
                f"Deleting this config removes {len(dependencies)} backend "
                "node(s) from the rotation."
            )
        ]
    return details


async def linode_instance_password_reset_dependency_walk(
    client: RetryableClient, linode_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name the downtime the reset costs.

    The API powers the Linode down and back up to apply the password, which no
    descriptor says, and a running instance loses service while it happens.
    """
    del client, linode_id
    details: DryRunDetails = {
        "side_effects": [
            "The instance is powered down and rebooted to apply the new root password."
        ]
    }
    if state.text("status") == _INSTANCE_STATUS_RUNNING:
        details["warnings"] = [
            (
                "Instance is currently running; the reset shuts it down and "
                "reboots it, causing downtime."
            )
        ]
    return details


async def linode_instance_rebuild_dependency_walk(
    client: RetryableClient, linode_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name what a rebuild erases.

    Every disk is recreated from the new image, and the current image is named
    in a warning because that is what the caller is replacing. Best-effort: a
    failed disk list becomes a warning rather than refusing the preview.
    """
    warnings: list[str] = []
    try:
        disks = await client.list_instance_disks(linode_id)
    except (APIError, NetworkError, httpx.HTTPError) as exc:
        warnings.append(f"Could not list instance disks: {exc}")
        disks = []

    # Double quotes match the Go walk's %q formatting, so the fixture-pinned
    # side-effect text is identical across languages.
    side_effects = [
        f'Disk "{disk.get("label", "")}" ({disk.get("size", 0)} MB, '
        f"{disk.get('filesystem', '')}) is erased and recreated from the new image."
        for disk in disks
    ]

    image = state.text("image")
    if image:
        warnings.append(
            f'Rebuild replaces the current image "{image}", destroys all data, '
            "and resets the root password."
        )
    else:
        warnings.append(
            "Rebuild destroys all data on the instance and resets the root password."
        )

    details: DryRunDetails = {"warnings": warnings}
    if side_effects:
        details["side_effects"] = side_effects
    return details


async def linode_vpc_subnet_delete_dependency_walk(
    client: RetryableClient, vpc_id: int, _subnet_id: int, state: DeclaredState
) -> DryRunDetails:
    """Name the Linodes whose interfaces sit in the subnet: each is detached
    rather than deleted.

    The parent VPC is read once to label the warning; a failed read leaves the
    label empty rather than refusing the preview.
    """
    details: DryRunDetails = {}
    dependencies: list[dict[str, Any]] = [
        {
            "kind": "instance",
            "id": linode_ref.number("id"),
            "action": "detached",
            "note": (
                f"{len(linode_ref.objects('interfaces'))} "
                "interface(s) in this subnet are detached."
            ),
        }
        for linode_ref in state.objects("linodes")
    ]
    if not dependencies:
        return details

    vpc_label = ""
    try:
        vpc = await client.get_vpc(vpc_id)
    except (APIError, NetworkError, httpx.HTTPError):
        vpc = None
    if vpc is not None:
        vpc_label = str(vpc.get("label", ""))

    subnet_label = state.text("label")
    details["dependencies"] = dependencies
    # Double quotes match the Go walk's %q formatting, so the fixture-pinned
    # warning text is identical across languages.
    details["warnings"] = [
        (
            f'{len(dependencies)} Linode(s) have interfaces in subnet "{subnet_label}" '
            f'(VPC "{vpc_label}") and will be detached.'
        )
    ]
    return details


async def linode_vlan_delete_fetch_state(
    client: RetryableClient, region_id: str, label: str
) -> Any:
    """Resolve one VLAN by region and label.

    VLANs expose no single-resource GET, only a paginated list, so this lists
    and filters; a VLAN that matches nothing raises, which the preview reports.
    """
    vlans: list[dict[str, Any]] = await client.list_vlans(
        page=1, page_size=_MAX_VLAN_PAGE_SIZE
    )
    for vlan in vlans:
        if vlan.get("region") == region_id and vlan.get("label") == label:
            return vlan
    msg = f"VLAN not found: {label} in region {region_id}"
    raise ValueError(msg)


async def linode_firewall_update_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Read the firewall the update changes and name the change.

    Go's hand-written handler fetched the state and Python's walked it; both
    halves are here.
    """
    firewall_id = required_int_id(arguments, "firewall_id")[0] or 0

    async def fetch(client: RetryableClient) -> Any:
        return await client.get_firewall(firewall_id)

    async def walk(_client: RetryableClient, state: Any) -> DryRunDetails:
        return _firewall_update_details(
            state, arguments.get("label"), arguments.get("status")
        )

    return await execute_dry_run(
        cfg,
        arguments,
        "linode_firewall_update",
        method,
        path,
        fetch,
        walk,
        request_body=body,
    )


def _firewall_update_details(
    state: Any, new_label: Any, new_status: Any
) -> DryRunDetails:
    """Report the label change and a status change against the firewall.

    The quoting is Go's %q rather than Python's !r, which is what every other
    migrated preview in this repo settled on and what the hand handler here
    diverged from.
    """
    side_effects: list[str] = []
    if new_label:
        from_label = getattr(state, "label", "")
        if from_label and from_label != new_label:
            side_effects.append(f'Label changes from "{from_label}" to "{new_label}".')
        else:
            side_effects.append(f'Label is set to "{new_label}".')
    if new_status:
        from_status = getattr(state, "status", "")
        if new_status != from_status:
            verb = "stops enforcing" if new_status == "disabled" else "starts enforcing"
            side_effects.append(
                f'Firewall status changes to "{new_status}"; this immediately '
                f"{verb} its rules."
            )
    return {"side_effects": side_effects} if side_effects else {}


def _object_upload_settings(settings: ObjectStorageConfig) -> ObjectStorageConfig:
    """Fill any data-plane budget the config left at zero.

    Both hooks resolve through here because a preview describing one presign
    lifetime while the transfer requested another would report a call the tool
    does not make.
    """
    return ObjectStorageConfig(
        filesystem_root=settings.filesystem_root,
        max_single_part_bytes=settings.max_single_part_bytes or 5 * 1024 * 1024 * 1024,
        transfer_timeout=objectdata.resolved_timeout(settings.transfer_timeout),
        presign_ttl_seconds=settings.presign_ttl_seconds or 3600,
    )


def _fill_presign_body(
    body: dict[str, Any] | None, settings: ObjectStorageConfig
) -> dict[str, Any] | None:
    """Add the members the caller may omit but the presign request has to carry.

    Content-Type especially: the signature covers it, so a URL signed without one
    and then PUT with one is refused by the endpoint.
    """
    if body is None:
        return None

    body.setdefault("content_type", objectdata.DEFAULT_CONTENT_TYPE)
    body.setdefault("expires_in", settings.presign_ttl_seconds)

    return body


def _object_upload_side_effects(
    arguments: dict[str, Any], settings: ObjectStorageConfig
) -> DryRunDetails:
    """Describe the transfer, naming the refusal when the file is already too big."""
    try:
        source = objectdata.inspect(
            str(arguments.get("source_path", "")), settings.filesystem_root
        )
        objectdata.check_single_part(source.size_bytes, settings.max_single_part_bytes)
    except objectdata.ObjectDataError as err:
        return {"warnings": [str(err)]}

    sentence = (
        f"{source.size_bytes} bytes will be uploaded to"
        f" '{arguments.get('name', '')}' in bucket"
        f" '{arguments.get('label', '')}' as a single part."
    )

    return {"side_effects": [sentence]}


async def linode_object_storage_object_upload_preview(
    cfg: Config,
    arguments: dict[str, Any],
    method: str,
    path: str,
    body: dict[str, Any] | None,
) -> list[TextContent]:
    """Report the file the upload would send, opening nothing and calling nothing.

    It stats the source, which the support-ticket attachment's preview
    deliberately does not do, and the difference is the point: the attachment's
    size does not change what its call does, while here the size decides whether
    the call is legal at all.
    """
    settings = _object_upload_settings(cfg.object_storage)
    details = _object_upload_side_effects(arguments, settings)

    return build_dry_run_response(
        "linode_object_storage_object_upload",
        arguments.get("environment", ""),
        method,
        path,
        None,
        request_body=_fill_presign_body(body, settings),
        side_effects=details.get("side_effects", []),
        warnings=details.get("warnings", []),
    )


async def linode_object_storage_object_upload_execute(
    client: RetryableClient,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    body: dict[str, Any] | None,
) -> dict[str, Any]:
    """Ask the API for a presigned URL, then send the file to it.

    The transfer is the hook's rather than the driver's because the request is a
    file body against a URL the API just minted, not the JSON body every
    generated mutation sends. Only one Linode operation happens here, the
    presign, which is why that is the route the tool declares.

    The file is checked before the presign call, so an oversized or unreadable
    source costs no request at all.
    """
    settings = _object_upload_settings(client.object_storage)

    source = objectdata.inspect(
        str(arguments.get("source_path", "")), settings.filesystem_root
    )
    objectdata.check_single_part(source.size_bytes, settings.max_single_part_bytes)

    presigned = await client.route_raw(
        "linode_object_storage_object_upload",
        *values,
        body=_fill_presign_body(body, settings),
    )

    # route_raw hands back whatever decoded, so an array or a scalar would reach
    # .get() and answer an AttributeError naming a Python type. Go's decode of
    # the same body fails with this sentence, so the refusal is worded to match.
    if not isinstance(presigned, dict):
        msg = (
            "failed to unmarshal object storage object upload object:"
            " response body is not a JSON object"
        )
        raise TypeError(msg)

    minted = cast("dict[str, Any]", presigned)

    uploaded = await objectdata.upload(
        str(minted.get("url", "")),
        source,
        str(arguments.get("content_type") or objectdata.DEFAULT_CONTENT_TYPE),
        settings.transfer_timeout,
    )

    return {
        "size_bytes": uploaded.size_bytes,
        "etag": uploaded.etag,
        "upload_mode": _UPLOAD_MODE_SINGLE,
    }


async def linode_object_storage_object_download_execute(
    client: RetryableClient,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    body: dict[str, Any] | None,
) -> dict[str, Any]:
    """Ask the API for a presigned URL, then follow it to the local file.

    The transfer is the hook's because the response is a stream of object bytes
    rather than the JSON body a generated read decodes. Only one Linode
    operation happens here, the presign, which is the route the tool declares.

    The destination is resolved before the presign call, so a refused download
    costs no request at all.
    """
    settings = _object_upload_settings(client.object_storage)

    destination = objectdata.resolve_destination(
        str(arguments.get("dest_path", "")),
        settings.filesystem_root,
        overwrite=bool(arguments.get("overwrite", False)),
    )

    presigned = await client.route_raw(
        "linode_object_storage_object_download",
        *values,
        body=_fill_presign_body(body, settings),
    )

    # route_raw hands back whatever decoded, so an array or a scalar would reach
    # .get() and answer an AttributeError naming a Python type. Go's decode of
    # the same body fails with this sentence, so the refusal is worded to match.
    if not isinstance(presigned, dict):
        msg = (
            "failed to unmarshal object storage object download object:"
            " response body is not a JSON object"
        )
        raise TypeError(msg)

    minted = cast("dict[str, Any]", presigned)

    fetched = await objectdata.download(
        str(minted.get("url", "")),
        destination,
        settings.transfer_timeout,
    )

    return {"size_bytes": fetched.size_bytes, "etag": fetched.etag}
