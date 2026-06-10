package tools_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

const (
	keyMode      = "mode"
	keyPlanID    = "plan_id"
	labelWebProd = "web-prod-01"
	// tsCosmeticBump is the post-plan value a two-stage test writes into a
	// hash-ignore field to prove a cosmetic change does not refuse the apply.
	tsCosmeticBump = "2026-09-09T09:09:09"
)

// twoStageDeleteServer serves GET /linode/instances/123 from the supplied
// state box (a test mutates it to simulate drift) and records each DELETE. The
// atomics keep the handler goroutine race-clean against the test's reads and
// writes between sequential calls.
func twoStageDeleteServer(
	t *testing.T,
	state *atomic.Pointer[linode.Instance],
	deleted *atomic.Bool,
) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET serves the current state for the plan/apply hash; any other
		// method is the mutating call. Some opted-in tools execute via POST
		// (backups cancel, password reset) rather than DELETE, so record the
		// mutation on any non-GET method.
		if r.Method != http.MethodGet {
			deleted.Store(true)
			w.WriteHeader(http.StatusOK)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(state.Load()); err != nil {
			t.Errorf("encode state: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

func instanceState() *atomic.Pointer[linode.Instance] {
	box := &atomic.Pointer[linode.Instance]{}
	box.Store(&linode.Instance{ID: 123, Label: labelWebProd, Status: statusRunning})

	return box
}

// TestInstanceDeleteTwoStagePlanThenApply covers the happy path: plan returns a
// plan_id and stores the plan without deleting; apply executes the delete and
// consumes the plan so a second apply reports PLAN_NOT_FOUND.
func TestInstanceDeleteTwoStagePlanThenApply(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if planResult.IsError {
		t.Fatalf("plan IsError, text: %s", dryRunResultText(t, planResult))
	}

	if deleted.Load() {
		t.Fatal("plan must not issue a DELETE")
	}

	plan := decodeBody(t, dryRunResultText(t, planResult))

	id, _ := plan["plan_id"].(string)
	if id == "" {
		t.Fatalf("plan response has no plan_id: %v", plan)
	}

	if store.Len() != 1 {
		t.Fatalf("store.Len() = %d, want 1", store.Len())
	}

	applyResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModeApply,
		keyPlanID:     id,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if applyResult.IsError {
		t.Fatalf("apply IsError, text: %s", dryRunResultText(t, applyResult))
	}

	if !deleted.Load() {
		t.Fatal("apply must issue a DELETE")
	}

	if !strings.Contains(dryRunResultText(t, applyResult), "removed successfully") {
		t.Errorf("apply text does not confirm deletion: %s", dryRunResultText(t, applyResult))
	}

	if store.Len() != 0 {
		t.Errorf("store.Len() = %d, want 0 (plan consumed)", store.Len())
	}

	again, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModeApply,
		keyPlanID:     id,
	}))
	if err != nil {
		t.Fatalf("second apply returned error: %v", err)
	}

	if !again.IsError || !strings.Contains(dryRunResultText(t, again), "PLAN_NOT_FOUND") {
		t.Errorf("second apply should report PLAN_NOT_FOUND, got: %s", dryRunResultText(t, again))
	}
}

// TestInstanceDeleteTwoStageApplyDrift covers refusal when the resource changed
// between plan and apply: the re-fetched state hashes differently, so apply
// reports PLAN_DRIFT_DETECTED and never deletes.
func TestInstanceDeleteTwoStageApplyDrift(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	id := makePlan(ctx, t, handler)

	state.Store(&linode.Instance{ID: 123, Label: "web-prod-01", Status: "offline"})

	applyResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModeApply,
		keyPlanID:     id,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	driftText := dryRunResultText(t, applyResult)
	if !applyResult.IsError || !strings.Contains(driftText, "PLAN_DRIFT_DETECTED") {
		t.Errorf("apply should report PLAN_DRIFT_DETECTED, got: %s", driftText)
	}

	// The refusal must name the field that drifted (only Status changed here).
	if !strings.Contains(driftText, "status") {
		t.Errorf("drift refusal should name the changed field 'status', got: %s", driftText)
	}

	if deleted.Load() {
		t.Error("drift must not issue a DELETE")
	}
}

