package linode

import (
	"context"
	"net/http"
	"net/url"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const endpointTags = "/tags"

// httpListTags retrieves tags visible to the authenticated account.
func (c *Client) httpListTags(ctx context.Context, page, pageSize int) (*PaginatedResponse[Tag], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointTags, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListTags", Err: err}
	}

	defer drainClose(resp)

	var tags PaginatedResponse[Tag]
	if err := c.handleResponse(resp, &tags); err != nil {
		return nil, err
	}

	return &tags, nil
}

// httpListTagsProto retrieves tags as proto messages for the proto-backed list
// path. The page/page_size pair flows through withPaginationQuery, so the
// request matches httpListTags.
func (c *Client) httpListTagsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Tag, error) {
	return listProtoElementsPaginated(ctx, c, "ListTags", endpointTags, page, pageSize,
		func() *linodev1.Tag { return &linodev1.Tag{} })
}

// httpListTaggedObjects retrieves objects that have the supplied tag label.
func (c *Client) httpListTaggedObjects(ctx context.Context, tagLabel string, page, pageSize int) (*PaginatedResponse[TaggedObject], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointTags+"/"+url.PathEscape(tagLabel), page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListTaggedObjects", Err: err}
	}

	defer drainClose(resp)

	var taggedObjects PaginatedResponse[TaggedObject]
	if err := c.handleResponse(resp, &taggedObjects); err != nil {
		return nil, err
	}

	return &taggedObjects, nil
}

// httpListTaggedObjectsProto retrieves tagged objects as proto messages for the
// proto-backed list path. The tag label is path-escaped and the page/page_size
// pair flows through withPaginationQuery, so the request matches
// httpListTaggedObjects.
func (c *Client) httpListTaggedObjectsProto(ctx context.Context, tagLabel string, page, pageSize int) ([]*linodev1.TaggedObject, error) {
	return listProtoElementsPaginated(ctx, c, "ListTaggedObjects", endpointTags+"/"+url.PathEscape(tagLabel), page, pageSize,
		func() *linodev1.TaggedObject { return &linodev1.TaggedObject{} })
}

// httpCreateTag creates a tag and optionally applies it to existing resources.
func (c *Client) httpCreateTag(ctx context.Context, req *CreateTagRequest) (*Tag, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointTags, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateTag", Err: err}
	}

	defer drainClose(resp)

	var tag Tag
	if err := c.handleResponse(resp, &tag); err != nil {
		return nil, err
	}

	return &tag, nil
}

// httpCreateTagProto creates a tag and decodes the response as a proto message.
func (c *Client) httpCreateTagProto(ctx context.Context, req *CreateTagRequest) (*linodev1.Tag, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointTags, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateTag", Err: err}
	}

	defer drainClose(resp)

	tag := &linodev1.Tag{}
	if err := c.handleProtoResponse(resp, tag); err != nil {
		return nil, err
	}

	return tag, nil
}

// httpDeleteTag deletes the supplied tag label from all objects on the account.
func (c *Client) httpDeleteTag(ctx context.Context, tagLabel string) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointTags + "/" + url.PathEscape(tagLabel)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteTag", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
