"""MCP server implementation for LinodeMCP."""

from __future__ import annotations

import asyncio
import inspect
import logging
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any, cast

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server
from mcp.types import TextContent, Tool

import linodemcp.tools as tools_module
from linodemcp.audit import Capability as AuditCapability
from linodemcp.audit import Mode, NoopSink, Sink, Status, new_event
from linodemcp.config import get_config_path
from linodemcp.linode import RetryableClient
from linodemcp.profiles import (
    Capability,
    Profile,
    Scope,
    ScopeValidationResult,
    TokenNotConfiguredError,
    ToolDescriptor,
    lookup_profile,
    resolve_active_profile,
    validate_scopes,
)
from linodemcp.profiles.builder import Registry as DraftRegistry
from linodemcp.tools import (
    handle_hello,
    handle_version,
)
from linodemcp.tools.linode_profile_builder import set_tool_catalog_provider
from linodemcp.tools.linode_profile_can_run import (
    set_can_run_active_profile_provider,
    set_can_run_catalog_provider,
)
from linodemcp.tools.linode_profile_draft import (
    set_draft_registry,
    set_profile_resolver,
)
from linodemcp.tools.linode_profile_draft_mutate import set_mutator_catalog_provider
from linodemcp.tools.linode_profile_draft_save import set_save_config_path_provider
from linodemcp.version import VERSION as LINODEMCP_VERSION

if TYPE_CHECKING:
    from linodemcp.config import Config

__all__ = ["Server", "ToolEntry", "get_tool_registry"]

logger = logging.getLogger(__name__)

# The MCP library's list_tools() and call_tool() methods lack return type
# annotations. These type aliases let us cast them to their actual signatures
# (verified from the library source) instead of suppressing type errors.
ListToolsDecorator = Callable[
    [Callable[[], Awaitable[list[Tool]]]],
    Callable[[], Awaitable[list[Tool]]],
]
CallToolDecorator = Callable[
    [Callable[..., Awaitable[list[Any]]]],
    Callable[..., Awaitable[list[Any]]],
]

# Each tool factory now returns (Tool, Capability). We invoke every factory
# once at module import time and store the resolved Tool plus its capability,
# matching the Go side's "factory called once at registration" semantics.
ToolFactory = Callable[[], tuple[Tool, Capability]]


@dataclass(frozen=True)
class ToolEntry:
    """A registered tool's name, MCP definition, capability tag, and handler.

    The ``tool`` field holds the already-materialized ``Tool`` instance;
    factories are not re-invoked per request. The ``capability`` field is
    ``Capability.Unknown`` for any tool still on the Phase 1 untagged
    allowlist; category PRs replace those with real capabilities.
    """

    name: str
    tool: Tool
    capability: Capability
    handle_fn: Callable[..., Awaitable[list[Any]]]
    # takes_config is True when the handler's signature accepts the Config
    # second argument. API tools take ``(arguments, cfg)``; CapMeta tools
    # that never touch the Linode API take ``(arguments,)`` only. Dispatch
    # reads this so it calls each handler with the right arity (computed
    # once at registry build, not per request).
    takes_config: bool


def _build_tool_registry() -> list[ToolEntry]:
    """Discover and instantiate every registered tool at import time.

    Scans ``linodemcp.tools.__all__`` for ``create_*_tool`` / ``handle_*``
    pairs, invokes each factory once to materialize the ``(Tool, Capability)``
    tuple, and stores it alongside the matching handler. New route tools are
    registered by exporting the matching create/handle pair from that module;
    there is intentionally no per-route server registry table to update.
    """
    # ``linodemcp.tools.__all__`` is the production registration surface:
    # exported create/handle pairs below become MCP tools without a per-route
    # table in this module.
    all_names = getattr(tools_module, "__all__", [])

    create_fns: dict[str, ToolFactory] = {}
    handle_fns: dict[str, Callable[..., Awaitable[list[Any]]]] = {}

    for name in all_names:
        if name.startswith("create_") and name.endswith("_tool"):
            # create_linode_instances_list_tool -> linode_instances_list
            tool_name = name[len("create_") : -len("_tool")]
            fn = getattr(tools_module, name, None)
            if fn is not None:
                create_fns[tool_name] = cast("ToolFactory", fn)
        elif name.startswith("handle_"):
            # handle_linode_instances_list -> linode_instances_list
            tool_name = name[len("handle_") :]
            fn = getattr(tools_module, name, None)
            if fn is not None:
                handle_fns[tool_name] = fn

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


