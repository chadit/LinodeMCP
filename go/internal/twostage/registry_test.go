package twostage_test

import (
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const toolOther = "linode_other"

func TestOptedInCapabilityDefaults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		capability profiles.Capability
		want       bool
	}{
		{"destroy opts in", profiles.CapDestroy, true},
		{"admin opts out (no admin tool is wired for two-stage)", profiles.CapAdmin, false},
		{"write opts out", profiles.CapWrite, false},
		{"read never opts in", profiles.CapRead, false},
		{"meta never opts in", profiles.CapMeta, false},
		{"unknown never opts in", profiles.CapUnknown, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := twostage.OptedIn("linode_some_tool", testCase.capability); got != testCase.want {
				t.Errorf("OptedIn(%v) = %v, want %v", testCase.capability, got, testCase.want)
			}
		})
	}
}

func TestPlanTTLFallsBackToDefault(t *testing.T) {
	t.Parallel()

	if got := twostage.PlanTTL("linode_unknown_tool"); got != twostage.DefaultPlanTTL {
		t.Errorf("PlanTTL = %v, want %v", got, twostage.DefaultPlanTTL)
	}
}

func TestSettingsOptedInHonorsOverride(t *testing.T) {
	t.Parallel()

	settings := twostage.Settings{OptIn: map[string]bool{
		"linode_destroyer":   false,
		"linode_writer_in":   true,
		"linode_no_override": true,
	}}

	cases := []struct {
		name       string
		tool       string
		capability profiles.Capability
		want       bool
	}{
		{"destroy forced out", "linode_destroyer", profiles.CapDestroy, false},
		{"write forced in", "linode_writer_in", profiles.CapWrite, true},
		{"no entry uses capability default", toolOther, profiles.CapDestroy, true},
		{"no entry write stays out", toolOther, profiles.CapWrite, false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := settings.OptedIn(testCase.tool, testCase.capability); got != testCase.want {
				t.Errorf("OptedIn(%q, %v) = %v, want %v", testCase.tool, testCase.capability, got, testCase.want)
			}
		})
	}
}

func TestSettingsPlanTTLPrecedence(t *testing.T) {
	t.Parallel()

	settings := twostage.Settings{
		DefaultTTL: 2 * time.Minute,
		ToolTTL: map[string]time.Duration{
			"linode_slow":   30 * time.Minute,
			"linode_zeroed": 0,
		},
	}

	cases := []struct {
		name string
		tool string
		want time.Duration
	}{
		{"per-tool override wins", "linode_slow", 30 * time.Minute},
		{"default applies when no per-tool entry", toolOther, 2 * time.Minute},
		{"non-positive per-tool entry falls back to default", "linode_zeroed", 2 * time.Minute},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := settings.PlanTTL(testCase.tool); got != testCase.want {
				t.Errorf("PlanTTL(%q) = %v, want %v", testCase.tool, got, testCase.want)
			}
		})
	}
}

func TestSettingsZeroValueMatchesPackageDefaults(t *testing.T) {
	t.Parallel()

	var settings twostage.Settings

	if got := settings.PlanTTL("linode_anything"); got != twostage.DefaultPlanTTL {
		t.Errorf("zero-value PlanTTL = %v, want %v", got, twostage.DefaultPlanTTL)
	}

	if !settings.OptedIn("linode_anything", profiles.CapDestroy) {
		t.Error("zero-value OptedIn(CapDestroy) = false, want true")
	}
}
