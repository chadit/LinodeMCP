"""Tests for the generated write body builder.

The assertions here are the Python half of a cross-language contract: every
case has a twin in go/internal/tools/gentools_body_test.go, because the point
of building a body this way is that both languages send the same bytes.
"""

from __future__ import annotations

import json
from typing import Any

import pytest

from linodemcp.tools.body import (
    ITEM_BOOL,
    ITEM_INT,
    ITEM_OBJECT,
    ITEM_STR,
    ITEM_STR_LIST,
    FoldMember,
    ItemField,
    WriteBody,
)

# The item shape linode_image_sharegroup_create declares: a required id and two
# optional overrides, in proto declaration order.
IMAGE_ITEMS = (
    ItemField("id", ITEM_STR, required=True),
    ItemField("label", ITEM_STR),
    ItemField("description", ITEM_STR),
)

# An item carrying an integer member, which no image item declares, so the
# integer branch has a shape of its own to be read through.
NUMBERED_ITEMS = (
    ItemField("linode_id", ITEM_INT, required=True),
    ItemField("label", ITEM_STR),
)

IMAGES_MUST_BE_OBJECTS = "images must be an array of objects"


def _rendered(body: WriteBody) -> str:
    """The body as the request layer will send it, key order included."""
    fields, message = body.result()
    assert message is None, f"unexpected validation failure: {message}"
    return json.dumps(fields, separators=(",", ":"))


def test_keeps_declaration_order() -> None:
    """A body reads in proto field order, not in the order arguments arrived.

    The rule reaches inside a typed list item too, which is its own mapping in
    both languages.
    """
    body = WriteBody(
        {
            "ttl_sec": 300,
            "soa_email": "admin@example.com",
            "type": "master",
            "domain": "example.com",
            "images": [{"description": "third", "label": "second", "id": "private/15"}],
        }
    )
    body.put_str("domain")
    body.put_str("type")
    body.set_str("soa_email")
    body.set_int("ttl_sec")
    body.set_message_list("images", IMAGE_ITEMS)

    assert _rendered(body) == (
        '{"domain":"example.com","type":"master"'
        ',"soa_email":"admin@example.com","ttl_sec":300'
        ',"images":[{"id":"private/15","label":"second","description":"third"}]}'
    )


def test_copies_by_presence() -> None:
    """An explicit empty value reaches the wire.

    ``""``, ``0``, and ``[]`` are values the Linode API acts on, so dropping
    them would turn "clear this field" into "leave it alone".
    """
    body = WriteBody({"description": "", "ttl_sec": 0, "tags": []})
    body.set_str("description")
    body.set_int("ttl_sec")
    body.set_str_list("tags")

    assert _rendered(body) == '{"description":"","ttl_sec":0,"tags":[]}'


def test_omits_what_the_caller_did_not_supply() -> None:
    """An absent optional field is not a request to set it to its zero."""
    body = WriteBody({"description": "set"})
    body.set_str("description")
    body.set_str("soa_email")
    body.set_int("ttl_sec")
    body.set_str_list("tags")

    assert _rendered(body) == '{"description":"set"}'


def test_puts_a_required_field_the_caller_omitted() -> None:
    """A proto field without ``optional`` travels on every call."""
    body = WriteBody({})
    body.put_str("domain")
    body.put_int("count")

    assert _rendered(body) == '{"domain":"","count":0}'


def test_accepts_an_integral_float() -> None:
    """1.0 reaches the wire as 1, which is what the API accepts."""
    body = WriteBody({"retry_sec": 1.0})
    body.set_int("retry_sec")

    assert _rendered(body) == '{"retry_sec":1}'


def test_rejects_a_boolean_as_an_integer() -> None:
    """``bool`` subclasses ``int`` in Python and does not in Go.

    Without the type check the two languages would disagree about whether
    ``true`` is an acceptable interval.
    """
    body = WriteBody({"ttl_sec": True})
    body.set_int("ttl_sec")

    assert body.result() == ({}, "ttl_sec must be an integer")


def test_reports_the_first_type_failure() -> None:
    """The message names the first problem, and nothing after it is read."""
    body = WriteBody({"description": "kept", "retry_sec": 1.5, "tags": "not-an-array"})
    body.set_str("description")
    body.set_int("retry_sec")
    body.set_str_list("tags")

    assert body.result() == ({}, "retry_sec must be an integer")


