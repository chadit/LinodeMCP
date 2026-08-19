"""No-replay coverage for the non-idempotent creates that reach ``route_raw``.

Each route here answers a POST by assigning a new ID, so replaying it after a
transient failure leaves a duplicate resource the caller never learns about.
The table covers the generated handler tier; the hand-written typed client
wrappers it once mirrored were removed with the dead client surface.
"""

from collections.abc import Awaitable, Callable
from typing import Any, TypeVar, cast
from unittest.mock import AsyncMock, patch

import pytest
from mcp.types import TextContent

from linodemcp.config import Config
from linodemcp.gentools import (
    handle_linode_domain_clone,
    handle_linode_domain_create,
    handle_linode_domain_import,
    handle_linode_domain_record_create,
    handle_linode_stackscript_create,
    handle_linode_volume_clone,
    handle_linode_volume_create,
)
from linodemcp.linode import (
    APIError,
    RetryableClient,
)
from linodemcp.linode.routes import route_for

T = TypeVar("T")

Handler = Callable[[dict[str, Any], Config], Awaitable[list[TextContent]]]

# A 500 rather than a 429: Python's _should_retry replays both through a POST,
# while Go's isRetryable declines a 5xx on a non-idempotent method.
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


_CREATES: list[
    tuple[str, Handler, dict[str, Any], str, tuple[object, ...], str, dict[str, Any]]
] = [
    (
        "domain_create",
        handle_linode_domain_create,
        {
            "domain": "example.com",
            "type": "master",
            "soa_email": "admin@example.com",
            "confirm": True,
        },
        "linode_domain_create",
        (),
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
        "linode_domain_import",
        (),
        "/domains/import",
        {"domain": "example.com", "remote_nameserver": "ns1.example.net"},
    ),
    (
        "domain_clone",
        handle_linode_domain_clone,
        {"domain_id": 12345, "domain": "clone.example.com", "confirm": True},
        "linode_domain_clone",
        (12345,),
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
        "linode_domain_record_create",
        (12345,),
        "/domains/12345/records",
        {"type": "A", "name": "www", "target": "8.8.8.8"},
    ),
    (
        "volume_create",
        handle_linode_volume_create,
        {"label": "my-volume", "region": "us-east", "confirm": True},
        "linode_volume_create",
        (),
        "/volumes",
        {"label": "my-volume", "region": "us-east"},
    ),
    (
        "volume_clone",
        handle_linode_volume_clone,
        {"volume_id": 12345, "label": "my-volume-clone", "confirm": True},
        "linode_volume_clone",
        (12345,),
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
        "linode_stackscript_create",
        (),
        "/linode/stackscripts",
        {
            "label": "my-script",
            "images": ["linode/ubuntu22.04"],
            "script": "#!/bin/bash",
        },
    ),
]


@pytest.mark.parametrize(
    ("name", "handler", "arguments", "tool", "values", "endpoint", "body"),
    _CREATES,
    ids=[case[0] for case in _CREATES],
)
async def test_create_does_not_replay_transient_failure(
    sample_config: Config,
    name: str,
    handler: Handler,
    arguments: dict[str, Any],
    tool: str,
    values: tuple[object, ...],
    endpoint: str,
    body: dict[str, Any],
) -> None:
    """One attempt reaches the API and the transient failure reaches the caller."""
    del name
    client = _FailingRetryableClient()
    route_raw = AsyncMock(side_effect=_TRANSIENT)
    cast("Any", client.client).route_raw = route_raw

    try:
        with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
            result = await handler(arguments, sample_config)
    finally:
        await client.close()

    assert client.retry_calls == 0
    route_raw.assert_awaited_once_with(tool, *values, body=body, query=None)
    # Pin the wire path through the contract that owns it, so a route edit that
    # moves one of these endpoints fails here rather than passing silently.
    assert route_for(tool).endpoint(*values) == endpoint
    assert "upstream failure" in result[0].text
    assert result[0].text.startswith("Failed to ")
