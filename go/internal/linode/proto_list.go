package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// listEnvelopeDataKey is the member the standard Linode page envelope wraps its
// elements under.
const listEnvelopeDataKey = "data"

// noPage and noPageSize tell a paginated fetcher there are no page controls
// left to append. The routed twins below merge the controls into the query
// themselves because withPaginationQuery joins with "?", which would leave two
// separators in a URL that already carries filters of its own.
const (
	noPage     = 0
	noPageSize = 0
)

// listProtoElements fetches a list endpoint and decodes each element of its
// {data:[...]} envelope into a fresh proto message. newElem returns an empty
// element message (e.g. func() *linodev1.Domain { return &linodev1.Domain{} });
// operation names the call for error wrapping.
func listProtoElements[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	newElem func() T,
) ([]T, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	return decodeProtoElements[T](resp, client, operation, newElem)
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
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.makeRequest(ctx, http.MethodGet, withPaginationQuery(endpoint, page, pageSize), nil)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	return decodeProtoElements[T](resp, client, operation, newElem)
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
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.makeRequest(ctx, http.MethodGet, withPaginationQuery(endpoint, page, pageSize), nil)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	return decodeProtoElementsRequiredData[T](resp, client, operation, newElem)
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
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	return decodeProtoElementsKeyed[T](resp, client, operation, itemsKey, newElem)
}

// listProtoElementsBare fetches endpoints whose response body is a top-level
// JSON array rather than the standard {data:[...]} page envelope.
func listProtoElementsBare[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, endpoint string,
	newElem func() T,
) ([]T, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := client.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	return decodeProtoElementsBare[T](resp, client, operation, newElem)
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
	operation, tool, rawQuery string,
	values []any,
	fetch func(endpoint string) ([]T, error),
) ([]T, error) {
	_, endpoint, err := routedRequest(tool, rawQuery, values)
	if err != nil {
		return nil, wrapRequestError(operation, err)
	}

	return fetch(endpoint)
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

// listProtoElementsRouted is listProtoElements for a call site whose path comes
// from the proto contract instead of a string it assembled. tool names the
// route, values fill its path slots in declared order, and rawQuery carries
// whatever filters the site composed, already encoded. Every Routed twin below
// takes those same three in place of an endpoint.
func listProtoElementsRouted[T proto.Message](
	ctx context.Context,
	client *Client,
	operation, tool, rawQuery string,
	values []any,
	newElem func() T,
) ([]T, error) {
	return routedList(operation, tool, rawQuery, values, func(endpoint string) ([]T, error) {
		return listProtoElements(ctx, client, operation, endpoint, newElem)
	})
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

	return routedList(operation, tool, query, values, func(endpoint string) ([]T, error) {
		return listProtoElementsPaginated(ctx, client, operation, endpoint, noPage, noPageSize, newElem)
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

	return routedList(operation, tool, query, values, func(endpoint string) ([]T, error) {
		return listProtoElementsPaginatedRequiredData(
			ctx, client, operation, endpoint, noPage, noPageSize, newElem,
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
	return routedList(operation, tool, rawQuery, values, func(endpoint string) ([]T, error) {
		return listProtoElementsKeyed(ctx, client, operation, endpoint, itemsKey, newElem)
	})
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
	return routedList(operation, tool, rawQuery, values, func(endpoint string) ([]T, error) {
		return listProtoElementsBare(ctx, client, operation, endpoint, newElem)
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

	// Closest existing sentinel: the data member carried no JSON array because
	// it was absent or null. A sentinel naming that precisely belongs in
	// errors.go.
	if rawItems == nil {
		return nil, fmt.Errorf(
			"failed to unmarshal %s list envelope: %s member is missing or null: %w",
			operation,
			listEnvelopeDataKey,
			errResponseBodyNotJSONArray,
		)
	}

	return decodeRawProtoItems[T](rawItems, operation, newElem)
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

	var rawItems []json.RawMessage
	if raw, ok := envelope[itemsKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s list envelope: %w", operation, err)
		}
	}

	return rawItems, nil
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
