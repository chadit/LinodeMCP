"""Tests for Longview tools."""

from typing import Any, cast
from unittest.mock import AsyncMock

import pytest

from linodemcp.profiles import Capability
from linodemcp.tools import linode_longview


class _FakeClient:
    def __init__(self) -> None:
        self.list_longview_clients = AsyncMock(
            return_value={"data": [{"id": 123}], "page": 2, "pages": 3}
        )


def _text(result: list[Any]) -> str:
    return str(result[0].text)


def test_longview_clients_list_tool_schema() -> None:
    tool, capability = linode_longview.create_linode_longview_clients_list_tool()

    assert tool.name == "linode_longview_clients_list"
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
    result = await linode_longview.handle_linode_longview_clients_list(
        {"page": 2, "page_size": 50}, cast("Any", object())
    )

    fake.list_longview_clients.assert_awaited_once_with(page=2, page_size=50)
    assert "123" in _text(result)


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
    result = await linode_longview.handle_linode_longview_clients_list(
        arguments, cast("Any", object())
    )

    assert message in _text(result)
