package main_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// clientDir is the Linode client package, two levels up from this command's
// package directory (go test runs with that directory as cwd). profilesDir is
// the package whose scope validator reads two routes through the typed lookup,
// which is the only hand-written route evidence left once the client names no
// tool of its own.
const (
	clientDir   = "../../internal/linode"
	profilesDir = "../../internal/profiles"
)

// fakeDeleteTool is the tool the contracted fixtures name at their one plain
// call site.
const fakeDeleteTool = "fake_thing_delete"

// dump is the command's JSON contract, mirrored here rather than shared so a
// change to the real struct has to be made deliberately in both places.
type dump struct {
	Routes     []string         `json:"routes"`
	Contracted []contractedSite `json:"contracted"`
	Unresolved []string         `json:"unresolved"`
}

// contractedSite mirrors one entry of the contracted list the gate looks up in
// the proto contract: a tool named at the site, or the message a typed lookup
// there names in its place.
type contractedSite struct {
	Tool    string `json:"tool"`
	Message string `json:"message"`
	Site    string `json:"site"`
}

// runDump executes the command the way the Python gate does. Black-box on
// purpose: the gate depends on JSON on stdout and a non-zero exit on failure,
// so the test exercises that contract rather than internals.
func runDump(t *testing.T, dir string) (dump, []byte, error) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "go", "run", ".", "-client-dir", dir)

	var out, errBuf bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return dump{}, errBuf.Bytes(), err
	}

	var decoded dump
	if unmarshalErr := json.Unmarshal(out.Bytes(), &decoded); unmarshalErr != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", unmarshalErr, out.Bytes())
	}

	return decoded, errBuf.Bytes(), nil
}

// writeFixture writes one Go source file into a fresh directory. The file only
// has to parse: the dumper reads source and never builds it.
func writeFixture(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "client.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return dir
}

// writeFixtureDir writes a fixture into a named subdirectory of a shared root.
// The multi-directory scan needs two separate arguments, not two neighboring
// files.
func writeFixtureDir(t *testing.T, root, name, source string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "client.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return dir
}

// fixtureGeneratedCaller is the shape a born-generated tool has: it names its
// tool to a primitive that lives in another package and builds no path itself.
const fixtureGeneratedCaller = `package gentools

func handleThingGet(ctx context.Context, id string) error {
	return client.CallProtoRouteQuery(ctx, "fake_thing_get", []any{id})
}
`

// TestDumpFollowsACallerInASecondDirectory pins the scan's scope. A tool born
// generated has no call site in the client package: the client declares the
// primitive and the generated package names the tool to it, so scanning the
// client alone would report that route as one no client can build.
func TestDumpFollowsACallerInASecondDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	client := writeFixtureDir(t, root, "client", fixtureChainedBuilder)
	generated := writeFixtureDir(t, root, "generated", fixtureGeneratedCaller)

	got, stderr, err := runDump(t, client+","+generated)
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Unresolved) != 0 {
		t.Errorf("unresolved call sites across the two directories: %v", got.Unresolved)
	}

	var found bool

	for _, site := range got.Contracted {
		if site.Tool == "fake_thing_get" {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("contracted = %v, want the tool named in the second directory", got.Contracted)
	}
}

// TestDumpNamesADirectoryThatIsNotThere pins the other half: the generated tree
// is gitignored, so a checkout where it has not been written must fail rather
// than report every born-generated route as one nothing builds.
func TestDumpNamesADirectoryThatIsNotThere(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	client := writeFixtureDir(t, root, "client", fixtureChainedBuilder)
	absent := filepath.Join(root, "generated")

	_, stderr, err := runDump(t, client+","+absent)
	if err == nil {
		t.Fatal("route-dump accepted a directory that does not exist")
	}

	if !strings.Contains(string(stderr), absent) {
		t.Errorf("stderr = %s, want the missing directory named", stderr)
	}
}

