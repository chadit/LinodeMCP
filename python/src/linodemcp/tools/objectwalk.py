"""Evaluate the open-object walks a tool's input message declares.

An open object is a map or a repeated Struct field: the device slots on a config
write, the boot helpers beside them, the phone numbers on a Managed contact, the
SSH block on Managed settings, the default firewall assignments, the grant
sections on a user's grants. Every value such a field can hold is a
``google.protobuf.Value``, so the message accepts them all and a buf.validate
rule sees a size rather than the members the API reads. That is why these were
the last checks still written out by hand in each language.

The declaration is read from the descriptor at call time, the same way
``constraints`` reads rules, so declaring a walk is a proto edit and the
generated handler only names the seam. Go's ``toolwalk`` package reads the same
declaration.
"""

from __future__ import annotations

import re
from typing import TYPE_CHECKING, Any, cast

from google.protobuf import descriptor_pool

from linodemcp.genpb.linode.mcp.v1 import options_pb2
from linodemcp.tools.constraints import load_contract

if TYPE_CHECKING:
    from google.protobuf.descriptor import Descriptor

# protoc types a generated extension as a plain FieldDescriptor while the
# runtime's own accessors ask for the extension-handle type, so the handle is
# named once here rather than at every use.
_OBJECT_WALK: Any = options_pb2.object_walk

# The kinds a member's value is held to. Named rather than read off the enum so
# the branch below reads as the shapes it accepts.
_KIND_INT = options_pb2.VALUE_KIND_INT
_KIND_BOOL = options_pb2.VALUE_KIND_BOOL
_KIND_TEXT = options_pb2.VALUE_KIND_TEXT
_KIND_OBJECT = options_pb2.VALUE_KIND_OBJECT

# What a sentence names, filled from where the walk is when it answers. See
# ObjectWalk in options.proto.
_FIELD = "{field}"
_KEY = "{key}"
_KEYS = "{keys}"
_PARENT = "{parent}"
_VALUES = "{values}"
_MINIMUM = "{minimum}"
_MAXIMUM = "{maximum}"
_MAX_LENGTH = "{max_length}"

# What a non_blank member refuses. Unicode spacing included, so the answer
# matches Go's strings.TrimSpace rather than an ASCII-only test.
_BLANK = re.compile(r"^\s*$")


def walk(input_message: str, arguments: dict[str, Any]) -> str:
    """Answer the first walk the arguments break, in the words the walk declares.

    The answer is the tool's own sentence: nothing here wraps or prefixes it,
    because a caller reads it as the reason their call was refused. An empty
    string means the arguments break none, which is also the answer for a
    message that declares no walk.

    ``input_message`` is the full name of the tool's input message, such as
    ``linode.mcp.v1.InstanceConfigCreateInput``.
    """
    descriptor = _walked(input_message)
    if descriptor is None:
        return ""

    for declared in descriptor.GetOptions().Extensions[_OBJECT_WALK]:
        message = _run_walk(descriptor, declared, arguments)
        if message:
            return message

    return ""


def _walked(input_message: str) -> Descriptor | None:
    """Resolve a message that declares walks, or None when it declares none."""
    load_contract()

    try:
        descriptor = descriptor_pool.Default().FindMessageTypeByName(input_message)
    except KeyError:
        return None

    if not descriptor.GetOptions().Extensions[_OBJECT_WALK]:
        return None

    return descriptor


def _run_walk(descriptor: Descriptor, declared: Any, arguments: dict[str, Any]) -> str:
    """Apply one declaration to each argument it names, in declared order."""
    for name in declared.field:
        message = _walk_argument(declared, descriptor, str(name), arguments)
        if message:
            return message

    return ""


