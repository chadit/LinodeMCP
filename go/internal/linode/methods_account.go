package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

const (
	endpointProfile             = "/profile"
	endpointProfileGrants       = "/profile/grants"
	endpointAccount             = "/account"
	endpointAccountAgreements   = "/account/agreements"
	endpointAccountAvailability = "/account/availability"
)

// GetProfile retrieves the authenticated user's profile from the Linode API.
func (c *Client) httpGetProfile(ctx context.Context) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfile, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfile", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// httpGetProfileGrants retrieves /profile/grants. Returns a Grants
// struct for OAuth tokens; PATs return an empty payload by design (the
// Linode API still returns 200 with zero-valued fields). Callers
// distinguish PAT vs OAuth by checking Profile.Scopes != "" first; this
// method does not need to know which token type the caller has.
func (c *Client) httpGetProfileGrants(ctx context.Context) (*Grants, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointProfileGrants, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetProfileGrants", Err: err}
	}

	defer drainClose(resp)

	var grants Grants
	if err := c.handleResponse(resp, &grants); err != nil {
		return nil, err
	}

	return &grants, nil
}

// GetAccount retrieves the authenticated user's account information from the Linode API.
func (c *Client) httpGetAccount(ctx context.Context) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccount, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// httpGetAccountAgreements retrieves account agreement acknowledgment status.
func (c *Client) httpGetAccountAgreements(ctx context.Context) (*AccountAgreements, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointAccountAgreements, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetAccountAgreements", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var agreements AccountAgreements
	if err := c.handleResponse(resp, &agreements); err != nil {
		return nil, err
	}

	return &agreements, nil
}

// httpListAccountAvailability retrieves account service availability by region.
func (c *Client) httpListAccountAvailability(ctx context.Context, page, pageSize int) (*PaginatedResponse[AccountAvailability], error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointAccountAvailability
	query := url.Values{}

	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}

	if pageSize > 0 {
		query.Set("page_size", strconv.Itoa(pageSize))
	}

	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAccountAvailability", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var availability PaginatedResponse[AccountAvailability]
	if err := c.handleResponse(resp, &availability); err != nil {
		return nil, err
	}

	return &availability, nil
}

// httpAcknowledgeAccountAgreements acknowledges account agreements via POST /v4/account/agreements.
func (c *Client) httpAcknowledgeAccountAgreements(ctx context.Context, req *AcknowledgeAccountAgreementsRequest) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointAccountAgreements, req)
	if err != nil {
		return &NetworkError{Operation: "AcknowledgeAccountAgreements", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	return c.handleResponse(resp, nil)
}

// httpUpdateAccount updates account billing/contact fields via PUT /v4/account.
func (c *Client) httpUpdateAccount(ctx context.Context, req *UpdateAccountRequest) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointAccount, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateAccount", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var account Account
	if err := c.handleResponse(resp, &account); err != nil {
		return nil, err
	}

	return &account, nil
}

// httpUpdateProfile updates the authenticated user's profile via PUT /v4/profile.
func (c *Client) httpUpdateProfile(ctx context.Context, req *UpdateProfileRequest) (*Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPut, endpointProfile, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateProfile", Err: err}
	}

	defer drainClose(resp) // errcheck: body close is best-effort; all account methods use this pattern

	var profile Profile
	if err := c.handleResponse(resp, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}
