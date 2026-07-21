package tools_test

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	paginationCasePageZero           = "page zero"
	paginationCasePageString         = "page string"
	paginationCasePageFractional     = "page fractional"
	paginationCasePageSizeTooSmall   = "page_size too small"
	paginationCasePageSizeTooLarge   = "page_size too large"
	paginationCasePageSizeString     = "page_size string"
	paginationCasePageSizeFractional = "page_size fractional"
	paginationMessagePageMustBe      = "page must be"
	invoiceItemLabel                 = "Nanode 1GB"
	messageInvoiceIDPositive         = "invoice_id must be a positive integer"
	keyLoginID                       = "login_id"
	accountLoginUsername             = "account-login-user"
	loginIDCaseZero                  = "login_id zero"
	loginIDCaseNegative              = "login_id negative"
	loginIDCaseFractional            = "login_id fractional"
	keyRedirectURI                   = "redirect_uri"
	keyID                            = "id"
	keyStatus                        = "status"
	keySecret                        = "secret"
	keyExpiry                        = "expiry"
	keyClientID                      = "client_id"
	keyAppID                         = "app_id"
	keyDeviceID                      = "device_id"
	keyUserAgent                     = "user_agent"
	keyLastRemoteAddr                = "last_remote_addr"
	profileAppID                     = 12345
	profileDeviceID                  = 12345
	profileAppLabel                  = "Example OAuth App"
	profileDeviceUserAgent           = "Mozilla/5.0"
	profileDeviceLastAuthenticated   = "2024-01-02T03:04:05"
	keyISOCode                       = "iso_code"
	keyPhoneNumber                   = "phone_number"
	profilePhoneISOCode              = "US"
	profilePhoneNumber               = "+15551234567"
	profilePhoneOTPCode              = "123456"
	keyPreferences                   = "preferences"
	profilePreferenceKeyTheme        = "theme"
	profilePreferenceValueDark       = "dark"
	errISOCodeNonEmpty               = "iso_code must be a non-empty string"
	errOTPCodeNonEmpty               = "otp_code must be a non-empty string"
	invalidProfileIDSlash            = "12/345"
	invalidProfileIDQuery            = "12?345"
	errProfileAppIDRequired          = "app_id is required"
	errProfileAppIDPositive          = "app_id must be a positive integer"
	errProfileDeviceIDRequired       = "device_id is required"
	errProfileDeviceIDPositive       = "device_id must be a positive integer"
	oauthClientID                    = "client-123"
	invalidClientIDSlash             = "client/123"
	invalidClientIDQuery             = "client?123"
	caseClientIDSlash                = "client_id with slash"
	caseClientIDQuerySeparator       = "client_id with query separator"
	caseClientIDEmpty                = "empty client_id"
	caseClientIDNumeric              = "numeric client_id"
	errClientIDRequired              = "client_id is required"
	errClientIDNonEmpty              = "client_id must be a non-empty string"
	errClientIDNoSeparators          = "client_id must not contain path separators"
	oauthClientLabel                 = "my app"
	oauthClientRedirectURI           = "https://example.com/callback"
	caseFalse                        = "false"
	caseBlank                        = "blank"
	caseWhitespacePadded             = "whitespace padded"
	errRedirectURIRequired           = "redirect_uri is required"
	blankString                      = "   "
	invalidBetaIDSlash               = "example/open"
	invalidBetaIDQuery               = "example?open=1"
	invalidBetaIDPadded              = " example_open "
	keyPublic                        = "public"
	keyThumbnailPNGBase64            = "thumbnail_png_base64"
	oauthClientThumbnailPNG          = "png-bytes"
)

// End-to-end verification of the hello tool.
func TestHelloTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := tools.NewHelloTool(nil)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != toolHello {
			t.Errorf("tool.Name = %v, want %v", tool.Name, toolHello)
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("default name", func(t *testing.T) {
		t.Parallel()

		req := mcp.CallToolRequest{}

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if len(result.Content) == 0 {
			t.Fatal("result.Content is empty")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "World") {
			t.Errorf("textContent.Text does not contain %v", "World")
		}

		if !strings.Contains(textContent.Text, "LinodeMCP") {
			t.Errorf("textContent.Text does not contain %v", "LinodeMCP")
		}
	})

	t.Run("custom name", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{keyName: "Alice"})

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if len(result.Content) == 0 {
			t.Fatal("result.Content is empty")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "Alice") {
			t.Errorf("textContent.Text does not contain %v", "Alice")
		}
	})
}

// End-to-end verification of the version tool.
func TestVersionTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := tools.NewVersionTool(nil)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "version" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "version")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		result, err := handler(t.Context(), mcp.CallToolRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if len(result.Content) == 0 {
			t.Fatal("result.Content is empty")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		var info appinfo.Info

		err = json.Unmarshal([]byte(textContent.Text), &info)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Version != appinfo.Version {
			t.Errorf("info.Version = %v, want %v", info.Version, appinfo.Version)
		}
	})
}

// End-to-end verification of the instance listing workflow.
func TestLinodeInstancesListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeInstanceListTool(cfg)

	if tool.Name != canRunReadTool {
		t.Errorf("tool.Name = %v, want %v", tool.Name, canRunReadTool)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstancesListToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, handler := tools.NewLinodeInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{canRunKeyEnv: "nonexistent"})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeInstancesListToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: "", Token: ""},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeInstancesListToolSuccess(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{
		{ID: 1, Label: "web-1", Status: statusRunning},
		{ID: 2, Label: "db-1", Status: "stopped"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    instances,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "web-1") {
		t.Errorf("textContent.Text does not contain %v", "web-1")
	}

	if !strings.Contains(textContent.Text, "db-1") {
		t.Errorf("textContent.Text does not contain %v", "db-1")
	}
}

// End-to-end verification of the profile tool.
func TestLinodeProfileTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeProfileTool(cfg)

		if tool.Name != "linode_profile_get" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_get")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		profile := linode.Profile{
			Username: "testuser",
			Email:    "test@example.com",
			UID:      42,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(profile); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "testuser") {
			t.Errorf("textContent.Text does not contain %v", "testuser")
		}
	})
}

func TestLinodeProfilePreferencesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfilePreferencesTool(cfg)

	if tool.Name != "linode_profile_preferences_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_preferences_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeProfilePreferencesToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{},
			},
		},
	}
	_, _, handler := tools.NewLinodeProfilePreferencesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeProfilePreferencesToolApiFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeProfilePreferencesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeProfilePreferencesToolSuccess(t *testing.T) {
	t.Parallel()

	preferences := linode.ProfilePreferences{
		"desktop_notifications": true,
		"sort_order":            "ascending",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfilePreferences {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePreferences)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(preferences); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeProfilePreferencesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "desktop_notifications") {
		t.Errorf("textContent.Text does not contain %v", "desktop_notifications")
	}

	if !strings.Contains(textContent.Text, "ascending") {
		t.Errorf("textContent.Text does not contain %v", "ascending")
	}
}

// End-to-end verification of the instance get workflow.
func TestLinodeInstanceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeInstanceGetTool(cfg)

	if tool.Name != "linode_instance_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_instance_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeInstanceGetToolMissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeInstanceGetToolInvalidInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: notANumber})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeInstanceGetToolSuccess(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{
		ID:     123,
		Label:  "test-instance",
		Status: statusRunning,
		Region: regionUSEast,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != instanceGetPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, instanceGetPath)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(instance); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: "123"})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "test-instance") {
		t.Errorf("textContent.Text does not contain %v", "test-instance")
	}

	if !strings.Contains(textContent.Text, statusRunning) {
		t.Errorf("textContent.Text does not contain %v", statusRunning)
	}
}

// End-to-end verification of account info retrieval.
func TestLinodeAccountTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeAccountTool(cfg)

		if tool.Name != "linode_account_get" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_get")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		account := linode.Account{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Company:   "TestCo",
			Balance:   100.50,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != tcAccount {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccount)
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(account); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := handler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "Test") {
			t.Errorf("textContent.Text does not contain %v", "Test")
		}

		if !strings.Contains(textContent.Text, "test@example.com") {
			t.Errorf("textContent.Text does not contain %v", "test@example.com")
		}
	})
}

func TestLinodeAccountTransferToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountTransferTool(cfg)

	if tool.Name != "linode_account_transfer_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_transfer_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(string(tool.RawInputSchema), canRunKeyEnv) {
		t.Errorf("tool.RawInputSchema missing key %v", canRunKeyEnv)
	}

	if strings.Contains(string(tool.RawInputSchema), keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountTransferToolSuccess(t *testing.T) {
	t.Parallel()

	transfer := linode.AccountTransfer{
		Billable: 10,
		Quota:    4000,
		Used:     123,
		RegionTransfers: []linode.AccountRegionTransfer{{
			ID:       "us-east",
			Billable: 2,
			Quota:    1000,
			Used:     50,
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/transfer" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/transfer")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(transfer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountTransferTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "4000") {
		t.Errorf("textContent.Text does not contain %v", "4000")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}

	if !strings.Contains(textContent.Text, "us-east") {
		t.Errorf("textContent.Text does not contain %v", "us-east")
	}
}

func TestLinodeAccountTransferToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/transfer" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/transfer")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountTransferTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_transfer_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_transfer_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of account settings retrieval.
func TestLinodeAccountSettingsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountSettingsTool(cfg)

	if tool.Name != "linode_account_settings_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_settings_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountSettingsToolSuccess(t *testing.T) {
	t.Parallel()

	longviewSubscription := "longview-3"
	objectStorage := statusActive
	settings := linode.AccountSettings{
		BackupsEnabled:          true,
		Managed:                 false,
		NetworkHelper:           true,
		LongviewSubscription:    &longviewSubscription,
		ObjectStorage:           &objectStorage,
		InterfacesForNewLinodes: "linode_default_but_legacy_config_allowed",
		MaintenancePolicy:       maintenancePolicyMigrate,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountSettingsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountSettingsTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountSettingsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, tcBackupsEnabled) {
		t.Errorf("textContent.Text does not contain %v", tcBackupsEnabled)
	}

	if !strings.Contains(textContent.Text, tcNetworkHelper) {
		t.Errorf("textContent.Text does not contain %v", tcNetworkHelper)
	}

	if !strings.Contains(textContent.Text, "longview-3") {
		t.Errorf("textContent.Text does not contain %v", "longview-3")
	}

	if !strings.Contains(textContent.Text, maintenancePolicyMigrate) {
		t.Errorf("textContent.Text does not contain %v", maintenancePolicyMigrate)
	}
}

func TestLinodeAccountSettingsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountSettingsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountSettingsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountSettingsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed") {
		t.Errorf("textContent.Text does not contain %v", "Failed")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of account agreement retrieval.
func TestLinodeAccountAgreementsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountAgreementsTool(cfg)

	if tool.Name != "linode_account_agreement_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_agreement_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountAgreementsToolSuccess(t *testing.T) {
	t.Parallel()

	agreements := linode.AccountAgreements{
		BillingAgreement:       true,
		EUModel:                true,
		MasterServiceAgreement: true,
		PrivacyPolicy:          false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountAgreementsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountAgreementsTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(agreements); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountAgreementsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "billing_agreement") {
		t.Errorf("textContent.Text does not contain %v", "billing_agreement")
	}

	if !strings.Contains(textContent.Text, keyPrivacyPolicy) {
		t.Errorf("textContent.Text does not contain %v", keyPrivacyPolicy)
	}
}

func TestLinodeAccountAgreementsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountAgreementsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountAgreementsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountAgreementsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed") {
		t.Errorf("textContent.Text does not contain %v", "Failed")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of account maintenance listing.
func TestLinodeAccountMaintenanceToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountMaintenanceTool(cfg)

	if tool.Name != "linode_account_maintenance_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_maintenance_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountMaintenanceToolSuccess(t *testing.T) {
	const (
		accountMaintenancePath       = "/account/maintenance"
		accountMaintenanceLabel      = "web-1"
		accountMaintenanceEntityType = "linode"
		accountMaintenanceURL        = "/v4/linode/instances/123"
	)

	t.Parallel()

	maintenance := linode.PaginatedResponse[linode.AccountMaintenance]{
		Data: []linode.AccountMaintenance{{
			Entity: linode.AccountMaintenanceEntity{ID: 123, Label: accountMaintenanceLabel, Type: accountMaintenanceEntityType, URL: accountMaintenanceURL},
			Reason: "Scheduled migration",
			Status: statusPending,
			Type:   "reboot",
			When:   "2026-06-01T00:00:00",
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountMaintenancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountMaintenancePath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(maintenance); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountMaintenanceTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountMaintenanceLabel) {
		t.Errorf("textContent.Text does not contain %v", accountMaintenanceLabel)
	}

	if !strings.Contains(textContent.Text, "Scheduled migration") {
		t.Errorf("textContent.Text does not contain %v", "Scheduled migration")
	}
}

func TestLinodeAccountMaintenanceToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountMaintenanceTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeAccountMaintenanceToolApiError(t *testing.T) {
	const (
		accountMaintenancePath       = "/account/maintenance"
		accountMaintenanceLabel      = "web-1"
		accountMaintenanceEntityType = "linode"
		accountMaintenanceURL        = "/v4/linode/instances/123"
	)

	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountMaintenancePath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountMaintenancePath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountMaintenanceTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of maintenance policies listing.
func TestLinodeMaintenancePoliciesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeMaintenancePoliciesTool(cfg)

	if tool.Name != "linode_maintenance_policy_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_maintenance_policy_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeMaintenancePoliciesToolSuccess(t *testing.T) {
	const (
		maintenancePoliciesPath = "/maintenance/policies"
		maintenancePolicySlug   = "linode/migrate"
		maintenancePolicyLabel  = "Migrate"
	)

	t.Parallel()

	policies := linode.PaginatedResponse[linode.MaintenancePolicy]{
		Data: []linode.MaintenancePolicy{{
			Slug:                  maintenancePolicySlug,
			Label:                 maintenancePolicyLabel,
			Description:           "Migrates the Linode during maintenance.",
			Type:                  "migrate",
			NotificationPeriodSec: 86400,
			IsDefault:             true,
		}},
		Page:    1,
		Pages:   1,
		Results: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != maintenancePoliciesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, maintenancePoliciesPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(policies); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMaintenancePoliciesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, maintenancePolicySlug) {
		t.Errorf("textContent.Text does not contain %v", maintenancePolicySlug)
	}

	if !strings.Contains(textContent.Text, maintenancePolicyLabel) {
		t.Errorf("textContent.Text does not contain %v", maintenancePolicyLabel)
	}
}

func TestLinodeMaintenancePoliciesToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeMaintenancePoliciesTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeMaintenancePoliciesToolApiError(t *testing.T) {
	const (
		maintenancePoliciesPath = "/maintenance/policies"
		maintenancePolicySlug   = "linode/migrate"
		maintenancePolicyLabel  = "Migrate"
	)

	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != maintenancePoliciesPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, maintenancePoliciesPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeMaintenancePoliciesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of regional account availability retrieval.
func TestLinodeAccountAvailabilityGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

	if tool.Name != "linode_account_availability_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_availability_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountAvailabilityGetToolSuccess(t *testing.T) {
	t.Parallel()

	availability := linode.AccountAvailability{
		Available:   []string{serviceLinodes, serviceNodeBalancers},
		Region:      regionUSEast,
		Unavailable: []string{"Kubernetes", serviceBlockStorage},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability/"+regionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability/"+regionUSEast)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(availability); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}

	if !strings.Contains(textContent.Text, serviceLinodes) {
		t.Errorf("textContent.Text does not contain %v", serviceLinodes)
	}
}

func TestLinodeAccountAvailabilityGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability/"+regionUSEast {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability/"+regionUSEast)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_availability_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_availability_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountAvailabilityGetToolInvalidRegionRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingRegion, args: map[string]any{}, wantMessage: "region_id is required"},
		{name: caseEmpty, args: map[string]any{keyRegionID: ""}, wantMessage: errRegionIDNonEmpty},
		{name: caseNumber, args: map[string]any{keyRegionID: 123}, wantMessage: errRegionIDNonEmpty},
		{name: caseSlash, args: map[string]any{keyRegionID: regionIDSlashValue}, wantMessage: errRegionIDSlug},
		{name: caseQuery, args: map[string]any{keyRegionID: regionIDQueryValue}, wantMessage: errRegionIDSlug},
		{name: caseDotTraversal, args: map[string]any{keyRegionID: pathTraversalValue}, wantMessage: errRegionIDSlug},
		{name: "whitespace", args: map[string]any{keyRegionID: "us east"}, wantMessage: errRegionIDSlug},
		{name: "fragment", args: map[string]any{keyRegionID: "us-east#frag"}, wantMessage: errRegionIDSlug},
		{name: "ampersand", args: map[string]any{keyRegionID: "us-east&x"}, wantMessage: errRegionIDSlug},
		{name: "equals", args: map[string]any{keyRegionID: "us-east=1"}, wantMessage: errRegionIDSlug},
		{name: "backslash", args: map[string]any{keyRegionID: `us\east`}, wantMessage: errRegionIDSlug},
		{name: "uppercase", args: map[string]any{keyRegionID: "US-east"}, wantMessage: errRegionIDSlug},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountAvailabilityGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account notifications retrieval.
func TestLinodeAccountNotificationsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountNotificationsTool(cfg)

	if tool.Name != "linode_account_notification_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_notification_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountNotificationsToolSuccess(t *testing.T) {
	t.Parallel()

	notifications := linode.PaginatedResponse[linode.AccountNotification]{
		Data: []linode.AccountNotification{{
			Label:    "Scheduled maintenance",
			Message:  "Maintenance is scheduled for a Linode.",
			Severity: "major",
			Type:     "maintenance",
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/notifications" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/notifications")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(notifications); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountNotificationsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Scheduled maintenance") {
		t.Errorf("textContent.Text does not contain %v", "Scheduled maintenance")
	}

	if !strings.Contains(textContent.Text, "major") {
		t.Errorf("textContent.Text does not contain %v", "major")
	}
}

func TestLinodeAccountNotificationsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/notifications" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/notifications")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountNotificationsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountNotificationsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: "page_size must be"},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: "page_size must be"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountNotificationsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of available beta programs retrieval.
func TestLinodeBetasToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeBetasTool(cfg)

	if tool.Name != "linode_beta_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_beta_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}
}

func TestLinodeBetasToolSuccess(t *testing.T) {
	t.Parallel()

	description := "This beta lets users try an example feature."
	betas := linode.PaginatedResponse[linode.BetaProgram]{
		Data: []linode.BetaProgram{{
			BetaClass:      "open",
			Description:    &description,
			Ended:          nil,
			GreenlightOnly: false,
			ID:             betaExampleOpen,
			Label:          labelExampleOpenBeta,
			MoreInfo:       "https://example.com/beta",
			Started:        "2024-01-02T03:04:05",
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/betas" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(betas); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeBetasTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, betaExampleOpen) {
		t.Errorf("textContent.Text does not contain %v", betaExampleOpen)
	}

	if !strings.Contains(textContent.Text, labelExampleOpenBeta) {
		t.Errorf("textContent.Text does not contain %v", labelExampleOpenBeta)
	}

	if !strings.Contains(textContent.Text, "greenlight_only") {
		t.Errorf("textContent.Text does not contain %v", "greenlight_only")
	}
}

func TestLinodeBetasToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/betas" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeBetasTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeBetasToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeBetasTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of available beta program retrieval.
func TestLinodeBetaGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeBetaGetTool(cfg)

	if tool.Name != "linode_beta_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_beta_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyBetaIDPath) {
		t.Errorf("RawInputSchema missing key %v", keyBetaIDPath)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeBetaGetToolSuccess(t *testing.T) {
	t.Parallel()

	description := "This beta lets users try an example feature."
	beta := linode.BetaProgram{
		BetaClass:      "open",
		Description:    &description,
		Ended:          nil,
		GreenlightOnly: false,
		ID:             betaExampleOpen,
		Label:          labelExampleOpenBeta,
		MoreInfo:       "https://example.com/beta",
		Started:        "2024-01-02T03:04:05",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas/"+betaExampleOpen)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(beta); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeBetaGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyBetaIDPath: betaExampleOpen})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, betaExampleOpen) {
		t.Errorf("textContent.Text does not contain %v", betaExampleOpen)
	}

	if !strings.Contains(textContent.Text, labelExampleOpenBeta) {
		t.Errorf("textContent.Text does not contain %v", labelExampleOpenBeta)
	}

	if !strings.Contains(textContent.Text, "greenlight_only") {
		t.Errorf("textContent.Text does not contain %v", "greenlight_only")
	}
}

func TestLinodeBetaGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/betas/"+betaExampleOpen)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeBetaGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyBetaIDPath: betaExampleOpen})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_beta_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_beta_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeBetaGetToolInvalidIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissing, args: map[string]any{}, wantMessage: errBetaIDRequired},
		{name: caseEmpty, args: map[string]any{keyBetaIDPath: ""}, wantMessage: errBetaIDNonEmpty},
		{name: caseBlank, args: map[string]any{keyBetaIDPath: blankString}, wantMessage: errBetaIDNonEmpty},
		{name: caseNumeric, args: map[string]any{keyBetaIDPath: 123}, wantMessage: errBetaIDNonEmpty},
		{name: caseSlash, args: map[string]any{keyBetaIDPath: invalidBetaIDSlash}, wantMessage: errBetaIDChars},
		{name: caseQuery, args: map[string]any{keyBetaIDPath: invalidBetaIDQuery}, wantMessage: errBetaIDChars},
		{name: caseDotTraversal, args: map[string]any{keyBetaIDPath: pathTraversalValue}, wantMessage: errBetaIDChars},
		{name: caseWhitespacePadded, args: map[string]any{keyBetaIDPath: invalidBetaIDPadded}, wantMessage: errBetaIDChars},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeBetaGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

// End-to-end verification of enrolled account beta programs retrieval.
func TestLinodeAccountBetasToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountBetasTool(cfg)

	if tool.Name != "linode_account_beta_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_beta_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountBetasToolSuccess(t *testing.T) {
	t.Parallel()

	description := "This is an open public beta for an example feature."
	betas := linode.PaginatedResponse[linode.AccountBetaProgram]{
		Data: []linode.AccountBetaProgram{{
			Description: &description,
			Ended:       nil,
			Enrolled:    "2023-09-11T00:00:00",
			ID:          betaExampleOpen,
			Label:       labelExampleOpenBeta,
			Started:     "2023-07-11T00:00:00",
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountBetasTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountBetasTestPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(betas); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountBetasTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, betaExampleOpen) {
		t.Errorf("textContent.Text does not contain %v", betaExampleOpen)
	}

	if !strings.Contains(textContent.Text, labelExampleOpenBeta) {
		t.Errorf("textContent.Text does not contain %v", labelExampleOpenBeta)
	}
}

func TestLinodeAccountBetasToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountBetasTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountBetasTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountBetasTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountBetasToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountBetasTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account invoice lookup.
func TestLinodeAccountInvoiceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

	if tool.Name != "linode_account_invoice_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_invoice_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountInvoiceGetToolSuccess(t *testing.T) {
	t.Parallel()

	invoice := linode.AccountInvoice{ID: accountInvoiceID, Date: "2024-01-31T00:00:00", Label: "Invoice #12345", Total: 11.00}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/invoices/12345" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/invoices/12345")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(invoice); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "12345") {
		t.Errorf("textContent.Text does not contain %v", "12345")
	}

	if !strings.Contains(textContent.Text, "Invoice #12345") {
		t.Errorf("textContent.Text does not contain %v", "Invoice #12345")
	}

	if !strings.Contains(textContent.Text, "11") {
		t.Errorf("textContent.Text does not contain %v", "11")
	}
}

func TestLinodeAccountInvoiceGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/invoices/12345" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/invoices/12345")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_invoice_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_invoice_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountInvoiceGetToolInvalidInvoiceIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: "missing invoice id", args: map[string]any{}},
		{name: "string invoice id", args: map[string]any{keyInvoiceID: "12345"}},
		{name: "zero invoice id", args: map[string]any{keyInvoiceID: 0}},
		{name: "negative invoice id", args: map[string]any{keyInvoiceID: -1}},
		{name: "fractional invoice id", args: map[string]any{keyInvoiceID: 1.5}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountInvoiceGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "invoice_id") {
				t.Errorf("textContent.Text does not contain %v", "invoice_id")
			}
		})
	}
}

// End-to-end verification of profile security questions retrieval.
func TestLinodeProfileSecurityQuestionsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeProfileSecurityQuestionsTool(cfg)
	if tool.Name != "linode_profile_security_question_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_security_question_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeProfileSecurityQuestionsToolSuccess(t *testing.T) {
	t.Parallel()

	questions := map[string]any{
		"security_questions": []map[string]any{{
			keyID:      1,
			"question": "What is your favorite color?",
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(questions); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileSecurityQuestionsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "security_questions") {
		t.Errorf("textContent.Text does not contain %v", "security_questions")
	}

	if !strings.Contains(textContent.Text, "What is your favorite color?") {
		t.Errorf("textContent.Text does not contain %v", "What is your favorite color?")
	}
}

func TestLinodeProfileSecurityQuestionsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileSecurityQuestions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileSecurityQuestions)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileSecurityQuestionsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of profile trusted devices retrieval.
func TestLinodeProfileDevicesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeProfileDevicesTool(cfg)
	if tool.Name != "linode_profile_device_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_device_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeProfileDevicesToolSuccess(t *testing.T) {
	t.Parallel()

	devices := map[string]any{
		keyData: []map[string]any{{keyID: 123, keyUserAgent: "Mozilla/5.0", keyLastRemoteAddr: "192.0.2.1"}},
		keyPage: 2, keyPages: 3, keyResults: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/profile/devices" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/devices")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(devices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDevicesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}

	if !strings.Contains(textContent.Text, "Mozilla/5.0") {
		t.Errorf("textContent.Text does not contain %v", "Mozilla/5.0")
	}

	if !strings.Contains(textContent.Text, "192.0.2.1") {
		t.Errorf("textContent.Text does not contain %v", "192.0.2.1")
	}
}

func TestLinodeProfileDevicesToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/profile/devices" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/devices")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDevicesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeProfileDevicesToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeProfileDevicesTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeProfileTFAEnableToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileTFAEnableTool(cfg)

	if tool.Name != "linode_profile_tfa_enable" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_tfa_enable")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfileTFAEnableToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/tfa-enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/tfa-enable")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("request body should be readable: %v", readErr)

			return
		}

		if len(body) != 0 {
			t.Errorf("string(body) = %v, want empty", string(body))
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keySecret: "JBSWY3DPEHPK3PXP", keyExpiry: tfaConfirmExpiry}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileTFAEnableTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "JBSWY3DPEHPK3PXP") {
		t.Errorf("error text %q does not contain %q", text.Text, "JBSWY3DPEHPK3PXP")
	}

	// The one-time secret is returned by design and the response carries the
	// save-the-secret warning byte-identically to the Python implementation.
	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Save this two-factor authentication secret now") {
		t.Errorf("response %q does not carry the save-the-secret warning", text.Text)
	}
}

func TestLinodeProfileTFAEnableToolDryRunPreviewsWithoutPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileTFAEnableTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "dry_run") {
		t.Errorf("error text %q does not contain %q", text.Text, "dry_run")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "/profile/tfa-enable") {
		t.Errorf("error text %q does not contain %q", text.Text, "/profile/tfa-enable")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "side_effects") {
		t.Errorf("error text %q does not contain %q", text.Text, "side_effects")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "must be confirmed") {
		t.Errorf("error text %q does not contain %q", text.Text, "must be confirmed")
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}

func TestLinodeProfileTFAEnableToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/tfa-enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/tfa-enable")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileTFAEnableTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to generate linode_profile_tfa_enable") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to generate linode_profile_tfa_enable")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileTFAEnableToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileTFAEnableTool(cfg)

			args := map[string]any{}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfilePhoneNumberSendToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfilePhoneNumberSendTool(cfg)

	if tool.Name != "linode_profile_phone_number_send" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_phone_number_send")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyISOCode, keyPhoneNumber, keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfilePhoneNumberSendToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("request body should be readable: %v", readErr)

			return
		}

		{
			var wantJSON, gotJSON any
			if err := json.Unmarshal([]byte(`{"iso_code":"US","phone_number":"+15551234567"}`), &wantJSON); err != nil {
				t.Errorf("invalid expected JSON: %v", err)
			}

			if err := json.Unmarshal([]byte(string(body)), &gotJSON); err != nil {
				t.Errorf("invalid actual JSON: %v", err)
			}

			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("JSON = %s, want %s", string(body), `{"iso_code":"US","phone_number":"+15551234567"}`)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberSendTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyISOCode: profilePhoneISOCode, keyPhoneNumber: profilePhoneNumber, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Profile phone number verification code sent successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "Profile phone number verification code sent successfully")
	}
}

func TestLinodeProfilePhoneNumberSendToolDryRunPreviewsWithoutPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberSendTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyISOCode: profilePhoneISOCode, keyPhoneNumber: profilePhoneNumber, keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dryRun, dryRunOK := body["dry_run"].(bool)
	if !dryRunOK {
		t.Fatal("dryRunOK = false, want true")
	}

	if !dryRun {
		t.Error("dryRun = false, want true")
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Fatal("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], tcProfilePhoneNumber) {
		t.Errorf("got %v, want %v", would["path"], tcProfilePhoneNumber)
	}

	previewBody, previewBodyOK := would["body"].(map[string]any)
	if !previewBodyOK {
		t.Fatal("previewBodyOK = false, want true")
	}

	if !reflect.DeepEqual(previewBody[keyISOCode], profilePhoneISOCode) {
		t.Errorf("previewBody[keyISOCode] = %v, want %v", previewBody[keyISOCode], profilePhoneISOCode)
	}

	if !reflect.DeepEqual(previewBody[keyPhoneNumber], profilePhoneNumber) {
		t.Errorf("previewBody[keyPhoneNumber] = %v, want %v", previewBody[keyPhoneNumber], profilePhoneNumber)
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}
}

func TestLinodeProfilePhoneNumberSendToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberSendTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyISOCode: profilePhoneISOCode, keyPhoneNumber: profilePhoneNumber, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to send linode_profile_phone_number_send") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to send linode_profile_phone_number_send")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfilePhoneNumberSendToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePhoneNumberSendTool(cfg)

			args := map[string]any{keyISOCode: profilePhoneISOCode, keyPhoneNumber: profilePhoneNumber}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfilePhoneNumberSendToolRequiredArgumentsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing iso_code", args: map[string]any{keyPhoneNumber: profilePhoneNumber, keyConfirm: true}, wantMessage: errISOCodeNonEmpty},
		{name: "blank iso_code", args: map[string]any{keyISOCode: blankString, keyPhoneNumber: profilePhoneNumber, keyConfirm: true}, wantMessage: errISOCodeNonEmpty},
		{name: "numeric iso_code", args: map[string]any{keyISOCode: 1, keyPhoneNumber: profilePhoneNumber, keyConfirm: true}, wantMessage: errISOCodeNonEmpty},
		{name: "missing phone_number", args: map[string]any{keyISOCode: profilePhoneISOCode, keyConfirm: true}, wantMessage: "phone_number must be a non-empty string"},
		{name: "blank phone_number", args: map[string]any{keyISOCode: profilePhoneISOCode, keyPhoneNumber: blankString, keyConfirm: true}, wantMessage: "phone_number must be a non-empty string"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePhoneNumberSendTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfilePhoneNumberDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfilePhoneNumberDeleteTool(cfg)

	if tool.Name != "linode_profile_phone_number_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_phone_number_delete")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfilePhoneNumberDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Profile phone number deleted successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "Profile phone number deleted successfully")
	}
}

func TestLinodeProfilePhoneNumberDeleteToolDryRunPreviewsWithoutDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "dry_run") {
		t.Errorf("error text %q does not contain %q", text.Text, "dry_run")
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}
}

func TestLinodeProfilePhoneNumberDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfilePhoneNumber {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfilePhoneNumber)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete profile phone number") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete profile phone number")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfilePhoneNumberDeleteToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePhoneNumberDeleteTool(cfg)

			args := map[string]any{}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfilePhoneNumberVerifyToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfilePhoneNumberVerifyTool(cfg)

	if tool.Name != "linode_profile_phone_number_verify" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_phone_number_verify")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyOTPCode, keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfilePhoneNumberVerifyToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/phone-number/verify" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/phone-number/verify")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		body, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("request body should be readable: %v", readErr)

			return
		}

		{
			var wantJSON, gotJSON any
			if err := json.Unmarshal([]byte(`{"otp_code":"123456"}`), &wantJSON); err != nil {
				t.Errorf("invalid expected JSON: %v", err)
			}

			if err := json.Unmarshal([]byte(string(body)), &gotJSON); err != nil {
				t.Errorf("invalid actual JSON: %v", err)
			}

			if !reflect.DeepEqual(gotJSON, wantJSON) {
				t.Errorf("JSON = %s, want %s", string(body), `{"otp_code":"123456"}`)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberVerifyTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyOTPCode: profilePhoneOTPCode, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Profile phone number verified successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "Profile phone number verified successfully")
	}
}

func TestLinodeProfilePhoneNumberVerifyToolDryRunPreviewsWithoutPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberVerifyTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyOTPCode: profilePhoneOTPCode, keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dryRun, dryRunOK := body["dry_run"].(bool)
	if !dryRunOK {
		t.Fatal("dryRunOK = false, want true")
	}

	if !dryRun {
		t.Error("dryRun = false, want true")
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Fatal("wouldOK = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/profile/phone-number/verify") {
		t.Errorf("got %v, want %v", would["path"], "/profile/phone-number/verify")
	}

	previewBody, previewBodyOK := would["body"].(map[string]any)
	if !previewBodyOK {
		t.Fatal("previewBodyOK = false, want true")
	}

	if !reflect.DeepEqual(previewBody[keyOTPCode], profilePhoneOTPCode) {
		t.Errorf("previewBody[keyOTPCode] = %v, want %v", previewBody[keyOTPCode], profilePhoneOTPCode)
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}
}

func TestLinodeProfilePhoneNumberVerifyToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/profile/phone-number/verify" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/phone-number/verify")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfilePhoneNumberVerifyTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyOTPCode: profilePhoneOTPCode, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to verify linode_profile_phone_number_verify") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to verify linode_profile_phone_number_verify")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfilePhoneNumberVerifyToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePhoneNumberVerifyTool(cfg)

			args := map[string]any{keyOTPCode: profilePhoneOTPCode}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfilePhoneNumberVerifyToolRequiredArgumentsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing otp_code", args: map[string]any{keyConfirm: true}, wantMessage: errOTPCodeNonEmpty},
		{name: "blank otp_code", args: map[string]any{keyOTPCode: blankString, keyConfirm: true}, wantMessage: errOTPCodeNonEmpty},
		{name: "numeric otp_code", args: map[string]any{keyOTPCode: 1, keyConfirm: true}, wantMessage: errOTPCodeNonEmpty},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfilePhoneNumberVerifyTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

// End-to-end verification of profile OAuth app authorization retrieval.
func TestLinodeProfileAppsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeProfileAppsTool(cfg)
	if tool.Name != "linode_profile_app_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_app_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeProfileAppsToolSuccess(t *testing.T) {
	t.Parallel()

	apps := map[string]any{
		keyData: []map[string]any{{
			keyID:           123,
			keyLabel:        "example-app",
			"scopes":        "linodes:read_only",
			"website":       "example.org",
			"created":       longviewClientCreatedAt,
			"expiry":        "2018-01-15T00:01:01",
			"thumbnail_url": "https://example.com/icon.png",
		}},
		keyPage:    2,
		keyPages:   3,
		keyResults: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/profile/apps" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/apps")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(apps); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}

	if !strings.Contains(textContent.Text, "example-app") {
		t.Errorf("textContent.Text does not contain %v", "example-app")
	}

	if !strings.Contains(textContent.Text, "linodes:read_only") {
		t.Errorf("textContent.Text does not contain %v", "linodes:read_only")
	}

	if !strings.Contains(textContent.Text, "example.org") {
		t.Errorf("textContent.Text does not contain %v", "example.org")
	}
}

func TestLinodeProfileAppsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/profile/apps" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/apps")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeProfileAppsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeProfileAppsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account OAuth clients retrieval.
func TestLinodeAccountOAuthClientsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)
	if tool.Name != "linode_account_oauth_client_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountOAuthClientsToolSuccess(t *testing.T) {
	t.Parallel()

	clients := map[string]any{
		keyData: []map[string]any{{
			keyID:          "2737bf16b39ab5d7b4a1",
			keyLabel:       "example-client",
			keyRedirectURI: "https://example.com/oauth/callback",
			keySecret:      "super-secret-client-secret",
			keyStatus:      statusActive,
		}},
		keyPage:    2,
		keyPages:   3,
		keyResults: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountOAuthClientsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountOAuthClientsTestPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(clients); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "2737bf16b39ab5d7b4a1") {
		t.Errorf("textContent.Text does not contain %v", "2737bf16b39ab5d7b4a1")
	}

	if !strings.Contains(textContent.Text, "example-client") {
		t.Errorf("textContent.Text does not contain %v", "example-client")
	}

	if strings.Contains(textContent.Text, "super-secret-client-secret") {
		t.Errorf("textContent.Text should not contain %v", "super-secret-client-secret")
	}
}

func TestLinodeAccountOAuthClientsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountOAuthClientsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountOAuthClientsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountOAuthClientsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of child account lookup.
func TestLinodeAccountChildAccountGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

	if tool.Name != "linode_account_child_account_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_child_account_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountChildAccountGetToolSuccess(t *testing.T) {
	t.Parallel()

	childAccount := linode.ChildAccount{
		EUUID:         childAccountEUUID,
		Company:       companyAcme,
		Email:         "jkowalski@example.com",
		FirstName:     "John",
		LastName:      "Smith",
		BillingSource: "external",
		CreditCard: linode.ChildAccountCreditCard{
			Expiry:   "11/2024",
			LastFour: "0111",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56")
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		// The extra field the proto does not model must be dropped by the
		// DiscardUnknown decode, proving the output routes through the proto
		// serializer rather than the legacy struct.
		wrapped := struct {
			linode.ChildAccount

			NotInProto string `json:"not_in_proto"`
		}{ChildAccount: childAccount, NotInProto: valNotInProto}

		if err := json.NewEncoder(w).Encode(wrapped); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, childAccountEUUID) {
		t.Errorf("textContent.Text does not contain %v", childAccountEUUID)
	}

	if !strings.Contains(textContent.Text, companyAcme) {
		t.Errorf("textContent.Text does not contain %v", companyAcme)
	}

	if !strings.Contains(textContent.Text, "0111") {
		t.Errorf("textContent.Text does not contain %v", "0111")
	}

	if strings.Contains(textContent.Text, keyNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeAccountChildAccountGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_child_account_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_child_account_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountChildAccountGetToolInvalidEuuidRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing euuid", args: map[string]any{}, wantMessage: "euuid is required"},
		{name: "empty euuid", args: map[string]any{keyEUUID: ""}, wantMessage: errEUUIDNonEmpty},
		{name: "numeric euuid", args: map[string]any{keyEUUID: 123}, wantMessage: errEUUIDNonEmpty},
		{name: "euuid with slash", args: map[string]any{keyEUUID: "child/account"}, wantMessage: errEUUIDNoSeparators},
		{name: "euuid with query separator", args: map[string]any{keyEUUID: "child?account"}, wantMessage: errEUUIDNoSeparators},
		{name: "euuid with traversal", args: map[string]any{keyEUUID: pathTraversalValue}, wantMessage: errEUUIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountChildAccountGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account events retrieval.
func TestLinodeAccountEventsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountEventsTool(cfg)

	if tool.Name != "linode_account_event_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_event_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountEventsToolSuccess(t *testing.T) {
	t.Parallel()

	events := linode.PaginatedResponse[linode.AccountEvent]{
		Data:    []linode.AccountEvent{{ID: 123, Action: "ticket_create", Status: "failed", Username: "adevi"}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/events" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/events")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(events); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountEventsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "ticket_create") {
		t.Errorf("textContent.Text does not contain %v", "ticket_create")
	}

	if !strings.Contains(textContent.Text, "adevi") {
		t.Errorf("textContent.Text does not contain %v", "adevi")
	}
}

func TestLinodeAccountEventsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/events" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/events")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountEventsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountEventsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountEventsTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account user listing.
func TestLinodeAccountUsersToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUsersTool(cfg)

	if tool.Name != "linode_account_user_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_user_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountUsersToolSuccess(t *testing.T) {
	t.Parallel()

	users := linode.PaginatedResponse[linode.AccountUser]{
		Data:    []linode.AccountUser{{Username: accountLoginUsername, Email: "user@example.com", Restricted: true, TFAEnabled: true}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountUsersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountUsersTestPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(users); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUsersTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountLoginUsername) {
		t.Errorf("textContent.Text does not contain %v", accountLoginUsername)
	}

	if !strings.Contains(textContent.Text, "user@example.com") {
		t.Errorf("textContent.Text does not contain %v", "user@example.com")
	}
}

func TestLinodeAccountUsersToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountUsersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountUsersTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountUsersTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountUsersToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountUsersTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account login listing.
func TestLinodeAccountLoginsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountLoginsTool(cfg)

	if tool.Name != "linode_account_login_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_login_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountLoginsToolSuccess(t *testing.T) {
	t.Parallel()

	logins := linode.PaginatedResponse[linode.AccountLogin]{
		Data:    []linode.AccountLogin{{ID: 123, Username: accountLoginUsername, IP: testNetIPv4AddressTen, Status: statusSuccessful}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/logins" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/logins")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(logins); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountLoginsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountLoginUsername) {
		t.Errorf("textContent.Text does not contain %v", accountLoginUsername)
	}

	if !strings.Contains(textContent.Text, testNetIPv4AddressTen) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressTen)
	}
}

func TestLinodeAccountLoginsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/logins" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/logins")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountLoginsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountLoginsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountLoginsTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of profile login retrieval.
func TestLinodeProfileLoginGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileLoginGetTool(cfg)

	if tool.Name != "linode_profile_login_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_login_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyLoginID) {
		t.Errorf("rawSchema missing key %v", keyLoginID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("rawSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeProfileLoginGetToolSuccess(t *testing.T) {
	t.Parallel()

	login := linode.AccountLogin{ID: 123, Username: accountLoginUsername, IP: testNetIPv4AddressTen, Status: statusSuccessful}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/profile/logins/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/logins/123")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(login); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileLoginGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLoginID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountLoginUsername) {
		t.Errorf("textContent.Text does not contain %v", accountLoginUsername)
	}

	if !strings.Contains(textContent.Text, testNetIPv4AddressTen) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressTen)
	}
}

func TestLinodeProfileLoginGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/profile/logins/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/logins/123")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileLoginGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLoginID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_profile_login_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_profile_login_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeProfileLoginGetToolInvalidLoginIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissing, args: map[string]any{}},
		{name: loginIDCaseZero, args: map[string]any{keyLoginID: 0}},
		{name: loginIDCaseNegative, args: map[string]any{keyLoginID: -1}},
		{name: loginIDCaseFractional, args: map[string]any{keyLoginID: 12.5}},
		{name: "huge numeric", args: map[string]any{keyLoginID: 1e100}},
		{name: "above safe integer", args: map[string]any{keyLoginID: 9007199254740992.0}},
		{name: "path separator string", args: map[string]any{keyLoginID: pathSeparatorValue}},
		{name: "query separator string", args: map[string]any{keyLoginID: "12?debug=true"}},
		{name: "traversal string", args: map[string]any{keyLoginID: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileLoginGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

// End-to-end verification of account login retrieval.
func TestLinodeAccountLoginGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountLoginGetTool(cfg)

	if tool.Name != "linode_account_login_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_login_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyLoginID) {
		t.Errorf("rawSchema missing key %v", keyLoginID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("rawSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountLoginGetToolSuccess(t *testing.T) {
	t.Parallel()

	login := linode.AccountLogin{ID: 123, Username: accountLoginUsername, IP: testNetIPv4AddressTen, Status: statusSuccessful}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/logins/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/logins/123")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(login); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountLoginGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLoginID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountLoginUsername) {
		t.Errorf("textContent.Text does not contain %v", accountLoginUsername)
	}

	if !strings.Contains(textContent.Text, testNetIPv4AddressTen) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressTen)
	}
}

func TestLinodeAccountLoginGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/logins/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/logins/123")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountLoginGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLoginID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_login_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_login_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountLoginGetToolInvalidLoginIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissing, args: map[string]any{}},
		{name: loginIDCaseZero, args: map[string]any{keyLoginID: 0}},
		{name: loginIDCaseNegative, args: map[string]any{keyLoginID: -1}},
		{name: loginIDCaseFractional, args: map[string]any{keyLoginID: 12.5}},
		{name: "huge numeric", args: map[string]any{keyLoginID: 1e100}},
		{name: "above safe integer", args: map[string]any{keyLoginID: 9007199254740992.0}},
		{name: "path separator string", args: map[string]any{keyLoginID: pathSeparatorValue}},
		{name: "query separator string", args: map[string]any{keyLoginID: "12?debug=true"}},
		{name: "traversal string", args: map[string]any{keyLoginID: pathTraversalValue}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountLoginGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

// End-to-end verification of child account listing.
func TestLinodeAccountChildAccountsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

	if tool.Name != "linode_account_child_account_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_child_account_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountChildAccountsToolSuccess(t *testing.T) {
	t.Parallel()

	childAccounts := linode.PaginatedResponse[linode.ChildAccount]{
		Data: []linode.ChildAccount{{
			EUUID:         childAccountEUUID,
			Company:       companyAcme,
			Email:         "jkowalski@example.com",
			FirstName:     "John",
			LastName:      "Smith",
			BillingSource: "external",
			CreditCard: linode.ChildAccountCreditCard{
				Expiry:   "11/2024",
				LastFour: "0111",
			},
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/child-accounts" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(childAccounts); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, childAccountEUUID) {
		t.Errorf("textContent.Text does not contain %v", childAccountEUUID)
	}

	if !strings.Contains(textContent.Text, companyAcme) {
		t.Errorf("textContent.Text does not contain %v", companyAcme)
	}

	if !strings.Contains(textContent.Text, "0111") {
		t.Errorf("textContent.Text does not contain %v", "0111")
	}
}

func TestLinodeAccountChildAccountsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/child-accounts" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountChildAccountsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountChildAccountsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeProfileAppDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileAppDeleteTool(cfg)

	if tool.Name != "linode_profile_app_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_app_delete")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyAppID, keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfileAppDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyAppID: profileAppID, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Profile app 12345 revoked successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "Profile app 12345 revoked successfully")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "\"app_id\"") {
		t.Errorf("response %q does not echo app_id", text.Text)
	}
}

func TestLinodeProfileAppDeleteToolDryRunPreviewsWithoutDelete(t *testing.T) {
	t.Parallel()

	var deleteCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		deleteCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: profileAppID, keyLabel: profileAppLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyAppID: profileAppID, keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "dry_run") {
		t.Errorf("error text %q does not contain %q", text.Text, "dry_run")
	}

	if deleteCalls.Load() != int32(1) {
		t.Errorf("deleteCalls.Load() = %v, want %v", deleteCalls.Load(), int32(1))
	}
}

func TestLinodeProfileAppDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyAppID: profileAppID, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_profile_app_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_profile_app_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileAppDeleteToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileAppDeleteTool(cfg)

			args := map[string]any{keyAppID: profileAppID}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileAppDeleteToolInvalidAppIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, want: errProfileAppIDRequired},
		{name: caseZero, args: map[string]any{keyAppID: 0, keyConfirm: true, keyConfirmedDryRun: true}, want: errProfileAppIDPositive},
		{name: caseString, args: map[string]any{keyAppID: "12345", keyConfirm: true, keyConfirmedDryRun: true}, want: errProfileAppIDPositive},
		{name: caseSlash, args: map[string]any{keyAppID: invalidProfileIDSlash, keyConfirm: true, keyConfirmedDryRun: true}, want: errProfileAppIDPositive},
		{name: caseQuery, args: map[string]any{keyAppID: invalidProfileIDQuery, keyConfirm: true, keyConfirmedDryRun: true}, want: errProfileAppIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyAppID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errProfileAppIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileAppDeleteTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileDeviceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileDeviceGetTool(cfg)

	if tool.Name != "linode_profile_device_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_device_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, keyDeviceID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDeviceID)
	}

	if strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeProfileDeviceGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: profileDeviceID, keyUserAgent: profileDeviceUserAgent, "last_authenticated": profileDeviceLastAuthenticated, keyLastRemoteAddr: testNetIPv4AddressOne, keyNotInProto: valNotInProto,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDeviceGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDeviceID: profileDeviceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, profileDeviceUserAgent) {
		t.Errorf("textContent.Text does not contain %v", profileDeviceUserAgent)
	}

	if !strings.Contains(textContent.Text, testNetIPv4AddressOne) {
		t.Errorf("textContent.Text does not contain %v", testNetIPv4AddressOne)
	}

	if strings.Contains(textContent.Text, "not_in_proto") {
		t.Errorf("textContent.Text unexpectedly contains dropped unknown field: %v", textContent.Text)
	}
}

func TestLinodeProfileDeviceGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDeviceGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDeviceID: profileDeviceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_profile_device_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_profile_device_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileDeviceGetToolInvalidDeviceIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: errProfileDeviceIDRequired},
		{name: caseZero, args: map[string]any{keyDeviceID: 0}, want: errProfileDeviceIDPositive},
		{name: caseNegative, args: map[string]any{keyDeviceID: -1}, want: errProfileDeviceIDPositive},
		{name: "fractional device_id", args: map[string]any{keyDeviceID: 1.5}, want: errProfileDeviceIDPositive},
		{name: caseString, args: map[string]any{keyDeviceID: "12345"}, want: errProfileDeviceIDPositive},
		{name: caseSlash, args: map[string]any{keyDeviceID: invalidProfileIDSlash}, want: errProfileDeviceIDPositive},
		{name: caseQuery, args: map[string]any{keyDeviceID: invalidProfileIDQuery}, want: errProfileDeviceIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyDeviceID: pathTraversalValue}, want: errProfileDeviceIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileDeviceGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileDeviceRevokeToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileDeviceRevokeTool(cfg)

	if tool.Name != "linode_profile_device_revoke" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_device_revoke")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyDeviceID, keyConfirm, keyDryRun} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeProfileDeviceRevokeToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDeviceRevokeTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDeviceID: profileDeviceID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Profile trusted device 12345 revoked successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "Profile trusted device 12345 revoked successfully")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "\"device_id\"") {
		t.Errorf("response %q does not echo device_id", text.Text)
	}
}

func TestLinodeProfileDeviceRevokeToolDryRunPreviewsWithoutDelete(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{keyID: profileDeviceID, "user_agent": "curl/8.0"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDeviceRevokeTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDeviceID: profileDeviceID, keyDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "dry_run") {
		t.Errorf("error text %q does not contain %q", text.Text, "dry_run")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestLinodeProfileDeviceRevokeToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcProfileDevices12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileDevices12345)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileDeviceRevokeTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyDeviceID: profileDeviceID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_profile_device_revoke") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_profile_device_revoke")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileDeviceRevokeToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumeric, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileDeviceRevokeTool(cfg)

			args := map[string]any{keyDeviceID: profileDeviceID}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Set confirm=true to proceed") {
				t.Errorf("error text %q does not contain %q", text.Text, "Set confirm=true to proceed")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileDeviceRevokeToolInvalidDeviceIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errProfileDeviceIDRequired},
		{name: caseZero, args: map[string]any{keyDeviceID: 0, keyConfirm: true}, want: errProfileDeviceIDPositive},
		{name: caseNegative, args: map[string]any{keyDeviceID: -1, keyConfirm: true}, want: errProfileDeviceIDPositive},
		{name: caseString, args: map[string]any{keyDeviceID: "67890", keyConfirm: true}, want: errProfileDeviceIDPositive},
		{name: caseSlash, args: map[string]any{keyDeviceID: "67/890", keyConfirm: true}, want: errProfileDeviceIDPositive},
		{name: caseQuery, args: map[string]any{keyDeviceID: "67?890", keyConfirm: true}, want: errProfileDeviceIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyDeviceID: pathTraversalValue, keyConfirm: true}, want: errProfileDeviceIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileDeviceRevokeTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeProfileAppGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeProfileAppGetTool(cfg)

	if tool.Name != "linode_profile_app_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_profile_app_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyAppID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyAppID)
	}

	if strings.Contains(string(tool.RawInputSchema), keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeProfileAppGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: profileAppID, keyLabel: profileAppLabel, "scopes": "linodes:read_only", "website": "https://example.com",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyAppID: profileAppID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, profileAppLabel) {
		t.Errorf("textContent.Text does not contain %v", profileAppLabel)
	}

	if !strings.Contains(textContent.Text, "linodes:read_only") {
		t.Errorf("textContent.Text does not contain %v", "linodes:read_only")
	}
}

func TestLinodeProfileAppGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcProfileApps12345 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileApps12345)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeProfileAppGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyAppID: profileAppID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_profile_app_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_profile_app_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeProfileAppGetToolInvalidAppIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: errProfileAppIDRequired},
		{name: caseZero, args: map[string]any{keyAppID: 0}, want: errProfileAppIDPositive},
		{name: caseNegative, args: map[string]any{keyAppID: -1}, want: errProfileAppIDPositive},
		{name: "fractional app_id", args: map[string]any{keyAppID: 1.5}, want: errProfileAppIDPositive},
		{name: caseString, args: map[string]any{keyAppID: "12345"}, want: errProfileAppIDPositive},
		{name: caseSlash, args: map[string]any{keyAppID: invalidProfileIDSlash}, want: errProfileAppIDPositive},
		{name: caseQuery, args: map[string]any{keyAppID: invalidProfileIDQuery}, want: errProfileAppIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyAppID: pathTraversalValue}, want: errProfileAppIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeProfileAppGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

// End-to-end verification of account OAuth client lookup.
func TestLinodeAccountOAuthClientGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)

	if tool.Name != "linode_account_oauth_client_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyClientID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyClientID)
	}
}

func TestLinodeAccountOAuthClientGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountOauthClientsClient123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keySupportTicketID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, "status": statusActive, keySecret: "server-secret",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, oauthClientID) {
		t.Errorf("textContent.Text does not contain %v", oauthClientID)
	}

	if !strings.Contains(textContent.Text, oauthClientLabel) {
		t.Errorf("textContent.Text does not contain %v", oauthClientLabel)
	}

	if strings.Contains(textContent.Text, "server-secret") {
		t.Errorf("textContent.Text should not contain %v", "server-secret")
	}
}

func TestLinodeAccountOAuthClientGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountOauthClientsClient123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_account_oauth_client_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_account_oauth_client_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountOAuthClientGetToolInvalidClientIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: errClientIDRequired},
		{name: caseClientIDEmpty, args: map[string]any{keyClientID: ""}, want: errClientIDNonEmpty},
		{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123}, want: errClientIDNonEmpty},
		{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash}, want: errClientIDNoSeparators},
		{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery}, want: errClientIDNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue}, want: errClientIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

// End-to-end verification of account OAuth client update.
func TestLinodeAccountOAuthClientUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)

	if tool.Name != "linode_account_oauth_client_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyClientID) {
		t.Errorf("RawInputSchema missing key %v", keyClientID)
	}

	if !strings.Contains(rawSchema, keyLabel) {
		t.Errorf("RawInputSchema missing key %v", keyLabel)
	}

	if !strings.Contains(rawSchema, keyRedirectURI) {
		t.Errorf("RawInputSchema missing key %v", keyRedirectURI)
	}

	if !strings.Contains(rawSchema, keyPublic) {
		t.Errorf("RawInputSchema missing key %v", keyPublic)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountOAuthClientUpdateToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, include: true},
		{name: caseString, confirm: boolStringTrue, include: true},
		{name: caseNumeric, confirm: 1, include: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)

			args := map[string]any{keyClientID: oauthClientID, keyLabel: oauthClientLabel}
			if testCase.include {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeAccountOAuthClientUpdateToolInvalidArgsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true, keyLabel: oauthClientLabel}, want: errClientIDRequired},
		{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyLabel: oauthClientLabel, keyConfirm: true}, want: errClientIDNoSeparators},
		{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyLabel: oauthClientLabel, keyConfirm: true}, want: errClientIDNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyLabel: oauthClientLabel, keyConfirm: true}, want: errClientIDNoSeparators},
		{name: "no update fields", args: map[string]any{keyClientID: oauthClientID, keyConfirm: true}, want: "at least one of label, redirect_uri, or public is required"},
		{name: "blank label", args: map[string]any{keyClientID: oauthClientID, keyLabel: blankString, keyConfirm: true}, want: errLabelRequired},
		{name: "blank redirect_uri", args: map[string]any{keyClientID: oauthClientID, keyRedirectURI: blankString, keyConfirm: true}, want: errRedirectURIRequired},
		{name: "string public", args: map[string]any{keyClientID: oauthClientID, keyPublic: boolStringTrue, keyConfirm: true}, want: "public must be a boolean"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	label := oauthClientLabel
	redirectURI := oauthClientRedirectURI
	public := true
	want := linode.UpdateOAuthClientRequest{
		Label:       &label,
		RedirectURI: &redirectURI,
		Public:      &public,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccountOauthClientsClient123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), "application/json")
		}

		var got linode.UpdateOAuthClientRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("request body = %+v, want %+v", got, want)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.OAuthClient{ID: oauthClientID, Label: oauthClientLabel, Public: true, RedirectURI: oauthClientRedirectURI, Status: statusActive}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyPublic: true, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "OAuth client updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "OAuth client updated successfully")
	}

	if !strings.Contains(textContent.Text, oauthClientID) {
		t.Errorf("textContent.Text does not contain %v", oauthClientID)
	}
}

func TestLinodeAccountOAuthClientUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccountOauthClientsClient123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyLabel: oauthClientLabel, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_account_oauth_client_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_account_oauth_client_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account OAuth client thumbnail update.
func TestLinodeAccountOAuthClientThumbnailUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)

	if tool.Name != "linode_account_oauth_client_thumbnail_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_thumbnail_update")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyClientID) {
		t.Errorf("RawInputSchema missing key %v", keyClientID)
	}

	if !strings.Contains(rawSchema, keyThumbnailPNGBase64) {
		t.Errorf("RawInputSchema missing key %v", keyThumbnailPNGBase64)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountOAuthClientThumbnailUpdateToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, include: true},
		{name: caseString, confirm: boolStringTrue, include: true},
		{name: caseNumeric, confirm: 1, include: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)

			args := map[string]any{keyClientID: oauthClientID, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG))}
			if testCase.include {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeAccountOAuthClientThumbnailUpdateToolInvalidClientIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDRequired},
		{name: caseClientIDEmpty, args: map[string]any{keyClientID: "", keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNonEmpty},
		{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNonEmpty},
		{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNoSeparators},
		{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true}, want: errClientIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientThumbnailUpdateToolInvalidThumbnailRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		want  string
	}{
		{name: caseMissing, want: "thumbnail_png_base64 is required"},
		{name: caseString, value: blankString, want: "thumbnail_png_base64 must be a non-empty string"},
		{name: caseNumeric, value: 123, want: "thumbnail_png_base64 must be a non-empty string"},
		{name: "malformed base64", value: "not base64", want: "thumbnail_png_base64 must be valid standard base64"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
			args := map[string]any{keyClientID: oauthClientID, keyConfirm: true}

			if testCase.value != nil {
				args[keyThumbnailPNGBase64] = testCase.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientThumbnailUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccountOauthClientsClient123Thumbnail {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123Thumbnail)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if r.Header.Get("Content-Type") != "image/png" {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), "image/png")
		}

		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, []byte(oauthClientThumbnailPNG)) {
			t.Errorf("got = %v, want %v", got, []byte(oauthClientThumbnailPNG))
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "OAuth client thumbnail updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "OAuth client thumbnail updated successfully")
	}

	if !strings.Contains(textContent.Text, oauthClientID) {
		t.Errorf("textContent.Text does not contain %v", oauthClientID)
	}
}

func TestLinodeAccountOAuthClientThumbnailUpdateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccountOauthClientsClient123Thumbnail {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123Thumbnail)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailUpdateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyThumbnailPNGBase64: base64.StdEncoding.EncodeToString([]byte(oauthClientThumbnailPNG)), keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update linode_account_oauth_client_thumbnail_update") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update linode_account_oauth_client_thumbnail_update")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account OAuth client thumbnail retrieval.
func TestLinodeAccountOAuthClientThumbnailGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)

	if tool.Name != "linode_account_oauth_client_thumbnail_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_thumbnail_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyClientID) {
		t.Errorf("RawInputSchema missing key %v", keyClientID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountOAuthClientThumbnailGetToolInvalidClientIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: errClientIDRequired},
		{name: caseClientIDEmpty, args: map[string]any{keyClientID: ""}, want: errClientIDNonEmpty},
		{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123}, want: errClientIDNonEmpty},
		{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash}, want: errClientIDNoSeparators},
		{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery}, want: errClientIDNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue}, want: errClientIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientThumbnailGetToolSuccess(t *testing.T) {
	t.Parallel()

	thumbnailPNG := []byte("png-bytes")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountOauthClientsClient123Thumbnail {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123Thumbnail)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "image/png")

		_, writeErr := w.Write(thumbnailPNG)
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, oauthClientID) {
		t.Errorf("textContent.Text does not contain %v", oauthClientID)
	}

	if !strings.Contains(textContent.Text, base64.StdEncoding.EncodeToString(thumbnailPNG)) {
		t.Errorf("textContent.Text does not contain %v", base64.StdEncoding.EncodeToString(thumbnailPNG))
	}
}

func TestLinodeAccountOAuthClientThumbnailGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountOauthClientsClient123Thumbnail {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123Thumbnail)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: "Not Found"}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientThumbnailGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to get OAuth client thumbnail") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to get OAuth client thumbnail")
	}
}

// End-to-end verification of account OAuth client deletion.
func TestLinodeAccountOAuthClientDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)

	if tool.Name != "linode_account_oauth_client_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_delete")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyClientID) {
		t.Errorf("RawInputSchema missing key %v", keyClientID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountOAuthClientDeleteToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, include: true},
		{name: caseString, confirm: boolStringTrue, include: true},
		{name: caseNumeric, confirm: 1, include: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)

			args := map[string]any{keyClientID: oauthClientID}
			if testCase.include {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeAccountOAuthClientDeleteToolInvalidClientIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, want: errClientIDRequired},
		{name: caseClientIDEmpty, args: map[string]any{keyClientID: "", keyConfirm: true, keyConfirmedDryRun: true}, want: errClientIDNonEmpty},
		{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123, keyConfirm: true, keyConfirmedDryRun: true}, want: errClientIDNonEmpty},
		{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyConfirm: true, keyConfirmedDryRun: true}, want: errClientIDNoSeparators},
		{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyConfirm: true, keyConfirmedDryRun: true}, want: errClientIDNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errClientIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcAccountOauthClientsClient123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "OAuth client deleted successfully") {
		t.Errorf("textContent.Text does not contain %v", "OAuth client deleted successfully")
	}

	if !strings.Contains(textContent.Text, oauthClientID) {
		t.Errorf("textContent.Text does not contain %v", oauthClientID)
	}
}

func TestLinodeAccountOAuthClientDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcAccountOauthClientsClient123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountOauthClientsClient123)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_account_oauth_client_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_account_oauth_client_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account OAuth client secret reset.
func TestLinodeAccountOAuthClientResetSecretToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)

	if tool.Name != "linode_account_oauth_client_secret_reset" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_secret_reset")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyClientID) {
		t.Errorf("RawInputSchema missing key %v", keyClientID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountOAuthClientResetSecretToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, include: true},
		{name: caseString, confirm: boolStringTrue, include: true},
		{name: caseNumeric, confirm: 1, include: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)

			args := map[string]any{keyClientID: oauthClientID}
			if testCase.include {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeAccountOAuthClientResetSecretToolInvalidClientIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errClientIDRequired},
		{name: caseClientIDEmpty, args: map[string]any{keyClientID: "", keyConfirm: true}, want: errClientIDNonEmpty},
		{name: caseClientIDNumeric, args: map[string]any{keyClientID: 123, keyConfirm: true}, want: errClientIDNonEmpty},
		{name: caseClientIDSlash, args: map[string]any{keyClientID: invalidClientIDSlash, keyConfirm: true}, want: errClientIDNoSeparators},
		{name: caseClientIDQuerySeparator, args: map[string]any{keyClientID: invalidClientIDQuery, keyConfirm: true}, want: errClientIDNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyClientID: pathTraversalValue, keyConfirm: true}, want: errClientIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientResetSecretToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/oauth-clients/client-123/reset-secret" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/client-123/reset-secret")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.OAuthClientSecret{Secret: "new-secret-once"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "OAuth client secret reset successfully") {
		t.Errorf("textContent.Text does not contain %v", "OAuth client secret reset successfully")
	}

	if !strings.Contains(textContent.Text, "IMPORTANT") {
		t.Errorf("textContent.Text does not contain %v", "IMPORTANT")
	}

	if !strings.Contains(textContent.Text, "new-secret-once") {
		t.Errorf("textContent.Text does not contain %v", "new-secret-once")
	}
}

