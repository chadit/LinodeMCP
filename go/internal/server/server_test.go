package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	logLevelInfo    = "info"
	envKeyDefault   = "default"
	envLabelDefault = "Default"
	apiURLLinodeV4  = "https://api.linode.com/v4"
	tokenShort      = "tok"
	serverNameTest  = "Test"
	transportStdio  = "stdio"
	hostLocalhost   = "127.0.0.1"

	// profileSingleTool is the name shared by the filter and reload tests
	// that construct a user-defined profile containing exactly one tool.
	profileSingleTool = "single-tool"

	linodeAccountCallMessage = `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {"name": "linode_account", "arguments": {}}
	}`
)

// baseTestConfig returns a minimal Config sufficient to construct a Server.
// Tests that need a specific active profile copy the value and adjust the
// ActiveProfile and ProfilesBuiltinOverrides fields before calling
// server.New. The shared shape keeps the call sites focused on the profile
// behavior under test.
func baseTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Name:      serverNameTest,
			LogLevel:  logLevelInfo,
			Transport: transportStdio,
			Host:      hostLocalhost,
			Port:      8080,
		},
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenShort},
			},
		},
	}
}

// fullAccessConfig returns a config that selects the built-in full-access
// profile and overrides its default-disabled state so server.New registers
// every tool. Used by tests that need the full tool surface and do not care
// about profile filtering specifically.
func fullAccessConfig() *config.Config {
	cfg := baseTestConfig()
	cfg.ActiveProfile = profiles.BuiltinFullAccess
	cfg.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{
		profiles.BuiltinFullAccess: {Disabled: false},
	}

	return cfg
}

