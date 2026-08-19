"""Route lookup for the Linode API operation each tool declares.

Every non-meta tool's proto *Input message carries a `linode.mcp.v1.tool_route`
option holding the tool name, HTTP method, and path template. This is the
client-side reader, so a call site names its tool and hands over its path values
instead of writing the route out a second time.

`make tool-routes` holds every path parameter to exactly one appearance in its
template, which is what lets slots be read off the template and filled
positionally.
"""

from __future__ import annotations

import importlib
import pkgutil
import re
from dataclasses import dataclass
from enum import Enum
from functools import cache
from types import MappingProxyType
from typing import TYPE_CHECKING
from urllib.parse import quote

from google.protobuf import message_factory

if TYPE_CHECKING:
    from collections.abc import Iterable, Iterator, Mapping
    from types import ModuleType

    from google.protobuf.descriptor import Descriptor, FieldDescriptor
    from google.protobuf.message import Message

_GENERATED_PACKAGE = "linodemcp.genpb.linode.mcp.v1"

# One named path parameter: "/domains/{domain_id}". The name shape matches the
# tool-routes gate, so a template this misses would already have been rejected.
_SLOT = re.compile(r"\{([a-z][a-z0-9_]*)\}")


class RouteError(ValueError):
    """A tool has no declared route, or a route was filled with bad values.

    Every case is a defect in what the caller asked for, never a failure of the
    call itself, so it inherits ValueError: the handler helpers report it as an
    argument error rather than a failed Linode call, and the retry layer
    replays only APIError and NetworkError. Go draws the same line with
    sentinels outside its network error type.
    """


DEFAULT_SURFACE_SEGMENT = "v4"

# The ApiSurface enum values, kept as numbers so this reader does not depend on
# the generated module being importable to answer what a surface means.
_SURFACE_SEGMENTS = {
    0: DEFAULT_SURFACE_SEGMENT,
    1: DEFAULT_SURFACE_SEGMENT,
    2: "v4beta",
}


def surface_segment(surface: int) -> str:
    """The base path segment a declared surface names.

    An unknown value is refused, never defaulted: answering a beta-only route on
    /v4 is a 404, and answering a v4 route on /v4beta reaches a different
    resource surface, so guessing either way addresses the wrong thing. Go's
    SurfaceSegment refuses the same set.
    """
    segment = _SURFACE_SEGMENTS.get(surface)
    if segment is None:
        msg = f"unknown API surface {surface}"
        raise RouteError(msg)
    return segment


def base_for(base: str, segment: str) -> str:
    """The base one surface's calls go to, given the configured one.

    Only a base whose last segment is the default surface is re-pointed, so a
    proxy, a mock, or a base an environment already pointed at the beta surface
    is used exactly as configured. That is what keeps the per-environment apiUrl
    override winning: a surface picks among versions of one deployment, it never
    picks the deployment. Go's BaseFor implements the same rule.
    """
    suffix = "/" + DEFAULT_SURFACE_SEGMENT
    if not base.endswith(suffix):
        return base
    return base[: -len(suffix)] + "/" + segment


@dataclass(frozen=True)
class Route:
    """One tool's declared operation, with its path slots in template order."""

    tool: str
    method: str
    template: str
    slots: tuple[str, ...]
    # The declared ApiSurface value, kept as declared rather than resolved so
    # validate() reports one this build cannot address at startup.
    surface: int = 0

    def endpoint(self, *values: object) -> str:
        """Fill the template's slots from values, in declared order.

        Arity mismatch raises instead of returning what was filled so far: a
        half-filled template still looks like a URL and would reach the API as
        one.
        """
        if len(values) != len(self.slots):
            msg = (
                f"route {self.tool} takes {len(self.slots)} path value(s),"
                f" got {len(values)}"
            )
            raise RouteError(msg)

        filled = self.template
        for slot, value in zip(self.slots, values, strict=True):
            # Encoding before substitution keeps a value from acting as markup:
            # no brace in it can open a later slot, no slash can invent a
            # path segment.
            text = _escape_segment(_slot_text(self.tool, slot, value))
            filled = filled.replace("{" + slot + "}", text)
        return filled

    def preview_endpoint(self, *values: object) -> str:
        """The filled path a preview and a staged plan report.

        A preview names the request that would run, so it carries the surface
        the client will actually address. Reporting the default base for a call
        that goes elsewhere describes a request nothing makes, which is the
        defect class this whole capability exists to remove. Go renders the
        same string, in its renderer arm rather than here.
        """
        filled = self.endpoint(*values)
        segment = surface_segment(self.surface)
        if segment == DEFAULT_SURFACE_SEGMENT:
            return filled
        return "/" + segment + filled