// fixtureClient exercises every endpoint shape the real client builds, so a
// resolver that stops understanding one fails here by name rather than as a
// count that moved.
const fixtureClient = `package fake

const (
	endpointThings = "/things"
	endpointNested = endpointThings + "/nested"
)

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
}

func (c *Client) makeRequestWithContentType(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, body)
}

func listThings[T any](ctx context.Context, client *Client, operation, endpoint string) ([]T, error) {
	return nil, client.makeRequest(ctx, http.MethodGet, withQuery(endpoint), nil)
}

func withQuery(endpoint string, page int) string {
	if page > 0 {
		return endpoint + "?page=1"
	}

	return endpoint
}

func thingEndpoint(id int) string {
	return fmt.Sprintf(endpointThings+"/%s/sub", url.PathEscape(strconv.Itoa(id)))
}

func (c *Client) httpConstantOnly(ctx context.Context) error {
	return c.makeRequest(ctx, http.MethodPost, endpointThings, nil)
}

func (c *Client) httpFormatWithConcatenatedBase(ctx context.Context, id int) error {
	endpoint := fmt.Sprintf(endpointThings+"/%s", url.PathEscape(strconv.Itoa(id)))

	return c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
}

func (c *Client) httpFormatWithBaseAsArgument(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("%s/%s/versions", endpointNested, url.PathEscape(id))

	return c.makeRequest(ctx, http.MethodGet, endpoint, nil)
}

func (c *Client) httpConcatenatedSegments(ctx context.Context, name string) error {
	endpoint := endpointThings + "/named/" + url.PathEscape(name)

	return c.makeRequest(ctx, http.MethodPut, endpoint, nil)
}

func (c *Client) httpAppendedQuery(ctx context.Context, skip bool) error {
	endpoint := endpointNested
	if skip {
		endpoint += "?skip=true"
	}

	return c.makeRequest(ctx, http.MethodGet, endpoint, nil)
}

func (c *Client) httpThroughHelper(ctx context.Context, id int) error {
	return c.makeRequest(ctx, http.MethodGet, thingEndpoint(id), nil)
}

func (c *Client) httpThroughWrapper(ctx context.Context) error {
	_, err := listThings[string](ctx, c, "ListThings", endpointThings+"/listed")

	return err
}

func (c *Client) httpSecondPrimitive(ctx context.Context, id string) error {
	endpoint := endpointThings + "/" + url.PathEscape(id) + "/thumbnail"

	return c.makeRequestWithContentType(ctx, http.MethodPut, endpoint, nil, "image/png")
}
`

func TestDumpResolvesEveryEndpointShape(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureClient))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	want := []string{
		"DELETE /things/{p}",
		"GET /things/listed",
		"GET /things/nested",
		"GET /things/nested/{p}/versions",
		"GET /things/{p}/sub",
		"POST /things",
		"PUT /things/named/{p}",
		"PUT /things/{p}/thumbnail",
	}

	if !slices.Equal(got.Routes, want) {
		t.Errorf("routes = %v, want %v", got.Routes, want)
	}

	if len(got.Unresolved) != 0 {
		t.Errorf("unresolved = %v, want none", got.Unresolved)
	}
}

// TestDumpReportsWhatItCannotFollow proves an endpoint the resolver cannot read
// is named rather than dropped. A dropped one reads as a client that does not
// build the route, the false negative this tool exists to remove.
func TestDumpReportsWhatItCannotFollow(t *testing.T) {
	t.Parallel()

	const source = `package fake

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
}

func (c *Client) httpKnown(ctx context.Context) error {
	return c.makeRequest(ctx, http.MethodGet, "/known", nil)
}

func (c *Client) httpMystery(ctx context.Context) error {
	return c.makeRequest(ctx, http.MethodGet, elsewhere.Endpoint(), nil)
}
`

	got, stderr, err := runDump(t, writeFixture(t, source))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if !slices.Equal(got.Routes, []string{"GET /known"}) {
		t.Errorf("routes = %v, want the one resolvable route", got.Routes)
	}

	if len(got.Unresolved) != 1 {
		t.Fatalf("unresolved = %v, want the one call site that could not be read", got.Unresolved)
	}

	for _, want := range []string{"httpMystery", "unresolved path"} {
		if !strings.Contains(got.Unresolved[0], want) {
			t.Errorf("unresolved entry %q should mention %q", got.Unresolved[0], want)
		}
	}
}

