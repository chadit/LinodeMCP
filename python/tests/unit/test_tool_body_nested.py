"""Tests for the recursive typed message reader in the write body builder.

Every case here has a twin in go/internal/tools/gentools_body_nested_test.go,
because the point of reading a nested body this way is that both languages
accept the same calls and refuse the rest with the same sentence.
"""

from __future__ import annotations

import json
from typing import Any

import pytest

from linodemcp.tools.body import (
    ITEM_BOOL,
    ITEM_INT,
    ITEM_MESSAGE,
    ITEM_MESSAGE_LIST,
    ITEM_STR,
    ItemField,
    WriteBody,
)

# The shape the recursive reader exists for: a named message whose members carry
# named messages of their own, two deep, which is what a Linode interface's vpc
# arm declares.
VPC_FIELDS = (
    ItemField("subnet_id", ITEM_INT, required=True),
    ItemField(
        "ipv4",
        ITEM_MESSAGE,
        fields=(
            ItemField(
                "addresses",
                ITEM_MESSAGE_LIST,
                fields=(
                    ItemField("address", ITEM_STR, required=True),
                    ItemField("primary", ITEM_BOOL),
                ),
            ),
            ItemField(
                "ranges",
                ITEM_MESSAGE_LIST,
                fields=(ItemField("range", ITEM_STR, required=True),),
            ),
        ),
    ),
)

# One item message carrying a singular nested message, for the repeated field's
# half of the same reader.
INTERFACE_ITEMS = (
    ItemField("label", ITEM_STR, required=True),
    ItemField(
        "ipv4",
        ITEM_MESSAGE,
        fields=(ItemField("address", ITEM_STR, required=True),),
    ),
)

ADDRESS_ONE = "10.0.0.4"
ADDRESS_TWO = "10.0.0.5"
RANGE_ONE = "10.0.0.0/24"


def _rendered(body: WriteBody) -> str:
    """The body as the request layer will send it, key order included."""
    fields, message = body.result()
    assert message is None, f"unexpected validation failure: {message}"
    return json.dumps(fields, separators=(",", ":"))


def _refusal(body: WriteBody) -> str | None:
    """The sentence a refused body answers with, and nothing written."""
    fields, message = body.result()
    assert fields == {}, f"a refused body wrote {fields}"
    return None if message is None else str(message)


def _vpc_argument() -> tuple[dict[str, Any], dict[str, Any]]:
    """One well-formed vpc argument and the nested member cases reach into.

    It is built fresh per case, so a case that rewrites a member cannot reach
    the one after it.
    """
    ipv4: dict[str, Any] = {
        "addresses": [
            {"address": ADDRESS_ONE, "primary": True},
            {"address": ADDRESS_TWO},
        ],
        "ranges": [{"range": RANGE_ONE}],
    }
    return {"subnet_id": 456, "ipv4": ipv4}, ipv4


def test_set_message_writes_every_depth_in_declaration_order() -> None:
    """Declaration order is wire order at every depth.

    A caller who scrambles a nested member still reaches the same bytes Go
    does.
    """
    scrambled = {
        "ipv4": {
            "ranges": [{"range": RANGE_ONE}],
            "addresses": [{"primary": True, "address": ADDRESS_ONE}],
        },
        "subnet_id": 456,
    }
    body = WriteBody({"vpc": scrambled})
    body.set_message("vpc", VPC_FIELDS)

    assert _rendered(body) == (
        '{"vpc":{"subnet_id":456,"ipv4":{"addresses":[{"address":"10.0.0.4"'
        ',"primary":true}],"ranges":[{"range":"10.0.0.0/24"}]}}}'
    )


def test_set_message_omits_absent_nested_members() -> None:
    """An optional member nobody sent is not a request to set it to its zero."""
    body = WriteBody({"vpc": {"subnet_id": 456}})
    body.set_message("vpc", VPC_FIELDS)

    assert _rendered(body) == '{"vpc":{"subnet_id":456}}'


def test_set_message_omits_what_the_caller_did_not_supply() -> None:
    """A message field carries no proto3 optional, so absence is read here."""
    body = WriteBody({})
    body.set_message("vpc", VPC_FIELDS)

    assert _rendered(body) == "{}"


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        ({}, "{}"),
        ({"vpc": None}, '{"vpc":null}'),
        ({"vpc": {"subnet_id": 456}}, '{"vpc":{"subnet_id":456}}'),
    ],
    ids=["omitted", "explicit null", "supplied message"],
)
def test_set_message_or_null_keeps_the_three_states_apart(
    arguments: dict[str, Any], expected: str
) -> None:
    """An explicit null is the value the API acts on.

    Collapsing it into absence would report a detach nothing performed.
    """
    body = WriteBody(arguments)
    body.set_message_or_null("vpc", VPC_FIELDS)

    assert _rendered(body) == expected


def test_set_message_or_null_refuses_a_non_object() -> None:
    """The nullable setter words its one sentence differently from the plain one."""
    body = WriteBody({"vpc": "detach"})
    body.set_message_or_null("vpc", VPC_FIELDS)

    assert _refusal(body) == "vpc must be an object or null"


