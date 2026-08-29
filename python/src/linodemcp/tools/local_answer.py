"""The Python engine a declared ``local_answer`` runs on.

Holds the subsystem operations a meta tool is built from and the readers their
inputs arrive through. An operation is handed typed values and nothing else, so
it can neither read the argument map nor tell which tool reached it: it answers
a plain body and the condition it met without words, and the generated handler
projects that body onto the message the tool's own contract declares and writes
the sentence. Naming no message is the output half of the same guard the typed
parameters are the input half of.

Mirrors ``go/internal/tools/local_answer.go``.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from enum import Enum, auto
from typing import TYPE_CHECKING, Any, cast

from google.protobuf import json_format
from google.protobuf.descriptor import FieldDescriptor
from mcp.types import TextContent

from linodemcp.genlocal import CallEntry
from linodemcp.linode.routes import message_class
from linodemcp.tools.argreader import tool_bool, tool_int, tool_string, tool_string_list
from linodemcp.tools.helpers import error_response, read_optional_time
from linodemcp.tools.proto_response import proto_to_canonical_dict

if TYPE_CHECKING:
    from google.protobuf.message import Message


class LocalRefusal(Enum):
    """One condition an operation reports.

    Spelled for the condition rather than for any tool, so two tools sharing an
    operation share its report.
    """

    NONE = auto()
    DRAFT_MISSING = auto()
    NOT_FOUND = auto()
    ALREADY_EXISTS = auto()
    READ_FAILED = auto()
    WRITE_FAILED = auto()
    INPUT_REJECTED = auto()
    BUILTIN_PROFILE = auto()


# The keys one call-list entry is read through, and the environment argument a
# checked call may carry. They belong to the entry's own shape rather than to
# any tool, so the reader spells them once.
_ENTRY_TOOL = "tool"
_ENTRY_ARGUMENTS = "args"
_ENTRY_ENVIRONMENT = "environment"


@dataclass(frozen=True)
class LocalOutcome:
    """Everything an operation answers.

    Either the body to project, or the condition it met and the cause a
    sentence may name.
    """

    body: dict[str, Any]
    cause: str
    refusal: LocalRefusal


def local_answered(body: dict[str, Any]) -> LocalOutcome:
    """The outcome of an operation that ran to a body."""
    return LocalOutcome(body=body, cause="", refusal=LocalRefusal.NONE)


def local_refused(refusal: LocalRefusal, cause: str = "") -> LocalOutcome:
    """The outcome of an operation that met a condition."""
    return LocalOutcome(body={}, cause=cause, refusal=refusal)


def local_unreported(cause: str) -> LocalOutcome:
    """The outcome of an operation failing in a way its declaration misses.

    No tool words a sentence for it, so the outcome carries no body and the
    handler answers the disagreement between the body and the message rather
    than a body half of nothing.
    """
    return LocalOutcome(body={}, cause=cause, refusal=LocalRefusal.NONE)


def local_string(arguments: dict[str, Any], argument: str) -> str:
    """Read one text input off the call.

    Reading through here rather than at each call site is what holds both
    languages to one coercion rule.
    """
    return tool_string(arguments, argument)


def local_int(arguments: dict[str, Any], argument: str) -> int:
    """Read one whole-number input off the call."""
    return tool_int(arguments, argument)


def local_bool(arguments: dict[str, Any], argument: str) -> bool:
    """Read one flag input off the call."""
    return tool_bool(arguments, argument)


def local_bool_sent(arguments: dict[str, Any], argument: str) -> bool | None:
    """Read one flag input together with whether the call carried it.

    A setter needs the difference: False alone cannot separate a flag turned
    off from a flag the caller never mentioned.
    """
    if argument not in arguments:
        return None

    return tool_bool(arguments, argument)


def local_string_list(arguments: dict[str, Any], argument: str) -> list[str]:
    """Read one list-of-text input off the call."""
    return tool_string_list(arguments, argument)


def local_string_list_sent(
    arguments: dict[str, Any], argument: str
) -> list[str] | None:
    """Read one list-of-text input together with whether the call carried it.

    For the reason ``local_bool_sent`` does: an empty list replaces a setting
    and an absent one leaves it alone.
    """
    if argument not in arguments:
        return None

    return tool_string_list(arguments, argument)


def local_timestamp(arguments: dict[str, Any], argument: str) -> tuple[Any, str]:
    """Read one RFC 3339 bound, answering a cause for a value it cannot read.

    The declaring tool's sentence names the argument, so the cause names only
    what the value got wrong.
    """
    return read_optional_time(tool_string(arguments, argument))


def local_call_entries(arguments: dict[str, Any], argument: str) -> list[CallEntry]:
    """Read one call-list input off the call.

    An entry that is not an object names no call and is dropped; a tool named
    as anything but text reads as empty, which is the answer the text reader
    gives too. The record it answers is the generated one, because a subsystem
    takes the list as a typed parameter and the answers module is the one place
    both the engine and the handlers can name a type from.
    """
    raw = arguments.get(argument)
    if not isinstance(raw, list):
        return []

    entries: list[CallEntry] = []
    for item in cast("list[object]", raw):
        if not isinstance(item, dict):
            continue

        entry = cast("dict[str, Any]", item)
        name = entry.get(_ENTRY_TOOL)
        entries.append(
            CallEntry(
                tool=name if isinstance(name, str) else "",
                environment=_entry_environment(entry),
            )
        )

    return entries


def _entry_environment(entry: dict[str, Any]) -> str:
    """The environment one entry's arguments name, empty where it named none.

    An empty environment and an absent one are one answer, because neither says
    which environment the call would reach.
    """
    raw = entry.get(_ENTRY_ARGUMENTS)
    if not isinstance(raw, dict):
        return ""

    arguments = cast("dict[str, Any]", raw)
    environment = arguments.get(_ENTRY_ENVIRONMENT)

    return environment if isinstance(environment, str) else ""


# The messages whose body is free-form JSON rather than a member set the
# contract declares, so the nested rule stops at them.
_WELL_KNOWN_PREFIX = "google.protobuf."


def local_quoted(value: str) -> str:
    """One value in quotes, spelled the way the other language's %q spells it."""
    return json.dumps(value)


