"""The declared dependency-walk engine.

The emitter renders each walk as a WalkSpec literal and this interpreter runs
them, so both languages word one blast radius from one declaration. Mirrors
Go's internal/tools/walk_spec.go: same element handling, same template
vocabulary, same drop rule, same failure wording.
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any, cast

from linodemcp.tools.declared_state import DeclaredState
from linodemcp.tools.drivers import read_envelope_state, read_route_state

if TYPE_CHECKING:
    from linodemcp.linode import RetryableClient
    from linodemcp.tools.helpers import DryRunDetails

_PLACEHOLDER = re.compile(r"\{([^{}]+)\}")

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class WalkFilter:
    """Keeps elements whose field matches a literal or an argument."""

    field: str
    value: str = ""
    argument: str = ""
    fold: bool = False


@dataclass(frozen=True)
class WalkEmit:
    """What each element becomes."""

    kind: str = ""
    kind_field: str = ""
    kind_fallback: str = ""
    id_field: str = ""
    label_field: str = ""
    label_fallback: str = ""
    action: str = ""
    note: str = ""
    side_effects: bool = False


@dataclass(frozen=True)
class WalkWarning:
    """One sentence about the whole, said under one predicate."""

    template: str
    when_field: str = ""
    when_equals: str = ""
    when_positive: str = ""
    when_present: str = ""
    when_absent: str = ""
    when_truncated: bool = False
    when_any: bool = False


@dataclass(frozen=True)
class WalkEnrich:
    """One best-effort read decorating the templates."""

    tool: str
    fields: tuple[str, ...]
    values: tuple[object, ...] = ()


@dataclass(frozen=True)
class WalkSpec:
    """One declared walk, resolved to values the runtime can act on."""

    emit: WalkEmit = field(default_factory=WalkEmit)
    list_tool: str = ""
    list_member: str = ""
    state_member: str = ""
    state_scalar: str = ""
    list_error_warning: str = ""
    filter: WalkFilter | None = None
    enrich: WalkEnrich | None = None
    warnings: tuple[WalkWarning, ...] = ()
    values: tuple[object, ...] = ()


async def run_dependency_walks(
    client: RetryableClient,
    specs: list[WalkSpec],
    state: DeclaredState,
    arguments: dict[str, Any],
) -> DryRunDetails:
    """Run each spec in order over the declared state and assemble one answer.

    This is what a dry run reports and a plan carries beside its hash.
    """
    details: DryRunDetails = {}
    for spec in specs:
        await _run_walk(client, spec, details, state, arguments)
    return details


async def _run_walk(
    client: RetryableClient,
    spec: WalkSpec,
    details: DryRunDetails,
    state: DeclaredState,
    arguments: dict[str, Any],
) -> None:
    """One walk: gather elements, filter, emit, then the warnings."""
    elements, total = await _elements(client, spec, details, state)
    kept = _filtered(spec, elements, arguments)
    pools = await _pools(client, spec, state, arguments, kept)

    emitted = 0
    for element in kept:
        if _emit_element(spec, details, element, pools):
            emitted += 1

    _say_warnings(spec, details, state, pools, kept, emitted, total, len(elements))


async def _elements(
    client: RetryableClient,
    spec: WalkSpec,
    details: DryRunDetails,
    state: DeclaredState,
) -> tuple[list[DeclaredState], int]:
    """The walk's elements and the envelope total when one rides along.

    A failed list becomes the declared warning rather than an error, landing
    before the declared warnings while the walk continues over no elements.
    """
    if spec.list_tool and spec.list_member:
        return await _member_elements(client, spec, details)

    if spec.list_tool:
        return await _list_elements(client, spec, details)

    if spec.state_scalar:
        number = state.number(spec.state_scalar)
        if number is not None and number > 0:
            return [state], 0
        return [], 0

    elements = state.objects(spec.state_member)
    total = state.number("results")
    return elements, total if total is not None else len(elements)


async def _list_elements(
    client: RetryableClient, spec: WalkSpec, details: DryRunDetails
) -> tuple[list[DeclaredState], int]:
    """The walk's own list read, best-effort."""
    try:
        envelope = await read_envelope_state(client, *spec.values, tool=spec.list_tool)
    except Exception as failure:
        details.setdefault("warnings", []).append(
            spec.list_error_warning.replace("{error}", str(failure))
        )
        return [], 0

    elements = envelope.objects("data")
    total = envelope.number("results")
    return elements, total if total is not None else len(elements)


