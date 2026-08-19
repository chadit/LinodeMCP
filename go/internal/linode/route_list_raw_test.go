package linode_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	// reservedIPListTool is the paged read whose elements restore the nulls
	// their decode drops, which is what makes it the raw list primitive's
	// subject.
	reservedIPListTool = "linode_networking_reserved_ip_list"
	// interfaceListTool answers its elements under a member of its own rather
	// than under the standard page's data.
	interfaceListTool = "linode_instance_interface_list"
	// configInterfaceListTool answers a bare array, which carries no member to
	// restore an element's nulls out of.
	configInterfaceListTool = "linode_instance_config_interface_list"
	// objectListTool pages by a cursor the raw primitive hands back no room
	// for.
	objectListTool = "linode_object_storage_bucket_object_list"
	// deviceListTool reads an absent data member as a malformed body rather
	// than as an empty page.
	deviceListTool = "linode_profile_device_list"
)

// newReservedIPElem is the element factory the raw list primitive fills.
func newReservedIPElem() *linodev1.ReservedIPAddress {
	return &linodev1.ReservedIPAddress{}
}

// TestListProtoRouteRawHandsBackEachElementBody: the decode drops the six
// documented nulls, so the caller needs each element's bytes to write them
// back into the element they came from.
func TestListProtoRouteRawHandsBackEachElementBody(t *testing.T) {
	t.Parallel()

	body := `{"data":[{"address":"192.0.2.10","gateway":null,"region":"us-east"},` +
		`{"address":"192.0.2.11","region":"us-east"}],"page":1,"pages":1,"results":2}`

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	items, raws, err := linode.ListProtoRouteRaw(t.Context(), newRouteTestClient(t, server.URL),
		reservedIPListTool, nil, "", 2, 50, newReservedIPElem)
	if err != nil {
		t.Fatalf("ListProtoRouteRaw: %v", err)
	}

	if gotQuery != tcPage2PageSize50 {
		t.Errorf("query = %q, want %q", gotQuery, tcPage2PageSize50)
	}

	if len(items) != 2 || len(raws) != 2 {
		t.Fatalf("len(items) = %d, len(raws) = %d, want 2 and 2", len(items), len(raws))
	}

	if items[0].GetAddress() != reservedAddress {
		t.Errorf("items[0] address = %q, want %q", items[0].GetAddress(), reservedAddress)
	}

	want := `{"address":"192.0.2.10","gateway":null,"region":"us-east"}`
	if string(raws[0]) != want {
		t.Errorf("raws[0] = %s, want %s", raws[0], want)
	}
}

// TestListProtoRouteRawReadsAKeyedMemberWithoutPageControls: a keyed route
// hands over its whole collection under the member it names, so the page pair
// would be a query it never publishes.
func TestListProtoRouteRawReadsAKeyedMemberWithoutPageControls(t *testing.T) {
	t.Parallel()

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery

		if _, err := w.Write([]byte(`{"interfaces":[{"id":7}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	items, raws, err := linode.ListProtoRouteRaw(t.Context(), newRouteTestClient(t, server.URL),
		interfaceListTool, []any{5}, "", 1, 25,
		func() *linodev1.InstanceInterface { return &linodev1.InstanceInterface{} })
	if err != nil {
		t.Fatalf("ListProtoRouteRaw: %v", err)
	}

	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}

	if len(items) != 1 || len(raws) != 1 {
		t.Fatalf("len(items) = %d, len(raws) = %d, want 1 and 1", len(items), len(raws))
	}
}

// TestListProtoRouteRawHoldsARequiredDataPageToItsMember: the shape that reads
// an absent data member as malformed rather than empty keeps that reading here,
// so the two primitives cannot answer the same body differently.
func TestListProtoRouteRawHoldsARequiredDataPageToItsMember(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"page":1,"pages":1,"results":0}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, _, err := linode.ListProtoRouteRaw(t.Context(), newRouteTestClient(t, server.URL),
		deviceListTool, nil, "", 1, 25,
		func() *linodev1.TrustedDevice { return &linodev1.TrustedDevice{} })
	if err == nil {
		t.Fatal("ListProtoRouteRaw read an absent data member as an empty page")
	}
}

// TestListProtoRouteRawRefusesAShapeWithNoPageMember: a bare array and a marker
// page each answer something a per-element restoration cannot read, so the
// primitive says so rather than answering a page with no nulls in it.
func TestListProtoRouteRawRefusesAShapeWithNoPageMember(t *testing.T) {
	t.Parallel()

	for _, tool := range []string{configInterfaceListTool, objectListTool} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("request reached the API on a refused shape")
			}))
			defer server.Close()

			_, _, err := linode.ListProtoRouteRaw(t.Context(), newRouteTestClient(t, server.URL),
				tool, []any{5, "bucket"}, "", 1, 25, newReservedIPElem)
			if err == nil {
				t.Fatal("ListProtoRouteRaw accepted a shape carrying no page member")
			}
		})
	}
}

// TestListProtoRouteRawReportsAMalformedPage: the envelope is held to being an
// object before a member is read out of it, the same check the non-raw
// primitive makes.
func TestListProtoRouteRawReportsAMalformedPage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`[]`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	_, _, err := linode.ListProtoRouteRaw(t.Context(), newRouteTestClient(t, server.URL),
		reservedIPListTool, nil, "", 1, 25, newReservedIPElem)
	if err == nil {
		t.Fatal("ListProtoRouteRaw accepted a page that is not an object")
	}
}
