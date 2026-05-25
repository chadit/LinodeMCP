package linode

import (
	"context"
	"net/http"
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
