"""Linode API client."""

import asyncio
import base64
import enum
import ipaddress
import logging
import re
import secrets
import threading
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from dataclasses import field as dc_field
from datetime import datetime
from pathlib import Path
from typing import Any, BinaryIO, TypeGuard, TypeVar, cast
from urllib.parse import quote, urlencode

import httpx

T = TypeVar("T")

logger = logging.getLogger(__name__)

_PLACEMENT_GROUP_LABEL_PATTERN = re.compile(
    r"^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$"
)

# Validation patterns
VALID_SSH_KEY_PREFIXES = (
    "ssh-rsa",
    "ssh-ed25519",
    "ecdsa-sha2-nistp256",
    "ecdsa-sha2-nistp384",
    "ecdsa-sha2-nistp521",
    "ssh-dss",
)

VALID_DNS_NAME_PATTERN = re.compile(
    r"^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?"
    r"(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$|^@$|^$"
)

# Validation constants
MIN_SSH_KEY_LENGTH = 80
MAX_SSH_KEY_LENGTH = 16000
MIN_PASSWORD_LENGTH = 12
MAX_PASSWORD_LENGTH = 128
MAX_DNS_NAME_LENGTH = 253
MIN_VOLUME_SIZE_GB = 10
MAX_VOLUME_SIZE_GB = 10240
MAX_LABEL_LENGTH = 64
MAX_PROFILE_TOKEN_LABEL_LENGTH = 100
PROFILE_SECURITY_QUESTION_COUNT = 3
MIN_PROFILE_SECURITY_RESPONSE_LENGTH = 3
MAX_PROFILE_SECURITY_RESPONSE_LENGTH = 17
MIN_DISK_SIZE_MB = 1
MAX_DISK_SIZE_MB = 524288
MIN_PAGE_SIZE = 25
MAX_PAGE_SIZE = 500


def validate_disk_size(size: int) -> None:
    """Validate disk size in MB."""
    if size < MIN_DISK_SIZE_MB:
        msg = "disk size must be at least 1 MB"
        raise ValueError(msg)
    if size > MAX_DISK_SIZE_MB:
        msg = "disk size cannot exceed 524288 MB (512 GB)"
        raise ValueError(msg)


def validate_ssh_key(key: str) -> None:
    """Validate SSH key format."""
    if not key:
        msg = "ssh_key is required"
        raise ValueError(msg)

    key = key.strip()
    if not any(key.startswith(f"{prefix} ") for prefix in VALID_SSH_KEY_PREFIXES):
        msg = "invalid SSH key format: use ssh-rsa, ssh-ed25519, or ecdsa-sha2-*"
        raise ValueError(msg)

    if len(key) < MIN_SSH_KEY_LENGTH or len(key) > MAX_SSH_KEY_LENGTH:
        msg = "invalid SSH key length: key appears malformed"
        raise ValueError(msg)


def validate_root_password(password: str | None) -> None:
    """Validate root password strength."""
    if not password:
        return  # Password is optional

    if len(password) < MIN_PASSWORD_LENGTH:
        msg = "root_pass must be at least 12 characters"
        raise ValueError(msg)

    if len(password) > MAX_PASSWORD_LENGTH:
        msg = "root_pass must not exceed 128 characters"
        raise ValueError(msg)

    has_upper = any(c.isupper() for c in password)
    has_lower = any(c.islower() for c in password)
    has_digit = any(c.isdigit() for c in password)

    if not (has_upper and has_lower and has_digit):
        msg = "root_pass must contain uppercase, lowercase, and digits"
        raise ValueError(msg)


def validate_dns_record_name(name: str) -> None:
    """Validate DNS record name."""
    if len(name) > MAX_DNS_NAME_LENGTH:
        msg = "DNS record name exceeds maximum length of 253 characters"
        raise ValueError(msg)

    if name and name != "@" and not VALID_DNS_NAME_PATTERN.match(name):
        msg = "invalid DNS record name: alphanumeric, hyphens, dots only"
        raise ValueError(msg)


def validate_dns_record_target(record_type: str, target: str) -> None:
    """Validate DNS record target based on type."""
    if not target:
        msg = "target is required"
        raise ValueError(msg)

    record_type = record_type.upper()

    if record_type == "A":
        try:
            ip = ipaddress.ip_address(target)
        except ValueError:
            msg = "A record target must be a valid IPv4 address"
            raise ValueError(msg) from None

        if not isinstance(ip, ipaddress.IPv4Address):
            msg = "A record target must be a valid IPv4 address"
            raise ValueError(msg)

        if ip.is_private or ip.is_loopback:
            msg = "A record target cannot be a private IP address"
            raise ValueError(msg)


def validate_firewall_policy(policy: str) -> None:
    """Validate firewall policy value."""
    if policy.upper() not in ("ACCEPT", "DROP"):
        msg = f"firewall policy must be 'ACCEPT' or 'DROP', got '{policy}'"
        raise ValueError(msg)


def validate_volume_size(size: int) -> None:
    """Validate volume size."""
    if size < MIN_VOLUME_SIZE_GB:
        msg = "volume size must be at least 10 GB"
        raise ValueError(msg)
    if size > MAX_VOLUME_SIZE_GB:
        msg = "volume size cannot exceed 10240 GB (10 TB)"
        raise ValueError(msg)


def _is_firewall_rule_list(value: Any) -> TypeGuard[list[dict[str, Any]]]:
    if not isinstance(value, list):
        return False
    rules = cast("list[object]", value)
    return all(isinstance(rule, dict) for rule in rules)


def _validate_firewall_rules_update_request(
    firewall_id: Any, inbound: Any, outbound: Any
) -> None:
    """Validate a firewall rules update request."""
    if (
        not isinstance(firewall_id, int)
        or isinstance(firewall_id, bool)
        or firewall_id <= 0
    ):
        msg = "firewall_id must be a positive integer"
        raise ValueError(msg)
    if not _is_firewall_rule_list(inbound):
        msg = "inbound must be a list of rule objects"
        raise TypeError(msg)
    if not _is_firewall_rule_list(outbound):
        msg = "outbound must be a list of rule objects"
        raise TypeError(msg)


def validate_label(label: str | None) -> None:
    """Validate resource label."""
    if not label:
        return

    if len(label) > MAX_LABEL_LENGTH:
        msg = "label must not exceed 64 characters"
        raise ValueError(msg)

    for char in label:
        if not (char.isalnum() or char in "_-."):
            msg = f"label contains invalid character '{char}'"
            raise ValueError(msg)


def build_profile_token_create_body(
    expiry: str | None = None,
    label: str | None = None,
    scopes: str | None = None,
) -> dict[str, Any]:
    """Validate profile token create fields and build the request body."""
    body: dict[str, Any] = {}
    if expiry is not None:
        if not expiry.strip():
            msg = "expiry must be a non-empty ISO 8601 timestamp or null"
            raise ValueError(msg)
        try:
            datetime.fromisoformat(expiry)
        except ValueError as exc:
            msg = "expiry must be a valid ISO 8601 timestamp"
            raise ValueError(msg) from exc
        body["expiry"] = expiry
    if label is not None:
        if not label.strip():
            msg = "label must be a non-empty string"
            raise ValueError(msg)
        if len(label) > MAX_PROFILE_TOKEN_LABEL_LENGTH:
            msg = "label must be 100 characters or fewer"
            raise ValueError(msg)
        body["label"] = label
    if scopes is not None:
        if not scopes.strip():
            msg = "scopes must be a non-empty string"
            raise ValueError(msg)
        body["scopes"] = scopes
    return body


def build_profile_security_questions_body(
    security_questions: object,
) -> dict[str, Any]:
    """Validate profile security question answers and build request body."""
    if not isinstance(security_questions, list):
        msg = "security_questions must be a list"
        raise TypeError(msg)
    question_items = cast("list[object]", security_questions)
    if len(question_items) != PROFILE_SECURITY_QUESTION_COUNT:
        msg = "security_questions must contain exactly 3 answers"
        raise ValueError(msg)
    body_questions: list[dict[str, Any]] = []
    question_ids: set[int] = set()
    for item in question_items:
        if not isinstance(item, dict):
            msg = "security_questions entries must be objects"
            raise TypeError(msg)
        question = cast("dict[str, object]", item)

        question_id = question.get("question_id")
        if (
            isinstance(question_id, bool)
            or not isinstance(question_id, int)
            or question_id < 1
        ):
            msg = "question_id must be a positive integer"
            raise ValueError(msg)
        if question_id in question_ids:
            msg = "security_questions question_id values must be unique"
            raise ValueError(msg)
        question_ids.add(question_id)

        response = question.get("response")
        if not isinstance(response, str):
            msg = "response must be a string"
            raise TypeError(msg)
        if (
            len(response) < MIN_PROFILE_SECURITY_RESPONSE_LENGTH
            or len(response) > MAX_PROFILE_SECURITY_RESPONSE_LENGTH
        ):
            msg = "response length must be between 3 and 17 characters"
            raise ValueError(msg)

        body_questions.append({"question_id": question_id, "response": response})

    return {"security_questions": body_questions}


# HTTP status code constants
HTTP_BAD_REQUEST = 400
HTTP_UNAUTHORIZED = 401
HTTP_FORBIDDEN = 403
HTTP_TOO_MANY_REQUESTS = 429
HTTP_SERVER_ERROR = 500
HTTP_SERVER_ERROR_MAX = 600

__all__ = [
    "UDF",
    "VPC",
    "VPCIP",
    "APIError",
    "Account",
    "Addons",
    "Alerts",
    "Backup",
    "Backups",
    "BackupsAddon",
    "Client",
    "Domain",
    "DomainRecord",
    "Firewall",
    "FirewallAddresses",
    "FirewallRule",
    "FirewallRules",
    "Image",
    "Instance",
    "InstanceType",
    "LKEAPIEndpoint",
    "LKECluster",
    "LKEControlPlane",
    "LKEControlPlaneACL",
    "LKEControlPlaneACLAddresses",
    "LKEDashboard",
    "LKEKubeconfig",
    "LKENode",
    "LKENodePool",
    "LKENodePoolAutoscaler",
    "LKENodePoolDisk",
    "LKERegionPrice",
    "LKETierVersion",
    "LKEType",
    "LKETypePrice",
    "LKEVersion",
    "LinodeError",
    "NetworkError",
    "NodeBalancer",
    "Price",
    "Profile",
    "Promo",
    "RateLimiter",
    "Region",
    "Resolver",
    "RetryConfig",
    "RetryableClient",
    "RetryableError",
    "SSHKey",
    "Schedule",
    "Specs",
    "StackScript",
    "Transfer",
    "VPCSubnet",
    "Volume",
    "is_retryable",
    "validate_disk_size",
    "validate_dns_record_name",
    "validate_dns_record_target",
    "validate_firewall_policy",
    "validate_label",
    "validate_root_password",
    "validate_ssh_key",
    "validate_volume_size",
]


class LinodeError(Exception):
    """Base Linode error."""


class CircuitOpenError(LinodeError):
    """Raised when the circuit breaker is open and rejecting requests.

    Callers can catch this specifically to distinguish "we never tried"
    from "we tried and the upstream failed".
    """


class APIError(LinodeError):
    """Linode API error."""

    def __init__(self, status_code: int, message: str, field: str = "") -> None:
        self.status_code = status_code
        self.message = message
        self.field = field
        super().__init__(self._format_message())

    def _format_message(self) -> str:
        if self.field:
            return (
                f"Linode API error (status {self.status_code}): "
                f"{self.message} (field: {self.field})"
            )
        return f"Linode API error (status {self.status_code}): {self.message}"

    def is_authentication_error(self) -> bool:
        """Check if this is an authentication error."""
        return self.status_code == HTTP_UNAUTHORIZED

    def is_rate_limit_error(self) -> bool:
        """Check if this is a rate limit error."""
        return self.status_code == HTTP_TOO_MANY_REQUESTS

    def is_forbidden_error(self) -> bool:
        """Check if this is a forbidden error."""
        return self.status_code == HTTP_FORBIDDEN

    def is_server_error(self) -> bool:
        """Check if this is a server error."""
        return HTTP_SERVER_ERROR <= self.status_code < HTTP_SERVER_ERROR_MAX


class NetworkError(LinodeError):
    """Network-related error."""

    def __init__(self, operation: str, error: Exception) -> None:
        self.operation = operation
        self.error = error
        super().__init__(f"network error during {operation}: {error}")


class RetryableError(LinodeError):
    """Error that can be retried."""

    def __init__(self, error: Exception, retry_after: float = 0) -> None:
        self.error = error
        self.retry_after = retry_after
        msg = f"retryable error: {error}"
        if retry_after > 0:
            msg = f"retryable error (retry after {retry_after}s): {error}"
        super().__init__(msg)


@dataclass
class Profile:
    """Linode user profile.

    ``scopes`` is populated for personal access tokens (the ``/profile``
    response includes the space-delimited scope string). OAuth tokens
    leave it empty; Phase 6 scope validation falls back to
    ``/profile/grants`` for those.
    """

    username: str
    email: str
    timezone: str
    email_notifications: bool
    restricted: bool
    two_factor_auth: bool
    uid: int
    scopes: str = ""


@dataclass
class Grant:
    """Permission an OAuth token has on a single Linode resource.

    The Linode API groups grants by resource category (linode, domain,
    nodebalancer, etc); each entry names a specific resource the token
    can touch. ``permissions`` is ``"read_only"``, ``"read_write"``, or
    an empty string when the OAuth grant carries no permission.
    """

    id: int
    label: str
    permissions: str


@dataclass
class GlobalGrants:
    """Account-level permission booleans for an OAuth token.

    Mirrors the Linode ``/profile/grants.global`` shape. Each capability
    is its own bool, matching the wire format so scope-comparison code
    can read them without magic-string lookups.
    """

    account_access: str = ""
    add_databases: bool = False
    add_domains: bool = False
    add_firewalls: bool = False
    add_images: bool = False
    add_linodes: bool = False
    add_longview: bool = False
    add_nodebalancers: bool = False
    add_stackscripts: bool = False
    add_volumes: bool = False
    add_vpcs: bool = False
    cancel_account: bool = False
    child_account_access: bool = False
    longview_subscription: bool = False


def _empty_grant_list() -> list[Grant]:
    """Typed factory for per-resource grant slices in Grants defaults.

    Plain ``default_factory=list`` resolves to ``list[Unknown]`` under
    pyright strict; this helper pins the element type so dataclass
    defaults round-trip cleanly through type checking.
    """
    return []


@dataclass
class Grants:
    """Full ``/profile/grants`` response for OAuth tokens.

    PATs always return an empty Grants object; their scope information is
    on ``Profile.scopes`` instead. Phase 6's profile loader checks both.
    """

    global_: GlobalGrants = dc_field(default_factory=GlobalGrants)
    linode: list[Grant] = dc_field(default_factory=_empty_grant_list)
    domain: list[Grant] = dc_field(default_factory=_empty_grant_list)
    nodebalancer: list[Grant] = dc_field(default_factory=_empty_grant_list)
    image: list[Grant] = dc_field(default_factory=_empty_grant_list)
    longview: list[Grant] = dc_field(default_factory=_empty_grant_list)
    stackscript: list[Grant] = dc_field(default_factory=_empty_grant_list)
    volume: list[Grant] = dc_field(default_factory=_empty_grant_list)
    database: list[Grant] = dc_field(default_factory=_empty_grant_list)
    firewall: list[Grant] = dc_field(default_factory=_empty_grant_list)
    vpc: list[Grant] = dc_field(default_factory=_empty_grant_list)
    lkecluster: list[Grant] = dc_field(default_factory=_empty_grant_list)


@dataclass
class Specs:
    """Instance hardware specifications."""

    disk: int
    memory: int
    vcpus: int
    gpus: int
    transfer: int


@dataclass
class Alerts:
    """Alert settings for an instance."""

    cpu: int
    network_in: int
    network_out: int
    transfer_quota: int
    io: int


@dataclass
class Schedule:
    """Backup schedule settings."""

    day: str
    window: str


@dataclass
class Backup:
    """Backup snapshot."""

    id: int
    label: str
    status: str
    type: str
    region: str
    created: str
    updated: str
    finished: str


@dataclass
class Backups:
    """Backup settings."""

    enabled: bool
    available: bool
    schedule: Schedule
    last_successful: Backup | None = None


# CURRENT_INTERFACE_GENERATION is the Linode Interfaces generation this codebase
# targets. The Linode API rejects POST /linode/instances payloads whose
# interface_generation does not match the account's enabled generation, so this
# constant is the single source of truth for the wire value. Mirrors the Go
# linode.CurrentInterfaceGeneration constant.
CURRENT_INTERFACE_GENERATION = "linode"


@dataclass
class InterfaceIPv4Address:
    """Single IPv4 address on an interface."""

    address: str
    primary: bool = False


@dataclass
class InterfaceIPv6Range:
    """IPv6 range on an interface."""

    range: str


@dataclass
class InterfacePublicIPv4:
    """Public IPv4 sub-config. Field set is conservative pending live capture."""

    addresses: list[InterfaceIPv4Address] = dc_field(
        default_factory=list[InterfaceIPv4Address]
    )


@dataclass
class InterfacePublicIPv6:
    """Public IPv6 sub-config. Field set is conservative pending live capture."""

    ranges: list[InterfaceIPv6Range] = dc_field(
        default_factory=list[InterfaceIPv6Range]
    )


@dataclass
class InterfacePublicConfig:
    """Public-interface configuration."""

    ipv4: InterfacePublicIPv4 | None = None
    ipv6: InterfacePublicIPv6 | None = None


@dataclass
class InterfaceVPCIPv4:
    """VPC IPv4 sub-config."""

    addresses: list[InterfaceIPv4Address] = dc_field(
        default_factory=list[InterfaceIPv4Address]
    )


@dataclass
class InterfaceVPCConfig:
    """VPC-attached-interface configuration."""

    subnet_id: int
    ipv4: InterfaceVPCIPv4 | None = None


@dataclass
class InterfaceVLANConfig:
    """VLAN-attached-interface configuration."""

    vlan_label: str
    ipam_address: str = ""


@dataclass
class InterfaceDefaultRoute:
    """Whether the interface owns the default route per address family. A
    family is sent only when True; False values are omitted from the wire so
    the API treats them as unset.
    """

    ipv4: bool = False
    ipv6: bool = False


@dataclass
class InstanceInterface:
    """Network interface on a Linode instance under the current Interfaces
    generation. Exactly one of public, vpc, or vlan is set per interface.
    """

    id: int = 0
    public: InterfacePublicConfig | None = None
    vpc: InterfaceVPCConfig | None = None
    vlan: InterfaceVLANConfig | None = None
    default_route: InterfaceDefaultRoute | None = None
    firewall_id: int | None = None
    mac_address: str = ""


@dataclass
class Instance:
    """Linode instance."""

    id: int
    label: str
    status: str
    type: str
    region: str
    image: str
    ipv4: list[str]
    ipv6: str
    hypervisor: str
    specs: Specs
    alerts: Alerts
    backups: Backups
    created: str
    updated: str
    group: str
    tags: list[str]
    watchdog_enabled: bool
    interface_generation: str = ""
    interfaces: list[InstanceInterface] = dc_field(
        default_factory=list[InstanceInterface]
    )


@dataclass
class Promo:
    """Active promotion on an account."""

    description: str
    summary: str
    credit_monthly_cap: str
    credit_remaining: str
    expire_dt: str
    image_url: str
    service_type: str
    this_month_credit_remaining: str


@dataclass
class Account:
    """Linode account."""

    first_name: str
    last_name: str
    email: str
    company: str
    address_1: str
    address_2: str
    city: str
    state: str
    zip: str
    country: str
    phone: str
    balance: float
    balance_uninvoiced: float
    capabilities: list[str]
    active_since: str
    euuid: str
    billing_source: str
    active_promotions: list[Promo]


@dataclass
class Resolver:
    """DNS resolvers for a region."""

    ipv4: str
    ipv6: str


@dataclass
class Region:
    """Linode region (datacenter)."""

    id: str
    label: str
    country: str
    capabilities: list[str]
    status: str
    resolvers: Resolver
    site_type: str


@dataclass
class Price:
    """Pricing for a Linode type."""

    hourly: float
    monthly: float


@dataclass
class BackupsAddon:
    """Backup add-on pricing."""

    price: Price


@dataclass
class Addons:
    """Add-on pricing for a Linode type."""

    backups: BackupsAddon


@dataclass
class InstanceType:
    """Linode instance type (plan)."""

    id: str
    label: str
    class_: str  # class is reserved keyword
    disk: int
    memory: int
    vcpus: int
    gpus: int
    network_out: int
    transfer: int
    price: Price
    addons: Addons
    successor: str | None


@dataclass
class Volume:
    """Linode block storage volume."""

    id: int
    label: str
    status: str
    size: int
    region: str
    linode_id: int | None
    linode_label: str | None
    filesystem_path: str
    tags: list[str]
    created: str
    updated: str
    hardware_type: str


@dataclass
class Image:
    """Linode image (OS image or custom image)."""

    id: str
    label: str
    description: str
    type: str
    is_public: bool
    deprecated: bool
    size: int
    vendor: str
    status: str
    created: str
    created_by: str
    expiry: str | None
    eol: str | None
    capabilities: list[str]
    tags: list[str]


# Stage 3: Extended read operations


@dataclass
class SSHKey:
    """SSH key associated with a Linode profile."""

    id: int
    label: str
    ssh_key: str
    created: str


@dataclass
class Domain:
    """Linode DNS domain."""

    id: int
    domain: str
    type: str
    status: str
    soa_email: str
    description: str
    tags: list[str]
    created: str
    updated: str


@dataclass
class DomainZoneFile:
    """DNS zone file for a domain."""

    zone_file: list[str]


@dataclass
class DomainRecord:
    """DNS record for a domain."""

    id: int
    type: str
    name: str
    target: str
    priority: int
    weight: int
    port: int
    ttl_sec: int
    created: str
    updated: str


@dataclass
class FirewallAddresses:
    """IP addresses for a firewall rule."""

    ipv4: list[str]
    ipv6: list[str]


@dataclass
class FirewallRule:
    """Firewall rule."""

    action: str
    protocol: str
    ports: str
    addresses: FirewallAddresses
    label: str
    description: str


@dataclass
class FirewallRules:
    """Firewall rules configuration."""

    inbound: list[FirewallRule]
    inbound_policy: str
    outbound: list[FirewallRule]
    outbound_policy: str


@dataclass
class Firewall:
    """Linode Cloud Firewall."""

    id: int
    label: str
    status: str
    rules: FirewallRules
    tags: list[str]
    created: str
    updated: str


@dataclass
class FirewallTemplate:
    """Linode Cloud Firewall Template."""

    slug: str
    label: str
    description: str
    rules: FirewallRules


@dataclass
class Transfer:
    """Transfer usage data."""

    in_: float
    out: float
    total: float


@dataclass
class NodeBalancer:
    """Linode NodeBalancer."""

    id: int
    label: str
    region: str
    hostname: str
    ipv4: str
    ipv6: str
    client_conn_throttle: int
    transfer: Transfer
    tags: list[str]
    created: str
    updated: str


@dataclass
class UDF:
    """User defined field for StackScript."""

    label: str
    name: str
    example: str
    oneof: str
    default: str


@dataclass
class StackScript:
    """Linode StackScript."""

    id: int
    username: str
    user_gravatar_id: str
    label: str
    description: str
    images: list[str]
    deployments_total: int
    deployments_active: int
    is_public: bool
    mine: bool
    created: str
    updated: str
    script: str
    user_defined_fields: list[UDF]


# LKE (Linode Kubernetes Engine) types


@dataclass
class LKEControlPlane:
    """Control plane configuration of an LKE cluster."""

    high_availability: bool


@dataclass
class LKECluster:
    """Linode Kubernetes Engine cluster."""

    id: int
    label: str
    region: str
    k8s_version: str
    status: str
    tags: list[str]
    created: str
    updated: str
    control_plane: LKEControlPlane


@dataclass
class LKENodePoolAutoscaler:
    """Autoscaling settings for a node pool."""

    enabled: bool
    min: int
    max: int


@dataclass
class LKENodePoolDisk:
    """Disk configuration in a node pool."""

    size: int
    type: str


@dataclass
class LKENode:
    """Node within an LKE node pool."""

    id: str
    instance_id: int
    status: str


@dataclass
class LKENodePool:
    """Node pool within an LKE cluster."""

    id: int
    cluster_id: int
    type: str
    count: int
    disks: list[LKENodePoolDisk]
    autoscaler: LKENodePoolAutoscaler | None
    nodes: list[LKENode]
    tags: list[str]


@dataclass
class LKEKubeconfig:
    """Base64-encoded kubeconfig for an LKE cluster."""

    kubeconfig: str


@dataclass
class LKEDashboard:
    """Dashboard URL for an LKE cluster."""

    url: str


@dataclass
class LKEAPIEndpoint:
    """API endpoint for an LKE cluster."""

    endpoint: str


@dataclass
class LKEVersion:
    """Available Kubernetes version for LKE."""

    id: str


@dataclass
class LKETypePrice:
    """Pricing for an LKE type."""

    hourly: float
    monthly: float


@dataclass
class LKERegionPrice:
    """Region-specific pricing for an LKE type."""

    id: str
    hourly: float
    monthly: float


@dataclass
class LKEType:
    """Node type available for LKE clusters."""

    id: str
    label: str
    price: LKETypePrice
    region_prices: list[LKERegionPrice]
    transfer: int


@dataclass
class LKETierVersion:
    """LKE tier version."""

    id: str
    tier: str


@dataclass
class LKEControlPlaneACLAddresses:
    """IP addresses in a control plane ACL."""

    ipv4: list[str]
    ipv6: list[str]


@dataclass
class LKEControlPlaneACL:
    """Control plane ACL for an LKE cluster."""

    enabled: bool
    addresses: LKEControlPlaneACLAddresses


@dataclass
class VPCSubnet:
    """Subnet within a VPC."""

    id: int
    label: str
    ipv4: str
    linodes: list[dict[str, Any]]
    created: str
    updated: str


@dataclass
class VPC:
    """Linode VPC."""

    id: int
    label: str
    description: str
    region: str
    subnets: list[VPCSubnet]
    created: str
    updated: str


@dataclass
class VPCIP:
    """IP address associated with a VPC."""

    address: str
    address_range: str | None
    vpc_id: int
    subnet_id: int
    region: str
    linode_id: int
    config_id: int
    interface_id: int
    active: bool
    nat_1_1: str | None
    gateway: str | None
    prefix: int | None
    subnet_mask: str | None


def _build_public_interface_entry(
    firewall_id: int, route_ipv4: bool, route_ipv6: bool
) -> dict[str, Any]:
    """Build a single public-interface entry for the POST /linode/instances
    payload. default_route keys are included only when True so the API does
    not see "ipv4": false as "this interface owns the IPv4 default."
    """
    default_route: dict[str, bool] = {}
    if route_ipv4:
        default_route["ipv4"] = True
    if route_ipv6:
        default_route["ipv6"] = True

    entry: dict[str, Any] = {
        "public": {},
        "firewall_id": firewall_id,
    }
    if default_route:
        entry["default_route"] = default_route

    return entry


def _parse_default_route(data: dict[str, Any] | None) -> InterfaceDefaultRoute | None:
    """Parse a default_route subobject. Missing or empty returns None so the
    caller can leave the field unset.
    """
    if not data:
        return None

    return InterfaceDefaultRoute(
        ipv4=bool(data.get("ipv4", False)),
        ipv6=bool(data.get("ipv6", False)),
    )


def _parse_instance_interface(data: dict[str, Any]) -> InstanceInterface:
    """Parse a single interface object from the API response. Only top-level
    fields and the default_route subobject are extracted; deeper sub-config
    parsing (public.ipv4.addresses, vpc.ipv4.addresses, etc.) is deferred to
    the live-response capture follow-up tracked in the spec's sticky issue.
    """
    public: InterfacePublicConfig | None = None
    if "public" in data and data["public"] is not None:
        public = InterfacePublicConfig()

    return InstanceInterface(
        id=data.get("id", 0),
        public=public,
        default_route=_parse_default_route(data.get("default_route")),
        firewall_id=data.get("firewall_id"),
        mac_address=data.get("mac_address", ""),
    )


def _build_monitor_service_alert_definition_body(
    *,
    label: object,
    severity: object,
    rule_criteria: object,
    trigger_conditions: object,
    channel_ids: object,
    description: object,
    entity_ids: object,
) -> dict[str, Any]:
    """Validate and build a monitor service alert definition payload."""
    if not isinstance(label, str) or not label:
        msg = "label is required"
        raise ValueError(msg)
    if type(severity) is not int:
        msg = "severity must be a valid integer"
        raise TypeError(msg)
    if severity not in {0, 1, 2, 3}:
        msg = "severity must be one of 0, 1, 2, or 3"
        raise ValueError(msg)
    if not isinstance(rule_criteria, dict) or not rule_criteria:
        msg = "rule_criteria must be a non-empty object"
        raise ValueError(msg)
    if not isinstance(trigger_conditions, dict) or not trigger_conditions:
        msg = "trigger_conditions must be a non-empty object"
        raise ValueError(msg)
    if (
        not isinstance(channel_ids, list)
        or not channel_ids
        or any(type(item) is not int for item in cast("list[object]", channel_ids))
    ):
        msg = "channel_ids must be a non-empty list of integers"
        raise ValueError(msg)
    if entity_ids is not None and (
        not isinstance(entity_ids, list)
        or not entity_ids
        or any(type(item) is not int for item in cast("list[object]", entity_ids))
    ):
        msg = "entity_ids must be a non-empty list of integers"
        raise ValueError(msg)
    if description is not None and not isinstance(description, str):
        msg = "description must be a string"
        raise ValueError(msg)

    checked_rule_criteria = cast("dict[str, Any]", rule_criteria)
    checked_trigger_conditions = cast("dict[str, Any]", trigger_conditions)
    checked_channel_ids = cast("list[int]", channel_ids)
    checked_entity_ids = cast("list[int] | None", entity_ids)

    body: dict[str, Any] = {
        "label": label,
        "severity": severity,
        "rule_criteria": checked_rule_criteria,
        "trigger_conditions": checked_trigger_conditions,
        "channel_ids": checked_channel_ids,
    }
    if description is not None:
        body["description"] = description
    if checked_entity_ids is not None:
        body["entity_ids"] = checked_entity_ids
    return body


