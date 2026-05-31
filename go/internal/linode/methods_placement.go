package linode

import (
	"context"
	"fmt"
	"net/http"
)

const endpointPlacementGroups = "/placement/groups"

// ListPlacementGroups retrieves placement groups for the authenticated account.
func (c *Client) httpListPlacementGroups(ctx context.Context, page, pageSize int) (*PaginatedResponse[PlacementGroup], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointPlacementGroups, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListPlacementGroups", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[PlacementGroup]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// GetPlacementGroup retrieves a single placement group by ID.
func (c *Client) httpGetPlacementGroup(ctx context.Context, groupID int) (*PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointPlacementGroups+"/%d", groupID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetPlacementGroup", Err: err}
	}

	defer drainClose(resp)

	var group PlacementGroup
	if err := c.handleResponse(resp, &group); err != nil {
		return nil, err
	}

	return &group, nil
}
