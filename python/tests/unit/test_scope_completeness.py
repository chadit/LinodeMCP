"""Completeness guard for the per-tool scope mapping.

Closes the silent gap the scope parity gate cannot see: when BOTH
languages forget to map a tool family, the dumps agree on empty and
parity passes. ``required_scopes`` returns an empty list for unknown
names by design (the loader degrades to a warning), so without this test
a forgotten mapping ships as a tool no profile scope-check ever
restricts. Mirrors ``go/internal/server/scope_completeness_test.go``.
"""

from __future__ import annotations

from linodemcp.profiles import Capability, required_scopes
from linodemcp.server import get_tool_registry

# Mirrors the documented scopeless list in _is_scopeless_route: public
# catalog routes (kernels, database engines and types, network transfer
# prices) plus token-only routes documented with an empty scope list
# (betas, maintenance policies). Keep the two lists in step; the test
# fails in both directions when they drift.
_SCOPELESS_TOOLS = frozenset(
    {
        "linode_kernel_get",
        "linode_kernel_list",
        "linode_database_engine_get",
        "linode_database_engine_list",
        "linode_database_type_get",
        "linode_database_type_list",
        "linode_network_transfer_price_list",
        "linode_beta_get",
        "linode_beta_list",
        "linode_maintenance_policy_list",
    }
)


def test_every_registered_tool_resolves_scopes() -> None:
    """Every non-meta tool maps to scopes or sits on the scopeless list.

    Also checks the inverse: every scopeless entry must both exist in
    the registry and still resolve empty, so the list cannot go stale.
    """
    seen: set[str] = set()

    for entry in get_tool_registry():
        if entry.capability == Capability.Meta:
            continue

        scopes = required_scopes(entry.name, entry.capability)

        if entry.name in _SCOPELESS_TOOLS:
            seen.add(entry.name)
            assert scopes == [], (
                f"{entry.name} is on the scopeless list but resolves "
                f"{scopes}; remove the stale entry"
            )
            continue

        assert scopes, (
            f"{entry.name} (capability {entry.capability.name}) resolves "
            "no scope; map it in scope.py or add it to the documented "
            "scopeless list"
        )

    missing = _SCOPELESS_TOOLS - seen
    assert not missing, (
        f"scopeless entries are not registered tools: {sorted(missing)}; "
        "fix the names or drop them"
    )
