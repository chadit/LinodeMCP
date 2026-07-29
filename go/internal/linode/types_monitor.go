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

// MonitorMetrics describes metric data returned for a monitoring service entity.
type MonitorMetrics map[string]any

// CreateMonitorServiceTokenRequest describes a monitor service token create request.
type CreateMonitorServiceTokenRequest struct {
	EntityIDs []int `json:"entity_ids"`
}

// AlertDefinition describes a monitoring alert definition.
type AlertDefinition struct {
	Criteria          map[string]any `json:"criteria"`
	RuleCriteria      map[string]any `json:"rule_criteria,omitempty"`
	TriggerConditions map[string]any `json:"trigger_conditions,omitempty"`
	Label             string         `json:"label"`
	Type              string         `json:"type"`
	ServiceType       string         `json:"service_type"`
	Description       string         `json:"description"`
	Status            string         `json:"status,omitempty"`
	ChannelIDs        []int          `json:"channel_ids,omitempty"`
	EntityIDs         []string       `json:"entity_ids,omitempty"`
	GroupBy           []string       `json:"group_by,omitempty"`
	ID                int            `json:"id"`
	Severity          int            `json:"severity"`
}

// CreateAlertDefinitionRequest describes a monitoring alert definition create request.
type CreateAlertDefinitionRequest struct {
	Description       *string        `json:"description,omitempty"`
	RuleCriteria      map[string]any `json:"rule_criteria"`
	TriggerConditions map[string]any `json:"trigger_conditions"`
	Label             string         `json:"label"`
	ChannelIDs        []int          `json:"channel_ids"`
	EntityIDs         []string       `json:"entity_ids,omitempty"`
	Severity          int            `json:"severity"`
}

// CloneAlertDefinitionRequest describes a monitoring alert definition clone request.
// Pointer collection fields preserve the distinction between an omitted override
// and an explicitly empty array or object.
type CloneAlertDefinitionRequest struct {
	ChannelIDs        *[]int          `json:"channel_ids,omitempty"`
	Description       *string         `json:"description,omitempty"`
	GroupBy           *[]string       `json:"group_by,omitempty"`
	RuleCriteria      *map[string]any `json:"rule_criteria,omitempty"`
	Severity          *int            `json:"severity,omitempty"`
	TriggerConditions *map[string]any `json:"trigger_conditions,omitempty"`
	Label             string          `json:"label"`
}

// UpdateAlertDefinitionRequest describes a monitoring alert definition update request.
type UpdateAlertDefinitionRequest struct {
	Description       *string        `json:"description,omitempty"`
	Label             *string        `json:"label,omitempty"`
	RuleCriteria      map[string]any `json:"rule_criteria,omitempty"`
	Severity          *int           `json:"severity,omitempty"`
	Status            *string        `json:"status,omitempty"`
	TriggerConditions map[string]any `json:"trigger_conditions,omitempty"`
	ChannelIDs        []int          `json:"channel_ids,omitempty"`
	EntityIDs         []string       `json:"entity_ids,omitempty"`
}
