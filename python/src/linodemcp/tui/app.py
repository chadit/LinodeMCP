"""Textual app and screens for ``linodemcp tui``.

Three screens make up the Phase 2 shell: a catalog of tools (grouped by
category, searchable, profile-filtered with a full-surface toggle), a tool form
built from the selected tool's schema, and a result view that runs the call
through the shared dispatch and shows the rendered output. A destructive tool
gets a preview-before-apply step.

The views are deliberately thin. All logic lives in ``model`` (catalog,
filtering, form fields, form-to-arguments) and ``run`` (dispatch + classify),
both unit tested without a terminal. This module wires those into Textual
widgets and handles navigation. Screens never reach for ``self.app`` (its
generic type defeats pyright strict); instead they post a typed navigation
``Message`` the app handles, which keeps the strict checkers happy without any
suppressions.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, cast

from textual.app import App, ComposeResult
from textual.containers import VerticalScroll
from textual.message import Message
from textual.screen import Screen
from textual.widgets import (
    Button,
    Checkbox,
    DataTable,
    Footer,
    Header,
    Input,
    Label,
    Select,
    Static,
)

from linodemcp.tui import extras, model
from linodemcp.tui.run import RunStatus, execute
from linodemcp.version import get_version_info

if TYPE_CHECKING:
    from linodemcp.tui.model import CatalogEntry, FormField
    from linodemcp.tui.runtime import TuiRuntime

# Output mode the TUI renders results in. JSON keeps the payload faithful.
_DEFAULT_OUTPUT = "json"


class OpenForm(Message):
    """A catalog selection that asks the app to open a tool's form."""

    def __init__(self, entry: CatalogEntry) -> None:
        super().__init__()
        self.entry = entry


class OpenResult(Message):
    """A form submission that asks the app to run a tool and show the result."""

    def __init__(self, entry: CatalogEntry, arguments: dict[str, object]) -> None:
        super().__init__()
        self.entry = entry
        self.arguments = arguments


class OpenAudit(Message):
    """A request from the catalog to open the audit viewer."""


class OpenProfiles(Message):
    """A request from the catalog to open the profile switcher."""


class OpenHealth(Message):
    """A request from the catalog to open the health/metrics view."""


class ProfileSwitched(Message):
    """The profile switcher reports a successful active-profile change.

    Carries the new name so the app can re-resolve the catalog's filtered view
    against the updated config.
    """

    def __init__(self, name: str) -> None:
        super().__init__()
        self.name = name


