package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	fwAssignIDs       = "firewall_ids"
	fwAssignLinodeID  = "linode_id"
	fwAssignBalancer  = "nodebalancer_id"
	fwAssignInstPath  = "/linode/instances/123/firewalls"
	fwAssignNBPath    = "/nodebalancers/5/firewalls"
	fwAssignPage      = `{"data":[{"id":9,"label":"edge","status":"enabled"}],"page":2,"pages":2,"results":1}`
	fwAssignCanonical = `[{"id":9,"label":"edge","status":"enabled","tags":[],"created":"","updated":""}]`
)

// TestInstanceFirewallUpdatePreviewReadsTheAssignmentsItReplaces pins the three
// coupled rulings on one call: the state is the firewall list normalized
// through its descriptor, the fetch asks for the page the live call would ask
// for, and the sentence Python carried alone now lands.
func TestInstanceFirewallUpdatePreviewReadsTheAssignmentsItReplaces(t *testing.T) {
	t.Parallel()

	server, asked := fwAssignAPI(t, fwAssignPage, http.StatusOK)

	request := requestWith(map[string]any{
		fwAssignLinodeID: float64(123), fwAssignIDs: []any{float64(1)},
		"page": float64(2), "page_size": float64(50), keyDryRun: true,
	})

	result, err := toolhooks.LinodeInstanceFirewallUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, fwAssignInstPath+"?page=2&page_size=50",
		map[string]any{fwAssignIDs: []int{1}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if *asked != "page=2&page_size=50" {
		t.Errorf("fetch query = %q, want the page the call would ask for", *asked)
	}

	preview := nbDecodePreview(t, resultText(t, result))

	fwAssignStateEquals(t, preview.CurrentState)

	want := "Firewall assignments for Linode 123 will be replaced."
	if len(preview.SideEffects) != 1 || preview.SideEffects[0] != want {
		t.Errorf("side_effects = %v, want [%q]", preview.SideEffects, want)
	}
}

// TestInstanceFirewallUpdatePreviewReportsAFailedRead covers the fetch that
// cannot answer: a preview built on a read that failed would describe state the
// caller does not have.
func TestInstanceFirewallUpdatePreviewReportsAFailedRead(t *testing.T) {
	t.Parallel()

	server, _ := fwAssignAPI(t, `{"errors":[{"reason":"unauthorized"}]}`, http.StatusUnauthorized)

	request := requestWith(map[string]any{
		fwAssignLinodeID: float64(123), fwAssignIDs: []any{float64(1)}, keyDryRun: true,
	})

	result, err := toolhooks.LinodeInstanceFirewallUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, fwAssignInstPath,
		map[string]any{fwAssignIDs: []int{1}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want the failed read reported")
	}
}

// TestNodeBalancerFirewallUpdatePreviewReadsItsOwnFirewalls pins the two
// convergences the twin carries: Python read the NodeBalancer where the state
// is its firewalls, and neither language said anything about the change.
func TestNodeBalancerFirewallUpdatePreviewReadsItsOwnFirewalls(t *testing.T) {
	t.Parallel()

	server, asked := fwAssignAPI(t, fwAssignPage, http.StatusOK)

	request := requestWith(map[string]any{
		fwAssignBalancer: float64(5), fwAssignIDs: []any{float64(1)}, keyDryRun: true,
	})

	result, err := toolhooks.LinodeNodebalancerFirewallUpdatePreview(t.Context(), &request,
		configFor(server.URL), http.MethodPut, fwAssignNBPath,
		map[string]any{fwAssignIDs: []int{1}})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	if *asked != "" {
		t.Errorf("fetch query = %q, want none for a caller who asked for no page", *asked)
	}

	preview := nbDecodePreview(t, resultText(t, result))

	fwAssignStateEquals(t, preview.CurrentState)

	if len(preview.SideEffects) != 0 {
		t.Errorf("side_effects = %v, want none", preview.SideEffects)
	}
}

// fwAssignStateEquals holds the preview state to the elements the tool's own
// answer carries: every member the descriptor declares, the unset message left
// out, read past the whitespace and key order the envelope normalizes.
func fwAssignStateEquals(t *testing.T, state json.RawMessage) {
	t.Helper()

	var got, want any

	if err := json.Unmarshal(state, &got); err != nil {
		t.Fatalf("decode current_state: %v", err)
	}

	if err := json.Unmarshal([]byte(fwAssignCanonical), &want); err != nil {
		t.Fatalf("decode want: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("current_state = %s, want %s", state, fwAssignCanonical)
	}
}

// fwAssignAPI answers one read with body and records the query it was asked
// under, which is what the page-carrying fetch is judged by.
func fwAssignAPI(t *testing.T, body string, status int) (*httptest.Server, *string) {
	t.Helper()

	asked := new(string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = r.URL.RawQuery

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write stub response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, asked
}
