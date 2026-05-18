"""Unit tests for Phase 6.4 scope comparison primitives.

Mirrors ``go/internal/profiles/scopecheck_test.go``. Pure-function
tests: parse a PAT scope string, flatten OAuth grants, and compare
required vs actual.
"""

from __future__ import annotations

from linodemcp.linode import GlobalGrants, Grant, Grants
from linodemcp.profiles import (
    Scope,
    compare_scopes,
    flatten_grants,
    parse_pat_scopes,
)


def test_parse_pat_scopes_empty() -> None:
    """Empty or whitespace-only PAT scope strings parse to []."""
    assert parse_pat_scopes("") == []
    assert parse_pat_scopes("   ") == []
    assert parse_pat_scopes("\t\n") == []


def test_parse_pat_scopes_splits() -> None:
    """Space-delimited scope tokens parse into a deduplicated sorted list."""
    got = parse_pat_scopes("linodes:read_write volumes:read_only domains:read_write")
    assert set(got) == {
        Scope.LinodesReadWrite,
        Scope.VolumesReadOnly,
        Scope.DomainsReadWrite,
    }


def test_parse_pat_scopes_dedupes() -> None:
    """A PAT response with duplicate tokens collapses to one Scope each."""
    scope = "linodes:read_write"
    got = parse_pat_scopes(f"{scope} {scope} volumes:read_only")
    assert len(got) == 2
    assert Scope.LinodesReadWrite in got
    assert Scope.VolumesReadOnly in got


def test_parse_pat_scopes_preserves_wildcard() -> None:
    """The all-access marker ``*`` parses to Scope.Wildcard."""
    assert parse_pat_scopes("*") == [Scope.Wildcard]


def test_parse_pat_scopes_skips_unknown() -> None:
    """Unknown scope strings are skipped (loader logs them as excess).

    A future Linode scope the catalog doesn't yet recognize must not
    crash the parser. The token still works at runtime against the
    API; we just can't compare it.
    """
    got = parse_pat_scopes("linodes:read_only future:unknown_perm")
    assert got == [Scope.LinodesReadOnly]


def test_flatten_grants_nil() -> None:
    """A None grants payload yields an empty list, not an exception."""
    assert flatten_grants(None) == []


def test_flatten_grants_empty() -> None:
    """A PAT returning an empty Grants payload flattens to no scopes."""
    assert flatten_grants(Grants()) == []


def test_flatten_grants_global_account_access() -> None:
    """AccountAccess maps to the matching account scope pair.

    ``read_write`` implies both account:read_only and account:read_write;
    ``read_only`` implies only account:read_only.
    """
    rw = flatten_grants(Grants(global_=GlobalGrants(account_access="read_write")))
    assert Scope.AccountReadOnly in rw
    assert Scope.AccountReadWrite in rw

    ro = flatten_grants(Grants(global_=GlobalGrants(account_access="read_only")))
    assert Scope.AccountReadOnly in ro
    assert Scope.AccountReadWrite not in ro, "read_only must not imply write"


def test_flatten_grants_add_flags() -> None:
    """Every ``add_<resource>`` boolean adds the matching :read_write."""
    got = flatten_grants(
        Grants(
            global_=GlobalGrants(
                add_linodes=True,
                add_domains=True,
                add_firewalls=True,
                add_images=True,
                add_nodebalancers=True,
                add_stackscripts=True,
                add_volumes=True,
                add_vpcs=True,
            )
        )
    )

    for want in (
        Scope.LinodesReadWrite,
        Scope.DomainsReadWrite,
        Scope.FirewallReadWrite,
        Scope.ImagesReadWrite,
        Scope.NodeBalancersReadWrite,
        Scope.StackScriptsReadWrite,
        Scope.VolumesReadWrite,
        Scope.VPCReadWrite,
    ):
        assert want in got


def test_flatten_grants_per_resource() -> None:
    """Per-resource grants imply the matching category scope.

    ``read_write`` adds both :read_only and :read_write;
    ``read_only`` adds only :read_only; empty permission adds nothing.
    """
    got = flatten_grants(
        Grants(
            linode=[Grant(id=1, label="web-1", permissions="read_write")],
            domain=[Grant(id=1, label="example.com", permissions="read_only")],
            volume=[Grant(id=1, label="data", permissions="")],
        )
    )

    assert Scope.LinodesReadWrite in got
    assert Scope.LinodesReadOnly in got
    assert Scope.DomainsReadOnly in got
    assert Scope.DomainsReadWrite not in got, (
        "read_only domain grant must not imply :read_write"
    )
    assert Scope.VolumesReadOnly not in got, "empty permission must contribute nothing"


def test_flatten_grants_dedupes_resources() -> None:
    """Multiple grants on the same resource collapse to one scope each."""
    got = flatten_grants(
        Grants(
            linode=[
                Grant(id=1, label="a", permissions="read_write"),
                Grant(id=2, label="b", permissions="read_write"),
                Grant(id=3, label="c", permissions="read_only"),
            ]
        )
    )

    linode_scopes = [
        s for s in got if s in {Scope.LinodesReadWrite, Scope.LinodesReadOnly}
    ]
    assert len(linode_scopes) == 2, (
        "should produce exactly two linode scopes (no duplicates)"
    )


def test_compare_scopes_all_present() -> None:
    """When the token has every required scope, nothing is missing or excess."""
    got = compare_scopes(
        [Scope.LinodesReadOnly, Scope.VolumesReadOnly],
        [Scope.LinodesReadOnly, Scope.VolumesReadOnly],
    )
    assert not got.has_missing
    assert not got.has_excess


def test_compare_scopes_missing_reports_gap() -> None:
    """The missing set is sorted and contains only required-but-absent scopes."""
    got = compare_scopes(
        [
            Scope.LinodesReadWrite,
            Scope.VolumesReadOnly,
            Scope.DomainsReadWrite,
        ],
        [Scope.LinodesReadWrite],
    )
    assert got.has_missing
    assert got.missing == (Scope.DomainsReadWrite, Scope.VolumesReadOnly)


def test_compare_scopes_excess_is_least_privilege_signal() -> None:
    """Extra scopes on the token surface as excess (warn, not fail)."""
    got = compare_scopes(
        [Scope.LinodesReadOnly],
        [Scope.LinodesReadOnly, Scope.VolumesReadWrite],
    )
    assert not got.has_missing
    assert got.has_excess
    assert got.excess == (Scope.VolumesReadWrite,)


def test_compare_scopes_wildcard_matches_everything() -> None:
    """A token carrying only ``*`` satisfies any required scope."""
    got = compare_scopes(
        [
            Scope.LinodesReadWrite,
            Scope.VolumesReadWrite,
            Scope.DomainsReadWrite,
        ],
        [Scope.Wildcard],
    )
    assert not got.has_missing, "wildcard token must satisfy any required scope"
    assert not got.has_excess, (
        "wildcard alone is not 'excess' since it is the literal grant"
    )


def test_compare_scopes_required_wildcard_is_no_op() -> None:
    """``*`` in the required list is ignored, not treated as a real scope."""
    got = compare_scopes(
        [Scope.Wildcard, Scope.LinodesReadOnly],
        [Scope.LinodesReadOnly],
    )
    assert not got.has_missing


def test_compare_scopes_empty_required_always_passes() -> None:
    """A profile with no required scopes never reports missing entries.

    Any token scope still surfaces as excess (least privilege violated).
    """
    got = compare_scopes([], [Scope.LinodesReadWrite])
    assert not got.has_missing
    assert got.has_excess
