package main

import (
	"fmt"
	"strconv"
	"strings"
)

// The one tail entry rendered from the surface; the rest splices back verbatim.
const locationOption = "(linode.mcp.v1.field_location)"

// Retirement is native so protoc enforces it, not a ledger that could drift.
const reservedKeyword = "reserved"

// Upstream answers for method and path; the tool name stays in the overlay.
const routeOption = "(linode.mcp.v1.tool_route)"

// surface is the half upstream techdocs can state. It has no wire numbers and
// no ordering, so no upstream drift can renumber a field.
type surface struct {
	// Fields is keyed by block path and name.
	Fields map[string]surfaceField
	// Values holds names only; an enum value's number is wire identity too.
	Values map[string]struct{}
	// Routes is keyed by the declaring message, so a dropped route leaves a gap.
	Routes map[string]surfaceRoute
}

// surfaceRoute is one route as upstream states it; the tool name is not here.
type surfaceRoute struct {
	Method string `json:"method"`
	// Path is the route template, placeholders included.
	Path string `json:"path"`
}

// surfaceField is one field as upstream knows it; the tags are the descriptor
// spelling this type crosses the file channel in.
type surfaceField struct {
	// Label is "optional", "repeated", "required", or "" for the bare form.
	Label string `json:"label,omitempty"`
	// Type is the declared type, including a map<k, v> spelling.
	Type string `json:"type"`
	// Location is the FIELD_LOCATION_* member, "" when the field states none.
	Location string `json:"location,omitempty"`
}

func newSurface() *surface {
	return &surface{
		Fields: map[string]surfaceField{},
		Values: map[string]struct{}{},
		Routes: map[string]surfaceRoute{},
	}
}

// declKey is the one spelling both halves look a declaration up by.
func declKey(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}

// overlayFile is the half this repo owns for one proto file: declaration order,
// wire numbers, prose, layout, and every option beyond field_location.
type overlayFile struct {
	// Path is repo-relative, which is how a finding names the file.
	Path string
	// Elements is the file in declaration order.
	Elements     []element
	FinalNewline bool
}

// element is one piece of a decomposed proto file. A raw span splices back
// verbatim; every other kind is re-rendered from the model.
type element interface {
	// render errors open with the file line, so the caller's path joins on.
	render(surf *surface) ([]string, error)
	// fact reports whether this element is re-rendered rather than copied.
	fact() bool
}

// rawSpan is overlay text that goes back exactly as it came.
type rawSpan struct {
	Lines []string
}

func (s rawSpan) render(_ *surface) ([]string, error) { return s.Lines, nil }

func (rawSpan) fact() bool { return false }

// blockOpen is a message, enum, oneof, or extend header.
type blockOpen struct {
	Indent string
	Kind   string
	Name   string
}

func (b blockOpen) render(_ *surface) ([]string, error) {
	return []string{b.Indent + b.Kind + " " + b.Name + " {"}, nil
}

func (blockOpen) fact() bool { return true }

// blockClose is the brace that ends one.
type blockClose struct {
	Indent string
}

func (b blockClose) render(_ *surface) ([]string, error) { return []string{b.Indent + "}"}, nil }

func (blockClose) fact() bool { return true }

// declAnchor is one field or enum value as the overlay holds it. The name is
// the join to the surface, which supplies label, type, and request position.
type declAnchor struct {
	Indent string
	Path   string
	Name   string
	Tail   fieldTail
	Number int
	// Line is the 1-based file line a refusal names.
	Line   int
	InEnum bool
}

func (*declAnchor) fact() bool { return true }

func (d *declAnchor) render(surf *surface) ([]string, error) {
	lines, err := d.declare(surf)
	if err != nil {
		return nil, fmt.Errorf("%d: %w", d.Line, err)
	}

	return lines, nil
}

func (d *declAnchor) declare(surf *surface) ([]string, error) {
	key := declKey(d.Path, d.Name)

	if d.InEnum {
		if _, ok := surf.Values[key]; !ok {
			return nil, fmt.Errorf("%w: %s", errNoSurfaceValue, key)
		}

		return d.Tail.render(d.Indent+d.Name+" = "+strconv.Itoa(d.Number), "")
	}

	field, ok := surf.Fields[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errNoSurfaceField, key)
	}

	var label string
	if field.Label != "" {
		label = field.Label + " "
	}

	head := d.Indent + label + field.Type + " " + d.Name + " = " + strconv.Itoa(d.Number)

	return d.Tail.render(head, field.Location)
}

