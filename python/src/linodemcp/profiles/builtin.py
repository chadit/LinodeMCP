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

Cross-language parity rules are exercised by tests in both implementations.
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


# Tool prefix categories, the same table go/internal/profiles/builtin.go
# declares as ``categoryTable``. ``make profile-resolution`` holds the two to
# one answer per tool: a category a tool has in one language and not the other
# is a profile that serves different tools depending on which client the caller
# runs. Every matching rule contributes, so declaration order is for reading
# rather than for precedence.
_TOOL_CATEGORIES: tuple[tuple[str, tuple[str, ...]], ...] = (
    # Per-instance sub-resources, listed ahead of compute for reading order:
    # storage-admin elevates this slice without the rest of compute.
    (
        "compute_deep",
        (
            "linode_instance_backup_",
            "linode_instance_backups_",
            "linode_instance_disk_",
            "linode_instance_ip_",
            "linode_instance_stats_",
            "linode_instance_transfer_",
        ),
    ),
    # Tool names are singular across verbs (linode_image_list,
    # linode_stackscript_create), so one prefix per resource covers a family's
    # reads and writes alike.
    (
        "compute",
        (
            "linode_image_",
            "linode_instance_",
            "linode_kernel_",
            "linode_placement_group_",
            "linode_region_",
            "linode_stackscript_",
            "linode_type_",
        ),
    ),
    # What the API gates on account:*, following scope.py's account rule. The
    # profile prefixes are enumerated rather than swept up under one
    # ``linode_profile_`` entry because the builder's own draft tools share
    # that prefix and never reach the API.
    (
        "account",
        (
            "linode_account_",
            "linode_beta_",
            "linode_lock_",
            "linode_maintenance_policy_",
            "linode_managed_",
            "linode_profile_app_",
            "linode_profile_device_",
            "linode_profile_grants_",
            "linode_profile_login_",
            "linode_profile_phone_number_",
            "linode_profile_preferences_",
            "linode_profile_security_",
            "linode_profile_tfa_",
            "linode_profile_token_",
            "linode_profile_update",
            "linode_support_ticket_",
            "linode_tag_",
        ),
    ),
    ("block_storage", ("linode_volume_",)),
    ("databases", ("linode_database_",)),
    ("object_storage", ("linode_object_storage_",)),
    ("dns", ("linode_domain_",)),
    (
        "networking",
        (
            "linode_firewall_",
            "linode_ipv6_",
            "linode_network_transfer_",
            "linode_networking_",
            "linode_nodebalancer_",
            "linode_vlan_",
        ),
    ),
    ("lke", ("linode_lke_",)),
    ("vpcs", ("linode_vpc_",)),
    ("security", ("linode_sshkey_",)),
    ("monitor", ("linode_monitor_",)),
    # Longview carries its own longview:* scope, so a profile that elevates
    # monitor must not reach it.
    ("longview", ("linode_longview_",)),
    ("iam", ("linode_iam_",)),
)


# Exact tool names that belong to the ``core`` category. Core is the session's
# own starting point rather than a slice of the Linode surface, so it stands
# apart from the prefix table and no profile elevates it; account-gated tools
# live in ``account``, which one can.
_CORE_TOOL_NAMES: frozenset[str] = frozenset(
    {"hello", "version", "linode_profile_get", "linode_account_get"}
)

# Elevated-category marker meaning "every category". Spelled the same as Go's
# allEnvironments sentinel so the wildcard profiles resolve identically; without
# it a mutator matching no prefix reaches nothing here while Go's short-circuit
# still serves it.
_ALL_CATEGORIES = "*"


def categories(tool_name: str) -> list[str]:
    """Return every category a tool name belongs to.

    Mirrors the Go ``profiles.Categories`` shape so the Phase 8.2
    builder tool ``linode_profile_list_tools`` can return the same
    field structure across languages. Every category whose prefix list
    matches contributes one entry, so a tool can land in more than one
    and a profile elevating any of them serves it. An empty list signals
    "no known category"; the caller decides how to render that.

    Core tools (hello, version, linode_profile_get, linode_account_get) live
    in their own bucket and bypass the prefix walk.
    """
    if tool_name in _CORE_TOOL_NAMES:
        return ["core"]

    return [
        category
        for category, prefixes in _TOOL_CATEGORIES
        if tool_name.startswith(prefixes)
    ]


