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
from typing import TYPE_CHECKING, Any, NamedTuple, cast
from urllib.parse import urlencode

from google.protobuf import json_format
from google.protobuf.descriptor import FieldDescriptor

from linodemcp.linode.routes import (
    ElementLift,
    ListEnvelope,
    ListShape,
    RouteError,
    contract_for,
    echo_argument,
    list_envelope_for,
    message_class,
    response_descriptor,
    route_for,
)
from linodemcp.profiles import Capability
from linodemcp.tools.declared_state import DeclaredState, project_declared_state
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
from linodemcp.tools.preview import preview_reported_lines
from linodemcp.tools.proto_response import (
    restore_explicit_nulls,
    serialize_api_response,
    serialize_list_response,
    serialize_struct_response,
)
from linodemcp.tools.transport import (
    PresignPreview,
    PresignTransfer,
    TransportSpec,
    preview_presign_source,
    run_transport,
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
    from linodemcp.tools.preview import PreviewLines

# The envelope field every mutation response carries alongside its payload.
_MESSAGE_FIELD = "message"

# The member a declared warning_message is reported in, for the mutations whose
# answer carries a notice beside the prose: a secret shown once and never again.
_WARNING_FIELD = "warning"

# Names a message the tools own rather than one the API sends, which is what
# tells a prose-less envelope from the API resource itself. Shape cannot: a
# resource holds nested objects too. go/cmd/toolgen binds the same suffix.
_ENVELOPE_SUFFIX = "Response"

# The single member a stood-in preview value carries, so a reader can tell a
# withheld value from an empty one. go/internal/tools binds the same name.
_REDACTED_KEY = "redacted"

# The payload message of a route with no schema to model, which is the one shape
# a JSON null answer has an exact reading in.
_STRUCT_MESSAGE = "google.protobuf.Struct"

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
_PLACEHOLDER = re.compile(
    r"\{(?P<name>[a-z0-9_]+)(?::(?:0(?P<width>\d+)|(?P<count>len)))?\}"
)


class DriverError(ValueError):
    """A tool's contract cannot drive the tier it was handed to.

    Every case is a defect in the contract or in the call site, never a failed
    Linode call, so it inherits ValueError for the same reason RouteError does:
    that is the class the handler helpers report as an argument error and the
    retry layer declines to replay.
    """


class MatchMode(Enum):
    """How a client-side list filter compares its argument to an item field.

    The four modes are the ones Go's containsFilter, fieldFilter, boolFilter,
    and FilterByMember implement, so a filter declared once reads the same on
    both sides. Case folding applies to every text mode because the Linode API
    returns record types uppercase and domain types lowercase.
    """

    CONTAINS = "contains"
    EQUALS = "equals"
    BOOL = "bool"
    MEMBER = "member"


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
class PreviewStandIn:
    """One member of a repeated argument's entries a preview reports text for.

    ``redact_preview`` stands in for a whole member and would take the rest of
    each entry with it. The security answers are declared this way because the
    question ids beside them are what a caller checks before confirming.
    ``WriteBody.StandingIn`` in go/internal/tools answers the same shape.
    """

    argument: str
    member: str
    text: str


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

# The page a synthesized collection fetch reads. Go's collectionStatePage is the
# same one for the same reason: one call at the standard maximum is what the
# route hands over at once.
_COLLECTION_STATE_PAGE = 1


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


def _route_subject(tool: str) -> str:
    """Name a read in the report its answer gets for a body that is not an object.

    A decoder's own complaint cannot say which call answered badly. The prose is
    derived rather than declared, the way the emitter derives a mutation's
    subject from the same tool name, and Go's linode.routeSubject derives it
    identically: linode_networking_reserved_ip_get reads as "networking reserved
    ip get".
    """
    return tool.removeprefix("linode_").replace("_", " ")


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

    wanted = str(value).lower()

    if spec.mode is MatchMode.MEMBER:
        return _member_matches(item.get(spec.field), wanted)

    field_text = str(item.get(spec.field, "")).lower()
    if spec.mode is MatchMode.CONTAINS:
        return wanted in field_text
    return field_text == wanted


def _member_matches(entries: object, wanted: str) -> bool:
    """Whether a list-valued item field carries the value the filter asked for.

    A field the API answered with something other than a list matches nothing
    rather than raising: a filter narrows a page, so an odd item is one that
    does not qualify.
    """
    if not isinstance(entries, list):
        return False

    return any(str(entry).lower() == wanted for entry in cast("list[object]", entries))


def _applied_filters(
    arguments: Mapping[str, Any], filters: Sequence[ListFilter]
) -> list[tuple[ListFilter, object]]:
    """The filters this call actually asked for, in declaration order.

    An absent or empty argument means the caller did not ask, so it neither
    narrows the page nor appears in the echo. Declaration order fixes the
    echo's wording, so a filter list is ordered as deliberately as it is
    spelled.

    Go reads every filter, bool ones included, with request.GetString, so a
    wrong-typed argument reads as unasked there. Stringifying one here narrowed
    the page to nothing where Go answered it whole.
    """
    asked: list[tuple[ListFilter, object]] = []
    for spec in filters:
        value = arguments.get(spec.argument)
        if isinstance(value, str) and value:
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
    emitter refuses one at generation time.
    """
    template = contract_for(tool).error_message
    if not template:
        msg = f"tool {tool} declares no error_message to report a failure with"
        raise DriverError(msg)

    return _PLACEHOLDER.sub(
        lambda match: _padded(
            _failure_value(
                tool,
                match.group("name"),
                bool(match.group("count")),
                arguments,
                values,
            ),
            match.group("width"),
        ),
        template,
    )


def _padded(text: str, width: str | None) -> str:
    """One placeholder's text, zero-padded when the template asked for a width.

    A month reads as 03 rather than 3, which is what a date looks like. Padding
    applies to digits only, since padding text with zeros would be a different
    value rather than the same one written out.
    """
    if not width or not text.isdigit():
        return text

    return text.zfill(int(width))


def _failure_value(
    tool: str,
    name: str,
    count: bool,
    arguments: Mapping[str, Any],
    values: Mapping[str, object],
) -> str:
    """One failure-template placeholder's text."""
    if name == _ERROR_PLACEHOLDER:
        return "{" + _ERROR_PLACEHOLDER + "}"
    if count:
        return _count_text(name, arguments)
    if name in values:
        return str(values[name])
    if name in arguments:
        return str(arguments[name])
    msg = f"error_message for {tool} fills {name} from no path value and no argument"
    raise DriverError(msg)


async def _gated_read_opening(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    path_values: Mapping[str, object],
    error: str | None,
    preview_error: str | None,
    side_effects: PreviewLines,
    warnings: PreviewLines,
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None,
) -> list[TextContent] | None:
    """The two branches a gated read opens with, or None to make the call.

    The preview a dry run answers, and the gate the fetch waits behind, in the
    order the write tier holds them in and for the same reason. The route is
    resolved inside the preview branch rather than up front, because an id the
    route cannot build a path out of is the caller's to hear about in the
    tool's own words rather than as a RouteError.
    """
    if is_dry_run(arguments):
        if preview_error is not None:
            return error_response(preview_error)

        route = route_for(tool)
        endpoint = route.preview_endpoint(*_path_values(tool, path_values))

        # A read sends nothing, so the preview reports no request body.
        return await _derived_preview(
            cfg,
            arguments,
            tool=tool,
            method=route.method,
            endpoint=endpoint,
            body=None,
            side_effects=side_effects,
            warnings=warnings,
            state_fetch=state_fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(contract_for(tool).confirm_message)

    if error is not None:
        return error_response(error)

    return None


async def run_assembled_read_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    transport: TransportSpec,
    assembled: tuple[str, ...],
    echo_members: Mapping[str, object] | None = None,
    path_values: Mapping[str, object] | None = None,
) -> list[TextContent]:
    """Read through a route whose answer is not JSON.

    The OAuth client thumbnail arrives as raw PNG bytes, so there is nothing to
    decode: the declared transport owns the transfer and the encoding, and what
    it brings back fills the members the response declares as assembled. Every
    other member is placed here from the call, which is why the answer is built
    rather than parsed.

    echo_members is keyed by the response member rather than by the argument,
    since nothing maps one to the other here: the caller resolved the alias.
    """
    values = _path_values(tool, path_values or {})
    failure = _failure_text(tool, arguments, path_values or {})

    async def call(client: RetryableClient) -> dict[str, Any]:
        moved = await run_transport(transport, client, tool, arguments, values, None)
        filled = _assembled_values(tool, assembled, moved)
        return serialize_api_response(
            {**(echo_members or {}), **filled}, _new_response(tool)
        )

    return await execute_tool(cfg, arguments, "", call, failure=failure)


def _assembled_values(
    tool: str, assembled: tuple[str, ...], answered: Mapping[str, Any]
) -> dict[str, Any]:
    """Hold a transport's answer to the members the contract declares.

    Go's emitter reads these members off a typed response message, so its
    compiler refuses a name the response does not carry and leaves an unset one
    at its zero. Nothing types a dict, so the same two mistakes are caught here:
    a member the transfer left unfilled would serialize as a zero the caller
    reads as real, and one it invented would be dropped without a word.
    """
    missing = sorted(name for name in assembled if name not in answered)
    unknown = sorted(name for name in answered if name not in assembled)

    if missing or unknown:
        msg = (
            f"{tool} transport answered members {sorted(answered)},"
            f" want exactly {sorted(assembled)}"
        )
        raise ValueError(msg)

    return dict(answered)


async def run_get_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    path_values: Mapping[str, object] | None = None,
    query: str = "",
    wrapper_field: str = "",
    struct_response: bool = False,
    gated: bool = False,
    error: str | None = None,
    preview_error: str | None = None,
    side_effects: PreviewLines = (),
    warnings: PreviewLines = (),
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None = None,
) -> list[TextContent]:
    """Fetch one resource and answer with the message the contract names.

    The whole call is the contract: the route addresses the resource, the
    response message shapes the answer, and error_message words the failure.
    Nothing is left for a caller to pass except the path values it validated.

    query carries the parameters a single-resource route publishes, already
    encoded: the page controls on /databases/types/{id}, the object key on the
    bucket ACL route. wrapper_field names the member a tool's own response
    envelope decodes the API body into, for the tools that have always answered
    under a key of their own rather than with the body itself.

    struct_response is the handful of reads whose API body has no proto model,
    where the decoded object is the answer and there is nothing to place it in.

    gated marks a read the caller can preview and must confirm, which the
    credential reads are: the answer is the secret, so seeing the call has to
    be reachable without making it. Its error and preview_error carry the
    handler's validation as text for the same reason the write tier passes
    them: a preview reports its verdict before anything else, and a live call
    reports it only after the confirm gate, so a caller who has not confirmed
    learns nothing about whether their arguments would have been accepted. An
    ungated read passes none of the three and answers its own arguments where
    it reads them.
    """
    if gated:
        answered = await _gated_read_opening(
            cfg,
            arguments,
            tool=tool,
            path_values=path_values or {},
            error=error,
            preview_error=preview_error,
            side_effects=side_effects,
            warnings=warnings,
            state_fetch=state_fetch,
        )
        if answered is not None:
            return answered

    values = _path_values(tool, path_values or {})
    failure = _failure_text(tool, arguments, path_values or {})
    nulls = contract_for(tool).explicit_null_fields

    async def call(client: RetryableClient) -> dict[str, Any]:
        # Named only where there is one: nearly every read carries no query,
        # and a blank one would put an argument in the call the route never
        # sees.
        if query:
            raw = await client.route_raw(tool, *values, query=query)
        else:
            raw = await client.route_raw(tool, *values)
        # A free-form read decodes a JSON null into an empty object, which is
        # what Go's map decode answers; a bare array or scalar is still refused.
        if struct_response and raw is None:
            raw = {}
        # ParseDict reads a bare array, string or number as an empty message and
        # reports success carrying no data, so the malformed body is refused
        # here. Go's client refuses the same body with the same sentence, which
        # is what lets one fixture cover both.
        if not isinstance(raw, dict):
            msg = f"{_route_subject(tool)} response must be a JSON object"
            raise TypeError(msg)
        body = cast("dict[str, Any]", raw)
        if struct_response:
            return serialize_struct_response(body)
        if wrapper_field:
            body = {wrapper_field: body}
        message = _new_response(tool)
        serialized = serialize_api_response(body, message)
        # A read's answer IS the body it decoded, so the declared nulls go back
        # onto the response itself: there is no envelope assembled around it for
        # them to land in. go/cmd/toolgen emits the same restoration.
        return restore_explicit_nulls(body, serialized, nulls, message.DESCRIPTOR)

    return await execute_tool(cfg, arguments, "", call, failure=failure)


