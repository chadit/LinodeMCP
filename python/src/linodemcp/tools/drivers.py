"""Config-shaped drivers for the routed tool tiers.

A caller names its tool and hands over only what the descriptors cannot answer:
the validated path values, the request body, the per-tool validators, the
dependency walks, and the error-taxonomy verb. Everything mechanical comes from
the tool's own proto options, read through linodemcp.linode.routes: the route,
the response message, the confirm prose, the success prose, the drift
hash-ignore key, and the retry policy. Go collapsed the same repetition into
config objects long ago (helpers.go's proto-list factories, destroy.go's
DestructiveAction family); these are the missing Python half.

Two ordering rules are the reason these are drivers rather than helpers, since
both are invisible in a handler that gets them wrong:

- A dry run validates before it gates on confirm; a live call gates on confirm
  before it validates. A live call must not disclose that its arguments would
  have been accepted before the caller has confirmed.
- A destroy runs its two-stage plan/apply branch ahead of both, because a plan
  is neither a preview nor an execution and must be reachable without confirm.

Bespoke work stays with the caller behind named seams, the same shape Go's
DestructiveAction takes: validators, nested-object flattening, side-effect
prose, and dependency walks are per-tool judgments no descriptor carries.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from enum import Enum
from typing import TYPE_CHECKING, Any, cast

from google.protobuf.descriptor import FieldDescriptor

from linodemcp.linode.routes import (
    contract_for,
    message_class,
    response_descriptor,
    route_for,
)
from linodemcp.profiles import Capability
from linodemcp.tools.helpers import (
    STANDARD_PAGE_SIZE_MAX,
    STANDARD_PAGE_SIZE_MIN,
    DryRunDetails,
    build_dry_run_response,
    error_response,
    execute_dry_run,
    execute_tool,
    is_dry_run,
    pagination_int_argument,
    pagination_query,
)
from linodemcp.tools.proto_response import (
    serialize_api_response,
    serialize_list_response,
)
from linodemcp.tools.twostage_destroy import run_two_stage_destroy
from linodemcp.twostage.hash_ignore import hash_ignore_fields

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable, Mapping, Sequence

    from google.protobuf.descriptor import Descriptor
    from google.protobuf.message import Message
    from mcp.types import TextContent

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

# The envelope field every mutation response carries alongside its payload.
_MESSAGE_FIELD = "message"

# The placeholder a failure template leaves for the failure itself. It is the
# one name that is not an input field, so it survives the fill below and
# execute_tool puts the exception in it.
_ERROR_PLACEHOLDER = "error"

# The shared sentence a collection answers a failed fetch with. error_message is
# deliberately unset across the list surface, since naming the tool a page came
# from adds nothing a caller does not already know. Go's list machinery has
# answered exactly this for every list tool since before the option existed.
_LIST_FAILURE = "Failed to retrieve items: {error}"

# The placeholder syntax success_message uses, repeating the pattern
# scripts/verify_tool_response.py reads a template with rather than importing
# it: that gate is offline tooling and this runs in the server, so the two
# agreeing is what makes the gate's verdict mean something here. Anything the
# pattern does not match is literal text.
_PLACEHOLDER = re.compile(r"\{([a-z0-9_]+)\}")


class DriverError(ValueError):
    """A tool's contract cannot drive the tier it was handed to.

    Every case is a defect in the contract or in the call site, never a failed
    Linode call, so it inherits ValueError for the same reason RouteError does:
    that is the class the handler helpers report as an argument error and the
    retry layer declines to replay.
    """


class MatchMode(Enum):
    """How a client-side list filter compares its argument to an item field.

    The three modes are the ones Go's containsFilter, fieldFilter, and
    boolFilter implement, so a filter declared once reads the same on both
    sides. Case folding applies to both text modes because the Linode API
    returns record types uppercase and domain types lowercase.
    """

    CONTAINS = "contains"
    EQUALS = "equals"
    BOOL = "bool"


@dataclass(frozen=True)
class ListFilter:
    """One client-side filter: which argument drives it and which field it reads.

    These routes accept no equivalent query parameter, so the page comes back
    whole and the narrowing happens here. That is also why applied filters are
    echoed in the response: a count of 3 out of a 25-item page is otherwise
    indistinguishable from an account with 3 domains.
    """

    argument: str
    field: str
    mode: MatchMode = MatchMode.CONTAINS


@dataclass(frozen=True)
class PageBounds:
    """The page_size range a list route accepts.

    Most collections take the standard 25 to 500. Families that publish their
    own range pass it here rather than reimplementing the argument reader,
    which keeps the range and its error text in one place.
    """

    page_size_min: int
    page_size_max: int | None = None


# The bounds every collection without a published range of its own uses.
STANDARD_PAGE_BOUNDS = PageBounds(STANDARD_PAGE_SIZE_MIN, STANDARD_PAGE_SIZE_MAX)


def _repeated_field(descriptor: Descriptor) -> FieldDescriptor:
    """The one repeated field of a list-response message.

    A list envelope is {count, filter?, <items>}, so its single repeated field
    is the item member, and its name is the key the serializer wraps the page
    under. None and more than one both raise instead of guessing, because a
    wrong key would not fail: serialize_api_response ignores unknown fields, so
    the tool would answer with a well-formed envelope holding an empty list.
    """
    repeated = [field for field in descriptor.fields if field.is_repeated]
    if len(repeated) != 1:
        names = ", ".join(field.name for field in repeated) or "none"
        msg = (
            f"{descriptor.full_name} has {len(repeated)} repeated fields ({names}),"
            " so its list envelope key is ambiguous"
        )
        raise DriverError(msg)
    return repeated[0]


def _new_response(tool: str) -> Message:
    """A fresh instance of the message a tool answers with."""
    return message_class(contract_for(tool).response)()


def _path_values(
    tool: str, values: Mapping[str, object], *, allow_extra: bool = False
) -> tuple[object, ...]:
    """Order path values the way the route template names them.

    Callers pass their validated ids by name, so neither the call site nor a
    generator has to know the order the template happens to use, and a renamed
    or reordered path parameter fails here rather than addressing the wrong
    resource.

    allow_extra is for the destroy tier, whose id map feeds the response echo
    as well as the route: the SSL-certificate delete is addressed by region and
    label but answers with the bucket too. Everywhere else the map has one
    consumer, and an unused name is a mistake worth reporting.
    """
    route = route_for(tool)
    missing = [slot for slot in route.slots if slot not in values]
    if missing:
        msg = f"route {tool} needs path values for {', '.join(missing)}"
        raise DriverError(msg)
    extra = sorted(set(values) - set(route.slots))
    if extra and not allow_extra:
        msg = f"route {tool} has no path slot named {', '.join(extra)}"
        raise DriverError(msg)
    return tuple(values[slot] for slot in route.slots)


def _matches(item: Mapping[str, Any], spec: ListFilter, value: object) -> bool:
    """Whether one decoded item passes one filter."""
    if spec.mode is MatchMode.BOOL:
        return bool(item.get(spec.field)) is (str(value).lower() == "true")

    field_text = str(item.get(spec.field, "")).lower()
    wanted = str(value).lower()
    if spec.mode is MatchMode.CONTAINS:
        return wanted in field_text
    return field_text == wanted


def _applied_filters(
    arguments: Mapping[str, Any], filters: Sequence[ListFilter]
) -> list[tuple[ListFilter, object]]:
    """The filters this call actually asked for, in declaration order.

    An absent or empty argument means the caller did not ask, so it neither
    narrows the page nor appears in the echo. Declaration order fixes the
    echo's wording, so a filter list is ordered as deliberately as it is
    spelled.
    """
    asked: list[tuple[ListFilter, object]] = []
    for spec in filters:
        value = arguments.get(spec.argument)
        if value:
            asked.append((spec, value))
    return asked


def _failure_text(
    tool: str, arguments: Mapping[str, Any], values: Mapping[str, object]
) -> str:
    """The sentence a failed Linode call answers with, from the contract.

    The placeholders name fields of the tool's own input, filled from the
    validated path values first and the raw arguments after, so a template
    reporting an id reports the number the request was addressed with rather
    than whatever text arrived. {error} is left in place for execute_tool. An
    unresolvable name raises rather than rendering the brace text, and both
    emitters refuse one at generation time.
    """
    template = contract_for(tool).error_message
    if not template:
        msg = f"tool {tool} declares no error_message to report a failure with"
        raise DriverError(msg)

    return _PLACEHOLDER.sub(
        lambda match: _failure_value(tool, match.group(1), arguments, values), template
    )


def _failure_value(
    tool: str, name: str, arguments: Mapping[str, Any], values: Mapping[str, object]
) -> str:
    """One failure-template placeholder's text."""
    if name == _ERROR_PLACEHOLDER:
        return "{" + _ERROR_PLACEHOLDER + "}"
    if name in values:
        return str(values[name])
    if name in arguments:
        return str(arguments[name])
    msg = f"error_message for {tool} fills {name} from no path value and no argument"
    raise DriverError(msg)


