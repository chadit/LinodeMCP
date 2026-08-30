package tools_test

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const (
	planWalkFailureText     = "Failed to compute plan dependencies: walk failed"
	applyRefetchFailureText = "Failed to re-fetch state for apply: "
	keyCurrentStateHash     = "current_state_hash"
)

// planningAction is a destroy action driven straight through the shared flow,
// which is how a tool author sees the plan/apply contract before a generated
// handler wraps it. executed reports whether the removal ran.
func planningAction(fetch stateFetcher, executed *atomic.Bool) *tools.DestructiveAction {
	return &tools.DestructiveAction{
		ToolName:   canRunDestroyTool,
		Method:     http.MethodDelete,
		Path:       instancePath123,
		FetchState: fetch,
		Execute: func(context.Context, *linode.Client) error {
			executed.Store(true)

			return nil
		},
		Success: func() proto.Message {
			return &linodev1.InstanceDeleteResponse{Message: "Instance 123 removed successfully", InstanceId: 123}
		},
	}
}

// noRequestConfig points at a closed port so any read the action performs
// through the client fails loudly instead of hanging.
func noRequestConfig() *config.Config {
	return &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
	}}
}

// runPlan drives a mode:"plan" call for the action and returns the plan_id.
func runPlan(ctx context.Context, t *testing.T, cfg *config.Config, action *tools.DestructiveAction, args map[string]any) string {
	t.Helper()

	planArgs := map[string]any{keyMode: twostage.ModePlan}
	maps.Copy(planArgs, args)

	request := createRequestWithArgs(t, planArgs)

	result, err := tools.RunDestructiveAction(ctx, &request, cfg, action)
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("plan IsError, text: %s", dryRunResultText(t, result))
	}

	id, _ := decodeBody(t, dryRunResultText(t, result))[keyPlanID].(string)
	if id == "" {
		t.Fatal("plan response has no plan_id")
	}

	return id
}

// TestTwoStagePlanRefusesAnEnvironmentTheConfigDoesNotName covers a plan asked
// for under an environment the operator never configured: nothing is read,
// nothing is stored, and the caller reads the same refusal a dry run gives.
func TestTwoStagePlanRefusesAnEnvironmentTheConfigDoesNotName(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := gentools.NewLinodeInstanceDeleteTool(cfg)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123), keyMode: twostage.ModePlan, keyEnvironment: tcStaging,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if got := errorText(t, result); got != envNotFoundStagingText {
		t.Errorf("text = %q, want %q", got, envNotFoundStagingText)
	}

	if store.Len() != 0 {
		t.Errorf("store.Len() = %d, want no plan stored", store.Len())
	}
}

// TestTwoStagePlanReportsTheDependencyWalkFailure covers a walk that could not
// finish during planning: no plan is stored, because a plan is a strict
// superset of a dry run and a dry run would have refused too.
func TestTwoStagePlanReportsTheDependencyWalkFailure(t *testing.T) {
	t.Parallel()

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)

	action := planningAction(func(context.Context, *linode.Client) (any, error) {
		return map[string]any{keyStateID: 123}, nil
	}, &atomic.Bool{})
	action.DependencyWalk = func(context.Context, *linode.Client, any) (tools.DryRunDetails, error) {
		return tools.DryRunDetails{}, errWalkFailed
	}

	request := createRequestWithArgs(t, map[string]any{keyMode: twostage.ModePlan})

	result, err := tools.RunDestructiveAction(ctx, &request, noRequestConfig(), action)
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if got := errorText(t, result); got != planWalkFailureText {
		t.Errorf("text = %q, want %q", got, planWalkFailureText)
	}

	if store.Len() != 0 {
		t.Errorf("store.Len() = %d, want no plan stored", store.Len())
	}
}