// TestTwoStagePlanIncludesDependencies confirms a mode:"plan" response is a
// richer dry-run: it runs the tool's dependency walk and carries the resulting
// dependencies and warnings, the same enrichment dry_run:true produces.
func TestTwoStagePlanIncludesDependencies(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		keyLabel:       "data-01",
		keyStatus:      statusActive,
		keyLinodeID:    float64(999),
		"linode_label": "web-01",
	}
	state := &atomic.Pointer[map[string]any]{}
	state.Store(&base)

	deleted := &atomic.Bool{}
	cfg := twoStageJSONServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeVolumeDeleteTool(cfg)

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyVolumeID: float64(123),
		keyMode:     twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	body := decodeBody(t, dryRunResultText(t, planResult))

	deps, ok := body["dependencies"].([]any)
	if !ok || len(deps) == 0 {
		t.Fatalf("plan response should carry the walk's dependencies, got: %v", body["dependencies"])
	}

	if _, ok := body["warnings"].([]any); !ok {
		t.Errorf("plan response should carry the walk's warnings, got: %v", body["warnings"])
	}
}

// rebuildServer serves GET /linode/instances/123 (and its disk list, which the
// rebuild dependency walk reads) from the state box and records the POST that
// applies the rebuild, returning the instance body so the client decode passes.
func rebuildServer(
	t *testing.T,
	state *atomic.Pointer[linode.Instance],
	rebuilt *atomic.Bool,
) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rebuild") {
			rebuilt.Store(true)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(state.Load()); err != nil {
			t.Errorf("encode state: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

// TestInstanceRebuildTwoStagePlanThenApply proves the CapDestroy rebuild action,
// which routes through the shared destroy flow, honors plan/apply: the plan
// produces an id and runs the dependency walk (so the body carries warnings)
// without rebuilding, and the apply issues the POST.
func TestInstanceRebuildTwoStagePlanThenApply(t *testing.T) {
	t.Parallel()

	state := instanceState()
	rebuilt := &atomic.Bool{}
	cfg := rebuildServer(t, state, rebuilt)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceRebuildTool(cfg)

	rebuildArgs := map[string]any{
		keyLinodeID: float64(123),
		keyImage:    "linode/ubuntu24.04",
		keyRootPass: "Abcdefgh1234", // betterleaks:allow test fixture
	}

	planArgs := maps.Clone(rebuildArgs)
	planArgs[keyMode] = twostage.ModePlan

	planResult, err := handler(ctx, createRequestWithArgs(t, planArgs))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if planResult.IsError {
		t.Fatalf("plan IsError, text: %s", dryRunResultText(t, planResult))
	}

	if rebuilt.Load() {
		t.Fatal("plan must not issue a rebuild")
	}

	plan := decodeBody(t, dryRunResultText(t, planResult))

	id, _ := plan["plan_id"].(string)
	if id == "" {
		t.Fatalf("plan response has no plan_id: %v", plan)
	}

	if _, ok := plan["warnings"].([]any); !ok {
		t.Errorf("rebuild plan should carry the walk's warnings, got: %v", plan["warnings"])
	}

	applyArgs := maps.Clone(rebuildArgs)
	applyArgs[keyMode] = twostage.ModeApply
	applyArgs[keyPlanID] = id

	applyResult, err := handler(ctx, createRequestWithArgs(t, applyArgs))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if applyResult.IsError {
		t.Fatalf("apply IsError, text: %s", dryRunResultText(t, applyResult))
	}

	if !rebuilt.Load() {
		t.Fatal("apply must issue a rebuild")
	}

	if !strings.Contains(dryRunResultText(t, applyResult), "rebuilt with image") {
		t.Errorf("apply text does not confirm rebuild: %s", dryRunResultText(t, applyResult))
	}
}

// resizeServer serves the GET instance and GET disks that the resize composite
// fetch reads, and records the POST that applies the resize. The disk list is
// stable across plan and apply, so only an intentional change would drift.
func resizeServer(
	t *testing.T,
	state *atomic.Pointer[linode.Instance],
	resized *atomic.Bool,
) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/resize"):
			resized.Store(true)
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/disks"):
			disks := map[string]any{keyData: []any{map[string]any{"id": 1, keySize: 25600, "filesystem": "ext4"}}}
			if err := json.NewEncoder(w).Encode(disks); err != nil {
				t.Errorf("encode disks: %v", err)
			}
		default:
			if err := json.NewEncoder(w).Encode(state.Load()); err != nil {
				t.Errorf("encode state: %v", err)
			}
		}
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

// TestInstanceResizeTwoStageOptInPlanThenApply proves the CapWrite resize tool
// runs plan/apply once an operator opts it in via the two_stage config, and the
// plan carries the resize side-effects produced by the dependency walk.
func TestInstanceResizeTwoStageOptInPlanThenApply(t *testing.T) {
	t.Parallel()

	box := &atomic.Pointer[linode.Instance]{}
	box.Store(&linode.Instance{ID: 123, Label: labelWebProd, Type: typeG6Nanode1, Status: statusRunning})

	resized := &atomic.Bool{}
	cfg := resizeServer(t, box, resized)
	cfg.TwoStage = config.TwoStageConfig{OptIn: map[string]bool{"linode_instance_resize": true}}

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	resizeArgs := map[string]any{keyInstanceID: float64(123), keyType: typeG6Standard1}

	planArgs := maps.Clone(resizeArgs)
	planArgs[keyMode] = twostage.ModePlan

	planResult, err := handler(ctx, createRequestWithArgs(t, planArgs))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if planResult.IsError {
		t.Fatalf("plan IsError, text: %s", dryRunResultText(t, planResult))
	}

	if resized.Load() {
		t.Fatal("plan must not issue a resize")
	}

	plan := decodeBody(t, dryRunResultText(t, planResult))

	id, _ := plan["plan_id"].(string)
	if id == "" {
		t.Fatalf("plan response has no plan_id: %v", plan)
	}

	if _, ok := plan["side_effects"].([]any); !ok {
		t.Errorf("resize plan should carry the walk's side_effects, got: %v", plan["side_effects"])
	}

	applyArgs := maps.Clone(resizeArgs)
	applyArgs[keyMode] = twostage.ModeApply
	applyArgs[keyPlanID] = id

	applyResult, err := handler(ctx, createRequestWithArgs(t, applyArgs))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if applyResult.IsError {
		t.Fatalf("apply IsError, text: %s", dryRunResultText(t, applyResult))
	}

	if !resized.Load() {
		t.Fatal("apply must issue a resize")
	}
}

// TestInstanceResizeTwoStageDefaultOff proves resize stays single-step until an
// operator opts it in: a mode:"plan" call without the config override falls
// through and stores no plan, because CapWrite does not opt in by default.
func TestInstanceResizeTwoStageDefaultOff(t *testing.T) {
	t.Parallel()

	box := &atomic.Pointer[linode.Instance]{}
	box.Store(&linode.Instance{ID: 123, Type: typeG6Nanode1})

	resized := &atomic.Bool{}
	cfg := resizeServer(t, box, resized)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	if _, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyType:       typeG6Standard1,
		keyMode:       twostage.ModePlan,
	})); err != nil {
		t.Fatalf("plan call returned error: %v", err)
	}

	if store.Len() != 0 {
		t.Errorf("resize is opt-in; a default plan call must store nothing, store.Len() = %d", store.Len())
	}

	if resized.Load() {
		t.Error("plan mode must not resize")
	}
}

