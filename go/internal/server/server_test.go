package server_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/server"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// TestNew verifies server creation under various conditions including
// nil config, valid config, and tool registration.
//
// Workflow:
//  1. **nil config**: Confirm that a nil config returns ErrConfigNil
//  2. **valid config**: Confirm that a valid config creates a functional server
//  3. **tools registered**: Confirm that tool registration populates the tool list
//
// Expected Behavior:
//   - Nil config returns error and nil server
//   - Valid config returns initialized server with MCP and config
//   - Tool count matches the expected 125 registered tools
//
// Purpose: End-to-end verification of server construction and initialization.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("nil config returns error", func(t *testing.T) {
		t.Parallel()

		srv, err := server.New(nil)
		require.Error(t, err, "New with nil config should return an error")
		assert.Nil(t, srv, "server should be nil when config is nil")
		assert.ErrorIs(t, err, server.ErrConfigNil, "error should be ErrConfigNil")
	})

	t.Run("valid config creates server", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Server: config.ServerConfig{
				Name:      "TestMCP",
				LogLevel:  "info",
				Transport: "stdio",
				Host:      "127.0.0.1",
				Port:      8080,
			},
			Environments: map[string]config.EnvironmentConfig{
				"default": {
					Label: "Default",
					Linode: config.LinodeConfig{
						APIURL: "https://api.linode.com/v4",
						Token:  "test-token",
					},
				},
			},
		}

		srv, err := server.New(cfg)

		require.NoError(t, err, "New with valid config should not return an error")
		require.NotNil(t, srv, "server should not be nil with valid config")
		assert.Len(t, srv.Tools(), 125, "should have 125 registered tools")
	})

	t.Run("tools are registered", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Server: config.ServerConfig{
				Name:     "TestMCP",
				LogLevel: "info",
			},
			Environments: map[string]config.EnvironmentConfig{
				"default": {
					Label:  "Default",
					Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "tok"},
				},
			},
		}

		srv, err := server.New(cfg)
		require.NoError(t, err, "New should succeed with valid config")

		assert.NotEmpty(t, srv.Tools(), "should have at least one tool registered")
	})
}

// TestToolWrapperMethods verifies that ToolWrapper correctly exposes
// the tool definition and handler function for every registered tool.
func TestToolWrapperMethods(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{Name: "Test", LogLevel: "info"},
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "tok"},
			},
		},
	}

	srv, err := server.New(cfg)
	require.NoError(t, err, "New should succeed with valid config")

	for _, tool := range srv.Tools() {
		assert.NotEmpty(t, tool.Name(), "tool name should not be empty")
		assert.NotEmpty(t, tool.Description(), "tool description should not be empty")
		assert.NotNil(t, tool.InputSchema(), "tool input schema should not be nil")
	}
}

// TestToolWrapperExecuteReturnsError verifies that calling Execute on a
// toolWrapper returns ErrExecuteNotImplemented since handlers are dispatched
// through the MCP server, not through the wrapper directly.
func TestToolWrapperExecuteReturnsError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{Name: "Test", LogLevel: "info"},
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "tok"},
			},
		},
	}

	srv, err := server.New(cfg)
	require.NoError(t, err, "New should succeed with valid config")
	require.NotEmpty(t, srv.Tools(), "server should have registered tools")

	result, execErr := srv.Tools()[0].Execute(t.Context(), nil)
	assert.Nil(t, result, "Execute should return nil result")
	assert.ErrorIs(t, execErr, server.ErrExecuteNotImplemented, "Execute should return ErrExecuteNotImplemented")
}

// TestHelloToolHandlerDispatch verifies that the hello tool handler can be
// called directly and returns the expected greeting text.
func TestHelloToolHandlerDispatch(t *testing.T) {
	t.Parallel()

	_, handler := tools.NewHelloTool()

	request := mcp.CallToolRequest{}
	request.Params.Name = "hello"
	request.Params.Arguments = map[string]any{"name": "Test"}

	result, err := handler(t.Context(), request)

	require.NoError(t, err, "hello handler should not return an error")
	require.NotNil(t, result, "hello handler should return a result")
	require.Len(t, result.Content, 1, "result should have exactly one content item")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "first content item should be TextContent")
	assert.Contains(t, textContent.Text, "Hello, Test!", "greeting should contain the provided name")
}
