package profiles_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// Longview is gated on its own longview:* scope, so it is its own category:
// folding it into monitor would have let a monitor-scoped profile write to a
// product its token has no scope for, and Python already filed it apart.
func TestLongviewToolsAreTheirOwnCategoryApartFromMonitor(t *testing.T) {
	t.Parallel()

	for _, toolName := range []string{
		"linode_longview_client_create",
		"linode_longview_client_get",
		"linode_longview_plan_update",
		"linode_longview_type_list",
	} {
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			cats := gentools.CategoriesFor(toolName)
			if !slices.Contains(cats, categoryLongview) {
				t.Errorf("CategoriesFor(%q) = %v, want it to contain longview", toolName, cats)
			}

			if slices.Contains(cats, categoryMonitor) {
				t.Errorf("CategoriesFor(%q) = %v, want no monitor entry", toolName, cats)
			}
		})
	}
}

func TestMonitorToolsStayInMonitor(t *testing.T) {
	t.Parallel()

	for _, toolName := range []string{
		"linode_monitor_alert_channel_list",
		"linode_monitor_alert_definition_list",
	} {
		t.Run(toolName, func(t *testing.T) {
			t.Parallel()

			if cats := gentools.CategoriesFor(toolName); !slices.Contains(cats, categoryMonitor) {
				t.Errorf("CategoriesFor(%q) = %v, want it to contain monitor", toolName, cats)
			}
		})
	}
}
