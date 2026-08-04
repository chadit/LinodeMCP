// Package linoderoute resolves the Linode API operation an MCP tool calls from
// the proto contract, so a client method names its tool instead of spelling out
// a URL the contract already declares. Every non-meta tool's *Input message
// carries a linode.mcp.v1.tool_route option holding the tool name, HTTP method,
// and path template; before this package nothing read it at runtime, so each
// language's client repeated the route with only an offline gate catching drift.
package linoderoute

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// protoPackage scopes the descriptor walk to this repo's contract; the global
// registry also holds descriptor.proto and whatever a dependency links in.
const protoPackage = "linode.mcp.v1"

const (
	slotOpen  = "{"
	slotClose = "}"
)

// Route is one tool's declared operation. Slots are in template order, which is
// the argument order Endpoint fills.
type Route struct {
	Tool     string
	Method   string
	Template string
	Slots    []string
}

// For returns the route a tool declares.
func For(tool string) (Route, error) {
	var found Route

	walk(func(declared *linodev1.ToolRoute) bool {
		if declared.GetTool() != tool {
			return true
		}

		found = newRoute(declared)

		return false
	})

	if found.Tool == "" {
		return Route{}, fmt.Errorf("%w: %s", ErrNoRoute, tool)
	}

	return found, nil
}

// All returns every declared route, tool-sorted.
func All() []Route {
	routes := make([]Route, 0)

	walk(func(declared *linodev1.ToolRoute) bool {
		routes = append(routes, newRoute(declared))

		return true
	})

	slices.SortFunc(routes, func(left, right Route) int {
		return strings.Compare(left.Tool, right.Tool)
	})

	return routes
}

// Resolve returns the method and the filled path a tool's declared route
// produces, both in one call because a request needs both.
func Resolve(tool string, values ...any) (string, string, error) {
	route, err := For(tool)
	if err != nil {
		return "", "", err
	}

	endpoint, err := route.Endpoint(values...)
	if err != nil {
		return "", "", err
	}

	return route.Method, endpoint, nil
}

// Endpoint fills Template's slots from values, in declared order. A wrong value
// count or an empty value is an error rather than a shorter path: "/tags/" still
// reaches the API and addresses the parent collection instead of one resource,
// which for a DELETE is every resource in it.
func (r Route) Endpoint(values ...any) (string, error) {
	if len(values) != len(r.Slots) {
		return "", fmt.Errorf("%w: %s takes %d, got %d",
			ErrValueCount, r.Tool, len(r.Slots), len(values))
	}

	parts, err := split(r.Template)
	if err != nil {
		return "", err
	}

	var (
		built strings.Builder
		next  int
	)

	// Slot values run about the size of the slots they replace.
	built.Grow(len(r.Template))

	for _, part := range parts {
		if !part.slot {
			built.WriteString(part.text)

			continue
		}

		if err := writeValue(&built, values[next], part.text, r.Tool); err != nil {
			return "", err
		}

		next++
	}

	return built.String(), nil
}

// check reports a route whose slots disagree with its template. Reading the
// slots back out proves both describe the same path, which is what a caller
// relies on when it passes values positionally.
func (r Route) check() error {
	parts, err := split(r.Template)
	if err != nil {
		return err
	}

	if found := slotNames(parts); !slices.Equal(found, r.Slots) {
		return fmt.Errorf("%w: %s declares slots %v for %s",
			ErrTemplate, r.Tool, r.Slots, r.Template)
	}

	return nil
}

// Validate reports a contract that cannot describe the surface it claims to: a
// message whose options do not name exactly one tool at exactly one tier, and a
// route whose slots do not match its template. The server runs it at startup so
// a gap stops the process instead of surfacing as a wrong URL on the first tool
// call. It takes no arguments because tool_meta tells a meta tool apart from a
// tool someone forgot to route, so no caller has to supply the routed set.
func Validate() error {
	return ValidateContract(declarations(), All())
}

// ValidateContract is Validate over supplied inputs. Exported for the same
// reason server.ValidateGeneratedSchemas is: `make tool-capability` and `make
// field-location` reject a bad declaration or template before generation, so
// neither failure can come out of the shipped descriptors and proving the checks
// still bite means handing them a broken contract directly. Declarations run
// first because a message naming no tool, or two, is what makes the route half's
// reading of the same contract meaningless.
func ValidateContract(declared []Declaration, routes []Route) error {
	if err := validateDeclarations(declared); err != nil {
		return err
	}

	return validateRoutes(routes)
}

