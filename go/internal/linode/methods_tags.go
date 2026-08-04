package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Each method below calls one Linode tag endpoint through its tool's declared
// proto route, so paths live in the proto rather than here, and the *Proto
// variants decode into generated proto elements. Tag labels reach the route
// builder raw: it escapes each value into a single path segment, so a label
// containing a slash cannot address a different route.

// httpListTagsProto retrieves the account's tags.
func (c *Client) httpListTagsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Tag, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListTags",
		"linode_tag_list", "", nil, page, pageSize,
		func() *linodev1.Tag { return &linodev1.Tag{} })
}

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

// httpListTaggedObjectsProto is httpListTaggedObjects for the proto path.
func (c *Client) httpListTaggedObjectsProto(ctx context.Context, tagLabel string, page, pageSize int) ([]*linodev1.TaggedObject, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListTaggedObjects",
		"linode_tag_object_list", "", []any{tagLabel}, page, pageSize,
		func() *linodev1.TaggedObject { return &linodev1.TaggedObject{} })
}

// httpCreateTagProto creates a tag and applies it to the objects named in req.
func (c *Client) httpCreateTagProto(ctx context.Context, req *CreateTagRequest) (*linodev1.Tag, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_tag_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateTag", err)
	}

	defer drainClose(resp)

	tag := &linodev1.Tag{}
	if err := c.handleProtoResponse(resp, tag); err != nil {
		return nil, err
	}

	return tag, nil
}

// httpDeleteTag deletes the supplied tag label from all objects on the account.
//
// The route comes from the proto contract rather than from a path and a verb
// written here, and the label goes to the builder raw: it escapes each value
// into one path segment, so a tag containing a slash cannot address a different
// route and neither client has to remember to encode.
func (c *Client) httpDeleteTag(ctx context.Context, tagLabel string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_tag_delete", nil, tagLabel)
	if err != nil {
		return wrapRequestError("DeleteTag", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
