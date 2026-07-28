"""No-replay coverage for the non-idempotent creates that reach ``post_raw``.

Each route here answers a POST by assigning a new ID, so replaying it after a
transient failure leaves a duplicate resource the caller never learns about.
The Go twins live in ``go/internal/linode/create_no_replay_test.go`` and the two
tables are meant to stay in step.
"""

from collections.abc import Awaitable, Callable
from typing import Any, TypeVar, cast
from unittest.mock import AsyncMock, patch

import pytest
from mcp.types import TextContent

from linodemcp.config import Config
from linodemcp.linode import (
    APIError,
    CircuitBreaker,
    CircuitOpenError,
    Domain,
    RetryableClient,
)
from linodemcp.tools.linode_domain_records import handle_linode_domain_record_create
from linodemcp.tools.linode_domains_write import (
    handle_linode_domain_clone,
    handle_linode_domain_create,
    handle_linode_domain_import,
)
from linodemcp.tools.linode_stackscripts import handle_linode_stackscript_create
from linodemcp.tools.linode_volumes_write import (
    handle_linode_volume_clone,
    handle_linode_volume_create,
)

T = TypeVar("T")

Handler = Callable[[dict[str, Any], Config], Awaitable[list[TextContent]]]

# A 500 rather than a 429 because Python's _should_retry replays both through a
# POST. Go's isRetryable declines a 5xx on a non-idempotent method, so the two
# languages were not equally exposed before the guard landed: Python replayed
# every retryable class, Go only the rate limit.
_TRANSIENT = APIError(500, "upstream failure")


class _FailingRetryableClient(RetryableClient):
    """Retryable client double that fails the test if replay retry is entered."""

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        self.retry_calls = 0

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        del func, args
        self.retry_calls += 1
        raise AssertionError("non-idempotent create must not use replay retry")


_CREATES: list[tuple[str, Handler, dict[str, Any], str, dict[str, Any]]] = [
    (
        "domain_create",
        handle_linode_domain_create,
        {
            "domain": "example.com",
            "type": "master",
            "soa_email": "admin@example.com",
            "confirm": True,
        },
        "/domains",
        {"domain": "example.com", "type": "master", "soa_email": "admin@example.com"},
    ),
    (
        "domain_import",
        handle_linode_domain_import,
        {
            "domain": "example.com",
            "remote_nameserver": "ns1.example.net",
            "confirm": True,
        },
        "/domains/import",
        {"domain": "example.com", "remote_nameserver": "ns1.example.net"},
    ),
    (
        "domain_clone",
        handle_linode_domain_clone,
        {"domain_id": 12345, "domain": "clone.example.com", "confirm": True},
        "/domains/12345/clone",
        {"domain": "clone.example.com"},
    ),
    (
        "domain_record_create",
        handle_linode_domain_record_create,
        {
            "domain_id": 12345,
            "type": "A",
            "name": "www",
            "target": "8.8.8.8",
            "confirm": True,
        },
        "/domains/12345/records",
        {"type": "A", "name": "www", "target": "8.8.8.8"},
    ),
    (
        "volume_create",
        handle_linode_volume_create,
        {"label": "my-volume", "region": "us-east", "confirm": True},
        "/volumes",
        {"label": "my-volume", "region": "us-east"},
    ),
    (
        "volume_clone",
        handle_linode_volume_clone,
        {"volume_id": 12345, "label": "my-volume-clone", "confirm": True},
        "/volumes/12345/clone",
        {"label": "my-volume-clone"},
    ),
    (
        "stackscript_create",
        handle_linode_stackscript_create,
        {
            "label": "my-script",
            "images": ["linode/ubuntu22.04"],
            "script": "#!/bin/bash",
            "confirm": True,
        },
        "/linode/stackscripts",
        {
            "label": "my-script",
            "images": ["linode/ubuntu22.04"],
            "script": "#!/bin/bash",
        },
    ),
]


@pytest.mark.parametrize(
    ("name", "handler", "arguments", "endpoint", "body"),
    _CREATES,
    ids=[case[0] for case in _CREATES],
)
async def test_create_does_not_replay_transient_failure(
    sample_config: Config,
    name: str,
    handler: Handler,
    arguments: dict[str, Any],
    endpoint: str,
    body: dict[str, Any],
) -> None:
    """One attempt reaches the API and the transient failure reaches the caller."""
    del name
    client = _FailingRetryableClient()
    post_raw = AsyncMock(side_effect=_TRANSIENT)
    cast("Any", client.client).post_raw = post_raw

    try:
        with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
            result = await handler(arguments, sample_config)
    finally:
        await client.close()

    assert client.retry_calls == 0
    post_raw.assert_awaited_once_with(endpoint, body)
    assert "upstream failure" in result[0].text
    assert result[0].text.startswith("Failed to ")


