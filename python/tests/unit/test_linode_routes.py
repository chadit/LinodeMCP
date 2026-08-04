"""Tests for the proto-backed route builder and the client method that uses it.

The builder is the first runtime reader of the `linode.mcp.v1.tool_route`
options; before it, only the offline gates read them.
"""

from __future__ import annotations

import importlib
import pkgutil
from dataclasses import replace
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import httpx
import pytest

from linodemcp.linode import (
    Client,
    LinodeError,
    NetworkError,
    RetryableClient,
    RetryConfig,
)
from linodemcp.linode.routes import (
    Route,
    RouteError,
    all_routes,
    contract_for,
    index_routes,
    input_descriptor,
    route_for,
    validate,
)

if TYPE_CHECKING:
    from collections.abc import Iterator

_GENERATED_PACKAGE = "linodemcp.genpb.linode.mcp.v1"

# The Go builder carries the identical table, so a change to either language's
# escaping has to move both and cannot slip through as a one-sided path.
_ENCODING_VECTORS = [
    ("plain", "plain"),
    ("has space", "has%20space"),
    ("a/b", "a%2Fb"),
    ("a:b@c", "a:b%40c"),
    ("2001:db8::/64", "2001:db8::%2F64"),
    ("ubuntu22.04", "ubuntu22.04"),
    (".", "%2E"),
    ("..", "%2E%2E"),
    ("a+b,c", "a%2Bb%2Cc"),
    ("50%", "50%25"),
    ("naïve", "na%C3%AFve"),
    ("-._~", "-._~"),
]

# Types no path slot accepts. bool leads because Python counts it as an int, so
# an unguarded slot would render it as "True".
_REJECTED_VALUES = [True, 1.5, None, b"raw", ("a",)]

# Handlers read ids out of an `arguments: dict[str, Any]`, so nothing checks the
# type between the tool call and the client. Typed Any for that reason, not to
# get an ill-typed literal past the checker.
_UNTYPED_EMPTY_ID: Any = ""


def _retry_config() -> RetryConfig:
    """Retry settings that replay as production does, without the waiting."""
    return RetryConfig(max_retries=2, base_delay=0.0, jitter_enabled=False)


def _declared_by_descriptor() -> Iterator[tuple[str, str, str]]:
    """Read (tool, method, path) straight from the descriptors.

    Deliberately a second walk rather than a call into the builder: it is the
    oracle the builder is checked against, so the count it finds is the count
    the builder has to resolve.
    """
    options = importlib.import_module(f"{_GENERATED_PACKAGE}.options_pb2")
    package = importlib.import_module(_GENERATED_PACKAGE)

    for info in pkgutil.iter_modules(package.__path__):
        if not info.name.endswith("_pb2"):
            continue
        module = importlib.import_module(f"{_GENERATED_PACKAGE}.{info.name}")
        for descriptor in module.DESCRIPTOR.message_types_by_name.values():
            declared = descriptor.GetOptions()
            if not declared.HasExtension(options.tool_route):
                continue
            route = declared.Extensions[options.tool_route]
            yield str(route.tool), str(route.method), str(route.path)


def test_builder_resolves_every_declared_route() -> None:
    """Every tool_route the descriptors carry resolves, with the same values."""
    declared = sorted(_declared_by_descriptor())

    assert declared, "no tool_route options in the descriptors; run `make proto`"

    resolved = [(route.tool, route.method, route.template) for route in all_routes()]

    assert resolved == declared
    assert route_for("linode_instance_delete").template == (
        "/linode/instances/{instance_id}"
    )


def test_every_route_fills_with_no_placeholder_left() -> None:
    """Each declared route renders to a path with nothing left to substitute."""
    for route in all_routes():
        filled = route.endpoint(*range(len(route.slots)))

        assert "{" not in filled
        assert "}" not in filled
        assert filled.startswith("/")


def test_endpoint_fills_slots_in_declared_order() -> None:
    """Values land in the slots the template names, left to right."""
    route = route_for("linode_instance_config_delete")

    assert route.endpoint(4242, 77) == "/linode/instances/4242/configs/77"


def test_endpoint_encodes_path_values() -> None:
    """A value cannot smuggle in path segments or a further placeholder."""
    route = route_for("linode_tag_delete")

    assert route.endpoint("a/b") == "/tags/a%2Fb"
    assert route.endpoint("{linode_id}") == "/tags/%7Blinode_id%7D"


@pytest.mark.parametrize(("value", "encoded"), _ENCODING_VECTORS)
def test_endpoint_escaping_matches_the_shared_vectors(value: str, encoded: str) -> None:
    """Escaping is unreserved-only percent-encoding with uppercase hex.

    Go resolves the same tool from the same proto route, so a path either
    client builds for a given value has to be the same string.
    """
    route = route_for("linode_tag_delete")

    assert route.endpoint(value) == f"/tags/{encoded}"