def _escape_segment(text: str) -> str:
    """Percent-encode one path value the way both clients agreed to.

    The kept set is RFC 3986's unreserved characters plus the colon. RFC 3986
    allows either form for the colon, and it stays literal because both clients
    always sent it that way on the IPv6 and IP address routes, tests on each
    side pin that form, and the API has never been proven to accept %3A there.
    The Go builder keeps the identical set.

    A value of nothing but dots must not pass through literally: "." and ".."
    normalize to the parent collection, which for a DELETE is every resource in
    it. Encoded, they name the same resource after server-side decoding while
    staying inert during normalization. quote() leaves dots alone since they
    are unreserved, so this guard sits in front.
    """
    if text and set(text) == {"."}:
        return "%2E" * len(text)
    return quote(text, safe=":")


def _slot_text(tool: str, slot: str, value: object) -> str:
    """Render one path value as the text that fills its slot.

    Every message names the tool and the slot because a positional call site
    gives no other clue which argument was wrong.

    Only str and int are accepted, the pair the Go builder takes, so a value
    cannot resolve to one path in one client and a different one in the other.
    bool is refused on its own because Python counts it as an int and no path
    slot names a flag.

    An empty rendering is refused for the reason a wrong value count is: it
    collapses "/tags/{tag_label}" to "/tags/", which still reaches the API and
    addresses the whole collection, which for a DELETE is every member.
    """
    if isinstance(value, bool) or not isinstance(value, str | int):
        msg = (
            f"route {tool} slot {slot} takes a str or an int,"
            f" got {type(value).__name__}"
        )
        raise RouteError(msg)

    text = value if isinstance(value, str) else str(value)
    if not text:
        msg = f"route {tool} slot {slot} got an empty path value"
        raise RouteError(msg)

    return text


@cache
def _options() -> ModuleType:
    """The generated options module, imported once.

    It has to come from the same import as the descriptors or the extension
    identities differ and every HasExtension call reports False.
    """
    return importlib.import_module(f"{_GENERATED_PACKAGE}.options_pb2")


def _message_descriptors() -> Iterator[Descriptor]:
    """Every top-level message descriptor the generated tree carries.

    One walk for every reader here: walking twice for two facts carried by one
    message is what would let the readers disagree about which messages exist.
    """
    package = importlib.import_module(_GENERATED_PACKAGE)

    modules = [
        info.name
        for info in pkgutil.iter_modules(package.__path__)
        if info.name.endswith("_pb2")
    ]
    for name in modules:
        module = importlib.import_module(f"{_GENERATED_PACKAGE}.{name}")
        yield from module.DESCRIPTOR.message_types_by_name.values()


def _declared() -> Iterator[Route]:
    """Every tool_route option carried by the generated descriptors."""
    options = _options()

    for descriptor in _message_descriptors():
        declared = descriptor.GetOptions()
        if not declared.HasExtension(options.tool_route):
            continue
        route = declared.Extensions[options.tool_route]
        path = str(route.path)
        yield Route(
            tool=str(route.tool),
            method=str(route.method),
            template=path,
            slots=tuple(_SLOT.findall(path)),
            # The surface rides on the message beside the route, so it is read
            # from the same descriptor here rather than in a second walk.
            surface=int(declared.Extensions[options.tool_api_surface]),
        )