class _OpenCircuitRetryableClient(RetryableClient):
    """Retryable client double whose breaker is already open.

    The breaker is installed in __init__ so the assignment stays inside the
    class hierarchy that declares it.
    """

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        breaker = CircuitBreaker(1, 3600.0)
        breaker.record_failure()
        self._circuit = breaker


class _RetryPathRetryableClient(RetryableClient):
    """Double that records the replay path instead of sleeping through it."""

    def __init__(self) -> None:
        super().__init__("https://api.linode.com/v4", "test-token")
        self.retry_calls = 0

    async def _execute_with_retry(
        self, func: Callable[..., Awaitable[T]], *args: Any
    ) -> T:
        del func, args
        self.retry_calls += 1
        raise _TRANSIENT


# The typed RetryableClient wrappers, which the tools reach instead of post_raw.
# Argument tuples are the required positionals only; the wrapper fills in its
# own defaults on the way to the inner client. The clone and add entries belong
# here because they allocate a new server-assigned resource exactly the way the
# create entries do.
#
# Entries whose wrapper delegates straight to the inner client, with no
# _execute_* call at all, are protected too and belong here: that bypass is the
# other no-replay idiom in the client, and a table that only recognized
# _execute_without_retry would read those as unprotected.
_TYPED_NO_REPLAY: list[tuple[str, tuple[Any, ...], dict[str, Any]]] = [
    ("create_ssh_key", ("my-key", "ssh-rsa AAAA"), {}),
    ("create_instance_raw", ("us-east", "g6-nanode-1", 42), {}),
    ("create_firewall_raw", ("my-fw",), {}),
    ("create_object_storage_key", ("my-key",), {}),
    ("create_lke_cluster", ("my-cluster", "us-east", "1.29", []), {}),
    ("create_lke_node_pool", (123, "g6-standard-1", 3), {}),
    ("create_vpc", ("my-vpc", "us-east"), {}),
    ("create_vpc_subnet", (123, "my-subnet", "10.0.0.0/24"), {}),
    ("create_instance_config", (123, "boot-config", {"sda": {"disk_id": 1}}), {}),
    ("create_instance_disk", (123, "my-disk", 1024), {}),
    ("create_instance_backup", (123,), {}),
    ("clone_instance_raw", (123,), {}),
    ("clone_instance_disk", (123, 456), {}),
    ("add_instance_config_interface", (123, 456, {"purpose": "public"}), {}),
    ("create_firewall_device", (123, 456, "linode"), {}),
    ("create_monitor_service_token", ("linode", [123]), {}),
    ("create_managed_contact", (), {"email": "ops@example.com", "name": "Ops"}),
    (
        "create_managed_service",
        (),
        {
            "label": "svc",
            "service_type": "url",
            "address": "https://example.com",
            "timeout": 30,
        },
    ),
    (
        "create_monitor_service_alert_definition",
        ("linode",),
        {
            "label": "alert",
            "severity": 1,
            "rule_criteria": {},
            "trigger_conditions": {},
            "channel_ids": [1],
        },
    ),
    ("create_nodebalancer_config", (123, {"port": 80}), {}),
    ("create_nodebalancer_config_node", (123, 456, {"address": "192.0.2.10:80"}), {}),
    ("create_reserved_ip", ("us-east",), {}),
    ("create_mysql_database_instance", ({"label": "db"},), {}),
    ("create_postgresql_database_instance", ({"label": "db"},), {}),
    ("create_account_child_account_token", ("euuid-1",), {}),
    ("create_account_user", ("user", "user@example.com", False), {}),
    ("create_account_oauth_client", ("app", "https://example.com/cb"), {}),
    ("create_account_payment_method", ("credit_card", {}, False), {}),
    ("create_account_payment", ("10.00",), {}),
    ("create_account_service_transfer", ([123],), {}),
    ("create_image_sharegroup", ("grp",), {}),
    ("create_image_sharegroup_token", ("sharegroup-uuid",), {}),
    ("create_longview_client", ("lv",), {}),
    ("create_managed_credential", (), {"label": "cred", "password": "pw"}),
    ("clone_domain", (123, "clone.example.com"), {}),
    ("import_domain", ("example.com", "ns1.example.net"), {}),
]