async def run_body_read_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    body: dict[str, Any],
    echo_args: Mapping[str, str | int] | None = None,
    path_values: Mapping[str, object] | None = None,
    transport: TransportSpec | None = None,
    assembled: tuple[str, ...] = (),
) -> list[TextContent]:
    """Read through a route that takes a request body.

    The presigned-URL create signs what its body describes and the metric query
    asks for a window of samples; neither stores anything, so both are reads
    with a request rather than mutations. There is no gate and no preview around
    either, since a caller can ask for the same answer again, and the handler
    answers its own argument failures where it reads them.

    The answer is the resource the call decodes into, or the envelope carrying
    that resource beside the prose and the ids the read echoes back. A body that
    is not a JSON object is refused here for the reason run_get_tool refuses
    one: ParseDict reads a bare array, string or number as an empty message and
    reports success carrying no data, where Go's decode of the same body fails.

    transport makes the live call in place of the routed request, for a route
    whose answer no decode can place: the Object Storage download asks for a
    presigned URL and then follows it, so what the caller reads is the
    transfer's result rather than any member of the API's reply. The members it
    fills are named by assembled and held to that list, and the sentence the
    answer carries is the success_message the contract declares.
    """
    values = _path_values(tool, path_values or {})
    failure = _failure_text(tool, arguments, path_values or {})
    # An assembled answer has no decoded resource to place, so there is no
    # payload field to resolve and asking for one refuses the shape outright.
    payload_field = None if transport is not None else resolve_payload_field(tool)

    def members(payload: Mapping[str, Any]) -> dict[str, str]:
        return _write_members(
            tool,
            arguments,
            payload,
            payload_descriptor=None
            if payload_field is None
            else payload_field.message_type,
            carries_message=True,
            success_message=None,
        )

    if payload_field is not None:
        # Resolved against an empty answer before the call goes out, the way the
        # write tier resolves its own: a sentence naming nothing fillable is a
        # contract defect, and finding it afterwards would report a failure over
        # an answer already in hand.
        members({})

    echo = _echo_values(tool, echo_args) if echo_args else {}

    async def call(client: RetryableClient) -> dict[str, Any]:
        if transport is not None:
            filled = _assembled_values(
                tool,
                assembled,
                await run_transport(transport, client, tool, arguments, values, body),
            )
            return serialize_api_response(
                {
                    _MESSAGE_FIELD: _success_text(tool, arguments, {}, None, None),
                    **echo,
                    **filled,
                },
                _new_response(tool),
            )
        raw = await _route_write(client, tool, values, body)
        if not isinstance(raw, dict):
            msg = f"{_route_subject(tool)} response must be a JSON object"
            raise TypeError(msg)
        payload = cast("dict[str, Any]", raw)
        message = _new_response(tool)
        if payload_field is None:
            return serialize_api_response(payload, message)
        return serialize_api_response(
            {**members(payload), **echo, payload_field.name: payload}, message
        )

    return await execute_tool(cfg, arguments, "", call, failure=failure)


async def run_list_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    error_action: str = "",
    path_values: Mapping[str, object] | None = None,
    filters: Sequence[ListFilter] = (),
    forwarded: Sequence[str] = (),
    echoes: Sequence[str] = (),
    pagination: PageBounds | None = STANDARD_PAGE_BOUNDS,
) -> list[TextContent]:
    """Fetch one page of a collection, narrow it, and answer in the list envelope.

    The envelope key comes from the response message's single repeated field.
    path_values names the caller's already-validated ids; validating them stays
    with the caller because the required-argument text is per-family prose no
    descriptor holds.

    error_action is the verb a handler that predates the contract reports its
    failure with. Left out, the tool's own error_message answers when it
    declares one and the shared list sentence when it does not.

    forwarded names the query arguments the route itself filters on, which the
    request carries rather than this driver applying: an S3 prefix selects the
    objects the API returns, where a country narrows the page already fetched.

    echoes names the forwarded arguments a marker-paged answer reports it was
    narrowed by. The route applied them, so nothing is filtered here; the echo
    still says what was asked for, which is what the page envelope's own filters
    report. The cursor arguments are left out: a position is not a filter.

    pagination is the page_size range the route publishes, or None for a route
    that returns its whole collection. Range failures answer before a client is
    opened, which keeps a bad page number off the wire.
    """
    asked = _applied_filters(arguments, filters)
    echo = _filter_echo(
        [(name, arguments.get(name)) for name in echoes]
        if echoes
        else [(spec.argument, value) for spec, value in asked]
    )

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

    query = with_query_arguments(arguments, query, forwarded)

    element = _repeated_field(response_descriptor(tool))
    key = element.name
    values = _path_values(tool, path_values or {})
    nulls = contract_for(tool).explicit_null_fields

    def keep(item: dict[str, Any]) -> bool:
        return all(_matches(item, spec, value) for spec, value in asked)

    envelope = list_envelope_for(tool)

    async def call(client: RetryableClient) -> dict[str, Any]:
        raw = await client.route_raw(tool, *values, query=query)
        page = _page_of(raw, envelope)
        serialized = serialize_list_response(
            page,
            key,
            _new_response(tool),
            filter_value=echo,
            item_filter=keep if asked else None,
            extras=_marker_cursor(raw) if envelope.shape is ListShape.MARKER else None,
        )
        if not nulls:
            return serialized
        return _restore_page_nulls(page, serialized, key, nulls, element.message_type)

    return await execute_tool(
        cfg,
        arguments,
        error_action,
        call,
        failure=""
        if error_action
        else _list_failure(tool, arguments, path_values or {}),
    )


