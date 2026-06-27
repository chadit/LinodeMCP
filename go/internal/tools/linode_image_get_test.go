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

func TestLinodeImageGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageGetTool(cfg)

	if tool.Name != "linode_image_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if !strings.Contains(string(tool.RawInputSchema), keyImageID) {
		t.Errorf("tool.RawInputSchema missing key %v", keyImageID)
	}

	if strings.Contains(string(tool.RawInputSchema), keyConfirm) {
		t.Errorf("tool.RawInputSchema has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageGetToolSuccess(t *testing.T) {
	t.Parallel()

	image := linode.Image{ID: "linode/debian11", Label: imageUbuntu2204, Type: typeManualImage, Status: statusAvailable, Created: shareGroupCreated, Size: 2500}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/linode/debian11" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/linode/debian11")
		}

		if r.URL.EscapedPath() != "/images/linode%2Fdebian11" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/linode%2Fdebian11")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(image); err != nil {
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
	_, _, handler := tools.NewLinodeImageGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyImageID: "linode/debian11"})

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

	if !strings.Contains(textContent.Text, "linode/debian11") {
		t.Errorf("textContent.Text does not contain %v", "linode/debian11")
	}

	if !strings.Contains(textContent.Text, imageUbuntu2204) {
		t.Errorf("textContent.Text does not contain %v", imageUbuntu2204)
	}
}

func TestLinodeImageGetToolClientFailureReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/private/15" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/private/15")
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]any{{keyReason: errTemporaryFailure}},
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
	_, _, handler := tools.NewLinodeImageGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyImageID: "private/15"})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve image") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve image")
	}

	if !strings.Contains(textContent.Text, errTemporaryFailure) {
		t.Errorf("textContent.Text does not contain %v", errTemporaryFailure)
	}
}

func TestLinodeImageGetToolRejectsInvalidImageIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidValues := map[string]any{
		caseMissing:          nil,
		caseBlank:            "",
		caseNumeric:          123,
		"missing prefix":     "debian11",
		"unknown prefix":     "custom/debian11",
		"empty prefix":       "/debian11",
		"empty name":         "linode/",
		caseExtraSeparator:   "linode/debian/11",
		caseQuery:            "linode/debian11?arch=x64",
		caseFragment:         "linode/debian11#x64",
		caseDotdot:           pathTraversalValue,
		"prefixed traversal": "linode/..",
	}

	for name, value := range invalidValues {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
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
			_, _, handler := tools.NewLinodeImageGetTool(cfg)

			args := map[string]any{}
			if name != caseMissing {
				args[keyImageID] = value
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