# The two creates that still replay. Neither can leave a second resource
# behind: the bucket is identified by the region and label the caller chose,
# and the presigned URL is a signature the API computes rather than an object
# it stores.
_TYPED_RETRY_SAFE: list[tuple[str, tuple[Any, ...]]] = [
    ("create_object_storage_bucket", ("my-bucket", "us-east")),
    ("create_presigned_url", ("us-east", "my-bucket", "object.txt", "GET")),
]


@pytest.mark.parametrize(
    ("method", "args", "kwargs"),
    _TYPED_NO_REPLAY,
    ids=[case[0] for case in _TYPED_NO_REPLAY],
)
async def test_typed_create_does_not_replay_transient_failure(
    method: str, args: tuple[Any, ...], kwargs: dict[str, Any]
) -> None:
    """The wrapper makes one attempt and hands the failure straight back."""
    client = _FailingRetryableClient()
    inner = AsyncMock(side_effect=_TRANSIENT)
    setattr(cast("Any", client.client), method, inner)

    try:
        with pytest.raises(APIError):
            await getattr(client, method)(*args, **kwargs)
    finally:
        await client.close()

    assert client.retry_calls == 0
    inner.assert_awaited_once()


@pytest.mark.parametrize(
    ("method", "args", "kwargs"),
    _TYPED_NO_REPLAY,
    ids=[case[0] for case in _TYPED_NO_REPLAY],
)
async def test_typed_create_is_circuit_protected(
    method: str, args: tuple[Any, ...], kwargs: dict[str, Any]
) -> None:
    """An open breaker rejects the create before it reaches the network.

    Avoiding replay is not enough on its own. A wrapper that delegates straight
    to the inner client also avoids replay, but it skips the breaker, the rate
    limiter and the concurrency semaphore, so it can keep hammering an upstream
    that is already failing. This is the test that tells those two apart.
    """
    client = _OpenCircuitRetryableClient()
    inner = AsyncMock()
    setattr(cast("Any", client.client), method, inner)

    try:
        with pytest.raises(CircuitOpenError):
            await getattr(client, method)(*args, **kwargs)
    finally:
        await client.close()

    inner.assert_not_awaited()


@pytest.mark.parametrize(
    ("method", "args"),
    _TYPED_RETRY_SAFE,
    ids=[case[0] for case in _TYPED_RETRY_SAFE],
)
async def test_typed_create_still_replays_when_harmless(
    method: str, args: tuple[Any, ...]
) -> None:
    """Guard the two deliberate exceptions against a later blanket conversion."""
    client = _RetryPathRetryableClient()

    try:
        with pytest.raises(APIError):
            await getattr(client, method)(*args)
    finally:
        await client.close()

    assert client.retry_calls == 1


def _domain_fixture(domain: str) -> Domain:
    """Build the Domain the inner client hands back on a clone or import."""
    return Domain(
        id=456,
        domain=domain,
        type="master",
        status="active",
        soa_email="admin@example.com",
        description="",
        tags=[],
        created="2026-07-01T00:00:00",
        updated="2026-07-01T00:00:00",
    )


# The two domain wrappers whose no-replay conversion also had to keep handing
# the created Domain back. Both allocate a new server-assigned zone, and both
# return a value the tool renders, so a conversion that dropped the return would
# report success with an empty domain rather than the one the API just made.
_TYPED_NO_REPLAY_RETURNING_DOMAIN: list[tuple[str, tuple[Any, ...], str]] = [
    ("clone_domain", (123, "clone.example.com"), "clone.example.com"),
    ("import_domain", ("example.com", "ns1.example.net"), "example.com"),
]


@pytest.mark.parametrize(
    ("method", "args", "want_domain"),
    _TYPED_NO_REPLAY_RETURNING_DOMAIN,
    ids=[case[0] for case in _TYPED_NO_REPLAY_RETURNING_DOMAIN],
)
async def test_typed_create_returns_inner_result(
    method: str, args: tuple[Any, ...], want_domain: str
) -> None:
    """One attempt, no replay path, and the inner client's Domain comes back."""
    client = _FailingRetryableClient()
    created = _domain_fixture(want_domain)
    inner = AsyncMock(return_value=created)
    setattr(cast("Any", client.client), method, inner)

    try:
        result = await getattr(client, method)(*args)
    finally:
        await client.close()

    assert result is created
    assert result.domain == want_domain
    assert client.retry_calls == 0
    inner.assert_awaited_once_with(*args)
