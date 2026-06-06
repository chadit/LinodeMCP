package profiles_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
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

func assertNil(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if !isNil(got) {
		errorf(t, "expected nil%s, got %#v", messageSuffix(msgAndArgs...), got)
	}
}

func assertEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if !isEmpty(got) {
		errorf(t, "expected empty%s, got %#v", messageSuffix(msgAndArgs...), got)
	}
}

func assertNotEmpty(t *testing.T, got any, msgAndArgs ...any) {
	t.Helper()

	if isEmpty(got) {
		errorf(t, "expected non-empty value%s", messageSuffix(msgAndArgs...))
	}
}

func assertNotEmptyf(t *testing.T, got any, format string, args ...any) {
	t.Helper()

	assertNotEmpty(t, got, fmt.Sprintf(format, args...))
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

func assertLenf(t *testing.T, got any, want int, format string, args ...any) {
	t.Helper()

	assertLen(t, got, want, fmt.Sprintf(format, args...))
}

func assertContains[T comparable](t *testing.T, got []T, want T, msgAndArgs ...any) {
	t.Helper()

	if !slices.Contains(got, want) {
		errorf(t, "expected %#v to contain %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func assertContainsf[T comparable](t *testing.T, got []T, want T, format string, args ...any) {
	t.Helper()

	assertContains(t, got, want, fmt.Sprintf(format, args...))
}

func assertNotContains[T comparable](t *testing.T, got []T, want T, msgAndArgs ...any) {
	t.Helper()

	if slices.Contains(got, want) {
		errorf(t, "expected %#v not to contain %#v%s", got, want, messageSuffix(msgAndArgs...))
	}
}

func assertNotContainsf[T comparable](t *testing.T, got []T, want T, format string, args ...any) {
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

func requireError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		failf(t, "expected error")
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

func requireNotEmptyf(t *testing.T, got any, format string, args ...any) {
	t.Helper()

	requireNotEmpty(t, got, fmt.Sprintf(format, args...))
}

func requireTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		failf(t, "expected true%s", messageSuffix(msgAndArgs...))
	}
}

func requireTruef(t *testing.T, got bool, format string, args ...any) {
	t.Helper()

	requireTrue(t, got, fmt.Sprintf(format, args...))
}

func requireLen(t *testing.T, got any, want int, msgAndArgs ...any) {
	t.Helper()

	if length := valueLen(got); length != want {
		failf(t, "length differs%s\nwant: %d\n got: %d", messageSuffix(msgAndArgs...), want, length)
	}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)

	kind := reflected.Kind()
	if kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface ||
		kind == reflect.Map || kind == reflect.Pointer || kind == reflect.Slice {
		return reflected.IsNil()
	}

	return false
}

// isEmpty mirrors testify empty-value semantics for the assertion subset used here.
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
