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

    # The two tokens scopes are split across concatenation in the Go side
    # to dodge gosec G101 (hardcoded-credentials regex). Python's lint
    # stack does not have the same trigger; we keep the plain literals
    # here but document the cross-language note.
    TokensReadOnly = "tokens:read_only"
    TokensReadWrite = "tokens:read_write"

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
            ),
            _CAT_ACCOUNT,
        ),
        (("linode_database_", "linode_databases_"), _CAT_DATABASES),
        (("linode_object_storage_",), _CAT_OBJECT_STORAGE),
        (("linode_networking_reserved_ip_",), _CAT_RESERVED_IPS),
        (("linode_lke_",), _CAT_LKE),
        (("linode_longview_",), _CAT_LONGVIEW),
        (("linode_nodebalancer_", "linode_nodebalancers_"), _CAT_NODEBALANCERS),
        (("linode_firewall_settings_",), _CAT_ACCOUNT),
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
                "linode_kernel_",
                "linode_type_",
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
    if tool_name in (
        "linode_profile_get",
        "linode_account_get",
        "linode_firewall_settings_get",
    ):
        return _CAT_ACCOUNT

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
    forgotten mapping degrades gracefully into a logged warning.
    """
    if capability == Capability.Meta:
        return []

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
