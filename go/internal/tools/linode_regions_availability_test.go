package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeRegionAvailabilityListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeRegionAvailabilityListTool(cfg)

		checkEqual(t, "linode_region_availability_list", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "availability list should be read-only")
		checkNotEmpty(t, tool.Description, "tool should have a description")
		requireNotNil(t, handler, "handler should not be nil")
		checkContains(t, tool.InputSchema.Properties, "environment", "schema should include environment")
		checkNoConfirm(t, tool.InputSchema.Properties, "read-only route should not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/regions/availability", r.URL.Path, "request path should match the documented route")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyRegion: regionUSEast, "plan": "g6-standard-1", statusAvailable: true}}}), "encoding response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeRegionAvailabilityListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		checkFalse(t, result.IsError, "result should be successful")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		checkContains(t, textContent.Text, "region availability", "response should identify availability list")
		checkContains(t, textContent.Text, "g6-standard-1", "response should include plan")
	})

	t.Run("api failure", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeRegionAvailabilityListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))

		requireNoError(t, err, "handler should return API failure as a tool error")
		requireNotNil(t, result, "result should not be nil")
		checkTrue(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		checkContains(t, textContent.Text, "Failed to retrieve", "response should identify failed request")
		checkContains(t, textContent.Text, errForbidden, "response should include API error detail")
	})
}

func TestLinodeRegionAvailabilityGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

		checkEqual(t, "linode_region_availability_get", tool.Name, "tool name should match")
		checkEqual(t, profiles.CapRead, capability, "availability get should be read-only")
		checkNotEmpty(t, tool.Description, "tool should have a description")
		requireNotNil(t, handler, "handler should not be nil")
		checkContains(t, tool.InputSchema.Properties, keyRegionID, "schema should include region_id")
		checkContains(t, tool.InputSchema.Properties, "environment", "schema should include environment")
		checkContains(t, tool.InputSchema.Required, keyRegionID, "region_id must be marked required")
		checkNoConfirm(t, tool.InputSchema.Properties, "read-only route should not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
			checkEqual(t, "/regions/us-east/availability", r.URL.Path, "request path should match the documented route")
			checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyRegion: regionUSEast, "plan": "g6-standard-1", statusAvailable: true}}}), "encoding response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast}))

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		checkFalse(t, result.IsError, "result should be successful")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		checkContains(t, textContent.Text, "g6-standard-1", "response should include plan")
		checkContains(t, textContent.Text, regionUSEast, "response should include region")
	})

	t.Run("invalid region id", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissingRegion, args: map[string]any{}, want: errRegionIDNonEmpty},
			{name: caseEmpty, args: map[string]any{keyRegionID: ""}, want: errRegionIDNonEmpty},
			{name: caseSlash, args: map[string]any{keyRegionID: regionIDSlashValue}, want: errRegionInvalid},
			{name: caseQuery, args: map[string]any{keyRegionID: regionIDQueryValue}, want: errRegionInvalid},
			{name: caseDotTraversal, args: map[string]any{keyRegionID: pathTraversalValue}, want: errRegionInvalid},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				requireNoError(t, err, "validation failure should be returned as tool error")
				requireNotNil(t, result, "result should not be nil")
				checkTrue(t, result.IsError, "invalid region_id should be an error result")
				assertErrorContains(t, result, testCase.want)
			})
		}
	})

	t.Run("api failure", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeRegionAvailabilityGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyRegionID: regionUSEast}))

		requireNoError(t, err, "handler should return API failure as a tool error")
		requireNotNil(t, result, "result should not be nil")
		checkTrue(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to retrieve")
		assertErrorContains(t, result, errForbidden)
	})
}
