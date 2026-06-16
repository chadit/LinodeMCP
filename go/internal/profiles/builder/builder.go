// Package builder implements the in-memory draft registry used by the
// Phase 8 profile builder tools. A Draft is a mutable, server-process-local
// snapshot of a Profile under construction. Drafts do not persist across
// restarts. The Save step (Phase 8.5) is the bridge from a Draft back into
// Config.Profiles.
//
// This package is intentionally independent of the server package and the
// MCP wire format. Phase 8.2 onward wraps Registry operations in tool
// handlers; the wrapping lives in internal/tools, not here.
package builder

import (
	"slices"
	"sync"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// Draft is the in-memory shape of a profile under construction. Field
// semantics match profiles.Profile so the Save step can produce a
// config.UserProfileConfig without translation. Disabled is intentionally
// absent: drafts cannot be saved into a disabled state.
type Draft struct {
	Name                string
	Description         string
	AllowedTools        []string
	AllowedEnvironments []string
	RequiredTokenScopes []string
	AllowYolo           bool
}

// Registry holds drafts keyed by name. Safe for concurrent use.
//
// The MCP server has exactly one Registry per process; each builder tool
// handler resolves it from the Server. Drafts share the registry across
// concurrent tool calls so a `_show` can race with a `_add_tools`. The
// RWMutex serializes the operations.
type Registry struct {
	mu     sync.RWMutex
	drafts map[string]*Draft
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{drafts: make(map[string]*Draft)}
}

// Create starts a new draft. If cloneFrom is non-nil, the draft seeds its
// fields from that profile (tools, environments, scopes, yolo flag).
// Description seeds from cloneFrom.Description if cloneFrom is set,
// otherwise empty. The seeded slices are copied so later edits to the
// draft do not mutate the source profile.
//
// Returns ErrDraftNameEmpty if name is empty, ErrDraftExists if a draft
// with that name already lives in the registry.
func (r *Registry) Create(name string, cloneFrom *profiles.Profile) (*Draft, error) {
	if name == "" {
		return nil, ErrDraftNameEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.drafts[name]; ok {
		return nil, ErrDraftExists
	}

	draft := &Draft{Name: name}
	if cloneFrom != nil {
		draft.Description = cloneFrom.Description
		draft.AllowedTools = slices.Clone(cloneFrom.AllowedTools)
		draft.AllowedEnvironments = slices.Clone(cloneFrom.AllowedEnvironments)
		draft.RequiredTokenScopes = slices.Clone(cloneFrom.RequiredTokenScopes)
		draft.AllowYolo = cloneFrom.AllowYolo
	}

	r.drafts[name] = draft

	return draft, nil
}

// Get returns the draft and true if a draft with the given name lives in
// the registry, otherwise nil and false. The returned pointer is the
// registry's own draft; callers that mutate it should hold no assumption
// of isolation. Phase 8.4 tool handlers acquire the registry's lock
// through dedicated mutation methods rather than mutating the pointer
// directly.
func (r *Registry) Get(name string) (*Draft, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	draft, ok := r.drafts[name]

	return draft, ok
}

// Discard removes the named draft. Returns true if the draft was present,
// false if no draft by that name existed. Idempotent.
func (r *Registry) Discard(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.drafts[name]; !ok {
		return false
	}

	delete(r.drafts, name)

	return true
}

// List returns the names of every draft currently in the registry, sorted.
// Returns an empty slice (never nil) when the registry is empty.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.drafts))
	for name := range r.drafts {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

// AddTools expands the given patterns against the catalog, merges the
// matches into the named draft's AllowedTools (deduplicated), and
// returns the sorted list of newly-added names. Names already on the
// draft are not duplicated and not reported in the return value.
//
// Returns ErrDraftNotFound when the draft is not in the registry.
// An empty patterns slice or one that matches nothing returns an
// empty slice and no error: tools may want to call _add_tools with
// zero patterns to confirm the draft still exists.
func (r *Registry) AddTools(
	draftName string,
	patterns []string,
	catalog []profiles.ToolDescriptor,
) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	draft, ok := r.drafts[draftName]
	if !ok {
		return nil, ErrDraftNotFound
	}

	matched := MatchPatterns(patterns, catalog)
	existing := make(map[string]struct{}, len(draft.AllowedTools))

	for _, name := range draft.AllowedTools {
		existing[name] = struct{}{}
	}

	added := make([]string, 0, len(matched))

	for _, name := range matched {
		if _, dup := existing[name]; dup {
			continue
		}

		existing[name] = struct{}{}

		added = append(added, name)
	}

	merged := make([]string, 0, len(existing))
	for name := range existing {
		merged = append(merged, name)
	}

	slices.Sort(merged)
	slices.Sort(added)

	draft.AllowedTools = merged

	return added, nil
}

// RemoveTools expands the patterns against the draft's CURRENT
// AllowedTools and removes the matches. Patterns target the draft's
// state directly so a wildcard like "linode_instance_*" removes
// exactly the instance tools the draft already had, regardless of
// what the live catalog contains.
//
// Returns ErrDraftNotFound when the draft is not in the registry.
// Returns the sorted list of removed names.
func (r *Registry) RemoveTools(draftName string, patterns []string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	draft, ok := r.drafts[draftName]
	if !ok {
		return nil, ErrDraftNotFound
	}

	// Build a synthetic catalog of the draft's current tools so
	// MatchPatterns drives over them. Capability isn't used by the
	// matcher; the zero value is fine.
	catalog := make([]profiles.ToolDescriptor, len(draft.AllowedTools))
	for i, name := range draft.AllowedTools {
		catalog[i] = profiles.ToolDescriptor{Name: name}
	}

	matched := MatchPatterns(patterns, catalog)
	removeSet := make(map[string]struct{}, len(matched))

	for _, name := range matched {
		removeSet[name] = struct{}{}
	}

	kept := make([]string, 0, len(draft.AllowedTools))

	for _, name := range draft.AllowedTools {
		if _, rm := removeSet[name]; rm {
			continue
		}

		kept = append(kept, name)
	}

	draft.AllowedTools = kept

	return matched, nil
}

// SetAllowedEnvironments replaces the draft's AllowedEnvironments
// with the given list. Empty slice and nil are both valid (means
// "any environment", matching profiles.Profile semantics).
//
// Returns ErrDraftNotFound when the draft is not in the registry.
func (r *Registry) SetAllowedEnvironments(draftName string, envs []string) error {
	return r.mutate(draftName, func(draft *Draft) {
		draft.AllowedEnvironments = slices.Clone(envs)
	})
}

// SetRequiredTokenScopes replaces the draft's RequiredTokenScopes
// with the given list. Empty slice and nil are both valid (means
// "no scope requirement declared").
//
// Returns ErrDraftNotFound when the draft is not in the registry.
func (r *Registry) SetRequiredTokenScopes(draftName string, scopes []string) error {
	return r.mutate(draftName, func(draft *Draft) {
		draft.RequiredTokenScopes = slices.Clone(scopes)
	})
}

// SetAllowYolo sets the draft's AllowYolo flag.
//
// Returns ErrDraftNotFound when the draft is not in the registry.
func (r *Registry) SetAllowYolo(draftName string, allow bool) error {
	return r.mutate(draftName, func(draft *Draft) {
		draft.AllowYolo = allow
	})
}

// mutate is the locked-and-found helper that the per-field setters
// share. Keeps the per-setter body to a single field assignment.
func (r *Registry) mutate(draftName string, apply func(*Draft)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	draft, ok := r.drafts[draftName]
	if !ok {
		return ErrDraftNotFound
	}

	apply(draft)

	return nil
}
