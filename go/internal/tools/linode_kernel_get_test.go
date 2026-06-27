package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeKernelGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeKernelGetTool(cfg)

	if tool.Name != "linode_kernel_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_kernel_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	rawSchema := string(tool.RawInputSchema)
	if !strings.Contains(rawSchema, keyKernelID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyKernelID)
	}

	if strings.Contains(rawSchema, keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeKernelGetToolSuccess(t *testing.T) {
	t.Parallel()

	kernel := linode.Kernel{ID: kernelLatestFixture, Label: kernelLabelFixture, Version: "6.8.9", Architecture: "x86_64"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/kernels/linode/latest-64bit" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/kernels/linode/latest-64bit")
		}

		if r.URL.EscapedPath() != "/linode/kernels/linode%2Flatest-64bit" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/linode/kernels/linode%2Flatest-64bit")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(kernel); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := kernelTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeKernelGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyKernelID: kernelLatestFixture})

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

	if !strings.Contains(textContent.Text, kernelLatestFixture) {
		t.Errorf("textContent.Text does not contain %v", kernelLatestFixture)
	}

	if !strings.Contains(textContent.Text, kernelLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", kernelLabelFixture)
	}
}

func TestLinodeKernelGetToolClientFailureReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/kernels/linode/latest-64bit" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/kernels/linode/latest-64bit")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	cfg := kernelTestConfig(srv.URL)
	_, _, handler := tools.NewLinodeKernelGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyKernelID: kernelLatestFixture})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve kernel") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve kernel")
	}

	if !strings.Contains(textContent.Text, errTemporaryFailure) {
		t.Errorf("textContent.Text does not contain %v", errTemporaryFailure)
	}
}

func TestLinodeKernelGetToolRejectsInvalidKernelIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidValues := map[string]any{
		caseMissing:        nil,
		caseBlank:          "",
		caseNumeric:        123,
		"missing prefix":   "latest-64bit",
		"empty prefix":     "/latest-64bit",
		"empty name":       "linode/",
		caseExtraSeparator: "linode/latest/64bit",
		caseQuery:          "linode/latest-64bit?arch=x64",
		caseFragment:       "linode/latest-64bit#x64",
		caseDotdot:         pathTraversalValue,
		"prefixed dotdot":  "linode/..",
	}

	for name, value := range invalidValues {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			cfg := kernelTestConfig(srv.URL)
			_, _, handler := tools.NewLinodeKernelGetTool(cfg)

			args := map[string]any{}
			if name != caseMissing {
				args[keyKernelID] = value
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

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func kernelTestConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}
