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

func TestLinodeImageShareGroupByTokenGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupByTokenGetTool(cfg)

	if tool.Name != "linode_image_sharegroup_by_token_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_by_token_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if _, ok := tool.InputSchema.Properties[keyTokenUUID]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyTokenUUID)
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupByTokenGetToolSuccess(t *testing.T) {
	t.Parallel()

	description := shareGroupDescription
	updated := shareGroupUpdated
	shareGroup := linode.ImageShareGroup{
		ID:          1,
		UUID:        shareGroupUUIDFixture,
		Label:       shareGroupLabelFixture,
		Description: &description,
		IsSuspended: false,
		Created:     shareGroupCreated,
		Updated:     &updated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/tokens/"+shareGroupTokenGetUUID+"/sharegroup" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID+"/sharegroup")
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
	_, _, handler := tools.NewLinodeImageShareGroupByTokenGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyTokenUUID: shareGroupTokenGetUUID})

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

func TestLinodeImageShareGroupByTokenGetToolRejectsInvalidTokenUuidBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidValues := map[string]any{
		caseSlash:   tokenUUIDWithSlash,
		caseQuery:   tokenUUIDWithQuery,
		"fragment":  tokenUUIDWithFragment,
		caseDotdot:  tokenUUIDWithDotdot,
		caseNotUUID: invalidTokenUUID,
		"blank":     "   ",
		caseNumeric: 123,
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
			_, _, handler := tools.NewLinodeImageShareGroupByTokenGetTool(cfg)

			req := createRequestWithArgs(t, map[string]any{keyTokenUUID: value})

			result, err := handler(t.Context(), req)
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

func TestLinodeImageShareGroupByTokenGetToolMissingTokenUuid(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeImageShareGroupByTokenGetTool(cfg)

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

func TestLinodeImageShareGroupByTokenGetToolClientError(t *testing.T) {
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
	_, _, handler := tools.NewLinodeImageShareGroupByTokenGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyTokenUUID: shareGroupTokenGetUUID})

	result, err := handler(t.Context(), req)
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

	if !strings.Contains(textContent.Text, "Failed to retrieve image share group by token") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve image share group by token")
	}
}
