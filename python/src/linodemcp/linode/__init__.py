"""Linode API client."""

import asyncio
import enum
import functools
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
from typing import Any, TypeVar, cast
from urllib.parse import urlencode

import httpx

from linodemcp.config import ObjectStorageConfig
from linodemcp.linode.metrics import get_api_recorder, metrics_endpoint
from linodemcp.linode.routes import (
    DEFAULT_SURFACE_SEGMENT,
    base_for,
    route_for,
    surface_segment,
)

_MANAGED_SERVICE_TIMEOUT_MAX = 255

T = TypeVar("T")

LINODE_STATS_MIN_YEAR = 1970
LINODE_STATS_MAX_YEAR = 9999
LINODE_STATS_MAX_MONTH = 12
MANAGED_LINODE_SSH_PORT_MAX = 65535
MANAGED_LINODE_SSH_USER_MAX_LENGTH = 32
_UNSET: Any = object()

logger = logging.getLogger(__name__)


def _validate_positive_path_int(value: object, name: str) -> int:
    """Validate a finite positive integer path parameter."""
    if not isinstance(value, int) or isinstance(value, bool) or value < 1:
        msg = f"{name} must be a positive integer"
        raise ValueError(msg)
    return value


_PLACEMENT_GROUP_LABEL_PATTERN = re.compile(
    r"^[A-Za-z0-9](?:[A-Za-z0-9._-]*[A-Za-z0-9])?$"
)

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
            msg = "a record target cannot be a private IP address"
            raise ValueError(msg)


def validate_ipv4_address(address: object) -> None:
    """Validate that a value is an IPv4 address string."""
    if not isinstance(address, str):
        msg = "ipv4 must be a valid IPv4 address"
        raise TypeError(msg)

    try:
        ipaddress.IPv4Address(address)
    except ipaddress.AddressValueError:
        msg = "ipv4 must be a valid IPv4 address"
        raise ValueError(msg) from None


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


def validate_image_sharegroups_image_id(image_id: Any) -> str:
    """Validate an image ID used for the image sharegroups route."""
    if not isinstance(image_id, str) or not image_id.strip():
        msg = "image_id must be a non-empty string"
        raise ValueError(msg)
    image_id_value = image_id.strip()
    if "?" in image_id_value or ".." in image_id_value:
        msg = "image_id must not contain query or traversal segments"
        raise ValueError(msg)
    return image_id_value


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
    created: str = ""
    updated: str = ""
    version: int = 0


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
    retry_sec: int = 0
    master_ips: list[str] = dc_field(default_factory=list[str])
    axfr_ips: list[str] = dc_field(default_factory=list[str])
    expire_sec: int = 0
    refresh_sec: int = 0
    ttl_sec: int = 0
    group: str = ""


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
    service: str = ""
    protocol: str = ""
    tag: str = ""


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
    rev_note: str = ""


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


def _parse_interface_ipv4_address(data: dict[str, Any]) -> InterfaceIPv4Address:
    """Parse a single IPv4 address entry on an interface."""
    return InterfaceIPv4Address(
        address=data.get("address", ""),
        primary=bool(data.get("primary", False)),
    )


def _parse_interface_public(
    data: dict[str, Any] | None,
) -> InterfacePublicConfig | None:
    """Parse a public-interface subobject, including the ipv4 addresses and
    ipv6 ranges. A present-but-empty object yields a non-None config with no
    sub-config, matching the Go pointer that round-trips as ``{}``.
    """
    if data is None:
        return None

    ipv4: InterfacePublicIPv4 | None = None
    if (ipv4_data := data.get("ipv4")) is not None:
        ipv4 = InterfacePublicIPv4(
            addresses=[
                _parse_interface_ipv4_address(addr)
                for addr in ipv4_data.get("addresses", [])
            ]
        )

    ipv6: InterfacePublicIPv6 | None = None
    if (ipv6_data := data.get("ipv6")) is not None:
        ipv6 = InterfacePublicIPv6(
            ranges=[
                InterfaceIPv6Range(range=item.get("range", ""))
                for item in ipv6_data.get("ranges", [])
            ]
        )

    return InterfacePublicConfig(ipv4=ipv4, ipv6=ipv6)


def _parse_interface_vpc(data: dict[str, Any] | None) -> InterfaceVPCConfig | None:
    """Parse a vpc-interface subobject, including the ipv4 addresses."""
    if data is None:
        return None

    ipv4: InterfaceVPCIPv4 | None = None
    if (ipv4_data := data.get("ipv4")) is not None:
        ipv4 = InterfaceVPCIPv4(
            addresses=[
                _parse_interface_ipv4_address(addr)
                for addr in ipv4_data.get("addresses", [])
            ]
        )

    return InterfaceVPCConfig(subnet_id=data.get("subnet_id", 0), ipv4=ipv4)


def _parse_interface_vlan(data: dict[str, Any] | None) -> InterfaceVLANConfig | None:
    """Parse a vlan-interface subobject."""
    if data is None:
        return None

    return InterfaceVLANConfig(
        vlan_label=data.get("vlan_label", ""),
        ipam_address=data.get("ipam_address", ""),
    )


def _parse_instance_interface(data: dict[str, Any]) -> InstanceInterface:
    """Parse a single interface object from the API response, including the
    public, vpc, and vlan sub-configs. Mirrors the Go client's struct-tag
    deserialization of Instance.Interfaces so a read returns the same shape
    in both implementations.
    """
    return InstanceInterface(
        id=data.get("id", 0),
        public=_parse_interface_public(data.get("public")),
        vpc=_parse_interface_vpc(data.get("vpc")),
        vlan=_parse_interface_vlan(data.get("vlan")),
        default_route=_parse_default_route(data.get("default_route")),
        firewall_id=data.get("firewall_id"),
        mac_address=data.get("mac_address", ""),
        created=data.get("created", ""),
        updated=data.get("updated", ""),
        version=data.get("version", 0),
    )


def _serialize_interface_ipv4_address(addr: InterfaceIPv4Address) -> dict[str, Any]:
    """Serialize an IPv4 address with Go's omitempty on ``primary``."""
    body: dict[str, Any] = {"address": addr.address}
    if addr.primary:
        body["primary"] = addr.primary
    return body


def _serialize_interface_public(public: InterfacePublicConfig) -> dict[str, Any]:
    """Serialize a public config, omitting empty sub-objects like Go does."""
    body: dict[str, Any] = {}
    if public.ipv4 is not None:
        ipv4: dict[str, Any] = {}
        if public.ipv4.addresses:
            ipv4["addresses"] = [
                _serialize_interface_ipv4_address(addr)
                for addr in public.ipv4.addresses
            ]
        body["ipv4"] = ipv4
    if public.ipv6 is not None:
        ipv6: dict[str, Any] = {}
        if public.ipv6.ranges:
            ipv6["ranges"] = [{"range": item.range} for item in public.ipv6.ranges]
        body["ipv6"] = ipv6
    return body