async def _member_elements(
    client: RetryableClient, spec: WalkSpec, details: DryRunDetails
) -> tuple[list[DeclaredState], int]:
    """The nested-member list a non-envelope route answers: the instance IP
    read keeps ipv4.public and nothing else.
    """
    try:
        declared = await read_route_state(client, *spec.values, tool=spec.list_tool)
    except Exception as failure:
        details.setdefault("warnings", []).append(
            spec.list_error_warning.replace("{error}", str(failure))
        )
        return [], 0

    elements = _nested_objects(declared, spec.list_member)
    return elements, len(elements)


def _nested_objects(declared: DeclaredState, path: str) -> list[DeclaredState]:
    """Walk a dotted member path down to its repeated elements."""
    head, dot, rest = path.partition(".")
    if dot:
        return _nested_objects(declared.object(head), rest)
    return declared.objects(head)


# The amount a dry run reports when it cannot price the change: a failed price
# read, a state carrying no type, or a call with no price to read at all.
# go/internal/tools binds the same sentinel as BillingUnknown.
BILLING_UNKNOWN = "unknown"


@dataclass(frozen=True)
class BillingSpec:
    """Prices what a removal stops paying for."""

    price_tool: str
    type_field: str
    amount: str
    sentence: str
    unknown_sentence: str
    absent_type_sentence: str = ""


async def run_billing_delta(
    client: RetryableClient,
    spec: BillingSpec,
    state: DeclaredState,
    details: DryRunDetails,
) -> None:
    """Word the monthly consequence beside the walks' answer."""
    typed = state.text(spec.type_field)
    if not typed:
        details["billing_delta"] = {
            "monthly_change_usd": BILLING_UNKNOWN,
            "note": spec.absent_type_sentence,
        }
        return

    monthly = await _monthly_price(client, spec, typed)
    if monthly is None:
        details["billing_delta"] = {
            "monthly_change_usd": BILLING_UNKNOWN,
            "note": spec.unknown_sentence,
        }
        return

    amount = spec.amount.replace("{monthly:.2f}", f"{monthly:.2f}")
    details["billing_delta"] = {"monthly_change_usd": amount, "note": spec.sentence}


async def _monthly_price(
    client: RetryableClient, spec: BillingSpec, typed: str
) -> float | None:
    """The type's monthly price, best-effort."""
    try:
        declared = await read_route_state(client, typed, tool=spec.price_tool)
    except Exception:
        return None

    monthly = declared.object("price").fields.get("monthly")
    if isinstance(monthly, (int, float)) and not isinstance(monthly, bool):
        return float(monthly)
    return None


def _filtered(
    spec: WalkSpec, elements: list[DeclaredState], arguments: dict[str, Any]
) -> list[DeclaredState]:
    """The elements the declared filter matches."""
    if spec.filter is None:
        return elements

    wanted = spec.filter.value
    if spec.filter.argument:
        wanted = _render(arguments.get(spec.filter.argument))

    return [
        element
        for element in elements
        if _field_matches(element, spec.filter.field, wanted, spec.filter.fold)
    ]


