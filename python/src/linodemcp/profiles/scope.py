"""Linode token scope catalog and per-tool scope mapping.

Mirrors ``go/internal/profiles/scope.go``. The Linode API documents scopes
as ``<resource>:<permission>`` pairs (e.g. ``linodes:read_only``,
``volumes:read_write``). Personal access tokens carry their scopes as a
space-delimited string in the ``/profile`` response; OAuth tokens express
the same information through the structured ``/profile/grants`` response.
Phase 6's loader compares the active profile's required scopes against the
token's actual scopes and fails (or warns) on mismatch.

Cross-language parity: the Go and Python catalogs must agree on the exact
string values for each constant (a Phase 6 parity test verifies this).
"""

from __future__ import annotations

from enum import StrEnum

from linodemcp.profiles.capability import Capability


class Scope(StrEnum):
    """Linode OAuth/PAT scope strings.

    Values match the Linode API exactly so they round-trip through the
    /profile and /profile/grants responses without translation. Adding a
    new scope is additive: unknown ones flow through as plain strings
    and the loader logs a warning rather than failing.
    """

    Wildcard = "*"

    AccountReadOnly = "account:read_only"
    AccountReadWrite = "account:read_write"

    DatabasesReadOnly = "databases:read_only"
    DatabasesReadWrite = "databases:read_write"

    DomainsReadOnly = "domains:read_only"
    DomainsReadWrite = "domains:read_write"

    EventsReadOnly = "events:read_only"
    EventsReadWrite = "events:read_write"

    FirewallReadOnly = "firewall:read_only"
    FirewallReadWrite = "firewall:read_write"

    ImagesReadOnly = "images:read_only"
    ImagesReadWrite = "images:read_write"

    IPsReadOnly = "ips:read_only"
    IPsReadWrite = "ips:read_write"

    ReservedIPsReadOnly = "reserved-ips:read_only"
    ReservedIPsReadWrite = "reserved-ips:read_write"

    LinodesReadOnly = "linodes:read_only"
    LinodesReadWrite = "linodes:read_write"

    LKEReadOnly = "lke:read_only"
    LKEReadWrite = "lke:read_write"

    LongviewReadOnly = "longview:read_only"
    LongviewReadWrite = "longview:read_write"

    MaintenanceReadOnly = "maintenance:read_only"
    MaintenanceReadWrite = "maintenance:read_write"

    NodeBalancersReadOnly = "nodebalancers:read_only"
    NodeBalancersReadWrite = "nodebalancers:read_write"

    ObjectStorageReadOnly = "object_storage:read_only"
    ObjectStorageReadWrite = "object_storage:read_write"

    StackScriptsReadOnly = "stackscripts:read_only"
    StackScriptsReadWrite = "stackscripts:read_write"

    UsersReadOnly = "users:read_only"
    UsersReadWrite = "users:read_write"

    VolumesReadOnly = "volumes:read_only"
    VolumesReadWrite = "volumes:read_write"

    VPCReadOnly = "vpc:read_only"
    VPCReadWrite = "vpc:read_write"


# Scope category names. Internal markers used by the prefix mapping; they
# round-trip into scope strings via _scope_for. Extracting them as constants
# keeps the prefix dispatcher and the scope catalog from drifting on the
# literal spelling.
_CAT_ACCOUNT = "account"
_CAT_DATABASES = "databases"
_CAT_DOMAINS = "domains"
_CAT_FIREWALL = "firewall"
_CAT_IMAGES = "images"
_CAT_IPS = "ips"
_CAT_LINODES = "linodes"
_CAT_LKE = "lke"
_CAT_LONGVIEW = "longview"
_CAT_NODEBALANCERS = "nodebalancers"
_CAT_OBJECT_STORAGE = "object_storage"
_CAT_RESERVED_IPS = "reserved-ips"
_CAT_STACKSCRIPTS = "stackscripts"
_CAT_VOLUMES = "volumes"
_CAT_VPC = "vpc"


