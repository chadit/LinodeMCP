package toolhooks_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The routes each cross-family destroy hook reads, kept beside the tools they
// serve.
const (
	crossSubnetPath      = "/vpcs/77/subnets/8"
	crossVPCPath         = "/vpcs/77"
	crossVLANListPath    = "/networking/vlans"
	crossSampleRegion    = "us-east"
	crossSampleVLANLabel = "vl-app"
	crossSubnetLabel     = "subnet-a"
)

// escapedRouteStubServer answers a stubbed route with one body, matching on the
// escaped path so a segment carrying a slash is looked up as the live call
// sends it rather than as net/http decodes it back.
func escapedRouteStubServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, stubbed := routes[r.URL.EscapedPath()]
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
}

// crossFetchCase is one destroy's state read: the route it must ask for, the
// body waiting there, and the call that goes and gets it.
type crossFetchCase struct {
	read  func(client *linode.Client) (any, error)
	name  string
	route string
	body  string
}

// crossFetchCases enumerates every cross-family destroy hook once, so the
// success and failure tests below stay one list rather than two.
func crossFetchCases(t *testing.T) []crossFetchCase {
	t.Helper()

	return []crossFetchCase{
		{
			name:  configPurposeVlan,
			route: crossVLANListPath,
			body:  `{"data":[{"label":"vl-app","region":"us-east","linodes":[11]}],"page":1,"pages":1,"results":1}`,
			read: func(client *linode.Client) (any, error) {
				return toolhooks.LinodeVlanDeleteFetchState(
					t.Context(), client, crossSampleRegion, crossSampleVLANLabel,
				)
			},
		},
	}
}

// TestCrossDestroyFetchStateReadsItsResource: every destroy previews and plans
// over the resource its own family reads, so each hook goes through that typed
// client method rather than a generic decode.
func TestCrossDestroyFetchStateReadsItsResource(t *testing.T) {
	t.Parallel()

	for _, testCase := range crossFetchCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := escapedRouteStubServer(t, map[string]string{testCase.route: testCase.body})
			client := linode.NewClient(server.URL, "token", configFor(server.URL))

			state, err := testCase.read(client)
			if err != nil {
				t.Fatalf("fetch state: %v", err)
			}

			if state == nil {
				t.Fatal("state is nil, want the resource")
			}
		})
	}
}

// TestCrossDestroyFetchStateReportsAFailedRead: a read that fails is an error
// rather than a nil state, so the preview says so instead of reporting an empty
// resource over a delete that has not happened yet.
func TestCrossDestroyFetchStateReportsAFailedRead(t *testing.T) {
	t.Parallel()

	for _, testCase := range crossFetchCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := escapedRouteStubServer(t, nil)
			client := linode.NewClient(server.URL, "token", configFor(server.URL))

			state, err := testCase.read(client)
			if err == nil {
				t.Fatalf("state = %v, want an error", state)
			}
		})
	}
}

