package cli

import (
	"context"
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/chadit/LinodeMCP/go/internal/server"
)

// auditRecentLimit caps how many events the viewer pulls in one refresh.
// Enough to show recent activity without an unbounded table. The
// linode_audit_recent tool (reused from audit_cmd.go's toolAuditRecent
// constant) is CapMeta, so the viewer works in every profile.
const auditRecentLimit = 50

// auditColumns is the fixed-width column header for the events table, kept
// here so the header and the row formatter share one layout.
const auditColumns = "TS                   TOOL                                     CAP        MODE   STATUS   PLAN"

// AuditEventRow is the framework-agnostic projection of one audit event
// for the viewer: the columns a user reads to see what the TUI just did.
// Built from the linode_audit_recent JSON payload, so the mapping is
// testable without a server or the Bubble Tea widgets.
type AuditEventRow struct {
	Timestamp  string
	Tool       string
	Capability string
	Mode       string
	Status     string
	PlanID     string
}

// AuditEventRows parses a linode_audit_recent JSON payload and projects
// each event into an AuditEventRow. The payload shape is
// {"count":N,"events":[...]} where each event carries ts, tool,
// tool_capability, mode, status, and an optional plan_id. A payload that
// does not parse yields no rows and an error, so the caller can show the
// raw text instead of a blank table.
//
// Exported and pure: it reads only the JSON bytes, so the row mapping (the
// part worth testing) needs no dispatch.
func AuditEventRows(payload string) ([]AuditEventRow, error) {
	var decoded struct {
		Events []struct {
			TS         string  `json:"ts"`
			Tool       string  `json:"tool"`
			Capability string  `json:"tool_capability"`
			Mode       string  `json:"mode"`
			Status     string  `json:"status"`
			PlanID     *string `json:"plan_id"`
		} `json:"events"`
	}

	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return nil, wrapDecode("audit recent", err)
	}

	rows := make([]AuditEventRow, 0, len(decoded.Events))

	for _, event := range decoded.Events {
		rows = append(rows, AuditEventRow{
			Timestamp:  shortTimestamp(event.TS),
			Tool:       event.Tool,
			Capability: event.Capability,
			Mode:       defaultDash(event.Mode),
			Status:     event.Status,
			PlanID:     planIDOrDash(event.PlanID),
		})
	}

	return rows, nil
}

// RenderAuditRows formats the projected rows as fixed-width table lines
// under the shared header, so the viewport shows aligned columns. An empty
// row set renders a "no events" note rather than a bare header.
func RenderAuditRows(rows []AuditEventRow) string {
	if len(rows) == 0 {
		return auditColumns + "\n(no audit events yet)"
	}

	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, auditColumns)

	for idx := range rows {
		lines = append(lines, formatAuditRow(&rows[idx]))
	}

	return strings.Join(lines, "\n")
}

// formatAuditRow lays one row into the fixed column widths the header
// uses. Values are truncated to their column so a long tool name can't
// push the rest out of alignment.
func formatAuditRow(row *AuditEventRow) string {
	return fixedWidth(row.Timestamp, auditColTimestamp) + " " +
		fixedWidth(row.Tool, auditColTool) + " " +
		fixedWidth(row.Capability, auditColCapability) + " " +
		fixedWidth(row.Mode, auditColMode) + " " +
		fixedWidth(row.Status, auditColStatus) + " " +
		row.PlanID
}

// Column widths for the audit table, matching the auditColumns header.
const (
	auditColTimestamp  = 20
	auditColTool       = 40
	auditColCapability = 10
	auditColMode       = 6
	auditColStatus     = 8
)

// auditModel is the audit viewer screen. It dispatches linode_audit_recent
// through the shared server and renders the projected events in a
// scrollable viewport, so the user sees the TUI's own recent activity.
type auditModel struct {
	srv       *server.Server
	viewport  viewport.Model
	cancel    context.CancelFunc
	loaded    bool
	err       error
	requestID uint64
	width     int
	height    int
}

// newAuditModel builds the audit viewer over an open server. The first
// refresh is kicked off by the parent via refreshCmd when the screen
// opens; until it lands the viewport shows a loading note.
func newAuditModel(srv *server.Server) auditModel {
	return auditModel{
		srv:      srv,
		viewport: viewport.New(),
	}
}

// setSize lays the viewport out within the available space.
func (m *auditModel) setSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
}

// refreshCmd returns a tea.Cmd that dispatches linode_audit_recent through
// the shared dispatch (the same path the run screen uses) and delivers the
// result as an auditLoadedMsg. include_meta is true so the viewer shows the
// TUI's own meta calls, which is the point: seeing what the TUI just did.
func (m *auditModel) refreshCmd() tea.Cmd {
	m.cancelRefresh()

	m.requestID++
	requestID := m.requestID
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	srv := m.srv
	target := m

	return func() tea.Msg {
		defer cancel()

		args := map[string]any{"limit": auditRecentLimit, "include_meta": true}

		result, err := dispatchCall(ctx, srv, toolAuditRecent, args)

		return auditLoadedMsg{model: target, requestID: requestID, result: result, err: err}
	}
}

func (m *auditModel) cancelRefresh() {
	if m.cancel == nil {
		return
	}

	m.cancel()
	m.cancel = nil
}

// auditLoadedMsg carries a finished linode_audit_recent dispatch back into
// the update loop.
type auditLoadedMsg struct {
	model     *auditModel
	requestID uint64
	result    CallResult
	err       error
}

// handleLoaded records a finished refresh and loads the rendered events
// into the viewport. A transport error or an error result shows its text
// so the user sees why the table is empty.
func (m *auditModel) handleLoaded(msg auditLoadedMsg) {
	m.cancel = nil
	m.loaded = true
	m.err = msg.err

	if msg.err != nil {
		m.viewport.SetContent("audit query failed: " + msg.err.Error())

		return
	}

	if msg.result.IsError {
		m.viewport.SetContent("audit query returned an error:\n" + msg.result.Text)

		return
	}

	m.viewport.SetContent(renderAuditPayload(msg.result.Text))
	m.viewport.GotoTop()
}

// renderAuditPayload turns the JSON payload into the events table, or
// falls back to the raw payload when it does not parse (so nothing is
// silently swallowed).
func renderAuditPayload(payload string) string {
	rows, err := AuditEventRows(payload)
	if err != nil {
		return payload
	}

	return RenderAuditRows(rows)
}

// update forwards scroll messages to the viewport once events are loaded.
func (m *auditModel) update(msg tea.Msg) tea.Cmd {
	if !m.loaded {
		return nil
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	return cmd
}

// view renders the audit viewport, or a loading note before the first
// refresh lands.
func (m *auditModel) view() string {
	if !m.loaded {
		return "Loading recent audit events..."
	}

	return m.viewport.View()
}
