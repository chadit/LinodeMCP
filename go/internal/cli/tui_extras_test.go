package cli_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/cli"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// auditPayloadWith builds a linode_audit_recent JSON payload around the
// given event objects, so a test can state events compactly and feed the
// row mapper the exact shape the tool emits.
func auditPayloadWith(events string) string {
	return `{"count":1,"events":[` + events + `]}`
}

// TestAuditEventRowsMapsFields checks the audit row mapper projects each
// event's display columns from the JSON payload: timestamp trimmed, tool,
// capability, mode, status, and the plan id when present.
func TestAuditEventRowsMapsFields(t *testing.T) {
	t.Parallel()

	payload := auditPayloadWith(`{
		"ts":"2026-06-14T09:15:30.123Z",
		"tool":"linode_instance_delete",
		"tool_capability":"destroy",
		"mode":"apply",
		"status":"success",
		"plan_id":"plan-abc"
	}`)

	rows, err := cli.AuditEventRows(payload)
	if err != nil {
		t.Fatalf("AuditEventRows: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	row := rows[0]
	if row.Timestamp != "2026-06-14 09:15:30" {
		t.Errorf("timestamp = %q, want trimmed 2026-06-14 09:15:30", row.Timestamp)
	}

	if row.Tool != "linode_instance_delete" {
		t.Errorf("tool = %q", row.Tool)
	}

	if row.Capability != "destroy" {
		t.Errorf("capability = %q, want destroy", row.Capability)
	}

	if row.Mode != "apply" {
		t.Errorf("mode = %q, want apply", row.Mode)
	}

	if row.Status != "success" {
		t.Errorf("status = %q, want success", row.Status)
	}

	if row.PlanID != "plan-abc" {
		t.Errorf("plan_id = %q, want plan-abc", row.PlanID)
	}
}

// TestAuditEventRowsDashesMissingOptionals checks an event with no mode and
// a null plan_id renders dashes in those columns rather than blanks, so the
// table reads as "not set" instead of looking like a gap.
func TestAuditEventRowsDashesMissingOptionals(t *testing.T) {
	t.Parallel()

	payload := auditPayloadWith(`{
		"ts":"2026-06-14T09:15:30Z",
		"tool":"linode_instance_list",
		"tool_capability":"read",
		"mode":"",
		"status":"success",
		"plan_id":null
	}`)

	rows, err := cli.AuditEventRows(payload)
	if err != nil {
		t.Fatalf("AuditEventRows: %v", err)
	}

	if rows[0].Mode != "-" {
		t.Errorf("mode = %q, want dash", rows[0].Mode)
	}

	if rows[0].PlanID != "-" {
		t.Errorf("plan_id = %q, want dash", rows[0].PlanID)
	}
}

// TestAuditEventRowsRejectsBadJSON checks a malformed payload returns an
// error (so the viewer can fall back to the raw text) rather than silently
// yielding an empty table.
func TestAuditEventRowsRejectsBadJSON(t *testing.T) {
	t.Parallel()

	_, err := cli.AuditEventRows("{not json")
	if err == nil {
		t.Fatal("AuditEventRows accepted malformed JSON")
	}
}

// TestRenderAuditRowsEmpty checks the renderer shows a no-events note plus
// the header rather than a bare header when there are no rows.
func TestRenderAuditRowsEmpty(t *testing.T) {
	t.Parallel()

	out := cli.RenderAuditRows(nil)
	if !strings.Contains(out, "no audit events") {
		t.Errorf("empty render missing the no-events note: %q", out)
	}
}

// TestRenderAuditRowsIncludesValues checks a rendered non-empty table
// contains the tool name and status, so the viewer shows the event data.
func TestRenderAuditRowsIncludesValues(t *testing.T) {
	t.Parallel()

	rows, err := cli.AuditEventRows(auditPayloadWith(`{
		"ts":"2026-06-14T09:15:30Z","tool":"linode_volume_create",
		"tool_capability":"write","mode":"","status":"success","plan_id":null
	}`))
	if err != nil {
		t.Fatalf("AuditEventRows: %v", err)
	}

	out := cli.RenderAuditRows(rows)
	if !strings.Contains(out, "linode_volume_create") || !strings.Contains(out, "success") {
		t.Errorf("rendered table missing event data:\n%s", out)
	}
}

// TestProfileListEntriesMarksActive checks the profile list construction
// reuses the config catalog and marks exactly the active profile, so the
// switcher highlights the right one. The active profile is set to a
// built-in; every built-in plus user profile appears.
func TestProfileListEntriesMarksActive(t *testing.T) {
	t.Parallel()

	cfg := testCatalog()
	cfg.ActiveProfile = profiles.BuiltinReadonlyFull

	entries := cli.ProfileListEntries(cfg)
	if len(entries) == 0 {
		t.Fatal("ProfileListEntries returned no entries")
	}

	names := make([]string, 0, len(entries))

	var activeCount int

	for _, entry := range entries {
		names = append(names, entry.Name)

		if entry.Active {
			activeCount++

			if entry.Name != profiles.BuiltinReadonlyFull {
				t.Errorf("active marker on %q, want %q", entry.Name, profiles.BuiltinReadonlyFull)
			}
		}
	}

	if activeCount != 1 {
		t.Errorf("active count = %d, want exactly 1", activeCount)
	}

	if !slices.Contains(names, profiles.BuiltinComputeAdmin) {
		t.Errorf("entries missing a known built-in (compute-admin): %v", names)
	}
}

// TestProfileListEntriesDefaultsActive checks that an unset ActiveProfile
// marks the default profile active, matching ResolveActiveName.
func TestProfileListEntriesDefaultsActive(t *testing.T) {
	t.Parallel()

	cfg := testCatalog()
	cfg.ActiveProfile = ""

	for _, entry := range cli.ProfileListEntries(cfg) {
		if entry.Name == profiles.BuiltinDefault && !entry.Active {
			t.Error("default profile not marked active when ActiveProfile is unset")
		}

		if entry.Name != profiles.BuiltinDefault && entry.Active {
			t.Errorf("non-default profile %q marked active when ActiveProfile is unset", entry.Name)
		}
	}
}

// TestProfileListEntriesNilConfig checks a nil config yields no entries
// rather than panicking.
func TestProfileListEntriesNilConfig(t *testing.T) {
	t.Parallel()

	if entries := cli.ProfileListEntries(nil); entries != nil {
		t.Errorf("nil config yielded %d entries, want none", len(entries))
	}
}

// TestHealthLinesRendersVersionAndAudit checks the health mapping always
// renders the build/version block and, given a valid audit payload, the
// audit subsystem fields (paths, disk bytes, dropped counter).
func TestHealthLinesRendersVersionAndAudit(t *testing.T) {
	t.Parallel()

	info := appinfo.Info{
		Version:    testVersion,
		APIVersion: "1.2.3",
		Commit:     "abcdef",
		BuildDate:  "2026-06-14",
		Platform:   "linux/amd64",
	}

	payload := `{
		"jsonl_path":"/var/log/linodemcp/audit.log",
		"active_log_exists":true,
		"rotated_file_count":2,
		"disk_bytes":4096,
		"dropped_events":7,
		"sqlite":null
	}`

	out := strings.Join(cli.HealthLines(payload, &info), "\n")

	for _, want := range []string{testVersion, "linux/amd64", "/var/log/linodemcp/audit.log", "4096", "7"} {
		if !strings.Contains(out, want) {
			t.Errorf("health output missing %q:\n%s", want, out)
		}
	}
}

// TestHealthLinesBadAuditStillShowsVersion checks that a malformed audit
// payload does not lose the version block: the version still renders and the
// audit section notes it could not be read.
func TestHealthLinesBadAuditStillShowsVersion(t *testing.T) {
	t.Parallel()

	info := appinfo.Info{Version: testVersion}

	out := strings.Join(cli.HealthLines("{not json", &info), "\n")

	if !strings.Contains(out, testVersion) {
		t.Errorf("version block lost on bad audit payload:\n%s", out)
	}

	if !strings.Contains(out, "could not read audit health") {
		t.Errorf("bad audit payload not flagged:\n%s", out)
	}
}

// TestHealthLinesIncludesSQLite checks the SQLite block renders when the
// payload carries SQLite health, so an operator with SQLite enabled sees
// its event count and size.
func TestHealthLinesIncludesSQLite(t *testing.T) {
	t.Parallel()

	info := appinfo.Info{Version: testVersion}

	payload := `{
		"jsonl_path":"/tmp/audit.log","active_log_exists":true,
		"rotated_file_count":0,"disk_bytes":0,"dropped_events":0,
		"sqlite":{"path":"/tmp/audit.db","event_count":42,"db_bytes":8192}
	}`

	out := strings.Join(cli.HealthLines(payload, &info), "\n")

	if !strings.Contains(out, "/tmp/audit.db") || !strings.Contains(out, "42") {
		t.Errorf("SQLite health not rendered:\n%s", out)
	}
}
