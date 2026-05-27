package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const imageReplicateToolName = "linode_image_replicate"

const regionUSMiami = "us-mia"

func TestLinodeImageReplicateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeImageReplicateTool(&config.Config{})

		assert.Equal(t, imageReplicateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyImageID, "schema should include image_id")
		assert.Contains(t, props, keyRegions, "schema should include regions")
		assert.Contains(t, props, keyConfirm, "mutating replicate tool must require confirm")
		assert.Contains(t, tool.InputSchema.Required, keyImageID, "image_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyRegions, "regions must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/images/private%2F123/regions", r.URL.EscapedPath(), "request path should escape image ID")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, []any{regionUSMiami, regionUSEast}, body[keyRegions])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: privateImage123Fixture,
				keyLabel:  "replicated-image",
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageReplicateTool(imageReplicateConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage123Fixture,
			keyRegions: `["us-mia","us-east"]`,
			keyConfirm: true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "replicated successfully")
		assert.Contains(t, textContent.Text, privateImage123Fixture)
	})
}

func TestLinodeImageReplicateToolValidation(t *testing.T) {
	t.Parallel()

	for name, confirm := range map[string]any{
		caseMissingConfirm: nil,
		caseFalseConfirm:   false,
		caseStringConfirm:  boolStringTrue,
		caseNumericConfirm: 1,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			closeServer, handler := imageReplicateHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			args := map[string]any{keyImageID: privateImage123Fixture, keyRegions: singleRegionJSON}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	for _, tt := range []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingImageID, args: map[string]any{keyRegions: singleRegionJSON, keyConfirm: true}, want: "image_id must be a non-empty string"},
		{name: "slash-only image id", args: map[string]any{keyImageID: pathSeparatorValue, keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: caseQueryImageID, args: map[string]any{keyImageID: "private/123?bad", keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "fragment image id", args: map[string]any{keyImageID: "private/123#frag", keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: caseTraversalImageID, args: map[string]any{keyImageID: privateImageTraversalFixture, keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "extra segment image id", args: map[string]any{keyImageID: "private/123/456", keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "public image id", args: map[string]any{keyImageID: imageIDUbuntu2204, keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "non-numeric private image id", args: map[string]any{keyImageID: "private/not-a-number", keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "zero private image id", args: map[string]any{keyImageID: privateImageZeroFixture, keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "signed private image id", args: map[string]any{keyImageID: "private/+123", keyRegions: singleRegionJSON, keyConfirm: true}, want: errImageIDPathFragment},
		{name: "missing regions", args: map[string]any{keyImageID: privateImage123Fixture, keyConfirm: true}, want: "regions is required"},
		{name: "non-string regions", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: []any{regionUSEast}, keyConfirm: true}, want: "regions must be a JSON string array"},
		{name: "malformed regions", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `[`, keyConfirm: true}, want: "regions must be a JSON string array"},
		{name: "empty regions", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: databaseJSONArray, keyConfirm: true}, want: "regions must contain at least one region"},
		{name: "blank region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["us-east",""]`, keyConfirm: true}, want: "regions entries must be non-empty strings"},
		{name: "slash region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["us/east"]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
		{name: "query region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["us-east?x=1"]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
		{name: "fragment region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["us-east#frag"]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
		{name: "traversal region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `[".."]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
		{name: "uppercase region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["US-EAST"]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
		{name: "space region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["us east"]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
		{name: "spaced region", args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `[" us-east "]`, keyConfirm: true}, want: errRegionsLowercaseSlug},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			closeServer, handler := imageReplicateHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError, "invalid input should be an error result")
			assertErrorContains(t, result, tt.want)
			assert.Equal(t, int32(0), calls.Load(), "validation must happen before client call")
		})
	}

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageReplicateTool(imageReplicateConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage123Fixture,
			keyRegions: singleRegionJSON,
			keyConfirm: true,
		}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError, "upstream API error should be an error result")
		assertErrorContains(t, result, "Failed to replicate image")
	})
}

func imageReplicateConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}

func imageReplicateHandlerWithCallCounter(
	t *testing.T,
	calls *atomic.Int32,
) (func(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyBetaID: privateImage123Fixture}))
	}))

	_, _, handler := tools.NewLinodeImageReplicateTool(imageReplicateConfig(srv.URL))

	return srv.Close, handler
}
