"""Run a tool from the TUI through the shared dispatch.

The run screen calls ``execute`` with the arguments the form produced; this
module owns building the request through the live server's ``dispatch`` (the
same chokepoint Phase 1 ``call`` uses) and classifying the result. It is kept
separate from the Textual view so the dispatch-and-classify path is unit
testable without a running terminal.

Nothing here reimplements tool logic: it dispatches and then reuses the Phase 1
``_shared`` helpers to pull the text payload, decide success vs tool-error, and
render the JSON/table view.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum, auto
from typing import TYPE_CHECKING, Any

from linodemcp.cli._shared import is_error_result, render_output, result_text

if TYPE_CHECKING:
    from linodemcp.server import Server


class RunStatus(Enum):
    """Outcome of a TUI tool run.

    ``SUCCESS`` and ``TOOL_ERROR`` mirror the CLI's exit 0 / exit 1 split (a
    handler ran and returned data vs an error payload). ``REFUSED`` is the
    dispatch ``ValueError`` path (unknown or profile-filtered tool).
    ``DISPATCH_ERROR`` is an unexpected exception from the handler.
    """

    SUCCESS = auto()
    TOOL_ERROR = auto()
    REFUSED = auto()
    DISPATCH_ERROR = auto()


@dataclass(frozen=True)
class RunResult:
    """The classified result of a run, ready for the result view.

    ``text`` is the raw payload (JSON or an error message). ``status`` drives
    the styling and any follow-up (e.g. a destructive plan that should offer an
    apply step). ``rendered`` is the payload formatted for the chosen output
    mode, so the view just displays it.
    """

    status: RunStatus
    text: str
    rendered: str

    @property
    def is_success(self) -> bool:
        """True when a handler ran and returned a non-error payload."""
        return self.status is RunStatus.SUCCESS


async def execute(
    server: Server,
    tool_name: str,
    arguments: dict[str, Any],
    *,
    output: str = "json",
) -> RunResult:
    """Dispatch ``tool_name`` with ``arguments`` and classify the result.

    Drives the live server's ``dispatch`` so the call gets the same audit,
    profile filter, and dry-run/two-stage middleware an MCP request gets. A
    ``ValueError`` is the refused-tool path; any other exception is an
    unexpected handler error. On a clean return, the payload is checked against
    the error-text convention and rendered for the view.
    """
    try:
        result = await server.dispatch(tool_name, arguments)
    except ValueError as exc:
        text = str(exc)
        return RunResult(RunStatus.REFUSED, text, text)
    except Exception as exc:
        # Surface any handler crash to the UI rather than letting it bubble out
        # of the app loop; the run screen renders it as a dispatch error.
        text = f"dispatch failed: {exc}"
        return RunResult(RunStatus.DISPATCH_ERROR, text, text)

    text = result_text(result)
    if is_error_result(text):
        return RunResult(RunStatus.TOOL_ERROR, text, text)
    return RunResult(RunStatus.SUCCESS, text, render_output(text, output))