// TestTwoStageApplyKeepsThePlanWhenTheRefetchFails covers the API refusing the
// drift check's read at apply time: the removal does not run, the caller reads
// the re-fetch sentence, and the plan stays available for a retry once the
// API answers again.
func TestTwoStageApplyKeepsThePlanWhenTheRefetchFails(t *testing.T) {
	t.Parallel()

	refuseReads := &atomic.Bool{}
	deleted := &atomic.Bool{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			deleted.Store(true)
			w.WriteHeader(http.StatusOK)

			return
		}

		if refuseReads.Load() {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		writeJSON(t, w, Instance{ID: 123, Label: labelWebProd, Status: statusRunning})
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := gentools.NewLinodeInstanceDeleteTool(cfg)

	id := makePlan(ctx, t, handler)

	refuseReads.Store(true)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123), keyMode: twostage.ModeApply, keyPlanID: id,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if got := errorText(t, result); !strings.HasPrefix(got, applyRefetchFailureText) {
		t.Errorf("text = %q, want the re-fetch failure sentence", got)
	}

	if deleted.Load() {
		t.Error("a failed re-fetch must not issue the DELETE")
	}

	if store.Len() != 1 {
		t.Errorf("store.Len() = %d, want the plan kept for a retry", store.Len())
	}
}

// TestTwoStageApplyRefusesWhenTheEnvironmentWentAwayAfterPlanning covers a
// config reload dropping the environment a plan was made under: the apply
// refuses under the environment sentence and never deletes.
func TestTwoStageApplyRefusesWhenTheEnvironmentWentAwayAfterPlanning(t *testing.T) {
	t.Parallel()

	state := instanceState()
	deleted := &atomic.Bool{}
	cfg := twoStageDeleteServer(t, state, deleted)
	cfg.Environments[tcStaging] = cfg.Environments[envKeyDefault]

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)
	_, _, handler := gentools.NewLinodeInstanceDeleteTool(cfg)

	planResult, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123), keyMode: twostage.ModePlan, keyEnvironment: tcStaging,
	}))
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if planResult.IsError {
		t.Fatalf("plan IsError, text: %s", dryRunResultText(t, planResult))
	}

	id, _ := decodeBody(t, dryRunResultText(t, planResult))[keyPlanID].(string)

	delete(cfg.Environments, tcStaging)

	result, err := handler(ctx, createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123), keyMode: twostage.ModeApply, keyPlanID: id, keyEnvironment: tcStaging,
	}))
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if got := errorText(t, result); got != envNotFoundStagingText {
		t.Errorf("text = %q, want %q", got, envNotFoundStagingText)
	}

	if deleted.Load() {
		t.Error("an apply with no environment must not issue the DELETE")
	}
}

// TestTwoStageApplyRefusesArgsAddedAfterPlanning covers an apply that carries
// an argument the plan never had: even with every planned argument intact, the
// extra one is a mismatch and the removal does not run.
func TestTwoStageApplyRefusesArgsAddedAfterPlanning(t *testing.T) {
	t.Parallel()

	executed := &atomic.Bool{}
	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)

	action := planningAction(func(context.Context, *linode.Client) (any, error) {
		return map[string]any{keyStateID: 123}, nil
	}, executed)

	id := runPlan(ctx, t, noRequestConfig(), action, map[string]any{keyInstanceID: float64(123)})

	request := createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123), "force": true, keyMode: twostage.ModeApply, keyPlanID: id,
	})

	result, err := tools.RunDestructiveAction(ctx, &request, noRequestConfig(), action)
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if got := errorText(t, result); !strings.HasPrefix(got, twostage.ErrCodePlanArgsMismatch) {
		t.Errorf("text = %q, want a %s refusal", got, twostage.ErrCodePlanArgsMismatch)
	}

	if executed.Load() {
		t.Error("an apply with extra args must not run the removal")
	}
}

