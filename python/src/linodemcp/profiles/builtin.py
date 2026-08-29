"""Built-in profile catalog.

Nine profiles ship as code, not config: ``default``, ``readonly-full``,
``compute-admin``, ``network-admin``, ``kubernetes-admin``,
``storage-admin``, ``iam-admin``, ``full-access``, and ``emergency``. Users
cannot edit them; the only override knob is ``disabled`` (Phase 3).

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

if TYPE_CHECKING:
    from collections.abc import Sequence

__all__ = [
    "ToolDescriptor",
    "builtin_catalog_json",
    "builtin_profile_names",
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
    # The tool's declared OAuth scope strings, read from the generated
    # registry table at the call sites that build the catalog. Empty for
    # meta tools and documented-scopeless routes.
    scopes: tuple[str, ...] = ()
    # The tool's declared profile categories in declaration order, read
    # from the same generated table. Empty on a declared none.
    categories: tuple[str, ...] = ()


# Elevated-category marker meaning "every category". Spelled the same as Go's
# allEnvironments sentinel so the wildcard profiles resolve identically; without
# it a mutator with no category reaches nothing here while Go's
# short-circuit still serves it.
_ALL_CATEGORIES = "*"


# Capabilities that mutate state. Tools with these caps are gated by the
# category-elevation step; tools with Read/Meta capabilities are always
# included regardless of category.
_MUTATING_CAPABILITIES: frozenset[Capability] = frozenset(
    {Capability.Write, Capability.Destroy}
)


def _is_elevated(
    tool_categories: Sequence[str], elevated_categories: frozenset[str]
) -> bool:
    """Whether any of the tool's declared categories is one the profile
    elevates."""
    if _ALL_CATEGORIES in elevated_categories:
        return True

    return bool(set(tool_categories) & elevated_categories)


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
            tool.categories, elevated_categories
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
    "iam-admin": _ProfileBlueprint(
        description=(
            "Read everywhere plus write/destroy on identity and access "
            "management: role assignment, delegation, and IdP/SSO configs."
        ),
        # iam only: the legacy account grants tools sit in account, and Linode
        # documents mixing the two access-control systems as a security risk.
        elevated_categories=frozenset({"iam"}),
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
    """Return the deduplicated, sorted union of the catalog's declared
    per-tool scopes over the profile's allowed tools.

    Phase 6.3 derives required_token_scopes from the resolved tool list
    instead of hardcoding it on the blueprint. The previous static
    values had Linode-name drift (firewalls plural, ssh_keys, vpcs
    plural); deriving them fixes the spelling in one place. Tools the
    catalog doesn't know about contribute nothing.
    """
    scopes_by_name = {d.name: d.scopes for d in catalog}
    seen: set[str] = set()
    for tool_name in allowed_tools:
        seen.update(scopes_by_name.get(tool_name, ()))
    return tuple(sorted(seen))


def builtin_profile_names() -> frozenset[str]:
    """The names the built-in profiles occupy.

    Read off the blueprints rather than restated, so a built-in added here is
    refused as a save target without a second list needing the same edit.
    """
    return frozenset(_PROFILE_BLUEPRINTS)


def builtin_profiles(catalog: Sequence[ToolDescriptor]) -> dict[str, Profile]:
    """Build the nine built-in profiles against a tool catalog.

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
