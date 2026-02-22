package linode

// Domain represents a Linode DNS domain.
type Domain struct {
	ID          int      `json:"id"`
	Domain      string   `json:"domain"`
	Type        string   `json:"type"`   // master, slave
	Status      string   `json:"status"` // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email"`
	Description string   `json:"description"`
	RetrySec    int      `json:"retry_sec"`
	MasterIPs   []string `json:"master_ips"`
	AXFRIPs     []string `json:"axfr_ips"`
	ExpireSec   int      `json:"expire_sec"`
	RefreshSec  int      `json:"refresh_sec"`
	TTLSec      int      `json:"ttl_sec"`
	Tags        []string `json:"tags"`
	Created     string   `json:"created"`
	Updated     string   `json:"updated"`
	Group       string   `json:"group"`
}

// DomainRecord represents a DNS record within a domain.
type DomainRecord struct {
	ID       int    `json:"id"`
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name"`
	Target   string `json:"target"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	Port     int    `json:"port"`
	Service  string `json:"service"`
	Protocol string `json:"protocol"`
	TTLSec   int    `json:"ttl_sec"`
	Tag      string `json:"tag"`
	Created  string `json:"created"`
	Updated  string `json:"updated"`
}

// CreateDomainRequest represents the request body for creating a domain.
type CreateDomainRequest struct {
	Domain      string   `json:"domain"`
	Type        string   `json:"type"` // master, slave
	SOAEmail    string   `json:"soa_email,omitempty"`
	Description string   `json:"description,omitempty"`
	RetrySec    int      `json:"retry_sec,omitempty"`
	MasterIPs   []string `json:"master_ips,omitempty"`
	AXFRIPs     []string `json:"axfr_ips,omitempty"`
	ExpireSec   int      `json:"expire_sec,omitempty"`
	RefreshSec  int      `json:"refresh_sec,omitempty"`
	TTLSec      int      `json:"ttl_sec,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Group       string   `json:"group,omitempty"`
}

// UpdateDomainRequest represents the request body for updating a domain.
type UpdateDomainRequest struct {
	Domain      string   `json:"domain,omitempty"`
	Status      string   `json:"status,omitempty"` // active, disabled, edit_mode
	SOAEmail    string   `json:"soa_email,omitempty"`
	Description string   `json:"description,omitempty"`
	RetrySec    int      `json:"retry_sec,omitempty"`
	MasterIPs   []string `json:"master_ips,omitempty"`
	AXFRIPs     []string `json:"axfr_ips,omitempty"`
	ExpireSec   int      `json:"expire_sec,omitempty"`
	RefreshSec  int      `json:"refresh_sec,omitempty"`
	TTLSec      int      `json:"ttl_sec,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Group       string   `json:"group,omitempty"`
}

// CreateDomainRecordRequest represents the request body for creating a domain record.
type CreateDomainRecordRequest struct {
	Type     string `json:"type"` // A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, PTR
	Name     string `json:"name,omitempty"`
	Target   string `json:"target"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	TTLSec   int    `json:"ttl_sec,omitempty"`
	Tag      string `json:"tag,omitempty"`
}

// UpdateDomainRecordRequest represents the request body for updating a domain record.
type UpdateDomainRecordRequest struct {
	Name     string `json:"name,omitempty"`
	Target   string `json:"target,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Service  string `json:"service,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	TTLSec   int    `json:"ttl_sec,omitempty"`
	Tag      string `json:"tag,omitempty"`
}
