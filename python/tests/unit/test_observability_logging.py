"""Console tracebacks must not dump frame locals.

A failing frame can hold config values, including the API token, and rich's
default traceback rendering prints every local in every frame. The console
renderer is configured with show_locals=False so an exception log line
carries the message and exception text only. Separate from
test_observability.py, which mocks the opentelemetry modules at import time.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import structlog

from linodemcp.config import LoggingConfig, ObservabilityConfig
from linodemcp.observability import Observability

if TYPE_CHECKING:
    import pytest


def _fail_with_secret_local() -> None:
    """Raise from a frame whose locals hold a canary token.

    The canary is assembled at runtime so it exists only as a frame local,
    the way a real token loaded from config does; rich also renders source
    context lines, so a literal canary would appear via the source, not the
    locals dump this test pins.
    """
    secret_token = "-".join(("linode", "token", "must", "not", "leak"))
    msg = f"boom {len(secret_token)}"
    raise ValueError(msg)


def test_console_exception_log_omits_frame_locals(
    capsys: pytest.CaptureFixture[str],
) -> None:
    obs = Observability(
        ObservabilityConfig(logging=LoggingConfig(level="info", format="console"))
    )
    try:
        _fail_with_secret_local()
    except ValueError as exc:
        obs.logger.exception("startup step failed", error=str(exc))

    captured = capsys.readouterr().err
    assert "startup step failed" in captured
    assert "linode-token-must-not-leak" not in captured

    # Leave a clean slate for any later test that configures logging.
    structlog.reset_defaults()
