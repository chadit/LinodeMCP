package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// twoStageSettings resolves the operator-tunable two-stage parameters from
// config into a twostage.Settings. It reads through resolveConfig so a
// hot-reloaded two_stage block takes effect on the next call. Non-positive TTL
// values are dropped here so the resolver falls back to the next level.
func twoStageSettings(cfg *config.Config) twostage.Settings {
	cfgTS := resolveConfig(cfg).TwoStage

	settings := twostage.Settings{OptIn: cfgTS.OptIn}

	if cfgTS.DefaultPlanTTLSeconds != nil && *cfgTS.DefaultPlanTTLSeconds > 0 {
		settings.DefaultTTL = time.Duration(*cfgTS.DefaultPlanTTLSeconds) * time.Second
	}

	if len(cfgTS.ToolTTLSeconds) > 0 {
		settings.ToolTTL = make(map[string]time.Duration, len(cfgTS.ToolTTLSeconds))

		for tool, secs := range cfgTS.ToolTTLSeconds {
			if secs > 0 {
				settings.ToolTTL[tool] = time.Duration(secs) * time.Second
			}
		}
	}

	return settings
}

// planResponse is the wire shape a mode:"plan" call returns: a richer dry-run
// that also carries a plan_id, an expiry, and a hash of the current state. A
// later mode:"apply" call re-fetches the state, re-hashes, and refuses if the
// hash moved.
type planResponse struct {
	PlanID           string        `json:"plan_id"`
	CreatedAt        string        `json:"created_at"`
	ExpiresAt        string        `json:"expires_at"`
	Tool             string        `json:"tool"`
	Environment      string        `json:"environment"`
	WouldExecute     DryRunRequest `json:"would_execute"`
	CurrentState     any           `json:"current_state"`
	CurrentStateHash string        `json:"current_state_hash"`

	// Dependency-walk enrichment, mirroring the dry-run response so a plan is
	// a strict superset of a dry-run. A tool with no DependencyWalk leaves
	// these unset (omitempty), keeping the shape stable.
	Dependencies []DryRunDependency  `json:"dependencies,omitempty"`
	SideEffects  []string            `json:"side_effects,omitempty"`
	BillingDelta *DryRunBillingDelta `json:"billing_delta,omitempty"`
	Warnings     []string            `json:"warnings,omitempty"`
}

// runTwoStageBranch handles the plan and apply branches for a destroy tool. It
// returns handled=false to let the caller fall through to the existing
// single-step flow whenever two-stage does not apply: a permitted yolo call (it
// dominates), a call without mode:"plan"/"apply", a server that did not attach
// a plan store (a direct unit-test call), or a tool that is not opted in.
func runTwoStageBranch(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
) (*mcp.CallToolResult, bool) {
	if yoloAllowedFromContext(ctx) {
		return nil, false
	}

	mode := request.GetString(paramMode, "")
	if mode != twostage.ModePlan && mode != twostage.ModeApply {
		return nil, false
	}

	store := PlanStoreFromContext(ctx)
	if store == nil {
		return nil, false
	}

	if !twoStageSettings(cfg).OptedIn(action.ToolName, action.capability()) {
		return nil, false
	}

	if mode == twostage.ModePlan {
		return runPlan(ctx, request, cfg, action, store), true
	}

	return runApply(ctx, request, cfg, action, store), true
}

// runPlan fetches the current state, hashes it, stores a single-use plan whose
// apply callback re-runs the destroy, and returns the plan preview.
func runPlan(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
	store *twostage.PlanStore,
) *mcp.CallToolResult {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error())
	}

	state, fetchErr := action.FetchState(ctx, client)
	if fetchErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch state for plan: %v", fetchErr))
	}

	hash, fields, hashErr := stateHashAndFields(state, action.HashIgnore)
	if hashErr != nil {
		return mcp.NewToolResultError(hashErr.Error())
	}

	planID, idErr := twostage.NewPlanID()
	if idErr != nil {
		return mcp.NewToolResultError(idErr.Error())
	}

	details, walkErr := planDependencies(ctx, client, action, state)
	if walkErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to compute plan dependencies: %v", walkErr))
	}

	now := time.Now()
	expires := now.Add(twoStageSettings(cfg).PlanTTL(action.ToolName))
	env := request.GetString(paramEnvironment, "")

	store.Put(&twostage.PlanEntry{
		ID:          planID,
		Tool:        action.ToolName,
		Environment: env,
		Args:        nonControlArgs(request.GetArguments()),
		StateHash:   hash,
		StateFields: fields,
		PlannedAt:   now,
		ExpiresAt:   expires,
		Apply: func(applyCtx context.Context) (*mcp.CallToolResult, error) {
			return executeDestroy(applyCtx, request, cfg, action)
		},
	})

	return buildPlanResponse(planID, now, expires, action, env, state, hash, &details)
}

