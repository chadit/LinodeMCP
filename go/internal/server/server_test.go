package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/server"
)

func TestNew_NilConfig(t *testing.T) {
	t.Parallel()

	srv, err := server.New(nil)
	require.Error(t, err)
	assert.Nil(t, srv)
	assert.ErrorIs(t, err, server.ErrConfigNil)
}

func TestNew_ValidConfig(t *testing.T) {
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

	require.NoError(t, err)
	require.NotNil(t, srv)
	assert.True(t, srv.HasMCP(), "MCP server should be initialized.")
	assert.True(t, srv.HasConfig(), "config should be stored.")
	assert.Equal(t, 63, srv.GetToolCount(), "should have 63 registered tools.")
}

func TestNew_ToolsRegistered(t *testing.T) {
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
	require.NoError(t, err)

	// Verify tools were registered by checking the tool count.
	assert.Greater(t, srv.GetToolCount(), 0, "should have at least one tool registered.")
}

func TestToolWrapper_Methods(t *testing.T) {
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
	require.NoError(t, err)

	// The tools slice should have toolWrapper items.
	for _, tool := range srv.Tools() {
		assert.NotEmpty(t, tool.Name(), "tool name should not be empty.")
		assert.NotEmpty(t, tool.Description(), "tool description should not be empty.")
		assert.NotNil(t, tool.InputSchema(), "tool input schema should not be nil.")
	}
}

func TestToolWrapper_ExecuteReturnsError(t *testing.T) {
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
	require.NoError(t, err)
	require.NotEmpty(t, srv.Tools())

	result, execErr := srv.Tools()[0].Execute(t.Context(), nil)
	assert.Nil(t, result)
	assert.ErrorIs(t, execErr, server.ErrExecuteNotImplemented)
}
