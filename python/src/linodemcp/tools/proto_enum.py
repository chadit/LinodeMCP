"""Validate optional string arguments against proto enum value sets.

Mirrors ``go/internal/tools/proto_enum.go``: the allowed values come from a
generated proto enum (minus the proto3 ``unspecified`` zero sentinel), so
handler validation stays in sync with the generated JSON Schema without a
hand-maintained allowlist. The error text and value order match the Go side so
the message-parity gate stays green.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from google.protobuf.internal.enum_type_wrapper import EnumTypeWrapper

# ENUM_SENTINEL is the proto3 zero-value name every MCP enum defines. It is not a
# real Linode API value: the generated JSON Schema strips it and validation
# rejects it.
ENUM_SENTINEL = "unspecified"


def enum_value_names(enum: EnumTypeWrapper) -> tuple[str, ...]:
    """Return a proto enum's API value names, minus the zero sentinel.

    ``items()`` yields (name, number) pairs in definition order, which is
    enum-number order, matching the Go side's ordering for identical error
    messages.
    """
    return tuple(name for name, _ in enum.items() if name != ENUM_SENTINEL)


def enum_choice_error(value: object, key: str, enum: EnumTypeWrapper) -> str | None:
    """Validate an already-read enum value against a proto enum.

    Empty or non-string is treated as absent and allowed (callers enforce
    required-ness separately, before this). Passing the zero sentinel is rejected
    like any other invalid value. Returns None when valid or absent, else the
    "<key> must be one of: ..." message (text and order match the Go side).
    """
    if not isinstance(value, str) or not value:
        return None
    allowed = enum_value_names(enum)
    if value in allowed:
        return None
    return f"{key} must be one of: {', '.join(allowed)}"


def optional_enum_error(
    arguments: dict[str, Any], key: str, enum: EnumTypeWrapper
) -> str | None:
    """Return an error if an optional enum argument is present and invalid.

    Empty or absent is allowed (the field is optional).
    """
    return enum_choice_error(arguments.get(key), key, enum)


def required_enum_error(
    arguments: dict[str, Any], key: str, enum: EnumTypeWrapper
) -> str | None:
    """Return an error if a required enum argument is absent or invalid.

    Unlike optional_enum_error, an empty or absent value is rejected (the field
    is required), producing the same "<key> must be one of: ..." message as an
    invalid value. Returns None when valid.
    """
    value = arguments.get(key)
    allowed = enum_value_names(enum)
    if isinstance(value, str) and value in allowed:
        return None
    return f"{key} must be one of: {', '.join(allowed)}"
