package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const imageReplicateToolName = "linode_image_replicate"

const regionUSMiami = "us-mia"

func TestLinodeImageReplicateToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageReplicateTool(&config.Config{})

	if tool.Name != imageReplicateToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageReplicateToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keyImageID]; !ok {
		t.Errorf("props missing key %v", keyImageID)
	}

	if _, ok := props[keyRegions]; !ok {
		t.Errorf("props missing key %v", keyRegions)
	}

	if _, ok := props[keyConfirm]; !ok {
		t.Errorf("props missing key %v", keyConfirm)
	}

	for _, key := range []string{keyImageID, keyRegions, keyConfirm} {
		if !slices.Contains(tool.InputSchema.Required, key) {
			t.Errorf("tool.InputSchema.Required does not contain %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageReplicateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.EscapedPath() != "/images/private%2F123/regions" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/private%2F123/regions")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyRegions], []any{regionUSMiami, regionUSEast}) {
			t.Errorf("body[keyRegions] = %v, want %v", body[keyRegions], []any{regionUSMiami, regionUSEast})
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID: privateImage123Fixture,
			keyLabel:  "replicated-image",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageReplicateTool(imageReplicateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyImageID: privateImage123Fixture,
		keyRegions: `["us-mia","us-east"]`,
		keyConfirm: true,
	}))
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

	if !strings.Contains(textContent.Text, "replicated successfully") {
		t.Errorf("textContent.Text does not contain %v", "replicated successfully")
	}

	if !strings.Contains(textContent.Text, privateImage123Fixture) {
		t.Errorf("textContent.Text does not contain %v", privateImage123Fixture)
	}
}

func TestLinodeImageReplicateToolValidationClientErrorConfirm(t *testing.T) {
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
		})
	}
}

func TestLinodeImageReplicateToolValidationClientErrorTt(t *testing.T) {
	t.Parallel()

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
		{name: caseBlankRegion, args: map[string]any{keyImageID: privateImage123Fixture, keyRegions: `["us-east",""]`, keyConfirm: true}, want: "regions entries must be non-empty strings"},
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, tt.want) {
				t.Errorf("error text %q does not contain %q", text.Text, tt.want)
			}

			if calls.Load() != int32(0) {
				t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
			}
		})
	}
}

func TestLinodeImageReplicateToolValidationClientErrorDirect(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	_, _, handler := tools.NewLinodeImageReplicateTool(imageReplicateConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyImageID: privateImage123Fixture,
		keyRegions: singleRegionJSON,
		keyConfirm: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to replicate image") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to replicate image")
	}
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

		if err := json.NewEncoder(w).Encode(map[string]any{keyBetaID: privateImage123Fixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))

	_, _, handler := tools.NewLinodeImageReplicateTool(imageReplicateConfig(srv.URL))

	return srv.Close, handler
}
