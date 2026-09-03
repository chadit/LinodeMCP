"""Driver behavior the migrated tools do not reach on their own.

The proof that the drivers preserve behavior is that every existing tool test
and behavior fixture passes against handlers that now route through them. This
file covers the rest: the branches no migrated tool exercises (a route that
takes no pagination) and the contract defects each driver refuses rather than
answering with a plausible-looking wrong envelope. Those refusals are the
reason the drivers can be trusted to read a descriptor instead of a literal, so
they need a test that proves they bite.
"""

from __future__ import annotations

import dataclasses
import json
from dataclasses import replace
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.linode import APIError
from linodemcp.linode.routes import (
    Contract,
    RouteError,
    contract_for,
    message_class,
    response_descriptor,
)
from linodemcp.tools.drivers import (
    DriverError,
    ListFilter,
    MatchMode,
    PreviewStandIn,
    read_collection_state,
    read_route_state,
    resolve_payload_field,
    run_acknowledge_tool,
    run_assembled_read_tool,
    run_body_read_tool,
    run_destructive_tool,
    run_get_tool,
    run_list_tool,
    run_write_tool,
)
from linodemcp.tools.preview import preview_sentence
from linodemcp.tools.transport import (
    MultipartUpload,
    PresignRemove,
    PresignTransfer,
    RawBody,
)

if TYPE_CHECKING:
    from pathlib import Path

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient


def _client(**attrs: object) -> AsyncMock:
    """An AsyncMock RetryableClient usable as an async context manager."""
    client = AsyncMock()
    for name, value in attrs.items():
        getattr(client, name).return_value = value
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


def test_contract_reader_answers_the_five_declarations() -> None:
    """Each option reads back as the proto spells it, not as a default."""
    create = contract_for("linode_domain_create")
    assert create.response == "linode.mcp.v1.DomainWriteResponse"
    assert create.confirm_message.endswith("Set confirm=true to proceed.")
    assert create.success_message == "Domain '{domain}' (ID: {id}) created successfully"
    assert create.retry_disabled is True
    assert contract_for("linode_domain_delete").resource_type == "Domain"
    assert contract_for("linode_domain_update").retry_disabled is False


def test_contract_reader_rejects_an_unknown_tool() -> None:
    """A name no message declares is a defect, not an empty contract."""
    with pytest.raises(RouteError, match="declares no contract"):
        contract_for("linode_not_a_tool")


def test_message_class_rejects_an_ungenerated_name() -> None:
    """A response naming a message the tree does not carry fails by name."""
    with pytest.raises(RouteError, match="no generated message named"):
        message_class("linode.mcp.v1.NotAMessage")


def test_response_descriptor_rejects_a_tool_with_no_response() -> None:
    """The tools whose Linode body has no proto model cannot drive a tier."""
    with pytest.raises(RouteError, match="declares no response message"):
        response_descriptor("linode_profile_preferences_get")


def test_response_descriptor_resolves_the_declared_message() -> None:
    """A declared response resolves to the descriptor that names it."""
    assert response_descriptor("linode_domain_list").full_name == (
        "linode.mcp.v1.DomainListResponse"
    )


async def test_list_driver_omits_the_query_for_an_unpaginated_route(
    sample_config: Config,
) -> None:
    """A route that publishes no page range is called with no query string.

    Every list tool migrated so far paginates, so this is the branch a
    collection returned whole would take.
    """
    client = _client(route_raw={"data": [{"id": 1, "domain": "example.com"}]})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await run_list_tool(
            sample_config,
            {"page": 3},
            tool="linode_domain_list",
            error_action="retrieve domains",
            pagination=None,
        )

    client.route_raw.assert_awaited_once_with("linode_domain_list", query="")


async def test_list_driver_reports_a_page_size_outside_its_bounds(
    sample_config: Config,
) -> None:
    """A range failure answers before a client is opened."""
    with patch("linodemcp.tools.helpers.RetryableClient") as opened:
        result = await run_list_tool(
            sample_config,
            {"page_size": 1},
            tool="linode_domain_list",
            error_action="retrieve domains",
        )

    assert "page_size must be an integer from 25 through 500" in result[0].text
    opened.assert_not_called()


async def test_list_driver_rejects_a_path_value_the_route_does_not_name(
    sample_config: Config,
) -> None:
    """A misspelled path value fails here rather than addressing nothing."""
    with pytest.raises(DriverError, match="has no path slot named zone_id"):
        await run_list_tool(
            sample_config,
            {},
            tool="linode_domain_record_list",
            error_action="retrieve domain records",
            path_values={"domain_id": 5, "zone_id": 9},
        )


async def test_list_driver_rejects_a_missing_path_value(
    sample_config: Config,
) -> None:
    """A sub-resource route called with no id fails before the URL is built."""
    with pytest.raises(DriverError, match="needs path values for domain_id"):
        await run_list_tool(
            sample_config,
            {},
            tool="linode_domain_record_list",
            error_action="retrieve domain records",
        )


async def test_list_driver_echoes_only_the_filters_asked_for(
    sample_config: Config,
) -> None:
    """An absent filter neither narrows the page nor appears in the echo."""
    page = {
        "data": [
            {"id": 1, "domain": "example.com", "type": "master"},
            {"id": 2, "domain": "other.net", "type": "slave"},
        ]
    }
    client = _client(route_raw=page)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {"type": "SLAVE"},
            tool="linode_domain_list",
            error_action="retrieve domains",
            filters=(
                ListFilter("domain_contains", "domain"),
                ListFilter("type", "type", MatchMode.EQUALS),
            ),
        )

    body = result[0].text
    assert '"filter": "type=SLAVE"' in body
    assert "other.net" in body
    assert "example.com" not in body


async def test_list_driver_refuses_an_ambiguous_envelope(
    sample_config: Config,
) -> None:
    """A response with several repeated fields names no single item member.

    A get tool answers with the bare resource, and Domain carries three
    repeated fields, so driving one through the list driver is the shape the
    derivation has to refuse: picking one would answer with a well-formed
    envelope holding the wrong list.
    """
    with pytest.raises(DriverError, match="list envelope key is ambiguous"):
        await run_list_tool(
            sample_config,
            {},
            tool="linode_domain_get",
            error_action="retrieve domains",
            path_values={"domain_id": 5},
        )


def test_write_driver_places_a_body_by_name_not_by_shape() -> None:
    """A prose-less response wraps its object only when it is named an envelope.

    ProfileTokenCreateResponse is {warning, token}, so the decoded token goes in
    the member. PlacementGroup holds a nested migrations object and is the
    resource itself, and reading shape alone would answer a placement-group
    update with an envelope holding its migrations. ProfileTfaEnableResponse is
    named like an envelope but carries no object, so it stays the resource.
    """
    token = resolve_payload_field("linode_profile_token_create")

    assert token is not None
    assert token.name == "token"
    assert resolve_payload_field("linode_placement_group_update") is None
    assert resolve_payload_field("linode_profile_tfa_enable") is None


async def test_write_driver_refuses_a_response_that_is_not_an_envelope(
    sample_config: Config,
) -> None:
    """A response with no object field cannot carry a decoded body.

    The delete responses are the shape that proves it: they carry an id echo
    and no resource, so nothing in them can hold what a write returns.
    """
    with pytest.raises(DriverError, match="is not a write envelope"):
        await run_write_tool(
            sample_config,
            {"confirm": True, "domain_id": 5},
            tool="linode_domain_delete",
            error_action="delete domain",
            body={},
            path_values={"domain_id": 5},
        )


async def test_write_driver_answers_with_a_bare_resource(
    sample_config: Config,
) -> None:
    """A response with no message field is the API resource itself.

    linode_placement_group_update answers with PlacementGroup, so there is no
    member to place the body in and no prose to report over it. Wrapping it
    would hand the caller a shape the tool has never returned.
    """
    client = _client(route_raw={"id": 7, "label": "web", "region": "us-east"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "group_id": 7},
            tool="linode_placement_group_update",
            error_action="",
            body={"label": "web"},
            path_values={"group_id": 7},
        )

    answered = json.loads(result[0].text)

    assert answered["id"] == 7
    assert answered["label"] == "web"
    assert "message" not in answered


async def test_write_driver_reports_the_values_the_answer_carried(
    sample_config: Config,
) -> None:
    """One decode places the resource and the value beside it.

    POST /images/upload answers {image, upload_to}, so the URL has no argument
    to be echoed from. Placing the whole body under the image member would lose
    it and nest the resource a level too deep.
    """
    body = {
        "image": {"id": "private/1", "label": "iso", "status": "pending_upload"},
        "upload_to": "https://upload.example/one-time",
    }
    client = _client(route_raw=body)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "label": "iso", "region": "us-east"},
            tool="linode_image_upload",
            error_action="",
            body={"label": "iso", "region": "us-east"},
        )

    answered = json.loads(result[0].text)

    assert answered["upload_to"] == "https://upload.example/one-time"
    assert answered["image"]["id"] == "private/1"
    assert answered["message"] == "Image upload 'iso' (private/1) created successfully"