def _field_matches(element: DeclaredState, path: str, wanted: str, fold: bool) -> bool:
    """One possibly-dotted element field, one level into an open object."""
    head, dot, rest = path.partition(".")
    if not dot:
        return _value_equals(element.fields.get(head), wanted, fold)

    if rest.startswith("*."):
        member = rest[2:]
        for value in element.object(head).fields.values():
            if isinstance(value, dict) and _value_equals(
                cast("dict[str, Any]", value).get(member), wanted, fold
            ):
                return True
        return False

    return _value_equals(element.object(head).fields.get(rest), wanted, fold)


def _value_equals(value: object, wanted: str, fold: bool) -> bool:
    rendered = _render(value)
    if fold:
        return rendered.lower() == wanted.lower()
    return rendered == wanted


def _emit_element(
    spec: WalkSpec,
    details: DryRunDetails,
    element: DeclaredState,
    pools: _Pools,
) -> bool:
    """One element into the declared target.

    A note whose element placeholders all render empty drops the line, the
    rule every declared sentence follows; the dependency itself still lands.
    """
    if spec.emit.side_effects:
        line, said = _fill_template(spec.emit.note, element, pools)
        if said:
            details.setdefault("side_effects", []).append(line)
            return True
        return False

    kind = spec.emit.kind
    if spec.emit.kind_field:
        kind = (
            _render(_field_value(element, spec.emit.kind_field))
            or spec.emit.kind_fallback
        )

    dependency: dict[str, Any] = {"kind": kind, "action": spec.emit.action}

    if spec.emit.id_field:
        identifier = _field_value(element, spec.emit.id_field)
        if identifier is not None:
            dependency["id"] = identifier

    if spec.emit.label_field:
        label = _render(_field_value(element, spec.emit.label_field))
        if not label and spec.emit.label_fallback:
            label, said = _fill_template(spec.emit.label_fallback, element, pools)
            label = label if said else ""
        if label:
            dependency["label"] = label

    line, said = _fill_template(spec.emit.note, element, pools)
    if said:
        dependency["note"] = line

    details.setdefault("dependencies", []).append(dependency)
    return True


def _field_value(element: DeclaredState, path: str) -> Any:
    head, dot, rest = path.partition(".")
    if not dot:
        return element.fields.get(head)
    return element.object(head).fields.get(rest)


def _say_warnings(
    spec: WalkSpec,
    details: DryRunDetails,
    state: DeclaredState,
    pools: _Pools,
    kept: list[DeclaredState],
    emitted: int,
    total: int,
    fetched: int,
) -> None:
    """The whole-walk sentences whose predicates hold."""
    pools.aggregates = {
        "count": str(emitted),
        "total": str(fetched),
        "max_results": str(max(total, emitted)),
    }

    for warning in spec.warnings:
        _fill_sums(
            pools.aggregates,
            warning.template + " {" + warning.when_positive + "}",
            kept,
        )
        if not _warning_holds(warning, state, pools.aggregates):
            continue
        line, said = _fill_template(warning.template, state, pools)
        if said:
            details.setdefault("warnings", []).append(line)


def _fill_sums(
    aggregates: dict[str, str], text: str, kept: list[DeclaredState]
) -> None:
    """Every {sum:...} the text names, computed over the kept elements.

    A numeric field summed, or a nested list's length summed for the sum:len
    form, matching Go's fillSums.
    """
    for match in _PLACEHOLDER.finditer(text):
        name = match.group(1)
        if not name.startswith("sum:") or aggregates.get(name):
            continue
        operand = name[4:]
        total = 0
        for element in kept:
            if operand.startswith("len:"):
                total += len(element.objects(operand[4:]))
            else:
                number = element.number(operand)
                if number is not None:
                    total += number
        aggregates[name] = str(total)


def _warning_holds(
    warning: WalkWarning, state: DeclaredState, aggregates: dict[str, str]
) -> bool:
    if warning.when_field:
        return state.text(warning.when_field).lower() == warning.when_equals.lower()
    if warning.when_positive:
        return _aggregate_positive(warning.when_positive, state, aggregates)
    if warning.when_present:
        return _render(state.fields.get(warning.when_present)) != ""
    if warning.when_absent:
        return _render(state.fields.get(warning.when_absent)) == ""
    return _window_holds(warning, aggregates)


