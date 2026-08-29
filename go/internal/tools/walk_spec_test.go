package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The declared walk engine, held to the behaviors the hand bodies it replaces
// pinned: per-element emission, the envelope total, predicate warnings, the
// drop rule, and a failed list becoming its declared warning.

// The literals only this table spells.
const (
	walkIPListTool   = "linode_instance_ip_list"
	walkUnknownPrice = "unknown"
	walkRecordNS     = "NS"
	walkImageFixture = "linode/ubuntu24.04"
	keyWalkDevices   = "devices"
	keyWalkEntity    = "entity"
	walkKindResource = "resource"
	walkMemberPools  = "pools"
	walkEnrichTool   = "linode_instance_get"
)

func runWalks(
	t *testing.T, specs []tools.WalkSpec, state tools.DeclaredState,
) tools.DryRunDetails {
	t.Helper()

	details, err := tools.RunDependencyWalks(t.Context(), nil, specs, state, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	return details
}

func TestWalkEngineEmitsStateMembersWithWarnings(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{
		managedServiceLabelParam: "pg-app",
		keyPlacementGroupMembers: []any{
			tools.DeclaredState{keyManagedLinodeSettingsLinodeID: 123},
			tools.DeclaredState{keyManagedLinodeSettingsLinodeID: 456},
		},
	}

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyPlacementGroupMembers,
		Emit: tools.WalkSpecEmit{
			Kind: tcInstance, IDField: keyManagedLinodeSettingsLinodeID, Action: tools.DependencyActionDetached,
			Note: "Linode is removed from the placement group; the instance is not deleted.",
		},
		Warnings: []tools.WalkSpecWarning{
			{
				Template: "Deleting this placement group detaches {count} Linode(s); the instances are not deleted.",
				WhenAny:  true,
			},
		},
	}}, state)

	if len(details.Dependencies) != 2 || details.Dependencies[0].Kind != tcInstance {
		t.Fatalf("dependencies = %+v, want the two detached members", details.Dependencies)
	}

	if len(details.Warnings) != 1 || details.Warnings[0] != "Deleting this placement group detaches 2 Linode(s); the instances are not deleted." {
		t.Errorf("warnings = %v, want the counted sentence", details.Warnings)
	}
}

func TestWalkEngineSaysNothingOverAnEmptyMember(t *testing.T) {
	t.Parallel()

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyPlacementGroupMembers,
		Emit:        tools.WalkSpecEmit{Kind: tcInstance, Action: tools.DependencyActionDetached},
		Warnings:    []tools.WalkSpecWarning{{Template: "detaches {count}", WhenAny: true}},
	}}, tools.DeclaredState{keyPlacementGroupMembers: []any{}})

	if len(details.Dependencies) != 0 || len(details.Warnings) != 0 {
		t.Errorf("details = %+v, want silence", details)
	}
}

func TestWalkEngineScalarEmitsOnPositiveAlone(t *testing.T) {
	t.Parallel()

	spec := tools.WalkSpec{
		StateScalar: keyManagedLinodeSettingsLinodeID,
		Emit: tools.WalkSpecEmit{
			Kind: tcInstance, IDField: keyManagedLinodeSettingsLinodeID, Action: tools.DependencyActionDetached,
			Note: "Volume is attached; it detaches from this instance before deletion.",
		},
	}

	attached := runWalks(t, []tools.WalkSpec{spec},
		tools.DeclaredState{keyManagedLinodeSettingsLinodeID: json.Number("123")})
	if len(attached.Dependencies) != 1 {
		t.Fatalf("dependencies = %+v, want the one attachment", attached.Dependencies)
	}

	detached := runWalks(t, []tools.WalkSpec{spec}, tools.DeclaredState{})
	if len(detached.Dependencies) != 0 {
		t.Errorf("dependencies = %+v, want none for an unattached volume", detached.Dependencies)
	}
}

func TestWalkEngineFiltersFoldedAndCountsTheTotal(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{
		"records": []any{
			tools.DeclaredState{argType: "ns", managedContactNameParam: "a"},
			tools.DeclaredState{argType: walkRecordNS, managedContactNameParam: "b"},
			tools.DeclaredState{argType: "A", managedContactNameParam: "c"},
		},
	}

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: "records",
		Filter:      &tools.WalkSpecFilter{Field: argType, Value: walkRecordNS, Fold: true},
		Emit: tools.WalkSpecEmit{
			Kind: "domain_record", Action: "cascade_deleted", Note: "NS record for {name}",
		},
		Warnings: []tools.WalkSpecWarning{
			{
				Template: "Deleting this domain destroys {total} DNS record(s), including {count} NS record(s).",
				WhenAny:  true,
			},
		},
	}}, state)

	if len(details.Dependencies) != 2 {
		t.Fatalf("dependencies = %+v, want the two folded NS matches", details.Dependencies)
	}

	if details.Warnings[0] != "Deleting this domain destroys 3 DNS record(s), including 2 NS record(s)." {
		t.Errorf("warning = %q, want both aggregates", details.Warnings[0])
	}
}