// fixtureContracted is the client shape that resolves its route from the proto
// contract: a primitive taking a tool name where the others take a method and a
// path, one call site that names its tool, and one that takes the name from its
// caller.
const fixtureContracted = `package fake

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
}

func (c *Client) makeRouteRequest(ctx context.Context, tool string, payload any, values ...any) (*http.Response, error) {
	method, endpoint, err := linoderoute.Resolve(tool, values...)
	if err != nil {
		return nil, err
	}

	return c.makeRequest(ctx, method, endpoint, payload)
}

func (c *Client) httpDeleteThing(ctx context.Context, id int) error {
	_, err := c.makeRouteRequest(ctx, "fake_thing_delete", nil, id)

	return err
}

func (c *Client) httpDeleteWhatever(ctx context.Context, tool string, id int) error {
	_, err := c.makeRouteRequest(ctx, tool, nil, id)

	return err
}
`

// TestDumpNamesTheToolAContractedCallSiteUses covers the evidence a migrated
// call site leaves. It writes no path, so the tool name is the whole route and
// the gate resolves it through the same contract it checks against.
func TestDumpNamesTheToolAContractedCallSiteUses(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureContracted))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Contracted) != 1 {
		t.Fatalf("contracted = %v, want the one call site that names its tool", got.Contracted)
	}

	if got.Contracted[0].Tool != fakeDeleteTool {
		t.Errorf("contracted tool = %q, want %q", got.Contracted[0].Tool, fakeDeleteTool)
	}

	if !strings.Contains(got.Contracted[0].Site, "httpDeleteThing") {
		t.Errorf("contracted site %q should name the calling function", got.Contracted[0].Site)
	}
}

// TestDumpReportsAContractedCallWithNoToolName pins the hole this leaves open.
// A tool name the resolver cannot read leaves no route an offline reader can
// check, so it is reported the way an unreadable endpoint is rather than going
// silent and reading as a route the client never builds.
func TestDumpReportsAContractedCallWithNoToolName(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureContracted))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Unresolved) != 1 {
		t.Fatalf("unresolved = %v, want the one call site that does not name its tool", got.Unresolved)
	}

	for _, want := range []string{"httpDeleteWhatever", "unnamed tool"} {
		if !strings.Contains(got.Unresolved[0], want) {
			t.Errorf("unresolved entry %q should mention %q", got.Unresolved[0], want)
		}
	}
}

// TestDumpDoesNotReportTheRouteBuilderItself is the other half: the primitive
// looks up its own method and path at run time, so its body resolves to
// nothing. Reported as unresolved it would fail the gate on every run, and the
// only fix would be a baseline entry for a call site working as designed.
func TestDumpDoesNotReportTheRouteBuilderItself(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureContracted))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	for _, entry := range got.Unresolved {
		if strings.Contains(entry, "makeRouteRequest") {
			t.Errorf("unresolved entry %q reports the route builder's own body", entry)
		}
	}
}