class CatalogScreen(Screen[None]):
    """The tool catalog: search box plus a grouped, capability-tagged table.

    ``f`` toggles between the active-profile surface and the full registry.
    Selecting a row posts ``OpenForm``. The catalog data is held in this
    screen's own typed list and indexed by cursor row, so Textual's dynamic row
    keys never leak into the model.
    """

    BINDINGS = [  # noqa: RUF012 - Textual reads BINDINGS as a class attribute
        ("f", "toggle_surface", "Full surface"),
        ("a", "open_audit", "Audit"),
        ("p", "open_profiles", "Profiles"),
        ("h", "open_health", "Health"),
        ("escape", "app.quit", "Quit"),
    ]

    def __init__(self, runtime: TuiRuntime) -> None:
        super().__init__()
        self._runtime = runtime
        self._show_all = False
        self._visible: list[CatalogEntry] = []

    def action_open_audit(self) -> None:
        """Open the audit viewer."""
        self.post_message(OpenAudit())

    def action_open_profiles(self) -> None:
        """Open the profile switcher."""
        self.post_message(OpenProfiles())

    def action_open_health(self) -> None:
        """Open the health/metrics view."""
        self.post_message(OpenHealth())

    def compose(self) -> ComposeResult:
        """Lay out the header, search box, catalog table, and footer."""
        yield Header()
        yield Input(placeholder="filter tools (name or category)", id="filter")
        yield DataTable[str](id="catalog")
        yield Footer()

    def on_mount(self) -> None:
        """Initialize the table columns, load the catalog, and focus the table.

        The table (not the filter box) takes focus so the single-key bindings
        (``f`` to toggle the surface) fire instead of being typed into the
        search field. Tab or a click moves focus to the filter to search.
        """
        table = cast("DataTable[str]", self.query_one("#catalog", DataTable))
        table.cursor_type = "row"
        table.add_columns("tool", "capability", "category")
        self._reload()
        table.focus()

    def _current_allowed(self) -> frozenset[str] | None:
        """The allow-set for the catalog: None for full surface, else the
        active profile's tools (read live from the open server)."""
        if self._show_all:
            return None
        return frozenset(self._runtime.server.active_profile.allowed_tools)

    def _reload(self) -> None:
        """Rebuild the catalog from the registry and re-apply the filter."""
        query = self.query_one("#filter", Input).value
        self._apply(query)

    def _apply(self, query: str) -> None:
        """Filter the catalog for ``query`` and repaint the rows."""
        entries = model.build_catalog(allowed=self._current_allowed())
        self._visible = model.filter_catalog(entries, query)
        self._render_rows()

    def _render_rows(self) -> None:
        """Repopulate the table from ``self._visible``."""
        table = cast("DataTable[str]", self.query_one("#catalog", DataTable))
        table.clear()
        for entry in self._visible:
            table.add_row(entry.name, entry.capability_label, entry.category)
        surface = "full registry" if self._show_all else "active profile"
        self.sub_title = f"{len(self._visible)} tools ({surface})"

    def on_input_changed(self, event: Input.Changed) -> None:
        """Re-filter the catalog as the search box changes."""
        if event.input.id == "filter":
            self._apply(event.value)

    def on_data_table_row_selected(self, event: DataTable.RowSelected) -> None:
        """Post ``OpenForm`` for the selected tool by indexing the visible list."""
        row = event.cursor_row
        if 0 <= row < len(self._visible):
            self.post_message(OpenForm(self._visible[row]))

    def action_toggle_surface(self) -> None:
        """Flip between the active-profile surface and the full registry."""
        self._show_all = not self._show_all
        self._reload()

    def on_screen_resume(self) -> None:
        """Re-render when the catalog regains focus (e.g. after a profile switch).

        The profile switcher reloads the live server's active profile, so the
        catalog's profile-filtered surface may have changed; reloading here
        picks that up when the user returns. Skipped while showing the full
        registry, which is profile-independent.
        """
        if not self._show_all:
            self._reload()


