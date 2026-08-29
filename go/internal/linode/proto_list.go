package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// listEnvelopeDataKey is the member the standard Linode page envelope wraps its
// elements under.
const listEnvelopeDataKey = "data"

// The cursor members a marker-paged body carries beside its elements.
const (
	markerTruncatedKey = "is_truncated"
	markerNextKey      = "next_marker"
)

// ListMarkerPage is the cursor a marker-paged route answers beside its
// elements: whether objects remain past this page, and the marker a caller
// resumes from. The standard page envelope reports neither, which is why the
// marker shape carries its own.
type ListMarkerPage struct {
	NextMarker  string
	IsTruncated bool
}

// noPage and noPageSize tell a paginated fetcher there are no page controls
// left to append. The routed twins below merge the controls into the query
// themselves because withPaginationQuery joins with "?", which would leave two
// separators in a URL that already carries filters of its own.
const (
	noPage     = 0
	noPageSize = 0
)

// fetchList performs the GET behind every list fetcher and hands the response
// to decode.
//
// Sharing the request is what keeps one hand-built call site behind the whole
// list surface: the endpoint each caller passes was resolved from the route
// contract already, and repeating the request around each decoder only
// multiplied the places a timeout or a close could go missing.
func fetchList[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	decode func(resp *http.Response) ([]T, error),
) ([]T, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	return decode(resp)
}

// listProtoElementsPaginated is listProtoElements for endpoints that take
// page/page_size query params. It builds the URL with withPaginationQuery, the
// same helper the non-proto list methods use, so the runtime request matches
// the existing httpListX exactly.
//
// Sub-resource lists (e.g. /linode/instances/{linode_id}/configs) reuse this
// helper directly by formatting the path id into the endpoint first, so there
// is no separate sub-resource variant to keep in sync.
func listProtoElementsPaginated[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	page, pageSize int,
	newElem func() T,
) ([]T, error) {
	return fetchList(ctx, client, operation,
		withPaginationQuery(endpoint, page, pageSize),
		func(resp *http.Response) ([]T, error) {
			return decodeProtoElements[T](resp, client, operation, newElem)
		})
}

// listProtoElementsPaginatedRequiredData is listProtoElementsPaginated for
// endpoints whose page envelope must carry a data member. The lenient decode
// tail reads an absent or null data as an empty page, so a malformed trusted
// device response would read as "no remembered browser sessions". The Python
// client fails closed on those bodies, so endpoints that cannot afford the
// wrong answer decode through here and both clients report the same failure.
func listProtoElementsPaginatedRequiredData[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	page, pageSize int,
	newElem func() T,
) ([]T, error) {
	return fetchList(ctx, client, operation,
		withPaginationQuery(endpoint, page, pageSize),
		func(resp *http.Response) ([]T, error) {
			return decodeProtoElementsRequiredData[T](resp, client, operation, newElem)
		})
}

// listProtoElementsKeyed is listProtoElements for endpoints that wrap their
// elements under a key other than "data", such as the current Interfaces
// generation endpoint /linode/instances/{id}/interfaces returning
// {"interfaces":[...]}.
func listProtoElementsKeyed[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint, itemsKey string,
	newElem func() T,
) ([]T, error) {
	return fetchList(ctx, client, operation, endpoint, func(resp *http.Response) ([]T, error) {
		return decodeProtoElementsKeyed[T](resp, client, operation, itemsKey, newElem)
	})
}

// listProtoElementsBare fetches endpoints whose response body is a top-level
// JSON array rather than the standard {data:[...]} page envelope.
func listProtoElementsBare[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	newElem func() T,
) ([]T, error) {
	return fetchList(ctx, client, operation, endpoint, func(resp *http.Response) ([]T, error) {
		return decodeProtoElementsBare[T](resp, client, operation, newElem)
	})
}

// listProtoElementsSingleton reads a route whose whole answer is the one
// element the collection holds, and reports it as a page of one.
//
// The body is held to being a JSON object before it is decoded. protojson with
// DiscardUnknown reads an array or a null as an empty message, so a route that
// answered something else would come back as one blank element rather than as
// the failure it is.
func listProtoElementsSingleton[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	lifts []linoderoute.ElementLift,
	newElem func() T,
) ([]T, error) {
	return fetchList(ctx, client, operation, endpoint, func(resp *http.Response) ([]T, error) {
		return decodeProtoElementSingleton[T](resp, client, operation, lifts, newElem)
	})
}

