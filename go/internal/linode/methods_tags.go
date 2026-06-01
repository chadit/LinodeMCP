package linode

import (
	"context"
	"net/http"
	"net/url"
)

const endpointTags = "/tags"

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
