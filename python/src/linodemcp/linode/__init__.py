"""Linode API client."""

import asyncio
import secrets
from dataclasses import dataclass
from typing import Any

import httpx

# HTTP status code constants
HTTP_BAD_REQUEST = 400
HTTP_UNAUTHORIZED = 401
HTTP_FORBIDDEN = 403
HTTP_TOO_MANY_REQUESTS = 429
HTTP_SERVER_ERROR = 500
HTTP_SERVER_ERROR_MAX = 600

__all__ = [
    "UDF",
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
    "LinodeError",
    "NetworkError",
    "NodeBalancer",
    "Price",
    "Profile",
    "Promo",
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
    "Volume",
    "is_retryable",
]


class LinodeError(Exception):
    """Base Linode error."""


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
    """Linode user profile."""

    username: str
    email: str
    timezone: str
    email_notifications: bool
    restricted: bool
    two_factor_auth: bool
    uid: int


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


class Client:
    """Linode API client."""

    def __init__(self, api_url: str, token: str) -> None:
        self.base_url = api_url
        self.token = token
        self.client = httpx.AsyncClient(
            timeout=30.0,
            limits=httpx.Limits(max_keepalive_connections=10, max_connections=10),
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

    async def get_profile(self) -> Profile:
        """Get Linode user profile."""
        try:
            response = await self._make_request("GET", "/profile")
            data = response.json()
            return Profile(
                username=data["username"],
                email=data["email"],
                timezone=data["timezone"],
                email_notifications=data["email_notifications"],
                restricted=data["restricted"],
                two_factor_auth=data["two_factor_auth"],
                uid=data["uid"],
            )
        except httpx.HTTPError as e:
            raise NetworkError("GetProfile", e) from e

    async def list_instances(self) -> list[Instance]:
        """List Linode instances."""
        try:
            response = await self._make_request("GET", "/linode/instances")
            data = response.json()
            return [self._parse_instance(inst) for inst in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListInstances", e) from e

    async def get_instance(self, instance_id: int) -> Instance:
        """Get a specific Linode instance."""
        endpoint = f"/linode/instances/{instance_id}"
        try:
            response = await self._make_request("GET", endpoint)
            data = response.json()
            return self._parse_instance(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetInstance", e) from e

    async def get_account(self) -> Account:
        """Get Linode account information."""
        try:
            response = await self._make_request("GET", "/account")
            data = response.json()
            return self._parse_account(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetAccount", e) from e

    async def list_regions(self) -> list[Region]:
        """List Linode regions."""
        try:
            response = await self._make_request("GET", "/regions")
            data = response.json()
            return [self._parse_region(r) for r in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListRegions", e) from e

    async def list_types(self) -> list[InstanceType]:
        """List Linode instance types."""
        try:
            response = await self._make_request("GET", "/linode/types")
            data = response.json()
            return [self._parse_instance_type(t) for t in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListTypes", e) from e

    async def list_volumes(self) -> list[Volume]:
        """List Linode block storage volumes."""
        try:
            response = await self._make_request("GET", "/volumes")
            data = response.json()
            return [self._parse_volume(v) for v in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListVolumes", e) from e

    async def list_images(self) -> list[Image]:
        """List Linode images."""
        try:
            response = await self._make_request("GET", "/images")
            data = response.json()
            return [self._parse_image(i) for i in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListImages", e) from e

    # Stage 3: Extended read operations

    async def list_ssh_keys(self) -> list[SSHKey]:
        """List SSH keys."""
        try:
            response = await self._make_request("GET", "/profile/sshkeys")
            data = response.json()
            return [self._parse_ssh_key(k) for k in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListSSHKeys", e) from e

    async def list_domains(self) -> list[Domain]:
        """List domains."""
        try:
            response = await self._make_request("GET", "/domains")
            data = response.json()
            return [self._parse_domain(d) for d in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListDomains", e) from e

    async def get_domain(self, domain_id: int) -> Domain:
        """Get a specific domain."""
        endpoint = f"/domains/{domain_id}"
        try:
            response = await self._make_request("GET", endpoint)
            data = response.json()
            return self._parse_domain(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetDomain", e) from e

    async def list_domain_records(self, domain_id: int) -> list[DomainRecord]:
        """List domain records for a domain."""
        endpoint = f"/domains/{domain_id}/records"
        try:
            response = await self._make_request("GET", endpoint)
            data = response.json()
            return [self._parse_domain_record(r) for r in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListDomainRecords", e) from e

    async def list_firewalls(self) -> list[Firewall]:
        """List firewalls."""
        try:
            response = await self._make_request("GET", "/networking/firewalls")
            data = response.json()
            return [self._parse_firewall(f) for f in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListFirewalls", e) from e

    async def list_nodebalancers(self) -> list[NodeBalancer]:
        """List NodeBalancers."""
        try:
            response = await self._make_request("GET", "/nodebalancers")
            data = response.json()
            return [self._parse_nodebalancer(nb) for nb in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListNodeBalancers", e) from e

    async def get_nodebalancer(self, nodebalancer_id: int) -> NodeBalancer:
        """Get a specific NodeBalancer."""
        endpoint = f"/nodebalancers/{nodebalancer_id}"
        try:
            response = await self._make_request("GET", endpoint)
            data = response.json()
            return self._parse_nodebalancer(data)
        except httpx.HTTPError as e:
            raise NetworkError("GetNodeBalancer", e) from e

    async def list_stackscripts(self) -> list[StackScript]:
        """List StackScripts."""
        try:
            response = await self._make_request("GET", "/linode/stackscripts")
            data = response.json()
            return [self._parse_stackscript(s) for s in data.get("data", [])]
        except httpx.HTTPError as e:
            raise NetworkError("ListStackScripts", e) from e

    # Stage 4: Write operations

    async def create_ssh_key(self, label: str, ssh_key: str) -> SSHKey:
        """Create a new SSH key."""
        try:
            body = {"label": label, "ssh_key": ssh_key}
            response = await self._make_request("POST", "/profile/sshkeys", body)
            data = response.json()
            return self._parse_ssh_key(data)
        except httpx.HTTPError as e:
            raise NetworkError("CreateSSHKey", e) from e

    async def delete_ssh_key(self, ssh_key_id: int) -> None:
        """Delete an SSH key."""
        endpoint = f"/profile/sshkeys/{ssh_key_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteSSHKey", e) from e

    async def boot_instance(
        self, instance_id: int, config_id: int | None = None
    ) -> None:
        """Boot an instance."""
        endpoint = f"/linode/instances/{instance_id}/boot"
        try:
            body: dict[str, Any] = {}
            if config_id is not None:
                body["config_id"] = config_id
            await self._make_request("POST", endpoint, body if body else None)
        except httpx.HTTPError as e:
            raise NetworkError("BootInstance", e) from e

    async def reboot_instance(
        self, instance_id: int, config_id: int | None = None
    ) -> None:
        """Reboot an instance."""
        endpoint = f"/linode/instances/{instance_id}/reboot"
        try:
            body: dict[str, Any] = {}
            if config_id is not None:
                body["config_id"] = config_id
            await self._make_request("POST", endpoint, body if body else None)
        except httpx.HTTPError as e:
            raise NetworkError("RebootInstance", e) from e

    async def shutdown_instance(self, instance_id: int) -> None:
        """Shutdown an instance."""
        endpoint = f"/linode/instances/{instance_id}/shutdown"
        try:
            await self._make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("ShutdownInstance", e) from e

    async def create_instance(
        self,
        region: str,
        instance_type: str,
        image: str | None = None,
        label: str | None = None,
        root_pass: str | None = None,
        authorized_keys: list[str] | None = None,
        authorized_users: list[str] | None = None,
        booted: bool = True,
        backups_enabled: bool = False,
        private_ip: bool = False,
        tags: list[str] | None = None,
    ) -> Instance:
        """Create a new Linode instance."""
        try:
            body: dict[str, Any] = {
                "region": region,
                "type": instance_type,
                "booted": booted,
                "backups_enabled": backups_enabled,
                "private_ip": private_ip,
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

            response = await self._make_request("POST", "/linode/instances", body)
            data = response.json()
            return self._parse_instance(data)
        except httpx.HTTPError as e:
            raise NetworkError("CreateInstance", e) from e

    async def delete_instance(self, instance_id: int) -> None:
        """Delete an instance."""
        endpoint = f"/linode/instances/{instance_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
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
        try:
            body = {
                "type": instance_type,
                "allow_auto_disk_resize": allow_auto_disk_resize,
                "migration_type": migration_type,
            }
            await self._make_request("POST", endpoint, body)
        except httpx.HTTPError as e:
            raise NetworkError("ResizeInstance", e) from e

    async def create_firewall(
        self,
        label: str,
        inbound_policy: str = "ACCEPT",
        outbound_policy: str = "ACCEPT",
    ) -> Firewall:
        """Create a new firewall."""
        try:
            body = {
                "label": label,
                "rules": {
                    "inbound_policy": inbound_policy,
                    "outbound_policy": outbound_policy,
                },
            }
            response = await self._make_request("POST", "/networking/firewalls", body)
            data = response.json()
            return self._parse_firewall(data)
        except httpx.HTTPError as e:
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

            response = await self._make_request("PUT", endpoint, body)
            data = response.json()
            return self._parse_firewall(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateFirewall", e) from e

    async def delete_firewall(self, firewall_id: int) -> None:
        """Delete a firewall."""
        endpoint = f"/networking/firewalls/{firewall_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteFirewall", e) from e

    async def create_domain(
        self,
        domain: str,
        domain_type: str = "master",
        soa_email: str | None = None,
        description: str | None = None,
        tags: list[str] | None = None,
    ) -> Domain:
        """Create a new domain."""
        try:
            body: dict[str, Any] = {"domain": domain, "type": domain_type}
            if soa_email:
                body["soa_email"] = soa_email
            if description:
                body["description"] = description
            if tags:
                body["tags"] = tags

            response = await self._make_request("POST", "/domains", body)
            data = response.json()
            return self._parse_domain(data)
        except httpx.HTTPError as e:
            raise NetworkError("CreateDomain", e) from e

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

            response = await self._make_request("PUT", endpoint, body)
            data = response.json()
            return self._parse_domain(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateDomain", e) from e

    async def delete_domain(self, domain_id: int) -> None:
        """Delete a domain."""
        endpoint = f"/domains/{domain_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
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
        try:
            body: dict[str, Any] = {"type": record_type}
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

            response = await self._make_request("POST", endpoint, body)
            data = response.json()
            return self._parse_domain_record(data)
        except httpx.HTTPError as e:
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

            response = await self._make_request("PUT", endpoint, body)
            data = response.json()
            return self._parse_domain_record(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateDomainRecord", e) from e

    async def delete_domain_record(self, domain_id: int, record_id: int) -> None:
        """Delete a domain record."""
        endpoint = f"/domains/{domain_id}/records/{record_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
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
        try:
            body: dict[str, Any] = {"label": label, "size": size}
            if region:
                body["region"] = region
            if linode_id is not None:
                body["linode_id"] = linode_id
            if tags:
                body["tags"] = tags

            response = await self._make_request("POST", "/volumes", body)
            data = response.json()
            return self._parse_volume(data)
        except httpx.HTTPError as e:
            raise NetworkError("CreateVolume", e) from e

    async def attach_volume(
        self,
        volume_id: int,
        linode_id: int,
        config_id: int | None = None,
        persist_across_boots: bool = False,
    ) -> Volume:
        """Attach a volume to an instance."""
        endpoint = f"/volumes/{volume_id}/attach"
        try:
            body: dict[str, Any] = {
                "linode_id": linode_id,
                "persist_across_boots": persist_across_boots,
            }
            if config_id is not None:
                body["config_id"] = config_id

            response = await self._make_request("POST", endpoint, body)
            data = response.json()
            return self._parse_volume(data)
        except httpx.HTTPError as e:
            raise NetworkError("AttachVolume", e) from e

    async def detach_volume(self, volume_id: int) -> None:
        """Detach a volume from an instance."""
        endpoint = f"/volumes/{volume_id}/detach"
        try:
            await self._make_request("POST", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DetachVolume", e) from e

    async def resize_volume(self, volume_id: int, size: int) -> Volume:
        """Resize a volume."""
        endpoint = f"/volumes/{volume_id}/resize"
        try:
            body = {"size": size}
            response = await self._make_request("POST", endpoint, body)
            data = response.json()
            return self._parse_volume(data)
        except httpx.HTTPError as e:
            raise NetworkError("ResizeVolume", e) from e

    async def delete_volume(self, volume_id: int) -> None:
        """Delete a volume."""
        endpoint = f"/volumes/{volume_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteVolume", e) from e

    async def create_nodebalancer(
        self,
        region: str,
        label: str | None = None,
        client_conn_throttle: int = 0,
        tags: list[str] | None = None,
    ) -> NodeBalancer:
        """Create a new NodeBalancer."""
        try:
            body: dict[str, Any] = {
                "region": region,
                "client_conn_throttle": client_conn_throttle,
            }
            if label:
                body["label"] = label
            if tags:
                body["tags"] = tags

            response = await self._make_request("POST", "/nodebalancers", body)
            data = response.json()
            return self._parse_nodebalancer(data)
        except httpx.HTTPError as e:
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
        try:
            body: dict[str, Any] = {}
            if label:
                body["label"] = label
            if client_conn_throttle is not None:
                body["client_conn_throttle"] = client_conn_throttle
            if tags is not None:
                body["tags"] = tags

            response = await self._make_request("PUT", endpoint, body)
            data = response.json()
            return self._parse_nodebalancer(data)
        except httpx.HTTPError as e:
            raise NetworkError("UpdateNodeBalancer", e) from e

    async def delete_nodebalancer(self, nodebalancer_id: int) -> None:
        """Delete a NodeBalancer."""
        endpoint = f"/nodebalancers/{nodebalancer_id}"
        try:
            await self._make_request("DELETE", endpoint)
        except httpx.HTTPError as e:
            raise NetworkError("DeleteNodeBalancer", e) from e

    async def _make_request(
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
        except (ValueError, KeyError):
            pass

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
        )

    def _parse_account(self, data: dict[str, Any]) -> Account:
        """Parse account data from API response."""
        promotions = []
        for promo_data in data.get("active_promotions", []):
            promotions.append(
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
            )

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


class RetryableClient:
    """Linode API client with retry functionality."""

    def __init__(
        self, api_url: str, token: str, retry_config: RetryConfig | None = None
    ) -> None:
        self.client = Client(api_url, token)
        self.retry_config = retry_config or RetryConfig()

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

    async def get_account(self) -> Account:
        """Get Linode account information with retry."""
        result: Account = await self._execute_with_retry(self.client.get_account)
        return result

    async def list_regions(self) -> list[Region]:
        """List Linode regions with retry."""
        result: list[Region] = await self._execute_with_retry(self.client.list_regions)
        return result

    async def list_types(self) -> list[InstanceType]:
        """List Linode instance types with retry."""
        result: list[InstanceType] = await self._execute_with_retry(
            self.client.list_types
        )
        return result

    async def list_volumes(self) -> list[Volume]:
        """List Linode volumes with retry."""
        result: list[Volume] = await self._execute_with_retry(self.client.list_volumes)
        return result

    async def list_images(self) -> list[Image]:
        """List Linode images with retry."""
        result: list[Image] = await self._execute_with_retry(self.client.list_images)
        return result

    # Stage 3: Extended read operations

    async def list_ssh_keys(self) -> list[SSHKey]:
        """List SSH keys with retry."""
        result: list[SSHKey] = await self._execute_with_retry(self.client.list_ssh_keys)
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

    async def list_domain_records(self, domain_id: int) -> list[DomainRecord]:
        """List domain records with retry."""
        result: list[DomainRecord] = await self._execute_with_retry(
            self.client.list_domain_records, domain_id
        )
        return result

    async def list_firewalls(self) -> list[Firewall]:
        """List firewalls with retry."""
        result: list[Firewall] = await self._execute_with_retry(
            self.client.list_firewalls
        )
        return result

    async def list_nodebalancers(self) -> list[NodeBalancer]:
        """List NodeBalancers with retry."""
        result: list[NodeBalancer] = await self._execute_with_retry(
            self.client.list_nodebalancers
        )
        return result

    async def get_nodebalancer(self, nodebalancer_id: int) -> NodeBalancer:
        """Get a specific NodeBalancer with retry."""
        result: NodeBalancer = await self._execute_with_retry(
            self.client.get_nodebalancer, nodebalancer_id
        )
        return result

    async def list_stackscripts(self) -> list[StackScript]:
        """List StackScripts with retry."""
        result: list[StackScript] = await self._execute_with_retry(
            self.client.list_stackscripts
        )
        return result

    # Stage 4: Write operations with retry

    async def create_ssh_key(self, label: str, ssh_key: str) -> SSHKey:
        """Create SSH key with retry."""
        result: SSHKey = await self._execute_with_retry(
            self.client.create_ssh_key, label, ssh_key
        )
        return result

    async def delete_ssh_key(self, ssh_key_id: int) -> None:
        """Delete SSH key with retry."""
        await self._execute_with_retry(self.client.delete_ssh_key, ssh_key_id)

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
        image: str | None = None,
        label: str | None = None,
        root_pass: str | None = None,
        authorized_keys: list[str] | None = None,
        authorized_users: list[str] | None = None,
        booted: bool = True,
        backups_enabled: bool = False,
        private_ip: bool = False,
        tags: list[str] | None = None,
    ) -> Instance:
        """Create instance with retry."""
        result: Instance = await self._execute_with_retry(
            self.client.create_instance,
            region,
            instance_type,
            image,
            label,
            root_pass,
            authorized_keys,
            authorized_users,
            booted,
            backups_enabled,
            private_ip,
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

    async def _execute_with_retry(self, func: Any, *args: Any) -> Any:
        """Execute a function with retry logic."""
        last_error: Exception | None = None

        for attempt in range(self.retry_config.max_retries + 1):
            if attempt > 0:
                delay = self._calculate_delay(attempt)
                await asyncio.sleep(delay)

            try:
                return await func(*args)
            except Exception as e:
                last_error = e
                if attempt == self.retry_config.max_retries:
                    break
                if not self._should_retry(e):
                    raise

        raise last_error if last_error else LinodeError("Unknown error during retry")

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
