package tools_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// The values these previews are driven with, each only ever this file's.
const (
	ipv6RouteTargetFixture = "2001:db8::1"
	volumeLabelFixture     = "block-data"
)

// The wording order two declared previews choose between. The contract refuses
// a wording an earlier one always wins over, but nothing else says which of the
// reachable ones a given call reads, and the behavior fixtures pin one branch
// each. The IPv6 range's last wording is not covered here: its two targets are
// required as a pair, so a call reaching that wording is one the argument
// checks have already refused.

// declaredPreviewEffects reads the side effects a generated dry run reported.
func declaredPreviewEffects(t *testing.T, text string) []string {
	t.Helper()

	var preview struct {
		SideEffects []string `json:"side_effects"`
	}

	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview.SideEffects
}

// declaredPreviewWarnings reads the warnings a generated dry run reported.
func declaredPreviewWarnings(t *testing.T, text string) []string {
	t.Helper()

	var preview struct {
		Warnings []string `json:"warnings"`
	}

	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return preview.Warnings
}

func TestGeneratedNodeBalancerCreatePreviewChoosesItsWording(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{
			name: "a label is quoted into the sentence",
			args: map[string]any{managedServiceLabelParam: "web-lb"},
			want: `A new NodeBalancer "web-lb" will be created in region us-east.`,
		},
		{
			name: "no label names the region alone",
			args: map[string]any{},
			want: "A new NodeBalancer will be created in region us-east.",
		},
		{
			name: "a reserved address is named after the balancer",
			args: map[string]any{managedServiceLabelParam: "web-lb", keyIPv4: reservedIPAddressFixture},
			want: `A new NodeBalancer "web-lb" will be created in region us-east. Reserved IPv4 address 192.0.2.10 will be assigned.`,
		},
		{
			name: "a reserved address without a label still lands",
			args: map[string]any{keyIPv4: reservedIPAddressFixture},
			want: "A new NodeBalancer will be created in region us-east. Reserved IPv4 address 192.0.2.10 will be assigned.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyRegion: placementGroupCreateRegion, keyDryRun: true}
			maps.Copy(args, testCase.args)

			_, _, handler := gentools.NewLinodeNodebalancerCreateTool(newTestConfig("http://127.0.0.1:1"))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			got := declaredPreviewEffects(t, resultText(t, result))
			if len(got) != 1 || got[0] != testCase.want {
				t.Errorf("side_effects = %v, want [%q]", got, testCase.want)
			}
		})
	}
}

func TestGeneratedIPv6RangeCreatePreviewChoosesItsWording(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want string
	}{
		{
			name: "a route target is named",
			args: map[string]any{"route_target": ipv6RouteTargetFixture},
			want: "A new IPv6 range with prefix length /64 will be allocated and routed to 2001:db8::1.",
		},
		{
			name: "an instance is named when no route target is",
			args: map[string]any{keyManagedLinodeSettingsLinodeID: float64(42)},
			want: "A new IPv6 range with prefix length /64 will be allocated and routed to instance 42.",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{"prefix_length": float64(64), keyDryRun: true}
			maps.Copy(args, testCase.args)

			_, _, handler := gentools.NewLinodeIPv6RangeCreateTool(newTestConfig("http://127.0.0.1:1"))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			got := declaredPreviewEffects(t, resultText(t, result))
			if len(got) != 1 || got[0] != testCase.want {
				t.Errorf("side_effects = %v, want [%q]", got, testCase.want)
			}
		})
	}
}

// TestGeneratedVolumeCreatePreviewDropsTheAttachLineItCannotFill: the attach
// sentence is the whole line a detached create has nothing to say on, and a
// size the caller left to the API is named by neither wording rather than
// reported as zero.
func TestGeneratedVolumeCreatePreviewDropsTheAttachLineItCannotFill(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args map[string]any
		name string
		want []string
	}{
		{
			name: "a sized volume in a region names both",
			args: map[string]any{keyRegion: placementGroupCreateRegion, keySize: float64(40)},
			want: []string{`A new 40 GB volume "block-data" will be created in region us-east.`},
		},
		{
			name: "an attached create names the instance on its own line",
			args: map[string]any{keyManagedLinodeSettingsLinodeID: float64(42), keySize: float64(10)},
			want: []string{
				`A new 10 GB volume "block-data" will be created.`,
				"The volume is attached to instance 42 on creation.",
			},
		},
		{
			name: "a size left to the API is named by neither wording",
			args: map[string]any{keyRegion: placementGroupCreateRegion},
			want: []string{`A new volume "block-data" will be created in region us-east.`},
		},
		{
			name: "an attached create with no size still names the instance",
			args: map[string]any{keyManagedLinodeSettingsLinodeID: float64(42)},
			want: []string{
				`A new volume "block-data" will be created.`,
				"The volume is attached to instance 42 on creation.",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{managedServiceLabelParam: volumeLabelFixture, keyDryRun: true}
			maps.Copy(args, testCase.args)

			_, _, handler := gentools.NewLinodeVolumeCreateTool(newTestConfig("http://127.0.0.1:1"))

			result, err := handler(t.Context(), createRequestWithArgs(t, args))
			if err != nil {
				t.Fatalf("preview: %v", err)
			}

			text := resultText(t, result)

			if got := declaredPreviewEffects(t, text); !slices.Equal(got, testCase.want) {
				t.Errorf("side_effects = %v, want %v", got, testCase.want)
			}

			want := []string{"Billing for the volume starts immediately on creation."}
			if got := declaredPreviewWarnings(t, text); !slices.Equal(got, want) {
				t.Errorf("warnings = %v, want %v", got, want)
			}
		})
	}
}