def _handler_takes_config(handle_fn: Callable[..., Awaitable[list[Any]]]) -> bool:
    """Report whether a tool handler accepts the Config second positional arg.

    API handlers are ``async def handle_x(arguments, cfg)``; CapMeta handlers
    that never touch the Linode API are ``async def handle_x(arguments)``.
    Dispatch uses this to call each handler with the correct arity instead of
    assuming every handler takes config (which crashed CapMeta tools).
    """
    # arguments + cfg; a handler with both positional params takes config.
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
    prior dry-run assertion or an explicit bypass. Tells the model the three
    ways forward. Mirrors the Go destroyBypassMessage exactly."""
    return (
        f"{tool_name} is destructive. Either:\n"
        "  1. Call with dry_run: true first to preview, then call again with\n"
        "     confirm: true, confirmed_dry_run: true\n"
        "  2. Call with confirm: true, confirm_bypass_dry_run: true to skip preview\n"
        "  3. Use yolo: true (only if profile allows)"
    )


def _destroy_bypass_error(tool_name: str, arguments: dict[str, Any]) -> str | None:
    """Enforce the Phase 3 bypass-dry-run gate for a CapDestroy tool.

    Returns an error message to short-circuit dispatch, or None to let the
    call proceed to the handler. Returns None for the no-confirm/no-bypass
    case so the handler's own (tool-specific) confirm message still fires.
    Mirrors the Go requireDestroyConfirmation logic.
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

    Mirrors the Go ``profilesCapabilityToAudit`` helper. Kept in the
    server module rather than the audit package so the audit package
    stays dependency-free of profiles.
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

    Each ``ToolEntry`` carries the materialized ``Tool``, its
    ``Capability`` tag, and the request handler. Callers must not mutate
    the returned list; treat it as a snapshot of the registry built once
    at module import.
    """
    return _TOOL_REGISTRY


class Server:
    """LinodeMCP server."""

    def __init__(self, config: Config) -> None:
        if not config:
            msg = "config cannot be None"
            raise ValueError(msg)

        self.config = config
        self.mcp = MCPServer(config.server.name)
        self._inflight = 0
        # Phase 1b: audit sink defaults to NoopSink so dispatch logs
        # nothing yet. Phase 2 swaps in the JSONL writer; tests inject
        # CapturingSink via set_audit_sink before exercising dispatch.
        self._audit_sink: Sink = NoopSink()
        # Phase 4c: PII redaction tier flag. Default False so tests
        # that build a Server without going through main keep
        # credential-only redaction. main flips it to
        # cfg.audit.redact_pii at startup (default True unless the
        # operator opts out).
        self._audit_redact_pii: bool = False
        self._idle = asyncio.Event()
        self._idle.set()

        # Phase 5 hot-reload uses this lock so a config-watcher firing in one
        # task can't race a tools/list arriving from the transport task. The
        # mutation block is small (frozenset rebuild + dict swap); contention
        # is incidental.
        self._reload_lock = asyncio.Lock()

        # Phase 4: resolve the active profile against the full registry so
        # _register_tools can skip everything outside the allow list. The
        # resolver raises ActiveProfileUnknownError or
        # ActiveProfileDisabledError on a bad config; let those propagate.
        self._descriptors = [
            ToolDescriptor(name=entry.name, capability=entry.capability)
            for entry in _TOOL_REGISTRY
        ]
        # Phase 8.2: wire the builder catalog bridge before profile
        # resolution so handlers fired during startup (none today, but
        # cheap insurance) see the live catalog. The descriptor list is
        # immutable after construction; the closure captures the same
        # list every handler call reads.
        set_tool_catalog_provider(lambda: self._descriptors)
        # Phase 8.3: wire the draft registry and clone-source resolver
        # so the _draft_new/_show/_discard handlers see the same
        # registry instance and can resolve clone_from against the
        # live config + descriptor list. One Registry per server
        # process; drafts do not persist across restarts.
        self._draft_registry = DraftRegistry()
        set_draft_registry(self._draft_registry)
        set_profile_resolver(
            lambda name: lookup_profile(name, config, self._descriptors)
        )
        # Phase 8.4: wire the mutator catalog bridge. _draft_add_tools
        # expands wildcards against the live catalog at call time.
        set_mutator_catalog_provider(lambda: self._descriptors)
        # Phase 8.5: wire the save tool to the config path. Save reads
        # fresh from disk on every call to avoid stomping concurrent
        # edits, then writes back via write_atomic. The provider is a
        # lambda over get_config_path so LINODEMCP_CONFIG_PATH env
        # overrides apply at call time.
        set_save_config_path_provider(lambda: str(get_config_path()))
        # Phase 3 (dry-run spec): wire the pre-check tool's catalog and
        # active-profile bridges. The active-profile lambda reads
        # self._active_profile at call time, so it reflects reload_profile.
        set_can_run_catalog_provider(lambda: self._descriptors)
        set_can_run_active_profile_provider(lambda: self._active_profile)
        self._active_profile = resolve_active_profile(config, self._descriptors)
        self._allowed_tool_names = frozenset(self._active_profile.allowed_tools)
        # _allowed_entries and _config_handlers are declared+initialized
        # inside _apply_active_profile so the type annotations live in one
        # place. Reload reuses the same helper to swap state.
        self._apply_active_profile(emit_filter_log=True)
        self._register_tools()

    @property
    def active_profile(self) -> Profile:
        """Resolved profile the server is running under.

        Used by tests today; Phase 5 hot-reload and the future audit
        middleware will read this too.
        """
        return self._active_profile

    @property
    def registered_tool_names(self) -> frozenset[str]:
        """Names of tools the active profile allowed through registration."""
        return self._allowed_tool_names

    def _yolo_active(self, arguments: dict[str, Any]) -> bool:
        """Report whether this call is a permitted yolo execution: yolo:true
        AND the active profile's allow_yolo. yolo:true alone (profile disallows)
        is NOT a permitted yolo and falls through to the normal gate."""
        return arguments.get("yolo") is True and self._active_profile.allow_yolo

    def _execution_mode(self, arguments: dict[str, Any]) -> Mode:
        """Derive the audit execution mode from the call's flags (mirrors the
        Go executionMode). yolo wins only when permitted."""
        if self._yolo_active(arguments):
            return Mode.YOLO
        if arguments.get("dry_run") is True:
            return Mode.DRY_RUN
        if arguments.get("confirm_bypass_dry_run") is True:
            return Mode.BYPASS_DRY_RUN
        return Mode.NORMAL

    async def dispatch(self, name: str, arguments: dict[str, Any]) -> list[Any]:
        """Invoke a registered tool handler with in-flight tracking.

        Wraps the handler call so shutdown() can drain active requests
        before the process exits. Public so tests can drive the dispatch
        path without going through the stdio MCP transport.

        Phase 1b adds audit-event capture around the inner dispatch:
        every reaching tool call builds an Event at entry and writes
        it to ``_audit_sink`` at exit, with status reflecting the
        outcome (success / error / refused).
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

        try:
            result = await self._dispatch_inner(name, arguments)
            event.finalize(Status.SUCCESS, _elapsed_ms(start_ns), "", "")
            self._audit_sink.write(event)
            return result
        except ValueError as exc:
            # _dispatch_inner raises ValueError for unknown / filtered
            # tool names. Audit as refused, not error: the handler
            # never ran.
            event.finalize(Status.REFUSED, _elapsed_ms(start_ns), str(exc), "")
            self._audit_sink.write(event)
            raise
        except Exception as exc:
            event.finalize(Status.ERROR, _elapsed_ms(start_ns), str(exc), "")
            self._audit_sink.write(event)
            raise
        finally:
            self._inflight -= 1
            if self._inflight == 0:
                self._idle.set()

    def set_audit_sink(self, sink: Sink | None) -> None:
        """Swap the audit sink.

        Phase 2 main wires this to the JSONL writer at startup; tests
        inject a CapturingSink before exercising the dispatch path.
        Passing None restores the NoopSink default rather than
        producing a None-deref on the next call.
        """
        self._audit_sink = sink if sink is not None else NoopSink()

    def set_audit_redact_pii(self, redact_pii: bool) -> None:
        """Select the redaction tier the capture middleware applies to
        event args (Phase 4c). Main wires this to
        ``cfg.audit.redact_pii`` at startup; tests use it to opt into
        PII redaction when asserting the combined-redaction path.
        """
        self._audit_redact_pii = redact_pii

    def _capability_for(self, name: str) -> AuditCapability:
        """Translate the registered tool's capability into the audit wire form.

        Returns ``CapabilityRead`` for unknown / filtered tools as a
        defensive default; those calls also get marked refused, so
        the capability value isn't load-bearing in the refusal path.
        """
        for entry in self._allowed_entries:
            if entry.name == name:
                return _audit_capability(entry.capability)
        return AuditCapability.READ

    async def validate_scopes(self) -> ScopeValidationResult:
        """Phase 6.4c: validate the active token's scopes.

        Builds a Linode client from the default environment in the
        current config and delegates to ``profiles.validate_scopes``
        for the PAT-vs-OAuth dispatch.

        Raises ``TokenNotConfiguredError`` (no API call made) when the
        active environment has no token set; the caller (main) decides
        whether to fail load (elevated profile) or warn-and-continue
        (read-only) per the missing-token policy.

        Other exceptions (``ProfileFetchError`` / ``GrantsFetchError``)
        come from the underlying API calls.
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

        ``hello`` and ``version`` keep their direct fast path because they
        take no config argument; they still go through the allow list so a
        profile that omits them cannot reach them via ``dispatch``.
        """
        if name not in self._allowed_tool_names:
            msg = f"Unknown tool: {name}"
            raise ValueError(msg)
        match name:
            case "hello":
                return await handle_hello(arguments)
            case "version":
                return await handle_version(arguments)
            case _ if name in self._config_handlers:
                if name in self._destroy_tools and arguments.get("dry_run") is not True:
                    if self._yolo_active(arguments):
                        # Permitted yolo bypasses the gate AND the handler's
                        # per-handler confirm requirement.
                        arguments = {**arguments, "confirm": True}
                    else:
                        gate_error = _destroy_bypass_error(name, arguments)
                        if gate_error is not None:
                            return [TextContent(type="text", text=gate_error)]

                handler = self._config_handlers[name]
                if self._config_takes_config[name]:
                    return await handler(arguments, self.config)
                return await handler(arguments)
            case _:
                msg = f"Unknown tool: {name}"
                raise ValueError(msg)

    def _apply_active_profile(self, *, emit_filter_log: bool) -> None:
        """Rebuild ``_allowed_entries`` and ``_config_handlers`` from the
        registry filtered by the current active profile.

        Called once at startup (with logging) and again on each successful
        ``reload_profile`` (without re-logging the filter rationale, which
        would spam logs on every config edit). The two derived dicts/lists
        feed ``_list_tools`` and ``_dispatch_inner``, both of which read
        mutable instance state so the swap takes effect on the next request
        without re-registering decorators.
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

    def _register_tools(self) -> None:
        """Wire the MCP server's list_tools and call_tool decorators.

        Both decorated callables read mutable instance state
        (``self._allowed_entries`` and ``self.dispatch``), so a later
        ``reload_profile`` only needs to swap that state; the decorators
        themselves stay registered for the lifetime of the server.
        """
        _list_tools_method = cast(
            "Callable[[], ListToolsDecorator]", self.mcp.list_tools
        )

        async def _list_tools() -> list[Tool]:
            return [entry.tool for entry in self._allowed_entries]

        _list_tools_method()(_list_tools)

        async def _call_tool(name: str, arguments: dict[str, Any]) -> list[Any]:
            """Dispatch via the tracked path so Shutdown can drain it."""
            return await self.dispatch(name, arguments)

        cast("CallToolDecorator", self.mcp.call_tool())(_call_tool)

    async def reload_profile(self, config: Config) -> None:
        """Swap the running server to the profile resolved from ``config``.

        On success, the active profile, allow list, allowed entries, and
        dispatch handler map are all updated atomically under
        ``_reload_lock``. The next ``tools/list`` request returns the new
        set; subsequent ``call_tool`` invocations check the new allow list.

        On error, no state is mutated. The caller sees the original
        resolver exception (``ActiveProfileUnknownError``,
        ``ActiveProfileDisabledError``, etc.); the running server keeps its
        current profile.

        In-flight tool handlers that already passed the dispatch gate
        continue to run unaffected; the lock only serializes reload steps
        and tools/list requests.
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

        async with stdio_server() as (read_stream, write_stream):
            await self.mcp.run(
                read_stream,
                write_stream,
                self.mcp.create_initialization_options(),
            )