def _walk_argument(
    declared: Any, descriptor: Descriptor, name: str, arguments: dict[str, Any]
) -> str:
    """Answer for one argument: whether it was sent, its shape, then its members.

    An explicit null reads as absent, which is how both hand walks read one: a
    caller who sent ``devices: null`` named no device slot.
    """
    where = {_FIELD: name}

    raw = arguments.get(name)
    if name not in arguments or raw is None:
        return _fill(declared.absent, where)

    # The field says which of the two open shapes this is. A name the message
    # does not carry never reaches here, because the emitter refuses a walk
    # that declares one.
    entry = descriptor.fields_by_name.get(name)
    nested = None if entry is None else entry.message_type
    if nested is not None and nested.GetOptions().map_entry:
        return _walk_object(declared, where, raw)

    return _walk_list(declared, where, raw)


def _walk_object(declared: Any, where: dict[str, str], raw: Any) -> str:
    """Answer for a map argument and the members inside it."""
    if not isinstance(raw, dict):
        return _fill(declared.unusable, where)

    obj = cast("dict[str, Any]", raw)
    if not obj and declared.empty:
        return _fill(declared.empty, where)

    return _walk_members(declared.members, obj, where)


def _walk_list(declared: Any, where: dict[str, str], raw: Any) -> str:
    """Answer for a repeated Struct argument, entry by entry.

    The index stands where a map walk's key does, since a list entry has no
    other name.
    """
    if not isinstance(raw, list):
        return _fill(declared.unusable, where)

    entries = cast("list[object]", raw)
    for index, entry in enumerate(entries):
        inside = {_FIELD: where[_FIELD], _PARENT: str(index)}
        message = _walk_entry(declared, inside, entry)
        if message:
            return message

    return ""


def _walk_entry(declared: Any, where: dict[str, str], entry: Any) -> str:
    """Answer for one list entry."""
    if not isinstance(entry, dict):
        return _fill(declared.element, where)

    obj = cast("dict[str, Any]", entry)

    return _walk_members(declared.members, obj, where)


def _walk_members(members: Any, obj: dict[str, Any], where: dict[str, str]) -> str:
    """Answer for one object: unknown keys, each declared member, then the arms."""
    message = _unknown_keys(members, obj, where)
    if message:
        return message

    for key in members.key:
        message = _check_value(members.value, obj, str(key), where)
        if message:
            return message

    for member in members.member:
        message = _check_value(member.value, obj, str(member.name), where)
        if message:
            return message

    return _require_arms(members.require, obj, where)


def _unknown_keys(members: Any, obj: dict[str, Any], where: dict[str, str]) -> str:
    """Answer for every key outside the vocabulary at once.

    A sentence naming ``{keys}`` reports all of them and one naming ``{key}``
    reports the first in sorted order.
    """
    if not members.unknown:
        return ""

    known = _vocabulary(members)
    extra = sorted(key for key in obj if key not in known)
    if not extra:
        return ""

    return _fill(members.unknown, {**where, _KEY: extra[0], _KEYS: ", ".join(extra)})


def _vocabulary(members: Any) -> tuple[str, ...]:
    """Every key an object may carry, shared-value keys first."""
    return (
        *(str(key) for key in members.key),
        *(str(member.name) for member in members.member),
    )


def _require_arms(require: Any, obj: dict[str, Any], where: dict[str, str]) -> str:
    """Answer for the members together: naming none of them, then naming two."""
    named = sum(1 for name in require.name if name in obj)

    if named == 0 and require.any_of:
        return _fill(require.any_of, where)

    if named > 1 and require.at_most_one:
        return _fill(require.at_most_one, where)

    return ""


def _check_value(
    value: Any, obj: dict[str, Any], key: str, where: dict[str, str]
) -> str:
    """Answer for one member: presence, then kind, then bounds or vocabulary."""
    here = {**where, _KEY: key}

    if key not in obj:
        return _fill_value(value.absent, here, value)

    raw = obj[key]
    if raw is None:
        if value.nullable:
            return ""

        return _fill_value(value.unusable, here, value)

    return _check_kind(value, raw, here)


