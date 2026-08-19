"""Tests for Longview tools."""

from typing import Any, cast

import pytest

from linodemcp import gentools as gentools_mod
from linodemcp.gentools import longview as longview_gen
from linodemcp.profiles import Capability


def _text(result: list[Any]) -> str:
    return str(result[0].text)


def test_longview_client_delete_tool_schema() -> None:
    tool, capability = gentools_mod.create_linode_longview_client_delete_tool()

    assert tool.name == "linode_longview_client_delete"
    assert capability is Capability.Destroy
    properties = tool.input_schema["properties"]
    assert properties["client_id"]["type"] == "integer"
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    assert tool.input_schema["required"] == ["client_id", "confirm"]


def test_longview_client_get_tool_schema() -> None:
    tool, capability = gentools_mod.create_linode_longview_client_get_tool()

    assert tool.name == "linode_longview_client_get"
    assert capability is Capability.Read
    assert tool.input_schema["required"] == ["client_id"]
    assert "client_id" in tool.input_schema["properties"]


@pytest.mark.asyncio
async def test_longview_client_get_handler_sanitizes_sensitive_fields(
    sample_config: Any, mock_linode_client: Any
) -> None:
    mock_linode_client.route_raw.return_value = {
        "id": 123,
        "label": "prod-longview",
        "api_key": "secret",
        "install_code": "install",
    }

    result = await gentools_mod.handle_linode_longview_client_get(
        {"client_id": 123}, sample_config
    )

    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_longview_client_get", 123
    )
    text = _text(result)
    assert "prod-longview" in text
    assert "api_key" not in text
    assert "install_code" not in text
    assert "secret" not in text
    assert "install" not in text


@pytest.mark.asyncio
@pytest.mark.parametrize("client_id", [None, 0, -1, True, "123", "1/2", "1?x=2", ".."])
async def test_longview_client_get_handler_rejects_invalid_client_id(
    monkeypatch: pytest.MonkeyPatch, client_id: object
) -> None:
    async def fake_run_get_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("run_get_tool should not be called")

    monkeypatch.setattr(longview_gen, "run_get_tool", fake_run_get_tool)
    arguments: dict[str, Any] = {}
    if client_id is not None:
        arguments["client_id"] = client_id

    result = await gentools_mod.handle_linode_longview_client_get(
        arguments, cast("Any", object())
    )

    assert "client_id" in _text(result)


def test_longview_plan_get_tool_schema() -> None:
    tool, capability = gentools_mod.create_linode_longview_plan_get_tool()

    assert tool.name == "linode_longview_plan_get"
    assert capability is Capability.Read
    assert "environment" in tool.input_schema["properties"]
    assert "required" not in tool.input_schema


def test_longview_types_list_tool_schema() -> None:
    tool, capability = gentools_mod.create_linode_longview_type_list_tool()

    assert tool.name == "linode_longview_type_list"
    assert capability is Capability.Read
    assert "environment" in tool.input_schema["properties"]
    assert "required" not in tool.input_schema


def test_longview_clients_list_tool_schema() -> None:
    tool, capability = gentools_mod.create_linode_longview_client_list_tool()

    assert tool.name == "linode_longview_client_list"
    assert capability is Capability.Read
    properties = tool.input_schema["properties"]
    assert properties["page"]["type"] == "integer"
    assert properties["page_size"]["type"] == "integer"
    assert "required" not in tool.input_schema


@pytest.mark.asyncio
async def test_longview_clients_list_handler_calls_client_with_pagination(
    sample_config: Any, mock_linode_client: Any
) -> None:
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": 123}],
        "page": 2,
        "pages": 3,
    }

    result = await gentools_mod.handle_linode_longview_client_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_longview_client_list", query="page=2&page_size=50"
    )
    assert "123" in _text(result)


def test_longview_subscriptions_list_tool_schema() -> None:
    tool, capability = gentools_mod.create_linode_longview_subscription_list_tool()

    assert tool.name == "linode_longview_subscription_list"
    assert capability is Capability.Read
    properties = tool.input_schema["properties"]
    assert properties["page"]["type"] == "integer"
    assert properties["page_size"]["type"] == "integer"
    assert "required" not in tool.input_schema


@pytest.mark.asyncio
async def test_longview_subscriptions_list_handler_calls_client_with_pagination(
    sample_config: Any, mock_linode_client: Any
) -> None:
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": "longview-3"}],
        "page": 2,
        "pages": 3,
    }

    result = await gentools_mod.handle_linode_longview_subscription_list(
        {"page": 2, "page_size": 50}, sample_config
    )

    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_longview_subscription_list", query="page=2&page_size=50"
    )
    assert "longview-3" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_longview_subscriptions_list_handler_rejects_invalid_pagination(
    monkeypatch: pytest.MonkeyPatch, arguments: dict[str, Any], message: str
) -> None:
    result = await gentools_mod.handle_linode_longview_subscription_list(
        arguments, cast("Any", object())
    )

    assert message in _text(result)


@pytest.mark.asyncio
async def test_longview_types_list_handler_calls_client(
    sample_config: Any, mock_linode_client: Any
) -> None:
    mock_linode_client.route_raw.return_value = {
        "data": [{"id": "g6-standard-2", "label": "2GB"}]
    }

    result = await gentools_mod.handle_linode_longview_type_list({}, sample_config)

    mock_linode_client.route_raw.assert_awaited_once_with(
        "linode_longview_type_list", query=""
    )
    assert "g6-standard-2" in _text(result)


@pytest.mark.asyncio
async def test_longview_plan_get_handler_calls_client(
    sample_config: Any, mock_linode_client: Any
) -> None:
    # The handler routes the raw plan through the LongviewSubscription proto, so a
    # field the proto does not model must drop, proving proto-canonical output.
    mock_linode_client.route_raw.return_value = {
        "id": "longview-100",
        "label": "Longview Pro",
        "clients_included": 40,
        "price": {"hourly": 0.06, "monthly": 40.0},
        "not_in_proto": "dropped",
    }

    result = await gentools_mod.handle_linode_longview_plan_get({}, sample_config)

    mock_linode_client.route_raw.assert_awaited_once_with("linode_longview_plan_get")
    text = _text(result)
    assert "Longview Pro" in text
    assert "not_in_proto" not in text


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be an integer greater than or equal to 1"),
        ({"page_size": 24}, "page_size must be an integer from 25 through 500"),
        ({"page_size": 501}, "page_size must be an integer from 25 through 500"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_longview_clients_list_handler_rejects_invalid_pagination(
    monkeypatch: pytest.MonkeyPatch, arguments: dict[str, Any], message: str
) -> None:
    result = await gentools_mod.handle_linode_longview_client_list(
        arguments, cast("Any", object())
    )

    assert message in _text(result)
