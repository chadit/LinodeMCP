package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const objectListPath = "/object-storage/buckets/us-east/reports/object-list"

// TestClientListObjectStorageBucketContentsProtoSuccess covers the S3 envelope
// this endpoint returns instead of the usual {data,page,pages,results} one: the
// truncation flag and the next marker ride alongside data[], and both have to
// survive onto the page struct or a caller can never walk past the first slice
// of a large bucket.
func TestClientListObjectStorageBucketContentsProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != objectListPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, objectListPath)
		}

		if r.URL.Query().Get("marker") != "page-1" {
			t.Errorf("marker = %v, want %v", r.URL.Query().Get("marker"), "page-1")
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{
				{"name": "june.csv", "size": 2048, "etag": "abc123"},
			},
			"is_truncated": true,
			"next_marker":  "page-2",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	page, err := client.ListObjectStorageBucketContentsProto(
		t.Context(), regionUSEast, "reports", map[string]string{"marker": "page-1"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if page == nil {
		t.Fatal("page is nil")
	}

	if len(page.Objects) != 1 {
		t.Fatalf("len(page.Objects) = %v, want %v", len(page.Objects), 1)
	}

	if page.Objects[0].GetName() != "june.csv" {
		t.Errorf("page.Objects[0].GetName() = %v, want %v", page.Objects[0].GetName(), "june.csv")
	}

	if page.Objects[0].GetSize() != int64(2048) {
		t.Errorf("page.Objects[0].GetSize() = %v, want %v", page.Objects[0].GetSize(), int64(2048))
	}

	if !page.IsTruncated {
		t.Error("page.IsTruncated = false, want true")
	}

	if page.NextMarker != "page-2" {
		t.Errorf("page.NextMarker = %v, want %v", page.NextMarker, "page-2")
	}
}

// TestClientListObjectStorageBucketContentsProtoError proves an API refusal
// surfaces as an error rather than an empty, non-truncated page, which a caller
// would read as "the bucket is empty".
func TestClientListObjectStorageBucketContentsProtoError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errNotFound}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	page, err := client.ListObjectStorageBucketContentsProto(t.Context(), regionUSEast, "reports", nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if page != nil {
		t.Errorf("page = %+v, want nil", page)
	}
}