class FormScreen(Screen[None]):
    """A form built from a tool's input schema, plus the safety controls.

    One input per schema property (required ones marked), and dedicated
    controls for dry-run, mode, confirm, confirmed-dry-run, yolo, and the
    environment picker. ``Run`` builds the arguments via the model (identical to
    the CLI ``call``) and posts ``OpenResult``.
    """

    BINDINGS = [  # noqa: RUF012 - Textual reads BINDINGS as a class attribute
        ("escape", "app.pop_screen", "Back"),
    ]

    def __init__(self, runtime: TuiRuntime, entry: CatalogEntry) -> None:
        super().__init__()
        self._runtime = runtime
        self._entry = entry
        self._tool = model.lookup_tool(entry.name)
        self._fields: list[FormField] = (
            model.build_form_fields(self._tool) if self._tool is not None else []
        )

    def compose(self) -> ComposeResult:
        """Render the field inputs and safety controls inside a scroll view."""
        yield Header()
        with VerticalScroll(id="form"):
            yield Label(f"Tool: {self._entry.name} [{self._entry.capability_label}]")
            if self._entry.is_destructive:
                yield Static(
                    "Destructive: preview (dry-run) before apply.",
                    id="destructive-note",
                )
            yield from self._compose_fields()
            yield from self._compose_safety()
            yield Button("Run", id="run", variant="primary")
        yield Footer()

    def _compose_fields(self) -> ComposeResult:
        """Yield a labeled input per schema field."""
        for fld in self._fields:
            yield Label(fld.label)
            yield Input(placeholder=fld.description, id=f"field-{fld.name}")

    def _compose_safety(self) -> ComposeResult:
        """Yield the safety controls (toggles, mode, environment picker)."""
        yield Label("Safety controls")
        yield Checkbox("dry-run", id="dry_run")
        yield Checkbox("confirm", id="confirm")
        yield Checkbox("confirmed-dry-run", id="confirmed_dry_run")
        yield Checkbox("yolo", id="yolo")
        mode_options = [(m, m) for m in model.MODE_CHOICES]
        yield Label("mode")
        yield Select[str](mode_options, value="none", allow_blank=False, id="mode")
        yield Label("environment")
        env_choices = model.configured_environments(
            list(self._runtime.config.environments.keys())
        )
        env_options = [(e or "(default)", e) for e in env_choices]
        yield Select[str](env_options, value="", allow_blank=False, id="environment")

    def _read_state(self) -> model.FormState:
        """Read the widgets back into a ``FormState`` the model can map.

        Field values come from each input; the safety controls come from the
        checkboxes and selects. This is the only place that touches widgets;
        the mapping to arguments stays in the model.
        """
        for fld in self._fields:
            fld.value = self.query_one(f"#field-{fld.name}", Input).value
        safety = model.SafetyControls(
            dry_run=self._checkbox("dry_run"),
            confirm=self._checkbox("confirm"),
            mode=self._select_value("mode"),
            confirmed_dry_run=self._checkbox("confirmed_dry_run"),
            yolo=self._checkbox("yolo"),
            environment=self._select_value("environment"),
        )
        return model.FormState(
            tool_name=self._entry.name, fields=self._fields, safety=safety
        )

    def _checkbox(self, widget_id: str) -> bool:
        """Read a checkbox's boolean value by id."""
        return self.query_one(f"#{widget_id}", Checkbox).value

    def _select_value(self, widget_id: str) -> str:
        """Read a select's string value by id (blank when unset)."""
        select = cast("Select[str]", self.query_one(f"#{widget_id}", Select))
        value = select.value
        return value if isinstance(value, str) else ""

    def on_button_pressed(self, event: Button.Pressed) -> None:
        """Build the call and post ``OpenResult``, or report a form error."""
        if event.button.id != "run":
            return
        if self._tool is None:
            self.notify("tool not found in registry", severity="error")
            return
        state = self._read_state()
        try:
            arguments = model.form_to_arguments(self._tool, state)
        except model.FormValidationError as exc:
            self.notify(str(exc), severity="error")
            return
        self.post_message(OpenResult(self._entry, arguments))


class ResultScreen(Screen[None]):
    """Runs the call through the shared dispatch and shows the result.

    The run happens in ``on_mount`` so the screen appears immediately with a
    "running" note, then updates with the rendered payload. The status styles
    the header line (success vs tool-error vs refused).
    """

    BINDINGS = [  # noqa: RUF012 - Textual reads BINDINGS as a class attribute
        ("escape", "app.pop_screen", "Back"),
    ]

    def __init__(
        self,
        runtime: TuiRuntime,
        entry: CatalogEntry,
        arguments: dict[str, object],
    ) -> None:
        super().__init__()
        self._runtime = runtime
        self._entry = entry
        self._arguments = arguments

    def compose(self) -> ComposeResult:
        """Lay out the status line and a scrollable result body."""
        yield Header()
        yield Label("running...", id="status")
        with VerticalScroll(id="result-body"):
            yield Static("", id="result-text")
        yield Footer()

    async def on_mount(self) -> None:
        """Dispatch the call and render the classified result."""
        result = await execute(
            self._runtime.server,
            self._entry.name,
            dict(self._arguments),
            output=_DEFAULT_OUTPUT,
        )
        self.query_one("#status", Label).update(self._status_line(result.status))
        self.query_one("#result-text", Static).update(result.rendered)

    def _status_line(self, status: RunStatus) -> str:
        """Map a run status to a short header line for the result screen."""
        mapping = {
            RunStatus.SUCCESS: f"OK: {self._entry.name}",
            RunStatus.TOOL_ERROR: f"tool error: {self._entry.name}",
            RunStatus.REFUSED: f"refused: {self._entry.name}",
            RunStatus.DISPATCH_ERROR: f"dispatch error: {self._entry.name}",
        }
        return mapping[status]