// End-to-end verification of server construction and initialization.
func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("nil config returns error", func(t *testing.T) {
		t.Parallel()

		srv, err := server.New(nil)
		require.Error(t, err, "New with nil config should return an error")
		assert.Nil(t, srv, "server should be nil when config is nil")
		assert.ErrorIs(t, err, server.ErrConfigNil, "error should be ErrConfigNil")
	})

	t.Run("valid config creates server with full-access profile", func(t *testing.T) {
		t.Parallel()

		cfg := fullAccessConfig()
		cfg.Server.Name = "TestMCP"
		cfg.Environments[envKeyDefault] = config.EnvironmentConfig{
			Label:  envLabelDefault,
			Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: "test-token"},
		}

		srv, err := server.New(cfg)

		require.NoError(t, err, "New with valid config should not return an error")
		require.NotNil(t, srv, "server should not be nil with valid config")
		assert.NotEmpty(t, srv.Tools(), "full-access should register every tool")
		assert.Equal(
			t,
			profiles.BuiltinFullAccess,
			srv.ActiveProfile().Name,
			"server should expose the resolved active profile",
		)
	})

	t.Run("tools are registered", func(t *testing.T) {
		t.Parallel()

		cfg := baseTestConfig()
		cfg.Server = config.ServerConfig{Name: "TestMCP", LogLevel: logLevelInfo}

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
		Server: config.ServerConfig{Name: serverNameTest, LogLevel: logLevelInfo},
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenShort},
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

func TestToolDescriptorsIncludesExpectedTools(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(baseTestConfig())
	want := map[string]profiles.Capability{
		"linode_instance_stats_month_get":    profiles.CapRead,
		"linode_instance_transfer_month_get": profiles.CapRead,
		"linode_firewall_rules_list":         profiles.CapRead,
		"linode_firewall_rules_update":       profiles.CapWrite,
		"linode_firewall_rule_version_get":   profiles.CapRead,
		"linode_firewall_devices_list":       profiles.CapRead,
		"linode_firewall_device_get":         profiles.CapRead,
		"linode_firewall_device_create":      profiles.CapWrite,
		"linode_firewall_device_delete":      profiles.CapDestroy,
		"linode_image_create":                profiles.CapWrite,
		"linode_image_replicate":             profiles.CapWrite,
		"linode_image_update":                profiles.CapWrite,

		"linode_image_sharegroup_images_add":                 profiles.CapWrite,
		"linode_image_sharegroup_members_add":                profiles.CapWrite,
		"linode_image_sharegroup_image_delete":               profiles.CapDestroy,
		"linode_image_sharegroup_image_update":               profiles.CapWrite,
		"linode_image_sharegroup_member_token_delete":        profiles.CapDestroy,
		"linode_image_sharegroup_token_create":               profiles.CapAdmin,
		"linode_domain_record_get":                           profiles.CapRead,
		"linode_domain_zone_file_get":                        profiles.CapRead,
		"linode_account_availability":                        profiles.CapRead,
		"linode_account_transfer":                            profiles.CapRead,
		"linode_account_settings":                            profiles.CapRead,
		"linode_account_settings_managed_enable":             profiles.CapAdmin,
		"linode_managed_credentials":                         profiles.CapRead,
		"linode_managed_credential_update":                   profiles.CapAdmin,
		"linode_managed_credential_username_password_update": profiles.CapAdmin,
		"linode_managed_ssh_key":                             profiles.CapRead,
		"linode_managed_credential_create":                   profiles.CapAdmin,
		"linode_managed_credential_get":                      profiles.CapAdmin,
		"linode_managed_credential_revoke":                   profiles.CapAdmin,
		"linode_longview_client_get":                         profiles.CapRead,
		"linode_longview_subscription_get":                   profiles.CapRead,
		"linode_account_maintenance":                         profiles.CapRead,
		"linode_maintenance_policies":                        profiles.CapRead,
		"linode_managed_contact_delete":                      profiles.CapDestroy,
		"linode_managed_contacts":                            profiles.CapRead,
		"linode_managed_linode_settings":                     profiles.CapRead,
		"linode_managed_stats":                               profiles.CapRead,
		"linode_managed_linode_settings_update":              profiles.CapAdmin,
		"linode_managed_service_delete":                      profiles.CapDestroy,
		"linode_managed_service_disable":                     profiles.CapAdmin,
		"linode_managed_service_enable":                      profiles.CapAdmin,
		"linode_managed_service_get":                         profiles.CapRead,
		"linode_managed_service_update":                      profiles.CapAdmin,
		"linode_managed_services":                            profiles.CapRead,
		"linode_managed_issue_get":                           profiles.CapRead,
		"linode_managed_issues":                              profiles.CapRead,
		"linode_managed_contact_update":                      profiles.CapAdmin,
		"linode_account_notifications":                       profiles.CapRead,
		"linode_longview_plan":                               profiles.CapRead,
		"linode_longview_types":                              profiles.CapRead,
		"linode_longview_subscriptions":                      profiles.CapRead,
		"linode_monitor_services":                            profiles.CapRead,
		"linode_monitor_service_get":                         profiles.CapRead,
		"linode_monitor_service_dashboards":                  profiles.CapRead,
		"linode_monitor_service_metrics":                     profiles.CapRead,
		"linode_monitor_service_metric_definitions":          profiles.CapRead,
		"linode_monitor_service_alert_definition_get":        profiles.CapRead,
		"linode_monitor_service_alert_definition_delete":     profiles.CapDestroy,

		"linode_monitor_service_alert_definition_update":        profiles.CapWrite,
		"linode_monitor_dashboards":                             profiles.CapRead,
		"linode_monitor_dashboard_get":                          profiles.CapRead,
		"linode_monitor_alert_definitions":                      profiles.CapRead,
		"linode_monitor_alert_channels":                         profiles.CapRead,
		"linode_longview_client_create":                         profiles.CapAdmin,
		"linode_longview_plan_update":                           profiles.CapAdmin,
		"linode_betas":                                          profiles.CapRead,
		"linode_beta_get":                                       profiles.CapRead,
		"linode_account_betas":                                  profiles.CapRead,
		"linode_account_oauth_clients":                          profiles.CapRead,
		"linode_longview_clients":                               profiles.CapRead,
		"linode_longview_client_update":                         profiles.CapAdmin,
		"linode_longview_client_delete":                         profiles.CapDestroy,
		"linode_account_payment_methods":                        profiles.CapRead,
		"linode_account_payment_method_create":                  profiles.CapAdmin,
		"linode_account_payment_method_delete":                  profiles.CapAdmin,
		"linode_account_payment_method_make_default":            profiles.CapAdmin,
		"linode_account_oauth_client_create":                    profiles.CapAdmin,
		"linode_account_oauth_client_update":                    profiles.CapAdmin,
		"linode_account_oauth_client_thumbnail_update":          profiles.CapAdmin,
		"linode_account_oauth_client_delete":                    profiles.CapAdmin,
		"linode_account_oauth_client_reset_secret":              profiles.CapAdmin,
		"linode_account_events":                                 profiles.CapRead,
		"linode_account_users":                                  profiles.CapRead,
		"linode_managed_service_create":                         profiles.CapAdmin,
		"linode_managed_linode_settings_get":                    profiles.CapRead,
		"linode_managed_contact_get":                            profiles.CapRead,
		"linode_account_user_get":                               profiles.CapRead,
		"linode_account_user_grants":                            profiles.CapRead,
		"linode_account_user_grants_update":                     profiles.CapAdmin,
		"linode_account_user_create":                            profiles.CapAdmin,
		"linode_managed_contact_create":                         profiles.CapAdmin,
		"linode_account_logins":                                 profiles.CapRead,
		"linode_account_invoices":                               profiles.CapRead,
		"linode_account_payments":                               profiles.CapRead,
		"linode_account_payment_create":                         profiles.CapAdmin,
		"linode_account_promo_credit":                           profiles.CapAdmin,
		"linode_account_invoice_get":                            profiles.CapRead,
		"linode_account_invoice_items":                          profiles.CapRead,
		"linode_account_child_accounts":                         profiles.CapRead,
		"linode_account_entity_transfers":                       profiles.CapRead,
		"linode_account_service_transfers":                      profiles.CapRead,
		"linode_account_service_transfer_get":                   profiles.CapRead,
		"linode_account_service_transfer_create":                profiles.CapAdmin,
		"linode_account_service_transfer_delete":                profiles.CapDestroy,
		"linode_account_service_transfer_accept":                profiles.CapAdmin,
		"linode_account_entity_transfer_get":                    profiles.CapRead,
		"linode_account_event_get":                              profiles.CapRead,
		"linode_account_event_seen":                             profiles.CapAdmin,
		"linode_account_entity_transfer_create":                 profiles.CapAdmin,
		"linode_account_child_account_get":                      profiles.CapRead,
		"linode_account_child_account_token":                    profiles.CapAdmin,
		"linode_account_beta_get":                               profiles.CapRead,
		"linode_account_beta_enroll":                            profiles.CapAdmin,
		"linode_account_cancel":                                 profiles.CapAdmin,
		"linode_account_availability_get":                       profiles.CapRead,
		"linode_instance_transfer_get":                          profiles.CapRead,
		"linode_kernel_list":                                    profiles.CapRead,
		"linode_instance_nodebalancer_list":                     profiles.CapRead,
		"linode_database_engine_list":                           profiles.CapRead,
		"linode_database_type_list":                             profiles.CapRead,
		"linode_database_type_get":                              profiles.CapRead,
		"linode_database_engine_get":                            profiles.CapRead,
		"linode_database_mysql_config_get":                      profiles.CapRead,
		"linode_database_postgresql_config_get":                 profiles.CapRead,
		"linode_database_instance_list":                         profiles.CapRead,
		"linode_database_postgresql_instance_list":              profiles.CapRead,
		"linode_database_instance_get":                          profiles.CapRead,
		"linode_database_postgresql_instance_get":               profiles.CapRead,
		"linode_database_instance_ssl_get":                      profiles.CapRead,
		"linode_database_postgresql_instance_ssl_get":           profiles.CapRead,
		"linode_database_instance_credentials_get":              profiles.CapAdmin,
		"linode_database_postgresql_instance_credentials_get":   profiles.CapAdmin,
		"linode_database_instance_credentials_reset":            profiles.CapAdmin,
		"linode_database_postgresql_instance_credentials_reset": profiles.CapAdmin,
		"linode_database_instance_create":                       profiles.CapWrite,
		"linode_database_postgresql_instance_create":            profiles.CapWrite,
		"linode_database_instance_update":                       profiles.CapWrite,
		"linode_database_postgresql_instance_update":            profiles.CapWrite,
		"linode_database_instance_delete":                       profiles.CapDestroy,
		"linode_database_postgresql_instance_delete":            profiles.CapDestroy,
		"linode_database_instance_patch":                        profiles.CapWrite,
		"linode_database_postgresql_instance_patch":             profiles.CapWrite,
		"linode_database_instance_suspend":                      profiles.CapWrite,
		"linode_database_postgresql_instance_suspend":           profiles.CapWrite,
		"linode_database_instance_resume":                       profiles.CapWrite,
		"linode_database_postgresql_instance_resume":            profiles.CapWrite,
		"linode_instance_firewalls_update":                      profiles.CapWrite,
		"linode_instance_firewalls_apply":                       profiles.CapWrite,
		"linode_interfaces_upgrade":                             profiles.CapWrite,
		"linode_instance_interfaces_list":                       profiles.CapRead,
		"linode_instance_interface_get":                         profiles.CapRead,
		"linode_instance_interface_firewalls_list":              profiles.CapRead,
		"linode_instance_interface_delete":                      profiles.CapDestroy,
		"linode_instance_interface_settings_get":                profiles.CapRead,
		"linode_instance_interface_settings_update":             profiles.CapWrite,
		"linode_instance_interface_history_list":                profiles.CapRead,
		"linode_instance_interface_add":                         profiles.CapWrite,
		"linode_instance_interface_update":                      profiles.CapWrite,
		"linode_instance_config_interface_add":                  profiles.CapWrite,
		"linode_instance_config_interface_get":                  profiles.CapRead,
		"linode_instance_config_interface_update":               profiles.CapWrite,
		"linode_instance_config_interface_delete":               profiles.CapDestroy,
		"linode_instance_firewall_list":                         profiles.CapRead,
	}

	for _, descriptor := range descriptors {
		if capability, ok := want[descriptor.Name]; ok {
			assert.Equal(t, capability, descriptor.Capability, "descriptor capability should match")
			delete(want, descriptor.Name)
		}
	}

	assert.Empty(t, want, "expected descriptors should be registered")
}

// TestToolWrapperExecuteReturnsError verifies that calling Execute on a
// toolWrapper returns ErrExecuteNotImplemented since handlers are dispatched
// through the MCP server, not through the wrapper directly.
func TestToolWrapperExecuteReturnsError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Server: config.ServerConfig{Name: serverNameTest, LogLevel: logLevelInfo},
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenShort},
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

