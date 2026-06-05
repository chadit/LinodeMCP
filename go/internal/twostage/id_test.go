package twostage_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/twostage"
)

func TestNewPlanIDHasPrefixAndIsValidV7(t *testing.T) {
	t.Parallel()

	id, err := twostage.NewPlanID()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(id, twostage.PlanIDPrefix), "id %q lacks the plan_ prefix", id)

	parsed, err := uuid.Parse(strings.TrimPrefix(id, twostage.PlanIDPrefix))
	require.NoError(t, err)
	assert.Equal(t, 7, int(parsed.Version()), "plan id must embed a UUIDv7 for time-sortability")
}

func TestNewPlanIDIsUnique(t *testing.T) {
	t.Parallel()

	const count = 1000

	seen := make(map[string]struct{}, count)

	for range count {
		id, err := twostage.NewPlanID()
		require.NoError(t, err)

		_, dup := seen[id]
		require.False(t, dup, "plan id collision: %s", id)

		seen[id] = struct{}{}
	}
}
