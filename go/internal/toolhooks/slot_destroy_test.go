package toolhooks_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The bodies each hook below reads its state out of, one per family.
const (
	slotPlacementGroupJSON = `{"id":123,"label":"pg-rack","region":"us-east",` +
		`"members":[{"linode_id":11,"is_compliant":true},{"linode_id":22,"is_compliant":true}]}`
	slotEmptyPlacementGroupJSON = `{"id":123,"label":"pg-rack","region":"us-east","members":[]}`
	slotNodePoolJSON            = `{"id":7,"type":"g6-standard-2","count":2,` +
		`"nodes":[{"id":"node-a","instance_id":11,"status":"ready"},{"id":"node-b","instance_id":22,"status":"ready"}]}`
	slotEmptyNodePoolJSON = `{"id":7,"type":"g6-standard-2","count":0,"nodes":[]}`
	slotNodeJSON          = `{"id":"node-a","instance_id":11,"status":"ready"}`
	slotUnbackedNodeJSON  = `{"id":"node-a","instance_id":0,"status":"provisioning"}`
	slotTaggedPageJSON    = `{"data":[` +
		`{"type":"linode","data":{"id":11,"label":"web-01"}},` +
		`{"data":{"id":22}}` +
		`],"page":1,"pages":2,"results":9}`
	slotEmptyTaggedPageJSON = `{"data":[],"page":1,"pages":1,"results":0}`
	slotInstanceJSON        = `{"id":5,"label":"web-01","image":"linode/ubuntu24.04","status":"running"}`
	slotImagelessJSON       = `{"id":5,"label":"web-01","status":"running"}`
	slotDisksJSON           = `{"data":[{"id":1,"label":"boot","size":25600,"filesystem":"ext4"}],` +
		`"page":1,"pages":1,"results":1}`
)

// slotServer answers every request with one body and records the path asked for.
func slotServer(t *testing.T, body string, gotPath *string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
}

// slotFailingServer refuses every request, which is what a fetch hook has to
// report rather than answering with an empty resource.
func slotFailingServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
}

// canceled is a context no call can be made under, which is what each walk's
// first check answers.
func canceled(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	return ctx
}

// fetchCase is one hook's happy path: the route it addresses and the state it
// answers with.
type fetchCase struct {
	call func(context.Context, *linode.Client) (any, error)
	name string
	body string
	path string
}

// TestSlotDestroyFetchStatesReadTheirResource walks every fetch hook the
// multi-slot destroys declare: each addresses its own route and hands back what
// a preview reports and a plan hashes.
func TestSlotDestroyFetchStatesReadTheirResource(t *testing.T) {
	t.Parallel()

	for _, entry := range slotFetchCases() {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string

			server := slotServer(t, entry.body, &gotPath)
			defer server.Close()

			state, err := entry.call(t.Context(), clientFor(t, server.URL))
			if err != nil {
				t.Fatalf("%s: %v", entry.name, err)
			}

			if state == nil {
				t.Fatalf("%s answered no state", entry.name)
			}

			if !strings.HasSuffix(gotPath, entry.path) {
				t.Errorf("path = %q, want it to end with %q", gotPath, entry.path)
			}
		})
	}
}

// TestSlotDestroyFetchStatesHandBackNothingOnAFailure: a failed read is
// reported rather than answered with an empty resource, which a preview would
// otherwise print as the state of a resource nobody could read.
func TestSlotDestroyFetchStatesHandBackNothingOnAFailure(t *testing.T) {
	t.Parallel()

	for _, entry := range slotFetchCases() {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()

			server := slotFailingServer(t)
			defer server.Close()

			state, err := entry.call(t.Context(), clientFor(t, server.URL))
			if err == nil {
				t.Fatalf("%s answered %v, want the failure", entry.name, state)
			}

			if state != nil {
				t.Errorf("state = %v, want nothing beside the failure", state)
			}
		})
	}
}

