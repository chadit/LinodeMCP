"""Shared input validation helpers."""

from typing import cast


def is_string_array(value: object) -> bool:
    """Report whether value is an array containing only strings."""
    return isinstance(value, list) and all(
        isinstance(item, str) for item in cast("list[object]", value)
    )


def is_non_blank_string_array(value: object) -> bool:
    """Report whether value is an array containing only non-blank strings."""
    return is_string_array(value) and all(
        item.strip() for item in cast("list[str]", value)
    )