def test_endpoint_renders_an_int_as_its_digits() -> None:
    """An int identifier fills its slot with nothing added around it."""
    assert route_for("linode_domain_delete").endpoint(123) == "/domains/123"


def test_endpoint_rejects_an_empty_value() -> None:
    """An empty value would address the collection, not a member of it."""
    route = route_for("linode_tag_delete")

    with pytest.raises(RouteError, match="linode_tag_delete slot tag_label"):
        route.endpoint("")


@pytest.mark.parametrize("value", _REJECTED_VALUES)
def test_endpoint_rejects_values_that_are_not_str_or_int(value: object) -> None:
    """Only the identifier types the Go builder takes can fill a slot."""
    route = route_for("linode_tag_delete")

    with pytest.raises(RouteError, match="takes a str or an int"):
        route.endpoint(value)


def test_endpoint_names_the_offending_type() -> None:
    """The message says what was passed, which a positional call site hides."""
    route = route_for("linode_tag_delete")

    with pytest.raises(RouteError, match="got bool"):
        route.endpoint(True)

    with pytest.raises(RouteError, match="got float"):
        route.endpoint(1.5)


@pytest.mark.parametrize("values", [(), (1, 2)])
def test_endpoint_arity_mismatch_raises(values: tuple[int, ...]) -> None:
    """Too few or too many values fails instead of yielding a partial path."""
    route = route_for("linode_domain_delete")

    with pytest.raises(RouteError, match="takes 1 path value"):
        route.endpoint(*values)


def test_route_for_unknown_tool_raises() -> None:
    """A tool the proto never declared has no route to guess at."""
    with pytest.raises(RouteError, match="declares no route"):
        route_for("linode_not_a_tool")


def test_validate_accepts_the_shipped_contract() -> None:
    """The descriptors that ship describe one tool per message at one tier.

    Zero arguments because the contract now names the whole surface itself; the
    tools a server staged are checked against it by validate_registered.
    """
    validate()


def test_index_rejects_two_messages_claiming_one_tool() -> None:
    """One tool with two routes is a broken contract, not a drift to absorb."""
    duplicate = Route("linode_domain_delete", "DELETE", "/domains/{domain_id}", ("id",))

    with pytest.raises(RouteError, match="more than one route"):
        index_routes([duplicate, duplicate])


@pytest.mark.asyncio
async def test_make_route_request_sends_body_when_given() -> None:
    """A routed write forwards its body to the resolved method and path."""
    client = Client("https://api.linode.com/v4", "test-token")
    body = {"domain": "example.com"}

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.make_route_request("linode_domain_create", body=body)

    mock_request.assert_awaited_once_with("POST", "/domains", body)
    await client.close()


@pytest.mark.asyncio
async def test_delete_domain_sends_the_same_url_as_before_the_migration() -> None:
    """The migrated call site still puts DELETE /v4/domains/4242 on the wire."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(204)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.delete_domain(4242)
    finally:
        await client.close()

    assert len(seen) == 1
    assert seen[0].method == "DELETE"
    assert seen[0].url.path == "/v4/domains/4242"
    assert seen[0].url.query == b""


@pytest.mark.asyncio
async def test_a_route_defect_reaches_the_caller_as_an_argument_error() -> None:
    """A bad path value is the caller's defect, so it is not a LinodeError.

    The migrated call site wraps its request in the same httpx handlers the
    hand-built one had, and a route defect is none of those, so it arrives
    unwrapped rather than dressed up as a failed Linode call.
    """
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(204)

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(RouteError) as raised:
            await client.make_route_request("linode_tag_delete", "")
    finally:
        await client.close()

    assert isinstance(raised.value, ValueError)
    assert not isinstance(raised.value, LinodeError)
    assert seen == []


@pytest.mark.asyncio
async def test_delete_domain_route_defect_is_not_a_network_error() -> None:
    """The one migrated call site keeps route defects out of NetworkError."""
    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(
        transport=httpx.MockTransport(lambda _request: httpx.Response(204))
    )

    try:
        with pytest.raises(RouteError) as raised:
            await client.delete_domain(_UNTYPED_EMPTY_ID)
    finally:
        await client.close()

    assert not isinstance(raised.value, NetworkError)


@pytest.mark.asyncio
async def test_the_retry_layer_never_replays_a_route_defect() -> None:
    """Replaying a wrong argument would just build the wrong path again."""
    retryable = RetryableClient(
        "https://api.linode.com/v4", "test-token", _retry_config()
    )
    retryable.client.client = httpx.AsyncClient(
        transport=httpx.MockTransport(lambda _request: httpx.Response(204))
    )
    attempt = AsyncMock(side_effect=retryable.client.delete_domain)

    try:
        with (
            patch.object(retryable.client, "delete_domain", attempt),
            pytest.raises(RouteError),
        ):
            await retryable.delete_domain(_UNTYPED_EMPTY_ID)
    finally:
        await retryable.close()

    assert attempt.await_count == 1


@pytest.mark.asyncio
async def test_the_retry_layer_still_replays_a_transport_failure() -> None:
    """A dropped connection through the routed path retries as it always did."""

    def handler(request: httpx.Request) -> httpx.Response:
        msg = "connection refused"
        raise httpx.ConnectError(msg, request=request)

    config = _retry_config()
    retryable = RetryableClient("https://api.linode.com/v4", "test-token", config)
    retryable.client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    attempt = AsyncMock(side_effect=retryable.client.delete_domain)

    try:
        with (
            patch.object(retryable.client, "delete_domain", attempt),
            pytest.raises(NetworkError),
        ):
            await retryable.delete_domain(4242)
    finally:
        await retryable.close()

    assert attempt.await_count == config.max_retries + 1


@pytest.mark.asyncio
async def test_make_route_request_appends_the_query_it_was_handed() -> None:
    """The contract owns the path; the already-encoded query rides behind it."""
    client = Client("https://api.linode.com/v4", "test-token")

    with patch.object(client, "make_request", new_callable=AsyncMock) as mock_request:
        await client.make_route_request(
            "linode_kernel_list", query="page=2&page_size=100"
        )

    mock_request.assert_awaited_once_with("GET", "/linode/kernels?page=2&page_size=100")
    await client.close()


@pytest.mark.asyncio
async def test_route_raw_resolves_the_method_and_decodes_the_body() -> None:
    """The routed twin of get_raw names a tool and neither a verb nor a path."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 7, "domain": "example.com"})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        data = await client.route_raw("linode_domain_get", 7)
    finally:
        await client.close()

    assert data == {"id": 7, "domain": "example.com"}
    assert len(seen) == 1
    assert seen[0].method == "GET"
    assert seen[0].url.path == "/v4/domains/7"


