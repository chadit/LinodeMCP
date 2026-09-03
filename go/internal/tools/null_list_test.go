package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The two tools the list arm and the collection fetch are proven through: the
// only paged read whose elements restore documented nulls, and the only removal
// whose API publishes no GET on the route it deletes.
const (
	nullListPath         = "/networking/reserved/ips"
	certificateListPath  = "/iam/idp-configs/cfg-1/certificates"
	certificateListTool  = "linode_iam_idp_config_certificate_list"
	certificateConfigID  = "cfg-1"
	certificateConfigArg = "idp_config_id"
	certificateWantedID  = "cert-2"
	certificateUnknownID = "cert-9"
	// absentEnvironment names no environment the config declares.
	absentEnvironment      = "absent"
	nullListUnassignedAddr = "192.0.2.11"
)

// newCertificate is the element factory the collection fetch fills.
func newCertificate() *linodev1.IamIdpConfigCertificate {
	return &linodev1.IamIdpConfigCertificate{}
}

// certificatePage is a two-element page of the collection the removal selects
// out of.
const certificatePage = `{"data":[{"id":"cert-1","certificate":"first"},` +
	`{"id":"cert-2","certificate":"second"}],"page":1,"pages":1,"results":2}`

// TestGeneratedNullListToolRestoresEachElementsNulls: the page's nulls belong
// to the addresses in it, so a caller can tell "no gateway" from "the API said
// nothing" for every element rather than for the page.
func TestGeneratedNullListToolRestoresEachElementsNulls(t *testing.T) {
	t.Parallel()

	body := `{"data":[{"address":"192.0.2.10","gateway":"192.0.2.1","region":"us-east"},` +
		`{"address":"192.0.2.11","assigned_entity":null,"gateway":null,"interface_id":null,` +
		`"linode_id":null,"rdns":null,"region":"us-east","vpc_nat_1_1":null}],` +
		`"page":1,"pages":1,"results":2}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != nullListPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nullListPath)
		}

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, _, handler := gentools.NewLinodeNetworkingReservedIPListTool(newTestConfig(server.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	page := decodeReservedIPPage(t, result)

	if len(page.ReservedIPs) != 2 {
		t.Fatalf("len(page.ReservedIPs) = %d, want %d", len(page.ReservedIPs), 2)
	}

	for _, name := range []string{
		"assigned_entity", reservedIPGatewayKey, "interface_id", "linode_id", "rdns", "vpc_nat_1_1",
	} {
		value, sent := page.ReservedIPs[1][name]
		if !sent {
			t.Errorf("unassigned element dropped %s", name)

			continue
		}

		if string(value) != "null" {
			t.Errorf("unassigned element %s = %s, want null", name, value)
		}
	}

	// The assigned element carries no restored key: the API sent values for
	// all six, so nothing was dropped for the restoration to write back.
	if _, sent := page.ReservedIPs[0]["assigned_entity"]; sent {
		t.Error("assigned element gained an assigned_entity the API never sent")
	}

	if address := page.ReservedIPs[1]["address"]; string(address) != `"`+nullListUnassignedAddr+`"` {
		t.Errorf("second element address = %s, want %q", address, nullListUnassignedAddr)
	}
}

// reservedIPPage is the answer shape the list tool serializes, read back with
// each element's members left undecoded so a restored null is distinguishable
// from an absent key.
type reservedIPPage struct {
	ReservedIPs []map[string]json.RawMessage `json:"reserved_ips"`
	Count       int32                        `json:"count"`
}

// decodeReservedIPPage reads a successful tool result back as the page it is.
func decodeReservedIPPage(t *testing.T, result *mcp.CallToolResult) reservedIPPage {
	t.Helper()

	if result.IsError {
		t.Fatalf("result.IsError = true, text = %s", resultText(t, result))
	}

	var page reservedIPPage
	if err := json.Unmarshal([]byte(resultText(t, result)), &page); err != nil {
		t.Fatalf("decode page: %v", err)
	}

	return page
}

// TestGeneratedNullListToolReportsItsDeclaredFailure: the collection declares a
// sentence of its own, so a failed fetch names the resource rather than
// answering the shared one.
func TestGeneratedNullListToolReportsItsDeclaredFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, _, handler := gentools.NewLinodeNetworkingReservedIPListTool(newTestConfig(server.URL))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result.IsError = false, want true")
	}

	if want := "Failed to list reserved IPv4 addresses"; !strings.Contains(resultText(t, result), want) {
		t.Errorf("text = %q, want it to contain %q", resultText(t, result), want)
	}
}

// TestGeneratedNullListToolRefusesBeforeItCalls covers the checks the
// null-restoring driver makes ahead of the fetch. The declared check and the
// path readers are handed in here rather than reached through a tool, because
// the one collection that restores nulls today is top-level and declares
// neither; the driver still takes both so a nested one needs no second driver.
func TestGeneratedNullListToolRefusesBeforeItCalls(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args     map[string]any
		validate tools.ListValidate
		name     string
		want     string
		readPath []tools.ListPathValue
	}{
		{
			name: "a rule the contract declares",
			args: map[string]any{},
			want: "idp_config_id is required",
		},
		{
			name:     "a path value the caller left out",
			readPath: []tools.ListPathValue{func(*mcp.CallToolRequest) (any, string) { return nil, errRegionRequired }},
			args:     map[string]any{certificateConfigArg: certificateConfigID},
			want:     errRegionRequired,
		},
		{
			name:     "a check the tool declares",
			validate: func(*mcp.CallToolRequest) string { return errRegionRequired },
			args:     map[string]any{certificateConfigArg: certificateConfigID},
			want:     errRegionRequired,
		},
		{
			name: "a page bound outside its range",
			args: map[string]any{certificateConfigArg: certificateConfigID, keyPageSize: 1},
			want: errStandardPageSizeRange,
		},
		{
			name: "an environment the config does not name",
			args: map[string]any{certificateConfigArg: certificateConfigID, "environment": absentEnvironment},
			want: absentEnvironment,
		},
		{
			name: "an argument the message does not declare",
			args: map[string]any{certificateConfigArg: certificateConfigID, "bogus_field": "x"},
			want: "Unsupported argument(s) for linode_iam_idp_config_certificate_list: bogus_field",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// The certificate collection is the contract here because it
			// declares a rule of its own, publishes the page pair, and is
			// nested: one tool exercises every check the driver makes ahead
			// of the fetch.
			_, handler := tools.NewGeneratedNullListTool(
				newTestConfig("http://127.0.0.1:1"),
				certificateListTool,
				"Lists the SAML signing certificates on one configuration.",
				"linode.mcp.v1.IamIdpConfigCertificateListInput",
				nil,
				testCase.validate,
				testCase.readPath,
				func(_ context.Context, _ *linode.Client, _ *mcp.CallToolRequest, _ []any, _, _ int,
				) ([]*linodev1.IamIdpConfigCertificate, []json.RawMessage, error) {
					t.Error("the fetch ran past a refusal")

					return nil, nil, nil
				},
				nil,
				func(items []*linodev1.IamIdpConfigCertificate, count int32, _ *string,
				) *linodev1.IamIdpConfigCertificateListResponse {
					return &linodev1.IamIdpConfigCertificateListResponse{Count: count, Certificates: items}
				},
			)

			result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.IsError {
				t.Fatal("result.IsError = false, want true")
			}

			if !strings.Contains(resultText(t, result), testCase.want) {
				t.Errorf("text = %q, want it to contain %q", resultText(t, result), testCase.want)
			}
		})
	}
}

// TestMarshalProtoListResponseRestoringNullsRefusesAMismatchedPage: the
// restoration pairs by position, so two lists of different lengths would write
// one resource's nulls onto another.
func TestMarshalProtoListResponseRestoringNullsRefusesAMismatchedPage(t *testing.T) {
	t.Parallel()

	page := &linodev1.ReservedIPListResponse{
		Count:       1,
		ReservedIps: []*linodev1.ReservedIPAddress{{Address: reservedIPAddressFixture}},
	}

	_, err := tools.MarshalProtoListResponseRestoringNulls(page, nil, []string{reservedIPGatewayKey})
	if !errors.Is(err, tools.ErrPageElementCount) {
		t.Fatalf("err = %v, want %v", err, tools.ErrPageElementCount)
	}
}

// TestMarshalProtoListResponseRestoringNullsRefusesAMessageWithNoPage: a
// restoration writing into elements needs a message that carries some.
func TestMarshalProtoListResponseRestoringNullsRefusesAMessageWithNoPage(t *testing.T) {
	t.Parallel()

	_, err := tools.MarshalProtoListResponseRestoringNulls(
		&linodev1.ReservedIPAddress{Address: reservedIPAddressFixture}, nil, []string{reservedIPGatewayKey},
	)
	if !errors.Is(err, tools.ErrNullMember) {
		t.Fatalf("err = %v, want %v", err, tools.ErrNullMember)
	}
}

// TestMarshalProtoListResponseRestoringNullsReportsAnUnserializableMessage: a
// string member that is not valid UTF-8 cannot serialize as protojson, and the
// page marshaller reports that instead of answering partial bytes.
func TestMarshalProtoListResponseRestoringNullsReportsAnUnserializableMessage(t *testing.T) {
	t.Parallel()

	page := &linodev1.ReservedIPListResponse{
		Count:       1,
		ReservedIps: []*linodev1.ReservedIPAddress{{Address: "\xff\xfe"}},
	}

	_, err := tools.MarshalProtoListResponseRestoringNulls(
		page, []json.RawMessage{json.RawMessage(`{}`)}, []string{reservedIPGatewayKey},
	)
	if err == nil {
		t.Fatal("marshaling invalid UTF-8 succeeded, want an error")
	}
}

// TestMarshalProtoListResponseRestoringNullsReportsAMalformedElementBody: an
// element body that opens as an object but does not parse reports the decode
// failure rather than restoring against garbage.
func TestMarshalProtoListResponseRestoringNullsReportsAMalformedElementBody(t *testing.T) {
	t.Parallel()

	page := &linodev1.ReservedIPListResponse{
		Count:       1,
		ReservedIps: []*linodev1.ReservedIPAddress{{Address: reservedIPAddressFixture}},
	}

	_, err := tools.MarshalProtoListResponseRestoringNulls(
		page, []json.RawMessage{json.RawMessage(`{"bad`)}, []string{reservedIPGatewayKey},
	)
	if !errors.Is(err, tools.ErrResponseDecode) {
		t.Fatalf("err = %v, want the decode failure", err)
	}
}