func TestWalkEngineTruncationAndConditionalWarnings(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{
		keyStatus:  "Running",
		keyData:    []any{tools.DeclaredState{argType: keyGrantSection}},
		keyResults: json.Number("9"),
	}

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyData,
		Emit: tools.WalkSpecEmit{
			KindField: argType, KindFallback: "resource", Action: tools.DependencyActionRemoved,
			Note: "Loses this tag; the resource itself is not deleted.",
		},
		Warnings: []tools.WalkSpecWarning{
			{Template: "removes it from {max_results} tagged object(s)", WhenAny: true},
			{Template: "Only the first {count} tagged object(s) are itemized.", WhenTruncated: true},
			{Template: "Instance is currently running.", WhenField: keyStatus, WhenEquals: "running"},
			{Template: "never said", WhenField: keyStatus, WhenEquals: "halted"},
		},
	}}, state)

	want := []string{
		"removes it from 9 tagged object(s)",
		"Only the first 1 tagged object(s) are itemized.",
		"Instance is currently running.",
	}
	if len(details.Warnings) != len(want) {
		t.Fatalf("warnings = %v, want %v", details.Warnings, want)
	}

	for index, sentence := range want {
		if details.Warnings[index] != sentence {
			t.Errorf("warning[%d] = %q, want %q", index, details.Warnings[index], sentence)
		}
	}
}

func TestWalkEngineDropsANoteNoPlaceholderFills(t *testing.T) {
	t.Parallel()

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: memberDisks,
		Emit: tools.WalkSpecEmit{
			SideEffects: true,
			Note:        `Disk "{label}" is erased.`,
		},
	}}, tools.DeclaredState{memberDisks: []any{tools.DeclaredState{keySize: json.Number("100")}}})

	if len(details.SideEffects) != 0 {
		t.Errorf("side effects = %v, want the unfillable line dropped", details.SideEffects)
	}
}

func TestWalkEngineRouteWalkWordsItsFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{{
			ListTool:         "linode_firewall_device_list",
			Values:           []any{123},
			New:              func() proto.Message { return &linodev1.FirewallDevice{} },
			Emit:             tools.WalkSpecEmit{Kind: "device", Action: tools.DependencyActionRemoved},
			ListErrorWarning: "Could not list firewall devices: {error}",
		}}, tools.DeclaredState{}, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Warnings) != 1 || len(details.Dependencies) != 0 {
		t.Fatalf("details = %+v, want the one failure warning", details)
	}
}

func TestWalkEngineGuardsOnNamedAggregates(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{
		keyCount: json.Number("2"),
		walkMemberPools: []any{
			tools.DeclaredState{keyCount: json.Number("3"), keyPlacementGroupLinodes: []any{
				tools.DeclaredState{keySupportTicketID: json.Number("1")},
			}},
			tools.DeclaredState{keyCount: json.Number("0"), keyPlacementGroupLinodes: []any{}},
		},
	}

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: walkMemberPools,
		Emit:        tools.WalkSpecEmit{Kind: "pool", Action: "cascade_deleted"},
		Warnings: []tools.WalkSpecWarning{
			{Template: "destroys {count} pool(s) and {sum:count} node(s)", WhenPositive: "sum:count"},
			{Template: "{sum:len:linodes} interface(s) detach", WhenPositive: "sum:len:linodes"},
			{Template: "never: state count", WhenPositive: "state:absent"},
		},
	}}, state)

	want := []string{
		"destroys 2 pool(s) and 3 node(s)",
		"1 interface(s) detach",
	}
	if len(details.Warnings) != len(want) || details.Warnings[0] != want[0] || details.Warnings[1] != want[1] {
		t.Errorf("warnings = %v, want %v", details.Warnings, want)
	}
}