// TestInstanceDeleteTwoStageApplyUnknownPlan covers an apply that references a
// plan id the store never held.
func TestInstanceDeleteTwoStageApplyUnknownPlan(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModeApply,
		keyPlanID:     "plan_does_not_exist",
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if !result.IsError || !strings.Contains(dryRunResultText(t, result), "PLAN_NOT_FOUND") {
		t.Errorf("apply should report PLAN_NOT_FOUND, got: %s", dryRunResultText(t, result))
	}

	if deleted.Load() {
		t.Error("unknown plan must not issue a DELETE")
	}
}

// TestInstanceDeleteTwoStageApplyExpired covers an apply after the plan TTL has
// elapsed, simulated by advancing the store's clock.
func TestInstanceDeleteTwoStageApplyExpired(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	id := makePlan(ctx, t, handler)

	current = time.Now().Add(10 * time.Minute)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModeApply,
		keyPlanID:     id,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if !result.IsError || !strings.Contains(dryRunResultText(t, result), "PLAN_EXPIRED") {
		t.Errorf("apply should report PLAN_EXPIRED, got: %s", dryRunResultText(t, result))
	}

	if deleted.Load() {
		t.Error("expired plan must not issue a DELETE")
	}
}

// TestInstanceDeleteTwoStageApplyArgsMismatch covers an apply whose supplied
// args differ from the stored plan args.
func TestInstanceDeleteTwoStageApplyArgsMismatch(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	id := makePlan(ctx, t, handler)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(999),
		keyMode:       twostage.ModeApply,
		keyPlanID:     id,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if !result.IsError || !strings.Contains(dryRunResultText(t, result), "PLAN_ARGS_MISMATCH") {
		t.Errorf("apply should report PLAN_ARGS_MISMATCH, got: %s", dryRunResultText(t, result))
	}

	if deleted.Load() {
		t.Error("args mismatch must not issue a DELETE")
	}
}

// makePlan runs a mode:"plan" call for instance 123 and returns its plan_id.
func makePlan(
	ctx context.Context,
	t *testing.T,
	handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
) string {
	t.Helper()

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("plan IsError, text: %s", dryRunResultText(t, result))
	}

	id, _ := decodeBody(t, dryRunResultText(t, result))["plan_id"].(string)
	if id == "" {
		t.Fatal("plan response has no plan_id")
	}

	return id
}

