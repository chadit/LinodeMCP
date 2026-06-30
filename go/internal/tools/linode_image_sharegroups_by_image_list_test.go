package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeImageShareGroupsByImageListToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

	if tool.Name != "linode_image_sharegroup_by_image_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_by_image_list")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[keyImageID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyImageID)
	}

	if !slices.Contains(tool.InputSchema.Required, keyImageID) {
		t.Errorf("tool.InputSchema.Required does not contain %v", keyImageID)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupsByImageListToolSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescription
	shareGroups := []linode.ImageShareGroup{{
		ID:           1,
		UUID:         shareGroupUUIDExample,
		Label:        "base-images",
		Description:  &description,
		IsSuspended:  false,
		Created:      "2025-04-14T22:44:02",
		Updated:      nil,
		Expiry:       nil,
		ImagesCount:  2,
		MembersCount: 3,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/images/private%2F12345/sharegroups" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F12345/sharegroups")
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
			keyResults: 51,
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
	_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyImageID: privateImage12345Fixture, keyPage: 2, keyPageSize: 25})

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

	if !strings.Contains(textContent.Text, "base-images") {
		t.Errorf("textContent.Text does not contain %v", "base-images")
	}

	if !strings.Contains(textContent.Text, shareGroupUUIDExample) {
		t.Errorf("textContent.Text does not contain %v", shareGroupUUIDExample)
	}
}

func TestLinodeImageShareGroupsByImageListToolRejectsInvalidImageIdBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidValues := map[string]any{
		"missing slash":       "private12345",
		caseExtraSeparator:    "private/12345/extra",
		"unsupported shared":  "shared/12345",
		"unsupported linode":  "linode/ubuntu24.04",
		"non-numeric private": "private/abc",
		"zero private":        privateImageZeroFixture,
		caseQuery:             "private/12345?query",
		caseDotdot:            privateImageTraversalFixture,
		caseEmpty:             blankString,
		caseNumeric:           12345,
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
			_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

			req := createRequestWithArgs(t, map[string]any{keyImageID: value})

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

func TestLinodeImageShareGroupsByImageListToolMissingImageId(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

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

func TestLinodeImageShareGroupsByImageListToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: temporaryFailure}},
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
	_, _, handler := tools.NewLinodeImageShareGroupsByImageListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyImageID: privateImage12345Fixture})

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
}