func TestWalkEnginePresencePairSelectsOneWording(t *testing.T) {
	t.Parallel()

	spec := func() tools.WalkSpec {
		return tools.WalkSpec{
			StateMember: memberDisks,
			Emit:        tools.WalkSpecEmit{SideEffects: true, Note: "Disk {label} is erased."},
			Warnings: []tools.WalkSpecWarning{
				{Template: `Rebuild replaces the current image "{state:image}".`, WhenPresent: keyImage},
				{Template: "Rebuild destroys all data on the instance.", WhenAbsent: keyImage},
			},
		}
	}

	imaged := runWalks(t, []tools.WalkSpec{spec()},
		tools.DeclaredState{keyImage: walkImageFixture, memberDisks: []any{}})
	if len(imaged.Warnings) != 1 || imaged.Warnings[0] != `Rebuild replaces the current image "linode/ubuntu24.04".` {
		t.Errorf("imaged warnings = %v, want the named wording alone", imaged.Warnings)
	}

	bare := runWalks(t, []tools.WalkSpec{spec()}, tools.DeclaredState{memberDisks: []any{}})
	if len(bare.Warnings) != 1 || bare.Warnings[0] != "Rebuild destroys all data on the instance." {
		t.Errorf("bare warnings = %v, want the fallback wording alone", bare.Warnings)
	}
}

func TestWalkEngineReadsDottedKindAndLabel(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{keyWalkDevices: []any{
		tools.DeclaredState{keyWalkEntity: map[string]any{
			argType: keyGrantSection, keySupportTicketID: json.Number("5"), managedServiceLabelParam: firewallDeviceLabelFixture,
		}},
	}}

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyWalkDevices,
		Emit: tools.WalkSpecEmit{
			KindField: "entity.type", KindFallback: walkKindResource,
			IDField: "entity.id", LabelField: "entity.label",
			Action: tools.DependencyActionRemoved, Note: "Loses this firewall.",
		},
	}}, state)

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %+v, want the one device", details.Dependencies)
	}

	dependency := details.Dependencies[0]
	if dependency.Kind != keyGrantSection || dependency.Label != firewallDeviceLabelFixture || walkRenderedID(dependency.ID) != "5" {
		t.Errorf("dependency = %+v, want the dotted entity fields", dependency)
	}
}

// walkRenderedID words an id the way the engine words values.
func walkRenderedID(value any) string {
	if value == nil {
		return ""
	}

	return fmt.Sprintf("%v", value)
}

func TestWalkEngineNoteReadsTheElementsOwnCount(t *testing.T) {
	t.Parallel()

	// The declared collision rule: inside a note the element's own count
	// wins, inside a warning the emitted aggregate does, and a warning with
	// no predicate always speaks.
	details := runWalks(t, []tools.WalkSpec{{
		StateMember: walkMemberPools,
		Emit: tools.WalkSpecEmit{
			Kind: "pool", Action: tools.DependencyActionCascadeDeleted,
			Note: "{count} node(s) ride along.",
		},
		Warnings: []tools.WalkSpecWarning{
			{Template: "destroys {count} pool(s)", WhenAny: true},
			{Template: "the walk always says this one"},
		},
	}}, tools.DeclaredState{walkMemberPools: []any{
		tools.DeclaredState{keyCount: json.Number("3")},
	}})

	if len(details.Dependencies) != 1 || details.Dependencies[0].Note != "3 node(s) ride along." {
		t.Errorf("dependencies = %+v, want the element's own count in the note", details.Dependencies)
	}

	want := []string{"destroys 1 pool(s)", "the walk always says this one"}
	if len(details.Warnings) != len(want) || details.Warnings[0] != want[0] || details.Warnings[1] != want[1] {
		t.Errorf("warnings = %v, want %v", details.Warnings, want)
	}
}

func TestWalkEngineFiltersByArgumentThroughOpenObjects(t *testing.T) {
	t.Parallel()

	state := tools.DeclaredState{keyData: []any{
		tools.DeclaredState{keySupportTicketID: json.Number("1"), keyWalkDevices: map[string]any{
			"sda": map[string]any{keyDiskID: json.Number("77")},
		}},
		tools.DeclaredState{keySupportTicketID: json.Number("2"), keyWalkDevices: map[string]any{
			"sdb": map[string]any{keyDiskID: json.Number("88")},
		}},
		tools.DeclaredState{keySupportTicketID: json.Number("3"), keyWalkDevices: map[string]any{
			"note": "raw",
		}},
	}}

	details, err := tools.RunDependencyWalks(t.Context(), nil, []tools.WalkSpec{{
		StateMember: keyData,
		Filter:      &tools.WalkSpecFilter{Field: "devices.*.disk_id", Argument: keyDiskID},
		Emit: tools.WalkSpecEmit{
			Kind: "config", IDField: keySupportTicketID,
			Action: tools.DependencyActionRemoved, Note: "references disk {arg:disk_id}",
		},
	}}, state, map[string]any{keyDiskID: 77})
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].Note != "references disk 77" {
		t.Errorf("dependencies = %+v, want the one starred match wording its argument", details.Dependencies)
	}
}

