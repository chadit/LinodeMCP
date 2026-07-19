package observability_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// TestDurationBucketsMatchSharedFixture pins this language's histogram
// bucket boundaries to testdata/observability/duration_buckets.json, the
// fixture every language asserts against. The exported Prometheus _bucket
// series are derived from these values, so a one-sided edit here would
// silently fork the metrics contract; this test makes it fail instead.
func TestDurationBucketsMatchSharedFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "testdata", "observability", "duration_buckets.json",
	))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}

	var fixture map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse shared fixture: %v", err)
	}

	checks := []struct {
		instrument string
		got        []float64
	}{
		{"linodemcp.request.duration.seconds", observability.RequestDurationBoundaries()},
		{"linodemcp.api.request.duration.seconds", observability.APIRequestDurationBoundaries()},
	}

	for _, check := range checks {
		wantRaw, ok := fixture[check.instrument].([]any)
		if !ok {
			t.Fatalf("fixture missing boundaries for %s", check.instrument)
		}

		if len(wantRaw) != len(check.got) {
			t.Fatalf("%s: fixture has %d boundaries, code declares %d",
				check.instrument, len(wantRaw), len(check.got))
		}

		for index, entry := range wantRaw {
			want, ok := entry.(float64)
			if !ok {
				t.Fatalf("%s: fixture boundary %d is not a number", check.instrument, index)
			}

			if check.got[index] != want {
				t.Errorf("%s boundary %d: code declares %v, fixture pins %v",
					check.instrument, index, check.got[index], want)
			}
		}
	}
}
