"""Pure model logic for the TUI, with no Textual import.

The screens (``app.py``) are thin views over this module. Everything that can
be tested without a running terminal lives here: building the catalog from the
registry, filtering it, turning a tool's input schema into a list of form
fields, and the most important piece, mapping a filled form back to the exact
arguments dict the Phase 1 ``call`` command would build.

Reuse, not reimplementation: ``form_to_arguments`` calls the same
``_shared.coerce_arg_pairs`` / ``fold_safety_flags`` the CLI uses, so a form
filled in the TUI and the same values passed to ``linodemcp call`` produce an
identical request. The TUI never builds its own coercion or safety-flag logic.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any

from linodemcp.cli._shared import (
    coerce_arg_pairs,
    fold_safety_flags,
    required_args,
    schema_properties,
)
from linodemcp.profiles import Capability
from linodemcp.profiles.builtin import categories
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from collections.abc import Sequence

    from mcp.types import Tool

# The label shown for tools that match no known category prefix. Kept here so
# the catalog and any test referring to the bucket use one spelling.
UNCATEGORIZED = "uncategorized"

# Mode choices for the two-stage write control. ``none`` means the field is not
# sent at all (a normal call); ``plan``/``apply`` map to the MCP ``mode`` field.
MODE_CHOICES = ("none", "plan", "apply")


@dataclass(frozen=True)
class CatalogEntry:
    """One tool in the catalog: its name, capability, and category.

    ``capability_label`` is the lowercased capability name (read/write/destroy/
    admin/meta) the catalog renders as a tag. ``is_destructive`` flags the
    tools the run screen must preview before applying.
    """

    name: str
    capability: Capability
    category: str

    @property
    def capability_label(self) -> str:
        """Lowercase capability tag for display."""
        return self.capability.name.lower()

    @property
    def is_destructive(self) -> bool:
        """True for Destroy-capability tools (plan/preview before apply)."""
        return self.capability == Capability.Destroy


def _primary_category(tool_name: str) -> str:
    """Return a tool's primary category for grouping, or ``UNCATEGORIZED``.

    Uses the public ``categories`` helper (the same category mapping the
    profile builder uses) and takes the first match, since the catalog groups
    each tool under one node. A tool that matches no category falls into the
    ``uncategorized`` bucket rather than vanishing.
    """
    cats = categories(tool_name)
    return cats[0] if cats else UNCATEGORIZED


def build_catalog(*, allowed: frozenset[str] | None) -> list[CatalogEntry]:
    """Build the catalog from the registry, optionally profile-filtered.

    With ``allowed`` set, only tools in that set are included (the active
    profile's surface). With ``allowed`` None, the full registry is returned
    (the "preview full surface" toggle). Entries are sorted by category then
    name so the grouped view is stable.
    """
    entries: list[CatalogEntry] = []
    for tool_entry in get_tool_registry():
        if allowed is not None and tool_entry.name not in allowed:
            continue
        category = _primary_category(tool_entry.name)
        entries.append(
            CatalogEntry(
                name=tool_entry.name,
                capability=tool_entry.capability,
                category=category,
            )
        )
    entries.sort(key=lambda e: (e.category, e.name))
    return entries


def filter_catalog(
    entries: Sequence[CatalogEntry],
    query: str,
) -> list[CatalogEntry]:
    """Return entries whose name or category contains ``query`` (case-insensitive).

    An empty or whitespace query returns every entry unchanged, so clearing the
    search box restores the full list. The match is a plain substring over the
    tool name and its category, which covers "show me the volume tools" and
    "show me everything in networking" without a query language.
    """
    needle = query.strip().lower()
    if not needle:
        return list(entries)
    return [
        e for e in entries if needle in e.name.lower() or needle in e.category.lower()
    ]


def group_by_category(
    entries: Sequence[CatalogEntry],
) -> list[tuple[str, list[CatalogEntry]]]:
    """Group entries by category, preserving the sorted order.

    Returns a list of ``(category, entries)`` pairs in category order, each
    inner list in name order. The catalog tree renders one node per category
    with the tools beneath it.
    """
    grouped: dict[str, list[CatalogEntry]] = {}
    for entry in entries:
        grouped.setdefault(entry.category, []).append(entry)
    return [(category, grouped[category]) for category in sorted(grouped)]


def lookup_tool(name: str) -> Tool | None:
    """Return the registered ``Tool`` definition for ``name``, or None.

    Uses the full registry so the form screen can render any tool's schema even
    when the catalog is showing the profile-filtered subset.
    """
    for tool_entry in get_tool_registry():
        if tool_entry.name == name:
            return tool_entry.tool
    return None


@dataclass
class FormField:
    """One editable field derived from a tool's input-schema property.

    ``json_type`` is the schema's declared type (``string``/``integer``/
    ``number``/``boolean``/...), used both to render the right widget and to
    coerce the entered text. ``required`` marks fields the schema lists as
    required. ``value`` holds the current text the user has entered (always a
    string in the widget; coercion happens at submit).
    """

    name: str
    json_type: str
    required: bool
    description: str
    value: str = ""

    @property
    def label(self) -> str:
        """Display label: ``name (type)`` with a ``*`` suffix when required."""
        suffix = " *" if self.required else ""
        return f"{self.name} ({self.json_type}){suffix}"


# Argument names the form must not render as plain fields: the safety controls
# own them, and the CLI folds them in via dedicated flags. Mirrors the Phase 1
# safety-flag set so the form and the CLI agree on which keys are "controls".
_SAFETY_ARG_NAMES = frozenset(
    {
        "dry_run",
        "confirm",
        "mode",
        "plan_id",
        "confirmed_dry_run",
        "yolo",
        "environment",
    }
)


def build_form_fields(tool: Tool) -> list[FormField]:
    """Build the editable fields for a tool, one per schema property.

    Safety-control keys (dry_run, confirm, mode, ...) are excluded: the form's
    dedicated safety controls own them so they are not duplicated as free-text
    fields. Required fields sort first, then alphabetical, so the user sees what
    they must fill at the top.
    """
    properties = schema_properties(tool)
    required = set(required_args(tool))

    fields: list[FormField] = []
    for name in properties:
        if name in _SAFETY_ARG_NAMES:
            continue
        prop = properties[name]
        json_type = str(prop.get("type", "string"))
        description = str(prop.get("description", ""))
        fields.append(
            FormField(
                name=name,
                json_type=json_type,
                required=name in required,
                description=description,
            )
        )
    fields.sort(key=lambda f: (not f.required, f.name))
    return fields


@dataclass
class SafetyControls:
    """The form's safety controls, mirroring the Phase 1 ``call`` flags.

    ``mode`` is one of ``MODE_CHOICES``; ``none`` means do not send a mode.
    ``environment`` is the empty string when unset (no environment key sent).
    These map one-to-one to the ``--dry-run``/``--confirm``/``--mode``/...
    flags, so the same dispatch gates apply.
    """

    dry_run: bool = False
    confirm: bool = False
    mode: str = "none"
    plan_id: str = ""
    confirmed_dry_run: bool = False
    yolo: bool = False
    environment: str = ""


def _empty_fields() -> list[FormField]:
    """Typed default factory for ``FormState.fields`` (pyright wants a concrete
    return type, which bare ``list`` does not provide under strict mode)."""
    return []


@dataclass
class FormState:
    """The full editable state of the tool form: fields plus safety controls."""

    tool_name: str
    fields: list[FormField] = field(default_factory=_empty_fields)
    safety: SafetyControls = field(default_factory=SafetyControls)


class FormValidationError(ValueError):
    """A form could not be turned into arguments (missing required field).

    Raised by ``form_to_arguments`` before any coercion so the run screen can
    show the problem without dispatching.
    """


def _collect_pairs(fields: Sequence[FormField]) -> list[str]:
    """Turn filled fields into the ``key=value`` pairs ``coerce_arg_pairs`` wants.

    Empty fields are skipped (an unset optional argument is simply absent). This
    reuses the exact CLI coercion path rather than re-implementing typing in the
    TUI, so a form value and the same ``--arg key=value`` coerce identically.
    """
    pairs: list[str] = []
    for fld in fields:
        if fld.value == "":
            continue
        pairs.append(f"{fld.name}={fld.value}")
    return pairs


def _check_required(fields: Sequence[FormField]) -> None:
    """Raise ``FormValidationError`` if any required field is still empty."""
    missing = [fld.name for fld in fields if fld.required and fld.value == ""]
    if missing:
        joined = ", ".join(missing)
        msg = f"missing required field(s): {joined}"
        raise FormValidationError(msg)


def form_to_arguments(tool: Tool, state: FormState) -> dict[str, Any]:
    """Map a filled form to the dispatch arguments dict.

    This is the contract test target: the result must equal what Phase 1
    ``call`` builds from the same field values and safety flags. It reuses the
    CLI's ``coerce_arg_pairs`` (typing per the tool schema) and
    ``fold_safety_flags`` (the MCP-keyed safety fields), so the two front-ends
    can never diverge in how they shape a request.

    Raises ``FormValidationError`` when a required field is empty.
    """
    _check_required(state.fields)
    pairs = _collect_pairs(state.fields)
    base = coerce_arg_pairs(pairs, schema_properties(tool))

    safety = state.safety
    mode = safety.mode if safety.mode in ("plan", "apply") else None
    plan_id = safety.plan_id or None
    environment = safety.environment or None

    return fold_safety_flags(
        base,
        dry_run=safety.dry_run,
        confirm=safety.confirm,
        mode=mode,
        plan_id=plan_id,
        confirmed_dry_run=safety.confirmed_dry_run,
        yolo=safety.yolo,
        environment=environment,
    )


def configured_environments(env_names: Sequence[str]) -> list[str]:
    """Return the environment picker's choices: a blank default plus names.

    The blank first entry means "send no environment key" (the tool uses its
    default). The rest are the configured environment names so the user can pick
    one without typing.
    """
    return ["", *env_names]
