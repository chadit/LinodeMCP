package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/cli"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// Screen chrome the assertions read. The header names the active screen and
// the form body leads with the tool it was opened for, so both are how a
// user tells where the TUI is.
const (
	titleCatalog = "LinodeMCP TUI - Catalog"
	titleForm    = "LinodeMCP TUI - Tool Form"
	titleRun     = "LinodeMCP TUI - Run & Result"
	titleHealth  = "LinodeMCP TUI - Health & Version"
	toolLinePre  = "Tool: "
)

// The tools and profile the driven sessions reach for.
//
// toolDomainGet sits in the read-only default profile and takes exactly one
// non-safety argument, an integer, so the form has a single text field and a
// non-numeric value reaches the server's own validation instead of the
// network. toolInstanceCreate is a write tool the read-only default filters
// out but compute-admin allows, which is how a profile switch shows up in
// the catalog. The run screen dispatches toolHello (declared alongside the
// call tests) because the meta greeting answers offline.
const (
	toolDomainGet       = "linode_domain_get"
	toolInstanceCreate  = "linode_instance_create"
	toolInstanceDelete  = "linode_instance_delete"
	profileComputeAdmin = "compute-admin"
)

// statusSwitchedToComputeAdmin is the whole line the shell reports after a
// switch, spelled out so a reworded status shows up as a failing pin.
const statusSwitchedToComputeAdmin = "active profile switched to compute-admin"

// Terminal geometry for the driven sessions. Wide and tall enough that the
// catalog list, the form controls, and the audit table all fit without the
// widgets paginating an assertion out of view.
const (
	testTUIWidth  = 120
	testTUIHeight = 40
)

// tuiDriver drives an exported TUI model the way the Bubble Tea runtime
// does: it applies a message, then runs the command the model answered with
// and applies that command's messages too. Without the second half a screen
// that loads through a command (the catalog filter, an audit refresh) never
// shows what a user would see.
type tuiDriver struct {
	model tea.Model
}

// newTUIDriver builds a session over an in-process server on the shared
// in-memory config fixture.
func newTUIDriver(t *testing.T) *tuiDriver {
	t.Helper()

	return driverOver(t, newTestServer(t), testConfig())
}

// newTUIDriverOnConfigFile builds a session whose config came from a file.
// The profile switcher needs one, because switching writes the new active
// profile back to that file and then reloads the running server from it.
func newTUIDriverOnConfigFile(t *testing.T, path string) *tuiDriver {
	t.Helper()

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config fixture: %v", err)
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	return driverOver(t, srv, cfg)
}

// driverOver wraps a runtime in a driver and gives it a terminal size,
// because the list and viewport widgets lay out to zero rows until the
// first resize arrives.
func driverOver(t *testing.T, srv *server.Server, cfg *config.Config) *tuiDriver {
	t.Helper()

	drv := &tuiDriver{model: cli.NewTUIModel(&cli.Runtime{Server: srv, Config: cfg})}
	drv.send(tea.WindowSizeMsg{Width: testTUIWidth, Height: testTUIHeight})

	return drv
}

// press applies a key press and returns the command the model answered
// with, so a caller can check for the program-exit command.
func (drv *tuiDriver) press(key tea.KeyPressMsg) tea.Cmd {
	return drv.send(key)
}

// paste applies bracketed-paste text to whatever holds focus. Pasting a
// whole string is one message where typing it would be one per rune, and
// every rune into a text input schedules a half-second cursor-blink timer
// the driver would then wait out.
func (drv *tuiDriver) paste(text string) {
	drv.send(tea.PasteMsg{Content: text})
}

// send applies one message and settles the command it produced.
func (drv *tuiDriver) send(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	drv.model, cmd = drv.model.Update(msg)
	drv.settle(cmd)

	return cmd
}

// settle runs a command and applies every message it yields, flattening a
// batch into its members. The commands those messages produce in turn are
// not followed: the only one the screens under test schedule is the cursor
// blink, which reschedules itself forever.
func (drv *tuiDriver) settle(cmd tea.Cmd) {
	if cmd == nil {
		return
	}

	msg := cmd()
	if batch, isBatch := msg.(tea.BatchMsg); isBatch {
		for _, member := range batch {
			drv.settle(member)
		}

		return
	}

	if msg == nil {
		return
	}

	drv.model, _ = drv.model.Update(msg)
}

