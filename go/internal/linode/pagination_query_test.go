package linode_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// pageQueryServer serves an empty paginated envelope and records the raw query
// string of the request it received, so a test can assert the page pair reached
// the wire rather than being dropped between the tool and the client.
func pageQueryServer(t *testing.T, wantPath string, gotQuery *string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, wantPath)
		}

		*gotQuery = r.URL.RawQuery

		w.Header().Set("Content-Type", tcApplicationJSON)

		body := map[string]any{"data": []any{}, "page": 1, "pages": 1, "results": 0}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
}

// TestPaginatedListsSendPageQuery covers every list client method that gained a
// page/page_size pair: the pair must reach the query string. A method that
// accepts the arguments and drops them would silently return page one forever,
// which is the failure the CLI cannot see.
func TestPaginatedListsSendPageQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		call func(client *linode.Client) error
	}{
		{
			name: "regions",
			path: "/regions",
			call: func(client *linode.Client) error {
				if _, err := client.ListRegionsProto(t.Context(), 2, 50); err != nil {
					return fmt.Errorf("list call: %w", err)
				}

				return nil
			},
		},
		{
			name: "object storage endpoints",
			path: "/object-storage/endpoints",
			call: func(client *linode.Client) error {
				if _, err := client.ListObjectStorageEndpointsProto(t.Context(), 2, 50); err != nil {
					return fmt.Errorf("list call: %w", err)
				}

				return nil
			},
		},
		{
			name: "instances",
			path: "/linode/instances",
			call: func(client *linode.Client) error {
				if _, err := client.ListInstancesProto(t.Context(), 2, 50); err != nil {
					return fmt.Errorf("list call: %w", err)
				}

				return nil
			},
		},
		{
			name: "images",
			path: "/images",
			call: func(client *linode.Client) error {
				if _, err := client.ListImagesProto(t.Context(), 2, 50); err != nil {
					return fmt.Errorf("list call: %w", err)
				}

				return nil
			},
		},
		{
			name: "instance disks",
			path: "/linode/instances/123/disks",
			call: func(client *linode.Client) error {
				if _, err := client.ListInstanceDisksProto(t.Context(), 123, 2, 50); err != nil {
					return fmt.Errorf("list call: %w", err)
				}

				return nil
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var gotQuery string

			srv := pageQueryServer(t, testCase.path, &gotQuery)
			defer srv.Close()

			client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

			if err := testCase.call(client); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if want := "page=2&page_size=50"; gotQuery != want {
				t.Errorf("query = %q, want %q", gotQuery, want)
			}
		})
	}
}

// TestPaginatedListsOmitUnsetPage checks the other half of the contract: with
// no page pair the query string stays empty, so the API's own default page
// applies and the request is byte-identical to the pre-pagination one.
func TestPaginatedListsOmitUnsetPage(t *testing.T) {
	t.Parallel()

	var gotQuery string

	srv := pageQueryServer(t, "/regions", &gotQuery)
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	if _, err := client.ListRegionsProto(t.Context(), 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}