async def test_write_driver_answers_with_scalars_the_api_sent(
    sample_config: Config,
) -> None:
    """The same shape with no resource member: the scalars are the answer.

    Read as an echo, the confirm would report an empty scratch code over an
    account whose recovery code has already been issued.
    """
    client = _client(
        route_raw={"scratch": "abcd-1234", "expiry": "2026-01-01T00:00:00"}
    )
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "tfa_code": "123456"},
            tool="linode_profile_tfa_enable_confirm",
            error_action="",
            body={"tfa_code": "123456"},
        )

    answered = json.loads(result[0].text)

    assert answered["scratch"] == "abcd-1234"
    assert answered["expiry"] == "2026-01-01T00:00:00"
    assert (
        answered["message"] == "Profile two-factor authentication enabled successfully"
    )


async def test_write_driver_reports_the_preview_verdict_before_confirm(
    sample_config: Config,
) -> None:
    """A preview reports a bad argument straight away; a live call gates first."""
    preview = await run_write_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_domain_create",
        error_action="create domain",
        body={},
        error="domain is required",
        preview_error="domain is required",
    )
    assert "domain is required" in preview[0].text

    live = await run_write_tool(
        sample_config,
        {},
        tool="linode_domain_create",
        error_action="create domain",
        body={},
        error="domain is required",
        preview_error="domain is required",
    )
    assert "This creates a DNS domain. Set confirm=true to proceed." in live[0].text


async def test_write_driver_redacts_a_declared_preview_member(
    sample_config: Config,
) -> None:
    """A preview never echoes a member the contract withholds.

    The twin lives in go/internal/tools/gentools_body_test.go: both languages
    report the same stand-in so one fixture covers the pair.
    """
    body = {"domain": "example.com", "soa_email": "root@example.com"}

    preview = await run_write_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_domain_create",
        error_action="create domain",
        body=body,
        redact_preview=("soa_email",),
    )

    assert '"soa_email": {' in preview[0].text
    assert '"redacted": true' in preview[0].text
    assert "root@example.com" not in preview[0].text
    assert '"domain": "example.com"' in preview[0].text
    assert body["soa_email"] == "root@example.com"


async def test_write_driver_fills_a_placeholder_from_the_arguments(
    sample_config: Config,
) -> None:
    """A name the response does not carry comes from the call's arguments."""
    client = _client(route_raw={"id": 5, "domain": "example.com"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "domain_id": 5},
            tool="linode_domain_update",
            error_action="update domain",
            body={"description": "x"},
            path_values={"domain_id": 5},
        )

    assert '"message": "Domain 5 modified successfully"' in result[0].text


async def test_destroy_driver_refuses_an_undeclared_message_with_no_override(
    sample_config: Config,
) -> None:
    """A tool the contract leaves undeclared must supply its own prose.

    It is refused before the DELETE goes out, so the missing message cannot
    surface over a resource that has already been removed.

    The declaration is taken away from a real contract rather than found on a
    live tool: every destroy the contract declares now carries a
    success_message, because the emitter refuses one that does not.
    """
    declared = contract_for("linode_domain_record_delete")
    undeclared = dataclasses.replace(declared, success_message="")

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        patch("linodemcp.tools.drivers.contract_for", return_value=undeclared),
        pytest.raises(DriverError, match="declares no success_message"),
    ):
        await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_record_delete",
            error_action="delete DNS record",
            id_args={"domain_id": 5, "record_id": 7},
            fetch_state=fetch,
        )

    client.route_call.assert_not_awaited()


async def test_destroy_driver_uses_the_supplied_prose_when_none_is_declared(
    sample_config: Config,
) -> None:
    """A call-site message answers for a tool the contract leaves undeclared.

    The declaration wins wherever there is one, so proving the override still
    reaches the answer means taking the declaration away: every destroy the
    contract declares now carries a success_message.
    """
    declared = contract_for("linode_domain_record_delete")
    undeclared = dataclasses.replace(declared, success_message="")

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        patch("linodemcp.tools.drivers.contract_for", return_value=undeclared),
    ):
        result = await run_destructive_tool(
            sample_config,
            {"confirm": True, "confirm_bypass_dry_run": True},
            tool="linode_domain_record_delete",
            error_action="delete DNS record",
            id_args={"domain_id": 5, "record_id": 7},
            fetch_state=fetch,
            success_message="record 7 is gone",
        )

    assert '"message": "record 7 is gone"' in result[0].text


async def test_destroy_driver_runs_the_transport_in_place_of_the_routed_removal(
    sample_config: Config,
) -> None:
    """A declared transport is the live call, so the routed delete never fires.

    The object delete signs a URL over its own POST and sends the DELETE to that
    URL, which no route builds. Go's twin is the destroyCall branch in
    go/cmd/toolgen/destroy.go rendering tools.RunPresignRemove.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    body = {"name": "releases/app.tar.gz", "method": "DELETE"}

    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        patch(
            "linodemcp.tools.drivers.run_transport", new=AsyncMock(return_value={})
        ) as transport,
    ):
        result = await run_destructive_tool(
            sample_config,
            {
                "region": "us-east",
                "label": "artifacts",
                "name": "releases/app.tar.gz",
                "confirm": True,
                "confirm_bypass_dry_run": True,
            },
            tool="linode_object_storage_object_delete",
            error_action="",
            id_args={"region": "us-east", "label": "artifacts"},
            fetch_state=fetch,
            body=body,
            transport=PresignRemove(url_field="url"),
        )

    client.route_delete.assert_not_awaited()
    client.route_call.assert_not_awaited()
    assert transport.await_count == 1
    awaited = transport.await_args
    assert awaited is not None
    assert awaited.args[0] == PresignRemove(url_field="url")
    assert awaited.args[2] == "linode_object_storage_object_delete"
    assert (
        "\"message\": \"Object 'releases/app.tar.gz' deleted from bucket 'artifacts'\""
        in result[0].text
    )


async def test_destroy_driver_refuses_an_echo_it_cannot_fill(
    sample_config: Config,
) -> None:
    """A response echoing a value the call was never given raises.

    The echo confirms which resource was removed, so filling it with a guess
    would report a deletion of something the caller did not ask about. No
    shipped tool has this shape, since the gates refuse one before it can be
    generated, so the member is made to name an absent argument here the same
    way the neighbouring tests hand the driver a contract directly.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        patch("linodemcp.tools.drivers.echo_argument", return_value="bucket"),
        pytest.raises(DriverError, match="names no argument it was given"),
    ):
        await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_object_storage_ssl_delete",
            error_action="delete the SSL certificate",
            id_args={"region": "us-east", "label": "assets"},
            fetch_state=fetch,
            success_message="gone",
        )

    client.route_call.assert_not_awaited()


async def test_destroy_driver_fills_the_echo_from_the_ids(
    sample_config: Config,
) -> None:
    """Every echo field named by an id is filled from the value given."""

    async def fetch(_client: RetryableClient) -> Any:
        return {"id": 7}

    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_record_delete",
            error_action="delete DNS record",
            id_args={"domain_id": 5, "record_id": 7},
            fetch_state=fetch,
            success_message="Record 7 removed successfully from domain 5",
        )

    body = result[0].text
    client.route_call.assert_awaited_once_with(
        "linode_domain_record_delete", 5, 7, retry=False
    )
    assert '"domain_id": 5' in body
    assert '"record_id": 7' in body
    assert '"message": "Record 7 removed successfully from domain 5"' in body


async def test_list_driver_lifts_the_member_the_envelope_declares(
    sample_config: Config,
) -> None:
    """The firewall history's version sits inside its rules, which declare none.

    Without the hoist the snapshot answers version 0, which reads as a real one.
    """
    client = _client(
        route_raw={
            "id": 5,
            "label": "web",
            "rules": {"version": 2, "inbound_policy": "DROP"},
        }
    )
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool="linode_firewall_rule_version_list",
            error_action="",
            path_values={"firewall_id": 5},
            pagination=None,
        )

    snapshot = json.loads(result[0].text)["firewall_rule_versions"][0]
    assert snapshot["version"] == 2
    assert snapshot["rules"]["inbound_policy"] == "DROP"


@pytest.mark.parametrize(
    "body",
    [
        {"id": 5, "version": 9},
        {"id": 5, "version": 9, "rules": {"inbound_policy": "DROP"}},
        {"id": 5, "version": 9, "rules": "none"},
    ],
)
async def test_list_driver_leaves_a_member_the_body_nests_nothing_under(
    sample_config: Config, body: dict[str, Any]
) -> None:
    """A source the answer does not carry leaves the member as it was decoded."""
    client = _client(route_raw=body)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool="linode_firewall_rule_version_list",
            error_action="",
            path_values={"firewall_id": 5},
            pagination=None,
        )

    assert json.loads(result[0].text)["firewall_rule_versions"][0]["version"] == 9


async def test_state_read_projects_the_body_through_the_sibling_response() -> None:
    """The state a removal previews is the body the API sent, keyed to the
    sibling GET's own message.

    A member that message does not model is dropped, and one the API never sent
    is not invented, which is what makes the preview report the resource rather
    than a shape assembled around it.
    """
    client = _client(
        route_raw={
            "id": 42,
            "label": "audit log sink",
            "unknown_member": "dropped",
        }
    )

    state = await read_route_state(
        client, 42, tool="linode_monitor_stream_destination_get"
    )

    client.route_raw.assert_awaited_once_with(
        "linode_monitor_stream_destination_get", 42
    )
    assert state.fields == {"id": 42, "label": "audit log sink"}