// planDependencies runs the action's dependency walk (if any) so the plan
// response is a strict superset of a dry-run. A nil walk yields empty details.
func planDependencies(
	ctx context.Context,
	client *linode.Client,
	action *DestructiveAction,
	state any,
) (DryRunDetails, error) {
	if action.DependencyWalk == nil {
		return DryRunDetails{}, nil
	}

	return action.DependencyWalk(ctx, client, state)
}

// runApply classifies the referenced plan, resolves the branch, and either runs
// the stored apply callback (after a drift check) or returns a refusal.
func runApply(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
	store *twostage.PlanStore,
) *mcp.CallToolResult {
	planID := request.GetString(paramPlanID, "")

	lookup, entry, changedFields, errResult := classifyPlan(ctx, request, cfg, action, store, planID)
	if errResult != nil {
		return errResult
	}

	decision := twostage.Resolve(twostage.Request{
		Capability:      action.capability(),
		TwoStageOptedIn: true,
		Mode:            twostage.ModeApply,
		PlanID:          planID,
		PlanLookup:      lookup,
	})
	if decision.Branch != twostage.BranchApply {
		return refusalResult(decision.ErrCode, planID, changedFields)
	}

	result, applyErr := entry.Apply(ctx)
	if applyErr != nil {
		return mcp.NewToolResultError(applyErr.Error())
	}

	store.Remove(planID)

	return result
}

// classifyPlan looks up the plan, checks the supplied args against the stored
// args, and re-fetches plus re-hashes the state to detect drift. The returned
// PlanLookup feeds twostage.Resolve. A non-nil result is a hard error (client
// or fetch failure) that short-circuits before resolution.
func classifyPlan(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
	store *twostage.PlanStore,
	planID string,
) (twostage.PlanLookup, *twostage.PlanEntry, []string, *mcp.CallToolResult) {
	entry, lookupErr := store.Get(planID)
	if errors.Is(lookupErr, twostage.ErrPlanExpired) {
		return twostage.PlanLookupExpired, nil, nil, nil
	}

	if lookupErr != nil {
		return twostage.PlanLookupUnknown, nil, nil, nil
	}

	supplied := nonControlArgs(request.GetArguments())
	if len(supplied) > 0 && !argsEqual(supplied, entry.Args) {
		return twostage.PlanLookupArgsMismatch, entry, nil, nil
	}

	client, clientErr := prepareClient(request, cfg)
	if clientErr != nil {
		return twostage.PlanLookupNotApplicable, nil, nil, mcp.NewToolResultError(clientErr.Error())
	}

	state, fetchErr := action.FetchState(ctx, client)
	if fetchErr != nil {
		return twostage.PlanLookupNotApplicable, nil, nil,
			mcp.NewToolResultError(fmt.Sprintf("Failed to re-fetch state for apply: %v", fetchErr))
	}

	hash, fields, hashErr := stateHashAndFields(state, action.HashIgnore)
	if hashErr != nil {
		return twostage.PlanLookupNotApplicable, nil, nil, mcp.NewToolResultError(hashErr.Error())
	}

	if hash != entry.StateHash {
		return twostage.PlanLookupDrifted, entry, changedFieldNames(entry.StateFields, fields), nil
	}

	return twostage.PlanLookupValid, entry, nil, nil
}

// executeDestroy runs the real delete: prepare the client, execute, and marshal
// the success body. It backs both the single-step path in RunDestructiveAction
// and the apply callback a plan stores.
func executeDestroy(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	action *DestructiveAction,
) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if execErr := action.Execute(ctx, client); execErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%s failed: %v", action.ToolName, execErr)), nil
	}

	return marshalDestroySuccess(action.Success())
}

// marshalDestroySuccess serializes a destroy Success body. A proto.Message goes
// through the proto-canonical marshaller so its output matches the Python side;
// everything else falls back to plain JSON for the legacy id-echo maps.
func marshalDestroySuccess(success any) (*mcp.CallToolResult, error) {
	if msg, ok := success.(proto.Message); ok {
		return MarshalProtoToolResponse(msg)
	}

	return MarshalToolResponse(success)
}

// stateHash returns a stable hash of the resource state with the named cosmetic
// fields stripped first, so a plan does not refuse on drift the user never
// caused. Go's json.Marshal sorts map keys, so the same state always encodes
// the same way.
// stateHashAndFields hashes a resource's state for drift detection and also
// returns its normalized top-level field map with the hash-ignore fields
// stripped. The map lets the apply path report which fields changed on a drift
// refusal. It is nil when the state does not serialize to a JSON object (for
// example a bare array), in which case the whole payload is hashed and no
// per-field diff is available. Plan and apply both call this, so the hash is
// computed identically on both sides.
func stateHashAndFields(state any, ignore []string) (string, map[string]any, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", nil, fmt.Errorf("marshal state for hash: %w", err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		sum := sha256.Sum256(data)

		return "sha256:" + hex.EncodeToString(sum[:]), nil, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", nil, fmt.Errorf("unmarshal state for hash: %w", err)
	}

	for _, field := range ignore {
		delete(obj, field)
	}

	stripped, err := json.Marshal(obj)
	if err != nil {
		return "", nil, fmt.Errorf("re-marshal stripped state for hash: %w", err)
	}

	sum := sha256.Sum256(stripped)

	return "sha256:" + hex.EncodeToString(sum[:]), obj, nil
}