// fixtureRoutedShapes holds the three routed primitives: one carrying a query
// string, one sending a pre-framed body under its own content type, and a
// routed list fetcher. None writes a URL, so every call site below is contract
// evidence and nothing here resolves to a route.
const fixtureRoutedShapes = `package fake

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
}

func (c *Client) makeRequestWithContentType(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, body)
}

func (c *Client) makeRouteRequestQuery(ctx context.Context, tool, rawQuery string, payload any, values ...any) (*http.Response, error) {
	method, endpoint, err := routedRequest(tool, rawQuery, values)
	if err != nil {
		return nil, err
	}

	return c.makeRequest(ctx, method, endpoint, payload)
}

func (c *Client) makeRouteRequestContentType(ctx context.Context, tool, contentType string, body io.Reader, values ...any) (*http.Response, error) {
	method, endpoint, err := routedRequest(tool, "", values)
	if err != nil {
		return nil, err
	}

	return c.makeRequestWithContentType(ctx, method, endpoint, body, contentType)
}

func listThings[T any](ctx context.Context, client *Client, operation, endpoint string) ([]T, error) {
	return nil, client.makeRequest(ctx, http.MethodGet, endpoint, nil)
}

func listThingsRouted[T any](ctx context.Context, client *Client, operation, tool, rawQuery string, values []any) ([]T, error) {
	return routedList(operation, tool, rawQuery, values, func(endpoint string) ([]T, error) {
		return listThings[T](ctx, client, operation, endpoint)
	})
}

func (c *Client) httpListThings(ctx context.Context) error {
	_, err := listThingsRouted[string](ctx, c, "ListThings", "fake_thing_list", "page=1", nil)

	return err
}

func (c *Client) httpFilterThings(ctx context.Context, id int) error {
	_, err := c.makeRouteRequestQuery(ctx, "fake_thing_filter", "skip=true", nil, id)

	return err
}

func (c *Client) httpUploadThing(ctx context.Context, id int, body io.Reader) error {
	_, err := c.makeRouteRequestContentType(ctx, "fake_thing_upload", "image/png", body, id)

	return err
}
`

// TestDumpNamesTheToolEveryRoutedShapeUses covers the routed shapes besides the
// plain primitive. A shape the resolver stops recognizing takes its call sites
// with it: they build no path, so losing them reads as a client that never had
// the route rather than as a resolver gap. The content-type primitive is the
// one recognized purely by inference, because it takes a tool and hands off to
// a function declaring an endpoint. Renaming either parameter silently drops it
// and every upload route with it.
func TestDumpNamesTheToolEveryRoutedShapeUses(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureRoutedShapes))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Unresolved) != 0 {
		t.Errorf("unresolved = %v, want none", got.Unresolved)
	}

	want := []contractedSite{
		{Tool: "fake_thing_filter", Site: "httpFilterThings"},
		{Tool: "fake_thing_list", Site: "httpListThings"},
		{Tool: "fake_thing_upload", Site: "httpUploadThing"},
	}

	if len(got.Contracted) != len(want) {
		t.Fatalf("contracted = %v, want one entry per routed call site", got.Contracted)
	}

	for _, entry := range want {
		if !hasContracted(got.Contracted, entry) {
			t.Errorf("contracted = %v, missing %q at %q", got.Contracted, entry.Tool, entry.Site)
		}
	}
}

// TestDumpAcceptsAClientWithNoBuiltPaths pins the migration's end state. Once
// every call site names a tool the client builds no path, and the hard fail on
// an empty dump has to read that as done rather than as a resolver that stopped
// following the call graph.
func TestDumpAcceptsAClientWithNoBuiltPaths(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureRoutedShapes))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Routes) != 0 {
		t.Errorf("routes = %v, want none: every call site names a tool", got.Routes)
	}
}

// hasContracted reports whether the dump names this tool, or this message, at a
// call site in the named function. The site carries a file and a line that
// shift with the fixture, so only the function name is matched.
func hasContracted(contracted []contractedSite, want contractedSite) bool {
	for _, entry := range contracted {
		if entry.Tool == want.Tool && entry.Message == want.Message && strings.Contains(entry.Site, want.Site) {
			return true
		}
	}

	return false
}

