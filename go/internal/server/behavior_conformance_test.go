package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// behaviorFixture is one shared cross-language behavior contract file from
// testdata/behavior/. The Python runner (tests/unit/test_behavior_conformance.py)
// replays the same cases, so a handler whose validation, coercion, error text,
// or outgoing HTTP request drifts from the other language fails one of the two
// runners. The HTTP layer is an in-process fake: no network, no credentials.
type behaviorFixture struct {
	Tool  string         `json:"tool"`
	Cases []behaviorCase `json:"cases"`
}

// behaviorCase drives one dispatch. Exactly one outcome field is set:
// ExpectError (a local validation failure, bare message text), ExpectAPIError
// (a response-handling failure after at least one HTTP call), ExpectRequest (the
// one HTTP call the handler must make), or ExpectResult (the successful response
// content, compared as parsed JSON so formatting is irrelevant).
//
// The fake API answers from APIResponses when present: a map keyed
// "METHOD /path" (no query string, so per-language pagination params don't
// fragment the routing) whose values are the JSON bodies to serve. A request
// with no matching key fails the case, but an unused key does not:
// implementations may fetch equivalent data from different endpoints, and
// the contract these fixtures pin is the OUTPUT, not the fetch pattern.
// Without APIResponses the single APIResponse (or {}) answers every request.
//
// A case whose args include dry_run:true additionally asserts that every
// captured request is a GET: a dry run may read whatever it needs to build
// its preview but must never mutate.
type behaviorCase struct {
	Name           string                     `json:"name"`
	Args           map[string]any             `json:"args"`
	APIResponse    json.RawMessage            `json:"api_response"`
	APIResponses   map[string]json.RawMessage `json:"api_responses"`
	ExpectError    string                     `json:"expect_error"`
	ExpectAPIError bool                       `json:"expect_api_error"`
	ExpectRequest  *behaviorRequest           `json:"expect_request"`
	ExpectResult   json.RawMessage            `json:"expect_result"`
}

// behaviorRequest is the expected outgoing HTTP call: method, path (with any
// query string), and the JSON body compared structurally.
type behaviorRequest struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body"`
}

// capturedRequest is one HTTP request the fake transport observed.
type capturedRequest struct {
	method string
	path   string
	body   []byte
}

// decodeBehaviorResult extracts isError and the first content text from a
// marshaled tools/call JSON-RPC response. Decoded through maps because the
// MCP envelope uses camelCase keys (isError) that the repo's JSON tag lint
// rejects on struct tags.
func decodeBehaviorResult(t *testing.T, rawResponse []byte) (bool, string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(rawResponse, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, hasResult := decoded["result"].(map[string]any)
	if !hasResult {
		t.Fatalf("no result in response: %s", rawResponse)
	}

	isError, _ := result["isError"].(bool)

	content, hasContent := result["content"].([]any)
	if !hasContent || len(content) == 0 {
		t.Fatalf("no content in response: %s", rawResponse)
	}

	first, isObject := content[0].(map[string]any)
	if !isObject {
		t.Fatalf("unexpected content shape: %s", rawResponse)
	}

	text, _ := first["text"].(string)

	return isError, text
}

// TestBehaviorConformance replays every shared behavior fixture through the
// real server dispatch path (registration, profile filter, middleware,
// handler, client) with the HTTP transport faked by httptest.
func TestBehaviorConformance(t *testing.T) {
	t.Parallel()

	dir := filepath.Join("..", "..", "..", "testdata", "behavior")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // fixture path from testdata
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var fixture behaviorFixture
		if err := json.Unmarshal(raw, &fixture); err != nil {
			t.Fatalf("%s: unexpected error: %v", entry.Name(), err)
		}

		for _, testCase := range fixture.Cases {
			t.Run(fixture.Tool+"/"+testCase.Name, func(t *testing.T) {
				t.Parallel()
				runBehaviorCase(t, fixture.Tool, &testCase)
			})
		}
	}
}

// resolveBehaviorResponse picks the body and status for one fake-API request.
// Routed mode (api_responses) matches on "METHOD /path" with the query string
// stripped; a miss serves 404 and reports notFound so the case fails loudly.
func resolveBehaviorResponse(testCase *behaviorCase, method, path string) (json.RawMessage, int, bool) {
	if testCase.APIResponses == nil {
		response := testCase.APIResponse
		if response == nil {
			response = json.RawMessage(`{}`)
		}

		return response, http.StatusOK, true
	}

	response, ok := testCase.APIResponses[method+" "+path]
	if !ok {
		return json.RawMessage(`{}`), http.StatusNotFound, false
	}

	return response, http.StatusOK, true
}

