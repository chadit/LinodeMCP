package linode

// CreateSupportTicketRequest contains the request body for POST /support/tickets.
type CreateSupportTicketRequest struct {
	ManagedIssue     *bool   `json:"managed_issue,omitempty"`
	NodeBalancerID   *int    `json:"nodebalancer_id,omitempty"`
	VPCID            *int    `json:"vpc_id,omitempty"`
	DomainID         *int    `json:"domain_id,omitempty"`
	FirewallID       *int    `json:"firewall_id,omitempty"`
	LinodeID         *int    `json:"linode_id,omitempty"`
	DatabaseID       *int    `json:"database_id,omitempty"`
	LKEClusterID     *int    `json:"lkecluster_id,omitempty"`
	Bucket           *string `json:"bucket,omitempty"`
	LongviewClientID *int    `json:"longviewclient_id,omitempty"`
	Region           *string `json:"region,omitempty"`
	Severity         *int    `json:"severity,omitempty"`
	VolumeID         *int    `json:"volume_id,omitempty"`
	VLAN             *string `json:"vlan,omitempty"`
	Summary          string  `json:"summary"`
	Description      string  `json:"description"`
}

// CreateSupportTicketReplyRequest contains the request body for POST /support/tickets/{ticket_id}/replies.
type CreateSupportTicketReplyRequest struct {
	Description string `json:"description"`
}

// SupportTicket represents one support ticket returned by GET /support/tickets.
type SupportTicket struct {
	Closed      *string                   `json:"closed"`
	Entity      *SupportTicketEntity      `json:"entity"`
	Status      string                    `json:"status"`
	Description string                    `json:"description"`
	GravatarID  string                    `json:"gravatar_id"`
	Opened      string                    `json:"opened"`
	OpenedBy    string                    `json:"opened_by"`
	Summary     string                    `json:"summary"`
	Updated     string                    `json:"updated"`
	UpdatedBy   string                    `json:"updated_by"`
	Attachments []SupportTicketAttachment `json:"attachments"`
	ID          int                       `json:"id"`
	Closable    bool                      `json:"closable"`
}

// CreateSupportTicketAttachmentRequest carries the local file path uploaded to
// POST /support/tickets/{ticket_id}/attachments. The endpoint consumes
// multipart/form-data, so File is read from disk and streamed as the "file"
// form field rather than marshaled into a JSON body.
type CreateSupportTicketAttachmentRequest struct {
	File string `json:"file"`
}

// SupportTicketReply represents one reply returned by GET /support/tickets/{ticket_id}/replies.
type SupportTicketReply struct {
	Created     string `json:"created"`
	CreatedBy   string `json:"created_by"`
	Description string `json:"description"`
	GravatarID  string `json:"gravatar_id"`
	Updated     string `json:"updated"`
	UpdatedBy   string `json:"updated_by"`
	ID          int    `json:"id"`
}

// SupportTicketAttachment represents one attachment on a support ticket.
type SupportTicketAttachment struct {
	Filename string `json:"filename"`
	ID       int    `json:"id"`
	Size     int    `json:"size"`
}

// SupportTicketEntity identifies the API entity attached to a support ticket.
type SupportTicketEntity struct {
	ID    any    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}