def _restore_page_nulls(
    page: Any,
    serialized: dict[str, Any],
    key: str,
    names: Sequence[str],
    element: Any,
) -> dict[str, Any]:
    """Write each element's documented nulls back into the serialized page.

    A page's nulls are its elements', never its own: the count and the filter
    echo are assembled from the call, so nothing at the envelope level ever
    comes back null. page is what _page_of normalized, so the bodies sit under
    data whatever shape the route answered with, and they pair with the
    serialized elements by position because a tool restoring nulls declares no
    filter to narrow one list and not the other.

    page and element are typed Any for the reason restore_explicit_nulls types
    its descriptor that way: both come from the C (upb) backend, which pyright
    cannot unify with the pure-Python stubs.
    """
    raw_items = cast("dict[str, Any]", page).get("data")
    if not isinstance(raw_items, list):
        raw_items = []

    serialized[key] = [
        restore_explicit_nulls(raw_item, item, names, element)
        for raw_item, item in zip(
            cast("list[dict[str, Any]]", raw_items),
            cast("list[dict[str, Any]]", serialized[key]),
            strict=True,
        )
    ]
    return serialized


def _filter_echo(applied: Sequence[tuple[str, object]]) -> str | None:
    """The sentence a page reports it was narrowed by.

    An argument the caller left out is skipped, and a page narrowed by nothing
    echoes nothing. Go's echoedFilter joins the same pairs the same way, so one
    fixture covers both clients.
    """
    named = [f"{name}={value}" for name, value in applied if value not in ("", None)]

    return ", ".join(named) or None


def _marker_cursor(raw: Any) -> dict[str, Any]:
    """The cursor a marker-paged body carries beside its elements.

    next_marker travels only when the route handed one out, matching Go's
    TextOrAbsent: an empty cursor is no resume point, and reporting one would
    have a caller ask for a page that does not exist.
    """
    body = _envelope_body(raw)
    cursor: dict[str, Any] = {"is_truncated": bool(body.get("is_truncated", False))}

    if marker := body.get("next_marker"):
        cursor["next_marker"] = marker

    return cursor


def _list_failure(
    tool: str, arguments: Mapping[str, Any], values: Mapping[str, object]
) -> str:
    """The sentence a collection reports a failed fetch with.

    A tool declaring error_message means to say something its family's callers
    recognize, naming the bucket or the region the page came from. Answering
    the shared sentence over that declaration was prose drift no gate saw, so
    the declaration wins wherever there is one.
    """
    if not contract_for(tool).error_message:
        return _LIST_FAILURE

    return _failure_text(tool, arguments, values)


def with_query(endpoint: str, query: str) -> str:
    """The route a preview reports: the endpoint, then the encoded query.

    A preview reporting the bare path over a call that carries page controls
    would describe a different request from the one the live branch makes,
    which is the one thing a dry run cannot afford to do. Go's
    tools.PathWithQuery spells it the same way, so both clients report one
    route.
    """
    if not query:
        return endpoint

    return f"{endpoint}?{query}"


def with_query_arguments(
    arguments: Mapping[str, Any], query: str, names: Sequence[str]
) -> str:
    """Append the parameters the route filters on to an already-encoded query.


    An argument the caller left out is skipped rather than sent empty: an empty
    prefix and no prefix select different objects. A flag travels only when it
    is true, which is how skip_ipv6_rdns has always been sent.
    """
    # Sorted and encoded through urlencode because Go builds the same query
    # with url.Values.Encode, which sorts by name and escapes a slash. Two
    # clients spelling one filter differently is the divergence this whole
    # contract exists to stop.
    pairs = sorted(
        (name, text)
        for name in names
        if (text := _query_argument_text(arguments.get(name, None))) != ""
    )
    if not pairs:
        return query

    encoded = urlencode(pairs)

    return f"{query}&{encoded}" if query else encoded


def state_read_query(arguments: Mapping[str, Any], filled: Mapping[str, str]) -> str:
    """The query a declared fetch addresses its read with.

    Each of that read's own query parameters paired with the argument this tool
    fills it from. The two names are usually the same, and the pairing is what
    lets them differ the way a path slot's mapping does. A parameter whose
    argument the call did not carry is left off, which is what an optional one
    means.
    """
    pairs = sorted(
        (parameter, text)
        for parameter, argument in filled.items()
        if (text := _query_argument_text(arguments.get(argument, None))) != ""
    )

    return urlencode(pairs) if pairs else ""


def _query_argument_text(raw: object) -> str:
    """One forwarded argument as the API reads it, "" when it does not travel."""
    if isinstance(raw, bool):
        return "true" if raw else ""
    if isinstance(raw, str):
        return raw
    if isinstance(raw, (int, float)):
        return str(raw)

    return ""


def _lift_element_members(
    element: dict[str, Any], lifts: Sequence[ElementLift]
) -> dict[str, Any]:
    """Hoist the members a route nests a level down onto the element body.

    Rewriting the body rather than the decoded message is what keeps this one
    step for every element type: the member fills the way the API's own
    top-level members do. A source the body does not carry leaves the member
    alone, so the decode's own answer stands. Go's liftElementMembers hoists the
    same way.
    """
    lifted = dict(element)
    for lift in lifts:
        value = _nested_value(element, lift.source.split("."))
        if value is not _ABSENT:
            lifted[lift.member] = value
    return lifted


# The answer _nested_value gives for a path that leads nowhere, which None
# cannot: a member the API sent as null is a value the lift still hoists.
_ABSENT = object()


def _nested_value(fields: Mapping[str, Any], path: Sequence[str]) -> Any:
    """Walk a dotted path through an element body.

    A member that is not an object holds no path to walk, which reads as absent
    rather than as a failure.
    """
    if path[0] not in fields:
        return _ABSENT
    value = fields[path[0]]
    if len(path) == 1:
        return value
    if not isinstance(value, dict):
        return _ABSENT
    return _nested_value(cast("dict[str, Any]", value), path[1:])


def _page_of(raw: Any, envelope: ListEnvelope) -> Any:
    """Normalize a list body into the {data: [...]} page the serializer reads.

    Only the routes that answer otherwise are reshaped, and each keeps the
    check its shape earns: a bare body is held to being an array here, since a
    null one would otherwise reach the serializer as an empty collection where
    Go's decode of the same body fails; a keyed member is still held to being an
    array of objects downstream; and a page that declared its data member
    required fails here rather than passing a truncated body off as an empty
    collection. Go's fetchRoutedPage splits the same four ways.
    """
    if envelope.shape is ListShape.BARE:
        if not isinstance(raw, list):
            msg = "list response must be an array"
            raise TypeError(msg)
        bare: dict[str, Any] = {"data": raw}
        return bare

    if envelope.shape is ListShape.SINGLETON:
        # The route answers the one element the collection holds. Held to being
        # an object first: a list or a null decoded straight through would
        # reach the serializer as one blank element rather than as the failure
        # it is, which is the check Go's singleton fetcher makes too.
        element = _lift_element_members(_envelope_body(raw), envelope.lift)
        singleton: dict[str, Any] = {"data": [element]}
        return singleton

    if envelope.shape is ListShape.KEYED:
        keyed: dict[str, Any] = {"data": _envelope_body(raw).get(envelope.member, [])}
        return keyed

    # The marker shape needs no reshaping: its elements already sit under data,
    # and the cursor beside them is read separately by _marker_cursor.
    if (
        envelope.shape is ListShape.REQUIRED_DATA
        and _envelope_body(raw).get("data") is None
    ):
        msg = "list response data member is missing or null"
        raise TypeError(msg)

    return raw


