package linode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

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
	return decodeProtoElementsKeyed[T](resp, client, operation, "data", newElem)
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

// decodeProtoElementsBareOrData reads the list body from resp as either a
// top-level JSON array or a {data:[...]} page envelope, then protojson-decodes
// each element the same way decodeProtoElementsKeyed does. It buffers the body
// to dispatch on the first non-space byte for update endpoints that tolerate
// both response shapes.
func decodeProtoElementsBareOrData[T proto.Message](
	resp *http.Response,
	client *Client,
	operation string,
	newElem func() T,
) ([]T, error) {
	var body json.RawMessage

	if err := client.handleResponse(resp, &body); err != nil {
		return nil, err
	}

	rawItems, err := rawListItemsBareOrData(body, operation)
	if err != nil {
		return nil, err
	}

	return decodeRawProtoItems[T](rawItems, operation, newElem)
}

// rawListItemsBareOrData extracts raw element messages from a top-level JSON
// array or a {data:[...]} page envelope.
func rawListItemsBareOrData(body json.RawMessage, operation string) ([]json.RawMessage, error) {
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '[' {
		var rawItems []json.RawMessage
		if err := json.Unmarshal(trimmed, &rawItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s array: %w", operation, err)
		}

		return rawItems, nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s list envelope: %w", operation, err)
	}

	raw, ok := envelope["data"]
	if !ok || len(raw) == 0 {
		return nil, nil
	}

	var rawItems []json.RawMessage
	if err := json.Unmarshal(raw, &rawItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s list envelope: %w", operation, err)
	}

	return rawItems, nil
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
	var envelope map[string]json.RawMessage

	if err := client.handleResponse(resp, &envelope); err != nil {
		return nil, err
	}

	var rawItems []json.RawMessage
	if raw, ok := envelope[itemsKey]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &rawItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s list envelope: %w", operation, err)
		}
	}

	return decodeRawProtoItems[T](rawItems, operation, newElem)
}

// decodeRawProtoItems protojson-decodes each raw list element into a fresh proto
// message with DiscardUnknown. It is the shared per-element decode tail of the
// proto list fetchers (the data[] / custom-key, bare-only, and bare-or-data
// paths), so every fetcher decodes elements identically.
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
