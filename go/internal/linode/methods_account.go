package linode

import (
	"context"
	"net/http"
)

const (
	endpointProfile = "/profile"
	endpointAccount = "/account"
)

// GetProfile retrieves the authenticated user's profile from the Linode API.
func (c *Client) httpGetProfile(ctx context.Context) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfile, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfile", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// GetAccount retrieves the authenticated user's account information from the Linode API.
func (c *Client) httpGetAccount(ctx context.Context) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccount, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccount", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}
