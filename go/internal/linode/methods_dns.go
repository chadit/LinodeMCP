package linode

import (
	"context"
	"fmt"
	"net/http"
)

const (
	endpointDomains = "/domains"
)

// ListDomains retrieves all DNS domains for the authenticated user.
func (c *Client) httpListDomains(ctx context.Context) ([]Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointDomains, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDomains", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[Domain]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetDomain retrieves a single DNS domain by its ID.
func (c *Client) httpGetDomain(ctx context.Context, domainID int) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d", domainID)

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
func (c *Client) httpListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDomainRecords", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	var response PaginatedResponse[DomainRecord]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// CreateDomain creates a new DNS domain.
func (c *Client) httpCreateDomain(ctx context.Context, req *CreateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDomains, req)
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
func (c *Client) httpUpdateDomain(ctx context.Context, domainID int, req *UpdateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
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
func (c *Client) httpDeleteDomain(ctx context.Context, domainID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDomain", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}

// CreateDomainRecord creates a new DNS record within a domain.
func (c *Client) httpCreateDomainRecord(ctx context.Context, domainID int, req *CreateDomainRecordRequest) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records", domainID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
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
func (c *Client) httpUpdateDomainRecord(ctx context.Context, domainID, recordID int, req *UpdateDomainRecordRequest) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
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
func (c *Client) httpDeleteDomainRecord(ctx context.Context, domainID, recordID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDomainRecord", Err: err}
	}

	defer func() { _ = resp.Body.Close() }()

	return c.handleResponse(resp, nil)
}
