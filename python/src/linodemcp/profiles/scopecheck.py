"""Scope comparison primitives for Phase 6.4 token validation.

Mirrors ``go/internal/profiles/scopecheck.go``. Pure functions: parse a
PAT scope string into a Scope set, flatten an OAuth Grants payload into
the same set shape, and compare a profile's required scopes against
what the token carries.

The loader (wired in Phase 6.4b) takes a comparison result and decides
policy: missing scopes are a hard failure, excess scopes warn unless
strict mode promotes them to errors.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import TYPE_CHECKING

from linodemcp.profiles.scope import Scope

if TYPE_CHECKING:
    from linodemcp.linode import GlobalGrants, Grant, Grants


def parse_pat_scopes(scope_str: str) -> list[Scope]:
    """Split a Linode PAT scope string into a deduplicated sorted Scope list.

    The format is space-delimited tokens; ``"*"`` stands for "all
    permissions" on every category. Whitespace-only or empty input
    yields an empty list. Unknown scope strings are still accepted: the
    Scope enum constructor falls back to a string-valued enum lookup
    that allows future scopes the catalog doesn't yet name.
    """
    fields_ = scope_str.split()
    if not fields_:
        return []

    seen: set[str] = set()
    for token in fields_:
        seen.add(token)

    out: list[Scope] = []
    for token in sorted(seen):
        # StrEnum constructor matches by value; unknown strings raise.
        # We swallow unknown ones because future Linode scopes shouldn't
        # crash the loader (the comparison logic treats unrecognized
        # actuals as excess, which the warn path handles).
        try:
            out.append(Scope(token))
        except ValueError:
            # Skip unrecognized scope strings rather than failing the
            # whole parse. The token still works at runtime; we just
            # can't compare it against the catalog.
            continue
    return out


def _add_pair(perm: str, read_only: Scope, read_write: Scope, seen: set[Scope]) -> None:
    """Add the scope pair implied by a permission string.

    Mirrors Go's addPair helper: ``"read_write"`` implies both scopes;
    ``"read_only"`` implies only the read-only scope; anything else
    (empty string, unknown value) contributes nothing.
    """
    if perm == "read_write":
        seen.add(read_write)
        seen.add(read_only)
    elif perm == "read_only":
        seen.add(read_only)


def _collect_global(global_: GlobalGrants, seen: set[Scope]) -> None:
    """Walk the OAuth global grant booleans and add the implied scopes.

    AccountAccess gives the account read_only/read_write pair directly;
    each ``add_<resource>`` boolean implies the corresponding category's
    :read_write (write subsumes read).
    """
    _add_pair(
        global_.account_access,
        Scope.AccountReadOnly,
        Scope.AccountReadWrite,
        seen,
    )

    pairs = [
        (global_.add_linodes, Scope.LinodesReadOnly, Scope.LinodesReadWrite),
        (global_.add_domains, Scope.DomainsReadOnly, Scope.DomainsReadWrite),
        (global_.add_firewalls, Scope.FirewallReadOnly, Scope.FirewallReadWrite),
        (global_.add_images, Scope.ImagesReadOnly, Scope.ImagesReadWrite),
        (global_.add_databases, Scope.DatabasesReadOnly, Scope.DatabasesReadWrite),
        (global_.add_longview, Scope.LongviewReadOnly, Scope.LongviewReadWrite),
        (
            global_.add_nodebalancers,
            Scope.NodeBalancersReadOnly,
            Scope.NodeBalancersReadWrite,
        ),
        (
            global_.add_stackscripts,
            Scope.StackScriptsReadOnly,
            Scope.StackScriptsReadWrite,
        ),
        (global_.add_volumes, Scope.VolumesReadOnly, Scope.VolumesReadWrite),
        (global_.add_vpcs, Scope.VPCReadOnly, Scope.VPCReadWrite),
    ]
    for has_flag, read_only, read_write in pairs:
        if has_flag:
            seen.add(read_write)
            seen.add(read_only)


def _collect_resources(grants: Grants, seen: set[Scope]) -> None:
    """Walk the per-resource grant lists and add the scopes they imply."""
    category_map: list[tuple[list[Grant], Scope, Scope]] = [
        (list(grants.linode), Scope.LinodesReadOnly, Scope.LinodesReadWrite),
        (list(grants.domain), Scope.DomainsReadOnly, Scope.DomainsReadWrite),
        (
            list(grants.nodebalancer),
            Scope.NodeBalancersReadOnly,
            Scope.NodeBalancersReadWrite,
        ),
        (list(grants.image), Scope.ImagesReadOnly, Scope.ImagesReadWrite),
        (list(grants.longview), Scope.LongviewReadOnly, Scope.LongviewReadWrite),
        (
            list(grants.stackscript),
            Scope.StackScriptsReadOnly,
            Scope.StackScriptsReadWrite,
        ),
        (list(grants.volume), Scope.VolumesReadOnly, Scope.VolumesReadWrite),
        (list(grants.database), Scope.DatabasesReadOnly, Scope.DatabasesReadWrite),
        (list(grants.firewall), Scope.FirewallReadOnly, Scope.FirewallReadWrite),
        (list(grants.vpc), Scope.VPCReadOnly, Scope.VPCReadWrite),
        (list(grants.lkecluster), Scope.LKEReadOnly, Scope.LKEReadWrite),
    ]
    for entries, read_only, read_write in category_map:
        for entry in entries:
            _add_pair(entry.permissions, read_only, read_write, seen)


def flatten_grants(grants: Grants | None) -> list[Scope]:
    """Walk a /profile/grants response and return the effective Scope set.

    Global account booleans produce account-level scopes; per-resource
    grants produce the matching category scope. Empty/null permission
    entries contribute nothing. The output is sorted and deduplicated.
    A ``None`` input yields an empty list (matches Go's nil-safe path).
    """
    if grants is None:
        return []

    seen: set[Scope] = set()
    _collect_global(grants.global_, seen)
    _collect_resources(grants, seen)

    return sorted(seen, key=lambda s: s.value)


@dataclass(frozen=True)
class ScopeComparison:
    """Result of comparing a profile's required scopes against actuals.

    ``missing`` lists scopes the profile requires but the token lacks;
    these are a hard failure at load time. ``excess`` lists scopes the
    token carries that the profile doesn't require; warning by default,
    strict mode promotes them to errors.
    """

    missing: tuple[Scope, ...] = field(default_factory=tuple)
    excess: tuple[Scope, ...] = field(default_factory=tuple)

    @property
    def has_missing(self) -> bool:
        """True if the token is under-scoped for the active profile."""
        return bool(self.missing)

    @property
    def has_excess(self) -> bool:
        """True if the token carries more access than the profile needs."""
        return bool(self.excess)


def compare_scopes(required: list[Scope], actual: list[Scope]) -> ScopeComparison:
    """Return the missing and excess sets between required and actual.

    Set-based; order doesn't matter. Output is sorted ascending so
    error messages stay stable. Wildcard handling:

    - ``Scope.Wildcard`` ("*") in ``actual`` matches every required
      scope. A token with just ``"*"`` satisfies any profile.
    - ``Scope.Wildcard`` in ``required`` is ignored (not meaningful for
      derived profiles; user-defined profiles may include it).
    """
    actual_set = set(actual)
    has_wildcard = Scope.Wildcard in actual_set

    required_set = {s for s in required if s != Scope.Wildcard}

    missing: list[Scope] = []
    if not has_wildcard:
        missing = sorted(
            (s for s in required_set if s not in actual_set),
            key=lambda s: s.value,
        )

    excess = sorted(
        (s for s in actual_set if s != Scope.Wildcard and s not in required_set),
        key=lambda s: s.value,
    )

    return ScopeComparison(missing=tuple(missing), excess=tuple(excess))