async def run_get_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    path_values: Mapping[str, object] | None = None,
) -> list[TextContent]:
    """Fetch one resource and answer with the message the contract names.

    The whole call is the contract: the route addresses the resource, the
    response message shapes the answer, and error_message words the failure.
    Nothing is left for a caller to pass except the path values it validated,
    which is why this driver takes no prose where the list one accepts a verb.
    """
    values = _path_values(tool, path_values or {})
    failure = _failure_text(tool, arguments, path_values or {})

    async def call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.route_raw(tool, *values)
        return serialize_api_response(raw, _new_response(tool))

    return await execute_tool(cfg, arguments, "", call, failure=failure)


async def run_list_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    error_action: str = "",
    path_values: Mapping[str, object] | None = None,
    filters: Sequence[ListFilter] = (),
    pagination: PageBounds | None = STANDARD_PAGE_BOUNDS,
) -> list[TextContent]:
    """Fetch one page of a collection, narrow it, and answer in the list envelope.

    The envelope key comes from the response message's single repeated field.
    path_values names the caller's already-validated ids; validating them stays
    with the caller because the required-argument text is per-family prose no
    descriptor holds.

    error_action is the verb a handler that predates the contract reports its
    failure with. Left out, the shared list sentence answers instead, which is
    what the contract asks for by leaving error_message unset here.

    pagination is the page_size range the route publishes, or None for a route
    that returns its whole collection. Range failures answer before a client is
    opened, which keeps a bad page number off the wire.
    """
    asked = _applied_filters(arguments, filters)
    echo = ", ".join(f"{spec.argument}={value}" for spec, value in asked) or None

    query = ""
    if pagination is not None:
        try:
            page = pagination_int_argument(arguments, "page", 1)
            page_size = pagination_int_argument(
                arguments,
                "page_size",
                pagination.page_size_min,
                pagination.page_size_max,
            )
        except (TypeError, ValueError) as exc:
            return error_response(str(exc))
        query = pagination_query(page, page_size)

    key = _repeated_field(response_descriptor(tool)).name
    values = _path_values(tool, path_values or {})

    def keep(item: dict[str, Any]) -> bool:
        return all(_matches(item, spec, value) for spec, value in asked)

    async def call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.route_raw(tool, *values, query=query)
        return serialize_list_response(
            raw,
            key,
            _new_response(tool),
            filter_value=echo,
            item_filter=keep if asked else None,
        )

    return await execute_tool(
        cfg,
        arguments,
        error_action,
        call,
        failure="" if error_action else _LIST_FAILURE,
    )