// slotFetchCases lists every fetch hook this file's tools declare.
func slotFetchCases() []fetchCase {
	return []fetchCase{
		{
			name: "tag_delete",
			body: slotTaggedPageJSON,
			path: "/tags/prod",
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return toolhooks.LinodeTagDeleteFetchState(ctx, client, "prod")
			},
		},
	}
}

// walkCase is one walk's cancellation check, which every state-only walk
// answers before it reads anything.
type walkCase struct {
	call func(context.Context, *linode.Client) (tools.DryRunDetails, error)
	name string
}

// TestSlotDestroyWalksRefuseACanceledContext: a walk that read past a canceled
// context would spend the caller's remaining budget assembling a preview
// nobody is waiting for.
func TestSlotDestroyWalksRefuseACanceledContext(t *testing.T) {
	t.Parallel()

	cases := []walkCase{
		{
			name: "placement_group_delete",
			call: func(ctx context.Context, client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodePlacementGroupDeleteDependencyWalk(
					ctx, client, 123, declaredState(t, slotPlacementGroupJSON, &linodev1.PlacementGroup{}))
			},
		},
		{
			name: "lke_pool_delete",
			call: func(ctx context.Context, client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodeLkePoolDeleteDependencyWalk(
					ctx, client, 123, 7, declaredState(t, slotNodePoolJSON, &linodev1.LKENodePool{}))
			},
		},
		{
			name: "lke_node_delete",
			call: func(ctx context.Context, client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodeLkeNodeDeleteDependencyWalk(
					ctx, client, 123, "node-a", declaredState(t, slotNodeJSON, &linodev1.LKENode{}))
			},
		},
		{
			name: "tag_delete",
			call: func(ctx context.Context, client *linode.Client) (tools.DryRunDetails, error) {
				return toolhooks.LinodeTagDeleteDependencyWalk(ctx, client, "prod", nil)
			},
		},
	}

	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()

			if _, err := entry.call(canceled(t), nil); err == nil {
				t.Errorf("%s walked under a canceled context", entry.name)
			}
		})
	}
}

// TestLinodeTagDeleteDependencyWalkIgnoresAStateItCannotRead: a preview whose
// state came back as something else reports the route alone rather than
// guessing at dependencies it never saw.
func TestLinodeTagDeleteDependencyWalkIgnoresAStateItCannotRead(t *testing.T) {
	t.Parallel()

	details, err := toolhooks.LinodeTagDeleteDependencyWalk(t.Context(), nil, "prod", nil)
	if err != nil || len(details.Dependencies) != 0 {
		t.Errorf("tag walk = %+v, %v, want nothing", details, err)
	}
}