// view renders the current screen as the plain words a user reads. The
// styling codes come out because lipgloss puts a color reset between a
// control's label and its value, so a phrase like "[dry-run] on" is only
// contiguous once they are gone.
func (drv *tuiDriver) view() string {
	styling := regexp.MustCompile("\x1b\\[[0-9;]*m")

	return styling.ReplaceAllString(drv.model.View().Content, "")
}

// Key presses the assertions send, spelled once so a test reads as the
// keystrokes a user makes.
func keyRune(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func keyCtrl(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl}
}

func keyCode(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// quits reports whether a command is the one that ends the program.
func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}

	_, isQuit := cmd().(tea.QuitMsg)

	return isQuit
}

// openToolForm selects a tool from the catalog and asserts its form opened.
func openToolForm(t *testing.T, drv *tuiDriver, tool string) {
	t.Helper()

	selectCatalogTool(drv, tool)
	wantContains(t, "form screen", drv.view(), toolLinePre+tool)
}

// selectCatalogTool filters the catalog down to one tool and selects it,
// which opens that tool's form: slash starts the filter, the pasted name
// narrows it, the first enter accepts the filter and the second selects the
// highlighted row.
func selectCatalogTool(drv *tuiDriver, tool string) {
	drv.press(keyRune('/'))
	drv.paste(tool)
	drv.press(keyCode(tea.KeyEnter))
	drv.press(keyCode(tea.KeyEnter))
}

// switchToProfile opens the profile switcher, filters to one profile and
// selects it, which writes the new active profile to the config file and
// reloads the running server from it.
func switchToProfile(drv *tuiDriver, name string) {
	drv.press(keyCtrl('p'))
	drv.press(keyRune('/'))
	drv.paste(name)
	drv.press(keyCode(tea.KeyEnter))
	drv.press(keyCode(tea.KeyEnter))
}

// seedAuditLog writes one event into the audit log under stateHome, so the
// audit viewer has something to render without the session having
// dispatched anything first.
func seedAuditLog(t *testing.T, stateHome, tool string) {
	t.Helper()

	dir := filepath.Join(stateHome, "linodemcp")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("make audit dir: %v", err)
	}

	stamp := time.Date(2026, time.May, 20, 9, 30, 15, 0, time.UTC)
	event := &audit.Event{
		Ts:             audit.EventTimestamp(stamp),
		TsUnixNs:       stamp.UnixNano(),
		EventId:        "evt_seeded",
		Tool:           tool,
		ToolCapability: string(audit.CapabilityRead),
		Status:         string(audit.StatusSuccess),
	}

	file, err := os.Create(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("create audit log: %v", err)
	}

	defer func() { _ = file.Close() }()

	if err := audit.EncodeEvents(file, []*audit.Event{event}, audit.ExportFormatNDJSON); err != nil {
		t.Fatalf("encode seeded event: %v", err)
	}
}

// TestTUIArrowMovesCatalogSelection checks the down key moves the catalog
// highlight: selecting straight away opens one tool's form, and selecting
// after one step down opens a different tool's.
func TestTUIArrowMovesCatalogSelection(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	first := newTUIDriver(t)
	first.press(keyCode(tea.KeyEnter))

	moved := newTUIDriver(t)
	moved.press(keyCode(tea.KeyDown))
	moved.press(keyCode(tea.KeyEnter))

	wantContains(t, "first form", first.view(), titleForm)
	wantContains(t, "moved form", moved.view(), titleForm)

	if first.view() == moved.view() {
		t.Error("the down key left the catalog on the same tool; both selections opened the same form")
	}
}

// TestTUIScopeToggleRevealsFullCatalog checks ctrl+a swaps the catalog
// between the active profile's surface and the full registry, and back.
func TestTUIScopeToggleRevealsFullCatalog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	wantContains(t, "catalog", drv.view(), "Tools (active profile)")

	drv.press(keyCtrl('a'))
	wantContains(t, "toggled catalog", drv.view(), "Tools (full catalog)")

	drv.press(keyCtrl('a'))
	wantContains(t, "restored catalog", drv.view(), "Tools (active profile)")
}

