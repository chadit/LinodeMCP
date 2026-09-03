package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
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
	profilePreferenceValueDark       = "dark"
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
	accountPaymentsTestPath          = "/account/payments"
	accountPaymentMethodsTestPath    = "/account/payment-methods"
	accountServiceTransfersTestPath  = "/account/service-transfers"
	keyPaymentType                   = "type"
	keyPaymentData                   = "data"
)

// End-to-end verification of the hello tool.
func TestHelloTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := gentools.NewHelloTool(nil)

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

// HelloInput declares one field and no rules, so a call naming nothing else
// answers and a call naming anything else is refused. Pinned because the meta
// tier used to take an undeclared argument and greet the caller anyway.
func TestHelloToolAnswersItsOwnFieldAndRefusesAnother(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewHelloTool(nil)

	answered, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if answered.IsError {
		t.Fatal("answered.IsError = true, want false")
	}

	textContent, isText := answered.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	if !strings.Contains(textContent.Text, "Hello, World!") {
		t.Errorf("textContent.Text does not contain %v", "Hello, World!")
	}

	refused, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"unexpected": "argument"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !refused.IsError {
		t.Fatal("refused.IsError = false, want true")
	}

	refusal, isRefusalText := refused.Content[0].(mcp.TextContent)
	if !isRefusalText {
		t.Fatal("isRefusalText = false, want true")
	}

	if want := "Unsupported argument(s) for hello: unexpected"; refusal.Text != want {
		t.Errorf("refusal.Text = %v, want %v", refusal.Text, want)
	}
}

