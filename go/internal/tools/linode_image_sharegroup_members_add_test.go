package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const imageShareGroupMembersAddToolName = "linode_image_sharegroup_member_add"

func TestLinodeImageShareGroupMembersAddToolDefinition(t *testing.T) {
	t.Parallel()

	tool, capability, handler := tools.NewLinodeImageShareGroupMembersAddTool(&config.Config{})

	if tool.Name != imageShareGroupMembersAddToolName {
		t.Errorf("tool.Name = %v, want %v", tool.Name, imageShareGroupMembersAddToolName)
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	for _, key := range []string{keyShareGroupID, keyLabel, keyToken, keyConfirm} {
		if _, ok := tool.InputSchema.Properties[key]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", key)
		}
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeImageShareGroupMembersAddToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/sharegroups/54321/members" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/54321/members")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+tokenTest {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+tokenTest)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyLabel], memberLabelFixture) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], memberLabelFixture)
		}

		if !reflect.DeepEqual(body[keyToken], memberTokenFixture) {
			t.Errorf("body[keyToken] = %v, want %v", body[keyToken], memberTokenFixture)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyBetaID:       54321,
			keyLabel:        "Engineering Share Group",
			keyDescription:  "Shared engineering images",
			"members_count": 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(imageShareGroupMembersAddConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 54321,
		keyLabel:        " Engineering ",
		keyToken:        " member-token ",
		keyConfirm:      true,
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

	if !strings.Contains(textContent.Text, "Added members") {
		t.Errorf("textContent.Text does not contain %v", "Added members")
	}

	if !strings.Contains(textContent.Text, "Engineering Share Group") {
		t.Errorf("textContent.Text does not contain %v", "Engineering Share Group")
	}
}

func TestLinodeImageShareGroupMembersAddToolValidationClientErrorConfirm(t *testing.T) {
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

func TestLinodeImageShareGroupMembersAddToolValidationClientErrorArgs(t *testing.T) {
	t.Parallel()

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

func TestLinodeImageShareGroupMembersAddToolValidationClientErrorDirect(t *testing.T) {
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
	_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(imageShareGroupMembersAddConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyShareGroupID: 54321,
		keyLabel:        memberLabelFixture,
		keyToken:        memberTokenFixture,
		keyConfirm:      true,
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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to add members to image share group") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to add members to image share group")
	}
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