// TestDumpSkipsTestFiles proves an endpoint written in a _test.go never counts
// as evidence that the client builds that route.
func TestDumpSkipsTestFiles(t *testing.T) {
	t.Parallel()

	dir := writeFixture(t, `package fake

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
}

func (c *Client) httpReal(ctx context.Context) error {
	return c.makeRequest(ctx, http.MethodGet, "/real", nil)
}
`)

	const testSource = `package fake

func (c *Client) httpFromATest(ctx context.Context) error {
	return c.makeRequest(ctx, http.MethodGet, "/only-in-a-test", nil)
}
`

	path := filepath.Join(dir, "client_test.go")
	if err := os.WriteFile(path, []byte(testSource), 0o600); err != nil {
		t.Fatalf("write test fixture: %v", err)
	}

	got, stderr, err := runDump(t, dir)
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if !slices.Equal(got.Routes, []string{"GET /real"}) {
		t.Errorf("routes = %v, want only the non-test route", got.Routes)
	}
}

// TestDumpWithoutARequestPrimitiveIsHardFail proves the tripwire: a tree where
// nothing builds a request exits non-zero instead of reporting a client with no
// routes, which the gate would read as every contracted route missing.
func TestDumpWithoutARequestPrimitiveIsHardFail(t *testing.T) {
	t.Parallel()

	_, stderr, err := runDump(t, t.TempDir())
	if err == nil {
		t.Fatal("expected non-zero exit on a tree with no request primitive, got success")
	}

	if !bytes.Contains(stderr, []byte("endpoint")) {
		t.Errorf("stderr should name what it looked for; got: %s", stderr)
	}
}

// TestDumpResolvesTheRealClient pins the two properties the gate depends on:
// every request call site resolves, and a route the client assembles from a
// base constant and a format verb is present. That second shape is the one a
// text search cannot find, which is why this command exists.
func TestDumpResolvesTheRealClient(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, clientDir+","+profilesDir)
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Unresolved) != 0 {
		t.Errorf("unresolved call sites in the real client: %v", got.Unresolved)
	}

	// The dead-client sweep took the last hand-assembled route (PUT /profile)
	// with its dead method, so the client builds no path by hand anymore:
	// TestDumpAcceptsAClientWithNoBuiltPaths pins that end state.
	if len(got.Routes) != 0 {
		t.Errorf("hand-built routes survived the migration: %v", got.Routes)
	}

	// The client spells no tool of its own any more: the one hand-written read
	// left, the scope validator's, names its generated message to the typed
	// lookup, and that evidence has to survive as a contracted entry or the
	// migration silently costs the route its coverage.
	for _, site := range got.Contracted {
		if site.Tool != "" {
			t.Errorf("a hand-written call site still spells its tool: %+v", site)
		}
	}

	for _, message := range []string{"ProfileGetInput", "ProfileGrantsGetInput"} {
		if !hasMessage(got.Contracted, message) {
			t.Errorf("contracted call sites are missing the typed lookup of %q: %v", message, got.Contracted)
		}
	}
}

// hasMessage reports whether the dump names this message at some typed lookup.
func hasMessage(contracted []contractedSite, message string) bool {
	for _, entry := range contracted {
		if entry.Message == message {
			return true
		}
	}

	return false
}

// fixtureTypedLookup is the shape the scope validator has: a builder in one
// package that takes a tool, and a caller in another that names no tool at all,
// handing a generated message type to the typed lookup instead. The second
// caller hands the lookup a value rather than a type, which no offline reader
// can name.
const fixtureTypedLookup = `package validator

func readThing(ctx context.Context, client Reader) error {
	tool, err := linoderoute.ToolOf(&fakev1.ThingGetInput{})
	if err != nil {
		return err
	}

	var thing map[string]any

	return client.CallRouteJSON(ctx, tool, nil, &thing)
}

func readWhatever(ctx context.Context, client Reader, input proto.Message) error {
	tool, err := linoderoute.ToolOf(input)
	if err != nil {
		return err
	}

	return client.CallRouteJSON(ctx, tool, nil, nil)
}
`

// fixtureJSONBuilder is the client half of that shape: the exported primitive
// takes a tool and hands it to the route primitive, so callers elsewhere carry
// the evidence.
const fixtureJSONBuilder = fixtureChainedBuilder + `
func (c *Client) CallRouteJSON(ctx context.Context, tool string, values []any, out any) error {
	_, err := c.makeRouteRequest(ctx, tool, nil, values...)

	return err
}
`

