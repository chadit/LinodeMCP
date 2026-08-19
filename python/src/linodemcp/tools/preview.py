"""The prose a declared preview reports, and the arguments it reads.

Go's twin lives in go/internal/tools/preview_sentence.go and answers the same
way, so one declared sentence reaches both languages as one sentence.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

if TYPE_CHECKING:
    from collections.abc import Sequence

# The brackets an argument name sits inside a wording.
_OPEN = "{"
_CLOSE = "}"


def preview_text(arguments: dict[str, Any], name: str) -> str:
    """A text argument as a declared preview sentence reports it.

    Empty when the call carries none or carries something that is not text,
    which is what lets a wording naming the argument step aside for the next
    one: a caller who supplied no label reads the sentence that names none
    rather than one with a gap in it.
    """
    value = arguments.get(name)
    return value if isinstance(value, str) else ""


def preview_number(arguments: dict[str, Any], name: str) -> str:
    """A whole-number argument as a declared preview sentence reports it.

    Zero reads as none for the same reason the hooks this replaces read it that
    way: the ids a sentence names are resource ids, and no resource has id zero.
    """
    value = arguments.get(name)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return ""
    if isinstance(value, float) and not value.is_integer():
        return ""
    whole = int(value)
    return str(whole) if whole else ""


# A flag's three states as a declared preview reads it. A declaration answers
# each with a wording of its own, so absent is not folded onto false here and
# each family says which it means.
FLAG_ABSENT = "absent"
FLAG_TRUE = "true"
FLAG_FALSE = "false"


def preview_flag(arguments: dict[str, Any], name: str) -> str:
    """The state of a bool argument a chosen line selects its wording by."""
    return _flag_of(arguments.get(name))


def preview_member_flag(arguments: dict[str, Any], name: str, member: str) -> str:
    """The state of a bool inside an open-object argument.

    That is where the families sending their flag inside the payload carry it.
    """
    obj = arguments.get(name)
    if not isinstance(obj, dict):
        return FLAG_ABSENT
    return _flag_of(cast("dict[str, Any]", obj).get(member))


def _flag_of(value: Any) -> str:
    """One value read as a flag.

    Anything that is not a bool reads as absent, which is what both hand walks
    this replaces did with a flag arriving as text.
    """
    if not isinstance(value, bool):
        return FLAG_ABSENT
    return FLAG_TRUE if value else FLAG_FALSE


def preview_chosen(
    values: dict[str, str],
    state: str,
    when_true: str,
    when_false: str,
    when_absent: str,
) -> str:
    """The wording a flag selects, filled the way an ordered wording is.

    "" for a state the declaration answers with no wording, which drops the
    line.
    """
    wording = {FLAG_TRUE: when_true, FLAG_FALSE: when_false}.get(state, when_absent)
    if not wording:
        return ""
    return preview_sentence(values, wording)


def preview_sentence(values: dict[str, str], *templates: str) -> str:
    """The wording a declared preview line reports.

    The first wording whose placeholders all carry a value, and "" when none is
    complete. A line with no wording to report is dropped rather than reported
    with a gap in it, which is what lets a whole sentence be conditional: the
    volume create names the instance it attaches to only when the call names
    one.
    """
    for template in templates:
        if _filled(values, template):
            return _expand(values, template)

    return ""


def preview_reported_lines(lines: Sequence[str]) -> list[str]:
    """The declared lines that have something to say.

    A line whose wordings could not be filled reports "" and is dropped here,
    so a conditional sentence leaves no empty line behind it.
    """
    return [line for line in lines if line]


def _filled(values: dict[str, str], template: str) -> bool:
    """Whether every placeholder in a wording has a value."""
    return all(values.get(name, "") for name in _placeholders(template))


def _expand(values: dict[str, str], template: str) -> str:
    """Each placeholder's value written into the wording."""
    out: list[str] = []
    rest = template
    while True:
        before, marker, after = rest.partition(_OPEN)
        if not marker:
            out.append(rest)
            return "".join(out)
        out.append(before)
        name, closed, remainder = after.partition(_CLOSE)
        if not closed:
            out.append(_OPEN + after)
            return "".join(out)
        out.append(values.get(name, ""))
        rest = remainder


def _placeholders(template: str) -> list[str]:
    """The argument names one wording reads."""
    names: list[str] = []
    rest = template
    while True:
        _, marker, after = rest.partition(_OPEN)
        if not marker:
            return names
        name, closed, remainder = after.partition(_CLOSE)
        if not closed:
            return names
        names.append(name)
        rest = remainder