// TestInstanceDeleteTwoStageIgnoresCosmeticDrift confirms a change limited to a
// hash-ignore field (Instance.updated) does not count as drift, so the apply
// still executes.
func TestInstanceDeleteTwoStageIgnoresCosmeticDrift(t *testing.T) {
	t.Parallel()

	state := &atomic.Pointer[linode.Instance]{}
	state.Store(&linode.Instance{
		ID: 123, Label: labelWebProd, Status: statusRunning, Updated: "2026-01-01T00:00:00",
	})

	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	id := makePlan(ctx, t, handler)

	// Only the cosmetic "updated" timestamp moves; this must not be drift.
	state.Store(&linode.Instance{
		ID: 123, Label: labelWebProd, Status: statusRunning, Updated: tsCosmeticBump,
	})

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModeApply,
		keyPlanID:     id,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("a cosmetic-only change must not refuse, got: %s", dryRunResultText(t, result))
	}

	if !deleted.Load() {
		t.Error("apply after a cosmetic-only change must DELETE")
	}
}

// twoStageJSONServer serves the current state map for GET and records each
// DELETE, so one helper drives the plan/apply flow for any delete-by-ID tool
// regardless of the resource type the tool fetches.
func twoStageJSONServer(
	t *testing.T,
	state *atomic.Pointer[map[string]any],
	deleted *atomic.Bool,
) *config.Config {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET serves the current state for the plan/apply hash; any other
		// method is the mutating call. Some opted-in tools execute via POST
		// (backups cancel, password reset) rather than DELETE, so record the
		// mutation on any non-GET method.
		if r.Method != http.MethodGet {
			deleted.Store(true)
			w.WriteHeader(http.StatusOK)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(state.Load()); err != nil {
			t.Errorf("encode state: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
}

// twoStageSingleIDCase drives one single-ID delete tool through plan then apply.
type twoStageSingleIDCase struct {
	name      string
	handlerOf func(*config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	idKey     string
	idVal     any
	baseState map[string]any
}

// twoStageSingleIDCases lists the opted-in single-ID delete tools whose fetched
// state carries an "updated" timestamp the per-type HashIgnore list strips. The
// table lives in its own function so the test body stays within maintidx's
// maintainability bound.
func twoStageSingleIDCases() []twoStageSingleIDCase {
	return []twoStageSingleIDCase{
		{
			name: "volume_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeVolumeDeleteTool(cfg)

				return h
			},
			idKey:     keyVolumeID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "data-01", keyStatus: statusActive, keyUpdated: "2025-12-01T00:00:00"},
		},
		{
			name: "lke_cluster_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeLKEClusterDeleteTool(cfg)

				return h
			},
			idKey:     keyClusterID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "lke-prod", keyStatus: statusReady, keyUpdated: "2025-11-01T00:00:00"},
		},
		{
			name: "firewall_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeFirewallDeleteTool(cfg)

				return h
			},
			idKey:     keyFirewallID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "fw-edge", keyStatus: statusEnabled, keyUpdated: "2025-10-01T00:00:00"},
		},
		{
			name: "nodebalancer_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeNodeBalancerDeleteTool(cfg)

				return h
			},
			idKey:     keyNodeBalancerID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "nb-app", keyUpdated: "2025-09-01T00:00:00"},
		},
		{
			name: "vpc_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeVPCDeleteTool(cfg)

				return h
			},
			idKey:     keyVPCID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "vpc-core", keyUpdated: "2025-08-01T00:00:00"},
		},
		{
			name: "domain_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeDomainDeleteTool(cfg)

				return h
			},
			idKey:     keyDomainID,
			idVal:     float64(123),
			baseState: map[string]any{keyStatus: statusActive, keyUpdated: "2025-07-01T00:00:00"},
		},
		{
			name: "stackscript_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeStackScriptDeleteTool(cfg)

				return h
			},
			idKey:     keyStackScriptID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "deploy-web", "deployments_total": float64(7), keyUpdated: "2025-06-01T00:00:00"},
		},
		{
			name: "sshkey_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeSSHKeyDeleteTool(cfg)

				return h
			},
			idKey:     keySSHKeyID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "laptop-key", keyUpdated: "2025-05-01T00:00:00"},
		},
		{
			name: "placement_group_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodePlacementGroupDeleteTool(cfg)

				return h
			},
			idKey:     keyPlacementGroupID,
			idVal:     "123",
			baseState: map[string]any{keyLabel: "pg-rack", keyUpdated: "2025-04-01T00:00:00"},
		},
		{
			name: "image_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeImageDeleteTool(cfg)

				return h
			},
			idKey:     keyImageID,
			idVal:     "private/123",
			baseState: map[string]any{keyLabel: "golden-img", keyStatus: statusAvailable, keyUpdated: "2025-03-01T00:00:00"},
		},
		{
			name: "database_instance_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeDatabaseInstanceDeleteTool(cfg)

				return h
			},
			idKey:     keyInstanceID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "db-prod", keyStatus: statusActive, keyUpdated: "2025-02-01T00:00:00"},
		},
		{
			name: "database_postgresql_instance_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool(cfg)

				return h
			},
			idKey:     keyInstanceID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "pg-prod", keyStatus: statusActive, keyUpdated: "2025-01-15T00:00:00"},
		},
		{
			name: "image_sharegroup_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeImageShareGroupDeleteTool(cfg)

				return h
			},
			idKey:     keyShareGroupID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "share-team", keyUpdated: "2025-01-10T00:00:00"},
		},
		{
			name: "image_sharegroup_token_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeImageShareGroupTokenDeleteTool(cfg)

				return h
			},
			idKey:     keyTokenUUID,
			idVal:     "11111111-1111-1111-1111-111111111111",
			baseState: map[string]any{keyLabel: "share-team", keyUpdated: "2025-01-09T00:00:00"},
		},
		{
			name: "instance_backups_cancel",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeInstanceBackupsCancelTool(cfg)

				return h
			},
			idKey:     keyLinodeID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "web-01", keyStatus: statusRunning, keyUpdated: "2025-01-08T00:00:00"},
		},
		{
			name: "lke_kubeconfig_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeLKEKubeconfigDeleteTool(cfg)

				return h
			},
			idKey:     keyClusterID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "lke-kc", keyStatus: statusReady, keyUpdated: "2025-01-07T00:00:00"},
		},
		{
			name: "lke_service_token_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeLKEServiceTokenDeleteTool(cfg)

				return h
			},
			idKey:     keyClusterID,
			idVal:     float64(123),
			baseState: map[string]any{keyLabel: "lke-st", keyStatus: statusReady, keyUpdated: "2025-01-06T00:00:00"},
		},
	}
}