def _envelope_body(raw: Any) -> dict[str, Any]:
    """A list body held to being a JSON object before a member is read from it."""
    if not isinstance(raw, dict):
        msg = "list response must be an object"
        raise TypeError(msg)

    return cast("dict[str, Any]", raw)


def _wraps_payload(descriptor: Descriptor, payloads: list[FieldDescriptor]) -> bool:
    """Whether a prose-less response wraps its object rather than being it.

    The name is the signal, the same one go/cmd/toolgen reads: a message
    the tools own ends in Response, and an API resource does not.
    """
    return len(payloads) == 1 and descriptor.full_name.endswith(_ENVELOPE_SUFFIX)


def resolve_payload_field(tool: str) -> FieldDescriptor | None:
    """The field of a write response that carries the API object.

    A mutation answers with {message, <object>}, with {<object>, warning} where
    the API sends no prose to report, or with the object itself, which is the
    None case: nothing to place the body in, so the response decodes into it.
    Any other shape raises rather than silently dropping the object, since
    serialize_api_response ignores what it cannot place.

    A prose-less answer is told apart by name, because shape cannot: the token
    create's ProfileTokenCreateResponse wraps its object, while PlacementGroup
    holds a nested object and is the resource.

    An envelope the API's body fills that carries no resource member is the None
    case too: its scalars are the whole answer, so the body decodes into the
    envelope with nothing to place inside it.
    """
    descriptor = response_descriptor(tool)
    payloads = [
        field
        for field in descriptor.fields
        if field.type == FieldDescriptor.TYPE_MESSAGE and not field.is_repeated
    ]
    if not payloads and contract_for(tool).response_body_fields:
        return None
    if _MESSAGE_FIELD not in descriptor.fields_by_name and not _wraps_payload(
        descriptor, payloads
    ):
        return None
    if len(payloads) != 1:
        msg = (
            f"{descriptor.full_name} is not a write envelope: it needs exactly"
            f" one message field beside its {_MESSAGE_FIELD}, and has"
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
        lambda match: _resolve(
            match.group("name"),
            bool(match.group("count")),
            arguments,
            payload,
            payload_descriptor,
        ),
        template,
    )


def _count_text(name: str, arguments: Mapping[str, Any]) -> str:
    """How many entries a repeated argument carries.

    A JSON array is the one shape both languages count the same way, so a
    caller who sent anything else has sent nothing to count. Go's
    tools.ArgumentLen answers 0 for the same calls.
    """
    entries = arguments.get(name)
    if not isinstance(entries, list):
        return "0"

    return str(len(cast("list[object]", entries)))


def _resolve(
    name: str,
    count: bool,
    arguments: Mapping[str, Any],
    payload: Mapping[str, Any],
    payload_descriptor: Descriptor | None,
) -> str:
    """One placeholder's text, from the response first and the arguments after.

    A count is read off the arguments alone: the entries the caller sent are
    what both languages count the same way, where a decoded response list is
    not.
    """
    if count:
        return _count_text(name, arguments)
    if payload_descriptor is not None:
        field = payload_descriptor.fields_by_name.get(name)
        if field is not None:
            return _field_text(payload, field)
    if name in arguments:
        return _argument_text(arguments[name])
    msg = f"success_message fills {name} from no response field and no argument"
    raise DriverError(msg)


def _argument_text(value: Any) -> str:
    """One argument's text, spelled the way Go's verb prints it.

    A flag is the only value the two languages disagree on: %t writes true and
    false, where str() writes True and False. Checked ahead of everything else
    because a bool is an int here.
    """
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


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


def _warning_member(
    tool: str,
    arguments: Mapping[str, Any],
    payload: Mapping[str, Any],
    payload_descriptor: Descriptor | None,
) -> dict[str, str]:
    """The warning member a mutation's answer carries, empty when it declares none.

    It is filled the way the success sentence beside it is, so a notice can
    report the id or the label the API settled on rather than the one asked
    for. Returned as the member rather than the text so a response with no
    warning field adds nothing to the envelope.
    """
    declared = contract_for(tool)
    if not declared.warning_declared:
        return {}

    return {
        _WARNING_FIELD: _fill(
            declared.warning_message, arguments, payload, payload_descriptor
        )
    }


def _write_members(
    tool: str,
    arguments: Mapping[str, Any],
    payload: Mapping[str, Any],
    *,
    payload_descriptor: Descriptor | None,
    carries_message: bool,
    success_message: str | None,
) -> dict[str, str]:
    """The prose members a mutation's answer carries around its decoded body.

    A response with no message field carries no sentence, which is the token
    create's {warning, token}: the notice is the only member left to fill, and
    asking for a success_message there would report prose the serializer drops.
    """
    warning = _warning_member(tool, arguments, payload, payload_descriptor)
    if not carries_message:
        return warning

    text = _success_text(tool, arguments, payload, payload_descriptor, success_message)

    return {_MESSAGE_FIELD: text, **warning}


async def _route_write(
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any],
    query: str = "",
) -> Any:
    """Make the mutation call under the contract's retry policy.

    retry_disabled marks a route that must not be replayed: the API assigns the
    new resource's id, so a retry after a timeout leaves a duplicate the caller
    never learns about and is billed for. The retrying case names no policy
    because route_raw already defaults to retrying.

    query carries the parameters the route publishes, already encoded, for the
    handful of mutations that take any. A mutation with none names no query at
    all, which is the shape Go keeps as a separate primitive.
    """
    options: dict[str, Any] = {"body": body}

    if query:
        options["query"] = query

    if contract_for(tool).retry_disabled:
        options["retry"] = False

    return await client.route_raw(tool, *values, **options)


async def _routed_delete(
    client: RetryableClient, tool: str, values: tuple[object, ...]
) -> None:
    """Make the removal call under the contract's retry policy.

    The destroy tier's whole request: no body to build and nothing to decode,
    so the tool and its ids are all a call site has left to say. Every DELETE
    the tier serves refuses a replay and so do the rebuilds and recycles, which
    all declare retry_disabled rather than each call site deciding.
    """
    if contract_for(tool).retry_disabled:
        await client.route_call(tool, *values, retry=False)
        return
    await client.route_call(tool, *values)


async def _routed_write(
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any],
    invalid_response_subject: str,
    query: str = "",
    *,
    free_form: bool = False,
) -> Any:
    """Make the mutation call and, when asked, hold its response to an object.

    invalid_response_subject, when set, rejects a body that is not a JSON
    object. Every write wants that check; only the tools whose fixtures pin the
    wording carry it, so migrating one does not reword another's error.

    free_form marks a payload with no schema to model, which is the one shape a
    JSON null has an exact reading in: the API sent no members, which is the
    empty object. On any other response a null would be a guess at what the API
    meant, so it stays refused there. Go's client draws the same line.
    """
    if not invalid_response_subject:
        return await _route_write(client, tool, values, body, query)

    try:
        raw = await _route_write(client, tool, values, body, query)
    except ValueError as exc:
        msg = f"{invalid_response_subject} response is invalid: {exc}"
        raise ValueError(msg) from exc
    if free_form and raw is None:
        return {}
    if not isinstance(raw, dict):
        msg = f"{invalid_response_subject} response must be a JSON object"
        raise TypeError(msg)
    return cast("dict[str, Any]", raw)


def _redacted_body(body: dict[str, Any], names: Sequence[str]) -> dict[str, Any]:
    """The body a preview reports, with each named member stood in for.

    Only a member the call actually carries is replaced, so a preview never
    claims to be sending something the request left out. The live body is
    untouched; go/internal/tools's WriteBody.Redacting answers the same shape.
    """
    if not names:
        return body
    return {
        key: {_REDACTED_KEY: True} if key in names else value
        for key, value in body.items()
    }


def _stood_in_body(
    body: dict[str, Any], stand_ins: Sequence[PreviewStandIn]
) -> dict[str, Any]:
    """The body a preview reports, with one member of each entry stood in for.

    The entries keep their order and every other member, so a caller still
    reads the call the request would make. The live body is untouched;
    go/internal/tools's WriteBody.StandingIn answers the same shape.
    """
    reported = body
    for stand_in in stand_ins:
        entries = reported.get(stand_in.argument)
        if not isinstance(entries, list):
            continue
        reported = reported | {
            stand_in.argument: [
                _stood_in_entry(cast("dict[str, Any]", entry), stand_in)
                if isinstance(entry, dict)
                else entry
                for entry in cast("Sequence[Any]", entries)
            ]
        }
    return reported


