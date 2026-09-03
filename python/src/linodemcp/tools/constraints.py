"""Evaluate the buf.validate rules a tool's input message declares.

A rule is written once in the proto contract and read here and by Go's
``toolvalidate``, so an argument check has one wording instead of one per
language. Before this module every such check was a hand-written hook in each
language and nothing held the two copies to the same answer.

Only message-level rules are read. A field-level buf.validate rule is
deliberately not used anywhere in the contract: protoschema-jsonschema turns one
into a JSON Schema keyword, which changes the schema every client already reads,
and schema stability is a contract of its own.

``protovalidate`` resolves its own ``buf.validate`` import through the aliases
``linodemcp/__init__.py`` registers, which is why importing it here needs no
setup of its own.
"""

from __future__ import annotations

import importlib
import pkgutil
import re
from functools import cache
from typing import TYPE_CHECKING, Any

import protovalidate
from google.protobuf import descriptor_pool, json_format
from google.protobuf.descriptor import FieldDescriptor
from google.protobuf.message_factory import GetMessageClass

from linodemcp.genpb.buf.validate import validate_pb2
from linodemcp.genpb.linode.mcp import v1 as genpb
from linodemcp.genpb.linode.mcp.v1 import options_pb2

if TYPE_CHECKING:
    from google.protobuf.descriptor import Descriptor
    from google.protobuf.message import Message

_BODY_OR_QUERY = (
    options_pb2.FIELD_LOCATION_BODY,
    options_pb2.FIELD_LOCATION_QUERY,
)

# protoc types a generated extension as a plain FieldDescriptor while the
# runtime's own accessors ask for the extension-handle type, so the two
# disagree wherever a handle is passed. Naming the handles once, typed as the
# runtime treats them, keeps that disagreement in one place instead of at every
# use.
_MESSAGE_RULES: Any = validate_pb2.message
_FIELD_LOCATION: Any = options_pb2.field_location

# The names a call carries for the server rather than for the route, so no input
# message declares them and the refusal below has to let them through by name.
# twostage_destroy.py reads confirmed_dry_run and confirm_bypass_dry_run;
# server/__init__.py reads yolo. Declaring them as system param fields, which
# would retire this set, is booked as a follow-up.
_ENGINE_CONTROL_ARGUMENTS = frozenset(
    {"confirm_bypass_dry_run", "confirmed_dry_run", "yolo"}
)

# The JSON number grammar a string offered for an integer field has to spell end
# to end, because each language's reader is lenient in its own direction
# otherwise: Go's takes the 123 out of "123/456" and Python's accepts spellings
# of its own such as "1_000". re.ASCII keeps \d off Arabic-Indic digits, which
# int() reads and Go's identical pattern does not.
_WHOLE_NUMBER = re.compile(r"^-?(0|[1-9]\d*)(\.\d+)?([eE][-+]?\d+)?$", re.ASCII)

_INTEGER_TYPES = frozenset(
    {
        FieldDescriptor.TYPE_INT32,
        FieldDescriptor.TYPE_INT64,
        FieldDescriptor.TYPE_UINT32,
        FieldDescriptor.TYPE_UINT64,
        FieldDescriptor.TYPE_SINT32,
        FieldDescriptor.TYPE_SINT64,
        FieldDescriptor.TYPE_FIXED32,
        FieldDescriptor.TYPE_FIXED64,
        FieldDescriptor.TYPE_SFIXED32,
        FieldDescriptor.TYPE_SFIXED64,
    }
)


def check(input_message: str, arguments: dict[str, Any]) -> str:
    """Answer the first rule the arguments break, in the words the rule declares.

    The answer is the tool's own sentence: nothing here wraps or prefixes it,
    because a caller reads it as the reason their call was refused rather than
    as a validation library's report. An empty string means the arguments break
    no rule, which is also the answer for a message that declares none.

    ``input_message`` is the full name of the tool's input message, such as
    ``linode.mcp.v1.DomainCreateInput``.
    """
    descriptor = _constrained(input_message)
    if descriptor is None:
        return ""

    message = _build(descriptor, arguments)
    if message is None:
        return ""

    try:
        protovalidate.validate(message)
    except protovalidate.ValidationError as failure:
        return _first_violation(failure)

    return ""


@cache
def load_contract() -> None:
    """Register every contract message with the default descriptor pool.

    A pool lookup only finds a message whose generated module has been
    imported, and a handler does not import its own: a tool whose input message
    nothing else pulls in would then read as declaring no rules and skip every
    check it declares. Go has no equivalent gap, since its handlers import the
    generated package and its init registers the whole file set.

    Public because ``objectwalk`` resolves out of the same pool and would
    otherwise carry a second copy of this.
    """
    for module in pkgutil.iter_modules(genpb.__path__):
        if module.name.endswith("_pb2"):
            importlib.import_module(f"{genpb.__name__}.{module.name}")


