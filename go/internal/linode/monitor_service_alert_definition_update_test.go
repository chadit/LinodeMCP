package linode_test

import (
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func monitorAlertDefinitionUpdateRequest() *linode.UpdateAlertDefinitionRequest {
	description := "Updated alert when CPU usage is high"
	severity := 1
	status := statusEnabledFixture
	label := monitorAlertDefinitionLabel + " Updated"

	return &linode.UpdateAlertDefinitionRequest{
		ChannelIDs:  []int{546, 392},
		Description: &description,
		EntityIDs:   []string{"13116"},
		Label:       &label,
		RuleCriteria: map[string]any{
			"rules": []any{map[string]any{
				"metric":     "cpu_usage",
				"operator":   "gt",
				keyThreshold: float64(80),
			}},
		},
		Severity: &severity,
		Status:   &status,
		TriggerConditions: map[string]any{
			"criteria_condition":        "ALL",
			"evaluation_period_seconds": float64(300),
			"polling_interval_seconds":  float64(300),
			"trigger_occurrences":       float64(3),
		},
	}
}