def _not_an_object(_argument: dict[str, Any], _ipv4: dict[str, Any]) -> Any:
    """A vpc argument that is not an object at all."""
    return "detach"


def _drop_required(argument: dict[str, Any], _ipv4: dict[str, Any]) -> Any:
    """A vpc argument missing the member its message requires."""
    del argument["subnet_id"]
    return argument


def _nested_not_an_object(argument: dict[str, Any], _ipv4: dict[str, Any]) -> Any:
    """A vpc argument whose nested message arrived as an array."""
    argument["ipv4"] = []
    return argument


def _nested_list_not_an_array(argument: dict[str, Any], ipv4: dict[str, Any]) -> Any:
    """A vpc argument whose nested list arrived as an object."""
    ipv4["addresses"] = {}
    return argument


def _entry_missing_required(argument: dict[str, Any], ipv4: dict[str, Any]) -> Any:
    """A nested entry missing the member its item message requires."""
    ipv4["addresses"] = [{"address": ADDRESS_ONE}, {"primary": True}]
    return argument


def _entry_wrong_kind(argument: dict[str, Any], ipv4: dict[str, Any]) -> Any:
    """A nested entry whose flag arrived as text."""
    ipv4["addresses"] = [{"address": ADDRESS_ONE, "primary": "yes"}]
    return argument


def _nested_unknown_member(argument: dict[str, Any], ipv4: dict[str, Any]) -> Any:
    """A nested message carrying a member it does not declare."""
    ipv4["extra"] = 1
    return argument


def _entry_unknown_member(argument: dict[str, Any], ipv4: dict[str, Any]) -> Any:
    """A nested entry carrying a member it does not declare."""
    ipv4["addresses"] = [{"address": ADDRESS_ONE, "extra": 1}]
    return argument


@pytest.mark.parametrize(
    ("build", "expected"),
    [
        (_not_an_object, "vpc must be an object"),
        (_drop_required, "vpc.subnet_id is required"),
        (_nested_not_an_object, "vpc.ipv4 must be an object"),
        (_nested_list_not_an_array, "vpc.ipv4.addresses must be an array of objects"),
        (_entry_missing_required, "vpc.ipv4.addresses[1].address is required"),
        (_entry_wrong_kind, "vpc.ipv4.addresses[0].primary must be a boolean"),
        (_nested_unknown_member, 'vpc.ipv4 has no field "extra"'),
        (_entry_unknown_member, 'vpc.ipv4.addresses[0] has no field "extra"'),
    ],
    ids=[
        "field is not an object",
        "required member missing at depth zero",
        "nested message is not an object",
        "nested list is not an array",
        "required member missing inside a nested entry",
        "wrong kind inside a nested entry",
        "undeclared member on a nested message",
        "undeclared member on a nested entry",
    ],
)
def test_set_message_refusals_name_the_path_they_failed_at(
    build: Any, expected: str
) -> None:
    """A nested failure addresses the member by the path the reader took."""
    argument = build(*_vpc_argument())
    body = WriteBody({"vpc": argument})
    body.set_message("vpc", VPC_FIELDS)

    assert _refusal(body) == expected


def test_set_message_list_writes_nested_items_in_order() -> None:
    """An item member carrying a message is held to the same shape a field is."""
    body = WriteBody(
        {"interfaces": [{"ipv4": {"address": ADDRESS_ONE}, "label": "first"}]}
    )
    body.set_message_list("interfaces", INTERFACE_ITEMS)

    assert _rendered(body) == (
        '{"interfaces":[{"label":"first","ipv4":{"address":"10.0.0.4"}}]}'
    )


def test_set_message_list_names_the_entry_a_nested_failure_sits_in() -> None:
    """The index travels into the nested path, so the caller knows which entry."""
    body = WriteBody(
        {"interfaces": [{"label": "first"}, {"label": "second", "ipv4": {}}]}
    )
    body.set_message_list("interfaces", INTERFACE_ITEMS)

    assert _refusal(body) == "interfaces[1].ipv4.address is required"


def test_set_message_list_refuses_a_non_array() -> None:
    """A typed list is held to the outer shape the free-form object list is."""
    body = WriteBody({"interfaces": {}})
    body.set_message_list("interfaces", (ItemField("label", ITEM_STR),))

    assert _refusal(body) == "interfaces must be an array of objects"


@pytest.mark.parametrize(
    ("value", "expected", "failure"),
    [
        (False, '{"interfaces":[{"primary":false}]}', None),
        ("false", "{}", "interfaces[0].primary must be a boolean"),
    ],
    ids=["boolean", "string"],
)
def test_item_bool_reads_only_a_boolean(
    value: Any, expected: str, failure: str | None
) -> None:
    """A string that reads like a flag is not one."""
    body = WriteBody({"interfaces": [{"primary": value}]})
    body.set_message_list("interfaces", (ItemField("primary", ITEM_BOOL),))

    fields, message = body.result()
    assert message == failure
    assert json.dumps(fields, separators=(",", ":")) == expected
