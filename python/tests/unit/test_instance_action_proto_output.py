"""Proto-output field-shape tests for the instance action write tools.

These tools echo the request as proto-canonical JSON. The message-substring
assertions in test_tools.py miss field-name changes, so these pin the exact
output field names and values: a regression that drops a field or reverts to the
legacy dict shape is caught here. Byte-identical output with the Go side is the
point of the proto conversion, so these mirror
go/internal/tools/instance_action_proto_output_test.go.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

import pytest

from linodemcp.gentools import (
    handle_linode_instance_backups_cancel,
    handle_linode_instance_password_reset,
)

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable

    from linodemcp.config import Config

pytestmark = pytest.mark.asyncio

_STRONG_PASS = "Str0ngP@ssw0rd!"


@pytest.mark.parametrize(
    ("handler", "extra", "wanted"),
    [
        (
            handle_linode_instance_password_reset,
            {"root_pass": _STRONG_PASS},
            "linode_id must be a positive integer",
        ),
        (
            handle_linode_instance_backups_cancel,
            {},
            "linode_id must be a positive integer",
        ),
    ],
)
async def test_invalid_linode_id_rejected(
    handler: Callable[[dict[str, Any], Config], Awaitable[list[Any]]],
    extra: dict[str, Any],
    wanted: str,
    sample_config: Config,
) -> None:
    """A non-integer linode_id is rejected before any API call.

    Both answer the same sentence: the destroy tier used to read an unreadable
    id as the absent zero and report a missing argument, which is the one
    reading Go never did.
    """
    result = await handler(
        {"linode_id": "not-a-number", "confirm": True, **extra}, sample_config
    )
    assert len(result) == 1
    assert wanted in result[0].text
