"""The request body a mutating tool sends, assembled from its contract.

A body is built in the order its fields are declared in the proto. Python dicts
already keep insertion order and Go sorts a map's keys when it marshals, so
naming the order here is what lets ``go/internal/tools/gentools_body.go`` put
the same bytes on the wire for the same call.

Presence, not truthiness, decides what a ``set_`` writes: a field the caller
supplied travels even when its value is the type's zero, because ``""``, ``0``,
and ``[]`` are values the Linode API accepts and acts on. A ``put_`` writes
unconditionally, which is what a proto field without ``optional`` declares.

The first type failure is kept and every later field is skipped, so the message
a caller sees is the first thing wrong with the call rather than the last.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, Literal, NamedTuple, cast

from linodemcp.tools.helpers import optional_tags_argument

if TYPE_CHECKING:
    from collections.abc import Callable, Sequence

ItemKind = Literal[
    "str", "int", "object", "str_list", "bool", "message", "message_list"
]

ITEM_STR: ItemKind = "str"
"""Reads a string member of a typed message."""

ITEM_INT: ItemKind = "int"
"""Reads an integer member of a typed message."""

ITEM_OBJECT: ItemKind = "object"
"""Reads a free-form object member, the shape an item carries when its inner
members vary by kind (a node pool's autoscaler)."""

ITEM_STR_LIST: ItemKind = "str_list"
"""Reads a repeated string member (a node pool's tags)."""

ITEM_BOOL: ItemKind = "bool"
"""Reads a boolean member of a typed message."""

ITEM_MESSAGE: ItemKind = "message"
"""Reads a member carrying a named message of its own, held to the members that
message declares (an interface's vpc ipv4 block)."""

ITEM_MESSAGE_LIST: ItemKind = "message_list"
"""Reads a member repeating a named message, each entry held to the same
members (a vpc ipv4 block's addresses)."""

_STR_LIST_PROSE = "an array of strings"
"""The shape a string array is refused with, wherever the array sits."""

_OBJECT_LIST_PROSE = "an array of objects"
"""The shape an array of objects is refused with, wherever the array sits."""


class ItemField(NamedTuple):
    """One member of a typed message, in the order the proto declares it.

    ``required`` is the inverse of proto3 ``optional``, the same presence rule
    the scalar setters read. ``fields`` is the shape a nested message is read
    under, empty for a kind that reads a value instead.
    """

    name: str
    kind: ItemKind
    required: bool = False
    fields: tuple[ItemField, ...] = ()


class FoldMember(NamedTuple):
    """One member of an assembled body object.

    ``argument`` is the flat tool argument the value comes from, empty for a
    member the contract fixes. ``value`` is what the member travels as when no
    argument fills it: a fixed member's whole value, or a folded argument's
    default. ``None`` leaves the key out, which is what an argument with no
    declared default means. ``key`` is the dotted path it lands at inside the
    member. go/internal/tools's FoldMember carries the same four.
    """

    key: str
    kind: ItemKind = ITEM_STR
    argument: str = ""
    value: Any = None


class WriteBody:
    """One mutating call's JSON request body under construction."""

    def __init__(self, arguments: dict[str, Any]) -> None:
        """Start a body over one tool call's arguments."""
        self._arguments = arguments
        self._fields: dict[str, Any] = {}
        self._renames: dict[str, str] = {}
        self._message: str | None = None

    def rename(self, argument: str, key: str) -> None:
        """Declare the wire key one argument is written under.

        A field the API names differently than the tool does needs both names:
        ``linode_account_user_update`` takes ``new_username`` so the argument
        cannot collide with the username in its path, and the API reads that
        rename as ``username``.
        """
        self._renames[argument] = key

    def put_str(self, name: str) -> None:
        """Write a string field that travels on every call."""
        self._put(name, "", lambda raw: _str_value(name, raw))

    def put_int(self, name: str) -> None:
        """Write an integer field that travels on every call."""
        self._put(name, 0, lambda raw: _int_value(name, raw))

    def set_object(self, name: str) -> None:
        """Write a free-form object field when the caller supplied one.

        A map field cannot carry proto3 ``optional``, so presence is read from
        the arguments rather than the declaration: an omitted argument leaves
        the member off the wire entirely, where writing ``{}`` would tell the
        API the caller sent an empty object. An explicit ``{}`` still travels.
        """
        self._set(name, lambda raw: _object_value(name, raw))

    def set_object_root(self, name: str) -> None:
        """Write a free-form object argument's own members as the whole body.

        The profile preferences PUT reads ``{"theme":"dark"}``, not
        ``{"preferences":{"theme":"dark"}}``. An empty object is refused rather
        than sent: the members are the body, so an empty one is an empty body,
        which is the one thing a route reading the root cannot be asked to do.

        The members travel in sorted order because they have none of their own:
        the contract declares the field, not what a caller puts in it, and
        sorting is the one order both languages can reach without inventing one.
        """
        if self._message is not None:
            return

        raw = self._arguments.get(name)
        if not isinstance(raw, dict) or not raw:
            self._message = f"{name} must be a non-empty object"
            return

        members = cast("dict[str, Any]", raw)
        for key in sorted(members):
            self._fields[key] = members[key]

    def set_object_or_null(self, name: str) -> None:
        """Write a free-form object field that also accepts an explicit null.

        The three states the API reads stay apart: an omitted argument leaves
        the member off the wire, an explicit null travels as ``null``, and an
        object travels as itself. The Linode database update routes read
        ``"private_network": null`` as "detach from the VPC", so a null
        collapsed into absence would report a detach nothing performed.
        """
        self._set(name, lambda raw: _object_or_null_value(name, raw))

    def set_str_map(self, name: str) -> None:
        """Write a string-to-string map field when the caller supplied one.

        The two sentences it answers are separate because the caller who sent
        an array and the caller who sent ``{"a": 1}`` have different things to
        fix.
        """
        self._set(name, lambda raw: _str_map_value(name, raw))

    def set_object_list(self, name: str) -> None:
        """Write a repeated free-form object field when the caller supplied one.

        An empty list is a value: it clears the field rather than leaving it
        alone, the same way an empty string list does.
        """
        self._set(name, lambda raw: _object_list_value(name, raw))

    def set_message(self, name: str, fields: Sequence[ItemField]) -> None:
        """Write a singular named-message field when the caller supplied one.

        It is held to the members its message declares. A message field carries
        no proto3 ``optional``, so presence is read from the arguments the way
        every object shape reads it.
        """
        failure = f"{name} must be {_item_prose(ITEM_MESSAGE)}"
        self._set(name, lambda raw: _message_value(name, failure, fields, raw))

    def set_message_or_null(self, name: str, fields: Sequence[ItemField]) -> None:
        """Write a singular named-message field that also accepts an explicit null.

        The three states the API reads stay apart the way ``set_object_or_null``
        keeps them for the free-form shape. A Linode interface update reads
        ``"vpc": null`` as "detach this arm", so a null collapsed into absence
        would report a detach nothing performed.
        """
        failure = f"{name} must be an object or null"
        self._set(name, lambda raw: _message_or_null_value(name, failure, fields, raw))

    def set_message_list(self, name: str, fields: Sequence[ItemField]) -> None:
        """Write a repeated named-message field when the caller supplied one.

        Each item is held to the members its message declares. An unknown
        member is refused rather than dropped, which is what the generated item
        schema already advertises with ``additionalProperties: false``;
        dropping it would send a call the caller never asked for.
        """
        failure = f"{name} must be {_OBJECT_LIST_PROSE}"
        self._set(name, lambda raw: _message_list_value(name, failure, fields, raw))

    def set_str(self, name: str) -> None:
        """Write an optional string field when the caller supplied one."""
        self._set(name, lambda raw: _str_value(name, raw))

    def set_str_or_null(self, name: str) -> None:
        """Write a string field that also accepts an explicit null.

        The three states the API reads stay apart: an omitted argument leaves
        the member off the wire, an explicit null travels as ``null``, and a
        string travels as itself. The managed service update reads
        ``"notes": null`` as "erase the notes", so a null collapsed into
        absence would report a clear nothing performed.
        """
        self._set(name, lambda raw: _str_or_null_value(name, raw))

    def set_int(self, name: str) -> None:
        """Write an optional integer field when the caller supplied one.

        An explicit null reads as "use the default": the key is omitted rather
        than sent as 0, which is never an id the caller meant. Mirrors Go's
        SetInt.
        """
        if name in self._arguments and self._arguments[name] is None:
            return
        self._set(name, lambda raw: _int_value(name, raw))

    def put_bool(self, name: str) -> None:
        """Write a boolean field that travels on every call."""
        self._put(name, False, lambda raw: _bool_value(name, raw))

    def set_bool(self, name: str) -> None:
        """Write an optional boolean field when the caller supplied one."""
        self._set(name, lambda raw: _bool_value(name, raw))

    def set_str_list(self, name: str) -> None:
        """Write a repeated string field when the caller supplied one.

        An empty list is a value: it clears the field rather than leaving it
        alone.
        """
        self._set(name, lambda raw: _str_list_value(name, raw))

    def set_comma_str_list(self, name: str) -> None:
        """Write a string argument as the array of its comma-separated segments.

        Each segment is trimmed and the blank ones are dropped. A caller pastes
        one line of authorized keys and the API reads an array, so the split is
        the whole of the difference between the two. An argument whose segments
        are all blank travels as an empty array rather than leaving the member
        off, the same way an empty string list does.
        """
        self._set(name, lambda raw: _comma_str_list_value(name, raw))

    def set_int_list(self, name: str) -> None:
        """Write a repeated integer field when the caller supplied one.

        An empty list is a value, the same way an empty string list is.
        """
        self._set(name, lambda raw: _int_list_value(name, raw))

    def set_tags(self, name: str) -> None:
        """Write the ``tags`` field when the caller supplied one.

        Tags have one reader across the whole surface: entries are trimmed, a
        blank one is refused, and a JSON-encoded array arrives as readily as a
        native one, because a client that cannot send an array still has to
        reach the same request body.
        """
        self._set(name, _tags_value)

    def fold(self, name: str, members: Sequence[FoldMember]) -> None:
        """Write a body member assembled from the flat arguments beside it.

        A firewall create takes ``inbound_policy`` and ``outbound_policy`` and
        the API reads both inside ``rules``, so the two arguments travel one
        level down or the API never sees them. The tool always sends the member,
        which is what the API requires of that one.

        A key the caller wrote themselves is left alone and the rest fold in
        beside it, which is what keeps the flat arguments optional: a caller who
        spells the whole object out is not overruled by a default they never
        asked for. Go's WriteBody.Fold merges the same way.
        """
        if self._message is not None:
            return

        base, message = self._fold_base(name)
        if message is not None:
            self._message = message
            return

        folded, message = self._fold_members(name, base, members)
        if message is not None:
            self._message = message
            return

        self._fields[self._renames.get(name, name)] = _sorted_members(folded)

    def fold_list(self, name: str, members: Sequence[FoldMember]) -> None:
        """Write an assembled member the API reads as an array.

        An instance create is given one interface, built from the firewall id
        and the route flags the caller sent flat. A caller who supplies the list
        gets it sent verbatim: synthesizing an element beside theirs would
        attach an interface they never asked for, and merging into the first
        would rewrite one they described in full. Go's WriteBody.FoldList
        answers the same way.
        """
        if self._message is not None:
            return

        if name in self._arguments:
            items = _object_items(self._arguments[name])
            if items is None:
                self._message = f"{name} must be {_OBJECT_LIST_PROSE}"
                return
            self._fields[self._renames.get(name, name)] = _sorted_members(items)
            return

        folded, message = self._fold_members(name, {}, members)
        if message is not None:
            self._message = message
            return

        self._fields[self._renames.get(name, name)] = [_sorted_members(folded)]

    def constant(self, name: str, value: Any) -> None:
        """Write a body member the tool always sends and takes no argument for.

        Every instance this server creates is on the current Linode Interfaces
        generation, so the create says so on every call and no caller chooses
        it.
        """
        if self._message is not None:
            return

        self._fields[self._renames.get(name, name)] = value

    def result(self) -> tuple[dict[str, Any], str | None]:
        """The finished body, or the first type failure the call carried."""
        if self._message is not None:
            return {}, self._message
        return self._fields, None

    def _fold_base(self, name: str) -> tuple[dict[str, Any], str | None]:
        """The object a fold starts from: the caller's, or an empty one.

        Copied rather than written through, so the arguments the rest of the
        call reads are the ones the caller sent.
        """
        if name not in self._arguments:
            return {}, None

        raw = self._arguments[name]
        if not isinstance(raw, dict):
            return {}, f"{name} must be an object"

        return dict(cast("dict[str, Any]", raw)), None

    def _fold_members(
        self, name: str, base: dict[str, Any], members: Sequence[FoldMember]
    ) -> tuple[dict[str, Any], str | None]:
        """Write each declared member, skipping the keys the caller filled."""
        for member in members:
            value, travels, message = self._fold_value(name, member)
            if message is not None:
                return {}, message
            if not travels:
                continue
            _set_fold_key(base, member.key.split("."), value)

        return base, None

    def _fold_value(
        self, name: str, member: FoldMember
    ) -> tuple[Any, bool, str | None]:
        """One member's value: the caller's, the declared default, or none."""
        if not member.argument:
            return member.value, True, None

        if member.argument not in self._arguments:
            return member.value, member.value is not None, None

        value, ok = _item_value(member.kind, self._arguments[member.argument])
        if not ok:
            failure = f"{name}.{member.key} must be {_item_prose(member.kind)}"
            return None, False, failure

        return value, True, None

    def _put(
        self, name: str, absent: Any, convert: Callable[[Any], tuple[Any, str | None]]
    ) -> None:
        """Write one field that travels on every call, whether supplied or not.

        A wrongly typed value is refused rather than read as the zero: no rule
        can speak for a field whose value the message cannot hold, so this is
        the only place the mismatch can still be reported.
        """
        if self._message is not None:
            return

        if name not in self._arguments:
            self._fields[self._renames.get(name, name)] = absent
            return

        value, message = convert(self._arguments[name])
        if message is not None:
            self._message = message
            return

        self._fields[self._renames.get(name, name)] = value

    def _set(self, name: str, convert: Callable[[Any], tuple[Any, str | None]]) -> None:
        """Write one optional field, or keep the first type failure.

        Skipping the rest after a failure is what makes the reported message the
        first problem with the call rather than whichever field happens to be
        declared last.
        """
        if self._message is not None or name not in self._arguments:
            return

        value, message = convert(self._arguments[name])
        if message is not None:
            self._message = message
            return

        self._fields[self._renames.get(name, name)] = value


def _set_fold_key(object_: dict[str, Any], path: list[str], value: Any) -> None:
    """Write one value at a dotted path, leaving a key the caller wrote alone.

    Each object it descends through is copied first, so a nested member the
    caller supplied is read rather than written over. Go's setFoldKey descends
    the same way.
    """
    head = path[0]

    if len(path) == 1:
        object_.setdefault(head, value)
        return

    existing = object_.get(head)
    nested = (
        dict(cast("dict[str, Any]", existing)) if isinstance(existing, dict) else {}
    )
    _set_fold_key(nested, path[1:], value)
    object_[head] = nested


def _str_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One string field's value, or the message a wrong type answers with."""
    if not isinstance(raw, str):
        return None, f"{name} must be a string"
    return raw, None


def _str_or_null_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One nullable string field's value, with an explicit null kept as one.

    ``None`` reaches the body as ``null`` rather than being refused as a
    non-string, because that is the value the API acts on.
    """
    if raw is None:
        return None, None
    if not isinstance(raw, str):
        return None, f"{name} must be a string or null"
    return raw, None


def _int_value(name: str, raw: Any) -> tuple[int, str | None]:
    """One integer field's value, or the message a wrong type answers with.

    ``type(raw) is int`` rather than ``isinstance`` because ``bool`` subclasses
    ``int`` and ``true`` is not a number the API accepts. A float passes only
    when exactly integral, which also rejects NaN and the infinities. Go's
    reader accepts the same two shapes, since a JSON-RPC number reaches it as a
    float and reaches Python as an int.
    """
    if type(raw) is int:
        return raw, None
    if isinstance(raw, float) and raw.is_integer():
        return int(raw), None
    if raw is None:
        return 0, None
    return 0, f"{name} must be an integer"


def _object_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One free-form object field's value, key-sorted at every level.

    Go marshals a map with its keys sorted and cannot be told otherwise, so
    sorting here is what keeps the two languages sending the same bytes for one
    call.
    """
    if not isinstance(raw, dict):
        return None, f"{name} must be an object"
    return _sorted_members(cast("dict[str, Any]", raw)), None


def _object_or_null_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One nullable object field's value, with an explicit null kept as one.

    ``None`` reaches the body as ``null`` rather than being refused as a
    non-object, because that is the value the API acts on.
    """
    if raw is None:
        return None, None
    if not isinstance(raw, dict):
        return None, f"{name} must be an object or null"
    return _sorted_members(cast("dict[str, Any]", raw)), None


def _str_map_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One string-to-string map field's value, key-sorted like every object.

    A non-text value is refused rather than rendered: what Python would print
    for it is not what the caller meant to send.
    """
    if not isinstance(raw, dict):
        return None, f"{name} must be an object"
    members = cast("dict[str, Any]", raw)
    if not all(isinstance(member, str) for member in members.values()):
        return None, f"{name} values must be strings"
    return {key: members[key] for key in sorted(members)}, None


def _sorted_members(value: Any) -> Any:
    """A JSON value with every object's members in key order."""
    if isinstance(value, dict):
        members = cast("dict[str, Any]", value)
        return {key: _sorted_members(members[key]) for key in sorted(members)}
    if isinstance(value, list):
        return [_sorted_members(item) for item in cast("list[object]", value)]
    return value


def _object_items(raw: Any) -> list[dict[str, Any]] | None:
    """The array of objects a repeated object field arrives as, else ``None``.

    An entry that is not an object is refused rather than skipped: a grant
    nobody could read is not one the API should be told to apply. Both the
    free-form and the typed list readers accept exactly this shape.
    """
    if not isinstance(raw, list):
        return None

    found: list[dict[str, Any]] = []
    for item in cast("list[object]", raw):
        if not isinstance(item, dict):
            return None
        found.append(cast("dict[str, Any]", item))
    return found


def _object_list_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One repeated object field's value, or the message a wrong type answers.

    Members are key-sorted the way ``_object_value`` sorts them, since Go
    marshals each entry's map with its keys sorted and cannot be told
    otherwise.
    """
    items = _object_items(raw)
    if items is None:
        return None, f"{name} must be {_OBJECT_LIST_PROSE}"
    return [_sorted_members(item) for item in items], None


def _message_value(
    path: str, failure: str, fields: Sequence[ItemField], raw: Any
) -> tuple[Any, str | None]:
    """One named-message value, held to the members its message declares.

    ``failure`` is the sentence a value that is not an object is refused with,
    which is the one place the nullable setter's prose differs.
    """
    if not isinstance(raw, dict):
        return None, failure
    return _message_object(path, fields, cast("dict[str, Any]", raw))


def _message_or_null_value(
    path: str, failure: str, fields: Sequence[ItemField], raw: Any
) -> tuple[Any, str | None]:
    """One nullable named-message value, with an explicit null kept as one."""
    if raw is None:
        return None, None
    return _message_value(path, failure, fields, raw)


def _message_list_value(
    path: str, failure: str, fields: Sequence[ItemField], raw: Any
) -> tuple[Any, str | None]:
    """One typed list value, or the first entry that fails its shape.

    The outer shape is the free-form reader's, so a typed list accepts exactly
    what an object list does; what it adds is holding each entry to the members
    its message declares.
    """
    items = _object_items(raw)
    if items is None:
        return None, failure

    built: list[dict[str, Any]] = []
    for index, item in enumerate(items):
        member, message = _message_object(f"{path}[{index}]", fields, item)
        if message is not None:
            return None, message
        built.append(cast("dict[str, Any]", member))
    return built, None


def _message_object(
    path: str, fields: Sequence[ItemField], item: dict[str, Any]
) -> tuple[Any, str | None]:
    """One object held to the members a message declares, in declaration order.

    ``path`` is what the sentences below address the object by: the field name
    at the top, and the member or the indexed entry each nested reader reached
    it through underneath.

    The first failure answers the whole call, the way the body's first type
    failure does, so the caller hears the first thing wrong rather than the
    last.
    """
    built: dict[str, Any] = {}
    for entry in fields:
        value, travels, message = _item_member(path, entry, item)
        if message is not None:
            return None, message
        if travels:
            built[entry.name] = value

    unknown = _unknown_item_key(fields, item)
    if unknown is not None:
        return None, f'{path} has no field "{unknown}"'
    return built, None


def _item_member(
    path: str, entry: ItemField, item: dict[str, Any]
) -> tuple[Any, bool, str | None]:
    """One declared member of one object: its value, whether it travels, a failure."""
    member = f"{path}.{entry.name}"
    if entry.name not in item:
        if entry.required:
            return None, False, f"{member} is required"
        return None, False, None

    value, message = _member_value(member, entry, item[entry.name])
    if message is not None:
        return None, False, message
    return value, True, None


def _member_value(path: str, entry: ItemField, raw: Any) -> tuple[Any, str | None]:
    """One supplied member read under its declared kind.

    A member carrying a message of its own reads through the same readers a
    field-level message does, and every other kind reads a value.
    """
    failure = f"{path} must be {_item_prose(entry.kind)}"
    if entry.kind == ITEM_MESSAGE:
        return _message_value(path, failure, entry.fields, raw)
    if entry.kind == ITEM_MESSAGE_LIST:
        return _message_list_value(path, failure, entry.fields, raw)

    value, ok = _item_value(entry.kind, raw)
    if not ok:
        return None, failure
    return value, None


def _item_prose(kind: ItemKind) -> str:
    """One item kind named the way its refusal sentence reads."""
    if kind == ITEM_INT:
        return "an integer"
    if kind in {ITEM_OBJECT, ITEM_MESSAGE}:
        return "an object"
    if kind == ITEM_STR_LIST:
        return _STR_LIST_PROSE
    if kind == ITEM_MESSAGE_LIST:
        return _OBJECT_LIST_PROSE
    if kind == ITEM_BOOL:
        return "a boolean"
    return "a string"


def _item_value(kind: ItemKind, raw: Any) -> tuple[Any, bool]:
    """One item member read under its declared kind.

    A null is refused where a null scalar reads as zero: a member nobody sent
    is not one the API should be told to act on.
    """
    if kind == ITEM_INT:
        return _item_int_value(raw)
    if kind == ITEM_STR_LIST:
        return _str_list_items(raw)
    if kind == ITEM_OBJECT:
        return (
            (_sorted_members(cast("dict[str, Any]", raw)), True)
            if isinstance(raw, dict)
            else (None, False)
        )
    if kind == ITEM_BOOL:
        return (raw, True) if isinstance(raw, bool) else (None, False)
    return (raw, True) if isinstance(raw, str) else (None, False)


def _item_int_value(raw: Any) -> tuple[int, bool]:
    """The int and float shapes a JSON-RPC number arrives as.

    ``type(raw) is int`` rather than ``isinstance`` because ``bool`` subclasses
    ``int``, the same reason ``_int_value`` spells it that way.
    """
    if type(raw) is int:
        return raw, True
    if isinstance(raw, float) and raw.is_integer():
        return int(raw), True
    return 0, False


def _unknown_item_key(fields: Sequence[ItemField], item: dict[str, Any]) -> str | None:
    """The first member an item carries that its message does not declare.

    Key order rather than arrival order, because a Go map iterates in none and
    the two languages have to report the same sentence.
    """
    declared = {entry.name for entry in fields}
    unknown = sorted(key for key in item if key not in declared)
    return unknown[0] if unknown else None


def _bool_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One boolean field's value, or the message a wrong type answers with."""
    if not isinstance(raw, bool):
        return None, f"{name} must be a boolean"
    return raw, None


def _int_list_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One repeated integer field's value, or the message a wrong type answers.

    A null entry is refused where a null scalar reads as zero: a list is a set
    of ids, and an id nobody sent is not one of them.
    """
    failure = f"{name} must be an array of integers"
    if not isinstance(raw, list):
        return None, failure

    found: list[int] = []
    for item in cast("list[object]", raw):
        if item is None:
            return None, failure
        value, message = _int_value(name, item)
        if message is not None:
            return None, failure
        found.append(value)
    return found, None


def _tags_value(raw: Any) -> tuple[Any, str | None]:
    """The ``tags`` field's value under the surface-wide tags reader.

    It delegates to the reader the hand-written handlers call so a tag set
    reaches the API through one set of rules, whichever tool sent it.
    """
    tags, message = optional_tags_argument({"tags": raw})
    if message:
        return None, message
    return tags, None


def _str_list_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One repeated string field's value, or the message a wrong type answers."""
    value, ok = _str_list_items(raw)
    if not ok:
        return None, f"{name} must be {_STR_LIST_PROSE}"
    return value, None


def _comma_str_list_value(name: str, raw: Any) -> tuple[Any, str | None]:
    """One comma-composed field's array, or the message a wrong type answers.

    The argument is text, so the sentence a non-string answers is the string
    one: the array is the wire shape, not the shape the caller sends.
    """
    if not isinstance(raw, str):
        return None, f"{name} must be a string"
    trimmed = (segment.strip() for segment in raw.split(","))
    return [segment for segment in trimmed if segment], None


def _str_list_items(raw: Any) -> tuple[list[str] | None, bool]:
    """The array of strings a repeated string field arrives as.

    Shared with the item reader so a list inside a typed item is held to
    exactly what a top-level one is.
    """
    if not isinstance(raw, list) or not all(
        isinstance(item, str) for item in cast("list[object]", raw)
    ):
        return None, False
    return cast("list[str]", raw), True
