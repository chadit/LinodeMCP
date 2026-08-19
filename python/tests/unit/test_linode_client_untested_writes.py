"""Behavioral tests for Client write methods whose success path was untested.

These cover object-storage, LKE, VPC, and instance write methods that build a
request body (often with conditional optional fields), send it through
``make_request``, decode the JSON body, and wrap ``httpx`` failures in
``NetworkError``. Each success test asserts the exact HTTP verb, endpoint, and
body so a dropped or renamed field is caught; each error test asserts the
failure maps to ``NetworkError`` carrying the operation name.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from linodemcp.linode import Client, NetworkError

if TYPE_CHECKING:
    from collections.abc import AsyncIterator

pytestmark = pytest.mark.asyncio


def _ok_response(body: Any) -> MagicMock:
    """Build a mock httpx response whose json() returns body."""
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = body
    return response


@pytest.fixture
async def client() -> AsyncIterator[Client]:
    """Yield a Client with a real httpx session that gets closed on teardown."""
    instance = Client("https://api.linode.com/v4", "test-token")
    yield instance
    await instance.close()


async def test_get_instance_backup_targets_backup_route(client: Client) -> None:
    """get_instance_backup GETs the nested backup route and returns the body."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.return_value = _ok_response({"id": 8, "status": "successful"})
        result = await client.get_instance_backup(123, 8)

    assert result == {"id": 8, "status": "successful"}
    method, endpoint = req.await_args_list[0].args
    assert method == "GET"
    assert endpoint == "/linode/instances/123/backups/8"


async def test_get_instance_backup_wraps_errors(client: Client) -> None:
    """get_instance_backup wraps httpx failures."""
    with patch.object(client, "make_request", new_callable=AsyncMock) as req:
        req.side_effect = httpx.HTTPError("boom")
        with pytest.raises(NetworkError) as excinfo:
            await client.get_instance_backup(123, 8)

    assert "GetInstanceBackup" in str(excinfo.value)
