package linode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// listEnvelopeDataKey is the member the standard Linode page envelope wraps its
// elements under.
const listEnvelopeDataKey = "data"

// listProtoElements fetches a paginated list endpoint and protojson-decodes each
// data[] element into a fresh proto message. This is the shared decode path for
// every proto-backed list tool: it decodes the {data:[...]} envelope, then
// protojson-decodes each element with DiscardUnknown so the output matches the
// Go proto read path and the Python serializer element-for-element.
//
// newElem returns a fresh, empty element message (e.g. func() *linodev1.Domain {
// return &linodev1.Domain{} }); operation names the call for error wrapping.
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
// page/page_size query params. It builds the request URL with withPaginationQuery
// (the same helper the non-proto list methods use, so the runtime request matches
// the existing httpListX exactly), then decodes the {data:[...]} envelope the same
// way listProtoElements does.
//
// Sub-resource paginated lists (e.g. /linode/instances/{linode_id}/configs with
// page/page_size) reuse this helper directly: the caller formats the path id into
// the endpoint string before calling, exactly like the existing httpListX, so this
// helper just adds pagination to an already-path-formatted endpoint. There is no
// separate listProtoElementsSubresourcePaginated because it would be byte-for-byte
// identical to this function.
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
// tail reads an absent or null data as an empty page, so a malformed body comes
// back as a confident "nothing here"; on a security surface such as the trusted
// device list that reads as "no remembered browser sessions". The Python client
// already fails closed on those bodies, so endpoints that cannot afford the
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
// elements under a key other than "data". The current Interfaces generation
// endpoint /linode/instances/{id}/interfaces returns {"interfaces":[...]} rather
// than the usual {"data":[...]} page envelope, so this fetcher reads itemsKey
// instead. The decode tail (DiscardUnknown protojson per element) is shared with
// the data[] path via decodeProtoElementsKeyed.
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

// decodeProtoElements reads the {data:[...]} list envelope from resp and
// protojson-decodes each element into a fresh proto message with DiscardUnknown,
// matching the Go proto read path and the Python serializer element-for-element.
// It is the shared decode tail of the proto list fetchers.
func decodeProtoElements[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	newElem func() T,
) ([]T, error) {
	return decodeProtoElementsKeyed[T](resp, client, operation, listEnvelopeDataKey, newElem)
}

// decodeProtoElementsRequiredData is decodeProtoElements for endpoints that must
// see a data member. An absent or null data is a malformed body rather than an
// empty page, so it fails instead of decoding zero elements. See
// listProtoElementsPaginatedRequiredData for why some endpoints need that.
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

	// errResponseBodyNotJSONArray is the closest existing sentinel: the data
	// member did not carry a JSON array because it was absent or null. A
	// sentinel naming that precisely belongs in errors.go.
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
// protojson-decodes each element into a fresh proto message with DiscardUnknown.
// itemsKey is "data" for the standard page envelope and "interfaces" for the
// current Interfaces generation endpoint. It is the shared decode tail of the
// proto list fetchers.
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

// readListEnvelopeItems reads resp as a list envelope and returns the raw
// elements stored under itemsKey, still undecoded. A nil result means the
// envelope parsed but carried no such member (absent or null), which the
// callers read differently: the lenient tail treats it as an empty page and the
// required-data tail rejects it.
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
// message with DiscardUnknown. It is the shared per-element decode tail of the
// proto list fetchers (the data[] / custom-key and bare-only paths), so every
// fetcher decodes elements identically.
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