def index_routes(routes: Iterable[Route]) -> Mapping[str, Route]:
    """Tool name to the one route that tool declares.

    Two messages claiming one tool is a broken contract rather than drift, so
    it fails here instead of one of them silently winning. Public because that
    rule holds for any route source, not just the descriptor walk above.
    """
    table: dict[str, Route] = {}
    for route in routes:
        if route.tool in table:
            msg = f"tool {route.tool} declares more than one route"
            raise RouteError(msg)
        table[route.tool] = route
    return MappingProxyType(table)


@cache
def _table() -> Mapping[str, Route]:
    """The declared routes, read once.

    The walk imports every generated module, so it runs on first use rather
    than at import of this module.
    """
    return index_routes(_declared())


def route_for(tool: str) -> Route:
    """The route a tool declares."""
    route = _table().get(tool)
    if route is None:
        msg = f"tool {tool} declares no route"
        raise RouteError(msg)
    return route


def all_routes() -> tuple[Route, ...]:
    """Every declared route, tool-sorted."""
    return tuple(sorted(_table().values(), key=lambda route: route.tool))


@dataclass(frozen=True)
class Contract:
    """What a tool's input message declares about the call beyond its route.

    tool_route answers which operation runs and how to address it. These five
    options answer what comes back and how the MCP layer frames it, and each
    one replaces a fact both languages used to spell out again at their own
    call sites. Reading them here rather than at each handler is what makes the
    proto the one place the answer lives.

    An unset option reads back as its zero value, which is the same shape a
    message carrying no options at all produces. That is deliberate: `make
    tool-response` is what decides whether an unset option is a gap or a
    documented exception, so this reader reports what is declared and leaves
    the judgment there.
    """

    tool: str
    # Full name of the message carrying these options, which is also the input
    # schema the tool advertises. It is not an option, it is where the options
    # were read from, and it is here so a consumer that starts from a tool name
    # can reach the input contract without repeating the descriptor walk.
    input_message: str
    # Full name of the message the handler serializes, "" for the handful whose
    # Linode response is an open-ended object with no proto model.
    response: str
    confirm_message: str
    success_message: str
    # The notice a mutation reports beside its success_message, "" for the
    # responses that declare no warning field to carry one.
    warning_message: str
    # Whether warning_message was written at all, which is what separates a tool
    # answering with a deliberately empty notice from one declaring no notice:
    # the text alone reads back as "" for both.
    warning_declared: bool
    resource_type: str
    retry_disabled: bool
    # The sentence the tool advertises to a client, and the one a failed Linode
    # call answers with. Both are prose a client reads directly, and both were
    # spelled out per language until the contract could carry them.
    description: str
    error_message: str
    # The steps of the handler that are hand-written rather than derived, named
    # by kind. Each language implements a declared kind under a name it derives
    # from the tool and the kind, so a hook is bound with nothing written out.
    hooks: tuple[str, ...]
    # Fields of the message the raw API body decodes into that the API sends as
    # an explicit null, which the serializer drops and the answer writes back.
    explicit_null_fields: tuple[str, ...]
    # Members of a mutation's declared response the API's answer fills rather
    # than the call, which is what makes the envelope itself the message the
    # body decodes into.
    response_body_fields: tuple[str, ...]