def _payload_field(tool: str) -> FieldDescriptor:
    """The field of a write response that carries the API object.

    A mutation answers with {message, <object>}, so the one message-typed field
    beside the message is where the decoded response goes. Any other shape
    raises rather than silently dropping the object, since
    serialize_api_response ignores what it cannot place.
    """
    descriptor = response_descriptor(tool)
    payloads = [
        field
        for field in descriptor.fields
        if field.type == FieldDescriptor.TYPE_MESSAGE
    ]
    if len(payloads) != 1 or _MESSAGE_FIELD not in descriptor.fields_by_name:
        msg = (
            f"{descriptor.full_name} is not a write envelope: it needs a"
            f" {_MESSAGE_FIELD} field and exactly one message field, and has"
            f" {len(payloads)} message field(s)"
        )
        raise DriverError(msg)
    return payloads[0]


def _field_text(payload: Mapping[str, Any], field: FieldDescriptor) -> str:
    """Render one response field the way a hand-written message read it.

    The handlers this replaces read their interpolated values through raw_str
    and raw_int, which answer the field's zero value when the API omitted it or
    sent another type. Rendering the proto zero here keeps that: a create whose
    response carries no id still reports "(ID: 0)", not "(ID: None)", and the
    behavior fixtures pin that exact text.
    """
    value = payload.get(field.name)
    if not field.is_repeated:
        if field.type == FieldDescriptor.TYPE_STRING:
            return value if isinstance(value, str) else ""
        if field.cpp_type in (
            FieldDescriptor.CPPTYPE_INT32,
            FieldDescriptor.CPPTYPE_INT64,
            FieldDescriptor.CPPTYPE_UINT32,
            FieldDescriptor.CPPTYPE_UINT64,
        ):
            return (
                str(value)
                if isinstance(value, int) and not isinstance(value, bool)
                else "0"
            )
    # A list, a bool, and a float each render differently in the two languages
    # (["a"] against [a], True against true, 1.0 against 1), so a template that
    # names one is refused rather than rendered differently on each side.
    msg = (
        f"success_message cannot fill {field.name}: only single string and"
        f" integer fields have a rendering both languages agree on"
    )
    raise DriverError(msg)


