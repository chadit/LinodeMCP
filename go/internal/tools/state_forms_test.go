package tools_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestFetchCompositeStateReportsAnUnreadableElement: a page element the
// projection cannot read fails the fetch whole, so a plan never hashes half a
// state.
func TestFetchCompositeStateReportsAnUnreadableElement(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(`{"data":[null],"page":1,"pages":1,"results":1}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := linode.NewClient(server.URL, tokenTest, nil, linode.WithMaxRetries(0))

	_, err := tools.FetchCompositeState(t.Context(), client, []tools.CompositeCall{
		{
			Tool:   toolInstanceDiskList,
			Member: memberDisks,
			Fields: []string{keySupportTicketID},
			Values: []any{123},
			List:   true,
			New:    func() proto.Message { return &linodev1.InstanceDisk{} },
		},
	})
	if err == nil {
		t.Fatal("an unreadable element answered a state, want the failure")
	}
}

// scriptedServer answers one body per request, in order, repeating the last.
func scriptedServer(t *testing.T, bodies ...string) *httptest.Server {
	t.Helper()

	var served int

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := bodies[min(served, len(bodies)-1)]
		served++

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
}

func stateFormClient(url string) *linode.Client {
	return linode.NewClient(url, tokenTest, nil, linode.WithMaxRetries(0))
}

func newVLAN() *linodev1.VLAN { return &linodev1.VLAN{} }

// The tools and members the state-form cases read through, shared so the
// package's literal budget stays flat.
const (
	toolInstanceGet      = "linode_instance_get"
	toolInstanceDiskList = "linode_instance_disk_list"
	memberDisks          = "disks"
	scanVLANLabel        = "vl-app"
)

// TestFetchCollectionScanFindsAMatchPastTheFirstPage: the scan pages to the
// end, so a resource on the second page is found rather than reported missing.
func TestFetchCollectionScanFindsAMatchPastTheFirstPage(t *testing.T) {
	t.Parallel()

	fullEntries := make([]string, 0, 500)
	for range 500 {
		fullEntries = append(fullEntries, `{"label":"other","region":"us-east","linodes":[]}`)
	}

	fullPage := `{"data":[` + strings.Join(fullEntries, ",") + `],"page":1,"pages":2,"results":501}`
	lastPage := `{"data":[{"label":"vl-app","region":"us-east","linodes":[7]}],"page":2,"pages":2,"results":501}`

	server := scriptedServer(t, fullPage, lastPage)
	defer server.Close()

	state, err := tools.FetchCollectionScan(t.Context(), stateFormClient(server.URL),
		"linode_vlan_list", []any{}, newVLAN,
		func(item *linodev1.VLAN) bool { return item.GetLabel() == scanVLANLabel },
		"label='vl-app'")
	if err != nil {
		t.Fatalf("FetchCollectionScan: %v", err)
	}

	declared, err := tools.DeclaredStateOf(state)
	if err != nil {
		t.Fatalf("DeclaredStateOf: %v", err)
	}

	if declared.Text("label") != scanVLANLabel {
		t.Errorf("label = %q, want vl-app", declared.Text("label"))
	}
}

// TestFetchCollectionScanReportsAMissingElement: the not-found sentence names
// the pairs, which is what a preview reports for a resource nothing matched.
func TestFetchCollectionScanReportsAMissingElement(t *testing.T) {
	t.Parallel()

	server := scriptedServer(t, `{"data":[],"page":1,"pages":1,"results":0}`)
	defer server.Close()

	_, err := tools.FetchCollectionScan(t.Context(), stateFormClient(server.URL),
		"linode_vlan_list", []any{}, newVLAN,
		func(*linodev1.VLAN) bool { return false },
		"label='ghost'")

	if !errors.Is(err, tools.ErrCollectionElement) {
		t.Errorf("err = %v, want the collection not-found sentinel", err)
	}
}

// TestFetchCollectionScanHandsBackAFailedRead: the route's own failure is the
// report, not a sentence wrapped around it.
func TestFetchCollectionScanHandsBackAFailedRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := tools.FetchCollectionScan(t.Context(), stateFormClient(server.URL),
		"linode_vlan_list", []any{}, newVLAN,
		func(*linodev1.VLAN) bool { return true },
		"label='vl-app'")
	if err == nil {
		t.Error("a failed read answered a state, want the failure")
	}
}

func envelopeState(t *testing.T, body string) (any, error) {
	t.Helper()

	server := scriptedServer(t, body)
	defer server.Close()

	state, err := tools.FetchEnvelopeState(t.Context(), stateFormClient(server.URL),
		"linode_tag_object_list", []any{canRunEnvProd}, "",
		func() *linodev1.TaggedObject { return &linodev1.TaggedObject{} })
	if err != nil {
		return nil, fmt.Errorf("envelope state: %w", err)
	}

	return state, nil
}

// TestFetchEnvelopeStateKeepsDataAndResults: the envelope itself is the state,
// the elements projected and the API's total riding along.
func TestFetchEnvelopeStateKeepsDataAndResults(t *testing.T) {
	t.Parallel()

	state, err := envelopeState(t,
		`{"data":[{"type":"linode","data":{"id":5}}],"page":1,"pages":5,"results":9}`)
	if err != nil {
		t.Fatalf("FetchEnvelopeState: %v", err)
	}

	declared, err := tools.DeclaredStateOf(state)
	if err != nil {
		t.Fatalf("DeclaredStateOf: %v", err)
	}

	if results, _ := declared.Number("results"); results != 9 {
		t.Errorf("results = %d, want the envelope total 9", results)
	}

	if len(declared.Objects("data")) != 1 {
		t.Errorf("data = %v, want the one projected element", declared.Objects("data"))
	}
}

// TestFetchEnvelopeStateReadsThePageItWasGiven: a replacement previews the page
// its own call publishes, so the read carries the controls rather than falling
// back to the route's defaults and describing a different page.
func TestFetchEnvelopeStateReadsThePageItWasGiven(t *testing.T) {
	t.Parallel()

	asked := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked <- r.URL.RawQuery

		if _, err := w.Write([]byte(`{"data":[],"results":0}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	if _, err := tools.FetchEnvelopeState(t.Context(), stateFormClient(server.URL),
		"linode_tag_object_list", []any{canRunEnvProd}, listPageQuery,
		func() *linodev1.TaggedObject { return &linodev1.TaggedObject{} }); err != nil {
		t.Fatalf("FetchEnvelopeState: %v", err)
	}

	if query := <-asked; query != listPageQuery {
		t.Errorf("query = %q, want the controls the caller published", query)
	}
}

// TestFetchEnvelopeStateRefusesABrokenEnvelope: each malformed member answers
// the decode's own report rather than a state with a hole in it.
func TestFetchEnvelopeStateRefusesABrokenEnvelope(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"data that is not a list":         `{"data":{"nope":true},"results":1}`,
		"a results that is not a number":  `{"data":[],"results":"many"}`,
		"an element the contract refuses": `{"data":[{"type":123}],"results":1}`,
		"an element sent as null":         `{"data":[null],"results":1}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := envelopeState(t, body); err == nil {
				t.Error("a broken envelope answered a state, want the failure")
			}
		})
	}
}

// TestFetchCompositeStateAssemblesItsMembers: each call lands its kept fields
// under its member, an object for a read and an array for a list.
func TestFetchCompositeStateAssemblesItsMembers(t *testing.T) {
	t.Parallel()

	server := scriptedServer(t,
		`{"id":123,"type":"g6-nanode-1","label":"cosmetic"}`,
		`{"data":[{"id":1,"size":100,"filesystem":"ext4","label":"cosmetic"}],"page":1,"pages":1,"results":1}`)
	defer server.Close()

	state, err := tools.FetchCompositeState(t.Context(), stateFormClient(server.URL), []tools.CompositeCall{
		{
			Tool: toolInstanceGet, Member: "instance", Fields: []string{argType},
			Values: []any{123},
			New:    func() proto.Message { return &linodev1.Instance{} },
		},
		{
			Tool: toolInstanceDiskList, Member: memberDisks, Fields: []string{"id", "size"},
			Values: []any{123}, List: true,
			New: func() proto.Message { return &linodev1.InstanceDisk{} },
		},
	})
	if err != nil {
		t.Fatalf("FetchCompositeState: %v", err)
	}

	declared, err := tools.DeclaredStateOf(state)
	if err != nil {
		t.Fatalf("DeclaredStateOf: %v", err)
	}

	if declared.Object("instance").Text("type") != typeG6Nanode1 {
		t.Errorf("instance = %v, want the kept type", declared.Object("instance"))
	}

	if declared.Object("instance").Text("label") != "" {
		t.Error("a cosmetic field survived the kept subset")
	}

	if len(declared.Objects(memberDisks)) != 1 {
		t.Errorf("disks = %v, want the one kept element", declared.Objects(memberDisks))
	}
}

// TestFetchCompositeStateFailsWholeOnAFailedRead: a plan hashed over half a
// state would refuse an apply for a change nobody made.
func TestFetchCompositeStateFailsWholeOnAFailedRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := tools.FetchCompositeState(t.Context(), stateFormClient(server.URL), []tools.CompositeCall{
		{
			Tool: toolInstanceGet, Member: "instance", Fields: []string{argType},
			Values: []any{123},
			New:    func() proto.Message { return &linodev1.Instance{} },
		},
	})
	if err == nil {
		t.Error("a failed read answered a state, want the failure")
	}
}

// TestFetchEnvelopeStateHandsBackAFailedRead: the route's own failure is the
// report, the same stance every state form takes.
func TestFetchEnvelopeStateHandsBackAFailedRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := tools.FetchEnvelopeState(t.Context(), stateFormClient(server.URL),
		"linode_tag_object_list", []any{canRunEnvProd}, "",
		func() *linodev1.TaggedObject { return &linodev1.TaggedObject{} })
	if err == nil {
		t.Error("a failed read answered a state, want the failure")
	}
}

// TestDeclaredStateObjectReadsEitherShapeAndAnswersEmpty: a member can arrive
// as the projected shape or the raw map, and one nobody sent reads as empty.
func TestDeclaredStateObjectReadsEitherShapeAndAnswersEmpty(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{
		"projected": tools.DeclaredState{argType: typeG6Nanode1},
		"raw":       map[string]any{"type": typeG6Standard1},
	}

	if got := state.Object("projected").Text("type"); got != typeG6Nanode1 {
		t.Errorf("projected type = %q, want g6-nanode-1", got)
	}

	if got := state.Object("raw").Text("type"); got != typeG6Standard1 {
		t.Errorf("raw type = %q, want g6-standard-1", got)
	}

	if got := state.Object("absent"); len(got) != 0 {
		t.Errorf("absent member = %v, want empty", got)
	}
}
