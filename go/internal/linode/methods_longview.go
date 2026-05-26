package linode

import (
	"context"
	"net/http"
	"net/url"
)

// httpGetLongviewPlan retrieves the account Longview subscription plan.
func (c *Client) httpGetLongviewPlan(ctx context.Context) (*LongviewSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointLongviewPlan, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLongviewPlan", Err: err}
	}

	defer drainClose(resp)

	var plan LongviewSubscription
	if err := c.handleResponse(resp, &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

// httpListLongviewTypes retrieves the available Longview subscription types.
func (c *Client) httpListLongviewTypes(ctx context.Context) (*PaginatedResponse[LongviewType], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointLongviewTypes, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLongviewTypes", Err: err}
	}

	defer drainClose(resp)

	var types PaginatedResponse[LongviewType]
	if err := c.handleResponse(resp, &types); err != nil {
		return nil, err
	}

	return &types, nil
}

// httpListLongviewSubscriptions retrieves available Longview subscription plans.
func (c *Client) httpListLongviewSubscriptions(ctx context.Context, page, pageSize int) (*PaginatedResponse[LongviewSubscription], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointLongviewSubscriptions, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListLongviewSubscriptions", Err: err}
	}

	defer drainClose(resp)

	var subscriptions PaginatedResponse[LongviewSubscription]
	if err := c.handleResponse(resp, &subscriptions); err != nil {
		return nil, err
	}

	return &subscriptions, nil
}

// httpCreateLongviewClient creates a Longview client.
func (c *Client) httpCreateLongviewClient(ctx context.Context, req *CreateLongviewClientRequest) (*CreatedLongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointLongviewClients, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateLongviewClient", Err: err}
	}

	defer drainClose(resp)

	var client CreatedLongviewClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// httpGetLongviewClient retrieves one Longview client by ID.
func (c *Client) httpGetLongviewClient(ctx context.Context, clientID string) (*LongviewClient, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointLongviewClients + "/" + url.PathEscape(clientID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetLongviewClient", Err: err}
	}

	defer drainClose(resp)

	var client LongviewClient
	if err := c.handleResponse(resp, &client); err != nil {
		return nil, err
	}

	return &client, nil
}

// httpUpdateLongviewPlan updates the account Longview subscription plan.
func (c *Client) httpUpdateLongviewPlan(ctx context.Context, req *UpdateLongviewPlanRequest) (*LongviewSubscription, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointLongviewPlan, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateLongviewPlan", Err: err}
	}

	defer drainClose(resp)

	var plan LongviewSubscription
	if err := c.handleResponse(resp, &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}
