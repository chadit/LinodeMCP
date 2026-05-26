package linode

// AlertDefinition describes a monitoring alert definition.
type AlertDefinition struct {
	ID          int            `json:"id"`
	Label       string         `json:"label"`
	Type        string         `json:"type"`
	ServiceType string         `json:"service_type"`
	Description string         `json:"description"`
	Severity    int            `json:"severity"`
	Criteria    map[string]any `json:"criteria"`
}

// AlertChannel describes a monitoring alert channel.
type AlertChannel struct {
	ID          int                 `json:"id"`
	Label       string              `json:"label"`
	Type        string              `json:"type"`
	ChannelType string              `json:"channel_type"`
	Content     AlertChannelContent `json:"content"`
	Alerts      []AlertChannelAlert `json:"alerts"`
	Created     string              `json:"created"`
	CreatedBy   string              `json:"created_by"`
	Updated     string              `json:"updated"`
	UpdatedBy   string              `json:"updated_by"`
}

// AlertChannelContent describes alert channel delivery settings.
type AlertChannelContent struct {
	Email AlertChannelEmailContent `json:"email"`
}

// AlertChannelEmailContent describes email delivery settings for an alert channel.
type AlertChannelEmailContent struct {
	EmailAddresses []string `json:"email_addresses"`
}

// AlertChannelAlert describes an alert associated with an alert channel.
type AlertChannelAlert struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}