def _window_holds(warning: WalkWarning, aggregates: dict[str, str]) -> bool:
    """The count-window predicates and the unconditional default."""
    if warning.when_truncated:
        return int(aggregates["max_results"]) > int(aggregates["count"])
    if warning.when_any:
        return aggregates["count"] != "0"
    return True


def _aggregate_positive(
    name: str, state: DeclaredState, aggregates: dict[str, str]
) -> bool:
    """One named aggregate as the guard value, matching Go's reading."""
    if name.startswith("state:"):
        number = state.number(name[6:])
        return number is not None and number > 0
    return int(aggregates.get(name, "0") or "0") > 0


@dataclass
class _Pools:
    """Everything a template can interpolate beside the element."""

    state: DeclaredState
    arguments: dict[str, Any]
    enriched: DeclaredState = field(default_factory=lambda: DeclaredState({}))
    aggregates: dict[str, str] = field(default_factory=dict[str, str])


async def _pools(
    client: RetryableClient,
    spec: WalkSpec,
    state: DeclaredState,
    arguments: dict[str, Any],
    kept: list[DeclaredState],
) -> _Pools:
    """The interpolation pools, performing the enrichment read.

    The read runs only when the walk kept an element, the laziness the hand
    bodies had by skipping the call outright over nothing.
    """
    pools = _Pools(state=state, arguments=arguments)
    if spec.enrich is None or not kept:
        return pools

    try:
        pools.enriched = await read_route_state(
            client, *spec.enrich.values, tool=spec.enrich.tool
        )
    except Exception as failure:
        # Decoration only: the walk's own lines stand without it.
        logger.debug("walk enrichment read failed", extra={"error": str(failure)})
    return pools


def _fill_template(
    template: str, element: DeclaredState, pools: _Pools
) -> tuple[str, bool]:
    """One interpolated template.

    Reports False when every element placeholder rendered empty, which drops
    the line.
    """
    if not template:
        return "", False

    element_placeholders = 0
    filled = 0

    def _one(match: re.Match[str]) -> str:
        nonlocal element_placeholders, filled
        value, is_element = _fill_placeholder(match.group(1), element, pools)
        if is_element:
            element_placeholders += 1
            if value:
                filled += 1
        return value

    line = _PLACEHOLDER.sub(_one, template)
    if element_placeholders > 0 and filled == 0:
        return "", False
    return line, True


def _fill_placeholder(
    name: str, element: DeclaredState, pools: _Pools
) -> tuple[str, bool]:
    """One placeholder, reporting whether it read the element."""
    # A computed aggregate wins only where one was computed, which is what
    # resolves the collision rule by position: warnings fill after the
    # aggregates land, notes before, so a note's {count} is the element's own.
    if pools.aggregates.get(name):
        return pools.aggregates[name], False
    if name.startswith("len:"):
        return str(len(element.objects(name[4:]))), True
    prefixed = _fill_prefixed(name, pools)
    if prefixed is not None:
        return prefixed, False
    return _render(_field_value(element, name)), True


def _fill_prefixed(name: str, pools: _Pools) -> str | None:
    """The pool-fed placeholder forms, None for an element field."""
    if name.startswith("arg:"):
        return _render(pools.arguments.get(name[4:]))
    if name.startswith("state:"):
        return _render(pools.state.fields.get(name[6:]))
    if name.startswith("enrich:"):
        return _render(pools.enriched.fields.get(name[7:]))
    if name.startswith("sum:"):
        return pools.aggregates.get(name, "")
    return None


def _render(value: object) -> str:
    """One value worded the way both languages spell it."""
    if value is None:
        return ""
    if value is True:
        return "true"
    if value is False:
        return "false"
    return str(value)
