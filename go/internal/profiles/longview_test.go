package profiles_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestLongviewCategory(t *testing.T) {
	t.Parallel()

	assertContains(t, profiles.Categories("linode_longview_plan"), "monitor")
	assertContains(t, profiles.Categories("linode_longview_types"), "monitor")
	assertContains(t, profiles.Categories("linode_longview_subscriptions"), "monitor")
	assertContains(t, profiles.Categories("linode_longview_client_create"), "monitor")
	assertContains(t, profiles.Categories("linode_longview_plan_update"), "monitor")
	assertContains(t, profiles.Categories("linode_monitor_alert_definitions"), "monitor")
	assertContains(t, profiles.Categories("linode_monitor_alert_channels"), "monitor")
}