// TestShutdownReturnsImmediatelyWithNoInflight verifies that Shutdown does
// not deadlock when the WaitGroup counter is zero.
func TestShutdownReturnsImmediatelyWithNoInflight(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	require.NoError(t, srv.Shutdown(ctx), "Shutdown should return nil with no in-flight handlers")
}

// TestShutdownDrainsInflightHandlers dispatches a blocking tool call through
// HandleMessage, then asserts Shutdown waits until that call finishes.
func TestShutdownDrainsInflightHandlers(t *testing.T) {
	t.Parallel()

	handlerEntered := make(chan struct{})
	releaseHandler := make(chan struct{})

	apiServer := newBlockingAccountServer(t, handlerEntered, releaseHandler)
	defer apiServer.Close()

	srv := newTestServerWithAPIURL(t, apiServer.URL)

	dispatchCtx, cancelDispatch := context.WithCancel(t.Context())
	defer cancelDispatch()

	dispatchDone := make(chan struct{})

	go func() {
		defer close(dispatchDone)

		response := srv.HandleMessage(dispatchCtx, []byte(linodeAccountCallMessage))
		assert.NotNil(t, response, "HandleMessage should return a JSON-RPC response")
	}()

	waitForHandlerEntry(t, handlerEntered)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	shutdownDone := make(chan error, 1)

	go func() {
		shutdownDone <- srv.Shutdown(ctx)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown returned before the handler finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseHandler)

	require.NoError(t, <-shutdownDone, "Shutdown should drain the in-flight call")
	waitForDispatchDone(t, dispatchDone)
}