func TestWalkEngineFoldsADottedFilterComparison(t *testing.T) {
	t.Parallel()

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyData,
		Filter:      &tools.WalkSpecFilter{Field: "entity.type", Value: keyGrantSection, Fold: true},
		Emit:        tools.WalkSpecEmit{Kind: tcInstance, Action: tools.DependencyActionDetached},
	}}, tools.DeclaredState{keyData: []any{
		tools.DeclaredState{keyWalkEntity: map[string]any{argType: "LINODE"}},
		tools.DeclaredState{keyWalkEntity: map[string]any{argType: "volume"}},
	}})

	if len(details.Dependencies) != 1 {
		t.Errorf("dependencies = %+v, want the folded match alone", details.Dependencies)
	}
}

func TestWalkEngineSaysASideEffectLineThatFills(t *testing.T) {
	t.Parallel()

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: memberDisks,
		Emit: tools.WalkSpecEmit{
			SideEffects: true,
			Note:        `Disk "{label}" (encrypted {encrypted}) is erased.`,
		},
	}}, tools.DeclaredState{memberDisks: []any{
		tools.DeclaredState{managedServiceLabelParam: "boot", "encrypted": true},
		tools.DeclaredState{managedServiceLabelParam: "scratch", "encrypted": false},
	}})

	want := []string{
		`Disk "boot" (encrypted true) is erased.`,
		`Disk "scratch" (encrypted false) is erased.`,
	}
	if len(details.SideEffects) != len(want) || details.SideEffects[0] != want[0] || details.SideEffects[1] != want[1] {
		t.Errorf("side effects = %v, want %v", details.SideEffects, want)
	}
}

func TestWalkEngineFallsBackForAnUntypedElementAndAnUnsummedNote(t *testing.T) {
	t.Parallel()

	// Sums are warning vocabulary, so a note naming one reads empty rather
	// than reading the element.
	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyData,
		Emit: tools.WalkSpecEmit{
			KindField: argType, KindFallback: walkKindResource,
			Action: tools.DependencyActionRemoved, Note: "loses {sum:count}",
		},
	}}, tools.DeclaredState{keyData: []any{tools.DeclaredState{}}})

	if len(details.Dependencies) != 1 {
		t.Fatalf("dependencies = %+v, want the untyped element", details.Dependencies)
	}

	if details.Dependencies[0].Kind != walkKindResource || details.Dependencies[0].Note != "loses " {
		t.Errorf("dependency = %+v, want the fallback kind and the empty sum", details.Dependencies[0])
	}
}

