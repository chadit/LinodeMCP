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
// package directory (go test runs with that directory as cwd).
const clientDir = "../../internal/linode"

// dump is the command's JSON contract, mirrored here rather than shared so a
// change to the real struct has to be made deliberately in both places.
type dump struct {
	Routes     []string `json:"routes"`
	Unresolved []string `json:"unresolved"`
}

// runDump executes the command the way the Python gate does and returns its
// streams. Black-box on purpose: the gate depends on the real contract (JSON on
// stdout, non-zero exit on failure), so the test exercises that, not internals.
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

// writeFixture writes one Go source file into a fresh directory and returns it.
// The file only has to parse: the dumper reads source and never builds it.
func writeFixture(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "client.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	return dir
}

// fixtureClient exercises every endpoint shape the real client builds, so a
// resolver that stops understanding one of them fails here with the shape
// named rather than as a count that moved.
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
// is named rather than dropped. A dropped one would read as a client that does
// not build the route, which is the false negative this tool exists to remove.
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

// TestDumpSkipsTestFiles proves a fixture endpoint in a _test.go never counts
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
// routes, which the gate would read as every contracted route being missing.
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
// base constant and a format verb is present. That second one is the shape a
// text search cannot find, which is the whole reason this command exists.
func TestDumpResolvesTheRealClient(t *testing.T) {
	t.Parallel()

	got, stderr, err := runDump(t, clientDir)
	if err != nil {
		t.Fatalf("route-dump failed: %v\nstderr: %s", err, stderr)
	}

	if len(got.Unresolved) != 0 {
		t.Errorf("unresolved call sites in the real client: %v", got.Unresolved)
	}

	// Built as endpointInstanceDeep + "/%s/interfaces", so no search for the
	// whole path matches the source.
	const assembled = "GET /linode/instances/{p}/interfaces"
	if !slices.Contains(got.Routes, assembled) {
		t.Errorf("route surface is missing %q", assembled)
	}

	const profileSecurityQuestions = "GET /profile/security-questions"
	if !slices.Contains(got.Routes, profileSecurityQuestions) {
		t.Errorf("route surface is missing %q", profileSecurityQuestions)
	}
}