async def test_state_read_projects_through_the_payload_member() -> None:
    """A read that answers a wrapper around the resource projects through the
    member: the API's body is the resource, and the wrapper's own members are
    not its, so projecting through the envelope would report nothing."""
    client = _client(route_raw={"id": 5, "label": "data-vol", "linode_id": None})

    state = await read_route_state(
        client, 5, tool="linode_volume_get", payload="volume"
    )

    assert state.fields == {"id": 5, "label": "data-vol", "linode_id": None}


async def test_state_read_refuses_a_payload_member_the_read_does_not_answer() -> None:
    """The emitter refuses this at contract build; the runtime says so too, so a
    read renamed under a stale call site reports rather than previews nothing."""
    client = _client(route_raw={"id": 5})

    with pytest.raises(RouteError, match="answers no message member: absent"):
        await read_route_state(client, 5, tool="linode_volume_get", payload="absent")


async def test_state_read_refuses_a_body_that_is_not_an_object() -> None:
    """A bare array would parse as an empty message and preview nothing."""
    client = _client(route_raw=[{"id": 42}])

    with pytest.raises(TypeError, match="response must be an object"):
        await read_route_state(client, 42, tool="linode_monitor_stream_destination_get")


_CERTIFICATE_LIST = "linode_iam_idp_config_certificate_list"


def _certificate_page() -> dict[str, Any]:
    """A two-element page of the collection a certificate removal selects out of."""
    return {
        "data": [
            {"id": "cert-1", "certificate": "first"},
            {"id": "cert-2", "certificate": "second"},
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }


async def test_collection_state_read_selects_the_addressed_element() -> None:
    """No GET reads one certificate, so the parent collection is read instead.

    The trailing path id picks the element out of the page, and the page is
    asked for at the standard maximum so one call carries as much as the route
    hands over.
    """
    client = _client(route_raw=_certificate_page())

    state = await read_collection_state(
        client, "cfg-1", tool=_CERTIFICATE_LIST, member="id", value="cert-2"
    )

    client.route_raw.assert_awaited_once_with(
        _CERTIFICATE_LIST, "cfg-1", query="page=1&page_size=500"
    )
    assert state.text("id") == "cert-2"
    assert state.text("certificate") == "second"


async def test_collection_state_read_reports_a_missing_element() -> None:
    """A page carrying no such id says so rather than previewing another one."""
    client = _client(route_raw=_certificate_page())

    with pytest.raises(LookupError, match="carries no id 'cert-9'"):
        await read_collection_state(
            client, "cfg-1", tool=_CERTIFICATE_LIST, member="id", value="cert-9"
        )


async def test_list_tool_restores_each_elements_nulls(sample_config: Config) -> None:
    """A page's nulls belong to the addresses in it, not to the page.

    The assigned element gains nothing: the API sent all six, so there is
    nothing dropped for the restoration to write back. Go's
    MarshalProtoListResponseRestoringNulls answers the same bytes.
    """
    page = {
        "data": [
            {"address": "192.0.2.10", "gateway": "192.0.2.1", "region": "us-east"},
            {
                "address": "192.0.2.11",
                "assigned_entity": None,
                "gateway": None,
                "interface_id": None,
                "linode_id": None,
                "rdns": None,
                "region": "us-east",
                "vpc_nat_1_1": None,
            },
        ],
        "page": 1,
        "pages": 1,
        "results": 2,
    }
    client = _client(route_raw=page)

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool="linode_networking_reserved_ip_list",
        )

    answered = json.loads(result[0].text)
    unassigned = answered["reserved_ips"][1]
    for name in (
        "assigned_entity",
        "gateway",
        "interface_id",
        "linode_id",
        "rdns",
        "vpc_nat_1_1",
    ):
        assert name in unassigned, name
        assert unassigned[name] is None, name

    assert "assigned_entity" not in answered["reserved_ips"][0]


def _broken(tool: str, **overrides: Any) -> Contract:
    """One tool's contract with a declaration replaced.

    The gates reject every shape below before it can be generated, so proving
    the drivers still refuse them means handing one over directly. routes.py's
    validate_declarations is public for the same reason.
    """
    return replace(contract_for(tool), **overrides)


def test_response_descriptor_rejects_a_response_that_is_not_generated() -> None:
    """A response naming a message the tree does not carry fails by name."""
    broken = _broken("linode_domain_get", response="linode.mcp.v1.Absent")
    with (
        patch("linodemcp.linode.routes.contract_for", return_value=broken),
        pytest.raises(RouteError, match="which is not generated"),
    ):
        response_descriptor("linode_domain_get")


async def test_write_driver_refuses_a_placeholder_naming_a_repeated_field(
    sample_config: Config,
) -> None:
    """A list has no rendering the two languages agree on, so it is refused."""
    broken = _broken("linode_domain_create", success_message="tags: {tags}")
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=broken),
        pytest.raises(DriverError, match="only single string and integer"),
    ):
        await run_write_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_create",
            error_action="create domain",
            body={},
        )


async def test_write_driver_counts_the_entries_a_repeated_argument_carries(
    sample_config: Config,
) -> None:
    """The count form is how a list reaches prose at all.

    This is the sentence linode_placement_group_assign has always answered
    with, rendered from the template rather than from a hand-written len().
    """
    template = "Assigned {linodes:len} Linode(s) to placement group {group_id}"
    counted = _broken("linode_domain_create", success_message=template)
    client = _client(route_raw={"id": 5, "domain": "example.com"})
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=counted),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "group_id": 789, "linodes": [1, 2]},
            tool="linode_domain_create",
            error_action="create domain",
            body={},
        )

    assert '"message": "Assigned 2 Linode(s) to placement group 789"' in result[0].text


async def test_write_driver_counts_a_repeated_argument_in_failure_prose(
    sample_config: Config,
) -> None:
    """The count form renders in the failure sentence the same way as success.

    A failed membership change still reports how many entries the caller sent,
    since the list itself never fits prose.
    """
    template = "Failed to assign {linodes:len} Linode(s) to group {group_id}: {error}"
    counted = _broken("linode_domain_create", error_message=template)
    client = _client(route_raw=APIError(500, "upstream failure"))
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=counted),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "group_id": 789, "linodes": [1, 2]},
            tool="linode_domain_create",
            error_action="",
            body={},
        )

    assert result[0].text.startswith("Failed to assign 2 Linode(s) to group 789:")


async def test_write_driver_leaves_a_present_member_alone_when_restoring(
    sample_config: Config,
) -> None:
    """A declared null never overwrites a member the serialization carries.

    Restoration exists for keys protojson drops; a key that made it into the
    serialized member already says what the API said.
    """
    nulled = _broken("linode_domain_create", explicit_null_fields=("description",))
    client = _client(route_raw={"id": 5, "domain": "example.com", "description": None})
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=nulled),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_write_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_create",
            error_action="create domain",
            body={},
        )

    assert '"description": ""' in result[0].text


async def test_write_driver_skips_restoration_when_the_member_is_absent(
    sample_config: Config,
) -> None:
    """An envelope answering without its member has nothing to restore into.

    protojson drops an unset message member entirely, so the driver answers
    the envelope as built rather than inventing the member to hold a null.
    """
    nulled = _broken(
        "linode_domain_create",
        success_message="created",
        explicit_null_fields=("description",),
    )
    client = _client(route_raw={})
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=nulled),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_write_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_create",
            error_action="create domain",
            body={},
        )

    assert '"message": "created"' in result[0].text


async def test_write_driver_counts_only_a_list(sample_config: Config) -> None:
    """A caller who sent anything but a JSON array has sent nothing to count.

    Counting the characters of a string would report a number that means
    nothing, and Go's tools.ArgumentLen answers 0 for the same calls.
    """
    counted = _broken("linode_domain_create", success_message="{linodes:len} sent")
    cases: list[tuple[Any, str]] = [
        ([1, 2, 3], "3 sent"),
        ([], "0 sent"),
        (None, "0 sent"),
        ("1,2", "0 sent"),
    ]

    for sent, want in cases:
        client = _client(route_raw={"id": 5, "domain": "example.com"})
        with (
            patch("linodemcp.tools.drivers.contract_for", return_value=counted),
            patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        ):
            result = await run_write_tool(
                sample_config,
                {"confirm": True, "linodes": sent},
                tool="linode_domain_create",
                error_action="create domain",
                body={},
            )

        assert f'"message": "{want}"' in result[0].text


async def test_write_driver_refuses_a_placeholder_naming_nothing(
    sample_config: Config,
) -> None:
    """A placeholder matching neither a response field nor an argument raises."""
    broken = _broken("linode_domain_create", success_message="made {nowhere}")
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=broken),
        pytest.raises(DriverError, match="from no response field and no argument"),
    ):
        await run_write_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_create",
            error_action="create domain",
            body={},
        )


