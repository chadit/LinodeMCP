"""Built-in profile catalog.

Eight profiles ship as code, not config: ``default``, ``readonly-full``,
``compute-admin``, ``network-admin``, ``kubernetes-admin``,
``storage-admin``, ``full-access``, and ``emergency``. Users cannot edit
them; the only override knob is ``disabled`` (Phase 3).

The catalog accepts a tool descriptor list as input rather than reaching
into ``linodemcp.server.get_tool_registry()``. ``linodemcp.server`` already
imports ``linodemcp.profiles`` for the ``Capability`` enum; importing the
server here would create a cycle. Callers (tests, the server) pass in the
descriptors they assemble locally.

Cross-language parity rules live in ``.claude/tmp/builtin_profiles_spec.md``
and are exercised by tests in both implementations.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from typing import TYPE_CHECKING

from linodemcp.profiles.capability import Capability
from linodemcp.profiles.profile import Profile
from linodemcp.profiles.scope import required_scopes

if TYPE_CHECKING:
    from collections.abc import Sequence

__all__ = [
    "ToolDescriptor",
    "builtin_catalog_json",
    "builtin_profiles",
]


@dataclass(frozen=True)
class ToolDescriptor:
    """Lean view of a registered tool used by built-in profile resolution.

    Only the fields the resolver actually reads. Tests synthesize these
    directly; the live server adapts ``ToolEntry`` to this shape at the
    call site (kept out of this module to avoid the import cycle described
    in the module docstring).
    """

    name: str
    capability: Capability


# Tool prefix categories. Order matters: the resolver checks longer, more
# specific prefixes before shorter ones so ``linode_instance_backup_*`` lands
# in ``compute_deep`` instead of being captured by ``linode_instance_`` in
# ``compute``. See ``.claude/tmp/builtin_profiles_spec.md`` for the rules.
#
# The spec defines ``compute``, ``compute_actions``, and ``compute_deep``
# as three categories but allows collapsing the first two; every built-in
# that uses ``compute_actions`` also uses ``compute``, so we fold them into
# a single ``compute`` category. ``compute_deep`` stays separate because
# ``storage-admin`` needs deep (backups) without the rest of compute.
_TOOL_CATEGORIES: tuple[tuple[str, tuple[str, ...]], ...] = (
    # Core. Exact tool names rather than prefixes; matched separately below.
    # Listed here for completeness but enforced via ``_CORE_TOOL_NAMES``.
    ("core", ()),
    # compute_deep must come before compute so ``linode_instance_backup_*``
    # is not absorbed by the broader ``linode_instance_`` prefix.
    (
        "compute_deep",
        (
            "linode_instance_backup_",
            "linode_instance_backups_",
            "linode_instance_disk_",
            "linode_instance_disks_",
            "linode_instance_ip_",
            "linode_instance_ips_",
        ),
    ),
    (
        "compute",
        (
            "linode_instance_",
            "linode_instances_",
            "linode_regions_",
            "linode_types_",
            "linode_image_",
            "linode_images_",
            "linode_stackscripts_",
            "linode_stackscript_",
        ),
    ),
    ("account", ("linode_account_", "linode_profile_token_")),
    ("block_storage", ("linode_volume_", "linode_volumes_")),
    ("object_storage", ("linode_object_storage_",)),
    # dns lists ``linode_domain_record_`` before ``linode_domain_`` for the
    # same longest-prefix reason; all three share the same category so the
    # order is cosmetic here, but kept for clarity.
    (
        "dns",
        ("linode_domain_record_", "linode_domain_", "linode_domains_"),
    ),
    (
        "networking",
        (
            "linode_firewall_",
            "linode_firewalls_",
            "linode_nodebalancer_",
            "linode_nodebalancers_",
            "linode_vlan_",
            "linode_vlans_",
            "linode_ipv6_range_",
        ),
    ),
    ("lke", ("linode_lke_",)),
    ("vpcs", ("linode_vpc_", "linode_vpcs_")),
    ("security", ("linode_sshkey_", "linode_sshkeys_")),
    ("monitor", ("linode_monitor_",)),
)


# Exact tool names that belong to the ``core`` category. Core tools are
# meta/read-only and ship in every profile via the always-include rule, so
# the category is informational rather than load-bearing. Listed for
# completeness and to keep the categorizer total over the tool surface.
_CORE_TOOL_NAMES: frozenset[str] = frozenset(
    {"hello", "version", "linode_profile", "linode_account"}
)


def _categorize(tool_name: str) -> str | None:
    """Return the category a tool name belongs to, or ``None`` if unknown.

    Longest-prefix-wins over the category list. A tool that matches no
    prefix and is not a core name returns ``None``; the resolver treats
    that as "no elevated category" and includes the tool only via its
    capability tag (Read/Meta).
    """
    if tool_name in _CORE_TOOL_NAMES:
        return "core"
    for category, prefixes in _TOOL_CATEGORIES:
        for prefix in prefixes:
            if tool_name.startswith(prefix):
                return category
    return None


# Capabilities that mutate state. Tools with these caps are gated by the
# category-elevation step; tools with Read/Meta capabilities are always
# included regardless of category.
_MUTATING_CAPABILITIES: frozenset[Capability] = frozenset(
    {Capability.Write, Capability.Destroy}
)


def _resolve_allowed_tools(
    catalog: Sequence[ToolDescriptor],
    elevated_categories: frozenset[str],
) -> tuple[str, ...]:
    """Pick the tools a profile permits given its elevated categories.

    Read and Meta tools are always included. Mutating tools (Write,
    Destroy) are included only if their category is in
    ``elevated_categories``. Admin is excluded from every built-in per
    spec; if a future tool carries ``Capability.Admin`` it falls through
    to the catch-all and is not selected.
    """
    selected: list[str] = []
    for tool in catalog:
        if tool.capability in (Capability.Read, Capability.Meta):
            selected.append(tool.name)
            continue
        if tool.capability in _MUTATING_CAPABILITIES:
            category = _categorize(tool.name)
            if category is not None and category in elevated_categories:
                selected.append(tool.name)
    return tuple(sorted(selected))


# Profile blueprints, keyed by name. The resolver fills in ``allowed_tools``
# per catalog; everything else is fixed by spec.
@dataclass(frozen=True)
class _ProfileBlueprint:
    """Static fields for a built-in profile.

    Decoupled from ``Profile`` so the catalog (which is module-level) does
    not bake in an empty ``allowed_tools`` tuple that callers might mistake
    for "no tools resolved".
    """

    description: str
    elevated_categories: frozenset[str]
    required_token_scopes: tuple[str, ...]
    allow_yolo: bool
    disabled: bool


_PROFILE_BLUEPRINTS: dict[str, _ProfileBlueprint] = {
    "default": _ProfileBlueprint(
        description=(
            "Safe read-only default profile. Cannot execute writes, destroys, "
            "or admin operations."
        ),
        elevated_categories=frozenset(),
        required_token_scopes=("*:read_only",),
        allow_yolo=False,
        disabled=False,
    ),
    "readonly-full": _ProfileBlueprint(
        description="Explicit read-only profile spanning every category.",
        elevated_categories=frozenset(),
        required_token_scopes=("*:read_only",),
        allow_yolo=False,
        disabled=False,
    ),
    "compute-admin": _ProfileBlueprint(
        description=(
            "Read everywhere plus write/destroy on compute, block storage, "
            "and SSH keys."
        ),
        elevated_categories=frozenset(
            {"compute", "compute_deep", "block_storage", "security"}
        ),
        required_token_scopes=(
            "linodes:read_write",
            "ssh_keys:read_write",
            "volumes:read_write",
        ),
        allow_yolo=False,
        disabled=False,
    ),
    "network-admin": _ProfileBlueprint(
        description=(
            "Read everywhere plus write/destroy on networking, DNS, and VPCs."
        ),
        elevated_categories=frozenset({"dns", "networking", "vpcs"}),
        required_token_scopes=(
            "domains:read_write",
            "firewalls:read_write",
            "nodebalancers:read_write",
            "vpcs:read_write",
        ),
        allow_yolo=False,
        disabled=False,
    ),
    "kubernetes-admin": _ProfileBlueprint(
        description=("Read everywhere plus write/destroy on LKE, compute, and VPCs."),
        elevated_categories=frozenset({"lke", "compute", "compute_deep", "vpcs"}),
        required_token_scopes=(
            "linodes:read_write",
            "lke:read_write",
            "vpcs:read_write",
        ),
        allow_yolo=False,
        disabled=False,
    ),
    "storage-admin": _ProfileBlueprint(
        description=(
            "Read everywhere plus write/destroy on object storage, block "
            "storage, and backups."
        ),
        elevated_categories=frozenset(
            {"block_storage", "object_storage", "compute_deep"}
        ),
        required_token_scopes=(
            "object_storage:read_write",
            "volumes:read_write",
        ),
        allow_yolo=False,
        disabled=False,
    ),
    "full-access": _ProfileBlueprint(
        description=(
            "Read, write, and destroy across every category. Disabled by default."
        ),
        elevated_categories=frozenset(
            {
                "account",
                "compute",
                "compute_deep",
                "block_storage",
                "object_storage",
                "dns",
                "networking",
                "lke",
                "vpcs",
                "security",
                "monitor",
            }
        ),
        required_token_scopes=("*:read_write",),
        allow_yolo=False,
        disabled=True,
    ),
    "emergency": _ProfileBlueprint(
        description=(
            "Break-glass profile: full access plus yolo execution. Disabled by default."
        ),
        elevated_categories=frozenset(
            {
                "account",
                "compute",
                "compute_deep",
                "block_storage",
                "object_storage",
                "dns",
                "networking",
                "lke",
                "vpcs",
                "security",
                "monitor",
            }
        ),
        required_token_scopes=("*:read_write",),
        allow_yolo=True,
        disabled=True,
    ),
}


def _compute_required_scopes(
    catalog: Sequence[ToolDescriptor], allowed_tools: tuple[str, ...]
) -> tuple[str, ...]:
    """Return the deduplicated, sorted union of required_scopes over the
    profile's allowed tools.

    Phase 6.3 derives required_token_scopes from the resolved tool list
    instead of hardcoding it on the blueprint. The previous static
    values had Linode-name drift (firewalls plural, ssh_keys, vpcs
    plural); deriving them fixes the spelling in one place. Tools the
    catalog doesn't know about contribute nothing, matching the
    best-effort fallback in required_scopes itself.
    """
    cap_by_name = {d.name: d.capability for d in catalog}
    seen: set[str] = set()
    for tool_name in allowed_tools:
        capability = cap_by_name.get(tool_name)
        if capability is None:
            continue
        for scope in required_scopes(tool_name, capability):
            seen.add(scope.value)
    return tuple(sorted(seen))


def builtin_profiles(catalog: Sequence[ToolDescriptor]) -> dict[str, Profile]:
    """Build the eight built-in profiles against a tool catalog.

    Pure function: no I/O, no global state mutation. Call once per
    server-start (Phase 4 wiring) or per test. The returned dict's
    insertion order matches ``_PROFILE_BLUEPRINTS``, which is the order
    used by the parity test for deterministic JSON output.
    """
    profiles: dict[str, Profile] = {}
    for name, blueprint in _PROFILE_BLUEPRINTS.items():
        allowed = _resolve_allowed_tools(catalog, blueprint.elevated_categories)
        profiles[name] = Profile(
            name=name,
            description=blueprint.description,
            allowed_tools=allowed,
            allowed_environments=(),
            required_token_scopes=_compute_required_scopes(catalog, allowed),
            allow_yolo=blueprint.allow_yolo,
            disabled=blueprint.disabled,
        )
    return profiles


def builtin_catalog_json(catalog: Sequence[ToolDescriptor]) -> str:
    """Canonical JSON dump of the built-in catalog for parity testing.

    Profile keys are sorted alphabetically, and ``allowed_tools`` within
    each profile is already sorted by ``_resolve_allowed_tools``. The
    output is byte-stable across runs so the Go-side parity test can
    compare it directly.
    """
    profiles = builtin_profiles(catalog)
    payload = {
        name: {
            "name": profile.name,
            "description": profile.description,
            "allowed_tools": list(profile.allowed_tools),
            "allowed_environments": list(profile.allowed_environments),
            "required_token_scopes": list(profile.required_token_scopes),
            "allow_yolo": profile.allow_yolo,
            "disabled": profile.disabled,
        }
        for name, profile in profiles.items()
    }
    return json.dumps(payload, sort_keys=True, indent=2)
