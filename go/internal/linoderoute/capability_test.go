package linoderoute_test

import (
	"errors"
	"strings"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// No generator emits these, so every declaration below is built by hand:
// firstMessage and secondMessage exist to have two messages claim one tool.
const (
	brokenMessage = "linode.mcp.v1.BrokenInput"
	firstMessage  = "linode.mcp.v1.FirstInput"
	secondMessage = "linode.mcp.v1.SecondInput"
)

// ordinaryArgument stands in for any argument a tool legitimately takes, beside
// the reserved ones the guard refuses.
const ordinaryArgument = "label"

// instanceDelete comes from the route tests; these add a meta and a read tool.
const (
	versionTool = "version"
	tagList     = "linode_tag_list"
)

// Read against the manifest rather than a count, so adding a tool has to move
// both the contract and the cross-language manifest or fail here.
func TestToolsCoversTheCapabilityManifest(t *testing.T) {
	t.Parallel()

	want := manifestTools(t)

	got := make([]string, 0, len(linoderoute.Tools()))
	for _, tool := range linoderoute.Tools() {
		got = append(got, tool.Name)
	}

	listed := make([]string, 0, len(want))
	for name := range want {
		listed = append(listed, name)
	}

	if missing := difference(listed, got); len(missing) > 0 {
		t.Errorf("tools the contract does not declare: %s", strings.Join(missing, ", "))
	}

	if extra := difference(got, listed); len(extra) > 0 {
		t.Errorf("declared tools the manifest does not list: %s", strings.Join(extra, ", "))
	}
}

// Meta tools reach no Linode route. The manifest says that with a tier, the
// contract with the marker carrying the tool name, and the two must agree.
func TestToolsAgreesWithTheManifestOnWhichAreMeta(t *testing.T) {
	t.Parallel()

	tiers := manifestTools(t)

	for _, tool := range linoderoute.Tools() {
		tier, listed := tiers[tool.Name]
		if !listed {
			continue
		}

		if want := tier != metaTier; tool.Routed != want {
			t.Errorf("Tools()[%q].Routed = %v, want %v for tier %s",
				tool.Name, tool.Routed, want, tier)
		}
	}
}

// Pinned by hand so a walk that keeps the tool count right while reading the
// tier off an adjacent message, or defaulting it, still fails.
func TestToolsReportsTheDeclaredTier(t *testing.T) {
	t.Parallel()

	want := map[string]linodev1.ToolCapability{
		instanceDelete: linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
		tagList:        linodev1.ToolCapability_TOOL_CAPABILITY_READ,
		versionTool:    linodev1.ToolCapability_TOOL_CAPABILITY_META,
	}

	found := make(map[string]linodev1.ToolCapability, len(want))

	for _, tool := range linoderoute.Tools() {
		if _, pinned := want[tool.Name]; pinned {
			found[tool.Name] = tool.Capability
		}
	}

	for name, tier := range want {
		if found[name] != tier {
			t.Errorf("Tools()[%q].Capability = %v, want %v", name, found[name], tier)
		}
	}
}

// A message declaring no tier reads back as the unspecified zero value, so none
// may reach Tools(): a walk that stopped reading the option would still pass.
func TestEveryDeclaredToolCarriesATier(t *testing.T) {
	t.Parallel()

	tools := linoderoute.Tools()
	if len(tools) == 0 {
		t.Fatal("len(Tools()) = 0, want the contract to declare tools")
	}

	for _, tool := range tools {
		if tool.Capability == linodev1.ToolCapability_TOOL_CAPABILITY_UNSPECIFIED {
			t.Errorf("Tools()[%q].Capability = unspecified, want a declared tier", tool.Name)
		}
	}
}

// Each case is a way a message can look annotated while naming no tool, naming
// two, or claiming a tier that contradicts the marker beside it.
func TestValidateContractRejectsBrokenDeclarations(t *testing.T) {
	t.Parallel()

	for name, declared := range map[string]linoderoute.Declaration{
		"route and meta together": {
			Message:    brokenMessage,
			RouteTool:  instanceDelete,
			MetaTool:   versionTool,
			Capability: linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
		},
		"tier with no marker": {
			Message:    brokenMessage,
			Capability: linodev1.ToolCapability_TOOL_CAPABILITY_READ,
		},
		"route with no tier": {
			Message:   brokenMessage,
			RouteTool: instanceDelete,
		},
		"meta with no tier": {
			Message:  brokenMessage,
			MetaTool: versionTool,
		},
		"meta marker at a routed tier": {
			Message:    brokenMessage,
			MetaTool:   versionTool,
			Capability: linodev1.ToolCapability_TOOL_CAPABILITY_READ,
		},
		"routed marker at the meta tier": {
			Message:    brokenMessage,
			RouteTool:  instanceDelete,
			Capability: linodev1.ToolCapability_TOOL_CAPABILITY_META,
		},
		"surface on a tool that reaches no route": {
			Message:         brokenMessage,
			MetaTool:        versionTool,
			Capability:      linodev1.ToolCapability_TOOL_CAPABILITY_META,
			Surface:         linodev1.ApiSurface_API_SURFACE_V4BETA,
			SurfaceDeclared: true,
		},
		"the default surface written out": {
			Message:         brokenMessage,
			RouteTool:       instanceDelete,
			Capability:      linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
			Surface:         linodev1.ApiSurface_API_SURFACE_V4,
			SurfaceDeclared: true,
		},
		"the zero surface written out": {
			Message:         brokenMessage,
			RouteTool:       instanceDelete,
			Capability:      linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
			Surface:         linodev1.ApiSurface_API_SURFACE_UNSPECIFIED,
			SurfaceDeclared: true,
		},
	} {
		err := linoderoute.ValidateContract([]linoderoute.Declaration{declared}, nil)
		if !errors.Is(err, linoderoute.ErrDeclaration) {
			t.Errorf("ValidateContract() error for %s = %v, want ErrDeclaration", name, err)
		}
	}
}

// A startup failure over 461 messages is only actionable if the text names the
// message and the tier it claimed.
func TestValidateContractNamesTheBrokenMessage(t *testing.T) {
	t.Parallel()

	declared := linoderoute.Declaration{
		Message:    brokenMessage,
		Capability: linodev1.ToolCapability_TOOL_CAPABILITY_READ,
	}

	err := linoderoute.ValidateContract([]linoderoute.Declaration{declared}, nil)

	want := linoderoute.ErrDeclaration.Error() + ": " + brokenMessage +
		": declares TOOL_CAPABILITY_READ but no marker names its tool"
	if err == nil || err.Error() != want {
		t.Errorf("ValidateContract() error = %v, want %v", err, want)
	}
}

// Two messages naming one tool leaves no answer to which input the tool takes,
// and whichever one a walk reached first would silently win.
func TestValidateContractRejectsTwoMessagesClaimingOneTool(t *testing.T) {
	t.Parallel()

	first := linoderoute.Declaration{
		Message:    firstMessage,
		RouteTool:  instanceDelete,
		Capability: linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
	}
	second := first
	second.Message = secondMessage

	err := linoderoute.ValidateContract([]linoderoute.Declaration{first, second}, nil)
	if !errors.Is(err, linoderoute.ErrDeclaration) {
		t.Fatalf("ValidateContract() error = %v, want ErrDeclaration", err)
	}

	want := linoderoute.ErrDeclaration.Error() + ": " + secondMessage +
		": names " + instanceDelete + ", which " + firstMessage + " already declares"
	if err.Error() != want {
		t.Errorf("ValidateContract() error = %v, want %v", err, want)
	}
}

// A message that declares nothing at all is not a defect: every response type
// is one.
func TestValidateContractAcceptsWhatTheContractHolds(t *testing.T) {
	t.Parallel()

	declared := []linoderoute.Declaration{
		{
			Message:    "linode.mcp.v1.InstanceDeleteInput",
			RouteTool:  instanceDelete,
			Capability: linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
		},
		{
			Message:    "linode.mcp.v1.VersionInput",
			MetaTool:   versionTool,
			Capability: linodev1.ToolCapability_TOOL_CAPABILITY_META,
		},
		{Message: "linode.mcp.v1.VersionResponse"},
	}

	if err := linoderoute.ValidateContract(declared, nil); err != nil {
		t.Errorf("ValidateContract() = %v, want nil", err)
	}
}

// The startup path a real server takes, so a false positive stops it starting.
func TestValidateRegisteredAcceptsTheDeclaredSet(t *testing.T) {
	t.Parallel()

	if err := linoderoute.ValidateRegistered(declaredNames(t)); err != nil {
		t.Errorf("ValidateRegistered() = %v, want nil", err)
	}
}

// A staged tool the contract never declared has no tier to filter it by; a
// declared tool nothing staged is a handler dropped without the contract moving
// with it. Neither is visible from the other direction, so both are reported.
func TestValidateRegisteredReportsBothDirections(t *testing.T) {
	t.Parallel()

	declared := declaredNames(t)
	staged := append([]string{unknownTool}, declared[1:]...)

	err := linoderoute.ValidateRegistered(staged)
	if !errors.Is(err, linoderoute.ErrRegistered) {
		t.Fatalf("ValidateRegistered() error = %v, want ErrRegistered", err)
	}

	want := linoderoute.ErrRegistered.Error() +
		": staged but not declared: " + unknownTool +
		"; declared but not staged: " + declared[0]
	if err.Error() != want {
		t.Errorf("ValidateRegistered() error = %v, want %v", err, want)
	}
}

// Staging nothing is not "nothing to check", it is every declared tool missing.
// A check that passed here would pass for a server that registered no tools.
func TestValidateRegisteredReportsAnEmptyCatalog(t *testing.T) {
	t.Parallel()

	if err := linoderoute.ValidateRegistered(nil); !errors.Is(err, linoderoute.ErrRegistered) {
		t.Errorf("ValidateRegistered(nil) error = %v, want ErrRegistered", err)
	}
}

// declaredNames is the set a server has to stage.
func declaredNames(t *testing.T) []string {
	t.Helper()

	tools := linoderoute.Tools()
	if len(tools) == 0 {
		t.Fatal("len(Tools()) = 0, want the contract to declare tools")
	}

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}

	return names
}

