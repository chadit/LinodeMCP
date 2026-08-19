package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// A generated list tool reads its page through ListProtoRoute for every route,
// so which JSON shape that body is decoded as comes from the tool's declared
// list_envelope. These pin that each declared shape reads the body the route
// really sends: assuming the standard page for all of them answers a populated
// collection as an empty one and reports success.

const (
	envelopeToken = "list-envelope-token"
	keyInterfaces = "interfaces"
)

// envelopeServer answers every request with body, already encoded.
func envelopeServer(t *testing.T, body any) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))

	t.Cleanup(srv.Close)

	return srv
}

// The interfaces route puts its elements under a member of its own, so a reader
// looking under "data" would answer an empty page for a Linode that has two.
func TestListProtoRouteDecodesAKeyedEnvelope(t *testing.T) {
	t.Parallel()

	srv := envelopeServer(t, map[string]any{
		keyInterfaces: []any{map[string]any{keyID: 11}, map[string]any{keyID: 12}},
	})
	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	items, err := linode.ListProtoRoute(t.Context(), client, "linode_instance_interface_list",
		[]any{7}, "", 1, 25, func() *linodev1.InstanceInterface { return &linodev1.InstanceInterface{} })
	if err != nil {
		t.Fatalf("ListProtoRoute() error = %v, want no error", err)
	}

	if len(items) != 2 {
		t.Fatalf("len(items) = %v, want %v", len(items), 2)
	}

	if items[0].GetId() != 11 {
		t.Errorf("items[0].GetId() = %v, want %v", items[0].GetId(), 11)
	}
}

// The region availability route answers a top-level array with no envelope at
// all, which a page reader reports as a malformed body rather than decoding.
func TestListProtoRouteDecodesABareArray(t *testing.T) {
	t.Parallel()

	srv := envelopeServer(t, []any{
		map[string]any{keyRegion: accountTransferRegion, "plan": "g6-standard-1"},
	})
	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	items, err := linode.ListProtoRoute(t.Context(), client, "linode_region_availability_get",
		[]any{"us-east"}, "", 1, 25, func() *linodev1.RegionAvailability { return &linodev1.RegionAvailability{} })
	if err != nil {
		t.Fatalf("ListProtoRoute() error = %v, want no error", err)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %v, want %v", len(items), 1)
	}

	if items[0].GetRegion() != accountTransferRegion {
		t.Errorf("items[0].GetRegion() = %v, want %v", items[0].GetRegion(), accountTransferRegion)
	}
}

// A trusted-device page with no data member is a truncated body, not a profile
// with no remembered browsers, so this route fails closed.
func TestListProtoRouteRefusesAPageMissingItsRequiredData(t *testing.T) {
	t.Parallel()

	srv := envelopeServer(t, map[string]any{keyPage: 1, keyResults: 0})
	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	_, err := linode.ListProtoRoute(t.Context(), client, "linode_profile_device_list",
		nil, "", 1, 25, func() *linodev1.TrustedDevice { return &linodev1.TrustedDevice{} })
	if err == nil {
		t.Fatal("ListProtoRoute() error = nil, want a malformed-body failure")
	}
}

// The same absent data member on an ordinary collection is an empty page, which
// is what keeps the strict reading scoped to the routes that declared it.
func TestListProtoRouteReadsAnAbsentDataMemberAsAnEmptyPage(t *testing.T) {
	t.Parallel()

	srv := envelopeServer(t, map[string]any{keyPage: 1, keyResults: 0})
	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	items, err := linode.ListProtoRoute(t.Context(), client, "linode_domain_list",
		nil, "", 1, 25, func() *linodev1.Domain { return &linodev1.Domain{} })
	if err != nil {
		t.Fatalf("ListProtoRoute() error = %v, want no error", err)
	}

	if len(items) != 0 {
		t.Errorf("len(items) = %v, want %v", len(items), 0)
	}
}