// TestTUIFilterNarrowsCatalogToOneTool checks the catalog filter narrows the
// list: after filtering on a tool name, the highlighted row is that tool, so
// selecting opens its form rather than the alphabetically first tool's.
func TestTUIFilterNarrowsCatalogToOneTool(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	unfiltered := drv.view()

	openToolForm(t, drv, toolDomainGet)

	wantContains(t, "form screen", drv.view(), titleForm)
	wantNotContains(t, "unfiltered catalog", unfiltered, toolLinePre+toolDomainGet)
}

// TestTUIHealthViewRendersVersionRows checks ctrl+h opens the health view
// and that its refresh renders the build/version block and the audit
// subsystem block, which is what the view exists to show.
func TestTUIHealthViewRendersVersionRows(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	drv.press(keyCtrl('h'))

	view := drv.view()
	wantContains(t, "health screen", view, titleHealth)
	wantContains(t, "health screen", view, "Build & version")
	wantContains(t, "health screen", view, "version:")
	wantContains(t, "health screen", view, "platform:")
	wantContains(t, "health screen", view, "Audit subsystem")
	wantContains(t, "health screen", view, "jsonl path:")
}

// TestTUIAuditViewRendersSeededEvents checks ctrl+e opens the audit viewer
// and that its refresh renders the events already on disk, in the table the
// viewer draws.
func TestTUIAuditViewRendersSeededEvents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	seedAuditLog(t, stateHome, toolInstLst)

	drv := newTUIDriver(t)
	drv.press(keyCtrl('e'))

	view := drv.view()
	wantContains(t, "audit screen", view, "LinodeMCP TUI - Audit")
	wantContains(t, "audit screen", view, toolInstLst)
	wantContains(t, "audit screen", view, "2026-05-20 09:30:15")
	wantNotContains(t, "audit screen", view, "(no audit events yet)")
}

// TestTUIEscapeReturnsFromHealthToCatalog checks esc leaves an extras screen
// and puts the catalog back, so the shell's back key returns to the hub.
func TestTUIEscapeReturnsFromHealthToCatalog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	drv.press(keyCtrl('h'))
	wantContains(t, "health screen", drv.view(), titleHealth)

	drv.press(keyCode(tea.KeyEscape))
	wantContains(t, "catalog screen", drv.view(), titleCatalog)
}

// TestTUIRunReportsToolErrorForBadValue checks a value that does not fit a
// field's schema type comes back as a tool error the run screen names. The
// TUI hands the raw string to the same dispatch the CLI uses rather than
// second-guessing the schema, so the server's own message is what the user
// reads.
func TestTUIRunReportsToolErrorForBadValue(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolDomainGet)

	drv.paste("not-a-number")
	drv.press(keyCtrl('r'))

	view := drv.view()
	wantContains(t, "run screen", view, titleRun)
	wantContains(t, "run screen", view, "tool returned an error result")
	wantContains(t, "run screen", view, "domain_id is required")
}

// TestTUIQuitKeysQuit checks both quit bindings end the program from the
// catalog.
func TestTUIQuitKeysQuit(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "q", key: keyRune('q')},
		{name: "ctrl+c", key: keyCtrl('c')},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())

			drv := newTUIDriver(t)
			if !quits(drv.press(testCase.key)) {
				t.Errorf("%s did not quit the program", testCase.name)
			}
		})
	}
}

// TestTUIQuitKeyDefersToOpenCatalogFilter checks the letter quit binding is
// suppressed while the catalog filter is open, so typing a "q" into a search
// does not exit the app, and works again once the filter is closed.
func TestTUIQuitKeyDefersToOpenCatalogFilter(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	drv.press(keyRune('/'))

	if quits(drv.press(keyRune('q'))) {
		t.Fatal("q quit the program while the catalog filter was open")
	}

	wantContains(t, "catalog screen", drv.view(), titleCatalog)

	drv.press(keyCode(tea.KeyEscape))

	if !quits(drv.press(keyRune('q'))) {
		t.Error("q did not quit after the catalog filter was closed")
	}
}

// TestRunTUICommandQuitsOnPipedKey checks the input seam: given a reader
// carrying a quit key instead of a terminal, the command runs the program to
// a clean exit rather than failing on the missing TTY.
func TestRunTUICommandQuitsOnPipedKey(t *testing.T) {
	t.Setenv("LINODEMCP_CONFIG_PATH", writeTestConfigFile(t))
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("open pipe: %v", err)
	}

	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
	})

	if _, err := writer.WriteString("q"); err != nil {
		t.Fatalf("write quit key: %v", err)
	}

	var errOut bytes.Buffer

	// The drawn frames go to io.Discard because the renderer writes them from
	// its own goroutine; the exit code and the diagnostics are the contract.
	if code := cli.RunTUICommandWithInput(reader, io.Discard, &errOut); code != 0 {
		t.Fatalf("exit code = %d (stderr: %s), want 0", code, errOut.String())
	}
}

