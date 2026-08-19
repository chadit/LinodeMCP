"""The sentences the account write tools migrated in this wave answer.

The surviving hooks here are the only place their wording is written down.
The old stance that the two languages' sentences differ on purpose is
overtaken (full-generation ruling, 2026-08-16): the contract carries one
sentence for both languages, so every retired hook's wording converged on
the shared declaration, and the remaining divergences below last only
until their hooks retire.
"""

from __future__ import annotations

from typing import Any

import pytest

from linodemcp import toolhooks

SETTINGS_NO_FIELD = "At least one account settings field is required"
CHILD_EUUID = "A1BC2DEF-34GH-567I-J890KLMN12O34P56"


@pytest.mark.parametrize(
    ("arguments", "expected_entities", "ids_left"),
    [
        ({"linode_ids": [7, 9]}, {"linodes": [7, 9]}, False),
        # A caller-supplied entities object can name types linode_ids cannot.
        (
            {"linode_ids": [7], "entities": {"domains": [9]}},
            {"domains": [9]},
            True,
        ),
        ({"linode_ids": [7], "entities": {}}, {"linodes": [7]}, False),
        ({}, None, False),
        ({"linode_ids": "seven"}, None, True),
        ({"linode_ids": []}, None, True),
        ({"linode_ids": [0]}, None, True),
        ({"linode_ids": [True]}, None, True),
    ],
)
def test_service_transfer_create_normalize(
    arguments: dict[str, Any],
    expected_entities: dict[str, Any] | None,
    ids_left: bool,
) -> None:
    """The convenience form becomes entities, which is all the route accepts."""
    supplied_ids = "linode_ids" in arguments
    toolhooks.linode_account_service_transfer_create_normalize(arguments)

    assert arguments.get("entities") == expected_entities
    assert ("linode_ids" in arguments) is (supplied_ids and ids_left)
