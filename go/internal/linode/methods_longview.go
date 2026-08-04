package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpGetLongviewPlan retrieves the account Longview subscription plan.
func (c *Client) httpGetLongviewPlan(ctx context.Context) (*LongviewSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_plan_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetLongviewPlan", err)
	}

	defer drainClose(resp)

	var plan LongviewSubscription
	if err := c.handleResponse(resp, &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

// httpGetLongviewPlanProto retrieves the account Longview subscription plan and
// decodes it into the proto LongviewSubscription element for the proto-backed
// read path.
func (c *Client) httpGetLongviewPlanProto(ctx context.Context) (*linodev1.LongviewSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_plan_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetLongviewPlan", err)
	}

	defer drainClose(resp)

	plan := &linodev1.LongviewSubscription{}
	if err := c.handleProtoResponse(resp, plan); err != nil {
		return nil, err
	}

	return plan, nil
}

// httpListLongviewTypesProto retrieves the available Longview subscription types
// as proto messages for the proto-backed list path. The /longview/types endpoint
// returns a {data,page,...} page envelope, so listProtoElements reads data.
func (c *Client) httpListLongviewTypesProto(ctx context.Context) ([]*linodev1.LongviewType, error) {
	return listProtoElementsRouted(ctx, c, "ListLongviewTypes",
		"linode_longview_type_list", "", nil,
		func() *linodev1.LongviewType { return &linodev1.LongviewType{} })
}

// httpListLongviewSubscriptionsProto retrieves available Longview subscription
// plans as proto messages for the proto-backed list path. The page/page_size
// pair flows through withPaginationQuery, so the request matches
// httpListLongviewSubscriptions.
func (c *Client) httpListLongviewSubscriptionsProto(ctx context.Context, page, pageSize int) ([]*linodev1.LongviewSubscription, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListLongviewSubscriptions",
		"linode_longview_subscription_list", "", nil, page, pageSize,
		func() *linodev1.LongviewSubscription { return &linodev1.LongviewSubscription{} })
}

// CreateLongviewClientProto creates a Longview client and returns the proto
// CreatedLongviewClient element (carries the one-time install secret).
func (c *Client) CreateLongviewClientProto(ctx context.Context, req *CreateLongviewClientRequest) (*linodev1.CreatedLongviewClient, error) {
	return c.httpCreateLongviewClientProto(ctx, req)
}

// httpCreateLongviewClientProto creates a Longview client and decodes the
// response into the proto CreatedLongviewClient element.
func (c *Client) httpCreateLongviewClientProto(ctx context.Context, req *CreateLongviewClientRequest) (*linodev1.CreatedLongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_client_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateLongviewClient", err)
	}

	defer drainClose(resp)

	client := &linodev1.CreatedLongviewClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpGetLongviewClient retrieves one Longview client by ID.
func (c *Client) httpGetLongviewClient(ctx context.Context, clientID string) (*LongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_client_get", nil, clientID)
	if err != nil {
		return nil, wrapRequestError("GetLongviewClient", err)
	}

	defer drainClose(resp)

	var client LongviewClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// httpGetLongviewClientProto retrieves a Longview client as a proto message.
func (c *Client) httpGetLongviewClientProto(ctx context.Context, clientID string) (*linodev1.LongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_client_get", nil, clientID)
	if err != nil {
		return nil, wrapRequestError("GetLongviewClient", err)
	}

	defer drainClose(resp)

	client := &linodev1.LongviewClient{}
	if err := c.handleProtoResponse(resp, client); err != nil {
		return nil, err
	}

	return client, nil
}

// httpGetLongviewSubscriptionProto retrieves a Longview subscription as a proto
// message.
func (c *Client) httpGetLongviewSubscriptionProto(ctx context.Context, subscriptionID string) (*linodev1.LongviewSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_subscription_get", nil, subscriptionID)
	if err != nil {
		return nil, wrapRequestError("GetLongviewSubscription", err)
	}

	defer drainClose(resp)

	subscription := &linodev1.LongviewSubscription{}
	if err := c.handleProtoResponse(resp, subscription); err != nil {
		return nil, err
	}

	return subscription, nil
}

// UpdateLongviewPlanProto updates the account Longview subscription plan and
// returns the proto LongviewSubscription element.
func (c *Client) UpdateLongviewPlanProto(ctx context.Context, req *UpdateLongviewPlanRequest) (*linodev1.LongviewSubscription, error) {
	return c.httpUpdateLongviewPlanProto(ctx, req)
}

// httpUpdateLongviewPlanProto updates the account Longview subscription plan and
// decodes the response into the proto LongviewSubscription element.
func (c *Client) httpUpdateLongviewPlanProto(ctx context.Context, req *UpdateLongviewPlanRequest) (*linodev1.LongviewSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_longview_plan_update", req)
	if err != nil {
		return nil, wrapRequestError("UpdateLongviewPlan", err)
	}

	defer drainClose(resp)

	plan := &linodev1.LongviewSubscription{}
	if err := c.handleProtoResponse(resp, plan); err != nil {
		return nil, err
	}

	return plan, nil
}
