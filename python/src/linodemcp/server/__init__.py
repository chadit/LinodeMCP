"""MCP server implementation for LinodeMCP."""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any, cast

from mcp.server import Server as MCPServer
from mcp.server.stdio import stdio_server
from mcp.types import Tool

import linodemcp.tools as tools_module
from linodemcp.linode import RetryableClient
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
from linodemcp.tools import (
    handle_hello,
    handle_version,
)

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


def _build_tool_registry() -> list[ToolEntry]:
    """Discover and instantiate every registered tool at import time.

    Scans ``linodemcp.tools.__all__`` for ``create_*_tool`` / ``handle_*``
    pairs, invokes each factory once to materialize the ``(Tool, Capability)``
    tuple, and stores it alongside the matching handler.
    """
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
            )
        )

    return entries


_TOOL_REGISTRY = _build_tool_registry()


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

    async def dispatch(self, name: str, arguments: dict[str, Any]) -> list[Any]:
        """Invoke a registered tool handler with in-flight tracking.

        Wraps the handler call so shutdown() can drain active requests
        before the process exits. Public so tests can drive the dispatch
        path without going through the stdio MCP transport.
        """
        self._inflight += 1
        self._idle.clear()
        try:
            return await self._dispatch_inner(name, arguments)
        finally:
            self._inflight -= 1
            if self._inflight == 0:
                self._idle.set()

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
                return await self._config_handlers[name](arguments, self.config)
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
