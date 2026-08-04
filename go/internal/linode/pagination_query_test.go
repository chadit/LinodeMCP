package linode_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// pageQueryServer serves an empty paginated envelope and records the request's
// raw query string, so a test can assert the page pair reached the wire rather
// than being dropped between the tool and the client.
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

// pageQueryProbe drops the decoded elements: these tests assert on the query
// string the method put on the wire, and pageQueryServer's envelope is empty.
func pageQueryProbe[T any](_ []T, err error) error {
	if err != nil {
		return fmt.Errorf("list call: %w", err)
	}

	return nil
}

// TestPaginatedListsSendPageQuery covers every list client method that gained a
// page/page_size pair. A method that accepts the arguments and drops them
// returns page one forever, a failure the CLI cannot see.
func TestPaginatedListsSendPageQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		call func(client *linode.Client) error
		name string
		path string
	}{
		{
			name: "regions",
			path: "/regions",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListRegionsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "object storage endpoints",
			path: "/object-storage/endpoints",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListObjectStorageEndpointsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "instances",
			path: "/linode/instances",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListInstancesProto(t.Context(), 2, 50))
			},
		},
		{
			name: "images",
			path: "/images",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListImagesProto(t.Context(), 2, 50))
			},
		},
		{
			name: "instance disks",
			path: "/linode/instances/123/disks",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListInstanceDisksProto(t.Context(), 123, 2, 50))
			},
		},
		{
			name: "domains",
			path: clientRoutePathDomains,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListDomainsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "domain records",
			path: "/domains/123/records",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListDomainRecordsProto(t.Context(), 123, 2, 50))
			},
		},
		{
			name: "firewalls",
			path: clientRoutePathNetworkingFirewalls,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListFirewallsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "nodebalancers",
			path: clientRoutePathNodebalancers,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListNodeBalancersProto(t.Context(), 2, 50))
			},
		},
		{
			name: "ssh keys",
			path: clientRoutePathProfileSshkeys,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListSSHKeysProto(t.Context(), 2, 50))
			},
		},
		{
			name: "stackscripts",
			path: clientRoutePathLinodeStackscripts,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListStackScriptsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "volumes",
			path: clientRoutePathVolumes,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListVolumesProto(t.Context(), 2, 50))
			},
		},
		{
			name: "vpcs",
			path: clientRoutePathVpcs,
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListVPCsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "vpc ip addresses across all vpcs",
			path: "/vpcs/ips",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListVPCIPsProto(t.Context(), 2, 50))
			},
		},
		{
			name: "vpc ip addresses",
			path: "/vpcs/123/ips",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListVPCIPAddressesProto(t.Context(), 123, 2, 50))
			},
		},
		{
			name: "vpc subnets",
			path: "/vpcs/123/subnets",
			call: func(client *linode.Client) error {
				return pageQueryProbe(client.ListVPCSubnetsProto(t.Context(), 123, 2, 50))
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