func TestWalkEngineEnrichesTemplatesFromARead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"label": "web-01"}`))
	}))
	defer server.Close()

	spec := func() tools.WalkSpec {
		return tools.WalkSpec{
			StateScalar: keyManagedLinodeSettingsLinodeID,
			Enrich: &tools.WalkSpecEnrich{
				New:    func() proto.Message { return &linodev1.Instance{} },
				Tool:   walkEnrichTool,
				Fields: []string{managedServiceLabelParam},
				Values: []any{5},
			},
			Emit: tools.WalkSpecEmit{
				Kind: tcInstance, IDField: keyManagedLinodeSettingsLinodeID,
				Action: tools.DependencyActionDetached, Note: "attached to {enrich:label}",
			},
		}
	}

	state := tools.DeclaredState{keyManagedLinodeSettingsLinodeID: json.Number("5")}

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{spec()}, state, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].Note != "attached to web-01" {
		t.Fatalf("dependencies = %+v, want the enriched label", details.Dependencies)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	bare, err := tools.RunDependencyWalks(t.Context(), stateFormClient(broken.URL),
		[]tools.WalkSpec{spec()}, state, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	// Decoration only: the walk's own line stands without the read.
	if len(bare.Dependencies) != 1 || bare.Dependencies[0].Note != "attached to " {
		t.Errorf("dependencies = %+v, want the line with the read gone", bare.Dependencies)
	}
}

func TestWalkEngineRefusesACanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := tools.RunDependencyWalks(ctx, nil, nil, tools.DeclaredState{}, nil); err == nil {
		t.Fatal("RunDependencyWalks accepted a canceled context")
	}
}

func TestWalkEngineCountsANestedListInANote(t *testing.T) {
	t.Parallel()

	details := runWalks(t, []tools.WalkSpec{{
		StateMember: keyPlacementGroupLinodes,
		Emit: tools.WalkSpecEmit{
			Kind: tcInstance, IDField: keySupportTicketID,
			Action: tools.DependencyActionDetached,
			Note:   "{len:interfaces} interface(s) in this subnet are detached.",
		},
	}}, tools.DeclaredState{keyPlacementGroupLinodes: []any{
		tools.DeclaredState{keySupportTicketID: json.Number("9"), "interfaces": []any{
			tools.DeclaredState{}, tools.DeclaredState{},
		}},
	}})

	if len(details.Dependencies) != 1 || details.Dependencies[0].Note != "2 interface(s) in this subnet are detached." {
		t.Errorf("dependencies = %+v, want the counted interfaces", details.Dependencies)
	}
}

func TestWalkEngineFallsBackToTheEnrichedLabel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"label": "web-01"}`))
	}))
	defer server.Close()

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{{
			StateScalar: keyManagedLinodeSettingsLinodeID,
			Enrich: &tools.WalkSpecEnrich{
				New:    func() proto.Message { return &linodev1.Instance{} },
				Tool:   walkEnrichTool,
				Fields: []string{managedServiceLabelParam},
				Values: []any{5},
			},
			Emit: tools.WalkSpecEmit{
				Kind: tcInstance, IDField: keyManagedLinodeSettingsLinodeID,
				LabelField: "linode_label", LabelFallback: "{enrich:label}",
				Action: tools.DependencyActionDetached,
			},
		}}, tools.DeclaredState{keyManagedLinodeSettingsLinodeID: json.Number("5")}, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].Label != firewallDeviceLabelFixture {
		t.Errorf("dependencies = %+v, want the enriched label standing in", details.Dependencies)
	}
}

func TestWalkEngineSkipsTheEnrichmentOverNothingKept(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)

		_, _ = w.Write([]byte(`{"label": "web-01"}`))
	}))
	defer server.Close()

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{{
			StateMember: keyPlacementGroupMembers,
			Enrich: &tools.WalkSpecEnrich{
				New:    func() proto.Message { return &linodev1.Instance{} },
				Tool:   walkEnrichTool,
				Fields: []string{managedServiceLabelParam},
				Values: []any{5},
			},
			Emit: tools.WalkSpecEmit{Kind: tcInstance, Action: tools.DependencyActionDetached},
			Warnings: []tools.WalkSpecWarning{
				{Template: "in {enrich:label}", WhenAny: true},
			},
		}}, tools.DeclaredState{keyPlacementGroupMembers: []any{}}, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Warnings) != 0 || calls.Load() != 0 {
		t.Errorf("warnings = %v with %d read(s), want silence without a read",
			details.Warnings, calls.Load())
	}
}

func TestWalkEngineReadsPastANullDeviceSlot(t *testing.T) {
	t.Parallel()

	// The API spells an unused slot as null; the element decode prunes it
	// while the filter reads the slot that is there.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data": [{"id": 91, "label": "boot-config",` +
			` "devices": {"sda": {"disk_id": 5}, "sdb": null},` +
			` "we\"ird\u0001": 1}], "results": 1}`))
	}))
	defer server.Close()

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{{
			ListTool: "linode_instance_config_list",
			Values:   []any{123},
			New:      func() proto.Message { return &linodev1.InstanceConfig{} },
			Filter:   &tools.WalkSpecFilter{Field: "devices.*.disk_id", Argument: keyDiskID},
			Emit: tools.WalkSpecEmit{
				Kind: "instance_config", IDField: keySupportTicketID,
				Action: tools.DependencyActionRemoved,
			},
		}}, tools.DeclaredState{}, map[string]any{keyDiskID: 5})
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Dependencies) != 1 || walkRenderedID(details.Dependencies[0].ID) != "91" {
		t.Errorf("dependencies = %+v, want the config past its null slot", details.Dependencies)
	}
}