// TestTwoStageDeleteToolsAcrossResources runs plan then apply against each
// opted-in single-ID delete tool, bumping only a hash-ignore field between the
// two calls. Each tool must produce a plan_id, skip the DELETE during plan, then
// execute it on apply despite the cosmetic change, proving the tool is opted in
// and its per-type HashIgnore list works.
func TestTwoStageDeleteToolsAcrossResources(t *testing.T) {
	t.Parallel()

	for _, testCase := range twoStageSingleIDCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			base := maps.Clone(testCase.baseState)
			state := &atomic.Pointer[map[string]any]{}
			state.Store(&base)

			deleted := &atomic.Bool{}
			cfg := twoStageJSONServer(t, state, deleted)

			store := twostage.NewPlanStore()
			ctx := tools.WithPlanStore(t.Context(), store)
			handler := testCase.handlerOf(cfg)

			id := twoStagePlanID(ctx, t, handler, testCase.idKey, testCase.idVal, deleted, store)

			// Bump only the cosmetic timestamp; the per-type HashIgnore list
			// must keep this from registering as drift.
			drifted := maps.Clone(base)
			drifted[keyUpdated] = tsCosmeticBump
			state.Store(&drifted)

			applyResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
				testCase.idKey: testCase.idVal,
				keyMode:        twostage.ModeApply,
				keyPlanID:      id,
			}))
			if err != nil {
				t.Fatalf("apply returned error: %v", err)
			}

			if applyResult.IsError {
				t.Fatalf("a cosmetic-only change must not refuse, got: %s", dryRunResultText(t, applyResult))
			}

			if !deleted.Load() {
				t.Error("apply must issue a DELETE")
			}

			if store.Len() != 0 {
				t.Errorf("store.Len() = %d, want 0 (plan consumed)", store.Len())
			}
		})
	}
}

// twoStagePlanID runs the plan call for a delete tool, asserts the plan did not
// delete and was stored, and returns its plan_id for the follow-up apply.
func twoStagePlanID(
	ctx context.Context,
	t *testing.T,
	handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
	idKey string,
	idVal any,
	deleted *atomic.Bool,
	store *twostage.PlanStore,
) string {
	t.Helper()

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		idKey:   idVal,
		keyMode: twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if planResult.IsError {
		t.Fatalf("plan IsError: %s", dryRunResultText(t, planResult))
	}

	if deleted.Load() {
		t.Fatal("plan must not issue a DELETE")
	}

	id, _ := decodeBody(t, dryRunResultText(t, planResult))["plan_id"].(string)
	if id == "" {
		t.Fatalf("plan response has no plan_id")
	}

	if store.Len() != 1 {
		t.Fatalf("store.Len() = %d, want 1", store.Len())
	}

	return id
}

