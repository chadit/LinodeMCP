// Package server provides the LinodeMCP MCP server implementation.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/profiles/builder"
	"github.com/chadit/LinodeMCP/go/internal/tools"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
	"github.com/chadit/LinodeMCP/go/pkg/contracts"
)

// toolHandler is the callback signature mcp-go invokes for each tool call.
type toolHandler = func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error)

// toolEntry is what the per-category collectors stage in pass 1. Pass 2 calls
// addTool only for entries whose name appears in the resolved profile's
// AllowedTools.
type toolEntry struct {
	tool       mcp.Tool
	handler    toolHandler
	capability profiles.Capability
}

// Server represents a LinodeMCP server.
type Server struct {
	config *config.Config
	mcp    *server.MCPServer

	// registered tracks the tools currently live in mcp-go by name, so
	// ReloadProfile can compute additions and removals without walking the
	// tools slice.
	registered map[string]*toolWrapper

	// draftRegistry holds profile-builder drafts in memory only, one set per
	// server process. The _draft_save tool is the bridge from this registry
	// back into Config.Profiles.
	draftRegistry *builder.Registry

	// auditSink consumes audit events emitted by the per-handler capture
	// middleware. Defaults to NoopSink; tests inject a CapturingSink via
	// SetAuditSink to assert handler-call events.
	auditSink audit.Sink

	// planStore holds outstanding two-stage plans for the life of the
	// process. The capture middleware attaches it to every call's context so
	// a destroy handler can produce a plan and apply it after a drift check.
	// Start launches the TTL janitor.
	planStore *twostage.PlanStore

	// metrics wraps each tool dispatch. Defaults to a no-op so a Server built
	// without observability (tests, the CLI front-end) dispatches unchanged;
	// the serve path injects the real *observability.Observability via
	// SetMetricsRecorder. Skip that call and the recording middleware is
	// built but never reached, so /metrics carries no application series.
	metrics MetricsRecorder

	tools []contracts.Tool

	// allEntries holds every tool the server could register, regardless of
	// the active profile. Built once in New so ReloadProfile can re-add tools
	// filtered out at startup without re-running the collectors.
	allEntries []toolEntry

	activeProfile profiles.Profile

	inflight sync.WaitGroup

	// profileMu guards activeProfile, tools, and registered. mcp-go's
	// internal mutex protects its own tool map, but the Server's view of
	// which tools are live needs its own gate.
	profileMu sync.RWMutex

	// shutdownMu serializes positive inflight.Add calls with Shutdown's
	// inflight.Wait call. sync.WaitGroup requires Add to happen before Wait
	// when the counter is zero, so tool dispatch must pass this gate first.
	shutdownMu sync.Mutex

	// auditRedactPII selects which redaction tier the capture middleware
	// uses. False applies the always-on credential list only; true also
	// applies the PII list. Default false so tests that build a Server
	// without going through main keep credential-only behavior; main wires
	// it to cfg.Audit.RedactPII via SetAuditRedactPII at startup.
	auditRedactPII bool

	shuttingDown bool
}

// New creates a new LinodeMCP server. A profile that cannot be resolved
// (unknown name, disabled built-in) errors here rather than at request time,
// so a misconfigured server fails fast instead of silently registering
// nothing.
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
		planStore:     twostage.NewPlanStore(),
		metrics:       noopMetricsRecorder{},
	}

	srv.allEntries = collectAllToolEntries(cfg)
	srv.allEntries = append(srv.allEntries, builderToolEntries(srv)...)

	if err := srv.registerTools(); err != nil {
		return nil, err
	}

	return srv, nil
}

// ValidateGeneratedSchemas rejects any tool still advertising the permissive
// placeholder toolschemas.Schema returns for a name the proto contract does not
// define. Such a tool would accept any arguments at all, so the server refuses
// to start rather than serve it. The Python twin aborts startup at the same
// point, so neither server serves a tool that validates nothing.
//
// Exported because the failure it guards cannot be produced from a config: the
// tool factories embed their schemas at compile time, so a test has to hand
// this a tool directly to prove the check still bites.
func ValidateGeneratedSchemas(list []mcp.Tool) error {
	for i := range list {
		if toolschemas.IsFallback(list[i].RawInputSchema) {
			return fmt.Errorf("%w: %s", ErrSchemaNotGenerated, list[i].Name)
		}
	}

	return nil
}

