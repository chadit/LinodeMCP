package cli

import (
	"context"
	"io"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// tuiScreen identifies which screen the TUI is showing. The catalog is the
// hub: form and run flow off it (catalog -> form -> run), and the Phase 3
// extras (audit, profile, health) open from it by global keys and return
// to it on back.
type tuiScreen int

const (
	screenCatalog tuiScreen = iota
	screenForm
	screenRun
	screenAudit
	screenProfile
	screenHealth
)

// chromeReservedRows is the number of terminal rows the header and footer
// occupy, subtracted from the window height when sizing the active
// screen's body so the chrome never overlaps the content.
const chromeReservedRows = 4

// tuiKeyMap holds the global key bindings the shell interprets before
// handing a message to the active screen. Screen-local keys (list filter,
// text entry, viewport scroll) are handled by the widgets themselves.
type tuiKeyMap struct {
	Quit    key.Binding
	Back    key.Binding
	Select  key.Binding
	Toggle  key.Binding
	Submit  key.Binding
	Confirm key.Binding
	Format  key.Binding
	Audit   key.Binding
	Profile key.Binding
	Health  key.Binding
	Refresh key.Binding
}

// defaultTUIKeyMap returns the shell's key bindings. The bindings are data
// so the help line and the Update switch read the same source.
func defaultTUIKeyMap() tuiKeyMap {
	return tuiKeyMap{
		Quit:    key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q", "quit")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Select:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Toggle:  key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "scope")),
		Submit:  key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "run")),
		Confirm: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
		Format:  key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "json/table")),
		Audit:   key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "audit")),
		Profile: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "profiles")),
		Health:  key.NewBinding(key.WithKeys("ctrl+h"), key.WithHelp("ctrl+h", "health")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
}

// tuiModel is the root Bubble Tea model. It owns the active screen and the
// three sub-models by pointer plus the open server and the configured
// environment names. Update and View take pointer receivers so the model
// is mutated in place across the message loop (and so gocritic's hugeParam
// stays quiet without copying the model on every message). The runtime
// stays open for the whole session; RunTUICommand closes it after the
// program exits, not here.
type tuiModel struct {
	srv     *server.Server
	envs    []string
	cfg     *config.Config
	keys    tuiKeyMap
	screen  tuiScreen
	catalog *catalogModel
	form    *formModel
	run     *runModel
	audit   *auditModel
	profile *profileModel
	health  *healthModel
	status  string
	width   int
	height  int
}

// newTUIModel builds the root model from an open runtime. It seeds the
// catalog from the server's tool views and captures the configured
// environment names for the form's environment picker. Returns a pointer
// because the model satisfies tea.Model with pointer receivers.
func newTUIModel(runtime *Runtime) *tuiModel {
	catalog := newCatalogModel(runtime.Server)

	return &tuiModel{
		srv:     runtime.Server,
		envs:    environmentNames(runtime),
		cfg:     runtime.Config,
		keys:    defaultTUIKeyMap(),
		screen:  screenCatalog,
		catalog: &catalog,
	}
}

// environmentNames returns the sorted names of the configured Linode
// environments, used to populate the form's environment picker. A runtime
// built from the no-config fallback has none, so the picker offers only
// the "leave unset" choice and Linode-API tools report the missing
// environment at call time.
func environmentNames(runtime *Runtime) []string {
	if runtime.Config == nil {
		return nil
	}

	names := make([]string, 0, len(runtime.Config.Environments))
	for name := range runtime.Config.Environments {
		names = append(names, name)
	}

	return sortedStrings(names)
}

// Init satisfies tea.Model. The shell has no startup command; the catalog
// list renders from its seeded items.
func (*tuiModel) Init() tea.Cmd {
	return nil
}

// Update is the root message router. Window-size messages relay to every
// screen so each lays out correctly; the dispatch-result message is
// handled by the run screen; key messages first try the global bindings,
// then fall through to the active screen. The model is mutated in place
// and returned, satisfying tea.Model with a pointer receiver.
func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(typed)
	case runResultMsg:
		m.run.handleResult(typed)
		m.status = resultStatus(typed)

		return m, nil
	case auditLoadedMsg:
		m.audit.handleLoaded(typed)

		return m, nil
	case healthLoadedMsg:
		m.health.handleLoaded(typed)

		return m, nil
	case profileSwitchedMsg:
		return m.handleProfileSwitched(typed)
	case tea.KeyPressMsg:
		return m.handleKey(typed)
	default:
		return m.routeToScreen(msg)
	}
}

