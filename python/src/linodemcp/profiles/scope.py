"""Linode token scope vocabulary.

Mirrors ``go/internal/profiles/scope.go``. The Linode API documents scopes
as ``<resource>:<permission>`` pairs (e.g. ``linodes:read_only``,
``volumes:read_write``). Personal access tokens carry their scopes as a
space-delimited string in the ``/profile`` response; OAuth tokens express
the same information through the structured ``/profile/grants`` response.
Phase 6's loader compares the active profile's required scopes against the
token's actual scopes and fails (or warns) on mismatch.

Per-tool required scopes are declared in the proto contract and served by
the generated registry tables (``linodemcp.gentools.scopes_for``), so this
module only spells the values token parsing compares against.

Cross-language parity: the Go and Python catalogs must agree on the exact
string values for each constant.
"""

from __future__ import annotations

from enum import StrEnum


class Scope(StrEnum):
    """Linode OAuth/PAT scope strings.

    The wildcard PAT scope plus the pairs the grants converter in
    scopecheck.py names. Values match the Linode API exactly so they
    round-trip through the /profile and /profile/grants responses without
    translation; unknown ones flow through as plain strings and the
    loader logs a warning rather than failing.
    """

    Wildcard = "*"

    AccountReadOnly = "account:read_only"
    AccountReadWrite = "account:read_write"

    DatabasesReadOnly = "databases:read_only"
    DatabasesReadWrite = "databases:read_write"

    DomainsReadOnly = "domains:read_only"
    DomainsReadWrite = "domains:read_write"

    FirewallReadOnly = "firewall:read_only"
    FirewallReadWrite = "firewall:read_write"

    ImagesReadOnly = "images:read_only"
    ImagesReadWrite = "images:read_write"

    LinodesReadOnly = "linodes:read_only"
    LinodesReadWrite = "linodes:read_write"

    LKEReadOnly = "lke:read_only"
    LKEReadWrite = "lke:read_write"

    LongviewReadOnly = "longview:read_only"
    LongviewReadWrite = "longview:read_write"

    NodeBalancersReadOnly = "nodebalancers:read_only"
    NodeBalancersReadWrite = "nodebalancers:read_write"

    StackScriptsReadOnly = "stackscripts:read_only"
    StackScriptsReadWrite = "stackscripts:read_write"

    VolumesReadOnly = "volumes:read_only"
    VolumesReadWrite = "volumes:read_write"

    VPCReadOnly = "vpc:read_only"
    VPCReadWrite = "vpc:read_write"
