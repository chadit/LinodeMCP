"""Focused passwordless Linode provisioning coverage."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any, cast
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import (
    CircuitOpenError,
    Client,
    RetryableClient,
    validate_instance_authentication,
)
from linodemcp.tools.linode_instance_actions import (
    create_linode_instance_rebuild_tool,
    handle_linode_instance_rebuild,
)
from linodemcp.tools.linode_instance_disks import (
    create_linode_instance_disk_create_tool,
    handle_linode_instance_disk_create,
)
from linodemcp.tools.linode_instance_write import (
    create_linode_instance_create_tool,
    handle_linode_instance_create,
)
from linodemcp.twostage import reset_plan_store, set_plan_store
from linodemcp.twostage.store import PlanStore

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.config import Config

_PASSWORD = "Passwordless123"
_CREATE = {"region": "us-east", "type": "g6-nanode-1", "firewall_id": 99}
_REBUILD = {"linode_id": 123, "image": "linode/ubuntu24.04"}
_DISK = {"linode_id": 123, "label": "boot", "size": 1024}
_ROUTES: list[tuple[Callable[..., Any], dict[str, Any], str, bool]] = [
    (handle_linode_instance_create, _CREATE, "create_instance_raw", False),
    (handle_linode_instance_rebuild, _REBUILD, "rebuild_instance", False),
    (handle_linode_instance_disk_create, _DISK, "create_instance_disk", True),
]
_AUTH = [
    pytest.param("root_pass", _PASSWORD, _PASSWORD, id="password"),
    pytest.param(
        "authorized_keys", ["ssh-ed25519 AAAA"], ["ssh-ed25519 AAAA"], id="key"
    ),
    pytest.param("authorized_users", ["alice"], ["alice"], id="user"),
]


@pytest.mark.parametrize(("handler", "base", "method", "disk_strings"), _ROUTES)
@pytest.mark.parametrize(("field", "value", "expected"), _AUTH)
async def test_handlers_accept_each_authentication_method(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    handler: Callable[..., Any],
    base: dict[str, Any],
    method: str,
    disk_strings: bool,
    field: str,
    value: object,
    expected: object,
) -> None:
    getattr(mock_linode_client, method).return_value = {
        "id": 123,
        "label": "created",
        "region": "us-east",
    }
    argument = value
    if disk_strings and isinstance(value, list):
        argument = ",".join(cast("list[str]", value))

    await handler({**base, field: argument, "confirm": True}, sample_config)

    assert getattr(mock_linode_client, method).call_args.kwargs[field] == expected


@pytest.mark.parametrize(("handler", "base", "method", "_disk_strings"), _ROUTES)
async def test_handlers_reject_missing_authentication_before_client_call(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    handler: Callable[..., Any],
    base: dict[str, Any],
    method: str,
    _disk_strings: bool,
) -> None:
    result = await handler({**base, "confirm": True}, sample_config)

    assert "at least one authentication method" in result[0].text
    getattr(mock_linode_client, method).assert_not_called()


@pytest.mark.parametrize(("handler", "base", "method", "disk_strings"), _ROUTES)
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_handlers_require_literal_true_confirmation(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    handler: Callable[..., Any],
    base: dict[str, Any],
    method: str,
    disk_strings: bool,
    confirm: object,
) -> None:
    users: str | list[str] = "alice" if disk_strings else ["alice"]
    result = await handler(
        {**base, "authorized_users": users, "confirm": confirm}, sample_config
    )

    assert "confirm=true" in result[0].text
    getattr(mock_linode_client, method).assert_not_called()


@pytest.mark.parametrize(("field", "value", "_expected"), _AUTH)
async def test_create_client_sends_exact_authentication_body(
    field: str, value: object, _expected: object
) -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 123, "label": "created", "region": "us-east"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.return_value = response
        await cast("Any", client.create_instance_raw)(
            "us-east", "g6-nanode-1", 99, **{field: value}
        )

        request.assert_awaited_once_with(
            "POST",
            "/linode/instances",
            {
                "region": "us-east",
                "type": "g6-nanode-1",
                "interface_generation": "linode",
                "interfaces": [
                    {
                        "public": {},
                        "default_route": {"ipv4": True, "ipv6": True},
                        "firewall_id": 99,
                    }
                ],
                field: value,
            },
        )

    await client.close()


@pytest.mark.parametrize(("field", "value", "_expected"), _AUTH)
async def test_rebuild_client_sends_exact_authentication_body(
    field: str, value: object, _expected: object
) -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 123}

    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.return_value = response
        await cast("Any", client.rebuild_instance)(
            123, "linode/ubuntu24.04", **{field: value}
        )

        request.assert_awaited_once_with(
            "POST",
            "/linode/instances/123/rebuild",
            {"image": "linode/ubuntu24.04", field: value},
        )

    await client.close()


@pytest.mark.parametrize(("field", "value", "_expected"), _AUTH)
async def test_disk_client_sends_exact_authentication_body(
    field: str, value: object, _expected: object
) -> None:
    client = Client("https://api.linode.com/v4", "test-token")
    response = MagicMock()
    response.json.return_value = {"id": 123}

    with patch.object(client, "make_request", new_callable=AsyncMock) as request:
        request.return_value = response
        await cast("Any", client.create_instance_disk)(
            123, "boot", 1024, **{field: value}
        )

        request.assert_awaited_once_with(
            "POST",
            "/linode/instances/123/disks",
            {"label": "boot", "size": 1024, field: value},
        )

    await client.close()


def test_schemas_expose_passwordless_authentication_and_confirmation() -> None:
    schemas = [
        create_linode_instance_create_tool()[0].inputSchema,
        create_linode_instance_rebuild_tool()[0].inputSchema,
        create_linode_instance_disk_create_tool()[0].inputSchema,
    ]
    for schema in schemas:
        assert {"root_pass", "authorized_keys", "authorized_users"} <= schema[
            "properties"
        ].keys()
        assert schema["properties"]["confirm"]["type"] == "boolean"
        assert "confirm" in schema["required"]
    assert "root_pass" not in schemas[1]["required"]


@pytest.mark.parametrize(
    ("root_pass", "authorized_keys", "authorized_users", "error"),
    [
        (None, "ssh-ed25519 AAAA", None, "authorized_keys must be a list"),
        (None, [1], None, "authorized_keys must contain only strings"),
        (None, None, "alice", "authorized_users must be a list"),
        (
            None,
            ["ssh-ed25519 AAAA", ""],
            None,
            "authorized_keys entries must not be empty",
        ),
        (
            None,
            None,
            ["alice", "   "],
            "authorized_users entries must not be empty",
        ),
        (None, [], [], "at least one authentication method"),
        (1, None, None, "root_pass must be a string"),
    ],
)
def test_authentication_validation_rejects_malformed_values(
    root_pass: object,
    authorized_keys: object,
    authorized_users: object,
    error: str,
) -> None:
    with pytest.raises((TypeError, ValueError), match=error):
        validate_instance_authentication(root_pass, authorized_keys, authorized_users)


@pytest.mark.parametrize(
    ("handler", "base", "method"),
    [
        (handle_linode_instance_rebuild, _REBUILD, "rebuild_instance"),
        (handle_linode_instance_disk_create, _DISK, "create_instance_disk"),
    ],
)
@pytest.mark.parametrize("linode_id", [True, 0, -1, "1/2", "1?x", ".."])
async def test_mutating_handlers_reject_malformed_linode_ids(
    mock_linode_client: AsyncMock,
    sample_config: Config,
    handler: Callable[..., Any],
    base: dict[str, Any],
    method: str,
    linode_id: object,
) -> None:
    result = await handler(
        {
            **base,
            "linode_id": linode_id,
            "authorized_users": "alice"
            if method == "create_instance_disk"
            else ["alice"],
            "confirm": True,
        },
        sample_config,
    )
    assert "integer" in result[0].text
    getattr(mock_linode_client, method).assert_not_called()


@pytest.mark.parametrize(
    ("method", "args"),
    [
        ("create_instance_raw", ("us-east", "g6-nanode-1", 99)),
        ("rebuild_instance", (123, "linode/ubuntu24.04")),
        ("create_instance_disk", (123, "boot", 1024)),
    ],
)
async def test_retryable_client_does_not_replay_provisioning_posts(
    method: str, args: tuple[object, ...]
) -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    controlled_client = cast("Any", client)
    operation = AsyncMock(side_effect=httpx.ConnectTimeout("transient"))
    limiter_wait = AsyncMock()
    setattr(client.client, method, operation)

    with (
        patch.object(controlled_client._limiter, "wait", limiter_wait),
        patch.object(controlled_client._circuit, "allow") as allow,
        patch.object(controlled_client._circuit, "record_failure") as record_failure,
        pytest.raises(httpx.ConnectTimeout),
    ):
        await getattr(client, method)(*args, authorized_users=["alice"])

    operation.assert_awaited_once()
    limiter_wait.assert_awaited_once()
    allow.assert_called_once()
    record_failure.assert_called_once()
    await client.close()


async def test_execute_once_fails_fast_when_circuit_is_open() -> None:
    client = RetryableClient("https://api.linode.com/v4", "test-token")
    operation = AsyncMock()
    controlled_client = cast("Any", client)
    with (
        patch.object(client.client, "create_instance_raw", operation),
        patch.object(
            controlled_client._circuit,
            "allow",
            side_effect=CircuitOpenError("circuit breaker open"),
        ),
        pytest.raises(CircuitOpenError),
    ):
        await client.create_instance_raw(
            "us-east",
            "g6-nanode-1",
            99,
            authorized_users=["alice"],
        )

    operation.assert_not_awaited()
    await client.close()


async def test_passwordless_rebuild_preserves_two_stage_plan_apply(
    mock_linode_client: AsyncMock, sample_config: Config
) -> None:
    mock_linode_client.get_instance.return_value = {"id": 123, "status": "offline"}
    mock_linode_client.list_instance_disks.return_value = []
    mock_linode_client.rebuild_instance.return_value = {"id": 123}
    arguments = {
        **_REBUILD,
        "authorized_users": ["alice"],
    }
    store = PlanStore()
    token = set_plan_store(store)
    try:
        plan = await handle_linode_instance_rebuild(
            {**arguments, "mode": "plan"}, sample_config
        )
        plan_id = json.loads(plan[0].text)["plan_id"]

        await handle_linode_instance_rebuild(
            {**arguments, "mode": "apply", "plan_id": plan_id}, sample_config
        )

        mock_linode_client.rebuild_instance.assert_awaited_once_with(
            123,
            image="linode/ubuntu24.04",
            root_pass=None,
            authorized_keys=None,
            authorized_users=["alice"],
        )
        assert await store.length() == 0
    finally:
        reset_plan_store(token)
