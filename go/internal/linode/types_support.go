package linode

// SupportTicket represents one support ticket returned by GET /support/tickets.
type SupportTicket struct {
	Attachments []SupportTicketAttachment `json:"attachments"`
	Closable    bool                      `json:"closable"`
	Closed      *string                   `json:"closed"`
	Description string                    `json:"description"`
	Entity      *SupportTicketEntity      `json:"entity"`
	GravatarID  string                    `json:"gravatar_id"`
	ID          int                       `json:"id"`
	Opened      string                    `json:"opened"`
	OpenedBy    string                    `json:"opened_by"`
	Status      string                    `json:"status"`
	Summary     string                    `json:"summary"`
	Updated     string                    `json:"updated"`
	UpdatedBy   string                    `json:"updated_by"`
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