class AuditScreen(Screen[None]):
    """A live view of recent audit events (the ``linode_audit_recent`` feed).

    Dispatches ``linode_audit_recent`` through the same shared ``execute`` the
    other screens use (it is CapMeta, so it runs under any profile), parses the
    payload with ``extras``, and renders one row per event. ``r`` refreshes the
    feed so the user sees what the TUI just did, including this view's own meta
    events (it requests ``include_meta``).
    """

    BINDINGS = [  # noqa: RUF012 - Textual reads BINDINGS as a class attribute
        ("r", "refresh", "Refresh"),
        ("escape", "app.pop_screen", "Back"),
    ]

    def __init__(self, runtime: TuiRuntime) -> None:
        super().__init__()
        self._runtime = runtime

    def compose(self) -> ComposeResult:
        """Lay out the header, the audit table, and the footer."""
        yield Header()
        yield DataTable[str](id="audit")
        yield Footer()

    async def on_mount(self) -> None:
        """Set up the columns and load the first batch of events."""
        table = cast("DataTable[str]", self.query_one("#audit", DataTable))
        table.cursor_type = "row"
        table.add_columns(*extras.AUDIT_COLUMNS)
        await self._load()

    async def _load(self) -> None:
        """Dispatch ``linode_audit_recent`` and repaint the table.

        Uses the shared ``execute`` so the read itself goes through dispatch
        (and is audited). ``include_meta`` is on so the user sees the TUI's own
        activity, which is the point of this screen.
        """
        result = await execute(
            self._runtime.server,
            "linode_audit_recent",
            {"limit": 50, "include_meta": True},
            output=_DEFAULT_OUTPUT,
        )
        rows = extras.parse_audit_events(result.text)
        table = cast("DataTable[str]", self.query_one("#audit", DataTable))
        table.clear()
        for row in rows:
            table.add_row(*row.as_cells())
        self.sub_title = f"{len(rows)} recent events"

    async def action_refresh(self) -> None:
        """Reload the audit feed."""
        await self._load()


class ProfileScreen(Screen[None]):
    """List the configured + built-in profiles and switch the active one.

    The list comes from ``extras.profile_rows`` (the same catalog ``profile
    list`` shows). Selecting a row switches the active profile via
    ``extras.switch_active_profile``, the same human-only config write the CLI
    ``profile use`` performs (no MCP tool exposes it). On success it posts
    ``ProfileSwitched`` so the app re-resolves the catalog's filtered view.
    """

    BINDINGS = [  # noqa: RUF012 - Textual reads BINDINGS as a class attribute
        ("escape", "app.pop_screen", "Back"),
    ]

    def __init__(self, runtime: TuiRuntime) -> None:
        super().__init__()
        self._runtime = runtime
        self._names: list[str] = []

    def compose(self) -> ComposeResult:
        """Lay out the header, an instruction line, the table, and the footer."""
        yield Header()
        yield Label(
            "Select a profile to make it active (writes the config file).",
            id="profile-help",
        )
        yield DataTable[str](id="profiles")
        yield Footer()

    def on_mount(self) -> None:
        """Set up the columns, load the profile list, and focus the table."""
        table = cast("DataTable[str]", self.query_one("#profiles", DataTable))
        table.cursor_type = "row"
        table.add_columns("active", "name", "state", "tools")
        self._load()
        table.focus()

    def _load(self) -> None:
        """Read the profile rows and repaint the table.

        Holds the names in row order so a selection can resolve the chosen name
        without Textual's dynamic row keys.
        """
        rows = extras.profile_rows(self._runtime.config_path)
        self._names = [row.name for row in rows]
        table = cast("DataTable[str]", self.query_one("#profiles", DataTable))
        table.clear()
        for row in rows:
            table.add_row(*row.as_cells())
        active = next((r.name for r in rows if r.active), "?")
        self.sub_title = f"active: {active}"

    def on_data_table_row_selected(self, event: DataTable.RowSelected) -> None:
        """Switch the active profile to the selected row's profile."""
        row = event.cursor_row
        if not 0 <= row < len(self._names):
            return
        name = self._names[row]
        try:
            extras.switch_active_profile(self._runtime.config_path, name)
        except extras.ProfileSwitchError as exc:
            self.notify(str(exc), severity="error")
            return
        self.notify(f"active profile switched to {name}")
        self._load()
        self.post_message(ProfileSwitched(name))

    def row_for_profile(self, name: str) -> int | None:
        """Return the table row index for a profile name, or None if absent.

        Public so a caller (or a test) can target a profile by name without
        reaching into the row-name list the selection handler uses.
        """
        try:
            return self._names.index(name)
        except ValueError:
            return None


