package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
)

// msgBuilderUnconfigured is what a builder tool answers when it was called
// with no server state attached. Both languages answer these exact words, so a
// caller reads the same refusal whichever binary served it.
const msgBuilderUnconfigured = "draft registry not configured"

// BuilderState is the server-scoped state the profile-builder tools read: the
// draft registry they mutate, the catalog they compose a profile against, the
// profile a pre-check answers for, and the configuration a clone source is
// resolved against. The server attaches it to every call's context, the way it
// attaches the two-stage plan store.
//
// Catalog and ActiveProfile are read at call time rather than captured, so a
// hot reload is reflected without re-registering a tool.
type BuilderState struct {
	Drafts        *builder.Registry
	Catalog       func() []profiles.ToolDescriptor
	ActiveProfile func() profiles.Profile
	Config        *config.Config
}

// whole reports whether every member the builder tools read is present. A
// partial state is treated as no state: a tool that reached a nil member would
// fail on the deref rather than answer the sentence the caller can act on.
func (s *BuilderState) whole() bool {
	return s != nil && s.Drafts != nil && s.Catalog != nil &&
		s.ActiveProfile != nil && s.Config != nil
}

// builderStateCtxKey namespaces the builder state on a context. Its own
// unexported type is what keeps the value from colliding with another
// package's.
type builderStateCtxKey struct{}

// WithBuilderState attaches the server's profile-builder state to a context.
// The server middleware sets it on every call; tools that are not builder tools
// ignore it.
func WithBuilderState(ctx context.Context, state *BuilderState) context.Context {
	return context.WithValue(ctx, builderStateCtxKey{}, state)
}

// BuilderStateFromContext returns the state the server attached, or nil when a
// handler was called without a server behind it (a unit test calling it
// directly). A builder tool refuses rather than answering from nothing.
func BuilderStateFromContext(ctx context.Context) *BuilderState {
	state, _ := ctx.Value(builderStateCtxKey{}).(*BuilderState)

	return state
}

// BuilderStateOrRefusal answers the attached state, or the refusal the tool
// reports in its place. Exactly one of the two is non-nil.
func BuilderStateOrRefusal(ctx context.Context) (*BuilderState, *mcp.CallToolResult) {
	state := BuilderStateFromContext(ctx)
	if !state.whole() {
		return nil, mcp.NewToolResultError(msgBuilderUnconfigured)
	}

	return state, nil
}
