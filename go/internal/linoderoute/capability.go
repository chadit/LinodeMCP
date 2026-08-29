package linoderoute

import (
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// unspecified is the capability enum's zero value, which is what a message
// declaring no tier reads back as.
const unspecified = linodev1.ToolCapability_TOOL_CAPABILITY_UNSPECIFIED

// meta is the tier of a tool that reaches no Linode route, working on local
// config or session state instead.
const meta = linodev1.ToolCapability_TOOL_CAPABILITY_META

// Tool is one tool the proto contract declares.
type Tool struct {
	Name       string
	Capability linodev1.ToolCapability
	Routed     bool
}

// Declaration is what one message's options say about a tool, kept as read so a
// message whose options contradict each other is reported rather than silently
// resolved to one of the readings. An absent marker and one naming nothing both
// leave the name empty: neither says which tool the message belongs to.
type Declaration struct {
	Message    string
	RouteTool  string
	MetaTool   string
	Capability linodev1.ToolCapability
	Surface    linodev1.ApiSurface
	// SurfaceDeclared separates a message that wrote the option out from one that
	// left it absent, which read back the same otherwise: both answer v4.
	SurfaceDeclared bool
}

// Name is the tool a declaration belongs to. A well-formed declaration carries
// the name in exactly one of its two markers, so a routed tool's name is
// written once.
func (d Declaration) Name() string {
	if d.RouteTool != "" {
		return d.RouteTool
	}

	return d.MetaTool
}

// wellFormed reports whether a declaration names one tool at one tier, the
// condition for it to resolve to a Tool at all.
func (d Declaration) wellFormed() bool {
	return len(d.defects()) == 0 && d.Name() != ""
}

// defects lists every way one message's options fail to describe exactly one
// tool at exactly one tier. A message naming no tool and declaring no tier is
// clean, which is how every response and nested type falls out here.
func (d Declaration) defects() []string {
	if d.RouteTool == "" && d.MetaTool == "" && d.Capability == unspecified {
		return nil
	}

	found := make([]string, 0)

	if d.RouteTool != "" && d.MetaTool != "" {
		found = append(found, fmt.Sprintf(
			"%s: names %s as a routed tool and %s as a meta tool, and a tool is one or the other",
			d.Message, d.RouteTool, d.MetaTool,
		))
	}

	if d.RouteTool == "" && d.MetaTool == "" {
		found = append(found, fmt.Sprintf(
			"%s: declares %s but no marker names its tool", d.Message, d.Capability,
		))
	}

	found = append(found, d.surfaceDefects()...)

	return append(found, d.tierDefects()...)
}

// surfaceDefects lists the ways a declared surface cannot apply. A meta tool
// reaches no Linode API, so it has no surface to answer on, and writing the
// default out gives v4 a second spelling the emitter relies on not existing.
func (d Declaration) surfaceDefects() []string {
	if !d.SurfaceDeclared {
		return nil
	}

	if d.RouteTool == "" {
		return []string{fmt.Sprintf(
			"%s: declares %s but no tool_route", d.Message, d.Surface,
		)}
	}

	if d.Surface == linodev1.ApiSurface_API_SURFACE_UNSPECIFIED ||
		d.Surface == linodev1.ApiSurface_API_SURFACE_V4 {
		return []string{fmt.Sprintf(
			"%s: declares %s, which is the default an unannotated tool already gets",
			d.Message, d.Surface,
		)}
	}

	return nil
}

// tierDefects lists the ways a declared tier disagrees with the marker beside
// it. A meta marker and the meta tier say one fact twice, so letting them
// differ would put two answers to "is this tool routed" in the contract itself.
func (d Declaration) tierDefects() []string {
	if d.Capability == unspecified {
		return []string{fmt.Sprintf(
			"%s: names %s but declares no capability", d.Message, d.Name(),
		)}
	}

	if d.MetaTool != "" && d.Capability != meta {
		return []string{fmt.Sprintf(
			"%s: %s carries a meta marker but declares %s",
			d.Message, d.MetaTool, d.Capability,
		)}
	}

	if d.RouteTool != "" && d.Capability == meta {
		return []string{fmt.Sprintf(
			"%s: %s declares a route and %s, and a meta tool reaches no route",
			d.Message, d.RouteTool, d.Capability,
		)}
	}

	return nil
}

// Tools returns every tool the contract declares, name-sorted. A message whose
// options contradict each other names no tool here; Validate is what reports it.
func Tools() []Tool {
	found := make([]Tool, 0)

	for _, declared := range declarations() {
		if !declared.wellFormed() {
			continue
		}

		found = append(found, Tool{
			Name:       declared.Name(),
			Capability: declared.Capability,
			Routed:     declared.RouteTool != "",
		})
	}

	slices.SortFunc(found, func(left, right Tool) int {
		return strings.Compare(left.Name, right.Name)
	})

	return found
}

// validateDeclarations reports every message that does not name exactly one
// tool at exactly one tier, including two messages that name the same tool.
func validateDeclarations(declared []Declaration) error {
	defects := make([]string, 0)
	claimed := make(map[string]string, len(declared))

	for _, entry := range declared {
		defects = append(defects, entry.defects()...)

		name := entry.Name()
		if first, taken := claimed[name]; name != "" && taken {
			defects = append(defects, fmt.Sprintf(
				"%s: names %s, which %s already declares", entry.Message, name, first,
			))
		}

		claimed[name] = entry.Message
	}

	if len(defects) == 0 {
		return nil
	}

	slices.Sort(defects)

	return fmt.Errorf("%w: %s", ErrDeclaration, strings.Join(defects, "; "))
}

// ValidateRegistered reports the tools a server staged that the contract does
// not declare and the tools it declares that the server did not stage. Both
// directions fail: a staged tool with no declaration has no tier to filter it
// by, and a declared tool nothing staged is a dropped or renamed handler.
func ValidateRegistered(registered []string) error {
	declared := Tools()

	names := make([]string, 0, len(declared))
	for _, tool := range declared {
		names = append(names, tool.Name)
	}

	report := make([]string, 0)

	if extra := difference(registered, names); len(extra) > 0 {
		report = append(report,
			"staged but not declared: "+strings.Join(extra, ", "))
	}

	if missing := difference(names, registered); len(missing) > 0 {
		report = append(report,
			"declared but not staged: "+strings.Join(missing, ", "))
	}

	if len(report) == 0 {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrRegistered, strings.Join(report, "; "))
}

// difference returns the sorted entries of left that right does not carry.
func difference(left, right []string) []string {
	held := make(map[string]struct{}, len(right))
	for _, entry := range right {
		held[entry] = struct{}{}
	}

	missing := make([]string, 0)

	for _, entry := range left {
		if _, ok := held[entry]; !ok {
			missing = append(missing, entry)
		}
	}

	slices.Sort(missing)

	return slices.Compact(missing)
}

// declarations reads what every message in the contract declares about a tool.
// Each option goes through the comma-ok form and then its getter, so an absent
// option and one holding another type both land on a nil marker whose getter
// answers with the empty name, avoiding a branch no descriptor could reach.
func declarations() []Declaration {
	found := make([]Declaration, 0)

	walkMessages(func(message protoreflect.MessageDescriptor) bool {
		options := message.Options()
		route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
		meta, _ := proto.GetExtension(options, linodev1.E_ToolMeta).(*linodev1.ToolMeta)
		capability, _ := proto.GetExtension(
			options, linodev1.E_ToolCapability,
		).(linodev1.ToolCapability)
		surface, _ := proto.GetExtension(
			options, linodev1.E_ToolApiSurface,
		).(linodev1.ApiSurface)

		found = append(found, Declaration{
			Message:         string(message.FullName()),
			RouteTool:       route.GetTool(),
			MetaTool:        meta.GetTool(),
			Capability:      capability,
			Surface:         surface,
			SurfaceDeclared: proto.HasExtension(options, linodev1.E_ToolApiSurface),
		})

		return true
	})

	return found
}

// reservedArgument reports an argument name a tool input may never declare. The
// surface a tool answers on is a property of the tool, and an argument by any of
// these names would let a caller, or a model reading the advertised schema, move
// a call to a surface the contract did not declare.
func reservedArgument(name string) bool {
	switch name {
	case "api_version", "api_surface", "surface", "beta":
		return true
	}

	return false
}

// ValidateArguments reports tool inputs that name the API surface as an
// argument, keyed by message name.
//
// Exported for the reason ValidateContract is: `make tool-routes` refuses the
// shape before it can be generated, so proving the check still bites means
// handing it an input that carries one.
//
// Callers pass tool inputs only. VersionResponse legitimately declares an
// api_version field, and a response is not something a caller fills.
func ValidateArguments(inputs map[string][]string) error {
	defects := make([]string, 0)

	for message, arguments := range inputs {
		for _, argument := range arguments {
			if !reservedArgument(argument) {
				continue
			}

			defects = append(defects, fmt.Sprintf(
				"%s: declares argument %q, and the API surface is a per-tool"+
					" declaration rather than an argument", message, argument,
			))
		}
	}

	if len(defects) == 0 {
		return nil
	}

	slices.Sort(defects)

	return fmt.Errorf("%w: %s", ErrDeclaration, strings.Join(defects, "; "))
}

// inputArguments is the field-name list of every message that names a tool.
func inputArguments() map[string][]string {
	found := make(map[string][]string)

	walkMessages(func(message protoreflect.MessageDescriptor) bool {
		options := message.Options()
		route, _ := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute)
		meta, _ := proto.GetExtension(options, linodev1.E_ToolMeta).(*linodev1.ToolMeta)

		if route.GetTool() == "" && meta.GetTool() == "" {
			return true
		}

		fields := message.Fields()
		names := make([]string, 0, fields.Len())

		for i := range fields.Len() {
			names = append(names, string(fields.Get(i).Name()))
		}

		found[string(message.FullName())] = names

		return true
	})

	return found
}