// liftElementMembers fills the members a route nests a level down, after the
// element itself is decoded.
//
// The value is applied by decoding a one-member patch through the same
// descriptor and merging it, so a lifted member fills exactly the way the API's
// own top-level members do, for every element type and every field kind. The
// patch is built from the declared member name, which is a proto field name, so
// the only thing that can be malformed in it is the value the route nested.
//
// A source the body does not carry leaves the member alone, so the decode's own
// answer stands.
func liftElementMembers[T proto.Message](
	item T, fields map[string]json.RawMessage, operation string, lifts []linoderoute.ElementLift,
) error {
	for _, lift := range lifts {
		value, found := nestedValue(fields, strings.Split(lift.Source, "."))
		if !found {
			continue
		}

		patch := item.ProtoReflect().New().Interface()

		member := []byte(`{"` + lift.Member + `":` + string(value) + `}`)
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(member, patch); err != nil {
			return fmt.Errorf("failed to unmarshal %s member %s: %w", operation, lift.Member, err)
		}

		proto.Merge(item, patch)
	}

	return nil
}

// nestedValue walks a dotted path through an element body, answering the raw
// value and whether the path led anywhere. A member that is not an object holds
// no path to walk, which reads as not found rather than as a failure: the lift
// leaves the target alone and the decode's own answer stands.
func nestedValue(fields map[string]json.RawMessage, path []string) (json.RawMessage, bool) {
	value, present := fields[path[0]]
	if !present || len(path) == 1 {
		return value, present
	}

	// A member that is not an object leaves nested nil, and a read of a nil map
	// misses, which is the answer a path that leads nowhere already has.
	var nested map[string]json.RawMessage

	_ = json.Unmarshal(value, &nested)

	return nestedValue(nested, path[1:])
}

// decodeProtoElementSingleton reads one object as the page's only element.
//
// The body is held to being a JSON object before it is decoded. protojson with
// DiscardUnknown reads an array or a null as an empty message, so a route that
// answered something else would come back as one blank element rather than as
// the failure it is.
func decodeProtoElementSingleton[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	lifts []linoderoute.ElementLift,
	newElem func() T,
) ([]T, error) {
	var body json.RawMessage
	if err := client.handleResponse(resp, &body); err != nil {
		return nil, err
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil || probe == nil {
		return nil, fmt.Errorf("failed to unmarshal %s object: %w", operation, errResponseBodyNotJSONObject)
	}

	items, err := decodeRawProtoItems[T]([]json.RawMessage{body}, operation, newElem)
	if err != nil {
		return nil, err
	}

	if liftErr := liftElementMembers(items[0], probe, operation, lifts); liftErr != nil {
		return nil, liftErr
	}

	return items, nil
}

// routedList resolves the path a tool declares, attaches the query the call
// site composed, and hands the finished endpoint to the fetcher that reads the
// response. Every routed twin below goes through here, so a failure is
// classified once: a route the contract cannot resolve or fill never reached
// the network, so it comes back in the argument class rather than the transport
// class the fetchers report, because a second attempt would build the same
// unsendable request. The method the route declares is dropped rather than
// checked, since every fetcher below sends GET as its own constant.
func routedList[T proto.Message](
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	fetch func(surfaced *Client, endpoint string) ([]T, error),
) ([]T, error) {
	_, endpoint, segment, err := routedRequest(tool, rawQuery, values)
	if err != nil {
		return nil, wrapRequestError(operation, err)
	}

	// The fetchers below reach the wire through the client they are handed, so
	// the surface is applied here rather than inside each of them.
	return fetch(client.onSurface(segment), endpoint)
}

// pageQuery renders the page controls on their own, through the same encoder
// the hand-built list methods use, so a routed request spells them the same way
// as the call sites that have not moved yet.
func pageQuery(page, pageSize int) string {
	return strings.TrimPrefix(withPaginationQuery("", page, pageSize), "?")
}

// mergeQuery joins two already-encoded query strings, either of which may be
// empty.
func mergeQuery(left, right string) string {
	parts := make([]string, 0, 2)

	for _, part := range []string{left, right} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, "&")
}

// listProtoElementsPaginatedRouted is listProtoElementsPaginated for a call
// site that names its tool.
func listProtoElementsPaginatedRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	page, pageSize int,
	newElem func() T,
) ([]T, error) {
	query := mergeQuery(rawQuery, pageQuery(page, pageSize))

	return routedList(client, operation, tool, query, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return listProtoElementsPaginated(ctx, surfaced, operation, endpoint, noPage, noPageSize, newElem)
	})
}

// listProtoElementsPaginatedRequiredDataRouted is
// listProtoElementsPaginatedRequiredData for a call site that names its tool.
func listProtoElementsPaginatedRequiredDataRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	page, pageSize int,
	newElem func() T,
) ([]T, error) {
	query := mergeQuery(rawQuery, pageQuery(page, pageSize))

	return routedList(client, operation, tool, query, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return listProtoElementsPaginatedRequiredData(
			ctx, surfaced, operation, endpoint, noPage, noPageSize, newElem,
		)
	})
}