@pytest.mark.parametrize(
    ("name", "value", "setter", "expected"),
    [
        ("status", 1, "set_str", "status must be a string"),
        ("ttl_sec", "300", "set_int", "ttl_sec must be an integer"),
        (
            "master_ips",
            '["192.0.2.20"]',
            "set_str_list",
            "master_ips must be an array of strings",
        ),
        (
            "tags",
            ["prod", 5],
            "set_str_list",
            "tags must be an array of strings",
        ),
    ],
)
def test_reports_the_type_it_declares(
    name: str, value: Any, setter: str, expected: str
) -> None:
    """Each kind answers with the wording the hand-written builders used."""
    body = WriteBody({name: value})
    getattr(body, setter)(name)

    assert body.result() == ({}, expected)


def test_copies_a_boolean_by_presence() -> None:
    """An absent flag and one the caller turned off are different bodies."""
    supplied = WriteBody({"is_public": False})
    supplied.set_bool("is_public")
    assert _rendered(supplied) == '{"is_public":false}'

    absent = WriteBody({})
    absent.set_bool("is_public")
    assert _rendered(absent) == "{}"


def test_puts_a_boolean_the_caller_omitted() -> None:
    """A bool without ``optional`` travels, so an omitted one reads as false."""
    body = WriteBody({"public": True})
    body.put_bool("public")
    body.put_bool("restricted")

    assert _rendered(body) == '{"public":true,"restricted":false}'


@pytest.mark.parametrize(
    ("write", "arguments", "expected"),
    [
        ("put_bool", {"public": "yes"}, "public must be a boolean"),
        ("put_str", {"label": 7}, "label must be a string"),
        ("put_int", {"size": "big"}, "size must be an integer"),
    ],
)
def test_refuses_a_wrongly_typed_field_that_always_travels(
    write: str, arguments: dict[str, object], expected: str
) -> None:
    """No rule can speak for a field whose value the message cannot hold, so
    the body builder is the only place left to report it."""
    body = WriteBody(arguments)
    getattr(body, write)(next(iter(arguments)))

    assert body.result() == ({}, expected)


def test_keeps_the_first_failure_across_fields_that_always_travel() -> None:
    """The message a caller reads should be the first thing wrong with their
    call, so a later field neither overwrites it nor writes itself."""
    body = WriteBody({"label": 7, "size": "big"})
    body.put_str("label")
    body.put_int("size")

    assert body.result() == ({}, "label must be a string")


def test_puts_an_explicit_null_integer_as_zero() -> None:
    """A null is the shape an absent value arrives in from a client that sends
    every declared field, so it reads as the zero rather than as a wrong type."""
    body = WriteBody({"size": None})
    body.put_int("size")

    assert body.result() == ({"size": 0}, None)


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        ([1, 2], '{"linodes":[1,2]}'),
        ([], '{"linodes":[]}'),
        ([1.0], '{"linodes":[1]}'),
    ],
)
def test_reads_an_integer_list(value: Any, expected: str) -> None:
    """The shapes an id list arrives as all reach the same body."""
    body = WriteBody({"linodes": value})
    body.set_int_list("linodes")

    assert _rendered(body) == expected


@pytest.mark.parametrize(
    "value",
    ["1,2", ["1"], [1.5], [None], [True]],
)
def test_refuses_an_id_that_is_not_an_integer(value: Any) -> None:
    """A null entry is refused where a null scalar reads as zero."""
    body = WriteBody({"linodes": value})
    body.set_int_list("linodes")

    assert body.result() == ({}, "linodes must be an array of integers")


def test_reports_a_boolean_type_failure() -> None:
    """A flag of another type answers with the wording the hand code used."""
    body = WriteBody({"restricted": "true"})
    body.set_bool("restricted")

    assert body.result() == ({}, "restricted must be a boolean")


def test_empty_body_is_an_object() -> None:
    """A mutation with nothing set still sends a body."""
    assert _rendered(WriteBody({})) == "{}"


def test_set_object_writes_a_free_form_object() -> None:
    """Presence decides the member, so a supplied object travels as it arrived."""
    body = WriteBody({"data": {"cvv": "737", "card_number": "4111"}})
    body.set_object("data")

    assert _rendered(body) == '{"data":{"card_number":"4111","cvv":"737"}}'


def test_set_object_sorts_members_the_way_go_marshals_them() -> None:
    """Go sorts a map's keys at every level and cannot be told otherwise."""
    body = WriteBody({"data": {"b": {"z": 1, "a": 2}, "a": [{"y": 1, "x": 2}]}})
    body.set_object("data")

    assert _rendered(body) == ('{"data":{"a":[{"x":2,"y":1}],"b":{"a":2,"z":1}}}')