// TestFetchCollectionElementSelectsTheAddressedElement: the removal's trailing
// id names one element of the page its parent route answers with, and that is
// the resource a preview and a plan hash report.
func TestFetchCollectionElementSelectsTheAddressedElement(t *testing.T) {
	t.Parallel()

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != certificateListPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, certificateListPath)
		}

		gotQuery = r.URL.RawQuery

		if _, err := w.Write([]byte(certificatePage)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	state, err := tools.FetchCollectionElement(t.Context(), newCollectionClient(server.URL),
		certificateListTool, []any{certificateConfigID}, certificateWantedID, newCertificate,
		func(item *linodev1.IamIdpConfigCertificate) bool { return item.GetId() == certificateWantedID })
	if err != nil {
		t.Fatalf("FetchCollectionElement: %v", err)
	}

	if want := "page=1&page_size=500"; gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}

	declared, err := tools.DeclaredStateOf(state)
	if err != nil {
		t.Fatalf("DeclaredStateOf: %v", err)
	}

	if got := declared.Text("certificate"); got != nestedLabelTwo {
		t.Errorf("certificate = %q, want the addressed one", got)
	}
}

// TestFetchCollectionElementReportsAMissingID: a page that carries no element
// with the id says so, rather than previewing a resource the caller did not
// name.
func TestFetchCollectionElementReportsAMissingID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(certificatePage)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, err := tools.FetchCollectionElement(t.Context(), newCollectionClient(server.URL),
		certificateListTool, []any{certificateConfigID}, certificateUnknownID, newCertificate,
		func(item *linodev1.IamIdpConfigCertificate) bool { return item.GetId() == certificateUnknownID })
	if !errors.Is(err, tools.ErrCollectionElement) {
		t.Fatalf("err = %v, want %v", err, tools.ErrCollectionElement)
	}

	// The whole sentence is pinned rather than a fragment: Python's
	// read_collection_state raises this text, so drift in either language
	// shows up here.
	want := "collection holds no matching element: " +
		"linode_iam_idp_config_certificate_list carries no id 'cert-9'"
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err, want)
	}
}

// TestFetchCollectionElementPassesAFailedFetchThrough: the failure a caller
// sees is the route's own, not one this seam wrapped around it.
func TestFetchCollectionElementPassesAFailedFetchThrough(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := tools.FetchCollectionElement(t.Context(), newCollectionClient(server.URL),
		certificateListTool, []any{certificateConfigID}, certificateWantedID, newCertificate,
		func(_ *linodev1.IamIdpConfigCertificate) bool { return true })
	if errors.Is(err, tools.ErrCollectionElement) {
		t.Fatalf("err = %v, want the route's own failure rather than the selection's", err)
	}

	if _, isAPI := errors.AsType[*linode.APIError](err); !isAPI {
		t.Errorf("err = %v, want the route's own API failure", err)
	}
}

// newCollectionClient is the API client the collection fetch reads through,
// with retry off so a failing fetch reports once.
func newCollectionClient(baseURL string) *linode.Client {
	return linode.NewClient(baseURL, tokenTest, nil, linode.WithMaxRetries(0))
}