// TestLinodePlacementGroupDeleteDependencyWalkDetachesEveryMember: the
// instances survive the group, so each is reported as detached and the warning
// counts them.
func TestLinodePlacementGroupDeleteDependencyWalkDetachesEveryMember(t *testing.T) {
	t.Parallel()

	state := declaredState(t, slotPlacementGroupJSON, &linodev1.PlacementGroup{})

	details, err := toolhooks.LinodePlacementGroupDeleteDependencyWalk(t.Context(), nil, 123, state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 2 {
		t.Fatalf("dependencies = %d, want 2", len(details.Dependencies))
	}

	if details.Dependencies[0].Action != tools.DependencyActionDetached {
		t.Errorf("action = %q, want %q", details.Dependencies[0].Action, tools.DependencyActionDetached)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "detaches 2 Linode(s)") {
		t.Errorf("warnings = %v, want the member count", details.Warnings)
	}
}

// TestLinodePlacementGroupDeleteDependencyWalkSaysNothingForAnEmptyGroup: a
// group with no members detaches nothing, so there is no warning to give.
func TestLinodePlacementGroupDeleteDependencyWalkSaysNothingForAnEmptyGroup(t *testing.T) {
	t.Parallel()

	state := declaredState(t, slotEmptyPlacementGroupJSON, &linodev1.PlacementGroup{})

	details, err := toolhooks.LinodePlacementGroupDeleteDependencyWalk(t.Context(), nil, 123, state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 0 || len(details.Warnings) != 0 {
		t.Errorf("details = %+v, want nothing", details)
	}
}

// TestLinodeLkePoolDeleteDependencyWalkNamesEveryBackingLinode: the pool's
// nodes carry the Linodes the delete destroys, and the warning counts them.
func TestLinodeLkePoolDeleteDependencyWalkNamesEveryBackingLinode(t *testing.T) {
	t.Parallel()

	state := declaredState(t, slotNodePoolJSON, &linodev1.LKENodePool{})

	details, err := toolhooks.LinodeLkePoolDeleteDependencyWalk(t.Context(), nil, 123, 7, state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 2 {
		t.Fatalf("dependencies = %d, want 2", len(details.Dependencies))
	}

	if details.Dependencies[0].Label != "node-a" {
		t.Errorf("label = %v, want node-a", details.Dependencies[0].Label)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "destroys 2 node(s)") {
		t.Errorf("warnings = %v, want the node count", details.Warnings)
	}
}

// TestLinodeLkePoolDeleteDependencyWalkSkipsTheCountWarningForAnEmptyPool: a
// pool holding no nodes destroys no workload, so only the count sentence goes.
func TestLinodeLkePoolDeleteDependencyWalkSkipsTheCountWarningForAnEmptyPool(t *testing.T) {
	t.Parallel()

	state := declaredState(t, slotEmptyNodePoolJSON, &linodev1.LKENodePool{})

	details, err := toolhooks.LinodeLkePoolDeleteDependencyWalk(t.Context(), nil, 123, 7, state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", details.Warnings)
	}
}

// TestLinodeLkeNodeDeleteDependencyWalkNamesTheBackingLinode: the node carries
// the Linode that goes with it, and the pool-level effect is said either way.
func TestLinodeLkeNodeDeleteDependencyWalkNamesTheBackingLinode(t *testing.T) {
	t.Parallel()

	state := declaredState(t, slotNodeJSON, &linodev1.LKENode{})

	details, err := toolhooks.LinodeLkeNodeDeleteDependencyWalk(t.Context(), nil, 123, "node-a", state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %d, want 1", len(details.Dependencies))
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], "pool node count") {
		t.Errorf("warnings = %v, want the pool effect", details.Warnings)
	}
}

// TestLinodeLkeNodeDeleteDependencyWalkStillReportsAnUnbackedNode: a node with
// no Linode yet has nothing to cascade, and the pool effect still holds.
func TestLinodeLkeNodeDeleteDependencyWalkStillReportsAnUnbackedNode(t *testing.T) {
	t.Parallel()

	state := declaredState(t, slotUnbackedNodeJSON, &linodev1.LKENode{})

	details, err := toolhooks.LinodeLkeNodeDeleteDependencyWalk(t.Context(), nil, 123, "node-a", state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 0 {
		t.Errorf("dependencies = %+v, want none", details.Dependencies)
	}

	if len(details.Warnings) != 1 {
		t.Errorf("warnings = %v, want the pool effect", details.Warnings)
	}
}

// TestLinodeTagDeleteDependencyWalkCountsBeyondTheFirstPage: the itemized list
// is the page, and the count is the envelope's total, so a caller hears the
// real blast radius plus how much of it the preview could name.
func TestLinodeTagDeleteDependencyWalkCountsBeyondTheFirstPage(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := slotServer(t, slotTaggedPageJSON, &gotPath)
	defer server.Close()

	client := clientFor(t, server.URL)

	state, err := toolhooks.LinodeTagDeleteFetchState(t.Context(), client, "prod")
	if err != nil {
		t.Fatalf("fetch state: %v", err)
	}

	details, err := toolhooks.LinodeTagDeleteDependencyWalk(t.Context(), client, "prod", state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 2 {
		t.Fatalf("dependencies = %d, want 2", len(details.Dependencies))
	}

	// The second object declares no type, so it falls back to the generic kind.
	if details.Dependencies[0].Kind != "linode" || details.Dependencies[1].Kind != "resource" {
		t.Errorf("kinds = %q and %q, want linode and resource",
			details.Dependencies[0].Kind, details.Dependencies[1].Kind)
	}

	if len(details.Warnings) != 2 {
		t.Fatalf("warnings = %v, want the total and the itemized count", details.Warnings)
	}

	if !strings.Contains(details.Warnings[0], "from 9 tagged object(s)") {
		t.Errorf("first warning = %q, want the envelope total", details.Warnings[0])
	}

	if !strings.Contains(details.Warnings[1], "first 2 tagged object(s)") {
		t.Errorf("second warning = %q, want the itemized count", details.Warnings[1])
	}
}

// TestLinodeTagDeleteDependencyWalkSaysNothingForAnUntaggedLabel: a tag on
// nothing removes nothing, so there is no dependency and no warning.
func TestLinodeTagDeleteDependencyWalkSaysNothingForAnUntaggedLabel(t *testing.T) {
	t.Parallel()

	var gotPath string

	server := slotServer(t, slotEmptyTaggedPageJSON, &gotPath)
	defer server.Close()

	client := clientFor(t, server.URL)

	state, err := toolhooks.LinodeTagDeleteFetchState(t.Context(), client, "prod")
	if err != nil {
		t.Fatalf("fetch state: %v", err)
	}

	details, err := toolhooks.LinodeTagDeleteDependencyWalk(t.Context(), client, "prod", state)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Dependencies) != 0 || len(details.Warnings) != 0 {
		t.Errorf("details = %+v, want nothing", details)
	}
}

// TestLinodeInstanceRebuildDependencyWalkNamesEveryDiskAndTheImage: a rebuild
// erases the disks and replaces the image, and the caller hears both.
func TestLinodeInstanceRebuildDependencyWalkNamesEveryDiskAndTheImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := slotInstanceJSON
		if strings.HasSuffix(r.URL.Path, "/disks") {
			body = slotDisksJSON
		}

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := clientFor(t, server.URL)

	details, err := toolhooks.LinodeInstanceRebuildDependencyWalk(
		t.Context(), client, 5, declaredState(t, slotInstanceJSON, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.SideEffects) != 1 || !strings.Contains(details.SideEffects[0], `Disk "boot" (25600 MB, ext4)`) {
		t.Errorf("side effects = %v, want the disk", details.SideEffects)
	}

	if len(details.Warnings) != 1 || !strings.Contains(details.Warnings[0], `"linode/ubuntu24.04"`) {
		t.Errorf("warnings = %v, want the current image", details.Warnings)
	}
}

// TestLinodeInstanceRebuildDependencyWalkFallsBackWithoutAnImage: an instance
// carrying no image still gets the data-loss warning, minus the name.
func TestLinodeInstanceRebuildDependencyWalkFallsBackWithoutAnImage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := slotImagelessJSON
		if strings.HasSuffix(r.URL.Path, "/disks") {
			body = slotDisksJSON
		}

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := clientFor(t, server.URL)

	details, err := toolhooks.LinodeInstanceRebuildDependencyWalk(
		t.Context(), client, 5, declaredState(t, slotImagelessJSON, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.Warnings) != 1 || strings.Contains(details.Warnings[0], "replaces") {
		t.Errorf("warnings = %v, want the nameless form", details.Warnings)
	}
}

// TestLinodeInstanceRebuildDependencyWalkWarnsOnAFailedDiskList: the preview is
// still worth having without the disk picture, so the failure is a warning
// rather than a refusal.
func TestLinodeInstanceRebuildDependencyWalkWarnsOnAFailedDiskList(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/disks") {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		if _, err := w.Write([]byte(slotInstanceJSON)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := clientFor(t, server.URL)

	details, err := toolhooks.LinodeInstanceRebuildDependencyWalk(
		t.Context(), client, 5, declaredState(t, slotInstanceJSON, &linodev1.Instance{}),
	)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(details.SideEffects) != 0 {
		t.Errorf("side effects = %v, want none", details.SideEffects)
	}

	if len(details.Warnings) != 2 || !strings.Contains(details.Warnings[0], "Could not list instance disks") {
		t.Errorf("warnings = %v, want the failed list first", details.Warnings)
	}
}