class Client:
    """Linode API client."""

    def __init__(
        self,
        api_url: str,
        token: str,
        *,
        max_connections: int = 10,
        max_keepalive_connections: int = 10,
        keepalive_expiry: float = 30.0,
    ) -> None:
        self.base_url = api_url
        self.token = token
        # Retain the Limits object so observability and tests can read back
        # what was actually configured. httpx.AsyncClient consumes Limits
        # internally and does not expose it.
        self.limits = httpx.Limits(
            max_connections=max_connections,
            max_keepalive_connections=max_keepalive_connections,
            keepalive_expiry=keepalive_expiry,
        )
        self.client = httpx.AsyncClient(
            timeout=30.0,
            limits=self.limits,
        )

    async def close(self) -> None:
        """Close the HTTP client."""
        await self.client.aclose()

    async def __aenter__(self) -> "Client":
        """Async context manager entry."""
        return self

    async def __aexit__(self, *args: Any) -> None:
        """Async context manager exit."""
        await self.close()

    def _parse_profile(self, data: dict[str, Any]) -> Profile:
        """Parse a Linode profile response."""
        return Profile(
            username=data["username"],
            email=data["email"],
            timezone=data["timezone"],
            email_notifications=data["email_notifications"],
            restricted=data["restricted"],
            two_factor_auth=data["two_factor_auth"],
            uid=data["uid"],
            scopes=data.get("scopes", "") or "",
        )

    def _parse_grant(self, data: dict[str, Any]) -> Grant:
        """Parse a single per-resource OAuth grant entry."""
        return Grant(
            id=int(data.get("id", 0)),
            label=str(data.get("label", "")),
            permissions=str(data.get("permissions", "") or ""),
        )

    def _parse_grants(self, data: dict[str, Any]) -> Grants:
        """Parse the /profile/grants response into a structured Grants.

        PATs return an empty payload here; the returned Grants has all
        empty lists and a zero-valued GlobalGrants. The Phase 6 loader
        checks Profile.scopes first to decide which path to use.
        """
        global_raw_any: Any = data.get("global")
        global_raw: dict[str, Any] = (
            cast("dict[str, Any]", global_raw_any)
            if isinstance(global_raw_any, dict)
            else {}
        )
        global_grants = GlobalGrants(
            account_access=str(global_raw.get("account_access", "") or ""),
            add_databases=bool(global_raw.get("add_databases", False)),
            add_domains=bool(global_raw.get("add_domains", False)),
            add_firewalls=bool(global_raw.get("add_firewalls", False)),
            add_images=bool(global_raw.get("add_images", False)),
            add_linodes=bool(global_raw.get("add_linodes", False)),
            add_longview=bool(global_raw.get("add_longview", False)),
            add_nodebalancers=bool(global_raw.get("add_nodebalancers", False)),
            add_stackscripts=bool(global_raw.get("add_stackscripts", False)),
            add_volumes=bool(global_raw.get("add_volumes", False)),
            add_vpcs=bool(global_raw.get("add_vpcs", False)),
            cancel_account=bool(global_raw.get("cancel_account", False)),
            child_account_access=bool(global_raw.get("child_account_access", False)),
            longview_subscription=bool(global_raw.get("longview_subscription", False)),
        )

        def _list(key: str) -> list[Grant]:
            raw: Any = data.get(key)
            if not isinstance(raw, list):
                return []
            return [
                self._parse_grant(cast("dict[str, Any]", item))
                for item in cast("list[object]", raw)
                if isinstance(item, dict)
            ]

        return Grants(
            global_=global_grants,
            linode=_list("linode"),
            domain=_list("domain"),
            nodebalancer=_list("nodebalancer"),
            image=_list("image"),
            longview=_list("longview"),
            stackscript=_list("stackscript"),
            volume=_list("volume"),
            database=_list("database"),
            firewall=_list("firewall"),
            vpc=_list("vpc"),
            lkecluster=_list("lkecluster"),
        )

    async def get_profile(self) -> Profile:
        """Get Linode user profile."""
        try:
            response = await self.make_request("GET", "/profile")
            data = response.json()
            return self._parse_profile(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetProfile", e) from e

    async def update_profile(self, **fields: Any) -> Profile:
        """Update Linode user profile."""
        body = {key: value for key, value in fields.items() if value is not None}
        try:
            response = await self.make_request("PUT", "/profile", body)
            data = response.json()
            return self._parse_profile(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateProfile", e) from e

    async def get_profile_preferences(self) -> dict[str, Any]:
        """Get OAuth client-specific profile preferences."""
        try:
            response = await self.make_request("GET", "/profile/preferences")
            data: Any = response.json()
            if isinstance(data, dict):
                return cast("dict[str, Any]", data)
            return {}
        except httpx.HTTPError as e:
            raise NetworkError("GetProfilePreferences", e) from e

    async def update_profile_preferences(
        self, preferences: dict[str, Any]
    ) -> dict[str, Any]:
        """Update OAuth client-specific profile preferences."""
        try:
            response = await self.make_request(
                "PUT", "/profile/preferences", preferences
            )
            data: Any = response.json()
            if isinstance(data, dict):
                return cast("dict[str, Any]", data)
            return {}
        except httpx.HTTPError as e:
            raise NetworkError("UpdateProfilePreferences", e) from e

    async def get_profile_grants(self) -> Grants:
        """Get the /profile/grants response for OAuth scope inspection.

        PATs return an empty payload (200 with zero-valued fields); the
        Phase 6 profile loader checks ``Profile.scopes`` first and only
        consults Grants when the scope string is empty (OAuth path).
        """
        try:
            response = await self.make_request("GET", "/profile/grants")
            data: Any = response.json()
            if not isinstance(data, dict):
                return Grants()
            return self._parse_grants(cast("dict[str, Any]", data))
        except httpx.HTTPError as e:
            raise NetworkError("GetProfileGrants", e) from e

    async def list_instances(self) -> list[Instance]:
        """List Linode instances."""
        try:
            response = await self.make_request("GET", "/linode/instances")
            data = response.json()
            return [self._parse_instance(inst) for inst in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListInstances", e) from e

    async def get_instance(self, instance_id: int) -> Instance:
        """Get a specific Linode instance."""
        endpoint = f"/linode/instances/{instance_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_instance(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetInstance", e) from e

    async def update_instance(self, instance_id: int, **fields: Any) -> Instance:
        """Update a Linode instance."""
        endpoint = f"/linode/instances/{instance_id}"
        body = {key: value for key, value in fields.items() if value is not None}
        try:
            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            return self._parse_instance(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateInstance", e) from e

    async def get_account(self) -> Account:
        """Get Linode account information."""
        try:
            response = await self.make_request("GET", "/account")
            data = response.json()
            return self._parse_account(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetAccount", e) from e

    async def get_account_agreements(self) -> dict[str, Any]:
        """List agreements on the Linode account."""
        try:
            response = await self.make_request("GET", "/account/agreements")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountAgreements", e) from e

    async def get_account_settings(self) -> dict[str, Any]:
        """Get settings for the Linode account."""
        try:
            response = await self.make_request("GET", "/account/settings")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountSettings", e) from e

    async def enable_account_managed(self) -> dict[str, Any]:
        """Enable Linode Managed for the account."""
        try:
            response = await self.make_request(
                "POST", "/account/settings/managed-enable"
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("EnableAccountManaged", e) from e

    async def get_account_transfer(self) -> dict[str, Any]:
        """Get network transfer usage for the Linode account."""
        try:
            response = await self.make_request("GET", "/account/transfer")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountTransfer", e) from e

    async def list_account_logins(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List user logins on the Linode account."""
        endpoint = "/account/logins"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountLogins", e) from e

    async def list_account_users(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List users on the Linode account."""
        endpoint = "/account/users"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountUsers", e) from e

    async def delete_account_user(self, username: str) -> dict[str, Any]:
        """Delete a user on the Linode account."""
        encoded_username = quote(str(username), safe="")
        endpoint = f"/account/users/{encoded_username}"
        try:
            response = await self.make_request("DELETE", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("DeleteAccountUser", e) from e

    async def list_account_maintenance(self) -> dict[str, Any]:
        """List maintenances on the Linode account."""
        try:
            response = await self.make_request("GET", "/account/maintenance")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountMaintenance", e) from e

    async def list_account_oauth_clients(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List OAuth clients on the Linode account."""
        endpoint = "/account/oauth-clients"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountOAuthClients", e) from e

    async def update_account_oauth_client(
        self, client_id: str, **fields: Any
    ) -> dict[str, Any]:
        """Update an OAuth client on the Linode account."""
        encoded_client_id = quote(str(client_id), safe="")
        body = {key: value for key, value in fields.items() if value is not None}
        try:
            response = await self.make_request(
                "PUT", f"/account/oauth-clients/{encoded_client_id}", body
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateAccountOAuthClient", e) from e

    async def update_account_oauth_client_thumbnail(
        self, client_id: str
    ) -> dict[str, Any]:
        """Update an OAuth client's thumbnail on the Linode account."""
        encoded_client_id = quote(str(client_id), safe="")
        try:
            response = await self.make_request(
                "PUT",
                f"/account/oauth-clients/{encoded_client_id}/thumbnail",
                {},
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateAccountOAuthClientThumbnail", e) from e

    async def list_account_events(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List events on the Linode account."""
        endpoint = "/account/events"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountEvents", e) from e

    async def list_account_invoices(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List invoices on the Linode account."""
        endpoint = "/account/invoices"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountInvoices", e) from e

    async def list_account_payments(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List payments on the Linode account."""
        endpoint = "/account/payments"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountPayments", e) from e

    async def list_account_payment_methods(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List payment methods on the Linode account."""
        endpoint = "/account/payment-methods"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountPaymentMethods", e) from e

    async def list_account_notifications(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List notifications on the Linode account."""
        endpoint = "/account/notifications"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountNotifications", e) from e

    async def list_account_invoice_items(
        self, invoice_id: int, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List items on an account invoice."""
        encoded_invoice_id = quote(str(invoice_id), safe="")
        endpoint = f"/account/invoices/{encoded_invoice_id}/items"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountInvoiceItems", e) from e

    async def get_account_event(self, event_id: int) -> dict[str, Any]:
        """Get an event on the Linode account."""
        encoded_event_id = quote(str(event_id), safe="")
        endpoint = f"/account/events/{encoded_event_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountEvent", e) from e

    async def mark_account_event_seen(self, event_id: int) -> dict[str, Any]:
        """Mark an account event as seen."""
        encoded_event_id = quote(str(event_id), safe="")
        endpoint = f"/account/events/{encoded_event_id}/seen"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("MarkAccountEventSeen", e) from e

    async def acknowledge_account_agreements(
        self, agreements: dict[str, bool]
    ) -> dict[str, Any]:
        """Acknowledge account agreements."""
        try:
            response = await self.make_request(
                "POST", "/account/agreements", agreements
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("AcknowledgeAccountAgreements", e) from e

    async def update_account(self, **fields: Any) -> Account:
        """Update Linode account information."""
        body = {key: value for key, value in fields.items() if value is not None}
        try:
            response = await self.make_request("PUT", "/account", body)
            data = response.json()
            return self._parse_account(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateAccount", e) from e

    async def update_account_settings(self, **fields: Any) -> dict[str, Any]:
        """Update Linode account settings."""
        body = {key: value for key, value in fields.items() if value is not None}
        if not body:
            msg = "At least one account settings field is required"
            raise ValueError(msg)
        try:
            response = await self.make_request("PUT", "/account/settings", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateAccountSettings", e) from e

    async def create_account_user(
        self, username: str, email: str, restricted: bool
    ) -> dict[str, Any]:
        """Create a user on the Linode account."""
        body = {"username": username, "email": email, "restricted": restricted}
        try:
            response = await self.make_request("POST", "/account/users", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateAccountUser", e) from e

    async def update_account_user(
        self, current_username: str, **fields: Any
    ) -> dict[str, Any]:
        """Update an account user by username."""
        body = {key: value for key, value in fields.items() if value is not None}
        if not body:
            msg = "At least one account user field is required"
            raise ValueError(msg)
        encoded_username = quote(current_username, safe="")
        endpoint = f"/account/users/{encoded_username}"
        try:
            response = await self.make_request("PUT", endpoint, body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateAccountUser", e) from e

    async def update_account_user_grants(
        self, username: str, grants: dict[str, Any]
    ) -> dict[str, Any]:
        """Update grants for an account user."""
        encoded_username = quote(username, safe="")
        endpoint = f"/account/users/{encoded_username}/grants"
        try:
            response = await self.make_request("PUT", endpoint, grants)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateAccountUserGrants", e) from e

    async def create_account_oauth_client(
        self, label: str, redirect_uri: str
    ) -> dict[str, Any]:
        """Create an OAuth client on the Linode account."""
        body = {"label": label, "redirect_uri": redirect_uri}
        try:
            response = await self.make_request("POST", "/account/oauth-clients", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateAccountOAuthClient", e) from e

    async def create_account_payment_method(
        self, payment_type: str, data: dict[str, Any], is_default: bool
    ) -> dict[str, Any]:
        """Add a payment method to the Linode account."""
        body = {"type": payment_type, "data": data, "is_default": is_default}
        try:
            response = await self.make_request("POST", "/account/payment-methods", body)
            data_response: dict[str, Any] = response.json()
            return data_response
        except httpx.HTTPError as e:
            raise NetworkError("CreateAccountPaymentMethod", e) from e

    async def create_account_payment(
        self, payment_method_id: int, usd: str
    ) -> dict[str, Any]:
        """Make a payment on the Linode account."""
        body = {"payment_method_id": payment_method_id, "usd": usd}
        try:
            response = await self.make_request("POST", "/account/payments", body)
            data_response: dict[str, Any] = response.json()
            return data_response
        except httpx.HTTPError as e:
            raise NetworkError("CreateAccountPayment", e) from e

    async def add_account_promo_credit(self, promo_code: str) -> dict[str, Any]:
        """Add a promo credit to the Linode account."""
        body = {"promo_code": promo_code}
        try:
            response = await self.make_request("POST", "/account/promo-codes", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("AddAccountPromoCredit", e) from e

    async def create_account_service_transfer(
        self, linode_ids: list[int]
    ) -> dict[str, Any]:
        """Request a service transfer for Linode entities."""
        body = {"entities": {"linodes": linode_ids}}
        try:
            response = await self.make_request(
                "POST", "/account/service-transfers", body
            )
            data_response: dict[str, Any] = response.json()
            return data_response
        except httpx.HTTPError as e:
            raise NetworkError("CreateAccountServiceTransfer", e) from e

    async def delete_account_oauth_client(self, client_id: str) -> dict[str, Any]:
        """Delete an OAuth client on the Linode account."""
        encoded_client_id = quote(str(client_id), safe="")
        endpoint = f"/account/oauth-clients/{encoded_client_id}"
        try:
            response = await self.make_request("DELETE", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("DeleteAccountOAuthClient", e) from e

    async def delete_account_payment_method(
        self, payment_method_id: int | str
    ) -> dict[str, Any]:
        """Delete a payment method on the Linode account."""
        encoded_payment_method_id = quote(str(payment_method_id), safe="")
        endpoint = f"/account/payment-methods/{encoded_payment_method_id}"
        try:
            response = await self.make_request("DELETE", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("DeleteAccountPaymentMethod", e) from e

    async def cancel_account(self, comments: str | None = None) -> dict[str, Any]:
        """Cancel the Linode account."""
        body: dict[str, Any] = {}
        if comments is not None:
            body["comments"] = comments
        try:
            response = await self.make_request("POST", "/account/cancel", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CancelAccount", e) from e

    async def list_account_betas(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List enrolled Beta programs for the account."""
        endpoint = "/account/betas"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountBetas", e) from e

    async def list_betas(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List available Beta programs."""
        endpoint = "/betas"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListBetas", e) from e

    async def list_mysql_database_instances(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List MySQL Managed Database instances."""
        endpoint = "/databases/mysql/instances"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListMysqlDatabaseInstances", e) from e

    async def list_postgresql_database_instances(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List PostgreSQL Managed Database instances."""
        endpoint = "/databases/postgresql/instances"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListPostgresqlDatabaseInstances", e) from e

    async def list_database_instances(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List Managed Database instances."""
        endpoint = "/databases/instances"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListDatabaseInstances", e) from e

    async def create_mysql_database_instance(
        self, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Create or restore a MySQL Managed Database instance."""
        try:
            response = await self.make_request(
                "POST", "/databases/mysql/instances", payload
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateMysqlDatabaseInstance", e) from e

    async def create_postgresql_database_instance(
        self, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Create or restore a PostgreSQL Managed Database instance."""
        try:
            response = await self.make_request(
                "POST", "/databases/postgresql/instances", payload
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreatePostgresqlDatabaseInstance", e) from e

    async def delete_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Delete a MySQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}"
        try:
            response = await self.make_request("DELETE", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("DeleteMysqlDatabaseInstance", e) from e

    async def delete_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Delete a PostgreSQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}"
        try:
            response = await self.make_request("DELETE", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("DeletePostgresqlDatabaseInstance", e) from e

    async def patch_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Apply pending patches to a MySQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}/patch"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("PatchMysqlDatabaseInstance", e) from e

    async def patch_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Apply pending patches to a PostgreSQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}/patch"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("PatchPostgresqlDatabaseInstance", e) from e

    async def get_database_mysql_instance(self, instance_id: int) -> dict[str, Any]:
        """Get a MySQL Managed Database instance by ID."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseMySQLInstance", e) from e

    async def get_database_postgresql_instance(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get a PostgreSQL Managed Database instance by ID."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabasePostgreSQLInstance", e) from e

    async def reset_postgresql_database_credentials(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Reset PostgreSQL Managed Database credentials."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = (
            f"/databases/postgresql/instances/{encoded_instance_id}/credentials/reset"
        )
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ResetPostgresqlDatabaseCredentials", e) from e

    async def get_database_postgresql_instance_ssl(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Get a PostgreSQL Managed Database SSL certificate by instance ID."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}/ssl"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabasePostgreSQLInstanceSSL", e) from e

    async def get_database_mysql_instance_ssl(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Get a MySQL Managed Database SSL certificate by instance ID."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}/ssl"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseMySQLInstanceSSL", e) from e

    async def get_database_mysql_instance_credentials(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get credentials for a MySQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}/credentials"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseMySQLInstanceCredentials", e) from e

    async def get_database_postgresql_instance_credentials(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get credentials for a PostgreSQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}/credentials"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabasePostgreSQLInstanceCredentials", e) from e

    async def resume_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Resume a MySQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}/resume"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ResumeMysqlDatabaseInstance", e) from e

    async def suspend_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Suspend a MySQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}/suspend"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("SuspendMysqlDatabaseInstance", e) from e

    async def resume_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Resume a PostgreSQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}/resume"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ResumePostgreSQLDatabaseInstance", e) from e

    async def suspend_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Suspend a PostgreSQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/postgresql/instances/{encoded_instance_id}/suspend"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("SuspendPostgreSQLDatabaseInstance", e) from e

    async def update_mysql_database_instance(
        self, instance_id: int, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Update a MySQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        try:
            response = await self.make_request(
                "PUT", f"/databases/mysql/instances/{encoded_instance_id}", payload
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateMysqlDatabaseInstance", e) from e

    async def reset_mysql_database_credentials(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Reset MySQL Managed Database credentials."""
        encoded_instance_id = quote(str(instance_id), safe="")
        endpoint = f"/databases/mysql/instances/{encoded_instance_id}/credentials/reset"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ResetMysqlDatabaseCredentials", e) from e

    async def list_account_child_accounts(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List child accounts for the account."""
        endpoint = "/account/child-accounts"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountChildAccounts", e) from e

    async def list_account_service_transfers(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List service transfers for the account."""
        endpoint = "/account/service-transfers"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountServiceTransfers", e) from e

    async def create_account_child_account_token(self, euuid: str) -> dict[str, Any]:
        """Create a proxy user token for a child account."""
        encoded_euuid = quote(euuid, safe="")
        endpoint = f"/account/child-accounts/{encoded_euuid}/token"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateAccountChildAccountToken", e) from e

    async def get_account_beta(self, beta_id: str) -> dict[str, Any]:
        """Get an enrolled Beta program on the account."""
        encoded_beta_id = quote(beta_id, safe="")
        try:
            response = await self.make_request(
                "GET", f"/account/betas/{encoded_beta_id}"
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountBeta", e) from e

    async def get_account_child_account(self, euuid: str) -> dict[str, Any]:
        """Get a child account by EUUID."""
        encoded_euuid = quote(euuid, safe="")
        endpoint = f"/account/child-accounts/{encoded_euuid}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountChildAccount", e) from e

    async def get_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Get an account service transfer request by token."""
        encoded_token = quote(token, safe="")
        endpoint = f"/account/service-transfers/{encoded_token}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountServiceTransfer", e) from e

    async def accept_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Accept an account service transfer request by token."""
        encoded_token = quote(token, safe="")
        endpoint = f"/account/service-transfers/{encoded_token}/accept"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("AcceptAccountServiceTransfer", e) from e

    async def delete_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Cancel an account service transfer request by token."""
        encoded_token = quote(token, safe="")
        endpoint = f"/account/service-transfers/{encoded_token}"
        try:
            response = await self.make_request("DELETE", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("DeleteAccountServiceTransfer", e) from e

    async def get_account_invoice(self, invoice_id: int) -> dict[str, Any]:
        """Get an invoice by ID."""
        encoded_invoice_id = quote(str(invoice_id), safe="")
        endpoint = f"/account/invoices/{encoded_invoice_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountInvoice", e) from e

    async def get_account_oauth_client(self, client_id: str) -> dict[str, Any]:
        """Get an OAuth client by client ID."""
        encoded_client_id = quote(client_id, safe="")
        endpoint = f"/account/oauth-clients/{encoded_client_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountOAuthClient", e) from e

    async def get_account_payment(self, payment_id: int) -> dict[str, Any]:
        """Get an account payment by ID."""
        encoded_payment_id = quote(str(payment_id), safe="")
        endpoint = f"/account/payments/{encoded_payment_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountPayment", e) from e

    async def get_account_payment_method(
        self, payment_method_id: int
    ) -> dict[str, Any]:
        """Get an account payment method by ID."""
        encoded_payment_method_id = quote(str(payment_method_id), safe="")
        endpoint = f"/account/payment-methods/{encoded_payment_method_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountPaymentMethod", e) from e

    async def make_account_payment_method_default(
        self, payment_method_id: int
    ) -> dict[str, Any]:
        """Set an account payment method as the default payment method."""
        encoded_payment_method_id = quote(str(payment_method_id), safe="")
        endpoint = f"/account/payment-methods/{encoded_payment_method_id}/make-default"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("MakeAccountPaymentMethodDefault", e) from e

    async def reset_account_oauth_client_secret(self, client_id: str) -> dict[str, Any]:
        """Reset an OAuth client secret by client ID."""
        encoded_client_id = quote(client_id, safe="")
        endpoint = f"/account/oauth-clients/{encoded_client_id}/reset-secret"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ResetAccountOAuthClientSecret", e) from e

    async def get_account_oauth_client_thumbnail(
        self, client_id: str
    ) -> dict[str, str]:
        """Get an OAuth client's PNG thumbnail by client ID."""
        encoded_client_id = quote(client_id, safe="")
        endpoint = f"/account/oauth-clients/{encoded_client_id}/thumbnail"
        url = self.base_url + endpoint
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Accept": "image/png",
            "User-Agent": "LinodeMCP/1.0",
        }
        try:
            response = await self.client.request("GET", url, headers=headers)
            if response.status_code >= HTTP_BAD_REQUEST:
                self._handle_error_response(response)
            content_type = response.headers.get("Content-Type", "image/png")
            return {
                "content_type": content_type.split(";", 1)[0],
                "encoding": "base64",
                "data": base64.b64encode(response.content).decode("ascii"),
            }
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountOAuthClientThumbnail", e) from e

    async def get_account_login(self, login_id: int) -> dict[str, Any]:
        """Get an account login by ID."""
        encoded_login_id = quote(str(login_id), safe="")
        endpoint = f"/account/logins/{encoded_login_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountLogin", e) from e

    async def get_account_user(self, username: str) -> dict[str, Any]:
        """Get an account user by username."""
        encoded_username = quote(username, safe="")
        endpoint = f"/account/users/{encoded_username}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountUser", e) from e

    async def get_account_user_grants(self, username: str) -> dict[str, Any]:
        """List grants for an account user by username."""
        encoded_username = quote(username, safe="")
        endpoint = f"/account/users/{encoded_username}/grants"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountUserGrants", e) from e

    async def enroll_account_beta(self, beta_id: str) -> dict[str, Any]:
        """Enroll the account in a beta program."""
        try:
            response = await self.make_request(
                "POST", "/account/betas", {"id": beta_id}
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("EnrollAccountBeta", e) from e

    async def list_account_availability(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List available Linode services for the account."""
        endpoint = "/account/availability"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListAccountAvailability", e) from e

    async def get_account_availability(self, region_id: str) -> dict[str, Any]:
        """Get available Linode services for the account in a region."""
        encoded_region_id = quote(region_id, safe="")
        endpoint = f"/account/availability/{encoded_region_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountAvailability", e) from e

    async def get_beta(self, beta_id: str) -> dict[str, Any]:
        """Get an available Beta program."""
        encoded_beta_id = quote(beta_id, safe="")
        endpoint = f"/betas/{encoded_beta_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetBeta", e) from e

    async def get_database_engine(
        self, engine_id: str, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """Get a Managed Databases engine."""
        encoded_engine_id = quote(engine_id, safe="")
        endpoint = f"/databases/engines/{encoded_engine_id}"
        # The OpenAPI contract documents page/page_size for this endpoint.
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseEngine", e) from e

    async def get_database_type(
        self, type_id: object, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """Get a Managed Databases type."""
        if not isinstance(type_id, str):
            raise TypeError("type_id must be a string")
        if not type_id or type_id.strip() != type_id:
            raise ValueError("type_id is required")
        if ".." in type_id or not re.fullmatch(r"[A-Za-z0-9._-]+", type_id):
            raise ValueError(
                "type_id must use letters, numbers, dots, underscores, and hyphens"
            )
        encoded_type_id = quote(type_id, safe="")
        endpoint = f"/databases/types/{encoded_type_id}"
        params: dict[str, int] = {}
        if page is not None:
            if type(page) is not int or page < 1:
                raise ValueError("page must be an integer at least 1")
            params["page"] = page
        if page_size is not None:
            if (
                type(page_size) is not int
                or page_size < MIN_PAGE_SIZE
                or page_size > MAX_PAGE_SIZE
            ):
                raise ValueError("page_size must be an integer between 25 and 500")
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseType", e) from e

    async def list_regions(self) -> list[Region]:
        """List Linode regions."""
        try:
            response = await self.make_request("GET", "/regions")
            data = response.json()
            return [self._parse_region(r) for r in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListRegions", e) from e

    async def get_region(self, region_id: str) -> Region:
        """Get a Linode region."""
        encoded_region_id = quote(region_id, safe="")
        endpoint = f"/regions/{encoded_region_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_region(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetRegion", e) from e

    async def get_region_availability(self, region_id: str) -> list[dict[str, Any]]:
        """Get compute instance availability for a region."""
        encoded_region_id = quote(region_id, safe="")
        endpoint = f"/regions/{encoded_region_id}/availability"
        try:
            response = await self.make_request("GET", endpoint)
            data: list[dict[str, Any]] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetRegionAvailability", e) from e

    async def list_regions_availability(self) -> list[dict[str, Any]]:
        """List compute instance availability across regions."""
        try:
            response = await self.make_request("GET", "/regions/availability")
            data: list[dict[str, Any]] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListRegionsAvailability", e) from e

    async def list_types(self) -> list[InstanceType]:
        """List Linode instance types."""
        try:
            response = await self.make_request("GET", "/linode/types")
            data = response.json()
            return [self._parse_instance_type(t) for t in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListTypes", e) from e

    async def list_database_engines(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List Linode Managed Databases engines."""
        endpoint = "/databases/engines"
        params: dict[str, int] = {}
        if page is not None:
            if type(page) is not int or page < 1:
                raise ValueError("page must be an integer at least 1")
            params["page"] = page
        if page_size is not None:
            if (
                type(page_size) is not int
                or page_size < MIN_PAGE_SIZE
                or page_size > MAX_PAGE_SIZE
            ):
                raise ValueError("page_size must be an integer between 25 and 500")
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListDatabaseEngines", e) from e

    async def list_database_types(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List available Linode Managed Databases types."""
        endpoint = "/databases/types"
        params: dict[str, int] = {}
        if page is not None:
            if type(page) is not int or page < 1:
                raise ValueError("page must be an integer at least 1")
            params["page"] = page
        if page_size is not None:
            if (
                type(page_size) is not int
                or page_size < MIN_PAGE_SIZE
                or page_size > MAX_PAGE_SIZE
            ):
                raise ValueError("page_size must be an integer between 25 and 500")
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListDatabaseTypes", e) from e

    async def get_database_mysql_config(self) -> dict[str, Any]:
        """List MySQL Managed Database advanced parameters."""
        try:
            response = await self.make_request("GET", "/databases/mysql/config")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseMySQLConfig", e) from e

    async def get_database_postgresql_config(self) -> dict[str, Any]:
        """List PostgreSQL Managed Database advanced parameters."""
        try:
            response = await self.make_request("GET", "/databases/postgresql/config")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabasePostgreSQLConfig", e) from e

    async def update_postgresql_database_instance(
        self, instance_id: int, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Update a PostgreSQL Managed Database instance."""
        encoded_instance_id = quote(str(instance_id), safe="")
        try:
            response = await self.make_request(
                "PUT", f"/databases/postgresql/instances/{encoded_instance_id}", payload
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdatePostgreSQLDatabaseInstance", e) from e

    async def list_volumes(self) -> list[Volume]:
        """List Linode block storage volumes."""
        try:
            response = await self.make_request("GET", "/volumes")
            data = response.json()
            return [self._parse_volume(v) for v in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListVolumes", e) from e

    async def list_volume_types(self) -> list[dict[str, Any]]:
        """List Linode block storage volume types."""
        try:
            response = await self.make_request("GET", "/volumes/types")
            data = response.json()
            volume_types: list[dict[str, Any]] = data.get("data", [])
            return volume_types
        except httpx.HTTPError as e:
            raise NetworkError("ListVolumeTypes", e) from e

    async def get_volume(self, volume_id: int) -> Volume:
        """Get a Linode block storage volume."""
        try:
            response = await self.make_request("GET", f"/volumes/{volume_id}")
            data = response.json()
            return self._parse_volume(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetVolume", e) from e

    async def list_images(self) -> list[Image]:
        """List Linode images."""
        try:
            response = await self.make_request("GET", "/images")
            data = response.json()
            return [self._parse_image(i) for i in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListImages", e) from e

    async def list_image_sharegroups(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List image share groups."""
        endpoint = "/images/sharegroups"
        params: dict[str, int] = {}
        if page is not None:
            if type(page) is not int or page < 1:
                raise ValueError("page must be an integer at least 1")
            params["page"] = page
        if page_size is not None:
            if (
                type(page_size) is not int
                or page_size < MIN_PAGE_SIZE
                or page_size > MAX_PAGE_SIZE
            ):
                raise ValueError("page_size must be an integer between 25 and 500")
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListImageSharegroups", e) from e

    async def delete_image_sharegroup(self, sharegroup_id: str) -> None:
        """Delete a single image share group."""
        sharegroup_id_path = quote(str(sharegroup_id), safe="")
        try:
            await self.make_request(
                "DELETE", f"/images/sharegroups/{sharegroup_id_path}"
            )
        except httpx.HTTPError as e:
            raise NetworkError("DeleteImageSharegroup", e) from e

    async def create_image_sharegroup(
        self,
        label: str,
        description: str | None = None,
        images: list[dict[str, str]] | None = None,
    ) -> dict[str, Any]:
        """Create an image share group."""
        body: dict[str, Any] = {"label": label}
        if description is not None:
            body["description"] = description
        if images is not None:
            body["images"] = images

        try:
            response = await self.make_request("POST", "/images/sharegroups", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateImageShareGroup", e) from e

    async def get_image_sharegroup(self, sharegroup_id: str) -> dict[str, Any]:
        """Get a single image share group."""
        sharegroup_id_path = quote(str(sharegroup_id), safe="")
        try:
            response = await self.make_request(
                "GET", f"/images/sharegroups/{sharegroup_id_path}"
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetImageSharegroup", e) from e

    async def update_image_sharegroup(
        self,
        sharegroup_id: str,
        *,
        label: str | None = None,
        description: str | None = None,
    ) -> dict[str, Any]:
        """Update a single image share group."""
        sharegroup_id_path = quote(str(sharegroup_id), safe="")
        if label is None and description is None:
            raise ValueError("at least one of label or description must be provided")
        body: dict[str, Any] = {}
        if label is not None:
            body["label"] = label
        if description is not None:
            body["description"] = description
        try:
            response = await self.make_request(
                "PUT", f"/images/sharegroups/{sharegroup_id_path}", body
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateImageSharegroup", e) from e

    async def list_image_sharegroup_tokens(self) -> dict[str, Any]:
        """List image share group tokens for the user."""
        try:
            response = await self.make_request("GET", "/images/sharegroups/tokens")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListImageSharegroupTokens", e) from e

    async def get_image_sharegroup_token(self, token_uuid: str) -> dict[str, Any]:
        """Get a single image share group token."""
        token_uuid_path = quote(token_uuid, safe="")
        try:
            response = await self.make_request(
                "GET", f"/images/sharegroups/tokens/{token_uuid_path}"
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetImageSharegroupToken", e) from e

    async def get_image_sharegroup_by_token(self, token_uuid: str) -> dict[str, Any]:
        """Get the image share group associated with a token."""
        token_uuid_path = quote(token_uuid, safe="")
        try:
            response = await self.make_request(
                "GET", f"/images/sharegroups/tokens/{token_uuid_path}/sharegroup"
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetImageSharegroupByToken", e) from e

    async def list_image_sharegroup_images_by_token(
        self, token_uuid: str
    ) -> dict[str, Any]:
        """List images available through an image share group token."""
        token_uuid_path = quote(token_uuid, safe="")
        try:
            response = await self.make_request(
                "GET",
                f"/images/sharegroups/tokens/{token_uuid_path}/sharegroup/images",
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListImageSharegroupImagesByToken", e) from e

    async def list_image_sharegroup_images(self, sharegroup_id: str) -> dict[str, Any]:
        """List images available in an image share group."""
        sharegroup_id_path = quote(str(sharegroup_id), safe="")
        try:
            response = await self.make_request(
                "GET", f"/images/sharegroups/{sharegroup_id_path}/images"
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListImageSharegroupImages", e) from e

    async def create_image_sharegroup_token(
        self, valid_for_sharegroup_uuid: str, label: str | None = None
    ) -> dict[str, Any]:
        """Create an image share group token."""
        body: dict[str, Any] = {"valid_for_sharegroup_uuid": valid_for_sharegroup_uuid}
        if label is not None:
            body["label"] = label
        try:
            response = await self.make_request(
                "POST", "/images/sharegroups/tokens", body
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateImageSharegroupToken", e) from e

    async def update_image_sharegroup_token(
        self, token_uuid: str, label: str
    ) -> dict[str, Any]:
        """Update an image share group token label."""
        token_uuid_path = quote(token_uuid, safe="")
        try:
            response = await self.make_request(
                "PUT",
                f"/images/sharegroups/tokens/{token_uuid_path}",
                {"label": label},
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateImageSharegroupToken", e) from e

    async def delete_image_sharegroup_token(self, token_uuid: str) -> None:
        """Delete an image share group token."""
        token_uuid_path = quote(token_uuid, safe="")
        try:
            await self.make_request(
                "DELETE", f"/images/sharegroups/tokens/{token_uuid_path}"
            )
        except httpx.HTTPError as e:
            raise NetworkError("DeleteImageSharegroupToken", e) from e

    async def create_image(
        self,
        disk_id: int,
        label: str | None = None,
        description: str | None = None,
        cloud_init: bool | None = None,
        tags: list[str] | None = None,
    ) -> Image:
        """Create a private image from a Linode disk."""
        body: dict[str, Any] = {"disk_id": disk_id}
        if label is not None:
            body["label"] = label
        if description is not None:
            body["description"] = description
        if cloud_init is not None:
            body["cloud_init"] = cloud_init
        if tags is not None:
            body["tags"] = tags

        try:
            response = await self.make_request("POST", "/images", body)
            data = response.json()
            return self._parse_image(data)
        except httpx.HTTPError as e:
            raise NetworkError("CreateImage", e) from e

    # Stage 3: Extended read operations

    async def list_ssh_keys(self) -> list[SSHKey]:
        """List SSH keys."""
        try:
            response = await self.make_request("GET", "/profile/sshkeys")
            data = response.json()
            return [self._parse_ssh_key(k) for k in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListSSHKeys", e) from e

    async def get_ssh_key(self, ssh_key_id: int) -> SSHKey:
        """Get a specific SSH key."""
        try:
            response = await self.make_request("GET", f"/profile/sshkeys/{ssh_key_id}")
            data = response.json()
            return self._parse_ssh_key(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetSSHKey", e) from e

    async def list_domains(self) -> list[Domain]:
        """List domains."""
        try:
            response = await self.make_request("GET", "/domains")
            data = response.json()
            return [self._parse_domain(d) for d in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListDomains", e) from e

    async def get_domain(self, domain_id: int) -> Domain:
        """Get a specific domain."""
        endpoint = f"/domains/{domain_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_domain(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetDomain", e) from e

    async def get_domain_zone_file(self, domain_id: int) -> DomainZoneFile:
        """Get a domain zone file."""
        if type(domain_id) is not int or domain_id <= 0:
            msg = "domain_id must be a positive integer"
            raise ValueError(msg)
        encoded_domain_id = quote(str(domain_id), safe="")
        endpoint = f"/domains/{encoded_domain_id}/zone-file"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            zone_file = data.get("zone_file", [])
            return DomainZoneFile(zone_file=list(zone_file))
        except httpx.HTTPError as e:
            raise NetworkError("GetDomainZoneFile", e) from e

    async def list_domain_records(self, domain_id: int) -> list[DomainRecord]:
        """List domain records for a domain."""
        endpoint = f"/domains/{domain_id}/records"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return [self._parse_domain_record(r) for r in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListDomainRecords", e) from e

    async def get_domain_record(self, domain_id: int, record_id: int) -> DomainRecord:
        """Get a specific domain record."""
        endpoint = f"/domains/{domain_id}/records/{record_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_domain_record(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetDomainRecord", e) from e

    async def list_firewalls(self) -> list[Firewall]:
        """List firewalls."""
        try:
            response = await self.make_request("GET", "/networking/firewalls")
            data = response.json()
            return [self._parse_firewall(f) for f in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListFirewalls", e) from e

    async def get_firewall(self, firewall_id: int) -> Firewall:
        """Get a specific firewall."""
        endpoint = f"/networking/firewalls/{firewall_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_firewall(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewall", e) from e

    async def get_firewall_settings(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List default firewall settings."""
        endpoint = "/networking/firewalls/settings"
        for name, value in (("page", page), ("page_size", page_size)):
            if value is not None and (type(value) is not int or value <= 0):
                msg = f"{name} must be a positive integer"
                raise ValueError(msg)
        params: dict[str, Any] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint = f"{endpoint}?{urlencode(params)}"
        try:
            response = await self.make_request("GET", endpoint)
            return cast("dict[str, Any]", response.json())
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallSettings", e) from e

    async def update_firewall_settings(
        self, default_firewall_ids: dict[str, int]
    ) -> dict[str, Any]:
        """Update default firewalls."""
        valid_keys = {
            "linode",
            "nodebalancer",
            "public_interface",
            "vpc_interface",
        }
        invalid_keys = set(default_firewall_ids) - valid_keys
        if invalid_keys:
            keys = ", ".join(sorted(invalid_keys))
            msg = f"default_firewall_ids contains unsupported keys: {keys}"
            raise ValueError(msg)
        if not default_firewall_ids:
            msg = "default_firewall_ids must contain at least one firewall ID"
            raise ValueError(msg)
        for key, value in default_firewall_ids.items():
            if type(value) is not int or value <= 0:
                msg = f"default_firewall_ids.{key} must be a positive integer"
                raise ValueError(msg)

        body = {"default_firewall_ids": default_firewall_ids}

        try:
            response = await self.make_request(
                "PUT", "/networking/firewalls/settings", body
            )
            return cast("dict[str, Any]", response.json())
        except httpx.HTTPError as e:
            raise NetworkError("UpdateFirewallSettings", e) from e

    async def list_firewall_templates(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List firewall templates."""
        endpoint = "/networking/firewalls/templates"
        params: dict[str, Any] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint = f"{endpoint}?{urlencode(params)}"
        try:
            response = await self.make_request("GET", endpoint)
            return cast("dict[str, Any]", response.json())
        except httpx.HTTPError as e:
            raise NetworkError("ListFirewallTemplates", e) from e

    async def get_firewall_template(
        self, slug: str, page: int | None = None, page_size: int | None = None
    ) -> FirewallTemplate:
        """Get a firewall template by slug."""
        safe_slug = quote(slug, safe="")
        endpoint = f"/networking/firewalls/templates/{safe_slug}"
        params: dict[str, Any] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint = f"{endpoint}?{urlencode(params)}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_firewall_template(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallTemplate", e) from e

    async def get_firewall_rules(self, firewall_id: int) -> FirewallRules:
        """Get firewall rules for a specific firewall."""
        endpoint = f"/networking/firewalls/{firewall_id}/rules"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_firewall_rules(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallRules", e) from e

    async def get_firewall_rule_version(
        self, firewall_id: int, version: str
    ) -> FirewallRule:
        """Get a specific firewall rule version."""
        safe_version = quote(version, safe="")
        endpoint = f"/networking/firewalls/{firewall_id}/history/rules/{safe_version}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_firewall_rule(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallRuleVersion", e) from e

    async def get_firewall_device(
        self, firewall_id: int, device_id: int
    ) -> dict[str, Any]:
        """Get a specific firewall device."""
        safe_firewall_id = quote(str(firewall_id), safe="")
        safe_device_id = quote(str(device_id), safe="")
        endpoint = f"/networking/firewalls/{safe_firewall_id}/devices/{safe_device_id}"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallDevice", e) from e

    async def delete_firewall_device(
        self, firewall_id: int | str, device_id: int | str
    ) -> None:
        """Delete a device from a Cloud Firewall."""
        safe_firewall_id = quote(str(firewall_id), safe="")
        safe_device_id = quote(str(device_id), safe="")
        endpoint = f"/networking/firewalls/{safe_firewall_id}/devices/{safe_device_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteFirewallDevice", e) from e

    async def list_firewall_devices(
        self,
        firewall_id: int | str,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List devices attached to a Cloud Firewall."""
        safe_firewall_id = quote(str(firewall_id), safe="")
        endpoint = f"/networking/firewalls/{safe_firewall_id}/devices"
        params: dict[str, Any] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint = f"{endpoint}?{urlencode(params)}"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("ListFirewallDevices", e) from e

    async def list_firewall_rule_versions(self, firewall_id: int) -> list[Firewall]:
        """List firewall rule versions (history)."""
        endpoint = f"/networking/firewalls/{firewall_id}/history"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return [self._parse_firewall(f) for f in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListFirewallRuleVersions", e) from e

    async def list_vlans(self) -> list[dict[str, Any]]:
        """List VLANs."""
        try:
            response = await self.make_request("GET", "/networking/vlans")
            data = response.json()
            vlans: list[dict[str, Any]] = data.get("data", [])
            return vlans
        except httpx.HTTPError as e:
            raise NetworkError("ListVLANs", e) from e

    async def delete_vlan(self, region_id: str, label: str) -> None:
        """Delete a VLAN."""
        endpoint = f"/networking/vlans/{region_id}/{label}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteVLAN", e) from e

    async def share_ipv4s(self, ips: list[str], linode_id: int) -> dict[str, Any]:
        """Share IPv4 addresses with a Linode."""
        try:
            body: dict[str, Any] = {"ips": ips, "linode_id": linode_id}
            response = await self.make_request("POST", "/networking/ips/share", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ShareIPv4s", e) from e

    async def assign_ipv4s(
        self, region: str, assignments: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Assign IPv4 addresses to Linodes."""
        try:
            body: dict[str, Any] = {
                "region": region,
                "assignments": assignments,
            }
            response = await self.make_request("POST", "/networking/ips/assign", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("AssignIPv4s", e) from e

    async def list_tags(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account tags."""
        endpoint = "/tags"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListTags", e) from e

    async def list_tagged_objects(
        self, tag_label: str, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List objects assigned to a tag."""
        encoded_label = quote(tag_label, safe="")
        endpoint = f"/tags/{encoded_label}"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListTaggedObjects", e) from e

    async def create_tag(
        self,
        label: str,
        domains: list[int] | None = None,
        linodes: list[int] | None = None,
        nodebalancers: list[int] | None = None,
        volumes: list[int] | None = None,
    ) -> dict[str, Any]:
        """Create a tag and optionally assign supported resources."""
        body: dict[str, Any] = {"label": label}
        if domains is not None:
            body["domains"] = domains
        if linodes is not None:
            body["linodes"] = linodes
        if nodebalancers is not None:
            body["nodebalancers"] = nodebalancers
        if volumes is not None:
            body["volumes"] = volumes
        try:
            response = await self.make_request("POST", "/tags", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateTag", e) from e

    async def delete_tag(self, tag_label: str) -> None:
        """Delete a tag."""
        encoded_label = quote(tag_label, safe="")
        endpoint = f"/tags/{encoded_label}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteTag", e) from e

    async def create_support_ticket(
        self,
        summary: str,
        description: str,
        *,
        bucket: str | None = None,
        database_id: int | None = None,
        domain_id: int | None = None,
        firewall_id: int | None = None,
        linode_id: int | None = None,
        lkecluster_id: int | None = None,
        longviewclient_id: int | None = None,
        managed_issue: bool | None = None,
        nodebalancer_id: int | None = None,
        region: str | None = None,
        severity: int | None = None,
        vlan: str | None = None,
        volume_id: int | None = None,
        vpc_id: int | None = None,
    ) -> dict[str, Any]:
        """Create a support ticket."""
        body: dict[str, Any] = {
            "summary": summary,
            "description": description,
        }
        optional_fields: dict[str, Any] = {
            "bucket": bucket,
            "database_id": database_id,
            "domain_id": domain_id,
            "firewall_id": firewall_id,
            "linode_id": linode_id,
            "lkecluster_id": lkecluster_id,
            "longviewclient_id": longviewclient_id,
            "managed_issue": managed_issue,
            "nodebalancer_id": nodebalancer_id,
            "region": region,
            "severity": severity,
            "vlan": vlan,
            "volume_id": volume_id,
            "vpc_id": vpc_id,
        }
        body.update(
            {key: value for key, value in optional_fields.items() if value is not None}
        )
        try:
            response = await self.make_request("POST", "/support/tickets", body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateSupportTicket", e) from e

    async def get_managed_stats(self) -> dict[str, Any]:
        """List Managed statistics from the last 24 hours."""
        try:
            response = await self.make_request("GET", "/managed/stats")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetManagedStats", e) from e

    async def list_support_tickets(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List support tickets."""
        endpoint = "/support/tickets"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListSupportTickets", e) from e

    async def get_support_ticket(self, ticket_id: int) -> dict[str, Any]:
        """Get a support ticket."""
        endpoint = f"/support/tickets/{ticket_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetSupportTicket", e) from e

    async def list_support_ticket_replies(
        self, ticket_id: int, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List replies for a support ticket."""
        endpoint = f"/support/tickets/{ticket_id}/replies"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListSupportTicketReplies", e) from e

    async def create_support_ticket_reply(
        self, ticket_id: int, description: str
    ) -> dict[str, Any]:
        """Create a reply for a support ticket."""
        endpoint = f"/support/tickets/{ticket_id}/replies"
        try:
            response = await self.make_request(
                "POST", endpoint, {"description": description}
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateSupportTicketReply", e) from e

    async def close_support_ticket(self, ticket_id: int) -> dict[str, Any]:
        """Close a support ticket."""
        endpoint = f"/support/tickets/{ticket_id}/close"
        try:
            response = await self.make_request("POST", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CloseSupportTicket", e) from e

    async def create_support_ticket_attachment(
        self, ticket_id: int, file: str
    ) -> dict[str, Any]:
        """Create an attachment for a support ticket from a local file path."""
        endpoint = f"/support/tickets/{ticket_id}/attachments"
        try:
            response = await self.make_file_request("POST", endpoint, file)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateSupportTicketAttachment", e) from e

    async def list_nodebalancers(self) -> list[NodeBalancer]:
        """List NodeBalancers."""
        try:
            response = await self.make_request("GET", "/nodebalancers")
            data = response.json()
            return [self._parse_nodebalancer(nb) for nb in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancers", e) from e

    async def list_nodebalancer_types(self) -> list[dict[str, Any]]:
        """List NodeBalancer types."""
        try:
            response = await self.make_request("GET", "/nodebalancers/types")
            data = response.json()
            types: list[dict[str, Any]] = data.get("data", [])
            return types
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerTypes", e) from e

    async def get_nodebalancer(self, nodebalancer_id: int) -> NodeBalancer:
        """Get a specific NodeBalancer."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            return self._parse_nodebalancer(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancer", e) from e

    async def get_nodebalancer_stats(self, nodebalancer_id: int) -> dict[str, Any]:
        """Get statistics for a specific NodeBalancer."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}/stats"
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancerStats", e) from e

    async def get_nodebalancer_vpc_config(
        self, nodebalancer_id: int, vpc_config_id: int
    ) -> dict[str, Any]:
        """Get a NodeBalancer VPC configuration."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_vpc_config_id = quote(str(vpc_config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/vpcs/{encoded_vpc_config_id}"
        )
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancerVPCConfig", e) from e

    async def list_nodebalancer_vpc_configs(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List VPC configurations for a NodeBalancer."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}/vpcs"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerVPCConfigs", e) from e

    async def update_nodebalancer_firewalls(
        self,
        nodebalancer_id: int,
        firewall_ids: list[int],
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """Update firewall assignments for a NodeBalancer."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}/firewalls"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request(
                "PUT", endpoint, {"firewall_ids": firewall_ids}
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateNodeBalancerFirewalls", e) from e

    async def rebuild_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> dict[str, Any]:
        """Rebuild a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
            f"{encoded_config_id}/rebuild"
        )
        try:
            response = await self.make_request("POST", endpoint, {})
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("RebuildNodeBalancerConfig", e) from e

    async def list_nodebalancer_configs(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List configs for a NodeBalancer."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}/configs"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerConfigs", e) from e

    async def create_nodebalancer_config(
        self, nodebalancer_id: int, fields: dict[str, Any]
    ) -> dict[str, Any]:
        """Create a NodeBalancer config."""
        if (
            not isinstance(nodebalancer_id, int)  # pyright: ignore[reportUnnecessaryIsInstance]
            or isinstance(nodebalancer_id, bool)
            or nodebalancer_id < 1
        ):
            raise ValueError("nodebalancer_id must be a positive integer")
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}/configs"
        try:
            response = await self.make_request("POST", endpoint, fields)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateNodeBalancerConfig", e) from e

    async def get_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> dict[str, Any]:
        """Get a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/{encoded_config_id}"
        )
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancerConfig", e) from e

    async def update_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int, fields: dict[str, Any]
    ) -> dict[str, Any]:
        """Update a NodeBalancer config."""
        if (
            not isinstance(nodebalancer_id, int)  # pyright: ignore[reportUnnecessaryIsInstance]
            or isinstance(nodebalancer_id, bool)
            or nodebalancer_id < 1
        ):
            raise ValueError("nodebalancer_id must be a positive integer")
        if (
            not isinstance(config_id, int)  # pyright: ignore[reportUnnecessaryIsInstance]
            or isinstance(config_id, bool)
            or config_id < 1
        ):
            raise ValueError("config_id must be a positive integer")
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/{encoded_config_id}"
        )
        logger.info(
            "Updating NodeBalancer config",
            extra={
                "nodebalancer_id": nodebalancer_id,
                "config_id": config_id,
            },
        )
        try:
            response = await self.make_request("PUT", endpoint, fields)
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating config: %s", e)
            raise NetworkError("UpdateNodeBalancerConfig", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating config: %s", e)
            raise NetworkError("UpdateNodeBalancerConfig", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error updating config: status %d", e.response.status_code
            )
            raise NetworkError("UpdateNodeBalancerConfig", e) from e
        except httpx.HTTPError as e:
            raise NetworkError("UpdateNodeBalancerConfig", e) from e

    async def delete_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> None:
        """Delete a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/{encoded_config_id}"
        )
        logger.info(
            "Deleting NodeBalancer config",
            extra={
                "nodebalancer_id": nodebalancer_id,
                "config_id": config_id,
            },
        )
        try:
            await self.make_request("DELETE", endpoint)
            logger.info(
                "NodeBalancer config deleted",
                extra={
                    "nodebalancer_id": nodebalancer_id,
                    "config_id": config_id,
                },
            )
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting config: %s", e)
            raise NetworkError("DeleteNodeBalancerConfig", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting config: %s", e)
            raise NetworkError("DeleteNodeBalancerConfig", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error deleting config: status %d", e.response.status_code
            )
            raise NetworkError("DeleteNodeBalancerConfig", e) from e
        except httpx.HTTPError as e:
            raise NetworkError("DeleteNodeBalancerConfig", e) from e

    async def list_nodebalancer_config_nodes(
        self,
        nodebalancer_id: int,
        config_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List nodes in a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
            f"{encoded_config_id}/nodes"
        )
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerConfigNodes", e) from e

    async def create_nodebalancer_config_node(
        self,
        nodebalancer_id: int,
        config_id: int,
        fields: dict[str, Any],
    ) -> dict[str, Any]:
        """Create a node in a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
            f"{encoded_config_id}/nodes"
        )
        try:
            response = await self.make_request("POST", endpoint, fields)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CreateNodeBalancerConfigNode", e) from e

    async def update_nodebalancer_config_node(
        self,
        nodebalancer_id: int,
        config_id: int,
        node_id: int,
        fields: dict[str, Any],
    ) -> dict[str, Any]:
        """Update a node in a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        encoded_node_id = quote(str(node_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
            f"{encoded_config_id}/nodes/{encoded_node_id}"
        )
        try:
            response = await self.make_request("PUT", endpoint, fields)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("UpdateNodeBalancerConfigNode", e) from e

    async def delete_nodebalancer_config_node(
        self, nodebalancer_id: int, config_id: int, node_id: int
    ) -> None:
        """Delete a node from a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        encoded_node_id = quote(str(node_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
            f"{encoded_config_id}/nodes/{encoded_node_id}"
        )
        logger.info(
            "Deleting NodeBalancer config node",
            extra={
                "nodebalancer_id": nodebalancer_id,
                "config_id": config_id,
                "node_id": node_id,
            },
        )
        try:
            await self.make_request("DELETE", endpoint)
            logger.info(
                "NodeBalancer config node deleted",
                extra={
                    "nodebalancer_id": nodebalancer_id,
                    "config_id": config_id,
                    "node_id": node_id,
                },
            )
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting config node: %s", e)
            raise NetworkError("DeleteNodeBalancerConfigNode", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting config node: %s", e)
            raise NetworkError("DeleteNodeBalancerConfigNode", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error deleting config node: status %d", e.response.status_code
            )
            raise NetworkError("DeleteNodeBalancerConfigNode", e) from e
        except httpx.HTTPError as e:
            raise NetworkError("DeleteNodeBalancerConfigNode", e) from e

    async def get_nodebalancer_config_node(
        self, nodebalancer_id: int, config_id: int, node_id: int
    ) -> dict[str, Any]:
        """Get a node from a NodeBalancer config."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        encoded_config_id = quote(str(config_id), safe="")
        encoded_node_id = quote(str(node_id), safe="")
        endpoint = (
            f"/nodebalancers/{encoded_nodebalancer_id}/configs/"
            f"{encoded_config_id}/nodes/{encoded_node_id}"
        )
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancerConfigNode", e) from e

    async def list_nodebalancer_firewalls(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List firewalls assigned to a NodeBalancer."""
        encoded_nodebalancer_id = quote(str(nodebalancer_id), safe="")
        endpoint = f"/nodebalancers/{encoded_nodebalancer_id}/firewalls"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)
        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerFirewalls", e) from e

    async def list_stackscripts(self) -> list[StackScript]:
        """List StackScripts."""
        try:
            response = await self.make_request("GET", "/linode/stackscripts")
            data = response.json()
            return [self._parse_stackscript(s) for s in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListStackScripts", e) from e

    async def create_stackscript(
        self,
        label: str,
        images: list[str],
        script: str,
        description: str | None = None,
        is_public: bool | None = None,
        rev_note: str | None = None,
    ) -> StackScript:
        """Create a StackScript."""
        body: dict[str, Any] = {
            "label": label,
            "images": images,
            "script": script,
        }
        if description is not None:
            body["description"] = description
        if is_public is not None:
            body["is_public"] = is_public
        if rev_note is not None:
            body["rev_note"] = rev_note

        try:
            response = await self.make_request("POST", "/linode/stackscripts", body)
            data = response.json()
            return self._parse_stackscript(data)
        except httpx.HTTPError as e:
            raise NetworkError("CreateStackScript", e) from e

    # Phase 1: Object Storage read operations

    async def list_object_storage_buckets(self) -> list[dict[str, Any]]:
        """List Object Storage buckets."""
        try:
            response = await self.make_request("GET", "/object-storage/buckets")
            data = response.json()
            buckets: list[dict[str, Any]] = data.get("data", [])
            return buckets
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageBuckets", e) from e

    async def list_object_storage_buckets_for_region(
        self, region_id: str
    ) -> list[dict[str, Any]]:
        """List Object Storage buckets in a region."""
        encoded_region_id = quote(str(region_id), safe="")
        try:
            response = await self.make_request(
                "GET", f"/object-storage/buckets/{encoded_region_id}"
            )
            data = response.json()
            buckets: list[dict[str, Any]] = data.get("data", [])
            return buckets
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageBucketsForRegion", e) from e

    async def get_object_storage_bucket(
        self, region: str, label: str
    ) -> dict[str, Any]:
        """Get a specific Object Storage bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}"
        try:
            response = await self.make_request("GET", endpoint)
            bucket: dict[str, Any] = response.json()
            return bucket
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageBucket", e) from e

    async def list_object_storage_bucket_contents(
        self, region: str, label: str, params: dict[str, str] | None = None
    ) -> dict[str, Any]:
        """List contents of an Object Storage bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}/object-list"

        if params:
            filtered = {
                key: params[key]
                for key in ("prefix", "delimiter", "marker", "page_size")
                if key in params
            }
            if filtered:
                endpoint += "?" + urlencode(filtered)

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageBucketContents", e) from e

    async def list_object_storage_clusters(self) -> list[dict[str, Any]]:
        """List Object Storage clusters."""
        try:
            response = await self.make_request("GET", "/object-storage/clusters")
            data = response.json()
            clusters: list[dict[str, Any]] = data.get("data", [])
            return clusters
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageClusters", e) from e

    async def get_object_storage_cluster(self, cluster_id: str) -> dict[str, Any]:
        """Get a specific Object Storage cluster."""
        encoded_cluster_id = quote(str(cluster_id), safe="")
        endpoint = f"/object-storage/clusters/{encoded_cluster_id}"
        try:
            response = await self.make_request("GET", endpoint)
            cluster: dict[str, Any] = response.json()
            return cluster
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageCluster", e) from e

    async def list_object_storage_endpoints(self) -> list[dict[str, Any]]:
        """List Object Storage endpoints."""
        try:
            response = await self.make_request("GET", "/object-storage/endpoints")
            data = response.json()
            endpoints: list[dict[str, Any]] = data.get("data", [])
            return endpoints
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageEndpoints", e) from e

    async def list_object_storage_types(self) -> list[dict[str, Any]]:
        """List Object Storage types/pricing."""
        try:
            response = await self.make_request("GET", "/object-storage/types")
            data = response.json()
            types: list[dict[str, Any]] = data.get("data", [])
            return types
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageTypes", e) from e

    async def list_object_storage_keys(self) -> list[dict[str, Any]]:
        """List all Object Storage access keys."""
        try:
            response = await self.make_request("GET", "/object-storage/keys")
            data = response.json()
            keys: list[dict[str, Any]] = data.get("data", [])
            return keys
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageKeys", e) from e

    async def get_object_storage_key(self, key_id: int) -> dict[str, Any]:
        """Get a specific Object Storage access key."""
        endpoint = f"/object-storage/keys/{key_id}"
        try:
            response = await self.make_request("GET", endpoint)
            key: dict[str, Any] = response.json()
            return key
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageKey", e) from e

    async def get_object_storage_transfer(self) -> dict[str, Any]:
        """Get Object Storage outbound data transfer usage."""
        try:
            response = await self.make_request("GET", "/object-storage/transfer")
            transfer: dict[str, Any] = response.json()
            return transfer
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageTransfer", e) from e

    async def get_network_transfer_prices(self) -> dict[str, Any]:
        """Get network transfer prices."""
        try:
            response = await self.make_request("GET", "/network-transfer/prices")
            prices: dict[str, Any] = response.json()
            return prices
        except httpx.HTTPError as e:
            raise NetworkError("GetNetworkTransferPrices", e) from e

    async def cancel_object_storage(self) -> dict[str, Any]:
        """Cancel Object Storage service for the account."""
        try:
            response = await self.make_request("POST", "/object-storage/cancel")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("CancelObjectStorage", e) from e

    async def list_object_storage_quotas(self) -> list[dict[str, Any]]:
        """List Object Storage quotas."""
        try:
            response = await self.make_request("GET", "/object-storage/quotas")
            data = response.json()
            quotas: list[dict[str, Any]] = data.get("data", [])
            return quotas
        except httpx.HTTPError as e:
            raise NetworkError("ListObjectStorageQuotas", e) from e

    async def get_object_storage_quota(self, obj_quota_id: str) -> dict[str, Any]:
        """Get a single Object Storage quota."""
        encoded_quota_id = quote(obj_quota_id, safe="")
        endpoint = f"/object-storage/quotas/{encoded_quota_id}"
        try:
            response = await self.make_request("GET", endpoint)
            quota: dict[str, Any] = response.json()
            return quota
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageQuota", e) from e

    async def get_object_storage_quota_usage(
        self, obj_quota_id: int | str
    ) -> dict[str, Any]:
        """Get Object Storage quota usage data."""
        encoded_quota_id = quote(str(obj_quota_id), safe="")
        endpoint = f"/object-storage/quotas/{encoded_quota_id}/usage"
        try:
            response = await self.make_request("GET", endpoint)
            usage: dict[str, Any] = response.json()
            return usage
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageQuotaUsage", e) from e

    async def get_object_storage_bucket_access(
        self, region: str, label: str
    ) -> dict[str, Any]:
        """Get bucket ACL and CORS settings."""
        endpoint = f"/object-storage/buckets/{region}/{label}/access"
        try:
            response = await self.make_request("GET", endpoint)
            access: dict[str, Any] = response.json()
            return access
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageBucketAccess", e) from e

    # Stage 5 Phase 3: Object Storage write operations

    async def create_object_storage_bucket(
        self,
        label: str,
        region: str,
        acl: str | None = None,
        cors_enabled: bool | None = None,
    ) -> dict[str, Any]:
        """Create a new Object Storage bucket."""
        try:
            body: dict[str, Any] = {
                "label": label,
                "region": region,
            }
            if acl is not None:
                body["acl"] = acl
            if cors_enabled is not None:
                body["cors_enabled"] = cors_enabled
            response = await self.make_request("POST", "/object-storage/buckets", body)
            bucket: dict[str, Any] = response.json()
            return bucket
        except httpx.HTTPError as e:
            raise NetworkError("CreateObjectStorageBucket", e) from e

    async def delete_object_storage_bucket(self, region: str, label: str) -> None:
        """Delete an Object Storage bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteObjectStorageBucket", e) from e

    async def update_object_storage_bucket_access(
        self,
        region: str,
        label: str,
        acl: str | None = None,
        cors_enabled: bool | None = None,
    ) -> None:
        """Update bucket ACL and CORS settings."""
        endpoint = f"/object-storage/buckets/{region}/{label}/access"
        try:
            body: dict[str, Any] = {}
            if acl is not None:
                body["acl"] = acl
            if cors_enabled is not None:
                body["cors_enabled"] = cors_enabled
            await self.make_request("PUT", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateObjectStorageBucketAccess", e) from e

    async def allow_object_storage_bucket_access(
        self,
        region: str,
        label: str,
        acl: str | None = None,
        cors_enabled: bool | None = None,
    ) -> dict[str, Any]:
        """Allow access to an Object Storage bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}/access"
        try:
            body: dict[str, Any] = {}
            if acl is not None:
                body["acl"] = acl
            if cors_enabled is not None:
                body["cors_enabled"] = cors_enabled
            response = await self.make_request("POST", endpoint, body)
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("AllowObjectStorageBucketAccess", e) from e

    # Stage 5 Phase 4: Object Storage access key write operations

    async def create_object_storage_key(
        self,
        label: str,
        bucket_access: list[dict[str, str]] | None = None,
    ) -> dict[str, Any]:
        """Create a new Object Storage access key."""
        try:
            body: dict[str, Any] = {"label": label}
            if bucket_access is not None:
                body["bucket_access"] = bucket_access
            response = await self.make_request("POST", "/object-storage/keys", body)
            key: dict[str, Any] = response.json()
            return key
        except httpx.HTTPError as e:
            raise NetworkError("CreateObjectStorageKey", e) from e

    async def update_object_storage_key(
        self,
        key_id: int,
        label: str | None = None,
        bucket_access: list[dict[str, str]] | None = None,
    ) -> None:
        """Update an Object Storage access key."""
        endpoint = f"/object-storage/keys/{key_id}"
        try:
            body: dict[str, Any] = {}
            if label is not None:
                body["label"] = label
            if bucket_access is not None:
                body["bucket_access"] = bucket_access
            await self.make_request("PUT", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateObjectStorageKey", e) from e

    async def delete_object_storage_key(self, key_id: int) -> None:
        """Delete (revoke) an Object Storage access key."""
        endpoint = f"/object-storage/keys/{key_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteObjectStorageKey", e) from e

    # Stage 5 Phase 5: Presigned URLs, Object ACL, and SSL.

    async def create_presigned_url(
        self,
        region: str,
        label: str,
        name: str,
        method: str,
        expires_in: int = 3600,
    ) -> dict[str, Any]:
        """Generate a presigned URL for an object."""
        endpoint = f"/object-storage/buckets/{region}/{label}/object-url"
        body: dict[str, Any] = {
            "method": method,
            "name": name,
            "expires_in": expires_in,
        }
        try:
            response = await self.make_request("POST", endpoint, body)
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("CreatePresignedURL", e) from e

    async def get_object_acl(
        self, region: str, label: str, name: str
    ) -> dict[str, Any]:
        """Get the ACL for an object in Object Storage."""
        endpoint = f"/object-storage/buckets/{region}/{label}/object-acl?" + urlencode(
            {"name": name}
        )
        try:
            response = await self.make_request("GET", endpoint)
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectACL", e) from e

    async def update_object_acl(
        self, region: str, label: str, name: str, acl: str
    ) -> dict[str, Any]:
        """Update the ACL for an object in Object Storage."""
        endpoint = f"/object-storage/buckets/{region}/{label}/object-acl"
        body = {"acl": acl, "name": name}
        try:
            response = await self.make_request("PUT", endpoint, body)
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("UpdateObjectACL", e) from e

    async def get_bucket_ssl(self, region: str, label: str) -> dict[str, Any]:
        """Get the SSL/TLS certificate status for a bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}/ssl"
        try:
            response = await self.make_request("GET", endpoint)
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("GetBucketSSL", e) from e

    async def upload_bucket_ssl(
        self, region: str, label: str, certificate: str, private_key: str
    ) -> dict[str, Any]:
        """Upload an SSL/TLS certificate for a bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}/ssl"
        body = {"certificate": certificate, "private_key": private_key}
        try:
            response = await self.make_request("POST", endpoint, body)
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("UploadBucketSSL", e) from e

    async def delete_bucket_ssl(self, region: str, label: str) -> None:
        """Delete the SSL/TLS certificate from a bucket."""
        endpoint = f"/object-storage/buckets/{region}/{label}/ssl"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteBucketSSL", e) from e

    # Stage 4: Write operations

    async def create_ssh_key(self, label: str, ssh_key: str) -> SSHKey:
        """Create a new SSH key."""
        validate_label(label)
        validate_ssh_key(ssh_key)

        logger.info("Creating SSH key", extra={"label": label})

        try:
            body = {"label": label, "ssh_key": ssh_key}
            response = await self.make_request("POST", "/profile/sshkeys", body)
            data = response.json()
            result = self._parse_ssh_key(data)
            logger.info("SSH key created", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating SSH key: %s", e)
            raise NetworkError("CreateSSHKey", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating SSH key: %s", e)
            raise NetworkError("CreateSSHKey", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating SSH key")
            raise NetworkError("CreateSSHKey", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating SSH key: %s", e)
            raise NetworkError("CreateSSHKey", e) from e

    async def update_ssh_key(self, ssh_key_id: int, label: str) -> SSHKey:
        """Update an SSH key."""
        validate_label(label)
        endpoint = f"/profile/sshkeys/{ssh_key_id}"

        logger.info("Updating SSH key", extra={"ssh_key_id": ssh_key_id})

        try:
            body = {"label": label}
            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = self._parse_ssh_key(data)
            logger.info("SSH key updated", extra={"ssh_key_id": ssh_key_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating SSH key: %s", e)
            raise NetworkError("UpdateSSHKey", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating SSH key: %s", e)
            raise NetworkError("UpdateSSHKey", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating SSH key")
            raise NetworkError("UpdateSSHKey", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating SSH key: %s", e)
            raise NetworkError("UpdateSSHKey", e) from e

    async def delete_ssh_key(self, ssh_key_id: int) -> None:
        """Delete an SSH key."""
        endpoint = f"/profile/sshkeys/{ssh_key_id}"
        logger.info("Deleting SSH key", extra={"ssh_key_id": ssh_key_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("SSH key deleted", extra={"ssh_key_id": ssh_key_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting SSH key: %s", e)
            raise NetworkError("DeleteSSHKey", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting SSH key: %s", e)
            raise NetworkError("DeleteSSHKey", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting SSH key")
            raise NetworkError("DeleteSSHKey", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting SSH key: %s", e)
            raise NetworkError("DeleteSSHKey", e) from e

    async def create_profile_tfa_secret(self) -> dict[str, Any]:
        """Create a two-factor authentication secret."""
        logger.info("Creating profile two-factor authentication secret")

        try:
            response = await self.make_request("POST", "/profile/tfa-enable")
            result: dict[str, Any] = response.json()
            logger.info("Profile two-factor authentication secret created")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout creating profile two-factor authentication "
                "secret: %s",
                e,
            )
            raise NetworkError("CreateProfileTFASecret", e) from e
        except httpx.ReadTimeout as e:
            logger.exception(
                "Read timeout creating profile two-factor authentication secret: %s", e
            )
            raise NetworkError("CreateProfileTFASecret", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error creating profile two-factor authentication secret"
            )
            raise NetworkError("CreateProfileTFASecret", e) from e
        except httpx.HTTPError as e:
            logger.exception(
                "HTTP error creating profile two-factor authentication secret: %s", e
            )
            raise NetworkError("CreateProfileTFASecret", e) from e

    async def confirm_profile_tfa_enable(
        self, tfa_code: str | None = None
    ) -> dict[str, Any]:
        """Confirm two-factor authentication enablement."""
        body: dict[str, Any] = {}
        if tfa_code is not None:
            body["tfa_code"] = tfa_code

        logger.info("Confirming profile two-factor authentication enablement")

        try:
            response = await self.make_request(
                "POST", "/profile/tfa-enable-confirm", body
            )
            result: dict[str, Any] = response.json()
            logger.info("Profile two-factor authentication enablement confirmed")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout confirming profile two-factor authentication: %s", e
            )
            raise NetworkError("ConfirmProfileTFAEnable", e) from e
        except httpx.ReadTimeout as e:
            logger.exception(
                "Read timeout confirming profile two-factor authentication: %s", e
            )
            raise NetworkError("ConfirmProfileTFAEnable", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error confirming profile two-factor authentication")
            raise NetworkError("ConfirmProfileTFAEnable", e) from e
        except httpx.HTTPError as e:
            logger.exception(
                "HTTP error confirming profile two-factor authentication: %s", e
            )
            raise NetworkError("ConfirmProfileTFAEnable", e) from e

    async def disable_profile_tfa(self) -> dict[str, Any]:
        """Disable two-factor authentication."""
        logger.info("Disabling profile two-factor authentication")

        try:
            response = await self.make_request("POST", "/profile/tfa-disable")
            result: dict[str, Any] = response.json()
            logger.info("Profile two-factor authentication disabled")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout disabling profile two-factor authentication: %s", e
            )
            raise NetworkError("DisableProfileTFA", e) from e
        except httpx.ReadTimeout as e:
            logger.exception(
                "Read timeout disabling profile two-factor authentication: %s", e
            )
            raise NetworkError("DisableProfileTFA", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error disabling profile two-factor authentication")
            raise NetworkError("DisableProfileTFA", e) from e
        except httpx.HTTPError as e:
            logger.exception(
                "HTTP error disabling profile two-factor authentication: %s", e
            )
            raise NetworkError("DisableProfileTFA", e) from e

    async def send_profile_phone_number_verification(
        self, iso_code: str, phone_number: str
    ) -> dict[str, Any]:
        """Send a profile phone number verification code."""
        body = {"iso_code": iso_code, "phone_number": phone_number}
        logger.info("Sending profile phone number verification code")

        try:
            response = await self.make_request("POST", "/profile/phone-number", body)
            result: dict[str, Any] = response.json()
            logger.info("Profile phone number verification code sent")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout sending profile phone number verification code: %s",
                e,
            )
            raise NetworkError("SendProfilePhoneNumberVerification", e) from e
        except httpx.ReadTimeout as e:
            logger.exception(
                "Read timeout sending profile phone number verification code: %s", e
            )
            raise NetworkError("SendProfilePhoneNumberVerification", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error sending profile phone number verification code"
            )
            raise NetworkError("SendProfilePhoneNumberVerification", e) from e
        except httpx.HTTPError as e:
            logger.exception(
                "HTTP error sending profile phone number verification code: %s", e
            )
            raise NetworkError("SendProfilePhoneNumberVerification", e) from e

    async def verify_profile_phone_number(self, otp_code: str) -> dict[str, Any]:
        """Verify a profile phone number with a one-time SMS code."""
        body = {"otp_code": otp_code}
        logger.info("Verifying profile phone number")

        try:
            response = await self.make_request(
                "POST", "/profile/phone-number/verify", body
            )
            result: dict[str, Any] = response.json()
            logger.info("Profile phone number verified")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout verifying profile phone number: %s", e)
            raise NetworkError("VerifyProfilePhoneNumber", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout verifying profile phone number: %s", e)
            raise NetworkError("VerifyProfilePhoneNumber", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error verifying profile phone number")
            raise NetworkError("VerifyProfilePhoneNumber", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error verifying profile phone number: %s", e)
            raise NetworkError("VerifyProfilePhoneNumber", e) from e

    async def delete_profile_phone_number(self) -> dict[str, Any]:
        """Delete the verified profile phone number."""
        logger.info("Deleting profile phone number")

        try:
            response = await self.make_request("DELETE", "/profile/phone-number")
            result: dict[str, Any] = response.json()
            logger.info("Profile phone number deleted")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting profile phone number: %s", e)
            raise NetworkError("DeleteProfilePhoneNumber", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting profile phone number: %s", e)
            raise NetworkError("DeleteProfilePhoneNumber", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting profile phone number")
            raise NetworkError("DeleteProfilePhoneNumber", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting profile phone number: %s", e)
            raise NetworkError("DeleteProfilePhoneNumber", e) from e

    async def list_profile_security_questions(self) -> dict[str, Any]:
        """List available profile security questions."""
        logger.info("Listing profile security questions")

        try:
            response = await self.make_request("GET", "/profile/security-questions")
            result: dict[str, Any] = response.json()
            logger.info("Profile security questions listed")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout listing profile security questions: %s", e
            )
            raise NetworkError("ListProfileSecurityQuestions", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing profile security questions: %s", e)
            raise NetworkError("ListProfileSecurityQuestions", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing profile security questions")
            raise NetworkError("ListProfileSecurityQuestions", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing profile security questions: %s", e)
            raise NetworkError("ListProfileSecurityQuestions", e) from e

    async def answer_profile_security_questions(
        self, security_questions: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Answer profile security questions."""
        body = build_profile_security_questions_body(security_questions)
        logger.info(
            "Answering profile security questions",
            extra={"security_question_count": len(security_questions)},
        )

        try:
            response = await self.make_request(
                "POST", "/profile/security-questions", body
            )
            result: dict[str, Any] = response.json()
            logger.info("Profile security questions answered")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout answering profile security questions: %s", e
            )
            raise NetworkError("AnswerProfileSecurityQuestions", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout answering profile security questions: %s", e)
            raise NetworkError("AnswerProfileSecurityQuestions", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error answering profile security questions")
            raise NetworkError("AnswerProfileSecurityQuestions", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error answering profile security questions: %s", e)
            raise NetworkError("AnswerProfileSecurityQuestions", e) from e

    async def create_profile_token(
        self,
        expiry: str | None = None,
        label: str | None = None,
        scopes: str | None = None,
    ) -> dict[str, Any]:
        """Create a personal access token."""
        body = build_profile_token_create_body(
            expiry=expiry, label=label, scopes=scopes
        )

        logger.info("Creating profile token")

        try:
            response = await self.make_request("POST", "/profile/tokens", body)
            result: dict[str, Any] = response.json()
            logger.info("Profile token created", extra={"token_id": result.get("id")})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating profile token: %s", e)
            raise NetworkError("CreateProfileToken", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating profile token: %s", e)
            raise NetworkError("CreateProfileToken", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating profile token")
            raise NetworkError("CreateProfileToken", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating profile token: %s", e)
            raise NetworkError("CreateProfileToken", e) from e

    def _parse_profile_tokens_page(
        self, response: httpx.Response
    ) -> tuple[list[dict[str, Any]], int]:
        """Parse one /profile/tokens page and return tokens plus total pages."""
        data_raw: Any = response.json()
        if not isinstance(data_raw, dict):
            msg = "profile tokens response must be an object"
            raise TypeError(msg)
        data = cast("dict[str, Any]", data_raw)
        pages_raw = data.get("pages", 1)
        if not isinstance(pages_raw, int) or isinstance(pages_raw, bool):
            msg = "profile tokens response pages must be an integer"
            raise TypeError(msg)
        if pages_raw < 1:
            msg = "profile tokens response pages must be positive"
            raise ValueError(msg)
        items_raw = data.get("data")
        if not isinstance(items_raw, list):
            msg = "profile tokens response data must be a list"
            raise TypeError(msg)
        items = cast("list[object]", items_raw)
        tokens: list[dict[str, Any]] = []
        for item in items:
            if not isinstance(item, dict):
                msg = "profile tokens response data entries must be objects"
                raise TypeError(msg)
            tokens.append(cast("dict[str, Any]", item))
        return tokens, pages_raw

    def _parse_profile_logins_page(
        self, response: httpx.Response
    ) -> tuple[list[dict[str, Any]], int]:
        """Parse a paginated profile logins response."""
        data_raw = response.json()
        if not isinstance(data_raw, dict):
            msg = "profile logins response must be an object"
            raise TypeError(msg)
        data = cast("dict[str, Any]", data_raw)
        pages_raw = data.get("pages", 1)
        if not isinstance(pages_raw, int) or isinstance(pages_raw, bool):
            msg = "profile logins response pages must be an integer"
            raise TypeError(msg)
        if pages_raw < 1:
            msg = "profile logins response pages must be positive"
            raise ValueError(msg)
        items_raw = data.get("data")
        if not isinstance(items_raw, list):
            msg = "profile logins response data must be a list"
            raise TypeError(msg)
        items = cast("list[object]", items_raw)
        logins: list[dict[str, Any]] = []
        for item in items:
            if not isinstance(item, dict):
                msg = "profile logins response data entries must be objects"
                raise TypeError(msg)
            logins.append(cast("dict[str, Any]", item))
        return logins, pages_raw

    async def list_profile_logins(self) -> list[dict[str, Any]]:
        """List profile logins."""
        logger.info("Listing profile logins")

        try:
            logins: list[dict[str, Any]] = []
            page = 1
            pages = 1
            while page <= pages:
                endpoint = "/profile/logins"
                if page > 1:
                    endpoint = f"/profile/logins?page={page}"
                response = await self.make_request("GET", endpoint)
                page_logins, pages = self._parse_profile_logins_page(response)
                logins.extend(page_logins)
                page += 1
            logger.info("Profile logins listed", extra={"login_count": len(logins)})
            return logins
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout listing profile logins: %s", e)
            raise NetworkError("ListProfileLogins", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing profile logins: %s", e)
            raise NetworkError("ListProfileLogins", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing profile logins")
            raise NetworkError("ListProfileLogins", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing profile logins: %s", e)
            raise NetworkError("ListProfileLogins", e) from e

    def _parse_profile_devices_page(
        self, response: httpx.Response
    ) -> tuple[list[dict[str, Any]], int]:
        """Parse a paginated profile trusted devices response."""
        data_raw = response.json()
        if not isinstance(data_raw, dict):
            msg = "profile devices response must be an object"
            raise TypeError(msg)
        data = cast("dict[str, Any]", data_raw)
        pages_raw = data.get("pages", 1)
        if not isinstance(pages_raw, int) or isinstance(pages_raw, bool):
            msg = "profile devices response pages must be an integer"
            raise TypeError(msg)
        if pages_raw < 1:
            msg = "profile devices response pages must be positive"
            raise ValueError(msg)
        items_raw = data.get("data")
        if not isinstance(items_raw, list):
            msg = "profile devices response data must be a list"
            raise TypeError(msg)
        items = cast("list[object]", items_raw)
        devices: list[dict[str, Any]] = []
        for item in items:
            if not isinstance(item, dict):
                msg = "profile devices response data entries must be objects"
                raise TypeError(msg)
            devices.append(cast("dict[str, Any]", item))
        return devices, pages_raw

    async def list_profile_devices(self) -> list[dict[str, Any]]:
        """List trusted profile devices."""
        logger.info("Listing profile trusted devices")

        try:
            devices: list[dict[str, Any]] = []
            page = 1
            pages = 1
            while page <= pages:
                endpoint = "/profile/devices"
                if page > 1:
                    endpoint = f"/profile/devices?page={page}"
                response = await self.make_request("GET", endpoint)
                page_devices, pages = self._parse_profile_devices_page(response)
                devices.extend(page_devices)
                page += 1
            logger.info(
                "Profile trusted devices listed",
                extra={"device_count": len(devices)},
            )
            return devices
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout listing profile trusted devices: %s", e
            )
            raise NetworkError("ListProfileDevices", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing profile trusted devices: %s", e)
            raise NetworkError("ListProfileDevices", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing profile trusted devices")
            raise NetworkError("ListProfileDevices", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing profile trusted devices: %s", e)
            raise NetworkError("ListProfileDevices", e) from e

    async def list_profile_tokens(self) -> list[dict[str, Any]]:
        """List personal access tokens."""
        logger.info("Listing profile tokens")

        try:
            tokens: list[dict[str, Any]] = []
            page = 1
            pages = 1
            while page <= pages:
                endpoint = "/profile/tokens"
                if page > 1:
                    endpoint = f"/profile/tokens?page={page}"
                response = await self.make_request("GET", endpoint)
                page_tokens, pages = self._parse_profile_tokens_page(response)
                tokens.extend(page_tokens)
                page += 1
            logger.info("Profile tokens listed", extra={"token_count": len(tokens)})
            return tokens
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout listing profile tokens: %s", e)
            raise NetworkError("ListProfileTokens", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing profile tokens: %s", e)
            raise NetworkError("ListProfileTokens", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing profile tokens")
            raise NetworkError("ListProfileTokens", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing profile tokens: %s", e)
            raise NetworkError("ListProfileTokens", e) from e

    async def get_profile_token(self, token_id: int) -> dict[str, Any]:
        """Get a personal access token."""
        encoded_token_id = quote(str(token_id), safe="")
        endpoint = f"/profile/tokens/{encoded_token_id}"
        logger.info("Getting profile token", extra={"token_id": token_id})

        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            logger.info("Profile token retrieved", extra={"token_id": token_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting profile token: %s", e)
            raise NetworkError("GetProfileToken", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting profile token: %s", e)
            raise NetworkError("GetProfileToken", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting profile token")
            raise NetworkError("GetProfileToken", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting profile token: %s", e)
            raise NetworkError("GetProfileToken", e) from e

    async def get_profile_login(self, login_id: int) -> dict[str, Any]:
        """Get a profile login."""
        encoded_login_id = quote(str(login_id), safe="")
        endpoint = f"/profile/logins/{encoded_login_id}"
        logger.info("Getting profile login", extra={"login_id": login_id})

        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            logger.info("Profile login retrieved", extra={"login_id": login_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting profile login: %s", e)
            raise NetworkError("GetProfileLogin", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting profile login: %s", e)
            raise NetworkError("GetProfileLogin", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting profile login")
            raise NetworkError("GetProfileLogin", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting profile login: %s", e)
            raise NetworkError("GetProfileLogin", e) from e

    async def get_profile_device(self, device_id: int) -> dict[str, Any]:
        """Get a trusted profile device."""
        encoded_device_id = quote(str(device_id), safe="")
        endpoint = f"/profile/devices/{encoded_device_id}"
        logger.info("Getting profile trusted device", extra={"device_id": device_id})

        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            logger.info(
                "Profile trusted device retrieved", extra={"device_id": device_id}
            )
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting profile trusted device: %s", e)
            raise NetworkError("GetProfileDevice", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting profile trusted device: %s", e)
            raise NetworkError("GetProfileDevice", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting profile trusted device")
            raise NetworkError("GetProfileDevice", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting profile trusted device: %s", e)
            raise NetworkError("GetProfileDevice", e) from e

    async def list_profile_apps(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List OAuth app authorizations from the profile."""
        endpoint = "/profile/apps"
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        if params:
            endpoint += "?" + urlencode(params)

        logger.info("Listing profile app authorizations")

        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            logger.info("Profile app authorizations listed")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout listing profile app authorizations: %s", e
            )
            raise NetworkError("ListProfileApps", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing profile app authorizations: %s", e)
            raise NetworkError("ListProfileApps", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing profile app authorizations")
            raise NetworkError("ListProfileApps", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing profile app authorizations: %s", e)
            raise NetworkError("ListProfileApps", e) from e

    async def get_profile_app(self, app_id: int) -> dict[str, Any]:
        """Get an OAuth app authorization from the profile."""
        encoded_app_id = quote(str(app_id), safe="")
        endpoint = f"/profile/apps/{encoded_app_id}"
        logger.info("Getting profile app authorization", extra={"app_id": app_id})

        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            logger.info("Profile app authorization retrieved", extra={"app_id": app_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout getting profile app authorization: %s", e
            )
            raise NetworkError("GetProfileApp", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting profile app authorization: %s", e)
            raise NetworkError("GetProfileApp", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting profile app authorization")
            raise NetworkError("GetProfileApp", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting profile app authorization: %s", e)
            raise NetworkError("GetProfileApp", e) from e

    async def delete_profile_app(self, app_id: int) -> None:
        """Revoke OAuth app access from the profile."""
        encoded_app_id = quote(str(app_id), safe="")
        endpoint = f"/profile/apps/{encoded_app_id}"
        logger.info("Revoking profile app access", extra={"app_id": app_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Profile app access revoked", extra={"app_id": app_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout revoking profile app access: %s", e)
            raise NetworkError("DeleteProfileApp", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout revoking profile app access: %s", e)
            raise NetworkError("DeleteProfileApp", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error revoking profile app access")
            raise NetworkError("DeleteProfileApp", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error revoking profile app access: %s", e)
            raise NetworkError("DeleteProfileApp", e) from e

    async def delete_profile_device(self, device_id: int) -> None:
        """Revoke a trusted profile device."""
        encoded_device_id = quote(str(device_id), safe="")
        endpoint = f"/profile/devices/{encoded_device_id}"
        logger.info("Revoking profile trusted device", extra={"device_id": device_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info(
                "Profile trusted device revoked", extra={"device_id": device_id}
            )
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout revoking profile trusted device: %s", e
            )
            raise NetworkError("DeleteProfileDevice", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout revoking profile trusted device: %s", e)
            raise NetworkError("DeleteProfileDevice", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error revoking profile trusted device")
            raise NetworkError("DeleteProfileDevice", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error revoking profile trusted device: %s", e)
            raise NetworkError("DeleteProfileDevice", e) from e

    async def delete_profile_token(self, token_id: int) -> None:
        """Revoke a personal access token."""
        encoded_token_id = quote(str(token_id), safe="")
        endpoint = f"/profile/tokens/{encoded_token_id}"
        logger.info("Revoking profile token", extra={"token_id": token_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Profile token revoked", extra={"token_id": token_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout revoking profile token: %s", e)
            raise NetworkError("DeleteProfileToken", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout revoking profile token: %s", e)
            raise NetworkError("DeleteProfileToken", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error revoking profile token")
            raise NetworkError("DeleteProfileToken", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error revoking profile token: %s", e)
            raise NetworkError("DeleteProfileToken", e) from e

    async def update_profile_token(self, token_id: int, label: str) -> dict[str, Any]:
        """Update a personal access token."""
        encoded_token_id = quote(str(token_id), safe="")
        endpoint = f"/profile/tokens/{encoded_token_id}"
        body = {"label": label}
        logger.info("Updating profile token", extra={"token_id": token_id})

        try:
            response = await self.make_request("PUT", endpoint, body)
            result: dict[str, Any] = response.json()
            logger.info("Profile token updated", extra={"token_id": token_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating profile token: %s", e)
            raise NetworkError("UpdateProfileToken", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating profile token: %s", e)
            raise NetworkError("UpdateProfileToken", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating profile token")
            raise NetworkError("UpdateProfileToken", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating profile token: %s", e)
            raise NetworkError("UpdateProfileToken", e) from e

    async def list_monitor_services(self) -> dict[str, Any]:
        """List supported Linode Metrics service types."""
        logger.info("Listing monitor services")

        try:
            response = await self.make_request("GET", "/monitor/services")
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout listing monitor services: %s", e)
            raise NetworkError("ListMonitorServices", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor services: %s", e)
            raise NetworkError("ListMonitorServices", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor services")
            raise NetworkError("ListMonitorServices", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor services: %s", e)
            raise NetworkError("ListMonitorServices", e) from e

    async def list_monitor_dashboards(self) -> dict[str, Any]:
        """List Linode Metrics dashboards."""
        logger.info("Listing monitor dashboards")

        try:
            response = await self.make_request("GET", "/monitor/dashboards")
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout listing monitor dashboards: %s", e)
            raise NetworkError("ListMonitorDashboards", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor dashboards: %s", e)
            raise NetworkError("ListMonitorDashboards", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor dashboards")
            raise NetworkError("ListMonitorDashboards", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor dashboards: %s", e)
            raise NetworkError("ListMonitorDashboards", e) from e

    async def list_monitor_alert_definitions(self) -> dict[str, Any]:
        """List Linode Metrics alert definitions."""
        logger.info("Listing monitor alert definitions")

        try:
            response = await self.make_request("GET", "/monitor/alert-definitions")
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout listing monitor alert definitions: %s", e
            )
            raise NetworkError("ListMonitorAlertDefinitions", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor alert definitions: %s", e)
            raise NetworkError("ListMonitorAlertDefinitions", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor alert definitions")
            raise NetworkError("ListMonitorAlertDefinitions", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor alert definitions: %s", e)
            raise NetworkError("ListMonitorAlertDefinitions", e) from e

    async def list_monitor_alert_channels(self) -> dict[str, Any]:
        """List Linode Metrics alert channels."""
        logger.info("Listing monitor alert channels")

        try:
            response = await self.make_request("GET", "/monitor/alert-channels")
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout listing monitor alert channels: %s", e)
            raise NetworkError("ListMonitorAlertChannels", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor alert channels: %s", e)
            raise NetworkError("ListMonitorAlertChannels", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor alert channels")
            raise NetworkError("ListMonitorAlertChannels", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor alert channels: %s", e)
            raise NetworkError("ListMonitorAlertChannels", e) from e

    async def get_monitor_service(self, service_type: str) -> dict[str, Any]:
        """Get details for a supported Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)

        encoded = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded}"
        logger.info("Getting monitor service", extra={"service_type": service_type})

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor service retrieved", extra={"service_type": service_type}
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting monitor service: %s", e)
            raise NetworkError("GetMonitorService", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting monitor service: %s", e)
            raise NetworkError("GetMonitorService", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting monitor service")
            raise NetworkError("GetMonitorService", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting monitor service: %s", e)
            raise NetworkError("GetMonitorService", e) from e

    async def update_monitor_alert_definition(
        self,
        service_type: str,
        alert_id: int,
        **fields: Any,
    ) -> dict[str, Any]:
        """Update a monitor service alert definition."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)
        if type(alert_id) is not int:
            msg = "alert_id must be a valid integer"
            raise TypeError(msg)
        if alert_id <= 0:
            msg = "alert_id must be a positive integer"
            raise ValueError(msg)
        encoded_service_type = quote(service_type, safe="")
        encoded_alert_id = quote(str(alert_id), safe="")
        endpoint = (
            f"/monitor/services/{encoded_service_type}"
            f"/alert-definitions/{encoded_alert_id}"
        )
        body = {key: value for key, value in fields.items() if value is not None}
        if not body:
            msg = "at least one update field is required"
            raise ValueError(msg)
        logger.info(
            "Updating monitor alert definition",
            extra={"service_type": service_type, "alert_id": alert_id},
        )
        try:
            response = await self.make_request("PUT", endpoint, body)
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor alert definition updated",
                extra={"service_type": service_type, "alert_id": alert_id},
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating monitor alert: %s", e)
            raise NetworkError("UpdateMonitorAlertDefinition", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating monitor alert: %s", e)
            raise NetworkError("UpdateMonitorAlertDefinition", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating monitor alert")
            raise NetworkError("UpdateMonitorAlertDefinition", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating monitor alert: %s", e)
            raise NetworkError("UpdateMonitorAlertDefinition", e) from e

    async def create_monitor_service_token(
        self, service_type: str, entity_ids: list[int]
    ) -> dict[str, Any]:
        """Create a Linode Metrics token scoped to a service type and entities."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)
        if not entity_ids:
            msg = "entity_ids must be a non-empty list"
            raise ValueError(msg)

        # URL-encode the path segment so unexpected characters can't escape it.
        encoded = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded}/token"
        logger.info(
            "Creating monitor service token",
            extra={"service_type": service_type, "entity_count": len(entity_ids)},
        )

        try:
            body = {"entity_ids": entity_ids}
            response = await self.make_request("POST", endpoint, body)
            data: dict[str, Any] = response.json()
            # Log success without the secret token value.
            logger.info(
                "Monitor service token created",
                extra={
                    "service_type": service_type,
                    "expiry": data.get("expiry"),
                },
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating monitor token: %s", e)
            raise NetworkError("CreateMonitorServiceToken", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating monitor token: %s", e)
            raise NetworkError("CreateMonitorServiceToken", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating monitor token")
            raise NetworkError("CreateMonitorServiceToken", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating monitor token: %s", e)
            raise NetworkError("CreateMonitorServiceToken", e) from e

    async def list_monitor_service_dashboards(
        self, service_type: str
    ) -> dict[str, Any]:
        """List dashboards for a Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)

        encoded = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded}/dashboards"
        logger.info(
            "Listing monitor service dashboards",
            extra={"service_type": service_type},
        )

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout listing monitor dashboards: %s", e)
            raise NetworkError("ListMonitorServiceDashboards", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor dashboards: %s", e)
            raise NetworkError("ListMonitorServiceDashboards", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor dashboards")
            raise NetworkError("ListMonitorServiceDashboards", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor dashboards: %s", e)
            raise NetworkError("ListMonitorServiceDashboards", e) from e

    async def get_monitor_dashboard(self, dashboard_id: int) -> dict[str, Any]:
        """Get a Linode Metrics dashboard by ID."""
        if type(dashboard_id) is not int:
            msg = "dashboard_id must be a valid integer"
            raise TypeError(msg)
        if dashboard_id <= 0:
            msg = "dashboard_id must be a positive integer"
            raise ValueError(msg)

        encoded_dashboard_id = quote(str(dashboard_id), safe="")
        endpoint = f"/monitor/dashboards/{encoded_dashboard_id}"
        logger.info(
            "Getting monitor dashboard",
            extra={"dashboard_id": dashboard_id},
        )

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting monitor dashboard: %s", e)
            raise NetworkError("GetMonitorDashboard", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting monitor dashboard: %s", e)
            raise NetworkError("GetMonitorDashboard", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting monitor dashboard")
            raise NetworkError("GetMonitorDashboard", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting monitor dashboard: %s", e)
            raise NetworkError("GetMonitorDashboard", e) from e

    async def read_monitor_service_metrics(self, service_type: str) -> dict[str, Any]:
        """Read metrics for a Linode Metrics service entity type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)

        encoded = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded}/metrics"
        logger.info(
            "Reading monitor service metrics",
            extra={"service_type": service_type},
        )

        try:
            response = await self.make_request("POST", endpoint, {})
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor service metrics read",
                extra={"service_type": service_type},
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout reading monitor metrics: %s", e)
            raise NetworkError("ReadMonitorServiceMetrics", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout reading monitor metrics: %s", e)
            raise NetworkError("ReadMonitorServiceMetrics", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error reading monitor metrics")
            raise NetworkError("ReadMonitorServiceMetrics", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error reading monitor metrics: %s", e)
            raise NetworkError("ReadMonitorServiceMetrics", e) from e

    async def list_monitor_service_metric_definitions(
        self, service_type: str
    ) -> dict[str, Any]:
        """List metric definitions for a Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)

        encoded = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded}/metric-definitions"
        logger.info(
            "Listing monitor service metric definitions",
            extra={"service_type": service_type},
        )

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor service metric definitions listed",
                extra={"service_type": service_type},
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout listing monitor metric definitions: %s", e
            )
            raise NetworkError("ListMonitorServiceMetricDefinitions", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor metric definitions: %s", e)
            raise NetworkError("ListMonitorServiceMetricDefinitions", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor metric definitions")
            raise NetworkError("ListMonitorServiceMetricDefinitions", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor metric definitions: %s", e)
            raise NetworkError("ListMonitorServiceMetricDefinitions", e) from e

    async def list_monitor_service_alert_definitions(
        self, service_type: str
    ) -> dict[str, Any]:
        """List alert definitions for a Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)

        encoded_service_type = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded_service_type}/alert-definitions"
        logger.info(
            "Listing monitor service alert definitions",
            extra={"service_type": service_type},
        )

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor service alert definitions listed",
                extra={"service_type": service_type},
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout listing monitor alert definitions: %s", e
            )
            raise NetworkError("ListMonitorServiceAlertDefinitions", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout listing monitor alert definitions: %s", e)
            raise NetworkError("ListMonitorServiceAlertDefinitions", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error listing monitor alert definitions")
            raise NetworkError("ListMonitorServiceAlertDefinitions", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error listing monitor alert definitions: %s", e)
            raise NetworkError("ListMonitorServiceAlertDefinitions", e) from e

    async def create_monitor_service_alert_definition(
        self,
        service_type: str,
        *,
        label: str,
        severity: int,
        rule_criteria: dict[str, Any],
        trigger_conditions: dict[str, Any],
        channel_ids: list[int],
        description: str | None = None,
        entity_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Create an alert definition for a Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)
        body = _build_monitor_service_alert_definition_body(
            label=label,
            severity=severity,
            rule_criteria=rule_criteria,
            trigger_conditions=trigger_conditions,
            channel_ids=channel_ids,
            description=description,
            entity_ids=entity_ids,
        )

        encoded_service_type = quote(service_type, safe="")
        endpoint = f"/monitor/services/{encoded_service_type}/alert-definitions"

        logger.info(
            "Creating monitor service alert definition",
            extra={"service_type": service_type, "label": label},
        )

        try:
            response = await self.make_request("POST", endpoint, body)
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor service alert definition created",
                extra={"service_type": service_type, "label": label},
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout creating monitor alert definition: %s", e
            )
            raise NetworkError("CreateMonitorServiceAlertDefinition", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating monitor alert definition: %s", e)
            raise NetworkError("CreateMonitorServiceAlertDefinition", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating monitor alert definition")
            raise NetworkError("CreateMonitorServiceAlertDefinition", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating monitor alert definition: %s", e)
            raise NetworkError("CreateMonitorServiceAlertDefinition", e) from e

    async def get_monitor_service_alert_definition(
        self, service_type: str, alert_id: int
    ) -> dict[str, Any]:
        """Get an alert definition for a Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)
        if type(alert_id) is not int:
            msg = "alert_id must be a valid integer"
            raise TypeError(msg)
        if alert_id <= 0:
            msg = "alert_id must be a positive integer"
            raise ValueError(msg)

        encoded_service_type = quote(service_type, safe="")
        encoded_alert_id = quote(str(alert_id), safe="")
        endpoint = (
            f"/monitor/services/{encoded_service_type}"
            f"/alert-definitions/{encoded_alert_id}"
        )
        logger.info(
            "Getting monitor service alert definition",
            extra={"service_type": service_type, "alert_id": alert_id},
        )

        try:
            response = await self.make_request("GET", endpoint)
            data: dict[str, Any] = response.json()
            logger.info(
                "Monitor service alert definition retrieved",
                extra={"service_type": service_type, "alert_id": alert_id},
            )
            return data
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout getting monitor alert definition: %s", e
            )
            raise NetworkError("GetMonitorServiceAlertDefinition", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting monitor alert definition: %s", e)
            raise NetworkError("GetMonitorServiceAlertDefinition", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting monitor alert definition")
            raise NetworkError("GetMonitorServiceAlertDefinition", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting monitor alert definition: %s", e)
            raise NetworkError("GetMonitorServiceAlertDefinition", e) from e

    async def delete_monitor_service_alert_definition(
        self, service_type: str, alert_id: int
    ) -> None:
        """Delete an alert definition for a Linode Metrics service type."""
        if not service_type:
            msg = "service_type is required"
            raise ValueError(msg)
        if isinstance(alert_id, bool):
            msg = "alert_id must be a valid integer"
            raise TypeError(msg)
        if alert_id <= 0:
            msg = "alert_id must be a positive integer"
            raise ValueError(msg)

        encoded_service_type = quote(service_type, safe="")
        encoded_alert_id = quote(str(alert_id), safe="")
        endpoint = (
            f"/monitor/services/{encoded_service_type}"
            f"/alert-definitions/{encoded_alert_id}"
        )
        logger.info(
            "Deleting monitor service alert definition",
            extra={"service_type": service_type, "alert_id": alert_id},
        )

        try:
            await self.make_request("DELETE", endpoint)
            logger.info(
                "Monitor service alert definition deleted",
                extra={"service_type": service_type, "alert_id": alert_id},
            )
        except httpx.ConnectTimeout as e:
            logger.exception(
                "Connection timeout deleting monitor alert definition: %s", e
            )
            raise NetworkError("DeleteMonitorServiceAlertDefinition", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting monitor alert definition: %s", e)
            raise NetworkError("DeleteMonitorServiceAlertDefinition", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting monitor alert definition")
            raise NetworkError("DeleteMonitorServiceAlertDefinition", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting monitor alert definition: %s", e)
            raise NetworkError("DeleteMonitorServiceAlertDefinition", e) from e

    async def boot_instance(
        self, instance_id: int, config_id: int | None = None
    ) -> None:
        """Boot an instance."""
        endpoint = f"/linode/instances/{instance_id}/boot"
        logger.info("Booting instance", extra={"instance_id": instance_id})

        try:
            body: dict[str, Any] = {}
            if config_id is not None:
                body["config_id"] = config_id
            await self.make_request("POST", endpoint, body or None)
            logger.info("Instance booted", extra={"instance_id": instance_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout booting instance: %s", e)
            raise NetworkError("BootInstance", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout booting instance: %s", e)
            raise NetworkError("BootInstance", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error booting instance")
            raise NetworkError("BootInstance", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error booting instance: %s", e)
            raise NetworkError("BootInstance", e) from e

    async def reboot_instance(
        self, instance_id: int, config_id: int | None = None
    ) -> None:
        """Reboot an instance."""
        endpoint = f"/linode/instances/{instance_id}/reboot"
        logger.info("Rebooting instance", extra={"instance_id": instance_id})

        try:
            body: dict[str, Any] = {}
            if config_id is not None:
                body["config_id"] = config_id
            await self.make_request("POST", endpoint, body or None)
            logger.info("Instance rebooted", extra={"instance_id": instance_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout rebooting instance: %s", e)
            raise NetworkError("RebootInstance", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout rebooting instance: %s", e)
            raise NetworkError("RebootInstance", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error rebooting instance")
            raise NetworkError("RebootInstance", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error rebooting instance: %s", e)
            raise NetworkError("RebootInstance", e) from e

    async def shutdown_instance(self, instance_id: int) -> None:
        """Shutdown an instance."""
        endpoint = f"/linode/instances/{instance_id}/shutdown"
        logger.info("Shutting down instance", extra={"instance_id": instance_id})

        try:
            await self.make_request("POST", endpoint)
            logger.info("Instance shut down", extra={"instance_id": instance_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout shutting down instance: %s", e)
            raise NetworkError("ShutdownInstance", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout shutting down instance: %s", e)
            raise NetworkError("ShutdownInstance", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error shutting down instance")
            raise NetworkError("ShutdownInstance", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error shutting down instance: %s", e)
            raise NetworkError("ShutdownInstance", e) from e

    async def create_instance(
        self,
        region: str,
        instance_type: str,
        firewall_id: int,
        image: str | None = None,
        label: str | None = None,
        root_pass: str | None = None,
        authorized_keys: list[str] | None = None,
        authorized_users: list[str] | None = None,
        booted: bool = True,
        backups_enabled: bool = False,
        route_ipv4: bool = True,
        route_ipv6: bool = True,
        tags: list[str] | None = None,
    ) -> Instance:
        """Create a new Linode instance under the current Linode Interfaces
        generation. firewall_id is required: the API rejects payloads without
        an interface-level firewall with "must have at least 1 interface
        defined to boot".
        """
        validate_label(label)
        validate_root_password(root_pass)

        logger.info(
            "Creating instance",
            extra={"region": region, "type": instance_type, "label": label},
        )

        try:
            body: dict[str, Any] = {
                "region": region,
                "type": instance_type,
                "booted": booted,
                "backups_enabled": backups_enabled,
                "interface_generation": CURRENT_INTERFACE_GENERATION,
                "interfaces": [
                    _build_public_interface_entry(firewall_id, route_ipv4, route_ipv6),
                ],
            }
            if image:
                body["image"] = image
            if label:
                body["label"] = label
            if root_pass:
                body["root_pass"] = root_pass
            if authorized_keys:
                body["authorized_keys"] = authorized_keys
            if authorized_users:
                body["authorized_users"] = authorized_users
            if tags:
                body["tags"] = tags

            response = await self.make_request("POST", "/linode/instances", body)
            data = response.json()
            result = self._parse_instance(data)
            logger.info("Instance created", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating instance: %s", e)
            raise NetworkError("CreateInstance", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating instance: %s", e)
            raise NetworkError("CreateInstance", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating instance")
            raise NetworkError("CreateInstance", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating instance: %s", e)
            raise NetworkError("CreateInstance", e) from e

    async def delete_instance(self, instance_id: int) -> None:
        """Delete an instance."""
        endpoint = f"/linode/instances/{instance_id}"
        logger.info("Deleting instance", extra={"instance_id": instance_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Instance deleted", extra={"instance_id": instance_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting instance: %s", e)
            raise NetworkError("DeleteInstance", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting instance: %s", e)
            raise NetworkError("DeleteInstance", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting instance")
            raise NetworkError("DeleteInstance", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting instance: %s", e)
            raise NetworkError("DeleteInstance", e) from e

    async def resize_instance(
        self,
        instance_id: int,
        instance_type: str,
        allow_auto_disk_resize: bool = True,
        migration_type: str = "warm",
    ) -> None:
        """Resize an instance."""
        endpoint = f"/linode/instances/{instance_id}/resize"
        logger.info(
            "Resizing instance",
            extra={"instance_id": instance_id, "new_type": instance_type},
        )

        try:
            body = {
                "type": instance_type,
                "allow_auto_disk_resize": allow_auto_disk_resize,
                "migration_type": migration_type,
            }
            await self.make_request("POST", endpoint, body)
            logger.info("Instance resized", extra={"instance_id": instance_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout resizing instance: %s", e)
            raise NetworkError("ResizeInstance", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout resizing instance: %s", e)
            raise NetworkError("ResizeInstance", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error resizing instance")
            raise NetworkError("ResizeInstance", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error resizing instance: %s", e)
            raise NetworkError("ResizeInstance", e) from e

    async def create_firewall(
        self,
        label: str,
        inbound_policy: str = "ACCEPT",
        outbound_policy: str = "ACCEPT",
    ) -> Firewall:
        """Create a new firewall."""
        validate_label(label)
        validate_firewall_policy(inbound_policy)
        validate_firewall_policy(outbound_policy)

        logger.info("Creating firewall", extra={"label": label})

        try:
            body = {
                "label": label,
                "rules": {
                    "inbound_policy": inbound_policy,
                    "outbound_policy": outbound_policy,
                },
            }
            response = await self.make_request("POST", "/networking/firewalls", body)
            data = response.json()
            result = self._parse_firewall(data)
            logger.info("Firewall created", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating firewall: %s", e)
            raise NetworkError("CreateFirewall", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating firewall: %s", e)
            raise NetworkError("CreateFirewall", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating firewall")
            raise NetworkError("CreateFirewall", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating firewall: %s", e)
            raise NetworkError("CreateFirewall", e) from e

    async def update_firewall(
        self,
        firewall_id: int,
        label: str | None = None,
        status: str | None = None,
        inbound_policy: str | None = None,
        outbound_policy: str | None = None,
    ) -> Firewall:
        """Update a firewall."""
        endpoint = f"/networking/firewalls/{firewall_id}"
        if inbound_policy:
            validate_firewall_policy(inbound_policy)
        if outbound_policy:
            validate_firewall_policy(outbound_policy)

        logger.info("Updating firewall", extra={"firewall_id": firewall_id})

        try:
            body: dict[str, Any] = {}
            if label:
                body["label"] = label
            if status:
                body["status"] = status
            if inbound_policy or outbound_policy:
                body["rules"] = {}
                if inbound_policy:
                    body["rules"]["inbound_policy"] = inbound_policy
                if outbound_policy:
                    body["rules"]["outbound_policy"] = outbound_policy

            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = self._parse_firewall(data)
            logger.info("Firewall updated", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating firewall: %s", e)
            raise NetworkError("UpdateFirewall", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating firewall: %s", e)
            raise NetworkError("UpdateFirewall", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating firewall")
            raise NetworkError("UpdateFirewall", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating firewall: %s", e)
            raise NetworkError("UpdateFirewall", e) from e

    async def delete_firewall(self, firewall_id: int) -> None:
        """Delete a firewall."""
        endpoint = f"/networking/firewalls/{firewall_id}"
        logger.info("Deleting firewall", extra={"firewall_id": firewall_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Firewall deleted", extra={"firewall_id": firewall_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting firewall: %s", e)
            raise NetworkError("DeleteFirewall", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting firewall: %s", e)
            raise NetworkError("DeleteFirewall", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting firewall")
            raise NetworkError("DeleteFirewall", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting firewall: %s", e)
            raise NetworkError("DeleteFirewall", e) from e

    async def create_firewall_device(
        self,
        firewall_id: int | str,
        device_id: int,
        device_type: str,
    ) -> dict[str, Any]:
        """Create a new device for a firewall."""
        logger.info("Creating firewall device", extra={"firewall_id": firewall_id})

        if isinstance(firewall_id, int) and firewall_id <= 0:
            raise ValueError("firewall_id must be a positive integer")
        if not str(firewall_id).strip():
            raise ValueError("firewall_id must be a positive integer")
        if type(device_id) is not int or device_id <= 0:
            raise ValueError("id must be a positive integer")
        if type(device_type) is not str or not device_type.strip():
            raise ValueError("type must be a non-empty string")

        safe_firewall_id = quote(str(firewall_id), safe="")
        endpoint = f"/networking/firewalls/{safe_firewall_id}/devices"
        body = {"id": device_id, "type": device_type}

        try:
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            logger.info(
                "Firewall device created",
                extra={"firewall_id": firewall_id, "device_id": result.get("id")},
            )
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating firewall device: %s", e)
            raise NetworkError("CreateFirewallDevice", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating firewall device: %s", e)
            raise NetworkError("CreateFirewallDevice", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating firewall device")
            raise NetworkError("CreateFirewallDevice", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating firewall device: %s", e)
            raise NetworkError("CreateFirewallDevice", e) from e

    async def update_firewall_rules(
        self,
        firewall_id: int,
        inbound: list[dict[str, Any]],
        outbound: list[dict[str, Any]],
    ) -> dict[str, list[dict[str, Any]]]:
        """Update firewall rules.

        Replaces the inbound and outbound rule sets for a firewall.
        """
        _validate_firewall_rules_update_request(firewall_id, inbound, outbound)

        endpoint = f"/networking/firewalls/{firewall_id}/rules"
        body: dict[str, Any] = {"inbound": inbound, "outbound": outbound}

        logger.info(
            "Updating firewall rules",
            extra={"firewall_id": firewall_id},
        )

        try:
            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = {
                "inbound": data.get("inbound", []),
                "outbound": data.get("outbound", []),
            }
            logger.info("Firewall rules updated", extra={"firewall_id": firewall_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating firewall rules: %s", e)
            raise NetworkError("UpdateFirewallRules", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating firewall rules: %s", e)
            raise NetworkError("UpdateFirewallRules", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating firewall rules")
            raise NetworkError("UpdateFirewallRules", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating firewall rules: %s", e)
            raise NetworkError("UpdateFirewallRules", e) from e

    async def create_domain(
        self,
        domain: str,
        domain_type: str = "master",
        soa_email: str | None = None,
        description: str | None = None,
        tags: list[str] | None = None,
    ) -> Domain:
        """Create a new domain."""
        validate_label(domain)

        logger.info("Creating domain", extra={"domain": domain})

        try:
            body: dict[str, Any] = {"domain": domain, "type": domain_type}
            if soa_email:
                body["soa_email"] = soa_email
            if description:
                body["description"] = description
            if tags:
                body["tags"] = tags

            response = await self.make_request("POST", "/domains", body)
            data = response.json()
            result = self._parse_domain(data)
            logger.info("Domain created", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating domain: %s", e)
            raise NetworkError("CreateDomain", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating domain: %s", e)
            raise NetworkError("CreateDomain", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating domain")
            raise NetworkError("CreateDomain", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating domain: %s", e)
            raise NetworkError("CreateDomain", e) from e

    async def clone_domain(self, domain_id: int | str, domain: str) -> Domain:
        """Clone a domain."""
        if not domain or domain != domain.strip():
            msg = "domain is required"
            raise ValueError(msg)
        validate_label(domain)
        encoded_domain_id = quote(str(domain_id), safe="")
        endpoint = f"/domains/{encoded_domain_id}/clone"
        logger.info("Cloning domain", extra={"domain_id": domain_id})

        try:
            response = await self.make_request("POST", endpoint, {"domain": domain})
            data = response.json()
            result = self._parse_domain(data)
            logger.info("Domain cloned", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout cloning domain: %s", e)
            raise NetworkError("CloneDomain", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout cloning domain: %s", e)
            raise NetworkError("CloneDomain", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error cloning domain")
            raise NetworkError("CloneDomain", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error cloning domain: %s", e)
            raise NetworkError("CloneDomain", e) from e

    async def import_domain(self, domain: str, remote_nameserver: str) -> Domain:
        """Import a domain from a remote nameserver."""
        if not domain or domain != domain.strip():
            msg = "domain is required"
            raise ValueError(msg)
        validate_label(domain)
        if not remote_nameserver or remote_nameserver != remote_nameserver.strip():
            msg = "remote_nameserver is required"
            raise ValueError(msg)

        logger.info("Importing domain", extra={"domain": domain})

        try:
            body: dict[str, Any] = {
                "domain": domain,
                "remote_nameserver": remote_nameserver,
            }
            response = await self.make_request("POST", "/domains/import", body)
            data = response.json()
            result = self._parse_domain(data)
            logger.info("Domain imported", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout importing domain: %s", e)
            raise NetworkError("ImportDomain", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout importing domain: %s", e)
            raise NetworkError("ImportDomain", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error importing domain")
            raise NetworkError("ImportDomain", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error importing domain: %s", e)
            raise NetworkError("ImportDomain", e) from e

    async def update_domain(
        self,
        domain_id: int,
        domain: str | None = None,
        soa_email: str | None = None,
        description: str | None = None,
        tags: list[str] | None = None,
    ) -> Domain:
        """Update a domain."""
        endpoint = f"/domains/{domain_id}"
        logger.info("Updating domain", extra={"domain_id": domain_id})

        try:
            body: dict[str, Any] = {}
            if domain:
                body["domain"] = domain
            if soa_email:
                body["soa_email"] = soa_email
            if description is not None:
                body["description"] = description
            if tags is not None:
                body["tags"] = tags

            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = self._parse_domain(data)
            logger.info("Domain updated", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating domain: %s", e)
            raise NetworkError("UpdateDomain", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating domain: %s", e)
            raise NetworkError("UpdateDomain", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating domain")
            raise NetworkError("UpdateDomain", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating domain: %s", e)
            raise NetworkError("UpdateDomain", e) from e

    async def delete_domain(self, domain_id: int) -> None:
        """Delete a domain."""
        endpoint = f"/domains/{domain_id}"
        logger.info("Deleting domain", extra={"domain_id": domain_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Domain deleted", extra={"domain_id": domain_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting domain: %s", e)
            raise NetworkError("DeleteDomain", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting domain: %s", e)
            raise NetworkError("DeleteDomain", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting domain")
            raise NetworkError("DeleteDomain", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting domain: %s", e)
            raise NetworkError("DeleteDomain", e) from e

    async def create_domain_record(
        self,
        domain_id: int,
        record_type: str,
        name: str | None = None,
        target: str | None = None,
        priority: int | None = None,
        weight: int | None = None,
        port: int | None = None,
        ttl_sec: int | None = None,
    ) -> DomainRecord:
        """Create a new domain record."""
        endpoint = f"/domains/{domain_id}/records"
        if name:
            validate_dns_record_name(name)
        if target:
            validate_dns_record_target(record_type, target)

        logger.info(
            "Creating domain record",
            extra={"domain_id": domain_id, "type": record_type, "record_name": name},
        )

        try:
            body: dict[str, Any] = {"type": record_type}
            optional_fields: dict[str, Any] = {
                "name": name,
                "target": target,
                "priority": priority,
                "weight": weight,
                "port": port,
                "ttl_sec": ttl_sec,
            }
            body.update({k: v for k, v in optional_fields.items() if v is not None})

            response = await self.make_request("POST", endpoint, body)
            data = response.json()
            result = self._parse_domain_record(data)
            logger.info("Domain record created", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating domain record: %s", e)
            raise NetworkError("CreateDomainRecord", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating domain record: %s", e)
            raise NetworkError("CreateDomainRecord", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error creating domain record: status %d", e.response.status_code
            )
            raise NetworkError("CreateDomainRecord", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating domain record: %s", e)
            raise NetworkError("CreateDomainRecord", e) from e

    async def update_domain_record(
        self,
        domain_id: int,
        record_id: int,
        name: str | None = None,
        target: str | None = None,
        priority: int | None = None,
        weight: int | None = None,
        port: int | None = None,
        ttl_sec: int | None = None,
    ) -> DomainRecord:
        """Update a domain record."""
        endpoint = f"/domains/{domain_id}/records/{record_id}"
        if name:
            validate_dns_record_name(name)

        logger.info(
            "Updating domain record",
            extra={"domain_id": domain_id, "record_id": record_id},
        )

        try:
            body: dict[str, Any] = {}
            if name is not None:
                body["name"] = name
            if target is not None:
                body["target"] = target
            if priority is not None:
                body["priority"] = priority
            if weight is not None:
                body["weight"] = weight
            if port is not None:
                body["port"] = port
            if ttl_sec is not None:
                body["ttl_sec"] = ttl_sec

            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = self._parse_domain_record(data)
            logger.info("Domain record updated", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating domain record: %s", e)
            raise NetworkError("UpdateDomainRecord", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating domain record: %s", e)
            raise NetworkError("UpdateDomainRecord", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error updating domain record: status %d", e.response.status_code
            )
            raise NetworkError("UpdateDomainRecord", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating domain record: %s", e)
            raise NetworkError("UpdateDomainRecord", e) from e

    async def delete_domain_record(self, domain_id: int, record_id: int) -> None:
        """Delete a domain record."""
        endpoint = f"/domains/{domain_id}/records/{record_id}"
        logger.info(
            "Deleting domain record",
            extra={"domain_id": domain_id, "record_id": record_id},
        )

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Domain record deleted", extra={"record_id": record_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting domain record: %s", e)
            raise NetworkError("DeleteDomainRecord", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting domain record: %s", e)
            raise NetworkError("DeleteDomainRecord", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error deleting domain record: status %d", e.response.status_code
            )
            raise NetworkError("DeleteDomainRecord", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting domain record: %s", e)
            raise NetworkError("DeleteDomainRecord", e) from e

    async def create_volume(
        self,
        label: str,
        region: str | None = None,
        linode_id: int | None = None,
        size: int = 20,
        tags: list[str] | None = None,
    ) -> Volume:
        """Create a new volume."""
        validate_label(label)
        validate_volume_size(size)

        logger.info("Creating volume", extra={"label": label, "size": size})

        try:
            body: dict[str, Any] = {"label": label, "size": size}
            if region:
                body["region"] = region
            if linode_id is not None:
                body["linode_id"] = linode_id
            if tags:
                body["tags"] = tags

            response = await self.make_request("POST", "/volumes", body)
            data = response.json()
            result = self._parse_volume(data)
            logger.info("Volume created", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating volume: %s", e)
            raise NetworkError("CreateVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating volume: %s", e)
            raise NetworkError("CreateVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error creating volume")
            raise NetworkError("CreateVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating volume: %s", e)
            raise NetworkError("CreateVolume", e) from e

    async def clone_volume(self, volume_id: int, label: str) -> Volume:
        """Clone a volume."""
        endpoint = f"/volumes/{volume_id}/clone"
        validate_label(label)

        logger.info("Cloning volume", extra={"volume_id": volume_id, "label": label})

        try:
            response = await self.make_request("POST", endpoint, {"label": label})
            data = response.json()
            result = self._parse_volume(data)
            logger.info(
                "Volume cloned",
                extra={"source_volume_id": volume_id, "cloned_volume_id": result.id},
            )
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout cloning volume: %s", e)
            raise NetworkError("CloneVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout cloning volume: %s", e)
            raise NetworkError("CloneVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error cloning volume")
            raise NetworkError("CloneVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error cloning volume: %s", e)
            raise NetworkError("CloneVolume", e) from e

    async def attach_volume(
        self,
        volume_id: int,
        linode_id: int,
        config_id: int | None = None,
        persist_across_boots: bool = False,
    ) -> Volume:
        """Attach a volume to an instance."""
        endpoint = f"/volumes/{volume_id}/attach"
        logger.info(
            "Attaching volume",
            extra={"volume_id": volume_id, "linode_id": linode_id},
        )

        try:
            body: dict[str, Any] = {
                "linode_id": linode_id,
                "persist_across_boots": persist_across_boots,
            }
            if config_id is not None:
                body["config_id"] = config_id

            response = await self.make_request("POST", endpoint, body)
            data = response.json()
            result = self._parse_volume(data)
            logger.info("Volume attached", extra={"volume_id": volume_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout attaching volume: %s", e)
            raise NetworkError("AttachVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout attaching volume: %s", e)
            raise NetworkError("AttachVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error attaching volume")
            raise NetworkError("AttachVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error attaching volume: %s", e)
            raise NetworkError("AttachVolume", e) from e

    async def detach_volume(self, volume_id: int) -> None:
        """Detach a volume from an instance."""
        endpoint = f"/volumes/{volume_id}/detach"
        logger.info("Detaching volume", extra={"volume_id": volume_id})

        try:
            await self.make_request("POST", endpoint)
            logger.info("Volume detached", extra={"volume_id": volume_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout detaching volume: %s", e)
            raise NetworkError("DetachVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout detaching volume: %s", e)
            raise NetworkError("DetachVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error detaching volume")
            raise NetworkError("DetachVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error detaching volume: %s", e)
            raise NetworkError("DetachVolume", e) from e

    async def resize_volume(self, volume_id: int, size: int) -> Volume:
        """Resize a volume."""
        endpoint = f"/volumes/{volume_id}/resize"
        validate_volume_size(size)

        logger.info("Resizing volume", extra={"volume_id": volume_id, "new_size": size})

        try:
            body = {"size": size}
            response = await self.make_request("POST", endpoint, body)
            data = response.json()
            result = self._parse_volume(data)
            logger.info("Volume resized", extra={"volume_id": volume_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout resizing volume: %s", e)
            raise NetworkError("ResizeVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout resizing volume: %s", e)
            raise NetworkError("ResizeVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error resizing volume")
            raise NetworkError("ResizeVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error resizing volume: %s", e)
            raise NetworkError("ResizeVolume", e) from e

    async def update_volume(
        self,
        volume_id: int,
        label: str | None = None,
        tags: list[str] | None = None,
    ) -> Volume:
        """Update a volume."""
        endpoint = f"/volumes/{volume_id}"

        if label is not None:
            validate_label(label)

        logger.info("Updating volume", extra={"volume_id": volume_id})

        try:
            body: dict[str, Any] = {}
            if label is not None:
                body["label"] = label
            if tags is not None:
                body["tags"] = tags

            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = self._parse_volume(data)
            logger.info("Volume updated", extra={"volume_id": volume_id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating volume: %s", e)
            raise NetworkError("UpdateVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating volume: %s", e)
            raise NetworkError("UpdateVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error updating volume")
            raise NetworkError("UpdateVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating volume: %s", e)
            raise NetworkError("UpdateVolume", e) from e

    async def delete_volume(self, volume_id: int) -> None:
        """Delete a volume."""
        endpoint = f"/volumes/{volume_id}"
        logger.info("Deleting volume", extra={"volume_id": volume_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("Volume deleted", extra={"volume_id": volume_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting volume: %s", e)
            raise NetworkError("DeleteVolume", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting volume: %s", e)
            raise NetworkError("DeleteVolume", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error deleting volume")
            raise NetworkError("DeleteVolume", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting volume: %s", e)
            raise NetworkError("DeleteVolume", e) from e

    async def create_nodebalancer(
        self,
        region: str,
        label: str | None = None,
        client_conn_throttle: int = 0,
        tags: list[str] | None = None,
    ) -> NodeBalancer:
        """Create a new NodeBalancer."""
        validate_label(label)

        logger.info("Creating NodeBalancer", extra={"region": region, "label": label})

        try:
            body: dict[str, Any] = {
                "region": region,
                "client_conn_throttle": client_conn_throttle,
            }
            if label:
                body["label"] = label
            if tags:
                body["tags"] = tags

            response = await self.make_request("POST", "/nodebalancers", body)
            data = response.json()
            result = self._parse_nodebalancer(data)
            logger.info(
                "NodeBalancer created", extra={"id": result.id, "label": result.label}
            )
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout creating NodeBalancer: %s", e)
            raise NetworkError("CreateNodeBalancer", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout creating NodeBalancer: %s", e)
            raise NetworkError("CreateNodeBalancer", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error creating NodeBalancer: status %d", e.response.status_code
            )
            raise NetworkError("CreateNodeBalancer", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error creating NodeBalancer: %s", e)
            raise NetworkError("CreateNodeBalancer", e) from e

    async def update_nodebalancer(
        self,
        nodebalancer_id: int,
        label: str | None = None,
        client_conn_throttle: int | None = None,
        tags: list[str] | None = None,
    ) -> NodeBalancer:
        """Update a NodeBalancer."""
        endpoint = f"/nodebalancers/{nodebalancer_id}"
        logger.info("Updating NodeBalancer", extra={"nodebalancer_id": nodebalancer_id})

        try:
            body: dict[str, Any] = {}
            if label:
                body["label"] = label
            if client_conn_throttle is not None:
                body["client_conn_throttle"] = client_conn_throttle
            if tags is not None:
                body["tags"] = tags

            response = await self.make_request("PUT", endpoint, body)
            data = response.json()
            result = self._parse_nodebalancer(data)
            logger.info("NodeBalancer updated", extra={"id": result.id})
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout updating NodeBalancer: %s", e)
            raise NetworkError("UpdateNodeBalancer", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout updating NodeBalancer: %s", e)
            raise NetworkError("UpdateNodeBalancer", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error updating NodeBalancer: status %d", e.response.status_code
            )
            raise NetworkError("UpdateNodeBalancer", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error updating NodeBalancer: %s", e)
            raise NetworkError("UpdateNodeBalancer", e) from e

    async def delete_nodebalancer(self, nodebalancer_id: int) -> None:
        """Delete a NodeBalancer."""
        endpoint = f"/nodebalancers/{nodebalancer_id}"
        logger.info("Deleting NodeBalancer", extra={"nodebalancer_id": nodebalancer_id})

        try:
            await self.make_request("DELETE", endpoint)
            logger.info("NodeBalancer deleted", extra={"id": nodebalancer_id})
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout deleting NodeBalancer: %s", e)
            raise NetworkError("DeleteNodeBalancer", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout deleting NodeBalancer: %s", e)
            raise NetworkError("DeleteNodeBalancer", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception(
                "HTTP error deleting NodeBalancer: status %d", e.response.status_code
            )
            raise NetworkError("DeleteNodeBalancer", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error deleting NodeBalancer: %s", e)
            raise NetworkError("DeleteNodeBalancer", e) from e

    # LKE (Linode Kubernetes Engine) operations

    async def list_lke_clusters(self) -> list[dict[str, Any]]:
        """List LKE clusters."""
        try:
            response = await self.make_request("GET", "/lke/clusters")
            data = response.json()
            clusters: list[dict[str, Any]] = data.get("data", [])
            return clusters
        except httpx.HTTPError as e:
            raise NetworkError("ListLKEClusters", e) from e

    async def get_lke_cluster(self, cluster_id: int) -> dict[str, Any]:
        """Get a specific LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}"
        try:
            response = await self.make_request("GET", endpoint)
            cluster: dict[str, Any] = response.json()
            return cluster
        except httpx.HTTPError as e:
            raise NetworkError("GetLKECluster", e) from e

    async def create_lke_cluster(
        self,
        label: str,
        region: str,
        k8s_version: str,
        node_pools: list[dict[str, Any]],
        tags: list[str] | None = None,
        control_plane: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Create a new LKE cluster."""
        try:
            body: dict[str, Any] = {
                "label": label,
                "region": region,
                "k8s_version": k8s_version,
                "node_pools": node_pools,
            }
            if tags is not None:
                body["tags"] = tags
            if control_plane is not None:
                body["control_plane"] = control_plane
            response = await self.make_request("POST", "/lke/clusters", body)
            cluster: dict[str, Any] = response.json()
            return cluster
        except httpx.HTTPError as e:
            raise NetworkError("CreateLKECluster", e) from e

    async def update_lke_cluster(
        self,
        cluster_id: int,
        label: str | None = None,
        k8s_version: str | None = None,
        tags: list[str] | None = None,
        control_plane: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Update an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}"
        try:
            body: dict[str, Any] = {}
            if label is not None:
                body["label"] = label
            if k8s_version is not None:
                body["k8s_version"] = k8s_version
            if tags is not None:
                body["tags"] = tags
            if control_plane is not None:
                body["control_plane"] = control_plane
            response = await self.make_request("PUT", endpoint, body)
            cluster: dict[str, Any] = response.json()
            return cluster
        except httpx.HTTPError as e:
            raise NetworkError("UpdateLKECluster", e) from e

    async def delete_lke_cluster(self, cluster_id: int) -> None:
        """Delete an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteLKECluster", e) from e

    async def recycle_lke_cluster(self, cluster_id: int) -> None:
        """Recycle all nodes in an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/recycle"
        try:
            await self.make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("RecycleLKECluster", e) from e

    async def regenerate_lke_cluster(self, cluster_id: int) -> None:
        """Regenerate the service token for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/regenerate"
        try:
            await self.make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("RegenerateLKECluster", e) from e

    async def list_lke_node_pools(self, cluster_id: int) -> list[dict[str, Any]]:
        """List node pools for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/pools"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            pools: list[dict[str, Any]] = data.get("data", [])
            return pools
        except httpx.HTTPError as e:
            raise NetworkError("ListLKENodePools", e) from e

    async def get_lke_node_pool(self, cluster_id: int, pool_id: int) -> dict[str, Any]:
        """Get a specific node pool."""
        endpoint = f"/lke/clusters/{cluster_id}/pools/{pool_id}"
        try:
            response = await self.make_request("GET", endpoint)
            pool: dict[str, Any] = response.json()
            return pool
        except httpx.HTTPError as e:
            raise NetworkError("GetLKENodePool", e) from e

    async def create_lke_node_pool(
        self,
        cluster_id: int,
        node_type: str,
        count: int,
        autoscaler: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> dict[str, Any]:
        """Create a new node pool in an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/pools"
        try:
            body: dict[str, Any] = {
                "type": node_type,
                "count": count,
            }
            if autoscaler is not None:
                body["autoscaler"] = autoscaler
            if tags is not None:
                body["tags"] = tags
            response = await self.make_request("POST", endpoint, body)
            pool: dict[str, Any] = response.json()
            return pool
        except httpx.HTTPError as e:
            raise NetworkError("CreateLKENodePool", e) from e

    async def update_lke_node_pool(
        self,
        cluster_id: int,
        pool_id: int,
        count: int | None = None,
        autoscaler: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> dict[str, Any]:
        """Update a node pool in an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/pools/{pool_id}"
        try:
            body: dict[str, Any] = {}
            if count is not None:
                body["count"] = count
            if autoscaler is not None:
                body["autoscaler"] = autoscaler
            if tags is not None:
                body["tags"] = tags
            response = await self.make_request("PUT", endpoint, body)
            pool: dict[str, Any] = response.json()
            return pool
        except httpx.HTTPError as e:
            raise NetworkError("UpdateLKENodePool", e) from e

    async def delete_lke_node_pool(self, cluster_id: int, pool_id: int) -> None:
        """Delete a node pool from an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/pools/{pool_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteLKENodePool", e) from e

    async def recycle_lke_node_pool(self, cluster_id: int, pool_id: int) -> None:
        """Recycle all nodes in a node pool."""
        endpoint = f"/lke/clusters/{cluster_id}/pools/{pool_id}/recycle"
        try:
            await self.make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("RecycleLKENodePool", e) from e

    async def get_lke_node(self, cluster_id: int, node_id: str) -> dict[str, Any]:
        """Get a specific node in an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/nodes/{node_id}"
        try:
            response = await self.make_request("GET", endpoint)
            node: dict[str, Any] = response.json()
            return node
        except httpx.HTTPError as e:
            raise NetworkError("GetLKENode", e) from e

    async def delete_lke_node(self, cluster_id: int, node_id: str) -> None:
        """Delete a specific node from an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/nodes/{node_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteLKENode", e) from e

    async def recycle_lke_node(self, cluster_id: int, node_id: str) -> None:
        """Recycle a specific node in an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/nodes/{node_id}/recycle"
        try:
            await self.make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("RecycleLKENode", e) from e

    async def get_lke_kubeconfig(self, cluster_id: int) -> dict[str, Any]:
        """Get the kubeconfig for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/kubeconfig"
        try:
            response = await self.make_request("GET", endpoint)
            kubeconfig: dict[str, Any] = response.json()
            return kubeconfig
        except httpx.HTTPError as e:
            raise NetworkError("GetLKEKubeconfig", e) from e

    async def delete_lke_kubeconfig(self, cluster_id: int) -> None:
        """Delete/regenerate the kubeconfig for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/kubeconfig"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteLKEKubeconfig", e) from e

    async def get_lke_dashboard(self, cluster_id: int) -> dict[str, Any]:
        """Get the dashboard URL for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/dashboard"
        try:
            response = await self.make_request("GET", endpoint)
            dashboard: dict[str, Any] = response.json()
            return dashboard
        except httpx.HTTPError as e:
            raise NetworkError("GetLKEDashboard", e) from e

    async def list_lke_api_endpoints(self, cluster_id: int) -> list[dict[str, Any]]:
        """List API endpoints for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/api-endpoints"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            endpoints: list[dict[str, Any]] = data.get("data", [])
            return endpoints
        except httpx.HTTPError as e:
            raise NetworkError("ListLKEAPIEndpoints", e) from e

    async def delete_lke_service_token(self, cluster_id: int) -> None:
        """Delete the service token for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/servicetoken"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteLKEServiceToken", e) from e

    async def get_lke_control_plane_acl(self, cluster_id: int) -> dict[str, Any]:
        """Get the control plane ACL for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/control_plane_acl"
        try:
            response = await self.make_request("GET", endpoint)
            acl: dict[str, Any] = response.json()
            return acl
        except httpx.HTTPError as e:
            raise NetworkError("GetLKEControlPlaneACL", e) from e

    async def update_lke_control_plane_acl(
        self,
        cluster_id: int,
        acl: dict[str, Any],
    ) -> dict[str, Any]:
        """Update the control plane ACL for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/control_plane_acl"
        try:
            body: dict[str, Any] = {"acl": acl}
            response = await self.make_request("PUT", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("UpdateLKEControlPlaneACL", e) from e

    async def delete_lke_control_plane_acl(self, cluster_id: int) -> None:
        """Delete the control plane ACL for an LKE cluster."""
        endpoint = f"/lke/clusters/{cluster_id}/control_plane_acl"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteLKEControlPlaneACL", e) from e

    async def list_lke_versions(self) -> list[dict[str, Any]]:
        """List available LKE Kubernetes versions."""
        try:
            response = await self.make_request("GET", "/lke/versions")
            data = response.json()
            versions: list[dict[str, Any]] = data.get("data", [])
            return versions
        except httpx.HTTPError as e:
            raise NetworkError("ListLKEVersions", e) from e

    async def get_lke_version(self, version_id: str) -> dict[str, Any]:
        """Get a specific LKE Kubernetes version."""
        endpoint = f"/lke/versions/{version_id}"
        try:
            response = await self.make_request("GET", endpoint)
            version: dict[str, Any] = response.json()
            return version
        except httpx.HTTPError as e:
            raise NetworkError("GetLKEVersion", e) from e

    async def list_lke_types(self) -> list[dict[str, Any]]:
        """List available LKE node types."""
        try:
            response = await self.make_request("GET", "/lke/types")
            data = response.json()
            types: list[dict[str, Any]] = data.get("data", [])
            return types
        except httpx.HTTPError as e:
            raise NetworkError("ListLKETypes", e) from e

    async def list_lke_tier_versions(self) -> list[dict[str, Any]]:
        """List LKE tier versions."""
        try:
            response = await self.make_request("GET", "/lke/tiers/versions")
            data = response.json()
            versions: list[dict[str, Any]] = data.get("data", [])
            return versions
        except httpx.HTTPError as e:
            raise NetworkError("ListLKETierVersions", e) from e

    # VPC operations

    async def list_vpcs(self) -> list[dict[str, Any]]:
        """List VPCs."""
        try:
            response = await self.make_request("GET", "/vpcs")
            data = response.json()
            vpcs: list[dict[str, Any]] = data.get("data", [])
            return vpcs
        except httpx.HTTPError as e:
            raise NetworkError("ListVPCs", e) from e

    async def get_vpc(self, vpc_id: int) -> dict[str, Any]:
        """Get a specific VPC."""
        endpoint = f"/vpcs/{vpc_id}"
        try:
            response = await self.make_request("GET", endpoint)
            vpc: dict[str, Any] = response.json()
            return vpc
        except httpx.HTTPError as e:
            raise NetworkError("GetVPC", e) from e

    async def create_vpc(
        self,
        label: str,
        region: str,
        description: str | None = None,
        subnets: list[dict[str, Any]] | None = None,
    ) -> dict[str, Any]:
        """Create a new VPC."""
        try:
            body: dict[str, Any] = {
                "label": label,
                "region": region,
            }
            if description is not None:
                body["description"] = description
            if subnets is not None:
                body["subnets"] = subnets
            response = await self.make_request("POST", "/vpcs", body)
            vpc: dict[str, Any] = response.json()
            return vpc
        except httpx.HTTPError as e:
            raise NetworkError("CreateVPC", e) from e

    async def update_vpc(
        self,
        vpc_id: int,
        label: str | None = None,
        description: str | None = None,
    ) -> dict[str, Any]:
        """Update a VPC."""
        endpoint = f"/vpcs/{vpc_id}"
        try:
            body: dict[str, Any] = {}
            if label is not None:
                body["label"] = label
            if description is not None:
                body["description"] = description
            response = await self.make_request("PUT", endpoint, body)
            vpc: dict[str, Any] = response.json()
            return vpc
        except httpx.HTTPError as e:
            raise NetworkError("UpdateVPC", e) from e

    async def delete_vpc(self, vpc_id: int) -> None:
        """Delete a VPC."""
        endpoint = f"/vpcs/{vpc_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteVPC", e) from e

    async def list_vpc_ips(self) -> list[dict[str, Any]]:
        """List all VPC IP addresses."""
        try:
            response = await self.make_request("GET", "/vpcs/ips")
            data = response.json()
            ips: list[dict[str, Any]] = data.get("data", [])
            return ips
        except httpx.HTTPError as e:
            raise NetworkError("ListVPCIPs", e) from e

    async def list_vpc_ip(self, vpc_id: int) -> list[dict[str, Any]]:
        """List IP addresses for a specific VPC."""
        endpoint = f"/vpcs/{vpc_id}/ips"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            ips: list[dict[str, Any]] = data.get("data", [])
            return ips
        except httpx.HTTPError as e:
            raise NetworkError("ListVPCIP", e) from e

    async def list_vpc_subnets(self, vpc_id: int) -> list[dict[str, Any]]:
        """List subnets for a VPC."""
        endpoint = f"/vpcs/{vpc_id}/subnets"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            subnets: list[dict[str, Any]] = data.get("data", [])
            return subnets
        except httpx.HTTPError as e:
            raise NetworkError("ListVPCSubnets", e) from e

    async def get_vpc_subnet(self, vpc_id: int, subnet_id: int) -> dict[str, Any]:
        """Get a specific VPC subnet."""
        endpoint = f"/vpcs/{vpc_id}/subnets/{subnet_id}"
        try:
            response = await self.make_request("GET", endpoint)
            subnet: dict[str, Any] = response.json()
            return subnet
        except httpx.HTTPError as e:
            raise NetworkError("GetVPCSubnet", e) from e

    async def create_vpc_subnet(
        self,
        vpc_id: int,
        label: str,
        ipv4: str,
    ) -> dict[str, Any]:
        """Create a new subnet in a VPC."""
        endpoint = f"/vpcs/{vpc_id}/subnets"
        try:
            body: dict[str, Any] = {
                "label": label,
                "ipv4": ipv4,
            }
            response = await self.make_request("POST", endpoint, body)
            subnet: dict[str, Any] = response.json()
            return subnet
        except httpx.HTTPError as e:
            raise NetworkError("CreateVPCSubnet", e) from e

    async def update_vpc_subnet(
        self,
        vpc_id: int,
        subnet_id: int,
        label: str,
    ) -> dict[str, Any]:
        """Update a VPC subnet."""
        endpoint = f"/vpcs/{vpc_id}/subnets/{subnet_id}"
        try:
            body: dict[str, Any] = {"label": label}
            response = await self.make_request("PUT", endpoint, body)
            subnet: dict[str, Any] = response.json()
            return subnet
        except httpx.HTTPError as e:
            raise NetworkError("UpdateVPCSubnet", e) from e

    async def delete_vpc_subnet(self, vpc_id: int, subnet_id: int) -> None:
        """Delete a VPC subnet."""
        endpoint = f"/vpcs/{vpc_id}/subnets/{subnet_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteVPCSubnet", e) from e

    async def create_ipv6_range(
        self,
        prefix_length: int,
        linode_id: int | None = None,
        route_target: str | None = None,
    ) -> dict[str, Any]:
        """Create an IPv6 range."""
        try:
            body: dict[str, Any] = {"prefix_length": prefix_length}
            if linode_id is not None:
                body["linode_id"] = linode_id
            if route_target is not None:
                body["route_target"] = route_target

            response = await self.make_request("POST", "/networking/ipv6/ranges", body)
            ipv6_range: dict[str, Any] = response.json()
            return ipv6_range
        except httpx.HTTPError as e:
            raise NetworkError("CreateIPv6Range", e) from e

    async def list_ipv6_ranges(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List IPv6 ranges."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        query_string = urlencode(params) if params else ""
        endpoint = (
            f"/networking/ipv6/ranges?{query_string}"
            if query_string
            else "/networking/ipv6/ranges"
        )
        try:
            response = await self.make_request("GET", endpoint)
            ipv6_ranges: dict[str, Any] = response.json()
            return ipv6_ranges
        except httpx.HTTPError as e:
            raise NetworkError("ListIPv6Ranges", e) from e

    async def list_ipv6_pools(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List IPv6 pools."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        query_string = urlencode(params) if params else ""
        endpoint = (
            f"/networking/ipv6/pools?{query_string}"
            if query_string
            else "/networking/ipv6/pools"
        )
        try:
            response = await self.make_request("GET", endpoint)
            ipv6_pools: dict[str, Any] = response.json()
            return ipv6_pools
        except httpx.HTTPError as e:
            raise NetworkError("ListIPv6Pools", e) from e

    async def list_placement_groups(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List placement groups."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        query_string = urlencode(params) if params else ""
        endpoint = (
            f"/placement/groups?{query_string}" if query_string else "/placement/groups"
        )
        try:
            response = await self.make_request("GET", endpoint)
            placement_groups: dict[str, Any] = response.json()
            return placement_groups
        except httpx.HTTPError as e:
            raise NetworkError("ListPlacementGroups", e) from e

    async def create_placement_group(
        self,
        label: str,
        region: str,
        placement_group_type: str,
        placement_group_policy: str,
    ) -> dict[str, Any]:
        """Create a placement group."""
        if not _PLACEMENT_GROUP_LABEL_PATTERN.fullmatch(label):
            raise ValueError(
                "label must start and end with an alphanumeric character "
                "and contain only alphanumeric characters, hyphens, "
                "underscores, or periods"
            )
        if not region:
            raise ValueError("region is required")
        if placement_group_type != "anti_affinity:local":
            raise ValueError("placement_group_type must be anti_affinity:local")
        if placement_group_policy not in {"strict", "flexible"}:
            raise ValueError("placement_group_policy must be strict or flexible")

        body: dict[str, Any] = {
            "label": label,
            "region": region,
            "placement_group_type": placement_group_type,
            "placement_group_policy": placement_group_policy,
        }

        try:
            response = await self.make_request("POST", "/placement/groups", body)
            placement_group: dict[str, Any] = response.json()
            return placement_group
        except httpx.HTTPError as e:
            raise NetworkError("CreatePlacementGroup", e) from e

    async def get_placement_group(self, group_id: int) -> dict[str, Any]:
        """Get a placement group."""
        encoded_group_id = quote(str(group_id), safe="")
        endpoint = f"/placement/groups/{encoded_group_id}"
        try:
            response = await self.make_request("GET", endpoint)
            placement_group: dict[str, Any] = response.json()
            return placement_group
        except httpx.HTTPError as e:
            raise NetworkError("GetPlacementGroup", e) from e

    async def assign_placement_group(
        self, group_id: int, linodes: list[int]
    ) -> dict[str, Any]:
        """Assign Linodes to a placement group."""
        encoded_group_id = quote(str(group_id), safe="")
        endpoint = f"/placement/groups/{encoded_group_id}/assign"
        try:
            response = await self.make_request("POST", endpoint, {"linodes": linodes})
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("AssignPlacementGroup", e) from e

    async def unassign_placement_group(
        self, group_id: int, linodes: list[int]
    ) -> dict[str, Any]:
        """Unassign Linodes from a placement group."""
        encoded_group_id = quote(str(group_id), safe="")
        endpoint = f"/placement/groups/{encoded_group_id}/unassign"
        try:
            response = await self.make_request("POST", endpoint, {"linodes": linodes})
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("UnassignPlacementGroup", e) from e

    async def delete_placement_group(self, group_id: int) -> None:
        """Delete a placement group."""
        encoded_group_id = quote(str(group_id), safe="")
        endpoint = f"/placement/groups/{encoded_group_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeletePlacementGroup", e) from e

    async def update_placement_group(self, group_id: int, label: str) -> dict[str, Any]:
        """Update a placement group."""
        if not _PLACEMENT_GROUP_LABEL_PATTERN.fullmatch(label):
            raise ValueError(
                "label must start and end with an alphanumeric character "
                "and contain only alphanumeric characters, hyphens, "
                "underscores, or periods"
            )

        encoded_group_id = quote(str(group_id), safe="")
        endpoint = f"/placement/groups/{encoded_group_id}"
        body: dict[str, Any] = {"label": label}

        try:
            response = await self.make_request("PUT", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("UpdatePlacementGroup", e) from e

    async def get_ipv6_range(self, ipv6_range: str) -> dict[str, Any]:
        """Get an IPv6 range."""
        encoded_range = quote(ipv6_range, safe="")
        endpoint = f"/networking/ipv6/ranges/{encoded_range}"
        try:
            response = await self.make_request("GET", endpoint)
            range_data: dict[str, Any] = response.json()
            return range_data
        except httpx.HTTPError as e:
            raise NetworkError("GetIPv6Range", e) from e

    async def delete_ipv6_range(self, ipv6_range: str) -> None:
        """Delete an IPv6 range."""
        encoded_range = quote(ipv6_range, safe="")
        endpoint = f"/networking/ipv6/ranges/{encoded_range}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteIPv6Range", e) from e

    # ── Instance Backups ──

    async def list_instance_backups(self, instance_id: int) -> dict[str, Any]:
        """List backups for an instance."""
        endpoint = f"/linode/instances/{instance_id}/backups"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceBackups", e) from e

    async def get_instance_backup(
        self, instance_id: int, backup_id: int
    ) -> dict[str, Any]:
        """Get a specific backup for an instance."""
        endpoint = f"/linode/instances/{instance_id}/backups/{backup_id}"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceBackup", e) from e

    async def create_instance_backup(
        self, instance_id: int, label: str | None = None
    ) -> dict[str, Any]:
        """Create a snapshot backup of an instance."""
        endpoint = f"/linode/instances/{instance_id}/backups"
        try:
            body: dict[str, Any] = {}
            if label is not None:
                body["label"] = label
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("CreateInstanceBackup", e) from e

    async def restore_instance_backup(
        self,
        instance_id: int,
        backup_id: int,
        linode_id: int,
        overwrite: bool = False,
    ) -> None:
        """Restore a backup to an instance."""
        endpoint = f"/linode/instances/{instance_id}/backups/{backup_id}/restore"
        try:
            body: dict[str, Any] = {
                "linode_id": linode_id,
                "overwrite": overwrite,
            }
            await self.make_request("POST", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("RestoreInstanceBackup", e) from e

    async def enable_instance_backups(self, instance_id: int) -> None:
        """Enable backups for an instance."""
        endpoint = f"/linode/instances/{instance_id}/backups/enable"
        try:
            await self.make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("EnableInstanceBackups", e) from e

    async def cancel_instance_backups(self, instance_id: int) -> None:
        """Cancel backups for an instance."""
        endpoint = f"/linode/instances/{instance_id}/backups/cancel"
        try:
            await self.make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("CancelInstanceBackups", e) from e

    # ── Instance Disks ──

    async def list_instance_disks(self, instance_id: int) -> list[dict[str, Any]]:
        """List disks for an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks"
        try:
            response = await self.make_request("GET", endpoint)
            data = response.json()
            disks: list[dict[str, Any]] = data.get("data", [])
            return disks
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceDisks", e) from e

    async def get_instance_disk(self, instance_id: int, disk_id: int) -> dict[str, Any]:
        """Get a specific disk for an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks/{disk_id}"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceDisk", e) from e

    async def create_instance_disk(
        self,
        instance_id: int,
        label: str,
        size: int,
        filesystem: str | None = None,
        image: str | None = None,
        root_pass: str | None = None,
    ) -> dict[str, Any]:
        """Create a disk for an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks"
        try:
            body: dict[str, Any] = {
                "label": label,
                "size": size,
            }
            if filesystem is not None:
                body["filesystem"] = filesystem
            if image is not None:
                body["image"] = image
            if root_pass is not None:
                body["root_pass"] = root_pass
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("CreateInstanceDisk", e) from e

    async def update_instance_disk(
        self,
        instance_id: int,
        disk_id: int,
        label: str | None = None,
    ) -> dict[str, Any]:
        """Update a disk for an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks/{disk_id}"
        try:
            body: dict[str, Any] = {}
            if label is not None:
                body["label"] = label
            response = await self.make_request("PUT", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("UpdateInstanceDisk", e) from e

    async def delete_instance_disk(self, instance_id: int, disk_id: int) -> None:
        """Delete a disk from an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks/{disk_id}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteInstanceDisk", e) from e

    async def clone_instance_disk(
        self, instance_id: int, disk_id: int
    ) -> dict[str, Any]:
        """Clone a disk on an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks/{disk_id}/clone"
        try:
            response = await self.make_request("POST", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("CloneInstanceDisk", e) from e

    async def resize_instance_disk(
        self, instance_id: int, disk_id: int, size: int
    ) -> None:
        """Resize a disk on an instance."""
        endpoint = f"/linode/instances/{instance_id}/disks/{disk_id}/resize"
        try:
            body: dict[str, Any] = {"size": size}
            await self.make_request("POST", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("ResizeInstanceDisk", e) from e

    # ── Instance IPs ──

    async def list_instance_ips(self, instance_id: int) -> dict[str, Any]:
        """List IP addresses for an instance."""
        endpoint = f"/linode/instances/{instance_id}/ips"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceIPs", e) from e

    async def get_instance_ip(self, instance_id: int, address: str) -> dict[str, Any]:
        """Get a specific IP address for an instance."""
        endpoint = f"/linode/instances/{instance_id}/ips/{quote(address, safe='')}"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceIP", e) from e

    async def get_networking_ip(self, address: str) -> dict[str, Any]:
        """Get a networking-level IP address."""
        endpoint = f"/networking/ips/{quote(address, safe='')}"
        try:
            response = await self.make_request("GET", endpoint)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetNetworkingIP", e) from e

    async def allocate_instance_ip(
        self,
        instance_id: int,
        ip_type: str,
        public: bool = True,
    ) -> dict[str, Any]:
        """Allocate a new IP address for an instance."""
        endpoint = f"/linode/instances/{instance_id}/ips"
        try:
            body: dict[str, Any] = {
                "type": ip_type,
                "public": public,
            }
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("AllocateInstanceIP", e) from e

    async def update_instance_ip(
        self,
        instance_id: int,
        address: str,
        rdns: str | None,
    ) -> dict[str, Any]:
        """Update reverse DNS for a specific IP address on an instance."""
        endpoint = f"/linode/instances/{instance_id}/ips/{quote(address, safe='')}"
        try:
            body: dict[str, Any] = {"rdns": rdns}
            response = await self.make_request("PUT", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("UpdateInstanceIP", e) from e

    async def list_networking_ips(
        self, skip_ipv6_rdns: bool = False
    ) -> list[dict[str, Any]]:
        """List all IP addresses at the networking level."""
        all_ips: list[dict[str, Any]] = []
        page = 1
        try:
            while True:
                endpoint = "/networking/ips"
                query_parts: list[str] = []
                if skip_ipv6_rdns:
                    query_parts.append("skip_ipv6_rdns=true")
                if page > 1:
                    query_parts.append(f"page={page}")
                if query_parts:
                    endpoint += "?" + "&".join(query_parts)

                response = await self.make_request("GET", endpoint)
                data = response.json()
                ips: list[dict[str, Any]] = data.get("data", [])
                all_ips.extend(ips)

                total_pages = data.get("pages", page)
                if not isinstance(total_pages, int) or page >= total_pages:
                    return all_ips
                page += 1
        except httpx.HTTPError as e:
            raise NetworkError("ListNetworkingIPs", e) from e

    async def update_networking_ip(
        self,
        address: str,
        rdns: str | None,
    ) -> dict[str, Any]:
        """Update reverse DNS for a networking-level IP address."""
        endpoint = f"/networking/ips/{quote(address, safe='')}"
        try:
            body: dict[str, Any] = {"rdns": rdns}
            response = await self.make_request("PUT", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("UpdateNetworkingIP", e) from e

    async def allocate_networking_ip(
        self,
        linode_id: int,
        ip_type: str,
        public: bool = True,
    ) -> dict[str, Any]:
        """Allocate a new IP address at the networking level."""
        endpoint = "/networking/ips"
        try:
            body: dict[str, Any] = {
                "linode_id": linode_id,
                "type": ip_type,
                "public": public,
            }
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("AllocateNetworkingIP", e) from e

    async def delete_instance_ip(self, instance_id: int, address: str) -> None:
        """Delete an IP address from an instance."""
        endpoint = f"/linode/instances/{instance_id}/ips/{quote(address, safe='')}"
        try:
            await self.make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteInstanceIP", e) from e

    # ── Instance Actions ──

    async def clone_instance(
        self,
        instance_id: int,
        region: str | None = None,
        instance_type: str | None = None,
        label: str | None = None,
        disks: list[int] | None = None,
        configs: list[int] | None = None,
    ) -> dict[str, Any]:
        """Clone an instance."""
        endpoint = f"/linode/instances/{instance_id}/clone"
        try:
            body: dict[str, Any] = {}
            if region is not None:
                body["region"] = region
            if instance_type is not None:
                body["type"] = instance_type
            if label is not None:
                body["label"] = label
            if disks is not None:
                body["disks"] = disks
            if configs is not None:
                body["configs"] = configs
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("CloneInstance", e) from e

    async def migrate_instance(
        self,
        instance_id: int,
        region: str | None = None,
    ) -> None:
        """Migrate an instance to a new region."""
        endpoint = f"/linode/instances/{instance_id}/migrate"
        try:
            body: dict[str, Any] = {}
            if region is not None:
                body["region"] = region
            await self.make_request("POST", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("MigrateInstance", e) from e

    async def rebuild_instance(
        self,
        instance_id: int,
        image: str,
        root_pass: str,
        authorized_keys: list[str] | None = None,
        authorized_users: list[str] | None = None,
    ) -> dict[str, Any]:
        """Rebuild an instance with a new image."""
        endpoint = f"/linode/instances/{instance_id}/rebuild"
        try:
            body: dict[str, Any] = {
                "image": image,
                "root_pass": root_pass,
            }
            if authorized_keys is not None:
                body["authorized_keys"] = authorized_keys
            if authorized_users is not None:
                body["authorized_users"] = authorized_users
            response = await self.make_request("POST", endpoint, body)
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("RebuildInstance", e) from e

    async def rescue_instance(
        self,
        instance_id: int,
        devices: dict[str, Any] | None = None,
    ) -> None:
        """Boot an instance into rescue mode."""
        endpoint = f"/linode/instances/{instance_id}/rescue"
        try:
            body: dict[str, Any] = {}
            if devices is not None:
                body["devices"] = devices
            await self.make_request("POST", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("RescueInstance", e) from e

    async def reset_instance_password(self, instance_id: int, root_pass: str) -> None:
        """Reset the root password for an instance."""
        endpoint = f"/linode/instances/{instance_id}/password"
        try:
            body: dict[str, Any] = {"root_pass": root_pass}
            await self.make_request("POST", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("ResetInstancePassword", e) from e

    async def make_request(
        self, method: str, endpoint: str, body: dict[str, Any] | None = None
    ) -> httpx.Response:
        """Make an HTTP request to the Linode API."""
        url = self.base_url + endpoint
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "User-Agent": "LinodeMCP/1.0",
        }

        if body is not None:
            response = await self.client.request(
                method, url, headers=headers, json=body
            )
        else:
            response = await self.client.request(method, url, headers=headers)

        if response.status_code >= HTTP_BAD_REQUEST:
            self._handle_error_response(response)

        return response

    async def make_file_request(
        self, method: str, endpoint: str, file_path: str
    ) -> httpx.Response:
        """Make a multipart file upload request to the Linode API."""
        url = self.base_url + endpoint
        path = Path(file_path)
        headers = {
            "Authorization": f"Bearer {self.token}",
            "User-Agent": "LinodeMCP/1.0",
        }

        with path.open("rb") as file_obj:
            file_handle: BinaryIO = file_obj
            response = await self.client.request(
                method,
                url,
                headers=headers,
                files={"file": (path.name, file_handle)},
            )

        if response.status_code >= HTTP_BAD_REQUEST:
            self._handle_error_response(response)

        return response

    def _handle_error_response(self, response: httpx.Response) -> None:
        """Handle error responses from the API."""
        try:
            error_data = response.json()
            errors = error_data.get("errors", [])
            if errors:
                raise APIError(
                    status_code=response.status_code,
                    message=errors[0].get("reason", "Unknown error"),
                    field=errors[0].get("field", ""),
                )
        except (ValueError, KeyError) as e:
            logger.debug("Failed to parse error response body: %s", e)

        if response.status_code == HTTP_UNAUTHORIZED:
            raise APIError(
                HTTP_UNAUTHORIZED, "Authentication failed. Please check your API token."
            )
        if response.status_code == HTTP_FORBIDDEN:
            raise APIError(
                HTTP_FORBIDDEN,
                "Access forbidden. Your API token may not have sufficient permissions.",
            )
        if response.status_code == HTTP_TOO_MANY_REQUESTS:
            retry_after = response.headers.get("Retry-After", "")
            message = "Rate limit exceeded. Please try again later."
            if retry_after:
                message = f"Rate limit exceeded. Retry after {retry_after}."
            raise APIError(HTTP_TOO_MANY_REQUESTS, message)
        if response.status_code >= HTTP_SERVER_ERROR:
            raise APIError(
                response.status_code, "Internal server error. Please try again later."
            )

        raise APIError(
            response.status_code,
            f"API request failed with status {response.status_code}",
        )

    def _parse_instance(self, data: dict[str, Any]) -> Instance:
        """Parse instance data from API response."""
        specs_data = data.get("specs", {})
        specs = Specs(
            disk=specs_data.get("disk", 0),
            memory=specs_data.get("memory", 0),
            vcpus=specs_data.get("vcpus", 0),
            gpus=specs_data.get("gpus", 0),
            transfer=specs_data.get("transfer", 0),
        )

        alerts_data = data.get("alerts", {})
        alerts = Alerts(
            cpu=alerts_data.get("cpu", 0),
            network_in=alerts_data.get("network_in", 0),
            network_out=alerts_data.get("network_out", 0),
            transfer_quota=alerts_data.get("transfer_quota", 0),
            io=alerts_data.get("io", 0),
        )

        backups_data = data.get("backups", {})
        schedule_data = backups_data.get("schedule", {})
        schedule = Schedule(
            day=schedule_data.get("day", ""),
            window=schedule_data.get("window", ""),
        )

        last_backup = None
        if last_data := backups_data.get("last_successful"):
            last_backup = Backup(
                id=last_data.get("id", 0),
                label=last_data.get("label", ""),
                status=last_data.get("status", ""),
                type=last_data.get("type", ""),
                region=last_data.get("region", ""),
                created=last_data.get("created", ""),
                updated=last_data.get("updated", ""),
                finished=last_data.get("finished", ""),
            )

        backups = Backups(
            enabled=backups_data.get("enabled", False),
            available=backups_data.get("available", False),
            schedule=schedule,
            last_successful=last_backup,
        )

        interfaces = [
            _parse_instance_interface(iface_data)
            for iface_data in data.get("interfaces", [])
        ]

        return Instance(
            id=data.get("id", 0),
            label=data.get("label", ""),
            status=data.get("status", ""),
            type=data.get("type", ""),
            region=data.get("region", ""),
            image=data.get("image", ""),
            ipv4=data.get("ipv4", []),
            ipv6=data.get("ipv6", ""),
            hypervisor=data.get("hypervisor", ""),
            specs=specs,
            alerts=alerts,
            backups=backups,
            created=data.get("created", ""),
            updated=data.get("updated", ""),
            group=data.get("group", ""),
            tags=data.get("tags", []),
            watchdog_enabled=data.get("watchdog_enabled", False),
            interface_generation=data.get("interface_generation", ""),
            interfaces=interfaces,
        )

    def _parse_account(self, data: dict[str, Any]) -> Account:
        """Parse account data from API response."""
        promotions = [
            Promo(
                description=promo_data.get("description", ""),
                summary=promo_data.get("summary", ""),
                credit_monthly_cap=promo_data.get("credit_monthly_cap", ""),
                credit_remaining=promo_data.get("credit_remaining", ""),
                expire_dt=promo_data.get("expire_dt", ""),
                image_url=promo_data.get("image_url", ""),
                service_type=promo_data.get("service_type", ""),
                this_month_credit_remaining=promo_data.get(
                    "this_month_credit_remaining", ""
                ),
            )
            for promo_data in data.get("active_promotions", [])
        ]

        return Account(
            first_name=data.get("first_name", ""),
            last_name=data.get("last_name", ""),
            email=data.get("email", ""),
            company=data.get("company", ""),
            address_1=data.get("address_1", ""),
            address_2=data.get("address_2", ""),
            city=data.get("city", ""),
            state=data.get("state", ""),
            zip=data.get("zip", ""),
            country=data.get("country", ""),
            phone=data.get("phone", ""),
            balance=data.get("balance", 0.0),
            balance_uninvoiced=data.get("balance_uninvoiced", 0.0),
            capabilities=data.get("capabilities", []),
            active_since=data.get("active_since", ""),
            euuid=data.get("euuid", ""),
            billing_source=data.get("billing_source", ""),
            active_promotions=promotions,
        )

    def _parse_region(self, data: dict[str, Any]) -> Region:
        """Parse region data from API response."""
        resolvers_data = data.get("resolvers", {})
        resolvers = Resolver(
            ipv4=resolvers_data.get("ipv4", ""),
            ipv6=resolvers_data.get("ipv6", ""),
        )

        return Region(
            id=data.get("id", ""),
            label=data.get("label", ""),
            country=data.get("country", ""),
            capabilities=data.get("capabilities", []),
            status=data.get("status", ""),
            resolvers=resolvers,
            site_type=data.get("site_type", ""),
        )

    def _parse_instance_type(self, data: dict[str, Any]) -> InstanceType:
        """Parse instance type data from API response."""
        price_data = data.get("price", {})
        price = Price(
            hourly=price_data.get("hourly", 0.0),
            monthly=price_data.get("monthly", 0.0),
        )

        addons_data = data.get("addons", {})
        backups_data = addons_data.get("backups", {})
        backups_price_data = backups_data.get("price", {})
        backups_price = Price(
            hourly=backups_price_data.get("hourly", 0.0),
            monthly=backups_price_data.get("monthly", 0.0),
        )
        backups_addon = BackupsAddon(price=backups_price)
        addons = Addons(backups=backups_addon)

        return InstanceType(
            id=data.get("id", ""),
            label=data.get("label", ""),
            class_=data.get("class", ""),
            disk=data.get("disk", 0),
            memory=data.get("memory", 0),
            vcpus=data.get("vcpus", 0),
            gpus=data.get("gpus", 0),
            network_out=data.get("network_out", 0),
            transfer=data.get("transfer", 0),
            price=price,
            addons=addons,
            successor=data.get("successor"),
        )

    def _parse_volume(self, data: dict[str, Any]) -> Volume:
        """Parse volume data from API response."""
        return Volume(
            id=data.get("id", 0),
            label=data.get("label", ""),
            status=data.get("status", ""),
            size=data.get("size", 0),
            region=data.get("region", ""),
            linode_id=data.get("linode_id"),
            linode_label=data.get("linode_label"),
            filesystem_path=data.get("filesystem_path", ""),
            tags=data.get("tags", []),
            created=data.get("created", ""),
            updated=data.get("updated", ""),
            hardware_type=data.get("hardware_type", ""),
        )

    def _parse_image(self, data: dict[str, Any]) -> Image:
        """Parse image data from API response."""
        return Image(
            id=data.get("id", ""),
            label=data.get("label", ""),
            description=data.get("description", ""),
            type=data.get("type", ""),
            is_public=data.get("is_public", False),
            deprecated=data.get("deprecated", False),
            size=data.get("size", 0),
            vendor=data.get("vendor", ""),
            status=data.get("status", ""),
            created=data.get("created", ""),
            created_by=data.get("created_by", ""),
            expiry=data.get("expiry"),
            eol=data.get("eol"),
            capabilities=data.get("capabilities", []),
            tags=data.get("tags", []),
        )

    # Stage 3: Parse methods

    def _parse_ssh_key(self, data: dict[str, Any]) -> SSHKey:
        """Parse SSH key data from API response."""
        return SSHKey(
            id=data.get("id", 0),
            label=data.get("label", ""),
            ssh_key=data.get("ssh_key", ""),
            created=data.get("created", ""),
        )

    def _parse_domain(self, data: dict[str, Any]) -> Domain:
        """Parse domain data from API response."""
        return Domain(
            id=data.get("id", 0),
            domain=data.get("domain", ""),
            type=data.get("type", ""),
            status=data.get("status", ""),
            soa_email=data.get("soa_email", ""),
            description=data.get("description", ""),
            tags=data.get("tags", []),
            created=data.get("created", ""),
            updated=data.get("updated", ""),
        )

    def _parse_domain_record(self, data: dict[str, Any]) -> DomainRecord:
        """Parse domain record data from API response."""
        return DomainRecord(
            id=data.get("id", 0),
            type=data.get("type", ""),
            name=data.get("name", ""),
            target=data.get("target", ""),
            priority=data.get("priority", 0),
            weight=data.get("weight", 0),
            port=data.get("port", 0),
            ttl_sec=data.get("ttl_sec", 0),
            created=data.get("created", ""),
            updated=data.get("updated", ""),
        )

    def _parse_firewall(self, data: dict[str, Any]) -> Firewall:
        """Parse firewall data from API response."""
        rules_data = data.get("rules", {})

        inbound_rules = [
            self._parse_firewall_rule(r) for r in rules_data.get("inbound", [])
        ]
        outbound_rules = [
            self._parse_firewall_rule(r) for r in rules_data.get("outbound", [])
        ]

        rules = FirewallRules(
            inbound=inbound_rules,
            inbound_policy=rules_data.get("inbound_policy", ""),
            outbound=outbound_rules,
            outbound_policy=rules_data.get("outbound_policy", ""),
        )

        return Firewall(
            id=data.get("id", 0),
            label=data.get("label", ""),
            status=data.get("status", ""),
            rules=rules,
            tags=data.get("tags", []),
            created=data.get("created", ""),
            updated=data.get("updated", ""),
        )

    def _parse_firewall_rules(self, data: dict[str, Any]) -> FirewallRules:
        """Parse firewall rules data from API response."""
        inbound_rules = [self._parse_firewall_rule(r) for r in data.get("inbound", [])]
        outbound_rules = [
            self._parse_firewall_rule(r) for r in data.get("outbound", [])
        ]

        return FirewallRules(
            inbound=inbound_rules,
            inbound_policy=data.get("inbound_policy", ""),
            outbound=outbound_rules,
            outbound_policy=data.get("outbound_policy", ""),
        )

    def _parse_firewall_rule(self, data: dict[str, Any]) -> FirewallRule:
        """Parse firewall rule data from API response."""
        addresses_data = data.get("addresses", {})
        addresses = FirewallAddresses(
            ipv4=addresses_data.get("ipv4", []),
            ipv6=addresses_data.get("ipv6", []),
        )

        return FirewallRule(
            action=data.get("action", ""),
            protocol=data.get("protocol", ""),
            ports=data.get("ports", ""),
            addresses=addresses,
            label=data.get("label", ""),
            description=data.get("description", ""),
        )

    def _parse_firewall_template(self, data: dict[str, Any]) -> FirewallTemplate:
        """Parse a FirewallTemplate from API response data."""
        rules_data = data.get("rules", {})
        rules = FirewallRules(
            inbound=[
                self._parse_firewall_rule(r) for r in rules_data.get("inbound", [])
            ],
            outbound=[
                self._parse_firewall_rule(r) for r in rules_data.get("outbound", [])
            ],
            inbound_policy=rules_data.get("inbound_policy", "DROP"),
            outbound_policy=rules_data.get("outbound_policy", "ACCEPT"),
        )
        return FirewallTemplate(
            slug=data["slug"],
            label=data.get("label", ""),
            description=data.get("description", ""),
            rules=rules,
        )

    def _parse_nodebalancer(self, data: dict[str, Any]) -> NodeBalancer:
        """Parse NodeBalancer data from API response."""
        transfer_data = data.get("transfer", {})
        transfer = Transfer(
            in_=transfer_data.get("in", 0.0),
            out=transfer_data.get("out", 0.0),
            total=transfer_data.get("total", 0.0),
        )

        return NodeBalancer(
            id=data.get("id", 0),
            label=data.get("label", ""),
            region=data.get("region", ""),
            hostname=data.get("hostname", ""),
            ipv4=data.get("ipv4", ""),
            ipv6=data.get("ipv6", ""),
            client_conn_throttle=data.get("client_conn_throttle", 0),
            transfer=transfer,
            tags=data.get("tags", []),
            created=data.get("created", ""),
            updated=data.get("updated", ""),
        )

    def _parse_stackscript(self, data: dict[str, Any]) -> StackScript:
        """Parse StackScript data from API response."""
        user_defined_fields = [
            UDF(
                label=udf.get("label", ""),
                name=udf.get("name", ""),
                example=udf.get("example", ""),
                oneof=udf.get("oneof", ""),
                default=udf.get("default", ""),
            )
            for udf in data.get("user_defined_fields", [])
        ]

        return StackScript(
            id=data.get("id", 0),
            username=data.get("username", ""),
            user_gravatar_id=data.get("user_gravatar_id", ""),
            label=data.get("label", ""),
            description=data.get("description", ""),
            images=data.get("images", []),
            deployments_total=data.get("deployments_total", 0),
            deployments_active=data.get("deployments_active", 0),
            is_public=data.get("is_public", False),
            mine=data.get("mine", False),
            created=data.get("created", ""),
            updated=data.get("updated", ""),
            script=data.get("script", ""),
            user_defined_fields=user_defined_fields,
        )


@dataclass
class RetryConfig:
    """Configuration for retry behavior."""

    max_retries: int = 3
    base_delay: float = 1.0
    max_delay: float = 30.0
    backoff_factor: float = 2.0
    jitter_enabled: bool = True
    circuit_breaker_threshold: int = 5
    circuit_breaker_timeout: float = 30.0
    rate_limit_per_minute: int = 700
    pool_max_connections: int = 10
    pool_max_keepalive_connections: int = 10
    pool_keepalive_expiry: float = 30.0


_SECONDS_PER_MINUTE = 60.0


class RateLimiter:
    """Asyncio token-bucket rate limiter.

    Capacity equals the per-minute budget so a fully-replenished bucket
    permits one minute's worth of burst, then settles to the steady refill
    rate. A non-positive rate disables the limiter (wait is a no-op).
    """

    def __init__(self, per_minute: int) -> None:
        self._enabled = per_minute > 0
        if not self._enabled:
            return
        self._capacity = float(per_minute)
        self._refill_rate = self._capacity / _SECONDS_PER_MINUTE
        self._tokens = self._capacity
        self._last_refill = time.monotonic()
        self._lock = asyncio.Lock()

    async def wait(self) -> None:
        """Block until one token is available; cancellation propagates.

        Each call consumes exactly one token. Disabled limiters return
        immediately so callers don't need a special case.
        """
        if not self._enabled:
            return
        while True:
            async with self._lock:
                self._refill()
                if self._tokens >= 1:
                    self._tokens -= 1
                    return
                needed = 1 - self._tokens
                wait_time = needed / self._refill_rate
            # Sleep outside the lock so other coroutines can refill checks
            # while this one is parked.
            await asyncio.sleep(wait_time)

    def _refill(self) -> None:
        now = time.monotonic()
        elapsed = now - self._last_refill
        if elapsed > 0:
            self._tokens = min(
                self._capacity, self._tokens + elapsed * self._refill_rate
            )
            self._last_refill = now


class _CircuitState(enum.Enum):
    """Lifecycle position of the circuit breaker."""

    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"


class CircuitBreaker:
    """Counting circuit breaker.

    Trips after `threshold` consecutive failures, stays open for `timeout`
    seconds, then admits one probe (half-open). A successful probe closes;
    a failing probe re-opens the timer. A non-positive threshold disables
    the breaker entirely (`allow` is always a no-op).
    """

    def __init__(self, threshold: int, timeout: float) -> None:
        self._threshold = threshold
        self._timeout = timeout
        self._state = _CircuitState.CLOSED
        self._consecutive_failures = 0
        self._opened_at = 0.0
        self._lock = threading.Lock()

    def allow(self) -> None:
        """Raise CircuitOpenError if the breaker is rejecting requests.

        Transitions OPEN -> HALF_OPEN once the cooldown elapses, admitting
        exactly one probe. Concurrent calls during the in-flight probe see
        HALF_OPEN and are rejected.
        """
        if self._threshold <= 0:
            return

        with self._lock:
            if self._state is _CircuitState.CLOSED:
                return
            if self._state is _CircuitState.OPEN:
                if time.monotonic() - self._opened_at >= self._timeout:
                    self._state = _CircuitState.HALF_OPEN
                    return
                raise CircuitOpenError("circuit breaker open")
            # HALF_OPEN: a probe is already in flight; reject.
            raise CircuitOpenError("circuit breaker open")

    def record_success(self) -> None:
        """Close the breaker and reset the failure counter."""
        if self._threshold <= 0:
            return

        with self._lock:
            self._consecutive_failures = 0
            self._state = _CircuitState.CLOSED

    def record_failure(self) -> None:
        """Increment the failure counter; trip once threshold is reached.

        Callers should invoke this only for upstream-health failures (5xx,
        network, 429). Auth errors and caller cancellations are not the
        breaker's concern.
        """
        if self._threshold <= 0:
            return

        with self._lock:
            self._consecutive_failures += 1
            if self._consecutive_failures >= self._threshold:
                self._state = _CircuitState.OPEN
                self._opened_at = time.monotonic()


class RetryableClient:
    """Linode API client with retry functionality and a circuit breaker."""

    def __init__(
        self, api_url: str, token: str, retry_config: RetryConfig | None = None
    ) -> None:
        self.retry_config = retry_config or RetryConfig()
        self.client = Client(
            api_url,
            token,
            max_connections=self.retry_config.pool_max_connections,
            max_keepalive_connections=self.retry_config.pool_max_keepalive_connections,
            keepalive_expiry=self.retry_config.pool_keepalive_expiry,
        )
        self._request_semaphore = asyncio.Semaphore(10)
        self._circuit = CircuitBreaker(
            self.retry_config.circuit_breaker_threshold,
            self.retry_config.circuit_breaker_timeout,
        )
        self._limiter = RateLimiter(self.retry_config.rate_limit_per_minute)

    async def close(self) -> None:
        """Close the HTTP client."""
        await self.client.close()

    async def __aenter__(self) -> "RetryableClient":
        """Async context manager entry."""
        return self

    async def __aexit__(self, *args: Any) -> None:
        """Async context manager exit."""
        await self.close()

    async def get_profile(self) -> Profile:
        """Get Linode user profile with retry."""
        result: Profile = await self._execute_with_retry(self.client.get_profile)
        return result

    async def update_profile(self, **fields: Any) -> Profile:
        """Update Linode user profile with retry."""
        result: Profile = await self._execute_with_retry(
            lambda: self.client.update_profile(**fields)
        )
        return result

    async def get_profile_preferences(self) -> dict[str, Any]:
        """Get OAuth client-specific profile preferences with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_preferences
        )
        return result

    async def update_profile_preferences(
        self, preferences: dict[str, Any]
    ) -> dict[str, Any]:
        """Update OAuth client-specific profile preferences with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_profile_preferences, preferences
        )
        return result

    async def get_profile_grants(self) -> Grants:
        """Get /profile/grants with retry. PATs return an empty Grants."""
        result: Grants = await self._execute_with_retry(self.client.get_profile_grants)
        return result

    async def list_instances(self) -> list[Instance]:
        """List Linode instances with retry."""
        result: list[Instance] = await self._execute_with_retry(
            self.client.list_instances
        )
        return result

    async def get_instance(self, instance_id: int) -> Instance:
        """Get a specific Linode instance with retry."""
        result: Instance = await self._execute_with_retry(
            self.client.get_instance, instance_id
        )
        return result

    async def update_instance(self, instance_id: int, **fields: Any) -> Instance:
        """Update a Linode instance with retry."""
        result: Instance = await self._execute_with_retry(
            lambda: self.client.update_instance(instance_id, **fields)
        )
        return result

    async def get_account(self) -> Account:
        """Get Linode account information with retry."""
        result: Account = await self._execute_with_retry(self.client.get_account)
        return result

    async def get_account_agreements(self) -> dict[str, Any]:
        """List account agreements with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_agreements
        )
        return result

    async def get_account_settings(self) -> dict[str, Any]:
        """Get account settings with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_settings
        )
        return result

    async def enable_account_managed(self) -> dict[str, Any]:
        """Enable Linode Managed by delegating once without retry."""
        return await self.client.enable_account_managed()

    async def get_account_transfer(self) -> dict[str, Any]:
        """Get account network transfer usage with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_transfer
        )
        return result

    async def list_account_logins(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account logins with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_logins(page=page, page_size=page_size)
        )
        return result

    async def list_account_users(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account users with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_users(page=page, page_size=page_size)
        )
        return result

    async def delete_account_user(self, username: str) -> dict[str, Any]:
        """Delete an account user by delegating once without retry."""
        return await self.client.delete_account_user(username)

    async def list_account_maintenance(self) -> dict[str, Any]:
        """List account maintenance with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_account_maintenance
        )
        return result

    async def list_account_oauth_clients(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account OAuth clients with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_oauth_clients(
                page=page, page_size=page_size
            )
        )
        return result

    async def update_account_oauth_client(
        self, client_id: str, **fields: Any
    ) -> dict[str, Any]:
        """Update an account OAuth client with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.update_account_oauth_client(client_id, **fields)
        )
        return result

    async def update_account_oauth_client_thumbnail(
        self, client_id: str
    ) -> dict[str, Any]:
        """Update an account OAuth client thumbnail without replaying the write."""
        return await self.client.update_account_oauth_client_thumbnail(client_id)

    async def list_account_events(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account events with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_events(page=page, page_size=page_size)
        )
        return result

    async def list_account_invoices(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account invoices with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_invoices(page=page, page_size=page_size)
        )
        return result

    async def list_account_payments(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account payments with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_payments(page=page, page_size=page_size)
        )
        return result

    async def list_account_payment_methods(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account payment methods with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_payment_methods(
                page=page, page_size=page_size
            )
        )
        return result

    async def list_account_notifications(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account notifications with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_notifications(
                page=page, page_size=page_size
            )
        )
        return result

    async def list_account_invoice_items(
        self, invoice_id: int, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account invoice items with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_invoice_items(
                invoice_id, page=page, page_size=page_size
            )
        )
        return result

    async def get_account_event(self, event_id: int) -> dict[str, Any]:
        """Get an account event with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_event, event_id
        )
        return result

    async def mark_account_event_seen(self, event_id: int) -> dict[str, Any]:
        """Mark an account event seen once without retry replay."""
        return await self.client.mark_account_event_seen(event_id)

    async def acknowledge_account_agreements(
        self, agreements: dict[str, bool]
    ) -> dict[str, Any]:
        """Acknowledge account agreements once without retry replay."""
        return await self.client.acknowledge_account_agreements(agreements)

    async def list_account_betas(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List enrolled Beta programs for the account with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_betas(page=page, page_size=page_size)
        )
        return result

    async def list_betas(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List available Beta programs with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_betas(page=page, page_size=page_size)
        )
        return result

    async def list_mysql_database_instances(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List MySQL Managed Database instances with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_mysql_database_instances(
                page=page, page_size=page_size
            )
        )
        return result

    async def list_postgresql_database_instances(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List PostgreSQL Managed Database instances with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_postgresql_database_instances(
                page=page, page_size=page_size
            )
        )
        return result

    async def list_database_instances(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List Managed Database instances with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_database_instances(page=page, page_size=page_size)
        )
        return result

    async def create_mysql_database_instance(
        self, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Create or restore a MySQL Managed Database once without retry replay."""
        return await self.client.create_mysql_database_instance(payload)

    async def create_postgresql_database_instance(
        self, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Create or restore a PostgreSQL Managed Database once without retry replay."""
        return await self.client.create_postgresql_database_instance(payload)

    async def delete_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Delete a MySQL Managed Database once without retry replay."""
        return await self.client.delete_mysql_database_instance(instance_id)

    async def delete_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Delete a PostgreSQL Managed Database once without retry replay."""
        return await self.client.delete_postgresql_database_instance(instance_id)

    async def resume_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Resume a MySQL Managed Database once without retry replay."""
        return await self.client.resume_mysql_database_instance(instance_id)

    async def suspend_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Suspend a MySQL Managed Database once without retry replay."""
        return await self.client.suspend_mysql_database_instance(instance_id)

    async def resume_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Resume a PostgreSQL Managed Database once without retry replay."""
        return await self.client.resume_postgresql_database_instance(instance_id)

    async def update_mysql_database_instance(
        self, instance_id: int, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Update a MySQL Managed Database once without retry replay."""
        return await self.client.update_mysql_database_instance(instance_id, payload)

    async def patch_mysql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Patch a MySQL Managed Database once without retry replay."""
        return await self.client.patch_mysql_database_instance(instance_id)

    async def patch_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Patch a PostgreSQL Managed Database once without retry replay."""
        return await self.client.patch_postgresql_database_instance(instance_id)

    async def enroll_account_beta(self, beta_id: str) -> dict[str, Any]:
        """Enroll in an account beta once without retry replay."""
        return await self.client.enroll_account_beta(beta_id)

    async def get_database_mysql_instance(self, instance_id: int) -> dict[str, Any]:
        """Get a MySQL Managed Database instance with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_mysql_instance, instance_id
        )
        return result

    async def get_database_postgresql_instance(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get a PostgreSQL Managed Database instance with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_postgresql_instance, instance_id
        )
        return result

    async def reset_postgresql_database_credentials(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Reset PostgreSQL Managed Database credentials once without retry replay."""
        return await self.client.reset_postgresql_database_credentials(instance_id)

    async def get_database_postgresql_instance_ssl(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Get a PostgreSQL Managed Database SSL certificate with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_postgresql_instance_ssl, instance_id
        )
        return result

    async def suspend_postgresql_database_instance(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Suspend a PostgreSQL Managed Database once without retry replay."""
        return await self.client.suspend_postgresql_database_instance(instance_id)

    async def get_database_mysql_instance_ssl(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Get a MySQL Managed Database SSL certificate with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_mysql_instance_ssl, instance_id
        )
        return result

    async def get_database_mysql_instance_credentials(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get MySQL Managed Database credentials with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_mysql_instance_credentials, instance_id
        )
        return result

    async def get_database_postgresql_instance_credentials(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get PostgreSQL Managed Database credentials with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_postgresql_instance_credentials, instance_id
        )
        return result

    async def reset_mysql_database_credentials(
        self, instance_id: int | str
    ) -> dict[str, Any]:
        """Reset MySQL Managed Database credentials once without retry replay."""
        return await self.client.reset_mysql_database_credentials(instance_id)

    async def list_account_child_accounts(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List child accounts for the account with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_child_accounts(
                page=page, page_size=page_size
            )
        )
        return result

    async def list_account_service_transfers(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account service transfers with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_service_transfers(
                page=page, page_size=page_size
            )
        )
        return result

    async def create_account_child_account_token(self, euuid: str) -> dict[str, Any]:
        """Create a child account proxy token once without retry replay."""
        return await self.client.create_account_child_account_token(euuid)

    async def update_account(self, **fields: Any) -> Account:
        """Update Linode account information with retry."""
        result: Account = await self._execute_with_retry(
            lambda: self.client.update_account(**fields)
        )
        return result

    async def update_account_settings(self, **fields: Any) -> dict[str, Any]:
        """Update Linode account settings once without retry replay."""
        return await self.client.update_account_settings(**fields)

    async def create_account_user(
        self, username: str, email: str, restricted: bool
    ) -> dict[str, Any]:
        """Create an account user once without retry replay."""
        return await self.client.create_account_user(username, email, restricted)

    async def update_account_user(
        self, current_username: str, **fields: Any
    ) -> dict[str, Any]:
        """Update an account user once without retry replay."""
        return await self.client.update_account_user(current_username, **fields)

    async def update_account_user_grants(
        self, username: str, grants: dict[str, Any]
    ) -> dict[str, Any]:
        """Update account user grants once without retry replay."""
        return await self.client.update_account_user_grants(username, grants)

    async def create_account_oauth_client(
        self, label: str, redirect_uri: str
    ) -> dict[str, Any]:
        """Create an OAuth client once without retry replay."""
        return await self.client.create_account_oauth_client(label, redirect_uri)

    async def create_account_payment_method(
        self, payment_type: str, data: dict[str, Any], is_default: bool
    ) -> dict[str, Any]:
        """Add an account payment method once without retry replay."""
        return await self.client.create_account_payment_method(
            payment_type, data, is_default
        )

    async def create_account_payment(
        self, payment_method_id: int, usd: str
    ) -> dict[str, Any]:
        """Make an account payment once without retry replay."""
        return await self.client.create_account_payment(payment_method_id, usd)

    async def add_account_promo_credit(self, promo_code: str) -> dict[str, Any]:
        """Add an account promo credit once without retry replay."""
        return await self.client.add_account_promo_credit(promo_code)

    async def create_account_service_transfer(
        self, linode_ids: list[int]
    ) -> dict[str, Any]:
        """Request an account service transfer once without retry replay."""
        return await self.client.create_account_service_transfer(linode_ids)

    async def delete_account_oauth_client(self, client_id: str) -> dict[str, Any]:
        """Delete an OAuth client once without retry replay."""
        return await self.client.delete_account_oauth_client(client_id)

    async def reset_account_oauth_client_secret(self, client_id: str) -> dict[str, Any]:
        """Reset an OAuth client secret once without retry replay."""
        return await self.client.reset_account_oauth_client_secret(client_id)

    async def delete_account_payment_method(
        self, payment_method_id: int | str
    ) -> dict[str, Any]:
        """Delete an account payment method once without retry replay."""
        return await self.client.delete_account_payment_method(payment_method_id)

    async def cancel_account(self, comments: str | None = None) -> dict[str, Any]:
        """Cancel the Linode account once without retry replay."""
        return await self.client.cancel_account(comments=comments)

    async def get_account_beta(self, beta_id: str) -> dict[str, Any]:
        """Get an enrolled Beta program on the account with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_beta, beta_id
        )
        return result

    async def get_account_child_account(self, euuid: str) -> dict[str, Any]:
        """Get a child account by EUUID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_child_account, euuid
        )
        return result

    async def get_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Get an account service transfer request by token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_service_transfer, token
        )
        return result

    async def accept_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Accept an account service transfer once without retry replay."""
        return await self.client.accept_account_service_transfer(token)

    async def delete_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Cancel an account service transfer request once without retry replay."""
        return await self.client.delete_account_service_transfer(token)

    async def get_account_invoice(self, invoice_id: int) -> dict[str, Any]:
        """Get an invoice by ID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_invoice, invoice_id
        )
        return result

    async def get_account_oauth_client(self, client_id: str) -> dict[str, Any]:
        """Get an OAuth client by client ID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_oauth_client, client_id
        )
        return result

    async def get_account_payment(self, payment_id: int) -> dict[str, Any]:
        """Get an account payment by ID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_payment, payment_id
        )
        return result

    async def get_account_payment_method(
        self, payment_method_id: int
    ) -> dict[str, Any]:
        """Get an account payment method by ID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_payment_method, payment_method_id
        )
        return result

    async def make_account_payment_method_default(
        self, payment_method_id: int
    ) -> dict[str, Any]:
        """Set a default payment method once without retry replay."""
        return await self.client.make_account_payment_method_default(payment_method_id)

    async def get_account_oauth_client_thumbnail(
        self, client_id: str
    ) -> dict[str, str]:
        """Get an OAuth client's PNG thumbnail by client ID with retry."""
        result: dict[str, str] = await self._execute_with_retry(
            self.client.get_account_oauth_client_thumbnail, client_id
        )
        return result

    async def get_account_login(self, login_id: int) -> dict[str, Any]:
        """Get an account login by ID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_login, login_id
        )
        return result

    async def get_account_user(self, username: str) -> dict[str, Any]:
        """Get an account user by username with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_user, username
        )
        return result

    async def get_account_user_grants(self, username: str) -> dict[str, Any]:
        """List grants for an account user by username with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_user_grants, username
        )
        return result

    async def get_beta(self, beta_id: str) -> dict[str, Any]:
        """Get an available Beta program with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_beta, beta_id
        )
        return result

    async def list_regions(self) -> list[Region]:
        """List Linode regions with retry."""
        result: list[Region] = await self._execute_with_retry(self.client.list_regions)
        return result

    async def get_region(self, region_id: str) -> Region:
        """Get a Linode region with retry."""
        result: Region = await self._execute_with_retry(
            self.client.get_region, region_id
        )
        return result

    async def get_region_availability(self, region_id: str) -> list[dict[str, Any]]:
        """Get compute instance availability for a region with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.get_region_availability, region_id
        )
        return result

    async def list_regions_availability(self) -> list[dict[str, Any]]:
        """List compute instance availability across regions with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_regions_availability
        )
        return result

    async def get_database_engine(
        self, engine_id: str, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """Get a Managed Databases engine with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_database_engine(
                engine_id, page=page, page_size=page_size
            )
        )
        return result

    async def get_database_type(
        self, type_id: str, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """Get a Managed Databases type with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_database_type(
                type_id, page=page, page_size=page_size
            )
        )
        return result

    async def list_types(self) -> list[InstanceType]:
        """List Linode instance types with retry."""
        result: list[InstanceType] = await self._execute_with_retry(
            self.client.list_types
        )
        return result

    async def list_database_engines(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List database engines with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_database_engines(page=page, page_size=page_size)
        )
        return result

    async def list_database_types(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List database types with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_database_types(page=page, page_size=page_size)
        )
        return result

    async def get_database_mysql_config(self) -> dict[str, Any]:
        """List MySQL Managed Database advanced parameters with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_mysql_config
        )
        return result

    async def get_database_postgresql_config(self) -> dict[str, Any]:
        """List PostgreSQL Managed Database advanced parameters with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_database_postgresql_config
        )
        return result

    async def update_postgresql_database_instance(
        self, instance_id: int, payload: dict[str, Any]
    ) -> dict[str, Any]:
        """Update a PostgreSQL Managed Database once without retry replay."""
        return await self.client.update_postgresql_database_instance(
            instance_id, payload
        )

    async def list_volumes(self) -> list[Volume]:
        """List Linode volumes with retry."""
        result: list[Volume] = await self._execute_with_retry(self.client.list_volumes)
        return result

    async def list_volume_types(self) -> list[dict[str, Any]]:
        """List Linode volume types with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_volume_types
        )
        return result

    async def get_volume(self, volume_id: int) -> Volume:
        """Get volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.get_volume, volume_id
        )
        return result

    async def list_images(self) -> list[Image]:
        """List Linode images with retry."""
        result: list[Image] = await self._execute_with_retry(self.client.list_images)
        return result

    async def list_image_sharegroups(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List image share groups with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_image_sharegroups(page=page, page_size=page_size)
        )
        return result

    async def delete_image_sharegroup(self, sharegroup_id: str) -> None:
        """Delete a single image share group without retry replay."""
        await self.client.delete_image_sharegroup(sharegroup_id)

    async def create_image_sharegroup(
        self,
        label: str,
        description: str | None = None,
        images: list[dict[str, str]] | None = None,
    ) -> dict[str, Any]:
        """Create an image share group once without retry replay."""
        return await self.client.create_image_sharegroup(
            label=label, description=description, images=images
        )

    async def get_image_sharegroup(self, sharegroup_id: str) -> dict[str, Any]:
        """Get a single image share group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_image_sharegroup(sharegroup_id)
        )
        return result

    async def update_image_sharegroup(
        self,
        sharegroup_id: str,
        *,
        label: str | None = None,
        description: str | None = None,
    ) -> dict[str, Any]:
        """Update a single image share group without retry replay."""
        if label is None and description is None:
            raise ValueError("at least one of label or description must be provided")
        return await self.client.update_image_sharegroup(
            sharegroup_id, label=label, description=description
        )

    async def list_image_sharegroup_tokens(self) -> dict[str, Any]:
        """List image share group tokens for the user with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_image_sharegroup_tokens
        )
        return result

    async def get_image_sharegroup_token(self, token_uuid: str) -> dict[str, Any]:
        """Get a single image share group token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_image_sharegroup_token(token_uuid)
        )
        return result

    async def get_image_sharegroup_by_token(self, token_uuid: str) -> dict[str, Any]:
        """Get an image share group by token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_image_sharegroup_by_token(token_uuid)
        )
        return result

    async def list_image_sharegroup_images_by_token(
        self, token_uuid: str
    ) -> dict[str, Any]:
        """List images by share group token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_image_sharegroup_images_by_token(token_uuid)
        )
        return result

    async def list_image_sharegroup_images(self, sharegroup_id: str) -> dict[str, Any]:
        """List images by share group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_image_sharegroup_images(sharegroup_id)
        )
        return result

    async def create_image_sharegroup_token(
        self, valid_for_sharegroup_uuid: str, label: str | None = None
    ) -> dict[str, Any]:
        """Create an image share group token without retry replay."""
        return await self.client.create_image_sharegroup_token(
            valid_for_sharegroup_uuid=valid_for_sharegroup_uuid, label=label
        )

    async def update_image_sharegroup_token(
        self, token_uuid: str, label: str
    ) -> dict[str, Any]:
        """Update an image share group token without retry replay."""
        return await self.client.update_image_sharegroup_token(
            token_uuid=token_uuid, label=label
        )

    async def delete_image_sharegroup_token(self, token_uuid: str) -> None:
        """Delete an image share group token without retry replay."""
        await self.client.delete_image_sharegroup_token(token_uuid=token_uuid)

    async def create_image(
        self,
        disk_id: int,
        label: str | None = None,
        description: str | None = None,
        cloud_init: bool | None = None,
        tags: list[str] | None = None,
    ) -> Image:
        """Create a private image from a Linode disk with retry."""
        result: Image = await self._execute_with_retry(
            lambda: self.client.create_image(
                disk_id=disk_id,
                label=label,
                description=description,
                cloud_init=cloud_init,
                tags=tags,
            )
        )
        return result

    # Stage 3: Extended read operations

    async def list_ssh_keys(self) -> list[SSHKey]:
        """List SSH keys with retry."""
        result: list[SSHKey] = await self._execute_with_retry(self.client.list_ssh_keys)
        return result

    async def get_ssh_key(self, ssh_key_id: int) -> SSHKey:
        """Get a specific SSH key with retry."""
        result: SSHKey = await self._execute_with_retry(
            self.client.get_ssh_key, ssh_key_id
        )
        return result

    async def list_domains(self) -> list[Domain]:
        """List domains with retry."""
        result: list[Domain] = await self._execute_with_retry(self.client.list_domains)
        return result

    async def get_domain(self, domain_id: int) -> Domain:
        """Get a specific domain with retry."""
        result: Domain = await self._execute_with_retry(
            self.client.get_domain, domain_id
        )
        return result

    async def get_domain_zone_file(self, domain_id: int) -> DomainZoneFile:
        """Get a domain zone file with retry."""
        result: DomainZoneFile = await self._execute_with_retry(
            self.client.get_domain_zone_file, domain_id
        )
        return result

    async def list_domain_records(self, domain_id: int) -> list[DomainRecord]:
        """List domain records with retry."""
        result: list[DomainRecord] = await self._execute_with_retry(
            self.client.list_domain_records, domain_id
        )
        return result

    async def get_domain_record(self, domain_id: int, record_id: int) -> DomainRecord:
        """Get a specific domain record with retry."""
        result: DomainRecord = await self._execute_with_retry(
            self.client.get_domain_record, domain_id, record_id
        )
        return result

    async def list_firewalls(self) -> list[Firewall]:
        """List firewalls with retry."""
        result: list[Firewall] = await self._execute_with_retry(
            self.client.list_firewalls
        )
        return result

    async def get_firewall(self, firewall_id: int) -> Firewall:
        """Get a specific firewall with retry."""
        result: Firewall = await self._execute_with_retry(
            self.client.get_firewall, firewall_id
        )
        return result

    async def get_firewall_settings(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List default firewall settings with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_firewall_settings, page, page_size
        )
        return result

    async def list_firewall_templates(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List firewall templates with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_firewall_templates, page, page_size
        )
        return result

    async def get_firewall_template(
        self, slug: str, page: int | None = None, page_size: int | None = None
    ) -> FirewallTemplate:
        """Get a firewall template with retry."""
        result: FirewallTemplate = await self._execute_with_retry(
            self.client.get_firewall_template, slug, page, page_size
        )
        return result

    async def get_firewall_rules(self, firewall_id: int) -> FirewallRules:
        """Get firewall rules with retry."""
        result: FirewallRules = await self._execute_with_retry(
            self.client.get_firewall_rules, firewall_id
        )
        return result

    async def get_firewall_rule_version(
        self, firewall_id: int, version: str
    ) -> FirewallRule:
        """Get firewall rule version with retry."""
        result: FirewallRule = await self._execute_with_retry(
            self.client.get_firewall_rule_version, firewall_id, version
        )
        return result

    async def get_firewall_device(
        self, firewall_id: int, device_id: int
    ) -> dict[str, Any]:
        """Get a firewall device with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_firewall_device, firewall_id, device_id
        )
        return result

    async def list_firewall_devices(
        self,
        firewall_id: int | str,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List firewall devices with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_firewall_devices(
                firewall_id, page=page, page_size=page_size
            )
        )
        return result

    async def list_firewall_rule_versions(self, firewall_id: int) -> list[Firewall]:
        """List firewall rule versions with retry."""
        result: list[Firewall] = await self._execute_with_retry(
            self.client.list_firewall_rule_versions, firewall_id
        )
        return result

    async def list_vlans(self) -> list[dict[str, Any]]:
        """List VLANs with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_vlans
        )
        return result

    async def delete_vlan(self, region_id: str, label: str) -> None:
        """Delete VLAN with retry."""
        await self._execute_with_retry(self.client.delete_vlan, region_id, label)

    async def share_ipv4s(self, ips: list[str], linode_id: int) -> dict[str, Any]:
        """Share IPv4s with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.share_ipv4s, ips, linode_id
        )
        return result

    async def assign_ipv4s(
        self, region: str, assignments: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Assign IPv4s without replay retry."""
        return await self.client.assign_ipv4s(region, assignments)

    async def list_account_availability(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List available Linode services for the account with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_account_availability(
                page=page, page_size=page_size
            )
        )
        return result

    async def get_account_availability(self, region_id: str) -> dict[str, Any]:
        """Get available Linode services in a region with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_availability, region_id
        )
        return result

    async def list_tags(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List account tags with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_tags(page=page, page_size=page_size)
        )
        return result

    async def list_tagged_objects(
        self, tag_label: str, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List objects assigned to a tag with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_tagged_objects(
                tag_label, page=page, page_size=page_size
            )
        )
        return result

    async def create_tag(
        self,
        label: str,
        domains: list[int] | None = None,
        linodes: list[int] | None = None,
        nodebalancers: list[int] | None = None,
        volumes: list[int] | None = None,
    ) -> dict[str, Any]:
        """Create tag with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.create_tag(
                label,
                domains=domains,
                linodes=linodes,
                nodebalancers=nodebalancers,
                volumes=volumes,
            )
        )
        return result

    async def delete_tag(self, tag_label: str) -> None:
        """Delete tag with retry."""
        await self._execute_with_retry(self.client.delete_tag, tag_label)

    async def create_support_ticket(
        self,
        summary: str,
        description: str,
        *,
        bucket: str | None = None,
        database_id: int | None = None,
        domain_id: int | None = None,
        firewall_id: int | None = None,
        linode_id: int | None = None,
        lkecluster_id: int | None = None,
        longviewclient_id: int | None = None,
        managed_issue: bool | None = None,
        nodebalancer_id: int | None = None,
        region: str | None = None,
        severity: int | None = None,
        vlan: str | None = None,
        volume_id: int | None = None,
        vpc_id: int | None = None,
    ) -> dict[str, Any]:
        """Create a support ticket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.create_support_ticket(
                summary,
                description,
                bucket=bucket,
                database_id=database_id,
                domain_id=domain_id,
                firewall_id=firewall_id,
                linode_id=linode_id,
                lkecluster_id=lkecluster_id,
                longviewclient_id=longviewclient_id,
                managed_issue=managed_issue,
                nodebalancer_id=nodebalancer_id,
                region=region,
                severity=severity,
                vlan=vlan,
                volume_id=volume_id,
                vpc_id=vpc_id,
            )
        )
        return result

    async def get_managed_stats(self) -> dict[str, Any]:
        """List Managed statistics with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_managed_stats
        )
        return result

    async def get_support_ticket(self, ticket_id: int) -> dict[str, Any]:
        """Get a support ticket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_support_ticket, ticket_id
        )
        return result

    async def list_support_tickets(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List support tickets with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_support_tickets(page=page, page_size=page_size)
        )
        return result

    async def list_support_ticket_replies(
        self, ticket_id: int, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List support ticket replies with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_support_ticket_replies(
                ticket_id, page=page, page_size=page_size
            )
        )
        return result

    async def create_support_ticket_reply(
        self, ticket_id: int, description: str
    ) -> dict[str, Any]:
        """Create a support ticket reply with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_support_ticket_reply, ticket_id, description
        )
        return result

    async def close_support_ticket(self, ticket_id: int) -> dict[str, Any]:
        """Close a support ticket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.close_support_ticket, ticket_id
        )
        return result

    async def create_support_ticket_attachment(
        self, ticket_id: int, file: str
    ) -> dict[str, Any]:
        """Create a support ticket attachment with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_support_ticket_attachment, ticket_id, file
        )
        return result

    async def list_nodebalancers(self) -> list[NodeBalancer]:
        """List NodeBalancers with retry."""
        result: list[NodeBalancer] = await self._execute_with_retry(
            self.client.list_nodebalancers
        )
        return result

    async def list_nodebalancer_types(self) -> list[dict[str, Any]]:
        """List NodeBalancer types with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_nodebalancer_types
        )
        return result

    async def get_nodebalancer(self, nodebalancer_id: int) -> NodeBalancer:
        """Get a specific NodeBalancer with retry."""
        result: NodeBalancer = await self._execute_with_retry(
            self.client.get_nodebalancer, nodebalancer_id
        )
        return result

    async def get_nodebalancer_stats(self, nodebalancer_id: int) -> dict[str, Any]:
        """Get NodeBalancer statistics with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_nodebalancer_stats, nodebalancer_id
        )
        return result

    async def get_nodebalancer_vpc_config(
        self, nodebalancer_id: int, vpc_config_id: int
    ) -> dict[str, Any]:
        """Get a NodeBalancer VPC configuration with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_nodebalancer_vpc_config,
            nodebalancer_id,
            vpc_config_id,
        )
        return result

    async def list_nodebalancer_vpc_configs(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List VPC configurations for a NodeBalancer with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_nodebalancer_vpc_configs(
                nodebalancer_id, page=page, page_size=page_size
            )
        )
        return result

    async def update_nodebalancer_firewalls(
        self,
        nodebalancer_id: int,
        firewall_ids: list[int],
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """Update NodeBalancer firewall assignments without replay retry."""
        return await self.client.update_nodebalancer_firewalls(
            nodebalancer_id, firewall_ids, page=page, page_size=page_size
        )

    async def rebuild_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> dict[str, Any]:
        """Rebuild a NodeBalancer config without replay retry."""
        return await self.client.rebuild_nodebalancer_config(nodebalancer_id, config_id)

    async def list_nodebalancer_configs(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List NodeBalancer configs with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_nodebalancer_configs(
                nodebalancer_id, page=page, page_size=page_size
            )
        )
        return result

    async def create_nodebalancer_config(
        self, nodebalancer_id: int, fields: dict[str, Any]
    ) -> dict[str, Any]:
        """Create a NodeBalancer config without replay retry."""
        return await self.client.create_nodebalancer_config(nodebalancer_id, fields)

    async def get_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> dict[str, Any]:
        """Get a NodeBalancer config with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_nodebalancer_config,
            nodebalancer_id,
            config_id,
        )
        return result

    async def update_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int, fields: dict[str, Any]
    ) -> dict[str, Any]:
        """Update a NodeBalancer config without replay retry."""
        return await self.client.update_nodebalancer_config(
            nodebalancer_id, config_id, fields
        )

    async def delete_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> None:
        """Delete a NodeBalancer config without replay retry."""
        return await self.client.delete_nodebalancer_config(nodebalancer_id, config_id)

    async def list_nodebalancer_config_nodes(
        self,
        nodebalancer_id: int,
        config_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List nodes in a NodeBalancer config with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_nodebalancer_config_nodes(
                nodebalancer_id, config_id, page=page, page_size=page_size
            )
        )
        return result

    async def create_nodebalancer_config_node(
        self, nodebalancer_id: int, config_id: int, fields: dict[str, Any]
    ) -> dict[str, Any]:
        """Create a NodeBalancer config node without replay retry."""
        return await self.client.create_nodebalancer_config_node(
            nodebalancer_id, config_id, fields
        )

    async def update_nodebalancer_config_node(
        self,
        nodebalancer_id: int,
        config_id: int,
        node_id: int,
        fields: dict[str, Any],
    ) -> dict[str, Any]:
        """Update a NodeBalancer config node without replay retry."""
        return await self.client.update_nodebalancer_config_node(
            nodebalancer_id, config_id, node_id, fields
        )

    async def delete_nodebalancer_config_node(
        self, nodebalancer_id: int, config_id: int, node_id: int
    ) -> None:
        """Delete a NodeBalancer config node without replay retry."""
        return await self.client.delete_nodebalancer_config_node(
            nodebalancer_id, config_id, node_id
        )

    async def get_nodebalancer_config_node(
        self, nodebalancer_id: int, config_id: int, node_id: int
    ) -> dict[str, Any]:
        """Get a NodeBalancer config node with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_nodebalancer_config_node,
            nodebalancer_id,
            config_id,
            node_id,
        )
        return result

    async def list_nodebalancer_firewalls(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List firewalls assigned to a NodeBalancer with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_nodebalancer_firewalls(
                nodebalancer_id, page=page, page_size=page_size
            )
        )
        return result

    async def list_stackscripts(self) -> list[StackScript]:
        """List StackScripts with retry."""
        result: list[StackScript] = await self._execute_with_retry(
            self.client.list_stackscripts
        )
        return result

    async def create_stackscript(
        self,
        label: str,
        images: list[str],
        script: str,
        description: str | None = None,
        is_public: bool | None = None,
        rev_note: str | None = None,
    ) -> StackScript:
        """Create a StackScript with retry."""
        result: StackScript = await self._execute_with_retry(
            lambda: self.client.create_stackscript(
                label=label,
                images=images,
                script=script,
                description=description,
                is_public=is_public,
                rev_note=rev_note,
            )
        )
        return result

    # Phase 1: Object Storage read operations with retry

    async def list_object_storage_buckets(self) -> list[dict[str, Any]]:
        """List Object Storage buckets with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_buckets
        )
        return result

    async def list_object_storage_buckets_for_region(
        self, region_id: str
    ) -> list[dict[str, Any]]:
        """List Object Storage buckets in a region with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_buckets_for_region, region_id
        )
        return result

    async def get_object_storage_bucket(
        self, region: str, label: str
    ) -> dict[str, Any]:
        """Get a specific Object Storage bucket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_bucket, region, label
        )
        return result

    async def list_object_storage_bucket_contents(
        self, region: str, label: str, params: dict[str, str] | None = None
    ) -> dict[str, Any]:
        """List contents of an Object Storage bucket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_object_storage_bucket_contents, region, label, params
        )
        return result

    async def list_object_storage_clusters(self) -> list[dict[str, Any]]:
        """List Object Storage clusters with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_clusters
        )
        return result

    async def get_object_storage_cluster(self, cluster_id: str) -> dict[str, Any]:
        """Get a specific Object Storage cluster with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_cluster, cluster_id
        )
        return result

    async def list_object_storage_types(self) -> list[dict[str, Any]]:
        """List Object Storage types/pricing with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_types
        )
        return result

    async def list_object_storage_endpoints(self) -> list[dict[str, Any]]:
        """List Object Storage endpoints with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_endpoints
        )
        return result

    async def list_object_storage_keys(self) -> list[dict[str, Any]]:
        """List Object Storage access keys with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_keys
        )
        return result

    async def get_object_storage_key(self, key_id: int) -> dict[str, Any]:
        """Get a specific Object Storage access key with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_key, key_id
        )
        return result

    async def get_object_storage_transfer(self) -> dict[str, Any]:
        """Get Object Storage transfer usage with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_transfer
        )
        return result

    async def get_network_transfer_prices(self) -> dict[str, Any]:
        """Get network transfer prices with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_network_transfer_prices
        )
        return result

    async def cancel_object_storage(self) -> dict[str, Any]:
        """Cancel Object Storage service for the account without retry replay."""
        result: dict[str, Any] = await self.client.cancel_object_storage()
        return result

    async def list_object_storage_quotas(self) -> list[dict[str, Any]]:
        """List Object Storage quotas with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_object_storage_quotas
        )
        return result

    async def get_object_storage_quota(self, obj_quota_id: str) -> dict[str, Any]:
        """Get a single Object Storage quota with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_quota, obj_quota_id
        )
        return result

    async def get_object_storage_quota_usage(
        self, obj_quota_id: int | str
    ) -> dict[str, Any]:
        """Get Object Storage quota usage with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_quota_usage, obj_quota_id
        )
        return result

    async def get_object_storage_bucket_access(
        self, region: str, label: str
    ) -> dict[str, Any]:
        """Get bucket ACL/CORS settings with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_bucket_access, region, label
        )
        return result

    # Stage 5 Phase 3: Object Storage write operations with retry

    async def create_object_storage_bucket(
        self,
        label: str,
        region: str,
        acl: str | None = None,
        cors_enabled: bool | None = None,
    ) -> dict[str, Any]:
        """Create Object Storage bucket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_object_storage_bucket,
            label,
            region,
            acl,
            cors_enabled,
        )
        return result

    async def delete_object_storage_bucket(self, region: str, label: str) -> None:
        """Delete Object Storage bucket with retry."""
        await self._execute_with_retry(
            self.client.delete_object_storage_bucket,
            region,
            label,
        )

    async def update_object_storage_bucket_access(
        self,
        region: str,
        label: str,
        acl: str | None = None,
        cors_enabled: bool | None = None,
    ) -> None:
        """Update bucket access settings with retry."""
        await self._execute_with_retry(
            self.client.update_object_storage_bucket_access,
            region,
            label,
            acl,
            cors_enabled,
        )

    async def allow_object_storage_bucket_access(
        self,
        region: str,
        label: str,
        acl: str | None = None,
        cors_enabled: bool | None = None,
    ) -> dict[str, Any]:
        """Allow bucket access settings with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.allow_object_storage_bucket_access,
            region,
            label,
            acl,
            cors_enabled,
        )
        return result

    # Stage 5 Phase 4: Object Storage access key write operations with retry

    async def create_object_storage_key(
        self,
        label: str,
        bucket_access: list[dict[str, str]] | None = None,
    ) -> dict[str, Any]:
        """Create Object Storage access key with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_object_storage_key,
            label,
            bucket_access,
        )
        return result

    async def update_object_storage_key(
        self,
        key_id: int,
        label: str | None = None,
        bucket_access: list[dict[str, str]] | None = None,
    ) -> None:
        """Update Object Storage access key with retry."""
        await self._execute_with_retry(
            self.client.update_object_storage_key,
            key_id,
            label,
            bucket_access,
        )

    async def delete_object_storage_key(self, key_id: int) -> None:
        """Delete Object Storage access key with retry."""
        await self._execute_with_retry(self.client.delete_object_storage_key, key_id)

    async def create_presigned_url(
        self,
        region: str,
        label: str,
        name: str,
        method: str,
        expires_in: int = 3600,
    ) -> dict[str, Any]:
        """Generate presigned URL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_presigned_url,
            region,
            label,
            name,
            method,
            expires_in,
        )
        return result

    async def get_object_acl(
        self, region: str, label: str, name: str
    ) -> dict[str, Any]:
        """Get object ACL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_acl, region, label, name
        )
        return result

    async def update_object_acl(
        self, region: str, label: str, name: str, acl: str
    ) -> dict[str, Any]:
        """Update object ACL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_object_acl, region, label, name, acl
        )
        return result

    async def get_bucket_ssl(self, region: str, label: str) -> dict[str, Any]:
        """Get bucket SSL status with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_bucket_ssl, region, label
        )
        return result

    async def upload_bucket_ssl(
        self, region: str, label: str, certificate: str, private_key: str
    ) -> dict[str, Any]:
        """Upload bucket SSL certificate with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.upload_bucket_ssl, region, label, certificate, private_key
        )
        return result

    async def delete_bucket_ssl(self, region: str, label: str) -> None:
        """Delete bucket SSL certificate with retry."""
        await self._execute_with_retry(self.client.delete_bucket_ssl, region, label)

    # Stage 4: Write operations with retry

    async def create_ssh_key(self, label: str, ssh_key: str) -> SSHKey:
        """Create SSH key with retry."""
        result: SSHKey = await self._execute_with_retry(
            self.client.create_ssh_key, label, ssh_key
        )
        return result

    async def update_ssh_key(self, ssh_key_id: int, label: str) -> SSHKey:
        """Update SSH key with retry."""
        result: SSHKey = await self._execute_with_retry(
            self.client.update_ssh_key, ssh_key_id, label
        )
        return result

    async def delete_ssh_key(self, ssh_key_id: int) -> None:
        """Delete SSH key with retry."""
        await self._execute_with_retry(self.client.delete_ssh_key, ssh_key_id)

    async def create_profile_tfa_secret(self) -> dict[str, Any]:
        """Create a profile two-factor authentication secret with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_profile_tfa_secret
        )
        return result

    async def confirm_profile_tfa_enable(
        self, tfa_code: str | None = None
    ) -> dict[str, Any]:
        """Confirm profile two-factor authentication enablement with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.confirm_profile_tfa_enable, tfa_code
        )
        return result

    async def disable_profile_tfa(self) -> dict[str, Any]:
        """Disable profile two-factor authentication with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.disable_profile_tfa
        )
        return result

    async def send_profile_phone_number_verification(
        self, iso_code: str, phone_number: str
    ) -> dict[str, Any]:
        """Send a profile phone number verification code with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.send_profile_phone_number_verification, iso_code, phone_number
        )
        return result

    async def verify_profile_phone_number(self, otp_code: str) -> dict[str, Any]:
        """Verify a profile phone number with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.verify_profile_phone_number, otp_code
        )
        return result

    async def delete_profile_phone_number(self) -> dict[str, Any]:
        """Delete the profile phone number with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.delete_profile_phone_number
        )
        return result

    async def list_profile_security_questions(self) -> dict[str, Any]:
        """List available profile security questions with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_profile_security_questions
        )
        return result

    async def answer_profile_security_questions(
        self, security_questions: list[dict[str, Any]]
    ) -> dict[str, Any]:
        """Answer profile security questions with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.answer_profile_security_questions, security_questions
        )
        return result

    async def create_profile_token(
        self,
        *,
        expiry: str | None = None,
        label: str | None = None,
        scopes: str | None = None,
    ) -> dict[str, Any]:
        """Create a profile token with retry."""

        async def _create_profile_token() -> dict[str, Any]:
            return await self.client.create_profile_token(
                expiry=expiry, label=label, scopes=scopes
            )

        result: dict[str, Any] = await self._execute_with_retry(_create_profile_token)
        return result

    async def list_profile_tokens(self) -> list[dict[str, Any]]:
        """List profile tokens with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_profile_tokens
        )
        return result

    async def get_profile_token(self, token_id: int) -> dict[str, Any]:
        """Get a profile token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_token, token_id
        )
        return result

    async def list_profile_logins(self) -> list[dict[str, Any]]:
        """List profile logins with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_profile_logins
        )
        return result

    async def get_profile_login(self, login_id: int) -> dict[str, Any]:
        """Get a profile login with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_login, login_id
        )
        return result

    async def list_profile_devices(self) -> list[dict[str, Any]]:
        """List profile trusted devices with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_profile_devices
        )
        return result

    async def get_profile_device(self, device_id: int) -> dict[str, Any]:
        """Get a profile trusted device with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_device, device_id
        )
        return result

    async def list_profile_apps(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List profile OAuth app authorizations with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_profile_apps(page=page, page_size=page_size)
        )
        return result

    async def get_profile_app(self, app_id: int) -> dict[str, Any]:
        """Get a profile OAuth app authorization with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_app, app_id
        )
        return result

    async def delete_profile_app(self, app_id: int) -> None:
        """Revoke profile OAuth app access with retry."""
        await self._execute_with_retry(self.client.delete_profile_app, app_id)

    async def delete_profile_device(self, device_id: int) -> None:
        """Revoke a profile trusted device with retry."""
        await self._execute_with_retry(self.client.delete_profile_device, device_id)

    async def delete_profile_token(self, token_id: int) -> None:
        """Revoke a profile token with retry."""
        await self._execute_with_retry(self.client.delete_profile_token, token_id)

    async def update_profile_token(self, token_id: int, label: str) -> dict[str, Any]:
        """Update a profile token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_profile_token, token_id, label
        )
        return result

    async def list_monitor_services(self) -> dict[str, Any]:
        """List monitor services with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_services
        )
        return result

    async def list_monitor_dashboards(self) -> dict[str, Any]:
        """List monitor dashboards with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_dashboards
        )
        return result

    async def list_monitor_alert_definitions(self) -> dict[str, Any]:
        """List monitor alert definitions with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_alert_definitions
        )
        return result

    async def list_monitor_alert_channels(self) -> dict[str, Any]:
        """List monitor alert channels with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_alert_channels
        )
        return result

    async def get_monitor_service(self, service_type: str) -> dict[str, Any]:
        """Get monitor service details with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_monitor_service, service_type
        )
        return result

    async def create_monitor_service_token(
        self, service_type: str, entity_ids: list[int]
    ) -> dict[str, Any]:
        """Create a monitor service token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_monitor_service_token, service_type, entity_ids
        )
        return result

    async def update_monitor_alert_definition(
        self,
        service_type: str,
        alert_id: int,
        **fields: Any,
    ) -> dict[str, Any]:
        """Update a monitor alert definition without retrying the PUT."""
        return await self.client.update_monitor_alert_definition(
            service_type, alert_id, **fields
        )

    async def list_monitor_service_dashboards(
        self, service_type: str
    ) -> dict[str, Any]:
        """List monitor service dashboards with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_service_dashboards, service_type
        )
        return result

    async def get_monitor_dashboard(self, dashboard_id: int) -> dict[str, Any]:
        """Get a monitor dashboard with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_monitor_dashboard, dashboard_id
        )
        return result

    async def read_monitor_service_metrics(self, service_type: str) -> dict[str, Any]:
        """Read monitor service metrics with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.read_monitor_service_metrics, service_type
        )
        return result

    async def list_monitor_service_metric_definitions(
        self, service_type: str
    ) -> dict[str, Any]:
        """List monitor service metric definitions with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_service_metric_definitions, service_type
        )
        return result

    async def list_monitor_service_alert_definitions(
        self, service_type: str
    ) -> dict[str, Any]:
        """List monitor service alert definitions with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_monitor_service_alert_definitions, service_type
        )
        return result

    async def create_monitor_service_alert_definition(
        self,
        service_type: str,
        *,
        label: str,
        severity: int,
        rule_criteria: dict[str, Any],
        trigger_conditions: dict[str, Any],
        channel_ids: list[int],
        description: str | None = None,
        entity_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        """Create a monitor service alert definition without retry replay."""
        result: dict[
            str, Any
        ] = await self.client.create_monitor_service_alert_definition(
            service_type,
            label=label,
            severity=severity,
            rule_criteria=rule_criteria,
            trigger_conditions=trigger_conditions,
            channel_ids=channel_ids,
            description=description,
            entity_ids=entity_ids,
        )
        return result

    async def get_monitor_service_alert_definition(
        self, service_type: str, alert_id: int
    ) -> dict[str, Any]:
        """Get a monitor service alert definition with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_monitor_service_alert_definition, service_type, alert_id
        )
        return result

    async def delete_monitor_service_alert_definition(
        self, service_type: str, alert_id: int
    ) -> None:
        """Delete a monitor service alert definition without retry replay."""
        await self.client.delete_monitor_service_alert_definition(
            service_type, alert_id
        )

    async def boot_instance(
        self, instance_id: int, config_id: int | None = None
    ) -> None:
        """Boot instance with retry."""
        await self._execute_with_retry(
            self.client.boot_instance, instance_id, config_id
        )

    async def reboot_instance(
        self, instance_id: int, config_id: int | None = None
    ) -> None:
        """Reboot instance with retry."""
        await self._execute_with_retry(
            self.client.reboot_instance, instance_id, config_id
        )

    async def shutdown_instance(self, instance_id: int) -> None:
        """Shutdown instance with retry."""
        await self._execute_with_retry(self.client.shutdown_instance, instance_id)

    async def create_instance(
        self,
        region: str,
        instance_type: str,
        firewall_id: int,
        image: str | None = None,
        label: str | None = None,
        root_pass: str | None = None,
        authorized_keys: list[str] | None = None,
        authorized_users: list[str] | None = None,
        booted: bool = True,
        backups_enabled: bool = False,
        route_ipv4: bool = True,
        route_ipv6: bool = True,
        tags: list[str] | None = None,
    ) -> Instance:
        """Create instance with retry. firewall_id is required under the
        current Linode Interfaces generation.
        """
        result: Instance = await self._execute_with_retry(
            self.client.create_instance,
            region,
            instance_type,
            firewall_id,
            image,
            label,
            root_pass,
            authorized_keys,
            authorized_users,
            booted,
            backups_enabled,
            route_ipv4,
            route_ipv6,
            tags,
        )
        return result

    async def delete_instance(self, instance_id: int) -> None:
        """Delete instance with retry."""
        await self._execute_with_retry(self.client.delete_instance, instance_id)

    async def resize_instance(
        self,
        instance_id: int,
        instance_type: str,
        allow_auto_disk_resize: bool = True,
        migration_type: str = "warm",
    ) -> None:
        """Resize instance with retry."""
        await self._execute_with_retry(
            self.client.resize_instance,
            instance_id,
            instance_type,
            allow_auto_disk_resize,
            migration_type,
        )

    async def create_firewall(
        self,
        label: str,
        inbound_policy: str = "ACCEPT",
        outbound_policy: str = "ACCEPT",
    ) -> Firewall:
        """Create firewall with retry."""
        result: Firewall = await self._execute_with_retry(
            self.client.create_firewall, label, inbound_policy, outbound_policy
        )
        return result

    async def update_firewall(
        self,
        firewall_id: int,
        label: str | None = None,
        status: str | None = None,
        inbound_policy: str | None = None,
        outbound_policy: str | None = None,
    ) -> Firewall:
        """Update firewall with retry."""
        result: Firewall = await self._execute_with_retry(
            self.client.update_firewall,
            firewall_id,
            label,
            status,
            inbound_policy,
            outbound_policy,
        )
        return result

    async def delete_firewall(self, firewall_id: int) -> None:
        """Delete firewall with retry."""
        await self._execute_with_retry(self.client.delete_firewall, firewall_id)

    async def create_firewall_device(
        self,
        firewall_id: int | str,
        device_id: int,
        device_type: str,
    ) -> dict[str, Any]:
        """Create a new device for a firewall with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_firewall_device, firewall_id, device_id, device_type
        )
        return result

    async def update_firewall_rules(
        self,
        firewall_id: int,
        inbound: list[dict[str, Any]],
        outbound: list[dict[str, Any]],
    ) -> dict[str, list[dict[str, Any]]]:
        """Update firewall rules with retry."""
        result: dict[str, list[dict[str, Any]]] = await self._execute_with_retry(
            self.client.update_firewall_rules,
            firewall_id,
            inbound,
            outbound,
        )
        return result

    async def delete_firewall_device(
        self, firewall_id: int | str, device_id: int | str
    ) -> None:
        """Delete a firewall device without replay retry."""
        await self.client.delete_firewall_device(firewall_id, device_id)

    async def update_firewall_settings(
        self, default_firewall_ids: dict[str, int]
    ) -> dict[str, Any]:
        """Update default firewall settings without replaying the PUT."""
        return await self.client.update_firewall_settings(default_firewall_ids)

    async def create_domain(
        self,
        domain: str,
        domain_type: str = "master",
        soa_email: str | None = None,
        description: str | None = None,
        tags: list[str] | None = None,
    ) -> Domain:
        """Create domain with retry."""
        result: Domain = await self._execute_with_retry(
            self.client.create_domain, domain, domain_type, soa_email, description, tags
        )
        return result

    async def clone_domain(self, domain_id: int, domain: str) -> Domain:
        """Clone domain without replaying the POST."""
        return await self.client.clone_domain(domain_id, domain)

    async def import_domain(self, domain: str, remote_nameserver: str) -> Domain:
        """Import domain without replaying the POST."""
        return await self.client.import_domain(domain, remote_nameserver)

    async def update_domain(
        self,
        domain_id: int,
        domain: str | None = None,
        soa_email: str | None = None,
        description: str | None = None,
        tags: list[str] | None = None,
    ) -> Domain:
        """Update domain with retry."""
        result: Domain = await self._execute_with_retry(
            self.client.update_domain, domain_id, domain, soa_email, description, tags
        )
        return result

    async def delete_domain(self, domain_id: int) -> None:
        """Delete domain with retry."""
        await self._execute_with_retry(self.client.delete_domain, domain_id)

    async def create_domain_record(
        self,
        domain_id: int,
        record_type: str,
        name: str | None = None,
        target: str | None = None,
        priority: int | None = None,
        weight: int | None = None,
        port: int | None = None,
        ttl_sec: int | None = None,
    ) -> DomainRecord:
        """Create domain record with retry."""
        result: DomainRecord = await self._execute_with_retry(
            self.client.create_domain_record,
            domain_id,
            record_type,
            name,
            target,
            priority,
            weight,
            port,
            ttl_sec,
        )
        return result

    async def update_domain_record(
        self,
        domain_id: int,
        record_id: int,
        name: str | None = None,
        target: str | None = None,
        priority: int | None = None,
        weight: int | None = None,
        port: int | None = None,
        ttl_sec: int | None = None,
    ) -> DomainRecord:
        """Update domain record with retry."""
        result: DomainRecord = await self._execute_with_retry(
            self.client.update_domain_record,
            domain_id,
            record_id,
            name,
            target,
            priority,
            weight,
            port,
            ttl_sec,
        )
        return result

    async def delete_domain_record(self, domain_id: int, record_id: int) -> None:
        """Delete domain record with retry."""
        await self._execute_with_retry(
            self.client.delete_domain_record, domain_id, record_id
        )

    async def create_volume(
        self,
        label: str,
        region: str | None = None,
        linode_id: int | None = None,
        size: int = 20,
        tags: list[str] | None = None,
    ) -> Volume:
        """Create volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.create_volume, label, region, linode_id, size, tags
        )
        return result

    async def clone_volume(self, volume_id: int, label: str) -> Volume:
        """Clone volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.clone_volume, volume_id, label
        )
        return result

    async def attach_volume(
        self,
        volume_id: int,
        linode_id: int,
        config_id: int | None = None,
        persist_across_boots: bool = False,
    ) -> Volume:
        """Attach volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.attach_volume,
            volume_id,
            linode_id,
            config_id,
            persist_across_boots,
        )
        return result

    async def detach_volume(self, volume_id: int) -> None:
        """Detach volume with retry."""
        await self._execute_with_retry(self.client.detach_volume, volume_id)

    async def resize_volume(self, volume_id: int, size: int) -> Volume:
        """Resize volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.resize_volume, volume_id, size
        )
        return result

    async def delete_volume(self, volume_id: int) -> None:
        """Delete volume with retry."""
        await self._execute_with_retry(self.client.delete_volume, volume_id)

    async def update_volume(
        self,
        volume_id: int,
        label: str | None = None,
        tags: list[str] | None = None,
    ) -> Volume:
        """Update volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.update_volume, volume_id, label, tags
        )
        return result

    async def create_nodebalancer(
        self,
        region: str,
        label: str | None = None,
        client_conn_throttle: int = 0,
        tags: list[str] | None = None,
    ) -> NodeBalancer:
        """Create NodeBalancer with retry."""
        result: NodeBalancer = await self._execute_with_retry(
            self.client.create_nodebalancer, region, label, client_conn_throttle, tags
        )
        return result

    async def update_nodebalancer(
        self,
        nodebalancer_id: int,
        label: str | None = None,
        client_conn_throttle: int | None = None,
        tags: list[str] | None = None,
    ) -> NodeBalancer:
        """Update NodeBalancer with retry."""
        result: NodeBalancer = await self._execute_with_retry(
            self.client.update_nodebalancer,
            nodebalancer_id,
            label,
            client_conn_throttle,
            tags,
        )
        return result

    async def delete_nodebalancer(self, nodebalancer_id: int) -> None:
        """Delete NodeBalancer with retry."""
        await self._execute_with_retry(self.client.delete_nodebalancer, nodebalancer_id)

    # LKE operations with retry

    async def list_lke_clusters(self) -> list[dict[str, Any]]:
        """List LKE clusters with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_lke_clusters
        )
        return result

    async def get_lke_cluster(self, cluster_id: int) -> dict[str, Any]:
        """Get a specific LKE cluster with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_cluster, cluster_id
        )
        return result

    async def create_lke_cluster(
        self,
        label: str,
        region: str,
        k8s_version: str,
        node_pools: list[dict[str, Any]],
        tags: list[str] | None = None,
        control_plane: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Create LKE cluster with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_lke_cluster,
            label,
            region,
            k8s_version,
            node_pools,
            tags,
            control_plane,
        )
        return result

    async def update_lke_cluster(
        self,
        cluster_id: int,
        label: str | None = None,
        k8s_version: str | None = None,
        tags: list[str] | None = None,
        control_plane: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Update LKE cluster with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_lke_cluster,
            cluster_id,
            label,
            k8s_version,
            tags,
            control_plane,
        )
        return result

    async def delete_lke_cluster(self, cluster_id: int) -> None:
        """Delete LKE cluster with retry."""
        await self._execute_with_retry(self.client.delete_lke_cluster, cluster_id)

    async def recycle_lke_cluster(self, cluster_id: int) -> None:
        """Recycle LKE cluster nodes with retry."""
        await self._execute_with_retry(self.client.recycle_lke_cluster, cluster_id)

    async def regenerate_lke_cluster(self, cluster_id: int) -> None:
        """Regenerate LKE cluster service token with retry."""
        await self._execute_with_retry(self.client.regenerate_lke_cluster, cluster_id)

    async def list_lke_node_pools(self, cluster_id: int) -> list[dict[str, Any]]:
        """List LKE node pools with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_lke_node_pools, cluster_id
        )
        return result

    async def get_lke_node_pool(self, cluster_id: int, pool_id: int) -> dict[str, Any]:
        """Get a specific LKE node pool with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_node_pool, cluster_id, pool_id
        )
        return result

    async def create_lke_node_pool(
        self,
        cluster_id: int,
        node_type: str,
        count: int,
        autoscaler: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> dict[str, Any]:
        """Create LKE node pool with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_lke_node_pool,
            cluster_id,
            node_type,
            count,
            autoscaler,
            tags,
        )
        return result

    async def update_lke_node_pool(
        self,
        cluster_id: int,
        pool_id: int,
        count: int | None = None,
        autoscaler: dict[str, Any] | None = None,
        tags: list[str] | None = None,
    ) -> dict[str, Any]:
        """Update LKE node pool with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_lke_node_pool,
            cluster_id,
            pool_id,
            count,
            autoscaler,
            tags,
        )
        return result

    async def delete_lke_node_pool(self, cluster_id: int, pool_id: int) -> None:
        """Delete LKE node pool with retry."""
        await self._execute_with_retry(
            self.client.delete_lke_node_pool, cluster_id, pool_id
        )

    async def recycle_lke_node_pool(self, cluster_id: int, pool_id: int) -> None:
        """Recycle LKE node pool with retry."""
        await self._execute_with_retry(
            self.client.recycle_lke_node_pool, cluster_id, pool_id
        )

    async def get_lke_node(self, cluster_id: int, node_id: str) -> dict[str, Any]:
        """Get a specific LKE node with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_node, cluster_id, node_id
        )
        return result

    async def delete_lke_node(self, cluster_id: int, node_id: str) -> None:
        """Delete LKE node with retry."""
        await self._execute_with_retry(self.client.delete_lke_node, cluster_id, node_id)

    async def recycle_lke_node(self, cluster_id: int, node_id: str) -> None:
        """Recycle LKE node with retry."""
        await self._execute_with_retry(
            self.client.recycle_lke_node, cluster_id, node_id
        )

    async def get_lke_kubeconfig(self, cluster_id: int) -> dict[str, Any]:
        """Get LKE kubeconfig with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_kubeconfig, cluster_id
        )
        return result

    async def delete_lke_kubeconfig(self, cluster_id: int) -> None:
        """Delete LKE kubeconfig with retry."""
        await self._execute_with_retry(self.client.delete_lke_kubeconfig, cluster_id)

    async def get_lke_dashboard(self, cluster_id: int) -> dict[str, Any]:
        """Get LKE dashboard URL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_dashboard, cluster_id
        )
        return result

    async def list_lke_api_endpoints(self, cluster_id: int) -> list[dict[str, Any]]:
        """List LKE API endpoints with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_lke_api_endpoints, cluster_id
        )
        return result

    async def delete_lke_service_token(self, cluster_id: int) -> None:
        """Delete LKE service token with retry."""
        await self._execute_with_retry(self.client.delete_lke_service_token, cluster_id)

    async def get_lke_control_plane_acl(self, cluster_id: int) -> dict[str, Any]:
        """Get LKE control plane ACL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_control_plane_acl, cluster_id
        )
        return result

    async def update_lke_control_plane_acl(
        self,
        cluster_id: int,
        acl: dict[str, Any],
    ) -> dict[str, Any]:
        """Update LKE control plane ACL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_lke_control_plane_acl, cluster_id, acl
        )
        return result

    async def delete_lke_control_plane_acl(self, cluster_id: int) -> None:
        """Delete LKE control plane ACL with retry."""
        await self._execute_with_retry(
            self.client.delete_lke_control_plane_acl, cluster_id
        )

    async def list_lke_versions(self) -> list[dict[str, Any]]:
        """List LKE versions with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_lke_versions
        )
        return result

    async def get_lke_version(self, version_id: str) -> dict[str, Any]:
        """Get a specific LKE version with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_version, version_id
        )
        return result

    async def list_lke_types(self) -> list[dict[str, Any]]:
        """List LKE node types with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_lke_types
        )
        return result

    async def list_lke_tier_versions(self) -> list[dict[str, Any]]:
        """List LKE tier versions with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_lke_tier_versions
        )
        return result

    # VPC operations with retry

    async def list_vpcs(self) -> list[dict[str, Any]]:
        """List VPCs with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_vpcs
        )
        return result

    async def get_vpc(self, vpc_id: int) -> dict[str, Any]:
        """Get a specific VPC with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_vpc, vpc_id
        )
        return result

    async def create_vpc(
        self,
        label: str,
        region: str,
        description: str | None = None,
        subnets: list[dict[str, Any]] | None = None,
    ) -> dict[str, Any]:
        """Create VPC with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_vpc,
            label,
            region,
            description,
            subnets,
        )
        return result

    async def update_vpc(
        self,
        vpc_id: int,
        label: str | None = None,
        description: str | None = None,
    ) -> dict[str, Any]:
        """Update VPC with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_vpc,
            vpc_id,
            label,
            description,
        )
        return result

    async def delete_vpc(self, vpc_id: int) -> None:
        """Delete VPC with retry."""
        await self._execute_with_retry(self.client.delete_vpc, vpc_id)

    async def list_vpc_ips(self) -> list[dict[str, Any]]:
        """List all VPC IP addresses with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_vpc_ips
        )
        return result

    async def list_vpc_ip(self, vpc_id: int) -> list[dict[str, Any]]:
        """List IPs for a specific VPC with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_vpc_ip, vpc_id
        )
        return result

    async def list_vpc_subnets(self, vpc_id: int) -> list[dict[str, Any]]:
        """List VPC subnets with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_vpc_subnets, vpc_id
        )
        return result

    async def get_vpc_subnet(self, vpc_id: int, subnet_id: int) -> dict[str, Any]:
        """Get a specific VPC subnet with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_vpc_subnet, vpc_id, subnet_id
        )
        return result

    async def create_vpc_subnet(
        self,
        vpc_id: int,
        label: str,
        ipv4: str,
    ) -> dict[str, Any]:
        """Create VPC subnet with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_vpc_subnet,
            vpc_id,
            label,
            ipv4,
        )
        return result

    async def update_vpc_subnet(
        self,
        vpc_id: int,
        subnet_id: int,
        label: str,
    ) -> dict[str, Any]:
        """Update VPC subnet with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_vpc_subnet,
            vpc_id,
            subnet_id,
            label,
        )
        return result

    async def delete_vpc_subnet(self, vpc_id: int, subnet_id: int) -> None:
        """Delete VPC subnet with retry."""
        await self._execute_with_retry(self.client.delete_vpc_subnet, vpc_id, subnet_id)

    async def create_ipv6_range(
        self,
        prefix_length: int,
        linode_id: int | None = None,
        route_target: str | None = None,
    ) -> dict[str, Any]:
        """Create IPv6 range with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_ipv6_range,
            prefix_length,
            linode_id,
            route_target,
        )
        return result

    async def list_ipv6_ranges(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List IPv6 ranges with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_ipv6_ranges(page=page, page_size=page_size)
        )
        return result

    async def list_ipv6_pools(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List IPv6 pools with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_ipv6_pools(page=page, page_size=page_size)
        )
        return result

    async def list_placement_groups(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List placement groups with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_placement_groups(page=page, page_size=page_size)
        )
        return result

    async def create_placement_group(
        self,
        label: str,
        region: str,
        placement_group_type: str,
        placement_group_policy: str,
    ) -> dict[str, Any]:
        """Create placement group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_placement_group,
            label,
            region,
            placement_group_type,
            placement_group_policy,
        )
        return result

    async def get_placement_group(self, group_id: int) -> dict[str, Any]:
        """Get placement group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_placement_group, group_id
        )
        return result

    async def assign_placement_group(
        self, group_id: int, linodes: list[int]
    ) -> dict[str, Any]:
        """Assign Linodes to a placement group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.assign_placement_group, group_id, linodes
        )
        return result

    async def unassign_placement_group(
        self, group_id: int, linodes: list[int]
    ) -> dict[str, Any]:
        """Unassign Linodes from a placement group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.unassign_placement_group, group_id, linodes
        )
        return result

    async def delete_placement_group(self, group_id: int) -> None:
        """Delete placement group with retry."""
        await self._execute_with_retry(self.client.delete_placement_group, group_id)

    async def update_placement_group(self, group_id: int, label: str) -> dict[str, Any]:
        """Update placement group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_placement_group, group_id, label
        )
        return result

    async def get_ipv6_range(self, ipv6_range: str) -> dict[str, Any]:
        """Get IPv6 range with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_ipv6_range, ipv6_range
        )
        return result

    async def delete_ipv6_range(self, ipv6_range: str) -> None:
        """Delete IPv6 range with retry."""
        await self._execute_with_retry(self.client.delete_ipv6_range, ipv6_range)

    # ── Instance Backups (retry wrappers) ──

    async def list_instance_backups(self, instance_id: int) -> dict[str, Any]:
        """List instance backups with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_instance_backups, instance_id
        )
        return result

    async def get_instance_backup(
        self, instance_id: int, backup_id: int
    ) -> dict[str, Any]:
        """Get instance backup with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_backup,
            instance_id,
            backup_id,
        )
        return result

    async def create_instance_backup(
        self, instance_id: int, label: str | None = None
    ) -> dict[str, Any]:
        """Create instance backup with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_instance_backup,
            instance_id,
            label,
        )
        return result

    async def restore_instance_backup(
        self,
        instance_id: int,
        backup_id: int,
        linode_id: int,
        overwrite: bool = False,
    ) -> None:
        """Restore instance backup with retry."""
        await self._execute_with_retry(
            self.client.restore_instance_backup,
            instance_id,
            backup_id,
            linode_id,
            overwrite,
        )

    async def enable_instance_backups(self, instance_id: int) -> None:
        """Enable instance backups with retry."""
        await self._execute_with_retry(self.client.enable_instance_backups, instance_id)

    async def cancel_instance_backups(self, instance_id: int) -> None:
        """Cancel instance backups with retry."""
        await self._execute_with_retry(self.client.cancel_instance_backups, instance_id)

    # ── Instance Disks (retry wrappers) ──

    async def list_instance_disks(self, instance_id: int) -> list[dict[str, Any]]:
        """List instance disks with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_instance_disks, instance_id
        )
        return result

    async def get_instance_disk(self, instance_id: int, disk_id: int) -> dict[str, Any]:
        """Get instance disk with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_disk,
            instance_id,
            disk_id,
        )
        return result

    async def create_instance_disk(
        self,
        instance_id: int,
        label: str,
        size: int,
        filesystem: str | None = None,
        image: str | None = None,
        root_pass: str | None = None,
    ) -> dict[str, Any]:
        """Create instance disk with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.create_instance_disk,
            instance_id,
            label,
            size,
            filesystem,
            image,
            root_pass,
        )
        return result

    async def update_instance_disk(
        self,
        instance_id: int,
        disk_id: int,
        label: str | None = None,
    ) -> dict[str, Any]:
        """Update instance disk with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_instance_disk,
            instance_id,
            disk_id,
            label,
        )
        return result

    async def delete_instance_disk(self, instance_id: int, disk_id: int) -> None:
        """Delete instance disk with retry."""
        await self._execute_with_retry(
            self.client.delete_instance_disk,
            instance_id,
            disk_id,
        )

    async def clone_instance_disk(
        self, instance_id: int, disk_id: int
    ) -> dict[str, Any]:
        """Clone instance disk with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.clone_instance_disk,
            instance_id,
            disk_id,
        )
        return result

    async def resize_instance_disk(
        self, instance_id: int, disk_id: int, size: int
    ) -> None:
        """Resize instance disk with retry."""
        await self._execute_with_retry(
            self.client.resize_instance_disk,
            instance_id,
            disk_id,
            size,
        )

    # ── Instance IPs (retry wrappers) ──

    async def list_instance_ips(self, instance_id: int) -> dict[str, Any]:
        """List instance IPs with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.list_instance_ips, instance_id
        )
        return result

    async def get_instance_ip(self, instance_id: int, address: str) -> dict[str, Any]:
        """Get instance IP with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_ip, instance_id, address
        )
        return result

    async def get_networking_ip(self, address: str) -> dict[str, Any]:
        """Get networking IP with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_networking_ip, address
        )
        return result

    async def allocate_instance_ip(
        self,
        instance_id: int,
        ip_type: str,
        public: bool = True,
    ) -> dict[str, Any]:
        """Allocate instance IP with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.allocate_instance_ip,
            instance_id,
            ip_type,
            public,
        )
        return result

    async def update_instance_ip(
        self,
        instance_id: int,
        address: str,
        rdns: str | None,
    ) -> dict[str, Any]:
        """Update instance IP RDNS with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_instance_ip,
            instance_id,
            address,
            rdns,
        )
        return result

    async def list_networking_ips(
        self, skip_ipv6_rdns: bool = False
    ) -> list[dict[str, Any]]:
        """List networking IPs with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_networking_ips, skip_ipv6_rdns
        )
        return result

    async def update_networking_ip(
        self,
        address: str,
        rdns: str | None,
    ) -> dict[str, Any]:
        """Update networking IP RDNS with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.update_networking_ip,
            address,
            rdns,
        )
        return result

    async def allocate_networking_ip(
        self,
        linode_id: int,
        ip_type: str,
        public: bool = True,
    ) -> dict[str, Any]:
        """Allocate networking IP with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.allocate_networking_ip,
            linode_id,
            ip_type,
            public,
        )
        return result

    async def delete_instance_ip(self, instance_id: int, address: str) -> None:
        """Delete instance IP with retry."""
        await self._execute_with_retry(
            self.client.delete_instance_ip,
            instance_id,
            address,
        )

    # ── Instance Actions (retry wrappers) ──

    async def clone_instance(
        self,
        instance_id: int,
        region: str | None = None,
        instance_type: str | None = None,
        label: str | None = None,
        disks: list[int] | None = None,
        configs: list[int] | None = None,
    ) -> dict[str, Any]:
        """Clone instance with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.clone_instance,
            instance_id,
            region,
            instance_type,
            label,
            disks,
            configs,
        )
        return result

    async def migrate_instance(
        self,
        instance_id: int,
        region: str | None = None,
    ) -> None:
        """Migrate instance with retry."""
        await self._execute_with_retry(
            self.client.migrate_instance,
            instance_id,
            region,
        )

    async def rebuild_instance(
        self,
        instance_id: int,
        image: str,
        root_pass: str,
        authorized_keys: list[str] | None = None,
        authorized_users: list[str] | None = None,
    ) -> dict[str, Any]:
        """Rebuild instance with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.rebuild_instance,
            instance_id,
            image,
            root_pass,
            authorized_keys,
            authorized_users,
        )
        return result

    async def rescue_instance(
        self,
        instance_id: int,
        devices: dict[str, Any] | None = None,
    ) -> None:
        """Rescue instance with retry."""
        await self._execute_with_retry(
            self.client.rescue_instance,
            instance_id,
            devices,
        )

    async def reset_instance_password(self, instance_id: int, root_pass: str) -> None:
        """Reset instance password with retry."""
        await self._execute_with_retry(
            self.client.reset_instance_password,
            instance_id,
            root_pass,
        )

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        """Execute a function with retry logic and circuit breaker.

        Raises CircuitOpenError if the breaker is rejecting requests so
        callers can fail fast instead of waiting on the upstream.
        """
        # Fail fast when the breaker is open. Done before the semaphore so
        # we don't hold a slot while rejecting.
        self._circuit.allow()

        async with self._request_semaphore:
            last_error: Exception | None = None

            for attempt in range(self.retry_config.max_retries + 1):
                if attempt > 0:
                    delay = self._calculate_delay(attempt)
                    await asyncio.sleep(delay)

                # Gate the network attempt on the per-client rate limiter so
                # the bucket drains per network call (initial + retries), not
                # per logical operation.
                await self._limiter.wait()

                try:
                    result = await func(*args)
                except Exception as exc:
                    last_error = exc
                    if not self._should_retry(exc):
                        # Non-retryable (auth, etc.) is not the breaker's concern.
                        raise
                    if attempt == self.retry_config.max_retries:
                        break
                else:
                    self._circuit.record_success()
                    return result

            # Retries exhausted on a retryable failure: this is the signal
            # the breaker tracks.
            self._circuit.record_failure()
            raise last_error or LinodeError("Unknown retry error")

    def _calculate_delay(self, attempt: int) -> float:
        """Calculate delay for retry with exponential backoff and jitter."""
        delay = self.retry_config.base_delay * (
            self.retry_config.backoff_factor ** (attempt - 1)
        )

        if self.retry_config.jitter_enabled:
            jitter = delay * 0.1 * secrets.SystemRandom().random()
            delay += jitter

        return min(delay, self.retry_config.max_delay)

    def _should_retry(self, error: Exception) -> bool:
        """Determine if an error should be retried."""
        if isinstance(error, APIError):
            if error.is_rate_limit_error() or error.is_server_error():
                return True
            if error.is_authentication_error() or error.is_forbidden_error():
                return False

        return isinstance(error, NetworkError | httpx.TimeoutException)


def is_retryable(error: Exception) -> bool:
    """Check if an error is retryable."""
    if isinstance(error, RetryableError):
        return True
    if isinstance(error, APIError):
        return error.is_rate_limit_error() or error.is_server_error()
    return isinstance(error, (NetworkError, httpx.TimeoutException))