// builderToolEntries assembles the profile-builder tool entries. Built outside
// collectAllToolEntries because the handlers need a closure over the server
// itself, and tagged CapMeta so the active profile filter always passes them
// through. ToolCatalog reports the builder tools themselves, which is
// deliberate: a user composing a new profile wants to see what they inherit.
func builderToolEntries(srv *Server) []toolEntry {
	listTool, listCap, listHandler := tools.NewLinodeProfileListToolsTool(srv.ToolCatalog)
	catTool, catCap, catHandler := tools.NewLinodeProfileListCategoriesTool(srv.ToolCatalog)
	newTool, newCap, newHandler := tools.NewLinodeProfileDraftNewTool(srv.draftRegistry, srv.LookupProfile)
	showTool, showCap, showHandler := tools.NewLinodeProfileDraftShowTool(srv.draftRegistry)
	discardTool, discardCap, discardHandler := tools.NewLinodeProfileDraftDiscardTool(srv.draftRegistry)
	addToolsTool, addToolsCap, addToolsHandler := tools.NewLinodeProfileDraftAddToolsTool(srv.draftRegistry, srv.ToolCatalog)
	removeToolsTool, removeToolsCap, removeToolsHandler := tools.NewLinodeProfileDraftRemoveToolsTool(srv.draftRegistry)
	setTool, setCap, setHandler := tools.NewLinodeProfileDraftSetTool(srv.draftRegistry)
	saveTool, saveCap, saveHandler := tools.NewLinodeProfileDraftSaveTool(srv.draftRegistry, config.Path)
	canRunTool, canRunCap, canRunHandler := tools.NewLinodeProfileCanRunTool(srv.ToolCatalog, srv.ActiveProfile)

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
		{tool: canRunTool, capability: canRunCap, handler: canRunHandler},
	}
}

type toolWrapper struct {
	tool       mcp.Tool
	capability profiles.Capability
}

func (tw *toolWrapper) Name() string        { return tw.tool.Name }
func (tw *toolWrapper) Description() string { return tw.tool.Description }
func (tw *toolWrapper) InputSchema() any    { return tw.tool.InputSchema }

// Capability returns the tool's capability tag. Server-internal: the public
// pkg/contracts.Tool interface deliberately does not expose this.
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

// Tools returns the registered tool list as a snapshot copy, so callers can
// iterate safely while ReloadProfile mutates the live set.
func (s *Server) Tools() []contracts.Tool {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	out := make([]contracts.Tool, len(s.tools))
	copy(out, s.tools)

	return out
}

// ActiveProfile returns the profile the server is currently running under, by
// value so callers cannot mutate the server's internal state.
func (s *Server) ActiveProfile() profiles.Profile {
	s.profileMu.RLock()
	defer s.profileMu.RUnlock()

	return s.activeProfile
}

// LookupProfile resolves a profile by name across both built-in and
// user-defined entries, returning the materialized Profile and true on hit.
// Ignores the Disabled flag so _draft_new can clone from disabled built-ins
// like full-access and emergency. User-defined entries shadow built-ins by
// name, matching ResolveActiveProfile's precedence.
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
// registry. The pointer is stable for the server's lifetime; tests get a fresh
// registry by constructing their own Server.
func (s *Server) DraftRegistry() *builder.Registry {
	return s.draftRegistry
}

// ToolCatalog returns a snapshot of every tool the server could register,
// regardless of the active profile's filter. Profile filtering controls which
// subset reaches handlers, but the catalog stays the full menu so the builder
// tools can compose a new profile against anything available. Order matches
// collectAllToolEntries: category by category, factory by factory.
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

// ValidateScopes runs token-scope validation against the active profile,
// building a Linode client from the config's default environment and
// delegating to profiles.ValidateScopes for PAT-vs-OAuth dispatch.
//
// Returns profiles.ErrTokenNotConfigured, with no API call made, when that
// environment has no token set; the caller decides whether to fail load
// (elevated profile) or warn and continue (read-only). Other errors come from
// the underlying /profile and /profile/grants calls.
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

// parseRequiredScopes converts the profile's stored []string scopes into the
// typed slice ValidateScopes expects. Stored as strings so user-defined
// profiles can declare scopes the catalog doesn't yet name.
func parseRequiredScopes(stored []string) []profiles.Scope {
	out := make([]profiles.Scope, len(stored))
	for i, s := range stored {
		out[i] = profiles.Scope(s)
	}

	return out
}

// ToolInfo describes a registered tool's capability and input schema, so tests
// in package server_test can inspect the capability tag without widening the
// public contracts.Tool interface.
type ToolInfo struct {
	InputSchema    mcp.ToolInputSchema
	Name           string
	RawInputSchema json.RawMessage
	Capability     profiles.Capability
}