// twoStageTwoIDPlanID runs the plan call for a two-ID delete tool, asserts the
// plan did not delete and was stored, and returns its plan_id.
func twoStageTwoIDPlanID(
	ctx context.Context,
	t *testing.T,
	handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error),
	outerKey, innerKey string,
	deleted *atomic.Bool,
	store *twostage.PlanStore,
) string {
	t.Helper()

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		outerKey: float64(10),
		innerKey: float64(20),
		keyMode:  twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if planResult.IsError {
		t.Fatalf("plan IsError: %s", dryRunResultText(t, planResult))
	}

	if deleted.Load() {
		t.Fatal("plan must not issue a DELETE")
	}

	id, _ := decodeBody(t, dryRunResultText(t, planResult))["plan_id"].(string)
	if id == "" {
		t.Fatalf("plan response has no plan_id")
	}

	if store.Len() != 1 {
		t.Fatalf("store.Len() = %d, want 1", store.Len())
	}

	return id
}

// TestTwoStageTwoIDDeleteTools is the two-ID analog of
// TestTwoStageDeleteToolsAcrossResources: it drives each opted-in delete tool
// that takes an (outer, inner) ID pair through plan then apply, bumping only a
// hash-ignore field between the calls. The two-ID path
// (RunDestructiveActionByTwoIDs) funnels into RunDestructiveAction, so the
// same two-stage branch and HashIgnore handling apply.
func TestTwoStageTwoIDDeleteTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		handlerOf func(*config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		outerKey  string
		innerKey  string
		baseState map[string]any
		cosmetic  string
		driftVal  any
	}{
		{
			name: "instance_disk_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeInstanceDiskDeleteTool(cfg)

				return h
			},
			outerKey:  keyLinodeID,
			innerKey:  keyDiskID,
			baseState: map[string]any{keyLabel: "boot-disk", keyStatus: statusReady, keyUpdated: "2025-12-02T00:00:00"},
			cosmetic:  keyUpdated,
			driftVal:  tsCosmeticBump,
		},
		{
			name: "vpc_subnet_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeVPCSubnetDeleteTool(cfg)

				return h
			},
			outerKey:  keyVPCID,
			innerKey:  keySubnetID,
			baseState: map[string]any{keyLabel: "subnet-a", keyUpdated: "2025-11-02T00:00:00"},
			cosmetic:  keyUpdated,
			driftVal:  tsCosmeticBump,
		},
		{
			name: "domain_record_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeDomainRecordDeleteTool(cfg)

				return h
			},
			outerKey:  keyDomainID,
			innerKey:  keyRecordID,
			baseState: map[string]any{keyName: "www", keyUpdated: "2025-10-02T00:00:00"},
			cosmetic:  keyUpdated,
			driftVal:  tsCosmeticBump,
		},
		{
			name: "lke_pool_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeLKEPoolDeleteTool(cfg)

				return h
			},
			outerKey:  keyClusterID,
			innerKey:  keyPoolID,
			baseState: map[string]any{keyType: typeG6Standard1, "count": float64(3)},
			cosmetic:  "nodes",
			driftVal:  []any{map[string]any{keyStatus: statusReady}},
		},
		{
			name: "firewall_device_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

				return h
			},
			outerKey:  keyFirewallID,
			innerKey:  keyFirewallDeviceID,
			baseState: map[string]any{keyStatus: statusReady, keyUpdated: "2025-12-03T00:00:00"},
			cosmetic:  keyUpdated,
			driftVal:  tsCosmeticBump,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			base := maps.Clone(testCase.baseState)
			state := &atomic.Pointer[map[string]any]{}
			state.Store(&base)

			deleted := &atomic.Bool{}
			cfg := twoStageJSONServer(t, state, deleted)

			store := twostage.NewPlanStore()
			ctx := tools.WithPlanStore(t.Context(), store)
			handler := testCase.handlerOf(cfg)

			id := twoStageTwoIDPlanID(ctx, t, handler, testCase.outerKey, testCase.innerKey, deleted, store)

			drifted := maps.Clone(base)
			drifted[testCase.cosmetic] = testCase.driftVal
			state.Store(&drifted)

			applyResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
				testCase.outerKey: float64(10),
				testCase.innerKey: float64(20),
				keyMode:           twostage.ModeApply,
				keyPlanID:         id,
			}))
			if err != nil {
				t.Fatalf("apply returned error: %v", err)
			}

			if applyResult.IsError {
				t.Fatalf("a cosmetic-only change must not refuse, got: %s", dryRunResultText(t, applyResult))
			}

			if !deleted.Load() {
				t.Error("apply must issue a DELETE")
			}

			if store.Len() != 0 {
				t.Errorf("store.Len() = %d, want 0 (plan consumed)", store.Len())
			}
		})
	}
}

