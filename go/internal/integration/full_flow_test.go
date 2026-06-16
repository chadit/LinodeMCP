// Package integration_test holds the cross-spec integration test: one
// happy path dispatched through the real server entry point, proving
// the trust-and-safety pieces compose. Profile filtering exposes only
// the compute-admin surface, the pre-check tool issues verdicts from
// the active profile, a destructive tool runs the two-stage plan and
// apply flow against a fake Linode API, and the audit log records both
// stages with their modes. The per-feature matrices live in each
// package's own tests; this is the fit-together check.
package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

const (
	toolInstanceDelete = "linode_instance_delete"
	toolInstanceList   = "linode_instance_list"
	toolInstanceCreate = "linode_instance_create"
	toolDomainList     = "linode_domain_list"
	toolVPCDelete      = "linode_vpc_delete"
	toolCanRun         = "linode_profile_can_run"
	toolAuditRecent    = "linode_audit_recent"

	keyInstanceID = "instance_id"
	keyMode       = "mode"
	keyPlanID     = "plan_id"
	keyTool       = "tool"

	instanceID   = 123
	instancePath = "/linode/instances/123"
)

// fakeLinodeAPI serves the single instance the flow deletes. A GET for
// the instance path returns its JSON; every other GET (the plan stage's
// dependency walk fetches volumes, IPs, firewalls, and the type) gets
// an empty paginated list, which the best-effort walk tolerates. The
// only non-GET in the flow is the apply stage's DELETE, recorded in the
// atomic so the test can prove the plan stage never mutates.
func fakeLinodeAPI(t *testing.T, deleted *atomic.Bool) string {
	t.Helper()

	instance := map[string]any{
		"id":     instanceID,
		"label":  "itest-web",
		"status": "running",
		"type":   "g6-nanode-1",
		"region": "us-east",
	}
	emptyPage := map[string]any{"data": []any{}, "page": 1, "pages": 1, "results": 0}

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			deleted.Store(true)
			w.WriteHeader(http.StatusOK)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		body := emptyPage
		if r.URL.Path == instancePath {
			body = instance
		}

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("encode fake API response: %v", err)
		}
	}))
	t.Cleanup(apiServer.Close)

	return apiServer.URL
}

// newComputeAdminServer constructs a server with the compute-admin
// builtin active, pointed at the fake API, and wires the real JSONL
// audit sink. The sink writes under ResolveDefaultAuditDir, the same
// directory the audit query tools read, so redirecting XDG_STATE_HOME
// to a temp dir (done by the test via t.Setenv) makes the write and
// read sides meet on disk.
func newComputeAdminServer(t *testing.T, apiURL string) *server.Server {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      "IntegrationTest",
			LogLevel:  "info",
			Transport: "stdio",
			Host:      "127.0.0.1",
			Port:      8080,
		},
		ActiveProfile: profiles.BuiltinComputeAdmin,
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: apiURL, Token: "itest-token"},
			},
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	sink, err := audit.NewJSONLSink(audit.ResolveDefaultAuditDir())
	if err != nil {
		t.Fatalf("NewJSONLSink: %v", err)
	}

	srv.SetAuditSink(sink)

	return srv
}

