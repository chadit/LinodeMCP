"""MCP server implementation for LinodeMCP."""

from __future__ import annotations

import asyncio
import inspect
import logging
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from datetime import timedelta
from typing import TYPE_CHECKING, Any, Protocol, cast

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server
from mcp.types import CallToolResult, ListToolsResult, TextContent, Tool

import linodemcp.gentools as gentools_module
import linodemcp.tools as tools_module
from linodemcp.audit import Capability as AuditCapability
from linodemcp.audit import Mode, NoopSink, Sink, Status, new_event
from linodemcp.linode import RetryableClient
from linodemcp.linode.metrics import reset_api_recorder, set_api_recorder
from linodemcp.linode.routes import validate as validate_tool_contract
from linodemcp.linode.routes import validate_registered
from linodemcp.profiles import (
    Capability,
    Profile,
    Scope,
    ScopeValidationResult,
    TokenNotConfiguredError,
    ToolDescriptor,
    resolve_active_profile,
    validate_scopes,
)
from linodemcp.profiles.builder import Registry as DraftRegistry
from linodemcp.tools.builderstate import (
    BuilderState,
    reset_builder_state,
    set_builder_state,
)
from linodemcp.twostage import reset_plan_store, set_plan_store
from linodemcp.twostage.store import PlanStore
from linodemcp.version import VERSION as LINODEMCP_VERSION

if TYPE_CHECKING:
    from types import ModuleType

    from mcp.server import ServerRequestContext
    from mcp.types import CallToolRequestParams, PaginatedRequestParams

    from linodemcp.config import Config

__all__ = ["Server", "ToolEntry", "get_tool_registry"]

logger = logging.getLogger(__name__)

# Factories run once at module import, matching the Go side's
# "factory called once at registration" semantics.
ToolFactory = Callable[[], tuple[Tool, Capability]]


@dataclass(frozen=True)
class ToolEntry:
    """A registered tool's name, MCP definition, capability tag, and handler.

    ``tool`` is already materialized; factories are not re-invoked per request.
    ``capability`` is ``Capability.Unknown`` for tools still on the untagged
    allowlist.
    """

    name: str
    tool: Tool
    capability: Capability
    handle_fn: Callable[..., Awaitable[list[Any]]]
    # API tools take ``(arguments, cfg)``; CapMeta tools that never touch the
    # Linode API take ``(arguments,)``. Dispatch reads this for the right
    # arity, computed once at registry build rather than per request.
    takes_config: bool


def _build_tool_registry() -> list[ToolEntry]:
    """Discover and instantiate every registered tool at import time.

    A tool is registered by exporting its ``create_*_tool`` / ``handle_*`` pair
    from a registration module; there is intentionally no per-route table here.
    Two modules are scanned because a tool is either hand-written in
    ``linodemcp.tools`` or emitted into ``linodemcp.gentools`` by
    go/cmd/toolgen from the proto contract. Reading both the same way is
    what lets a tool move between them without this module changing.
    """
    create_fns: dict[str, ToolFactory] = {}
    handle_fns: dict[str, Callable[..., Awaitable[list[Any]]]] = {}

    for module in (tools_module, gentools_module):
        _collect_registrations(module, create_fns, handle_fns)

    entries: list[ToolEntry] = []
    for tool_name in sorted(create_fns.keys()):
        create_fn = create_fns[tool_name]
        handle_fn = handle_fns.get(tool_name)
        if handle_fn is None:
            logger.warning("No handler found for tool: %s", tool_name)
            continue
        tool, capability = create_fn()
        entries.append(
            ToolEntry(
                name=tool_name,
                tool=tool,
                capability=capability,
                handle_fn=handle_fn,
                takes_config=_handler_takes_config(handle_fn),
            )
        )

    return entries


def _collect_registrations(
    module: ModuleType,
    create_fns: dict[str, ToolFactory],
    handle_fns: dict[str, Callable[..., Awaitable[list[Any]]]],
) -> None:
    """Read one registration module's exported create/handle pairs.

    A name the module exports but does not define is skipped rather than
    raising; the caller's create/handle pairing is what reports a
    half-registered tool.
    """
    for name in getattr(module, "__all__", []):
        fn = getattr(module, name, None)
        if fn is None:
            continue
        if name.startswith("create_") and name.endswith("_tool"):
            # create_linode_instance_list_tool -> linode_instance_list
            create_fns[name[len("create_") : -len("_tool")]] = cast("ToolFactory", fn)
        elif name.startswith("handle_"):
            # handle_linode_instance_list -> linode_instance_list
            handle_fns[name[len("handle_") :]] = fn