def _stood_in_entry(entry: dict[str, Any], stand_in: PreviewStandIn) -> dict[str, Any]:
    """One entry as the preview reports it.

    An entry carrying no such member is answered unchanged, which is what a
    member the caller left out already means.
    """
    if stand_in.member not in entry:
        return entry
    return entry | {stand_in.member: stand_in.text}


def _restore_member_nulls(
    serialized: dict[str, Any],
    payload: Mapping[str, Any],
    names: Sequence[str],
    payload_field: FieldDescriptor,
) -> dict[str, Any]:
    """Restore the declared nulls inside the member the raw body decoded into.

    An envelope's own members are assembled from the call and never arrive as
    null, so the restoration reaches only the resource under payload_field.
    """
    if not names or payload_field.message_type is None:
        return serialized
    member = serialized.get(payload_field.name)
    if not isinstance(member, dict):
        return serialized  # pragma: no cover - serializer always emits a dict member
    serialized[payload_field.name] = restore_explicit_nulls(
        payload, cast("dict[str, Any]", member), names, payload_field.message_type
    )
    return serialized


def _transfer_cautions(transfer: PresignPreview | None) -> list[str]:
    """The guard's refusal as a warning, and nothing for a transfer it accepted."""
    if transfer is None or not transfer.refusal:
        return []

    return [transfer.refusal]


async def _derived_preview(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    method: str,
    endpoint: str,
    body: dict[str, Any] | None,
    side_effects: PreviewLines,
    warnings: PreviewLines,
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None,
    billing_delta: dict[str, Any] | None = None,
    transfer: PresignPreview | None = None,
) -> list[TextContent]:
    """The preview a tool's declarations give it.

    Without a state fetch it reports the request alone, which is the right
    shape for a create: there is no existing resource to compare against. With
    one it fetches first, so the caller sees the resource the change lands on
    beside the prose saying what the change is.

    A declared line whose wordings could not be filled reports nothing, so the
    lines are filtered before they are reported.

    transfer is what a presigned upload's guard measured, handed to the lines
    that word it; a source the guard refused is reported ahead of them.
    """
    if state_fetch is None:
        effects = preview_reported_lines(side_effects, transfer)
        cautions = _transfer_cautions(transfer) + preview_reported_lines(
            warnings, transfer
        )

        return build_dry_run_response(
            tool,
            arguments.get("environment", ""),
            method,
            endpoint,
            None,
            request_body=body,
            side_effects=effects or None,
            warnings=cautions or None,
            billing_delta=billing_delta,
        )

    async def declared(_client: RetryableClient, state: Any) -> DryRunDetails:
        details: DryRunDetails = {}
        reported = preview_reported_lines(side_effects, state)
        cautioned = preview_reported_lines(warnings, state)
        if reported:
            details["side_effects"] = reported
        if cautioned:
            details["warnings"] = cautioned
        if billing_delta:
            details["billing_delta"] = billing_delta
        return details

    return await execute_dry_run(
        cfg,
        arguments,
        tool,
        method,
        endpoint,
        state_fetch,
        declared,
        request_body=body,
    )


async def _write_preview(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    endpoint: str,
    body: dict[str, Any] | None,
    preview_error: str | None,
    side_effects: PreviewLines,
    warnings: PreviewLines,
    billing_delta: dict[str, Any] | None,
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None,
) -> list[TextContent]:
    """A mutation's dry-run branch: the verdict first, then the reported call.

    preview_error surfaces ahead of everything because a preview reports what
    is wrong with the arguments before it describes a call built from them.
    """
    if preview_error is not None:
        return error_response(preview_error)

    method = route_for(tool).method

    return await _derived_preview(
        cfg,
        arguments,
        tool=tool,
        method=method,
        endpoint=endpoint,
        body=body,
        side_effects=side_effects,
        warnings=warnings,
        state_fetch=state_fetch,
        billing_delta=billing_delta,
    )


async def _paged_write(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any],
    query: str,
    error_action: str,
    path_values: Mapping[str, object],
) -> list[TextContent]:
    """Answer a mutation whose route reports a page rather than a resource.

    Both firewall replacements are the shape: the PUT reports every assignment
    that now exists, in the same {data, page, pages, results} envelope the list
    route uses. Serializing that into the declared response directly places none
    of its members, so the caller reads an empty collection over a replacement
    that already happened.

    No filter travels: a mutation narrows nothing, and the emitter has already
    refused a response carrying members the page decode cannot fill. Go's
    linode.ListProtoRouteBody reads the same body the same way.
    """
    key = _repeated_field(response_descriptor(tool)).name
    envelope = list_envelope_for(tool)

    async def call(client: RetryableClient) -> dict[str, Any]:
        raw = await _route_write(client, tool, values, body, query)
        return serialize_list_response(
            _page_of(raw, envelope), key, _new_response(tool)
        )

    return await execute_tool(
        cfg,
        arguments,
        error_action,
        call,
        failure="" if error_action else _failure_text(tool, arguments, path_values),
    )