// View renders the active screen wrapped in the shared header and footer.
func (m *tuiModel) View() tea.View {
	header := tuiHeaderStyle.Render(screenTitle(m.screen))
	footer := tuiFooterStyle.Render(m.footer())
	view := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, m.body(), footer))
	view.AltScreen = true

	return view
}

// handleProfileSwitched applies the outcome of a profile switch. On
// success it reloads the running server's profile (so the catalog's
// filtered view reflects the new active profile in place) and rebuilds the
// catalog and switcher from the reloaded config; a failure leaves the
// session unchanged and shows the error.
func (m *tuiModel) handleProfileSwitched(msg profileSwitchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.status = "profile switch failed: " + msg.name

		return m, nil
	}

	cfg, err := reloadServerProfile(m.srv, "")
	if err != nil {
		m.status = "switched to " + msg.name + " (takes effect next launch: " + err.Error() + ")"

		return m, nil
	}

	m.cfg = cfg
	m.rebuildCatalog()
	m.profile.rebuild(cfg)
	m.status = "active profile switched to " + msg.name

	return m, nil
}

// rebuildCatalog rebuilds the catalog from the server's current views,
// called after a profile reload so the profile-filtered list reflects the
// new active profile without restarting the TUI.
func (m *tuiModel) rebuildCatalog() {
	catalog := newCatalogModel(m.srv)
	catalog.setSize(m.width, max(m.height-chromeReservedRows, 1))
	m.catalog = &catalog
}

// handleResize records the new terminal size and lays out the active
// screen's body, reserving rows for the header and footer chrome.
func (m *tuiModel) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	bodyHeight := max(msg.Height-chromeReservedRows, 1)

	m.catalog.setSize(msg.Width, bodyHeight)

	if m.form != nil {
		m.form.setSize(msg.Width, bodyHeight)
	}

	if m.run != nil {
		m.run.setSize(msg.Width, bodyHeight)
	}

	if m.audit != nil {
		m.audit.setSize(msg.Width, bodyHeight)
	}

	if m.profile != nil {
		m.profile.setSize(msg.Width, bodyHeight)
	}

	if m.health != nil {
		m.health.setSize(msg.Width, bodyHeight)
	}

	return m, nil
}

// handleKey applies the global bindings valid in the current screen, then
// routes anything unhandled to the active screen. The quit binding is
// suppressed while the catalog filter is open so typing "q" into a filter
// doesn't exit the app.
func (m *tuiModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit) && !m.catalogIsFiltering() {
		return m, tea.Quit
	}

	switch m.screen {
	case screenCatalog:
		return m.handleCatalogKey(msg)
	case screenForm:
		return m.handleFormKey(msg)
	case screenRun:
		return m.handleRunKey(msg)
	case screenAudit:
		return m.handleAuditKey(msg)
	case screenProfile:
		return m.handleProfileKey(msg)
	case screenHealth:
		return m.handleHealthKey(msg)
	default:
		return m, nil
	}
}

// catalogIsFiltering reports whether the catalog screen is active with its
// text filter open, so global single-letter keys defer to the filter.
func (m *tuiModel) catalogIsFiltering() bool {
	return m.screen == screenCatalog && m.catalog.filtering()
}

// handleCatalogKey handles keys on the catalog screen: toggle the
// profile/full scope, open the form for the selected tool, or fall through
// to the list (navigation and filtering). The select and toggle keys defer
// to the list while its filter is open.
func (m *tuiModel) handleCatalogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.catalog.filtering() {
		switch {
		case key.Matches(msg, m.keys.Toggle):
			m.catalog.toggleScope()

			return m, nil
		case key.Matches(msg, m.keys.Select):
			return m.openForm()
		case key.Matches(msg, m.keys.Audit):
			return m.openAudit()
		case key.Matches(msg, m.keys.Profile):
			return m.openProfile()
		case key.Matches(msg, m.keys.Health):
			return m.openHealth()
		}
	}

	cmd := m.catalog.update(msg)

	return m, cmd
}

// openAudit opens the audit viewer and kicks off its first refresh, which
// dispatches linode_audit_recent through the shared dispatch.
func (m *tuiModel) openAudit() (tea.Model, tea.Cmd) {
	audit := newAuditModel(m.srv)
	audit.setSize(m.width, max(m.height-chromeReservedRows, 1))
	m.audit = &audit
	m.screen = screenAudit
	m.status = ""

	return m, m.audit.refreshCmd()
}

