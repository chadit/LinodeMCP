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
}