// routeAnchor is one message's tool_route as the overlay holds it. Keeping the
// option verbatim would let an upstream route drop reach the tree unnoticed.
type routeAnchor struct {
	Indent string
	Inner  string
	// Message is the block path the surface states the route under.
	Message string
	// Tool is the name literal verbatim, quotes included; MCP naming is ours.
	Tool string
	// Line is the 1-based file line a refusal names.
	Line int
}

func (*routeAnchor) fact() bool { return true }

func (r *routeAnchor) render(surf *surface) ([]string, error) {
	route, ok := surf.Routes[r.Message]
	if !ok {
		return nil, fmt.Errorf("%d: %w: %s", r.Line, errNoSurfaceRoute, r.Message)
	}

	return []string{
		r.Indent + "option " + routeOption + " = {",
		r.Inner + "tool: " + r.Tool,
		r.Inner + "method: " + strconv.Quote(route.Method),
		r.Inner + "path: " + strconv.Quote(route.Path),
		r.Indent + "};",
	}, nil
}

// retiredField renders the two reserved statements that keep a dropped number
// and name spent, which protoc then refuses to let anything reuse.
type retiredField struct {
	Indent string
	Name   string
	Number int
}

func (retiredField) fact() bool { return true }

func (r retiredField) render(_ *surface) ([]string, error) {
	return []string{
		r.Indent + reservedKeyword + " " + strconv.Itoa(r.Number) + ";",
		r.Indent + reservedKeyword + ` "` + r.Name + `";`,
	}, nil
}

// fieldTail is everything after a declaration's number except field_location:
// this repo's options, the comments between them, and the layout.
type fieldTail struct {
	// Trailing is whatever followed the semicolon, including its leading space.
	Trailing string
	// CloseIndent is the indent the multi-line closing bracket sits at.
	CloseIndent string
	// Entries is the bracket list in order, empty when there is no list.
	Entries   []tailEntry
	Multiline bool
}

// tailEntry is one option in the bracket list. A location entry carries no text
// of its own: the surface supplies it.
type tailEntry struct {
	Indent string
	// Text is the entry verbatim from its first non-space byte, with any
	// separating comma removed. Empty for a location entry.
	Text string
	// Lead is the comment lines written above the entry, verbatim.
	Lead []string
	// Location marks the one entry rendered from the surface.
	Location bool
}

func (e tailEntry) body(location string) (string, error) {
	if !e.Location {
		return e.Text, nil
	}

	if location == "" {
		return "", errNoSurfaceLocation
	}

	return locationOption + " = " + location, nil
}

func (t fieldTail) render(head, location string) ([]string, error) {
	if len(t.Entries) == 0 {
		return []string{head + ";" + t.Trailing}, nil
	}

	if !t.Multiline {
		return t.renderInline(head, location)
	}

	return t.renderBlock(head, location)
}

func (t fieldTail) renderInline(head, location string) ([]string, error) {
	parts := make([]string, 0, len(t.Entries))

	for _, entry := range t.Entries {
		text, err := entry.body(location)
		if err != nil {
			return nil, err
		}

		parts = append(parts, text)
	}

	return []string{head + " [" + strings.Join(parts, ", ") + "];" + t.Trailing}, nil
}

func (t fieldTail) renderBlock(head, location string) ([]string, error) {
	lines := []string{head + " ["}

	for idx, entry := range t.Entries {
		text, err := entry.body(location)
		if err != nil {
			return nil, err
		}

		var separator string
		if idx < len(t.Entries)-1 {
			separator = ","
		}

		lines = append(lines, entry.Lead...)
		lines = append(lines, entry.Indent+text+separator)
	}

	return append(lines, t.CloseIndent+"];"+t.Trailing), nil
}

// render puts the file back from the two halves, so a byte match proves the
// split kept everything.
func (o *overlayFile) render(surf *surface) (string, error) {
	lines := make([]string, 0, len(o.Elements))

	for _, elem := range o.Elements {
		rendered, err := elem.render(surf)
		if err != nil {
			return "", fmt.Errorf("%s:%w", o.Path, err)
		}

		lines = append(lines, rendered...)
	}

	text := strings.Join(lines, "\n")
	if o.FinalNewline {
		text += "\n"
	}

	return text, nil
}

// counts answers re-rendered elements and verbatim spans, the gate's scan size.
func (o *overlayFile) counts() (int, int) {
	var facts, spans int

	for _, elem := range o.Elements {
		if elem.fact() {
			facts++

			continue
		}

		spans++
	}

	return facts, spans
}