// validateRoutes reports a route whose slots do not match its template.
func validateRoutes(routes []Route) error {
	for _, route := range routes {
		if err := route.check(); err != nil {
			return err
		}
	}

	return nil
}

// segment is one piece of a path template: a literal, or a named slot.
type segment struct {
	text string
	slot bool
}

// split breaks a path template into its literal and slot pieces.
func split(template string) ([]segment, error) {
	var parts []segment

	rest := template

	for {
		open := strings.Index(rest, slotOpen)
		if open < 0 {
			return append(parts, segment{text: rest, slot: false}), nil
		}

		closed := strings.Index(rest[open:], slotClose)
		if closed < 0 {
			return nil, fmt.Errorf("%w: unclosed slot in %s", ErrTemplate, template)
		}

		name := rest[open+len(slotOpen) : open+closed]
		if name == "" || strings.Contains(name, slotOpen) {
			return nil, fmt.Errorf("%w: bad slot name in %s", ErrTemplate, template)
		}

		parts = append(parts,
			segment{text: rest[:open], slot: false},
			segment{text: name, slot: true},
		)
		rest = rest[open+closed+len(slotClose):]
	}
}

// slotNames lists the slot pieces of an already-split template, in order.
func slotNames(parts []segment) []string {
	names := make([]string, 0, len(parts))

	for _, part := range parts {
		if part.slot {
			names = append(names, part.text)
		}
	}

	return names
}

// writeValue appends one path value, escaped, naming the slot and the tool on
// failure: a positional call site gives no other clue which argument was wrong.
// Escaping lives here so no call site can address the wrong route by skipping it.
func writeValue(built *strings.Builder, value any, slot, tool string) error {
	text, err := slotText(value)
	if err != nil {
		return fmt.Errorf("%w: %s slot %s", err, tool, slot)
	}

	if text == "" {
		return fmt.Errorf("%w: %s slot %s", ErrEmptyValue, tool, slot)
	}

	built.WriteString(escapeSegment(text))

	return nil
}

// slotText renders one path value. The accepted set is the identifier types the
// Linode API addresses resources by; anything else formats into a segment no
// endpoint answers, so it fails here instead of on the wire.
func slotText(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case int:
		return strconv.Itoa(typed), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	}

	return "", fmt.Errorf("%w: %T", ErrValueType, value)
}

// newRoute builds the route one declaration describes.
func newRoute(declared *linodev1.ToolRoute) Route {
	route := Route{
		Tool:     declared.GetTool(),
		Method:   declared.GetMethod(),
		Template: declared.GetPath(),
		Slots:    nil,
	}

	// A template that does not parse leaves the slots empty, which check
	// reports as a mismatch instead of the route resolving to a broken path.
	if parts, err := split(route.Template); err == nil {
		route.Slots = slotNames(parts)
	}

	return route
}

// walk hands every declared tool_route to visit, stopping early when visit
// returns false. A message with no tool_route reads back as the option's zero
// value, which is how a request or response type, and a meta tool's input, fall
// out here. Nothing memoizes, since this repo bans package-level state; passing
// the declaration rather than a built Route keeps that affordable, because a
// lookup parses no other tool's template and the descriptors are compiled in.
func walk(visit func(declared *linodev1.ToolRoute) bool) {
	walkMessages(func(message protoreflect.MessageDescriptor) bool {
		declared, isRoute := proto.GetExtension(
			message.Options(), linodev1.E_ToolRoute,
		).(*linodev1.ToolRoute)
		if !isRoute || declared.GetTool() == "" {
			return true
		}

		return visit(declared)
	})
}

// walkMessages hands every top-level message of this repo's proto package to
// visit, stopping early when visit returns false. capability.go's tool reader
// shares this traversal: two walks over the registry for two facts carried by
// one message could disagree about which messages exist.
func walkMessages(visit func(message protoreflect.MessageDescriptor) bool) {
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			if !visit(messages.Get(i)) {
				return false
			}
		}

		return true
	})
}
