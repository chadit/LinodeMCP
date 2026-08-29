package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// The three declared state forms beyond a single-resource read: a scan of a
// whole collection, a page envelope kept as the state, and a composite
// assembled from more than one call. Each mirrors a Python driver of the same
// name, and each answers the DeclaredState shape every declared fetch does.

// FetchCollectionScan pages the whole named collection and answers the first
// element the match selects, projected the way a single-resource fetch is.
// Unlike FetchCollectionElement's one-page read, a scan pages to the end: the
// resource it looks for is named by field values rather than an id, so there
// is no page its position can be derived from. wanted words the pairs for the
// not-found sentence.
func FetchCollectionScan[T proto.Message](
	ctx context.Context,
	client *linode.Client,
	tool string,
	values []any,
	newElem func() T,
	match func(T) bool,
	wanted string,
) (any, error) {
	for page := collectionStatePage; ; page++ {
		items, raws, err := linode.ListProtoRouteRaw(
			ctx, client, tool, values, "", page, standardPageSizeMax, newElem,
		)

		found, state, err := scannedElement(items, raws, err, match)
		if err != nil || found {
			return state, err
		}

		if len(items) < standardPageSizeMax {
			return nil, fmt.Errorf("%w: %s carries no element matching %s",
				ErrCollectionElement, tool, wanted)
		}
	}
}

// scannedElement picks the match out of one fetched page, or hands the fetch's
// own failure back so the route's sentence survives unwrapped.
func scannedElement[T proto.Message](
	items []T, raws []json.RawMessage, err error, match func(T) bool,
) (bool, any, error) {
	if err != nil {
		return false, nil, err
	}

	for index, item := range items {
		if match(item) {
			state, projectErr := ProjectDeclaredState(raws[index], item)

			return true, state, projectErr
		}
	}

	return false, nil, nil
}

// FetchEnvelopeState reads the named list once and answers the page envelope
// itself as the state: the projected elements under "data" and the API's total
// under "results". The total rides along because a truncated first page must
// not understate what the caller is about to touch.
//
// query is the read's own page controls, empty for a read taken at the route's
// defaults. A replacement previews the page its own call publishes, so the
// state and the answer describe one page rather than two.
func FetchEnvelopeState[T proto.Message](
	ctx context.Context,
	client *linode.Client,
	tool string,
	values []any,
	query string,
	newElem func() T,
) (any, error) {
	// The envelope decodes into a free-form Struct so the shared status and
	// shape handling applies; the members are read from the raw bytes below.
	var envelope structpb.Struct

	body, err := client.CallProtoRouteQueryRaw(ctx, tool, values, query, &envelope)

	return envelopeState(body, err, tool, newElem)
}

// envelopeState projects one raw page envelope: each data element through the
// element's own message, and the results total as the number the API spelled.
// It takes the fetch's own results the way scannedElement does, so a failed
// read reports the route's own sentence rather than one wrapped around it.
func envelopeState[T proto.Message](
	body json.RawMessage, err error, tool string, newElem func() T,
) (any, error) {
	if err != nil {
		return nil, err
	}

	var members map[string]json.RawMessage

	// The shared response handler proved the body is a JSON object before it
	// handed the bytes back, so this decode cannot fail.
	_ = json.Unmarshal(body, &members)

	var data []json.RawMessage
	if raw, carried := members["data"]; carried {
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrResponseDecode, err)
		}
	}

	var results json.Number
	if raw, carried := members["results"]; carried {
		if err := json.Unmarshal(raw, &results); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrResponseDecode, err)
		}
	}

	elements := make([]any, 0, len(data))

	for _, raw := range data {
		element := newElem()

		// The prune runs first so a non-object element reads as this decode
		// failure; the projection still reads the original bytes, nulls and
		// all.
		pruned, pruneErr := withoutNulls(raw)
		if pruneErr != nil {
			return nil, fmt.Errorf("%s: %w: %w", tool, ErrResponseDecode, pruneErr)
		}

		projected, err := ProjectDeclaredState(raw, element)
		if err != nil {
			return nil, err
		}

		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(pruned, element); err != nil {
			return nil, fmt.Errorf("%s: %w", tool, err)
		}

		elements = append(elements, projected)
	}

	return DeclaredState{"data": elements, "results": results}, nil
}

// withoutNulls prunes null-valued members ahead of the contract decode. The
// API spells an empty slot as null (a config's unused device), which the
// projection reports as sent while protojson refuses it on a typed map
// member; Python's ParseDict tolerates the same body. An element that is not
// an object errors, which the caller words as its decode failure.
func withoutNulls(raw json.RawMessage) (json.RawMessage, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, fmt.Errorf("prune nulls: %w", err)
	}

	pruned := make(map[string]json.RawMessage, len(members))

	for name, member := range members {
		if kept, keep := prunedMember(member); keep {
			pruned[name] = kept
		}
	}

	return rebuiltObject(pruned), nil
}

// prunedMember keeps one member, recursing into an object value; a null
// reads as absent, the way the projection's consumers treat it.
func prunedMember(member json.RawMessage) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(member)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, false
	}

	if len(trimmed) > 0 && trimmed[0] == '{' {
		if inner, err := withoutNulls(trimmed); err == nil {
			return inner, true
		}
	}

	return member, true
}