def test_set_object_omits_the_member_the_caller_did_not_supply() -> None:
    """An omitted map leaves the key off the wire rather than sending {}.

    Writing an empty object would tell the API the caller sent one, which the
    hand-written handlers this replaces never did.
    """
    body = WriteBody({})
    body.set_object("data")

    assert _rendered(body) == "{}"


def test_set_object_writes_an_explicitly_empty_object() -> None:
    """Presence is the argument's, not the value's: {} is a value."""
    body = WriteBody({"data": {}})
    body.set_object("data")

    assert _rendered(body) == '{"data":{}}'


@pytest.mark.parametrize("raw", ["credit card", 5, ["a"], True, None])
def test_set_object_reports_a_non_object(raw: Any) -> None:
    """The sentence names the field, which the caller cannot otherwise see."""
    body = WriteBody({"data": raw})
    body.set_object("data")

    assert body.result() == ({}, "data must be an object")


def test_set_object_root_hoists_the_members() -> None:
    """The argument's own keys are the body, sorted, with no member around them."""
    body = WriteBody({"data": {"theme": "dark", "collapsed": True}})
    body.set_object_root("data")

    assert _rendered(body) == '{"collapsed":true,"theme":"dark"}'


@pytest.mark.parametrize("raw", [{}, "dark", 5, ["a"], True, None])
def test_set_object_root_reports_an_empty_or_wrong_argument(raw: Any) -> None:
    """The members are the body, so an empty object is an empty request."""
    body = WriteBody({"data": raw})
    body.set_object_root("data")

    assert body.result() == ({}, "data must be a non-empty object")


def test_set_object_root_keeps_the_first_failure() -> None:
    """The reported message is the first thing wrong with the call.

    A hoist after one is skipped rather than overwriting it, which is the rule
    every other setter follows.
    """
    body = WriteBody({"label": 7, "data": {"theme": "dark"}})
    body.set_str("label")
    body.set_object_root("data")

    assert body.result() == ({}, "label must be a string")


def test_set_object_root_reports_an_omitted_argument() -> None:
    """A root body cannot be left off the way a member can."""
    body = WriteBody({})
    body.set_object_root("data")

    assert body.result() == ({}, "data must be a non-empty object")


def test_set_object_list_writes_each_entry() -> None:
    """A grant section keeps its order and every member."""
    body = WriteBody(
        {
            "linode": [
                {"permissions": "read_only", "id": 1},
                {"id": 2, "permissions": "read_write"},
            ]
        }
    )
    body.set_object_list("linode")

    assert _rendered(body) == (
        '{"linode":[{"id":1,"permissions":"read_only"},'
        '{"id":2,"permissions":"read_write"}]}'
    )


def test_set_object_list_writes_an_empty_list_the_caller_supplied() -> None:
    """An empty list clears the section rather than leaving it alone."""
    body = WriteBody({"linode": []})
    body.set_object_list("linode")

    assert _rendered(body) == '{"linode":[]}'


def test_set_object_list_omits_the_section_the_caller_did_not_supply() -> None:
    """A section nobody sent is not one the API should be told to clear."""
    body = WriteBody({})
    body.set_object_list("linode")

    assert _rendered(body) == "{}"


@pytest.mark.parametrize(
    "raw",
    [
        "read_only",
        5,
        True,
        None,
        {"id": 1},
        [{"id": 1}, "read_only"],
        [None],
    ],
)
def test_set_object_list_reports_an_entry_that_is_not_an_object(raw: Any) -> None:
    """One unreadable entry refuses the whole section."""
    body = WriteBody({"linode": raw})
    body.set_object_list("linode")

    assert body.result() == ({}, "linode must be an array of objects")


def test_rename_writes_the_declared_wire_key() -> None:
    """The argument keeps the tool's name and the body carries the API's."""
    body = WriteBody({"new_username": "chad", "email": "a@b.c"})
    body.rename("new_username", "username")
    body.set_str("new_username")
    body.set_str("email")

    assert _rendered(body) == '{"username":"chad","email":"a@b.c"}'


def test_rename_leaves_an_omitted_argument_off() -> None:
    """A rename says what a supplied argument is called, not that it travels."""
    body = WriteBody({})
    body.rename("new_username", "username")
    body.set_str("new_username")

    assert _rendered(body) == "{}"


def test_rename_carries_a_field_that_always_travels() -> None:
    """The wire key holds for a put too, which writes on every call."""
    body = WriteBody({})
    body.rename("new_username", "username")
    body.put_str("new_username")

    assert _rendered(body) == '{"username":""}'