class HealthScreen(Screen[None]):
    """Audit subsystem health plus the build/version info and a metrics pointer.

    Dispatches ``linode_audit_health`` through the shared ``execute`` and maps
    the payload with ``extras`` (jsonl path, disk bytes, rotated count, dropped
    counter, sqlite). Adds the version rows from the build info and a one-line
    pointer to the Prometheus endpoint (the TUI does not scrape it).
    """

    BINDINGS = [  # noqa: RUF012 - Textual reads BINDINGS as a class attribute
        ("escape", "app.pop_screen", "Back"),
    ]

    def __init__(self, runtime: TuiRuntime) -> None:
        super().__init__()
        self._runtime = runtime

    def compose(self) -> ComposeResult:
        """Lay out the header, the health table, the metrics pointer, footer."""
        yield Header()
        with VerticalScroll(id="health-body"):
            yield DataTable[str](id="health")
            yield Static("", id="metrics-pointer")
        yield Footer()

    async def on_mount(self) -> None:
        """Set up the table, dispatch health, and render the rows + pointer."""
        table = cast("DataTable[str]", self.query_one("#health", DataTable))
        table.add_columns("field", "value")

        result = await execute(
            self._runtime.server,
            "linode_audit_health",
            {},
            output=_DEFAULT_OUTPUT,
        )
        for row in extras.health_rows(result.text):
            table.add_row(row.label, row.value)
        table.add_row("", "")
        for row in extras.version_rows(get_version_info().to_dict()):
            table.add_row(row.label, row.value)

        metrics = self._runtime.config.observability.metrics
        pointer = extras.metrics_pointer(
            enabled=metrics.enabled,
            port=metrics.prometheus.port,
            path=metrics.prometheus.path,
        )
        self.query_one("#metrics-pointer", Static).update(pointer)
        self.sub_title = "audit health and build info"


class LinodeTUI(App[None]):
    """The top-level Textual app holding the open server for the session.

    The runtime (server + audit sink) is built once and held open for the whole
    session, so every screen's dispatch reuses it. The catalog is the start
    screen. Navigation is message-driven: screens post ``OpenForm`` /
    ``OpenResult`` and the app pushes the next screen, so screens never touch
    the generically-typed ``self.app``.
    """

    CSS = """
    #filter { dock: top; }
    #status { padding: 1 2; }
    """
    TITLE = "LinodeMCP"

    def __init__(self, runtime: TuiRuntime) -> None:
        super().__init__()
        self._runtime = runtime

    def on_mount(self) -> None:
        """Open the catalog as the initial screen."""
        self.push_screen(CatalogScreen(self._runtime))

    def on_open_form(self, message: OpenForm) -> None:
        """Push the tool form for the catalog's selected tool."""
        self.push_screen(FormScreen(self._runtime, message.entry))

    def on_open_result(self, message: OpenResult) -> None:
        """Push the result screen, which runs the call on mount."""
        self.push_screen(ResultScreen(self._runtime, message.entry, message.arguments))

    def on_open_audit(self, message: OpenAudit) -> None:
        """Push the audit viewer."""
        _ = message
        self.push_screen(AuditScreen(self._runtime))

    def on_open_profiles(self, message: OpenProfiles) -> None:
        """Push the profile switcher."""
        _ = message
        self.push_screen(ProfileScreen(self._runtime))

    def on_open_health(self, message: OpenHealth) -> None:
        """Push the health/metrics view."""
        _ = message
        self.push_screen(HealthScreen(self._runtime))

    async def on_profile_switched(self, message: ProfileSwitched) -> None:
        """Reload the live server's profile so the session reflects the switch.

        The switcher already wrote the config; this swaps the running server's
        active profile from the new file. The catalog re-reads the live active
        profile when it resumes, so its filtered surface follows the change.
        """
        _ = message
        await self._runtime.reload_profile_from_disk()
