package twostage_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/twostage"
)

const (
	planA = "plan_a"
	planX = "plan_x"
)

func TestPlanStorePutAndGet(t *testing.T) {
	t.Parallel()

	now := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return now }))

	entry := &twostage.PlanEntry{ID: planA, ExpiresAt: now.Add(time.Minute)}
	store.Put(entry)

	got, err := store.Get(planA)
	require.NoError(t, err)
	assert.Same(t, entry, got)
	assert.Equal(t, 1, store.Len())
}

func TestPlanStoreGetUnknownReturnsNotFound(t *testing.T) {
	t.Parallel()

	store := twostage.NewPlanStore()

	_, err := store.Get("plan_missing")
	require.ErrorIs(t, err, twostage.ErrPlanNotFound)
}

func TestPlanStoreGetExpiredReturnsExpired(t *testing.T) {
	t.Parallel()

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))

	store.Put(&twostage.PlanEntry{ID: planA, ExpiresAt: current.Add(time.Minute)})

	current = current.Add(2 * time.Minute)

	_, err := store.Get(planA)
	require.ErrorIs(t, err, twostage.ErrPlanExpired)
}

func TestPlanStoreTakeIsSingleUse(t *testing.T) {
	t.Parallel()

	now := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return now }))

	store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: now.Add(time.Minute)})

	got, err := store.Take(planX)
	require.NoError(t, err)
	assert.Equal(t, planX, got.ID)

	_, err = store.Take(planX)
	require.ErrorIs(t, err, twostage.ErrPlanNotFound)
	assert.Equal(t, 0, store.Len())
}

func TestPlanStoreTakeExpiredStillRemoves(t *testing.T) {
	t.Parallel()

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))

	store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: current.Add(time.Minute)})

	current = current.Add(2 * time.Minute)

	_, err := store.Take(planX)
	require.ErrorIs(t, err, twostage.ErrPlanExpired)
	assert.Equal(t, 0, store.Len(), "an expired plan is dropped on take")
}

func TestPlanStoreRemoveIsNoOpWhenAbsent(t *testing.T) {
	t.Parallel()

	store := twostage.NewPlanStore()
	store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: time.Now().Add(time.Minute)})

	store.Remove(planX)
	store.Remove("plan_never_existed")

	assert.Equal(t, 0, store.Len())
}

func TestPlanStoreSweepDropsOnlyExpired(t *testing.T) {
	t.Parallel()

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))

	store.Put(&twostage.PlanEntry{ID: "plan_short", ExpiresAt: current.Add(time.Minute)})
	store.Put(&twostage.PlanEntry{ID: "plan_long", ExpiresAt: current.Add(time.Hour)})

	current = current.Add(10 * time.Minute)

	removed := store.Sweep()
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, store.Len())

	_, err := store.Get("plan_long")
	require.NoError(t, err)
}

func TestPlanStorePutEvictsOldestAtCeiling(t *testing.T) {
	t.Parallel()

	base := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return base }))

	for idx := range twostage.MaxOutstandingPlans {
		store.Put(&twostage.PlanEntry{
			ID:        fmt.Sprintf("plan_%04d", idx),
			PlannedAt: base.Add(time.Duration(idx) * time.Millisecond),
			ExpiresAt: base.Add(time.Hour),
		})
	}

	require.Equal(t, twostage.MaxOutstandingPlans, store.Len())

	store.Put(&twostage.PlanEntry{
		ID:        "plan_newest",
		PlannedAt: base.Add(time.Hour),
		ExpiresAt: base.Add(2 * time.Hour),
	})

	assert.Equal(t, twostage.MaxOutstandingPlans, store.Len(), "ceiling holds after eviction")

	_, err := store.Get("plan_0000")
	require.ErrorIs(t, err, twostage.ErrPlanNotFound, "the oldest plan was evicted")

	newest, err := store.Get("plan_newest")
	require.NoError(t, err)
	assert.Equal(t, "plan_newest", newest.ID)
}
