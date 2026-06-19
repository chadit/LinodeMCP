package cli

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// metricsEndpointNote points the user at where Prometheus metrics are
// exposed when observability is enabled, without the TUI scraping them.
// The serve path binds the metrics listener; one-shot/TUI sessions do not,
// so this is a pointer, not a live gauge.
const metricsEndpointNote = "Prometheus metrics: scrape the server's /metrics endpoint when observability.metrics is enabled (the serve process binds it; the TUI does not)."

// healthSnapshot is the subset of the linode_audit_health payload the view
// renders. Decoded from JSON so the field names match the tool's output.
type healthSnapshot struct {
	JSONLPath        string `json:"jsonl_path"`
	ActiveLogExists  bool   `json:"active_log_exists"`
	RotatedFileCount int    `json:"rotated_file_count"`
	DiskBytes        int64  `json:"disk_bytes"`
	DroppedEvents    int64  `json:"dropped_events"`
	SQLite           *struct {
		Path       string `json:"path"`
		EventCount int64  `json:"event_count"`
		DBBytes    int64  `json:"db_bytes"`
	} `json:"sqlite"`
}

// HealthLines projects the audit-health JSON payload and the build/version
// info into the labeled lines the view renders. The version block always
// renders (it needs no dispatch); the audit block renders when the payload
// parses, else a single line noting the payload could not be read. Keeping
// the mapping pure makes the label/value derivation testable without a
// server.
//
// info is the build/version metadata from appinfo.Get() (taken by pointer
// to dodge gocritic's hugeParam); payload is the linode_audit_health result
// text.
func HealthLines(payload string, info *appinfo.Info) []string {
	head := []string{
		"Build & version",
		"  version:    " + info.Version,
		"  api:        " + info.APIVersion,
		"  commit:     " + info.Commit,
		"  build date: " + info.BuildDate,
		"  platform:   " + info.Platform,
		"",
		"Audit subsystem",
	}

	audit := healthAuditLines(payload)
	lines := make([]string, 0, len(head)+len(audit))
	lines = append(lines, head...)
	lines = append(lines, audit...)

	return lines
}

// healthAuditLines projects the audit-health portion. A payload that does
// not parse yields a single explanatory line rather than a blank section,
// so a malformed result is visible.
func healthAuditLines(payload string) []string {
	var snapshot healthSnapshot
	if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
		return []string{"  (could not read audit health: " + err.Error() + ")"}
	}

	lines := []string{
		"  jsonl path:     " + snapshot.JSONLPath,
		"  active log:     " + boolLabel(snapshot.ActiveLogExists),
		"  rotated files:  " + strconv.Itoa(snapshot.RotatedFileCount),
		"  disk bytes:     " + strconv.FormatInt(snapshot.DiskBytes, 10),
		"  dropped events: " + strconv.FormatInt(snapshot.DroppedEvents, 10),
	}

	if snapshot.SQLite != nil {
		lines = append(
			lines,
			"  sqlite path:    "+snapshot.SQLite.Path,
			"  sqlite events:  "+strconv.FormatInt(snapshot.SQLite.EventCount, 10),
			"  sqlite bytes:   "+strconv.FormatInt(snapshot.SQLite.DBBytes, 10),
		)
	}

	return lines
}

// healthModel is the health/metrics view. It dispatches linode_audit_health
// through the shared server, combines the result with appinfo.Get(), and
// renders both in a scrollable viewport, plus a pointer to the metrics
// endpoint.
type healthModel struct {
	srv      *server.Server
	viewport viewport.Model
	loaded   bool
	width    int
	height   int
}

// newHealthModel builds the health view over an open server. The first
// refresh is kicked off by the parent when the screen opens.
func newHealthModel(srv *server.Server) healthModel {
	return healthModel{
		srv:      srv,
		viewport: viewport.New(),
	}
}

// setSize lays the viewport out within the available space.
func (m *healthModel) setSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
}

// refreshCmd returns a tea.Cmd that dispatches linode_audit_health through
// the shared dispatch and delivers the result as a healthLoadedMsg. Reuses
// the same dispatch path the run and audit screens use.
func (m *healthModel) refreshCmd() tea.Cmd {
	srv := m.srv

	return func() tea.Msg {
		result, err := dispatchCall(context.Background(), srv, toolAuditHealth, map[string]any{})

		return healthLoadedMsg{result: result, err: err}
	}
}

// healthLoadedMsg carries a finished linode_audit_health dispatch back into
// the update loop.
type healthLoadedMsg struct {
	result CallResult
	err    error
}

// handleLoaded records a finished refresh and loads the rendered health
// report into the viewport. The version block renders even when the audit
// dispatch fails, so the user always sees the build info.
func (m *healthModel) handleLoaded(msg healthLoadedMsg) {
	m.loaded = true

	payload := msg.result.Text
	if msg.err != nil {
		payload = "(audit health query failed: " + msg.err.Error() + ")"
	}

	info := appinfo.Get()
	lines := HealthLines(payload, &info)
	lines = append(lines, "", metricsEndpointNote)

	m.viewport.SetContent(strings.Join(lines, "\n"))
	m.viewport.GotoTop()
}

// update forwards scroll messages to the viewport once the report loads.
func (m *healthModel) update(msg tea.Msg) tea.Cmd {
	if !m.loaded {
		return nil
	}

	var cmd tea.Cmd

	m.viewport, cmd = m.viewport.Update(msg)

	return cmd
}

// view renders the health viewport, or a loading note before the first
// refresh lands.
func (m *healthModel) view() string {
	if !m.loaded {
		return "Loading audit health..."
	}

	return m.viewport.View()
}
