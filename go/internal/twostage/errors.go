package twostage

import "errors"

// Apply-path error codes. They double as the "error" field in the structured
// tool result the spec defines, so callers can branch on a stable string.
const (
	ErrCodePlanNotFound     = "PLAN_NOT_FOUND"
	ErrCodePlanExpired      = "PLAN_EXPIRED"
	ErrCodePlanDrift        = "PLAN_DRIFT_DETECTED"
	ErrCodePlanArgsMismatch = "PLAN_ARGS_MISMATCH"
)

// Sentinel errors for the store layer. The apply helper maps these to the
// structured result shapes in a later slice; keeping them as sentinels lets
// the store stay free of any mcp-go dependency.
var (
	// ErrPlanNotFound is returned when no plan with the given ID exists, either
	// because it was never created, was already applied (single-use), or the
	// process restarted and dropped all in-memory plans.
	ErrPlanNotFound = errors.New("no plan with that id")

	// ErrPlanExpired is returned when a plan exists but its TTL has elapsed.
	ErrPlanExpired = errors.New("plan expired")
)