// TestShutdownTimesOutOnStuckHandler dispatches a tool call that stays inside
// the HTTP handler, then asserts Shutdown returns the ctx error when the
// deadline elapses before drain completes.
func TestShutdownTimesOutOnStuckHandler(t *testing.T) {
	t.Parallel()

	handlerEntered := make(chan struct{})
	releaseHandler := make(chan struct{})

	apiServer := newBlockingAccountServer(t, handlerEntered, releaseHandler)
	defer apiServer.Close()

	srv := newTestServerWithAPIURL(t, apiServer.URL)

	dispatchCtx, cancelDispatch := context.WithCancel(t.Context())
	defer cancelDispatch()

	dispatchDone := make(chan struct{})

	go func() {
		defer close(dispatchDone)

		response := srv.HandleMessage(dispatchCtx, []byte(linodeAccountCallMessage))
		assert.NotNil(t, response, "HandleMessage should return a JSON-RPC response")
	}()

	waitForHandlerEntry(t, handlerEntered)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	err := srv.Shutdown(ctx)

	close(releaseHandler)

	require.Error(t, err, "Shutdown should return an error when the drain times out")
	require.ErrorIs(t, err, context.DeadlineExceeded, "Shutdown should wrap the context error")
	waitForDispatchDone(t, dispatchDone)
}

