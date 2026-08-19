package toolhooks_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	keyPaymentUSD = "usd"
	// paymentAmount is a well-formed amount every case that is not about the
	// amount itself can lean on.
	paymentAmount = "25.50"
)

// TestLinodeAccountPaymentCreateNormalizeTrimsTheAmount: the trimmed amount is
// what this tool has always sent, so the padding must not survive into the body.
func TestLinodeAccountPaymentCreateNormalizeTrimsTheAmount(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		supplied any
		want     any
	}{
		"padded amount": {supplied: "  " + paymentAmount + "  ", want: paymentAmount},
		"clean amount":  {supplied: paymentAmount, want: paymentAmount},
		"blank amount":  {supplied: pgPaddingLabel, want: ""},
		"non-string amount is left for the rules to refuse": {supplied: 25.5, want: 25.5},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestWith(map[string]any{keyPaymentUSD: test.supplied})
			toolhooks.LinodeAccountPaymentCreateNormalize(&request)

			if got := request.GetArguments()[keyPaymentUSD]; got != test.want {
				t.Errorf("usd = %v, want %v", got, test.want)
			}
		})
	}
}

// TestLinodeAccountPaymentCreateNormalizeLeavesAnAbsentAmountAbsent: adding the
// key would turn "usd is required" into a blank amount the rules answer for
// differently.
func TestLinodeAccountPaymentCreateNormalizeLeavesAnAbsentAmountAbsent(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{})
	toolhooks.LinodeAccountPaymentCreateNormalize(&request)

	if _, supplied := request.GetArguments()[keyPaymentUSD]; supplied {
		t.Error("usd was added, want it left absent")
	}
}