// TestTwoStageConfigTTLOverride confirms a two_stage.default_plan_ttl_seconds
// override drives the plan's lifetime: the gap between created_at and
// expires_at equals the configured value, not the 5-minute built-in default.
func TestTwoStageConfigTTLOverride(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	ttlSeconds := 60
	cfg.TwoStage = config.TwoStageConfig{DefaultPlanTTLSeconds: &ttlSeconds}

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	body := decodeBody(t, dryRunResultText(t, planResult))

	created, errCreated := time.Parse(time.RFC3339, asString(body["created_at"]))
	if errCreated != nil {
		t.Fatalf("parse created_at: %v", errCreated)
	}

	expires, errExpires := time.Parse(time.RFC3339, asString(body["expires_at"]))
	if errExpires != nil {
		t.Fatalf("parse expires_at: %v", errExpires)
	}

	if got := expires.Sub(created); got != 60*time.Second {
		t.Errorf("plan lifetime = %v, want 60s from the config override", got)
	}
}

// TestTwoStageConfigOptOutFallsThrough confirms a two_stage.opt_in entry that
// forces a CapDestroy tool out makes a mode:"plan" call fall through to the
// normal single-step flow: no plan is stored and no delete fires.
func TestTwoStageConfigOptOutFallsThrough(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)
	cfg.TwoStage = config.TwoStageConfig{OptIn: map[string]bool{canRunDestroyTool: false}}

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if store.Len() != 0 {
		t.Errorf("opted-out tool must not store a plan, store.Len() = %d", store.Len())
	}

	if deleted.Load() {
		t.Error("plan mode on an opted-out tool must not DELETE")
	}

	if !result.IsError {
		t.Errorf("opted-out plan call should fall through to the confirm-required error, got: %s", dryRunResultText(t, result))
	}
}

// TestTwoStageConfigPerToolTTLOverride confirms a two_stage.tool_ttl_seconds
// entry drives that tool's plan lifetime, exercising the per-tool branch of the
// settings builder (distinct from the default_plan_ttl_seconds path).
func TestTwoStageConfigPerToolTTLOverride(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)
	cfg.TwoStage = config.TwoStageConfig{
		ToolTTLSeconds: map[string]int{canRunDestroyTool: 120},
	}

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	body := decodeBody(t, dryRunResultText(t, planResult))

	created, errCreated := time.Parse(time.RFC3339, asString(body["created_at"]))
	if errCreated != nil {
		t.Fatalf("parse created_at: %v", errCreated)
	}

	expires, errExpires := time.Parse(time.RFC3339, asString(body["expires_at"]))
	if errExpires != nil {
		t.Fatalf("parse expires_at: %v", errExpires)
	}

	if got := expires.Sub(created); got != 120*time.Second {
		t.Errorf("plan lifetime = %v, want 120s from the per-tool override", got)
	}
}

// TestTwoStagePlanFetchError confirms a failed state fetch during plan returns
// an error result and stores no plan, so there is nothing to apply.
func TestTwoStagePlanFetchError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := tools.NewLinodeInstanceDeleteTool(cfg)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyMode:       twostage.ModePlan,
	}))
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}

	if !result.IsError || !strings.Contains(dryRunResultText(t, result), "Failed to fetch state for plan") {
		t.Errorf("plan with a failing fetch should error, got: %s", dryRunResultText(t, result))
	}

	if store.Len() != 0 {
		t.Errorf("no plan should be stored on fetch failure, Len = %d", store.Len())
	}
}

func asString(value any) string {
	s, _ := value.(string)

	return s
}

func decodeBody(t *testing.T, text string) map[string]any {
	t.Helper()

	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return body
}

