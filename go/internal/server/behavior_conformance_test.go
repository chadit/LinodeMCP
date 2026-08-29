package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/genlocal"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
	"github.com/chadit/LinodeMCP/go/internal/server"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// behaviorFixture is one shared cross-language behavior contract file from
// testdata/behavior/. The Python runner (tests/unit/test_behavior_conformance.py)
// replays the same cases, so a handler whose validation, coercion, error text,
// or outgoing HTTP request drifts from the other language fails one of the two
// runners. The HTTP layer is an in-process fake: no network, no credentials.
type behaviorFixture struct {
	Tool  string            `json:"tool"`
	Cases []json.RawMessage `json:"cases"`
}

// behaviorCase drives one dispatch. Exactly one outcome field is set:
// ExpectError (an exact local validation failure that forbids HTTP calls),
// ExpectAPIError (a shared substring in an error after at least one HTTP call),
// ExpectRequest (the one HTTP call the handler must make), or ExpectResult (the
// successful response content, compared as parsed JSON so formatting is
// irrelevant). ExpectAPIError uses a substring so language-specific error
// framing stays outside the shared contract.
//
// The fake API answers from APIResponses when present: a map keyed
// "METHOD /path" (no query string, so per-language pagination params don't
// fragment the routing) whose values are the JSON bodies to serve. A request
// with no matching key fails the case, but an unused key does not:
// implementations may fetch equivalent data from different endpoints, and
// the contract these fixtures pin is the OUTPUT, not the fetch pattern.
// Without APIResponses the single APIResponse (or {}) answers every request.
// APIResponseRaw serves exact bytes instead, including empty or malformed JSON,
// and APIStatus overrides HTTP 200 in single-response mode.
//
// A case whose args include dry_run:true additionally asserts that every
// captured request is a GET: a dry run may read whatever it needs to build
// its preview but must never mutate.
type behaviorCase struct {
	Name           string                     `json:"name"`
	Args           map[string]any             `json:"args"`
	APIResponse    json.RawMessage            `json:"api_response"`
	APIResponseRaw *string                    `json:"api_response_raw"`
	APIStatus      *int                       `json:"api_status"`
	APIResponses   map[string]json.RawMessage `json:"api_responses"`
	// APIResponseHeaders answers headers per routed key, for a leg whose result
	// is a header rather than a body: a presigned PUT reports the stored
	// object's entity tag in ETag and sends no JSON at all.
	APIResponseHeaders map[string]map[string]string `json:"api_response_headers"`
	// Files are materialized into a temp directory before the case runs, so a
	// tool taking a local path has something real to read. {{file:name}}
	// substitutes the absolute path into any string argument.
	Files map[string]behaviorFile `json:"files"`
	// AuditStore seeds the audit backing store before the server starts, which
	// is what gives the audit query tools a reproducible thing to read.
	AuditStore     *behaviorAuditStore `json:"audit_store"`
	Config         *behaviorConfig     `json:"config"`
	ExpectAPIError string              `json:"expect_api_error"`
	ExpectError    string              `json:"expect_error"`
	// Now fixes the reference clock, RFC 3339. The audit report resolves a
	// relative since_offset against it; a wall-clock window has no pinnable
	// answer.
	Now           string           `json:"now"`
	ExpectRequest *behaviorRequest `json:"expect_request"`
	// ExpectRequests pins several calls in order, beside whichever outcome the
	// case asserts. ExpectRequest holds a tool to exactly one call, which a
	// two-leg tool cannot satisfy: the Object Storage transfers presign first
	// and then move the bytes, so the body the presign carries has no other
	// place to be pinned.
	ExpectRequests []behaviorRequest `json:"expect_requests"`
	ExpectResult   json.RawMessage   `json:"expect_result"`
	// HostDecided names the answer fields whose exact value the host or the
	// database driver picks, so expect_result cannot spell them.
	HostDecided []behaviorHostDecided `json:"host_decided"`
	// SetupCalls run against the SAME server before the case's own call, and
	// their answers are not asserted. Drafts live in server-process memory, so
	// this is the only way a builder success path is reachable at all: the draft
	// a show or a save acts on exists because an earlier call made it.
	SetupCalls []behaviorSetupCall `json:"setup_calls"`
}

// behaviorCaseFields returns every key a case may carry. A field one runner
// honors and the other ignores is exactly the drift these fixtures exist to
// stop, so an unrecognized key fails the case in both languages rather than
// passing quietly in one.
func behaviorCaseFields() map[string]bool {
	return map[string]bool{
		"name": true, "args": true, "api_response": true, "api_response_raw": true,
		"api_status": true, "api_responses": true, "api_response_headers": true,
		"files": true, "config": true, "expect_api_error": true, "expect_error": true,
		"expect_request": true, "expect_requests": true, "expect_result": true,
		"setup_calls": true, "audit_store": true, "now": true, "host_decided": true,
	}
}