// TestDumpNamesTheMessageATypedLookupHandsOver pins the typed call site: the
// message named to the lookup travels out in the tool's place, so the gate can
// resolve it through the contract, and a lookup handed a value it cannot read
// is reported as an unnamed tool rather than dropped.
func TestDumpNamesTheMessageATypedLookupHandsOver(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	client := writeFixtureDir(t, root, "client", fixtureJSONBuilder)
	validator := writeFixtureDir(t, root, "validator", fixtureTypedLookup)

	got, stderr, err := runDump(t, client+","+validator)
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	want := []contractedSite{
		{Tool: fakeDeleteTool, Site: "httpDeleteThing"},
		{Message: "ThingGetInput", Site: "readThing"},
	}

	if len(got.Contracted) != len(want) {
		t.Fatalf("contracted = %v, want one entry per call site", got.Contracted)
	}

	for _, entry := range want {
		if !hasContracted(got.Contracted, entry) {
			t.Errorf("contracted = %v, missing %+v", got.Contracted, entry)
		}
	}

	if len(got.Unresolved) != 1 {
		t.Fatalf("unresolved = %v, want the one lookup handed a value", got.Unresolved)
	}

	for _, part := range []string{"readWhatever", "unnamed tool"} {
		if !strings.Contains(got.Unresolved[0], part) {
			t.Errorf("unresolved entry %q should mention %q", got.Unresolved[0], part)
		}
	}
}

// fixtureChainedBuilder is the shape the generated tool package calls into: an
// exported primitive that takes a tool and hands it to another primitive rather
// than to a function carrying a path. Its callers live in another package, so
// no tool name it carries is written in this one.
const fixtureChainedBuilder = `package fake

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	return http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, nil)
}

func (c *Client) makeRouteRequest(ctx context.Context, tool string, payload any, values ...any) (*http.Response, error) {
	method, endpoint, err := linoderoute.Resolve(tool, values...)
	if err != nil {
		return nil, err
	}

	return c.makeRequest(ctx, method, endpoint, payload)
}

func (c *Client) CallProtoRouteQuery(ctx context.Context, tool string, values []any) error {
	_, err := c.makeRouteRequest(ctx, tool, nil, values...)

	return err
}

func (c *Client) httpDeleteThing(ctx context.Context, id int) error {
	_, err := c.makeRouteRequest(ctx, "fake_thing_delete", nil, id)

	return err
}
`

// TestDumpTreatsAnExportedToolForwarderAsAPrimitive pins the shape the
// generated tool package needs. An exported forwarder is called from outside
// this package, where the tool name is written and this scan cannot see it, so
// reporting it would report the design rather than a gap.
func TestDumpTreatsAnExportedToolForwarderAsAPrimitive(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureChainedBuilder))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	for _, entry := range got.Unresolved {
		if strings.Contains(entry, "CallProtoRouteQuery") {
			t.Errorf("unresolved names the exported primitive: %q", entry)
		}
	}

	if len(got.Contracted) != 1 || got.Contracted[0].Tool != fakeDeleteTool {
		t.Errorf("contracted = %v, want only the call site that names its tool", got.Contracted)
	}
}

// TestDumpStillReportsAnUnexportedToolForwarder is the other half of the same
// rule. Everything reaching an unexported forwarder is in this package, so a
// tool name it carries is one this scan can read: a forwarder with no such call
// site is the hole the unresolved report exists to show, and the exported case
// above must not have widened its way out of that report.
func TestDumpStillReportsAnUnexportedToolForwarder(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, writeFixture(t, fixtureContracted))
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Unresolved) != 1 || !strings.Contains(got.Unresolved[0], "httpDeleteWhatever") {
		t.Fatalf("unresolved = %v, want the unexported forwarder", got.Unresolved)
	}
}