// callTool dispatches one tools/call through the server's MCP entry
// point and returns the text payload. Transport errors, JSON-RPC
// errors, and tool-level IsError results all fail the test, so the
// flow steps can consume the return value directly.
func callTool(t *testing.T, srv *server.Server, callID int, tool string, args map[string]any) string {
	t.Helper()

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      callID,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		t.Fatalf("marshal %s request: %v", tool, err)
	}

	raw, err := json.Marshal(srv.HandleMessage(t.Context(), message))
	if err != nil {
		t.Fatalf("marshal %s response: %v", tool, err)
	}

	// The MCP wire shape uses camelCase keys (isError), so the response
	// is walked as generic maps rather than decoded into tagged structs.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal %s response: %v", tool, err)
	}

	if rpcErr, isMap := decoded["error"].(map[string]any); isMap {
		t.Fatalf("%s JSON-RPC error: %v", tool, rpcErr["message"])
	}

	result, isMap := decoded["result"].(map[string]any)
	if !isMap {
		t.Fatalf("%s response has no result: %s", tool, raw)
	}

	content, isSlice := result["content"].([]any)
	if !isSlice || len(content) == 0 {
		t.Fatalf("%s returned no content: %s", tool, raw)
	}

	first, isMap := content[0].(map[string]any)
	if !isMap {
		t.Fatalf("%s first content block is not an object: %v", tool, content[0])
	}

	text, _ := first["text"].(string)

	if isError, _ := result["isError"].(bool); isError {
		t.Fatalf("%s tool error: %s", tool, text)
	}

	return text
}

// decodeMap parses a tool's JSON text payload into a generic map.
func decodeMap(t *testing.T, text string) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode tool payload %q: %v", text, err)
	}

	return body
}

// TestFullFlowComputeAdminPlanApplyAudit is the Layer 3 cross-spec
// happy path from the roadmap's testing strategy:
//
//  1. The server starts with the compute-admin profile.
//  2. linode_profile_can_run issues verdicts: the compute calls and a
//     DNS read are allowed (reads pass in every profile), while an
//     elevated tool outside the profile's categories is blocked.
//  3. linode_instance_delete with mode "plan" returns a plan_id and
//     must not mutate.
//  4. mode "apply" with that plan_id detects no drift and executes.
//  5. linode_audit_recent shows the plan and apply events with the
//     active profile, their modes, and the apply call's plan_id.
//
// t.Setenv pins XDG_STATE_HOME (and so the audit directory) to a temp
// dir, which also serializes this test; do not add t.Parallel.
func TestFullFlowComputeAdminPlanApplyAudit(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deleted := &atomic.Bool{}
	srv := newComputeAdminServer(t, fakeLinodeAPI(t, deleted))

	assertCanRunVerdicts(t, srv)

	planID := planInstanceDelete(t, srv, deleted)

	applyInstanceDelete(t, srv, planID, deleted)
	assertAuditTrail(t, srv, planID)
}

// assertCanRunVerdicts checks the pre-check tool against the active
// profile's design: read tools pass in every profile (the DNS list is
// allowed even though DNS is outside compute-admin's categories), the
// compute mutators are allowed, and an elevated tool outside the
// profile's categories (a VPC destroy) is blocked. The summary counts
// must agree with the per-call verdicts.
func assertCanRunVerdicts(t *testing.T, srv *server.Server) {
	t.Helper()

	calls := []any{
		map[string]any{keyTool: toolInstanceList},
		map[string]any{keyTool: toolInstanceCreate},
		map[string]any{keyTool: toolInstanceDelete},
		map[string]any{keyTool: toolDomainList},
		map[string]any{keyTool: toolVPCDelete},
	}
	body := decodeMap(t, callTool(t, srv, 1, toolCanRun, map[string]any{"calls": calls}))

	results, isSlice := body["results"].([]any)
	if !isSlice || len(results) != len(calls) {
		t.Fatalf("results = %v, want %d verdicts", body["results"], len(calls))
	}

	wantAllowed := map[string]bool{
		toolInstanceList:   true,
		toolInstanceCreate: true,
		toolInstanceDelete: true,
		toolDomainList:     true,
		toolVPCDelete:      false,
	}

	for _, entry := range results {
		row, isMap := entry.(map[string]any)
		if !isMap {
			t.Fatalf("verdict row is not an object: %v", entry)
		}

		name, _ := row[keyTool].(string)
		allowed, _ := row["allowed"].(bool)

		if want, known := wantAllowed[name]; !known || allowed != want {
			t.Errorf("can_run verdict for %s = %v, want %v", name, allowed, wantAllowed[name])
		}
	}

	summary, isMap := body["summary"].(map[string]any)
	if !isMap {
		t.Fatalf("summary missing from can_run response: %v", body)
	}

	if got, _ := summary["allowed"].(float64); got != 4 {
		t.Errorf("summary.allowed = %v, want 4", summary["allowed"])
	}

	if got, _ := summary["blocked"].(float64); got != 1 {
		t.Errorf("summary.blocked = %v, want 1", summary["blocked"])
	}
}