def test_rename_leaves_every_other_field_alone() -> None:
    """A field with no rename keeps its own name."""
    body = WriteBody({"label": "web", "new_username": "chad"})
    body.rename("new_username", "username")
    body.set_str("label")
    body.set_str("new_username")

    assert _rendered(body) == '{"label":"web","username":"chad"}'


def test_set_object_or_null_omits_an_absent_member() -> None:
    """The first of the three states: no argument means no key."""
    body = WriteBody({})
    body.set_object_or_null("private_network")

    assert _rendered(body) == "{}"


def test_set_object_or_null_sends_an_explicit_null() -> None:
    """The state set_object cannot express.

    The API reads it as a detach, so collapsing it into absence would report a
    change nothing performed.
    """
    body = WriteBody({"private_network": None})
    body.set_object_or_null("private_network")

    assert _rendered(body) == '{"private_network":null}'


def test_set_object_or_null_sends_a_supplied_object() -> None:
    """The third state travels exactly as the plain object setter sends it."""
    body = WriteBody({"private_network": {"vpc_id": 7, "subnet_id": 3}})
    body.set_object_or_null("private_network")

    assert _rendered(body) == '{"private_network":{"subnet_id":3,"vpc_id":7}}'


def test_set_object_or_null_sends_an_empty_object() -> None:
    """An empty object is a value the caller supplied, not a detach."""
    body = WriteBody({"private_network": {}})
    body.set_object_or_null("private_network")

    assert _rendered(body) == '{"private_network":{}}'


def test_set_object_or_null_sorts_members_the_way_go_marshals_them() -> None:
    """Go sorts a map's keys at every level and cannot be told otherwise."""
    body = WriteBody({"private_network": {"b": {"z": 1, "a": 2}, "a": 1}})
    body.set_object_or_null("private_network")

    assert _rendered(body) == '{"private_network":{"a":1,"b":{"a":2,"z":1}}}'


@pytest.mark.parametrize("raw", ["detach", 5, ["a"], True])
def test_set_object_or_null_reports_a_non_object(raw: Any) -> None:
    """The sentence names null as an accepted value, which set_object's does not."""
    body = WriteBody({"private_network": raw})
    body.set_object_or_null("private_network")

    _, message = body.result()
    assert message == "private_network must be an object or null"


def test_set_object_or_null_keeps_the_first_failure() -> None:
    """A later field does not overwrite the report, as every setter behaves."""
    body = WriteBody({"private_network": "detach", "data": 5})
    body.set_object_or_null("private_network")
    body.set_object("data")

    _, message = body.result()
    assert message == "private_network must be an object or null"


@pytest.mark.parametrize(
    ("arguments", "expected"),
    [
        (
            {"images": [{"description": "third", "label": "second", "id": "p/15"}]},
            '{"images":[{"id":"p/15","label":"second","description":"third"}]}',
        ),
        ({"images": [{"id": "p/15"}]}, '{"images":[{"id":"p/15"}]}'),
        (
            {"images": [{"id": "p/15", "label": "first"}, {"id": "p/16"}]},
            '{"images":[{"id":"p/15","label":"first"},{"id":"p/16"}]}',
        ),
        ({"images": []}, '{"images":[]}'),
        ({}, "{}"),
    ],
)
def test_set_message_list_writes_each_item_in_declaration_order(
    arguments: dict[str, Any], expected: str
) -> None:
    """An item's members reach the wire in the order its message declares them."""
    body = WriteBody(arguments)
    body.set_message_list("images", IMAGE_ITEMS)

    assert _rendered(body) == expected


@pytest.mark.parametrize(
    "raw",
    [
        '[{"id":"p/15"}]',
        5,
        True,
        None,
        {"id": "p/15"},
        ["p/15"],
        [{"id": "p/15"}, 2],
        [None],
    ],
)
def test_set_message_list_refuses_the_outer_shape_the_object_list_refuses(
    raw: Any,
) -> None:
    """The array itself is read by the free-form reader, sentence and all."""
    body = WriteBody({"images": raw})
    body.set_message_list("images", IMAGE_ITEMS)

    assert body.result() == ({}, IMAGES_MUST_BE_OBJECTS)