def _fill(
    template: str,
    arguments: Mapping[str, Any],
    payload: Mapping[str, Any],
    payload_descriptor: Descriptor | None,
) -> str:
    """Render a success_message against the response and the call's arguments.

    The response wins a name both carry, because that is the value the API
    settled on: a create echoes the label it actually stored, which can differ
    from the one asked for. Names the response does not carry fall back to the
    arguments, which is how an update reports the path id it was given. An
    unresolvable placeholder raises, since `make tool-response` already rejects
    one that names no field of either message.
    """
    return _PLACEHOLDER.sub(
        lambda match: _resolve(match.group(1), arguments, payload, payload_descriptor),
        template,
    )


def _resolve(
    name: str,
    arguments: Mapping[str, Any],
    payload: Mapping[str, Any],
    payload_descriptor: Descriptor | None,
) -> str:
    """One placeholder's text, from the response first and the arguments after."""
    if payload_descriptor is not None:
        field = payload_descriptor.fields_by_name.get(name)
        if field is not None:
            return _field_text(payload, field)
    if name in arguments:
        return str(arguments[name])
    msg = f"success_message fills {name} from no response field and no argument"
    raise DriverError(msg)


def _success_text(
    tool: str,
    arguments: Mapping[str, Any],
    payload: Mapping[str, Any],
    payload_descriptor: Descriptor | None,
    override: str | None,
) -> str:
    """The prose a completed mutation reports.

    override exists for the tools whose message interpolates something no field
    carries, which the contract leaves undeclared rather than guessing at.
    Every one of them is a tool `make tool-response` already lists as
    undeclared.
    """
    template = contract_for(tool).success_message
    if template:
        return _fill(template, arguments, payload, payload_descriptor)
    if override is not None:
        return override
    msg = f"tool {tool} declares no success_message and none was supplied"
    raise DriverError(msg)