// The declaration is what the reader keys on, so it is read back here by name:
// a tool losing its annotation would otherwise only show up as an empty list.
func TestListEnvelopeForReadsTheDeclaredShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tool   string
		member string
		want   linoderoute.ListShape
	}{
		{tool: "linode_instance_interface_list", member: "interfaces", want: linoderoute.ListShapeKeyed},
		{
			tool:   "linode_profile_security_question_list",
			member: "security_questions",
			want:   linoderoute.ListShapeKeyed,
		},
		{tool: "linode_instance_config_interface_list", want: linoderoute.ListShapeBare},
		{tool: "linode_region_availability_get", want: linoderoute.ListShapeBare},
		{tool: "linode_profile_device_list", want: linoderoute.ListShapeRequiredData},
		{tool: "linode_domain_list", want: linoderoute.ListShapeData},
		{tool: "no_such_tool", want: linoderoute.ListShapeData},
	}

	for _, test := range tests {
		t.Run(test.tool, func(t *testing.T) {
			t.Parallel()

			got := linoderoute.ListEnvelopeFor(test.tool)

			if got.Shape != test.want {
				t.Errorf("got.Shape = %v, want %v", got.Shape, test.want)
			}

			if got.Member != test.member {
				t.Errorf("got.Member = %v, want %v", got.Member, test.member)
			}
		})
	}
}

// The rule-version history route answers one object, not a page of them, so a
// reader looking for a collection answers a real snapshot as an empty one.
func TestListProtoRouteDecodesASingletonEnvelope(t *testing.T) {
	t.Parallel()

	srv := envelopeServer(t, map[string]any{keyID: 9, "version": 3})
	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	items, err := linode.ListProtoRoute(t.Context(), client, "linode_firewall_rule_version_list",
		[]any{9}, "", 1, 25, func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
	if err != nil {
		t.Fatalf("ListProtoRoute() error = %v, want no error", err)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %v, want %v", len(items), 1)
	}

	if items[0].GetId() != 9 {
		t.Errorf("items[0].GetId() = %v, want %v", items[0].GetId(), 9)
	}
}

// A body that is not an object decodes to an empty message under
// DiscardUnknown, so the singleton reader has to refuse it rather than answer a
// page holding one blank element.
func TestListProtoRouteRefusesASingletonThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	for _, body := range []any{[]any{}, nil, "not an object", 1} {
		srv := envelopeServer(t, body)
		client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

		_, err := linode.ListProtoRoute(t.Context(), client, "linode_firewall_rule_version_list",
			[]any{9}, "", 1, 25,
			func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
		if err == nil {
			t.Errorf("ListProtoRoute() over %v error = nil, want a malformed-body failure", body)
		}
	}
}

// The query a caller composes has to reach the route, since a forwarded
// parameter is one the API itself filters on.
func TestListProtoRouteSendsTheQueryItWasGiven(t *testing.T) {
	t.Parallel()

	var gotQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("skip_ipv6_rdns")

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []any{}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))

	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	if _, err := linode.ListProtoRoute(t.Context(), client, "linode_networking_ip_list",
		nil, "skip_ipv6_rdns=true", 1, 25,
		func() *linodev1.IPAddress { return &linodev1.IPAddress{} }); err != nil {
		t.Fatalf("ListProtoRoute() error = %v, want no error", err)
	}

	if gotQuery != "true" {
		t.Errorf("skip_ipv6_rdns = %q, want %q", gotQuery, "true")
	}
}

// A singleton route that fails on the transport is reported as the failure it
// is, not as an empty page: the decode never runs, so nothing else would say so.
func TestListProtoRouteReportsASingletonTransportFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, envelopeToken, nil, linode.WithMaxRetries(0))

	if _, err := linode.ListProtoRoute(t.Context(), client, "linode_firewall_rule_version_list",
		[]any{9}, "", 1, 25,
		func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} }); err == nil {
		t.Error("ListProtoRoute() error = nil, want the API failure")
	}
}