@pytest.mark.parametrize(
    ("fields", "raw", "expected"),
    [
        (
            IMAGE_ITEMS,
            [{"id": "p/15", "label": 7}],
            "images[0].label must be a string",
        ),
        (
            NUMBERED_ITEMS,
            [{"linode_id": "twelve"}],
            "images[0].linode_id must be an integer",
        ),
        (
            NUMBERED_ITEMS,
            [{"linode_id": 1.5}],
            "images[0].linode_id must be an integer",
        ),
        (
            NUMBERED_ITEMS,
            [{"linode_id": None}],
            "images[0].linode_id must be an integer",
        ),
        (
            NUMBERED_ITEMS,
            [{"linode_id": True}],
            "images[0].linode_id must be an integer",
        ),
        (IMAGE_ITEMS, [{"label": "no id here"}], "images[0].id is required"),
        (
            IMAGE_ITEMS,
            [{"id": "p/15", "bogus": "surprise"}],
            'images[0] has no field "bogus"',
        ),
        (
            IMAGE_ITEMS,
            [{"id": "p/15"}, {"label": "no id here"}],
            "images[1].id is required",
        ),
        (
            IMAGE_ITEMS,
            [{"id": "p/15"}, {"label": "no id here"}, {"bogus": "surprise"}],
            "images[1].id is required",
        ),
    ],
)
def test_set_message_list_reports_the_member_it_declares(
    fields: tuple[ItemField, ...], raw: Any, expected: str
) -> None:
    """Every way one item can be wrong, named with the index that carries it."""
    body = WriteBody({"images": raw})
    body.set_message_list("images", fields)

    assert body.result() == ({}, expected)


def test_set_message_list_reports_the_first_member_in_declaration_order() -> None:
    """An item wrong in more than one way answers with its first problem."""
    body = WriteBody({"images": [{"label": 7, "bogus": "surprise"}]})
    body.set_message_list("images", IMAGE_ITEMS)

    assert body.result() == ({}, "images[0].id is required")


def test_set_message_list_accepts_an_integral_float() -> None:
    """A JSON-RPC number reaches Python as an int and Go as a float.

    An item's integer member reads both shapes, so one call sends one body
    whichever language built it.
    """
    body = WriteBody({"images": [{"linode_id": 12.0}, {"linode_id": 13}]})
    body.set_message_list("images", NUMBERED_ITEMS)

    assert _rendered(body) == '{"images":[{"linode_id":12},{"linode_id":13}]}'


def test_set_message_list_keeps_the_first_failure() -> None:
    """A later field does not overwrite the report, as every setter behaves."""
    body = WriteBody({"images": [{"label": "no id here"}], "data": 5})
    body.set_message_list("images", IMAGE_ITEMS)
    body.set_object("data")

    assert body.result() == ({}, "images[0].id is required")


@pytest.mark.parametrize(
    ("labels", "rendered"),
    [
        ({"zone": "a", "tier": "app"}, '{"labels":{"tier":"app","zone":"a"}}'),
        ({}, '{"labels":{}}'),
    ],
)
def test_set_str_map_writes_each_member(labels: dict[str, str], rendered: str) -> None:
    """A typed map travels key-sorted, and an empty one is a value."""
    body = WriteBody({"labels": labels})
    body.set_str_map("labels")

    assert _rendered(body) == rendered


def test_set_str_map_omits_the_member_the_caller_did_not_supply() -> None:
    """An omitted map leaves the key off the wire rather than sending {}."""
    body = WriteBody({})
    body.set_str_map("labels")

    assert _rendered(body) == "{}"


@pytest.mark.parametrize(
    ("raw", "message"),
    [
        ("tier=app", "labels must be an object"),
        (5, "labels must be an object"),
        (["tier"], "labels must be an object"),
        (None, "labels must be an object"),
        ({"tier": 5}, "labels values must be strings"),
        ({"tier": None}, "labels values must be strings"),
    ],
)
def test_set_str_map_reports_what_is_not_text_keyed_text(
    raw: Any, message: str
) -> None:
    """The caller who sent an array and the one who sent a number differ."""
    body = WriteBody({"labels": raw})
    body.set_str_map("labels")

    assert body.result() == ({}, message)


# The item shape linode_lke_cluster_create declares: the required type/count
# pair, then the two members proto3 gives no `optional` to spell, so their
# presence is read from the item rather than the declaration.
NODE_POOL_ITEMS = (
    ItemField("type", ITEM_STR, required=True),
    ItemField("count", ITEM_INT, required=True),
    ItemField("autoscaler", ITEM_OBJECT),
    ItemField("tags", ITEM_STR_LIST),
)

POOL_BASE = '{"node_pools":[{"type":"g6-standard-2","count":3'


def _pool(**members: Any) -> dict[str, Any]:
    """One node pool carrying the required pair plus whatever is named."""
    return {"type": "g6-standard-2", "count": 3, **members}