async def run_write_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    error_action: str,
    body: dict[str, Any],
    path_values: Mapping[str, object] | None = None,
    echo_args: Mapping[str, str | int] | None = None,
    error: str | None = None,
    preview_error: str | None = None,
    side_effects: PreviewLines = (),
    warnings: PreviewLines = (),
    billing_delta: dict[str, Any] | None = None,
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None = None,
    preview_request_body: bool = True,
    redact_preview: Sequence[str] = (),
    success_message: str | None = None,
    invalid_response_subject: str = "",
    query: str = "",
    paged: bool = False,
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
    resource the change lands on is reported beside the request. Without it the
    preview reports the request alone, the right shape for a create.

    preview_request_body is False for the updates whose preview predates the
    body echo and whose fixtures pin the shorter shape.
    """
    route = route_for(tool)
    reported = _redacted_body(body, redact_preview)

    # Resolved inside each branch, the way the acknowledge tier resolves it: an
    # argument the route cannot build a path out of is the caller's to hear
    # about in the tool's own words, and resolving up front raises over the
    # sentence the validation already has for it.
    if is_dry_run(arguments):
        return await _write_preview(
            cfg,
            arguments,
            tool=tool,
            endpoint=with_query(
                route.preview_endpoint(*_path_values(tool, path_values or {})), query
            ),
            body=reported if preview_request_body else None,
            preview_error=preview_error,
            side_effects=side_effects,
            warnings=warnings,
            billing_delta=billing_delta,
            state_fetch=state_fetch,
        )

    if arguments.get("confirm") is not True:
        return error_response(contract_for(tool).confirm_message)

    if error is not None:
        return error_response(error)

    values = _path_values(tool, path_values or {})

    if paged:
        return await _paged_write(
            cfg,
            arguments,
            tool=tool,
            values=values,
            body=body,
            query=query,
            error_action=error_action,
            path_values=path_values or {},
        )

    response = response_descriptor(tool)
    payload_field = resolve_payload_field(tool)
    carries_message = _MESSAGE_FIELD in response.fields_by_name
    # The resource a placeholder reads from: the member the body decodes into,
    # or the response itself when the response is the body.
    resource = response if payload_field is None else payload_field.message_type

    def members(payload: Mapping[str, Any]) -> dict[str, str]:
        return _write_members(
            tool,
            arguments,
            payload,
            payload_descriptor=resource,
            carries_message=carries_message,
            success_message=success_message,
        )

    # Rendered and resolved once against an empty response before anything is
    # sent: a placeholder or an echo naming nothing fillable is a contract
    # defect, and finding it after the create has run would report a failure
    # over a resource that now exists.
    members({})

    echo = _echo_values(tool, echo_args) if echo_args else {}

    nulls = contract_for(tool).explicit_null_fields
    answered = bool(contract_for(tool).response_body_fields)
    free_form = (
        payload_field is not None
        and payload_field.message_type is not None
        and payload_field.message_type.full_name == _STRUCT_MESSAGE
    )

    async def call(client: RetryableClient) -> dict[str, Any]:
        raw = await _routed_write(
            client, tool, values, body, invalid_response_subject, free_form=free_form
        )
        payload = cast("dict[str, Any]", raw) if isinstance(raw, dict) else {}
        message = _new_response(tool)
        if payload_field is None:
            # The body decoded into the response, with the notice written onto
            # it: there is no envelope to assemble the members around.
            serialized = serialize_api_response(
                {**cast("dict[str, Any]", raw), **members(payload)}, message
            )
            return restore_explicit_nulls(
                payload, serialized, nulls, message.DESCRIPTOR
            )
        if answered:
            # The body carries the resource under its own key alongside the
            # values the answer reports, so one decode places them all and the
            # prose reads the resource out of what it placed.
            nested = payload.get(payload_field.name)
            resource = (
                cast("dict[str, Any]", nested) if isinstance(nested, dict) else {}
            )
            return serialize_api_response({**payload, **members(resource)}, message)
        serialized = serialize_api_response(
            {**members(payload), **echo, payload_field.name: raw}, message
        )
        return _restore_member_nulls(serialized, payload, nulls, payload_field)

    # Same rule the list tier follows: error_action is the verb a handler that
    # predates the contract reports with, and leaving it out asks for the
    # sentence the tool declares. Only a declared one can name the ids the call
    # was addressed by, which "Failed to {verb}" has no room for.
    declared = "" if error_action else _failure_text(tool, arguments, path_values or {})

    return await execute_tool(cfg, arguments, error_action, call, failure=declared)


async def run_acknowledge_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    echo_args: Mapping[str, object],
    body: dict[str, Any] | None = None,
    path_values: Mapping[str, object] | None = None,
    error: str | None = None,
    preview_error: str | None = None,
    redact_preview: Sequence[str] = (),
    stand_in_preview: Sequence[PreviewStandIn] = (),
    side_effects: PreviewLines = (),
    warnings: PreviewLines = (),
    preview_request_body: bool = True,
    transport: TransportSpec | None = None,
    preview_transfer: bool = False,
    assembled: tuple[str, ...] = (),
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None = None,
    fetch_state: Callable[[RetryableClient], Awaitable[Any]] | None = None,
    dependency_walk: Callable[[RetryableClient, Any], Awaitable[DryRunDetails]]
    | None = None,
    capability: Capability = Capability.Write,
) -> list[TextContent]:
    """Run a mutation the API answers with nothing to decode.

    A detach, a revoke, an acknowledgement: the API reports the change by
    status, so the answer is assembled from the call the way a delete's is.
    The gate order is the write tier's, because that is what these tools are:
    a preview reports preview_error before it fetches anything, and a live call
    reports error only after the confirm gate.

    A tool declaring no body field sends none. An empty JSON object is a body,
    and adding one would change the call both languages make today.

    redact_preview stands in for the named members when the preview reports the
    body. The tier a tool lands on is decided by the shape of its answer, and a
    secret in the request does not stop being one because the API answers with
    nothing. stand_in_preview reaches one member of a repeated argument's
    entries instead, for the calls whose other members a caller checks before
    confirming.

    Both branches report their argument failure before the route is built. A
    slot the caller left empty has no rendering the builder will accept, so
    building first would answer a missing id with a RouteError instead of the
    sentence the tool declares for it.

    transport makes the live call in place of the routed JSON request, for a
    route whose request is something else: the support-ticket attachment posts
    multipart/form-data built from a local file. It receives the arguments the
    gate already accepted, the path values this driver resolved and the body it
    was given, so nothing re-reads an argument or rebuilds a body. The dry-run
    branch above never reaches it, because a preview makes no call.

    preview_transfer says the declared prose reads what a presigned upload
    would send: the dry run runs the transport's own guard against the source,
    fills the presign body the way the live call does, hands the measurement to
    the lines that word it, and reports a refused source as a warning.

    state_fetch is the preview's own read, which reports the resource the change
    lands on beside the request. It is separate from fetch_state below because
    the two answer different branches: a preview describes what is there now,
    while a plan hashes what it read for drift, and a family can want one
    without the other.

    fetch_state turns the tool into a staged one: mode:"plan" reports the change
    and stores it, mode:"apply" re-checks the resource for drift and sends the
    same body the single-step path would have. It answers ahead of the preview
    and the gate because a plan is neither, and its argument failure is reported
    first for the reason a delete reports one first: a plan that fetched state
    for a call it was going to refuse would spend a request saying so.
    """
    route = route_for(tool)

    if fetch_state is not None and _staged(arguments):
        staged = await _acknowledge_stage(
            cfg,
            arguments,
            tool=tool,
            route=route,
            echo_args=echo_args,
            body=body,
            path_values=path_values,
            error=error,
            transport=transport,
            fetch_state=fetch_state,
            dependency_walk=dependency_walk,
            capability=capability,
        )
        if staged is not None:
            return staged

    if is_dry_run(arguments):
        return await _acknowledge_preview(
            cfg,
            arguments,
            tool=tool,
            route=route,
            body=body,
            path_values=path_values,
            preview_error=preview_error,
            redact_preview=redact_preview,
            stand_in_preview=stand_in_preview,
            side_effects=side_effects,
            warnings=warnings,
            state_fetch=state_fetch,
            preview_request_body=preview_request_body,
            transport=transport,
            preview_transfer=preview_transfer,
        )

    if arguments.get("confirm") is not True:
        return error_response(contract_for(tool).confirm_message)

    if error is not None:
        return error_response(error)

    return await execute_tool(
        cfg,
        arguments,
        "",
        _acknowledge_report(
            tool, arguments, path_values, body, echo_args, transport, assembled
        ),
        failure=_failure_text(tool, arguments, path_values or {}),
    )


def _staged(arguments: Mapping[str, Any]) -> bool:
    """Whether a call is asking for a stage rather than the whole change."""
    return arguments.get("mode") in ("plan", "apply")


async def _acknowledge_stage(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    route: Any,
    echo_args: Mapping[str, object],
    body: dict[str, Any] | None,
    path_values: Mapping[str, object] | None,
    error: str | None,
    transport: TransportSpec | None,
    fetch_state: Callable[[RetryableClient], Awaitable[Any]],
    dependency_walk: Callable[[RetryableClient, Any], Awaitable[DryRunDetails]] | None,
    capability: Capability,
) -> list[TextContent] | None:
    """Plan or apply a staged mutation, or None to fall through.

    The whole flow is the delete tier's, reached from a second tier rather than
    copied: the plan store, the drift hash, the refusals and the single-use
    apply are one order both tiers share, and what a delete does not have is
    already a parameter. The apply sends the body this tier's own live path
    would have sent, which is what separates a staged write from a staged
    delete.

    route is typed loosely because the contract reader owns its shape and
    naming it here would import the reader for an annotation alone.
    """
    if error is not None:
        return error_response(error)

    return await run_two_stage_destroy(
        cfg,
        arguments,
        tool_name=tool,
        method=route.method,
        path=route.preview_endpoint(*_path_values(tool, path_values or {})),
        fetch_state=fetch_state,
        execute=_acknowledge_report(
            tool, arguments, path_values, body, echo_args, transport
        ),
        hash_ignore=hash_ignore_fields(contract_for(tool).resource_type),
        dependency_walk=dependency_walk,
        capability=capability,
    )


async def _acknowledge_preview(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    route: Any,
    body: dict[str, Any] | None,
    path_values: Mapping[str, object] | None,
    preview_error: str | None,
    redact_preview: Sequence[str],
    stand_in_preview: Sequence[PreviewStandIn],
    side_effects: PreviewLines,
    warnings: PreviewLines,
    state_fetch: Callable[[RetryableClient], Awaitable[Any]] | None,
    preview_request_body: bool,
    transport: TransportSpec | None,
    preview_transfer: bool,
) -> list[TextContent]:
    """Report the call rather than making it, with the declared members stood
    in for."""
    if preview_error is not None:
        return error_response(preview_error)

    endpoint = route.preview_endpoint(*_path_values(tool, path_values or {}))
    reported = (
        _stood_in_body(_redacted_body(body, redact_preview), stand_in_preview)
        if body
        else body
    )

    transfer = (
        _previewed_transfer(cfg, arguments, reported, tool, transport)
        if preview_transfer
        else None
    )

    return await _derived_preview(
        cfg,
        arguments,
        tool=tool,
        method=route.method,
        endpoint=endpoint,
        body=reported if preview_request_body else None,
        side_effects=side_effects,
        warnings=warnings,
        state_fetch=state_fetch,
        transfer=transfer,
    )


def _previewed_transfer(
    cfg: Config,
    arguments: dict[str, Any],
    body: dict[str, Any] | None,
    tool: str,
    transport: TransportSpec | None,
) -> PresignPreview:
    """The upload guard's measurement, for a tool whose prose reads it.

    Only a presigned upload has a source to measure, and the emitter refuses
    the placeholder on anything else, so reaching here with another transport
    is a contract defect rather than a call to answer.
    """
    if not isinstance(transport, PresignTransfer) or not transport.up:
        msg = f"{tool} previews a transfer it does not declare as a presigned upload"
        raise TypeError(msg)

    return preview_presign_source(cfg, arguments, body, transport.local_path_argument)


def _acknowledge_report(
    tool: str,
    arguments: dict[str, Any],
    path_values: Mapping[str, object] | None,
    body: dict[str, Any] | None,
    echo_args: Mapping[str, object],
    transport: TransportSpec | None,
    assembled: tuple[str, ...] = (),
) -> Callable[[RetryableClient], Awaitable[dict[str, Any]]]:
    """The live call and the answer it reports, for both the single-step path
    and the apply a plan stores.

    The route and the echo are resolved here rather than inside the call, for
    the reason the delete tier resolves its echo first: a response naming an
    argument the call was never given is a contract defect, and finding it
    afterwards would report a failure over a change that has already happened.

    assembled names the members the transport fills rather than the call: an
    upload reports the byte count and the ETag its transfer produced, which no
    argument and no declared sentence carries.
    """
    values = _path_values(tool, path_values or {})
    echo = _echo_values(tool, echo_args)
    text = _success_text(tool, arguments, {}, None, None)

    async def call(client: RetryableClient) -> dict[str, Any]:
        filled: Mapping[str, Any] = {}
        if transport is None:
            await _routed_acknowledge(client, tool, values, body)
        else:
            filled = _assembled_values(
                tool,
                assembled,
                await run_transport(transport, client, tool, arguments, values, body),
            )
        return serialize_api_response(
            {_MESSAGE_FIELD: text, **echo, **filled}, _new_response(tool)
        )

    return call


async def _routed_acknowledge(
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any] | None,
) -> None:
    """Make the call under the contract's retry policy, decoding nothing."""
    if body is None:
        await _routed_delete(client, tool, values)
        return
    await _route_write(client, tool, values, body)