async def _route_write(
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any],
) -> Any:
    """Make the mutation call under the contract's retry policy.

    retry_disabled marks a route that must not be replayed: the API assigns the
    new resource's id, so a retry after a timeout leaves a duplicate the caller
    never learns about and is billed for. The retrying case names no policy
    because route_raw already defaults to retrying.
    """
    if contract_for(tool).retry_disabled:
        return await client.route_raw(tool, *values, body=body, retry=False)
    return await client.route_raw(tool, *values, body=body)


async def _routed_write(
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any],
    invalid_response_subject: str,
) -> Any:
    """Make the mutation call and, when asked, hold its response to an object.

    invalid_response_subject, when set, rejects a body that is not a JSON
    object. Every write wants that check; only the tools whose fixtures pin the
    wording carry it, so migrating one does not reword another's error.
    """
    if not invalid_response_subject:
        return await _route_write(client, tool, values, body)

    try:
        raw = await _route_write(client, tool, values, body)
    except ValueError as exc:
        msg = f"{invalid_response_subject} response is invalid: {exc}"
        raise ValueError(msg) from exc
    if not isinstance(raw, dict):
        msg = f"{invalid_response_subject} response must be a JSON object"
        raise TypeError(msg)
    return cast("dict[str, Any]", raw)


async def run_write_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    error_action: str,
    body: dict[str, Any],
    path_values: Mapping[str, object] | None = None,
    error: str | None = None,
    preview_error: str | None = None,
    side_effects: Sequence[str] = (),
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None = None,
    walk: Callable[[RetryableClient, Any], Awaitable[DryRunDetails]] | None = None,
    preview_request_body: bool = True,
    success_message: str | None = None,
    invalid_response_subject: str = "",
) -> list[TextContent]:
    """Run a create or update: preview it, gate it, call it, report it.

    error and preview_error carry the caller's validation as text because when
    each one surfaces is the ordering rule this driver exists to hold: a
    preview reports preview_error before it fetches anything, and a live call
    reports error only after the confirm gate. They are separate because a
    family can deliberately check less before a preview than before a live
    call, and both languages have to check the same amount. The update family
    is the case: it holds a preview to the path id alone and validates the body
    after confirm on both sides, while a create passes one verdict to both.

    state_fetch turns the preview into the fetched-state variant, where the
    walk compares the request against what is there now. Without it the preview
    reports the request alone, the right shape for a create.

    preview_request_body is False for the updates whose preview predates the
    body echo and whose fixtures pin the shorter shape.
    """
    route = route_for(tool)
    values = _path_values(tool, path_values or {})
    endpoint = route.endpoint(*values)
    preview_body = body if preview_request_body else None

    if is_dry_run(arguments):
        if preview_error is not None:
            return error_response(preview_error)
        if state_fetch is None:
            return build_dry_run_response(
                tool,
                arguments.get("environment", ""),
                route.method,
                endpoint,
                None,
                request_body=preview_body,
                side_effects=list(side_effects) or None,
            )
        return await execute_dry_run(
            cfg,
            arguments,
            tool,
            route.method,
            endpoint,
            state_fetch,
            walk,
            request_body=preview_body,
        )

    if arguments.get("confirm") is not True:
        return error_response(contract_for(tool).confirm_message)

    if error is not None:
        return error_response(error)

    payload_field = _payload_field(tool)
    # Rendered once against an empty response before anything is sent: a
    # placeholder naming nothing fillable is a contract defect, and finding it
    # after the create has run would report a failure over a resource that now
    # exists.
    _success_text(tool, arguments, {}, payload_field.message_type, success_message)

    async def call(client: RetryableClient) -> dict[str, Any]:
        raw = await _routed_write(client, tool, values, body, invalid_response_subject)
        payload = cast("dict[str, Any]", raw) if isinstance(raw, dict) else {}
        text = _success_text(
            tool, arguments, payload, payload_field.message_type, success_message
        )
        return serialize_api_response(
            {_MESSAGE_FIELD: text, payload_field.name: raw}, _new_response(tool)
        )

    return await execute_tool(cfg, arguments, error_action, call)