@pytest.mark.parametrize(
    ("item", "rendered"),
    [
        (
            _pool(autoscaler={"max": 5}, tags=["team-a", "team-b"]),
            POOL_BASE + ',"autoscaler":{"max":5},"tags":["team-a","team-b"]}]}',
        ),
        (_pool(), POOL_BASE + "}]}"),
        (
            _pool(autoscaler={}, tags=[]),
            POOL_BASE + ',"autoscaler":{},"tags":[]}]}',
        ),
        # Sorted rather than sent as given, because Go marshals a map with its
        # keys sorted and cannot be told otherwise.
        (
            _pool(autoscaler={"min": 1, "max": 5}),
            POOL_BASE + ',"autoscaler":{"max":5,"min":1}}]}',
        ),
    ],
)
def test_set_message_list_writes_the_non_scalar_members_a_node_pool_declares(
    item: dict[str, Any], rendered: str
) -> None:
    """The two kinds beyond scalars travel under the same ordering rule.

    Each is optional because no proto3 map or list can declare presence.
    """
    body = WriteBody({"node_pools": [item]})
    body.set_message_list("node_pools", NODE_POOL_ITEMS)

    assert _rendered(body) == rendered


@pytest.mark.parametrize("raw", ["g6-standard-2", 1, True, None, [], [{"max": 5}]])
def test_set_message_list_refuses_a_non_object_item_member(raw: Any) -> None:
    """An object member is held to what the field-level object setter holds."""
    body = WriteBody({"node_pools": [_pool(autoscaler=raw)]})
    body.set_message_list("node_pools", NODE_POOL_ITEMS)

    assert body.result() == ({}, "node_pools[0].autoscaler must be an object")


@pytest.mark.parametrize("raw", ["team-a", 1, True, None, {}, ["team-a", 2], [None]])
def test_set_message_list_refuses_a_non_string_list_item_member(raw: Any) -> None:
    """A list member is held to exactly what a top-level string list is."""
    body = WriteBody({"node_pools": [_pool(tags=raw)]})
    body.set_message_list("node_pools", NODE_POOL_ITEMS)

    assert body.result() == ({}, "node_pools[0].tags must be an array of strings")


@pytest.mark.parametrize(
    ("arguments", "rendered"),
    [
        ({}, "{}"),
        ({"notes": None}, '{"notes":null}'),
        ({"notes": "on call"}, '{"notes":"on call"}'),
        ({"notes": ""}, '{"notes":""}'),
    ],
)
def test_set_str_or_null_keeps_the_three_states_apart(
    arguments: dict[str, Any], rendered: str
) -> None:
    """The API reads a null as "clear this", which absence does not say."""
    body = WriteBody(arguments)
    body.set_str_or_null("notes")

    assert _rendered(body) == rendered


@pytest.mark.parametrize("raw", [1, True, ["on call"], {}])
def test_set_str_or_null_refuses_what_is_neither_text_nor_null(raw: Any) -> None:
    """Both accepted states are named, since a null is not obvious otherwise."""
    body = WriteBody({"notes": raw})
    body.set_str_or_null("notes")

    assert body.result() == ({}, "notes must be a string or null")


def test_set_str_or_null_keeps_the_first_failure() -> None:
    """The null branch reports nothing, so skip-after-failure spans it."""
    body = WriteBody({"label": 1, "notes": None})
    body.set_str("label")
    body.set_str_or_null("notes")

    assert body.result() == ({}, "label must be a string")


# The comma-composed table, entry for entry the same as commaListCases in
# go/internal/tools/gentools_body_test.go. Both halves must render these bytes.
COMMA_LIST_CASES = [
    ("ssh-rsa AAA", '{"authorized_keys":["ssh-rsa AAA"]}'),
    (" a , b ", '{"authorized_keys":["a","b"]}'),
    ("a,,b,", '{"authorized_keys":["a","b"]}'),
    ("ssh-rsa AAA me@host", '{"authorized_keys":["ssh-rsa AAA me@host"]}'),
    ("", '{"authorized_keys":[]}'),
    (" , , ", '{"authorized_keys":[]}'),
]


@pytest.mark.parametrize(("value", "want"), COMMA_LIST_CASES)
def test_composes_a_comma_separated_list(value: str, want: str) -> None:
    """The argument is one line of text and the member is an array.

    The split is the whole difference between them: each segment trimmed, the
    blank ones dropped, and an argument with nothing left travelling as [].
    """
    body = WriteBody({"authorized_keys": value})
    body.set_comma_str_list("authorized_keys")

    assert _rendered(body) == want