def _contract_from(descriptor: Descriptor) -> Contract | None:
    """One message's contract, or None when it names no tool."""
    options = _options()
    declared = descriptor.GetOptions()

    tool = str(declared.Extensions[options.tool_route].tool) or str(
        declared.Extensions[options.tool_meta].tool
    )
    if not tool:
        return None

    return Contract(
        tool=tool,
        input_message=str(descriptor.full_name),
        response=str(declared.Extensions[options.tool_response]),
        confirm_message=str(declared.Extensions[options.confirm_message]),
        success_message=str(declared.Extensions[options.success_message]),
        warning_message=str(declared.Extensions[options.warning_message]),
        warning_declared=declared.HasExtension(options.warning_message),
        resource_type=str(declared.Extensions[options.resource_type]),
        retry_disabled=bool(declared.Extensions[options.retry_disabled]),
        description=str(declared.Extensions[options.tool_description]),
        error_message=str(declared.Extensions[options.error_message]),
        hooks=tuple(str(kind) for kind in declared.Extensions[options.tool_hooks]),
        explicit_null_fields=tuple(
            str(name) for name in declared.Extensions[options.explicit_null_fields]
        ),
        response_body_fields=tuple(
            str(name) for name in declared.Extensions[options.response_body_fields]
        ),
    )


@cache
def _contracts() -> Mapping[str, Contract]:
    """Tool name to what its input message declares, read once.

    A tool naming itself twice is caught by validate_declarations rather than
    here, so this keeps the first reading and lets that check report the pair.
    """
    table: dict[str, Contract] = {}
    for descriptor in _message_descriptors():
        contract = _contract_from(descriptor)
        if contract is not None:
            table.setdefault(contract.tool, contract)
    return MappingProxyType(table)


def contract_for(tool: str) -> Contract:
    """What the contract declares about one tool beyond its route."""
    contract = _contracts().get(tool)
    if contract is None:
        msg = f"tool {tool} declares no contract"
        raise RouteError(msg)
    return contract


@cache
def _descriptors_by_name() -> Mapping[str, Descriptor]:
    """Every top-level generated message, by full name.

    Built from the same walk the route and contract readers use, so a response
    a tool names is looked up in exactly the set of messages those readers saw.
    Going through the default descriptor pool instead would answer from
    whatever else the process has imported, which is the import-identity
    problem this module already guards against for the option extensions.
    """
    return MappingProxyType(
        {str(descriptor.full_name): descriptor for descriptor in _message_descriptors()}
    )


def message_class(full_name: str) -> type[Message]:
    """The generated class for a message the contract names.

    Callers build their own instance: a proto message is mutable and the
    serializers parse into the one they are handed, so handing out a shared
    instance would let one call's response leak into the next.
    """
    descriptor = _descriptors_by_name().get(full_name)
    if descriptor is None:
        msg = f"no generated message named {full_name}"
        raise RouteError(msg)
    generated: type[Message] = message_factory.GetMessageClass(descriptor)
    return generated


def input_descriptor(tool: str) -> Descriptor:
    """The descriptor of the message a tool takes its arguments from.

    Every tool has one, since it is the message its options were read off, so
    an absent descriptor here means the contract table and the descriptor table
    were built from different walks rather than that a tool declared nothing.
    """
    message = contract_for(tool).input_message
    descriptor = _descriptors_by_name().get(message)
    if descriptor is None:
        msg = f"tool {tool} declares its input on {message}, which is not generated"
        raise RouteError(msg)
    return descriptor


def response_descriptor(tool: str) -> Descriptor:
    """The descriptor of the message a tool answers with."""
    response = contract_for(tool).response
    if not response:
        msg = f"tool {tool} declares no response message"
        raise RouteError(msg)
    descriptor = _descriptors_by_name().get(response)
    if descriptor is None:
        msg = f"tool {tool} names response {response}, which is not generated"
        raise RouteError(msg)
    return descriptor


def echo_argument(member: FieldDescriptor) -> str:
    """The tool argument one response member is filled from.

    A member is resolved by its own name unless it declares echo_argument,
    which is for the members the API spells differently than the tool spells
    its argument. Both renderer arms read the same declaration, so a generated
    handler and this reader agree on which value fills which member.
    """
    declared = str(member.GetOptions().Extensions[_options().echo_argument])

    return declared or str(member.name)


class ListShape(Enum):
    """The JSON arrangement a list route answers with.

    Nearly every Linode collection arrives as the page envelope, which is why
    an undeclared route reads as DATA. The others are invisible from the
    response message, so a reader that assumed the page for all of them would
    answer a populated collection as an empty one and report success.
    """

    DATA = "data"
    REQUIRED_DATA = "required_data"
    KEYED = "keyed"
    BARE = "bare"
    SINGLETON = "singleton"
    MARKER = "marker"