def _read_from_response(field: Any, id_args: Mapping[str, object]) -> bool:
    """Whether a response field is decoded rather than echoed from the call.

    A sub-message the call carried is echoed, whether it repeats or not: that is
    how an opaque-bodied route answers with what it was given, and the
    bucket-access routes answer with no body at all, so decoding their member
    would report an ACL of "" over a change already made. One the call did not
    carry is left to the decode.

    field is typed Any for the reason the rest of this module's descriptor
    arguments are: the runtime descriptor comes from the C (upb) backend.
    """
    if field.type != FieldDescriptor.TYPE_MESSAGE:
        return False

    return field.name not in id_args


def _echo_values(tool: str, id_args: Mapping[str, object]) -> dict[str, Any]:
    """The echo a response carries, filled from the call's own arguments.

    A delete or an acknowledged mutation answers with nothing to decode, so the
    envelope is built rather than read: the message plus every other scalar the
    response declares, each named after the argument it echoes. That naming is
    what lets the echo be filled without a per-tool mapping, and a member the
    API spells its own way says which argument through echo_argument.

    A destroy hands its ids in under the route's own slot names, so the lookup
    goes through the argument rather than the member: keying the map by member
    would respell the sentence a refused id answers with.

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
        # The warning is prose the contract declares, not a value the call
        # carried, so it is filled beside the message rather than echoed.
        if field.name in {_MESSAGE_FIELD, _WARNING_FIELD}:
            continue
        if _read_from_response(field, id_args):
            continue
        argument = echo_argument(field)
        if argument not in id_args:
            msg = (
                f"{tool} answers with {field.name}, which names no argument it"
                " was given"
            )
            raise DriverError(msg)
        echo[field.name] = id_args[argument]
    return echo


def _destroy_failure(
    tool: str, arguments: Mapping[str, Any], values: Mapping[str, object]
) -> str:
    """The sentence a failed removal answers with.

    A tool declaring error_message means to say something its family's callers
    recognize, naming the resource it could not remove, so the declaration wins
    wherever there is one. The shared sentence stays here rather than in the
    contract because it is not a per-tool fact: a delete that fails names the
    tool and the failure, and there is nothing else to say.
    """
    if not contract_for(tool).error_message:
        return f"{tool} failed: {{error}}"

    return _failure_text(tool, arguments, values)


def _destroy_payload_field(tool: str) -> FieldDescriptor | None:
    """The response member a removal decodes the API's answer into, or None.

    Only a rebuild has one; every plain delete answers with an empty body, so
    its response declares scalars and nothing else. Separate from
    resolve_payload_field, which holds a mutation to carrying exactly one
    resource and would refuse the shape this tier normally has.
    """
    descriptor = response_descriptor(tool)
    for field in descriptor.fields:
        if field.type == FieldDescriptor.TYPE_MESSAGE and not field.is_repeated:
            return field
    return None


def _destroy_route(
    tool: str, id_args: Mapping[str, str | int], *, usable: bool
) -> tuple[tuple[object, ...], str]:
    """The path values and the endpoint a removal would address.

    Resolved only for a call that can make it: a slot the caller left blank has
    no path value, and asking the route for one raises over the sentence the
    missing-id check already holds for it.
    """
    if not usable:
        return (), ""

    values = _path_values(tool, id_args, allow_extra=True)

    return values, route_for(tool).preview_endpoint(*values)


def _destroy_preview_body(
    body: dict[str, Any] | None, redact_preview: Sequence[str]
) -> dict[str, Any] | None:
    """The body a removal's preview reports, with each declared member stood in
    for. A removal that sends nothing reports the route alone."""
    if body is None:
        return None

    return _redacted_body(body, redact_preview)


def _destroy_answer(
    tool: str,
    text: str,
    echo: Mapping[str, Any],
    payload_field: FieldDescriptor | None,
    decoded: Any,
) -> dict[str, Any]:
    """The envelope a completed removal reports.

    The sentence and the echo are assembled from the call, and a response
    declaring a resource member carries the decode beside them, which is how a
    rebuilt instance survives a tier whose other tools answer with nothing.
    """
    members: dict[str, Any] = {_MESSAGE_FIELD: text, **echo}

    if payload_field is not None:
        members[payload_field.name] = decoded

    return serialize_api_response(members, _new_response(tool))


async def _routed_removal(
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any] | None,
) -> Any:
    """Make the removal call and hand back whatever the API answered with.

    A plain delete sends nothing and reads nothing back; a removal that is a
    rebuild sends what it was given and answers with the resource it replaced.
    """
    if body is None:
        await _routed_delete(client, tool, values)
        return None

    return await _route_write(client, tool, values, body)


async def read_route_state(
    client: RetryableClient,
    *values: object,
    tool: str,
    member: str = "",
    payload: str = "",
    query: str = "",
) -> DeclaredState:
    """Read the resource a removal is about to take away, through a read route.

    tool is the read the removal derives or declares, so the state is projected
    through the message that read answers with rather than through a shape
    invented for the preview. It is keyword-only for the reason the other
    drivers take it that way: the route scanner reads the tool a call site names
    off that keyword, and a positional one would leave this helper's own
    route_raw call reading as a route nothing names.

    member is the raw-body key the resource sits under, empty for the reads that
    answer the resource itself. payload is the response member the resource
    itself is, for the reads that wrap it.

    query addresses the reads a path alone does not reach: the object ACL names
    its bucket in the path and its object in the query, so the resource is not
    reachable without both.

    A body that is not a JSON object is refused for the reason run_get_tool
    refuses one: ParseDict reads a bare array, string or number as an empty
    message and reports success carrying no data, where Go's decode of the same
    body fails.
    """
    # Passed only when there is one, so the reads a path alone addresses put the
    # same call on the wire as they did before query slots existed.
    if query:
        raw = await client.route_raw(tool, *values, query=query)
    else:
        raw = await client.route_raw(tool, *values)

    if not isinstance(raw, dict):
        msg = f"{tool} response must be an object"
        raise TypeError(msg)

    body = cast("dict[str, Any]", raw)
    if member:
        body = _state_member(body, tool, member)

    message = _state_message(tool, payload)
    # The decode is what refuses a body the read's contract does not describe.
    # What the state carries is the projection, which is the raw body itself
    # keyed to the members that message models.
    json_format.ParseDict(body, message, ignore_unknown_fields=True)

    return project_declared_state(body, message.DESCRIPTOR)


def _state_message(tool: str, payload: str) -> Message:
    """The message a state read decodes and projects through.

    A read that answers a wrapper around the resource names the member holding
    it, since the API's body is the resource and projecting through the wrapper
    would report an empty one.
    """
    message = _new_response(tool)
    if not payload:
        return message

    field = message.DESCRIPTOR.fields_by_name.get(payload)
    if field is None or field.message_type is None:
        msg = f"{tool} answers no message member: {payload}"
        raise RouteError(msg)

    return message_class(str(field.message_type.full_name))()


def _state_member(body: dict[str, Any], tool: str, member: str) -> dict[str, Any]:
    """The member of a state answer the resource sits under.

    Decoding the envelope instead would report an empty resource as the state a
    delete is about to remove. Go refuses the same answer with this sentence.
    """
    resource = body.get(member)
    if not isinstance(resource, dict):
        msg = f"{_route_subject(tool)} state response carries no member: {member}"
        raise TypeError(msg)

    return cast("dict[str, Any]", resource)


async def read_collection_state(
    client: RetryableClient, *values: object, tool: str, member: str, value: object
) -> DeclaredState:
    """Read the resource a removal is about to take away out of its collection.

    Three certificate routes exist on an identity-provider configuration and
    none of them reads one certificate, so there is no exact-path sibling to
    read through and the removal would otherwise have nothing to preview or
    hash. values fill the collection's own route, which is the removal's path
    with its trailing id dropped, and that id picks the element out of the page.

    One call at the standard maximum is what the route can hand over at once,
    and the collections this serves are small nested sets. An element past that
    page reports as missing rather than being previewed as something else. Go's
    FetchCollectionElement reads the same page and reports the same sentence.
    """
    element = _repeated_field(response_descriptor(tool))
    query = pagination_query(_COLLECTION_STATE_PAGE, STANDARD_PAGE_BOUNDS.page_size_max)
    raw = await client.route_raw(tool, *values, query=query)
    # The page is read raw rather than serialized because the element a removal
    # previews is reported from the body it arrived in, the way a resource read
    # on its own route is.
    page = _page_of(raw, list_envelope_for(tool))

    for item in cast("list[dict[str, Any]]", page.get("data", [])):
        if item.get(member) == value:
            return project_declared_state(item, element.message_type)

    msg = f"collection holds no matching element: {tool} carries no id '{value}'"
    raise LookupError(msg)


async def read_collection_scan(
    client: RetryableClient, *values: object, tool: str, matches: dict[str, object]
) -> DeclaredState:
    """Page the whole named collection and answer the first element every
    declared pair matches, projected the way a single-resource fetch is.

    Unlike read_collection_state's one-page read, a scan pages to the end: the
    resource is named by field values rather than an id, so no page holds a
    derivable position. Go's FetchCollectionScan pages and words the not-found
    sentence the same way.
    """
    element = _repeated_field(response_descriptor(tool))
    bound = STANDARD_PAGE_SIZE_MAX
    page_index = _COLLECTION_STATE_PAGE
    while True:
        raw = await client.route_raw(
            tool, *values, query=pagination_query(page_index, bound)
        )
        page = _page_of(raw, list_envelope_for(tool))
        data = cast("list[dict[str, Any]]", page.get("data", []))
        for item in data:
            if all(item.get(field) == value for field, value in matches.items()):
                return project_declared_state(item, element.message_type)
        if len(data) < bound:
            wanted = ", ".join(f"{field}='{value}'" for field, value in matches.items())
            msg = (
                "collection holds no matching element: "
                f"{tool} carries no element matching {wanted}"
            )
            raise LookupError(msg)
        page_index += 1


async def read_envelope_state(
    client: RetryableClient, *values: object, tool: str, query: str = ""
) -> DeclaredState:
    """Read the named list once and answer the page envelope itself as the
    state: the projected elements under "data" and the API's total under
    "results".

    The total rides along because a truncated first page must not understate
    what the caller is about to touch. query is the read's own page controls,
    empty for a read taken at the route's defaults: a replacement previews the
    page its own call publishes, so the state and the answer describe one page.
    Go's FetchEnvelopeState reads the same envelope and reports the same
    members.
    """
    element = _repeated_field(response_descriptor(tool))
    raw = (
        await client.route_raw(tool, *values, query=query)
        if query
        else await client.route_raw(tool, *values)
    )
    page = _page_of(raw, list_envelope_for(tool))
    data = [
        project_declared_state(cast("dict[str, Any]", item), element.message_type)
        for item in cast("list[Any]", page.get("data", []))
        if isinstance(item, dict)
    ]
    results = page.get("results")
    return DeclaredState(
        {"data": data, "results": results if isinstance(results, int) else None}
    )


class CompositeCall(NamedTuple):
    """One read of a composite state: the GET, its member, the fields kept."""

    tool: str
    member: str
    fields: tuple[str, ...]
    values: tuple[object, ...]
    is_list: bool


async def read_composite_state(
    client: RetryableClient, calls: Sequence[CompositeCall]
) -> DeclaredState:
    """Perform each call in declaration order and assemble the projected state.

    A failed call fails the fetch whole: a plan hashed over half a state would
    refuse an apply for a change nobody made. Everything outside each call's
    kept fields is dropped, which is what keeps a cosmetic field from refusing
    an apply. Go's FetchCompositeState mirrors both rules.
    """
    state: dict[str, Any] = {}
    for call in calls:
        state[call.member] = await _composite_member(client, call)
    return DeclaredState(state)


async def _composite_member(client: RetryableClient, call: CompositeCall) -> Any:
    """One composite call's answer, projected and cut to its kept fields."""
    if not call.is_list:
        resource = await read_route_state(client, *call.values, tool=call.tool)
        return _kept_fields(resource, call.fields)

    element = _repeated_field(response_descriptor(call.tool))
    bound = STANDARD_PAGE_SIZE_MAX
    kept: list[Any] = []
    page_index = _COLLECTION_STATE_PAGE
    while True:
        raw = await client.route_raw(
            call.tool, *call.values, query=pagination_query(page_index, bound)
        )
        page = _page_of(raw, list_envelope_for(call.tool))
        data = cast("list[dict[str, Any]]", page.get("data", []))
        kept.extend(
            _kept_fields(
                project_declared_state(item, element.message_type), call.fields
            )
            for item in data
        )
        if len(data) < bound:
            return kept
        page_index += 1