// A routed tool on a real non-default surface is the case the option exists
// for, so it has to pass the same validator that refuses the other shapes.
func TestValidateContractAcceptsANonDefaultSurface(t *testing.T) {
	t.Parallel()

	declared := linoderoute.Declaration{
		Message:         brokenMessage,
		RouteTool:       instanceDelete,
		Capability:      linodev1.ToolCapability_TOOL_CAPABILITY_DESTROY,
		Surface:         linodev1.ApiSurface_API_SURFACE_V4BETA,
		SurfaceDeclared: true,
	}

	if err := linoderoute.ValidateContract([]linoderoute.Declaration{declared}, nil); err != nil {
		t.Errorf("ValidateContract() = %v, want nil", err)
	}
}

// The surface is a declaration, so no tool input may name it as an argument: a
// caller reading the advertised schema could otherwise ask for a call on a
// surface the contract never declared. Nothing generated can carry one, so the
// input is handed over directly.
func TestValidateArgumentsRefusesASurfaceArgument(t *testing.T) {
	t.Parallel()

	for _, argument := range []string{"api_version", "api_surface", "surface", "beta"} {
		inputs := map[string][]string{brokenMessage: {ordinaryArgument, argument}}

		err := linoderoute.ValidateArguments(inputs)
		if !errors.Is(err, linoderoute.ErrDeclaration) {
			t.Errorf("ValidateArguments(%q) error = %v, want ErrDeclaration", argument, err)

			continue
		}

		// The message names the argument, since renaming it is the fix and a
		// finding that did not say which field would not point at one.
		want := linoderoute.ErrDeclaration.Error() + ": " + brokenMessage +
			`: declares argument "` + argument +
			`", and the API surface is a per-tool declaration rather than an argument`
		if err.Error() != want {
			t.Errorf("ValidateArguments(%q) error = %v, want %v", argument, err, want)
		}
	}
}

