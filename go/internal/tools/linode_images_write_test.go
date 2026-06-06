package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeImageShareGroupCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		assertEqual(t, "linode_image_sharegroup_create", tool.Name, "tool name should match")
		assertEqual(t, profiles.CapWrite, capability, "tool should be write capability")
		assertNotEmpty(t, tool.Description, "tool should have a description")
		assertContains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		assertContains(t, tool.InputSchema.Properties, keyImages, "schema should include images")
		assertContains(t, tool.InputSchema.Properties, keyConfirm, "mutating create tool must require confirm")
		requireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		description := shareGroupDescription
		updated := shareGroupUpdated

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assertEqual(t, http.MethodPost, r.Method, "request method should be POST")
			assertEqual(t, "/images/sharegroups", r.URL.Path, "request path should be /images/sharegroups")
			assertEmpty(t, r.URL.RawQuery, "request query should be empty")
			assertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			if !assertNoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assertEqual(t, imageShareGroupLabel, body[keyLabel])
			assertEqual(t, description, body[keyDescription])

			if !assertLen(t, body[keyImages], 1) {
				return
			}

			image, ok := body[keyImages].([]any)[0].(map[string]any)
			if !assertTrue(t, ok, "image payload should be an object") {
				return
			}

			assertEqual(t, "private/7", image[keyBetaID])

			w.Header().Set("Content-Type", "application/json")
			assertNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroup{
				ID:           1,
				UUID:         shareGroupUUIDExample,
				Label:        imageShareGroupLabel,
				Description:  &description,
				IsSuspended:  false,
				Created:      shareGroupCreated,
				Updated:      &updated,
				ImagesCount:  1,
				MembersCount: 0,
			}))
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
		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:       imageShareGroupLabel,
			keyDescription: description,
			keyImages:      `[{"id":" private/7 ","label":"Linux Debian"}]`,
			keyConfirm:     true,
		})
		result, err := handler(t.Context(), req)

		requireNoError(t, err, "handler should not return an error")
		requireNotNil(t, result, "result should not be nil")
		assertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		requireTrue(t, ok, "content should be TextContent")
		assertContains(t, textContent.Text, "created successfully", "response should include success message")
		assertContains(t, textContent.Text, imageShareGroupLabel, "response should contain share group label")
	})
}

func TestLinodeImageShareGroupCreateToolValidation(t *testing.T) {
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

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)

				w.WriteHeader(http.StatusOK)
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
			_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

			args := map[string]any{keyLabel: imageShareGroupLabel}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			requireNoError(t, err)
			requireNotNil(t, result)
			assertTrue(t, result.IsError, "missing or invalid confirm should be an error result")
			assertEqual(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	t.Run("missing label", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyConfirm: true}))

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "missing label should be an error result")
		assertErrorContains(t, result, "label is required")
	})

	t.Run("invalid images JSON", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:   imageShareGroupLabel,
			keyImages:  `[{"label":"missing id"}]`,
			keyConfirm: true,
		}))

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "image without id should be an error result")
		assertErrorContains(t, result, "image id is required")
	})
	t.Run("malformed images JSON", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:   imageShareGroupLabel,
			keyImages:  `[{`,
			keyConfirm: true,
		}))

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "malformed images JSON should be an error result")
		assertErrorContains(t, result, "invalid images JSON")
	})

	t.Run("non-string images rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)

			w.WriteHeader(http.StatusOK)
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
		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:   imageShareGroupLabel,
			keyImages:  []any{map[string]any{keyBetaID: "private/7"}},
			keyConfirm: true,
		}))

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "non-string images should be an error result")
		assertErrorContains(t, result, "images must be a JSON string")
		assertEqual(t, int32(0), calls.Load(), "images validation must happen before client call")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			assertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))
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
		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:   imageShareGroupLabel,
			keyConfirm: true,
		}))

		requireNoError(t, err)
		requireNotNil(t, result)
		assertTrue(t, result.IsError, "upstream API error should be an error result")
		assertErrorContains(t, result, "Failed to create image share group")
	})
}
