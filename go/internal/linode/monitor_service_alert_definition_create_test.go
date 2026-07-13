package linode_test

import (
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func monitorAlertDefinitionCreateRequest() *linode.CreateAlertDefinitionRequest {
	description := "Alert when CPU usage is high"

	return &linode.CreateAlertDefinitionRequest{
		ChannelIDs:  []int{546, 392},
		Description: &description,
		EntityIDs:   []string{"13116"},
		Label:       monitorAlertDefinitionLabel,
		RuleCriteria: map[string]any{
			"rules": []any{map[string]any{
				keyMetric:            "cpu_usage",
				"operator":           "gt",
				keyThreshold:         float64(80),
				"aggregate_function": "avg",
			}},
		},
		Severity: 2,
		TriggerConditions: map[string]any{
			"criteria_condition":        "ALL",
			"evaluation_period_seconds": float64(300),
			"polling_interval_seconds":  float64(300),
			"trigger_occurrences":       float64(3),
		},
	}
}