// listProtoElementsKeyedRouted is listProtoElementsKeyed for a call site that
// names its tool.
func listProtoElementsKeyedRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery, itemsKey string,
	values []any,
	newElem func() T,
) ([]T, error) {
	return routedList(client, operation, tool, rawQuery, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return listProtoElementsKeyed(ctx, surfaced, operation, endpoint, itemsKey, newElem)
	})
}

// listProtoElementsMemberRaw fetches one page and answers its elements beside
// the bodies they decoded from, so a caller can write back what the decode
// dropped. required tells an absent member from an empty page, the split
// decodeProtoElementsRequiredData makes for the same two readings.
func listProtoElementsMemberRaw[T proto.Message](
	ctx context.Context,
	client *Client,
	tool, rawQuery, itemsKey string,
	values []any,
	required bool,
	newElem func() T,
) ([]T, []json.RawMessage, error) {
	var raws []json.RawMessage

	items, err := routedList(client, tool, tool, rawQuery, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return fetchList(ctx, surfaced, tool, endpoint, func(resp *http.Response) ([]T, error) {
			rawItems, readErr := readListEnvelopeItems(resp, surfaced, tool, itemsKey)
			if readErr != nil {
				return nil, readErr
			}

			if rawItems == nil && required {
				return nil, missingListMember(tool, itemsKey)
			}

			raws = rawItems

			return decodeRawProtoItems[T](rawItems, tool, newElem)
		})
	})

	return items, raws, err
}

// listProtoElementsBareRouted is listProtoElementsBare for a call site that
// names its tool.
func listProtoElementsBareRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	newElem func() T,
) ([]T, error) {
	return routedList(client, operation, tool, rawQuery, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return listProtoElementsBare(ctx, surfaced, operation, endpoint, newElem)
	})
}

// listProtoElementsSingletonRouted is listProtoElementsSingleton for a call site
// that names its tool.
func listProtoElementsSingletonRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	lifts []linoderoute.ElementLift,
	newElem func() T,
) ([]T, error) {
	return routedList(client, operation, tool, rawQuery, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return listProtoElementsSingleton(ctx, surfaced, operation, endpoint, lifts, newElem)
	})
}

// decodeProtoElements reads the {data:[...]} list envelope from resp and
// decodes it through decodeProtoElementsKeyed.
func decodeProtoElements[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	newElem func() T,
) ([]T, error) {
	return decodeProtoElementsKeyed[T](resp, client, operation, listEnvelopeDataKey, newElem)
}

// decodeProtoElementsRequiredData is decodeProtoElements for endpoints that must
// see a data member: absent or null is a malformed body rather than an empty
// page. See listProtoElementsPaginatedRequiredData for why some endpoints need
// that.
func decodeProtoElementsRequiredData[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	newElem func() T,
) ([]T, error) {
	rawItems, err := readListEnvelopeItems(resp, client, operation, listEnvelopeDataKey)
	if err != nil {
		return nil, err
	}

	if rawItems == nil {
		return nil, missingListMember(operation, listEnvelopeDataKey)
	}

	return decodeRawProtoItems[T](rawItems, operation, newElem)
}

// missingListMember reports the envelope member a page that must carry one did
// not send.
//
// Closest existing sentinel: the member carried no JSON array because it was
// absent or null. A sentinel naming that precisely belongs in errors.go.
func missingListMember(operation, member string) error {
	return fmt.Errorf(
		"failed to unmarshal %s list envelope: %s member is missing or null: %w",
		operation,
		member,
		errResponseBodyNotJSONArray,
	)
}

// decodeProtoElementsBare reads a top-level JSON array from resp, then
// protojson-decodes each element the same way decodeProtoElementsKeyed does.
func decodeProtoElementsBare[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	newElem func() T,
) ([]T, error) {
	rawItems := []json.RawMessage{}

	if err := client.handleResponse(resp, &rawItems); err != nil {
		return nil, err
	}

	if rawItems == nil {
		return nil, fmt.Errorf("failed to unmarshal %s array: %w", operation, errResponseBodyNotJSONArray)
	}

	return decodeRawProtoItems[T](rawItems, operation, newElem)
}

// decodeProtoElementsKeyed reads the list envelope from resp under itemsKey and
// decodes each element. itemsKey is "data" for the standard page envelope and
// "interfaces" for the current Interfaces generation endpoint.
func decodeProtoElementsKeyed[T proto.Message](
	resp *http.Response,
	client *Client,
	operation, itemsKey string,
	newElem func() T,
) ([]T, error) {
	rawItems, err := readListEnvelopeItems(resp, client, operation, itemsKey)
	if err != nil {
		return nil, err
	}

	return decodeRawProtoItems[T](rawItems, operation, newElem)
}