// openProfile opens the profile switcher, seeded from the session's config.
func (m *tuiModel) openProfile() (tea.Model, tea.Cmd) {
	profile := newProfileModel(m.cfg, "")
	profile.setSize(m.width, max(m.height-chromeReservedRows, 1))
	m.profile = &profile
	m.screen = screenProfile
	m.status = ""

	return m, nil
}

// openHealth opens the health view and kicks off its first refresh, which
// dispatches linode_audit_health through the shared dispatch.
func (m *tuiModel) openHealth() (tea.Model, tea.Cmd) {
	health := newHealthModel(m.srv)
	health.setSize(m.width, max(m.height-chromeReservedRows, 1))
	m.health = &health
	m.screen = screenHealth
	m.status = ""

	return m, m.health.refreshCmd()
}

// handleAuditKey handles keys on the audit viewer: back to the catalog,
// refresh the feed, or scroll the viewport.
func (m *tuiModel) handleAuditKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenCatalog
		m.status = ""

		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		return m, m.audit.refreshCmd()
	default:
		cmd := m.audit.update(msg)

		return m, cmd
	}
}

// handleProfileKey handles keys on the profile switcher: back to the
// catalog, switch to the highlighted profile, or move through the list.
// The select key defers to the list filter while it is open.
func (m *tuiModel) handleProfileKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back) && !m.profile.filtering():
		m.screen = screenCatalog
		m.status = ""

		return m, nil
	case key.Matches(msg, m.keys.Select) && !m.profile.filtering():
		return m.switchProfile()
	default:
		cmd := m.profile.update(msg)

		return m, cmd
	}
}

// switchProfile fires the config-write for the highlighted profile through
// RunProfileUse. The result arrives as a profileSwitchedMsg, which reloads
// the running server and rebuilds the catalog on success.
func (m *tuiModel) switchProfile() (tea.Model, tea.Cmd) {
	name, ok := m.profile.selectedName()
	if !ok {
		return m, nil
	}

	m.status = "switching to " + name + "..."

	return m, m.profile.switchCmd(name)
}

// handleHealthKey handles keys on the health view: back to the catalog,
// refresh the report, or scroll the viewport.
func (m *tuiModel) handleHealthKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenCatalog
		m.status = ""

		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		return m, m.health.refreshCmd()
	default:
		cmd := m.health.update(msg)

		return m, cmd
	}
}

// openForm transitions from the catalog to the form for the highlighted
// tool, building the form fields from that tool's schema. A no-op (stay on
// the catalog) when the list is empty because every tool filtered out.
func (m *tuiModel) openForm() (tea.Model, tea.Cmd) {
	item, ok := m.catalog.selected()
	if !ok || item.meta == nil {
		return m, nil
	}

	form := newFormModel(item.meta.name, item.meta.schema, m.envs)
	form.setSize(m.width, max(m.height-chromeReservedRows, 1))
	m.form = &form
	m.screen = screenForm
	m.status = ""

	return m, nil
}

// handleFormKey handles keys on the form screen: back to the catalog, move
// focus between fields and controls, toggle the focused control, or
// submit. Plain typing and the field cursor are handled by the focused
// text input via the form's update.
func (m *tuiModel) handleFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	pressed := msg.Key()

	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenCatalog

		return m, nil
	case key.Matches(msg, m.keys.Submit):
		return m.submitForm()
	case pressed.Code == tea.KeyTab && pressed.Mod == 0:
		m.form.focusNext()

		return m, nil
	case pressed.Code == tea.KeyTab && pressed.Mod&tea.ModShift != 0:
		m.form.focusPrev()

		return m, nil
	case isControlToggleKey(msg) && m.form.focusOnControls():
		m.form.toggleControl()

		return m, nil
	default:
		cmd := m.form.update(msg)

		return m, cmd
	}
}

// submitForm builds the request from the form and moves to the run screen.
// A build error (a value that doesn't fit its schema, say) stays on the
// form and shows the message, mirroring how Phase 1's `call` reports a bad
// argument before dispatch.
func (m *tuiModel) submitForm() (tea.Model, tea.Cmd) {
	tool, args, err := m.form.buildRequest()
	if err != nil {
		m.status = "cannot build request: " + err.Error()

		return m, nil
	}

	run := newRunModel(m.srv, tool, args, capabilityFor(m.srv, tool))
	run.setSize(m.width, max(m.height-chromeReservedRows, 1))
	m.run = &run
	m.screen = screenRun

	if run.awaitingConfirm {
		m.status = "destructive tool: press y to confirm, esc to cancel"

		return m, nil
	}

	m.status = "running..."

	return m, m.run.dispatchCmd()
}

