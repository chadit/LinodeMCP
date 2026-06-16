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

func TestLinodeImageShareGroupMemberTokenGetToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, capability, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

	if tool.Name != "linode_image_sharegroup_member_token_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_image_sharegroup_member_token_get")
	}

	if capability != profiles.CapRead {
		t.Errorf("capability = %v, want %v", capability, profiles.CapRead)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyShareGroupID, keyTokenUUID} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if _, ok := tool.InputSchema.Properties[keyConfirm]; ok {
		t.Errorf("tool.InputSchema.Properties has unexpected key %v", keyConfirm)
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupMemberTokenGetToolSuccess(t *testing.T) {
	t.Parallel()

	updated := "2025-08-05T10:09:09"
	member := linode.ImageShareGroupMember{
		TokenUUID: shareGroupTokenGetUUID,
		Status:    statusActive,
		Label:     "Engineering - Backend",
		Created:   imageShareGroupTokenCreated,
		Updated:   &updated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/images/sharegroups/123/members/"+shareGroupTokenGetUUID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123/members/"+shareGroupTokenGetUUID)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(member); err != nil {
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
	_, _, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID})

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

	if !strings.Contains(textContent.Text, "Engineering - Backend") {
		t.Errorf("textContent.Text does not contain %v", "Engineering - Backend")
	}

	if !strings.Contains(textContent.Text, shareGroupTokenGetUUID) {
		t.Errorf("textContent.Text does not contain %v", shareGroupTokenGetUUID)
	}
}

func TestLinodeImageShareGroupMemberTokenGetToolRejectsInvalidPathParamsBeforeClientCall(t *testing.T) {
	t.Parallel()

	invalidArgs := map[string]map[string]any{
		"slash sharegroup_id":  {keyShareGroupID: paymentMethodIDSlash, keyTokenUUID: shareGroupTokenGetUUID},
		"query sharegroup_id":  {keyShareGroupID: pathQueryValue, keyTokenUUID: shareGroupTokenGetUUID},
		"dotdot sharegroup_id": {keyShareGroupID: pathTraversalValue, keyTokenUUID: shareGroupTokenGetUUID},
		"zero sharegroup_id":   {keyShareGroupID: 0, keyTokenUUID: shareGroupTokenGetUUID},
		"slash token_uuid":     {keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithSlash},
		"query token_uuid":     {keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithQuery},
		"dotdot token_uuid":    {keyShareGroupID: 123, keyTokenUUID: tokenUUIDWithDotdot},
		"numeric token_uuid":   {keyShareGroupID: 123, keyTokenUUID: 123},
	}

	for name, args := range invalidArgs {
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
			_, _, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

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

func TestLinodeImageShareGroupMemberTokenGetToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
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
	_, _, handler := tools.NewLinodeImageShareGroupMemberTokenGetTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyShareGroupID: 123, keyTokenUUID: shareGroupTokenGetUUID}))
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