// behaviorSetupCall is one prior tools/call a case sends before its own.
type behaviorSetupCall struct {
	Args map[string]any `json:"args"`
	Tool string         `json:"tool"`
}

// Type names a host-decided field may declare, and the two audit backends a
// seeded store may choose.
const (
	hostDecidedString   = "string"
	hostDecidedInteger  = "integer"
	behaviorStoreJSONL  = "jsonl"
	behaviorStoreSQLite = "sqlite"
)

// behaviorHostDecided names one answer field whose exact value the host or the
// database driver picks, beside what the fixture DOES assert about it: a
// pattern for a string, a floor for an integer. Everything a case does not name
// stays a whole-answer literal, so naming a field spends part of the pin and is
// allowed only where no literal holds on both a developer machine and the CI
// container. Reason says which, in the fixture, where the next reader sees it.
type behaviorHostDecided struct {
	Min     *int64 `json:"min"`
	Path    string `json:"path"`
	Type    string `json:"type"`
	Pattern string `json:"pattern"`
	Reason  string `json:"reason"`
}

// behaviorAuditStore is the audit state a case starts from. Events are
// re-encoded canonically rather than copied out of the fixture text, so both
// languages write byte-identical files and a health answer can pin disk_bytes.
type behaviorAuditStore struct {
	// Backend selects which store the events land in, "" meaning jsonl. A
	// sqlite case seeds ONLY the database, so an answer carrying the events
	// proves the tool read it rather than the log beside it.
	Backend string            `json:"backend"`
	Events  []json.RawMessage `json:"events"`
}

// behaviorPaths are the per-run locations the runner owns and a fixture names
// through a token, because neither can be spelled in a shared contract file.
type behaviorPaths struct {
	auditDir   string
	sqlitePath string
}

// stateful reports whether the case needs the process-scoped setup: a prior
// call against the same server, a seeded store, or a fixed clock. Those cases
// redirect the audit and config paths through the environment, which no test
// can do in parallel, so they run in their own serial lane.
func (c *behaviorCase) stateful() bool {
	return len(c.SetupCalls) > 0 || c.AuditStore != nil || c.Now != ""
}

// behaviorFile is one file a case needs on disk. Content writes literal text;
// Size and Fill generate a file too large to spell out in the fixture, which is
// how the over-ceiling case stays a 2 KB file rather than a 5 GiB one.
type behaviorFile struct {
	Content string `json:"content"`
	Fill    string `json:"fill"`
	Size    int    `json:"size"`
}

// behaviorConfig is the narrow config overlay a case may apply. Only the
// Object Storage data-plane block is reachable, by name: a general overlay
// would let a fixture change the profile or the auth it is being tested under,
// and a fixture that can change what it is being tested under is not a fixture.
//
// The keys are snake_case like the rest of the fixture schema, not the
// camelCase the operator's YAML uses: this is a fixture contract, and it is
// read by both runners rather than by the config loader.
type behaviorConfig struct {
	ObjectStorage *behaviorObjectStorage `json:"object_storage"`
	Audit         *behaviorAudit         `json:"audit"`
}

// behaviorAudit overlays the named reports a case runs under. Only the report
// catalog is reachable: it is the one audit setting whose absence makes a tool
// unpinnable, and it cannot change the profile or the auth the case runs under.
type behaviorAudit struct {
	Reports map[string]config.ReportConfig `json:"reports"`
}

// behaviorObjectStorage overlays the data-plane budgets a case runs under.
type behaviorObjectStorage struct {
	MaxSinglePartBytes int64 `json:"max_single_part_bytes"`
}

// behaviorRequest is the expected outgoing HTTP call: method, path (with any
// query string), and the JSON body compared structurally.
type behaviorRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	// APISurface is the base the call must go to, "" meaning v4. Absent is what
	// nearly every fixture says, so a tool that moved to another surface without
	// its fixture moving with it fails here rather than passing quietly.
	APISurface string          `json:"api_surface"`
	Body       json.RawMessage `json:"body"`
}

// surface is the declared surface, with the default filled in.
func (r *behaviorRequest) surface() string {
	if r.APISurface == "" {
		return linoderoute.DefaultSurfaceSegment
	}

	return r.APISurface
}

// splitSurface takes the leading version segment off a request path, answering
// it beside the rest. The fake base ends in one, so every request carries it;
// a path arriving without one means the client dropped the base's version.
func splitSurface(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/")

	segment, rest, found := strings.Cut(trimmed, "/")
	if !found {
		return segment, "/"
	}

	return segment, "/" + rest
}

// capturedRequest is one HTTP request the fake transport observed.
type capturedRequest struct {
	method  string
	path    string
	surface string
	body    []byte
}

