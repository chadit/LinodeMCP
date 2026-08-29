"""The prose a declared preview reports, and the arguments it reads.

Go's twin lives in go/internal/tools/preview_sentence.go and answers the same
way, so one declared sentence reaches both languages as one sentence.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from linodemcp.tools.declared_state import DeclaredState

if TYPE_CHECKING:
    from collections.abc import Callable, Sequence

    # The lines one half of a declared preview reports: the wordings themselves
    # when nothing in them reads the resource, and a function of the fetched
    # state when something does, since a sentence naming what the resource
    # carries cannot be worded until the read has answered.
    PreviewLines = Sequence[str] | Callable[[Any], Sequence[str]]

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


def _state_at(state: Any, path: str) -> tuple[DeclaredState, str]:
    """A declared state walked down to the object holding the path's last name.

    A state some other fetch produced reads as carrying nothing, so a wording
    over it steps aside rather than reporting a gap.
    """
    if not isinstance(state, DeclaredState):
        return DeclaredState({}), ""
    head, separator, member = path.partition(".")
    if not separator:
        return state, head
    return state.object(head), member


def preview_state_text(state: Any, path: str) -> str:
    """A text member of the fetched resource as a declared sentence reports it."""
    fields, name = _state_at(state, path)
    return fields.text(name)


def preview_state_number(state: Any, path: str) -> str:
    """A whole-number member of the fetched resource as a sentence reports it.

    Zero reads as none for the reason preview_number reads it that way: the
    sizes and ids a sentence names are absent at zero, which is what lets the
    wording naming a starting size give way to the one that names only the
    target.
    """
    fields, name = _state_at(state, path)
    whole = fields.number(name)
    return str(whole) if whole else ""


def preview_changed(reading: str, argument: str) -> str:
    """A state reading as a wording that only names a change reports it.

    The reading itself, and "" where it already matches the argument the call
    would set. Matching reads as absent so the wording naming the reading steps
    aside for the next one, which is what turns "Label changes from X to Y" into
    "Label is set to Y" without a second rule for choosing between them.
    """
    return "" if reading == argument else reading


def preview_carried(arguments: dict[str, Any], name: str) -> bool:
    """Whether the call sent an argument at all.

    Presence rather than emptiness is the question a line guarded on a value it
    never names asks: a number, a list and an object each have a real zero value
    a caller can mean, so a throttle of 0 and an empty tag list are both
    carried. A text argument is guarded on its own reader instead, which already
    collapses empty onto absent.
    """
    return name in arguments


def preview_carried_number(arguments: dict[str, Any], name: str) -> str:
    """A whole number a line has already guarded on being carried.

    Zero reports as the value it is rather than as no value, which is what the
    guard has already established.
    """
    value = arguments.get(name)
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        return ""
    if isinstance(value, float) and not value.is_integer():
        return ""
    return str(int(value))


def preview_guarded(carried: bool, line: str) -> str:
    """A line the call has to have asked for.

    The wording when it did, and "" when it did not, which drops the line.
    """
    return line if carried else ""


def preview_differs(reading: str, argument: str) -> bool:
    """Whether a line naming only the new value has a change to report.

    The call carries one, and the resource does not already hold it. A wording
    that names both values needs none of this, since the reading it names steps
    aside on its own.
    """
    return bool(argument) and reading != argument


def preview_or(value: str, folded: str) -> str:
    """A folded argument as a wording reports it.

    The value the call carried, or the one that argument's own fold sends in
    its place. The default is declared once, in the fold, so the sentence and
    the request it describes name the same value. Go's PreviewOr answers the
    same way.
    """
    return value or folded


def preview_folded(reading: str) -> str:
    """A state reading as a matched line compares it.

    A resource spells its own vocabularies, so a status the API sends as
    "Running" selects the arm declared for "running"; an argument's values are
    the ones the tool's schema publishes and are compared exactly. Go's
    PreviewFolded answers the same way.
    """
    return reading.lower()


def preview_matched(
    values: dict[str, str], value: str, arms: dict[str, str], otherwise: str
) -> str:
    """The wording a value selects, filled the way an ordered wording is.

    "" for a value the declaration answers with no wording, which drops the
    line.
    """
    wording = arms.get(value, otherwise)
    if not wording:
        return ""
    return preview_sentence(values, wording)


# The placeholder an entry fills, which is the one name a wording may read that
# is not an argument.
PREVIEW_ELEMENT_NAME = "element"


def _element_value(raw: Any) -> str:
    """One entry of a repeated argument as a wording reports it.

    "" for an entry neither language spells the same way.
    """
    if isinstance(raw, str):
        return raw
    if isinstance(raw, bool) or not isinstance(raw, (int, float)):
        return ""
    if isinstance(raw, float) and not raw.is_integer():
        return ""
    return str(int(raw))


def _elements(arguments: dict[str, Any], name: str) -> list[str]:
    """The entries of a repeated argument, the unspellable ones dropped."""
    raw = arguments.get(name)
    if not isinstance(raw, list):
        return []
    # Cast to Sequence rather than list: pyright reads a narrowed list's element
    # as unknown, and mypy calls a cast back to list[Any] redundant.
    entries = [_element_value(item) for item in cast("Sequence[Any]", raw)]
    return [entry for entry in entries if entry]


def preview_joined(arguments: dict[str, Any], name: str, separator: str) -> str:
    """The entries of a repeated argument as one value.

    For the line that names them all rather than one each.
    """
    return separator.join(_elements(arguments, name))


def preview_per_element(
    arguments: dict[str, Any], name: str, values: dict[str, str], template: str
) -> list[str]:
    """One line per entry of a repeated argument, in the order the call sent them.

    A list the call did not send writes no lines at all, which is what a preview
    owes a caller who asked for no memberships.
    """
    return [
        preview_sentence({**values, PREVIEW_ELEMENT_NAME: entry}, template)
        for entry in _elements(arguments, name)
    ]


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


def preview_reported_lines(lines: PreviewLines, state: Any = None) -> list[str]:
    """The declared lines that have something to say, worded against the state.

    A line whose wordings could not be filled reports "" and is dropped here,
    so a conditional sentence leaves no empty line behind it.
    """
    worded = lines(state) if callable(lines) else lines
    return [line for line in worded if line]


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
