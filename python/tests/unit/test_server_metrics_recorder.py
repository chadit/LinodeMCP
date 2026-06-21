"""The dispatch chokepoint must drive the injected metrics recorder.

Regression guard for the metrics-recording wiring: record_tool_call was dead
code because the Server held no recorder. Dispatching a tool must now drive the
injected recorder, otherwise the request metrics never move.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from linodemcp.server import Server

if TYPE_CHECKING:
    from linodemcp.config import Config


class _FakeRecorder:
    """Captures recorder calls so the test can assert dispatch drove them."""

    def __init__(self) -> None:
        """Start with empty capture lists."""
        self.tool_calls: list[tuple[str, bool]] = []
        self.api_calls: list[str] = []

    def record_tool_call(self, tool: str, duration_seconds: float, error: bool) -> None:
        """Capture a tool-call recording."""
        del duration_seconds
        self.tool_calls.append((tool, error))

    def record_api_request(
        self, endpoint: str, method: str, status: int, duration_seconds: float
    ) -> None:
        """Capture an API-request recording."""
        del method, status, duration_seconds
        self.api_calls.append(endpoint)


async def test_dispatch_records_tool_call(sample_config: Config) -> None:
    """Dispatching a tool drives record_tool_call with the tool name."""
    srv = Server(sample_config)
    recorder = _FakeRecorder()
    srv.set_metrics_recorder(recorder)

    await srv.dispatch("hello", {"name": "x"})

    assert ("hello", False) in recorder.tool_calls


async def test_set_metrics_recorder_none_restores_noop(
    sample_config: Config,
) -> None:
    """Passing None restores the no-op rather than crashing the next dispatch."""
    srv = Server(sample_config)
    recorder = _FakeRecorder()
    srv.set_metrics_recorder(recorder)
    srv.set_metrics_recorder(None)

    await srv.dispatch("hello", {"name": "x"})

    assert recorder.tool_calls == []