// The guard reads tool inputs only. VersionResponse declares an api_version
// field legitimately, and a response is not something a caller fills, so a
// guard that walked every message would fail the shipped contract on sight.
func TestValidateArgumentsLeavesOrdinaryArgumentsAlone(t *testing.T) {
	t.Parallel()

	inputs := map[string][]string{
		brokenMessage: {ordinaryArgument, "region", "api_version_note", "betas"},
	}

	if err := linoderoute.ValidateArguments(inputs); err != nil {
		t.Errorf("ValidateArguments() = %v, want nil", err)
	}
}

// The shipped inputs carry no such argument, read through the same walk the
// server validates with, so the guard is proven against the real contract too.
func TestTheShippedInputsNameNoSurfaceArgument(t *testing.T) {
	t.Parallel()

	if err := linoderoute.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// ValidateAll runs the argument guard ahead of the rest, because a tool taking
// its surface as an argument answers on whichever surface the caller asked for,
// which makes every other reading of the contract beside the point.
func TestValidateAllReportsASurfaceArgumentFirst(t *testing.T) {
	t.Parallel()

	inputs := map[string][]string{brokenMessage: {"beta"}}

	err := linoderoute.ValidateAll(inputs, nil, nil)
	if !errors.Is(err, linoderoute.ErrDeclaration) {
		t.Fatalf("ValidateAll() error = %v, want ErrDeclaration", err)
	}

	// Compared whole rather than searched, so this pins that the argument guard
	// is what answered and not the contract half reporting something else.
	want := linoderoute.ErrDeclaration.Error() + ": " + brokenMessage +
		`: declares argument "beta", and the API surface is a per-tool` +
		" declaration rather than an argument"
	if err.Error() != want {
		t.Errorf("ValidateAll() error = %v, want %v", err, want)
	}
}

// Clean inputs pass both halves, which is what the shipped contract is.
func TestValidateAllAcceptsACleanContract(t *testing.T) {
	t.Parallel()

	inputs := map[string][]string{brokenMessage: {ordinaryArgument, "region"}}

	if err := linoderoute.ValidateAll(inputs, nil, nil); err != nil {
		t.Errorf("ValidateAll() = %v, want nil", err)
	}
}
