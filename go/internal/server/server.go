// Package server provides the LinodeMCP MCP server implementation.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/internal/tools"
	"github.com/chadit/LinodeMCP/pkg/contracts"
)

// toolHandler is the callback signature mcp-go invokes for each tool call.
type toolHandler = func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// toolEntry is the staged shape produced by the per-category collectors before
// profile filtering decides which tools actually reach mcp-go. Pass 1 of the
// two-pass registration builds a flat slice of these; pass 2 calls addTool
// only for entries whose name appears in the resolved profile's AllowedTools.
type toolEntry struct {
	tool       mcp.Tool
	capability profiles.Capability
	handler    toolHandler
}

// Server represents a LinodeMCP server.
type Server struct {
	config        *config.Config
	mcp           *server.MCPServer
	tools         []contracts.Tool
	activeProfile profiles.Profile

	// shutdownMu serializes positive inflight.Add calls with Shutdown's
	// inflight.Wait call. sync.WaitGroup requires Add to happen before Wait
	// when the counter is zero, so tool dispatch must pass this gate first.
	shutdownMu   sync.Mutex
	shuttingDown bool
	inflight     sync.WaitGroup

	// allEntries holds every tool the server could register, regardless of
	// the active profile. Built once in New and reused by ReloadProfile so a
	// profile change can re-add tools that were filtered out at startup
	// without re-running the per-category collectors.
	allEntries []toolEntry

	// profileMu guards activeProfile, tools, and registered against
	// concurrent reads from Tools/ActiveProfile/ToolInfos and writes from
	// ReloadProfile. mcp-go's internal mutex protects its own tool map, but
	// the Server's view of which tools are live needs its own gate.
	profileMu sync.RWMutex

	// registered tracks the tools currently live in mcp-go by name, so
	// ReloadProfile can compute additions and removals without walking the
	// tools slice. The map value is the index into tools for O(1) lookup
	// when rebuilding the slice after a reload.
	registered map[string]*toolWrapper

	// draftRegistry holds Phase 8 profile-builder drafts. One per server
	// process. Drafts live in memory only; the Phase 8.5 _draft_save tool
	// is the bridge from this registry back into Config.Profiles.
	draftRegistry *builder.Registry

	// auditSink consumes audit events emitted by the per-handler
	// capture middleware (Phase 1b). Defaults to NoopSink so Phase
	// 1b ships without a real sink; Phase 2 swaps in the JSONL
	// writer. Tests inject a CapturingSink via SetAuditSink to
	// assert handler-call events.
	auditSink audit.Sink

	// auditRedactPII selects which redaction tier the capture
	// middleware uses (Phase 4c). False applies the always-on
	// credential list only; true also applies the PII list. main wires
	// this to cfg.Audit.RedactPII via SetAuditRedactPII at startup.
	// Default false so tests that build a Server without going through
	// main keep credential-only redaction behavior; production startup
	// flips it to true unless the operator opts out.
	auditRedactPII bool
}

// New creates a new LinodeMCP server. Returns an error if config is nil or if
// the active profile cannot be resolved (unknown profile, disabled built-in,
// etc.). A resolution error surfaces here rather than at request time so a
// misconfigured server fails fast instead of silently registering nothing.
func New(cfg *config.Config) (*Server, error) {
	if cfg == nil {
		return nil, ErrConfigNil
	}

	mcpServer := server.NewMCPServer(
		cfg.Server.Name,
		appinfo.Version,
		server.WithToolCapabilities(true),
	)

	srv := &Server{
		config:        cfg,
		mcp:           mcpServer,
		tools:         make([]contracts.Tool, 0),
		registered:    make(map[string]*toolWrapper),
		draftRegistry: builder.NewRegistry(),
		auditSink:     audit.NoopSink{},
	}

	srv.allEntries = collectAllToolEntries(cfg)
	srv.allEntries = append(srv.allEntries, builderToolEntries(srv)...)

	if err := srv.registerTools(); err != nil {
		return nil, err
	}

	return srv, nil
}

// builderToolEntries assembles the Phase 8 profile-builder tool
// entries. Built outside collectAllToolEntries because the builder
// handlers need a closure over the server itself (Server.ToolCatalog,
// future draft registry access). The CapMeta tag means the active
// profile filter always passes them through.
//
// Calling Server.ToolCatalog from a builder handler returns the
// catalog as it stands at call time, including the builder tools
// themselves. That self-inclusion is deliberate: a user composing a
// new profile may want to know which builder tools they're inheriting.
func builderToolEntries(srv *Server) []toolEntry {
	listTool, listCap, listHandler := tools.NewLinodeProfileListToolsTool(srv.ToolCatalog)
	catTool, catCap, catHandler := tools.NewLinodeProfileListCategoriesTool(srv.ToolCatalog)
	newTool, newCap, newHandler := tools.NewLinodeProfileDraftNewTool(srv.draftRegistry, srv.LookupProfile)
	showTool, showCap, showHandler := tools.NewLinodeProfileDraftShowTool(srv.draftRegistry)
	discardTool, discardCap, discardHandler := tools.NewLinodeProfileDraftDiscardTool(srv.draftRegistry)
	addToolsTool, addToolsCap, addToolsHandler := tools.NewLinodeProfileDraftAddToolsTool(srv.draftRegistry, srv.ToolCatalog)
	removeToolsTool, removeToolsCap, removeToolsHandler := tools.NewLinodeProfileDraftRemoveToolsTool(srv.draftRegistry)
	setTool, setCap, setHandler := tools.NewLinodeProfileDraftSetTool(srv.draftRegistry)
	saveTool, saveCap, saveHandler := tools.NewLinodeProfileDraftSaveTool(srv.draftRegistry, config.GetConfigPath)

	return []toolEntry{
		{tool: listTool, capability: listCap, handler: listHandler},
		{tool: catTool, capability: catCap, handler: catHandler},
		{tool: newTool, capability: newCap, handler: newHandler},
		{tool: showTool, capability: showCap, handler: showHandler},
		{tool: discardTool, capability: discardCap, handler: discardHandler},
		{tool: addToolsTool, capability: addToolsCap, handler: addToolsHandler},
		{tool: removeToolsTool, capability: removeToolsCap, handler: removeToolsHandler},
		{tool: setTool, capability: setCap, handler: setHandler},
		{tool: saveTool, capability: saveCap, handler: saveHandler},
	}
}

type toolWrapper struct {
	tool       mcp.Tool
	capability profiles.Capability
}

func (tw *toolWrapper) Name() string        { return tw.tool.Name }
func (tw *toolWrapper) Description() string { return tw.tool.Description }
func (tw *toolWrapper) InputSchema() any    { return tw.tool.InputSchema }