@pytest.mark.asyncio
async def test_route_raw_sends_an_empty_json_body_on_post() -> None:
    """post_raw always sent a body, so its twin puts the same bytes on the wire."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"id": 1})

    client = Client("https://api.linode.com/v4", "test-token")
    client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        await client.route_raw("linode_domain_create")
    finally:
        await client.close()

    assert len(seen) == 1
    assert seen[0].method == "POST"
    assert seen[0].content == b"{}"


@pytest.mark.asyncio
async def test_retryable_route_raw_forwards_keywords_through_the_executor() -> None:
    """The executor forwards positionals only, so the twin carries its keywords."""
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json={"data": []})

    retryable = RetryableClient(
        "https://api.linode.com/v4", "test-token", _retry_config()
    )
    retryable.client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        data = await retryable.route_raw("linode_kernel_list", query="page=2")
    finally:
        await retryable.close()

    assert data == {"data": []}
    assert len(seen) == 1
    assert seen[0].url.query == b"page=2"


@pytest.mark.asyncio
async def test_retryable_route_raw_makes_one_attempt_when_retry_is_off() -> None:
    """retry=False is the one-protected-attempt trade post_raw documents."""
    attempts: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        attempts.append(request)
        msg = "connection refused"
        raise httpx.ConnectError(msg, request=request)

    retryable = RetryableClient(
        "https://api.linode.com/v4", "test-token", _retry_config()
    )
    retryable.client.client = httpx.AsyncClient(transport=httpx.MockTransport(handler))

    try:
        with pytest.raises(httpx.ConnectError):
            await retryable.route_raw("linode_domain_get", 7, retry=False)
    finally:
        await retryable.close()

    assert len(attempts) == 1


def test_input_descriptor_answers_the_message_the_options_came_from() -> None:
    """A tool's input descriptor is reachable from its name alone.

    The generator reads a tool's fields through this rather than repeating the
    descriptor walk, so the fields it locates are the ones the contract reader
    read the tool's options off.
    """
    descriptor = input_descriptor("linode_domain_get")

    assert descriptor.full_name == "linode.mcp.v1.DomainGetInput"
    assert "domain_id" in descriptor.fields_by_name


def test_input_descriptor_rejects_an_unknown_tool() -> None:
    """A name no message declares has no input to answer with."""
    with pytest.raises(RouteError, match="declares no contract"):
        input_descriptor("linode_not_a_tool")


def test_input_descriptor_rejects_a_message_that_is_not_generated() -> None:
    """A contract naming an input the tree does not carry fails by name.

    The gates reject this before it can be generated, so proving the reader
    still refuses it means handing it a contract directly, the same way the
    driver tests do.
    """
    broken = replace(
        contract_for("linode_domain_get"), input_message="linode.mcp.v1.Absent"
    )
    with (
        patch("linodemcp.linode.routes.contract_for", return_value=broken),
        pytest.raises(RouteError, match="which is not generated"),
    ):
        input_descriptor("linode_domain_get")
