package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const imageShareGroupMembersAddToolName = "linode_image_sharegroup_members_add"

func TestLinodeImageShareGroupMembersAddTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeImageShareGroupMembersAddTool(&config.Config{})

		shareGroupAssertEqual(t, imageShareGroupMembersAddToolName, tool.Name, "tool name should match")
		shareGroupAssertEqual(t, profiles.CapWrite, capability, "tool should be write capability")
		shareGroupAssertNotEmpty(t, tool.Description, "tool should have a description")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyShareGroupID, "schema should include sharegroup_id")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyToken, "schema should include token")
		shareGroupAssertContains(t, tool.InputSchema.Properties, keyConfirm, "mutating member add tool must require confirm")
		shareGroupRequireNotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			shareGroupAssertEqual(t, http.MethodPost, r.Method, "request method should be POST")
			shareGroupAssertEqual(t, "/images/sharegroups/54321/members", r.URL.Path, "request path should include share group ID")
			shareGroupAssertEmpty(t, r.URL.RawQuery, "request query should be empty")
			shareGroupAssertEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			shareGroupAssertNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			shareGroupAssertEqual(t, memberLabelFixture, body[keyLabel], "label should be sent")
			shareGroupAssertEqual(t, memberTokenFixture, body[keyToken], "token should be sent")

			w.Header().Set("Content-Type", "application/json")
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID:       54321,
				keyLabel:        "Engineering Share Group",
				keyDescription:  "Shared engineering images",
				"members_count": 1,
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(imageShareGroupMembersAddConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 54321,
			keyLabel:        " Engineering ",
			keyToken:        " member-token ",
			keyConfirm:      true,
		}))

		shareGroupRequireNoError(t, err, "handler should not return an error")
		shareGroupRequireNotNil(t, result, "result should not be nil")
		shareGroupAssertFalse(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		shareGroupRequireTrue(t, ok, "content should be TextContent")
		shareGroupAssertContains(t, textContent.Text, "Added members", "response should include success message")
		shareGroupAssertContains(t, textContent.Text, "Engineering Share Group", "response should include share group label")
	})
}

func TestLinodeImageShareGroupMembersAddToolValidation(t *testing.T) {
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

			closeServer, handler := imageShareGroupMembersAddHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			args := map[string]any{keyShareGroupID: 54321, keyLabel: memberLabelFixture, keyToken: memberTokenFixture}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			shareGroupRequireNoError(t, err)
			shareGroupRequireNotNil(t, result)
			shareGroupAssertTrue(t, result.IsError, "missing or invalid confirm should be an error result")
			shareGroupAssertEqual(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	for name, args := range map[string]map[string]any{
		"invalid sharegroup id":            {keyShareGroupID: 0, keyLabel: memberLabelFixture, keyToken: memberTokenFixture, keyConfirm: true},
		caseSlashShareGroupID:              {keyShareGroupID: "12/34", keyLabel: memberLabelFixture, keyToken: memberTokenFixture, keyConfirm: true},
		caseQueryShareGroupID:              {keyShareGroupID: "12?34", keyLabel: memberLabelFixture, keyToken: memberTokenFixture, keyConfirm: true},
		caseDotTraversal:                   {keyShareGroupID: pathTraversalValue, keyLabel: memberLabelFixture, keyToken: memberTokenFixture, keyConfirm: true},
		caseMissingLabel:                   {keyShareGroupID: 54321, keyToken: memberTokenFixture, keyConfirm: true},
		caseBlankLabelImageShareGroupToken: {keyShareGroupID: 54321, keyLabel: blankString, keyToken: memberTokenFixture, keyConfirm: true},
		"numeric label":                    {keyShareGroupID: 54321, keyLabel: 123, keyToken: memberTokenFixture, keyConfirm: true},
		"missing token":                    {keyShareGroupID: 54321, keyLabel: memberLabelFixture, keyConfirm: true},
		"blank token":                      {keyShareGroupID: 54321, keyLabel: memberLabelFixture, keyToken: blankString, keyConfirm: true},
		"numeric token":                    {keyShareGroupID: 54321, keyLabel: memberLabelFixture, keyToken: 123, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			closeServer, handler := imageShareGroupMembersAddHandlerWithCallCounter(t, &calls)
			t.Cleanup(closeServer)

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			shareGroupRequireNoError(t, err)
			shareGroupRequireNotNil(t, result)
			shareGroupAssertTrue(t, result.IsError, "invalid input should be an error result")
			shareGroupAssertEqual(t, int32(0), calls.Load(), "input validation must happen before client call")
		})
	}

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			shareGroupAssertNoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))
		}))
		t.Cleanup(srv.Close)

		_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(imageShareGroupMembersAddConfig(srv.URL))
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: 54321,
			keyLabel:        memberLabelFixture,
			keyToken:        memberTokenFixture,
			keyConfirm:      true,
		}))

		shareGroupRequireNoError(t, err)
		shareGroupRequireNotNil(t, result)
		shareGroupAssertTrue(t, result.IsError, "upstream API error should be an error result")
		assertErrorContains(t, result, "Failed to add members to image share group")
	})
}

func imageShareGroupMembersAddConfig(apiURL string) *config.Config {
	return &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {
				Label:  envLabelDefault,
				Linode: config.LinodeConfig{APIURL: apiURL, Token: tokenTest},
			},
		},
	}
}

func imageShareGroupMembersAddHandlerWithCallCounter(
	t *testing.T,
	calls *atomic.Int32,
) (func(), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))

	_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(imageShareGroupMembersAddConfig(srv.URL))

	return srv.Close, handler
}