func TestWalkEngineWordsAScalarElementAsItsDecodeFailure(t *testing.T) {
	t.Parallel()

	// A list whose element is not an object cannot decode through the
	// contract, and the walk degrades to its declared warning.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data": [5], "results": 1}`))
	}))
	defer server.Close()

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{{
			ListTool:         "linode_instance_config_list",
			Values:           []any{123},
			New:              func() proto.Message { return &linodev1.InstanceConfig{} },
			Emit:             tools.WalkSpecEmit{Kind: "instance_config", Action: tools.DependencyActionRemoved},
			ListErrorWarning: "Could not list instance configs: {error}",
		}}, tools.DeclaredState{}, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Warnings) != 1 || len(details.Dependencies) != 0 {
		t.Errorf("details = %+v, want the one decode warning", details)
	}
}

func TestWalkEngineWalksANestedListMember(t *testing.T) {
	t.Parallel()

	// The IP read answers a nested object; the walk keeps ipv4.public and
	// must not emit the reserved address beside it.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ipv4": {"public": [{"address": "203.0.113.10"}],` +
			` "reserved": [{"address": "203.0.113.99"}]}}`))
	}))
	defer server.Close()

	details, err := tools.RunDependencyWalks(t.Context(), stateFormClient(server.URL),
		[]tools.WalkSpec{{
			ListTool:   walkIPListTool,
			ListMember: "ipv4.public",
			Values:     []any{123},
			New:        func() proto.Message { return &linodev1.InstanceIPsResponse{} },
			Emit: tools.WalkSpecEmit{
				Kind: "public_ip", LabelField: "address",
				Action: "released",
			},
			ListErrorWarning: "Could not list IP addresses: {error}",
		}}, tools.DeclaredState{}, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(details.Dependencies) != 1 || details.Dependencies[0].Label != "203.0.113.10" {
		t.Fatalf("dependencies = %+v, want the one public address", details.Dependencies)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	bare, err := tools.RunDependencyWalks(t.Context(), stateFormClient(broken.URL),
		[]tools.WalkSpec{{
			ListTool:   walkIPListTool,
			ListMember: "ipv4.public",
			Values:     []any{123},
			New:        func() proto.Message { return &linodev1.InstanceIPsResponse{} },
			Emit: tools.WalkSpecEmit{
				Kind: "public_ip", LabelField: "address", Action: "released",
			},
			ListErrorWarning: "Could not list IP addresses: {error}",
		}}, tools.DeclaredState{}, nil)
	if err != nil {
		t.Fatalf("RunDependencyWalks: %v", err)
	}

	if len(bare.Warnings) != 1 || len(bare.Dependencies) != 0 {
		t.Errorf("details = %+v, want the one failure warning", bare)
	}
}

func TestWalkEngineBillingWordsAllThreeBranches(t *testing.T) {
	t.Parallel()

	spec := func() *tools.BillingSpec {
		return &tools.BillingSpec{
			New:                func() proto.Message { return &linodev1.InstanceType{} },
			PriceTool:          "linode_type_get",
			TypeField:          argType,
			Amount:             "-{monthly:.2f}",
			Sentence:           "Instance billing stops. Attached volume billing continues.",
			UnknownSentence:    "Could not fetch type pricing for the estimate.",
			AbsentTypeSentence: "",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id": "g6-standard-2", "price": {"monthly": 24.0}}`))
	}))
	defer server.Close()

	var priced tools.DryRunDetails

	tools.RunBillingDelta(t.Context(), stateFormClient(server.URL), spec(),
		tools.DeclaredState{argType: nodePoolTypeFixture}, &priced)

	if priced.BillingDelta == nil || priced.BillingDelta.MonthlyChangeUSD != "-24.00" {
		t.Errorf("billing = %+v, want the signed amount", priced.BillingDelta)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	var unknown tools.DryRunDetails

	tools.RunBillingDelta(t.Context(), stateFormClient(broken.URL), spec(),
		tools.DeclaredState{argType: nodePoolTypeFixture}, &unknown)

	if unknown.BillingDelta == nil || unknown.BillingDelta.MonthlyChangeUSD != walkUnknownPrice ||
		unknown.BillingDelta.Note != "Could not fetch type pricing for the estimate." {
		t.Errorf("billing = %+v, want the unknown sentinel", unknown.BillingDelta)
	}

	var absent tools.DryRunDetails

	tools.RunBillingDelta(t.Context(), nil, spec(), tools.DeclaredState{}, &absent)

	if absent.BillingDelta == nil || absent.BillingDelta.MonthlyChangeUSD != walkUnknownPrice ||
		absent.BillingDelta.Note != "" {
		t.Errorf("billing = %+v, want the bare unknown", absent.BillingDelta)
	}
}