// Capability returns the tool's capability tag. Server-internal accessor used
// by the invariant tests and (in later phases) the audit middleware. The
// public pkg/contracts/Tool interface deliberately does not expose this.
func (tw *toolWrapper) Capability() profiles.Capability { return tw.capability }

// RawTool returns the underlying mcp.Tool so the invariant tests can inspect
// the input schema. Server-internal accessor.
func (tw *toolWrapper) RawTool() mcp.Tool { return tw.tool }

func (*toolWrapper) Execute(ctx context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("context canceled: %w", ctx.Err())
	default:
		return nil, ErrExecuteNotImplemented
	}
}

// Tools returns the registered tool list. The slice is a snapshot copy so
// callers can iterate safely even if ReloadProfile mutates the live set
// concurrently.
func (s *Server) Tools() []contracts.Tool {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	out := make([]contracts.Tool, len(s.tools))
	copy(out, s.tools)

	return out
}

// ActiveProfile returns the profile the server is currently running under.
// Reflects the most recent successful ReloadProfile if one has been called;
// otherwise the profile resolved at construction time. Returned by value so
// callers cannot mutate the server's internal state.
func (s *Server) ActiveProfile() profiles.Profile {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	return s.activeProfile
}

// LookupProfile resolves a profile by name across both built-in and
// user-defined entries. Used by Phase 8.3 _draft_new with the
// optional clone_from parameter. Returns the materialized Profile and
// true on hit; the zero Profile and false on miss. Ignores the
// Disabled flag so users can clone from disabled built-ins like
// full-access and emergency. User-defined entries shadow built-ins
// by name, matching ResolveActiveProfile's precedence.
func (s *Server) LookupProfile(name string) (profiles.Profile, bool) {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	descriptors := make([]profiles.ToolDescriptor, len(s.allEntries))
	for i := range s.allEntries {
		descriptors[i] = profiles.ToolDescriptor{
			Name:       s.allEntries[i].tool.Name,
			Capability: s.allEntries[i].capability,
		}
	}

	return profiles.LookupProfile(name, s.config, descriptors)
}

// DraftRegistry returns the server's in-memory profile-builder draft
// registry. Phase 8.3+ builder tool handlers acquire it through this
// accessor; tests inject a fresh registry by constructing their own
// Server. The returned pointer is stable for the server's lifetime.
func (s *Server) DraftRegistry() *builder.Registry {
	return s.draftRegistry
}

// ToolCatalog returns the full set of tools the server could register,
// regardless of the active profile's filter. The Phase 8 builder tools
// read this to surface the registerable surface to the model; profile
// filtering controls which subset reaches handlers, but the catalog is
// always the full menu so the user can build a new profile against
// anything available.
//
// Returns a snapshot copy so callers can iterate without holding the
// server lock. Order matches the construction order in
// collectAllToolEntries (category-by-category, factory-by-factory).
func (s *Server) ToolCatalog() []profiles.ToolDescriptor {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	out := make([]profiles.ToolDescriptor, len(s.allEntries))
	for i := range s.allEntries {
		out[i] = profiles.ToolDescriptor{
			Name:       s.allEntries[i].tool.Name,
			Capability: s.allEntries[i].capability,
		}
	}

	return out
}

// ValidateScopes runs Phase 6.4 token-scope validation against the
// active profile. It builds a Linode client from the default
// environment in the current config and delegates to
// profiles.ValidateScopes for PAT-vs-OAuth dispatch.
//
// Returns profiles.ErrTokenNotConfigured (no API call made) when the
// active environment has no token set; the caller decides whether to
// fail load (elevated profile) or warn-and-continue (read-only) per
// the missing-token policy. Other errors come from the underlying
// /profile and /profile/grants calls, wrapped in
// ErrProfileFetchFailed / ErrGrantsFetchFailed.
//
// On success, the returned ScopeValidationResult carries the actual
// scope set, the diff against the profile's required scopes, and the
// token kind for audit logging.
func (s *Server) ValidateScopes(ctx context.Context) (*profiles.ScopeValidationResult, error) {
	s.profileMu.RLock()
	cfg := s.config
	required := append([]profiles.Scope(nil), parseRequiredScopes(s.activeProfile.RequiredTokenScopes)...)
	s.profileMu.RUnlock()

	env, err := cfg.SelectEnvironment("default")
	if err != nil {
		return nil, fmt.Errorf("select default environment for scope validation: %w", err)
	}

	if env.Linode.Token == "" {
		return nil, profiles.ErrTokenNotConfigured
	}

	client := linode.NewClient(env.Linode.APIURL, env.Linode.Token, cfg)

	result, err := profiles.ValidateScopes(ctx, client, required)
	if err != nil {
		return nil, fmt.Errorf("validate scopes: %w", err)
	}

	return result, nil
}

// parseRequiredScopes converts the profile's stored []string scope
// values into the typed Scope slice ValidateScopes expects. Stored as
// strings so user-defined profiles can declare custom scopes the
// catalog doesn't yet name; the cast back to Scope is a string alias
// so no data is lost.
func parseRequiredScopes(stored []string) []profiles.Scope {
	out := make([]profiles.Scope, len(stored))
	for i, s := range stored {
		out[i] = profiles.Scope(s)
	}

	return out
}

// ToolInfo describes a registered tool's capability and input schema for the
// capability invariant tests. The public contracts.Tool deliberately stays
// minimal; this accessor lives on Server so tests in package server_test can
// inspect the capability tag without widening the public contract.
type ToolInfo struct {
	Name        string
	Capability  profiles.Capability
	InputSchema mcp.ToolInputSchema
}

// ToolInfos returns one entry per registered tool, exposing the capability
// tag and input schema. Test-only accessor; the audit middleware reads
// capability via its own server-internal path.
func (s *Server) ToolInfos() []ToolInfo {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	out := make([]ToolInfo, 0, len(s.tools))

	for _, t := range s.tools {
		wrapper, ok := t.(*toolWrapper)
		if !ok {
			continue
		}

		out = append(out, ToolInfo{
			Name:        wrapper.tool.Name,
			Capability:  wrapper.capability,
			InputSchema: wrapper.tool.InputSchema,
		})
	}

	return out
}

