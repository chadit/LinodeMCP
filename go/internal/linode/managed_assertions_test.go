package linode_test

import (
	"math"
	"testing"
)

func checkNotNil(t *testing.T, value any, msgAndArgs ...any) bool {
	t.Helper()

	if !isNil(value) {
		return true
	}

	t.Errorf("%s: expected non-nil value", failureMessage("expected non-nil value", msgAndArgs...))

	return false
}

func requireTrue(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()

	if value {
		return
	}

	t.Fatalf("%s: expected true", failureMessage("expected true", msgAndArgs...))
}

func checkInEpsilon(t *testing.T, expected, actual, epsilon float64, msgAndArgs ...any) {
	t.Helper()

	delta := math.Abs(expected - actual)
	if delta <= epsilon {
		return
	}

	t.Errorf("%s: expected %v and %v to differ by no more than %v", failureMessage("values are outside epsilon", msgAndArgs...), expected, actual, epsilon)
}
