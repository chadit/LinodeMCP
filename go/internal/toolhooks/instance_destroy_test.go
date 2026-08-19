package toolhooks_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	instanceDeletePath    = "/linode/instances/123"
	instanceDeleteType    = "g6-standard-2"
	instanceRunningStatus = "running"

	// The delete walk's own sentence, spelled here so the test fails when the
	// warning changes rather than when the walk stops adding one.
	runningDeleteWarningText = "Instance is currently running." +
		" Delete will not pause for a graceful shutdown."
)

// routeStubServer answers each path from routes and 404s anything else, so a
// walk that reaches a route the test did not stub fails visibly rather than
// silently degrading to its warning branch.
func routeStubServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, stubbed := routes[r.URL.Path]
		if !stubbed {
			w.WriteHeader(http.StatusNotFound)

			if _, err := w.Write([]byte(`{"errors":[{"reason":"not stubbed"}]}`)); err != nil {
				t.Errorf("write response: %v", err)
			}

			return
		}

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// fullWalkRoutes is the stub set where every sub-fetch of the delete walk
// answers, which is the branch that fills the dependency list.
func fullWalkRoutes() map[string]string {
	return map[string]string{
		instanceDeletePath:                `{"id":123,"label":"web-01","status":"running","type":"g6-standard-2"}`,
		instanceDeletePath + "/volumes":   `{"data":[{"id":11,"label":"data-vol","size":50}],"page":1,"pages":1,"results":1}`,
		instanceDeletePath + "/ips":       `{"ipv4":{"public":[{"address":"203.0.113.10"}]}}`,
		instanceDeletePath + "/firewalls": `{"data":[{"id":42,"label":"web-fw"}],"page":1,"pages":1,"results":1}`,
		// The type's monthly price is what the billing estimate reports back as
		// the change the delete makes.
		"/linode/types/" + instanceDeleteType: `{"id":"g6-standard-2","price":{"hourly":0.03,"monthly":20.0}}`,
	}
}

// TestLinodeInstanceDeleteDependencyWalkNamesEveryDependent: a delete detaches
// volumes, releases public addresses, and drops firewall attachments, and none
// of those are visible from the request alone.
func TestLinodeInstanceDeleteDependencyWalkNamesEveryDependent(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, fullWalkRoutes())
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	state := declaredState(t, `{"id":123,"status":"running","type":"g6-standard-2"}`, &linodev1.Instance{})

	details, err := toolhooks.LinodeInstanceDeleteDependencyWalk(t.Context(), client, 123, state)
	if err != nil {
		t.Fatalf("LinodeInstanceDeleteDependencyWalk: %v", err)
	}

	kinds := make([]string, 0, len(details.Dependencies))
	for i := range details.Dependencies {
		kinds = append(kinds, details.Dependencies[i].Kind)
	}

	if !slices.Equal(kinds, []string{"volume", "public_ip", nameFirewall}) {
		t.Errorf("kinds = %v, want [volume public_ip firewall]", kinds)
	}

	actions := []string{
		tools.DependencyActionDetached,
		tools.DependencyActionReleased,
		tools.DependencyActionRemoved,
	}
	for i := range details.Dependencies {
		if details.Dependencies[i].Action != actions[i] {
			t.Errorf("dependency %d action = %q, want %q", i, details.Dependencies[i].Action, actions[i])
		}
	}

	if details.BillingDelta == nil || details.BillingDelta.MonthlyChangeUSD != "-20.00" {
		t.Errorf("billing = %#v, want -20.00", details.BillingDelta)
	}

	if !slices.Contains(details.Warnings, runningDeleteWarningText) {
		t.Errorf("warnings = %v, want the running-delete warning", details.Warnings)
	}
}

