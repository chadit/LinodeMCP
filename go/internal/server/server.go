// Package server provides the LinodeMCP MCP server implementation.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
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
	inflight      sync.WaitGroup

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
		config:     cfg,
		mcp:        mcpServer,
		tools:      make([]contracts.Tool, 0),
		registered: make(map[string]*toolWrapper),
	}

	srv.allEntries = srv.collectAllToolEntries()

	if err := srv.registerTools(); err != nil {
		return nil, err
	}

	return srv, nil
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

// addTool registers a tool with mcp-go and the local list, wrapping the
// handler so each in-flight invocation is tracked in s.inflight. Shutdown
// uses that WaitGroup to drain handlers before returning. Takes the tool by
// pointer to satisfy gocritic's hugeParam check; mcp.AddTool needs a value
// so we deref at the call boundary. Capability is stashed on the wrapper so
// invariant tests and the audit middleware can read it without a side table.
//
// Caller MUST hold s.profileMu in write mode. addTool mutates s.tools and
// s.registered, both of which are guarded by that mutex.
func (s *Server) addTool(tool *mcp.Tool, capability profiles.Capability, handler toolHandler) {
	wrapped := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.inflight.Add(1)
		defer s.inflight.Done()

		return handler(ctx, req)
	}

	s.mcp.AddTool(*tool, wrapped)

	wrapper := &toolWrapper{tool: *tool, capability: capability}
	s.tools = append(s.tools, wrapper)
	s.registered[tool.Name] = wrapper
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

// collectAllToolEntries returns the flat list of every tool the server
// could register, ignoring profile filtering. Pass 1 of registerTools uses
// this to build the descriptor list for profile resolution; pass 2 then
// filters and calls addTool.
func (s *Server) collectAllToolEntries() []toolEntry {
	categoryEntries := [][]toolEntry{
		s.coreToolEntries(),
		s.computeToolEntries(),
		s.networkingToolEntries(),
		s.dnsToolEntries(),
		s.volumeToolEntries(),
		s.objectStorageToolEntries(),
		s.lkeToolEntries(),
		s.vpcToolEntries(),
		s.instanceDeepToolEntries(),
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

func (s *Server) coreToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		tools.NewHelloTool,
		tools.NewVersionTool,
		tools.NewLinodeProfileTool,
		tools.NewLinodeAccountTool,
	})
}

func (s *Server) computeToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		tools.NewLinodeInstancesTool,
		tools.NewLinodeInstanceGetTool,
		tools.NewLinodeRegionsListTool,
		tools.NewLinodeTypesListTool,
		tools.NewLinodeImagesListTool,
		tools.NewLinodeSSHKeysListTool,
		tools.NewLinodeStackScriptsListTool,
		tools.NewLinodeSSHKeyCreateTool,
		tools.NewLinodeSSHKeyDeleteTool,
		tools.NewLinodeInstanceBootTool,
		tools.NewLinodeInstanceRebootTool,
		tools.NewLinodeInstanceShutdownTool,
		tools.NewLinodeInstanceCreateTool,
		tools.NewLinodeInstanceDeleteTool,
		tools.NewLinodeInstanceResizeTool,
	})
}

func (s *Server) networkingToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		tools.NewLinodeFirewallsListTool,
		tools.NewLinodeNodeBalancersListTool,
		tools.NewLinodeNodeBalancerGetTool,
		tools.NewLinodeFirewallCreateTool,
		tools.NewLinodeFirewallUpdateTool,
		tools.NewLinodeFirewallDeleteTool,
		tools.NewLinodeNodeBalancerCreateTool,
		tools.NewLinodeNodeBalancerUpdateTool,
		tools.NewLinodeNodeBalancerDeleteTool,
	})
}

func (s *Server) dnsToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		tools.NewLinodeDomainsListTool,
		tools.NewLinodeDomainGetTool,
		tools.NewLinodeDomainRecordsListTool,
		tools.NewLinodeDomainCreateTool,
		tools.NewLinodeDomainUpdateTool,
		tools.NewLinodeDomainDeleteTool,
		tools.NewLinodeDomainRecordCreateTool,
		tools.NewLinodeDomainRecordUpdateTool,
		tools.NewLinodeDomainRecordDeleteTool,
	})
}

