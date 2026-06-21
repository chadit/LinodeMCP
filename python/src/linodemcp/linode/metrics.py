"""Linode API-request metric recording wired through a context variable.

The server injects a recorder into the dispatch context so the client records
each API round trip without importing observability directly. Mirrors the Go
linode/metrics.go context wiring.
"""

import contextvars
from typing import Protocol


class APIRecorder(Protocol):
    """Records metrics for one Linode API HTTP round trip."""

    def record_api_request(
        self, endpoint: str, method: str, status: int, duration_seconds: float
    ) -> None:
        """Record a single API request."""
        ...


_api_recorder: contextvars.ContextVar[APIRecorder | None] = contextvars.ContextVar(
    "linode_api_recorder", default=None
)


def set_api_recorder(
    recorder: APIRecorder | None,
) -> contextvars.Token[APIRecorder | None]:
    """Bind the recorder for the current context; returns a reset token."""
    return _api_recorder.set(recorder)


def reset_api_recorder(token: contextvars.Token[APIRecorder | None]) -> None:
    """Restore the recorder bound before the matching set_api_recorder."""
    _api_recorder.reset(token)


def get_api_recorder() -> APIRecorder | None:
    """Return the recorder bound for the current context, or None."""
    return _api_recorder.get()


def metrics_endpoint(endpoint: str) -> str:
    """Strip the query string so the endpoint metric label stays path-level.

    Path IDs still vary, so the endpoint label is higher cardinality than
    method or status; tighten to route templates if series count becomes a
    concern.
    """
    return endpoint.split("?", 1)[0]
