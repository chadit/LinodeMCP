package profiles_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestLongviewCategory(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_longview_plan_get"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_type_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_subscription_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_client_create"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_plan_update"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_alert_definition_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_alert_channel_list"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}