// changedFieldNames returns the sorted top-level keys whose values differ
// between the planned and current field maps (added, removed, or changed). It
// drives the changed-fields list in a drift refusal. Comparison is by canonical
// JSON encoding, so key ordering within nested objects is irrelevant. Returns
// nil when either map is nil (no per-field diff available).
func changedFieldNames(planned, current map[string]any) []string {
	if planned == nil || current == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(planned)+len(current))
	for key := range planned {
		seen[key] = struct{}{}
	}

	for key := range current {
		seen[key] = struct{}{}
	}

	var changed []string

	for key := range seen {
		if !jsonEqual(planned[key], current[key]) {
			changed = append(changed, key)
		}
	}

	slices.Sort(changed)

	return changed
}

// jsonEqual reports whether two values have the same canonical JSON encoding.
func jsonEqual(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)

	if leftErr != nil || rightErr != nil {
		return false
	}

	return bytes.Equal(leftBytes, rightBytes)
}

// buildPlanResponse renders the plan preview a mode:"plan" call returns.
func buildPlanResponse(
	planID string,
	createdAt, expiresAt time.Time,
	action *DestructiveAction,
	environment string,
	state any,
	hash string,
	details *DryRunDetails,
) *mcp.CallToolResult {
	result, err := MarshalToolResponse(planResponse{
		PlanID:           planID,
		CreatedAt:        createdAt.UTC().Format(time.RFC3339),
		ExpiresAt:        expiresAt.UTC().Format(time.RFC3339),
		Tool:             action.ToolName,
		Environment:      environment,
		WouldExecute:     DryRunRequest{Method: action.Method, Path: action.Path},
		CurrentState:     state,
		CurrentStateHash: hash,
		Dependencies:     details.Dependencies,
		SideEffects:      details.SideEffects,
		BillingDelta:     details.BillingDelta,
		Warnings:         details.Warnings,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error())
	}

	return result
}

// refusalResult maps an apply refusal code to a message that tells the model
// how to recover.
func refusalResult(errCode, planID string, changedFields []string) *mcp.CallToolResult {
	switch errCode {
	case twostage.ErrCodePlanExpired:
		return mcp.NewToolResultError(fmt.Sprintf(
			"PLAN_EXPIRED: plan %q has expired. Create a new plan with mode: \"plan\".", planID,
		))
	case twostage.ErrCodePlanArgsMismatch:
		return mcp.NewToolResultError(fmt.Sprintf(
			"PLAN_ARGS_MISMATCH: args supplied at apply time differ from plan %q. "+
				"Apply without passing args (the plan retains them), or create a new plan.", planID,
		))
	case twostage.ErrCodePlanDrift:
		changed := "one or more fields"
		if len(changedFields) > 0 {
			changed = strings.Join(changedFields, ", ")
		}

		return mcp.NewToolResultError(fmt.Sprintf(
			"PLAN_DRIFT_DETECTED: the resource changed since plan %q was created "+
				"(changed fields: %s). Create a new plan with mode: \"plan\" and review "+
				"before applying.", planID, changed,
		))
	default:
		return mcp.NewToolResultError(fmt.Sprintf(
			"PLAN_NOT_FOUND: no plan with id %q. Create a new plan with mode: \"plan\". "+
				"Plans do not persist across a server restart.", planID,
		))
	}
}

// nonControlArgs returns the tool's own arguments with the two-stage and
// confirmation control flags stripped, so the apply-time args comparison
// matches only the underlying call.
func nonControlArgs(args map[string]any) map[string]any {
	control := map[string]struct{}{
		paramMode:                {},
		paramPlanID:              {},
		paramConfirm:             {},
		paramDryRun:              {},
		paramConfirmedDryRun:     {},
		paramConfirmBypassDryRun: {},
		paramYolo:                {},
	}

	out := make(map[string]any, len(args))

	for key, val := range args {
		if _, isControl := control[key]; !isControl {
			out[key] = val
		}
	}

	return out
}

// argsEqual reports whether two argument maps are deep-equal. JSON-decoded
// values compare by value, so this implements the spec's strict-equality check.
func argsEqual(supplied, stored map[string]any) bool {
	if len(supplied) != len(stored) {
		return false
	}

	suppliedJSON, errA := json.Marshal(supplied)
	storedJSON, errB := json.Marshal(stored)

	if errA != nil || errB != nil {
		return false
	}

	return bytes.Equal(suppliedJSON, storedJSON)
}
