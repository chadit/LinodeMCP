// Package server provides the LinodeMCP MCP server implementation.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
	"github.com/chadit/LinodeMCP/pkg/contracts"
)

// Server represents a LinodeMCP server.
type Server struct {
	config *config.Config
	mcp    *server.MCPServer
	tools  []contracts.Tool
}

// ErrConfigNil is returned when a nil config is passed to New.
var ErrConfigNil = errors.New("config cannot be nil")

// ErrExecuteNotImplemented is returned when Execute is called on a toolWrapper.
var ErrExecuteNotImplemented = errors.New("execute method not implemented for wrapper")

// New creates a new LinodeMCP server.
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
		config: cfg,
		mcp:    mcpServer,
		tools:  make([]contracts.Tool, 0),
	}

	srv.registerTools()

	return srv, nil
}

type toolWrapper struct {
	tool mcp.Tool
}

func (tw *toolWrapper) Name() string        { return tw.tool.Name }
func (tw *toolWrapper) Description() string { return tw.tool.Description }
func (tw *toolWrapper) InputSchema() any    { return tw.tool.InputSchema }

func (*toolWrapper) Execute(_ context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
	return nil, ErrExecuteNotImplemented
}

// Start starts the LinodeMCP server using stdio transport.
func (s *Server) Start(_ context.Context) error {
	log.Printf("Starting LinodeMCP server with %d tools", len(s.tools))

	for _, tool := range s.tools {
		log.Printf("Registered tool: %s - %s", tool.Name(), tool.Description())
	}

	log.Printf("LinodeMCP server started")

	if err := server.ServeStdio(s.mcp); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// GetToolCount returns the number of registered tools.
func (s *Server) GetToolCount() int {
	return len(s.tools)
}

type toolFactory func(*config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error))

func (s *Server) registerToolFromFactory(factory toolFactory) {
	tool, handler := factory(s.config)
	s.mcp.AddTool(tool, handler)
	s.tools = append(s.tools, &toolWrapper{tool: tool})
}

func (s *Server) registerTools() {
	s.registerCoreTools()
	s.registerComputeTools()
	s.registerNetworkingTools()
	s.registerDNSTools()
	s.registerVolumeTools()
	s.registerObjectStorageTools()
}

func (s *Server) registerCoreTools() {
	helloTool, helloHandler := tools.NewHelloTool()
	s.mcp.AddTool(helloTool, helloHandler)
	s.tools = append(s.tools, &toolWrapper{tool: helloTool})

	versionTool, versionHandler := tools.NewVersionTool()
	s.mcp.AddTool(versionTool, versionHandler)
	s.tools = append(s.tools, &toolWrapper{tool: versionTool})

	for _, factory := range []toolFactory{
		tools.NewLinodeProfileTool,
		tools.NewLinodeAccountTool,
	} {
		s.registerToolFromFactory(factory)
	}
}

func (s *Server) registerComputeTools() {
	for _, factory := range []toolFactory{
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
	} {
		s.registerToolFromFactory(factory)
	}
}

func (s *Server) registerNetworkingTools() {
	for _, factory := range []toolFactory{
		tools.NewLinodeFirewallsListTool,
		tools.NewLinodeNodeBalancersListTool,
		tools.NewLinodeNodeBalancerGetTool,
		tools.NewLinodeFirewallCreateTool,
		tools.NewLinodeFirewallUpdateTool,
		tools.NewLinodeFirewallDeleteTool,
		tools.NewLinodeNodeBalancerCreateTool,
		tools.NewLinodeNodeBalancerUpdateTool,
		tools.NewLinodeNodeBalancerDeleteTool,
	} {
		s.registerToolFromFactory(factory)
	}
}

func (s *Server) registerDNSTools() {
	for _, factory := range []toolFactory{
		tools.NewLinodeDomainsListTool,
		tools.NewLinodeDomainGetTool,
		tools.NewLinodeDomainRecordsListTool,
		tools.NewLinodeDomainCreateTool,
		tools.NewLinodeDomainUpdateTool,
		tools.NewLinodeDomainDeleteTool,
		tools.NewLinodeDomainRecordCreateTool,
		tools.NewLinodeDomainRecordUpdateTool,
		tools.NewLinodeDomainRecordDeleteTool,
	} {
		s.registerToolFromFactory(factory)
	}
}

func (s *Server) registerVolumeTools() {
	for _, factory := range []toolFactory{
		tools.NewLinodeVolumesListTool,
		tools.NewLinodeVolumeCreateTool,
		tools.NewLinodeVolumeAttachTool,
		tools.NewLinodeVolumeDetachTool,
		tools.NewLinodeVolumeResizeTool,
		tools.NewLinodeVolumeDeleteTool,
	} {
		s.registerToolFromFactory(factory)
	}
}

func (s *Server) registerObjectStorageTools() {
	for _, factory := range []toolFactory{
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
	} {
		s.registerToolFromFactory(factory)
	}
}