def _handler_takes_config(handle_fn: Callable[..., Awaitable[list[Any]]]) -> bool:
    """Report whether a tool handler accepts the Config second positional arg.

    API handlers are ``handle_x(arguments, cfg)``; CapMeta handlers that never
    touch the Linode API are ``handle_x(arguments)``. Assuming every handler
    takes config crashed the CapMeta tools.
    """
    arity_with_config = 2
    positional = [
        param
        for param in inspect.signature(handle_fn).parameters.values()
        if param.kind
        in (inspect.Parameter.POSITIONAL_ONLY, inspect.Parameter.POSITIONAL_OR_KEYWORD)
    ]

    return len(positional) >= arity_with_config


def _destroy_bypass_message(tool_name: str) -> str:
    """The error a CapDestroy tool returns when confirm:true arrives without a
    prior dry-run assertion or an explicit bypass. Mirrors the Go
    destroyBypassMessage exactly."""
    return (
        f"{tool_name} is destructive. Either:\n"
        "  1. Call with dry_run: true first to preview, then call again with\n"
        "     confirm: true, confirmed_dry_run: true\n"
        "  2. Call with confirm: true, confirm_bypass_dry_run: true to skip preview\n"
        "  3. Use yolo: true (only if profile allows)"
    )


def _destroy_bypass_error(tool_name: str, arguments: dict[str, Any]) -> str | None:
    """Enforce the bypass-dry-run gate for a CapDestroy tool.

    Returns an error message to short-circuit dispatch, or None to let the call
    reach the handler. The no-confirm/no-bypass case returns None so the
    handler's own confirm message still fires. Mirrors the Go
    requireDestroyConfirmation logic.
    """
    confirm = arguments.get("confirm") is True
    confirmed = arguments.get("confirmed_dry_run") is True
    bypass = arguments.get("confirm_bypass_dry_run") is True

    if bypass and confirmed:
        return (
            "Pass either confirm_bypass_dry_run (skip preview) or "
            "confirmed_dry_run (preview was done), not both"
        )

    if not confirm:
        if bypass:
            return "confirm_bypass_dry_run only takes effect with confirm: true"
        return None

    if not confirmed and not bypass:
        return _destroy_bypass_message(tool_name)

    return None


_TOOL_REGISTRY = _build_tool_registry()


def _elapsed_ms(start_ns: int) -> int:
    """Compute elapsed milliseconds from a monotonic-ns start tick."""
    return (time.monotonic_ns() - start_ns) // 1_000_000


def _audit_capability(capability: Capability) -> AuditCapability:
    """Translate the profiles capability tag into the audit-wire form.

    Mirrors the Go ``profilesCapabilityToAudit``. It lives here rather than in
    the audit package so audit keeps no dependency on profiles.
    """
    match capability:
        case Capability.Read:
            return AuditCapability.READ
        case Capability.Write:
            return AuditCapability.WRITE
        case Capability.Destroy:
            return AuditCapability.DESTROY
        case Capability.Admin:
            return AuditCapability.ADMIN
        case Capability.Meta:
            return AuditCapability.META
        case _:
            return AuditCapability.READ


def get_tool_registry() -> list[ToolEntry]:
    """Return the eagerly-built registry for tests and introspection.

    Callers must not mutate the returned list; it is the registry built once at
    module import, not a copy.
    """
    return _TOOL_REGISTRY


class MetricsRecorder(Protocol):
    """Records metrics for tool dispatch and the Linode API calls a tool makes.

    Server depends on this narrow protocol rather than the concrete
    Observability type so tests can inject a fake recorder.
    """

    def record_tool_call(self, tool: str, duration_seconds: float, error: bool) -> None:
        """Record a completed tool dispatch."""
        ...

    def record_api_request(
        self, endpoint: str, method: str, status: int, duration_seconds: float
    ) -> None:
        """Record a completed Linode API request."""
        ...


class NoopMetricsRecorder:
    """Records nothing; the default for a Server without observability."""

    def record_tool_call(self, tool: str, duration_seconds: float, error: bool) -> None:
        """Discard tool-call metrics."""
        del tool, duration_seconds, error

    def record_api_request(
        self, endpoint: str, method: str, status: int, duration_seconds: float
    ) -> None:
        """Discard API-request metrics."""
        del endpoint, method, status, duration_seconds


