package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	// reservedIPGetTool is the read whose answer restores the nulls its decode
	// drops, which is what makes it the raw primitive's subject.
	reservedIPGetTool = "linode_networking_reserved_ip_get"
	// ruleVersionListTool is the one route answering a single object whose
	// version the contract hoists out of its rules.
	ruleVersionListTool = "linode_firewall_rule_version_list"
	// preferencesUpdateTool is the one mutation decoding into a free-form
	// payload, where a JSON null answer reads as no members rather than as a
	// malformed body.
	preferencesUpdateTool = "linode_profile_preferences_update"

	// reservedAddress is the address the raw read is addressed by.
	reservedAddress = "192.0.2.10"
)

// TestCallProtoRouteQueryRawHandsBackTheBodyItDecoded: the decode drops a
// documented null, so the caller needs the bytes to write it back.
func TestCallProtoRouteQueryRawHandsBackTheBodyItDecoded(t *testing.T) {
	t.Parallel()

	body := `{"address":"192.0.2.10","gateway":null,"region":"us-east"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	address := &linodev1.ReservedIPAddress{}

	raw, err := newRouteTestClient(t, server.URL).
		CallProtoRouteQueryRaw(t.Context(), reservedIPGetTool, []any{reservedAddress}, "", address)
	if err != nil {
		t.Fatalf("CallProtoRouteQueryRaw: %v", err)
	}

	if address.GetRegion() != accountTransferRegion {
		t.Errorf("decoded region = %q, want %q", address.GetRegion(), accountTransferRegion)
	}

	if string(raw) != body {
		t.Errorf("raw body = %s, want %s", raw, body)
	}
}

// TestCallProtoRouteQueryRawRefusesANonObjectBody: protojson would report the
// decoder's own complaint, which cannot say which call answered badly. The
// subject is derived from the tool name, and Python derives the same one.
func TestCallProtoRouteQueryRawRefusesANonObjectBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`["reserved"]`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, err := newRouteTestClient(t, server.URL).
		CallProtoRouteQueryRaw(t.Context(), reservedIPGetTool, []any{reservedAddress}, "",
			&linodev1.ReservedIPAddress{})
	if !errors.Is(err, linode.ErrWriteResponseNotObject) {
		t.Fatalf("error = %v, want the shared shape guard", err)
	}

	if want := "networking reserved ip get response must be a JSON object"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// TestCallProtoRouteQueryRefusesANonObjectBody: the plain read primitive holds
// its answer to the same shape, so every generated read reports one sentence.
func TestCallProtoRouteQueryRefusesANonObjectBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`"a domain"`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	err := newRouteTestClient(t, server.URL).
		CallProtoRouteQuery(t.Context(), domainGetTool, []any{5}, "", &linodev1.Domain{})
	if err == nil {
		t.Fatal("expected a refusal for a body that is not an object")
	}

	if want := "domain get response must be a JSON object"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// TestListProtoRouteLiftsTheMemberTheEnvelopeDeclares: the firewall history's
// version sits inside its rules, and the shared rules message declares none, so
// without the hoist the snapshot answers version 0.
func TestListProtoRouteLiftsTheMemberTheEnvelopeDeclares(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"id":5,"label":"web","rules":{"version":2,"inbound_policy":"DROP"}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	items, err := linode.ListProtoRoute(t.Context(), newRouteTestClient(t, server.URL),
		ruleVersionListTool, []any{5}, "", 0, 0,
		func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
	if err != nil {
		t.Fatalf("ListProtoRoute: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("items = %d, want the one snapshot the route answers with", len(items))
	}

	if items[0].GetVersion() != 2 {
		t.Errorf("version = %d, want 2 lifted out of rules", items[0].GetVersion())
	}

	if items[0].GetRules().GetInboundPolicy() != "DROP" {
		t.Errorf("rules survived the hoist as %+v, want the policy intact", items[0].GetRules())
	}
}

// TestListProtoRouteLeavesAMemberTheBodyNestsNothingUnder: a source the answer
// does not carry leaves the member as the decode found it, rather than writing
// a zero over it.
func TestListProtoRouteLeavesAMemberTheBodyNestsNothingUnder(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"no rules":          `{"id":5,"version":9}`,
		"rules without one": `{"id":5,"version":9,"rules":{"inbound_policy":"DROP"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if _, err := w.Write([]byte(body)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			items, err := linode.ListProtoRoute(t.Context(), newRouteTestClient(t, server.URL),
				ruleVersionListTool, []any{5}, "", 0, 0,
				func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
			if err != nil {
				t.Fatalf("ListProtoRoute: %v", err)
			}

			if len(items) != 1 || items[0].GetVersion() != 9 {
				t.Errorf("items = %+v, want the body's own version kept", items)
			}
		})
	}
}

// TestCallProtoRouteBodyReadsANullFreeFormAnswerAsEmpty: a route with no schema
// to model is the one shape a JSON null has an exact reading in.
func TestCallProtoRouteBodyReadsANullFreeFormAnswerAsEmpty(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`null`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	payload := &structpb.Struct{}

	err := newRouteTestClient(t, server.URL).CallProtoRouteBody(t.Context(), preferencesUpdateTool,
		nil, json.RawMessage(`{"theme":"dark"}`), "profile preferences update", payload)
	if err != nil {
		t.Fatalf("CallProtoRouteBody: %v", err)
	}

	if len(payload.GetFields()) != 0 {
		t.Errorf("payload = %+v, want no members", payload.GetFields())
	}
}

// TestCallProtoRouteBodyStillRefusesANonObjectFreeFormAnswer: only the null
// literal has a reading. An array is the malformed body it has always been.
func TestCallProtoRouteBodyStillRefusesANonObjectFreeFormAnswer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`[]`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	err := newRouteTestClient(t, server.URL).CallProtoRouteBody(t.Context(), preferencesUpdateTool,
		nil, json.RawMessage(`{"theme":"dark"}`), "profile preferences update", &structpb.Struct{})
	if err == nil {
		t.Fatal("expected a refusal for a body that is not an object")
	}

	if want := "profile preferences update response must be a JSON object"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// TestListProtoRouteRefusesAValueTheMemberCannotHold: the patch decodes through
// the same descriptor the element does, so a nested value of the wrong type is
// reported rather than merged as a zero.
func TestListProtoRouteRefusesAValueTheMemberCannotHold(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"id":5,"rules":{"version":"two"}}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, err := linode.ListProtoRoute(t.Context(), newRouteTestClient(t, server.URL),
		ruleVersionListTool, []any{5}, "", 0, 0,
		func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
	if err == nil {
		t.Fatal("expected the hoist to report a value the member cannot hold")
	}
}

// TestListProtoRouteReportsAnElementTheProtoCannotRead: the decode runs before
// the hoist, so an object whose members do not fit the descriptor is reported
// in the decoder's own words rather than reaching the hoist at all.
func TestListProtoRouteReportsAnElementTheProtoCannotRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"id":5,"rules":"none"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, err := linode.ListProtoRoute(t.Context(), newRouteTestClient(t, server.URL),
		ruleVersionListTool, []any{5}, "", 0, 0,
		func() *linodev1.FirewallRuleVersion { return &linodev1.FirewallRuleVersion{} })
	if err == nil {
		t.Fatal("expected the decode to report an element it cannot read")
	}
}

// TestCallProtoRouteQueryRawReportsAnUnfillableRoute: a route the contract
// cannot fill never reached the network, so it comes back in the argument class
// rather than as a transport failure a retry would repeat.
func TestCallProtoRouteQueryRawReportsAnUnfillableRoute(t *testing.T) {
	t.Parallel()

	_, err := newRouteTestClient(t, "http://127.0.0.1:1").
		CallProtoRouteQueryRaw(t.Context(), reservedIPGetTool, nil, "", &linodev1.ReservedIPAddress{})
	if err == nil {
		t.Fatal("expected the arity mismatch to be refused")
	}
}