def _scope_matrix() -> dict[str, tuple[Scope, Scope]]:
    """Return the read-only/read-write Scope pair for each category.

    Rebuilt per call so the table doesn't sit as a module-level mutable
    (matches Go's `scopeMatrix()` helper). Index 0 is read-only;
    index 1 is read-write.
    """
    return {
        _CAT_ACCOUNT: (Scope.AccountReadOnly, Scope.AccountReadWrite),
        _CAT_DATABASES: (Scope.DatabasesReadOnly, Scope.DatabasesReadWrite),
        _CAT_DOMAINS: (Scope.DomainsReadOnly, Scope.DomainsReadWrite),
        _CAT_FIREWALL: (Scope.FirewallReadOnly, Scope.FirewallReadWrite),
        _CAT_IMAGES: (Scope.ImagesReadOnly, Scope.ImagesReadWrite),
        _CAT_IPS: (Scope.IPsReadOnly, Scope.IPsReadWrite),
        _CAT_LINODES: (Scope.LinodesReadOnly, Scope.LinodesReadWrite),
        _CAT_LKE: (Scope.LKEReadOnly, Scope.LKEReadWrite),
        _CAT_LONGVIEW: (Scope.LongviewReadOnly, Scope.LongviewReadWrite),
        _CAT_NODEBALANCERS: (
            Scope.NodeBalancersReadOnly,
            Scope.NodeBalancersReadWrite,
        ),
        _CAT_OBJECT_STORAGE: (
            Scope.ObjectStorageReadOnly,
            Scope.ObjectStorageReadWrite,
        ),
        _CAT_RESERVED_IPS: (
            Scope.ReservedIPsReadOnly,
            Scope.ReservedIPsReadWrite,
        ),
        _CAT_STACKSCRIPTS: (
            Scope.StackScriptsReadOnly,
            Scope.StackScriptsReadWrite,
        ),
        _CAT_VOLUMES: (Scope.VolumesReadOnly, Scope.VolumesReadWrite),
        _CAT_VPC: (Scope.VPCReadOnly, Scope.VPCReadWrite),
    }


def _prefix_table() -> list[tuple[tuple[str, ...], str]]:
    """Return the prefix-to-category dispatch table.

    Order matters: longer or more specific prefixes appear first. SSH
    keys and monitor tools fold into the account category since both
    live under account-scoped endpoints. The function returns a fresh
    list per call so the data doesn't sit as module-level mutable.
    """
    return [
        (
            (
                "linode_account_",
                "linode_managed_",
                "linode_tag_",
                "linode_support_ticket_",
                # The whole profile subtree is account-gated in the API
                # docs, including /profile/tokens which the docs gate
                # with account:* rather than a dedicated tokens scope.
                "linode_profile_",
            ),
            _CAT_ACCOUNT,
        ),
        (("linode_database_", "linode_databases_"), _CAT_DATABASES),
        (("linode_object_storage_",), _CAT_OBJECT_STORAGE),
        (("linode_networking_reserved_ip_",), _CAT_RESERVED_IPS),
        # The /networking/ips, /networking/ipv4, and /networking/ipv6
        # routes all sit on ips:* scopes. Reserved-ip tools never reach
        # this row: the longer reserved-ip prefix above wins first.
        (
            (
                "linode_networking_ip_",
                "linode_networking_ipv4_",
                "linode_ipv6_",
            ),
            _CAT_IPS,
        ),
        (("linode_lke_",), _CAT_LKE),
        (("linode_longview_",), _CAT_LONGVIEW),
        (("linode_nodebalancer_", "linode_nodebalancers_"), _CAT_NODEBALANCERS),
        (("linode_firewall_", "linode_firewalls_"), _CAT_FIREWALL),
        (("linode_domain_", "linode_domains_"), _CAT_DOMAINS),
        (("linode_volume_", "linode_volumes_"), _CAT_VOLUMES),
        (("linode_stackscript_", "linode_stackscripts_"), _CAT_STACKSCRIPTS),
        (("linode_vpc_", "linode_vpcs_"), _CAT_VPC),
        (("linode_images_", "linode_image_"), _CAT_IMAGES),
        (
            ("linode_monitor_", "linode_sshkey_", "linode_sshkeys_"),
            _CAT_ACCOUNT,
        ),
        (
            (
                "linode_instance_",
                "linode_instances_",
                "linode_placement_group_",
                "linode_placement_groups_",
                "linode_region_",
                "linode_type_",
                # VLANs live under /networking/vlans but the API gates
                # them with linodes:* scopes.
                "linode_vlan_",
            ),
            _CAT_LINODES,
        ),
    ]


def _scope_category(tool_name: str) -> str | None:
    """Return the Linode scope category for ``tool_name``, or ``None``.

    Match order matters: longer prefixes are checked first so e.g.
    ``linode_instance_backup_*`` routes via the instance/linodes path
    rather than getting shadowed by a broader rule.
    """
    # The API's security metadata intentionally splits this route family:
    # create and collection-list use ips:*, while get, update, type-list,
    # and delete use reserved-ips:*. Keep only the two collection overrides
    # here; the item and pricing tools resolve through _prefix_table below.
    if tool_name in (
        "linode_networking_reserved_ip_create",
        "linode_networking_reserved_ip_list",
    ):
        return _CAT_IPS

    for prefixes, category in _prefix_table():
        if tool_name.startswith(prefixes):
            return category

    return None