// TestTUIFormControlsToggleSafetyFlags checks tab walks focus from the text
// fields onto the safety controls and that space acts on the focused one:
// the dry-run control flips, the mode control advances to plan, and shift
// plus tab walks focus back so the dry-run control flips again.
func TestTUIFormControlsToggleSafetyFlags(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolDomainGet)
	wantContains(t, "fresh form", drv.view(), "[dry-run] off")

	drv.press(keyCode(tea.KeyTab))
	drv.press(keyCode(tea.KeySpace))
	wantContains(t, "form after dry-run toggle", drv.view(), "[dry-run] on")

	drv.press(keyCode(tea.KeyTab))
	drv.press(keyCode(tea.KeySpace))
	wantContains(t, "form after mode toggle", drv.view(), "[mode] plan")

	drv.press(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	drv.press(keyCode(tea.KeySpace))
	wantContains(t, "form after stepping focus back", drv.view(), "[dry-run] off")
}

// TestTUIRunShowsToolOutput checks a filled form dispatched with the run key
// lands on the run screen with the tool's own payload and a success status.
func TestTUIRunShowsToolOutput(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolHello)

	drv.paste("Ada")
	drv.press(keyCtrl('r'))

	view := drv.view()
	wantContains(t, "run screen", view, titleRun)
	wantContains(t, "run screen", view, "Hello, Ada!")
	wantContains(t, "run screen", view, "done")
}

// TestTUIRunFormatKeyTogglesTableRendering checks the format key re-renders a
// finished result as a table without re-running the tool, and back to JSON.
func TestTUIRunFormatKeyTogglesTableRendering(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolHello)

	drv.paste("Ada")
	drv.press(keyCtrl('r'))
	wantNotContains(t, "json result", drv.view(), "FIELD")

	drv.press(keyCtrl('t'))
	wantContains(t, "table result", drv.view(), "FIELD")
	wantContains(t, "table result", drv.view(), "Hello, Ada!")

	drv.press(keyCtrl('t'))
	wantNotContains(t, "json result again", drv.view(), "FIELD")
}

// TestTUIRunEscapeReturnsToCatalog checks the back key leaves a finished run
// and puts the catalog back, so a user can queue another call.
func TestTUIRunEscapeReturnsToCatalog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolHello)

	drv.paste("Ada")
	drv.press(keyCtrl('r'))
	wantContains(t, "run screen", drv.view(), titleRun)

	drv.press(keyCode(tea.KeyEscape))
	wantContains(t, "catalog screen", drv.view(), titleCatalog)
}

// TestTUIAuditRefreshKeyPicksUpNewEvents checks the refresh key re-reads the
// audit log: the viewer opens on an empty log, and after an event lands on
// disk the refresh shows it.
func TestTUIAuditRefreshKeyPicksUpNewEvents(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	drv := newTUIDriver(t)
	drv.press(keyCtrl('e'))
	wantContains(t, "empty audit screen", drv.view(), "(no audit events yet)")

	seedAuditLog(t, stateHome, toolInstLst)
	drv.press(keyRune('r'))

	wantContains(t, "refreshed audit screen", drv.view(), toolInstLst)
}

// TestTUIProfileSwitchWritesActiveProfile checks selecting a profile in the
// switcher persists it as the active profile in the config file, which is
// what makes the choice outlive the session.
func TestTUIProfileSwitchWritesActiveProfile(t *testing.T) {
	path := writeTestConfigFile(t)
	t.Setenv("LINODEMCP_CONFIG_PATH", path)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriverOnConfigFile(t, path)
	switchToProfile(drv, profileComputeAdmin)
	wantContains(t, "profile screen", drv.view(), statusSwitchedToComputeAdmin)

	written, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload written config: %v", err)
	}

	if got := cli.ResolveActiveName(written); got != profileComputeAdmin {
		t.Errorf("active profile in config = %q, want %q", got, profileComputeAdmin)
	}
}

