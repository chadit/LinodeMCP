"""Tests for Longview tools."""

from typing import Any, cast
from unittest.mock import AsyncMock

import pytest

from linodemcp.profiles import Capability
from linodemcp.tools import linode_longview


class _FakeClient:
    def __init__(self) -> None:
        self.get_longview_plan = AsyncMock(
            return_value={"label": "Longview Pro", "clients_included": 40}
        )
        self.list_longview_clients = AsyncMock(
            return_value={"data": [{"id": 123}], "page": 2, "pages": 3}
        )
        self.get_longview_client = AsyncMock(
            return_value={
                "id": 123,
                "label": "prod-longview",
                "api_key": "secret",
                "install_code": "install",
            }
        )
        self.list_longview_subscriptions = AsyncMock(
            return_value={"data": [{"id": "longview-3"}], "page": 2, "pages": 3}
        )
        self.list_longview_types = AsyncMock(
            return_value={"data": [{"id": "g6-standard-2", "label": "2GB"}]}
        )
        self.delete_longview_client = AsyncMock(return_value=None)


def _text(result: list[Any]) -> str:
    return str(result[0].text)


def test_longview_client_delete_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_client_delete_tool()

    assert tool.name == "linode_longview_client_delete"
    assert capability is Capability.Destroy
    properties = tool.inputSchema["properties"]
    assert properties["client_id"]["minimum"] == 1
    assert properties["confirm"]["type"] == "boolean"
    assert properties["dry_run"]["type"] == "boolean"
    assert tool.inputSchema["required"] == ["client_id", "confirm"]