// readListEnvelopeItems reads resp as a list envelope and returns the still
// undecoded elements stored under itemsKey. A nil result means the envelope
// parsed but carried no such member (absent or null), which the callers read
// differently: the lenient tail treats it as an empty page, the required-data
// tail rejects it.
func readListEnvelopeItems(
	resp *http.Response,
	client *Client,
	operation, itemsKey string,
) ([]json.RawMessage, error) {
	envelope, err := readListEnvelopeMembers(resp, client, operation)
	if err != nil {
		return nil, err
	}

	return listEnvelopeItems(envelope, operation, itemsKey)
}

// readListEnvelopeMembers reads resp as a list envelope and returns its members
// still undecoded. The marker shape needs the cursor beside the elements, and
// the body can only be read once.
func readListEnvelopeMembers(
	resp *http.Response,
	client *Client,
	operation string,
) (map[string]json.RawMessage, error) {
	var envelope map[string]json.RawMessage

	if err := client.handleResponse(resp, &envelope); err != nil {
		return nil, err
	}

	if envelope == nil {
		return nil, fmt.Errorf(
			"failed to unmarshal %s list envelope: %w",
			operation,
			errResponseBodyNotJSONObject,
		)
	}

	return envelope, nil
}

// listEnvelopeItems pulls the still undecoded elements out of an already read
// envelope. A nil result means no such member, which readListEnvelopeItems
// documents the two readings of.
func listEnvelopeItems(
	envelope map[string]json.RawMessage,
	operation, itemsKey string,
) ([]json.RawMessage, error) {
	var rawItems []json.RawMessage

	if raw, ok := envelope[itemsKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s list envelope: %w", operation, err)
		}
	}

	return rawItems, nil
}

// listProtoElementsMarkerRouted reads a marker-paged collection: the elements
// under data, plus the cursor the caller resumes from. The cursor travels back
// out of the decode rather than being read from a second request, because the
// body carries both and can only be read once.
func listProtoElementsMarkerRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	newElem func() T,
) ([]T, ListMarkerPage, error) {
	var cursor ListMarkerPage

	items, err := routedList(client, operation, tool, rawQuery, values, func(surfaced *Client, endpoint string) ([]T, error) {
		return fetchList(ctx, surfaced, operation, endpoint, func(resp *http.Response) ([]T, error) {
			decoded, page, decodeErr := decodeProtoElementsMarker(resp, surfaced, operation, newElem)
			cursor = page

			return decoded, decodeErr
		})
	})

	return items, cursor, err
}

// decodeProtoElementsMarker reads {data, is_truncated, next_marker} and decodes
// the elements the same way every other shape does.
func decodeProtoElementsMarker[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	newElem func() T,
) ([]T, ListMarkerPage, error) {
	envelope, err := readListEnvelopeMembers(resp, client, operation)
	if err != nil {
		return nil, ListMarkerPage{}, err
	}

	rawItems, err := listEnvelopeItems(envelope, operation, listEnvelopeDataKey)
	if err != nil {
		return nil, ListMarkerPage{}, err
	}

	cursor, err := readMarkerCursor(envelope, operation)
	if err != nil {
		return nil, ListMarkerPage{}, err
	}

	items, err := decodeRawProtoItems[T](rawItems, operation, newElem)
	if err != nil {
		return nil, ListMarkerPage{}, err
	}

	return items, cursor, nil
}

// readMarkerCursor decodes the two cursor members. An absent member is the page
// that is not truncated, which is what the route sends on a complete listing.
func readMarkerCursor(
	envelope map[string]json.RawMessage, operation string,
) (ListMarkerPage, error) {
	var cursor ListMarkerPage

	if raw, ok := envelope[markerTruncatedKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &cursor.IsTruncated); err != nil {
			return ListMarkerPage{}, fmt.Errorf(
				"failed to unmarshal %s list envelope: %w", operation, err,
			)
		}
	}

	if raw, ok := envelope[markerNextKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &cursor.NextMarker); err != nil {
			return ListMarkerPage{}, fmt.Errorf(
				"failed to unmarshal %s list envelope: %w", operation, err,
			)
		}
	}

	return cursor, nil
}

// decodeRawProtoItems protojson-decodes each raw list element into a fresh proto
// message with DiscardUnknown, so the output matches the Go proto read path and
// the Python serializer element-for-element. Every fetcher above ends here, by
// the data[], custom-key, or bare path, so all of them decode elements the same
// way.
func decodeRawProtoItems[T proto.Message](
	rawItems []json.RawMessage,
	operation string,
	newElem func() T,
) ([]T, error) {
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	elems := make([]T, 0, len(rawItems))

	for _, raw := range rawItems {
		elem := newElem()
		if err := opts.Unmarshal(raw, elem); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s element: %w", operation, err)
		}

		elems = append(elems, elem)
	}

	return elems, nil
}