def _check_kind(value: Any, raw: Any, where: dict[str, str]) -> str:
    """Answer for a member the caller did send, under the kind it declares."""
    if value.kind == _KIND_INT:
        return _check_int(value, raw, where)

    if value.kind == _KIND_BOOL:
        if not isinstance(raw, bool):
            return _fill_value(value.unusable, where, value)
        return ""

    if value.kind == _KIND_TEXT:
        return _check_text(value, raw, where)

    if value.kind == _KIND_OBJECT:
        return _check_object(value, raw, where)

    return ""


def _check_int(value: Any, raw: Any, where: dict[str, str]) -> str:
    """Answer for a whole-number member and the bounds it declares.

    ``bool`` is excluded before ``int`` because True would otherwise read as 1.
    """
    number = _whole_number(raw)
    if number is None:
        return _fill_value(value.unusable, where, value)

    if value.minimum and number < value.minimum:
        return _refusal(value, where)

    if value.maximum and number > value.maximum:
        return _refusal(value, where)

    return ""


def _check_text(value: Any, raw: Any, where: dict[str, str]) -> str:
    """Answer for a text member: the blank, the cap, and the vocabulary."""
    if not isinstance(raw, str):
        return _fill_value(value.unusable, where, value)

    if value.non_blank and _BLANK.match(raw):
        return _refusal(value, where)

    if value.max_length and len(raw) > value.max_length:
        return _refusal(value, where)

    names = _enum_names(str(value.values))
    if names and raw not in names:
        return _refusal(value, where)

    return ""


def _check_object(value: Any, raw: Any, where: dict[str, str]) -> str:
    """Answer for a member the walk reads one level further in."""
    if not isinstance(raw, dict):
        return _fill_value(value.unusable, where, value)

    obj = cast("dict[str, Any]", raw)
    inside = {_FIELD: where[_FIELD], _PARENT: where.get(_KEY, "")}

    return _walk_members(value.members, obj, inside)


def _refusal(value: Any, where: dict[str, str]) -> str:
    """The sentence a value of the right kind and the wrong content gets.

    A member wording one sentence for every way it can be wrong leaves
    ``refused`` empty, and its unusable sentence answers here too.
    """
    if value.refused:
        return _fill_value(value.refused, where, value)

    return _fill_value(value.unusable, where, value)


def _whole_number(raw: Any) -> int | None:
    """The whole number a JSON value spells, or None when it spells none."""
    if isinstance(raw, bool):
        return None

    if isinstance(raw, int):
        return raw

    if isinstance(raw, float) and raw.is_integer():
        return int(raw)

    return None


def _enum_names(full_name: str) -> tuple[str, ...]:
    """One declared vocabulary: the enum's member names without the zero sentinel.

    Read from the descriptor in enum-number order, so the vocabulary has one
    home even as the enum grows.
    """
    load_contract()

    # An empty name does not resolve, which is the answer a member declaring no
    # vocabulary needs: it is held to none.
    try:
        enum = descriptor_pool.Default().FindEnumTypeByName(full_name)
    except KeyError:
        return ()

    numbered = sorted(
        (value.number, str(value.name)) for value in enum.values if value.number != 0
    )

    return tuple(name for _, name in numbered)


def _fill(sentence: str, where: dict[str, str]) -> str:
    """Write a walk's position into a declared sentence.

    An empty sentence stays empty, which is how a walk says it accepts what it
    just looked at.
    """
    if not sentence:
        return ""

    for placeholder, value in where.items():
        sentence = sentence.replace(placeholder, value)

    return sentence


def _fill_value(sentence: str, where: dict[str, str], value: Any) -> str:
    """Write the position and the bounds a member declares into its sentence.

    The bounds are filled from the declaration so a sentence stating one does
    not carry a second copy of the number.
    """
    if not sentence:
        return ""

    filled = _fill(sentence, where)
    filled = filled.replace(_VALUES, ", ".join(_enum_names(str(value.values))))
    filled = filled.replace(_MINIMUM, str(value.minimum))
    filled = filled.replace(_MAXIMUM, str(value.maximum))

    return filled.replace(_MAX_LENGTH, str(value.max_length))