# Capabilities that mutate state. Tools with these caps are gated by the
# category-elevation step; tools with Read/Meta capabilities are always
# included regardless of category.
_MUTATING_CAPABILITIES: frozenset[Capability] = frozenset(
    {Capability.Write, Capability.Destroy}
)


def _is_elevated(tool_name: str, elevated_categories: frozenset[str]) -> bool:
    """Whether any category the tool falls in is one the profile elevates."""
    if _ALL_CATEGORIES in elevated_categories:
        return True

    return bool(set(categories(tool_name)) & elevated_categories)


def _resolve_allowed_tools(
    catalog: Sequence[ToolDescriptor],
    elevated_categories: frozenset[str],
) -> tuple[str, ...]:
    """Pick the tools a profile permits given its elevated categories.

    Read and Meta tools are always included. Mutating tools (Write,
    Destroy) are included only if one of their categories is in
    ``elevated_categories``. Admin tools (account/child_account-scope
    operations: account administration, profile token/TFA/security/phone
    self-service, the Managed surface) are included only for the wildcard
    profiles, full-access and emergency; no category-specific admin profile
    grants them.
    """
    grants_admin = _ALL_CATEGORIES in elevated_categories
    selected: list[str] = []

    for tool in catalog:
        if tool.capability in (Capability.Read, Capability.Meta):
            selected.append(tool.name)
        elif tool.capability is Capability.Admin:
            if grants_admin:
                selected.append(tool.name)
        elif tool.capability in _MUTATING_CAPABILITIES and _is_elevated(
            tool.name, elevated_categories
        ):
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
    allow_yolo: bool
    disabled: bool


_PROFILE_BLUEPRINTS: dict[str, _ProfileBlueprint] = {
    "default": _ProfileBlueprint(
        description=(
            "Safe read-only default profile. Cannot execute writes, destroys, "
            "or admin operations."
        ),
        elevated_categories=frozenset(),
        allow_yolo=False,
        disabled=False,
    ),
    "readonly-full": _ProfileBlueprint(
        description="Explicit read-only profile spanning every category.",
        elevated_categories=frozenset(),
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
        allow_yolo=False,
        disabled=False,
    ),
    "network-admin": _ProfileBlueprint(
        description=(
            "Read everywhere plus write/destroy on networking, DNS, and VPCs."
        ),
        elevated_categories=frozenset({"dns", "networking", "vpcs"}),
        allow_yolo=False,
        disabled=False,
    ),
    "kubernetes-admin": _ProfileBlueprint(
        description=("Read everywhere plus write/destroy on LKE, compute, and VPCs."),
        elevated_categories=frozenset({"lke", "compute", "compute_deep", "vpcs"}),
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
        allow_yolo=False,
        disabled=False,
    ),
    "full-access": _ProfileBlueprint(
        description=(
            "Read, write, and destroy across every category. Disabled by default."
        ),
        elevated_categories=frozenset({_ALL_CATEGORIES}),
        allow_yolo=False,
        disabled=True,
    ),
    "emergency": _ProfileBlueprint(
        description=(
            "Break-glass profile: full access plus yolo execution. Disabled by default."
        ),
        elevated_categories=frozenset({_ALL_CATEGORIES}),
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
    used by the parity test for reproducible JSON output.
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
            elevated=has_mutating_tools(catalog, allowed),
            allow_yolo=blueprint.allow_yolo,
            disabled=blueprint.disabled,
        )
    return profiles


def has_mutating_tools(
    catalog: Sequence[ToolDescriptor], allowed_tools: Sequence[str]
) -> bool:
    """Return True when any allowed tool is Write, Destroy, or Admin.

    Drives the profile ``elevated`` flag; scope suffixes cannot, because
    the API documents write scopes on several read-only routes. Tools
    the catalog doesn't know about contribute nothing, matching
    ``_compute_required_scopes``.
    """
    cap_by_name = {tool.name: tool.capability for tool in catalog}
    mutating = (Capability.Write, Capability.Destroy, Capability.Admin)
    return any(cap_by_name.get(name) in mutating for name in allowed_tools)


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
            "elevated": profile.elevated,
            "allow_yolo": profile.allow_yolo,
            "disabled": profile.disabled,
        }
        for name, profile in profiles.items()
    }
    return json.dumps(payload, sort_keys=True, indent=2)