// twoStageMultiArgCase drives a delete tool whose path is keyed by something
// other than a single int or two ints (region/label, an IP address, a string
// node id, an IPv6 range, a tag label). args is the tool's full non-control
// argument set, replayed identically on plan and apply.
type twoStageMultiArgCase struct {
	name      string
	handlerOf func(*config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	args      map[string]any
	baseState map[string]any
}

// twoStageMultiArgCases lists the opted-in delete tools that take a non-int-ID
// path. Their fetched state carries no cosmetic timestamp (HashIgnore is nil),
// so the plan and apply run against identical state and the apply must execute
// without a drift refusal. The table lives in its own function to keep the test
// body within maintidx's bound.
func twoStageMultiArgCases() []twoStageMultiArgCase {
	return []twoStageMultiArgCase{
		{
			name: "instance_ip_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeInstanceIPDeleteTool(cfg)

				return h
			},
			args:      map[string]any{keyLinodeID: float64(123), keyAddress: "203.0.113.7"},
			baseState: map[string]any{keyAddress: "203.0.113.7", keyType: keyIPv4, "public": true},
		},
		{
			name: "instance_password_reset",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeInstancePasswordResetTool(cfg)

				return h
			},
			args:      map[string]any{keyLinodeID: float64(123), keyRootPass: "Sup3rSecretPass99"},
			baseState: map[string]any{keyLabel: "web-02", keyStatus: "offline"},
		},
		{
			name: "lke_node_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeLKENodeDeleteTool(cfg)

				return h
			},
			args:      map[string]any{keyClusterID: float64(123), keyNodeID: "node-xyz"},
			baseState: map[string]any{keySupportTicketID: "node-xyz", "instance_id": float64(456), keyStatus: statusReady},
		},
		{
			name: "ipv6_range_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeIPv6RangeDeleteTool(cfg)

				return h
			},
			args:      map[string]any{"ipv6_range": "2001:db8::/64"},
			baseState: map[string]any{"range": "2001:db8::", keyRegion: placementGroupCreateRegion, "prefix": float64(64)},
		},
		{
			name: "tag_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeTagDeleteTool(cfg)

				return h
			},
			args:      map[string]any{"tag_label": "prod"},
			baseState: map[string]any{keyData: []any{}},
		},
		{
			name: "vlan_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeVLANDeleteTool(cfg)

				return h
			},
			args: map[string]any{keyRegionID: placementGroupCreateRegion, keyLabel: "vl-app"},
			baseState: map[string]any{
				keyData: []any{map[string]any{keyRegion: placementGroupCreateRegion, keyLabel: "vl-app", keyPlacementGroupLinodes: []any{}}},
			},
		},
		{
			name: "object_storage_bucket_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

				return h
			},
			args:      map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest},
			baseState: map[string]any{keyLabel: bucketTest, keyRegion: regionUSEast1, "objects": float64(0)},
		},
		{
			name: "object_storage_ssl_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

				return h
			},
			args:      map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest},
			baseState: map[string]any{"ssl": true},
		},
		{
			name: "object_storage_key_delete",
			handlerOf: func(cfg *config.Config) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				_, _, h := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

				return h
			},
			args:      map[string]any{keyKeyID: float64(123)},
			baseState: map[string]any{keyLabel: "ci-key", "access_key": "AK", "id": float64(123)},
		},
	}
}

// TestTwoStageMultiArgDeleteTools drives each opted-in delete tool whose path is
// not a plain single or paired int ID through plan then apply against identical
// state. With no drift, the apply must execute the DELETE, proving the tool is
// opted in and its plan/apply wiring works end to end.
func TestTwoStageMultiArgDeleteTools(t *testing.T) {
	t.Parallel()

	for _, testCase := range twoStageMultiArgCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			base := maps.Clone(testCase.baseState)
			state := &atomic.Pointer[map[string]any]{}
			state.Store(&base)

			deleted := &atomic.Bool{}
			cfg := twoStageJSONServer(t, state, deleted)

			store := twostage.NewPlanStore()
			ctx := tools.WithPlanStore(t.Context(), store)
			handler := testCase.handlerOf(cfg)

			planArgs := maps.Clone(testCase.args)
			planArgs[keyMode] = twostage.ModePlan

			planResult, err := handler(ctx, createRequestWithArgs(t, planArgs))
			if err != nil {
				t.Fatalf("plan returned error: %v", err)
			}

			if planResult.IsError {
				t.Fatalf("plan IsError: %s", dryRunResultText(t, planResult))
			}

			if deleted.Load() {
				t.Fatal("plan must not issue a DELETE")
			}

			id, _ := decodeBody(t, dryRunResultText(t, planResult))["plan_id"].(string)
			if id == "" {
				t.Fatalf("plan response has no plan_id")
			}

			applyArgs := maps.Clone(testCase.args)
			applyArgs[keyMode] = twostage.ModeApply
			applyArgs[keyPlanID] = id

			applyResult, err := handler(ctx, createRequestWithArgs(t, applyArgs))
			if err != nil {
				t.Fatalf("apply returned error: %v", err)
			}

			if applyResult.IsError {
				t.Fatalf("apply on unchanged state must not refuse, got: %s", dryRunResultText(t, applyResult))
			}

			if !deleted.Load() {
				t.Error("apply must issue a DELETE")
			}

			if store.Len() != 0 {
				t.Errorf("store.Len() = %d, want 0 (plan consumed)", store.Len())
			}
		})
	}
}
