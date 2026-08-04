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
    run_destructive_tool,
    run_get_tool,
    run_list_tool,
    run_write_tool,
)

if TYPE_CHECKING:
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
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    async def execute(_client: RetryableClient) -> None:
        pytest.fail("the call must be refused before anything is deleted")

    client = _client()
    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        pytest.raises(DriverError, match="declares no success_message"),
    ):
        await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_delete",
            error_action="delete domain",
            id_args={"domain_id": 5},
            fetch_state=fetch,
            execute=execute,
        )


async def test_destroy_driver_refuses_an_echo_it_cannot_fill(
    sample_config: Config,
) -> None:
    """A response echoing a value the call was never given raises.

    The echo confirms which resource was removed, so filling it with a guess
    would report a deletion of something the caller did not ask about. The
    SSL-certificate delete is the live case: it is addressed by region and
    label but answers with the bucket, which the path never names.
    """

    async def fetch(_client: RetryableClient) -> Any:
        return {}

    async def execute(_client: RetryableClient) -> None:
        return None

    client = _client()
    with (
        patch("linodemcp.tools.helpers.RetryableClient", return_value=client),
        pytest.raises(DriverError, match="names no path id it was given"),
    ):
        await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_object_storage_ssl_delete",
            error_action="delete the SSL certificate",
            id_args={"region": "us-east", "label": "assets"},
            fetch_state=fetch,
            execute=execute,
            success_message="gone",
        )


async def test_destroy_driver_fills_the_echo_from_the_ids(
    sample_config: Config,
) -> None:
    """Every echo field named by an id is filled from the value given."""
    deleted: list[str] = []

    async def fetch(_client: RetryableClient) -> Any:
        return {"id": 7}

    async def execute(_client: RetryableClient) -> None:
        deleted.append("done")

    client = _client()
    with patch("linodemcp.tools.helpers.RetryableClient", return_value=client):
        result = await run_destructive_tool(
            sample_config,
            {"confirm": True},
            tool="linode_domain_record_delete",
            error_action="delete DNS record",
            id_args={"domain_id": 5, "record_id": 7},
            fetch_state=fetch,
            execute=execute,
            success_message="Record 7 removed successfully from domain 5",
        )

    body = result[0].text
    assert deleted == ["done"]
    assert '"domain_id": 5' in body
    assert '"record_id": 7' in body
    assert '"message": "Record 7 removed successfully from domain 5"' in body


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

    async def execute(_client: RetryableClient) -> None:
        pytest.fail("the call must be refused before anything is deleted")

    broken = _broken("linode_domain_delete", response="linode.mcp.v1.Domain")
    with (
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
            execute=execute,
            success_message="gone",
        )


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
