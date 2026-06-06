package server_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func messageSuffix(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}

	if format, ok := msgAndArgs[0].(string); ok {
		if len(msgAndArgs) > 1 {
			return ": " + fmt.Sprintf(format, msgAndArgs[1:]...)
		}

		return ": " + format
	}

	return ": " + fmt.Sprint(msgAndArgs...)
}

func failf(t *testing.T, format string, args ...any) {
	t.Helper()

	t.Fatalf(format, args...)
}

func errorf(t *testing.T, format string, args ...any) {
	t.Helper()

	t.Errorf(format, args...)
}

func assertEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()

	if !reflect.DeepEqual(want, got) {
		errorf(t, "values differ%s\nwant: %#v\n got: %#v", messageSuffix(msgAndArgs...), want, got)
	}
}

func assertEqualf(t *testing.T, want, got any, format string, args ...any) {
	t.Helper()

	assertEqual(t, want, got, fmt.Sprintf(format, args...))
}

func assertNotEqual(t *testing.T, want, got any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(want, got) {
		errorf(t, "expected values to differ%s, both were %#v", messageSuffix(msgAndArgs...), got)
	}
}

func assertNil(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if !isNil(got) {
		errorf(t, "expected nil%s, got %#v", messageSuffix(msgAndArgs...), got)
	}
}

func assertNotNil(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if isNil(got) {
		errorf(t, "expected non-nil value%s", messageSuffix(msgAndArgs...))
	}
}

func assertEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if !isEmpty(got) {
		errorf(t, "expected empty%s, got %#v", messageSuffix(msgAndArgs...), got)
	}
}

func assertEmptyf(t *testing.T, got any, format string, args ...any) {
	t.Helper()

	assertEmpty(t, got, fmt.Sprintf(format, args...))
}

func assertNotEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if isEmpty(got) {
		errorf(t, "expected non-empty value%s", messageSuffix(msgAndArgs...))
	}
}

func assertTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		errorf(t, "expected true%s", messageSuffix(msgAndArgs...))
	}
}

func assertTruef(t *testing.T, got bool, format string, args ...any) {
	t.Helper()

	assertTrue(t, got, fmt.Sprintf(format, args...))
}

func assertFalse(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if got {
		errorf(t, "expected false%s", messageSuffix(msgAndArgs...))
	}
}

func assertFalsef(t *testing.T, got bool, format string, args ...any) {
	t.Helper()

	assertFalse(t, got, fmt.Sprintf(format, args...))
}

func assertNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		errorf(t, "expected no error%s, got %v", messageSuffix(msgAndArgs...), err)
	}
}

func assertErrorIs(t *testing.T, got, want error, msgAndArgs ...any) {
	t.Helper()

	if !errors.Is(got, want) {
		errorf(t, "expected error %v in chain%s, got %v", want, messageSuffix(msgAndArgs...), got)
	}
}

func assertLen(t *testing.T, got any, want int, msgAndArgs ...any) {
	t.Helper()

	if length := valueLen(got); length != want {
		errorf(t, "length differs%s\nwant: %d\n got: %d", messageSuffix(msgAndArgs...), want, length)
	}
}

