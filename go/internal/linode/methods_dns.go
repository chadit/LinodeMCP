package linode

import (
	"context"
)

// httpGetDomain retrieves a single DNS domain by its ID.
func (c *Client) httpGetDomain(ctx context.Context, domainID int) (*Domain, error) {
	return routedGet[Domain](ctx, c, "GetDomain", "linode_domain_get", domainID)
}

// httpListDomainRecords retrieves one page of a domain's DNS records.
func (c *Client) httpListDomainRecords(ctx context.Context, domainID int) ([]DomainRecord, error) {
	return listData(routedGet[PaginatedResponse[DomainRecord]](ctx, c, "ListDomainRecords", "linode_domain_record_list", domainID))
}

// httpGetDomainRecord retrieves a single DNS record by ID within a domain.
func (c *Client) httpGetDomainRecord(ctx context.Context, domainID, recordID int) (*DomainRecord, error) {
	return routedGet[DomainRecord](ctx, c, "GetDomainRecord", "linode_domain_record_get", domainID, recordID)
}