async def test_destroy_driver_refuses_a_response_with_nothing_to_report_under(
    sample_config: Config,
) -> None:
    """A response with no message field cannot carry the completion prose."""

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    broken = _broken("linode_domain_delete", response="linode.mcp.v1.Domain")
    client = _client()
    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        patch("linodemcp.linode.routes.contract_for", return_value=broken),
        pytest.raises(DriverError, match="has no message field to report under"),
    ):
        await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_delete",
            error_action="delete domain",
            id_args={"domain_id": 5},
            fetch_state=fetch,
            success_message="gone",
        )

    client.route_call.assert_not_awaited()


async def test_get_driver_answers_the_declared_response(
    sample_config: Config,
) -> None:
    """A single-resource fetch addresses its route and shapes the response.

    The response message comes from the contract, so nothing at the call site
    names it, which is the difference between this driver and the handler it
    replaced.
    """
    client = _client(route_raw={"id": 5, "domain": "example.com"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
        )

    client.route_raw.assert_awaited_once_with("linode_domain_get", 5)
    assert "example.com" in result[0].text


async def test_get_driver_reports_the_declared_failure_prose(
    sample_config: Config,
) -> None:
    """A failed call answers the sentence error_message declares, ids filled.

    The ids come from the validated path values rather than the raw arguments,
    so the message reports the resource the request actually addressed.
    """
    client = _client()
    client.route_raw.side_effect = APIError(500, "boom")
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {"domain_id": "5"},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
        )

    assert result[0].text == (
        "Failed to retrieve domain 5: Linode API error (status 500): boom"
    )


async def test_get_driver_refuses_a_tool_with_no_failure_prose(
    sample_config: Config,
) -> None:
    """A tier whose whole failure report is its declared sentence cannot run
    without one, so it refuses rather than answering a bare exception."""
    broken = _broken("linode_domain_get", error_message="")
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=broken),
        pytest.raises(DriverError, match="declares no error_message"),
    ):
        await run_get_tool(
            sample_config,
            {},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
        )


async def test_get_driver_refuses_a_placeholder_naming_nothing(
    sample_config: Config,
) -> None:
    """A template naming neither a path value nor an argument fails rather
    than reaching a client with the brace text still in it."""
    broken = _broken("linode_domain_get", error_message="Failed on {zone_id}: {error}")
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=broken),
        pytest.raises(DriverError, match="zone_id"),
    ):
        await run_get_tool(
            sample_config,
            {},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
        )


async def test_body_read_driver_sends_its_body_and_decodes_the_answer(
    sample_config: Config,
) -> None:
    """A read on a body route posts what it built and answers with the resource.

    The download-URL create is the shape: the POST signs what the body describes
    and stores nothing, so the answer is decoded the way a read's is rather than
    assembled into a mutation envelope. The verb is not in the arguments because
    the tool pins it through body_constant.
    """
    client = _client(route_raw={"url": "https://example.test/signed"})
    body = {"name": "photo.jpg", "method": "GET"}
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_body_read_tool(
            sample_config,
            {"name": "photo.jpg"},
            tool="linode_object_storage_object_download_url_create",
            body=body,
            path_values={"region": "us-east-1", "label": "pics"},
        )

    client.route_raw.assert_awaited_once_with(
        "linode_object_storage_object_download_url_create",
        "us-east-1",
        "pics",
        body=body,
    )
    assert "https://example.test/signed" in result[0].text


async def test_body_read_driver_refuses_a_response_that_is_not_an_object(
    sample_config: Config,
) -> None:
    """A bare array reads as an empty message and reports success carrying
    nothing, where Go's decode of the same body fails, so it is refused here."""
    client = _client(route_raw=["not", "an", "object"])
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_body_read_tool(
            sample_config,
            {"name": "photo.jpg", "label": "pics"},
            tool="linode_object_storage_object_download_url_create",
            body={"name": "photo.jpg"},
            path_values={"region": "us-east-1", "label": "pics"},
        )

    assert (
        "object storage object download url create response must be a JSON object"
        in result[0].text
    )


async def test_body_read_driver_assembles_the_envelope_its_response_declares(
    sample_config: Config,
) -> None:
    """A body read whose answer wraps the decode reports the prose and the echo.

    The metric query is the shape: the samples land in the member the response
    declares, and the window the caller asked for is named beside them rather
    than left blank.
    """
    client = _client(route_raw={"cpu": [1.5]})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_body_read_tool(
            sample_config,
            {"service_type": "dbaas"},
            tool="linode_monitor_service_metric_query",
            body={},
            echo_args={"service_type": "dbaas"},
            path_values={"service_type": "dbaas"},
        )

    client.route_raw.assert_awaited_once_with(
        "linode_monitor_service_metric_query",
        "dbaas",
        body={},
        retry=False,
    )
    answered = json.loads(result[0].text)
    assert answered == {
        "message": "Monitor service metrics read for 'dbaas'",
        "service_type": "dbaas",
        "metrics": {"cpu": [1.5]},
    }


async def test_assembled_read_driver_places_what_its_transport_brought_back(
    sample_config: Config,
) -> None:
    """A read whose route answers with bytes builds its whole answer.

    Nothing decodes: the declared transport owns the transfer and the encoding,
    and the ids the call was addressed by are placed here beside what it
    returned.
    """
    client = _client()
    client.route_raw_body_read.return_value = b"PNGDATA"

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_assembled_read_tool(
            sample_config,
            {"client_id": "abc123"},
            tool="linode_account_oauth_client_thumbnail_get",
            transport=_thumbnail_transport(),
            assembled=("thumbnail_png_base64",),
            echo_members={"client_id": "abc123"},
            path_values={"client_id": "abc123"},
        )

    assert json.loads(result[0].text) == {
        "client_id": "abc123",
        "thumbnail_png_base64": "UE5HREFUQQ==",
    }
    client.route_raw_body_read.assert_awaited_once_with(
        "linode_account_oauth_client_thumbnail_get",
        "abc123",
        accept="image/png",
        retry=True,
    )
    client.route_raw.assert_not_awaited()


async def test_assembled_read_driver_refuses_a_transport_filling_other_members(
    sample_config: Config,
) -> None:
    """A transport that fills a member the response does not assemble is a defect.

    Go reads these off a typed response message, so its compiler catches both
    halves. Nothing types a dict, so a forgotten member would serialize as a
    zero the caller reads as real and an invented one would vanish; the driver
    says so instead.
    """
    client = _client()
    client.route_raw_body_read.return_value = b"PNGDATA"

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_assembled_read_tool(
            sample_config,
            {"client_id": "abc123"},
            tool="linode_account_oauth_client_thumbnail_get",
            transport=RawBody(content_type="image/png", answer_field="client_id"),
            assembled=("thumbnail_png_base64",),
            path_values={"client_id": "abc123"},
        )

    # Named rather than raised: the defect reaches the caller as the tool's own
    # error text, which is what every other contract disagreement here does.
    assert "want exactly ['thumbnail_png_base64']" in result[0].text


async def test_assembled_read_driver_reports_the_declared_failure_prose(
    sample_config: Config,
) -> None:
    """A failed fetch answers the sentence error_message declares, naming the id."""
    client = _client()
    client.route_raw_body_read.side_effect = APIError(404, "Not Found")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_assembled_read_tool(
            sample_config,
            {"client_id": "abc123"},
            tool="linode_account_oauth_client_thumbnail_get",
            transport=_thumbnail_transport(),
            assembled=("thumbnail_png_base64",),
            echo_members={"client_id": "abc123"},
            path_values={"client_id": "abc123"},
        )

    assert result[0].text == (
        "Failed to get account OAuth client thumbnail 'abc123':"
        " Linode API error (status 404): Not Found"
    )


def _thumbnail_transport() -> RawBody:
    """The arm the thumbnail read declares: bytes down, base64 into one member."""
    return RawBody(content_type="image/png", answer_field="thumbnail_png_base64")


async def test_body_read_driver_reports_the_declared_failure_prose(
    sample_config: Config,
) -> None:
    """A failed call answers the sentence error_message declares.

    The object name comes from the raw arguments and the bucket from the
    validated path values, which is what lets one sentence name both.
    """
    client = _client()
    client.route_raw.side_effect = APIError(500, "boom")
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_body_read_tool(
            sample_config,
            {"name": "photo.jpg"},
            tool="linode_object_storage_object_download_url_create",
            body={"name": "photo.jpg"},
            path_values={"region": "us-east-1", "label": "pics"},
        )

    assert result[0].text == (
        "Failed to generate download URL for 'photo.jpg' in bucket 'pics':"
        " Linode API error (status 500): boom"
    )


async def test_list_driver_answers_the_shared_failure_sentence(
    sample_config: Config,
) -> None:
    """A generated list names no verb, so the shared list sentence answers.

    error_message is deliberately unset across the list surface, and Go's list
    machinery has always answered this exact sentence, so both sides report a
    failed page the same way.
    """
    client = _client()
    client.route_raw.side_effect = APIError(500, "boom")
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(sample_config, {}, tool="linode_domain_list")

    assert result[0].text == (
        "Failed to retrieve items: Linode API error (status 500): boom"
    )