// End-to-end verification of the version tool.
func TestVersionTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := gentools.NewVersionTool(nil)

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
	tool, _, handler := gentools.NewLinodeInstanceListTool(cfg)

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
	_, _, handler := gentools.NewLinodeInstanceListTool(cfg)

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
	_, _, handler := gentools.NewLinodeInstanceListTool(cfg)

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

	instances := []Instance{
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
	_, _, handler := gentools.NewLinodeInstanceListTool(cfg)

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
		tool, _, handler := gentools.NewLinodeProfileGetTool(cfg)

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
		_, _, handler := gentools.NewLinodeProfileGetTool(cfg)

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
		_, _, handler := gentools.NewLinodeProfileGetTool(cfg)

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

// End-to-end verification of the instance get workflow.
func TestLinodeInstanceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeInstanceGetTool(cfg)

	if tool.Name != toolInstanceGet {
		t.Errorf("tool.Name = %v, want %v", tool.Name, toolInstanceGet)
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
	_, _, handler := gentools.NewLinodeInstanceGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeInstanceGetTool(cfg)

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

	instance := Instance{
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
	_, _, handler := gentools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyInstanceID: 123})

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
		tool, _, handler := gentools.NewLinodeAccountGetTool(cfg)

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

		account := Account{
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
		_, _, handler := gentools.NewLinodeAccountGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountTransferGetTool(cfg)

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

	transfer := AccountTransfer{
		Billable: 10,
		Quota:    4000,
		Used:     123,
		RegionTransfers: []AccountRegionTransfer{{
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
	_, _, handler := gentools.NewLinodeAccountTransferGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountTransferGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve the account transfer pool") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve the account transfer pool")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

// End-to-end verification of account settings retrieval.
func TestLinodeAccountSettingsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountSettingsGetTool(cfg)

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
	settings := AccountSettings{
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
	_, _, handler := gentools.NewLinodeAccountSettingsGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountSettingsGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountAgreementListTool(cfg)

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

	agreements := AccountAgreements{
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
	_, _, handler := gentools.NewLinodeAccountAgreementListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountAgreementListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountMaintenanceListTool(cfg)

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

	maintenance := PaginatedResponse[AccountMaintenance]{
		Data: []AccountMaintenance{{
			Entity: AccountMaintenanceEntity{ID: 123, Label: accountMaintenanceLabel, Type: accountMaintenanceEntityType, URL: accountMaintenanceURL},
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
	_, _, handler := gentools.NewLinodeAccountMaintenanceListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountMaintenanceListTool(cfg)
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
	_, _, handler := gentools.NewLinodeAccountMaintenanceListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeMaintenancePolicyListTool(cfg)

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

	policies := PaginatedResponse[MaintenancePolicy]{
		Data: []MaintenancePolicy{{
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
	_, _, handler := gentools.NewLinodeMaintenancePolicyListTool(cfg)

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
			_, _, handler := gentools.NewLinodeMaintenancePolicyListTool(cfg)
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
	_, _, handler := gentools.NewLinodeMaintenancePolicyListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountAvailabilityGetTool(cfg)

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

	availability := AccountAvailability{
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
	_, _, handler := gentools.NewLinodeAccountAvailabilityGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountAvailabilityGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve account availability for region") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve account availability for region")
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
			_, _, handler := gentools.NewLinodeAccountAvailabilityGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountNotificationListTool(cfg)

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

	notifications := PaginatedResponse[AccountNotification]{
		Data: []AccountNotification{{
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
	_, _, handler := gentools.NewLinodeAccountNotificationListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountNotificationListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountNotificationListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeBetaListTool(cfg)

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
	betas := PaginatedResponse[BetaProgram]{
		Data: []BetaProgram{{
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
	_, _, handler := gentools.NewLinodeBetaListTool(cfg)

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
	_, _, handler := gentools.NewLinodeBetaListTool(cfg)

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
			_, _, handler := gentools.NewLinodeBetaListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeBetaGetTool(cfg)

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
	beta := BetaProgram{
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
	_, _, handler := gentools.NewLinodeBetaGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeBetaGetTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve beta program") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve beta program")
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
			_, _, handler := gentools.NewLinodeBetaGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountBetaListTool(cfg)

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
	betas := PaginatedResponse[AccountBetaProgram]{
		Data: []AccountBetaProgram{{
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
	_, _, handler := gentools.NewLinodeAccountBetaListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountBetaListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountBetaListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountInvoiceGetTool(cfg)

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

	invoice := AccountInvoice{ID: accountInvoiceID, Date: "2024-01-31T00:00:00", Label: "Invoice #12345", Total: 11.00}

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
	_, _, handler := gentools.NewLinodeAccountInvoiceGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountInvoiceGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve invoice") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve invoice")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountInvoiceGetToolInvalidInvoiceIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
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
			_, _, handler := gentools.NewLinodeAccountInvoiceGetTool(cfg)

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

	tool, capability, handler := gentools.NewLinodeProfileSecurityQuestionListTool(cfg)
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
	_, _, handler := gentools.NewLinodeProfileSecurityQuestionListTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfileSecurityQuestionListTool(cfg)

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

	tool, capability, handler := gentools.NewLinodeProfileDeviceListTool(cfg)
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
	_, _, handler := gentools.NewLinodeProfileDeviceListTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfileDeviceListTool(cfg)

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
			_, _, handler := gentools.NewLinodeProfileDeviceListTool(cfg)
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

func TestLinodeProfilePhoneNumberDeleteToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeProfilePhoneNumberDeleteTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfilePhoneNumberDeleteTool(cfg)
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
	_, _, handler := gentools.NewLinodeProfilePhoneNumberDeleteTool(cfg)
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
	_, _, handler := gentools.NewLinodeProfilePhoneNumberDeleteTool(cfg)
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
		confirm any
		name    string
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
			_, _, handler := gentools.NewLinodeProfilePhoneNumberDeleteTool(cfg)

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

// End-to-end verification of profile OAuth app authorization retrieval.
func TestLinodeProfileAppsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}

	tool, capability, handler := gentools.NewLinodeProfileAppListTool(cfg)
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
			"created":       longviewClientCreatedFixture,
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
	_, _, handler := gentools.NewLinodeProfileAppListTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfileAppListTool(cfg)

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
			_, _, handler := gentools.NewLinodeProfileAppListTool(cfg)

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

	tool, capability, handler := gentools.NewLinodeAccountOauthClientListTool(cfg)
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
	_, _, handler := gentools.NewLinodeAccountOauthClientListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountOauthClientListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountOauthClientListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountChildAccountGetTool(cfg)

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

	childAccount := ChildAccount{
		EUUID:         childAccountEUUID,
		Company:       companyAcme,
		Email:         "jkowalski@example.com",
		FirstName:     "John",
		LastName:      "Smith",
		BillingSource: "external",
		CreditCard: ChildAccountCreditCard{
			Expiry:   "11/2024",
			LastFour: "0111",
		},
	}

	// The extra field the proto does not model must be dropped by the
	// DiscardUnknown decode, proving the output routes through the proto
	// serializer rather than the legacy struct.
	wrapped := withUnmodeledField(t, childAccount)

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
	_, _, handler := gentools.NewLinodeAccountChildAccountGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountChildAccountGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve child account") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve child account")
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
			_, _, handler := gentools.NewLinodeAccountChildAccountGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountEventListTool(cfg)

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

	events := PaginatedResponse[AccountEvent]{
		Data:    []AccountEvent{{ID: 123, Action: "ticket_create", Status: "failed", Username: "adevi"}},
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
	_, _, handler := gentools.NewLinodeAccountEventListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountEventListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountEventListTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeAccountUserListTool(cfg)

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

	users := PaginatedResponse[AccountUser]{
		Data:    []AccountUser{{Username: accountLoginUsername, Email: "user@example.com", Restricted: true, TFAEnabled: true}},
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
	_, _, handler := gentools.NewLinodeAccountUserListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountUserListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountUserListTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeAccountLoginListTool(cfg)

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

	logins := PaginatedResponse[AccountLogin]{
		Data:    []AccountLogin{{ID: 123, Username: accountLoginUsername, IP: testNetIPv4AddressTen, Status: statusSuccessful}},
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
	_, _, handler := gentools.NewLinodeAccountLoginListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountLoginListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountLoginListTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeProfileLoginGetTool(cfg)

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

	login := AccountLogin{ID: 123, Username: accountLoginUsername, IP: testNetIPv4AddressTen, Status: statusSuccessful}

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
	_, _, handler := gentools.NewLinodeProfileLoginGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfileLoginGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve profile login") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve profile login")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeProfileLoginGetToolInvalidLoginIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
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
			_, _, handler := gentools.NewLinodeProfileLoginGetTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeAccountLoginGetTool(cfg)

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

	login := AccountLogin{ID: 123, Username: accountLoginUsername, IP: testNetIPv4AddressTen, Status: statusSuccessful}

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
	_, _, handler := gentools.NewLinodeAccountLoginGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountLoginGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve account login") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve account login")
	}

	if !strings.Contains(textContent.Text, errForbidden) {
		t.Errorf("textContent.Text does not contain %v", errForbidden)
	}
}

func TestLinodeAccountLoginGetToolInvalidLoginIdRejectsBeforeClient(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
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
			_, _, handler := gentools.NewLinodeAccountLoginGetTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeAccountChildAccountListTool(cfg)

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

	childAccounts := PaginatedResponse[ChildAccount]{
		Data: []ChildAccount{{
			EUUID:         childAccountEUUID,
			Company:       companyAcme,
			Email:         "jkowalski@example.com",
			FirstName:     "John",
			LastName:      "Smith",
			BillingSource: "external",
			CreditCard: ChildAccountCreditCard{
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
	_, _, handler := gentools.NewLinodeAccountChildAccountListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountChildAccountListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountChildAccountListTool(cfg)

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

func TestLinodeProfileDeviceGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeProfileDeviceGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfileDeviceGetTool(cfg)
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
	_, _, handler := gentools.NewLinodeProfileDeviceGetTool(cfg)
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve trusted device") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve trusted device")
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
			_, _, handler := gentools.NewLinodeProfileDeviceGetTool(cfg)
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

func TestLinodeProfileAppGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeProfileAppGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeProfileAppGetTool(cfg)
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
	_, _, handler := gentools.NewLinodeProfileAppGetTool(cfg)
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve authorized app") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve authorized app")
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
			_, _, handler := gentools.NewLinodeProfileAppGetTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeAccountOauthClientGetTool(cfg)

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
			keySupportTicketID: oauthClientID, keyLabel: oauthClientLabel, keyRedirectURI: oauthClientRedirectURI, keyStatus: statusActive, keySecret: "server-secret",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeAccountOauthClientGetTool(cfg)
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
	_, _, handler := gentools.NewLinodeAccountOauthClientGetTool(cfg)
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve OAuth client") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve OAuth client")
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
			_, _, handler := gentools.NewLinodeAccountOauthClientGetTool(cfg)
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

// End-to-end verification of account OAuth client secret reset.
func TestLinodeAccountOAuthClientResetSecretToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountOauthClientSecretResetTool(cfg)

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
		confirm any
		name    string
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
			_, _, handler := gentools.NewLinodeAccountOauthClientSecretResetTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountOauthClientSecretResetTool(cfg)
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

		// The reset takes no arguments the API reads, so the generated body
		// builder sends the empty object every write tool sends.
		sent, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			t.Errorf("read request body: %v", readErr)
		}

		if strings.TrimSpace(string(sent)) != jsonObjectEmpty {
			t.Errorf("r.Body = %q, want %q", sent, jsonObjectEmpty)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(OAuthClientSecret{Secret: "new-secret-once"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeAccountOauthClientSecretResetTool(cfg)
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
	_, _, handler := gentools.NewLinodeAccountOauthClientSecretResetTool(cfg)
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to reset the secret for OAuth client") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to reset the secret for OAuth client")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account OAuth client creation.
func TestLinodeAccountOAuthClientCreateToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountOauthClientCreateTool(cfg)

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
		confirm any
		name    string
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
			_, _, handler := gentools.NewLinodeAccountOauthClientCreateTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountOauthClientCreateTool(cfg)
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

		var got CreateOAuthClientRequest
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

		if err := json.NewEncoder(w).Encode(CreatedOAuthClient{
			ID: oauthClientID, Label: oauthClientLabel, RedirectURI: oauthClientRedirectURI, Secret: "secret-once",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeAccountOauthClientCreateTool(cfg)
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
	_, _, handler := gentools.NewLinodeAccountOauthClientCreateTool(cfg)
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to create OAuth client") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to create OAuth client")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errForbidden) {
		t.Errorf("error text %q does not contain %q", text.Text, errForbidden)
	}
}

// End-to-end verification of account invoice listing.
func TestLinodeAccountInvoiceItemsToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountInvoiceItemListTool(cfg)

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

	items := PaginatedResponse[AccountInvoiceItem]{
		Data:    []AccountInvoiceItem{{Label: invoiceItemLabel, Quantity: 1, Total: 5.00, Type: "linode", UnitPrice: 5.00}},
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
	_, _, handler := gentools.NewLinodeAccountInvoiceItemListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountInvoiceItemListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountInvoiceItemListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountPaymentMethodListTool(cfg)

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

	methods := PaginatedResponse[AccountPaymentMethod]{
		Data:    []AccountPaymentMethod{{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}}},
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
	_, _, handler := gentools.NewLinodeAccountPaymentMethodListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountPaymentMethodListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountPaymentMethodListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountPaymentMethodGetTool(cfg)

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

	// The extra top-level field the proto does not model must be dropped by
	// the DiscardUnknown decode, proving proto-canonical output.
	method := withUnmodeledField(t, AccountPaymentMethod{ID: 123, Type: paymentMethodCreditCard, IsDefault: true, Data: map[string]any{keyLastFour: paymentMethodLastFour}})

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

		if err := json.NewEncoder(w).Encode(method); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
	_, _, handler := gentools.NewLinodeAccountPaymentMethodGetTool(cfg)
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
	_, _, handler := gentools.NewLinodeAccountPaymentMethodGetTool(cfg)
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve payment method") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve payment method")
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
			_, _, handler := gentools.NewLinodeAccountPaymentMethodGetTool(cfg)
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
	tool, capability, handler := gentools.NewLinodeAccountPaymentListTool(cfg)

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

	payments := PaginatedResponse[AccountPayment]{
		Data:    []AccountPayment{{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}},
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
	_, _, handler := gentools.NewLinodeAccountPaymentListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountPaymentListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountPaymentListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountPaymentGetTool(cfg)

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

	payment := AccountPayment{ID: 654, Date: "2024-02-01T00:00:00", USD: 20.25}

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
	_, _, handler := gentools.NewLinodeAccountPaymentGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountPaymentGetTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve payment") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve payment")
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
			_, _, handler := gentools.NewLinodeAccountPaymentGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountInvoiceListTool(cfg)

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

	invoices := PaginatedResponse[AccountInvoice]{
		Data:    []AccountInvoice{{ID: 987, Date: "2024-01-31T00:00:00", Label: "Invoice 987", Total: 42.50}},
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
	_, _, handler := gentools.NewLinodeAccountInvoiceListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountInvoiceListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountInvoiceListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountServiceTransferListTool(cfg)

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

	transfers := PaginatedResponse[AccountEntityTransfer]{
		Data: []AccountEntityTransfer{{
			Entities: AccountEntityTransferEntities{Linodes: []int{111, 222}},
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
	_, _, handler := gentools.NewLinodeAccountServiceTransferListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountServiceTransferListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountServiceTransferListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountEventGetTool(cfg)

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

		if err := json.NewEncoder(w).Encode(AccountEvent{
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
	_, _, handler := gentools.NewLinodeAccountEventGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountEventGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve account event") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve account event")
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
			_, _, handler := gentools.NewLinodeAccountEventGetTool(cfg)

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

func TestLinodeAccountServiceTransferGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountServiceTransferGetTool(cfg)

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

	transfer := AccountEntityTransfer{
		Entities: AccountEntityTransferEntities{Linodes: []int{111, 222}},
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
	_, _, handler := gentools.NewLinodeAccountServiceTransferGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountServiceTransferGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve service transfer") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve service transfer")
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
			_, _, handler := gentools.NewLinodeAccountServiceTransferGetTool(cfg)

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

// End-to-end verification of enrolled account beta program retrieval.
func TestLinodeAccountBetaGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountBetaGetTool(cfg)

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
	beta := AccountBetaProgram{
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
	_, _, handler := gentools.NewLinodeAccountBetaGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountBetaGetTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to retrieve account beta program") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to retrieve account beta program")
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
			_, _, handler := gentools.NewLinodeAccountBetaGetTool(cfg)

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

// End-to-end verification of account availability retrieval.
func TestLinodeAccountAvailabilityToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := gentools.NewLinodeAccountAvailabilityListTool(cfg)

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

	availability := PaginatedResponse[AccountAvailability]{
		Data: []AccountAvailability{{
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
	_, _, handler := gentools.NewLinodeAccountAvailabilityListTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountAvailabilityListTool(cfg)

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
			_, _, handler := gentools.NewLinodeAccountAvailabilityListTool(cfg)

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

// End-to-end verification of region listing and filtering.
func TestLinodeRegionsListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeRegionListTool(cfg)

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

	regions := []Region{
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
	_, _, handler := gentools.NewLinodeRegionListTool(cfg)

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

	regions := []Region{
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
	_, _, handler := gentools.NewLinodeRegionListTool(cfg)

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
	tool, _, handler := gentools.NewLinodeTypeListTool(cfg)

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

	types := []InstanceType{
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
	_, _, handler := gentools.NewLinodeTypeListTool(cfg)

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

	types := []InstanceType{
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
	_, _, handler := gentools.NewLinodeTypeListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeVolumeTypeListTool(cfg)

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

	volumeTypes := []VolumeType{{
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
	_, _, handler := gentools.NewLinodeVolumeTypeListTool(cfg)

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
	_, _, handler := gentools.NewLinodeVolumeTypeListTool(cfg)

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
	tool, _, handler := gentools.NewLinodeVolumeListTool(cfg)

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

	volumes := []Volume{
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
	_, _, handler := gentools.NewLinodeVolumeListTool(cfg)

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

	volumes := []Volume{
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
	_, _, handler := gentools.NewLinodeVolumeListTool(cfg)

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

	volumes := []Volume{
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
	_, _, handler := gentools.NewLinodeVolumeListTool(cfg)

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
	tool, _, handler := gentools.NewLinodeImageListTool(cfg)

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

	images := []Image{
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
	_, _, handler := gentools.NewLinodeImageListTool(cfg)

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

	images := []Image{
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
	_, _, handler := gentools.NewLinodeImageListTool(cfg)

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

	images := []Image{
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
	_, _, handler := gentools.NewLinodeImageListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeImageSharegroupTokenListTool(cfg)

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
	tokens := []ImageShareGroupToken{
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
	_, _, handler := gentools.NewLinodeImageSharegroupTokenListTool(cfg)

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
	_, _, handler := gentools.NewLinodeImageSharegroupTokenListTool(cfg)

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
	_, _, handler := gentools.NewLinodeImageSharegroupTokenListTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeImageSharegroupListTool(cfg)

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
	shareGroups := []ImageShareGroup{
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
	_, _, handler := gentools.NewLinodeImageSharegroupListTool(cfg)

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
	_, _, handler := gentools.NewLinodeImageSharegroupListTool(cfg)

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
	_, _, handler := gentools.NewLinodeImageSharegroupListTool(cfg)

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

// withUnmodeledField renders value as a JSON object carrying one extra
// top-level key the proto message does not model, so a success test can assert
// the DiscardUnknown decode drops it. Splicing the key in beats wrapping value
// in a struct that embeds it: embeddedstructfieldcheck pins the embedded field
// first while fieldalignment wants the trailing string ahead of it, and no
// field order satisfies both.
func withUnmodeledField(t *testing.T, value any) json.RawMessage {
	t.Helper()

	encoded, encodeErr := json.Marshal(value)
	if encodeErr != nil {
		t.Fatalf("unexpected error: %v", encodeErr)
	}

	fields := map[string]json.RawMessage{}
	if decodeErr := json.Unmarshal(encoded, &fields); decodeErr != nil {
		t.Fatalf("unexpected error: %v", decodeErr)
	}

	extra, extraErr := json.Marshal(valNotInProto)
	if extraErr != nil {
		t.Fatalf("unexpected error: %v", extraErr)
	}

	fields[keyNotInProto] = extra

	merged, mergeErr := json.Marshal(fields)
	if mergeErr != nil {
		t.Fatalf("unexpected error: %v", mergeErr)
	}

	return merged
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

// End-to-end verification of the SSH key get workflow.
func TestLinodeSSHKeyGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeSshkeyGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeSshkeyGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeSshkeyGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeSshkeyGetTool(cfg)

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

	sshKey := withUnmodeledField(t, SSHKey{ID: 42, Label: testKeyLabel, SSHKey: "ssh-rsa AAAA test@example.com", Created: "2024-01-01T00:00:00Z"})

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
	_, _, handler := gentools.NewLinodeSshkeyGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeSshkeyGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeDomainZoneFileGetTool(cfg)

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
		value any
		name  string
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
			_, _, handler := gentools.NewLinodeDomainZoneFileGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainZoneFileGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainZoneFileGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeDomainRecordGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainRecordGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainRecordGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainRecordGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainRecordGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeDomainRecordGetTool(cfg)

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
	tool, capability, handler := gentools.NewLinodeAccountSettingsManagedEnableTool(cfg)

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
		value any
		name  string
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
			_, _, handler := gentools.NewLinodeAccountSettingsManagedEnableTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountSettingsManagedEnableTool(cfg)

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
	_, _, handler := gentools.NewLinodeAccountSettingsManagedEnableTool(cfg)

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
