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
    "APIError",
    "Account",
    "Addons",
    "Alerts",
    "Backup",
    "Backups",
    "BackupsAddon",
    "Client",
    "Image",
    "Instance",
    "InstanceType",
    "LinodeError",
    "NetworkError",
    "Price",
    "Profile",
    "Promo",
    "Region",
    "Resolver",
    "RetryConfig",
    "RetryableClient",
    "RetryableError",
    "Schedule",
    "Specs",
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

    async def _make_request(self, method: str, endpoint: str) -> httpx.Response:
        """Make an HTTP request to the Linode API."""
        url = self.base_url + endpoint
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "User-Agent": "LinodeMCP/1.0",
        }

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
