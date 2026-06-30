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

// MonitorMetrics describes metric data returned for a monitoring service entity.
type MonitorMetrics map[string]any

// MonitorServiceToken describes a token returned for a monitoring service entity.
type MonitorServiceToken struct {
	Token  string `json:"token"`
	Expiry string `json:"expiry"`
}

// CreateMonitorServiceTokenRequest describes a monitor service token create request.
type CreateMonitorServiceTokenRequest struct {
	EntityIDs []int `json:"entity_ids"`
}

// AlertDefinition describes a monitoring alert definition.
type AlertDefinition struct {
	ID                int            `json:"id"`
	Label             string         `json:"label"`
	Type              string         `json:"type"`
	ServiceType       string         `json:"service_type"`
	Description       string         `json:"description"`
	Severity          int            `json:"severity"`
	Status            string         `json:"status,omitempty"`
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

// UpdateAlertDefinitionRequest describes a monitoring alert definition update request.
type UpdateAlertDefinitionRequest struct {
	ChannelIDs        []int          `json:"channel_ids,omitempty"`
	Description       *string        `json:"description,omitempty"`
	EntityIDs         []string       `json:"entity_ids,omitempty"`
	Label             *string        `json:"label,omitempty"`
	RuleCriteria      map[string]any `json:"rule_criteria,omitempty"`
	Severity          *int           `json:"severity,omitempty"`
	Status            *string        `json:"status,omitempty"`
	TriggerConditions map[string]any `json:"trigger_conditions,omitempty"`
}
