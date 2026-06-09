package twostage

import (
	"context"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// MaxOutstandingPlans caps the store. Once it is reached, the next Put evicts
// the plan with the oldest PlannedAt to make room. A later apply against an
// evicted plan returns ErrPlanNotFound, the same as for an unknown ID.
const MaxOutstandingPlans = 1000

// ApplyFunc executes the planned action at apply time. It runs only after the
// drift check passes. It must resolve config live rather than capturing a
// plan-time snapshot, so a token rotation or environment change between plan
// and apply takes effect (see the spec's handler-closure rule).
type ApplyFunc func(ctx context.Context) (*mcp.CallToolResult, error)

// PlanEntry is one outstanding plan held in process memory.
type PlanEntry struct {
	Apply       ApplyFunc
	Args        map[string]any
	ID          string
	Tool        string
	Environment string
	StateHash   string
	// StateFields is the normalized top-level field map of the planned state
	// (hash-ignore fields already stripped). On a drift refusal the apply path
	// diffs it against the re-fetched state to report which fields changed. Nil
	// when the state did not serialize to a JSON object.
	StateFields map[string]any
	PlannedAt   time.Time
	ExpiresAt   time.Time
}

// PlanStore holds outstanding plans in process memory. Plans do not survive a
// restart. A janitor drops expired entries on an interval; Put also enforces a
// hard ceiling by evicting the oldest plan when the store is full.
type PlanStore struct {
	now   func() time.Time
	plans map[string]*PlanEntry
	mu    sync.RWMutex
}

// Option configures a PlanStore at construction time.
type Option func(*PlanStore)

// WithClock overrides the wall clock the store uses for expiry decisions.
// Tests inject a controllable clock; production passes nothing and gets
// time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *PlanStore) {
		s.now = now
	}
}

// NewPlanStore returns an empty plan store. By default it reads the real wall
// clock; pass WithClock to override.
func NewPlanStore(opts ...Option) *PlanStore {
	store := &PlanStore{
		now:   time.Now,
		plans: make(map[string]*PlanEntry),
	}

	for _, opt := range opts {
		opt(store)
	}

	return store
}

// Put stores a plan, evicting the oldest entry first when the store is at its
// ceiling.
func (s *PlanStore) Put(entry *PlanEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.plans) >= MaxOutstandingPlans {
		s.evictOldestLocked()
	}

	s.plans[entry.ID] = entry
}

// Get returns a plan without consuming it. It reports ErrPlanNotFound for an
// unknown ID and ErrPlanExpired when the TTL has elapsed.
func (s *PlanStore) Get(planID string) (*PlanEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.plans[planID]
	if !ok {
		return nil, ErrPlanNotFound
	}

	if s.now().After(entry.ExpiresAt) {
		return nil, ErrPlanExpired
	}

	return entry, nil
}

// Take returns a plan and removes it in the same locked section, giving the
// apply path single-use semantics and preventing a concurrent double-apply.
// An expired plan is still removed; the caller receives ErrPlanExpired.
func (s *PlanStore) Take(planID string) (*PlanEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.plans[planID]
	if !ok {
		return nil, ErrPlanNotFound
	}

	delete(s.plans, planID)

	if s.now().After(entry.ExpiresAt) {
		return nil, ErrPlanExpired
	}

	return entry, nil
}

// Remove drops a plan by ID. It is a no-op when the ID is absent.
func (s *PlanStore) Remove(planID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.plans, planID)
}

// Sweep drops every expired plan and returns how many it removed. The janitor
// calls this on an interval; tests call it directly.
func (s *PlanStore) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()

	var removed int

	for key, entry := range s.plans {
		if now.After(entry.ExpiresAt) {
			delete(s.plans, key)

			removed++
		}
	}

	return removed
}

// Len reports the number of outstanding plans.
func (s *PlanStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.plans)
}

// StartJanitor runs a background loop that sweeps expired plans on the given
// interval until the context is canceled. It returns immediately; the loop
// owns its own goroutine.
func (s *PlanStore) StartJanitor(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Sweep()
			}
		}
	}()
}

// evictOldestLocked removes the plan with the earliest PlannedAt. The caller
// must hold the write lock.
func (s *PlanStore) evictOldestLocked() {
	var (
		oldestID string
		oldestAt time.Time
	)

	for key, entry := range s.plans {
		if oldestID == "" || entry.PlannedAt.Before(oldestAt) {
			oldestID = key
			oldestAt = entry.PlannedAt
		}
	}

	if oldestID != "" {
		delete(s.plans, oldestID)
	}
}