@pytest.mark.asyncio
async def test_longview_client_delete_handler_calls_client(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "delete Longview client"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_client_delete(
        {"client_id": 123, "confirm": True}, cast("Any", object())
    )

    fake.delete_longview_client.assert_awaited_once_with(123)
    assert "Longview client 123 deleted" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize("confirm", [None, False, "true", 1])
async def test_longview_client_delete_handler_requires_boolean_confirm(
    monkeypatch: pytest.MonkeyPatch, confirm: object
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    arguments: dict[str, Any] = {"client_id": 123}
    if confirm is not None:
        arguments["confirm"] = confirm

    result = await linode_longview.handle_linode_longview_client_delete(
        arguments, cast("Any", object())
    )

    fake.delete_longview_client.assert_not_awaited()
    assert "Set confirm=true to proceed" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize("client_id", [None, 0, -1, True, "123", "1/2", "1?x=2", ".."])
async def test_longview_client_delete_handler_rejects_invalid_client_id(
    monkeypatch: pytest.MonkeyPatch, client_id: object
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    arguments: dict[str, Any] = {"confirm": True}
    if client_id is not None:
        arguments["client_id"] = client_id

    result = await linode_longview.handle_linode_longview_client_delete(
        arguments, cast("Any", object())
    )

    fake.delete_longview_client.assert_not_awaited()
    assert "client_id" in _text(result)


@pytest.mark.asyncio
async def test_longview_client_delete_handler_dry_run_does_not_call_client() -> None:
    fake = _FakeClient()
    result = await linode_longview.handle_linode_longview_client_delete(
        {"client_id": 123, "confirm": True, "dry_run": True}, cast("Any", object())
    )

    fake.delete_longview_client.assert_not_awaited()
    text = _text(result)
    assert "linode_longview_client_delete" in text
    assert "DELETE" in text
    assert "/longview/clients/123" in text


def test_longview_client_get_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_client_get_tool()

    assert tool.name == "linode_longview_client_get"
    assert capability is Capability.Read
    assert tool.inputSchema["required"] == ["client_id"]
    assert tool.inputSchema["properties"]["client_id"]["minimum"] == 1


@pytest.mark.asyncio
async def test_longview_client_get_handler_sanitizes_sensitive_fields(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "retrieve Longview client"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_client_get(
        {"client_id": 123}, cast("Any", object())
    )

    fake.get_longview_client.assert_awaited_once_with(123)
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
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    arguments: dict[str, Any] = {}
    if client_id is not None:
        arguments["client_id"] = client_id

    result = await linode_longview.handle_linode_longview_client_get(
        arguments, cast("Any", object())
    )

    assert "client_id" in _text(result)


def test_longview_plan_get_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_plan_get_tool()

    assert tool.name == "linode_longview_plan_get"
    assert capability is Capability.Read
    assert "environment" in tool.inputSchema["properties"]
    assert "required" not in tool.inputSchema


def test_longview_types_list_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_type_list_tool()

    assert tool.name == "linode_longview_type_list"
    assert capability is Capability.Read
    assert "environment" in tool.inputSchema["properties"]
    assert "required" not in tool.inputSchema


def test_longview_clients_list_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_client_list_tool()

    assert tool.name == "linode_longview_client_list"
    assert capability is Capability.Read
    properties = tool.inputSchema["properties"]
    assert properties["page"]["minimum"] == 1
    assert properties["page_size"]["minimum"] == 25
    assert properties["page_size"]["maximum"] == 500
    assert "required" not in tool.inputSchema


@pytest.mark.asyncio
async def test_longview_clients_list_handler_calls_client_with_pagination(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "list Longview clients"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_client_list(
        {"page": 2, "page_size": 50}, cast("Any", object())
    )

    fake.list_longview_clients.assert_awaited_once_with(page=2, page_size=50)
    assert "123" in _text(result)


def test_longview_subscriptions_list_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_subscription_list_tool()

    assert tool.name == "linode_longview_subscription_list"
    assert capability is Capability.Read
    properties = tool.inputSchema["properties"]
    assert properties["page"]["minimum"] == 1
    assert properties["page_size"]["minimum"] == 25
    assert properties["page_size"]["maximum"] == 500
    assert "required" not in tool.inputSchema


@pytest.mark.asyncio
async def test_longview_subscriptions_list_handler_calls_client_with_pagination(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "list Longview subscriptions"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_subscription_list(
        {"page": 2, "page_size": 50}, cast("Any", object())
    )

    fake.list_longview_subscriptions.assert_awaited_once_with(page=2, page_size=50)
    assert "longview-3" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_longview_subscriptions_list_handler_rejects_invalid_pagination(
    monkeypatch: pytest.MonkeyPatch, arguments: dict[str, Any], message: str
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_subscription_list(
        arguments, cast("Any", object())
    )

    assert message in _text(result)


@pytest.mark.asyncio
async def test_longview_types_list_handler_calls_client(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "list Longview types"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_type_list(
        {}, cast("Any", object())
    )

    fake.list_longview_types.assert_awaited_once_with()
    assert "g6-standard-2" in _text(result)


@pytest.mark.asyncio
async def test_longview_plan_get_handler_calls_client(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeClient()

    async def fake_execute_tool(
        cfg: object, arguments: dict[str, Any], action: str, call: Any
    ) -> list[Any]:
        assert action == "get Longview plan"
        payload = await call(fake)
        return [type("Text", (), {"text": str(payload)})()]

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_plan_get(
        {}, cast("Any", object())
    )

    fake.get_longview_plan.assert_awaited_once_with()
    assert "Longview Pro" in _text(result)


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("arguments", "message"),
    [
        ({"page": 0}, "page must be at least 1"),
        ({"page_size": 24}, "page_size must be at least 25"),
        ({"page_size": 501}, "page_size must be at most 500"),
        ({"page": "2"}, "page must be an integer"),
        ({"page_size": True}, "page_size must be an integer"),
    ],
)
async def test_longview_clients_list_handler_rejects_invalid_pagination(
    monkeypatch: pytest.MonkeyPatch, arguments: dict[str, Any], message: str
) -> None:
    async def fake_execute_tool(*args: Any, **kwargs: Any) -> list[Any]:
        raise AssertionError("execute_tool should not be called")

    monkeypatch.setattr(linode_longview, "execute_tool", fake_execute_tool)
    result = await linode_longview.handle_linode_longview_client_list(
        arguments, cast("Any", object())
    )

    assert message in _text(result)
