package linode

import (
	"context"

	"google.golang.org/protobuf/proto"
)

// These primitives are the contract-driven twins of the hand-written methods in
// methods_*.go: they take the tool name and response message as arguments, so
// cmd/toolgen can emit a call for any tool the contract declares without a
// matching method existing first. Route resolution, decoding, and retry are
// unchanged. The retry loop's operation label is the tool name because a
// generated call site has no verb-and-noun like "GetDomain" to read, and
// inventing one would put a fact in the generator that nothing else can check.

// CallProtoRoute performs the route the named tool declares and decodes the
// response into msg. pathValues fill the route template's slots in declared
// order.
//
// msg is reset per attempt because protojson merges into a message rather than
// replacing it, so a retried attempt over a partly-filled msg would blend two
// responses. Hand-written methods avoid that by allocating inside the attempt,
// which a caller-supplied message rules out.
func (c *Client) CallProtoRoute(ctx context.Context, tool string, pathValues []any, msg proto.Message) error {
	return c.executeWithRetry(ctx, tool, func() error {
		proto.Reset(msg)

		attemptCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		resp, err := c.makeRouteRequest(attemptCtx, tool, nil, pathValues...)
		if err != nil {
			return wrapRequestError(tool, err)
		}

		defer drainClose(resp)

		return c.handleProtoResponse(resp, msg)
	})
}

// ListProtoRoute fetches one page of the collection the named tool declares and
// decodes each element into a fresh proto message. pathValues fill the route
// template's slots, so a sub-resource collection (/domains/{domain_id}/records)
// uses the same primitive as a top-level one.
//
// It is a function rather than a method because Go does not allow a method to
// introduce the type parameter the element type needs.
func ListProtoRoute[T proto.Message](
	ctx context.Context,
	client *Client,
	tool string,
	pathValues []any,
	page, pageSize int,
	newElem func() T,
) ([]T, error) {
	var items []T

	err := client.executeWithRetry(ctx, tool, func() error {
		var listErr error

		items, listErr = listProtoElementsPaginatedRouted(
			ctx, client, tool, tool, "", pathValues, page, pageSize, newElem,
		)

		return listErr
	})

	return items, err
}