@dataclass(frozen=True)
class ElementLift:
    """One element member the route nests somewhere else in the element body.

    The firewall history's version sits inside its rules object, and the shared
    rules message declares no version, so without the hoist the element answers
    with the zero the decode left there.
    """

    member: str
    source: str


@dataclass(frozen=True)
class ListEnvelope:
    """One tool's declared list shape, the member KEYED names, and the hoists."""

    shape: ListShape = ListShape.DATA
    member: str = ""
    lift: tuple[ElementLift, ...] = ()


_LIST_SHAPES = {
    1: ListShape.DATA,
    2: ListShape.REQUIRED_DATA,
    3: ListShape.KEYED,
    4: ListShape.BARE,
    5: ListShape.SINGLETON,
    6: ListShape.MARKER,
}


@cache
def list_envelope_for(tool: str) -> ListEnvelope:
    """The shape the named tool's list route replies with.

    Read from the contract rather than passed in at each call site, so one
    caller cannot decode a route differently from another. Go's
    linoderoute.ListEnvelopeFor answers the same question off the same option.
    """
    declared = input_descriptor(tool).GetOptions().Extensions[_options().list_envelope]
    shape = _LIST_SHAPES.get(int(declared.shape), ListShape.DATA)
    lift = tuple(
        ElementLift(member=str(entry.member), source=str(entry.source))
        for entry in declared.lift
    )
    return ListEnvelope(shape=shape, member=str(declared.member), lift=lift)


@dataclass(frozen=True)
class Tool:
    """One tool the proto contract declares: the name it registers under, the
    tier it is registered at, and whether it reaches a Linode route.

    The tier is the generated enum's value, so it reads back the way the
    descriptor stores it; capability_name renders it for a message.
    """

    name: str
    capability: int
    routed: bool


@dataclass(frozen=True)
class Declaration:
    """What one message's options say about a tool, kept as read rather than
    resolved into a Tool, so a message whose options contradict each other can
    be reported instead of silently resolving to one of the readings.

    An absent marker and one naming nothing both leave the tool name empty
    here, because a message carrying an unnamed marker leaves the same gap as
    one carrying no marker: nothing can say which tool it belongs to.
    """

    message: str
    route_tool: str
    meta_tool: str
    capability: int

    @property
    def name(self) -> str:
        """The tool this declaration belongs to.

        A well-formed declaration carries the name in exactly one of its two
        markers, which is what keeps a routed tool's name written once.
        """
        return self.route_tool or self.meta_tool

    def well_formed(self) -> bool:
        """Whether this names one tool at one tier, so it resolves to a Tool."""
        return not self.defects() and bool(self.name)

    def defects(self) -> list[str]:
        """Every way these options fail to describe one tool at one tier.

        A message that names no tool and declares no tier has none, which is
        how every response and nested type falls out here.
        """
        options = _options()
        if not self.name and self.capability == options.TOOL_CAPABILITY_UNSPECIFIED:
            return []

        found: list[str] = []
        if self.route_tool and self.meta_tool:
            found.append(
                f"{self.message}: names {self.route_tool} as a routed tool and"
                f" {self.meta_tool} as a meta tool, and a tool is one or the other"
            )
        if not self.name:
            found.append(
                f"{self.message}: declares {capability_name(self.capability)}"
                " but no marker names its tool"
            )
        return found + self._tier_defects()

    def _tier_defects(self) -> list[str]:
        """The ways the declared tier disagrees with the marker beside it.

        Meta is the tier of a tool that reaches no route, so the marker and the
        tier are two spellings of one fact, and letting them differ would put a
        second answer to "is this tool routed" inside the contract itself.
        """
        options = _options()
        tier = capability_name(self.capability)

        if self.capability == options.TOOL_CAPABILITY_UNSPECIFIED:
            return [f"{self.message}: names {self.name} but declares no capability"]
        if self.meta_tool and self.capability != options.TOOL_CAPABILITY_META:
            marker = (
                f"{self.message}: {self.meta_tool} carries a meta marker"
                f" but declares {tier}"
            )
            return [marker]
        if self.route_tool and self.capability == options.TOOL_CAPABILITY_META:
            routed = (
                f"{self.message}: {self.route_tool} declares a route and {tier},"
                " and a meta tool reaches no route"
            )
            return [routed]
        return []