def _scope_for(category: str, capability: Capability) -> Scope | None:
    """Map a category and capability to its Scope.

    Read-only capabilities pair with the :read_only scope; mutators
    (Write, Destroy, Admin) pair with :read_write since the Linode API
    does not distinguish further.
    """
    pair = _scope_matrix().get(category)
    if pair is None:
        return None
    if capability == Capability.Read:
        return pair[0]
    return pair[1]


def _is_scopeless_route(tool_name: str) -> bool:
    """Report whether the tool's route is documented with no OAuth scope.

    Two flavors collapse to the same empty return: public catalog routes
    that need no authentication at all (kernels, database engines and
    types), and token-only routes documented with an empty scope list,
    meaning any authenticated token may call them (betas, maintenance
    policies). Listing them explicitly separates "documented as
    scopeless" from "forgot to map", which the scope completeness test
    relies on.
    """
    return tool_name in (
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
    )


def _scope_overrides() -> dict[str, list[Scope]]:
    """Pin tools whose documented scope can't come from the matrix.

    Each entry mirrors the security block of the underlying operation in
    the Linode OpenAPI spec, which splits these routes away from the
    rest of their family.

    Two documented quirks are deliberately NOT mirrored here.
    GET /managed/contacts/{contactId} documents account:read_write on a
    read: encoding that would make the read-only builtin profiles carry
    a write scope and flip the missing-token elevation policy for
    everyone on the default profile. GET /placement/groups documents
    placement:read_only, a scope the spec's own OAuth catalog never
    defines, so tokens may not be able to carry it at all. Both tools
    stay on their family derivation until the upstream docs and the
    grantable scope set agree.
    """
    return {
        # PUT /networking/firewalls/settings is documented as
        # account:read_write while GET on the same route stays
        # firewall:read_only.
        "linode_firewall_settings_update": [Scope.AccountReadWrite],
        # GET .../instances/{id}/credentials is documented as
        # databases:read_only on both engines even though the tools
        # register as mutators (they expose secrets, so the server
        # gates them behind a stronger capability).
        "linode_database_mysql_instance_credentials_get": [Scope.DatabasesReadOnly],
        "linode_database_postgresql_instance_credentials_get": [
            Scope.DatabasesReadOnly
        ],
    }


def _additional_scopes(tool_name: str) -> list[Scope]:
    """Return secondary scopes a tool needs beyond its primary category.

    Some Linode endpoints touch multiple resource categories (creating a
    Linode from an image needs ``images:read_only`` alongside
    ``linodes:read_write``). Most tools return ``[]``.
    """
    if tool_name in (
        "linode_instance_create",
        "linode_instance_clone",
        "linode_instance_rebuild",
    ):
        return [Scope.ImagesReadOnly]
    if tool_name == "linode_lke_cluster_create":
        return [Scope.LinodesReadWrite]
    if tool_name in (
        # Assigning or sharing addresses targets Linodes, so the API
        # documents linodes:read_write alongside ips:read_write.
        "linode_networking_ip_assign",
        "linode_networking_ip_share",
        "linode_networking_ipv4_assign",
        "linode_networking_ipv4_share",
        "linode_ipv6_range_create",
    ):
        return [Scope.LinodesReadWrite]
    return []


def required_scopes(tool_name: str, capability: Capability) -> list[Scope]:
    """Return the Linode scope(s) a tool needs the active token to carry.

    The mapping is name-prefix based, mirroring how ``categorize()`` in
    builtin.py assigns tools to profile categories. The capability tells
    whether the tool reads or writes, which decides between :read_only
    and :read_write.

    Meta tools (``hello``, ``version``) return an empty list: they touch
    no Linode API. An empty return means "no scope required".

    Unknown tool names return an empty list too. The Phase 6.4 loader
    treats unknown names as best effort rather than a hard failure so a
    forgotten mapping degrades gracefully into a logged warning. The
    scope completeness test closes the remaining gap: every registered
    tool must resolve to a non-empty scope list or sit on the documented
    scopeless list, so a forgotten mapping fails tests instead of
    shipping as silently unrestricted.
    """
    if capability == Capability.Meta or _is_scopeless_route(tool_name):
        return []

    override = _scope_overrides().get(tool_name)
    if override is not None:
        return override

    category = _scope_category(tool_name)
    if category is None:
        return []

    scope = _scope_for(category, capability)
    if scope is None:
        return []

    extras = _additional_scopes(tool_name)
    if not extras:
        return [scope]
    return [scope, *extras]
