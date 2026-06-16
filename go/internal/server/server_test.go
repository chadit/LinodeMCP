package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
	"github.com/chadit/LinodeMCP/go/internal/tools"
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
		"params": {"name": "linode_account_get", "arguments": {}}
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
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		if srv != nil {
			t.Errorf("srv = %v, want nil", srv)
		}

		if !errors.Is(err, server.ErrConfigNil) {
			t.Errorf("error = %v, want %v", err, server.ErrConfigNil)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if srv == nil {
			t.Fatal("srv is nil")
		}

		if len(srv.Tools()) == 0 {
			t.Error("srv.Tools() is empty")
		}

		if srv.ActiveProfile().Name != profiles.BuiltinFullAccess {
			t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profiles.BuiltinFullAccess)
		}
	})

	t.Run("tools are registered", func(t *testing.T) {
		t.Parallel()

		cfg := baseTestConfig()
		cfg.Server = config.ServerConfig{Name: "TestMCP", LogLevel: logLevelInfo}

		srv, err := server.New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(srv.Tools()) == 0 {
			t.Error("srv.Tools() is empty")
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tool := range srv.Tools() {
		if tool.Name() == "" {
			t.Error("tool.Name() is empty")
		}

		if tool.Description() == "" {
			t.Error("tool.Description() is empty")
		}

		if tool.InputSchema() == nil {
			t.Error("tool.InputSchema() is nil")
		}
	}
}

func TestToolDescriptorsIncludesExpectedTools(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(baseTestConfig())
	want := map[string]profiles.Capability{
		"linode_instance_stats_month_get":    profiles.CapRead,
		"linode_instance_transfer_month_get": profiles.CapRead,
		"linode_firewall_rules_get":          profiles.CapRead,
		"linode_firewall_rules_update":       profiles.CapWrite,
		"linode_firewall_rule_version_get":   profiles.CapRead,
		"linode_firewall_device_list":        profiles.CapRead,
		"linode_firewall_device_get":         profiles.CapRead,
		"linode_firewall_device_create":      profiles.CapWrite,
		"linode_firewall_device_delete":      profiles.CapDestroy,
		"linode_networking_ip_get":           profiles.CapRead,
		"linode_networking_ip_update":        profiles.CapWrite,
		"linode_networking_ip_allocate":      profiles.CapWrite,
		"linode_networking_ip_assign":        profiles.CapWrite,
		"linode_networking_ipv4_assign":      profiles.CapWrite,
		"linode_networking_ipv4_share":       profiles.CapWrite,
		"linode_ipv6_range_delete":           profiles.CapDestroy,
		"linode_image_create":                profiles.CapWrite,
		"linode_image_replicate":             profiles.CapWrite,
		"linode_image_update":                profiles.CapWrite,
		"linode_tag_create":                  profiles.CapWrite,

		"linode_image_sharegroup_image_add":                  profiles.CapWrite,
		"linode_image_sharegroup_member_add":                 profiles.CapWrite,
		"linode_image_sharegroup_image_delete":               profiles.CapDestroy,
		"linode_image_sharegroup_image_update":               profiles.CapWrite,
		"linode_image_sharegroup_member_token_delete":        profiles.CapDestroy,
		"linode_image_sharegroup_token_create":               profiles.CapAdmin,
		"linode_domain_record_get":                           profiles.CapRead,
		"linode_domain_zone_file_get":                        profiles.CapRead,
		"linode_account_availability_list":                   profiles.CapRead,
		"linode_profile_preferences_get":                     profiles.CapRead,
		"linode_profile_token_create":                        profiles.CapAdmin,
		"linode_profile_token_delete":                        profiles.CapDestroy,
		"linode_profile_security_question_answer":            profiles.CapAdmin,
		"linode_profile_tfa_enable":                          profiles.CapAdmin,
		"linode_account_transfer_get":                        profiles.CapRead,
		"linode_account_settings_get":                        profiles.CapRead,
		"linode_account_settings_managed_enable":             profiles.CapAdmin,
		"linode_managed_credential_list":                     profiles.CapRead,
		"linode_managed_credential_update":                   profiles.CapAdmin,
		"linode_managed_credential_username_password_update": profiles.CapAdmin,
		"linode_managed_sshkey_get":                          profiles.CapRead,
		"linode_managed_credential_create":                   profiles.CapAdmin,
		"linode_managed_credential_get":                      profiles.CapAdmin,
		"linode_managed_credential_revoke":                   profiles.CapAdmin,
		"linode_longview_client_get":                         profiles.CapRead,
		"linode_longview_subscription_get":                   profiles.CapRead,
		"linode_account_maintenance_list":                    profiles.CapRead,
		"linode_tag_list":                                    profiles.CapRead,
		"linode_maintenance_policy_list":                     profiles.CapRead,
		"linode_managed_contact_delete":                      profiles.CapDestroy,
		"linode_managed_contact_list":                        profiles.CapRead,
		"linode_managed_linode_settings_list":                profiles.CapRead,
		"linode_managed_stats_get":                           profiles.CapRead,
		"linode_managed_linode_settings_update":              profiles.CapAdmin,
		"linode_managed_service_delete":                      profiles.CapDestroy,
		"linode_managed_service_disable":                     profiles.CapAdmin,
		"linode_managed_service_enable":                      profiles.CapAdmin,
		"linode_managed_service_get":                         profiles.CapRead,
		"linode_managed_service_update":                      profiles.CapAdmin,
		"linode_managed_service_list":                        profiles.CapRead,
		"linode_managed_issue_get":                           profiles.CapRead,
		"linode_managed_issue_list":                          profiles.CapRead,
		"linode_managed_contact_update":                      profiles.CapAdmin,
		"linode_account_notification_list":                   profiles.CapRead,
		"linode_longview_plan_get":                           profiles.CapRead,
		"linode_longview_type_list":                          profiles.CapRead,
		"linode_longview_subscription_list":                  profiles.CapRead,
		"linode_monitor_service_list":                        profiles.CapRead,
		"linode_monitor_service_get":                         profiles.CapRead,
		"linode_monitor_service_dashboard_list":              profiles.CapRead,
		"linode_monitor_service_metric_query":                profiles.CapRead,
		"linode_monitor_service_metric_definition_list":      profiles.CapRead,
		"linode_monitor_service_alert_definition_get":        profiles.CapRead,
		"linode_monitor_service_alert_definition_delete":     profiles.CapDestroy,

		"linode_monitor_service_alert_definition_update": profiles.CapWrite,
		"linode_monitor_dashboard_list":                  profiles.CapRead,
		"linode_monitor_dashboard_get":                   profiles.CapRead,
		"linode_monitor_alert_definition_list":           profiles.CapRead,
		"linode_monitor_alert_channel_list":              profiles.CapRead,
		"linode_longview_client_create":                  profiles.CapAdmin,
		"linode_longview_plan_update":                    profiles.CapAdmin,
		"linode_beta_list":                               profiles.CapRead,
		"linode_beta_get":                                profiles.CapRead,
		"linode_account_beta_list":                       profiles.CapRead,
		"linode_profile_phone_number_send":               profiles.CapWrite,
		"linode_profile_phone_number_delete":             profiles.CapDestroy,
		"linode_profile_phone_number_verify":             profiles.CapWrite,
		"linode_profile_tfa_disable":                     profiles.CapAdmin,
		"linode_profile_tfa_enable_confirm":              profiles.CapAdmin,
		"linode_profile_token_list":                      profiles.CapRead,
		"linode_profile_token_update":                    profiles.CapAdmin,
		"linode_profile_login_list":                      profiles.CapRead,
		"linode_profile_preferences_update":              profiles.CapWrite,
		"linode_profile_security_question_list":          profiles.CapRead,
		"linode_profile_device_list":                     profiles.CapRead,
		"linode_profile_login_get":                       profiles.CapRead,
		"linode_profile_app_get":                         profiles.CapRead,
		"linode_profile_app_delete":                      profiles.CapDestroy,
		"linode_profile_device_get":                      profiles.CapRead,
		"linode_profile_device_revoke":                   profiles.CapDestroy,
		"linode_account_oauth_client_list":               profiles.CapRead,
		"linode_profile_app_list":                        profiles.CapRead,
	}

	registeredNames := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		registeredNames[descriptor.Name] = struct{}{}
		if capability, ok := want[descriptor.Name]; ok {
			if descriptor.Capability != capability {
				t.Errorf("descriptor.Capability = %v, want %v", descriptor.Capability, capability)
			}

			delete(want, descriptor.Name)
		}
	}

	if _, ok := registeredNames["linode_account_entity_transfers"]; ok {
		t.Errorf("registeredNames has unexpected key %v", "linode_account_entity_transfers")
	}

	if _, ok := registeredNames["linode_account_entity_transfer_create"]; ok {
		t.Errorf("registeredNames has unexpected key %v", "linode_account_entity_transfer_create")
	}

	if _, ok := registeredNames["linode_account_entity_transfer_get"]; ok {
		t.Errorf("registeredNames has unexpected key %v", "linode_account_entity_transfer_get")
	}

	if len(want) != 0 {
		t.Errorf("want = %v, want empty", want)
	}
}

func TestToolDescriptorsIncludesExpectedToolsExtra(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(baseTestConfig())
	want := map[string]profiles.Capability{
		"linode_longview_client_list":                           profiles.CapRead,
		"linode_longview_client_update":                         profiles.CapAdmin,
		"linode_longview_client_delete":                         profiles.CapDestroy,
		"linode_account_payment_method_list":                    profiles.CapRead,
		"linode_account_payment_method_create":                  profiles.CapAdmin,
		"linode_account_payment_method_delete":                  profiles.CapAdmin,
		"linode_account_payment_method_make_default":            profiles.CapAdmin,
		"linode_account_oauth_client_create":                    profiles.CapAdmin,
		"linode_account_oauth_client_update":                    profiles.CapAdmin,
		"linode_account_oauth_client_thumbnail_update":          profiles.CapAdmin,
		"linode_account_oauth_client_delete":                    profiles.CapAdmin,
		"linode_account_oauth_client_secret_reset":              profiles.CapAdmin,
		"linode_account_event_list":                             profiles.CapRead,
		"linode_tag_delete":                                     profiles.CapDestroy,
		"linode_tag_object_list":                                profiles.CapRead,
		"linode_support_ticket_get":                             profiles.CapRead,
		"linode_support_ticket_reply_list":                      profiles.CapRead,
		"linode_support_ticket_list":                            profiles.CapRead,
		"linode_support_ticket_close":                           profiles.CapWrite,
		"linode_account_user_list":                              profiles.CapRead,
		"linode_managed_service_create":                         profiles.CapAdmin,
		"linode_managed_linode_settings_get":                    profiles.CapRead,
		"linode_managed_contact_get":                            profiles.CapRead,
		"linode_account_user_get":                               profiles.CapRead,
		"linode_profile_token_get":                              profiles.CapRead,
		"linode_account_user_grants_get":                        profiles.CapRead,
		"linode_account_user_grants_update":                     profiles.CapAdmin,
		"linode_account_user_create":                            profiles.CapAdmin,
		"linode_support_ticket_create":                          profiles.CapAdmin,
		"linode_support_ticket_attachment_create":               profiles.CapAdmin,
		"linode_support_ticket_reply_create":                    profiles.CapAdmin,
		"linode_managed_contact_create":                         profiles.CapAdmin,
		"linode_account_login_list":                             profiles.CapRead,
		"linode_account_invoice_list":                           profiles.CapRead,
		"linode_account_payment_list":                           profiles.CapRead,
		"linode_account_payment_create":                         profiles.CapAdmin,
		"linode_account_promo_credit_add":                       profiles.CapAdmin,
		"linode_account_invoice_get":                            profiles.CapRead,
		"linode_account_invoice_item_list":                      profiles.CapRead,
		"linode_account_child_account_list":                     profiles.CapRead,
		"linode_account_service_transfer_list":                  profiles.CapRead,
		"linode_account_service_transfer_get":                   profiles.CapRead,
		"linode_account_service_transfer_create":                profiles.CapAdmin,
		"linode_account_service_transfer_delete":                profiles.CapDestroy,
		"linode_account_service_transfer_accept":                profiles.CapAdmin,
		"linode_account_event_get":                              profiles.CapRead,
		"linode_account_event_seen":                             profiles.CapAdmin,
		"linode_account_child_account_get":                      profiles.CapRead,
		"linode_account_child_account_token_create":             profiles.CapAdmin,
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
		tcLinodeNodebalancerConfigGet:                           profiles.CapRead,
		"linode_nodebalancer_firewall_update":                   profiles.CapWrite,
		tcLinodeNodebalancerConfigRebuild:                       profiles.CapWrite,
		"linode_database_engine_get":                            profiles.CapRead,
		"linode_database_mysql_config_get":                      profiles.CapRead,
		"linode_database_postgresql_config_get":                 profiles.CapRead,
		"linode_database_mysql_instance_list":                   profiles.CapRead,
		"linode_database_postgresql_instance_list":              profiles.CapRead,
		"linode_database_mysql_instance_get":                    profiles.CapRead,
		"linode_database_postgresql_instance_get":               profiles.CapRead,
		"linode_database_mysql_instance_ssl_get":                profiles.CapRead,
		"linode_database_postgresql_instance_ssl_get":           profiles.CapRead,
		"linode_database_mysql_instance_credentials_get":        profiles.CapAdmin,
		"linode_database_postgresql_instance_credentials_get":   profiles.CapAdmin,
		"linode_database_mysql_instance_credentials_reset":      profiles.CapAdmin,
		"linode_database_postgresql_instance_credentials_reset": profiles.CapAdmin,
		"linode_database_mysql_instance_create":                 profiles.CapWrite,
		"linode_database_postgresql_instance_create":            profiles.CapWrite,
		"linode_database_mysql_instance_update":                 profiles.CapWrite,
		"linode_database_postgresql_instance_update":            profiles.CapWrite,
		"linode_database_mysql_instance_delete":                 profiles.CapDestroy,
		"linode_database_postgresql_instance_delete":            profiles.CapDestroy,
		"linode_database_mysql_instance_patch":                  profiles.CapWrite,
		"linode_database_postgresql_instance_patch":             profiles.CapWrite,
		"linode_database_mysql_instance_suspend":                profiles.CapWrite,
		"linode_database_postgresql_instance_suspend":           profiles.CapWrite,
		"linode_database_mysql_instance_resume":                 profiles.CapWrite,
		"linode_database_postgresql_instance_resume":            profiles.CapWrite,
		"linode_instance_firewall_update":                       profiles.CapWrite,
		"linode_instance_firewall_apply":                        profiles.CapWrite,
		"linode_instance_interface_upgrade":                     profiles.CapWrite,
		"linode_instance_interface_list":                        profiles.CapRead,
		"linode_instance_interface_get":                         profiles.CapRead,
		"linode_instance_interface_firewall_list":               profiles.CapRead,
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
			if descriptor.Capability != capability {
				t.Errorf("descriptor.Capability = %v, want %v", descriptor.Capability, capability)
			}

			delete(want, descriptor.Name)
		}
	}

	if len(want) != 0 {
		t.Errorf("want = %v, want empty", want)
	}
}

