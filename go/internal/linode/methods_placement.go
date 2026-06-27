package linode

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
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

// AssignPlacementGroupLinodes assigns Linodes to a placement group.
func (c *Client) httpAssignPlacementGroupLinodes(ctx context.Context, groupID int, req *AssignPlacementGroupLinodesRequest) (*PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID)) + "/assign"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AssignPlacementGroupLinodes", Err: err}
	}

	defer drainClose(resp)

	var group PlacementGroup
	if err := c.handleResponse(resp, &group); err != nil {
		return nil, err
	}

	return &group, nil
}

// httpAssignPlacementGroupLinodesProto assigns Linodes to a placement group and
// decodes the response as a proto message.
func (c *Client) httpAssignPlacementGroupLinodesProto(ctx context.Context, groupID int, req *AssignPlacementGroupLinodesRequest) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID)) + "/assign"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "AssignPlacementGroupLinodes", Err: err}
	}

	defer drainClose(resp)

	group := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, group); err != nil {
		return nil, err
	}

	return group, nil
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

// httpGetPlacementGroupProto retrieves one placement group as a proto message.
func (c *Client) httpGetPlacementGroupProto(ctx context.Context, groupID int) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointPlacementGroups+"/%d", groupID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetPlacementGroup", Err: err}
	}

	defer drainClose(resp)

	group := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, group); err != nil {
		return nil, err
	}

	return group, nil
}

// CreatePlacementGroup creates a Linode placement group.
func (c *Client) httpCreatePlacementGroup(ctx context.Context, req *CreatePlacementGroupRequest) (*PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointPlacementGroups, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreatePlacementGroup", Err: err}
	}

	defer drainClose(resp)

	var placementGroup PlacementGroup
	if err := c.handleResponse(resp, &placementGroup); err != nil {
		return nil, err
	}

	return &placementGroup, nil
}

// httpCreatePlacementGroupProto creates a placement group as a proto message.
func (c *Client) httpCreatePlacementGroupProto(ctx context.Context, req *CreatePlacementGroupRequest) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointPlacementGroups, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreatePlacementGroup", Err: err}
	}

	defer drainClose(resp)

	placementGroup := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, placementGroup); err != nil {
		return nil, err
	}

	return placementGroup, nil
}

// httpUpdatePlacementGroupProto updates a placement group as a proto message.
func (c *Client) httpUpdatePlacementGroupProto(ctx context.Context, groupID int, request *UpdatePlacementGroupRequest) (*linodev1.PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdatePlacementGroup", Err: err}
	}

	defer drainClose(resp)

	group := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, group); err != nil {
		return nil, err
	}

	return group, nil
}

// httpUpdatePlacementGroup updates a placement group label by ID.
func (c *Client) httpUpdatePlacementGroup(ctx context.Context, groupID int, request *UpdatePlacementGroupRequest) (*PlacementGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, request)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdatePlacementGroup", Err: err}
	}

	defer drainClose(resp)

	var group PlacementGroup
	if err := c.handleResponse(resp, &group); err != nil {
		return nil, err
	}

	return &group, nil
}

// DeletePlacementGroup deletes a placement group by ID.
func (c *Client) httpDeletePlacementGroup(ctx context.Context, groupID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeletePlacementGroup", Err: err}
	}

	defer drainClose(resp)

	if err := c.handleResponse(resp, nil); err != nil {
		return err
	}

	return nil
}

// UnassignPlacementGroup removes Linodes from a placement group.
func (c *Client) httpUnassignPlacementGroup(ctx context.Context, groupID int, req *PlacementGroupUnassignRequest) (*PlacementGroup, error) {
	if req == nil || len(req.Linodes) == 0 {
		return nil, ErrPlacementGroupUnassignLinodesRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID)) + "/unassign"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UnassignPlacementGroup", Err: err}
	}

	defer drainClose(resp)

	var placementGroup PlacementGroup
	if err := c.handleResponse(resp, &placementGroup); err != nil {
		return nil, err
	}

	return &placementGroup, nil
}

// httpUnassignPlacementGroupProto removes Linodes from a placement group and
// decodes the response as a proto message.
func (c *Client) httpUnassignPlacementGroupProto(ctx context.Context, groupID int, req *PlacementGroupUnassignRequest) (*linodev1.PlacementGroup, error) {
	if req == nil || len(req.Linodes) == 0 {
		return nil, ErrPlacementGroupUnassignLinodesRequired
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointPlacementGroups + "/" + url.PathEscape(strconv.Itoa(groupID)) + "/unassign"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UnassignPlacementGroup", Err: err}
	}

	defer drainClose(resp)

	placementGroup := &linodev1.PlacementGroup{}
	if err := c.handleProtoResponse(resp, placementGroup); err != nil {
		return nil, err
	}

	return placementGroup, nil
}
