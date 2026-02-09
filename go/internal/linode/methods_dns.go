package linode

import (
	"context"
	"fmt"
	"net/http"
)

// ListDomains retrieves all DNS domains for the authenticated user.
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, "/domains", nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDomains", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []Domain `json:"data"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		Results int      `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetDomain retrieves a single DNS domain by its ID.
func (c *Client) GetDomain(ctx context.Context, domainID int) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// ListDomainRecords retrieves all DNS records for a specific domain.
func (c *Client) ListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDomainRecords", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response struct {
		Data    []DomainRecord `json:"data"`
		Page    int            `json:"page"`
		Pages   int            `json:"pages"`
		Results int            `json:"results"`
	}

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// CreateDomain creates a new DNS domain.
func (c *Client) CreateDomain(ctx context.Context, req CreateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, "/domains", req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// UpdateDomain updates an existing DNS domain.
func (c *Client) UpdateDomain(ctx context.Context, domainID int, req UpdateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d", domainID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// DeleteDomain deletes a DNS domain and all its records.
func (c *Client) DeleteDomain(ctx context.Context, domainID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateDomainRecord creates a new DNS record within a domain.
func (c *Client) CreateDomainRecord(ctx context.Context, domainID int, req CreateDomainRecordRequest) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records", domainID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var record DomainRecord
	if err := c.handleResponse(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// UpdateDomainRecord updates an existing DNS record.
func (c *Client) UpdateDomainRecord(ctx context.Context, domainID, recordID int, req UpdateDomainRecordRequest) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records/%d", domainID, recordID)

	resp, err := c.makeJSONRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var record DomainRecord
	if err := c.handleResponse(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// DeleteDomainRecord deletes a DNS record from a domain.
func (c *Client) DeleteDomainRecord(ctx context.Context, domainID, recordID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("/domains/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