def _serialize_interface_vpc(vpc: InterfaceVPCConfig) -> dict[str, Any]:
    """Serialize a vpc config; ``subnet_id`` is always present like Go."""
    body: dict[str, Any] = {"subnet_id": vpc.subnet_id}
    if vpc.ipv4 is not None:
        ipv4: dict[str, Any] = {}
        if vpc.ipv4.addresses:
            ipv4["addresses"] = [
                _serialize_interface_ipv4_address(addr) for addr in vpc.ipv4.addresses
            ]
        body["ipv4"] = ipv4
    return body


def _serialize_interface_vlan(vlan: InterfaceVLANConfig) -> dict[str, Any]:
    """Serialize a vlan config with Go's omitempty on ``ipam_address``."""
    body: dict[str, Any] = {"vlan_label": vlan.vlan_label}
    if vlan.ipam_address:
        body["ipam_address"] = vlan.ipam_address
    return body


def _serialize_default_route(route: InterfaceDefaultRoute) -> dict[str, Any]:
    """Serialize a default_route, omitting a family when False like Go does."""
    body: dict[str, Any] = {}
    if route.ipv4:
        body["ipv4"] = route.ipv4
    if route.ipv6:
        body["ipv6"] = route.ipv6
    return body


def _serialize_instance_interface(iface: InstanceInterface) -> dict[str, Any]:
    """Serialize one interface to the same object the Go client emits, in
    struct-field order, with Go's omitempty applied to every field.
    """
    body: dict[str, Any] = {}
    if iface.id:
        body["id"] = iface.id
    if iface.public is not None:
        body["public"] = _serialize_interface_public(iface.public)
    if iface.vpc is not None:
        body["vpc"] = _serialize_interface_vpc(iface.vpc)
    if iface.vlan is not None:
        body["vlan"] = _serialize_interface_vlan(iface.vlan)
    if iface.default_route is not None:
        body["default_route"] = _serialize_default_route(iface.default_route)
    if iface.firewall_id is not None:
        body["firewall_id"] = iface.firewall_id
    if iface.mac_address:
        body["mac_address"] = iface.mac_address
    if iface.created:
        body["created"] = iface.created
    if iface.updated:
        body["updated"] = iface.updated
    if iface.version:
        body["version"] = iface.version
    return body


def _serialize_backups(backups: Backups) -> dict[str, Any]:
    """Serialize backup settings in proto-canonical order: schedule,
    last_successful (omitted when absent, matching protojson's handling of an
    unset message field), enabled, available.
    """
    body: dict[str, Any] = {
        "schedule": {
            "day": backups.schedule.day,
            "window": backups.schedule.window,
        },
    }
    if backups.last_successful is not None:
        snap = backups.last_successful
        body["last_successful"] = {
            "id": snap.id,
            "label": snap.label,
            "status": snap.status,
            "type": snap.type,
            "region": snap.region,
            "created": snap.created,
            "updated": snap.updated,
            "finished": snap.finished,
        }
    body["enabled"] = backups.enabled
    body["available"] = backups.available
    return body


def instance_to_response_dict(instance: Instance) -> dict[str, Any]:
    """Serialize an Instance to the JSON object the Go client emits from
    MarshalToolResponse(instance): every struct field in declaration order,
    with interface_generation and interfaces omitted when empty and the
    interface subtree following Go's omitempty tags. Keeps the
    linode_instance_get and linode_instance_list output identical across the
    Go and Python implementations.
    """
    body: dict[str, Any] = {
        "id": instance.id,
        "label": instance.label,
        "status": instance.status,
        "type": instance.type,
        "region": instance.region,
        "image": instance.image,
        "ipv4": instance.ipv4,
        "ipv6": instance.ipv6,
        "hypervisor": instance.hypervisor,
        "specs": {
            "disk": instance.specs.disk,
            "memory": instance.specs.memory,
            "vcpus": instance.specs.vcpus,
            "gpus": instance.specs.gpus,
            "transfer": instance.specs.transfer,
        },
        "alerts": {
            "cpu": instance.alerts.cpu,
            "network_in": instance.alerts.network_in,
            "network_out": instance.alerts.network_out,
            "transfer_quota": instance.alerts.transfer_quota,
            "io": instance.alerts.io,
        },
        "backups": _serialize_backups(instance.backups),
        "created": instance.created,
        "updated": instance.updated,
        "group": instance.group,
        "tags": instance.tags,
        "watchdog_enabled": instance.watchdog_enabled,
    }
    if instance.interface_generation:
        body["interface_generation"] = instance.interface_generation
    body["interfaces"] = [
        _serialize_instance_interface(iface) for iface in instance.interfaces
    ]
    return body


def parse_instance(data: dict[str, Any]) -> Instance:
    """Parse a raw /linode/instances API object into the Instance model.

    Module-level so tests and non-client callers can build a real Instance
    from a sparse API-shaped dict instead of hand-filling every field.
    """
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


