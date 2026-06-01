package linode

import (
	"context"
	"net/http"
	"net/url"
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
