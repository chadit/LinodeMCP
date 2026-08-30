package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// Repeated literals hoisted past the duplicate-string threshold. The tags texts
// are the exact messages both languages emit, so drift in either one fails here.
const (
	listPagePathDomains = "/domains"
	listPageQuery       = "page=2&page_size=50"
	errTagsNotJSONArray = "tags must be a JSON string array"
	errTagsBlankEntry   = "tags entries must be non-empty strings"
)

// toolConstructor is the shape every tool factory shares.
type toolConstructor func(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error))

// listPageServer serves an empty page envelope and records the query string it
// saw, so a test can assert the page pair survived the trip to the request.
func listPageServer(t *testing.T, wantPath string, gotQuery *string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, wantPath)
		}

		*gotQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/json")

		// Written as literal JSON rather than an encoded map: these envelope
		// keys belong to the API, not to this package's shared constants.
		if _, err := w.Write([]byte(`{"data":[],"page":1,"pages":1,"results":0}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
}

// TestListToolsSendPageQuery covers the list tools that gained page/page_size.
// The factory must thread the pair from the arguments through to the request,
// so a caller asking for page two does not silently keep getting page one.
func TestListToolsSendPageQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		newTool toolConstructor
		name    string
		path    string
	}{
		{name: "domains", path: listPagePathDomains, newTool: gentools.NewLinodeDomainListTool},
		{name: "firewalls", path: "/networking/firewalls", newTool: gentools.NewLinodeFirewallListTool},
		{name: "nodebalancers", path: "/nodebalancers", newTool: gentools.NewLinodeNodebalancerListTool},
		{name: "ssh keys", path: "/profile/sshkeys", newTool: gentools.NewLinodeSshkeyListTool},
		{name: "stackscripts", path: "/linode/stackscripts", newTool: gentools.NewLinodeStackscriptListTool},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var gotQuery string

			srv := listPageServer(t, testCase.path, &gotQuery)
			defer srv.Close()

			_, _, handler := testCase.newTool(newTestConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
				keyPage:     2,
				keyPageSize: 50,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsError {
				t.Errorf("result.IsError = true, want false")
			}

			if want := listPageQuery; gotQuery != want {
				t.Errorf("query = %q, want %q", gotQuery, want)
			}
		})
	}
}

// TestListToolsReadAnInt64Page pins the page reader's third number form: a
// caller handing the pair over as int64 rather than as the JSON float64 or the
// platform int reaches the same query.
func TestListToolsReadAnInt64Page(t *testing.T) {
	t.Parallel()

	var gotQuery string

	srv := listPageServer(t, listPagePathDomains, &gotQuery)
	defer srv.Close()

	_, _, handler := gentools.NewLinodeDomainListTool(newTestConfig(srv.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyPage:     int64(2),
		keyPageSize: int64(50),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Errorf("result.IsError = true, want false")
	}

	if gotQuery != listPageQuery {
		t.Errorf("query = %q, want %q", gotQuery, listPageQuery)
	}
}

// TestListToolsOmitUnsetPageQuery is the other half of the contract: with no
// page pair the request carries no query, so the API's own default page applies
// and the request matches the pre-pagination one byte for byte.
func TestListToolsOmitUnsetPageQuery(t *testing.T) {
	t.Parallel()

	var gotQuery string

	srv := listPageServer(t, listPagePathDomains, &gotQuery)
	defer srv.Close()

	_, _, handler := gentools.NewLinodeDomainListTool(newTestConfig(srv.URL))

	if _, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

// TestListToolsRejectOutOfRangePage checks the bounds reach the caller before
// the request does, so an unusable page never reaches the API.
func TestListToolsRejectOutOfRangePage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{
			name: "page below one",
			args: map[string]any{keyPage: 0},
			want: "page must be an integer greater than or equal to 1",
		},
		{
			name: "page_size below the minimum",
			args: map[string]any{keyPageSize: 24},
			want: "page_size must be an integer from 25 through 500",
		},
		{
			name: "page_size above the maximum",
			args: map[string]any{keyPageSize: 501},
			want: "page_size must be an integer from 25 through 500",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("handler reached the API with an out-of-range page")
			}))
			defer srv.Close()

			_, _, handler := gentools.NewLinodeDomainListTool(newTestConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertToolErrorContains(t, result, testCase.want)
		})
	}
}

// assertToolErrorContains fails unless result is an error result whose text
// contains want.
func assertToolErrorContains(t *testing.T, result *mcp.CallToolResult, want string) {
	t.Helper()

	if !result.IsError {
		t.Fatal("result.IsError = false, want true")
	}

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("content is not TextContent")
	}

	if !strings.Contains(text.Text, want) {
		t.Errorf("error text %q does not contain %q", text.Text, want)
	}
}

// TestWriteToolsRejectMalformedTags covers the tags validation every mutating
// tool that gained the field shares. A tags value the API cannot accept has to
// come back as a validation error rather than being dropped from the body,
// which would silently create or update the resource untagged.
func TestWriteToolsRejectMalformedTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		newTool toolConstructor
		args    map[string]any
		name    string
		want    string
	}{
		{
			name:    "instance create rejects a non-array tags",
			newTool: gentools.NewLinodeInstanceCreateTool,
			args:    map[string]any{keyConfirm: true, keyRegion: regionUSEast, keyType: typeG6Nanode1, keyFirewallID: 123, keyTags: 5},
			want:    errTagsNotJSONArray,
		},
		{
			name:    "instance create rejects a blank tag",
			newTool: gentools.NewLinodeInstanceCreateTool,
			args:    map[string]any{keyConfirm: true, keyRegion: regionUSEast, keyType: typeG6Nanode1, keyFirewallID: 123, keyTags: []any{envProd, " "}},
			want:    errTagsBlankEntry,
		},
		{
			name:    "firewall create rejects a non-array tags",
			newTool: gentools.NewLinodeFirewallCreateTool,
			args:    map[string]any{keyConfirm: true, keyLabel: "web-fw", keyTags: 5},
			want:    errTagsNotJSONArray,
		},
		{
			name:    "nodebalancer create rejects a non-array tags",
			newTool: gentools.NewLinodeNodebalancerCreateTool,
			args:    map[string]any{keyConfirm: true, keyRegion: regionUSEast, keyTags: 5},
			want:    errTagsNotJSONArray,
		},
		{
			name:    "nodebalancer update rejects a non-array tags",
			newTool: gentools.NewLinodeNodebalancerUpdateTool,
			args:    map[string]any{keyConfirm: true, keyNodeBalancerID: 789, keyTags: 5},
			want:    errTagsNotJSONArray,
		},
		{
			name:    "volume create rejects a non-array tags",
			newTool: gentools.NewLinodeVolumeCreateTool,
			args:    map[string]any{keyConfirm: true, keyLabel: "my-vol", keyRegion: regionUSEast, keyTags: 5},
			want:    errTagsNotJSONArray,
		},
		{
			name:    "domain update rejects a non-array tags",
			newTool: gentools.NewLinodeDomainUpdateTool,
			args:    map[string]any{keyConfirm: true, keyDomainID: 5, keyTags: 5},
			want:    errTagsNotJSONArray,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("handler reached the API with a malformed tags value")
			}))
			defer srv.Close()

			_, _, handler := testCase.newTool(newTestConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertToolErrorContains(t, result, testCase.want)
		})
	}
}

// TestLKEClusterIDReadToolsRejectMissingID covers the cluster_id guard on the
// two LKE read tools whose handlers no other test reaches with a missing id.
func TestLKEClusterIDReadToolsRejectMissingID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		newTool toolConstructor
		name    string
		want    string
	}{
		{name: "dashboard get", newTool: gentools.NewLinodeLkeDashboardGetTool, want: "cluster_id is required"},
		// The ACL route holds its id rule on the contract, where proto3 reads an
		// absent int32 as zero, so one sentence answers absent and unusable alike.
		{name: "acl get", newTool: gentools.NewLinodeLkeACLGetTool, want: "cluster_id must be a positive integer"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("handler reached the API without a cluster_id")
			}))
			defer srv.Close()

			_, _, handler := testCase.newTool(newTestConfig(srv.URL))

			result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assertToolErrorContains(t, result, testCase.want)
		})
	}
}
