package twostage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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

			got := twostage.OptedIn("linode_some_tool", testCase.capability)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestPlanTTLFallsBackToDefault(t *testing.T) {
	t.Parallel()

	assert.Equal(t, twostage.DefaultPlanTTL, twostage.PlanTTL("linode_unknown_tool"))
}