def _kept_fields(projected: DeclaredState, fields: tuple[str, ...]) -> DeclaredState:
    """Cut one projected state to the declared subset."""
    return DeclaredState(
        {name: projected.fields[name] for name in fields if name in projected.fields}
    )


async def run_destructive_tool(
    cfg: Config,
    arguments: dict[str, Any],
    *,
    tool: str,
    error_action: str,
    id_args: Mapping[str, str | int],
    fetch_state: Callable[[RetryableClient], Awaitable[Any]],
    dependency_walk: Callable[[RetryableClient, Any], Awaitable[DryRunDetails]]
    | None = None,
    error: str | None = None,
    success_message: str | None = None,
    body: dict[str, Any] | None = None,
    redact_preview: Sequence[str] = (),
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

    body is what a removal that is a rebuild rather than a delete sends, and
    redact_preview names the members a preview stands in for so a root password
    is not printed beside the route it would travel on. A response declaring a
    resource member decodes the answer into it, which is how a rebuilt instance
    survives a tier whose other tools answer with nothing.

    error follows run_write_tool's rule and this tier's own history: the delete
    handlers report a missing id before the plan and preview branches and after
    the confirm gate, and the behavior fixtures pin that asymmetry.
    """
    route = route_for(tool)
    contract = contract_for(tool)
    values, endpoint = _destroy_route(tool, id_args, usable=error is None)
    echo = _echo_values(tool, id_args)
    payload_field = _destroy_payload_field(tool)
    # The validated ids win over the raw arguments they were read from, leaving
    # a body member to fill from the call itself.
    filled = {**arguments, **id_args}

    def message() -> str:
        return _success_text(tool, filled, {}, None, success_message)

    # A DELETE answers with an empty body, so the sentence reads only the call
    # and is rendered before that call rather than after it: a placeholder
    # naming nothing fillable is a contract defect, and finding it afterwards
    # would report the failure over a resource that is already gone. A call the
    # checks above already refused is not rendered, since the argument a
    # placeholder names may be the very one it was refused for.
    if error is None:
        message()

    def answer(decoded: Any) -> dict[str, Any]:
        return _destroy_answer(tool, message(), echo, payload_field, decoded)

    async def execute_and_report(client: RetryableClient) -> dict[str, Any]:
        return answer(await _routed_removal(client, tool, values, body))

    staging = _staged(arguments)
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
            cfg,
            arguments,
            tool,
            route.method,
            endpoint,
            fetch_state,
            dependency_walk,
            request_body=_destroy_preview_body(body, redact_preview),
        )

    if arguments.get("confirm") is not True:
        return error_response(contract.confirm_message)

    if error is not None:
        return error_response(error)

    declared = "" if error_action else _destroy_failure(tool, arguments, id_args)

    return await execute_tool(
        cfg, arguments, error_action, execute_and_report, failure=declared
    )