func TestDeprecatedAccountEntityTransfersListToolRemoved(t *testing.T) {
	t.Parallel()

	srv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tool := range srv.Tools() {
		if tool.Name() == "linode_account_entity_transfers" {
			t.Errorf("tool.Name() = %v, do not want %v", tool.Name(), "linode_account_entity_transfers")
		}
	}

	for _, descriptor := range srv.ToolCatalog() {
		if descriptor.Name == "linode_account_entity_transfers" {
			t.Errorf("descriptor.Name = %v, do not want %v", descriptor.Name, "linode_account_entity_transfers")
		}
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(srv.Tools()) == 0 {
		t.Fatal("srv.Tools() is empty")
	}

	result, execErr := srv.Tools()[0].Execute(t.Context(), nil)
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if !errors.Is(execErr, server.ErrExecuteNotImplemented) {
		t.Errorf("error = %v, want %v", execErr, server.ErrExecuteNotImplemented)
	}
}

// TestShutdownReturnsImmediatelyWithNoInflight verifies that Shutdown does
// not deadlock when the WaitGroup counter is zero.
func TestShutdownReturnsImmediatelyWithNoInflight(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
		if response == nil {
			t.Error("response is nil")
		}
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

	if err := <-shutdownDone; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
		if response == nil {
			t.Error("response is nil")
		}
	}()

	waitForHandlerEntry(t, handlerEntered)

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	err := srv.Shutdown(ctx)

	close(releaseHandler)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want %v", err, context.DeadlineExceeded)
	}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

		_, err := w.Write([]byte(`{}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Content) != 1 {
		t.Fatalf("len(result.Content) = %d, want %d", len(result.Content), 1)
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Hello, Test!") {
		t.Errorf("textContent.Text does not contain %v", "Hello, Test!")
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigList(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_list", Capability: profiles.CapRead}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_list", Capability: profiles.CapRead})
	}

	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_firewall_list", Capability: profiles.CapRead}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_firewall_list", Capability: profiles.CapRead})
	}

	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_vpc_config_list", Capability: profiles.CapRead}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_vpc_config_list", Capability: profiles.CapRead})
	}

	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: tcLinodeNodebalancerConfigGet, Capability: profiles.CapRead}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: tcLinodeNodebalancerConfigGet, Capability: profiles.CapRead})
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigNodesList(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_list", Capability: profiles.CapRead}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_list", Capability: profiles.CapRead})
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigNodeGet(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_get", Capability: profiles.CapRead}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_get", Capability: profiles.CapRead})
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigCreate(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_create", Capability: profiles.CapWrite}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_create", Capability: profiles.CapWrite})
	}
}

func TestToolDescriptorsIncludesNodeBalancerNodeCreate(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_create", Capability: profiles.CapWrite}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_create", Capability: profiles.CapWrite})
	}
}

func TestToolDescriptorsIncludesNodeBalancerNodeDelete(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_delete", Capability: profiles.CapDestroy}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_node_delete", Capability: profiles.CapDestroy})
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigUpdate(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_update", Capability: profiles.CapWrite}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_update", Capability: profiles.CapWrite})
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigRebuild(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: tcLinodeNodebalancerConfigRebuild, Capability: profiles.CapWrite}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: tcLinodeNodebalancerConfigRebuild, Capability: profiles.CapWrite})
	}
}

func TestToolDescriptorsIncludesNodeBalancerConfigDelete(t *testing.T) {
	t.Parallel()

	descriptors := server.ToolDescriptors(&config.Config{})
	if !slices.Contains(descriptors, profiles.ToolDescriptor{Name: "linode_nodebalancer_config_delete", Capability: profiles.CapDestroy}) {
		t.Errorf("descriptors does not contain %v", profiles.ToolDescriptor{Name: "linode_nodebalancer_config_delete", Capability: profiles.CapDestroy})
	}
}