// behaviorOutcomeCount returns the number of usable outcome assertions set on
// a case. Empty error strings do not assert anything and are therefore invalid.
func behaviorOutcomeCount(testCase *behaviorCase) int {
	var count int

	if testCase.ExpectError != "" {
		count++
	}

	if testCase.ExpectAPIError != "" {
		count++
	}

	if testCase.ExpectRequest != nil {
		count++
	}

	if testCase.ExpectResult != nil {
		count++
	}

	return count
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

// callServerTool dispatches one tools/call through the real server and returns
// the answer text with its error flag. One copy of the JSON-RPC envelope keeps
// the tests that drive the server through its own wire from drifting apart, and
// it is what a fixture's setup calls reuse to reach the same server N+1 times.
func callServerTool(
	t *testing.T, srv *server.Server, clock func() time.Time, name string, args map[string]any,
) (bool, string) {
	t.Helper()

	message, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{callNameKey: name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A nil clock leaves the context untouched, so every caller but a fixture
	// that fixed one still reads the wall clock.
	response := srv.HandleMessage(tools.WithClock(t.Context(), clock), message)

	rawResponse, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return decodeBehaviorResult(t, rawResponse)
}

// loadBehaviorFixtures reads every shared fixture as (tool, decoded case),
// checking each case's keys against the schema this runner implements.
func loadBehaviorFixtures(t *testing.T) []behaviorNamedCase {
	t.Helper()

	dir := filepath.Join("..", "..", "..", "testdata", "behavior")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var loaded []behaviorNamedCase

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var fixture behaviorFixture
		if err := json.Unmarshal(raw, &fixture); err != nil {
			t.Fatalf("%s: unexpected error: %v", entry.Name(), err)
		}

		for _, rawCase := range fixture.Cases {
			loaded = append(loaded, decodeBehaviorCase(t, entry.Name(), fixture.Tool, rawCase))
		}
	}

	return loaded
}

// behaviorNamedCase is one decoded case beside the tool it drives.
type behaviorNamedCase struct {
	testCase *behaviorCase
	tool     string
}

// decodeBehaviorCase decodes one case and refuses any key the schema does not
// declare, so a fixture cannot ask for something only one runner implements.
func decodeBehaviorCase(t *testing.T, file, tool string, raw json.RawMessage) behaviorNamedCase {
	t.Helper()

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("%s: unexpected error: %v", file, err)
	}

	known := behaviorCaseFields()

	var unknown []string

	for key := range keys {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}

	if len(unknown) > 0 {
		sort.Strings(unknown)
		t.Fatalf("%s: case declares unknown field(s): %s", file, strings.Join(unknown, ", "))
	}

	testCase := new(behaviorCase)
	if err := json.Unmarshal(raw, testCase); err != nil {
		t.Fatalf("%s: unexpected error: %v", file, err)
	}

	return behaviorNamedCase{tool: tool, testCase: testCase}
}

// TestBehaviorConformance replays every shared behavior fixture through the
// real server dispatch path (registration, profile filter, middleware,
// handler, client) with the HTTP transport faked by httptest.
//
// Stateful cases run in TestBehaviorStatefulConformance instead: they redirect
// process-global paths, which no parallel test can do.
func TestBehaviorConformance(t *testing.T) {
	t.Parallel()

	for _, loaded := range loadBehaviorFixtures(t) {
		if loaded.testCase.stateful() {
			continue
		}

		t.Run(loaded.tool+"/"+loaded.testCase.Name, func(t *testing.T) {
			t.Parallel()
			runBehaviorCase(t, loaded.tool, loaded.testCase, behaviorPaths{})
		})
	}
}

// TestBehaviorStatefulConformance replays the cases that need a prior call, a
// seeded audit store, or a fixed clock. Each one redirects XDG_STATE_HOME and
// LINODEMCP_CONFIG_PATH into a directory it owns, which is process-global, so
// nothing here runs in parallel. The redirect is also what keeps a draft save
// off the operator's real config file.
func TestBehaviorStatefulConformance(t *testing.T) {
	// The lane redirects once here and again per case. The outer redirect is the
	// floor: no case in this lane can reach the operator's real audit directory
	// even if it seeds nothing.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	for _, loaded := range loadBehaviorFixtures(t) {
		if !loaded.testCase.stateful() {
			continue
		}

		t.Run(loaded.tool+"/"+loaded.testCase.Name, func(t *testing.T) {
			stateHome := t.TempDir()
			t.Setenv("XDG_STATE_HOME", stateHome)
			// A draft save writes config.Path() for real, so this redirect is
			// what keeps a success fixture off the operator's config file.
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			t.Setenv("LINODEMCP_CONFIG_PATH", configPath)
			seedBehaviorConfig(t, configPath)

			paths := behaviorPaths{auditDir: filepath.Join(stateHome, audit.UserAuditDirRelative)}
			if loaded.testCase.AuditStore != nil {
				paths.sqlitePath = seedBehaviorAuditStore(t, paths.auditDir, loaded.testCase.AuditStore)
			}

			runBehaviorCase(t, loaded.tool, loaded.testCase, paths)
		})
	}
}

func TestBehaviorOutcomeCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testCase behaviorCase
		want     int
	}{
		{name: "empty API error", testCase: behaviorCase{ExpectAPIError: ""}, want: 0},
		{
			name: "multiple outcomes",
			testCase: behaviorCase{
				ExpectAPIError: "response",
				ExpectResult:   json.RawMessage(`false`),
			},
			want: 2,
		},
		{name: "false result", testCase: behaviorCase{ExpectResult: json.RawMessage(`false`)}, want: 1},
		{
			name: "empty errors with result",
			testCase: behaviorCase{
				ExpectError:    "",
				ExpectAPIError: "",
				ExpectResult:   json.RawMessage(`false`),
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := behaviorOutcomeCount(&tt.testCase); got != tt.want {
				t.Errorf("behaviorOutcomeCount() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("unmarshaled null result", func(t *testing.T) {
		t.Parallel()

		var testCase behaviorCase
		if err := json.Unmarshal([]byte(`{"expect_result":null}`), &testCase); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if testCase.ExpectResult == nil {
			t.Fatal("ExpectResult = nil, want the JSON null literal")
		}

		if got := behaviorOutcomeCount(&testCase); got != 1 {
			t.Errorf("behaviorOutcomeCount() = %d, want 1", got)
		}
	})
}

func TestTakeHostDecided(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want    any
		name    string
		path    string
		present bool
	}{
		{name: "top level", path: "platform", want: "darwin/arm64", present: true},
		{name: "nested", path: "sqlite.db_bytes", want: 4096.0, present: true},
		{name: "renamed leaf", path: "sqlite.size_bytes", present: false},
		{name: "renamed parent", path: "store.db_bytes", present: false},
		{name: "leaf is not an object", path: "platform.db_bytes", present: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			seeded := []byte(`{"platform":"darwin/arm64","sqlite":{"db_bytes":4096}}`)

			var answer any
			if err := json.Unmarshal(seeded, &answer); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, present := takeHostDecided(answer, tt.path)
			if present != tt.present {
				t.Fatalf("present = %v, want %v", present, tt.present)
			}

			if present && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("value = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("a taken field leaves the rest alone", func(t *testing.T) {
		t.Parallel()

		answer := map[string]any{"platform": "darwin/arm64", "commit": "unknown"}
		if _, present := takeHostDecided(answer, "platform"); !present {
			t.Fatal("present = false, want true")
		}

		if !reflect.DeepEqual(answer, map[string]any{"commit": "unknown"}) {
			t.Errorf("remainder = %v, want only commit", answer)
		}
	})
}

func TestHostDecidedShapeRefusals(t *testing.T) {
	t.Parallel()

	floor := int64(1)
	stringField := behaviorHostDecided{
		Path: keyPath, Type: hostDecidedString, Pattern: "^/", Reason: "a temp directory",
	}
	integerField := behaviorHostDecided{
		Path: "db_bytes", Type: hostDecidedInteger, Min: &floor, Reason: "the driver",
	}

	noPattern := stringField
	noPattern.Pattern = ""

	noFloor := integerField
	noFloor.Min = nil

	noReason := stringField
	noReason.Reason = ""

	noPath := stringField
	noPath.Path = ""

	unknownType := stringField
	unknownType.Type = "any"

	tests := []struct {
		name  string
		field behaviorHostDecided
		valid bool
	}{
		{name: "string with a pattern", field: stringField, valid: true},
		{name: "integer with a floor", field: integerField, valid: true},
		{name: "string with no pattern", field: noPattern},
		{name: "integer with no floor", field: noFloor},
		{name: "no reason", field: noReason},
		{name: "no path", field: noPath},
		{name: "unknown type", field: unknownType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := hostDecidedEntryValid(&tt.field); got != tt.valid {
				t.Errorf("hostDecidedEntryValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// resolveBehaviorResponse picks the body and status for one fake-API request.
// Routed mode (api_responses) matches on "METHOD /path" with the query string
// stripped; a miss serves 404 and reports notFound so the case fails loudly.
func resolveBehaviorResponse(testCase *behaviorCase, method, path string) ([]byte, int, bool) {
	if testCase.APIResponses == nil {
		var response []byte

		switch {
		case testCase.APIResponseRaw != nil:
			response = []byte(*testCase.APIResponseRaw)
		case testCase.APIResponse != nil:
			response = testCase.APIResponse
		default:
			response = []byte(`{}`)
		}

		status := http.StatusOK
		if testCase.APIStatus != nil {
			status = *testCase.APIStatus
		}

		return response, status, true
	}

	response, ok := testCase.APIResponses[method+" "+path]
	if !ok {
		return []byte(`{}`), http.StatusNotFound, false
	}

	return response, http.StatusOK, true
}

// materializeBehaviorFiles writes the case's declared files into a temp
// directory and answers the arguments with {{file:name}} resolved to their
// absolute paths. A tool whose whole job is reading a local file cannot be
// pinned by a fixture that never puts one on disk.
//
// {{file_dir}} names the directory itself, which is what a tool that WRITES a
// file needs: a destination inside a directory the case owns and that no file
// occupies yet.
func materializeBehaviorFiles(
	t *testing.T, testCase *behaviorCase,
) map[string]any {
	t.Helper()

	dir := t.TempDir()
	defer func() {
		testCase.ExpectError = substituteFilePaths(testCase.ExpectError, dir, testCase.Files)
	}()

	paths := make(map[string]string, len(testCase.Files))

	for name, declared := range testCase.Files {
		content := []byte(declared.Content)
		if declared.Size > 0 {
			content = bytes.Repeat([]byte(declared.Fill), declared.Size)
		}

		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		paths[name] = path
	}

	resolved := make(map[string]any, len(testCase.Args))

	for key, value := range testCase.Args {
		text, isText := value.(string)
		if !isText {
			resolved[key] = value

			continue
		}

		for name, path := range paths {
			text = strings.ReplaceAll(text, "{{file:"+name+"}}", path)
		}

		resolved[key] = strings.ReplaceAll(text, "{{file_dir}}", dir)
	}

	return resolved
}

// substituteFilePaths resolves the file tokens in one string. The expected
// error text needs them as much as the arguments do: a refusal that names the
// local path it refused cannot be pinned any other way, because the path is a
// per-run temp directory.
func substituteFilePaths(text, dir string, files map[string]behaviorFile) string {
	for name := range files {
		text = strings.ReplaceAll(text, "{{file:"+name+"}}", filepath.Join(dir, name))
	}

	return strings.ReplaceAll(text, "{{file_dir}}", dir)
}

// substituteAPIBase resolves {{api_base}} in the routed responses. The fake
// server's port is picked at run time, so a fixture that has to name it (a
// presigned URL pointing back at the fake) cannot spell it out.
func substituteAPIBase(
	responses map[string]json.RawMessage, base string,
) map[string]json.RawMessage {
	resolved := make(map[string]json.RawMessage, len(responses))

	for key, value := range responses {
		resolved[key] = json.RawMessage(
			strings.ReplaceAll(string(value), "{{api_base}}", base),
		)
	}

	return resolved
}

// behaviorConfigFor builds the config one case runs under, pointed at the fake
// API and carrying the case's narrow overlay when it declared one.
func behaviorConfigFor(testCase *behaviorCase, apiURL string, paths behaviorPaths) *config.Config {
	cfg := fullAccessConfig()

	// The seeded database is what turns the sink on for the read path; nothing
	// in a fixture may reach the audit config directly.
	if paths.sqlitePath != "" {
		cfg.Audit.SQLite.Enabled = true
		cfg.Audit.SQLite.Path = paths.sqlitePath
	}

	if testCase.Config != nil && testCase.Config.ObjectStorage != nil {
		cfg.ObjectStorage.MaxSinglePartBytes = testCase.Config.ObjectStorage.MaxSinglePartBytes
	}

	if testCase.Config != nil && testCase.Config.Audit != nil {
		cfg.Audit.Reports = testCase.Config.Audit.Reports
	}

	cfg.Environments[envKeyDefault] = config.EnvironmentConfig{
		Label: envLabelDefault,
		// The version segment is what makes the fake base the shape a real
		// apiUrl has, so a surface swap can fire against it.
		Linode: config.LinodeConfig{
			APIURL: apiURL + "/" + linoderoute.DefaultSurfaceSegment,
			Token:  tokenShort,
		},
	}

	return cfg
}

// seedBehaviorConfig puts a loadable config at the redirected path. A draft
// save reads the file before it merges, so the redirect alone would answer a
// missing-file refusal instead of the success the fixture pins. Writing through
// the config package's own writer is what makes the seed loadable by
// definition rather than by a hand-kept literal.
func seedBehaviorConfig(t *testing.T, path string) {
	t.Helper()

	if err := config.WriteAtomic(path, fullAccessConfig()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// seedBehaviorAuditStore writes the case's events into the backend it declared
// and answers the database path when that backend is SQLite.
func seedBehaviorAuditStore(t *testing.T, dir string, store *behaviorAuditStore) string {
	t.Helper()

	switch store.Backend {
	case "", behaviorStoreJSONL:
		seedBehaviorJSONL(t, dir, store)

		return ""
	case behaviorStoreSQLite:
		return seedBehaviorSQLite(t, store)
	default:
		t.Fatalf("audit_store backend = %q, want jsonl or sqlite", store.Backend)

		return ""
	}
}

// seedBehaviorSQLite writes the case's events through the sink's own writer, so
// the seeded schema is the shipped schema rather than a hand-kept copy of it.
//
// The database sits outside the audit directory because the JSONL footprint
// counts every file beside the log: a database in there would make disk_bytes
// driver decided too, and the point of the backend is to keep exactly one
// field that way.
func seedBehaviorSQLite(t *testing.T, store *behaviorAuditStore) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "audit.db")

	sink, err := audit.NewSQLiteSink(t.Context(), path, config.DefaultAuditSQLiteBusyTimeoutMS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer func() { _ = sink.Close() }()

	for _, raw := range store.Events {
		event, err := genlocal.AuditEventFromRecord(canonicalEventLine(t, raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sink.Write(t.Context(), event)
	}

	return path
}

// seedBehaviorJSONL writes the case's events into the active JSONL log the
// audit readers walk. Both runners re-encode each event the same way, so the
// file's byte size is part of the shared contract rather than an accident of
// whichever language wrote it.
func seedBehaviorJSONL(t *testing.T, dir string, store *behaviorAuditStore) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var content bytes.Buffer

	for _, event := range store.Events {
		content.Write(canonicalEventLine(t, event))
		content.WriteString("\n")
	}

	path := filepath.Join(dir, audit.ActiveLogFileName)
	if err := os.WriteFile(path, content.Bytes(), 0o600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// canonicalEventLine re-encodes one seeded event with sorted keys, no HTML
// escaping, and integers left as written. UseNumber is what keeps a nanosecond
// timestamp from rounding through float64 and disagreeing with Python.
func canonicalEventLine(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var encoded bytes.Buffer

	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(value); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return bytes.TrimRight(encoded.Bytes(), "\n")
}

// behaviorClock answers the case's fixed clock, or nil for the wall clock.
func behaviorClock(t *testing.T, testCase *behaviorCase) func() time.Time {
	t.Helper()

	if testCase.Now == "" {
		return nil
	}

	fixed, err := time.Parse(time.RFC3339, testCase.Now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return func() time.Time { return fixed }
}

// substituteRunnerPaths resolves {{audit_dir}} and {{sqlite_path}} in an
// expected result. Both live in a per-run temp directory the runner picked, so
// an answer that reports its own store cannot be pinned any other way.
func substituteRunnerPaths(want json.RawMessage, paths behaviorPaths) json.RawMessage {
	text := string(want)

	if paths.auditDir != "" {
		text = strings.ReplaceAll(text, "{{audit_dir}}", paths.auditDir)
	}

	if paths.sqlitePath != "" {
		text = strings.ReplaceAll(text, "{{sqlite_path}}", paths.sqlitePath)
	}

	return json.RawMessage(text)
}

// checkHostDecidedShape refuses a declaration that would weaken the pin. A
// named field with no constraint is an ignored field, and a case with no
// expect_result has no answer to name a field in.
func checkHostDecidedShape(t *testing.T, testCase *behaviorCase) {
	t.Helper()

	if len(testCase.HostDecided) == 0 {
		return
	}

	if testCase.ExpectResult == nil {
		t.Fatal("host_decided needs expect_result: there is no answer to name a field in")
	}

	for index := range testCase.HostDecided {
		field := &testCase.HostDecided[index]
		if !hostDecidedEntryValid(field) {
			t.Fatalf("host_decided %q needs a reason, and must be a string with a"+
				" pattern or an integer with a min", field.Path)
		}
	}
}

// hostDecidedEntryValid reports whether one declaration still asserts something:
// a string carrying a pattern, or an integer carrying a floor. A type with
// neither would name a field and check nothing about it.
func hostDecidedEntryValid(field *behaviorHostDecided) bool {
	if field.Path == "" || field.Reason == "" {
		return false
	}

	stringShape := field.Type == hostDecidedString && field.Pattern != "" && field.Min == nil
	integerShape := field.Type == hostDecidedInteger && field.Min != nil && field.Pattern == ""

	return stringShape || integerShape
}

// takeHostDecided removes the field at a dotted path from a decoded answer and
// returns it, so everything left keeps its literal comparison.
func takeHostDecided(answer any, path string) (any, bool) {
	object, isObject := answer.(map[string]any)
	if !isObject {
		return nil, false
	}

	segments := strings.Split(path, ".")
	for _, segment := range segments[:len(segments)-1] {
		object, isObject = object[segment].(map[string]any)
		if !isObject {
			return nil, false
		}
	}

	leaf := segments[len(segments)-1]

	value, present := object[leaf]
	if !present {
		return nil, false
	}

	delete(object, leaf)

	return value, true
}

// checkHostDecidedValue asserts what the fixture does pin about a host-decided
// field: the declared type, and the pattern or the floor beside it.
func checkHostDecidedValue(t *testing.T, field *behaviorHostDecided, value any) {
	t.Helper()

	if field.Type == hostDecidedString {
		checkHostDecidedPattern(t, field, value)

		return
	}

	number, isNumber := value.(float64)
	if !isNumber || number != math.Trunc(number) {
		t.Fatalf("host-decided %q = %v, want an integer", field.Path, value)
	}

	if int64(number) < *field.Min {
		t.Errorf("host-decided %q = %d, want at least %d", field.Path, int64(number), *field.Min)
	}
}

// checkHostDecidedPattern asserts a host-decided string against its pattern.
func checkHostDecidedPattern(t *testing.T, field *behaviorHostDecided, value any) {
	t.Helper()

	text, isText := value.(string)
	if !isText {
		t.Fatalf("host-decided %q = %v, want a string", field.Path, value)
	}

	matched, err := regexp.MatchString(field.Pattern, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !matched {
		t.Errorf("host-decided %q = %q, want a match for %q", field.Path, text, field.Pattern)
	}
}

// runBehaviorCase dispatches one case and checks its expected outcome.
func runBehaviorCase(t *testing.T, toolName string, testCase *behaviorCase, paths behaviorPaths) {
	t.Helper()

	if count := behaviorOutcomeCount(testCase); count != 1 {
		t.Fatalf("outcome fields set = %d, want exactly 1 non-empty outcome", count)
	}

	checkHostDecidedShape(t, testCase)

	if testCase.APIResponse != nil && testCase.APIResponseRaw != nil {
		t.Fatal("api_response and api_response_raw are mutually exclusive")
	}

	if testCase.APIResponses != nil &&
		(testCase.APIResponse != nil || testCase.APIResponseRaw != nil || testCase.APIStatus != nil) {
		t.Fatal("api_responses cannot be combined with single-response fields")
	}

	var (
		captureMu sync.Mutex
		captured  []capturedRequest
		unmatched []string
	)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		// Match on the escaped path, not r.URL.Path: net/http decodes %2F back to
		// a slash, so a path arg carrying one would match a key the client never
		// sent, and the Python runner reads the raw URL. Keys stay comparable.
		// The fake base carries a version segment the way a real apiUrl does, so
		// the surface swap fires here exactly as it does in the Python runner.
		// Stripping it before matching keeps every fixture's api_responses key
		// and expected path written without one.
		surface, matchPath := splitSurface(r.URL.EscapedPath())
		_, requestURI := splitSurface(r.URL.RequestURI())
		record := capturedRequest{
			method: r.Method, path: requestURI, surface: surface, body: body,
		}
		response, status, matched := resolveBehaviorResponse(testCase, r.Method, matchPath)

		for header, value := range testCase.APIResponseHeaders[r.Method+" "+matchPath] {
			w.Header().Set(header, value)
		}

		func() {
			captureMu.Lock()
			defer captureMu.Unlock()

			captured = append(captured, record)

			if !matched {
				unmatched = append(unmatched, r.Method+" "+matchPath)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	defer apiSrv.Close()

	args := materializeBehaviorFiles(t, testCase)

	// Resolved after the server is up because the port is only known now, and
	// the routed responses are what carry a URL pointing back at it.
	if testCase.APIResponses != nil {
		testCase.APIResponses = substituteAPIBase(testCase.APIResponses,
			apiSrv.URL+"/"+linoderoute.DefaultSurfaceSegment)
	}

	srv, err := server.New(behaviorConfigFor(testCase, apiSrv.URL, paths))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clock := behaviorClock(t, testCase)

	for index := range testCase.SetupCalls {
		setup := testCase.SetupCalls[index]
		if isError, text := callServerTool(t, srv, clock, setup.Tool, setup.Args); isError {
			t.Fatalf("setup call %d (%s) failed: %s", index, setup.Tool, text)
		}
	}

	isError, text := callServerTool(t, srv, clock, toolName, args)

	if len(unmatched) > 0 {
		t.Errorf("requests with no api_responses entry: %s", strings.Join(unmatched, ", "))
	}

	if dryRun, _ := testCase.Args["dry_run"].(bool); dryRun {
		checkBehaviorNoMutation(t, captured)
	}

	switch {
	case testCase.ExpectError != "":
		checkBehaviorError(t, isError, text, captured, testCase.ExpectError)
	case testCase.ExpectAPIError != "":
		checkBehaviorAPIError(t, isError, text, captured, testCase.ExpectAPIError)
	case testCase.ExpectResult != nil:
		checkBehaviorResult(t, isError, text,
			substituteRunnerPaths(testCase.ExpectResult, paths), testCase.HostDecided)
	default:
		checkBehaviorRequest(t, isError, text, captured, testCase.ExpectRequest)
	}

	checkBehaviorRequests(t, captured, testCase.ExpectRequests)
}

// checkBehaviorRequests asserts the calls a case pins in order, one per
// captured request. Absent from a case, it asserts nothing: only the tools
// whose legs matter spell them out.
func checkBehaviorRequests(t *testing.T, captured []capturedRequest, want []behaviorRequest) {
	t.Helper()

	if len(want) == 0 {
		return
	}

	if len(captured) != len(want) {
		t.Fatalf("captured %d requests, want %d", len(captured), len(want))
	}

	for index := range want {
		checkCapturedRequest(t, captured[index], &want[index], index)
	}
}

// checkCapturedRequest holds one captured call to the leg the case declared.
func checkCapturedRequest(t *testing.T, got capturedRequest, want *behaviorRequest, index int) {
	t.Helper()

	if got.method != want.Method {
		t.Errorf("request %d method = %q, want %q", index, got.method, want.Method)
	}

	if got.path != want.Path {
		t.Errorf("request %d path = %q, want %q", index, got.path, want.Path)
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
		t.Errorf("request %d body = %s, want %s", index, got.body, want.Body)
	}
}

func TestResolveBehaviorResponse(t *testing.T) {
	t.Parallel()

	t.Run("raw empty body and status", func(t *testing.T) {
		t.Parallel()

		var empty string

		status := http.StatusBadRequest
		body, gotStatus, matched := resolveBehaviorResponse(
			&behaviorCase{APIResponseRaw: &empty, APIStatus: &status},
			http.MethodPost,
			"/domains",
		)

		if !matched {
			t.Fatal("matched = false, want true")
		}

		if gotStatus != status {
			t.Errorf("status = %d, want %d", gotStatus, status)
		}

		if len(body) != 0 {
			t.Errorf("body = %q, want empty", body)
		}
	})

	t.Run("raw malformed body", func(t *testing.T) {
		t.Parallel()

		raw := `{"id":`
		body, status, matched := resolveBehaviorResponse(
			&behaviorCase{APIResponseRaw: &raw},
			http.MethodPost,
			"/domains",
		)

		if !matched {
			t.Fatal("matched = false, want true")
		}

		if status != http.StatusOK {
			t.Errorf("status = %d, want %d", status, http.StatusOK)
		}

		if string(body) != raw {
			t.Errorf("body = %q, want %q", body, raw)
		}
	})
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
func checkBehaviorResult(
	t *testing.T, isError bool, text string, want json.RawMessage, hostDecided []behaviorHostDecided,
) {
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

	takeBehaviorHostDecided(t, gotValue, wantValue, hostDecided)

	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Errorf("result mismatch\ngot:\n%s\nwant:\n%s", text, want)
	}
}

// takeBehaviorHostDecided asserts each named field and lifts it out of the
// answer, leaving the rest to the literal comparison.
//
// A name the answer does not carry fails here rather than passing quietly,
// which is what stops a renamed field from turning a pin into a no-op, and a
// name expect_result also spells fails too: one owner per field.
func takeBehaviorHostDecided(t *testing.T, gotValue, wantValue any, hostDecided []behaviorHostDecided) {
	t.Helper()

	for index := range hostDecided {
		field := &hostDecided[index]

		if _, spelled := takeHostDecided(wantValue, field.Path); spelled {
			t.Fatalf("host-decided %q is also spelled in expect_result", field.Path)
		}

		value, present := takeHostDecided(gotValue, field.Path)
		if !present {
			t.Fatalf("host-decided %q is not in the answer", field.Path)
		}

		checkHostDecidedValue(t, field, value)
	}
}

// checkBehaviorError asserts a local validation failure with the exact bare
// message and no HTTP request. Go emits the message unwrapped; the Python
// runner adds an "Error: " prefix before comparing with the fixture text.
func checkBehaviorError(t *testing.T, isError bool, text string, captured []capturedRequest, want string) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if text != want {
		t.Errorf("error text = %q, want %q", text, want)
	}

	if len(captured) != 0 {
		t.Errorf("captured %d requests, want 0", len(captured))
	}
}

// checkBehaviorAPIError asserts an error containing the shared failure reason
// after the handler made at least one HTTP request.
func checkBehaviorAPIError(t *testing.T, isError bool, text string, captured []capturedRequest, want string) {
	t.Helper()

	if !isError {
		t.Fatalf("isError = false, want true (text %q)", text)
	}

	if !strings.Contains(text, want) {
		t.Errorf("error text = %q, want substring %q", text, want)
	}

	if len(captured) == 0 {
		t.Fatal("captured 0 requests, want at least 1")
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

	if got.surface != want.surface() {
		t.Errorf("api surface = %q, want %q", got.surface, want.surface())
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