def test_omits_an_unsupplied_comma_list() -> None:
    """The composition is a wire shape, not a reason to send [] unasked."""
    body = WriteBody({})
    body.set_comma_str_list("authorized_keys")

    assert _rendered(body) == "{}"


def test_refuses_a_non_string_comma_list() -> None:
    """The caller sends text, so the sentence names that rather than the array."""
    body = WriteBody({"authorized_keys": ["a"]})
    body.set_comma_str_list("authorized_keys")

    _, message = body.result()

    assert message == "authorized_keys must be a string"


def test_renames_a_composed_comma_list() -> None:
    """The composition and the rename are separate declarations."""
    body = WriteBody({"users": "a, b"})
    body.rename("users", "authorized_users")
    body.set_comma_str_list("users")

    assert _rendered(body) == '{"authorized_users":["a","b"]}'


# The fold the firewall create declares: two flat policy arguments the API reads
# inside its rules object, each defaulting to ACCEPT the way the hand-written
# builder does.
FIREWALL_RULES = (
    FoldMember("inbound_policy", ITEM_STR, argument="inbound_policy", value="ACCEPT"),
    FoldMember("outbound_policy", ITEM_STR, argument="outbound_policy", value="ACCEPT"),
)

# The instance create's one interface: the firewall id and the route flags the
# caller sends flat, plus the arm the tool always says it is attaching.
INSTANCE_INTERFACE = (
    FoldMember("firewall_id", ITEM_INT, argument="firewall_id"),
    FoldMember("default_route.ipv4", ITEM_BOOL, argument="route_ipv4", value=True),
    FoldMember("default_route.ipv6", ITEM_BOOL, argument="route_ipv6", value=True),
    FoldMember("public", ITEM_OBJECT, value={}),
)


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        ({}, '{"rules":{"inbound_policy":"ACCEPT","outbound_policy":"ACCEPT"}}'),
        (
            {"inbound_policy": "DROP", "outbound_policy": "DROP"},
            '{"rules":{"inbound_policy":"DROP","outbound_policy":"DROP"}}',
        ),
        (
            {"rules": {"inbound_policy": "DROP"}, "inbound_policy": "ACCEPT"},
            '{"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}',
        ),
        (
            {"rules": {"inbound_policy": "DROP", "outbound_policy": "DROP"}},
            '{"rules":{"inbound_policy":"DROP","outbound_policy":"DROP"}}',
        ),
    ],
    ids=["neither supplied", "both flat", "caller wrote one key", "caller wrote both"],
)
def test_folds_flat_arguments_into_the_member_the_api_reads(
    arguments: dict[str, Any], want: str
) -> None:
    """A fold is what lets a tool take an argument the API reads one level down.

    Posting inbound_policy at the top of the body looks like a working call and
    changes nothing. A key the caller wrote wins, and the rest fold in beside
    it, which is what keeps the flat arguments optional.
    """
    body = WriteBody(arguments)
    body.fold("rules", FIREWALL_RULES)

    assert _rendered(body) == want


# Annotated because the three argument maps carry different value types, which
# pyright otherwise reads as a partially unknown parametrize argument.
INTERFACE_CASES: list[tuple[dict[str, Any], str]] = [
    (
        {"firewall_id": 7},
        (
            '{"interfaces":[{"default_route":{"ipv4":true,"ipv6":true},'
            '"firewall_id":7,"public":{}}]}'
        ),
    ),
    (
        {"firewall_id": 7, "route_ipv6": False},
        (
            '{"interfaces":[{"default_route":{"ipv4":true,"ipv6":false},'
            '"firewall_id":7,"public":{}}]}'
        ),
    ),
    (
        {"firewall_id": 7, "interfaces": [{"vlan": {}}]},
        '{"interfaces":[{"vlan":{}}]}',
    ),
]


@pytest.mark.parametrize(
    ("arguments", "want"),
    INTERFACE_CASES,
    ids=["synthesized", "flag turned off", "caller supplied the list"],
)
def test_folds_a_synthesized_list_element(arguments: dict[str, Any], want: str) -> None:
    """The API refuses an instance create with no interfaces, so the tool builds one.

    A caller who supplies the list gets it sent verbatim: an element beside
    theirs would attach an interface they never asked for.
    """
    body = WriteBody(arguments)
    body.fold_list("interfaces", INSTANCE_INTERFACE)

    assert _rendered(body) == want


