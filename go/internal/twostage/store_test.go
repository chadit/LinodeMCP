package twostage_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

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
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got != entry {
		t.Error("Get returned a different entry pointer than was stored")
	}

	if store.Len() != 1 {
		t.Errorf("Len = %d, want 1", store.Len())
	}
}

// TestStartJanitorSweepsExpiredPlans drives the background sweeper under
// synctest's synthetic clock: a plan that expires while the janitor ticks is
// dropped without any explicit Sweep call. The expiry clock is an atomic so
// the janitor goroutine and the test don't race on it.
func TestStartJanitorSweepsExpiredPlans(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		base := time.Now()

		clock := &atomic.Pointer[time.Time]{}
		clock.Store(&base)

		store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return *clock.Load() }))
		store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: base.Add(time.Minute)})

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		store.StartJanitor(ctx, time.Second)

		// Move the clock past the plan's expiry, then let one tick elapse.
		future := base.Add(2 * time.Minute)
		clock.Store(&future)

		time.Sleep(2 * time.Second)
		synctest.Wait()

		if store.Len() != 0 {
			t.Errorf("janitor did not sweep the expired plan, Len = %d", store.Len())
		}
	})
}

func TestPlanStoreGetUnknownReturnsNotFound(t *testing.T) {
	t.Parallel()

	store := twostage.NewPlanStore()

	if _, err := store.Get("plan_missing"); !errors.Is(err, twostage.ErrPlanNotFound) {
		t.Fatalf("err = %v, want ErrPlanNotFound", err)
	}
}

func TestPlanStoreGetExpiredReturnsExpired(t *testing.T) {
	t.Parallel()

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))

	store.Put(&twostage.PlanEntry{ID: planA, ExpiresAt: current.Add(time.Minute)})

	current = current.Add(2 * time.Minute)

	if _, err := store.Get(planA); !errors.Is(err, twostage.ErrPlanExpired) {
		t.Fatalf("err = %v, want ErrPlanExpired", err)
	}
}

func TestPlanStoreTakeIsSingleUse(t *testing.T) {
	t.Parallel()

	now := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return now }))

	store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: now.Add(time.Minute)})

	got, err := store.Take(planX)
	if err != nil {
		t.Fatalf("first Take returned error: %v", err)
	}

	if got.ID != planX {
		t.Errorf("Take returned ID %q, want %q", got.ID, planX)
	}

	if _, err := store.Take(planX); !errors.Is(err, twostage.ErrPlanNotFound) {
		t.Fatalf("second Take err = %v, want ErrPlanNotFound", err)
	}

	if store.Len() != 0 {
		t.Errorf("Len = %d, want 0 after single use", store.Len())
	}
}

func TestPlanStoreTakeExpiredStillRemoves(t *testing.T) {
	t.Parallel()

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))

	store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: current.Add(time.Minute)})

	current = current.Add(2 * time.Minute)

	if _, err := store.Take(planX); !errors.Is(err, twostage.ErrPlanExpired) {
		t.Fatalf("err = %v, want ErrPlanExpired", err)
	}

	if store.Len() != 0 {
		t.Errorf("Len = %d, want 0 (an expired plan is dropped on take)", store.Len())
	}
}

func TestPlanStoreRemoveIsNoOpWhenAbsent(t *testing.T) {
	t.Parallel()

	store := twostage.NewPlanStore()
	store.Put(&twostage.PlanEntry{ID: planX, ExpiresAt: time.Now().Add(time.Minute)})

	store.Remove(planX)
	store.Remove("plan_never_existed")

	if store.Len() != 0 {
		t.Errorf("Len = %d, want 0", store.Len())
	}
}

func TestPlanStoreSweepDropsOnlyExpired(t *testing.T) {
	t.Parallel()

	current := time.Now()
	store := twostage.NewPlanStore(twostage.WithClock(func() time.Time { return current }))

	store.Put(&twostage.PlanEntry{ID: "plan_short", ExpiresAt: current.Add(time.Minute)})
	store.Put(&twostage.PlanEntry{ID: "plan_long", ExpiresAt: current.Add(time.Hour)})

	current = current.Add(10 * time.Minute)

	if removed := store.Sweep(); removed != 1 {
		t.Errorf("Sweep removed %d, want 1", removed)
	}

	if store.Len() != 1 {
		t.Errorf("Len = %d, want 1", store.Len())
	}

	if _, err := store.Get("plan_long"); err != nil {
		t.Errorf("the long-lived plan should survive the sweep: %v", err)
	}
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

	if store.Len() != twostage.MaxOutstandingPlans {
		t.Fatalf("Len = %d, want %d", store.Len(), twostage.MaxOutstandingPlans)
	}

	store.Put(&twostage.PlanEntry{
		ID:        "plan_newest",
		PlannedAt: base.Add(time.Hour),
		ExpiresAt: base.Add(2 * time.Hour),
	})

	if store.Len() != twostage.MaxOutstandingPlans {
		t.Errorf("Len = %d, want %d (ceiling holds after eviction)", store.Len(), twostage.MaxOutstandingPlans)
	}

	if _, err := store.Get("plan_0000"); !errors.Is(err, twostage.ErrPlanNotFound) {
		t.Errorf("oldest plan should have been evicted, err = %v", err)
	}

	if _, err := store.Get("plan_newest"); err != nil {
		t.Errorf("newest plan should be present: %v", err)
	}
}