// TestLinodeInstanceDeleteDependencyWalkWarnsPerFailedFetch: each sub-list is
// best-effort, so three failed fetches leave three warnings and a previewable
// answer rather than one error and nothing to decide on.
func TestLinodeInstanceDeleteDependencyWalkWarnsPerFailedFetch(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, nil)
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	state := declaredState(t, `{"id":123,"status":"offline","type":"g6-standard-2"}`, &linodev1.Instance{})

	details, err := toolhooks.LinodeInstanceDeleteDependencyWalk(t.Context(), client, 123, state)
	if err != nil {
		t.Fatalf("LinodeInstanceDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 0 {
		t.Errorf("dependencies = %v, want none", details.Dependencies)
	}

	prefixes := []string{
		"Could not list attached volumes:",
		"Could not list IP addresses:",
		"Could not list firewalls:",
	}
	for i, prefix := range prefixes {
		if i >= len(details.Warnings) || !strings.HasPrefix(details.Warnings[i], prefix) {
			t.Fatalf("warnings = %v, want one starting %q", details.Warnings, prefix)
		}
	}

	// An offline instance adds no running warning, so the three fetch failures
	// are the whole list.
	if len(details.Warnings) != len(prefixes) {
		t.Errorf("warnings = %v, want exactly %d", details.Warnings, len(prefixes))
	}

	if details.BillingDelta == nil || details.BillingDelta.MonthlyChangeUSD != tools.BillingUnknown {
		t.Errorf("billing = %#v, want the unknown sentinel", details.BillingDelta)
	}
}

// TestLinodeInstanceDeleteDependencyWalkReadsAStateSayingNothing: a Linode the
// read answered with nothing but an id still gets its dependencies fetched and
// the unknown-price sentinel, rather than a guess or a silent empty walk.
func TestLinodeInstanceDeleteDependencyWalkReadsAStateSayingNothing(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, fullWalkRoutes())
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeInstanceDeleteDependencyWalk(
		t.Context(), client, 123, declaredState(t, `{"id":123}`, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("LinodeInstanceDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 3 {
		t.Errorf("dependencies = %v, want three", details.Dependencies)
	}

	if details.BillingDelta == nil || details.BillingDelta.MonthlyChangeUSD != tools.BillingUnknown {
		t.Errorf("billing = %#v, want the unknown sentinel", details.BillingDelta)
	}

	if len(details.Warnings) != 0 {
		t.Errorf("warnings = %v, want none over a state naming no status", details.Warnings)
	}
}

// TestLinodeInstanceDeleteDependencyWalkReportsPricingItCannotRead covers the
// two estimate branches a fetched instance can land on: a type the pricing
// route refuses, and no type at all.
func TestLinodeInstanceDeleteDependencyWalkReportsPricingItCannotRead(t *testing.T) {
	t.Parallel()

	routes := fullWalkRoutes()
	delete(routes, "/linode/types/"+instanceDeleteType)

	server := routeStubServer(t, routes)
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	cases := []struct {
		name     string
		body     string
		wantNote string
	}{
		{
			name:     "pricing route refuses",
			body:     `{"id":123,"type":"g6-standard-2"}`,
			wantNote: "Could not fetch type pricing for the estimate.",
		},
		{
			name:     "no type to price",
			body:     `{"id":123}`,
			wantNote: "",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			details, err := toolhooks.LinodeInstanceDeleteDependencyWalk(
				t.Context(), client, 123, declaredState(t, testCase.body, &linodev1.Instance{}),
			)
			if err != nil {
				t.Fatalf("LinodeInstanceDeleteDependencyWalk: %v", err)
			}

			if details.BillingDelta == nil ||
				details.BillingDelta.MonthlyChangeUSD != tools.BillingUnknown {
				t.Fatalf("billing = %#v, want the unknown sentinel", details.BillingDelta)
			}

			if details.BillingDelta.Note != testCase.wantNote {
				t.Errorf("note = %q, want %q", details.BillingDelta.Note, testCase.wantNote)
			}
		})
	}
}

// TestLinodeInstanceDeleteDependencyWalkSkipsAddresslessIPs: a Linode with no
// IPv4 block reports no released addresses rather than an empty entry.
func TestLinodeInstanceDeleteDependencyWalkSkipsAddresslessIPs(t *testing.T) {
	t.Parallel()

	routes := fullWalkRoutes()
	routes[instanceDeletePath+"/ips"] = emptyJSONBody

	server := routeStubServer(t, routes)
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeInstanceDeleteDependencyWalk(
		t.Context(), client, 123, declaredState(t, `{"id":123,"type":"g6-standard-2"}`, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("LinodeInstanceDeleteDependencyWalk: %v", err)
	}

	kinds := make([]string, 0, len(details.Dependencies))
	for i := range details.Dependencies {
		kinds = append(kinds, details.Dependencies[i].Kind)
	}

	if !slices.Equal(kinds, []string{"volume", nameFirewall}) {
		t.Errorf("kinds = %v, want [volume firewall]", kinds)
	}
}
