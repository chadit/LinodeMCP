package tools_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Helpers named expect* stop the current test with t.Fatalf when the expectation fails.
// Helpers named check* record non-fatal failures with t.Errorf and let the test continue.

func expectationMessage(msg []string) string {
	if len(msg) == 0 {
		return ""
	}

	return ": " + msg[0]
}

// failExpectationf is the shared assertion backend.
// fatal=true preserves require-style guard semantics by stopping the test with t.Fatalf;
// fatal=false preserves assert-style semantics by recording a non-fatal t.Errorf.
func failExpectationf(t *testing.T, fatal bool, format string, args ...any) {
	t.Helper()

	if fatal {
		t.Fatalf(format, args...)
		return
	}

	t.Errorf(format, args...)
}

func expectEqual(t *testing.T, expected, actual any, msg ...string) {
	t.Helper()
	checkEqualWithMode(t, true, expected, actual, msg...)
}

func checkEqual(t *testing.T, expected, actual any, msg ...string) {
	t.Helper()
	checkEqualWithMode(t, false, expected, actual, msg...)
}

func checkEqualWithMode(t *testing.T, fatal bool, expected, actual any, msg ...string) {
	t.Helper()

	if !reflect.DeepEqual(expected, actual) {
		failExpectationf(t, fatal, "expected %v, got %v%s", expected, actual, expectationMessage(msg))
	}
}

func expectTrue(t *testing.T, actual bool, msg ...string) {
	t.Helper()
	checkTrueWithMode(t, true, actual, msg...)
}

func checkTrueWithMode(t *testing.T, fatal, actual bool, msg ...string) {
	t.Helper()

	if !actual {
		failExpectationf(t, fatal, "expected true%s", expectationMessage(msg))
	}
}

func expectFalse(t *testing.T, actual bool, msg ...string) {
	t.Helper()
	checkFalseWithMode(t, true, actual, msg...)
}

func checkFalseWithMode(t *testing.T, fatal, actual bool, msg ...string) {
	t.Helper()

	if actual {
		failExpectationf(t, fatal, "expected false%s", expectationMessage(msg))
	}
}

// expectNoError is fatal so callers may safely rely on err being nil after it returns.
func expectNoError(t *testing.T, err error, msg ...string) {
	t.Helper()

	checkNoErrorWithMode(t, true, err, msg...)
}

func checkNoError(t *testing.T, err error, msg ...string) bool {
	t.Helper()

	return checkNoErrorWithMode(t, false, err, msg...)
}

func checkNoErrorWithMode(t *testing.T, fatal bool, err error, msg ...string) bool {
	t.Helper()

	if err != nil {
		failExpectationf(t, fatal, "unexpected error %v%s", err, expectationMessage(msg))

		return false
	}

	return true
}

// expectNotNil is fatal so callers may safely dereference actual after it returns.
func expectNotNil(t *testing.T, actual any, msg ...string) {
	t.Helper()

	if firewallIsNil(actual) {
		failExpectationf(t, true, "expected non-nil value%s", expectationMessage(msg))
	}
}

func expectNil(t *testing.T, actual any, msg ...string) {
	t.Helper()

	if !firewallIsNil(actual) {
		failExpectationf(t, true, "expected nil, got %v%s", actual, expectationMessage(msg))
	}
}

func expectNotEmpty(t *testing.T, actual any, msg ...string) {
	t.Helper()

	if firewallIsEmpty(actual) {
		failExpectationf(t, true, "expected non-empty value%s", expectationMessage(msg))
	}
}

func checkEmpty(t *testing.T, actual any, msg ...string) {
	t.Helper()
	expectEmptyWithMode(t, false, actual, msg...)
}

func expectEmptyWithMode(t *testing.T, fatal bool, actual any, msg ...string) {
	t.Helper()

	if !firewallIsEmpty(actual) {
		failExpectationf(t, fatal, "expected empty value, got %v%s", actual, expectationMessage(msg))
	}
}

func expectContains(t *testing.T, container, item any, msg ...string) {
	t.Helper()
	expectContainsWithMode(t, true, container, item, msg...)
}