def instance_preview_state(instance: Instance) -> dict[str, Any]:
    """Serialize an Instance for a dry-run or plan current_state.

    Mirrors Go's json.Marshal of the Instance struct rather than the proto
    read shape: interface_generation and interfaces carry omitempty (dropped
    when empty), while the nil last_successful backup pointer marshals as an
    explicit null instead of being omitted the way protojson leaves an unset
    message field out.
    """
    body = instance_to_response_dict(instance)
    if not body.get("interfaces"):
        body.pop("interfaces", None)

    backups = body.get("backups")
    if isinstance(backups, dict):
        cast("dict[str, Any]", backups).setdefault("last_successful", None)
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
            response = await self.make_route_request("linode_profile_get")
            data = response.json()
            return self._parse_profile(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetProfile", e) from e

    async def get_profile_grants(self) -> Grants:
        """Get the /profile/grants response for OAuth scope inspection.

        PATs return an empty payload (200 with zero-valued fields); the
        Phase 6 profile loader checks ``Profile.scopes`` first and only
        consults Grants when the scope string is empty (OAuth path).
        """
        try:
            response = await self.make_route_request("linode_profile_grants_get")
            data: Any = response.json()
            if not isinstance(data, dict):
                return Grants()
            return self._parse_grants(cast("dict[str, Any]", data))
        except httpx.HTTPError as e:
            raise NetworkError("GetProfileGrants", e) from e

    async def list_instance_configs(
        self,
        linode_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List configuration profiles for a Linode instance."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_instance_config_list", linode_id, query=urlencode(params)
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceConfigs", e) from e

    async def get_instance_config(
        self, linode_id: int, config_id: int
    ) -> dict[str, Any]:
        """Get a configuration profile for a Linode instance."""
        try:
            response = await self.make_route_request(
                "linode_instance_config_get", linode_id, config_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceConfig", e) from e

    async def get_instance_config_interface(
        self, linode_id: int, config_id: int, interface_id: int
    ) -> dict[str, Any]:
        """Get an interface for a Linode instance configuration profile."""
        linode_id = _validate_positive_path_int(linode_id, "linode_id")
        config_id = _validate_positive_path_int(config_id, "config_id")
        interface_id = _validate_positive_path_int(interface_id, "interface_id")
        try:
            response = await self.make_route_request(
                "linode_instance_config_interface_get",
                linode_id,
                config_id,
                interface_id,
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceConfigInterface", e) from e

    async def get_instance_interface_settings(self, linode_id: int) -> dict[str, Any]:
        """List interface settings for a Linode instance."""
        linode_id = _validate_positive_path_int(linode_id, "linode_id")
        try:
            response = await self.make_route_request(
                "linode_instance_interface_settings_get", linode_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceInterfaceSettings", e) from e

    async def get_instance_interface(
        self, linode_id: int, interface_id: int
    ) -> dict[str, Any]:
        """Get an interface for a Linode instance."""
        linode_id = _validate_positive_path_int(linode_id, "linode_id")
        interface_id = _validate_positive_path_int(interface_id, "interface_id")
        try:
            response = await self.make_route_request(
                "linode_instance_interface_get", linode_id, interface_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceInterface", e) from e

    async def get_instance(self, instance_id: int) -> Instance:
        """Get a specific Linode instance."""
        try:
            response = await self.make_route_request("linode_instance_get", instance_id)
            data = response.json()
            return self._parse_instance(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetInstance", e) from e

    async def get_account(self) -> Account:
        """Get Linode account information."""
        try:
            response = await self.make_route_request("linode_account_get")
            data = response.json()
            return self._parse_account(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetAccount", e) from e

    async def get_account_settings(self) -> dict[str, Any]:
        """Get settings for the Linode account."""
        try:
            response = await self.make_route_request("linode_account_settings_get")
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountSettings", e) from e

    async def get_account_event(self, event_id: int) -> dict[str, Any]:
        """Get an event on the Linode account."""
        try:
            response = await self.make_route_request(
                "linode_account_event_get", event_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountEvent", e) from e

    async def get_database_mysql_instance(self, instance_id: int) -> dict[str, Any]:
        """Get a MySQL Managed Database instance by ID."""
        try:
            response = await self.make_route_request(
                "linode_database_mysql_instance_get", instance_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabaseMySQLInstance", e) from e

    async def get_database_postgresql_instance(
        self, instance_id: int
    ) -> dict[str, Any]:
        """Get a PostgreSQL Managed Database instance by ID."""
        try:
            response = await self.make_route_request(
                "linode_database_postgresql_instance_get", instance_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetDatabasePostgreSQLInstance", e) from e

    async def get_account_child_account(self, euuid: str) -> dict[str, Any]:
        """Get a child account by EUUID."""
        try:
            response = await self.make_route_request(
                "linode_account_child_account_get", euuid
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountChildAccount", e) from e

    async def get_account_service_transfer(self, token: str) -> dict[str, Any]:
        """Get an account service transfer request by token."""
        try:
            response = await self.make_route_request(
                "linode_account_service_transfer_get", token
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountServiceTransfer", e) from e

    async def get_account_oauth_client(self, client_id: str) -> dict[str, Any]:
        """Get an OAuth client by client ID."""
        try:
            response = await self.make_route_request(
                "linode_account_oauth_client_get", client_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountOAuthClient", e) from e

    async def get_account_payment_method(
        self, payment_method_id: int
    ) -> dict[str, Any]:
        """Get an account payment method by ID."""
        try:
            response = await self.make_route_request(
                "linode_account_payment_method_get", payment_method_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountPaymentMethod", e) from e

    async def get_account_user(self, username: str) -> dict[str, Any]:
        """Get an account user by username."""
        try:
            response = await self.make_route_request(
                "linode_account_user_get", username
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountUser", e) from e

    async def get_account_user_grants(self, username: str) -> dict[str, Any]:
        """List grants for an account user by username."""
        try:
            response = await self.make_route_request(
                "linode_account_user_grants_get", username
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetAccountUserGrants", e) from e

    async def get_type(self, type_id: str) -> InstanceType:
        """Get a Linode instance type."""
        if not type_id:
            raise ValueError("type_id is required")
        try:
            response = await self.make_route_request("linode_type_get", type_id)
            data = response.json()
            return self._parse_instance_type(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetType", e) from e

    async def get_volume(self, volume_id: int) -> Volume:
        """Get a Linode block storage volume."""
        try:
            response = await self.make_route_request("linode_volume_get", volume_id)
            data = response.json()
            return self._parse_volume(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetVolume", e) from e

    async def get_image_sharegroup(self, sharegroup_id: str) -> dict[str, Any]:
        """Get a single image share group."""
        try:
            response = await self.make_route_request(
                "linode_image_sharegroup_get", sharegroup_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetImageSharegroup", e) from e

    async def get_image_sharegroup_by_token(self, token_uuid: str) -> dict[str, Any]:
        """Get the image share group associated with a token."""
        try:
            response = await self.make_route_request(
                "linode_image_sharegroup_by_token_get", token_uuid
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetImageSharegroupByToken", e) from e

    async def get_image(self, image_id: str) -> Image:
        """Get a single Linode image."""
        try:
            response = await self.make_route_request("linode_image_get", image_id)
            data = response.json()
            return self._parse_image(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetImage", e) from e

    async def get_ssh_key(self, ssh_key_id: int) -> SSHKey:
        """Get a specific SSH key."""
        try:
            response = await self.make_route_request("linode_sshkey_get", ssh_key_id)
            data = response.json()
            return self._parse_ssh_key(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetSSHKey", e) from e

    async def get_domain(self, domain_id: int) -> Domain:
        """Get a specific domain."""
        try:
            response = await self.make_route_request("linode_domain_get", domain_id)
            data = response.json()
            return self._parse_domain(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetDomain", e) from e

    async def list_domain_records(self, domain_id: int) -> list[DomainRecord]:
        """List domain records for a domain."""
        try:
            response = await self.make_route_request(
                "linode_domain_record_list", domain_id
            )
            data = response.json()
            return [self._parse_domain_record(r) for r in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListDomainRecords", e) from e

    async def get_domain_record(self, domain_id: int, record_id: int) -> DomainRecord:
        """Get a specific domain record."""
        try:
            response = await self.make_route_request(
                "linode_domain_record_get", domain_id, record_id
            )
            data = response.json()
            return self._parse_domain_record(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetDomainRecord", e) from e

    async def get_firewall(self, firewall_id: int) -> Firewall:
        """Get a specific firewall."""
        try:
            response = await self.make_route_request("linode_firewall_get", firewall_id)
            data = response.json()
            return self._parse_firewall(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewall", e) from e

    async def get_firewall_settings(
        self, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List default firewall settings."""
        for name, value in (("page", page), ("page_size", page_size)):
            if value is not None and (type(value) is not int or value <= 0):
                msg = f"{name} must be a positive integer"
                raise ValueError(msg)
        params: dict[str, Any] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_firewall_settings_get", query=urlencode(params)
            )
            return cast("dict[str, Any]", response.json())
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallSettings", e) from e

    async def get_firewall_rules(self, firewall_id: int) -> FirewallRules:
        """Get firewall rules for a specific firewall."""
        try:
            response = await self.make_route_request(
                "linode_firewall_rules_get", firewall_id
            )
            data = response.json()
            return self._parse_firewall_rules(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallRules", e) from e

    async def get_firewall_device(
        self, firewall_id: int, device_id: int
    ) -> dict[str, Any]:
        """Get a specific firewall device."""
        try:
            response = await self.make_route_request(
                "linode_firewall_device_get", firewall_id, device_id
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetFirewallDevice", e) from e

    async def list_firewall_devices(
        self,
        firewall_id: int | str,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List devices attached to a Cloud Firewall."""
        params: dict[str, Any] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_firewall_device_list", firewall_id, query=urlencode(params)
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("ListFirewallDevices", e) from e

    async def list_vlans(
        self,
        page: int | None = None,
        page_size: int | None = None,
    ) -> list[dict[str, Any]]:
        """List VLANs."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_vlan_list", query=urlencode(params)
            )
            data = response.json()
            vlans: list[dict[str, Any]] = data.get("data", [])
            return vlans
        except httpx.HTTPError as e:
            raise NetworkError("ListVLANs", e) from e

    async def list_tagged_objects(
        self, tag_label: str, page: int | None = None, page_size: int | None = None
    ) -> dict[str, Any]:
        """List objects assigned to a tag."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_tag_object_list", tag_label, query=urlencode(params)
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListTaggedObjects", e) from e

    async def get_managed_credential(self, credential_id: int) -> dict[str, Any]:
        """Get a Managed credential by credential ID."""
        valid_credential_id = _validate_positive_path_int(
            credential_id, "credential_id"
        )
        try:
            response = await self.make_route_request(
                "linode_managed_credential_get", valid_credential_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetManagedCredential", e) from e

    async def get_managed_service(self, service_id: int) -> dict[str, Any]:
        """Get a Managed service monitor by service ID."""
        valid_service_id = _validate_positive_path_int(service_id, "service_id")
        try:
            response = await self.make_route_request(
                "linode_managed_service_get", valid_service_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetManagedService", e) from e

    async def get_managed_linode_settings(self, linode_id: int) -> dict[str, Any]:
        """Get Managed settings for a Linode."""
        valid_linode_id = _validate_positive_path_int(linode_id, "linode_id")
        try:
            response = await self.make_route_request(
                "linode_managed_linode_settings_get", valid_linode_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetManagedLinodeSettings", e) from e

    async def get_managed_contact(self, contact_id: int) -> dict[str, Any]:
        """Get a Managed contact."""
        valid_contact_id = _validate_positive_path_int(contact_id, "contact_id")
        try:
            response = await self.make_route_request(
                "linode_managed_contact_get", valid_contact_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetManagedContact", e) from e

    async def get_support_ticket(self, ticket_id: int) -> dict[str, Any]:
        """Get a support ticket."""
        try:
            response = await self.make_route_request(
                "linode_support_ticket_get", ticket_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetSupportTicket", e) from e

    async def get_nodebalancer(self, nodebalancer_id: int) -> NodeBalancer:
        """Get a specific NodeBalancer."""
        try:
            response = await self.make_route_request(
                "linode_nodebalancer_get", nodebalancer_id
            )
            data = response.json()
            return self._parse_nodebalancer(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancer", e) from e

    async def list_nodebalancer_configs(
        self,
        nodebalancer_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List configs for a NodeBalancer."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_nodebalancer_config_list",
                nodebalancer_id,
                query=urlencode(params),
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerConfigs", e) from e

    async def get_nodebalancer_config(
        self, nodebalancer_id: int, config_id: int
    ) -> dict[str, Any]:
        """Get a NodeBalancer config."""
        try:
            response = await self.make_route_request(
                "linode_nodebalancer_config_get", nodebalancer_id, config_id
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancerConfig", e) from e

    async def list_nodebalancer_config_nodes(
        self,
        nodebalancer_id: int,
        config_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List nodes in a NodeBalancer config."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_nodebalancer_config_node_list",
                nodebalancer_id,
                config_id,
                query=urlencode(params),
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerConfigNodes", e) from e

    async def get_nodebalancer_config_node(
        self, nodebalancer_id: int, config_id: int, node_id: int
    ) -> dict[str, Any]:
        """Get a node from a NodeBalancer config."""
        try:
            response = await self.make_route_request(
                "linode_nodebalancer_config_node_get",
                nodebalancer_id,
                config_id,
                node_id,
            )
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
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_nodebalancer_firewall_list",
                nodebalancer_id,
                query=urlencode(params),
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancerFirewalls", e) from e

    async def get_stackscript(self, stackscript_id: int | str) -> StackScript:
        """Get a StackScript by ID."""
        try:
            response = await self.make_route_request(
                "linode_stackscript_get", stackscript_id
            )
            data = response.json()
            return self._parse_stackscript(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetStackScript", e) from e

    async def get_object_storage_bucket(
        self, region: str, label: str
    ) -> dict[str, Any]:
        """Get a specific Object Storage bucket."""
        try:
            response = await self.make_route_request(
                "linode_object_storage_bucket_get", region, label
            )
            bucket: dict[str, Any] = response.json()
            return bucket
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageBucket", e) from e

    async def get_object_storage_key(self, key_id: int) -> dict[str, Any]:
        """Get a specific Object Storage access key."""
        try:
            response = await self.make_route_request(
                "linode_object_storage_key_get", key_id
            )
            key: dict[str, Any] = response.json()
            return key
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageKey", e) from e

    async def get_object_storage_bucket_access(
        self, region: str, label: str
    ) -> dict[str, Any]:
        """Get bucket ACL and CORS settings."""
        try:
            response = await self.make_route_request(
                "linode_object_storage_bucket_access_get", region, label
            )
            access: dict[str, Any] = response.json()
            return access
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectStorageBucketAccess", e) from e

    async def get_object_acl(
        self, region: str, label: str, name: str
    ) -> dict[str, Any]:
        """Get the ACL for an object in Object Storage."""
        try:
            response = await self.make_route_request(
                "linode_object_storage_object_acl_get",
                region,
                label,
                query=urlencode({"name": name}),
            )
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("GetObjectACL", e) from e

    async def get_bucket_ssl(self, region: str, label: str) -> dict[str, Any]:
        """Get the SSL/TLS certificate status for a bucket."""
        try:
            response = await self.make_route_request(
                "linode_object_storage_ssl_get", region, label
            )
            return dict(response.json())
        except httpx.HTTPError as e:
            raise NetworkError("GetBucketSSL", e) from e

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

    async def get_profile_token(self, token_id: int) -> dict[str, Any]:
        """Get a personal access token."""
        logger.info("Getting profile token", extra={"token_id": token_id})

        try:
            response = await self.make_route_request(
                "linode_profile_token_get", token_id
            )
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

    async def get_profile_device(self, device_id: int) -> dict[str, Any]:
        """Get a trusted profile device."""
        logger.info("Getting profile trusted device", extra={"device_id": device_id})

        try:
            response = await self.make_route_request(
                "linode_profile_device_get", device_id
            )
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

    async def get_longview_plan(self) -> dict[str, Any]:
        """Get the Longview plan for the account."""
        logger.info("Getting Longview plan")

        try:
            response = await self.make_route_request("linode_longview_plan_get")
            result: dict[str, Any] = response.json()
            logger.info("Longview plan retrieved")
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting Longview plan: %s", e)
            raise NetworkError("GetLongviewPlan", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting Longview plan: %s", e)
            raise NetworkError("GetLongviewPlan", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting Longview plan")
            raise NetworkError("GetLongviewPlan", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting Longview plan: %s", e)
            raise NetworkError("GetLongviewPlan", e) from e

    async def get_longview_client(self, client_id: object) -> dict[str, Any]:
        """Get a Longview client."""
        valid_client_id = _validate_positive_path_int(client_id, "client_id")
        try:
            response = await self.make_route_request(
                "linode_longview_client_get", valid_client_id
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.ConnectTimeout as e:
            logger.exception("Connection timeout getting Longview client: %s", e)
            raise NetworkError("GetLongviewClient", e) from e
        except httpx.ReadTimeout as e:
            logger.exception("Read timeout getting Longview client: %s", e)
            raise NetworkError("GetLongviewClient", e) from e
        except httpx.HTTPStatusError as e:
            logger.exception("HTTP error getting Longview client")
            raise NetworkError("GetLongviewClient", e) from e
        except httpx.HTTPError as e:
            logger.exception("HTTP error getting Longview client: %s", e)
            raise NetworkError("GetLongviewClient", e) from e

    async def get_profile_app(self, app_id: int) -> dict[str, Any]:
        """Get an OAuth app authorization from the profile."""
        logger.info("Getting profile app authorization", extra={"app_id": app_id})

        try:
            response = await self.make_route_request("linode_profile_app_get", app_id)
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

        logger.info(
            "Getting monitor service alert definition",
            extra={"service_type": service_type, "alert_id": alert_id},
        )

        try:
            response = await self.make_route_request(
                "linode_monitor_service_alert_definition_get", service_type, alert_id
            )
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

    async def get_lke_cluster(self, cluster_id: int) -> dict[str, Any]:
        """Get a specific LKE cluster."""
        try:
            response = await self.make_route_request(
                "linode_lke_cluster_get", cluster_id
            )
            cluster: dict[str, Any] = response.json()
            return cluster
        except httpx.HTTPError as e:
            raise NetworkError("GetLKECluster", e) from e

    async def list_lke_node_pools(self, cluster_id: int) -> list[dict[str, Any]]:
        """List node pools for an LKE cluster."""
        try:
            response = await self.make_route_request("linode_lke_pool_list", cluster_id)
            data = response.json()
            pools: list[dict[str, Any]] = data.get("data", [])
            return pools
        except httpx.HTTPError as e:
            raise NetworkError("ListLKENodePools", e) from e

    async def get_lke_node_pool(self, cluster_id: int, pool_id: int) -> dict[str, Any]:
        """Get a specific node pool."""
        try:
            response = await self.make_route_request(
                "linode_lke_pool_get", cluster_id, pool_id
            )
            pool: dict[str, Any] = response.json()
            return pool
        except httpx.HTTPError as e:
            raise NetworkError("GetLKENodePool", e) from e

    async def get_lke_node(self, cluster_id: int, node_id: str) -> dict[str, Any]:
        """Get a specific node in an LKE cluster."""
        try:
            response = await self.make_route_request(
                "linode_lke_node_get", cluster_id, node_id
            )
            node: dict[str, Any] = response.json()
            return node
        except httpx.HTTPError as e:
            raise NetworkError("GetLKENode", e) from e

    async def get_lke_control_plane_acl(self, cluster_id: int) -> dict[str, Any]:
        """Get the control plane ACL for an LKE cluster.

        The Linode API wraps the ACL under a top-level "acl" key. Unwrap it and
        return the bare ACL so the emitted shape matches the Go implementation
        ({"enabled": ..., "addresses": {"ipv4": [...], "ipv6": [...]}}).
        """
        try:
            response = await self.make_route_request("linode_lke_acl_get", cluster_id)
            payload: dict[str, Any] = response.json()
            raw_acl = payload.get("acl", payload)
            acl: dict[str, Any] = (
                cast("dict[str, Any]", raw_acl) if isinstance(raw_acl, dict) else {}
            )
            raw_addresses = acl.get("addresses")
            addresses: dict[str, Any] = (
                cast("dict[str, Any]", raw_addresses)
                if isinstance(raw_addresses, dict)
                else {}
            )
            return {
                "enabled": acl.get("enabled", False),
                "addresses": {
                    "ipv4": addresses.get("ipv4") or [],
                    "ipv6": addresses.get("ipv6") or [],
                },
            }
        except httpx.HTTPError as e:
            raise NetworkError("GetLKEControlPlaneACL", e) from e

    async def get_vpc(self, vpc_id: int) -> dict[str, Any]:
        """Get a specific VPC."""
        try:
            response = await self.make_route_request("linode_vpc_get", vpc_id)
            vpc: dict[str, Any] = response.json()
            return vpc
        except httpx.HTTPError as e:
            raise NetworkError("GetVPC", e) from e

    async def list_vpc_subnets(self, vpc_id: int) -> list[dict[str, Any]]:
        """List subnets for a VPC."""
        try:
            response = await self.make_route_request("linode_vpc_subnet_list", vpc_id)
            data = response.json()
            subnets: list[dict[str, Any]] = data.get("data", [])
            return subnets
        except httpx.HTTPError as e:
            raise NetworkError("ListVPCSubnets", e) from e

    async def get_vpc_subnet(self, vpc_id: int, subnet_id: int) -> dict[str, Any]:
        """Get a specific VPC subnet."""
        try:
            response = await self.make_route_request(
                "linode_vpc_subnet_get", vpc_id, subnet_id
            )
            subnet: dict[str, Any] = response.json()
            return subnet
        except httpx.HTTPError as e:
            raise NetworkError("GetVPCSubnet", e) from e

    async def get_placement_group(self, group_id: int) -> dict[str, Any]:
        """Get a placement group."""
        try:
            response = await self.make_route_request(
                "linode_placement_group_get", group_id
            )
            placement_group: dict[str, Any] = response.json()
            return placement_group
        except httpx.HTTPError as e:
            raise NetworkError("GetPlacementGroup", e) from e

    async def get_ipv6_range(self, ipv6_range: str) -> dict[str, Any]:
        """Get an IPv6 range."""
        try:
            response = await self.make_route_request(
                "linode_ipv6_range_get", ipv6_range
            )
            range_data: dict[str, Any] = response.json()
            return range_data
        except httpx.HTTPError as e:
            raise NetworkError("GetIPv6Range", e) from e

    async def get_reserved_ip(self, address: str) -> dict[str, Any]:
        """Get a reserved public IPv4 address."""
        try:
            response = await self.make_route_request(
                "linode_networking_reserved_ip_get", address
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetReservedIP", e) from e

    async def get_instance_backup(
        self, instance_id: int, backup_id: int
    ) -> dict[str, Any]:
        """Get a specific backup for an instance."""
        try:
            response = await self.make_route_request(
                "linode_instance_backup_get", instance_id, backup_id
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceBackup", e) from e

    async def list_instance_disks(
        self,
        instance_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> list[dict[str, Any]]:
        """List a page of disks for an instance."""
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_instance_disk_list", instance_id, query=urlencode(params)
            )
            data = response.json()
            disks: list[dict[str, Any]] = data.get("data", [])
            return disks
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceDisks", e) from e

    async def list_instance_volumes(
        self,
        linode_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List volumes attached to a Linode instance."""
        valid_linode_id = _validate_positive_path_int(linode_id, "linode_id")
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_instance_volume_list",
                valid_linode_id,
                query=urlencode(params),
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceVolumes", e) from e

    async def list_instance_firewalls(
        self,
        linode_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List firewalls assigned to a Linode instance."""
        valid_linode_id = _validate_positive_path_int(linode_id, "linode_id")
        params: dict[str, int] = {}
        if page is not None:
            params["page"] = page
        if page_size is not None:
            params["page_size"] = page_size
        try:
            response = await self.make_route_request(
                "linode_instance_firewall_list",
                valid_linode_id,
                query=urlencode(params),
            )
            data: dict[str, Any] = response.json()
            return data
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceFirewalls", e) from e

    async def get_instance_disk(self, instance_id: int, disk_id: int) -> dict[str, Any]:
        """Get a specific disk for an instance."""
        try:
            response = await self.make_route_request(
                "linode_instance_disk_get", instance_id, disk_id
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceDisk", e) from e

    async def list_instance_ips(self, instance_id: int) -> dict[str, Any]:
        """List IP addresses for an instance."""
        try:
            response = await self.make_route_request(
                "linode_instance_ip_list", instance_id
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("ListInstanceIPs", e) from e

    async def get_instance_ip(self, instance_id: int, address: str) -> dict[str, Any]:
        """Get a specific IP address for an instance."""
        try:
            response = await self.make_route_request(
                "linode_instance_ip_get", instance_id, address
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetInstanceIP", e) from e

    async def get_networking_ip(self, address: str) -> dict[str, Any]:
        """Get a networking-level IP address."""
        try:
            response = await self.make_route_request(
                "linode_networking_ip_get", address
            )
            result: dict[str, Any] = response.json()
            return result
        except httpx.HTTPError as e:
            raise NetworkError("GetNetworkingIP", e) from e

    async def make_request(
        self,
        method: str,
        endpoint: str,
        body: dict[str, Any] | None = None,
        *,
        base: str | None = None,
    ) -> httpx.Response:
        """Make an HTTP request to the Linode API.

        base is keyword-only and defaults to the configured one, so every call
        site that does not name a surface stays exactly as written.

        The default is resolved into the name before the concatenation rather
        than inside it: the route scanner reads `base_url + endpoint` as the
        endpoint alone, and an expression it cannot follow reads as a route with
        no evidence.
        """
        if base is None:
            base = self.base_url

        url = base + endpoint
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "User-Agent": "LinodeMCP/1.0",
        }

        start = time.monotonic()
        # Record in a finally so a transport failure (httpx ConnectError,
        # ReadTimeout, etc.) still counts: those raise before a response
        # exists, and an operator most wants failed calls on the dashboard.
        # Mirrors the Go client, which records right after the round trip with
        # status 0 when there is no response.
        status = 0
        try:
            if body is not None:
                response = await self.client.request(
                    method, url, headers=headers, json=body
                )
            else:
                response = await self.client.request(method, url, headers=headers)
            status = response.status_code
        finally:
            recorder = get_api_recorder()
            if recorder is not None:
                recorder.record_api_request(
                    metrics_endpoint(endpoint),
                    method,
                    status,
                    time.monotonic() - start,
                )

        if response.status_code >= HTTP_BAD_REQUEST:
            self._handle_error_response(response)

        return response

    async def make_route_request(
        self,
        tool: str,
        *values: object,
        body: dict[str, Any] | None = None,
        query: str | None = None,
    ) -> httpx.Response:
        """Make a request whose method and path come from the tool's proto route.

        A call site that names its tool stops carrying a second copy of the
        route the proto already declares, so the two cannot drift. Path values
        are positional, in the order the template names them.

        The query string arrives already encoded: the contract declares the
        path and nothing else, so query composition stays with the call site
        exactly as it was before the site was routed.

        The body is forwarded only when there is one, so a route call reaches
        make_request with the same arguments the hand-built call it replaces
        did.
        """
        route = route_for(tool)
        endpoint = route.endpoint(*values)
        if query:
            endpoint += "?" + query

        # The default surface sends the call exactly as it was sent before
        # surfaces existed, base argument included, which is what keeps every
        # unannotated tool's request identical. Go's onSurface answers the
        # receiver itself for the same reason.
        segment = surface_segment(route.surface)
        if segment == DEFAULT_SURFACE_SEGMENT:
            if body is None:
                return await self.make_request(route.method, endpoint)
            return await self.make_request(route.method, endpoint, body)

        base = base_for(self.base_url, segment)
        if body is None:
            return await self.make_request(route.method, endpoint, base=base)
        return await self.make_request(route.method, endpoint, body, base=base)

    async def make_route_request_content_type(
        self,
        tool: str,
        *values: object,
        content_type: str | None = None,
        content: bytes | None = None,
        accept: str | None = None,
    ) -> httpx.Response:
        """Make a route-resolved request whose body make_request cannot send.

        Go's makeRouteRequestContentType twin: the caller prepares the body
        bytes and the content type naming them, while the method and path come
        from the tool's proto route. accept negotiates the response instead,
        for the route that answers raw PNG bytes to a call with no body.

        Content-Type and Accept appear only when given, so each request
        carries exactly the headers of the hand-built call it replaced.
        """
        route = route_for(tool)
        endpoint = route.endpoint(*values)

        headers = {"Authorization": f"Bearer {self.token}"}
        if content_type is not None:
            headers["Content-Type"] = content_type
        if accept is not None:
            headers["Accept"] = accept
        headers["User-Agent"] = "LinodeMCP/1.0"

        # The default surface keeps the configured base untouched, the same
        # rule make_route_request applies.
        segment = surface_segment(route.surface)
        base = self.base_url
        if segment != DEFAULT_SURFACE_SEGMENT:
            base = base_for(self.base_url, segment)
        url = base + endpoint

        if content is None:
            response = await self.client.request(route.method, url, headers=headers)
        else:
            response = await self.client.request(
                route.method, url, headers=headers, content=content
            )

        if response.status_code >= HTTP_BAD_REQUEST:
            self._handle_error_response(response)

        return response

    async def route_raw(
        self,
        tool: str,
        *values: object,
        body: dict[str, Any] | None = None,
        query: str | None = None,
    ) -> Any:
        """Resolve a tool's route and return the decoded JSON body.

        The routed twin of get_raw, post_raw, and put_raw: the HTTP method
        travels with the path in the proto contract, so a caller names its
        tool and neither. POST and PUT default to an empty JSON body because
        the raw wrappers always sent one, and a routed call must put the same
        bytes on the wire as the hand-built call it replaces.
        """
        route = route_for(tool)
        if body is None and route.method in ("POST", "PUT"):
            body = {}
        response = await self.make_route_request(tool, *values, body=body, query=query)
        data: Any = response.json()
        return data

    async def route_call(self, tool: str, *values: object) -> None:
        """Perform a tool's route with no body and nothing to decode.

        This is the shape of a DELETE: the API reports the removal by status
        alone, and the tool's own answer is built from the ids it was addressed
        by. Decoding is what separates it from route_raw, not the method: an
        empty 200 body has no JSON in it to parse.
        """
        await self.make_route_request(tool, *values)

    async def route_multipart(
        self, tool: str, *values: object, part_name: str, file_path: str
    ) -> None:
        """Perform a tool's route with a multipart form framed from a local file.

        The form is rendered by httpx's own encoder, so the bytes and the
        boundary header match what a files= upload has always sent. Nothing is
        decoded: the tool's answer is built from the arguments it was called
        with.
        """
        path = Path(file_path)
        with path.open("rb") as file_obj:
            encoder = httpx.Request(
                "POST",
                "https://multipart.invalid",
                files={part_name: (path.name, file_obj)},
            )
            body = encoder.read()

        await self.make_route_request_content_type(
            tool,
            *values,
            content_type=encoder.headers["Content-Type"],
            content=body,
        )

    async def route_raw_body(
        self, tool: str, *values: object, content_type: str, payload: bytes
    ) -> None:
        """Perform a tool's route with payload as the whole body, no JSON around it."""
        await self.make_route_request_content_type(
            tool, *values, content_type=content_type, content=payload
        )

    async def route_raw_body_read(
        self, tool: str, *values: object, accept: str
    ) -> bytes:
        """Perform a tool's route and answer with the body as it arrived.

        The route sends the resource itself rather than a JSON document, so
        there is nothing to decode and the bytes are the answer.
        """
        response = await self.make_route_request_content_type(
            tool, *values, accept=accept
        )
        return response.content

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
        return parse_instance(data)

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
            retry_sec=data.get("retry_sec", 0),
            master_ips=data.get("master_ips", []),
            axfr_ips=data.get("axfr_ips", []),
            expire_sec=data.get("expire_sec", 0),
            refresh_sec=data.get("refresh_sec", 0),
            ttl_sec=data.get("ttl_sec", 0),
            group=data.get("group", ""),
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
            service=data.get("service") or "",
            protocol=data.get("protocol") or "",
            tag=data.get("tag") or "",
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
            rev_note=data.get("rev_note", ""),
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
        self,
        api_url: str,
        token: str,
        retry_config: RetryConfig | None = None,
        object_storage: ObjectStorageConfig | None = None,
    ) -> None:
        self.retry_config = retry_config or RetryConfig()
        # Carried on the client because the Object Storage execute hook is handed
        # a client and no config, and widening that hook signature is an emitter
        # change. Go's client carries it for the same reason.
        self.object_storage = object_storage or ObjectStorageConfig()
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

    async def route_raw(
        self,
        tool: str,
        *values: object,
        body: dict[str, Any] | None = None,
        query: str | None = None,
        retry: bool = True,
    ) -> Any:
        """Resolve a tool's route as raw decoded JSON, retrying by default.

        Callers creating non-idempotent resources select one protected
        attempt, the same trade post_raw documents. The partial exists because
        the retry executor forwards positional arguments only, and body and
        query are keyword-only on the client side.
        """
        execute = self._execute_with_retry if retry else self._execute_without_retry
        call = functools.partial(
            self.client.route_raw, tool, *values, body=body, query=query
        )
        result: Any = await execute(call)
        return result

    async def route_call(self, tool: str, *values: object, retry: bool = True) -> None:
        """Perform a tool's route with no body and nothing to decode, retrying.

        The destroy tier's whole request: a caller names its tool and the ids,
        and the route, the method, and the retry policy all come from the
        contract. Callers whose route is not replayable select one protected
        attempt, the same trade route_raw documents.
        """
        execute = self._execute_with_retry if retry else self._execute_without_retry
        await execute(self.client.route_call, tool, *values)

    async def route_multipart(
        self,
        tool: str,
        *values: object,
        part_name: str,
        file_path: str,
        retry: bool = True,
    ) -> None:
        """Frame a local file as a form and send it, retrying by default.

        The partial exists because the retry executor forwards positional
        arguments only, and the form's own parameters are keyword-only on the
        client side.
        """
        execute = self._execute_with_retry if retry else self._execute_without_retry
        await execute(
            functools.partial(
                self.client.route_multipart,
                tool,
                *values,
                part_name=part_name,
                file_path=file_path,
            )
        )

    async def route_raw_body(
        self,
        tool: str,
        *values: object,
        content_type: str,
        payload: bytes,
        retry: bool = True,
    ) -> None:
        """Send a bare body under one content type, retrying by default."""
        execute = self._execute_with_retry if retry else self._execute_without_retry
        await execute(
            functools.partial(
                self.client.route_raw_body,
                tool,
                *values,
                content_type=content_type,
                payload=payload,
            )
        )

    async def route_raw_body_read(
        self, tool: str, *values: object, accept: str, retry: bool = True
    ) -> bytes:
        """Read a route whose answer is the resource itself, retrying by default."""
        execute = self._execute_with_retry if retry else self._execute_without_retry
        result: bytes = await execute(
            functools.partial(
                self.client.route_raw_body_read, tool, *values, accept=accept
            )
        )
        return result

    async def get_profile(self) -> Profile:
        """Get Linode user profile with retry."""
        result: Profile = await self._execute_with_retry(self.client.get_profile)
        return result

    async def get_profile_grants(self) -> Grants:
        """Get /profile/grants with retry. PATs return an empty Grants."""
        result: Grants = await self._execute_with_retry(self.client.get_profile_grants)
        return result

    async def list_instance_configs(
        self,
        linode_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List configuration profiles for a Linode instance with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_instance_configs(
                linode_id, page=page, page_size=page_size
            )
        )
        return result

    async def get_instance_config(
        self, linode_id: int, config_id: int
    ) -> dict[str, Any]:
        """Get a Linode instance configuration profile with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_config, linode_id, config_id
        )
        return result

    async def get_instance_config_interface(
        self, linode_id: int, config_id: int, interface_id: int
    ) -> dict[str, Any]:
        """Get a Linode instance configuration profile interface with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_config_interface,
            linode_id,
            config_id,
            interface_id,
        )
        return result

    async def get_instance_interface_settings(self, linode_id: int) -> dict[str, Any]:
        """List Linode instance interface settings with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_interface_settings, linode_id
        )
        return result

    async def get_instance_interface(
        self, linode_id: int, interface_id: int
    ) -> dict[str, Any]:
        """Get Linode instance interface with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_instance_interface, linode_id, interface_id
        )
        return result

    async def get_instance(self, instance_id: int) -> Instance:
        """Get a specific Linode instance with retry."""
        result: Instance = await self._execute_with_retry(
            self.client.get_instance, instance_id
        )
        return result

    async def get_account(self) -> Account:
        """Get Linode account information with retry."""
        result: Account = await self._execute_with_retry(self.client.get_account)
        return result

    async def get_account_settings(self) -> dict[str, Any]:
        """Get account settings with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_settings
        )
        return result

    async def get_account_event(self, event_id: int) -> dict[str, Any]:
        """Get an account event with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_event, event_id
        )
        return result

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

    async def get_account_oauth_client(self, client_id: str) -> dict[str, Any]:
        """Get an OAuth client by client ID with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_account_oauth_client, client_id
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

    async def get_type(self, type_id: str) -> InstanceType:
        """Get a Linode instance type with retry."""
        result: InstanceType = await self._execute_with_retry(
            lambda: self.client.get_type(type_id)
        )
        return result

    async def get_volume(self, volume_id: int) -> Volume:
        """Get volume with retry."""
        result: Volume = await self._execute_with_retry(
            self.client.get_volume, volume_id
        )
        return result

    async def get_image_sharegroup(self, sharegroup_id: str) -> dict[str, Any]:
        """Get a single image share group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_image_sharegroup(sharegroup_id)
        )
        return result

    async def get_image_sharegroup_by_token(self, token_uuid: str) -> dict[str, Any]:
        """Get an image share group by token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.get_image_sharegroup_by_token(token_uuid)
        )
        return result

    async def get_image(self, image_id: str) -> Image:
        """Get a single Linode image with retry."""
        result: Image = await self._execute_with_retry(
            lambda: self.client.get_image(image_id)
        )
        return result

    async def get_ssh_key(self, ssh_key_id: int) -> SSHKey:
        """Get a specific SSH key with retry."""
        result: SSHKey = await self._execute_with_retry(
            self.client.get_ssh_key, ssh_key_id
        )
        return result

    async def get_domain(self, domain_id: int) -> Domain:
        """Get a specific domain with retry."""
        result: Domain = await self._execute_with_retry(
            self.client.get_domain, domain_id
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

    async def get_firewall_rules(self, firewall_id: int) -> FirewallRules:
        """Get firewall rules with retry."""
        result: FirewallRules = await self._execute_with_retry(
            self.client.get_firewall_rules, firewall_id
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

    async def list_vlans(
        self,
        page: int | None = None,
        page_size: int | None = None,
    ) -> list[dict[str, Any]]:
        """List VLANs with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            lambda: self.client.list_vlans(page=page, page_size=page_size)
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

    async def get_managed_credential(self, credential_id: int) -> dict[str, Any]:
        """Get a Managed credential with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_managed_credential, credential_id
        )
        return result

    async def get_managed_service(self, service_id: int) -> dict[str, Any]:
        """Get a Managed service monitor with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_managed_service, service_id
        )
        return result

    async def get_managed_linode_settings(self, linode_id: int) -> dict[str, Any]:
        """Get Managed settings for a Linode with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_managed_linode_settings, linode_id
        )
        return result

    async def get_managed_contact(self, contact_id: int) -> dict[str, Any]:
        """Get a Managed contact with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_managed_contact, contact_id
        )
        return result

    async def get_support_ticket(self, ticket_id: int) -> dict[str, Any]:
        """Get a support ticket with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_support_ticket, ticket_id
        )
        return result

    async def get_nodebalancer(self, nodebalancer_id: int) -> NodeBalancer:
        """Get a specific NodeBalancer with retry."""
        result: NodeBalancer = await self._execute_with_retry(
            self.client.get_nodebalancer, nodebalancer_id
        )
        return result

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

    async def get_stackscript(self, stackscript_id: int | str) -> StackScript:
        """Get a StackScript by ID with retry."""
        result: StackScript = await self._execute_with_retry(
            lambda: self.client.get_stackscript(stackscript_id)
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

    async def get_object_storage_key(self, key_id: int) -> dict[str, Any]:
        """Get a specific Object Storage access key with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_storage_key, key_id
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

    async def get_object_acl(
        self, region: str, label: str, name: str
    ) -> dict[str, Any]:
        """Get object ACL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_object_acl, region, label, name
        )
        return result

    async def get_bucket_ssl(self, region: str, label: str) -> dict[str, Any]:
        """Get bucket SSL status with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_bucket_ssl, region, label
        )
        return result

    async def get_profile_token(self, token_id: int) -> dict[str, Any]:
        """Get a profile token with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_token, token_id
        )
        return result

    async def get_profile_device(self, device_id: int) -> dict[str, Any]:
        """Get a profile trusted device with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_device, device_id
        )
        return result

    async def get_longview_plan(self) -> dict[str, Any]:
        """Get the Longview plan with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_longview_plan
        )
        return result

    async def get_longview_client(self, client_id: object) -> dict[str, Any]:
        """Get a Longview client with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_longview_client, client_id
        )
        return result

    async def get_profile_app(self, app_id: int) -> dict[str, Any]:
        """Get a profile OAuth app authorization with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_profile_app, app_id
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

    async def get_lke_cluster(self, cluster_id: int) -> dict[str, Any]:
        """Get a specific LKE cluster with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_cluster, cluster_id
        )
        return result

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

    async def get_lke_node(self, cluster_id: int, node_id: str) -> dict[str, Any]:
        """Get a specific LKE node with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_node, cluster_id, node_id
        )
        return result

    async def get_lke_control_plane_acl(self, cluster_id: int) -> dict[str, Any]:
        """Get LKE control plane ACL with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_lke_control_plane_acl, cluster_id
        )
        return result

    async def get_vpc(self, vpc_id: int) -> dict[str, Any]:
        """Get a specific VPC with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_vpc, vpc_id
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

    async def get_placement_group(self, group_id: int) -> dict[str, Any]:
        """Get placement group with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_placement_group, group_id
        )
        return result

    async def get_ipv6_range(self, ipv6_range: str) -> dict[str, Any]:
        """Get IPv6 range with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_ipv6_range, ipv6_range
        )
        return result

    async def get_reserved_ip(self, address: str) -> dict[str, Any]:
        """Get a reserved public IPv4 address with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            self.client.get_reserved_ip, address
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

    async def list_instance_disks(
        self,
        instance_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> list[dict[str, Any]]:
        """List a page of instance disks with retry."""
        result: list[dict[str, Any]] = await self._execute_with_retry(
            self.client.list_instance_disks, instance_id, page, page_size
        )
        return result

    async def list_instance_volumes(
        self,
        linode_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List Linode instance volumes with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_instance_volumes(
                linode_id, page=page, page_size=page_size
            )
        )
        return result

    async def list_instance_firewalls(
        self,
        linode_id: int,
        page: int | None = None,
        page_size: int | None = None,
    ) -> dict[str, Any]:
        """List Linode instance firewalls with retry."""
        result: dict[str, Any] = await self._execute_with_retry(
            lambda: self.client.list_instance_firewalls(
                linode_id, page=page, page_size=page_size
            )
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

    async def _execute_without_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        """Execute one protected network attempt without replaying mutations."""
        self._circuit.allow()

        async with self._request_semaphore:
            await self._limiter.wait()
            try:
                result = await func(*args)
            except Exception as exc:
                if self._should_retry(exc):
                    self._circuit.record_failure()
                raise

            self._circuit.record_success()
            return result

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