// AllToolInfos returns one entry per tool the server could register,
// independent of the active profile's filter. ToolInfos only sees the
// tools the active profile exposes; this returns the full catalog from
// allEntries. The audit redaction-coverage invariant uses this because
// a sensitive arg must be redacted whenever ANY profile can expose the
// tool, not just the profile that happens to be active.
func (s *Server) AllToolInfos() []ToolInfo {
	out := make([]ToolInfo, 0, len(s.allEntries))

	for i := range s.allEntries {
		out = append(out, ToolInfo{
			Name:        s.allEntries[i].tool.Name,
			Capability:  s.allEntries[i].capability,
			InputSchema: s.allEntries[i].tool.InputSchema,
		})
	}

	return out
}

// HandleMessage dispatches a JSON-RPC message into the underlying mcp-go
// server. Exposes the in-process transport for tests and embedders that
// don't go through stdio. Tool handlers invoked via this path are still
// tracked in the inflight WaitGroup, so Shutdown drains them correctly.
func (s *Server) HandleMessage(ctx context.Context, message json.RawMessage) mcp.JSONRPCMessage {
	return s.mcp.HandleMessage(ctx, message)
}

// Shutdown blocks until all in-flight tool handlers complete or ctx is
// canceled. Returns ctx.Err() on timeout so callers can distinguish a clean
// drain from a forced cutoff.
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdownMu.Lock()
	s.shuttingDown = true
	s.shutdownMu.Unlock()

	done := make(chan struct{})

	go func() {
		s.inflight.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown drain timed out: %w", ctx.Err())
	}
}

// Start starts the LinodeMCP server using stdio transport.
func (s *Server) Start(ctx context.Context) error {
	log.Printf("Starting LinodeMCP server with %d tools", len(s.tools))

	for _, tool := range s.tools {
		log.Printf("Registered tool: %s - %s", tool.Name(), tool.Description())
	}

	log.Printf("LinodeMCP server started")

	errCh := make(chan error, 1)

	go func() {
		errCh <- server.ServeStdio(s.mcp)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		return nil
	}
}

type toolFactory func(*config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error))

// ReloadProfile swaps the running server to the profile resolved from cfg.
// Tools the new profile allows that weren't previously registered are added;
// tools the new profile excludes are removed. mcp-go fires
// notifications/tools/list_changed on both paths so connected clients
// refresh their tool cache.
//
// On error (unknown profile, disabled built-in, malformed config) the server
// keeps its current active profile and tool set. A failed reload is a
// no-op, not a partial update.
//
// Concurrency: holds s.profileMu in write mode for the duration. Reads on
// Tools/ActiveProfile/ToolInfos block until the swap completes. In-flight
// tool handlers that already passed the dispatch gate continue to run; the
// reload only changes what the gate accepts on future calls.
func (s *Server) ReloadProfile(cfg *config.Config) error {
	if cfg == nil {
		return ErrConfigNil
	}

	s.profileMu.Lock()
	defer s.profileMu.Unlock()

	profile, err := s.resolveProfileLocked(cfg)
	if err != nil {
		return err
	}

	newAllowed := make(map[string]struct{}, len(profile.AllowedTools))
	for _, name := range profile.AllowedTools {
		newAllowed[name] = struct{}{}
	}

	toRemove := make([]string, 0, len(s.registered))

	for name := range s.registered {
		if _, ok := newAllowed[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}

	if len(toRemove) > 0 {
		s.mcp.DeleteTools(toRemove...)

		for _, name := range toRemove {
			delete(s.registered, name)
		}
	}

	var addedCount int

	for i := range s.allEntries {
		entry := &s.allEntries[i]

		if _, ok := newAllowed[entry.tool.Name]; !ok {
			continue
		}

		if _, alreadyLive := s.registered[entry.tool.Name]; alreadyLive {
			continue
		}

		s.addTool(&entry.tool, entry.capability, entry.handler)

		addedCount++
	}

	s.tools = s.tools[:0]

	for i := range s.allEntries {
		name := s.allEntries[i].tool.Name
		if w, ok := s.registered[name]; ok {
			s.tools = append(s.tools, w)
		}
	}

	previous := s.activeProfile.Name
	s.activeProfile = profile
	s.config = cfg

	slog.Info(
		"profile reloaded",
		"previous", previous,
		"current", profile.Name,
		"added", addedCount,
		"removed", len(toRemove),
		"live", len(s.registered),
	)

	return nil
}

// SetAuditSink swaps the audit sink. Phase 2 main wires this to the
// JSONL writer at startup; tests use it to inject CapturingSink
// before exercising the dispatch path. Passing nil restores the
// NoopSink default rather than producing a nil-deref crash.
func (s *Server) SetAuditSink(sink audit.Sink) {
	if sink == nil {
		sink = audit.NoopSink{}
	}

	s.auditSink = sink
}

// SetAuditRedactPII selects the redaction tier the capture middleware
// applies to event args (Phase 4c). Main wires this to
// cfg.Audit.RedactPII at startup; tests use it to opt into PII
// redaction when asserting the combined-redaction path.
func (s *Server) SetAuditRedactPII(redactPII bool) {
	s.auditRedactPII = redactPII
}

// addTool registers a tool with mcp-go and the local list, wrapping the
// handler so each in-flight invocation is tracked in s.inflight. Shutdown
// uses that WaitGroup to drain handlers before returning. Takes the tool by
// pointer to satisfy gocritic's hugeParam check; mcp.AddTool needs a value
// so we deref at the call boundary. Capability is stashed on the wrapper so
// invariant tests and the audit middleware can read it without a side table.
//
// Phase 1b adds audit-event capture: every reaching handler builds an
// Event at entry and writes it to s.auditSink at exit. The default
// sink is NoopSink, so Phase 1b ships without observable behavior
// change; Phase 2 swaps in the JSONL writer.
//
// Caller MUST hold s.profileMu in write mode. addTool mutates s.tools and
// s.registered, both of which are guarded by that mutex.
func (s *Server) addTool(tool *mcp.Tool, capability profiles.Capability, handler toolHandler) {
	toolName := tool.Name
	auditCapability := profilesCapabilityToAudit(capability)

	wrapped := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.shutdownMu.Lock()
		if s.shuttingDown {
			s.shutdownMu.Unlock()
			s.writeRefusalAuditEvent(ctx, toolName, auditCapability, &req, errServerShuttingDown)

			return nil, errServerShuttingDown
		}

		s.inflight.Add(1)
		s.shutdownMu.Unlock()

		defer s.inflight.Done()

		evt := s.newAuditEvent(toolName, auditCapability, &req)
		start := time.Now()

		result, err := handler(ctx, req)

		finalizeAuditEvent(&evt, start, err)
		s.auditSink.Write(context.WithoutCancel(ctx), &evt)

		return result, err
	}

	s.mcp.AddTool(*tool, wrapped)

	wrapper := &toolWrapper{tool: *tool, capability: capability}
	s.tools = append(s.tools, wrapper)
	s.registered[tool.Name] = wrapper
}

