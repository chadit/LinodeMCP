package linode

import (
	"context"
	"fmt"
	"net/http"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
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

	defer drainClose(resp)

	var response PaginatedResponse[Domain]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListDomainsProto retrieves all DNS domains as proto messages, decoded
// directly from the API JSON for the proto-backed read path.
func (c *Client) httpListDomainsProto(ctx context.Context) ([]*linodev1.Domain, error) {
	return listProtoElements(ctx, c, "ListDomains", endpointDomains,
		func() *linodev1.Domain { return &linodev1.Domain{} })
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

	defer drainClose(resp)

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

	defer drainClose(resp)

	var response PaginatedResponse[DomainRecord]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListDomainRecordsProto retrieves a domain's DNS records as proto messages
// for the proto-backed list path. The endpoint is formatted with the same
// fmt.Sprintf(endpointDomains+"/%d/records", domainID) pattern
// httpListDomainRecords uses, so the runtime path matches exactly.
func (c *Client) httpListDomainRecordsProto(ctx context.Context, domainID int) ([]*linodev1.DomainRecord, error) {
	endpoint := fmt.Sprintf(endpointDomains+"/%d/records", domainID)

	return listProtoElements(ctx, c, "ListDomainRecords", endpoint,
		func() *linodev1.DomainRecord { return &linodev1.DomainRecord{} })
}

// GetDomainZoneFile retrieves the rendered zone file for a domain.
func (c *Client) httpGetDomainZoneFile(ctx context.Context, domainID int) (*DomainZoneFile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/zone-file", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomainZoneFile", Err: err}
	}

	defer drainClose(resp)

	var zoneFile DomainZoneFile
	if err := c.handleResponse(resp, &zoneFile); err != nil {
		return nil, err
	}

	return &zoneFile, nil
}

// httpGetDomainZoneFileProto retrieves a domain's rendered zone file as a proto
// message.
func (c *Client) httpGetDomainZoneFileProto(ctx context.Context, domainID int) (*linodev1.DomainZoneFile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/zone-file", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomainZoneFile", Err: err}
	}

	defer drainClose(resp)

	zoneFile := &linodev1.DomainZoneFile{}
	if err := c.handleProtoResponse(resp, zoneFile); err != nil {
		return nil, err
	}

	return zoneFile, nil
}

// ImportDomain imports a domain zone from a remote nameserver.
func (c *Client) httpImportDomain(ctx context.Context, req *ImportDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDomains+"/import", req)
	if err != nil {
		return nil, &NetworkError{Operation: "ImportDomain", Err: err}
	}

	defer drainClose(resp)

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// httpImportDomainProto imports a domain and decodes the response as a proto message.
func (c *Client) httpImportDomainProto(ctx context.Context, req *ImportDomainRequest) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDomains+"/import", req)
	if err != nil {
		return nil, &NetworkError{Operation: "ImportDomain", Err: err}
	}

	defer drainClose(resp)

	domain := &linodev1.Domain{}
	if err := c.handleProtoResponse(resp, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// httpCloneDomainProto clones a domain and decodes the response as a proto message.
func (c *Client) httpCloneDomainProto(ctx context.Context, domainID int, req *CloneDomainRequest) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/clone", domainID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneDomain", Err: err}
	}

	defer drainClose(resp)

	domain := &linodev1.Domain{}
	if err := c.handleProtoResponse(resp, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// CloneDomain clones a DNS domain and its records.
func (c *Client) httpCloneDomain(ctx context.Context, domainID int, req *CloneDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/clone", domainID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CloneDomain", Err: err}
	}

	defer drainClose(resp)

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// CreateDomain creates a new DNS domain.
func (c *Client) httpCreateDomain(ctx context.Context, req *CreateDomainRequest) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDomains, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomain", Err: err}
	}

	defer drainClose(resp)

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

	defer drainClose(resp)

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// httpGetDomainProto retrieves a domain as a proto message.
func (c *Client) httpGetDomainProto(ctx context.Context, domainID int) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomain", Err: err}
	}

	defer drainClose(resp)

	domain := &linodev1.Domain{}
	if err := c.handleProtoResponse(resp, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// httpCreateDomainProto creates a domain as a proto message.
func (c *Client) httpCreateDomainProto(ctx context.Context, req *CreateDomainRequest) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDomains, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomain", Err: err}
	}

	defer drainClose(resp)

	domain := &linodev1.Domain{}
	if err := c.handleProtoResponse(resp, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// httpUpdateDomainProto updates a domain as a proto message.
func (c *Client) httpUpdateDomainProto(ctx context.Context, domainID int, req *UpdateDomainRequest) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d", domainID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDomain", Err: err}
	}

	defer drainClose(resp)

	domain := &linodev1.Domain{}
	if err := c.handleProtoResponse(resp, domain); err != nil {
		return nil, err
	}

	return domain, nil
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

	defer drainClose(resp)

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

	defer drainClose(resp)

	var record DomainRecord
	if err := c.handleResponse(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// GetDomainRecord retrieves a single DNS record by ID within a domain.
func (c *Client) httpGetDomainRecord(ctx context.Context, domainID, recordID int) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomainRecord", Err: err}
	}

	defer drainClose(resp)

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

	defer drainClose(resp)

	var record DomainRecord
	if err := c.handleResponse(resp, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// httpGetDomainRecordProto retrieves a domain record as a proto message.
func (c *Client) httpGetDomainRecordProto(ctx context.Context, domainID, recordID int) (*linodev1.DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDomainRecord", Err: err}
	}

	defer drainClose(resp)

	record := &linodev1.DomainRecord{}
	if err := c.handleProtoResponse(resp, record); err != nil {
		return nil, err
	}

	return record, nil
}

// httpCreateDomainRecordProto creates a domain record as a proto message.
func (c *Client) httpCreateDomainRecordProto(ctx context.Context, domainID int, req *CreateDomainRecordRequest) (*linodev1.DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records", domainID)

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDomainRecord", Err: err}
	}

	defer drainClose(resp)

	record := &linodev1.DomainRecord{}
	if err := c.handleProtoResponse(resp, record); err != nil {
		return nil, err
	}

	return record, nil
}

// httpUpdateDomainRecordProto updates a domain record as a proto message.
func (c *Client) httpUpdateDomainRecordProto(ctx context.Context, domainID, recordID int, req *UpdateDomainRecordRequest) (*linodev1.DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := fmt.Sprintf(endpointDomains+"/%d/records/%d", domainID, recordID)

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDomainRecord", Err: err}
	}

	defer drainClose(resp)

	record := &linodev1.DomainRecord{}
	if err := c.handleProtoResponse(resp, record); err != nil {
		return nil, err
	}

	return record, nil
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

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
