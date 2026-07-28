package linode

// Domain represents a Linode DNS domain.
type Domain struct {
	Created     string   `json:"created"`
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`   // master, slave
	Status      string   `json:"status"` // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email"`
	Description string   `json:"description"`
	Group       string   `json:"group"`
	Updated     string   `json:"updated"`
	AXFRIPs     []string `json:"axfr_ips"`
	Tags        []string `json:"tags"`
	MasterIPs   []string `json:"master_ips"`
	ExpireSec   int      `json:"expire_sec"`
	RefreshSec  int      `json:"refresh_sec"`
	TTLSec      int      `json:"ttl_sec"`
	ID          int      `json:"id"`
	RetrySec    int      `json:"retry_sec"`
}

// DomainZoneFile represents the rendered zone file lines for a domain.
type DomainZoneFile struct {
	ZoneFile []string `json:"zone_file"`
}

// DomainRecord represents a DNS record within a domain.
type DomainRecord struct {
	Protocol string `json:"protocol"`
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name"`
	Target   string `json:"target"`
	Updated  string `json:"updated"`
	Created  string `json:"created"`
	Tag      string `json:"tag"`
	Service  string `json:"service"`
	Port     int    `json:"port"`
	TTLSec   int    `json:"ttl_sec"`
	ID       int    `json:"id"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority"`
}

// ImportDomainRequest represents the request body for importing a domain zone.
type ImportDomainRequest struct {
	Domain           string `json:"domain"`
	RemoteNameserver string `json:"remote_nameserver"`
}

// CloneDomainRequest represents the request body for cloning a domain.
type CloneDomainRequest struct {
	Domain string `json:"domain"`
}

// CreateDomainRequest represents the request body for creating a domain.
//
// Every optional field is a pointer so an explicitly supplied zero value
// ("" description, 0 retry_sec, [] tags) still reaches the API. Value fields
// with omitempty could not tell "caller asked for the documented default"
// apart from "caller said nothing", which silently dropped explicit zeroes
// and made the outgoing body diverge from the other language's.
type CreateDomainRequest struct {
	AXFRIPs     *[]string `json:"axfr_ips,omitempty"`
	SOAEmail    *string   `json:"soa_email,omitempty"`
	Description *string   `json:"description,omitempty"`
	RetrySec    *int      `json:"retry_sec,omitempty"`
	MasterIPs   *[]string `json:"master_ips,omitempty"`
	ExpireSec   *int      `json:"expire_sec,omitempty"`
	RefreshSec  *int      `json:"refresh_sec,omitempty"`
	TTLSec      *int      `json:"ttl_sec,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Group       *string   `json:"group,omitempty"`
	Status      *string   `json:"status,omitempty"` // active, disabled
	Type        string    `json:"type"`             // master, slave
	Domain      string    `json:"domain"`
}

// UpdateDomainRequest represents the request body for updating a domain.
type UpdateDomainRequest struct {
	Domain      string   `json:"domain,omitempty"`
	Status      string   `json:"status,omitempty"` // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email,omitempty"`
	Description string   `json:"description,omitempty"`
	Group       string   `json:"group,omitempty"`
	MasterIPs   []string `json:"master_ips,omitempty"`
	AXFRIPs     []string `json:"axfr_ips,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	RetrySec    int      `json:"retry_sec,omitempty"`
	ExpireSec   int      `json:"expire_sec,omitempty"`
	RefreshSec  int      `json:"refresh_sec,omitempty"`
	TTLSec      int      `json:"ttl_sec,omitempty"`
}

// CreateDomainRecordRequest represents the request body for creating a domain record.
type CreateDomainRecordRequest struct {
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name,omitempty"`
	Target   string `json:"target"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	TTLSec   int    `json:"ttl_sec,omitempty"`
}

// UpdateDomainRecordRequest represents the request body for updating a domain record.
type UpdateDomainRecordRequest struct {
	Name     string `json:"name,omitempty"`
	Target   string `json:"target,omitempty"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	TTLSec   int    `json:"ttl_sec,omitempty"`
}
