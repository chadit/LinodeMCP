package toolhooks_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The routes each fetch_state and walk reads, kept beside the tools they serve.
const (
	firewallPath     = "/networking/firewalls/77"
	vpcDeletePath    = "/vpcs/77"
	nodebalancerPath = "/nodebalancers/77"
	objectKeyPath    = "/object-storage/keys/77"
	mysqlPath        = "/databases/mysql/instances/77"
	postgresPath     = "/databases/postgresql/instances/77"
)

// The family names each case is keyed by, shared with the instance destroy
// tests beside them so one spelling serves the whole package.
const (
	nameFirewall     = "firewall"
	nameVPC          = "vpc"
	nameNodebalancer = "nodebalancer"
)

// TestLinodeFirewallDeleteDependencyWalkNamesTheDevices: the resources a
// firewall guards survive the delete but stop being protected, which is the
// part of the change the request itself does not show.
func TestLinodeFirewallDeleteDependencyWalkNamesTheDevices(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		firewallPath + "/devices": `{"data":[{"id":5,"entity":{"id":9,"type":"linode","label":"web-01"}}],"page":1,"pages":1,"results":1}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeFirewallDeleteDependencyWalk(t.Context(), client, 77, nil)
	if err != nil {
		t.Fatalf("LinodeFirewallDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %v, want one", details.Dependencies)
	}

	device := details.Dependencies[0]
	if device.Kind != "linode" || device.ID != 9 || device.Label != "web-01" {
		t.Errorf("dependency = %#v, want the linode 9", device)
	}

	if device.Action != tools.DependencyActionRemoved {
		t.Errorf("action = %q, want %q", device.Action, tools.DependencyActionRemoved)
	}

	if len(details.Warnings) != 1 {
		t.Errorf("warnings = %v, want one", details.Warnings)
	}
}

// TestLinodeVPCDeleteDependencyWalkCountsDetachedInterfaces: the subnets go with
// the VPC and every Linode interface in them is detached, so the count is what
// a caller weighs the delete against.
func TestLinodeVPCDeleteDependencyWalkCountsDetachedInterfaces(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		vpcDeletePath + "/subnets": `{"data":[{"id":3,"label":"app","linodes":[{"id":1},{"id":2}]}],"page":1,"pages":1,"results":1}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeVPCDeleteDependencyWalk(t.Context(), client, 77, nil)
	if err != nil {
		t.Fatalf("LinodeVPCDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 ||
		details.Dependencies[0].Action != tools.DependencyActionCascadeDeleted {
		t.Fatalf("dependencies = %#v, want one cascade_deleted subnet", details.Dependencies)
	}

	if details.Dependencies[0].Note != "2 attached Linode interface(s)" {
		t.Errorf("note = %q, want the interface count", details.Dependencies[0].Note)
	}

	if len(details.Warnings) != 1 {
		t.Errorf("warnings = %v, want one", details.Warnings)
	}
}

// TestLinodeVPCDeleteDependencyWalkStaysQuietWithoutInterfaces: an empty subnet
// is still reported as a cascade, but nothing is detached, so no warning is
// added over it.
func TestLinodeVPCDeleteDependencyWalkStaysQuietWithoutInterfaces(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		vpcDeletePath + "/subnets": `{"data":[{"id":3,"label":"app","linodes":[]}],"page":1,"pages":1,"results":1}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeVPCDeleteDependencyWalk(t.Context(), client, 77, nil)
	if err != nil {
		t.Fatalf("LinodeVPCDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 {
		t.Errorf("dependencies = %v, want one", details.Dependencies)
	}

	if len(details.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", details.Warnings)
	}
}

// TestLinodeNodebalancerDeleteDependencyWalkNamesTheConfigs: each config takes
// its backend node list with it, so the configs are the cascade worth naming.
func TestLinodeNodebalancerDeleteDependencyWalkNamesTheConfigs(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		nodebalancerPath + "/configs": `{"data":[{"id":4,"protocol":"https","port":443}],"page":1,"pages":1,"results":1}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeNodebalancerDeleteDependencyWalk(t.Context(), client, 77, nil)
	if err != nil {
		t.Fatalf("LinodeNodebalancerDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 ||
		details.Dependencies[0].Action != tools.DependencyActionCascadeDeleted {
		t.Fatalf("dependencies = %#v, want one cascade_deleted config", details.Dependencies)
	}

	if details.Dependencies[0].Note != "https config on port 443" {
		t.Errorf("note = %q, want the protocol and port", details.Dependencies[0].Note)
	}

	if len(details.Warnings) != 1 {
		t.Errorf("warnings = %v, want one", details.Warnings)
	}
}

// TestWaveDestroyDependencyWalksDegradeToAWarning: each walk is best-effort, so
// a fetch it cannot make leaves a warning and a previewable answer rather than
// refusing the preview outright.
func TestWaveDestroyDependencyWalksDegradeToAWarning(t *testing.T) {
	t.Parallel()

	walks := map[string]struct {
		walk func(client *linode.Client) (tools.DryRunDetails, error)
		want string
	}{
		nameFirewall: {
			walk: func(client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodeFirewallDeleteDependencyWalk(t.Context(), client, 77, nil)
			},
			want: "Could not list firewall devices:",
		},
		nameVPC: {
			walk: func(client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodeVPCDeleteDependencyWalk(t.Context(), client, 77, nil)
			},
			want: "Could not list VPC subnets:",
		},
		nameNodebalancer: {
			walk: func(client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodeNodebalancerDeleteDependencyWalk(t.Context(), client, 77, nil)
			},
			want: "Could not list NodeBalancer configs:",
		},
	}

	for name, testCase := range walks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := routeStubServer(t, nil)
			client := linode.NewClient(server.URL, "token", configFor(server.URL))

			details, err := testCase.walk(client)
			if err != nil {
				t.Fatalf("walk: %v", err)
			}

			if len(details.Dependencies) != 0 {
				t.Errorf("dependencies = %v, want none", details.Dependencies)
			}

			if len(details.Warnings) != 1 ||
				!hasPrefix(details.Warnings[0], testCase.want) {
				t.Fatalf("warnings = %v, want one starting %q", details.Warnings, testCase.want)
			}
		})
	}
}

// hasPrefix keeps the warning assertions readable without pulling strings in for
// one call.
func hasPrefix(value, prefix string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}