func newTestServer(t *testing.T) *server.Server {
	t.Helper()

	return newTestServerWithAPIURL(t, apiURLLinodeV4)
}

func newTestServerWithAPIURL(t *testing.T, apiURL string) *server.Server {
	t.Helper()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      serverNameTest,
			LogLevel:  logLevelInfo,
			Transport: transportStdio,
			Host:      hostLocalhost,
			Port:      8080,
		},
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenShort},
			},
		},
	}

	srv, err := server.New(cfg)
	require.NoError(t, err, "test server construction should succeed")

	return srv
}

func newBlockingAccountServer(t *testing.T, handlerEntered chan<- struct{}, releaseHandler <-chan struct{}) *httptest.Server {
	t.Helper()

	var signalOnce sync.Once

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/account" {
			http.NotFound(w, r)

			return
		}

		signalOnce.Do(func() {
			close(handlerEntered)
		})

		select {
		case <-releaseHandler:
		case <-r.Context().Done():
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
}

func waitForHandlerEntry(t *testing.T, handlerEntered <-chan struct{}) {
	t.Helper()

	select {
	case <-handlerEntered:
	case <-time.After(time.Second):
		t.Fatal("blocking account handler didn't start")
	}
}

func waitForDispatchDone(t *testing.T, dispatchDone <-chan struct{}) {
	t.Helper()

	select {
	case <-dispatchDone:
	case <-time.After(time.Second):
		t.Fatal("dispatch goroutine still running after Shutdown returned")
	}
}

// TestHelloToolHandlerDispatch verifies that the hello tool handler can be
// called directly and returns the expected greeting text.
func TestHelloToolHandlerDispatch(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewHelloTool(nil)

	request := mcp.CallToolRequest{}
	request.Params.Name = "hello"
	request.Params.Arguments = map[string]any{"name": serverNameTest}

	result, err := handler(t.Context(), request)

	require.NoError(t, err, "hello handler should not return an error")
	require.NotNil(t, result, "hello handler should return a result")
	require.Len(t, result.Content, 1, "result should have exactly one content item")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "first content item should be TextContent")
	assert.Contains(t, textContent.Text, "Hello, Test!", "greeting should contain the provided name")
}