// newAuditEvent constructs an audit event for a reaching handler.
// Reads the active profile name under the profile read-lock so a
// concurrent hot-reload doesn't observe a torn pointer. Request is
// passed by pointer because mcp.CallToolRequest is ~80 bytes;
// gocritic flags the value form for hot-path callers.
func (s *Server) newAuditEvent(
	toolName string,
	capability audit.Capability,
	req *mcp.CallToolRequest,
) audit.Event {
	args := req.GetArguments()
	environment, _ := args["environment"].(string)

	s.profileMu.RLock()
	profileName := s.activeProfile.Name
	s.profileMu.RUnlock()

	return audit.NewEvent(
		toolName,
		capability,
		args,
		environment,
		profileName,
		"",
		0,
		appinfo.Version,
		s.auditRedactPII,
	)
}

// writeRefusalAuditEvent records a refusal at the shutdown gate. The
// handler never ran, so latency is zero and the error message names
// the refusal reason (errServerShuttingDown today). Future refusal
// sources (Phase 4 dry-run gate, validation failures) can call this
// same helper with their own error value.
func (s *Server) writeRefusalAuditEvent(
	ctx context.Context,
	toolName string,
	capability audit.Capability,
	req *mcp.CallToolRequest,
	refusalErr error,
) {
	evt := s.newAuditEvent(toolName, capability, req)
	evt.Finalize(audit.StatusRefused, 0, refusalErr.Error(), "")
	s.auditSink.Write(context.WithoutCancel(ctx), &evt)
}

// finalizeAuditEvent sets status/latency/error based on handler
// outcome. Success when err is nil; Error otherwise. Result-summary
// generation lands in Phase 2; for Phase 1b it stays empty.
func finalizeAuditEvent(evt *audit.Event, start time.Time, err error) {
	latency := time.Since(start)

	if err == nil {
		evt.Finalize(audit.StatusSuccess, latency, "", "")

		return
	}

	evt.Finalize(audit.StatusError, latency, err.Error(), "")
}

// profilesCapabilityToAudit translates the profiles capability tag
// into the audit-wire capability string. Kept here rather than in
// the audit package so the audit package stays dependency-free of
// profiles (per the package comment in event.go).
func profilesCapabilityToAudit(capability profiles.Capability) audit.Capability {
	switch capability {
	case profiles.CapRead:
		return audit.CapabilityRead
	case profiles.CapWrite:
		return audit.CapabilityWrite
	case profiles.CapDestroy:
		return audit.CapabilityDestroy
	case profiles.CapAdmin:
		return audit.CapabilityAdmin
	case profiles.CapMeta:
		return audit.CapabilityMeta
	case profiles.CapUnknown:
		return ""
	default:
		return ""
	}
}

// entryFromFactory invokes a tool factory and packages its three return
// values into a toolEntry. Pass 1 of registerTools relies on this so each
// per-category collector can stay free of mcp.AddTool side effects.
func entryFromFactory(cfg *config.Config, factory toolFactory) toolEntry {
	tool, capability, handler := factory(cfg)

	return toolEntry{tool: tool, capability: capability, handler: handler}
}

// entriesFromFactories applies entryFromFactory across a slice and returns
// the resulting entries. Per-category collectors call this so each list of
// factories stays a single expression.
func entriesFromFactories(cfg *config.Config, factories []toolFactory) []toolEntry {
	entries := make([]toolEntry, 0, len(factories))
	for _, factory := range factories {
		entries = append(entries, entryFromFactory(cfg, factory))
	}

	return entries
}

// registerTools runs the two-pass registration that produces the active
// tool surface. Pass 1 was completed by the caller (New) and stashed in
// s.allEntries. Pass 2 resolves the active profile against those entries
// and registers only the tools the profile permits.
//
// Returns the error from profiles.ResolveActiveProfile verbatim if the
// configured profile is unknown or disabled. The error wraps the package's
// sentinel via fmt.Errorf("...: %w", ...) so callers can match with
// errors.Is.
func (s *Server) registerTools() error {
	s.profileMu.Lock()
	defer s.profileMu.Unlock()

	profile, err := s.resolveProfileLocked(s.config)
	if err != nil {
		return err
	}

	s.activeProfile = profile

	allowed := make(map[string]struct{}, len(profile.AllowedTools))
	for _, name := range profile.AllowedTools {
		allowed[name] = struct{}{}
	}

	for i := range s.allEntries {
		entry := &s.allEntries[i]
		if _, ok := allowed[entry.tool.Name]; !ok {
			slog.Info(
				"profile filtered out tool at registration",
				"profile", profile.Name,
				"tool", entry.tool.Name,
				"capability", entry.capability.String(),
			)

			continue
		}

		s.addTool(&entry.tool, entry.capability, entry.handler)
	}

	return nil
}

// resolveProfileLocked is the shared pre-flight that registerTools and
// ReloadProfile use to validate a config against the cached registry. It
// returns the resolved Profile or the wrapped error from
// profiles.ResolveActiveProfile. Caller must hold s.profileMu (read or
// write); this method does not touch mutable server state.
func (s *Server) resolveProfileLocked(cfg *config.Config) (profiles.Profile, error) {
	registry := make([]profiles.ToolDescriptor, len(s.allEntries))
	for i := range s.allEntries {
		registry[i] = profiles.ToolDescriptor{
			Name:       s.allEntries[i].tool.Name,
			Capability: s.allEntries[i].capability,
		}
	}

	profile, err := profiles.ResolveActiveProfile(cfg, registry)
	if err != nil {
		return profiles.Profile{}, fmt.Errorf("resolve active profile: %w", err)
	}

	return profile, nil
}

// ToolDescriptors returns the flat list of (name, capability) pairs for
// every tool the package can register against the given config. Lets
// the CLI subcommands (profile list/show) enumerate the catalog without
// instantiating a full Server (which would require a valid active
// profile and would start up internal state). Pure: no goroutines, no
// I/O, no global side effects.
func ToolDescriptors(cfg *config.Config) []profiles.ToolDescriptor {
	entries := collectAllToolEntries(cfg)
	out := make([]profiles.ToolDescriptor, len(entries))

	for i := range entries {
		out[i] = profiles.ToolDescriptor{
			Name:       entries[i].tool.Name,
			Capability: entries[i].capability,
		}
	}

	return out
}