// handleRunKey handles keys on the run screen: back to the catalog, confirm
// a gated destructive call, toggle the result format, or scroll the result
// viewport. Confirm only fires while the gate is up.
func (m *tuiModel) handleRunKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = screenCatalog
		m.status = ""

		return m, nil
	case key.Matches(msg, m.keys.Confirm) && m.run.awaitingConfirm:
		m.status = "running..."

		return m, m.run.confirm()
	case key.Matches(msg, m.keys.Format):
		m.run.toggleFormat()

		return m, nil
	default:
		cmd := m.run.update(msg)

		return m, cmd
	}
}

// routeToScreen forwards a non-key, non-resize message to the active
// screen's update, so widget-internal commands (cursor blink) keep
// flowing.
func (m *tuiModel) routeToScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenCatalog:
		return m, m.catalog.update(msg)
	case screenForm:
		if m.form != nil {
			return m, m.form.update(msg)
		}

		return m, nil
	case screenRun:
		if m.run != nil {
			return m, m.run.update(msg)
		}

		return m, nil
	case screenAudit:
		if m.audit != nil {
			return m, m.audit.update(msg)
		}

		return m, nil
	case screenProfile:
		if m.profile != nil {
			return m, m.profile.update(msg)
		}

		return m, nil
	case screenHealth:
		if m.health != nil {
			return m, m.health.update(msg)
		}

		return m, nil
	default:
		return m, nil
	}
}

// body renders the active screen's content.
func (m *tuiModel) body() string {
	switch m.screen {
	case screenForm:
		return renderForm(m.form)
	case screenRun:
		return renderRun(m.run)
	case screenAudit:
		return m.audit.view()
	case screenProfile:
		return m.profile.view()
	case screenHealth:
		return m.health.view()
	case screenCatalog:
		return m.catalog.view()
	default:
		return m.catalog.view()
	}
}

// footer renders the status line plus a terse key hint for the active
// screen, so the most useful bindings are always visible.
func (m *tuiModel) footer() string {
	hint := screenKeyHint(m.screen)
	if m.status == "" {
		return hint
	}

	return m.status + "  |  " + hint
}

// isControlToggleKey reports whether a key should toggle or advance the
// focused form control: space or enter. Enter doubles as the control
// advance only when focus is on a control; the submit key (ctrl+r) is the
// dedicated run trigger so enter never accidentally dispatches.
func isControlToggleKey(msg tea.KeyPressMsg) bool {
	return msg.Key().Code == tea.KeySpace || msg.Key().Code == tea.KeyEnter
}

// resultStatus turns a finished dispatch into a one-line status. A
// transport error, a tool-level error result, and success each read
// differently so the user knows the outcome at a glance.
func resultStatus(msg runResultMsg) string {
	if msg.err != nil {
		return "dispatch failed: " + msg.err.Error()
	}

	if msg.result.IsError {
		return "tool returned an error result"
	}

	return "done"
}

// capabilityFor looks up a tool's capability across the full catalog so
// the run screen can decide whether to gate it. Defaults to CapUnknown
// when the tool isn't found, which does not gate.
func capabilityFor(srv *server.Server, tool string) profiles.Capability {
	for _, info := range srv.AllToolInfos() {
		if info.Name == tool {
			return info.Capability
		}
	}

	return profiles.CapUnknown
}

// RunTUICommand launches the interactive TUI and returns the process exit
// code. It builds one runtime, holds it open for the whole session, runs
// the Bubble Tea program against the given streams, and closes the runtime
// on exit. Like the rest of the CLI it falls back to an in-memory default
// config when no config file exists, so the catalog browses offline.
//
// out is the terminal the program draws to (os.Stdout in production);
// errOut carries setup diagnostics. The program's own goroutines live only
// for the duration of Run, so a clean exit leaves none behind.
func RunTUICommand(out, errOut io.Writer) int {
	quietStartupLogging(errOut)

	runtime, err := newRuntime(context.Background(), errOut)
	if err != nil {
		writef(errOut, "%v\n", err)

		return 1
	}
	defer runtime.Close()

	program := tea.NewProgram(newTUIModel(runtime), tea.WithOutput(out))
	if _, runErr := program.Run(); runErr != nil {
		writef(errOut, "tui error: %v\n", runErr)

		return 1
	}

	return 0
}
