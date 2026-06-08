package profiles_test

import (
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestLongviewCategory(t *testing.T) {
	t.Parallel()

	if !slices.Contains(profiles.Categories("linode_longview_plan"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_types"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_subscriptions"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_client_create"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_longview_plan_update"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_alert_definitions"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}

	if !slices.Contains(profiles.Categories("linode_monitor_alert_channels"), "monitor") {
		t.Errorf("collection does not contain %v", "monitor")
	}
}
