package toolhooks_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// The zone the fetch cases read and the records the walk reports on.
const (
	domainRecords = `{"data":[` +
		`{"id":41,"type":"NS","name":"example.com","target":"ns1.linode.com"},` +
		`{"id":42,"type":"A","name":"www","target":"203.0.113.10"}` +
		`],"page":1,"pages":1,"results":2}`
)

// clientFor builds a client against a stub server with retries off, so a test
// asserting on one request is not served by a replay of it.
func clientFor(t *testing.T, baseURL string) *linode.Client {
	t.Helper()

	return linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))
}

// TestLinodeDomainDeleteDependencyWalkNamesTheDelegationItBreaks: the NS
// records are the part of a zone a caller cannot put back from memory, so they
// are reported one by one while the rest are counted.
func TestLinodeDomainDeleteDependencyWalkNamesTheDelegationItBreaks(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(domainRecords)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	details, err := toolhooks.LinodeDomainDeleteDependencyWalk(
		t.Context(), clientFor(t, server.URL), 5, nil,
	)
	if err != nil {
		t.Fatalf("LinodeDomainDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %v, want the one NS record", details.Dependencies)
	}

	dependency := details.Dependencies[0]
	if dependency.Kind != "ns_record" || dependency.Label != "ns1.linode.com" {
		t.Errorf("dependency = %+v, want the NS record it destroys", dependency)
	}

	want := "Deleting this domain destroys 2 DNS record(s), including 1 NS record(s)."
	if len(details.Warnings) != 1 || details.Warnings[0] != want {
		t.Errorf("warnings = %v, want [%q]", details.Warnings, want)
	}
}

// TestLinodeDomainDeleteDependencyWalkDegradesToAWarning: a preview without the
// record picture is still worth more to a caller deciding whether to proceed
// than no preview at all, so a failed list does not fail the walk.
func TestLinodeDomainDeleteDependencyWalkDegradesToAWarning(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"boom"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	details, err := toolhooks.LinodeDomainDeleteDependencyWalk(
		t.Context(), clientFor(t, server.URL), 5, nil,
	)
	if err != nil {
		t.Fatalf("LinodeDomainDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 0 {
		t.Errorf("dependencies = %v, want none from a failed list", details.Dependencies)
	}

	if len(details.Warnings) != 1 || !strings.HasPrefix(details.Warnings[0], "Could not list domain records:") {
		t.Errorf("warnings = %v, want the list failure reported as one", details.Warnings)
	}
}