// ToolInfos returns one entry per registered tool. Test-only accessor; the
// audit middleware reads capability via its own server-internal path.
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
			Name:           wrapper.tool.Name,
			Capability:     wrapper.capability,
			InputSchema:    wrapper.tool.InputSchema,
			RawInputSchema: wrapper.tool.RawInputSchema,
		})
	}

	return out
}

// AllToolInfos returns one entry per tool the server could register, where
// ToolInfos sees only what the active profile exposes. The audit
// redaction-coverage invariant needs this: a sensitive arg must be redacted
// whenever ANY profile can expose the tool, not just the active one.
func (s *Server) AllToolInfos() []ToolInfo {
	out := make([]ToolInfo, 0, len(s.allEntries))

	for i := range s.allEntries {
		out = append(out, ToolInfo{
			Name:           s.allEntries[i].tool.Name,
			Capability:     s.allEntries[i].capability,
			InputSchema:    s.allEntries[i].tool.InputSchema,
			RawInputSchema: s.allEntries[i].tool.RawInputSchema,
		})
	}

	return out
}

// HandleMessage dispatches a JSON-RPC message into the underlying mcp-go
// server, exposing the in-process transport for tests and embedders that skip
// stdio. Handlers invoked this way are still tracked in the inflight
// WaitGroup, so Shutdown drains them correctly.
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

	s.planStore.StartJanitor(ctx, time.Minute)

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

// ReloadProfile swaps the running server to the profile resolved from cfg,
// adding tools the new profile allows and removing the ones it excludes.
// mcp-go fires notifications/tools/list_changed on both paths so connected
// clients refresh their tool cache.
//
// A failed reload is a no-op, not a partial update: the server keeps its
// current profile and tool set.
//
// Holds s.profileMu in write mode throughout. In-flight handlers that already
// passed the dispatch gate continue to run; the reload only changes what the
// gate accepts on future calls.
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

// SetAuditSink swaps the audit sink. Passing nil restores the NoopSink
// default rather than producing a nil-deref crash.
func (s *Server) SetAuditSink(sink audit.Sink) {
	if sink == nil {
		sink = audit.NoopSink{}
	}

	s.auditSink = sink
}

// SetAuditRedactPII selects the redaction tier the capture middleware applies
// to event args. Main wires this to cfg.Audit.RedactPII at startup.
func (s *Server) SetAuditRedactPII(redactPII bool) {
	s.auditRedactPII = redactPII
}

// MetricsRecorder records metrics for tool dispatch and the Linode API calls
// a tool makes. *observability.Observability satisfies it. The Server depends
// on this narrow interface rather than the concrete type so the package stays
// decoupled from observability and tests can inject a fake recorder. Both
// methods return nothing, mirroring the audit sink's Write: recording is a
// side effect that must never alter the dispatch result or error.
//
// RecordAPIRequest is here, rather than only on the linode client, because
// the dispatch chokepoint is the one place that holds the recorder; it
// injects the recorder into the request context (linode.WithAPIRecorder) so
// the client can report each API round trip without an observability import.
type MetricsRecorder interface {
	RecordToolCall(ctx context.Context, toolName string, duration time.Duration, err error)
	RecordAPIRequest(ctx context.Context, endpoint, method string, status int, duration float64)
}

// noopMetricsRecorder records nothing. It is the default so a Server built
// without observability dispatches unchanged.
type noopMetricsRecorder struct{}

func (noopMetricsRecorder) RecordToolCall(context.Context, string, time.Duration, error) {}

func (noopMetricsRecorder) RecordAPIRequest(context.Context, string, string, int, float64) {}

