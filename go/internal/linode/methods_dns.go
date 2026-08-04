package linode

import (
	"context"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// httpListDomainsProto retrieves one page of DNS domains as proto messages.
func (c *Client) httpListDomainsProto(ctx context.Context, page, pageSize int) ([]*linodev1.Domain, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListDomains",
		"linode_domain_list", "", nil, page, pageSize,
		func() *linodev1.Domain { return &linodev1.Domain{} })
}

// httpGetDomain retrieves a single DNS domain by its ID.
func (c *Client) httpGetDomain(ctx context.Context, domainID int) (*Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_domain_get", nil, domainID)
	if err != nil {
		return nil, wrapRequestError("GetDomain", err)
	}

	defer drainClose(resp)

	var domain Domain
	if err := c.handleResponse(resp, &domain); err != nil {
		return nil, err
	}

	return &domain, nil
}

// httpListDomainRecords retrieves one page of a domain's DNS records.
func (c *Client) httpListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_domain_record_list", nil, domainID)
	if err != nil {
		return nil, wrapRequestError("ListDomainRecords", err)
	}

	defer drainClose(resp)

	var response PaginatedResponse[DomainRecord]

	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListDomainRecordsProto is the proto-backed httpListDomainRecords. Both
// name the same tool, so both resolve the one declared route.
func (c *Client) httpListDomainRecordsProto(ctx context.Context, domainID, page, pageSize int) ([]*linodev1.DomainRecord, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListDomainRecords",
		"linode_domain_record_list", "", []any{domainID}, page, pageSize,
		func() *linodev1.DomainRecord { return &linodev1.DomainRecord{} })
}

// httpGetDomainZoneFileProto retrieves a domain's rendered zone file as a proto
// message.
func (c *Client) httpGetDomainZoneFileProto(ctx context.Context, domainID int) (*linodev1.DomainZoneFile, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_domain_zone_file_get", nil, domainID)
	if err != nil {
		return nil, wrapRequestError("GetDomainZoneFile", err)
	}

	defer drainClose(resp)

	zoneFile := &linodev1.DomainZoneFile{}
	if err := c.handleProtoResponse(resp, zoneFile); err != nil {
		return nil, err
	}

	return zoneFile, nil
}

// httpImportDomainProto imports a domain and decodes the response as a proto message.
func (c *Client) httpImportDomainProto(ctx context.Context, req *ImportDomainRequest) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_domain_import", req)
	if err != nil {
		return nil, wrapRequestError("ImportDomain", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_clone", req, domainID)
	if err != nil {
		return nil, wrapRequestError("CloneDomain", err)
	}

	defer drainClose(resp)

	domain := &linodev1.Domain{}
	if err := c.handleProtoResponse(resp, domain); err != nil {
		return nil, err
	}

	return domain, nil
}

// httpGetDomainProto retrieves a domain as a proto message.
func (c *Client) httpGetDomainProto(ctx context.Context, domainID int) (*linodev1.Domain, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_domain_get", nil, domainID)
	if err != nil {
		return nil, wrapRequestError("GetDomain", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateDomain", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_update", req, domainID)
	if err != nil {
		return nil, wrapRequestError("UpdateDomain", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_delete", nil, domainID)
	if err != nil {
		return wrapRequestError("DeleteDomain", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// GetDomainRecord retrieves a single DNS record by ID within a domain.
func (c *Client) httpGetDomainRecord(ctx context.Context, domainID, recordID int) (*DomainRecord, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_domain_record_get", nil, domainID, recordID)
	if err != nil {
		return nil, wrapRequestError("GetDomainRecord", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_record_get", nil, domainID, recordID)
	if err != nil {
		return nil, wrapRequestError("GetDomainRecord", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_record_create", req, domainID)
	if err != nil {
		return nil, wrapRequestError("CreateDomainRecord", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_record_update", req, domainID, recordID)
	if err != nil {
		return nil, wrapRequestError("UpdateDomainRecord", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_domain_record_delete", nil, domainID, recordID)
	if err != nil {
		return wrapRequestError("DeleteDomainRecord", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}