func (s *Server) volumeToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		tools.NewLinodeVolumesListTool,
		tools.NewLinodeVolumeCreateTool,
		tools.NewLinodeVolumeAttachTool,
		tools.NewLinodeVolumeDetachTool,
		tools.NewLinodeVolumeResizeTool,
		tools.NewLinodeVolumeDeleteTool,
	})
}

func (s *Server) objectStorageToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		tools.NewLinodeObjectStorageBucketsListTool,
		tools.NewLinodeObjectStorageBucketGetTool,
		tools.NewLinodeObjectStorageBucketContentsTool,
		tools.NewLinodeObjectStorageClustersListTool,
		tools.NewLinodeObjectStorageTypeListTool,
		tools.NewLinodeObjectStorageKeysListTool,
		tools.NewLinodeObjectStorageKeyGetTool,
		tools.NewLinodeObjectStorageTransferTool,
		tools.NewLinodeObjectStorageBucketAccessGetTool,
		tools.NewLinodeObjectStorageBucketCreateTool,
		tools.NewLinodeObjectStorageBucketDeleteTool,
		tools.NewLinodeObjectStorageBucketAccessUpdateTool,
		tools.NewLinodeObjectStorageKeyCreateTool,
		tools.NewLinodeObjectStorageKeyUpdateTool,
		tools.NewLinodeObjectStorageKeyDeleteTool,
		tools.NewLinodeObjectStoragePresignedURLTool,
		tools.NewLinodeObjectStorageObjectACLGetTool,
		tools.NewLinodeObjectStorageObjectACLUpdateTool,
		tools.NewLinodeObjectStorageSSLGetTool,
		tools.NewLinodeObjectStorageSSLDeleteTool,
	})
}

func (s *Server) vpcToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		// Read tools
		tools.NewLinodeVPCsListTool,
		tools.NewLinodeVPCGetTool,
		tools.NewLinodeVPCIPsListTool,
		tools.NewLinodeVPCIPListTool,
		tools.NewLinodeVPCSubnetsListTool,
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

func (s *Server) instanceDeepToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		// Backups
		tools.NewLinodeInstanceBackupsListTool,
		tools.NewLinodeInstanceBackupGetTool,
		tools.NewLinodeInstanceBackupCreateTool,
		tools.NewLinodeInstanceBackupRestoreTool,
		tools.NewLinodeInstanceBackupsEnableTool,
		tools.NewLinodeInstanceBackupsCancelTool,
		// Disks
		tools.NewLinodeInstanceDisksListTool,
		tools.NewLinodeInstanceDiskGetTool,
		tools.NewLinodeInstanceDiskCreateTool,
		tools.NewLinodeInstanceDiskUpdateTool,
		tools.NewLinodeInstanceDiskDeleteTool,
		tools.NewLinodeInstanceDiskCloneTool,
		tools.NewLinodeInstanceDiskResizeTool,
		// IPs
		tools.NewLinodeInstanceIPsListTool,
		tools.NewLinodeInstanceIPGetTool,
		tools.NewLinodeInstanceIPAllocateTool,
		tools.NewLinodeInstanceIPDeleteTool,
		// Actions
		tools.NewLinodeInstanceCloneTool,
		tools.NewLinodeInstanceMigrateTool,
		tools.NewLinodeInstanceRebuildTool,
		tools.NewLinodeInstanceRescueTool,
		tools.NewLinodeInstancePasswordResetTool,
	})
}

func (s *Server) lkeToolEntries() []toolEntry {
	return entriesFromFactories(s.config, []toolFactory{
		// Read tools
		tools.NewLinodeLKEClustersListTool,
		tools.NewLinodeLKEClusterGetTool,
		tools.NewLinodeLKEPoolsListTool,
		tools.NewLinodeLKEPoolGetTool,
		tools.NewLinodeLKENodeGetTool,
		tools.NewLinodeLKEKubeconfigGetTool,
		tools.NewLinodeLKEDashboardGetTool,
		tools.NewLinodeLKEAPIEndpointsListTool,
		tools.NewLinodeLKEACLGetTool,
		tools.NewLinodeLKEVersionsListTool,
		tools.NewLinodeLKEVersionGetTool,
		tools.NewLinodeLKETypesListTool,
		tools.NewLinodeLKETierVersionsListTool,
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
