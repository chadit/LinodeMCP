package twostage_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/chadit/LinodeMCP/internal/twostage"
)

func TestNewPlanIDHasPrefixAndIsValidV7(t *testing.T) {
	t.Parallel()

	id, err := twostage.NewPlanID()
	if err != nil {
		t.Fatalf("NewPlanID returned error: %v", err)
	}

	if !strings.HasPrefix(id, twostage.PlanIDPrefix) {
		t.Fatalf("id %q lacks the %q prefix", id, twostage.PlanIDPrefix)
	}

	parsed, err := uuid.Parse(strings.TrimPrefix(id, twostage.PlanIDPrefix))
	if err != nil {
		t.Fatalf("plan id body is not a valid UUID: %v", err)
	}

	if parsed.Version() != 7 {
		t.Errorf("UUID version = %d, want 7 for time-sortability", parsed.Version())
	}
}

func TestNewPlanIDIsUnique(t *testing.T) {
	t.Parallel()

	const count = 1000

	seen := make(map[string]struct{}, count)

	for range count {
		id, err := twostage.NewPlanID()
		if err != nil {
			t.Fatalf("NewPlanID returned error: %v", err)
		}

		if _, dup := seen[id]; dup {
			t.Fatalf("plan id collision: %s", id)
		}

		seen[id] = struct{}{}
	}
}