func expectContainsWithMode(t *testing.T, fatal bool, container, item any, msg ...string) {
	t.Helper()

	if !contains(container, item) {
		failExpectationf(t, fatal, "expected %v to contain %v%s", container, item, expectationMessage(msg))
	}
}

func expectNotContains(t *testing.T, container, item any, msg ...string) {
	t.Helper()

	if contains(container, item) {
		failExpectationf(t, true, "expected %v not to contain %v%s", container, item, expectationMessage(msg))
	}
}

func expectLen(t *testing.T, actual any, expected int, msg ...string) {
	t.Helper()

	checkLenWithMode(t, true, actual, expected, msg...)
}

func expectNumericEqual(t *testing.T, expected float64, actual any, msg ...string) {
	t.Helper()

	actualFloat, ok := numericFloat64(actual)
	if !ok {
		failExpectationf(t, true, "expected numeric value %v, got %T%s", expected, actual, expectationMessage(msg))
	}

	if actualFloat != expected {
		failExpectationf(t, true, "expected numeric value %v, got %v%s", expected, actual, expectationMessage(msg))
	}
}

func numericFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func checkLen(t *testing.T, actual any, expected int, msg ...string) bool {
	t.Helper()

	return checkLenWithMode(t, false, actual, expected, msg...)
}

func checkLenWithMode(t *testing.T, fatal bool, actual any, expected int, msg ...string) bool {
	t.Helper()

	value := reflect.ValueOf(actual)
	if !value.IsValid() {
		failExpectationf(t, fatal, "expected length %d, got nil%s", expected, expectationMessage(msg))

		return false
	}

	kind := value.Kind()
	if kind != reflect.Array && kind != reflect.Chan && kind != reflect.Map && kind != reflect.Slice && kind != reflect.String {
		failExpectationf(t, fatal, "expected value with length, got %T%s", actual, expectationMessage(msg))

		return false
	}

	if value.Len() != expected {
		failExpectationf(t, fatal, "expected length %d, got %d%s", expected, value.Len(), expectationMessage(msg))

		return false
	}

	return true
}

func expectStringElementsMatch(t *testing.T, expected, actual []string, msg ...string) {
	t.Helper()

	if len(expected) != len(actual) {
		failExpectationf(t, true, "expected elements %v, got %v%s", expected, actual, expectationMessage(msg))
	}

	counts := make(map[string]int, len(expected))
	for _, value := range expected {
		counts[value]++
	}

	for _, value := range actual {
		counts[value]--
	}

	for value, count := range counts {
		if count != 0 {
			failExpectationf(t, true, "element %q count mismatch in %v vs %v%s", value, expected, actual, expectationMessage(msg))
		}
	}
}

func firewallIsNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	kind := reflected.Kind()

	if kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice {
		return reflected.IsNil()
	}

	return false
}

func firewallIsEmpty(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	kind := reflected.Kind()

	if kind == reflect.Array || kind == reflect.Map || kind == reflect.Slice || kind == reflect.String {
		return reflected.Len() == 0
	}

	zero := reflect.Zero(reflected.Type()).Interface()

	return reflect.DeepEqual(value, zero)
}

func contains(container, item any) bool {
	if typed, ok := container.(string); ok {
		needle, ok := item.(string)

		return ok && strings.Contains(typed, needle)
	}

	if typed, ok := container.([]string); ok {
		needle, ok := item.(string)
		if !ok {
			return false
		}

		return slices.Contains(typed, needle)
	}

	reflected := reflect.ValueOf(container)
	if !reflected.IsValid() {
		return false
	}

	if reflected.Kind() == reflect.Array || reflected.Kind() == reflect.Slice {
		indices := make([]struct{}, reflected.Len())
		for i := range indices {
			if reflect.DeepEqual(reflected.Index(i).Interface(), item) {
				return true
			}
		}

		return false
	}

	if reflected.Kind() != reflect.Map {
		return false
	}

	key := reflect.ValueOf(item)
	if !key.IsValid() || !key.Type().AssignableTo(reflected.Type().Key()) {
		return false
	}

	return reflected.MapIndex(key).IsValid()
}
