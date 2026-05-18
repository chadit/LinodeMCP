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

	"github.com/chadit/LinodeMCP/internal/profiles"
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
