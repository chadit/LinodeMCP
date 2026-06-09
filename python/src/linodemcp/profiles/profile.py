"""``Profile`` dataclass: the user- or server-defined permission record.

A ``Profile`` names a set of tools, environments, and execution-mode
permissions that controls what the connected AI client can do. Built-in
profiles live in ``linodemcp.profiles.builtin``; user-defined profiles are
loaded from config (Phase 3). This module owns only the value type; it has
no I/O and no dependency on the tool registry.

Tuples are used for the list-shaped fields so the dataclass stays
``frozen=True`` and hashable. Callers that need a mutable view should copy
into a ``list`` at the call site.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Profile:
    """Named permission record for the active MCP session.

    Fields mirror the Go ``profiles.Profile`` struct field-for-field so the
    cross-language parity test can compare JSON exports without translation.
    """

    name: str
    description: str
    allowed_tools: tuple[str, ...]
    allowed_environments: tuple[str, ...] = ()
    required_token_scopes: tuple[str, ...] = ()
    allow_yolo: bool = False
    disabled: bool = False
