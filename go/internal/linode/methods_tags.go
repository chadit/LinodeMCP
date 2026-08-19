package linode

import (
	"context"
)

// Each method below calls one Linode tag endpoint through its tool's declared
// proto route, so paths live in the proto rather than here, and the *Proto
// variants decode into generated proto elements. Tag labels reach the route
// builder raw: it escapes each value into a single path segment, so a label
// containing a slash cannot address a different route.

// httpListTaggedObjects retrieves objects that have the supplied tag label.
func (c *Client) httpListTaggedObjects(ctx context.Context, tagLabel string, page, pageSize int) (*PaginatedResponse[TaggedObject], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_tag_object_list", pageQuery(page, pageSize), nil, tagLabel)
	if err != nil {
		return nil, wrapRequestError("ListTaggedObjects", err)
	}

	defer drainClose(resp)

	var taggedObjects PaginatedResponse[TaggedObject]
	if err := c.handleResponse(resp, &taggedObjects); err != nil {
		return nil, err
	}

	return &taggedObjects, nil
}
