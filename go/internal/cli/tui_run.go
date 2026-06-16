package cli

import (
	"context"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// resultFormat selects how the run screen renders a tool result. It
// reuses the Phase 1 output formats so JSON and table render identically
// to `linodemcp call --output ...`.
type resultFormat int

const (
	resultFormatJSON resultFormat = iota
	resultFormatTable
)

// runResultMsg carries a completed dispatch back into the Bubble Tea
// update loop. The dispatch runs in a tea.Cmd off the UI goroutine; this
// message delivers its outcome so the run model can render it. err is the
// transport/JSON-RPC failure (nil on a normal tool result, including a
// tool-level error which lands in result.IsError instead).
type runResultMsg struct {
	result CallResult
	err    error
}

// runModel is the run-and-result screen. It shows the pending request,
// drives the dispatch through the shared server, and renders the result
// in a scrollable viewport. For destructive tools it first requires an
// explicit confirm step (and surfaces a dry-run/plan preview) before the
// real call, matching the safety posture the dispatch already enforces.
type runModel struct {
	srv      *server.Server
	tool     string
	args     map[string]any
	cap      profiles.Capability
	viewport viewport.Model

	awaitingConfirm bool
	done            bool
	result          CallResult
	err             error
	format          resultFormat

	width  int
	height int
}

// newRunModel builds the run screen for a prepared request. capability
// decides whether an explicit confirm gate precedes the call: destructive
// and admin tools wait on confirmation, reads and writes run straight
// through (the dispatch still applies its own confirm/dry-run rules, so
// this gate is an extra UI guard, not the only one).
func newRunModel(
	srv *server.Server,
	tool string,
	args map[string]any,
	capability profiles.Capability,
) runModel {
	return runModel{
		srv:             srv,
		tool:            tool,
		args:            args,
		cap:             capability,
		viewport:        viewport.New(0, 0),
		awaitingConfirm: requiresConfirmGate(capability),
		format:          resultFormatJSON,
	}
}

// requiresConfirmGate reports whether a tool's capability warrants the
// extra in-TUI confirm step before dispatch. Destroy and admin mutate or
// remove resources, so the TUI pauses on them; reads and ordinary writes
// proceed (writes still hit the dispatch's own confirm requirement).
func requiresConfirmGate(capability profiles.Capability) bool {
	return capability == profiles.CapDestroy || capability == profiles.CapAdmin
}

// setSize lays out the result viewport within the available space,
// reserving rows for the surrounding chrome the parent draws.
func (m *runModel) setSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height

	if m.done {
		m.viewport.SetContent(m.renderedResult())
	}
}

// confirm clears the gate so the next dispatch trigger runs. Returns the
// dispatch command so the parent can fire it immediately after the user
// confirms a destructive call.
func (m *runModel) confirm() tea.Cmd {
	m.awaitingConfirm = false

	return m.dispatchCmd()
}

// dispatchCmd returns a tea.Cmd that runs the tool through the shared
// server dispatch off the UI goroutine and reports the outcome as a
// runResultMsg. The dispatch uses the same dispatchCall the CLI `call`
// uses, so the TUI gets the identical audit, profile, dry-run, and
// two-stage behavior with no duplicated logic.
func (m *runModel) dispatchCmd() tea.Cmd {
	srv, tool, args := m.srv, m.tool, m.args

	return func() tea.Msg {
		result, err := dispatchCall(context.Background(), srv, tool, args)

		return runResultMsg{result: result, err: err}
	}
}

// handleResult records a finished dispatch and loads the rendered text
// into the viewport. Called by the parent when a runResultMsg arrives.
func (m *runModel) handleResult(msg runResultMsg) {
	m.done = true
	m.result = msg.result
	m.err = msg.err
	m.viewport.SetContent(m.renderedResult())
	m.viewport.GotoTop()
}

// toggleFormat flips the result rendering between JSON and table and
// reloads the viewport, so a user can switch views without re-running.
func (m *runModel) toggleFormat() {
	m.format = otherFormat(m.format)

	if m.done {
		m.viewport.SetContent(m.renderedResult())
	}
}

// otherFormat returns the result format opposite the given one, so the
// toggle is a single expression rather than an if-else assignment.
func otherFormat(format resultFormat) resultFormat {
	if format == resultFormatJSON {
		return resultFormatTable
	}

	return resultFormatJSON
}

// renderedResult produces the text shown in the result viewport: a
// transport error message, or the tool payload rendered as JSON or a
// table. Table falls back to raw JSON when the payload is not a
// table-shaped structure, reusing the Phase 1 renderTable contract.
func (m *runModel) renderedResult() string {
	if m.err != nil {
		return "dispatch error: " + m.err.Error()
	}

	if m.format == resultFormatTable {
		if rendered, ok := renderTable(m.result.Text); ok {
			return rendered
		}
	}

	return m.result.Text
}

// update forwards scroll and navigation messages to the result viewport
// once a result is shown, returning any command it produced.
func (m *runModel) update(msg tea.Msg) tea.Cmd {
	if !m.done {
		return nil
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	return cmd
}
