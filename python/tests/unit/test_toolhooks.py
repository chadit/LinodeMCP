"""The Python half of the hook contract, from both directions.

A tool declares the hook KINDS it needs through ``tool_hooks`` on its *Input
message, and ``test_every_declared_hook_is_implemented`` below holds every
declared kind to having a function in ``linodemcp.toolhooks``, which the
emitter used to refuse at build time. That covers one direction. A function
implemented there that no tool declares has no such backstop, and it reads as
behavior a tool has when nothing calls it, so that is what these tests close.

The Go twin is ``TestContractNamesEveryImplementedHook`` in
``go/internal/toolhooks``. Both read the descriptors rather than a list, so
neither can go stale against the other.
"""

from __future__ import annotations

import importlib
import inspect

from linodemcp import toolhooks
from linodemcp.linode import routes

# The one hook kind the contract defines today.
KIND_VALIDATE = "validate"

# The one sentence the image hook answers for every unusable id, where Go's
# twin names three and accepts a third prefix. Both halves of that divergence
# are what the two handlers have always answered.
IMAGE_ID_REJECTION = "image_id must be a valid Linode image ID"


def _hook_name(tool: str, kind: str) -> str:
    """The function one tool's hook of a kind lives under.

    Spelled here rather than imported from the emitter because that is the
    point: the emitter derives the name it calls, and this derives the name it
    expects to find, so a change to the rule has to reach both.
    """
    return f"{tool}_{kind.replace('-', '_')}"


def _declared_hooks() -> set[str]:
    """Every function name the tool_hooks options resolve to.

    Read through the same contract reader the server and the emitter use, so a
    hook declared on a message this walk cannot reach would be missing from all
    three rather than only from here.
    """
    return {
        _hook_name(tool.name, kind)
        for tool in routes.tools()
        for kind in routes.contract_for(tool.name).hooks
    }


def _implemented_hooks() -> set[str]:
    """The public functions the hooks module defines, read from the module."""
    module = importlib.import_module(toolhooks.__name__)
    return {
        name
        for name, value in inspect.getmembers(module, inspect.isfunction)
        if not name.startswith("_") and value.__module__ == module.__name__
    }


def test_contract_declares_a_hook_at_all() -> None:
    """Zero-measurement guard: with no hook declared the comparison below holds
    trivially and would keep holding after the seam broke."""
    assert _declared_hooks()


def test_every_declared_hook_is_implemented() -> None:
    """The emitter refuses without this, and this says so where a reader looks."""
    assert _declared_hooks() <= _implemented_hooks()


def test_every_implemented_hook_is_declared() -> None:
    """A function nothing declares is never called: only generated code calls a
    hook, and the emitter writes the call from the declaration."""
    assert _implemented_hooks() <= _declared_hooks()


_INVALID_LABEL_CHAR = (
    "label contains an invalid character; use letters, digits, '_', '-', or '.'"
)