def local_response(response: str, body: dict[str, Any]) -> list[TextContent]:
    """Project what an operation answered onto the message its tool declares.

    Serializing here rather than in each generated handler is what keeps the
    answer canonical, the way Go's LocalResponse does.
    """
    message = message_class(response)()

    refusal = _fill_local_body(message, body)
    if refusal:
        return error_response(refusal)

    answer = json.dumps(proto_to_canonical_dict(message), indent=2)

    return [TextContent(type="text", text=answer)]


def _fill_local_body(message: Message, body: dict[str, Any]) -> str:
    """Write a plain body onto its message, answering their first disagreement.

    Both directions are held because either one answers a caller a shape the
    contract does not describe: a member the body leaves out would report a
    zero nothing computed, and one the message does not declare would be
    dropped in silence. Reading the message in declaration order and the body
    in name order is what makes one disagreement report the same way on every
    run and in both languages.
    """
    descriptor = message.DESCRIPTOR

    refusal = _local_body_members(descriptor, body)
    if refusal:
        return refusal

    refusal = _local_nested_members(descriptor, body)
    if refusal:
        return refusal

    try:
        json_format.ParseDict(body, message, ignore_unknown_fields=False)
    except json_format.ParseError as exc:
        return f"{descriptor.full_name}: the answer does not fit: {exc}"

    return ""


def _local_body_members(descriptor: Any, body: dict[str, Any]) -> str:
    """Hold one body's own members to the message's, reading the message in
    declaration order and the body in name order so one disagreement is
    reported the same way on every run and in both languages.

    The descriptor is untyped because protobuf's own reflection answers two
    descriptor classes for one message, a pure-Python one and an upb one, and
    naming either narrows this to the build that answered it.
    """
    declared = {field.name for field in descriptor.fields}

    for field in descriptor.fields:
        if field.name not in body:
            return f"{descriptor.full_name}: the answer leaves {field.name} unfilled"

    for name in sorted(body):
        if name not in declared:
            return (
                f"{descriptor.full_name}: the answer names {name},"
                " which the message does not declare"
            )

    return ""


def _local_nested_members(descriptor: Any, body: dict[str, Any]) -> str:
    """Hold every record nested in a body to the message that record fills.

    The top-level rule alone cannot see them, and ParseDict reads a member a
    nested record leaves out as that member's zero, so an incomplete record
    answers a value nothing computed and reports success.
    """
    for field in descriptor.fields:
        nested = _local_nested_descriptor(field)
        if nested is None:
            continue

        refusal = _local_nested_member(field, nested, body.get(field.name))
        if refusal:
            return refusal

    return ""


def _local_nested_descriptor(field: Any) -> Any:
    """The message one member's records fill, None where the member carries
    none: a scalar, a well-known wrapper whose body is free-form JSON, or a map
    keyed to a scalar.
    """
    if field.type != FieldDescriptor.TYPE_MESSAGE:
        return None

    if _is_keyed(field):
        return _declared_record(field.message_type.fields_by_name["value"])

    return _declared_record(field)


def _declared_record(field: Any) -> Any:
    """One member's own message where it declares a member set, None for a
    scalar and None for a well-known wrapper whose body is free-form JSON.
    """
    if field.type != FieldDescriptor.TYPE_MESSAGE:
        return None

    if field.message_type.full_name.startswith(_WELL_KNOWN_PREFIX):
        return None

    return field.message_type


def _is_keyed(field: Any) -> bool:
    """Whether a repeated member is a proto map rather than a list."""
    return bool(
        field.is_repeated
        and field.message_type is not None
        and field.message_type.GetOptions().map_entry
    )


def _local_nested_member(field: Any, nested: Any, value: Any) -> str:
    """Hold the records one member carries: a map's values, a list's elements,
    or the single record a member holds.
    """
    if _is_keyed(field):
        return _local_nested_records(nested, _keyed_records(value))

    if field.is_repeated:
        return _local_nested_records(nested, value)

    if value is None:
        return ""

    return _local_nested_records(nested, [value])


def _keyed_records(value: Any) -> list[Any]:
    """A keyed member's values in key order, so one disagreement is reported
    the same way on every run.
    """
    held = cast("dict[str, object]", value) if isinstance(value, dict) else {}

    return [held[key] for key in sorted(held)]


def _local_nested_records(nested: Any, records: Any) -> str:
    """Hold every record in one member to the message it fills, its own nested
    members included.

    A member carrying no list of records at all is left to ParseDict, which
    reports the type disagreement in its own words.
    """
    listed = cast("list[object]", records) if isinstance(records, list) else []

    for record in listed:
        if not isinstance(record, dict):
            continue

        held = cast("dict[str, Any]", record)

        refusal = _local_body_members(nested, held) or _local_nested_members(
            nested, held
        )
        if refusal:
            return refusal

    return ""