def _echo_values(tool: str, id_args: Mapping[str, str | int]) -> dict[str, Any]:
    """The id echo a delete response carries, filled from the call's ids.

    A DELETE answers with an empty body, so the envelope is built rather than
    decoded: the message plus every other scalar the response declares, each
    named after the path argument it echoes. That naming is what lets the echo
    be filled without a per-tool mapping.

    It is resolved before any branch runs, so a response naming a value the
    call was never given is refused while the resource still exists. Building
    it after the DELETE would report the failure over a deletion that already
    happened.
    """
    descriptor = response_descriptor(tool)
    if _MESSAGE_FIELD not in descriptor.fields_by_name:
        msg = f"{descriptor.full_name} has no {_MESSAGE_FIELD} field to report under"
        raise DriverError(msg)

    echo: dict[str, Any] = {}
    for field in descriptor.fields:
        if field.name == _MESSAGE_FIELD or field.type == FieldDescriptor.TYPE_MESSAGE:
            continue
        if field.name not in id_args:
            msg = (
                f"{tool} answers with {field.name}, which names no path id it was given"
            )
            raise DriverError(msg)
        echo[field.name] = id_args[field.name]
    return echo


async def run_destructive_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    error_action: str,
    id_args: Mapping[str, str | int],
    fetch_state: Callable[[RetryableClient], Awaitable[Any]],
    execute: Callable[[RetryableClient], Awaitable[None]],
    dependency_walk: Callable[[RetryableClient, Any], Awaitable[DryRunDetails]]
    | None = None,
    error: str | None = None,
    success_message: str | None = None,
    capability: Capability = Capability.Destroy,
) -> list[TextContent]:
    """Run a delete: plan or apply it, preview it, gate it, execute it, report it.

    The three branches run in the order a caller can reach them: a two-stage
    plan or apply is neither a preview nor an execution and answers first, then
    the preview, then the confirm gate. The bypass-dry-run gate that sits in
    front of all of this is the server's, at dispatch, and stays there: it
    applies to every destroy tool whether or not the tool routes through here.

    id_args names the caller's validated path ids. They fill the route, the
    response echo, and any success_message placeholder, so the id is written
    once at the call site and never again. A value is a string or an integer,
    the pair a path slot takes, since a resource can be addressed by label as
    readily as by number.

    error follows run_write_tool's rule and this tier's own history: the delete
    handlers report a missing id before the plan and preview branches and after
    the confirm gate, and the behavior fixtures pin that asymmetry.
    """
    route = route_for(tool)
    contract = contract_for(tool)
    values = _path_values(tool, id_args, allow_extra=True)
    endpoint = route.endpoint(*values)
    echo = _echo_values(tool, id_args)
    # A DELETE answers with an empty body, so the message reads only the ids
    # and can be built before the call rather than after it. That ordering is
    # what keeps a mis-declared message from surfacing over a resource that has
    # already been removed.
    text = _success_text(tool, id_args, {}, None, success_message)

    def report() -> dict[str, Any]:
        return serialize_api_response(
            {_MESSAGE_FIELD: text, **echo}, _new_response(tool)
        )

    async def execute_and_report(client: RetryableClient) -> dict[str, Any]:
        await execute(client)
        return report()

    staging = arguments.get("mode") in ("plan", "apply")
    previewing = is_dry_run(arguments)
    if error is not None and (staging or previewing):
        return error_response(error)

    if staging:
        staged = await run_two_stage_destroy(
            cfg,
            arguments,
            tool_name=tool,
            method=route.method,
            path=endpoint,
            fetch_state=fetch_state,
            execute=execute_and_report,
            hash_ignore=hash_ignore_fields(contract.resource_type),
            dependency_walk=dependency_walk,
            capability=capability,
        )
        if staged is not None:
            return staged

    if previewing:
        return await execute_dry_run(
            cfg, arguments, tool, route.method, endpoint, fetch_state, dependency_walk
        )

    if arguments.get("confirm") is not True:
        return error_response(contract.confirm_message)

    if error is not None:
        return error_response(error)

    return await execute_tool(cfg, arguments, error_action, execute_and_report)