// runBehaviorCase dispatches one case and checks its expected outcome.
func runBehaviorCase(t *testing.T, toolName string, testCase *behaviorCase) {
	t.Helper()

	var (
		captureMu sync.Mutex
		captured  []capturedRequest
		unmatched []string
	)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		record := capturedRequest{method: r.Method, path: r.URL.RequestURI(), body: body}
		response, status, matched := resolveBehaviorResponse(testCase, r.Method, r.URL.Path)

		func() {
			captureMu.Lock()
			defer captureMu.Unlock()

			captured = append(captured, record)

			if !matched {
				unmatched = append(unmatched, r.Method+" "+r.URL.Path)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	defer apiSrv.Close()

	cfg := fullAccessConfig()
	cfg.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label:  envLabelDefault,
		Linode: config.LinodeConfig{APIURL: apiSrv.URL, Token: tokenShort},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": toolName, "arguments": testCase.Args},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rawResponse, err := json.Marshal(srv.HandleMessage(t.Context(), message))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	isError, text := decodeBehaviorResult(t, rawResponse)

	if len(unmatched) > 0 {
		t.Errorf("requests with no api_responses entry: %s", strings.Join(unmatched, ", "))
	}

	if dryRun, _ := testCase.Args["dry_run"].(bool); dryRun {
		checkBehaviorNoMutation(t, captured)
	}

	switch {
	case testCase.ExpectError != "":
		checkBehaviorError(t, isError, text, testCase.ExpectError)
	case testCase.ExpectAPIError:
		checkBehaviorAPIError(t, isError, text, captured)
	case testCase.ExpectResult != nil:
		checkBehaviorResult(t, isError, text, testCase.ExpectResult)
	default:
		checkBehaviorRequest(t, isError, text, captured, testCase.ExpectRequest)
	}
}

// checkBehaviorAPIError asserts that response handling failed after the tool
// reached the fake API. Error text is deliberately language-specific.
func checkBehaviorAPIError(t *testing.T, isError bool, text string, captured []capturedRequest) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if len(captured) == 0 {
		t.Fatal("captured 0 requests, want at least 1")
	}
}

// checkBehaviorNoMutation asserts a dry-run case only ever read: the walk may
// GET whatever it needs for the preview, but any other verb is a mutation the
// dry-run contract forbids.
func checkBehaviorNoMutation(t *testing.T, captured []capturedRequest) {
	t.Helper()

	for _, request := range captured {
		if request.method != http.MethodGet {
			t.Errorf("dry-run case issued %s %s; only GET is allowed", request.method, request.path)
		}
	}
}

// checkBehaviorResult asserts a successful response whose content equals the
// expected JSON value. Comparison happens on parsed values so whitespace and
// key order are irrelevant; both languages must produce the same content for
// the same fixture, which is the cross-language contract this pins.
func checkBehaviorResult(t *testing.T, isError bool, text string, want json.RawMessage) {
	t.Helper()

	if isError {
		t.Fatalf("isError = true, want false (text %q)", text)
	}

	var gotValue, wantValue any

	if err := json.Unmarshal([]byte(text), &gotValue); err != nil {
		t.Fatalf("result is not JSON: %v\nresult text:\n%s", err, text)
	}

	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("result mismatch\ngot:\n%s\nwant:\n%s", text, want)
	}
}

// checkBehaviorError asserts a local validation failure with the exact bare
// message. Go emits the message unwrapped; the Python runner strips its
// "Error: " prefix so both compare against the same fixture text.
func checkBehaviorError(t *testing.T, isError bool, text, want string) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if text != want {
		t.Errorf("error text = %q, want %q", text, want)
	}
}

// checkBehaviorRequest asserts the handler made exactly the expected HTTP
// call and did not error locally.
func checkBehaviorRequest(t *testing.T, isError bool, text string, captured []capturedRequest, want *behaviorRequest) {
	t.Helper()

	if want == nil {
		t.Fatal("case has neither expect_error nor expect_request")
	}

	if isError {
		t.Fatalf("isError = true, want false (text %q)", text)
	}

	if len(captured) != 1 {
		t.Fatalf("captured %d requests, want 1", len(captured))
	}

	got := captured[0]

	if got.method != want.Method {
		t.Errorf("method = %q, want %q", got.method, want.Method)
	}

	if got.path != want.Path {
		t.Errorf("path = %q, want %q", got.path, want.Path)
	}

	if want.Body == nil {
		return
	}

	var gotBody, wantBody any

	if err := json.Unmarshal(got.body, &gotBody); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := json.Unmarshal(want.Body, &wantBody); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("request body = %s, want %s", got.body, want.Body)
	}
}
