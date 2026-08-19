"""The rewrites and checks the deep-residue hooks own.

Every rule these tools' contracts can carry is declared on their *Input
messages and covered by ``linodemcp.tools.constraints``; what is left here is
what a rule cannot say. A normalize hook rewrites the value the rules then read,
and a validate hook answers a question about the parsed argument rather than
about its text. The Go twin is ``deep_residue_test.go`` in
``go/internal/toolhooks``, and every sentence below is one both languages
answer.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock, patch

import pytest

from linodemcp import toolhooks
from linodemcp.gentools import handle_linode_volume_delete
from linodemcp.linode import StackScript

if TYPE_CHECKING:
    from linodemcp.config import Config

TARGET_PUBLIC = "8.8.8.8"
TARGET_PRIVATE = "10.0.0.5"
# The one sentence every unroutable address is refused with, whichever reserved
# block it falls in.
TARGET_PRIVATE_REFUSAL = "a record target cannot be a private IP address"
IMAGE_DEBIAN = "linode/debian12"
TAG_MEMBERS = "tag must be one of: issue, issuewild, iodef"
TARGET_NOT_IPV4 = "a record target must be a valid IPv4 address"


def _stackscript(label: str) -> StackScript:
    """The typed state the family's client method answers a fetch with."""
    return StackScript(
        id=123,
        username="tester",
        user_gravatar_id="",
        label=label,
        description="",
        images=[],
        deployments_total=0,
        deployments_active=0,
        is_public=False,
        mine=True,
        created="",
        updated="",
        script="",
        user_defined_fields=[],
    )


async def test_stackscript_update_preview_diffs_against_the_fetched_script(
    sample_config: Config,
) -> None:
    """The update preview reports what changes rather than what was asked for.

    The Go twin is TestLinodeStackscriptUpdatePreviewDiffsAgainstTheFetchedScript.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_stackscript.return_value = {"id": 123, "label": "before"}
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_stackscript_update_preview(
            sample_config,
            {
                "stackscript_id": 123,
                "label": "after",
                "script": "#!/bin/bash",
                "description": "new",
                "dry_run": True,
            },
            "PUT",
            "/linode/stackscripts/123",
            {"label": "after"},
        )

    preview = json.loads(result[0].text)

    assert preview["would_execute"]["body"] == {"label": "after"}
    assert preview["side_effects"] == [
        "Label is set to 'after'.",
        "The StackScript body is replaced.",
        "The StackScript description is updated.",
    ]


async def test_stackscript_update_preview_names_the_label_it_replaces(
    sample_config: Config,
) -> None:
    """A typed state carries the stored label, so the walk names both ends of
    the change rather than only the new one.
    """
    with patch("linodemcp.tools.helpers.RetryableClient") as mock_cls:
        mock_client = AsyncMock()
        mock_client.get_stackscript.return_value = _stackscript("before")
        mock_client.__aenter__.return_value = mock_client
        mock_client.__aexit__.return_value = None
        mock_cls.return_value = mock_client

        result = await toolhooks.linode_stackscript_update_preview(
            sample_config,
            {"stackscript_id": 123, "label": "after", "dry_run": True},
            "PUT",
            "/linode/stackscripts/123",
            {"label": "after"},
        )

    preview = json.loads(result[0].text)

    assert preview["side_effects"] == ["Label changes from 'before' to 'after'."]


async def test_destroy_driver_refuses_a_negative_id(sample_config: Config) -> None:
    """A below-zero path id is refused rather than spliced into the route.

    The Go twin is the destroyID guard in internal/tools/destroy.go, which the
    stackscript-delete behavior fixture pins for both languages.
    """
    result = await handle_linode_volume_delete(
        {"volume_id": -1, "confirm": True, "confirm_bypass_dry_run": True},
        sample_config,
    )

    assert "volume_id must be a positive integer" in result[0].text


PAYMENT_AMOUNT = "25.50"
PAYMENT_METHOD_POSITIVE = "payment_method_id must be a positive integer"


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({"usd": f"  {PAYMENT_AMOUNT}  "}, {"usd": PAYMENT_AMOUNT}),
        ({"usd": PAYMENT_AMOUNT}, {"usd": PAYMENT_AMOUNT}),
        ({"usd": "   "}, {"usd": ""}),
        ({"usd": 25.5}, {"usd": 25.5}),
        ({}, {}),
        ({"usd": PAYMENT_AMOUNT, "payment_method_id": None}, {"usd": PAYMENT_AMOUNT}),
        (
            {"usd": PAYMENT_AMOUNT, "payment_method_id": 7},
            {"usd": PAYMENT_AMOUNT, "payment_method_id": 7},
        ),
    ],
)
def test_account_payment_create_normalize_trims_the_amount(
    arguments: dict[str, Any], expected: dict[str, Any]
) -> None:
    """The trimmed amount is what this tool has always sent, and a null method
    is dropped because the body builder would send it as zero.

    The Go twin is TestLinodeAccountPaymentCreateNormalizeTrimsTheAmount.
    """
    toolhooks.linode_account_payment_create_normalize(arguments)

    assert arguments == expected


@pytest.mark.parametrize(
    ("restricted", "rendered"),
    [(None, "false"), (True, "true"), (False, "false")],
)
async def test_account_user_create_preview_names_the_user(
    sample_config: Config, restricted: bool | None, rendered: str
) -> None:
    """A user that does not exist yet has nothing to read first, so the preview
    carries the body it would send and the sentence naming who is added.

    The Go twin is TestLinodeAccountUserCreatePreviewNamesTheUser.
    """
    arguments: dict[str, Any] = {
        "username": "newuser",
        "email": "ops@example.com",
        "dry_run": True,
    }
    if restricted is not None:
        arguments["restricted"] = restricted

    result = await toolhooks.linode_account_user_create_preview(
        sample_config,
        arguments,
        "POST",
        "/account/users",
        {"username": "newuser", "email": "ops@example.com"},
    )

    preview = json.loads(result[0].text)

    assert preview["tool"] == "linode_account_user_create"
    assert preview["would_execute"]["body"] == {
        "username": "newuser",
        "email": "ops@example.com",
    }
    assert preview["current_state"] is None
    assert preview["side_effects"] == [
        f'A new account user "newuser" will be created with restricted={rendered}.'
    ]