// TestTwoStagePlanHashesACollectionStateAsAWhole covers a resource whose
// current state is a list rather than one object, the shape a rule set or a
// device list reports. The plan carries the list verbatim, an unchanged list
// applies, and a changed one refuses under the generic drift wording because
// a list has no top-level fields to name.
func TestTwoStagePlanHashesACollectionStateAsAWhole(t *testing.T) {
	t.Parallel()

	t.Run("unchanged list applies", func(t *testing.T) {
		t.Parallel()

		executed := &atomic.Bool{}
		store := twostage.NewPlanStore()
		ctx := tools.WithPlanStore(t.Context(), store)

		action := planningAction(func(context.Context, *linode.Client) (any, error) {
			return []any{map[string]any{keyStateID: 1}, map[string]any{keyStateID: 2}}, nil
		}, executed)

		id := runPlan(ctx, t, noRequestConfig(), action, nil)

		request := createRequestWithArgs(t, map[string]any{keyMode: twostage.ModeApply, keyPlanID: id})

		result, err := tools.RunDestructiveAction(ctx, &request, noRequestConfig(), action)
		if err != nil {
			t.Fatalf("apply returned error: %v", err)
		}

		if result.IsError {
			t.Fatalf("apply IsError, text: %s", dryRunResultText(t, result))
		}

		if !executed.Load() {
			t.Error("an unchanged list must apply")
		}
	})

	t.Run("changed list refuses without naming fields", func(t *testing.T) {
		t.Parallel()

		executed := &atomic.Bool{}
		store := twostage.NewPlanStore()
		ctx := tools.WithPlanStore(t.Context(), store)

		items := &atomic.Pointer[[]any]{}
		planned := []any{map[string]any{keyStateID: 1}}
		items.Store(&planned)

		action := planningAction(func(context.Context, *linode.Client) (any, error) {
			return *items.Load(), nil
		}, executed)

		planRequest := createRequestWithArgs(t, map[string]any{keyMode: twostage.ModePlan})

		planResult, err := tools.RunDestructiveAction(ctx, &planRequest, noRequestConfig(), action)
		if err != nil {
			t.Fatalf("plan returned error: %v", err)
		}

		plan := decodeBody(t, dryRunResultText(t, planResult))

		if _, isList := plan[keyCurrentState].([]any); !isList {
			t.Errorf("current_state = %v, want the list verbatim", plan[keyCurrentState])
		}

		if hash := asString(plan[keyCurrentStateHash]); !strings.HasPrefix(hash, "sha256:") {
			t.Errorf("current_state_hash = %q, want a sha256 digest", hash)
		}

		drifted := []any{map[string]any{keyStateID: 1}, map[string]any{keyStateID: 2}}
		items.Store(&drifted)

		request := createRequestWithArgs(t, map[string]any{keyMode: twostage.ModeApply, keyPlanID: asString(plan[keyPlanID])})

		result, err := tools.RunDestructiveAction(ctx, &request, noRequestConfig(), action)
		if err != nil {
			t.Fatalf("apply returned error: %v", err)
		}

		got := errorText(t, result)
		if !strings.HasPrefix(got, twostage.ErrCodePlanDrift) || !strings.Contains(got, "changed fields: one or more fields") {
			t.Errorf("text = %q, want a drift refusal with the generic field wording", got)
		}

		if executed.Load() {
			t.Error("a changed list must not apply")
		}
	})
}

// TestTwoStagePlanRefusesAStateThatCannotBeSerialized covers a fetch handing
// back something JSON cannot encode: the plan refuses with the marshal
// sentence instead of storing a plan under a hash of nothing.
func TestTwoStagePlanRefusesAStateThatCannotBeSerialized(t *testing.T) {
	t.Parallel()

	store := twostage.NewPlanStore()
	ctx := tools.WithPlanStore(t.Context(), store)

	action := planningAction(func(context.Context, *linode.Client) (any, error) {
		return map[string]any{"watcher": make(chan int)}, nil
	}, &atomic.Bool{})

	request := createRequestWithArgs(t, map[string]any{keyMode: twostage.ModePlan})

	result, err := tools.RunDestructiveAction(ctx, &request, noRequestConfig(), action)
	if err != nil {
		t.Fatalf("plan returned error: %v", err)
	}

	if got := errorText(t, result); !strings.HasPrefix(got, "marshal state for hash: ") {
		t.Errorf("text = %q, want the marshal failure sentence", got)
	}

	if store.Len() != 0 {
		t.Errorf("store.Len() = %d, want no plan stored", store.Len())
	}
}