// planInstanceDelete runs the plan stage and returns the plan_id. The
// fake API's deleted flag proves planning never mutates.
func planInstanceDelete(t *testing.T, srv *server.Server, deleted *atomic.Bool) string {
	t.Helper()

	body := decodeMap(t, callTool(t, srv, 2, toolInstanceDelete, map[string]any{
		keyInstanceID: instanceID,
		keyMode:       "plan",
	}))

	planID, _ := body[keyPlanID].(string)
	if planID == "" {
		t.Fatalf("plan response has no plan_id: %v", body)
	}

	if deleted.Load() {
		t.Fatal("plan stage must not issue a mutating call")
	}

	return planID
}

// applyInstanceDelete runs the apply stage with the stored plan and
// confirms the delete actually executed.
func applyInstanceDelete(t *testing.T, srv *server.Server, planID string, deleted *atomic.Bool) {
	t.Helper()

	text := callTool(t, srv, 3, toolInstanceDelete, map[string]any{
		keyInstanceID: instanceID,
		keyMode:       "apply",
		keyPlanID:     planID,
	})

	if !deleted.Load() {
		t.Fatal("apply stage must issue the mutating call")
	}

	if !strings.Contains(text, "removed successfully") {
		t.Errorf("apply response does not confirm deletion: %s", text)
	}
}

// assertAuditTrail reads the audit log back through the query tool and
// checks both stages were recorded: same tool, plan then apply modes,
// the active profile on each, and the apply call's recorded arguments
// carrying the plan_id from the plan response. Meta tools (the
// pre-check and the audit query itself) stay excluded by default.
func assertAuditTrail(t *testing.T, srv *server.Server, planID string) {
	t.Helper()

	body := decodeMap(t, callTool(t, srv, 4, toolAuditRecent, map[string]any{"limit": 20}))

	events, isSlice := body["events"].([]any)
	if !isSlice || len(events) == 0 {
		t.Fatalf("audit recent returned no events: %v", body)
	}

	var planEvent, applyEvent map[string]any

	for _, entry := range events {
		event, isMap := entry.(map[string]any)
		if !isMap {
			continue
		}

		if tool, _ := event[keyTool].(string); tool == toolCanRun {
			t.Errorf("meta event %s leaked into the default audit view", toolCanRun)
		}

		if tool, _ := event[keyTool].(string); tool != toolInstanceDelete {
			continue
		}

		switch mode, _ := event[keyMode].(string); mode {
		case "plan":
			planEvent = event
		case "apply":
			applyEvent = event
		}
	}

	if planEvent == nil || applyEvent == nil {
		t.Fatalf("missing plan or apply audit event: plan=%v apply=%v", planEvent, applyEvent)
	}

	for _, event := range []map[string]any{planEvent, applyEvent} {
		if profile, _ := event["profile"].(string); profile != profiles.BuiltinComputeAdmin {
			t.Errorf("event profile = %v, want %s", event["profile"], profiles.BuiltinComputeAdmin)
		}

		if status, _ := event["status"].(string); status != "success" {
			t.Errorf("event status = %v, want success", event["status"])
		}
	}

	args, isMap := applyEvent["args"].(map[string]any)
	if !isMap {
		t.Fatalf("apply event has no args: %v", applyEvent)
	}

	if got, _ := args[keyPlanID].(string); got != planID {
		t.Errorf("apply event args.plan_id = %v, want %s", args[keyPlanID], planID)
	}
}
