package twostage

import (
	"fmt"

	"github.com/google/uuid"
)

// PlanIDPrefix marks two-stage plan identifiers so the model can tell them
// apart from other IDs it handles.
const PlanIDPrefix = "plan_"

// NewPlanID returns a fresh, time-sortable plan identifier. UUIDv7 embeds a
// millisecond timestamp in its leading bits, so lexical order tracks creation
// order (the property the spec wanted from ULIDs) without taking on a new
// dependency: google/uuid is already in the module graph.
func NewPlanID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate plan id: %w", err)
	}

	return PlanIDPrefix + id.String(), nil
}