// rebuiltObject joins pruned members back into one JSON object. Keys marshal
// through the string form, which cannot fail; values re-enter verbatim.
func rebuiltObject(members map[string]json.RawMessage) json.RawMessage {
	built := make(json.RawMessage, 0, 64)
	built = append(built, '{')

	first := true

	for name, member := range members {
		if !first {
			built = append(built, ',')
		}

		first = false
		built = appendJSONString(built, name)
		built = append(built, ':')
		built = append(built, member...)
	}

	return append(built, '}')
}

// appendJSONString quotes one member name. Escaping the quote, the
// backslash, and the control range is the whole set JSON requires; the rest
// of a decoded name re-enters as the UTF-8 it already was.
func appendJSONString(built json.RawMessage, name string) json.RawMessage {
	// controlEnd is the first byte JSON allows unescaped.
	const controlEnd = 0x20

	const hexDigits = "0123456789abcdef"

	built = append(built, '"')

	for index := range len(name) {
		char := name[index]

		switch {
		case char == '"' || char == '\\':
			built = append(built, '\\', char)
		case char < controlEnd:
			// The shifts split one byte into its hex halves: >>4 is the
			// high digit, &0x0f masks the low one.
			built = append(built, '\\', 'u', '0', '0', hexDigits[char>>4], hexDigits[char&0x0f])
		default:
			built = append(built, char)
		}
	}

	return append(built, '"')
}

// CompositeCall is one read of a composite state: the declared GET it
// performs, the member its kept fields land under, and the fields kept.
type CompositeCall struct {
	// New builds the message the call's answer decodes through: the read's
	// resource for a single read, the page element for a list.
	New func() proto.Message
	// Tool is the declared read the call routes through.
	Tool string
	// Member is the projected state member this call's answer becomes.
	Member string
	// Fields is the subset kept; everything else is dropped before the state
	// is reported or hashed.
	Fields []string
	// Values fill the read's path slots, in slot order.
	Values []any
	// List is true when the read answers a page and the member is an array of
	// kept-field objects rather than one object.
	List bool
}

// FetchCompositeState performs each call in declaration order and assembles
// the projected state, one member per call. A failed call fails the fetch
// whole: a plan hashed over half a state would refuse an apply for a change
// nobody made.
func FetchCompositeState(
	ctx context.Context, client *linode.Client, calls []CompositeCall,
) (any, error) {
	state := DeclaredState{}

	for index := range calls {
		member, err := calls[index].fetch(ctx, client)
		if err != nil {
			return nil, err
		}

		state[calls[index].Member] = member
	}

	return state, nil
}

// fetch is one composite call's read, projected and cut to its kept fields.
func (call *CompositeCall) fetch(ctx context.Context, client *linode.Client) (any, error) {
	if call.List {
		return call.fetchList(ctx, client)
	}

	message := call.New()

	raw, err := client.CallProtoRouteState(ctx, call.Tool, call.Values, "", message)

	return keptResource(raw, err, message, call.Fields)
}

// keptResource projects one read's answer and cuts it to the kept fields,
// taking the read's own results so its failure sentence survives unwrapped.
func keptResource(
	raw json.RawMessage, err error, message proto.Message, fields []string,
) (any, error) {
	if err != nil {
		return nil, err
	}

	return keepFields(contractedProjection(raw, message), fields), nil
}

// contractedProjection projects a body its primitive already decoded against
// the contract, so the projection has nothing left to refuse; the impossible
// failure reads as an empty state rather than a second error path.
func contractedProjection(raw json.RawMessage, msg proto.Message) DeclaredState {
	projected, _ := ProjectDeclaredState(raw, msg)
	state, _ := projected.(DeclaredState)

	return state
}

// fetchList is one composite call over a paged read, kept fields per element.
func (call *CompositeCall) fetchList(ctx context.Context, client *linode.Client) (any, error) {
	kept := make([]any, 0)

	for page := collectionStatePage; ; page++ {
		items, raws, err := linode.ListProtoRouteRaw(
			ctx, client, call.Tool, call.Values, "", page, standardPageSizeMax,
			call.New,
		)

		kept, err = keptPage(kept, items, raws, err, call.Fields)
		if err != nil {
			return nil, err
		}

		if len(items) < standardPageSizeMax {
			return kept, nil
		}
	}
}

// keptPage appends one fetched page's elements, each projected and cut to the
// kept fields, taking the fetch's own results the way keptResource does.
func keptPage[T proto.Message](
	kept []any, items []T, raws []json.RawMessage, err error, fields []string,
) ([]any, error) {
	if err != nil {
		return nil, err
	}

	for index := range items {
		kept = append(kept, keepFields(contractedProjection(raws[index], items[index]), fields))
	}

	return kept, nil
}

// keepFields cuts a projected state to the declared subset, which is what
// keeps a cosmetic field from refusing an apply.
func keepFields(projected any, fields []string) DeclaredState {
	// The projection always answers DeclaredState; a nil map reads safely.
	state, _ := projected.(DeclaredState)

	kept := DeclaredState{}

	for _, name := range fields {
		if value, carried := state[name]; carried {
			kept[name] = value
		}
	}

	return kept
}