async def test_get_driver_fills_a_placeholder_from_the_arguments(
    sample_config: Config,
) -> None:
    """A name no path value carries is filled from the call's arguments.

    Path values win a name both hold, since they are the validated ones, but a
    template may also report an argument that never reaches the URL. That is
    what a query-carrying tool needs, and the driver serves every tier.
    """
    client = _client()
    client.route_raw.side_effect = APIError(500, "boom")
    broken = _broken(
        "linode_domain_get",
        error_message="Failed to retrieve domain {domain_id} of type {type}: {error}",
    )
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=broken),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_get_tool(
            sample_config,
            {"type": "master"},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
        )

    assert result[0].text.startswith("Failed to retrieve domain 5 of type master: ")


async def test_write_driver_previews_the_request_alone_without_a_state_fetch(
    sample_config: Config,
) -> None:
    """A create has no existing resource, so its preview reports what it would send."""
    result = await run_write_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_domain_create",
        error_action="",
        body={"domain": "example.com", "type": "master"},
        side_effects=("A new master DNS domain will be created.",),
    )

    preview = json.loads(result[0].text)

    assert preview["would_execute"]["method"] == "POST"
    assert preview["would_execute"]["path"] == "/domains"
    assert preview["would_execute"]["body"] == {
        "domain": "example.com",
        "type": "master",
    }
    assert preview["current_state"] is None
    assert preview["side_effects"] == ["A new master DNS domain will be created."]


async def test_write_driver_previews_against_fetched_state(
    sample_config: Config,
) -> None:
    """An update preview fetches first, so the declared prose lands beside the
    resource the change is about to alter."""

    async def fetch(_client: RetryableClient) -> Any:
        return {"description": "old"}

    client = _client(route_raw={})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"dry_run": True, "domain_id": 5},
            tool="linode_domain_update",
            error_action="",
            body={"description": "new"},
            path_values={"domain_id": 5},
            state_fetch=fetch,
            side_effects=("The DNS domain is updated.",),
            warnings=("Records under the domain keep their old values.",),
            preview_request_body=False,
        )

    preview = json.loads(result[0].text)

    assert preview["current_state"] == {"description": "old"}
    assert preview["side_effects"] == ["The DNS domain is updated."]
    assert preview["warnings"] == ["Records under the domain keep their old values."]
    assert "body" not in preview["would_execute"]


async def test_write_driver_carries_the_estimate_past_a_state_read(
    sample_config: Config,
) -> None:
    """A declared estimate reaches the report whether or not the tool reads
    state first.

    Go carries it through one path for both, so the fetched arm has to answer
    the same way the fetchless one does or the two languages report a different
    dry run for one declaration.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {"description": "old"}

    estimate = {"monthly_change_usd": "unknown", "note": "Pricing varies by region."}

    client = _client(route_raw={})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"dry_run": True, "domain_id": 5},
            tool="linode_domain_update",
            error_action="",
            body={"description": "new"},
            path_values={"domain_id": 5},
            state_fetch=fetch,
            billing_delta=estimate,
        )

    assert json.loads(result[0].text)["billing_delta"] == estimate


async def test_write_driver_reports_the_declared_failure_sentence(
    sample_config: Config,
) -> None:
    """An empty verb asks for the sentence the contract declares.

    Only a declared one can name the ids the call was addressed by, which is
    the whole reason a handler leaves the verb out.
    """
    client = _client(route_raw=APIError(500, "upstream failure"))
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "domain_id": 5},
            tool="linode_domain_update",
            error_action="",
            body={},
            path_values={"domain_id": 5},
        )

    assert result[0].text.startswith("Failed to modify domain 5:")


async def test_destroy_driver_removes_through_the_routed_primitive(
    sample_config: Config,
) -> None:
    """The removal is the route and nothing else.

    A delete sends no body and reads nothing back, so the tool and its ids are
    the whole call, and the generated destroys pass neither a client method nor
    a path.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_destructive_tool(
            sample_config,
            {"confirm": True, "domain_id": 5},
            tool="linode_domain_delete",
            error_action="",
            id_args={"domain_id": 5},
            fetch_state=fetch,
        )

    client.route_call.assert_awaited_once_with("linode_domain_delete", 5, retry=False)
    assert '"domain_id": 5' in result[0].text


@pytest.mark.parametrize(
    ("tool", "id_name", "want_retry_off"),
    [
        ("linode_lke_acl_delete", "cluster_id", True),
        ("linode_instance_backups_cancel", "linode_id", False),
    ],
    ids=["unretryable delete", "replayable cancel"],
)
async def test_destroy_driver_honors_the_declared_retry_policy(
    sample_config: Config, tool: str, id_name: str, want_retry_off: bool
) -> None:
    """A route the contract marks unretryable takes one protected attempt.

    Every DELETE the tier serves refuses a replay, so the replayable half is
    the POST actions, and reading the policy off the contract is what keeps the
    two apart rather than each call site deciding.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await run_destructive_tool(
            sample_config,
            {"confirm": True, id_name: 7},
            tool=tool,
            error_action="",
            id_args={id_name: 7},
            fetch_state=fetch,
        )

    if want_retry_off:
        client.route_call.assert_awaited_once_with(tool, 7, retry=False)
    else:
        client.route_call.assert_awaited_once_with(tool, 7)


async def test_destroy_driver_reports_the_shared_failure_sentence(
    sample_config: Config,
) -> None:
    """A caller naming no verb takes the sentence Go's destroy flow answers with.

    Leaving it to "Failed to {error_action}" with an empty verb would report
    "Failed to :", and the two languages would say different things about the
    same failed delete.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    client = _client()
    client.route_call.side_effect = APIError(500, "boom")
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_destructive_tool(
            sample_config,
            {"confirm": True, "domain_id": 5},
            tool="linode_domain_delete",
            error_action="",
            id_args={"domain_id": 5},
            fetch_state=fetch,
        )

    assert result[0].text.startswith("linode_domain_delete failed: ")


