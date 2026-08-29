"""One reader per argument type, matching the Go accessors exactly.

Go handlers read every tool argument through mcp-go's ``CallToolRequest``
getters (``GetString``, ``GetInt``, ``GetBool``, ``GetStringSlice``), which
coerce a wrongly typed value to the default rather than raising or keying on
it. Python handlers used to reach into the arguments dict with ``.get`` and a
per-tool cast, so the same malformed call answered differently in each
language: a non-string name created a draft here and refused there, ``"false"``
read True here and False there, ``int("abc")`` raised here and defaulted there.

These four functions are that one rule, restated once. Every handler binds its
arguments through them, so a future coercion divergence has exactly one place
to appear.
"""

from __future__ import annotations

import re
from typing import Any, cast

# Go's strconv.Atoi accepts an optional sign and base-10 digits with no
# surrounding space, where Python's int() also takes whitespace and
# underscores; the pattern holds the two to the same accepted spelling.
_GO_INT_TEXT = re.compile(r"^[+-]?[0-9]+$")

# Atoi reports a range error outside int64, where Python integers are
# unbounded, so a value past the edge reads as unparseable here too.
_INT64_MIN = -(1 << 63)
_INT64_MAX = (1 << 63) - 1

# The twelve spellings strconv.ParseBool accepts, and nothing else.
_GO_TRUE_TEXT = frozenset({"1", "t", "T", "TRUE", "true", "True"})
_GO_FALSE_TEXT = frozenset({"0", "f", "F", "FALSE", "false", "False"})


def tool_string(arguments: dict[str, Any], key: str, default: str = "") -> str:
    """The string argument at ``key``, or ``default`` when it is anything else."""
    value = arguments.get(key)
    if isinstance(value, str):
        return value

    return default


def tool_int(arguments: dict[str, Any], key: str, default: int = 0) -> int:
    """The int argument at ``key``, or ``default`` when it does not convert.

    Booleans convert to nothing because Go's type switch has no arm for them,
    and a float truncates toward zero the way a Go int conversion does.
    """
    value = arguments.get(key)
    if isinstance(value, bool):
        return default

    if isinstance(value, int):
        return value

    if isinstance(value, float):
        return int(value)

    if isinstance(value, str):
        return _int_from_text(value, default)

    return default


def _int_from_text(text: str, default: int) -> int:
    """One numeric string read the way strconv.Atoi reads it."""
    if not _GO_INT_TEXT.match(text):
        return default

    parsed = int(text)
    if parsed < _INT64_MIN or parsed > _INT64_MAX:
        return default

    return parsed


def tool_bool(arguments: dict[str, Any], key: str, *, default: bool = False) -> bool:
    """The bool argument at ``key``, or ``default`` when it does not convert.

    A string converts only through the spellings strconv.ParseBool accepts, so
    ``"false"`` reads False rather than the True that Python truthiness gives
    every non-empty string.
    """
    value = arguments.get(key)
    if isinstance(value, bool):
        return value

    if isinstance(value, str):
        return _bool_from_text(value, default=default)

    if isinstance(value, int | float):
        return value != 0

    return default


def _bool_from_text(text: str, *, default: bool) -> bool:
    """One boolean string read the way strconv.ParseBool reads it."""
    if text in _GO_TRUE_TEXT:
        return True

    if text in _GO_FALSE_TEXT:
        return False

    return default


def tool_string_list(arguments: dict[str, Any], key: str) -> list[str]:
    """The string entries of the array argument at ``key``.

    A non-string entry is dropped rather than coerced, matching Go's
    stringArrayArg: coercing one would put a value the caller never named into
    a saved draft.
    """
    raw = arguments.get(key)
    if not isinstance(raw, list):
        return []

    entries = cast("list[object]", raw)

    return [entry for entry in entries if isinstance(entry, str)]