// TestVlanDeleteFetchStateReportsAMissingVLAN: a VLAN the list does not carry
// is reported as not found rather than as an empty resource, since the preview
// would otherwise describe a delete against something that is not there.
func TestVlanDeleteFetchStateReportsAMissingVLAN(t *testing.T) {
	t.Parallel()

	server := escapedRouteStubServer(t, map[string]string{
		crossVLANListPath: `{"data":[{"label":"other","region":"us-east","linodes":[]}],` +
			`"page":1,"pages":1,"results":1}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	_, err := toolhooks.LinodeVlanDeleteFetchState(t.Context(), client, crossSampleRegion, crossSampleVLANLabel)
	if !errors.Is(err, tools.ErrVLANNotFound) {
		t.Fatalf("err = %v, want %v", err, tools.ErrVLANNotFound)
	}
}

// TestVPCSubnetDeleteDependencyWalkNamesDetachedLinodes: the fetched subnet
// carries the Linodes with interfaces in it, each detached rather than deleted,
// and the warning names the VPC the subnet belonged to.
func TestVPCSubnetDeleteDependencyWalkNamesDetachedLinodes(t *testing.T) {
	t.Parallel()

	server := escapedRouteStubServer(t, map[string]string{crossVPCPath: `{"id":77,"label":"prod-vpc"}`})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	state := declaredState(t,
		`{"id":8,"label":"subnet-a","linodes":[{"id":11,"interfaces":[{"id":1,"active":true}]}]}`,
		&linodev1.VpcSubnet{})

	details, err := toolhooks.LinodeVPCSubnetDeleteDependencyWalk(t.Context(), client, 77, 8, state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %d, want 1", len(details.Dependencies))
	}

	if details.Dependencies[0].Action != tools.DependencyActionDetached {
		t.Errorf("action = %q, want %q", details.Dependencies[0].Action, tools.DependencyActionDetached)
	}

	if details.Dependencies[0].Note != "1 interface(s) in this subnet are detached." {
		t.Errorf("note = %q", details.Dependencies[0].Note)
	}

	want := `1 Linode(s) have interfaces in subnet "subnet-a" (VPC "prod-vpc") and will be detached.`
	if len(details.Warnings) != 1 || details.Warnings[0] != want {
		t.Errorf("warnings = %v, want [%q]", details.Warnings, want)
	}
}

// TestVPCSubnetDeleteDependencyWalkLeavesTheVPCLabelEmpty: the VPC read is
// best-effort, so a failed one still reports the detached Linodes.
func TestVPCSubnetDeleteDependencyWalkLeavesTheVPCLabelEmpty(t *testing.T) {
	t.Parallel()

	server := escapedRouteStubServer(t, nil)
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	state := declaredState(t, `{"id":8,"label":"subnet-a","linodes":[{"id":11}]}`, &linodev1.VpcSubnet{})

	details, err := toolhooks.LinodeVPCSubnetDeleteDependencyWalk(t.Context(), client, 77, 8, state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	want := `1 Linode(s) have interfaces in subnet "subnet-a" (VPC "") and will be detached.`
	if len(details.Warnings) != 1 || details.Warnings[0] != want {
		t.Errorf("warnings = %v, want [%q]", details.Warnings, want)
	}
}

// TestVPCSubnetDeleteDependencyWalkSaysNothingWithoutLinodes: a subnet nothing
// is attached to takes nothing with it, so the walk reports no dependency and
// makes no VPC read to label a warning it would not write.
func TestVPCSubnetDeleteDependencyWalkSaysNothingWithoutLinodes(t *testing.T) {
	t.Parallel()

	var asked bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		asked = true

		if _, err := w.Write([]byte(`{"id":77,"label":"prod-vpc"}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeVPCSubnetDeleteDependencyWalk(
		t.Context(), client, 77, 8, declaredState(t, `{"id":8,"label":"subnet-a"}`, &linodev1.VpcSubnet{}),
	)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 0 || len(details.Warnings) != 0 {
		t.Errorf("details = %+v, want empty", details)
	}

	if asked {
		t.Error("the VPC was read for a warning the walk does not write")
	}
}

// TestVPCSubnetDeleteDependencyWalkReadsASubnetMissingItsMembers: a read that
// answered without the member reports no dependency, which is what an
// unattached subnet means. A state no declared fetch produced never reaches
// here: the generated call refuses it first.
func TestVPCSubnetDeleteDependencyWalkReadsASubnetMissingItsMembers(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "token", configFor("http://127.0.0.1:1"))

	details, err := toolhooks.LinodeVPCSubnetDeleteDependencyWalk(
		t.Context(), client, 77, 8, declaredState(t, `{"id":8}`, &linodev1.VpcSubnet{}),
	)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 0 || len(details.Warnings) != 0 {
		t.Errorf("details = %+v, want empty", details)
	}
}