async def test_get_driver_sends_the_query_a_route_publishes(
    sample_config: Config,
) -> None:
    """A single-resource route can take a query of its own.

    /databases/types/{id} publishes the same page controls a collection does,
    and the bucket ACL route is addressed by an object key, so the read tier
    has to carry a query where the contract declares one.
    """
    client = _client(route_raw={"id": "g6-standard-1"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await run_get_tool(
            sample_config,
            {},
            tool="linode_database_type_get",
            path_values={"type_id": "g6-standard-1"},
            query="page=1&page_size=25",
        )

    client.route_raw.assert_awaited_once_with(
        "linode_database_type_get", "g6-standard-1", query="page=1&page_size=25"
    )


async def test_get_driver_answers_under_the_envelope_member_it_is_given(
    sample_config: Config,
) -> None:
    """linode_volume_get has always answered {"volume": ...}.

    /volumes/{id} answers a bare volume, so a reader that serialized the body
    straight into the declared envelope would answer an empty object.
    """
    client = _client(route_raw={"id": 42, "label": "data"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {},
            tool="linode_volume_get",
            path_values={"volume_id": 42},
            wrapper_field="volume",
        )

    client.route_raw.assert_awaited_once_with("linode_volume_get", 42)
    answered = json.loads(result[0].text)
    assert answered["volume"]["id"] == 42
    assert answered["volume"]["label"] == "data"


async def test_list_driver_forwards_the_parameters_the_route_filters_on(
    sample_config: Config,
) -> None:
    """A forwarded argument reaches the API, where a filter narrows a page here.

    An absent one is left out rather than sent empty: an empty prefix and no
    prefix select different objects.
    """
    client = _client(route_raw={"data": []})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await run_list_tool(
            sample_config,
            {"skip_ipv6_rdns": True, "prefix": ""},
            tool="linode_domain_list",
            error_action="retrieve domains",
            pagination=None,
            forwarded=("skip_ipv6_rdns", "prefix"),
        )

    client.route_raw.assert_awaited_once_with(
        "linode_domain_list", query="skip_ipv6_rdns=true"
    )


@pytest.mark.parametrize(
    ("value", "want"),
    [
        (True, "skip_ipv6_rdns=true"),
        (False, ""),
        ("images/", "skip_ipv6_rdns=images%2F"),
        (25, "skip_ipv6_rdns=25"),
        (2.5, "skip_ipv6_rdns=2.5"),
        (["a"], ""),
        (None, ""),
    ],
    ids=["true flag", "false flag", "text", "int", "float", "list", "null"],
)
async def test_forwarded_argument_travels_as_the_api_reads_it(
    sample_config: Config, value: object, want: str
) -> None:
    """Each argument type reaches the route the way the hand tools sent it.

    A flag travels only when it is true, since asking the route for
    skip_ipv6_rdns=false asks for something it does not offer, and a value the
    reader has no spelling for is left out rather than stringified.
    """
    client = _client(route_raw={"data": []})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await run_list_tool(
            sample_config,
            {"skip_ipv6_rdns": value},
            tool="linode_domain_list",
            error_action="retrieve domains",
            pagination=None,
            forwarded=("skip_ipv6_rdns",),
        )

    client.route_raw.assert_awaited_once_with("linode_domain_list", query=want)


async def test_list_driver_answers_the_declared_sentence_when_there_is_one(
    sample_config: Config,
) -> None:
    """A collection declaring error_message says something its callers know.

    Answering the shared list sentence over that declaration was prose drift
    no gate saw, so the declaration wins wherever there is one.
    """
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    client.route_raw.side_effect = APIError(500, "boom")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool="linode_object_storage_bucket_by_region_list",
            path_values={"region": "us-east"},
        )

    assert "Failed to retrieve Object Storage buckets in region 'us-east'" in (
        result[0].text
    )


async def test_list_driver_answers_the_shared_sentence_without_a_declaration(
    sample_config: Config,
) -> None:
    """Nearly every collection declares none, and answers the one shared line."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    client.route_raw.side_effect = APIError(500, "boom")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(sample_config, {}, tool="linode_domain_list")

    assert "Failed to retrieve items" in result[0].text


async def test_list_driver_reads_a_singleton_body_as_one_element(
    sample_config: Config,
) -> None:
    """The rule-version history route answers one object, not a page of them."""
    client = _client(route_raw={"id": 9, "version": 3})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool="linode_firewall_rule_version_list",
            path_values={"firewall_id": 9},
        )

    answered = json.loads(result[0].text)
    assert answered["count"] == 1
    assert answered["firewall_rule_versions"][0]["id"] == 9


async def test_list_driver_refuses_a_singleton_that_is_not_an_object(
    sample_config: Config,
) -> None:
    """An array or a null would decode to one blank element rather than fail."""
    client = _client(route_raw=[{"id": 9}])
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool="linode_firewall_rule_version_list",
            path_values={"firewall_id": 9},
        )

    assert "list response must be an object" in result[0].text


async def test_list_driver_matches_one_entry_of_a_list_valued_field(
    sample_config: Config,
) -> None:
    """A region publishes many capabilities and a caller asks about one.

    Equality against the whole list cannot express that, which is why the
    member match exists rather than the argument being sent to the API.
    """
    page = {
        "data": [
            {"id": "us-east", "capabilities": ["Linodes", "GPU Linodes"]},
            {"id": "eu-west", "capabilities": ["Linodes"]},
        ]
    }
    client = _client(route_raw=page)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {"capability": "gpu linodes"},
            tool="linode_region_list",
            filters=(ListFilter("capability", "capabilities", mode=MatchMode.MEMBER),),
        )

    answered = json.loads(result[0].text)
    assert answered["count"] == 1
    assert answered["regions"][0]["id"] == "us-east"


async def test_list_driver_member_match_skips_a_field_that_is_not_a_list(
    sample_config: Config,
) -> None:
    """A filter narrows a page, so an item the API shaped oddly does not qualify."""
    page = {"data": [{"id": "us-east", "capabilities": "Linodes"}]}
    client = _client(route_raw=page)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {"capability": "Linodes"},
            tool="linode_region_list",
            filters=(ListFilter("capability", "capabilities", mode=MatchMode.MEMBER),),
        )

    assert json.loads(result[0].text)["count"] == 0


async def test_failure_template_zero_pads_the_width_it_declares(
    sample_config: Config,
) -> None:
    """A month reads as 03 rather than 3, which is what a date looks like.

    The hand-written handler this replaces rendered %02d, so a template without
    the width answered a different sentence for every month before October.
    """
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    client.route_raw.side_effect = APIError(500, "boom")

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {"linode_id": 7, "year": 2024, "month": 3},
            tool="linode_instance_stats_month_get",
            path_values={"linode_id": 7, "year": 2024, "month": 3},
        )

    assert "for instance 7 in 2024-03" in result[0].text


async def test_acknowledge_driver_reports_the_preview_verdict_before_confirm(
    sample_config: Config,
) -> None:
    """A preview reports a bad argument straight away; a live call gates first."""
    preview = await run_acknowledge_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_volume_detach",
        echo_args={"volume_id": 0},
        path_values={"volume_id": 0},
        error="volume_id is required",
        preview_error="volume_id is required",
    )
    assert "volume_id is required" in preview[0].text

    live = await run_acknowledge_tool(
        sample_config,
        {},
        tool="linode_volume_detach",
        echo_args={"volume_id": 0},
        path_values={"volume_id": 0},
        error="volume_id is required",
        preview_error="volume_id is required",
    )
    assert "Set confirm=true to proceed." in live[0].text


async def test_acknowledge_driver_previews_the_request_when_no_hook_owns_it(
    sample_config: Config,
) -> None:
    """A tool with no preview hook reports the call it would have made."""
    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"dry_run": True, "volume_id": 7},
            tool="linode_volume_detach",
            echo_args={"volume_id": 7},
            path_values={"volume_id": 7},
        )

    body = json.loads(result[0].text)
    assert body["would_execute"] == {
        "method": "POST",
        "path": "/volumes/7/detach",
    }
    client.route_call.assert_not_called()


async def test_acknowledge_driver_stands_in_for_one_member_of_each_entry(
    sample_config: Config,
) -> None:
    """The entries keep their order and every other member.

    The twin lives in go/internal/tools/gentools_body_test.go: both languages
    report the same stand-in so one fixture covers the pair.
    """
    body = {
        "security_questions": [
            {"question_id": 1, "response": "first answer"},
            {"question_id": 7, "response": "second answer"},
        ]
    }

    result = await run_acknowledge_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_profile_security_question_answer",
        echo_args={},
        body=body,
        stand_in_preview=(
            PreviewStandIn("security_questions", "response", "[redacted]"),
        ),
    )

    reported = json.loads(result[0].text)["would_execute"]["body"]
    assert reported == {
        "security_questions": [
            {"question_id": 1, "response": "[redacted]"},
            {"question_id": 7, "response": "[redacted]"},
        ]
    }
    assert body["security_questions"][0]["response"] == "first answer"


async def test_acknowledge_driver_stands_in_for_nothing_it_cannot_reach(
    sample_config: Config,
) -> None:
    """A member carrying no entries is reported as the body built it.

    Neither shape survives the checks a declared stand-in runs behind, so this
    is about the report never inventing what it was not given.
    """
    result = await run_acknowledge_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_profile_security_question_answer",
        echo_args={},
        body={"security_questions": "not a list", "note": "kept"},
        stand_in_preview=(
            PreviewStandIn("security_questions", "response", "[redacted]"),
            PreviewStandIn("nobody", "response", "[redacted]"),
        ),
    )

    reported = json.loads(result[0].text)["would_execute"]["body"]
    assert reported == {"security_questions": "not a list", "note": "kept"}


async def test_acknowledge_driver_leaves_an_entry_it_cannot_reach_alone(
    sample_config: Config,
) -> None:
    """A member the caller left out is not one the report invents.

    The twin is TestWriteBodyStandingInLeavesAnEntryWithoutTheMemberAlone.
    """
    result = await run_acknowledge_tool(
        sample_config,
        {"dry_run": True},
        tool="linode_profile_security_question_answer",
        echo_args={},
        body={"security_questions": [{"question_id": 9}, "not an object"]},
        stand_in_preview=(
            PreviewStandIn("security_questions", "response", "[redacted]"),
        ),
    )

    reported = json.loads(result[0].text)["would_execute"]["body"]
    assert reported == {"security_questions": [{"question_id": 9}, "not an object"]}


async def test_acknowledge_driver_sends_no_body_when_it_was_given_none(
    sample_config: Config,
) -> None:
    """An empty JSON object is a body, and these routes are called without one."""
    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"confirm": True, "volume_id": 7},
            tool="linode_volume_detach",
            echo_args={"volume_id": 7},
            path_values={"volume_id": 7},
        )

    client.route_call.assert_awaited_once_with("linode_volume_detach", 7)
    client.route_raw.assert_not_called()
    assert json.loads(result[0].text) == {
        "message": "Volume 7 detached successfully",
        "volume_id": 7,
    }


async def test_acknowledge_driver_sends_the_body_it_was_given(
    sample_config: Config,
) -> None:
    """A tool declaring body fields puts them on the wire and decodes nothing."""
    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"confirm": True},
            tool="linode_account_promo_credit_add",
            echo_args={"promo_code": "SPRING"},
            body={"promo_code": "SPRING"},
        )

    client.route_raw.assert_awaited_once_with(
        "linode_account_promo_credit_add", body={"promo_code": "SPRING"}
    )
    assert json.loads(result[0].text) == {
        "message": "Account promo credit applied successfully",
        "promo_code": "SPRING",
    }


async def test_acknowledge_driver_hands_the_live_call_to_a_declared_transport(
    sample_config: Config,
) -> None:
    """The transport makes the call and the driver still assembles the answer.

    The routes needing this send something other than a JSON body, so the
    routed request has to be replaced rather than adjusted. Everything around
    it stays the driver's: the gate, the prose, the echo.
    """
    client = _client()

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"confirm": True, "volume_id": 7},
            tool="linode_volume_detach",
            echo_args={"volume_id": 7},
            path_values={"volume_id": 7},
            transport=_probe_transport(),
        )

    client.route_multipart.assert_awaited_once()
    client.route_call.assert_not_called()
    client.route_raw.assert_not_called()
    assert json.loads(result[0].text) == {
        "message": "Volume 7 detached successfully",
        "volume_id": 7,
    }


async def test_acknowledge_driver_keeps_a_transport_out_of_the_dry_run(
    sample_config: Config,
) -> None:
    """A preview makes no call, so it never reaches the transport that makes one."""
    client = _client()

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"dry_run": True, "volume_id": 7},
            tool="linode_volume_detach",
            echo_args={"volume_id": 7},
            path_values={"volume_id": 7},
            transport=_probe_transport(),
        )

    client.route_multipart.assert_not_called()
    assert json.loads(result[0].text)["would_execute"] == {
        "method": "POST",
        "path": "/volumes/7/detach",
    }


async def test_acknowledge_driver_gates_a_transport_behind_confirm(
    sample_config: Config,
) -> None:
    """The transport is the call, so it waits behind the same gate the call does."""
    client = _client()

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"volume_id": 7},
            tool="linode_volume_detach",
            echo_args={"volume_id": 7},
            path_values={"volume_id": 7},
            transport=_probe_transport(),
        )

    client.route_multipart.assert_not_called()
    assert "Set confirm=true to proceed." in result[0].text


def _probe_transport() -> MultipartUpload:
    """The cheapest arm to stand in for a live call: it reads one argument and
    fills no response member, so a case about the driver says nothing about the
    transfer."""
    return MultipartUpload(file_argument="file", part_name="file")


async def test_acknowledge_driver_echoes_a_member_the_call_carried(
    sample_config: Config,
) -> None:
    """The bucket-access routes answer with no body, so the member comes back.

    Decoding the empty answer into ObjectStorageBucketAccess reports an ACL of
    "" over a change already made, which is the wrong answer this echo exists
    to replace. Both hand handlers report the same object today.
    """
    client = _client()
    access = {"acl": "public-read", "cors_enabled": False}
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_acknowledge_tool(
            sample_config,
            {"confirm": True, "region": "us-east", "label": "my-bucket"},
            tool="linode_object_storage_bucket_access_allow",
            echo_args={"access": access},
            body={"acl": "public-read"},
            path_values={"region": "us-east", "label": "my-bucket"},
        )

    assert json.loads(result[0].text) == {
        "message": (
            "Access settings for bucket 'my-bucket' in us-east applied successfully"
        ),
        "access": access,
    }


async def test_get_driver_answers_a_free_form_body_as_the_object_it_decoded(
    sample_config: Config,
) -> None:
    """A read whose API body has no proto model answers with the body itself.

    There is no message to place it in, so the object is the answer and the
    keys are sorted at every level the way Go's structpb + protojson sorts
    them.
    """
    client = _client(route_raw={"b": {"d": 2, "c": 1.0}, "a": True})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {},
            tool="linode_profile_preferences_get",
            struct_response=True,
        )

    client.route_raw.assert_awaited_once_with("linode_profile_preferences_get")
    assert result[0].text == json.dumps({"a": True, "b": {"c": 1, "d": 2}}, indent=2)


async def test_get_driver_reports_a_free_form_conversion_failure_as_declared(
    sample_config: Config,
) -> None:
    """Both ways a free-form read fails word the failure the same way.

    Go hands its serializer the declared sentence minus the error and appends
    the conversion failure to it; this side renders the same template through
    the shared reporter, so a body the Struct cannot hold reads alike in both.
    """
    client = _client(route_raw={"nested": object()})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {},
            tool="linode_profile_preferences_get",
            struct_response=True,
        )

    assert result[0].text.startswith("Failed to retrieve profile preferences: ")


def _gated(tool: str) -> Contract:
    """A read contract carrying the gate prose a gated read answers with.

    No read on the surface declares one yet, so the branch is proven against a
    stand-in rather than against a tool written to be gated.
    """
    return _broken(tool, confirm_message="This reads a secret. Set confirm=true.")


async def test_gated_read_previews_the_route_without_calling_it(
    sample_config: Config,
) -> None:
    """A dry run reports the call it would make and makes nothing.

    That is the whole point of gating a read: the answer is the secret, so
    seeing the request has to be reachable without producing it.
    """
    client = _client()
    with (
        patch(
            "linodemcp.tools.drivers.contract_for",
            return_value=_gated("linode_domain_get"),
        ),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_get_tool(
            sample_config,
            {"dry_run": True, "domain_id": 5},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
            gated=True,
            error=None,
            preview_error=None,
        )

    client.route_raw.assert_not_awaited()
    assert json.loads(result[0].text)["would_execute"] == {
        "method": "GET",
        "path": "/domains/5",
    }


async def test_gated_read_reports_its_preview_verdict_before_the_preview(
    sample_config: Config,
) -> None:
    """A dry run answers a rejected argument ahead of everything else."""
    with patch(
        "linodemcp.tools.drivers.contract_for", return_value=_gated("linode_domain_get")
    ):
        result = await run_get_tool(
            sample_config,
            {"dry_run": True},
            tool="linode_domain_get",
            path_values={"domain_id": 0},
            gated=True,
            error="domain_id is required",
            preview_error="domain_id is required",
        )

    assert result[0].text == "Error: domain_id is required"


async def test_gated_read_holds_the_fetch_behind_the_confirm_gate(
    sample_config: Config,
) -> None:
    """An unconfirmed live call answers the declared gate and calls nothing."""
    client = _client()
    with (
        patch(
            "linodemcp.tools.drivers.contract_for",
            return_value=_gated("linode_domain_get"),
        ),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_get_tool(
            sample_config,
            {"domain_id": 5},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
            gated=True,
            error=None,
            preview_error=None,
        )

    client.route_raw.assert_not_awaited()
    assert result[0].text == "Error: This reads a secret. Set confirm=true."


async def test_gated_read_reports_its_verdict_only_after_the_gate(
    sample_config: Config,
) -> None:
    """A caller who has not confirmed learns nothing about their arguments.

    Both calls below carry the same rejected argument. The unconfirmed one
    hears the gate, and only the confirmed one hears why the arguments were
    refused, which is the ordering rule this driver exists to hold.
    """
    with patch(
        "linodemcp.tools.drivers.contract_for", return_value=_gated("linode_domain_get")
    ):
        withheld = await run_get_tool(
            sample_config,
            {},
            tool="linode_domain_get",
            path_values={"domain_id": 0},
            gated=True,
            error="domain_id is required",
            preview_error="domain_id is required",
        )
        told = await run_get_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_get",
            path_values={"domain_id": 0},
            gated=True,
            error="domain_id is required",
            preview_error="domain_id is required",
        )

    assert withheld[0].text == "Error: This reads a secret. Set confirm=true."
    assert told[0].text == "Error: domain_id is required"


async def test_gated_read_fetches_once_confirmed(sample_config: Config) -> None:
    """A confirmed call with accepted arguments reaches the route."""
    client = _client(route_raw={"id": 5, "domain": "example.com"})
    with (
        patch(
            "linodemcp.tools.drivers.contract_for",
            return_value=_gated("linode_domain_get"),
        ),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_get_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
            gated=True,
            error=None,
            preview_error=None,
        )

    client.route_raw.assert_awaited_once_with("linode_domain_get", 5)
    assert "example.com" in result[0].text


async def test_ungated_read_answers_without_a_confirm_gate(
    sample_config: Config,
) -> None:
    """A read that is not gated is untouched by any of the above.

    It carries no gate, so an unconfirmed call still fetches, which is what
    every read on the surface does today.
    """
    client = _client(route_raw={"id": 5, "domain": "example.com"})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_get_tool(
            sample_config,
            {},
            tool="linode_domain_get",
            path_values={"domain_id": 5},
        )

    client.route_raw.assert_awaited_once_with("linode_domain_get", 5)
    assert "example.com" in result[0].text


# The one route on the surface declaring the marker envelope.
_MARKER_TOOL = "linode_object_storage_bucket_object_list"


async def test_marker_list_reports_the_cursor_beside_its_elements(
    sample_config: Config,
) -> None:
    """The cursor reaches the answer, which is why this shape has its own reader.

    Read as the page envelope, is_truncated and next_marker would be dropped
    and a caller holding a truncated page would see the whole collection.
    """
    client = _client(
        route_raw={
            "data": [{"name": "images/a.png"}, {"name": "images/b.png"}],
            "is_truncated": True,
            "next_marker": "images/b.png",
        }
    )
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {"prefix": "images/", "marker": "images/a.png", "page_size": "2"},
            tool=_MARKER_TOOL,
            path_values={"region": "us-east-1", "label": "photos"},
            forwarded=("prefix", "delimiter", "marker", "page_size"),
            echoes=("prefix", "delimiter"),
            pagination=None,
        )

    body = json.loads(result[0].text)
    assert body["count"] == 2
    assert body["is_truncated"] is True
    assert body["next_marker"] == "images/b.png"
    assert body["filter"] == "prefix=images/"


async def test_marker_list_forwards_the_cursor_without_page_controls(
    sample_config: Config,
) -> None:
    """The bound travels as declared text, never through the integer reader."""
    client = _client(route_raw={"data": [], "is_truncated": False})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        await run_list_tool(
            sample_config,
            {"marker": "images/a.png", "page_size": "250"},
            tool=_MARKER_TOOL,
            path_values={"region": "us-east-1", "label": "photos"},
            forwarded=("prefix", "delimiter", "marker", "page_size"),
            echoes=("prefix", "delimiter"),
            pagination=None,
        )

    client.route_raw.assert_awaited_once_with(
        _MARKER_TOOL,
        "us-east-1",
        "photos",
        query="marker=images%2Fa.png&page_size=250",
    )


async def test_marker_list_omits_a_marker_the_route_did_not_hand_out(
    sample_config: Config,
) -> None:
    """A complete listing carries no resume point a caller could act on."""
    client = _client(route_raw={"data": [{"name": "only.png"}]})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool=_MARKER_TOOL,
            path_values={"region": "us-east-1", "label": "photos"},
            forwarded=("prefix", "delimiter", "marker", "page_size"),
            echoes=("prefix", "delimiter"),
            pagination=None,
        )

    body = json.loads(result[0].text)
    assert body["is_truncated"] is False
    assert "next_marker" not in body
    assert "filter" not in body


async def test_marker_list_echoes_no_cursor_argument(
    sample_config: Config,
) -> None:
    """A position is not a filter, so the echo names neither cursor control."""
    client = _client(route_raw={"data": [], "is_truncated": False})
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {
                "prefix": "images/",
                "delimiter": "/",
                "marker": "images/a.png",
                "page_size": "2",
            },
            tool=_MARKER_TOOL,
            path_values={"region": "us-east-1", "label": "photos"},
            forwarded=("prefix", "delimiter", "marker", "page_size"),
            echoes=("prefix", "delimiter"),
            pagination=None,
        )

    assert json.loads(result[0].text)["filter"] == "prefix=images/, delimiter=/"


async def test_marker_list_holds_the_body_to_being_an_object(
    sample_config: Config,
) -> None:
    """An array body carries no cursor, and decoding it as one would answer a
    real failure as an empty untruncated page."""
    client = _client(route_raw=[{"name": "a.png"}])
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_list_tool(
            sample_config,
            {},
            tool=_MARKER_TOOL,
            path_values={"region": "us-east-1", "label": "photos"},
            forwarded=("prefix", "delimiter", "marker", "page_size"),
            echoes=("prefix", "delimiter"),
            pagination=None,
        )

    assert "must be an object" in result[0].text


@pytest.mark.parametrize(
    ("value", "want"),
    [(True, "true"), (False, "false"), (1, "1"), (0, "0"), ("yes", "yes")],
    ids=["true", "false", "one", "zero", "text"],
)
async def test_write_driver_renders_an_argument_the_way_gos_verb_prints_it(
    sample_config: Config, value: object, want: str
) -> None:
    """A flag is the one value the two languages would spell differently.

    Go's %t writes true and false where str() writes True and False, so a
    sentence naming a flag would read one way in each language. The bool is
    answered ahead of the integers because a bool is an int here.
    """
    flagged = _broken("linode_domain_create", success_message="flag {flag}")
    client = _client(route_raw={"id": 5, "domain": "example.com"})
    with (
        patch("linodemcp.tools.drivers.contract_for", return_value=flagged),
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
    ):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "flag": value},
            tool="linode_domain_create",
            error_action="create domain",
            body={},
        )

    assert f'"message": "flag {want}"' in result[0].text


async def test_write_driver_answers_a_paged_mutation_from_the_list_envelope(
    sample_config: Config,
) -> None:
    """A mutation whose route reports a page is read as one.

    PUT /linode/instances/{id}/firewalls answers {data, page, pages, results},
    which carries none of FirewallListResponse's members. Serializing it into
    that response directly reports zero firewalls assigned over a replacement
    that already happened. Go's linode.ListProtoRouteBody reads the same body.
    """
    page = {
        "data": [{"id": 11, "label": "web"}],
        "page": 1,
        "pages": 1,
        "results": 1,
    }
    client = _client(route_raw=page)
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_write_tool(
            sample_config,
            {"confirm": True, "linode_id": 5},
            tool="linode_instance_firewall_update",
            error_action="",
            body={"firewall_ids": [11]},
            path_values={"linode_id": 5},
            query="page=2",
            paged=True,
        )

    answered = json.loads(result[0].text)

    assert answered["count"] == 1
    assert answered["firewalls"][0]["id"] == 11
    assert answered["firewalls"][0]["label"] == "web"
    # A mutation narrows nothing, so the page's filter echo stays absent.
    assert "filter" not in answered
    assert client.route_raw.await_args.kwargs["query"] == "page=2"


async def test_write_driver_previews_the_route_with_its_query(
    sample_config: Config,
) -> None:
    """A dry run reports the call the live branch would make, page controls and all.

    The controls decide which assignments come back, so a preview naming the
    bare path describes a different request. Go's tools.PathWithQuery spells
    the reported route the same way.
    """
    result = await run_write_tool(
        sample_config,
        {"dry_run": True, "linode_id": 5},
        tool="linode_instance_firewall_update",
        error_action="",
        body={"firewall_ids": [11]},
        path_values={"linode_id": 5},
        query="page=2",
        paged=True,
    )

    assert "/linode/instances/5/firewalls?page=2" in result[0].text


async def test_read_route_state_decodes_the_member_the_resource_sits_under() -> None:
    """The control-plane ACL route wraps its resource under a key. Decoding the
    envelope would report an empty resource as the state a delete removes.
    """
    client = _client(
        route_raw={"acl": {"enabled": True, "addresses": {"ipv4": ["203.0.113.1/32"]}}}
    )

    state = await read_route_state(
        client, 12345, tool="linode_lke_acl_get", member="acl"
    )

    assert state.fields["enabled"] is True
    assert "acl" not in state.fields


async def test_read_route_state_refuses_an_answer_missing_its_member() -> None:
    """A wrapper that lost its key is reported rather than previewed as empty."""
    client = _client(route_raw={"something_else": {}})

    with pytest.raises(TypeError, match="carries no member: acl"):
        await read_route_state(client, 12345, tool="linode_lke_acl_get", member="acl")


async def test_read_route_state_reports_the_nulls_the_api_sent() -> None:
    """A key the API sent as null survives, so a plan does not hash it the same
    as one the API never mentioned. No declaration says which keys those are:
    the projection reports what the body carried.
    """
    client = _client(
        route_raw={"address": "203.0.113.10", "gateway": None, "rdns": None}
    )

    state = await read_route_state(
        client, "203.0.113.10", tool="linode_networking_reserved_ip_get"
    )

    assert state.fields == {"address": "203.0.113.10", "gateway": None, "rdns": None}


# The presigned upload's dry run through the acknowledge driver: the guard's
# measurement reaches the lines that word it, and a refused source is a warning.
UPLOAD_TRANSPORT = PresignTransfer(
    url_field="url",
    local_path_argument="source_path",
    up=True,
    content_type_argument="content_type",
    size_field="size_bytes",
    etag_field="etag",
    constants={"upload_mode": "single"},
)
UPLOAD_SIZE_WORDING = "{transport:size_bytes} bytes will be uploaded to '{name}'."


def _upload_effects(transfer: Any) -> tuple[str, ...]:
    """Word the one declared line against what the guard measured."""
    return (
        preview_sentence(
            {"transport:size_bytes": transfer.size_bytes, "name": "k"},
            UPLOAD_SIZE_WORDING,
        ),
    )


async def _upload_preview(
    sample_config: Config, source_path: str, transport: Any = UPLOAD_TRANSPORT
) -> dict[str, Any]:
    result = await run_acknowledge_tool(
        sample_config,
        {
            "dry_run": True,
            "region": "us-east",
            "label": "artifacts",
            "name": "k",
            "source_path": source_path,
        },
        tool="linode_object_storage_object_upload",
        echo_args={"label": "artifacts", "name": "k"},
        body={"name": "k"},
        path_values={"region": "us-east", "label": "artifacts"},
        side_effects=_upload_effects,
        transport=transport,
        preview_transfer=True,
    )
    decoded: dict[str, Any] = json.loads(result[0].text)
    return decoded


async def test_acknowledge_driver_words_the_preview_against_the_measured_transfer(
    sample_config: Config, tmp_path: Path
) -> None:
    """The size line reads what the guard measured, and the presign body is
    filled the way the live call fills it."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")

    answer = await _upload_preview(sample_config, str(source))

    assert answer["side_effects"] == ["20 bytes will be uploaded to 'k'."]
    assert answer["warnings"] == []
    assert answer["would_execute"]["body"] == {
        "name": "k",
        "content_type": "application/octet-stream",
        "expires_in": 3600,
    }


async def test_acknowledge_driver_warns_about_a_transfer_the_guard_refuses(
    sample_config: Config, tmp_path: Path
) -> None:
    """The refusal is reported and the size line, reading nothing, is dropped."""
    answer = await _upload_preview(sample_config, str(tmp_path / "absent.bin"))

    assert answer["side_effects"] == []
    assert len(answer["warnings"]) == 1
    assert "no readable file" in answer["warnings"][0]


async def test_acknowledge_driver_refuses_a_transfer_preview_it_cannot_measure(
    sample_config: Config, tmp_path: Path
) -> None:
    """Only a presigned upload has a source to measure; anything else reaching
    the flag is a contract defect, named as one."""
    download = replace(UPLOAD_TRANSPORT, up=False, content_type_argument="")

    for transport in (
        download,
        MultipartUpload(file_argument="file", part_name="file"),
    ):
        with pytest.raises(TypeError, match="does not declare as a presigned upload"):
            await _upload_preview(sample_config, str(tmp_path / "small.bin"), transport)