func TestLinodeAccountOAuthClientResetSecretToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/oauth-clients/client-123/reset-secret" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/oauth-clients/client-123/reset-secret")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientResetSecretTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyClientID: oauthClientID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to reset linode_account_oauth_client_secret_reset") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to reset linode_account_oauth_client_secret_reset")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account OAuth client creation.
func TestLinodeAccountOAuthClientCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)

	if tool.Name != "linode_account_oauth_client_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_oauth_client_create")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, monitorAlertDefinitionLabelParam) {
		t.Errorf("RawInputSchema missing key %v", managedServiceLabelParam)
	}

	if !strings.Contains(rawSchema, "redirect_uri") {
		t.Errorf("RawInputSchema missing key %v", "redirect_uri")
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountOAuthClientCreateToolConfirmRequiredBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
		include bool
	}{
		{name: caseMissing},
		{name: caseFalse, confirm: false, include: true},
		{name: "string", confirm: boolStringTrue, include: true},
		{name: "number", confirm: 1, include: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)

			args := map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI}
			if testCase.include {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeAccountOAuthClientCreateToolMissingRequiredArgsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing label", args: map[string]any{keyRedirectURI: oauthClientRedirectURI, keyConfirm: true}, want: errLabelRequired},
		{name: "non-string label", args: map[string]any{keyLabel: 123, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true}, want: errLabelRequired},
		{name: "blank label", args: map[string]any{keyLabel: blankString, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true}, want: errLabelRequired},
		{name: "missing redirect_uri", args: map[string]any{keyLabel: oauthClientLabel, keyConfirm: true}, want: errRedirectURIRequired},
		{name: "non-string redirect_uri", args: map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: 123, keyConfirm: true}, want: errRedirectURIRequired},
		{name: "blank redirect_uri", args: map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: blankString, keyConfirm: true}, want: errRedirectURIRequired},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountOAuthClientCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountOAuthClientsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountOAuthClientsTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("got %v, want %v", r.Header.Get("Content-Type"), "application/json")
		}

		var got linode.CreateOAuthClientRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got.Label != oauthClientLabel {
			t.Errorf("got.Label = %v, want %v", got.Label, oauthClientLabel)
		}

		if got.RedirectURI != oauthClientRedirectURI {
			t.Errorf("got.RedirectURI = %v, want %v", got.RedirectURI, oauthClientRedirectURI)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.CreatedOAuthClient{
			ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Secret: "secret-once",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "OAuth client created successfully") {
		t.Errorf("textContent.Text does not contain %v", "OAuth client created successfully")
	}

	if !strings.Contains(textContent.Text, "IMPORTANT") {
		t.Errorf("textContent.Text does not contain %v", "IMPORTANT")
	}

	if !strings.Contains(textContent.Text, "secret-once") {
		t.Errorf("textContent.Text does not contain %v", "secret-once")
	}
}

func TestLinodeAccountOAuthClientCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountOAuthClientsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountOAuthClientsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountOAuthClientCreateTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_account_oauth_client_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_account_oauth_client_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account invoice listing.
func TestLinodeAccountInvoiceItemsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

	if tool.Name != "linode_account_invoice_item_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_invoice_item_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "invoice_id") {
		t.Errorf("RawInputSchema missing key %v", "invoice_id")
	}

	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountInvoiceItemsToolSuccess(t *testing.T) {
	t.Parallel()

	items := linode.PaginatedResponse[linode.AccountInvoiceItem]{
		Data:    []linode.AccountInvoiceItem{{Label: invoiceItemLabel, Quantity: 1, Total: 5.00, Type: "linode", UnitPrice: 5.00}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/invoices/12345/items" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/invoices/12345/items")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(items); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID, keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, invoiceItemLabel) {
		t.Errorf("textContent.Text does not contain %v", invoiceItemLabel)
	}

	if !strings.Contains(textContent.Text, "5") {
		t.Errorf("textContent.Text does not contain %v", "5")
	}
}

func TestLinodeAccountInvoiceItemsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/invoices/12345/items" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/invoices/12345/items")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInvoiceID: accountInvoiceID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountInvoiceItemsToolInvalidInputsRejectBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing invoice id", args: map[string]any{}, wantMessage: "invoice_id is required"},
		{name: "invoice id string", args: map[string]any{keyInvoiceID: "12345"}, wantMessage: messageInvoiceIDPositive},
		{name: "invoice id fraction", args: map[string]any{keyInvoiceID: 12345.5}, wantMessage: messageInvoiceIDPositive},
		{name: "invoice id separator", args: map[string]any{keyInvoiceID: "12345/items"}, wantMessage: messageInvoiceIDPositive},
		{name: "invoice id query delimiter", args: map[string]any{keyInvoiceID: "12345?items"}, wantMessage: messageInvoiceIDPositive},
		{name: "invoice id traversal", args: map[string]any{keyInvoiceID: pathTraversalValue}, wantMessage: messageInvoiceIDPositive},
		{name: paginationCasePageZero, args: map[string]any{keyInvoiceID: accountInvoiceID, keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyInvoiceID: accountInvoiceID, keyPageSize: 501}, wantMessage: errPageSizeRange},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountInvoiceItemsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

	if tool.Name != "linode_account_payment_method_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_method_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentMethodsToolSuccess(t *testing.T) {
	t.Parallel()

	methods := linode.PaginatedResponse[linode.AccountPaymentMethod]{
		Data:    []linode.AccountPaymentMethod{{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountPaymentMethodsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentMethodsTestPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(methods); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, paymentMethodCreditCard) {
		t.Errorf("textContent.Text does not contain %v", paymentMethodCreditCard)
	}

	if !strings.Contains(textContent.Text, "1111") {
		t.Errorf("textContent.Text does not contain %v", "1111")
	}
}

func TestLinodeAccountPaymentMethodsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountPaymentMethodsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentMethodsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountPaymentMethodsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)

	if tool.Name != "linode_account_payment_method_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_method_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPaymentMethodID) {
		t.Errorf("RawInputSchema missing key %v", keyPaymentMethodID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentMethodGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPaymentMethods123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		// The extra top-level field the proto does not model must be dropped by
		// the DiscardUnknown decode, proving proto-canonical output.
		method := struct {
			linode.AccountPaymentMethod

			NotInProto string `json:"not_in_proto"`
		}{
			AccountPaymentMethod: linode.AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}},
			NotInProto:           valNotInProto,
		}

		if err := json.NewEncoder(w).Encode(method); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, paymentMethodCreditCard) {
		t.Errorf("textContent.Text does not contain %v", paymentMethodCreditCard)
	}

	if !strings.Contains(textContent.Text, paymentMethodLastFour) {
		t.Errorf("textContent.Text does not contain %v", paymentMethodLastFour)
	}

	if strings.Contains(textContent.Text, keyNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeAccountPaymentMethodGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountPaymentMethods123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods123)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_account_payment_method_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_account_payment_method_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountPaymentMethodGetToolInvalidPaymentMethodIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: errPaymentMethodIDRequired},
		{name: caseString, args: map[string]any{keyPaymentMethodID: idAbc123}, want: errPaymentMethodIDInteger},
		{name: caseZero, args: map[string]any{keyPaymentMethodID: float64(0)}, want: errPaymentMethodIDInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodGetTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

	if tool.Name != "linode_account_payment_method_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_method_create")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, keyPaymentType) {
		t.Errorf("tool.RawInputSchema missing key %v", keyPaymentType)
	}

	if !strings.Contains(raw, keyData) {
		t.Errorf("tool.RawInputSchema missing key %v", keyData)
	}

	if !strings.Contains(raw, keyIsDefault) {
		t.Errorf("tool.RawInputSchema missing key %v", keyIsDefault)
	}

	if !strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentMethodCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountPaymentMethodsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentMethodsTestPath)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any

		decodeErr := json.NewDecoder(r.Body).Decode(&body)
		if decodeErr != nil {
			t.Errorf("unexpected error: %v", decodeErr)
		}

		if decodeErr != nil {
			return
		}

		for key, want := range map[string]any{
			keyType:      paymentMethodCreditCard,
			keyIsDefault: true,
			keyData:      map[string]any{keyToken: paymentMethodToken},
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountPaymentMethod{ID: 321, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, paymentMethodCreatedMessage) {
		t.Errorf("textContent.Text does not contain %v", paymentMethodCreatedMessage)
	}

	if !strings.Contains(textContent.Text, paymentMethodLastFour) {
		t.Errorf("textContent.Text does not contain %v", paymentMethodLastFour)
	}

	var envelope map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if _, ok := envelope["payment_method"]; !ok {
		t.Errorf("envelope missing payment_method key, got %v", envelope)
	}

	if _, ok := envelope["method"]; ok {
		t.Errorf("envelope should not carry the legacy method key, got %v", envelope)
	}
}

func TestLinodeAccountPaymentMethodCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountPaymentMethodsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentMethodsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to create linode_account_payment_method_create") {
		t.Errorf("textContent.Text does not contain %v", "Failed to create linode_account_payment_method_create")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountPaymentMethodCreateToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: casePaymentMethodConfirmFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumeric, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)

			args := map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true}
			if testCase.name != "missing" {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
				t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodCreateToolRequiredArgumentValidationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing type", args: map[string]any{keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: true, keyConfirm: true}, want: "type is required"},
		{name: "missing data", args: map[string]any{keyType: paymentMethodCreditCard, keyIsDefault: true, keyConfirm: true}, want: "data is required"},
		{name: "missing is_default", args: map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyConfirm: true}, want: "is_default must be a boolean"},
		{name: "string is_default", args: map[string]any{keyType: paymentMethodCreditCard, keyData: map[string]any{keyToken: paymentMethodToken}, keyIsDefault: boolStringTrue, keyConfirm: true}, want: "is_default must be a boolean"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodCreateTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text does not contain %v", testCase.want)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)

	if tool.Name != "linode_account_payment_method_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_method_delete")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPaymentMethodID) {
		t.Errorf("RawInputSchema missing key %v", keyPaymentMethodID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentMethodDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcAccountPaymentMethods123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods123)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, paymentMethodDeletedMessage) {
		t.Errorf("textContent.Text does not contain %v", paymentMethodDeletedMessage)
	}
}

func TestLinodeAccountPaymentMethodDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcAccountPaymentMethods123 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountPaymentMethods123)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_account_payment_method_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_account_payment_method_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountPaymentMethodDeleteToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: casePaymentMethodConfirmFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumeric, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)

			args := map[string]any{keyPaymentMethodID: paymentMethodID}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodDeleteToolInvalidPaymentMethodIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, want: errPaymentMethodIDRequired},
		{name: caseString, args: map[string]any{keyPaymentMethodID: idAbc123, keyConfirm: true, keyConfirmedDryRun: true}, want: errPaymentMethodIDInteger},
		{name: caseZero, args: map[string]any{keyPaymentMethodID: float64(0), keyConfirm: true, keyConfirmedDryRun: true}, want: errPaymentMethodIDInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodDeleteTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodMakeDefaultToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)

	if tool.Name != "linode_account_payment_method_make_default" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_method_make_default")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPaymentMethodID) {
		t.Errorf("RawInputSchema missing key %v", keyPaymentMethodID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentMethodMakeDefaultToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/payment-methods/123/make-default" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/payment-methods/123/make-default")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Payment method set as default successfully") {
		t.Errorf("textContent.Text does not contain %v", "Payment method set as default successfully")
	}
}

func TestLinodeAccountPaymentMethodMakeDefaultToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/payment-methods/123/make-default" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/payment-methods/123/make-default")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)
	req := createRequestWithArgs(t, map[string]any{keyPaymentMethodID: paymentMethodID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to set linode_account_payment_method_make_default") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to set linode_account_payment_method_make_default")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountPaymentMethodMakeDefaultToolConfirmRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		confirm any
	}{
		{name: caseMissing},
		{name: casePaymentMethodConfirmFalse, confirm: false},
		{name: caseString, confirm: boolStringTrue},
		{name: caseNumeric, confirm: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)

			args := map[string]any{keyPaymentMethodID: paymentMethodID}
			if testCase.name != caseMissing {
				args[keyConfirm] = testCase.confirm
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeAccountPaymentMethodMakeDefaultToolInvalidPaymentMethodIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errPaymentMethodIDRequired},
		{name: caseString, args: map[string]any{keyPaymentMethodID: idAbc123, keyConfirm: true}, want: errPaymentMethodIDInteger},
		{name: caseZero, args: map[string]any{keyPaymentMethodID: float64(0), keyConfirm: true}, want: errPaymentMethodIDInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentMethodMakeDefaultTool(cfg)
			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountPaymentsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentsTool(cfg)

	if tool.Name != "linode_account_payment_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentsToolSuccess(t *testing.T) {
	t.Parallel()

	payments := linode.PaginatedResponse[linode.AccountPayment]{
		Data:    []linode.AccountPayment{{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountPaymentsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentsTestPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(payments); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountPaymentsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "654") {
		t.Errorf("textContent.Text does not contain %v", "654")
	}

	if !strings.Contains(textContent.Text, "20.25") {
		t.Errorf("textContent.Text does not contain %v", "20.25")
	}
}

func TestLinodeAccountPaymentsToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountPaymentsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountPaymentsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountPaymentsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountPaymentsToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentsTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeAccountPaymentGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

	if tool.Name != "linode_account_payment_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_payment_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPaymentID) {
		t.Errorf("rawSchema missing key %v", keyPaymentID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("rawSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountPaymentGetToolSuccess(t *testing.T) {
	t.Parallel()

	payment := linode.AccountPayment{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/payments/654" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/payments/654")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(payment); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPaymentID: 654})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "654") {
		t.Errorf("textContent.Text does not contain %v", "654")
	}

	if !strings.Contains(textContent.Text, "20.25") {
		t.Errorf("textContent.Text does not contain %v", "20.25")
	}
}

func TestLinodeAccountPaymentGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/payments/654" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/payments/654")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPaymentID: 654})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_account_payment_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_account_payment_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountPaymentGetToolInvalidPaymentIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing payment_id", args: map[string]any{}, want: "payment_id is required"},
		{name: "payment_id zero", args: map[string]any{keyPaymentID: 0}, want: errPaymentIDPositive},
		{name: "payment_id fractional", args: map[string]any{keyPaymentID: 1.5}, want: errPaymentIDPositive},
		{name: "payment_id oversized", args: map[string]any{keyPaymentID: 9007199254740992.0}, want: errPaymentIDPositive},
		{name: "payment_id string slash", args: map[string]any{keyPaymentID: pathSeparatorValue}, want: errPaymentIDPositive},
		{name: "payment_id string query", args: map[string]any{keyPaymentID: querySeparatorValue}, want: errPaymentIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyPaymentID: pathTraversalValue}, want: errPaymentIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountPaymentGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountInvoicesToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountInvoicesTool(cfg)

	if tool.Name != "linode_account_invoice_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_invoice_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountInvoicesToolSuccess(t *testing.T) {
	t.Parallel()

	invoices := linode.PaginatedResponse[linode.AccountInvoice]{
		Data:    []linode.AccountInvoice{{ID: 987, Date: "2024-01-31T00:00:00", Label: "Invoice 987", Total: 42.50}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/invoices" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/invoices")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(invoices); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountInvoicesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Invoice 987") {
		t.Errorf("textContent.Text does not contain %v", "Invoice 987")
	}

	if !strings.Contains(textContent.Text, "42.5") {
		t.Errorf("textContent.Text does not contain %v", "42.5")
	}
}

func TestLinodeAccountInvoicesToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/invoices" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/invoices")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountInvoicesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountInvoicesToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountInvoicesTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeAccountServiceTransfersToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

	if tool.Name != "linode_account_service_transfer_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_service_transfer_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyPage) {
		t.Errorf("RawInputSchema missing key %v", keyPage)
	}

	if !strings.Contains(rawSchema, keyPageSize) {
		t.Errorf("RawInputSchema missing key %v", keyPageSize)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountServiceTransfersToolSuccess(t *testing.T) {
	t.Parallel()

	transfers := linode.PaginatedResponse[linode.AccountEntityTransfer]{
		Data: []linode.AccountEntityTransfer{{
			Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
			IsSender: true,
			Status:   statusPending,
			Token:    accountEntityTransferToken,
		}},
		Page:    2,
		Pages:   4,
		Results: 80,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountServiceTransfersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountServiceTransfersTestPath)
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(transfers); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountEntityTransferToken) {
		t.Errorf("textContent.Text does not contain %v", accountEntityTransferToken)
	}

	if !strings.Contains(textContent.Text, "pending") {
		t.Errorf("textContent.Text does not contain %v", "pending")
	}

	if !strings.Contains(textContent.Text, "111") {
		t.Errorf("textContent.Text does not contain %v", "111")
	}
}

func TestLinodeAccountServiceTransfersToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != accountServiceTransfersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountServiceTransfersTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountServiceTransfersToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountServiceTransfersTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

func TestLinodeAccountEventGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountEventGetTool(cfg)

	if tool.Name != "linode_account_event_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_event_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyEventID) {
		t.Errorf("RawInputSchema missing key %v", keyEventID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountEventGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/account/events/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/events/123")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountEvent{
			ID:     accountEventID,
			Action: accountEventAction,
			Status: statusSuccessful,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountEventGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountEventAction) {
		t.Errorf("textContent.Text does not contain %v", accountEventAction)
	}
}

func TestLinodeAccountEventGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountEventGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_event_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_event_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountEventGetToolValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{}, want: errEventIDRequired},
		{name: caseString, args: map[string]any{keyEventID: "123"}, want: errEventIDPositive},
		{name: loginIDCaseZero, args: map[string]any{keyEventID: float64(0)}, want: errEventIDPositive},
		{name: loginIDCaseNegative, args: map[string]any{keyEventID: float64(-1)}, want: errEventIDPositive},
		{name: loginIDCaseFractional, args: map[string]any{keyEventID: 123.5}, want: errEventIDPositive},
		{name: "overflow", args: map[string]any{keyEventID: 1e100}, want: errEventIDPositive},
		{name: caseSlash, args: map[string]any{keyEventID: "12/3"}, want: errEventIDPositive},
		{name: caseQuery, args: map[string]any{keyEventID: "12?3"}, want: errEventIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyEventID: pathTraversalValue}, want: errEventIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountEventGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.want) {
				t.Errorf("textContent.Text does not contain %v", testCase.want)
			}
		})
	}
}

// End-to-end verification of marking an account event as seen.
func TestLinodeAccountEventSeenToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountEventSeenTool(cfg)

	if tool.Name != "linode_account_event_seen" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_event_seen")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyEventID) {
		t.Errorf("RawInputSchema missing key %v", keyEventID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountEventSeenToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

			args := map[string]any{keyEventID: float64(accountEventID)}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "confirm=true") {
				t.Errorf("error text %q does not contain %q", text.Text, "confirm=true")
			}
		})
	}
}

func TestLinodeAccountEventSeenToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/events/123/seen" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/events/123/seen")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID), keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "marked as seen") {
		t.Errorf("textContent.Text does not contain %v", "marked as seen")
	}

	if !strings.Contains(textContent.Text, "123") {
		t.Errorf("textContent.Text does not contain %v", "123")
	}
}

func TestLinodeAccountEventSeenToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEventID: float64(accountEventID), keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to mark linode_account_event_seen") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to mark linode_account_event_seen")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountEventSeenToolValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}, want: errEventIDRequired},
		{name: caseString, args: map[string]any{keyEventID: "123", keyConfirm: true}, want: errEventIDPositive},
		{name: loginIDCaseZero, args: map[string]any{keyEventID: float64(0), keyConfirm: true}, want: errEventIDPositive},
		{name: loginIDCaseNegative, args: map[string]any{keyEventID: float64(-1), keyConfirm: true}, want: errEventIDPositive},
		{name: loginIDCaseFractional, args: map[string]any{keyEventID: 123.5, keyConfirm: true}, want: errEventIDPositive},
		{name: "overflow", args: map[string]any{keyEventID: 1e100, keyConfirm: true}, want: errEventIDPositive},
		{name: caseSlash, args: map[string]any{keyEventID: "12/3", keyConfirm: true}, want: errEventIDPositive},
		{name: caseQuery, args: map[string]any{keyEventID: "12?3", keyConfirm: true}, want: errEventIDPositive},
		{name: caseDotTraversal, args: map[string]any{keyEventID: pathTraversalValue, keyConfirm: true}, want: errEventIDPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountEventSeenTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.want) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.want)
			}
		})
	}
}

func TestLinodeAccountServiceTransferGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

	if tool.Name != "linode_account_service_transfer_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_service_transfer_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyToken) {
		t.Errorf("RawInputSchema missing key %v", keyToken)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountServiceTransferGetToolSuccess(t *testing.T) {
	t.Parallel()

	transfer := linode.AccountEntityTransfer{
		Entities: linode.AccountEntityTransferEntities{Linodes: []int{111, 222}},
		IsSender: true,
		Status:   statusPending,
		Token:    accountServiceTransferToken,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(transfer); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, accountServiceTransferToken) {
		t.Errorf("textContent.Text does not contain %v", accountServiceTransferToken)
	}

	if !strings.Contains(textContent.Text, "pending") {
		t.Errorf("textContent.Text does not contain %v", "pending")
	}

	if !strings.Contains(textContent.Text, "111") {
		t.Errorf("textContent.Text does not contain %v", "111")
	}
}

func TestLinodeAccountServiceTransferGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve linode_account_service_transfer_get") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve linode_account_service_transfer_get")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountServiceTransferGetToolInvalidTokenRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissing, args: map[string]any{}, wantMessage: errTokenRequired},
		{name: caseEmpty, args: map[string]any{keyToken: ""}, wantMessage: errTokenNonEmpty},
		{name: caseString, args: map[string]any{keyToken: 123}, wantMessage: errTokenNonEmpty},
		{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash}, wantMessage: errTokenNoSeparators},
		{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery}, wantMessage: errTokenNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue}, wantMessage: errTokenNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountServiceTransferGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account service transfer creation.
func TestLinodeAccountServiceTransferCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

	if tool.Name != "linode_account_service_transfer_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_service_transfer_create")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	raw := string(tool.RawInputSchema)
	if !strings.Contains(raw, keyLinodeIDs) {
		t.Errorf("tool.RawInputSchema missing key %v", keyLinodeIDs)
	}

	if !strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountServiceTransferCreateToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

			args := map[string]any{keyLinodeIDs: []any{float64(123)}}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountServiceTransferCreateToolInvalidLinodeIdsRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing linode_ids", args: map[string]any{keyConfirm: true}, wantMessage: "linode_ids is required"},
		{name: "empty linode_ids", args: map[string]any{keyLinodeIDs: []any{}, keyConfirm: true}, wantMessage: "linode_ids must include at least one ID"},
		{name: "string linode_ids", args: map[string]any{keyLinodeIDs: "123", keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		{name: "string element", args: map[string]any{keyLinodeIDs: []any{"123"}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		{name: "zero element", args: map[string]any{keyLinodeIDs: []any{float64(0)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		{name: "negative element", args: map[string]any{keyLinodeIDs: []any{float64(-1)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		{name: "fractional element", args: map[string]any{keyLinodeIDs: []any{1.5}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
		{name: "overflow element", args: map[string]any{keyLinodeIDs: []any{float64(1 << 63)}, keyConfirm: true}, wantMessage: errLinodeIDsPositiveArray},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

			req := createRequestWithArgs(t, tt.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountServiceTransferCreateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountServiceTransfersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountServiceTransfersTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var got linode.CreateAccountServiceTransferRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got.Entities.Linodes, []int{123, 456}) {
			t.Errorf("got.Entities.Linodes = %v, want %v", got.Entities.Linodes, []int{123, 456})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.AccountEntityTransfer{
			Entities: linode.AccountEntityTransferEntities{Linodes: []int{123, 456}},
			Status:   statusPending,
			Token:    "service-transfer-token",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeIDs: []any{float64(123), float64(456)}, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "service-transfer-token") {
		t.Errorf("textContent.Text does not contain %v", "service-transfer-token")
	}

	if !strings.Contains(textContent.Text, "Account service transfer created successfully") {
		t.Errorf("textContent.Text does not contain the success message")
	}
}

func TestLinodeAccountServiceTransferCreateToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountServiceTransfersTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountServiceTransfersTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyLinodeIDs: []any{float64(123)}, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to create linode_account_service_transfer_create") {
		t.Errorf("textContent.Text does not contain %v", "Failed to create linode_account_service_transfer_create")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountServiceTransferAcceptToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

	if tool.Name != "linode_account_service_transfer_accept" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_service_transfer_accept")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyToken) {
		t.Errorf("RawInputSchema missing key %v", keyToken)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountServiceTransferAcceptToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

			args := map[string]any{keyToken: accountServiceTransferToken}
			if testCase.set {
				args[keyConfirm] = testCase.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountServiceTransferAcceptToolInvalidTokenRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true}, wantMessage: errTokenRequired},
		{name: caseEmpty, args: map[string]any{keyToken: "", keyConfirm: true}, wantMessage: errTokenNonEmpty},
		{name: caseString, args: map[string]any{keyToken: 123, keyConfirm: true}, wantMessage: errTokenNonEmpty},
		{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash, keyConfirm: true}, wantMessage: errTokenNoSeparators},
		{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery, keyConfirm: true}, wantMessage: errTokenNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue, keyConfirm: true}, wantMessage: errTokenNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountServiceTransferAcceptToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/service-transfers/service-token-example/accept" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/service-transfers/service-token-example/accept")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, accountServiceTransferToken) {
		t.Errorf("error text %q does not contain %q", text.Text, accountServiceTransferToken)
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "accepted successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "accepted successfully")
	}
}

func TestLinodeAccountServiceTransferAcceptToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/service-transfers/service-token-example/accept" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/service-transfers/service-token-example/accept")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to accept linode_account_service_transfer_accept") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to accept linode_account_service_transfer_accept")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account service transfer cancellation.
func TestLinodeAccountServiceTransferDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

	if tool.Name != "linode_account_service_transfer_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_service_transfer_delete")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyToken) {
		t.Errorf("RawInputSchema missing key %v", keyToken)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountServiceTransferDeleteToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

			args := map[string]any{keyToken: accountServiceTransferToken}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountServiceTransferDeleteToolInvalidTokenRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissing, args: map[string]any{keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errTokenRequired},
		{name: caseEmpty, args: map[string]any{keyToken: "", keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errTokenNonEmpty},
		{name: caseString, args: map[string]any{keyToken: 123, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errTokenNonEmpty},
		{name: caseSlash, args: map[string]any{keyToken: accountEntityTransferTokenSlash, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errTokenNoSeparators},
		{name: caseQuery, args: map[string]any{keyToken: accountEntityTransferTokenQuery, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errTokenNoSeparators},
		{name: caseDotTraversal, args: map[string]any{keyToken: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, wantMessage: errTokenNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountServiceTransferDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, accountServiceTransferToken) {
		t.Errorf("error text %q does not contain %q", text.Text, accountServiceTransferToken)
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "canceled successfully") {
		t.Errorf("error text %q does not contain %q", text.Text, "canceled successfully")
	}
}

func TestLinodeAccountServiceTransferDeleteToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcAccountServiceTransfersServiceTokenExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountServiceTransfersServiceTokenExample)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyToken: accountServiceTransferToken, keyConfirm: true, keyConfirmedDryRun: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to delete linode_account_service_transfer_delete") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to delete linode_account_service_transfer_delete")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of enrolled account beta program retrieval.
func TestLinodeAccountBetaGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountBetaGetTool(cfg)

	if tool.Name != "linode_account_beta_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_beta_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyBetaIDPath) {
		t.Errorf("RawInputSchema missing key %v", keyBetaIDPath)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema has unexpected key %v", keyConfirm)
	}
}

func TestLinodeAccountBetaGetToolSuccess(t *testing.T) {
	t.Parallel()

	description := "This is an open public beta for an example feature."
	beta := linode.AccountBetaProgram{
		Description: &description,
		Ended:       nil,
		Enrolled:    "2023-09-11T00:00:00",
		ID:          betaExampleOpen,
		Label:       labelExampleOpenBeta,
		Started:     "2023-07-11T00:00:00",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/betas/"+betaExampleOpen)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(beta); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountBetaGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyBetaIDPath: betaExampleOpen})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, betaExampleOpen) {
		t.Errorf("textContent.Text does not contain %v", betaExampleOpen)
	}

	if !strings.Contains(textContent.Text, labelExampleOpenBeta) {
		t.Errorf("textContent.Text does not contain %v", labelExampleOpenBeta)
	}
}

func TestLinodeAccountBetaGetToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/betas/"+betaExampleOpen {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/betas/"+betaExampleOpen)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountBetaGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyBetaIDPath: betaExampleOpen})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve linode_account_beta_get") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve linode_account_beta_get")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

func TestLinodeAccountBetaGetToolInvalidIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingConfirm, args: map[string]any{}, wantMessage: errBetaIDRequired},
		{name: caseEmpty, args: map[string]any{keyBetaIDPath: ""}, wantMessage: errBetaIDNonEmpty},
		{name: caseBlank, args: map[string]any{keyBetaIDPath: blankString}, wantMessage: errBetaIDNonEmpty},
		{name: caseNumeric, args: map[string]any{keyBetaIDPath: 123}, wantMessage: errBetaIDNonEmpty},
		{name: caseSlash, args: map[string]any{keyBetaIDPath: invalidBetaIDSlash}, wantMessage: errBetaIDChars},
		{name: caseQuery, args: map[string]any{keyBetaIDPath: invalidBetaIDQuery}, wantMessage: errBetaIDChars},
		{name: caseDotTraversal, args: map[string]any{keyBetaIDPath: pathTraversalValue}, wantMessage: errBetaIDChars},
		{name: caseWhitespacePadded, args: map[string]any{keyBetaIDPath: invalidBetaIDPadded}, wantMessage: errBetaIDChars},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountBetaGetTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of child account proxy user token creation.
func TestLinodeAccountChildAccountTokenToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

	if tool.Name != "linode_account_child_account_token_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_child_account_token_create")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyEUUID) {
		t.Errorf("RawInputSchema missing key %v", keyEUUID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountChildAccountTokenToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

			args := map[string]any{keyEUUID: childAccountEUUID}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountChildAccountTokenToolInvalidEuuidRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: "missing euuid", args: map[string]any{keyConfirm: true}, wantMessage: "euuid is required"},
		{name: "empty euuid", args: map[string]any{keyEUUID: "", keyConfirm: true}, wantMessage: errEUUIDNonEmpty},
		{name: "numeric euuid", args: map[string]any{keyEUUID: 123, keyConfirm: true}, wantMessage: errEUUIDNonEmpty},
		{name: "euuid with slash", args: map[string]any{keyEUUID: "child/account", keyConfirm: true}, wantMessage: errEUUIDNoSeparators},
		{name: "euuid with query separator", args: map[string]any{keyEUUID: "child?account", keyConfirm: true}, wantMessage: errEUUIDNoSeparators},
		{name: "euuid with traversal", args: map[string]any{keyEUUID: pathTraversalValue, keyConfirm: true}, wantMessage: errEUUIDNoSeparators},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountChildAccountTokenToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.ProxyUserToken{ID: 918, Label: "proxy-token", Scopes: "*", Token: "abcdefghijklmnop", Expiry: "2024-05-01T00:16:01"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "abcdefghijklmnop") {
		t.Errorf("error text %q does not contain %q", text.Text, "abcdefghijklmnop")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "proxy-token") {
		t.Errorf("error text %q does not contain %q", text.Text, "proxy-token")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Child account proxy token created successfully") {
		t.Errorf("text %q does not contain the success message", text.Text)
	}
}

func TestLinodeAccountChildAccountTokenToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/child-accounts/A1BC2DEF-34GH-567I-J890KLMN12O34P56/token")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountChildAccountTokenTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEUUID: childAccountEUUID, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create linode_account_child_account_token_create") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create linode_account_child_account_token_create")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account beta enrollment.
func TestLinodeAccountBetaEnrollToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

	if tool.Name != "linode_account_beta_enroll" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_beta_enroll")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyBetaID) {
		t.Errorf("RawInputSchema missing key %v", keyBetaID)
	}

	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountBetaEnrollToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

			args := map[string]any{keyBetaID: betaExampleOpen}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountBetaEnrollToolInvalidIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyConfirm: true}, wantMessage: errBetaIDRequired},
		{name: caseEmpty, args: map[string]any{keyBetaID: "", keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
		{name: caseBlank, args: map[string]any{keyBetaID: blankString, keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
		{name: caseNumeric, args: map[string]any{keyBetaID: 123, keyConfirm: true}, wantMessage: errBetaIDNonEmpty},
		{name: caseSlash, args: map[string]any{keyBetaID: invalidBetaIDSlash, keyConfirm: true}, wantMessage: errBetaIDChars},
		{name: caseQuery, args: map[string]any{keyBetaID: invalidBetaIDQuery, keyConfirm: true}, wantMessage: errBetaIDChars},
		{name: caseDotTraversal, args: map[string]any{keyBetaID: pathTraversalValue, keyConfirm: true}, wantMessage: errBetaIDChars},
		{name: caseWhitespacePadded, args: map[string]any{keyBetaID: invalidBetaIDPadded, keyConfirm: true}, wantMessage: errBetaIDChars},
		{name: "control", args: map[string]any{keyBetaID: "example\nopen", keyConfirm: true}, wantMessage: errBetaIDChars},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.wantMessage) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.wantMessage)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountBetaEnrollToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountBetasTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountBetasTestPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyBetaID], betaExampleOpen) {
			t.Errorf("body[keyBetaID] = %v, want %v", body[keyBetaID], betaExampleOpen)
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Account beta enrollment requested successfully") {
		t.Errorf("textContent.Text does not contain %v", "Account beta enrollment requested successfully")
	}

	if !strings.Contains(textContent.Text, betaExampleOpen) {
		t.Errorf("textContent.Text does not contain %v", betaExampleOpen)
	}
}

func TestLinodeAccountBetaEnrollToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountBetasTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountBetasTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountBetaEnrollTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyBetaID: betaExampleOpen, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to enroll linode_account_beta_enroll") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to enroll linode_account_beta_enroll")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account availability retrieval.
func TestLinodeAccountAvailabilityToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

	if tool.Name != "linode_account_availability_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_availability_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeAccountAvailabilityToolSuccess(t *testing.T) {
	t.Parallel()

	availability := linode.PaginatedResponse[linode.AccountAvailability]{
		Data: []linode.AccountAvailability{{
			Available:   []string{serviceLinodes, serviceNodeBalancers},
			Region:      regionUSEast,
			Unavailable: []string{"Kubernetes", serviceBlockStorage},
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(availability); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}

	if !strings.Contains(textContent.Text, serviceLinodes) {
		t.Errorf("textContent.Text does not contain %v", serviceLinodes)
	}
}

func TestLinodeAccountAvailabilityToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/account/availability" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/availability")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountAvailabilityToolInvalidPaginationRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
		{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
		{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
		{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
		{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{}
			_, _, handler := tools.NewLinodeAccountAvailabilityTool(cfg)

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.wantMessage) {
				t.Errorf("textContent.Text does not contain %v", testCase.wantMessage)
			}
		})
	}
}

// End-to-end verification of account agreement acknowledgement.
func TestLinodeAccountAgreementsAcknowledgeToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

	if tool.Name != "linode_account_agreement_acknowledge" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_agreement_acknowledge")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}

	if !strings.Contains(rawSchema, "billing_agreement") {
		t.Errorf("RawInputSchema missing key %v", "billing_agreement")
	}
}

func TestLinodeAccountAgreementsAcknowledgeToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

			args := map[string]any{keyPrivacyPolicy: true}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountAgreementsAcknowledgeToolEmptyAcknowledgementRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "at least one account agreement field is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "at least one account agreement field is required")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountAgreementsAcknowledgeToolFalseAgreementRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPrivacyPolicy: false, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "privacy_policy must be true when provided") {
		t.Errorf("error text %q does not contain %q", text.Text, "privacy_policy must be true when provided")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountAgreementsAcknowledgeToolMalformedFieldRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPrivacyPolicy: boolStringTrue, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "privacy_policy must be a boolean") {
		t.Errorf("error text %q does not contain %q", text.Text, "privacy_policy must be a boolean")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountAgreementsAcknowledgeToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != accountAgreementsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountAgreementsTestPath)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["billing_agreement"], true) {
			t.Errorf("got %v, want %v", body["billing_agreement"], true)
		}

		if !reflect.DeepEqual(body[keyPrivacyPolicy], true) {
			t.Errorf("body[keyPrivacyPolicy] = %v, want %v", body[keyPrivacyPolicy], true)
		}

		if _, ok := body["eu_model"]; ok {
			t.Errorf("body has unexpected key %v", "eu_model")
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountAgreementsAcknowledgeTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"billing_agreement": true, keyPrivacyPolicy: true, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Account agreements acknowledged successfully") {
		t.Errorf("textContent.Text does not contain %v", "Account agreements acknowledged successfully")
	}
}

// End-to-end verification of region listing and filtering.
func TestLinodeRegionsListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeRegionListTool(cfg)

	if tool.Name != "linode_region_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_region_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeRegionsListToolSuccess(t *testing.T) {
	t.Parallel()

	regions := []linode.Region{
		{ID: regionUSEast, Label: regionLabelNewark, Country: countryUS, Capabilities: []string{"Linodes", serviceBlockStorage}, Status: statusOK},
		{ID: regionEUWest, Label: "London, UK", Country: "uk", Capabilities: []string{"Linodes"}, Status: statusOK},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/regions" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/regions")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    regions,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeRegionListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}

	if !strings.Contains(textContent.Text, regionEUWest) {
		t.Errorf("textContent.Text does not contain %v", regionEUWest)
	}
}

func TestLinodeRegionsListToolFilterByCountry(t *testing.T) {
	t.Parallel()

	regions := []linode.Region{
		{ID: regionUSEast, Label: regionLabelNewark, Country: countryUS, Status: statusOK},
		{ID: regionUSWest, Label: "Fremont, CA", Country: countryUS, Status: statusOK},
		{ID: regionEUWest, Label: "London, UK", Country: "uk", Status: statusOK},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    regions,
			keyPage:    1,
			keyPages:   1,
			keyResults: 3,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeRegionListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"country": countryUS})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if got := listResponseCount(t, textContent.Text); got != 2 {
		t.Errorf("listResponseCount = %d, want %d", got, 2)
	}

	if !strings.Contains(textContent.Text, regionUSEast) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast)
	}

	if !strings.Contains(textContent.Text, regionUSWest) {
		t.Errorf("textContent.Text does not contain %v", regionUSWest)
	}

	if strings.Contains(textContent.Text, regionEUWest) {
		t.Errorf("textContent.Text should not contain %v", regionEUWest)
	}
}

// End-to-end verification of type listing and filtering.
func TestLinodeTypesListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeTypeListTool(cfg)

	if tool.Name != "linode_type_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_type_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeTypesListToolSuccess(t *testing.T) {
	t.Parallel()

	types := []linode.InstanceType{
		{ID: typeG6Nanode1, Label: invoiceItemLabel, Class: "nanode", Disk: 25600, Memory: 1024, VCPUs: 1},
		{ID: typeG6Standard2, Label: typeLinode4GB, Class: classStandard, Disk: 81920, Memory: 4096, VCPUs: 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/linode/types" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/types")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    types,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, typeG6Nanode1) {
		t.Errorf("textContent.Text does not contain %v", typeG6Nanode1)
	}

	if !strings.Contains(textContent.Text, typeG6Standard2) {
		t.Errorf("textContent.Text does not contain %v", typeG6Standard2)
	}

	if !strings.Contains(textContent.Text, `"types"`) {
		t.Errorf("textContent.Text does not contain the types key: %s", textContent.Text)
	}

	if count := listResponseCount(t, textContent.Text); count != 2 {
		t.Errorf("listResponseCount = %d, want 2", count)
	}
}

func TestLinodeTypesListToolFilterByClass(t *testing.T) {
	t.Parallel()

	types := []linode.InstanceType{
		{ID: typeG6Nanode1, Label: invoiceItemLabel, Class: "nanode"},
		{ID: typeG6Standard2, Label: typeLinode4GB, Class: classStandard},
		{ID: "g6-standard-4", Label: "Linode 8GB", Class: classStandard},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    types,
			keyPage:    1,
			keyPages:   1,
			keyResults: 3,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"class": classStandard})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if count := listResponseCount(t, textContent.Text); count != 2 {
		t.Errorf("listResponseCount = %d, want 2", count)
	}

	if strings.Contains(textContent.Text, typeG6Nanode1) {
		t.Errorf("textContent.Text should not contain %v", typeG6Nanode1)
	}
}

// End-to-end verification of volume type listing.
func TestLinodeVolumeTypesListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeVolumeTypeListTool(cfg)

	if tool.Name != "linode_volume_type_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_type_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVolumeTypesListToolSuccess(t *testing.T) {
	t.Parallel()

	volumeTypes := []linode.VolumeType{{
		keyBetaID: "storage",
		keyLabel:  "Block Storage",
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/volumes/types" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/types")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumeTypes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeVolumeTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Block Storage") {
		t.Errorf("textContent.Text does not contain %v", "Block Storage")
	}

	if !strings.Contains(textContent.Text, `"volume_types"`) {
		t.Errorf("textContent.Text does not contain the volume_types key: %s", textContent.Text)
	}

	if count := listResponseCount(t, textContent.Text); count != 1 {
		t.Errorf("listResponseCount = %d, want 1", count)
	}
}

func TestLinodeVolumeTypesListToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/volumes/types" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes/types")
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeVolumeTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of volume listing and filtering.
func TestLinodeVolumesListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeVolumeListTool(cfg)

	if tool.Name != "linode_volume_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_volume_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeVolumesListToolSuccess(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 1, Label: labelDataVol, Status: statusActive, Size: 100, Region: regionUSEast},
		{ID: 2, Label: labelBackupVol, Status: statusActive, Size: 50, Region: regionEUWest},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/volumes" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/volumes")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeVolumeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, labelDataVol) {
		t.Errorf("textContent.Text does not contain %v", labelDataVol)
	}

	if !strings.Contains(textContent.Text, labelBackupVol) {
		t.Errorf("textContent.Text does not contain %v", labelBackupVol)
	}
}

func TestLinodeVolumesListToolFilterByRegion(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 1, Label: labelDataVol, Region: regionUSEast},
		{ID: 2, Label: labelBackupVol, Region: regionEUWest},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeVolumeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}

	if !strings.Contains(textContent.Text, labelDataVol) {
		t.Errorf("textContent.Text does not contain %v", labelDataVol)
	}

	if strings.Contains(textContent.Text, labelBackupVol) {
		t.Errorf("textContent.Text should not contain %v", labelBackupVol)
	}
}

func TestLinodeVolumesListToolFilterByLabel(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 1, Label: labelDataVol, Region: regionUSEast},
		{ID: 2, Label: labelBackupVol, Region: regionEUWest},
		{ID: 3, Label: "data-backup", Region: regionUSWest},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    volumes,
			keyPage:    1,
			keyPages:   1,
			keyResults: 3,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeVolumeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label_contains": "backup"})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if got := listResponseCount(t, textContent.Text); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}

	if !strings.Contains(textContent.Text, labelBackupVol) {
		t.Errorf("textContent.Text does not contain %v", labelBackupVol)
	}

	if !strings.Contains(textContent.Text, "data-backup") {
		t.Errorf("textContent.Text does not contain %v", "data-backup")
	}
}

// End-to-end verification of image listing and filtering.
func TestLinodeImagesListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeImageListTool(cfg)

	if tool.Name != "linode_image_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImagesListToolSuccess(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Type: typeManualImage, IsPublic: true, Deprecated: false},
		{ID: privateImage12345Fixture, Label: "Custom Image", Type: typeManualImage, IsPublic: false, Deprecated: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/images" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, imageIDUbuntu2204) {
		t.Errorf("textContent.Text does not contain %v", imageIDUbuntu2204)
	}

	if !strings.Contains(textContent.Text, privateImage12345Fixture) {
		t.Errorf("textContent.Text does not contain %v", privateImage12345Fixture)
	}
}

func TestLinodeImagesListToolFilterByPublic(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: imageIDUbuntu2204, Label: imageUbuntu2204, IsPublic: true},
		{ID: privateImage12345Fixture, Label: "Custom Image", IsPublic: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"is_public": "false"})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if count := listResponseCount(t, textContent.Text); count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	if !strings.Contains(textContent.Text, privateImage12345Fixture) {
		t.Errorf("textContent.Text does not contain %v", privateImage12345Fixture)
	}

	if strings.Contains(textContent.Text, imageIDUbuntu2204) {
		t.Errorf("textContent.Text should not contain %v", imageIDUbuntu2204)
	}
}

func TestLinodeImagesListToolFilterByDeprecated(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Deprecated: false},
		{ID: "linode/ubuntu18.04", Label: "Ubuntu 18.04", Deprecated: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    images,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"deprecated": boolStringTrue})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if count := listResponseCount(t, textContent.Text); count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	if !strings.Contains(textContent.Text, "linode/ubuntu18.04") {
		t.Errorf("textContent.Text does not contain %v", "linode/ubuntu18.04")
	}

	if strings.Contains(textContent.Text, imageIDUbuntu2204) {
		t.Errorf("textContent.Text should not contain %v", imageIDUbuntu2204)
	}
}

func TestLinodeImageShareGroupTokensListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

	if tool.Name != "linode_image_sharegroup_token_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_token_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	raw := string(tool.RawInputSchema)
	for _, key := range []string{keyPage, keyPageSize} {
		if !strings.Contains(raw, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if strings.Contains(raw, keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupTokensListToolSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-04T11:09:09"
	expiry := "2025-09-04T10:09:09"
	tokens := []linode.ImageShareGroupToken{
		{
			TokenUUID:              "13428362-5458-4dad-b14b-8d0d4d648f8c",
			Status:                 statusActive,
			Label:                  "Backend Services - Engineering",
			Created:                imageShareGroupTokenCreated,
			Updated:                &updated,
			Expiry:                 &expiry,
			ValidForShareGroupUUID: "e1d0e58b-f89f-4237-84ab-b82077342359",
			ShareGroupUUID:         "e1d0e58b-f89f-4237-84ab-b82077342359",
			ShareGroupLabel:        shareGroupLabelFixture,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/tokens" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    tokens,
			keyPage:    2,
			keyPages:   3,
			keyResults: 7,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want 1", got)
	}

	if !strings.Contains(textContent.Text, "Backend Services - Engineering") {
		t.Errorf("textContent.Text does not contain %v", "Backend Services - Engineering")
	}

	if !strings.Contains(textContent.Text, "13428362-5458-4dad-b14b-8d0d4d648f8c") {
		t.Errorf("textContent.Text does not contain %v", "13428362-5458-4dad-b14b-8d0d4d648f8c")
	}
}

func TestLinodeImageShareGroupTokensListToolInvalidPagination(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPageSize: 24})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, errPageSizeRange) {
		t.Errorf("textContent.Text does not contain %v", errPageSizeRange)
	}
}

func TestLinodeImageShareGroupTokensListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "temporary failure"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageShareGroupTokensListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, "temporary failure") {
		t.Errorf("textContent.Text does not contain %v", "temporary failure")
	}
}

func TestLinodeImageShareGroupsListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

	if tool.Name != "linode_image_sharegroup_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyPage, keyPageSize} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("tool.RawInputSchema missing key %v", key)
		}
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupsListToolSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescription
	updated := shareGroupUpdated
	shareGroups := []linode.ImageShareGroup{
		{
			ID:           1,
			UUID:         shareGroupUUIDExample,
			Label:        shareGroupLabelFixture,
			Description:  &description,
			IsSuspended:  false,
			Created:      shareGroupCreated,
			Updated:      &updated,
			ImagesCount:  2,
			MembersCount: 3,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups")
		}

		if r.URL.RawQuery != longviewSubscriptionsToolQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsToolQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    shareGroups,
			keyPage:    2,
			keyPages:   3,
			keyResults: 7,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want 1", got)
	}

	if !strings.Contains(textContent.Text, shareGroupLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", shareGroupLabelFixture)
	}

	if !strings.Contains(textContent.Text, shareGroupUUIDExample) {
		t.Errorf("textContent.Text does not contain %v", shareGroupUUIDExample)
	}
}

func TestLinodeImageShareGroupsListToolInvalidPagination(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyPageSize: 24})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, errPageSizeRange) {
		t.Errorf("textContent.Text does not contain %v", errPageSizeRange)
	}
}

func TestLinodeImageShareGroupsListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "temporary failure"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeImageShareGroupsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}

	if !strings.Contains(textContent.Text, "temporary failure") {
		t.Errorf("textContent.Text does not contain %v", "temporary failure")
	}
}

// createRequestWithArgs builds a CallToolRequest with the given arguments.
func createRequestWithArgs(t *testing.T, args map[string]any) mcp.CallToolRequest {
	t.Helper()

	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// listResponseCount parses a proto-list tool response and returns its count
// field. Proto-backed list tools serialize through protojson, which varies the
// whitespace after the colon between runs, so the count must be read from the
// decoded JSON rather than matched as a raw substring.
func listResponseCount(t *testing.T, text string) int {
	t.Helper()

	var decoded struct {
		Count int `json:"count"`
	}

	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}

	return decoded.Count
}

// End-to-end verification of account cancellation.
func TestLinodeAccountCancelToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountCancelTool(cfg)

	if tool.Name != "linode_account_cancel" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_cancel")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}

	if !strings.Contains(rawSchema, keyComments) {
		t.Errorf("RawInputSchema missing key %v", keyComments)
	}
}

func TestLinodeAccountCancelToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

			args := map[string]any{keyComments: "leaving"}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountCancelToolMalformedCommentsRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyComments: 123, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "comments must be a string") {
		t.Errorf("error text %q does not contain %q", text.Text, "comments must be a string")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountCancelToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/cancel" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/cancel")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyComments], "leaving") {
			t.Errorf("body[keyComments] = %v, want %v", body[keyComments], "leaving")
		}

		w.Header().Set("Content-Type", "application/json")

		_, writeErr := w.Write([]byte(`{"survey_link":"https://example.test/survey"}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyComments: "leaving", keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Account canceled successfully") {
		t.Errorf("textContent.Text does not contain %v", "Account canceled successfully")
	}

	if !strings.Contains(textContent.Text, "https://example.test/survey") {
		t.Errorf("textContent.Text does not contain %v", "https://example.test/survey")
	}
}

func TestLinodeAccountCancelToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/cancel" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/cancel")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"could not charge card"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountCancelTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to cancel account") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to cancel account")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "could not charge card") {
		t.Errorf("error text %q does not contain %q", text.Text, "could not charge card")
	}
}

// End-to-end verification of account update.
func TestLinodeAccountUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountUpdateTool(cfg)

	if tool.Name != "linode_account_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}

	if !strings.Contains(rawSchema, "email") {
		t.Errorf("RawInputSchema missing key %v", "email")
	}
}

func TestLinodeAccountUpdateToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)

				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			cfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {
						Label:  envLabelDefault,
						Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
					},
				},
			}
			_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

			args := map[string]any{keyEmail: emailUpdatedExample}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountUpdateToolEmptyUpdateRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "at least one account field is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "at least one account field is required")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountUpdateToolMalformedFieldRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEmail: 123, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "email must be a string") {
		t.Errorf("error text %q does not contain %q", text.Text, "email must be a string")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountUpdateToolApiErrorProducesToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccount {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccount)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{"field": "email", keyReason: "invalid email format"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEmail: emailUpdatedExample, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update account") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update account")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "invalid email format") {
		t.Errorf("error text %q does not contain %q", text.Text, "invalid email format")
	}
}

func TestLinodeAccountUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	account := linode.Account{FirstName: nameUpdatedTest, LastName: "User", Email: emailUpdatedExample}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcAccount {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccount)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["email"], emailUpdatedExample) {
			t.Errorf("got %v, want %v", body["email"], emailUpdatedExample)
		}

		if !reflect.DeepEqual(body["first_name"], nameUpdatedTest) {
			t.Errorf("got %v, want %v", body["first_name"], nameUpdatedTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(account); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyEmail: emailUpdatedExample, "first_name": nameUpdatedTest, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Account updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "Account updated successfully")
	}

	if !strings.Contains(textContent.Text, emailUpdatedExample) {
		t.Errorf("textContent.Text does not contain %v", emailUpdatedExample)
	}
}

// TestLinodeAccountUpdateToolDryRun covers the Phase 1 dry-run path
// on the Admin-tier reference tool. Kept as a sibling function (not a
// subtest of TestLinodeAccountUpdateTool) so the parent function's
// maintidx stays under the per-function threshold.
func TestLinodeAccountUpdateToolDryRunSchemaProperty(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, _, _ := tools.NewLinodeAccountUpdateTool(cfg)

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, "dry_run") {
		t.Errorf("RawInputSchema missing key %v", "dry_run")
	}
}

func TestLinodeAccountUpdateToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	accountBody := `{"company":"Acme Corp","email":"ops@acme.example","first_name":"Pat","last_name":"Lee","phone":"+1-555-0100","city":"Springfield","country":"US"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != tcAccount {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccount)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(accountBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, dryRunHandler := tools.NewLinodeAccountUpdateTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyDryRun: true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_account_update") {
		t.Errorf("got %v, want %v", body["tool"], "linode_account_update")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "PUT") {
		t.Errorf("got %v, want %v", would["method"], "PUT")
	}

	if !reflect.DeepEqual(would["path"], tcAccount) {
		t.Errorf("got %v, want %v", would["path"], tcAccount)
	}

	state, stateIsObject := body["current_state"].(map[string]any)
	if !stateIsObject {
		t.Fatal("stateIsObject = false, want true")
	}

	if !reflect.DeepEqual(state["company"], "Acme Corp") {
		t.Errorf("got %v, want %v", state["company"], "Acme Corp")
	}

	if !reflect.DeepEqual(state["phone"], "+1-555-0100") {
		t.Errorf("got %v, want %v", state["phone"], "+1-555-0100")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeAccountUpdateToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"company":"NoConfirm Inc","email":"x@example.com"}`))
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, dryRunHandler := tools.NewLinodeAccountUpdateTool(dryRunCfg)

	// Intentionally omit confirm; the dry-run path must not gate on it.
	req := createRequestWithArgs(t, map[string]any{
		keyDryRun: true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

// End-to-end verification of the SSH key get workflow.
func TestLinodeSSHKeyGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	if tool.Name != "linode_sshkey_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_sshkey_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeSSHKeyGetToolMissingSshkeyId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeSSHKeyGetToolZeroSshkeyId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(0)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeSSHKeyGetToolNegativeSshkeyId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(-1)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeSSHKeyGetToolSuccess(t *testing.T) {
	t.Parallel()

	sshKey := struct {
		linode.SSHKey

		NotInProto string `json:"not_in_proto"`
	}{
		SSHKey:     linode.SSHKey{ID: 42, Label: testKeyLabel, SSHKey: "ssh-rsa AAAA test@example.com", Created: "2024-01-01T00:00:00Z"},
		NotInProto: valNotInProto,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/sshkeys/42" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/sshkeys/42")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(sshKey); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(42)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, testKeyLabel) {
		t.Errorf("textContent.Text does not contain %v", testKeyLabel)
	}

	if !strings.Contains(textContent.Text, "ssh-rsa AAAA test@example.com") {
		t.Errorf("textContent.Text does not contain the public key")
	}

	if strings.Contains(textContent.Text, "not_in_proto") {
		t.Errorf("textContent.Text unexpectedly contains dropped unknown field: %v", textContent.Text)
	}
}

func TestLinodeSSHKeyGetToolAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "Not found"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeSSHKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keySSHKeyID: float64(999)})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeDomainZoneFileGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

	if tool.Name != "linode_domain_zone_file_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_zone_file_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyDomainID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDomainID)
	}
}

func TestLinodeDomainZoneFileGetToolInvalidDomainIdRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseZero, value: 0, set: true},
		{name: "negative", value: -1, set: true},
		{name: caseSlash, value: "1/2", set: true},
		{name: caseQuery, value: "1?x=2", set: true},
		{name: "path traversal", value: pathTraversalValue, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

			args := map[string]any{}
			if tt.set {
				args[keyDomainID] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "domain_id must be a positive integer") {
				t.Errorf("error text %q does not contain %q", text.Text, "domain_id must be a positive integer")
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeDomainZoneFileGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/domains/123/zone-file" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/123/zone-file")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			"zone_file": []string{"; example.com [123]", domainZoneTTL},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var out map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	zoneLines, isArray := out["zone_file"].([]any)
	if !isArray || len(zoneLines) != 2 {
		t.Errorf("zone_file not a 2-element array: %v", out["zone_file"])
	}

	if !strings.Contains(textContent.Text, "; example.com [123]") {
		t.Errorf("textContent.Text does not contain %v", "; example.com [123]")
	}

	if !strings.Contains(textContent.Text, domainZoneTTL) {
		t.Errorf("textContent.Text does not contain %v", domainZoneTTL)
	}
}

func TestLinodeDomainZoneFileGetToolApiErrorMapsToToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "domain not found"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeDomainZoneFileGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: 999})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve zone file for domain 999") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve zone file for domain 999")
	}
}

func TestLinodeDomainRecordGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeDomainRecordGetTool(cfg)

	if tool.Name != "linode_domain_record_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_domain_record_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeDomainRecordGetToolMissingDomainId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyRecordID: 456})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "domain_id must be a positive integer") {
		t.Errorf("textContent.Text does not contain %v", "domain_id must be a positive integer")
	}
}

func TestLinodeDomainRecordGetToolMissingRecordId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: 123})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "record_id must be a positive integer") {
		t.Errorf("textContent.Text does not contain %v", "record_id must be a positive integer")
	}
}

func TestLinodeDomainRecordGetToolNegativeDomainId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: -1, keyRecordID: 456})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "domain_id must be a positive integer") {
		t.Errorf("textContent.Text does not contain %v", "domain_id must be a positive integer")
	}
}

func TestLinodeDomainRecordGetToolNegativeRecordId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: 123, keyRecordID: -1})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "record_id must be a positive integer") {
		t.Errorf("textContent.Text does not contain %v", "record_id must be a positive integer")
	}
}

func TestLinodeDomainRecordGetToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/domains/123/records/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/123/records/456")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keySupportTicketID: 456,
			keyType:            "A",
			keyName:            hostWWW,
			keyTarget:          testNetIPv4AddressOne,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
			},
		},
	}
	_, _, handler := tools.NewLinodeDomainRecordGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyDomainID: 123, keyRecordID: 456})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body[keyID] != float64(456) {
		t.Errorf("body[%q] = %v, want 456", keyID, body[keyID])
	}

	if body[keyName] != hostWWW {
		t.Errorf("body[%q] = %v, want %v", keyName, body[keyName], hostWWW)
	}

	if body[keyTarget] != testNetIPv4AddressOne {
		t.Errorf("body[%q] = %v, want %v", keyTarget, body[keyTarget], testNetIPv4AddressOne)
	}
}

// TestLinodeAccountSettingsManagedEnableTool provides end-to-end verification of enabling Linode Managed.
func TestLinodeAccountSettingsManagedEnableToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

	if tool.Name != "linode_account_settings_managed_enable" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_settings_managed_enable")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}
}

func TestLinodeAccountSettingsManagedEnableToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

			args := map[string]any{}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountSettingsManagedEnableToolApiErrorProducesToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/settings/managed-enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/settings/managed-enable")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "managed could not be enabled"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to enable Linode Managed") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to enable Linode Managed")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "managed could not be enabled") {
		t.Errorf("error text %q does not contain %q", text.Text, "managed could not be enabled")
	}
}

func TestLinodeAccountSettingsManagedEnableToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/account/settings/managed-enable" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/account/settings/managed-enable")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsManagedEnableTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Linode Managed enabled successfully") {
		t.Errorf("textContent.Text does not contain %v", "Linode Managed enabled successfully")
	}
}

// End-to-end verification of account settings update.
func TestLinodeAccountSettingsUpdateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	if tool.Name != "linode_account_settings_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_account_settings_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if capability != profiles.CapAdmin {
		t.Errorf("capability = %v, want %v", capability, profiles.CapAdmin)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("RawInputSchema missing key %v", keyConfirm)
	}

	if !strings.Contains(rawSchema, tcBackupsEnabled) {
		t.Errorf("RawInputSchema missing key %v", tcBackupsEnabled)
	}
}

func TestLinodeAccountSettingsUpdateToolConfirmRequiredBeforeClientCall(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissing, set: false},
		{name: caseConfirmFalse, value: false, set: true},
		{name: caseString, value: boolStringTrue, set: true},
		{name: caseNumeric, value: 1, set: true},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&calls, 1)

				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
			_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

			args := map[string]any{tcNetworkHelper: false}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			req := createRequestWithArgs(t, args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}

			if calls != int32(0) {
				t.Errorf("calls = %v, want %v", calls, int32(0))
			}
		})
	}
}

func TestLinodeAccountSettingsUpdateToolEmptyUpdateRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "at least one account settings field is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "at least one account settings field is required")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountSettingsUpdateToolMalformedFieldRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tcBackupsEnabled: "true", keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "backups_enabled must be a boolean") {
		t.Errorf("error text %q does not contain %q", text.Text, "backups_enabled must be a boolean")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

// TestLinodeAccountSettingsUpdateToolUnsupportedFieldRejectedBeforeClientCall pins
// the unknown-field rejection ported from Python (strictest-wins): Go previously
// ignored unknown args, now rejects them locally before any HTTP call.
func TestLinodeAccountSettingsUpdateToolUnsupportedFieldRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tcBackupsEnabled: true, "bogus_field": "x", keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("expected an error result")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Unsupported account settings field(s): bogus_field") {
		t.Errorf("error text %q does not contain the unsupported-field message", text.Text)
	}
}

func TestLinodeAccountSettingsUpdateToolMalformedStringFieldRejectedBeforeClientCall(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyMaintenancePolicy: 123, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "maintenance_policy must be a string") {
		t.Errorf("error text %q does not contain %q", text.Text, "maintenance_policy must be a string")
	}

	if calls != int32(0) {
		t.Errorf("calls = %v, want %v", calls, int32(0))
	}
}

func TestLinodeAccountSettingsUpdateToolApiErrorProducesToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != accountSettingsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountSettingsTestPath)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{"field": keyMaintenancePolicy, keyReason: "invalid maintenance policy"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyMaintenancePolicy: "invalid", keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to update account settings") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to update account settings")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "invalid maintenance policy") {
		t.Errorf("error text %q does not contain %q", text.Text, "invalid maintenance policy")
	}
}

func TestLinodeAccountSettingsUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	settings := linode.AccountSettings{BackupsEnabled: true, NetworkHelper: false, MaintenancePolicy: maintenancePolicyMigrate}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != accountSettingsTestPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, accountSettingsTestPath)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		for key, want := range map[string]any{
			tcBackupsEnabled:     true,
			tcNetworkHelper:      false,
			keyMaintenancePolicy: maintenancePolicyMigrate,
		} {
			if !reflect.DeepEqual(body[key], want) {
				t.Errorf("body[%v] = %v, want %v", key, body[key], want)
			}
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := tools.NewLinodeAccountSettingsUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{tcBackupsEnabled: true, tcNetworkHelper: false, keyMaintenancePolicy: maintenancePolicyMigrate, keyConfirm: true})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Account settings updated successfully") {
		t.Errorf("textContent.Text does not contain %v", "Account settings updated successfully")
	}

	if !strings.Contains(textContent.Text, maintenancePolicyMigrate) {
		t.Errorf("textContent.Text does not contain %v", maintenancePolicyMigrate)
	}
}