def test_folded_argument_of_the_wrong_kind_is_refused() -> None:
    """A folded argument is held to its declared kind, and the failure says where.

    Both fold shapes answer it, since a synthesized element reads its members
    the same way a merged object does.
    """
    body = WriteBody({"inbound_policy": 5})
    body.fold("rules", FIREWALL_RULES)

    assert body.result() == ({}, "rules.inbound_policy must be a string")

    listed = WriteBody({"firewall_id": "seven"})
    listed.fold_list("interfaces", INSTANCE_INTERFACE)

    assert listed.result() == ({}, "interfaces.firewall_id must be an integer")


def test_constant_member_travels_on_every_call() -> None:
    """Every instance this server creates is on the current interfaces generation."""
    body = WriteBody({})
    body.constant("interface_generation", "linode")

    assert _rendered(body) == '{"interface_generation":"linode"}'


def test_folds_nothing_after_a_failure() -> None:
    """The first type failure answers the whole call.

    A fold declared after one writes nothing rather than reporting a second
    problem, the way every other setter skips itself.
    """
    body = WriteBody({"label": 5})
    body.put_str("label")
    body.fold("rules", FIREWALL_RULES)
    body.fold_list("interfaces", INSTANCE_INTERFACE)
    body.constant("interface_generation", "linode")

    assert body.result() == ({}, "label must be a string")


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        ({"rules": "ACCEPT"}, "rules must be an object"),
        ({"interfaces": "public"}, "interfaces must be an array of objects"),
    ],
    ids=["rules is not an object", "interfaces is not an array"],
)
def test_misshapen_fold_target_is_refused(arguments: dict[str, Any], want: str) -> None:
    """An assembled member the caller supplied is still held to its shape."""
    body = WriteBody(arguments)
    body.fold("rules", FIREWALL_RULES)
    body.fold_list("interfaces", INSTANCE_INTERFACE)

    assert body.result() == ({}, want)


def test_folded_argument_with_no_default_is_omitted() -> None:
    """A folded argument the contract declares no default for is left out.

    Travelling as its type's zero would tell the API the caller chose one.
    """
    body = WriteBody({})
    body.fold_list(
        "interfaces", (FoldMember("firewall_id", ITEM_INT, argument="firewall_id"),)
    )

    assert _rendered(body) == '{"interfaces":[{}]}'


def _firewall_update_rules() -> tuple[FoldMember, ...]:
    """The update's fold: the create's two arguments with no default."""
    return (
        FoldMember("inbound_policy", ITEM_STR, argument="inbound_policy"),
        FoldMember("outbound_policy", ITEM_STR, argument="outbound_policy"),
    )


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        ({"label": "renamed"}, '{"label":"renamed"}'),
        ({"inbound_policy": "DROP"}, '{"rules":{"inbound_policy":"DROP"}}'),
        (
            {"inbound_policy": "DROP", "outbound_policy": "ACCEPT"},
            '{"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}',
        ),
        (
            {"rules": {"inbound_policy": "DROP"}},
            '{"rules":{"inbound_policy":"DROP"}}',
        ),
    ],
)
def test_omits_an_empty_optional_fold(arguments: dict[str, Any], want: str) -> None:
    """The firewall update reads ``rules`` as a replacement rather than a
    setting, so a call naming no policy leaves the member off the wire.

    Sending the empty object there would clear the ruleset the caller never
    mentioned, which is what separates fold_optional from fold.
    """
    body = WriteBody(arguments)
    body.set_str("label")
    body.fold_optional("rules", _firewall_update_rules())

    assert _rendered(body) == want


@pytest.mark.parametrize(
    ("arguments", "want"),
    [
        ({"rules": 5}, "rules must be an object"),
        ({"inbound_policy": 5}, "rules.inbound_policy must be a string"),
    ],
)
def test_bad_optional_fold_is_refused(arguments: dict[str, Any], want: str) -> None:
    """An optional fold is held to the same two failures the always-sent one is."""
    body = WriteBody(arguments)
    body.fold_optional("rules", _firewall_update_rules())

    assert body.result() == ({}, want)


def test_optional_fold_writes_nothing_after_a_failure() -> None:
    """The first type failure answers the whole call."""
    body = WriteBody({"label": 5})
    body.put_str("label")
    body.fold_optional("rules", _firewall_update_rules())

    assert body.result() == ({}, "label must be a string")


def test_set_int_omits_an_explicit_null() -> None:
    """null on an optional int means "use the default", never 0."""
    body = WriteBody({"description": "set", "ttl_sec": None})
    body.set_str("description")
    body.set_int("ttl_sec")

    assert _rendered(body) == '{"description":"set"}'
