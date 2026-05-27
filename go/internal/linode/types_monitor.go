package linode

// MonitorService describes a supported monitoring service type.
type MonitorService struct {
	Label       string `json:"label"`
	ServiceType string `json:"service_type"`
}

// MonitorMetricDefinition describes one monitoring metric definition.
type MonitorMetricDefinition struct {
	Label      string `json:"label"`
	Metric     string `json:"metric"`
	MetricType string `json:"metric_type"`
}

// MonitorDashboard describes a monitoring dashboard.
type MonitorDashboard map[string]any

// AlertDefinition describes a monitoring alert definition.
type AlertDefinition struct {
	ID                int            `json:"id"`
	Label             string         `json:"label"`
	Type              string         `json:"type"`
	ServiceType       string         `json:"service_type"`
	Description       string         `json:"description"`
	Severity          int            `json:"severity"`
	Criteria          map[string]any `json:"criteria"`
	RuleCriteria      map[string]any `json:"rule_criteria,omitempty"`
	TriggerConditions map[string]any `json:"trigger_conditions,omitempty"`
	ChannelIDs        []int          `json:"channel_ids,omitempty"`
	EntityIDs         []string       `json:"entity_ids,omitempty"`
}

// CreateAlertDefinitionRequest describes a monitoring alert definition create request.
type CreateAlertDefinitionRequest struct {
	ChannelIDs        []int          `json:"channel_ids"`
	Description       *string        `json:"description,omitempty"`
	EntityIDs         []string       `json:"entity_ids,omitempty"`
	Label             string         `json:"label"`
	RuleCriteria      map[string]any `json:"rule_criteria"`
	Severity          int            `json:"severity"`
	TriggerConditions map[string]any `json:"trigger_conditions"`
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