def unknown_arguments(
    arguments: dict[str, Any], tool_name: str, input_message: str
) -> str:
    """Answer for any argument the tool's input message does not declare.

    Mirrors Go's refusedAsUndeclared, sentence included: each language holds one
    copy and the behavior fixtures are what keep the two equal. A message the
    pool does not carry answers "", since without a descriptor there is no
    allowlist to hold the call to.
    """
    descriptor = _input_descriptor(input_message)
    if descriptor is None:
        return ""

    declared = set(descriptor.fields_by_name) | _ENGINE_CONTROL_ARGUMENTS
    unknown = sorted(name for name in arguments if name not in declared)
    if not unknown:
        return ""

    return f"Unsupported argument(s) for {tool_name}: {', '.join(unknown)}"


def _input_descriptor(input_message: str) -> Descriptor | None:
    """Resolve a tool's input message, or None when the pool does not carry it."""
    load_contract()

    try:
        return descriptor_pool.Default().FindMessageTypeByName(input_message)
    except KeyError:
        return None


def _constrained(input_message: str) -> Descriptor | None:
    """Resolve a message that declares rules, or None when it declares none.

    A message with none is the common case and costs one pool lookup, which is
    what keeps the seam on every generated handler affordable.
    """
    descriptor = _input_descriptor(input_message)
    if descriptor is None:
        return None

    if not descriptor.GetOptions().HasExtension(_MESSAGE_RULES):
        return None

    return descriptor


def _first_violation(failure: protovalidate.ValidationError) -> str:
    """Read the sentence out of a validation failure.

    Fail-fast is not asked for here, so the first violation is the one the rule
    order put first. The sentence is the rule's own: every rule the contract
    declares carries one, which Go's toolvalidate package pins, so this never
    reports the library's default wording.
    """
    violations: list[Any] = list(failure.violations)

    return str(violations[0].proto.message) if violations else ""


def _build(descriptor: Descriptor, arguments: dict[str, Any]) -> Message | None:
    """Assemble the input message from one call's arguments.

    What an argument the field cannot hold means depends on where the field
    travels, because that is what decides who reports it today:

    - A body or query field is read by the builder for that part of the request,
      which reports the mismatch in its own words. Those words are what clients
      read, so this answers None and leaves the whole check to it rather than
      pre-empting it with a rule.
    - A path or local field is read through an accessor that answers the zero
      value rather than raising, so the field is left absent here for the same
      reason: a caller who sent "abc" for an id gets the tool's own "is
      required" sentence.
    - A string naming no member of an enum field is set to the enum's zero,
      wherever it travels, because a string IS the shape an enum argument
      arrives in. That is what lets a rule tell an argument naming nothing from
      an absent one, the difference between "status must be one of ..." and no
      complaint at all.
    - A string offered for an integer field is read only when it spells one whole
      number, because a reader that takes "123/456" as 123 lets a rule pass on an
      id the path read then refuses in different words.
    """
    message = GetMessageClass(descriptor)()

    for name, value in arguments.items():
        field = descriptor.fields_by_name.get(name)
        if field is None:
            continue

        if not _misspelled_number(field, value) and _set_field(message, name, value):
            continue

        if _unknown_enum_name(field, value):
            setattr(message, name, 0)

            continue

        if _reported_elsewhere(field):
            return None

    return message


def _set_field(message: Message, name: str, value: Any) -> bool:
    """Write one argument into the message, reporting whether it had the type.

    It goes through the JSON reader so an argument is read exactly as the
    generated schema advertises it, rather than through a second hand-written
    reading of the same JSON.
    """
    try:
        json_format.ParseDict({name: value}, message)
    except json_format.ParseError:
        return False

    return True


def _misspelled_number(field: FieldDescriptor, value: Any) -> bool:
    """Report whether a value is a string offered for an integer field that
    spells anything but one whole number, the reading left to the field's own
    rule rather than to a JSON reader."""
    if not isinstance(value, str):
        return False

    return field.type in _INTEGER_TYPES and _WHOLE_NUMBER.fullmatch(value) is None


def _unknown_enum_name(field: FieldDescriptor, value: Any) -> bool:
    """Report whether a value is a string offered for an enum field naming none
    of its members, the one refusal above that a rule still gets to answer."""
    enum_type = field.enum_type
    if enum_type is None or field.is_repeated:
        return False

    if not isinstance(value, str):
        return False

    return value not in enum_type.values_by_name


def _reported_elsewhere(field: FieldDescriptor) -> bool:
    """Report whether another builder answers a type mismatch on this field,
    which is the case for everything that reaches the wire as body or query."""
    location: Any = field.GetOptions().Extensions[_FIELD_LOCATION]

    return bool(location in _BODY_OR_QUERY)
