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
    "APIError",
    "Client",
    "Grants",
    "LinodeError",
    "NetworkError",
    "Profile",
    "RateLimiter",
    "RetryConfig",
    "RetryableClient",
    "RetryableError",
    "is_retryable",
    "parse_grants",
    "parse_profile",
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


def parse_profile(data: dict[str, Any]) -> Profile:
    """Parse a /profile body into the dataclass the scope validator reads."""
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


def _parse_grant(data: dict[str, Any]) -> Grant:
    """Parse a single per-resource OAuth grant entry."""
    return Grant(
        id=int(data.get("id", 0)),
        label=str(data.get("label", "")),
        permissions=str(data.get("permissions", "") or ""),
    )


def parse_grants(body: Any) -> Grants:
    """Parse the /profile/grants body into a structured Grants.

    PATs return an empty payload here; the returned Grants has all empty lists
    and a zero-valued GlobalGrants, and a body that is not an object reads the
    same way. The scope validator checks Profile.scopes first to decide which
    path to use.
    """
    if not isinstance(body, dict):
        return Grants()
    data = cast("dict[str, Any]", body)
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
            _parse_grant(cast("dict[str, Any]", item))
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
        try:
            if segment == DEFAULT_SURFACE_SEGMENT:
                if body is None:
                    return await self.make_request(route.method, endpoint)
                return await self.make_request(route.method, endpoint, body)

            base = base_for(self.base_url, segment)
            if body is None:
                return await self.make_request(route.method, endpoint, base=base)
            return await self.make_request(route.method, endpoint, body, base=base)
        except httpx.TransportError as exc:
            # The retry layer replays NetworkError under the tool's name, the
            # shape Go's wrapRequestError gives every routed primitive.
            raise NetworkError(tool, exc) from exc

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

        try:
            if content is None:
                response = await self.client.request(route.method, url, headers=headers)
            else:
                response = await self.client.request(
                    route.method, url, headers=headers, content=content
                )
        except httpx.TransportError as exc:
            raise NetworkError(tool, exc) from exc

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
