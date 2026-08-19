package toolhooks_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	// lkePoolsJSON is a two-pool cluster, one pool per node type.
	lkePoolsJSON = `{"data":[` +
		`{"id":7,"type":"g6-standard-2","count":3},` +
		`{"id":8,"type":"g6-standard-4","count":2}` +
		`],"page":1,"pages":1,"results":2}`
	// lkeEmptyPoolsJSON is a cluster whose only pool holds no nodes, which is
	// what leaves the node-count warning off a preview that still lists a pool.
	lkeEmptyPoolsJSON = `{"data":[{"id":9,"type":"g6-standard-1","count":0}],"page":1,"pages":1,"results":1}`
)

// TestLinodeLkeClusterDeleteDependencyWalkCountsTheNodesItDestroys: the pools
// cascade and the nodes under them carry the running workloads, which is the
// part of the delete a caller cannot put back.
func TestLinodeLkeClusterDeleteDependencyWalkCountsTheNodesItDestroys(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(lkePoolsJSON)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	details, err := toolhooks.LinodeLkeClusterDeleteDependencyWalk(
		t.Context(), clientFor(t, server.URL), 123, declaredState(t, `{"id":123}`, &linodev1.LKECluster{}),
	)
	if err != nil {
		t.Fatalf("LinodeLkeClusterDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 2 {
		t.Fatalf("dependencies = %v, want one per node pool", details.Dependencies)
	}

	first := details.Dependencies[0]
	if first.Kind != "node_pool" || first.ID != 7 || first.Note != "3 node(s) of type g6-standard-2" {
		t.Errorf("dependency = %+v, want the first pool and what it holds", first)
	}

	want := "Deleting this cluster destroys 2 node pool(s) and 5 node(s); running workloads are lost."
	if len(details.Warnings) != 1 || details.Warnings[0] != want {
		t.Errorf("warnings = %v, want [%q]", details.Warnings, want)
	}
}

// TestLinodeLkeClusterDeleteDependencyWalkLeavesOutAnEmptyNodeCount: a pool
// with no nodes still cascades, so it is listed, but there is no workload to
// warn about and the sentence that names one stays off the preview.
func TestLinodeLkeClusterDeleteDependencyWalkLeavesOutAnEmptyNodeCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte(lkeEmptyPoolsJSON)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	details, err := toolhooks.LinodeLkeClusterDeleteDependencyWalk(
		t.Context(), clientFor(t, server.URL), 123, declaredState(t, `{"id":123}`, &linodev1.LKECluster{}),
	)
	if err != nil {
		t.Fatalf("LinodeLkeClusterDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %v, want the one empty pool", details.Dependencies)
	}

	if len(details.Warnings) != 0 {
		t.Errorf("warnings = %v, want none over a cluster holding no nodes", details.Warnings)
	}
}

// TestLinodeLkeClusterDeleteDependencyWalkDegradesToAWarning: a preview without
// the pool picture is still worth more to a caller deciding whether to proceed
// than no preview at all, so a failed list does not fail the walk.
func TestLinodeLkeClusterDeleteDependencyWalkDegradesToAWarning(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"boom"}]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	details, err := toolhooks.LinodeLkeClusterDeleteDependencyWalk(
		t.Context(), clientFor(t, server.URL), 123, declaredState(t, `{"id":123}`, &linodev1.LKECluster{}),
	)
	if err != nil {
		t.Fatalf("LinodeLkeClusterDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 0 {
		t.Errorf("dependencies = %v, want none from a failed list", details.Dependencies)
	}

	if len(details.Warnings) != 1 || !strings.HasPrefix(details.Warnings[0], "Could not list node pools:") {
		t.Errorf("warnings = %v, want the list failure reported as one", details.Warnings)
	}
}
