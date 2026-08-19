package toolhooks_test

import (
	"context"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The hooks the sub-resource destroys own: the state each previews and plans
// over, and the two walks that name what a removal takes with it. The Python
// twin is tests/unit/test_toolhooks_subresource_destroy.py, and every sentence
// below is one both languages answer.

const (
	configListPath   = "/linode/instances/123/configs"
	nbConfigReadPath = "/nodebalancers/5/configs/6"
	nbNodeListPath   = nbConfigReadPath + "/nodes"
	deviceReadPath   = "/networking/firewalls/12345/devices/456"
	passwordInstance = "/linode/instances/5"
)

// TestLinodeInstanceDiskDeleteDependencyWalkNamesTheConfigsPointingAtIt: a
// deleted disk leaves an empty device slot behind in every profile that named
// it, and only in those.
func TestLinodeInstanceDiskDeleteDependencyWalkNamesTheConfigsPointingAtIt(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		configListPath: `{"data":[` +
			`{"id":6,"label":"boot-config","devices":{"sda":{"disk_id":10}}},` +
			`{"id":7,"label":"rescue","devices":{"sda":{"volume_id":3}}}],` +
			`"page":1,"pages":1,"results":2}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeInstanceDiskDeleteDependencyWalk(t.Context(), client, 123, 10, nil)
	if err != nil {
		t.Fatalf("LinodeInstanceDiskDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].ID != 6 {
		t.Fatalf("dependencies = %#v, want only the profile naming the disk", details.Dependencies)
	}

	if details.Dependencies[0].Action != tools.DependencyActionRemoved {
		t.Errorf("action = %q, want %q", details.Dependencies[0].Action, tools.DependencyActionRemoved)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "leaves those slots empty") {
		t.Errorf("warnings = %#v, want the empty-slot warning", details.Warnings)
	}

	quiet, err := toolhooks.LinodeInstanceDiskDeleteDependencyWalk(t.Context(), client, 123, 99, nil)
	if err != nil {
		t.Fatalf("LinodeInstanceDiskDeleteDependencyWalk: %v", err)
	}

	if len(quiet.Dependencies) != 0 || len(quiet.Warnings) != 0 {
		t.Errorf("details = %#v, want nothing for a disk no profile names", quiet)
	}
}

// TestLinodeInstanceDiskDeleteDependencyWalkWarnsOnAFailedConfigList: the
// preview is worth having without the profile picture, so a failed list is a
// warning rather than a refusal.
func TestLinodeInstanceDiskDeleteDependencyWalkWarnsOnAFailedConfigList(t *testing.T) {
	t.Parallel()

	client := linode.NewClient(routeStubServer(t, nil).URL, "token", configFor("http://unused"))

	details, err := toolhooks.LinodeInstanceDiskDeleteDependencyWalk(t.Context(), client, 123, 10, nil)
	if err != nil {
		t.Fatalf("LinodeInstanceDiskDeleteDependencyWalk: %v", err)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "Could not list instance configs") {
		t.Errorf("warnings = %#v, want the failed-list warning", details.Warnings)
	}
}

// TestLinodeNodebalancerConfigDeleteDependencyWalkNamesEveryBackend: the config
// owns its backend nodes, so each one leaves the rotation with it.
func TestLinodeNodebalancerConfigDeleteDependencyWalkNamesEveryBackend(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		nbNodeListPath: `{"data":[{"id":9,"label":"backend-1","address":"192.168.1.5:80","mode":"accept"}],` +
			`"page":1,"pages":1,"results":1}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeNodebalancerConfigDeleteDependencyWalk(t.Context(), client, 5, 6, nil)
	if err != nil {
		t.Fatalf("LinodeNodebalancerConfigDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].ID != 9 {
		t.Fatalf("dependencies = %#v, want the one backend node", details.Dependencies)
	}

	if want := "backend 192.168.1.5:80 (accept)"; details.Dependencies[0].Note != want {
		t.Errorf("note = %q, want %q", details.Dependencies[0].Note, want)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "removes 1 backend node(s)") {
		t.Errorf("warnings = %#v, want the rotation warning", details.Warnings)
	}
}

// TestLinodeNodebalancerConfigDeleteDependencyWalkSaysNothingWithoutBackends: a
// config with an empty rotation has nothing to report, and a failed list still
// degrades to a warning rather than refusing the preview.
func TestLinodeNodebalancerConfigDeleteDependencyWalkSaysNothingWithoutBackends(t *testing.T) {
	t.Parallel()

	server := routeStubServer(t, map[string]string{
		nbNodeListPath: `{"data":[],"page":1,"pages":1,"results":0}`,
	})
	client := linode.NewClient(server.URL, "token", configFor(server.URL))

	details, err := toolhooks.LinodeNodebalancerConfigDeleteDependencyWalk(t.Context(), client, 5, 6, nil)
	if err != nil {
		t.Fatalf("LinodeNodebalancerConfigDeleteDependencyWalk: %v", err)
	}

	if len(details.Dependencies) != 0 || len(details.Warnings) != 0 {
		t.Errorf("details = %#v, want nothing for an empty rotation", details)
	}

	failing := linode.NewClient(routeStubServer(t, nil).URL, "token", configFor("http://unused"))

	degraded, err := toolhooks.LinodeNodebalancerConfigDeleteDependencyWalk(t.Context(), failing, 5, 6, nil)
	if err != nil {
		t.Fatalf("LinodeNodebalancerConfigDeleteDependencyWalk: %v", err)
	}

	if len(degraded.Warnings) != 1 || !strings.Contains(degraded.Warnings[0], "Could not list config backend nodes") {
		t.Errorf("warnings = %#v, want the failed-list warning", degraded.Warnings)
	}
}

// TestLinodeInstancePasswordResetDependencyWalkNamesTheDowntime: the API cycles
// the Linode to apply the password, which no descriptor says, and a running
// instance loses service while it happens.
func TestLinodeInstancePasswordResetDependencyWalkNamesTheDowntime(t *testing.T) {
	t.Parallel()

	running := declaredState(t, `{"id":5,"status":"running"}`, &linodev1.Instance{})

	details, err := toolhooks.LinodeInstancePasswordResetDependencyWalk(t.Context(), nil, 5, running)
	if err != nil {
		t.Fatalf("LinodeInstancePasswordResetDependencyWalk: %v", err)
	}

	if len(details.SideEffects) != 1 || !strings.Contains(details.SideEffects[0], "powered down and rebooted") {
		t.Errorf("side effects = %#v, want the reboot", details.SideEffects)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "causing downtime") {
		t.Errorf("warnings = %#v, want the downtime warning", details.Warnings)
	}

	stopped, err := toolhooks.LinodeInstancePasswordResetDependencyWalk(
		t.Context(), nil, 5, declaredState(t, `{"id":5,"status":"offline"}`, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("LinodeInstancePasswordResetDependencyWalk: %v", err)
	}

	if len(stopped.SideEffects) != 1 || len(stopped.Warnings) != 0 {
		t.Errorf("details = %#v, want the reboot alone for a stopped instance", stopped)
	}
}

// TestLinodeInstancePasswordResetDependencyWalkStopsOnACanceledCall: the walk
// runs inside a preview a caller can abandon, and an abandoned one answers the
// cancellation rather than a half-built picture.
func TestLinodeInstancePasswordResetDependencyWalkStopsOnACanceledCall(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := toolhooks.LinodeInstancePasswordResetDependencyWalk(
		ctx, nil, 5, declaredState(t, `{"id":5}`, &linodev1.Instance{}),
	); err == nil {
		t.Error("a canceled walk answered no error")
	}
}

// TestLinodeInstancePasswordResetDependencyWalkReadsPastAStatuslessState: a
// read that named no status still gets the reboot, since the API cycles the
// Linode either way.
func TestLinodeInstancePasswordResetDependencyWalkReadsPastAStatuslessState(t *testing.T) {
	t.Parallel()

	details, err := toolhooks.LinodeInstancePasswordResetDependencyWalk(
		t.Context(), nil, 5, declaredState(t, `{"id":5}`, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("LinodeInstancePasswordResetDependencyWalk: %v", err)
	}

	if len(details.SideEffects) != 1 || len(details.Warnings) != 0 {
		t.Errorf("details = %#v, want the reboot alone", details)
	}
}