// collectAllToolEntries returns the flat list of every tool the server
// could register, ignoring profile filtering. Used by Server.registerTools
// for the descriptor list it passes to profile resolution, and by the
// package-level ToolDescriptors helper for CLI enumeration. Takes the
// config directly so callers that haven't built a Server can still get
// the catalog.
func collectAllToolEntries(cfg *config.Config) []toolEntry {
	categoryEntries := [][]toolEntry{
		coreToolEntries(cfg),
		computeToolEntries(cfg),
		networkingToolEntries(cfg),
		dnsToolEntries(cfg),
		volumeToolEntries(cfg),
		objectStorageToolEntries(cfg),
		databaseToolEntries(cfg),
		lkeToolEntries(cfg),
		vpcToolEntries(cfg),
		instanceDeepToolEntries(cfg),
	}

	var total int
	for _, group := range categoryEntries {
		total += len(group)
	}

	entries := make([]toolEntry, 0, total)
	for _, group := range categoryEntries {
		entries = append(entries, group...)
	}

	return entries
}

func coreToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewHelloTool,
		tools.NewVersionTool,
		tools.NewLinodeProfileTool,
		tools.NewLinodeAccountTool,
		tools.NewLinodeAccountTransferTool,
		tools.NewLinodeAccountSettingsTool,
		tools.NewLinodeAccountSettingsUpdateTool,
		tools.NewLinodeAccountSettingsManagedEnableTool,
		tools.NewLinodeManagedCredentialsTool,
		tools.NewLinodeManagedCredentialUpdateTool,
		tools.NewLinodeManagedCredentialUsernamePasswordUpdateTool,
		tools.NewLinodeManagedSSHKeyTool,
		tools.NewLinodeManagedCredentialCreateTool,
		tools.NewLinodeManagedCredentialGetTool,
		tools.NewLinodeManagedCredentialRevokeTool,
		tools.NewLinodeManagedServiceCreateTool,
		tools.NewLinodeManagedLinodeSettingsGetTool,
		tools.NewLinodeManagedContactGetTool,
		tools.NewLinodeLongviewClientGetTool,
		tools.NewLinodeLongviewSubscriptionGetTool,
		tools.NewLinodeAccountAgreementsTool,
		tools.NewLinodeAccountMaintenanceTool,
		tools.NewLinodeMaintenancePoliciesTool,
		tools.NewLinodeManagedContactDeleteTool,
		tools.NewLinodeManagedContactsTool,
		tools.NewLinodeManagedLinodeSettingsTool,
		tools.NewLinodeManagedStatsTool,
		tools.NewLinodeManagedLinodeSettingsUpdateTool,
		tools.NewLinodeManagedServiceDeleteTool,
		tools.NewLinodeManagedServiceDisableTool,
		tools.NewLinodeManagedServiceEnableTool,
		tools.NewLinodeManagedServiceGetTool,
		tools.NewLinodeManagedServiceUpdateTool,
		tools.NewLinodeManagedServicesTool,
		tools.NewLinodeManagedIssueGetTool,
		tools.NewLinodeManagedIssuesTool,
		tools.NewLinodeManagedContactUpdateTool,
		tools.NewLinodeAccountNotificationsTool,
		tools.NewLinodeLongviewPlanTool,
		tools.NewLinodeLongviewTypesTool,
		tools.NewLinodeLongviewSubscriptionsTool,
		tools.NewLinodeMonitorServicesTool,
		tools.NewLinodeMonitorServiceGetTool,
		tools.NewLinodeMonitorServiceMetricDefinitionsTool,
		tools.NewLinodeMonitorServiceAlertDefinitionsTool,
		tools.NewLinodeMonitorServiceDashboardsTool,
		tools.NewLinodeMonitorServiceMetricsTool,
		tools.NewLinodeMonitorServiceTokenCreateTool,
		tools.NewLinodeMonitorServiceAlertDefinitionGetTool,
		tools.NewLinodeMonitorServiceAlertDefinitionCreateTool,
		tools.NewLinodeMonitorServiceAlertDefinitionDeleteTool,

		tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool,
		tools.NewLinodeMonitorDashboardsTool,
		tools.NewLinodeMonitorDashboardGetTool,
		tools.NewLinodeMonitorAlertDefinitionsTool,
		tools.NewLinodeMonitorAlertChannelsTool,
		tools.NewLinodeLongviewClientCreateTool,
		tools.NewLinodeLongviewPlanUpdateTool,
		tools.NewLinodeBetasTool,
		tools.NewLinodeBetaGetTool,
		tools.NewLinodeAccountBetasTool,
		tools.NewLinodeAccountOAuthClientsTool,
		tools.NewLinodeLongviewClientsTool,
		tools.NewLinodeLongviewClientUpdateTool,
		tools.NewLinodeLongviewClientDeleteTool,
		tools.NewLinodeAccountPaymentMethodsTool,
		tools.NewLinodeAccountPaymentMethodGetTool,
		tools.NewLinodeAccountPaymentMethodCreateTool,
		tools.NewLinodeAccountPaymentMethodDeleteTool,
		tools.NewLinodeAccountPaymentMethodMakeDefaultTool,
		tools.NewLinodeAccountOAuthClientGetTool,
		tools.NewLinodeAccountOAuthClientCreateTool,
		tools.NewLinodeAccountOAuthClientUpdateTool,
		tools.NewLinodeAccountOAuthClientThumbnailUpdateTool,
		tools.NewLinodeAccountOAuthClientThumbnailGetTool,
		tools.NewLinodeAccountOAuthClientDeleteTool,
		tools.NewLinodeAccountOAuthClientResetSecretTool,
		tools.NewLinodeAccountEventsTool,
		tools.NewLinodeAccountUsersTool,
		tools.NewLinodeAccountUserGetTool,
		tools.NewLinodeAccountUserGrantsTool,
		tools.NewLinodeAccountUserGrantsUpdateTool,
		tools.NewLinodeAccountUserUpdateTool,
		tools.NewLinodeAccountUserDeleteTool,
		tools.NewLinodeAccountUserCreateTool,
		tools.NewLinodeManagedContactCreateTool,
		tools.NewLinodeAccountLoginsTool,
		tools.NewLinodeAccountLoginGetTool,
		tools.NewLinodeAccountInvoicesTool,
		tools.NewLinodeAccountPaymentsTool,
		tools.NewLinodeAccountPaymentGetTool,
		tools.NewLinodeAccountPaymentCreateTool,
		tools.NewLinodeAccountPromoCreditTool,
		tools.NewLinodeAccountInvoiceGetTool,
		tools.NewLinodeAccountInvoiceItemsTool,
		tools.NewLinodeAccountChildAccountsTool,
		tools.NewLinodeAccountEntityTransfersTool,
		tools.NewLinodeAccountServiceTransfersTool,
		tools.NewLinodeAccountServiceTransferGetTool,
		tools.NewLinodeAccountServiceTransferCreateTool,
		tools.NewLinodeAccountServiceTransferDeleteTool,
		tools.NewLinodeAccountServiceTransferAcceptTool,
		tools.NewLinodeAccountEntityTransferGetTool,
		tools.NewLinodeAccountEventGetTool,
		tools.NewLinodeAccountEventSeenTool,
		tools.NewLinodeAccountEntityTransferCreateTool,
		tools.NewLinodeAccountEntityTransferAcceptTool,
		tools.NewLinodeAccountEntityTransferDeleteTool,
		tools.NewLinodeAccountChildAccountGetTool,
		tools.NewLinodeAccountChildAccountTokenTool,
		tools.NewLinodeAccountBetaGetTool,
		tools.NewLinodeAccountBetaEnrollTool,
		tools.NewLinodeAccountAvailabilityTool,
		tools.NewLinodeAccountAvailabilityGetTool,
		tools.NewLinodeAccountAgreementsAcknowledgeTool,
		tools.NewLinodeAccountCancelTool,
		tools.NewLinodeAccountUpdateTool,
		tools.NewLinodeAuditRecentTool,
		tools.NewLinodeAuditSummaryTool,
		tools.NewLinodeAuditHealthTool,
		tools.NewLinodeAuditExportTool,
		tools.NewLinodeAuditReportTool,
	})
}

func computeToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeInstanceListTool,
		tools.NewLinodeInstanceGetTool,
		tools.NewLinodeInstanceStatsByYearMonthTool,
		tools.NewLinodeInstanceTransferGetTool,
		tools.NewLinodePlacementGroupAssignTool,
		tools.NewLinodePlacementGroupGetTool,
		tools.NewLinodePlacementGroupDeleteTool,
		tools.NewLinodeRegionListTool,
		tools.NewLinodePlacementGroupListTool,
		tools.NewLinodePlacementGroupUpdateTool,
		tools.NewLinodeKernelListTool,
		tools.NewLinodeKernelGetTool,
		tools.NewLinodeTypeListTool,
		tools.NewLinodeTypeGetTool,
		tools.NewLinodeImageListTool,
		tools.NewLinodeImageGetTool,
		tools.NewLinodeImageDeleteTool,
		tools.NewLinodeImageUploadTool,
		tools.NewLinodeImageReplicateTool,
		tools.NewLinodePlacementGroupCreateTool,
		tools.NewLinodePlacementGroupUnassignTool,
		tools.NewLinodeImageUpdateTool,

		tools.NewLinodeImageShareGroupsListTool,
		tools.NewLinodeImageShareGroupGetTool,
		tools.NewLinodeImageShareGroupsByImageListTool,
		tools.NewLinodeImageShareGroupImagesListTool,
		tools.NewLinodeImageShareGroupMembersListTool,
		tools.NewLinodeImageShareGroupMemberTokenGetTool,
		tools.NewLinodeImageShareGroupMemberUpdateTool,

		tools.NewLinodeImageShareGroupCreateTool,
		tools.NewLinodeImageShareGroupImagesAddTool,
		tools.NewLinodeImageShareGroupImageUpdateTool,
		tools.NewLinodeImageShareGroupMembersAddTool,
		tools.NewLinodeImageShareGroupUpdateTool,

		tools.NewLinodeImageShareGroupDeleteTool,
		tools.NewLinodeImageShareGroupImageDeleteTool,
		tools.NewLinodeImageShareGroupTokensListTool,
		tools.NewLinodeImageShareGroupTokenCreateTool,
		tools.NewLinodeImageShareGroupTokenGetTool,
		tools.NewLinodeImageShareGroupTokenDeleteTool,
		tools.NewLinodeImageShareGroupMemberTokenDeleteTool,
		tools.NewLinodeImageShareGroupTokenImagesListTool,
		tools.NewLinodeImageShareGroupTokenUpdateTool,
		tools.NewLinodeImageShareGroupByTokenGetTool,
		tools.NewLinodeImageCreateTool,
		tools.NewLinodeSSHKeyListTool,
		tools.NewLinodeSSHKeyGetTool,
		tools.NewLinodeStackScriptGetTool,
		tools.NewLinodeStackScriptListTool,
		tools.NewLinodeStackScriptCreateTool,
		tools.NewLinodeStackScriptDeleteTool,
		tools.NewLinodeStackScriptUpdateTool,
		tools.NewLinodeSSHKeyCreateTool,
		tools.NewLinodeSSHKeyUpdateTool,
		tools.NewLinodeSSHKeyDeleteTool,
		tools.NewLinodeInstanceBootTool,
		tools.NewLinodeInstanceRebootTool,
		tools.NewLinodeInstanceShutdownTool,
		tools.NewLinodeInstanceCreateTool,
		tools.NewLinodeInstanceDeleteTool,
		tools.NewLinodeInstanceResizeTool,
	})
}

func networkingToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeFirewallListTool,
		tools.NewLinodeVLANsListTool,
		tools.NewLinodeVLANDeleteTool,
		tools.NewLinodeFirewallRulesListTool,
		tools.NewLinodeFirewallRulesUpdateTool,
		tools.NewLinodeFirewallRuleVersionsListTool,
		tools.NewLinodeFirewallRuleVersionGetTool,
		tools.NewLinodeFirewallDevicesListTool,
		tools.NewLinodeFirewallDeviceGetTool,
		tools.NewLinodeFirewallDeviceCreateTool,
		tools.NewLinodeFirewallDeviceDeleteTool,
		tools.NewLinodeFirewallSettingsListTool,
		tools.NewLinodeFirewallTemplatesListTool,
		tools.NewLinodeFirewallTemplateGetTool,
		tools.NewLinodeFirewallSettingsUpdateTool,
		tools.NewLinodeNetworkTransferPricesTool,
		tools.NewLinodeNetworkingIPListTool,
		tools.NewLinodeNetworkingIPGetTool,
		tools.NewLinodeNetworkingIPUpdateRDNSTool,
		tools.NewLinodeNetworkingIPAllocateTool,
		tools.NewLinodeNetworkingIPAssignTool,
		tools.NewLinodeNetworkingIPv4AssignTool,
		tools.NewLinodeNetworkingIPShareTool,
		tools.NewLinodeIPv6PoolsListTool,
		tools.NewLinodeIPv6RangesListTool,
		tools.NewLinodeIPv6RangeGetTool,
		tools.NewLinodeIPv6RangeCreateTool,
		tools.NewLinodeIPv6RangeDeleteTool,
		tools.NewLinodeNodeBalancerTypesTool,
		tools.NewLinodeNodeBalancerListTool,
		tools.NewLinodeNodeBalancerGetTool,
		tools.NewLinodeNodeBalancerStatsGetTool,
		tools.NewLinodeNodeBalancerVPCConfigGetTool,
		tools.NewLinodeNodeBalancerFirewallListTool,
		tools.NewLinodeNodeBalancerFirewallUpdateTool,
		tools.NewLinodeNodeBalancerVPCListTool,
		tools.NewLinodeNodeBalancerConfigListTool,
		tools.NewLinodeNodeBalancerConfigNodesListTool,
		tools.NewLinodeNodeBalancerConfigGetTool,

		tools.NewLinodeNodeBalancerConfigNodeGetTool,
		tools.NewLinodeNodeBalancerConfigCreateTool,
		tools.NewLinodeNodeBalancerNodeCreateTool,
		tools.NewLinodeNodeBalancerNodeDeleteTool,
		tools.NewLinodeNodeBalancerConfigUpdateTool,
		tools.NewLinodeNodeBalancerConfigRebuildTool,
		tools.NewLinodeNodeBalancerConfigDeleteTool,
		tools.NewLinodeNodeBalancerNodeUpdateTool,
		tools.NewLinodeFirewallCreateTool,
		tools.NewLinodeFirewallUpdateTool,
		tools.NewLinodeFirewallDeleteTool,
		tools.NewLinodeNodeBalancerCreateTool,
		tools.NewLinodeNodeBalancerUpdateTool,
		tools.NewLinodeNodeBalancerDeleteTool,
	})
}

func dnsToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeDomainListTool,
		tools.NewLinodeDomainGetTool,
		tools.NewLinodeDomainZoneFileGetTool,
		tools.NewLinodeDomainRecordListTool,
		tools.NewLinodeDomainRecordGetTool,
		tools.NewLinodeDomainImportTool,
		tools.NewLinodeDomainCreateTool,
		tools.NewLinodeDomainCloneTool,
		tools.NewLinodeDomainUpdateTool,
		tools.NewLinodeDomainDeleteTool,
		tools.NewLinodeDomainRecordCreateTool,
		tools.NewLinodeDomainRecordUpdateTool,
		tools.NewLinodeDomainRecordDeleteTool,
	})
}

func volumeToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeVolumeListTool,
		tools.NewLinodeVolumeCreateTool,
		tools.NewLinodeVolumeUpdateTool,
		tools.NewLinodeVolumeAttachTool,
		tools.NewLinodeVolumeDetachTool,
		tools.NewLinodeVolumeResizeTool,
		tools.NewLinodeVolumeDeleteTool,
	})
}

func objectStorageToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeObjectStorageBucketListTool,
		tools.NewLinodeObjectStorageBucketListByRegionTool,
		tools.NewLinodeObjectStorageBucketGetTool,
		tools.NewLinodeObjectStorageBucketContentsTool,
		tools.NewLinodeObjectStorageClusterListTool,
		tools.NewLinodeObjectStorageEndpointListTool,
		tools.NewLinodeObjectStorageTypeListTool,
		tools.NewLinodeObjectStorageQuotasListTool,
		tools.NewLinodeObjectStorageKeyListTool,
		tools.NewLinodeObjectStorageKeyGetTool,
		tools.NewLinodeObjectStorageTransferTool,
		tools.NewLinodeObjectStorageQuotaGetTool,
		tools.NewLinodeObjectStorageQuotaUsageTool,
		tools.NewLinodeObjectStorageCancelTool,
		tools.NewLinodeObjectStorageBucketAccessGetTool,
		tools.NewLinodeObjectStorageBucketCreateTool,
		tools.NewLinodeObjectStorageBucketDeleteTool,
		tools.NewLinodeObjectStorageBucketAccessAllowTool,
		tools.NewLinodeObjectStorageBucketAccessUpdateTool,
		tools.NewLinodeObjectStorageKeyCreateTool,
		tools.NewLinodeObjectStorageKeyUpdateTool,
		tools.NewLinodeObjectStorageKeyDeleteTool,
		tools.NewLinodeObjectStoragePresignedURLTool,
		tools.NewLinodeObjectStorageObjectACLGetTool,
		tools.NewLinodeObjectStorageObjectACLUpdateTool,
		tools.NewLinodeObjectStorageSSLGetTool,
		tools.NewLinodeObjectStorageSSLDeleteTool,
		tools.NewLinodeObjectStorageSSLUploadTool,
	})
}

func databaseToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeDatabaseEngineListTool,
		tools.NewLinodeDatabaseTypeListTool,
		tools.NewLinodeDatabaseTypeGetTool,
		tools.NewLinodeDatabaseEngineGetTool,
		tools.NewLinodeDatabaseMySQLConfigGetTool,
		tools.NewLinodeDatabasePostgreSQLConfigGetTool,
		tools.NewLinodeDatabaseInstanceListTool,
		tools.NewLinodeDatabasePostgreSQLInstanceListTool,
		tools.NewLinodeDatabaseInstanceGetTool,
		tools.NewLinodeDatabasePostgreSQLInstanceGetTool,
		tools.NewLinodeDatabaseInstanceSSLGetTool,
		tools.NewLinodeDatabasePostgreSQLInstanceSSLGetTool,
		tools.NewLinodeDatabaseInstanceCredentialsGetTool,
		tools.NewLinodeDatabasePostgreSQLInstanceCredentialsGetTool,
		tools.NewLinodeDatabaseInstanceCredentialsResetTool,
		tools.NewLinodeDatabasePostgreSQLInstanceCredentialsResetTool,
		tools.NewLinodeDatabaseInstanceCreateTool,
		tools.NewLinodeDatabasePostgreSQLInstanceCreateTool,
		tools.NewLinodeDatabaseInstanceUpdateTool,
		tools.NewLinodeDatabasePostgreSQLInstanceUpdateTool,
		tools.NewLinodeDatabaseInstanceDeleteTool,
		tools.NewLinodeDatabasePostgreSQLInstanceDeleteTool,
		tools.NewLinodeDatabaseInstancePatchTool,
		tools.NewLinodeDatabasePostgreSQLInstancePatchTool,
		tools.NewLinodeDatabaseInstanceSuspendTool,
		tools.NewLinodeDatabasePostgreSQLInstanceSuspendTool,
		tools.NewLinodeDatabaseInstanceResumeTool,
		tools.NewLinodeDatabasePostgreSQLInstanceResumeTool,
	})
}

func vpcToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		// Read tools
		tools.NewLinodeVPCListTool,
		tools.NewLinodeVPCGetTool,
		tools.NewLinodeVPCIPsListTool,
		tools.NewLinodeVPCIPListTool,
		tools.NewLinodeVPCSubnetListTool,
		tools.NewLinodeVPCSubnetGetTool,
		// Write tools
		tools.NewLinodeVPCCreateTool,
		tools.NewLinodeVPCUpdateTool,
		tools.NewLinodeVPCDeleteTool,
		tools.NewLinodeVPCSubnetCreateTool,
		tools.NewLinodeVPCSubnetUpdateTool,
		tools.NewLinodeVPCSubnetDeleteTool,
	})
}

func instanceDeepToolEntries(cfg *config.Config) []toolEntry {
	backupFactories := instanceBackupToolFactories()
	factories := make([]toolFactory, 0, 1+len(backupFactories))
	factories = append(
		factories,
		tools.NewLinodeInstanceStatsGetTool,
		tools.NewLinodeInstanceTransferMonthGetTool,
	)
	factories = append(factories, backupFactories...)
	factories = append(factories, instanceFirewallToolFactories()...)
	factories = append(factories, instanceInterfaceToolFactories()...)
	factories = append(factories, instanceConfigToolFactories()...)
	factories = append(factories, instanceNodeBalancerToolFactories()...)
	factories = append(factories, instanceDiskToolFactories()...)
	factories = append(factories, instanceIPToolFactories()...)
	factories = append(factories, instanceActionToolFactories()...)

	return entriesFromFactories(cfg, factories)
}

func instanceBackupToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceBackupListTool,
		tools.NewLinodeInstanceBackupGetTool,
		tools.NewLinodeInstanceBackupCreateTool,
		tools.NewLinodeInstanceBackupRestoreTool,
		tools.NewLinodeInstanceBackupsEnableTool,
		tools.NewLinodeInstanceBackupsCancelTool,
	}
}

func instanceFirewallToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceFirewallsUpdateTool,
		tools.NewLinodeInstanceFirewallsApplyTool,
		tools.NewLinodeInstanceFirewallListTool,
	}
}

func instanceInterfaceToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInterfacesUpgradeTool,
		tools.NewLinodeInstanceInterfacesListTool,
		tools.NewLinodeInstanceInterfaceGetTool,
		tools.NewLinodeInstanceInterfaceFirewallsListTool,
		tools.NewLinodeInstanceInterfaceDeleteTool,
		tools.NewLinodeInstanceInterfaceSettingsGetTool,
		tools.NewLinodeInstanceInterfaceSettingsUpdateTool,
		tools.NewLinodeInstanceInterfaceHistoryListTool,
		tools.NewLinodeInstanceInterfaceAddTool,
		tools.NewLinodeInstanceInterfaceUpdateTool,
	}
}

func instanceConfigToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceConfigListTool,
		tools.NewLinodeInstanceVolumeListTool,
		tools.NewLinodeInstanceConfigGetTool,
		tools.NewLinodeInstanceConfigInterfacesListTool,
		tools.NewLinodeInstanceConfigCreateTool,
		tools.NewLinodeInstanceConfigInterfaceAddTool,
		tools.NewLinodeInstanceConfigInterfaceGetTool,
		tools.NewLinodeInstanceConfigInterfaceUpdateTool,
		tools.NewLinodeInstanceConfigInterfaceDeleteTool,
		tools.NewLinodeInstanceConfigUpdateTool,
		tools.NewLinodeInstanceConfigInterfacesReorderTool,
		tools.NewLinodeInstanceConfigDeleteTool,
	}
}

func instanceNodeBalancerToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceNodeBalancerListTool,
	}
}

func instanceDiskToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceDiskListTool,
		tools.NewLinodeInstanceDiskGetTool,
		tools.NewLinodeInstanceDiskCreateTool,
		tools.NewLinodeInstanceDiskUpdateTool,
		tools.NewLinodeInstanceDiskDeleteTool,
		tools.NewLinodeInstanceDiskCloneTool,
		tools.NewLinodeInstanceDiskResizeTool,
		tools.NewLinodeInstanceDiskPasswordResetTool,
	}
}

func instanceIPToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceIPListTool,
		tools.NewLinodeInstanceIPGetTool,
		tools.NewLinodeInstanceIPAllocateTool,
		tools.NewLinodeInstanceIPUpdateRDNSTool,
		tools.NewLinodeInstanceIPDeleteTool,
	}
}

func instanceActionToolFactories() []toolFactory {
	return []toolFactory{
		tools.NewLinodeInstanceCloneTool,
		tools.NewLinodeInstanceMigrateTool,
		tools.NewLinodeInstanceMutateTool,
		tools.NewLinodeInstanceRebuildTool,
		tools.NewLinodeInstanceRescueTool,
		tools.NewLinodeInstancePasswordResetTool,
	}
}

func lkeToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		// Read tools
		tools.NewLinodeLKEClusterListTool,
		tools.NewLinodeLKEClusterGetTool,
		tools.NewLinodeLKEPoolListTool,
		tools.NewLinodeLKEPoolGetTool,
		tools.NewLinodeLKENodeGetTool,
		tools.NewLinodeLKEKubeconfigGetTool,
		tools.NewLinodeLKEDashboardGetTool,
		tools.NewLinodeLKEAPIEndpointListTool,
		tools.NewLinodeLKEACLGetTool,
		tools.NewLinodeLKEVersionListTool,
		tools.NewLinodeLKEVersionGetTool,
		tools.NewLinodeLKETypeListTool,
		tools.NewLinodeLKETierVersionListTool,
		tools.NewLinodeLKETierVersionGetTool,
		// Write tools
		tools.NewLinodeLKEClusterCreateTool,
		tools.NewLinodeLKEClusterUpdateTool,
		tools.NewLinodeLKEClusterDeleteTool,
		tools.NewLinodeLKEClusterRecycleTool,
		tools.NewLinodeLKEClusterRegenerateTool,
		tools.NewLinodeLKEPoolCreateTool,
		tools.NewLinodeLKEPoolUpdateTool,
		tools.NewLinodeLKEPoolDeleteTool,
		tools.NewLinodeLKEPoolRecycleTool,
		tools.NewLinodeLKENodeDeleteTool,
		tools.NewLinodeLKENodeRecycleTool,
		tools.NewLinodeLKEKubeconfigDeleteTool,
		tools.NewLinodeLKEServiceTokenDeleteTool,
		tools.NewLinodeLKEACLUpdateTool,
		tools.NewLinodeLKEACLDeleteTool,
	})
}
