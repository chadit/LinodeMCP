// Package server provides the LinodeMCP MCP server implementation.
package server

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
	"github.com/chadit/LinodeMCP/internal/version"
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
		version.Version,
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

func (tw *toolWrapper) Execute(_ context.Context, _ map[string]any) (*mcp.CallToolResult, error) {
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

func (s *Server) registerTools() {
	helloTool, helloHandler := tools.NewHelloTool()
	s.mcp.AddTool(helloTool, helloHandler)
	s.tools = append(s.tools, &toolWrapper{tool: helloTool})

	versionTool, versionHandler := tools.NewVersionTool()
	s.mcp.AddTool(versionTool, versionHandler)
	s.tools = append(s.tools, &toolWrapper{tool: versionTool})

	linodeProfileTool, linodeProfileHandler := tools.NewLinodeProfileTool(s.config)
	s.mcp.AddTool(linodeProfileTool, linodeProfileHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeProfileTool})

	linodeAccountTool, linodeAccountHandler := tools.NewLinodeAccountTool(s.config)
	s.mcp.AddTool(linodeAccountTool, linodeAccountHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeAccountTool})

	linodeInstancesTool, linodeInstancesHandler := tools.NewLinodeInstancesTool(s.config)
	s.mcp.AddTool(linodeInstancesTool, linodeInstancesHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstancesTool})

	linodeInstanceGetTool, linodeInstanceGetHandler := tools.NewLinodeInstanceGetTool(s.config)
	s.mcp.AddTool(linodeInstanceGetTool, linodeInstanceGetHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceGetTool})

	linodeRegionsTool, linodeRegionsHandler := tools.NewLinodeRegionsListTool(s.config)
	s.mcp.AddTool(linodeRegionsTool, linodeRegionsHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeRegionsTool})

	linodeTypesTool, linodeTypesHandler := tools.NewLinodeTypesListTool(s.config)
	s.mcp.AddTool(linodeTypesTool, linodeTypesHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeTypesTool})

	linodeVolumesTool, linodeVolumesHandler := tools.NewLinodeVolumesListTool(s.config)
	s.mcp.AddTool(linodeVolumesTool, linodeVolumesHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeVolumesTool})

	linodeImagesTool, linodeImagesHandler := tools.NewLinodeImagesListTool(s.config)
	s.mcp.AddTool(linodeImagesTool, linodeImagesHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeImagesTool})

	// Stage 3: Extended read operations
	linodeSSHKeysTool, linodeSSHKeysHandler := tools.NewLinodeSSHKeysListTool(s.config)
	s.mcp.AddTool(linodeSSHKeysTool, linodeSSHKeysHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeSSHKeysTool})

	linodeDomainsTool, linodeDomainsHandler := tools.NewLinodeDomainsListTool(s.config)
	s.mcp.AddTool(linodeDomainsTool, linodeDomainsHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainsTool})

	linodeDomainGetTool, linodeDomainGetHandler := tools.NewLinodeDomainGetTool(s.config)
	s.mcp.AddTool(linodeDomainGetTool, linodeDomainGetHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainGetTool})

	linodeDomainRecordsTool, linodeDomainRecordsHandler := tools.NewLinodeDomainRecordsListTool(s.config)
	s.mcp.AddTool(linodeDomainRecordsTool, linodeDomainRecordsHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainRecordsTool})

	linodeFirewallsTool, linodeFirewallsHandler := tools.NewLinodeFirewallsListTool(s.config)
	s.mcp.AddTool(linodeFirewallsTool, linodeFirewallsHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeFirewallsTool})

	linodeNodeBalancersTool, linodeNodeBalancersHandler := tools.NewLinodeNodeBalancersListTool(s.config)
	s.mcp.AddTool(linodeNodeBalancersTool, linodeNodeBalancersHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeNodeBalancersTool})

	linodeNodeBalancerGetTool, linodeNodeBalancerGetHandler := tools.NewLinodeNodeBalancerGetTool(s.config)
	s.mcp.AddTool(linodeNodeBalancerGetTool, linodeNodeBalancerGetHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeNodeBalancerGetTool})

	linodeStackScriptsTool, linodeStackScriptsHandler := tools.NewLinodeStackScriptsListTool(s.config)
	s.mcp.AddTool(linodeStackScriptsTool, linodeStackScriptsHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeStackScriptsTool})

	// Stage 4: Write operations - SSH Keys.
	linodeSSHKeyCreateTool, linodeSSHKeyCreateHandler := tools.NewLinodeSSHKeyCreateTool(s.config)
	s.mcp.AddTool(linodeSSHKeyCreateTool, linodeSSHKeyCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeSSHKeyCreateTool})

	linodeSSHKeyDeleteTool, linodeSSHKeyDeleteHandler := tools.NewLinodeSSHKeyDeleteTool(s.config)
	s.mcp.AddTool(linodeSSHKeyDeleteTool, linodeSSHKeyDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeSSHKeyDeleteTool})

	// Stage 4: Write operations - Instances.
	linodeInstanceBootTool, linodeInstanceBootHandler := tools.NewLinodeInstanceBootTool(s.config)
	s.mcp.AddTool(linodeInstanceBootTool, linodeInstanceBootHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceBootTool})

	linodeInstanceRebootTool, linodeInstanceRebootHandler := tools.NewLinodeInstanceRebootTool(s.config)
	s.mcp.AddTool(linodeInstanceRebootTool, linodeInstanceRebootHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceRebootTool})

	linodeInstanceShutdownTool, linodeInstanceShutdownHandler := tools.NewLinodeInstanceShutdownTool(s.config)
	s.mcp.AddTool(linodeInstanceShutdownTool, linodeInstanceShutdownHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceShutdownTool})

	linodeInstanceCreateTool, linodeInstanceCreateHandler := tools.NewLinodeInstanceCreateTool(s.config)
	s.mcp.AddTool(linodeInstanceCreateTool, linodeInstanceCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceCreateTool})

	linodeInstanceDeleteTool, linodeInstanceDeleteHandler := tools.NewLinodeInstanceDeleteTool(s.config)
	s.mcp.AddTool(linodeInstanceDeleteTool, linodeInstanceDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceDeleteTool})

	linodeInstanceResizeTool, linodeInstanceResizeHandler := tools.NewLinodeInstanceResizeTool(s.config)
	s.mcp.AddTool(linodeInstanceResizeTool, linodeInstanceResizeHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeInstanceResizeTool})

	// Stage 4: Write operations - Firewalls.
	linodeFirewallCreateTool, linodeFirewallCreateHandler := tools.NewLinodeFirewallCreateTool(s.config)
	s.mcp.AddTool(linodeFirewallCreateTool, linodeFirewallCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeFirewallCreateTool})

	linodeFirewallUpdateTool, linodeFirewallUpdateHandler := tools.NewLinodeFirewallUpdateTool(s.config)
	s.mcp.AddTool(linodeFirewallUpdateTool, linodeFirewallUpdateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeFirewallUpdateTool})

	linodeFirewallDeleteTool, linodeFirewallDeleteHandler := tools.NewLinodeFirewallDeleteTool(s.config)
	s.mcp.AddTool(linodeFirewallDeleteTool, linodeFirewallDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeFirewallDeleteTool})

	// Stage 4: Write operations - Domains.
	linodeDomainCreateTool, linodeDomainCreateHandler := tools.NewLinodeDomainCreateTool(s.config)
	s.mcp.AddTool(linodeDomainCreateTool, linodeDomainCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainCreateTool})

	linodeDomainUpdateTool, linodeDomainUpdateHandler := tools.NewLinodeDomainUpdateTool(s.config)
	s.mcp.AddTool(linodeDomainUpdateTool, linodeDomainUpdateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainUpdateTool})

	linodeDomainDeleteTool, linodeDomainDeleteHandler := tools.NewLinodeDomainDeleteTool(s.config)
	s.mcp.AddTool(linodeDomainDeleteTool, linodeDomainDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainDeleteTool})

	// Stage 4: Write operations - Domain Records.
	linodeDomainRecordCreateTool, linodeDomainRecordCreateHandler := tools.NewLinodeDomainRecordCreateTool(s.config)
	s.mcp.AddTool(linodeDomainRecordCreateTool, linodeDomainRecordCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainRecordCreateTool})

	linodeDomainRecordUpdateTool, linodeDomainRecordUpdateHandler := tools.NewLinodeDomainRecordUpdateTool(s.config)
	s.mcp.AddTool(linodeDomainRecordUpdateTool, linodeDomainRecordUpdateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainRecordUpdateTool})

	linodeDomainRecordDeleteTool, linodeDomainRecordDeleteHandler := tools.NewLinodeDomainRecordDeleteTool(s.config)
	s.mcp.AddTool(linodeDomainRecordDeleteTool, linodeDomainRecordDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeDomainRecordDeleteTool})

	// Stage 4: Write operations - Volumes.
	linodeVolumeCreateTool, linodeVolumeCreateHandler := tools.NewLinodeVolumeCreateTool(s.config)
	s.mcp.AddTool(linodeVolumeCreateTool, linodeVolumeCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeVolumeCreateTool})

	linodeVolumeAttachTool, linodeVolumeAttachHandler := tools.NewLinodeVolumeAttachTool(s.config)
	s.mcp.AddTool(linodeVolumeAttachTool, linodeVolumeAttachHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeVolumeAttachTool})

	linodeVolumeDetachTool, linodeVolumeDetachHandler := tools.NewLinodeVolumeDetachTool(s.config)
	s.mcp.AddTool(linodeVolumeDetachTool, linodeVolumeDetachHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeVolumeDetachTool})

	linodeVolumeResizeTool, linodeVolumeResizeHandler := tools.NewLinodeVolumeResizeTool(s.config)
	s.mcp.AddTool(linodeVolumeResizeTool, linodeVolumeResizeHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeVolumeResizeTool})

	linodeVolumeDeleteTool, linodeVolumeDeleteHandler := tools.NewLinodeVolumeDeleteTool(s.config)
	s.mcp.AddTool(linodeVolumeDeleteTool, linodeVolumeDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeVolumeDeleteTool})

	// Stage 4: Write operations - NodeBalancers.
	linodeNodeBalancerCreateTool, linodeNodeBalancerCreateHandler := tools.NewLinodeNodeBalancerCreateTool(s.config)
	s.mcp.AddTool(linodeNodeBalancerCreateTool, linodeNodeBalancerCreateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeNodeBalancerCreateTool})

	linodeNodeBalancerUpdateTool, linodeNodeBalancerUpdateHandler := tools.NewLinodeNodeBalancerUpdateTool(s.config)
	s.mcp.AddTool(linodeNodeBalancerUpdateTool, linodeNodeBalancerUpdateHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeNodeBalancerUpdateTool})

	linodeNodeBalancerDeleteTool, linodeNodeBalancerDeleteHandler := tools.NewLinodeNodeBalancerDeleteTool(s.config)
	s.mcp.AddTool(linodeNodeBalancerDeleteTool, linodeNodeBalancerDeleteHandler)
	s.tools = append(s.tools, &toolWrapper{tool: linodeNodeBalancerDeleteTool})

	// Stage 5: Object Storage read operations.
	objBucketsListTool, objBucketsListHandler := tools.NewLinodeObjectStorageBucketsListTool(s.config)
	s.mcp.AddTool(objBucketsListTool, objBucketsListHandler)
	s.tools = append(s.tools, &toolWrapper{tool: objBucketsListTool})

	objBucketGetTool, objBucketGetHandler := tools.NewLinodeObjectStorageBucketGetTool(s.config)
	s.mcp.AddTool(objBucketGetTool, objBucketGetHandler)
	s.tools = append(s.tools, &toolWrapper{tool: objBucketGetTool})

	objBucketContentsTool, objBucketContentsHandler := tools.NewLinodeObjectStorageBucketContentsTool(s.config)
	s.mcp.AddTool(objBucketContentsTool, objBucketContentsHandler)
	s.tools = append(s.tools, &toolWrapper{tool: objBucketContentsTool})

	objClustersListTool, objClustersListHandler := tools.NewLinodeObjectStorageClustersListTool(s.config)
	s.mcp.AddTool(objClustersListTool, objClustersListHandler)
	s.tools = append(s.tools, &toolWrapper{tool: objClustersListTool})

	objTypeListTool, objTypeListHandler := tools.NewLinodeObjectStorageTypeListTool(s.config)
	s.mcp.AddTool(objTypeListTool, objTypeListHandler)
	s.tools = append(s.tools, &toolWrapper{tool: objTypeListTool})
}
