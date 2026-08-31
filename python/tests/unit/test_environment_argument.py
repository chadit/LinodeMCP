"""The environment argument's refusal arms, held to Go's wording.

A value that is not text names no environment. Reading past it and preparing a
client for the default environment ran the call against whatever account that
environment's token belongs to, which is the divergence these tests close: Go's
prepareClient dropped the value silently and this language refused it under a
sentence Go never said.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp.tools import helpers

if TYPE_CHECKING:
    from linodemcp.config import Config


def _cm_client() -> AsyncMock:
    """Build an async-context-manager client mock for RetryableClient."""
    client = AsyncMock()
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


@pytest.mark.parametrize(
    ("value", "spelled"),
    [
        (5, "5"),
        (True, "true"),
        (["prod", "staging"], '["prod","staging"]'),
        ({"name": "prod"}, '{"name":"prod"}'),
    ],
)
async def test_refuses_an_environment_that_is_not_a_name(
    sample_config: Config, value: object, spelled: str
) -> None:
    """A number, a flag, a list, or an object is refused, spelled as JSON.

    The JSON spelling is what both languages report: an f-string wrote a flag as
    ``True`` where Go writes ``true``, so the same call read back two sentences.
    """

    async def _callback(_client: object) -> list[dict[str, Any]]:
        pytest.fail("the callback ran even though no client could be prepared")

    result = await helpers.execute_tool_list(
        sample_config, {"environment": value}, "list things", _callback
    )

    want = f"Error: environment not found in configuration: {spelled}"
    assert result[0].text == want


async def test_refuses_an_environment_the_config_does_not_name(
    sample_config: Config,
) -> None:
    """An unknown name reads back the sentence the wrong-typed values share."""

    async def _callback(_client: object) -> list[dict[str, Any]]:
        pytest.fail("the callback ran even though no client could be prepared")

    result = await helpers.execute_tool_list(
        sample_config, {"environment": "staging"}, "list things", _callback
    )

    assert result[0].text == "Error: environment not found in configuration: staging"


@pytest.mark.parametrize(
    "arguments",
    [{}, {"environment": None}, {"environment": ""}],
    ids=["absent", "explicit null", "empty string"],
)
async def test_reads_an_unnamed_environment_as_the_default(
    sample_config: Config, arguments: dict[str, Any]
) -> None:
    """The argument left out, an explicit null, and a blank all select the default.

    None of the three names an environment, so none may take the refusal arm the
    wrong-typed values take.
    """

    async def _callback(_client: object) -> list[dict[str, Any]]:
        return [{"id": 1}]

    with patch("linodemcp.tools.helpers.RetryableClient", return_value=_cm_client()):
        result = await helpers.execute_tool_list(
            sample_config, arguments, "list things", _callback
        )

    assert "environment not found" not in result[0].text