class Server:
    """LinodeMCP server."""

    def __init__(self, config: Config) -> None:
        if not config:
            msg = "config cannot be None"
            raise ValueError(msg)

        self.config = config
        self.mcp = MCPServer(
            config.server.name,
            on_list_tools=self._on_list_tools,
            on_call_tool=self._on_call_tool,
        )
        self._inflight = 0
        # Both default to no-ops so a Server built outside main (tests)
        # dispatches unchanged; main wires the JSONL sink and the real
        # Observability at startup.
        self._audit_sink: Sink = NoopSink()
        self._metrics: MetricsRecorder = NoopMetricsRecorder()
        # False keeps credential-only redaction for Servers built outside main;
        # main flips it to cfg.audit.redact_pii, which defaults to True.
        self._audit_redact_pii: bool = False
        self._plan_store = PlanStore()
        self._idle = asyncio.Event()
        self._idle.set()

        # Hot-reload takes this so a config-watcher firing in one task can't
        # race a tools/list arriving from the transport task.
        self._reload_lock = asyncio.Lock()

        # The active profile resolves against the full registry so registration
        # skips everything outside the allow list. A bad config raises out of
        # the resolver; let that propagate.
        self._descriptors = [
            ToolDescriptor(name=entry.name, capability=entry.capability)
            for entry in _TOOL_REGISTRY
        ]
        # The proto contract names the whole tool surface, so checking the
        # registry against it here fails construction on drift instead of
        # letting the gap surface as an unfilterable tool or an uncalled route.
        # Both raise RouteError.
        validate_tool_contract()
        validate_registered(entry.name for entry in _TOOL_REGISTRY)
        # One Registry per server process, shared by every draft handler.
        # Drafts do not persist across restarts.
        self._draft_registry = DraftRegistry()
        self._active_profile = resolve_active_profile(config, self._descriptors)
        # Built once and kept, so every dispatch publishes the same state and a
        # draft started by one call is there for the next. The catalog and
        # active-profile members are read at call time, so reload_profile
        # reaches an already-registered tool.
        self._builder_state = BuilderState(
            drafts=self._draft_registry,
            catalog=lambda: self._descriptors,
            active_profile=lambda: self._active_profile,
        )
        self._allowed_tool_names = frozenset(self._active_profile.allowed_tools)
        # _allowed_entries and _config_handlers are declared inside
        # _apply_active_profile so their annotations live in one place; reload
        # reuses the same helper.
        self._apply_active_profile(emit_filter_log=True)

    @property
    def active_profile(self) -> Profile:
        """Resolved profile the server is running under."""
        return self._active_profile

    @property
    def registered_tool_names(self) -> frozenset[str]:
        """Names of tools the active profile allowed through registration."""
        return self._allowed_tool_names

    def _yolo_active(self, arguments: dict[str, Any]) -> bool:
        """Report whether this call is a permitted yolo execution: yolo:true and
        the profile's allow_yolo. yolo:true alone falls through to the normal
        gate."""
        return arguments.get("yolo") is True and self._active_profile.allow_yolo

    def _execution_mode(self, arguments: dict[str, Any]) -> Mode:
        """Derive the audit execution mode from the call's flags (mirrors the
        Go executionMode). yolo wins only when permitted."""
        if self._yolo_active(arguments):
            return Mode.YOLO
        mode = arguments.get("mode")
        if mode == "apply":
            return Mode.APPLY
        if mode == "plan":
            return Mode.PLAN
        if arguments.get("dry_run") is True:
            return Mode.DRY_RUN
        if arguments.get("confirm_bypass_dry_run") is True:
            return Mode.BYPASS_DRY_RUN
        return Mode.NORMAL

    async def dispatch(self, name: str, arguments: dict[str, Any]) -> list[Any]:
        """Invoke a registered tool handler with in-flight tracking.

        The in-flight count is what lets shutdown() drain active requests before
        the process exits. Public so tests can drive dispatch without the stdio
        MCP transport. Every call that gets this far builds an audit Event at
        entry and writes it to ``_audit_sink`` at exit, with the status
        reflecting the outcome.
        """
        self._inflight += 1
        self._idle.clear()

        start_ns = time.monotonic_ns()
        environment = arguments.get("environment", "") if arguments else ""
        if not isinstance(environment, str):
            environment = ""

        event = new_event(
            tool=name,
            capability=self._capability_for(name),
            args=arguments,
            environment=environment,
            profile=self._active_profile.name,
            session_id="",
            credential_generation=0,
            linodemcp_version=LINODEMCP_VERSION,
            redact_pii=self._audit_redact_pii,
        )
        event.set_mode(self._execution_mode(arguments), "")

        plan_store_token = set_plan_store(self._plan_store)
        builder_state_token = set_builder_state(self._builder_state)
        # Bound per dispatch so the client records each Linode API round trip
        # it makes. Mirrors the Go WithAPIRecorder context value.
        api_recorder_token = set_api_recorder(self._metrics)
        try:
            result = await self._dispatch_inner(name, arguments)
            elapsed_ms = _elapsed_ms(start_ns)
            event.finalize(Status.SUCCESS, elapsed_ms, "", "")
            self._audit_sink.write(event)
            self._metrics.record_tool_call(name, elapsed_ms / 1000.0, error=False)
            return result
        except ValueError as exc:
            # _dispatch_inner raises ValueError for unknown or filtered tool
            # names. The handler never ran, so this audits as refused and
            # records no tool-call metric.
            event.finalize(Status.REFUSED, _elapsed_ms(start_ns), str(exc), "")
            self._audit_sink.write(event)
            raise
        except Exception as exc:
            elapsed_ms = _elapsed_ms(start_ns)
            event.finalize(Status.ERROR, elapsed_ms, str(exc), "")
            self._audit_sink.write(event)
            self._metrics.record_tool_call(name, elapsed_ms / 1000.0, error=True)
            raise
        finally:
            reset_api_recorder(api_recorder_token)
            reset_builder_state(builder_state_token)
            reset_plan_store(plan_store_token)
            self._inflight -= 1
            if self._inflight == 0:
                self._idle.set()

    def set_audit_sink(self, sink: Sink | None) -> None:
        """Swap the audit sink.

        Passing None restores the NoopSink default rather than producing a
        None-deref on the next call.
        """
        self._audit_sink = sink if sink is not None else NoopSink()

    def set_metrics_recorder(self, recorder: MetricsRecorder | None) -> None:
        """Wire the recorder used for tool-dispatch and API metrics.

        The serve path passes the real Observability so each call lands on the
        meter exposed at /metrics. Passing None restores the no-op default
        rather than producing a None-deref on the next dispatch.
        """
        self._metrics = recorder if recorder is not None else NoopMetricsRecorder()

    def set_audit_redact_pii(self, redact_pii: bool) -> None:
        """Select the redaction tier the capture middleware applies to event
        args. Main wires this to ``cfg.audit.redact_pii`` at startup.
        """
        self._audit_redact_pii = redact_pii

    def _capability_for(self, name: str) -> AuditCapability:
        """Translate the registered tool's capability into the audit wire form.

        Unknown or filtered tools fall back to READ; those calls are marked
        refused anyway, so the value is not load-bearing there.
        """
        for entry in self._allowed_entries:
            if entry.name == name:
                return _audit_capability(entry.capability)
        return AuditCapability.READ

    async def validate_scopes(self) -> ScopeValidationResult:
        """Validate the active token's scopes.

        Builds a client from the default environment and delegates to
        ``profiles.validate_scopes`` for the PAT-vs-OAuth dispatch. Raises
        ``TokenNotConfiguredError`` without making an API call when the
        environment has no token; main decides whether that fails load
        (elevated profile) or only warns (read-only). ``ProfileFetchError`` /
        ``GrantsFetchError`` come from the API calls themselves.
        """
        cfg = self.config
        env = cfg.environments.get("default")
        if env is None:
            msg = "default environment is required for scope validation"
            raise TokenNotConfiguredError(msg)
        if not env.linode.token:
            raise TokenNotConfiguredError(
                "active environment has no Linode token configured"
            )

        required = [Scope(s) for s in self._active_profile.required_token_scopes]

        client = RetryableClient(env.linode.api_url, env.linode.token)
        try:
            return await validate_scopes(client, required)
        finally:
            await client.close()

    async def shutdown(self, timeout: float = 10.0) -> bool:
        """Wait for in-flight tool handlers to complete.

        Returns True if drain finished cleanly, False on timeout. Callers
        decide what to do with a timeout (log, force-cutoff, etc.).
        """
        if self._inflight == 0:
            return True
        try:
            await asyncio.wait_for(self._idle.wait(), timeout=timeout)
        except TimeoutError:
            return False
        return True

    async def _dispatch_inner(self, name: str, arguments: dict[str, Any]) -> list[Any]:
        """Resolve a tool name to its handler and await the result.

        Every tool reaches its handler through the registry, and the allow list
        is what a profile that omits one is refused by.
        """
        if name not in self._allowed_tool_names:
            msg = f"Unknown tool: {name}"
            raise ValueError(msg)
        match name:
            case _ if name in self._config_handlers:
                two_stage_mode = arguments.get("mode") in ("plan", "apply")
                gated = (
                    name in self._destroy_tools
                    and arguments.get("dry_run") is not True
                    and not two_stage_mode
                )
                if gated:
                    if self._yolo_active(arguments):
                        # Permitted yolo bypasses the gate AND the handler's
                        # per-handler confirm requirement.
                        arguments = {**arguments, "confirm": True}
                    else:
                        gate_error = _destroy_bypass_error(name, arguments)
                        if gate_error is not None:
                            # "Error: " framing matches error_response and the
                            # Go side, where the gate returns an error result.
                            return [
                                TextContent(type="text", text=f"Error: {gate_error}")
                            ]

                handler = self._config_handlers[name]
                if self._config_takes_config[name]:
                    return await handler(arguments, self.config)
                return await handler(arguments)
            case _:
                msg = f"Unknown tool: {name}"
                raise ValueError(msg)

    def _apply_active_profile(self, *, emit_filter_log: bool) -> None:
        """Rebuild ``_allowed_entries`` and ``_config_handlers`` from the
        registry filtered by the active profile.

        Startup logs the filtered-out tools; ``reload_profile`` does not, since
        that would spam the log on every config edit. The derived list and dicts
        are read from mutable instance state on each request, so the swap takes
        effect without re-registering anything.
        """
        allowed_entries: list[ToolEntry] = []

        for entry in _TOOL_REGISTRY:
            if entry.name not in self._allowed_tool_names:
                if emit_filter_log:
                    logger.info(
                        "[profile=%s] filtered out tool: %s",
                        self._active_profile.name,
                        entry.name,
                    )
                continue
            allowed_entries.append(entry)

        self._allowed_entries: list[ToolEntry] = allowed_entries
        self._config_handlers: dict[str, Callable[..., Awaitable[list[Any]]]] = {
            entry.name: entry.handle_fn for entry in allowed_entries
        }
        self._config_takes_config: dict[str, bool] = {
            entry.name: entry.takes_config for entry in allowed_entries
        }
        # CapDestroy tools enforce the Phase 3 bypass-dry-run gate at dispatch
        # (Python has no shared destroy helper, so dispatch is the chokepoint).
        self._destroy_tools: frozenset[str] = frozenset(
            entry.name
            for entry in allowed_entries
            if entry.capability == Capability.Destroy
        )

    async def _on_list_tools(
        self,
        ctx: ServerRequestContext[Any],
        params: PaginatedRequestParams | None,
    ) -> ListToolsResult:
        """Return the tools the active profile allows.

        Reads ``self._allowed_entries`` on every call rather than capturing it,
        so ``reload_profile`` only has to swap that list for the next
        ``tools/list`` to reflect the new profile.
        """
        del ctx, params
        return ListToolsResult(tools=[entry.tool for entry in self._allowed_entries])

    async def _on_call_tool(
        self,
        ctx: ServerRequestContext[Any],
        params: CallToolRequestParams,
    ) -> CallToolResult:
        """Dispatch via the tracked path so shutdown can drain it.

        SDK 2.0 raises handler exceptions as JSON-RPC errors instead of turning
        them into ``isError`` results. Catching here keeps the wire shape the Go
        server produces: a normal result carrying the message with ``is_error``
        set, which the model can read and self-correct from.
        """
        del ctx
        try:
            content = await self.dispatch(params.name, dict(params.arguments or {}))
        except Exception as exc:
            return CallToolResult(
                content=[TextContent(type="text", text=str(exc))],
                is_error=True,
            )
        return CallToolResult(content=content)

    async def reload_profile(self, config: Config) -> None:
        """Swap the running server to the profile resolved from ``config``.

        Profile, allow list, allowed entries, and handler map all swap together
        under ``_reload_lock``, so the next ``tools/list`` and ``call_tool`` see
        the new set. On a resolver error nothing is mutated and the server keeps
        its current profile. Handlers already past the dispatch gate keep
        running; the lock only serializes reload against tools/list.
        """
        async with self._reload_lock:
            new_profile = resolve_active_profile(config, self._descriptors)

            previous = self._active_profile.name
            self._active_profile = new_profile
            self._allowed_tool_names = frozenset(new_profile.allowed_tools)
            self.config = config
            self._apply_active_profile(emit_filter_log=False)

            logger.info(
                "profile reloaded: previous=%s current=%s live=%d",
                previous,
                new_profile.name,
                len(self._allowed_entries),
            )

    async def start(self) -> None:
        """Start the MCP server using stdio transport."""
        logger.info(
            "Starting LinodeMCP server with %d tools (profile=%s)",
            len(self._allowed_entries),
            self._active_profile.name,
        )

        janitor = self._plan_store.start_janitor(timedelta(minutes=1))
        try:
            async with stdio_server() as (read_stream, write_stream):
                await self.mcp.run(
                    read_stream,
                    write_stream,
                    self.mcp.create_initialization_options(),
                )
        finally:
            janitor.cancel()