// SetMetricsRecorder wires the recorder used to wrap tool dispatch. The
// serve path passes the real *observability.Observability so every call
// records request totals, durations, and errors; the recording middleware
// is otherwise built but never reached. Passing nil restores the no-op
// default rather than producing a nil-deref crash.
func (s *Server) SetMetricsRecorder(recorder MetricsRecorder) {
	if recorder == nil {
		recorder = noopMetricsRecorder{}
	}

	s.metrics = recorder
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

		mode, yoloAllowed := s.executionMode(&req)
		evt.SetMode(mode, "")

		if yoloAllowed {
			ctx = tools.WithYoloAllowed(ctx)
		}

		ctx = tools.WithPlanStore(ctx, s.planStore)
		ctx = linode.WithAPIRecorder(ctx, s.metrics)

		result, err := handler(ctx, req)

		s.metrics.RecordToolCall(ctx, toolName, time.Since(start), err)
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

// executionMode derives the audit execution mode from the call's flags and
// reports whether a permitted yolo execution applies. yolo wins only when the
// active profile's allow_yolo is set; otherwise yolo:true is ignored here and
// the normal destroy gate applies. Reads the profile under the read-lock so a
// concurrent hot-reload can't observe a torn value.
func (s *Server) executionMode(req *mcp.CallToolRequest) (audit.Mode, bool) {
	args := req.GetArguments()

	if yolo, _ := args["yolo"].(bool); yolo {
		s.profileMu.RLock()
		allow := s.activeProfile.AllowYolo
		s.profileMu.RUnlock()

		if allow {
			return audit.ModeYolo, true
		}
	}

	mode, _ := args["mode"].(string)
	if mode == twostage.ModeApply {
		return audit.ModeApply, false
	}

	if mode == twostage.ModePlan {
		return audit.ModePlan, false
	}

	if dryRun, _ := args["dry_run"].(bool); dryRun {
		return audit.ModeDryRun, false
	}

	if bypass, _ := args["confirm_bypass_dry_run"].(bool); bypass {
		return audit.ModeBypassDryRun, false
	}

	return audit.ModeNormal, false
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
//
// Finishes by running ValidateGeneratedSchemas over the staged catalog, so a
// tool whose proto message has no generated schema fails startup instead of
// registering something that accepts any arguments.
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

	return ValidateCatalog(s.entryTools())
}

// ValidateCatalog runs the three contract checks a staged catalog has to pass
// before the server starts: every tool advertises a generated schema, the proto
// contract describes one tool per message at one capability tier, and the staged
// set is exactly the set that contract declares.
//
// The last one is what the capability option bought. The contract used to know
// only which tools were routed, so the server had to say which of its own tools
// to expect, and a tool it never staged at all was a gap neither side could see.
// Now the expected set comes from the descriptors, and a handler dropped or
// renamed without the contract moving with it fails startup by name.
//
// Exported for the same reason ValidateGeneratedSchemas is: none of these can be
// produced from a config, since the tool factories embed their schemas at
// compile time and the proto contract ships in the same binary, so proving the
// checks still bite means handing this a bad catalog directly.
func ValidateCatalog(list []mcp.Tool) error {
	if err := ValidateGeneratedSchemas(list); err != nil {
		return err
	}

	staged := make([]string, len(list))
	for i := range list {
		staged[i] = list[i].Name
	}

	// Both contract checks are evaluated before either is reported, so a
	// startup failure states everything wrong with the contract at once rather
	// than one defect per restart.
	contract := errors.Join(linoderoute.Validate(), linoderoute.ValidateRegistered(staged))
	if contract != nil {
		return fmt.Errorf("tool contract: %w", contract)
	}

	return nil
}

// entryTools copies the mcp.Tool out of every staged entry so the schema check
// can run over the exported type a test can also construct.
func (s *Server) entryTools() []mcp.Tool {
	list := make([]mcp.Tool, len(s.allEntries))
	for i := range s.allEntries {
		list[i] = s.allEntries[i].tool
	}

	return list
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
		generatedToolEntries(cfg),
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
		tools.NewLinodeProfilePreferencesTool,
		tools.NewLinodeProfilePreferencesUpdateTool,
		tools.NewLinodeProfileTokenCreateTool,
		tools.NewLinodeProfileTokenDeleteTool,
		tools.NewLinodeProfileSecurityQuestionsTool,
		tools.NewLinodeProfileSecurityQuestionsAnswerTool,
		tools.NewLinodeProfileTokensTool,
		tools.NewLinodeProfileTokenUpdateTool,
		tools.NewLinodeProfileLoginsTool,
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
		tools.NewLinodeTagsTool,
		tools.NewLinodeTagDeleteTool,
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
		tools.NewLinodeMonitorServiceAlertDefinitionCloneTool,
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
		tools.NewLinodeProfileTFAEnableTool,
		tools.NewLinodeProfilePhoneNumberSendTool,
		tools.NewLinodeProfilePhoneNumberDeleteTool,
		tools.NewLinodeProfilePhoneNumberVerifyTool,
		tools.NewLinodeProfileTFADisableTool,
		tools.NewLinodeProfileTFAEnableConfirmTool,
		tools.NewLinodeProfileDevicesTool,
		tools.NewLinodeProfileAppGetTool,
		tools.NewLinodeProfileAppDeleteTool,
		tools.NewLinodeProfileDeviceGetTool,
		tools.NewLinodeProfileDeviceRevokeTool,
		tools.NewLinodeAccountOAuthClientsTool,
		tools.NewLinodeProfileAppsTool,
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
		tools.NewLinodeTaggedObjectsTool,
		tools.NewLinodeSupportTicketGetTool,
		tools.NewLinodeSupportTicketRepliesTool,
		tools.NewLinodeSupportTicketsTool,
		tools.NewLinodeSupportTicketCloseTool,
		tools.NewLinodeAccountUsersTool,
		tools.NewLinodeAccountUserGetTool,
		tools.NewLinodeProfileTokenGetTool,
		tools.NewLinodeAccountUserGrantsTool,
		tools.NewLinodeAccountUserGrantsUpdateTool,
		tools.NewLinodeAccountUserUpdateTool,
		tools.NewLinodeAccountUserDeleteTool,
		tools.NewLinodeAccountUserCreateTool,
		tools.NewLinodeAccountSupportTicketCreateTool,
		tools.NewLinodeAccountSupportTicketAttachmentCreateTool,
		tools.NewLinodeAccountSupportTicketReplyCreateTool,
		tools.NewLinodeManagedContactCreateTool,
		tools.NewLinodeAccountLoginsTool,
		tools.NewLinodeAccountLoginGetTool,
		tools.NewLinodeProfileLoginGetTool,
		tools.NewLinodeAccountInvoicesTool,
		tools.NewLinodeAccountPaymentsTool,
		tools.NewLinodeAccountPaymentGetTool,
		tools.NewLinodeAccountPaymentCreateTool,
		tools.NewLinodeAccountPromoCreditTool,
		tools.NewLinodeAccountInvoiceGetTool,
		tools.NewLinodeAccountInvoiceItemsTool,
		tools.NewLinodeAccountChildAccountsTool,
		tools.NewLinodeAccountServiceTransfersTool,
		tools.NewLinodeAccountServiceTransferGetTool,
		tools.NewLinodeAccountServiceTransferCreateTool,
		tools.NewLinodeAccountServiceTransferDeleteTool,
		tools.NewLinodeAccountServiceTransferAcceptTool,
		tools.NewLinodeAccountEventGetTool,
		tools.NewLinodeAccountEventSeenTool,
		tools.NewLinodeAccountChildAccountGetTool,
		tools.NewLinodeAccountChildAccountTokenTool,
		tools.NewLinodeAccountBetaGetTool,
		tools.NewLinodeAccountBetaEnrollTool,
		tools.NewLinodeAccountAvailabilityTool,
		tools.NewLinodeAccountAvailabilityGetTool,
		tools.NewLinodeTagCreateTool,
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
		tools.NewLinodeRegionGetTool,
		tools.NewLinodeRegionAvailabilityListTool,
		tools.NewLinodeRegionAvailabilityGetTool,
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
		tools.NewLinodeInstanceUpdateTool,
		tools.NewLinodeInstanceDeleteTool,
		tools.NewLinodeInstanceResizeTool,
	})
}

func networkingToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeFirewallListTool,
		tools.NewLinodeFirewallGetTool,
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
		tools.NewLinodeNetworkingIPShareTool,
		tools.NewLinodeNetworkingIPv4AssignTool,
		tools.NewLinodeNetworkingIPv4ShareTool,
		tools.NewLinodeReservedIPListTool,
		tools.NewLinodeReservedIPGetTool,
		tools.NewLinodeReservedIPTypeListTool,
		tools.NewLinodeReservedIPCreateTool,
		tools.NewLinodeReservedIPUpdateTool,
		tools.NewLinodeReservedIPDeleteTool,
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

// generatedToolEntries stages the tools cmd/toolgen emits from the proto
// contract. They are collected apart from the category lists above because
// nothing here is a choice a reader can make: the set comes from
// docs/contracts/generated-tools.txt, and a factory named in a category list
// would be a second place a generated tool could be added or forgotten.
func generatedToolEntries(cfg *config.Config) []toolEntry {
	generated := gentools.Factories()

	factories := make([]toolFactory, 0, len(generated))
	for _, factory := range generated {
		factories = append(factories, toolFactory(factory))
	}

	return entriesFromFactories(cfg, factories)
}

func dnsToolEntries(cfg *config.Config) []toolEntry {
	return entriesFromFactories(cfg, []toolFactory{
		tools.NewLinodeDomainZoneFileGetTool,
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
		tools.NewLinodeVolumeGetTool,
		tools.NewLinodeVolumeTypeListTool,
		tools.NewLinodeVolumeCreateTool,
		tools.NewLinodeVolumeCloneTool,
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
		tools.NewLinodeDatabaseAllInstancesListTool,
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
		tools.NewLinodeVPCListTool,
		tools.NewLinodeVPCGetTool,
		tools.NewLinodeVPCIPsListTool,
		tools.NewLinodeVPCIPListTool,
		tools.NewLinodeVPCSubnetListTool,
		tools.NewLinodeVPCSubnetGetTool,
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
