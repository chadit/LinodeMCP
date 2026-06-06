package twostage_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

func TestOptedInCapabilityDefaults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		capability profiles.Capability
		want       bool
	}{
		{"destroy opts in", profiles.CapDestroy, true},
		{"admin opts in", profiles.CapAdmin, true},
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
