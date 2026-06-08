package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeImageShareGroupGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

	if tool.Name != "linode_image_sharegroup_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[keyShareGroupID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyShareGroupID)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupGetToolSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescription
	updated := shareGroupUpdated
	shareGroup := linode.ImageShareGroup{
		ID:           123,
		UUID:         shareGroupUUIDFixture,
		Label:        shareGroupLabelFixture,
		Description:  &description,
		IsSuspended:  false,
		Created:      shareGroupCreated,
		Updated:      &updated,
		ImagesCount:  2,
		MembersCount: 3,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(shareGroup); err != nil {
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
	_, _, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123})

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

	if !strings.Contains(textContent.Text, shareGroupLabelFixture) {
		t.Errorf("textContent.Text does not contain %v", shareGroupLabelFixture)
	}

	if !strings.Contains(textContent.Text, shareGroupUUIDFixture) {
		t.Errorf("textContent.Text does not contain %v", shareGroupUUIDFixture)
	}
}

func TestLinodeImageShareGroupGetToolClientFailureReturnsToolError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/123" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123")
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
	_, _, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123})

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

	if !strings.Contains(textContent.Text, "Failed to retrieve image share group") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve image share group")
	}

	if !strings.Contains(textContent.Text, errTemporaryFailure) {
		t.Errorf("textContent.Text does not contain %v", errTemporaryFailure)
	}
}

func TestLinodeImageShareGroupGetToolRejectsInvalidSharegroupIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidValues := map[string]any{
		caseMissing:  nil,
		caseZero:     0,
		"negative":   -1,
		"fractional": 1.5,
		caseSlash:    "123/456",
		caseQuery:    "1?foo=bar",
		caseDotdot:   pathTraversalValue,
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
			_, _, handler := tools.NewLinodeImageShareGroupGetTool(cfg)

			args := map[string]any{}
			if name != caseMissing {
				args[keyShareGroupID] = value
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
