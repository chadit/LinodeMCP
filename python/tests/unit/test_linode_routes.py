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
    RetryableClient,
    RetryConfig,
)
from linodemcp.linode.routes import (
    Route,
    RouteError,
    all_routes,
    base_for,
    contract_for,
    echo_argument,
    index_routes,
    input_descriptor,
    response_descriptor,
    route_for,
    surface_segment,
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


# The response members the shipped surface fills from a differently spelled
# argument, as {response message: {member: argument}}. Each reports what the
# resource becomes rather than what the tool was given: the resize answer names
# the plan the instance moves to (`type`), the disk resize answer carries the
# unit in its own name (`size`), and the SSL delete keys the bucket label under
# `bucket` to match the shape that tool has always answered with.
_ECHO_ALIASES = {
    "InstanceResizeWriteResponse": {"new_type": "type"},
    "InstanceDiskResizeWriteResponse": {"new_size_mb": "size"},
    "ObjectStorageSSLDeleteResponse": {"bucket": "label"},
}


def test_echo_alias_resolves_across_the_shipped_surface() -> None:
    """Every response member is filled by its own name, or by its declared alias.

    Reading them all is what keeps a declared alias from being read only in the
    emitter: both mutation drivers resolve their echo through this at runtime,
    so a member the alias does not reach would answer with a zero.
    """
    seen = 0
    for route in all_routes():
        if not contract_for(route.tool).response:
            continue
        response = response_descriptor(route.tool)
        aliases = _ECHO_ALIASES.get(response.name, {})
        for member in response.fields:
            assert echo_argument(member) == aliases.get(member.name, member.name)
            seen += 1

    assert seen > 0, "no response member was read, so this proves nothing"


def test_echo_alias_names_the_resize_argument() -> None:
    """The one declared alias resolves, so the table above cannot go stale by
    silently matching nothing."""
    member = response_descriptor("linode_instance_resize").fields_by_name["new_type"]

    assert echo_argument(member) == "type"


@pytest.mark.parametrize(
    ("base", "segment", "want"),
    [
        # The canonical deployment is the only shape that gets re-pointed.
        ("https://api.linode.com/v4", "v4beta", "https://api.linode.com/v4beta"),
        (
            "https://proxy.example/linode/v4",
            "v4beta",
            "https://proxy.example/linode/v4beta",
        ),
        # Every row below is a configured apiUrl that wins as written.
        ("https://api.linode.com/v4beta", "v4beta", "https://api.linode.com/v4beta"),
        ("http://127.0.0.1:8080", "v4beta", "http://127.0.0.1:8080"),
        ("https://api.linode.com/v4/", "v4beta", "https://api.linode.com/v4/"),
        ("https://v4.example.com", "v4beta", "https://v4.example.com"),
        ("https://api.linode.com/v4", "v4", "https://api.linode.com/v4"),
    ],
)
def test_base_for_repoints_only_a_default_suffixed_base(
    base: str, segment: str, want: str
) -> None:
    """The whole override rule, matching Go's BaseFor table row for row.

    A surface picks among versions of one deployment; it never picks the
    deployment, so anything that is not the canonical suffix is left alone.
    """
    assert base_for(base, segment) == want


def test_surface_segment_names_every_surface() -> None:
    """An undeclared surface reads as the zero value, so it must answer v4."""
    assert [surface_segment(value) for value in (0, 1, 2)] == ["v4", "v4", "v4beta"]


def test_surface_segment_refuses_an_unknown_surface() -> None:
    """Fail closed: a surface this build cannot address is never defaulted."""
    with pytest.raises(RouteError, match="unknown API surface"):
        surface_segment(99)


@pytest.mark.asyncio
async def test_make_route_request_sends_a_beta_route_to_the_beta_base() -> None:
    """A route on another surface reaches that base, path unchanged.

    Driven through a patched route because nothing is annotated yet: the client
    half has to be proven before a family moves, or the annotation slot would be
    landing the contract and the transport at once.
    """
    client = Client("https://api.linode.com/v4", "test-token")
    beta = Route(
        tool="linode_fake_list",
        method="GET",
        template="/fake",
        slots=(),
        surface=2,
    )

    with (
        patch("linodemcp.linode.route_for", return_value=beta),
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
    ):
        await client.make_route_request("linode_fake_list")

    mock_request.assert_awaited_once_with(
        "GET", "/fake", base="https://api.linode.com/v4beta"
    )


@pytest.mark.asyncio
async def test_make_route_request_sends_a_beta_write_body_to_the_beta_base() -> None:
    """A beta route carrying a body reaches the same base as one without.

    The body and the base are chosen on separate branches, so a write is the
    case that proves the second one forwards both.
    """
    client = Client("https://api.linode.com/v4", "test-token")
    beta = Route(
        tool="linode_fake_create",
        method="POST",
        template="/fake",
        slots=(),
        surface=2,
    )
    body = {"label": "example"}

    with (
        patch("linodemcp.linode.route_for", return_value=beta),
        patch.object(client, "make_request", new_callable=AsyncMock) as mock_request,
    ):
        await client.make_route_request("linode_fake_create", body=body)

    mock_request.assert_awaited_once_with(
        "POST", "/fake", body, base="https://api.linode.com/v4beta"
    )


@pytest.mark.asyncio
async def test_content_type_route_request_sends_a_beta_route_to_the_beta_base() -> None:
    """The content-type primitive re-points a beta route the same way.

    It reaches httpx directly rather than through make_request, so the base
    choice is its own branch and needs its own proof.
    """
    client = Client("https://api.linode.com/v4", "test-token")
    beta = Route(
        tool="linode_fake_thumbnail_update",
        method="PUT",
        template="/fake/thumbnail",
        slots=(),
        surface=2,
    )
    mock_response = AsyncMock()
    mock_response.status_code = 200

    with (
        patch("linodemcp.linode.route_for", return_value=beta),
        patch.object(client.client, "request", new_callable=AsyncMock) as mock_request,
    ):
        mock_request.return_value = mock_response

        await client.make_route_request_content_type(
            "linode_fake_thumbnail_update",
            content_type="image/png",
            content=b"png",
        )

    mock_request.assert_awaited_once_with(
        "PUT",
        "https://api.linode.com/v4beta/fake/thumbnail",
        headers={
            "Authorization": "Bearer test-token",
            "Content-Type": "image/png",
            "User-Agent": "LinodeMCP/1.0",
        },
        content=b"png",
    )
    await client.close()