func assertGreater(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if !compareNumbers(got, want, func(gotNumber, wantNumber float64) bool { return gotNumber > wantNumber }) {
		errorf(t, "expected %#v to be greater than %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func assertGreaterOrEqual(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if !compareNumbers(got, want, func(gotNumber, wantNumber float64) bool { return gotNumber >= wantNumber }) {
		errorf(t, "expected %#v to be greater than or equal to %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func assertContains(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if !contains(got, want) {
		errorf(t, "expected %#v to contain %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func assertContainsf(t *testing.T, got, want any, format string, args ...any) {
	t.Helper()

	assertContains(t, got, want, fmt.Sprintf(format, args...))
}

func assertNotContains(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if contains(got, want) {
		errorf(t, "expected %#v not to contain %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func assertNotContainsf(t *testing.T, got, want any, format string, args ...any) {
	t.Helper()

	assertNotContains(t, got, want, fmt.Sprintf(format, args...))
}

func assertElementsMatch[T comparable](t *testing.T, want, got []T, msgAndArgs ...any) {
	t.Helper()

	if !multisetEqual(want, got) {
		errorf(t, "elements differ%s\nwant: %#v\n got: %#v", messageSuffix(msgAndArgs...), want, got)
	}
}

func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		failf(t, "expected no error%s, got %v", messageSuffix(msgAndArgs...), err)
	}
}

func requireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err == nil {
		failf(t, "expected error%s", messageSuffix(msgAndArgs...))
	}
}

func requireErrorIs(t *testing.T, got, want error, msgAndArgs ...any) {
	t.Helper()

	if !errors.Is(got, want) {
		failf(t, "expected error %v in chain%s, got %v", want, messageSuffix(msgAndArgs...), got)
	}
}

func requireNotNil(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if isNil(got) {
		failf(t, "expected non-nil value%s", messageSuffix(msgAndArgs...))
	}
}

func requireNotEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if isEmpty(got) {
		failf(t, "expected non-empty value%s", messageSuffix(msgAndArgs...))
	}
}

func requireTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		failf(t, "expected true%s", messageSuffix(msgAndArgs...))
	}
}

func requireLen(t *testing.T, got any, want int, msgAndArgs ...any) {
	t.Helper()

	if length := valueLen(got); length != want {
		failf(t, "length differs%s\nwant: %d\n got: %d", messageSuffix(msgAndArgs...), want, length)
	}
}

func requireGreater(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if !compareNumbers(got, want, func(gotNumber, wantNumber float64) bool { return gotNumber > wantNumber }) {
		failf(t, "expected %#v to be greater than %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func requireContains(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if !contains(got, want) {
		failf(t, "expected %#v to contain %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func requireNotContains(t *testing.T, got, want any, msgAndArgs ...any) {
	t.Helper()

	if contains(got, want) {
		failf(t, "expected %#v not to contain %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	kind := reflected.Kind().String()

	return (kind == "chan" || kind == "func" || kind == "interface" ||
		kind == "map" || kind == "ptr" || kind == "slice") && reflected.IsNil()
}

func isEmpty(value any) bool {
	if isNil(value) {
		return true
	}

	reflected := reflect.ValueOf(value)

	kind := reflected.Kind()
	if kind == reflect.Array || kind == reflect.Chan || kind == reflect.Map ||
		kind == reflect.Slice || kind == reflect.String {
		return reflected.Len() == 0
	}

	if kind == reflect.Bool {
		return !reflected.Bool()
	}

	return reflect.DeepEqual(value, reflect.Zero(reflected.Type()).Interface())
}

func valueLen(value any) int {
	reflected := reflect.ValueOf(value)

	kind := reflected.Kind()
	if kind == reflect.Array || kind == reflect.Chan || kind == reflect.Map ||
		kind == reflect.Slice || kind == reflect.String {
		return reflected.Len()
	}

	return -1
}

func contains(container, item any) bool {
	if haystack, ok := container.(string); ok {
		needle, ok := item.(string)

		return ok && strings.Contains(haystack, needle)
	}

	reflected := reflect.ValueOf(container)
	if !reflected.IsValid() {
		return false
	}

	kind := reflected.Kind()
	if kind == reflect.Map {
		key := reflect.ValueOf(item)
		if !key.IsValid() || !key.Type().AssignableTo(reflected.Type().Key()) {
			return false
		}

		return reflected.MapIndex(key).IsValid()
	}

	if kind == reflect.Array || kind == reflect.Slice {
		length := reflected.Len()

		var index int
		for index < length {
			if reflect.DeepEqual(reflected.Index(index).Interface(), item) {
				return true
			}

			index++
		}
	}

	return false
}

func compareNumbers(got, want any, cmp func(float64, float64) bool) bool {
	gotNumber, gotConverted := toFloat64(got)
	if !gotConverted {
		return false
	}

	wantNumber, wantConverted := toFloat64(want)

	return wantConverted && cmp(gotNumber, wantNumber)
}

func toFloat64(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case float32:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func multisetEqual[T comparable](want, got []T) bool {
	if len(want) != len(got) {
		return false
	}

	counts := make(map[T]int, len(want))
	for _, item := range want {
		counts[item]++
	}

	for _, item := range got {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}

	return true
}
