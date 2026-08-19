package linoderoute

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// ListShape is the JSON arrangement a list route answers with.
type ListShape int

const (
	// ListShapeData is the standard Linode page, {data, page, pages, results},
	// where an absent data member reads as an empty page.
	ListShapeData ListShape = iota
	// ListShapeRequiredData is the same page with an absent data member read as
	// a malformed body instead of an empty one.
	ListShapeRequiredData
	// ListShapeKeyed puts the elements under a member of their own, named by
	// ListEnvelope.Member.
	ListShapeKeyed
	// ListShapeBare answers with a top-level JSON array and no envelope.
	ListShapeBare
	// ListShapeSingleton answers with one object that is the collection's only
	// element, which the rule-version history route does.
	ListShapeSingleton
	// ListShapeMarker answers with {data, is_truncated, next_marker} and pages
	// by that cursor rather than by number.
	ListShapeMarker
)

// ElementLift is one element member the route nests somewhere else in the same
// element body: the firewall history's version sits inside its rules object.
type ElementLift struct {
	// Member is the element member the value fills.
	Member string
	// Source is where the value comes from, a dotted path from the element's
	// own root: "rules.version".
	Source string
}

// ListEnvelope is the declared shape of one tool's list response, plus the
// member holding the elements when the shape names one and the members the
// route nests a level down.
type ListEnvelope struct {
	Member string
	Lift   []ElementLift
	Shape  ListShape
}

// ListEnvelopeFor answers the shape the named tool's route replies with.
//
// It is read here rather than passed in at each call site for the reason
// RetryDisabled documents: the fact belongs to the contract, and taking it as an
// argument would let one call site decode a route differently from another. An
// unknown tool answers the standard page, which is what nearly every collection
// sends; the route resolution that follows fails on the same name, so no request
// goes out on the strength of this answer.
func ListEnvelopeFor(tool string) ListEnvelope {
	var found ListEnvelope

	walkMessages(func(message protoreflect.MessageDescriptor) bool {
		declared, isRoute := proto.GetExtension(
			message.Options(), linodev1.E_ToolRoute,
		).(*linodev1.ToolRoute)
		if !isRoute || declared.GetTool() != tool {
			return true
		}

		envelope, ok := proto.GetExtension(
			message.Options(), linodev1.E_ListEnvelope,
		).(*linodev1.ListEnvelope)
		if ok {
			found = ListEnvelope{
				Member: envelope.GetMember(),
				Lift:   elementLifts(envelope.GetLift()),
				Shape:  listShape(envelope.GetShape()),
			}
		}

		return false
	})

	return found
}

// elementLifts reads the declared hoists in declaration order, nil when the
// envelope declares none.
func elementLifts(declared []*linodev1.ElementLift) []ElementLift {
	if len(declared) == 0 {
		return nil
	}

	lifts := make([]ElementLift, 0, len(declared))
	for _, lift := range declared {
		lifts = append(lifts, ElementLift{Member: lift.GetMember(), Source: lift.GetSource()})
	}

	return lifts
}

// listShape maps a declared shape onto the reader that decodes it. An
// unrecognized value answers the standard page for the reason ListEnvelopeFor
// documents.
func listShape(declared linodev1.ListEnvelope_Shape) ListShape {
	switch declared {
	case linodev1.ListEnvelope_SHAPE_REQUIRED_DATA:
		return ListShapeRequiredData
	case linodev1.ListEnvelope_SHAPE_KEYED:
		return ListShapeKeyed
	case linodev1.ListEnvelope_SHAPE_BARE:
		return ListShapeBare
	case linodev1.ListEnvelope_SHAPE_SINGLETON:
		return ListShapeSingleton
	case linodev1.ListEnvelope_SHAPE_MARKER:
		return ListShapeMarker
	case linodev1.ListEnvelope_SHAPE_UNSPECIFIED, linodev1.ListEnvelope_SHAPE_DATA:
	}

	return ListShapeData
}