// TestTUIProfileSwitchWidensCatalog checks the switch reaches the running
// server: a write tool the read-only default filters out is absent from the
// catalog before the switch and selectable after it, without a restart.
func TestTUIProfileSwitchWidensCatalog(t *testing.T) {
	path := writeTestConfigFile(t)
	t.Setenv("LINODEMCP_CONFIG_PATH", path)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriverOnConfigFile(t, path)

	selectCatalogTool(drv, toolInstanceCreate)
	wantNotContains(t, "read-only catalog", drv.view(), toolLinePre+toolInstanceCreate)

	drv.press(keyCode(tea.KeyEscape))
	switchToProfile(drv, profileComputeAdmin)
	wantContains(t, "profile screen", drv.view(), statusSwitchedToComputeAdmin)

	drv.press(keyCode(tea.KeyEscape))

	selectCatalogTool(drv, toolInstanceCreate)
	wantContains(t, "widened catalog", drv.view(), toolLinePre+toolInstanceCreate)
}

// TestTUIFormRendersToolArguments checks the form shows the tool's declared
// arguments, marked required and labeled with their schema type. Tools
// registered with a raw JSON schema leave the structured Properties empty,
// and a form built from that field alone offers no inputs at all, so the
// only value a call could carry would be its defaults.
func TestTUIFormRendersToolArguments(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolDomainGet)

	wantContains(t, "form screen", drv.view(), "* domain_id (integer)")
}

// TestTUIFormValueReachesTheTool checks a value entered on the form is the
// one the dispatched call carries, rather than the tool's own default.
func TestTUIFormValueReachesTheTool(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolHello)

	drv.paste("Ada")
	drv.press(keyCtrl('r'))

	view := drv.view()
	wantContains(t, "run screen", view, "Hello, Ada!")
	wantNotContains(t, "run screen", view, "Hello, World!")
}

// configWithActiveProfile stages the shared config fixture with an active
// profile set, so a session starts on a surface wider than the read-only
// default. Returns the file path.
func configWithActiveProfile(t *testing.T, name string) string {
	t.Helper()

	path := writeTestConfigFile(t)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open config fixture: %v", err)
	}

	defer func() { _ = file.Close() }()

	if _, err := file.WriteString("active_profile: " + name + "\n"); err != nil {
		t.Fatalf("set active profile: %v", err)
	}

	return path
}

// TestTUIFormEnvironmentControlPicksConfiguredEnvironment checks the
// environment control cycles through the configured environments, so a call
// can be aimed at one without leaving the form.
func TestTUIFormEnvironmentControlPicksConfiguredEnvironment(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriver(t)
	openToolForm(t, drv, toolDomainGet)
	wantContains(t, "fresh form", drv.view(), "[environment] (default)")

	for range 4 {
		drv.press(keyCode(tea.KeyTab))
	}

	drv.press(keyCode(tea.KeySpace))
	wantContains(t, "form after environment cycle", drv.view(), "[environment] "+testEnvKey)
}

// TestTUIProfileScreenMarksActiveProfile checks the switcher lists the
// profiles with the running one marked, which is how a user tells what they
// are switching away from.
func TestTUIProfileScreenMarksActiveProfile(t *testing.T) {
	path := writeTestConfigFile(t)
	t.Setenv("LINODEMCP_CONFIG_PATH", path)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriverOnConfigFile(t, path)
	drv.press(keyCtrl('p'))

	view := drv.view()
	wantContains(t, "profile screen", view, "* default (active)")
	wantContains(t, "profile screen", view, profileComputeAdmin)
}

// TestTUIDestructiveToolWaitsForConfirmation checks a destroy-capability tool
// does not dispatch on the run key alone: the run screen holds a confirm
// prompt, and only the confirm key sends the call.
func TestTUIDestructiveToolWaitsForConfirmation(t *testing.T) {
	path := configWithActiveProfile(t, profileComputeAdmin)
	t.Setenv("LINODEMCP_CONFIG_PATH", path)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	drv := newTUIDriverOnConfigFile(t, path)
	openToolForm(t, drv, toolInstanceDelete)

	drv.press(keyCtrl('r'))
	gated := drv.view()
	wantContains(t, "gated run screen", gated, "This tool is destructive. Press y to run, esc to cancel.")
	wantNotContains(t, "gated run screen", gated, "instance_id is required")

	drv.press(keyRune('y'))
	wantContains(t, "confirmed run screen", drv.view(), "instance_id is required")
}