def capability_name(capability: int) -> str:
    """Render one capability value the way the proto contract spells it."""
    return str(_options().ToolCapability.Name(capability))


def _declarations() -> Iterator[Declaration]:
    """What every message in the contract declares about a tool."""
    options = _options()

    for descriptor in _message_descriptors():
        declared = descriptor.GetOptions()
        yield Declaration(
            message=str(descriptor.full_name),
            route_tool=str(declared.Extensions[options.tool_route].tool),
            meta_tool=str(declared.Extensions[options.tool_meta].tool),
            capability=int(declared.Extensions[options.tool_capability]),
        )


@cache
def _declared_tools() -> tuple[Tool, ...]:
    """The tools the contract declares, read once and name-sorted."""
    found = [
        Tool(
            name=declared.name,
            capability=declared.capability,
            routed=bool(declared.route_tool),
        )
        for declared in _declarations()
        if declared.well_formed()
    ]
    return tuple(sorted(found, key=lambda tool: tool.name))


def tools() -> tuple[Tool, ...]:
    """Every tool the contract declares, name-sorted.

    A message whose options contradict each other names no tool here; validate
    is what reports it.
    """
    return _declared_tools()


def validate_declarations(declared: Iterable[Declaration]) -> None:
    """Report every message that does not name one tool at one tier.

    Public for the same reason index_routes is: none of the contradictions it
    reports can come out of the shipped descriptors, since `make
    tool-capability` rejects them before they can be generated, so proving the
    check still bites means handing it a broken declaration directly.
    """
    defects: list[str] = []
    claimed: dict[str, str] = {}

    for entry in declared:
        defects.extend(entry.defects())
        first = claimed.get(entry.name)
        if entry.name and first is not None:
            defects.append(
                f"{entry.message}: names {entry.name}, which {first} already declares"
            )
        claimed[entry.name] = entry.message

    if defects:
        msg = "broken tool declaration: " + "; ".join(sorted(defects))
        raise RouteError(msg)


def validate() -> None:
    """Report a contract that cannot describe the surface it claims to.

    It takes no arguments because the descriptors name the whole surface now.
    While only routes were declared, a meta tool was indistinguishable from a
    tool someone forgot to route, so the caller had to say which tools it
    expected to be routed; tool_meta closed that, and the expected set is no
    longer something a caller can get wrong.

    A slot list that disagrees with its template is not a failure this can
    have: slots are read off the template itself.

    A surface this build cannot address is one it can have, so every declared
    route resolves its segment here rather than failing at the one call that
    needed it.
    """
    validate_declarations(_declarations())

    for route in all_routes():
        surface_segment(route.surface)


def validate_registered(registered: Iterable[str]) -> None:
    """Report the difference between a server's staged tools and the declared
    set, in both directions.

    Either direction means the surface a client sees is not the surface the
    contract describes: a staged tool the contract never named has no tier to
    filter it by, and a declared tool nothing staged is a handler that was
    dropped or renamed without the contract moving with it.
    """
    declared = {tool.name for tool in tools()}
    staged = set(registered)

    report: list[str] = []
    if extra := sorted(staged - declared):
        report.append("staged but not declared: " + ", ".join(extra))
    if missing := sorted(declared - staged):
        report.append("declared but not staged: " + ", ".join(missing))

    if report:
        msg = "staged tools do not match the contract: " + "; ".join(report)
        raise RouteError(msg)
