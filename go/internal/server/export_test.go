package server

import "github.com/chadit/LinodeMCP/pkg/contracts"

// Tools returns the server's registered tools for testing.
func (s *Server) Tools() []contracts.Tool {
	return s.tools
}

// HasMCP returns true if the MCP server is initialized.
func (s *Server) HasMCP() bool {
	return s.mcp != nil
}

// HasConfig returns true if the config is stored.
func (s *Server) HasConfig() bool {
	return s.config != nil
}
