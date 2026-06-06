package builder_test

import (
	"errors"
	"fmt"
	"reflect"
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

func assertFalse(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if got {
		errorf(t, "expected false%s", messageSuffix(msgAndArgs...))
	}
}

func assertTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		errorf(t, "expected true%s", messageSuffix(msgAndArgs...))
	}
}

func assertLen(t *testing.T, got any, want int, msgAndArgs ...any) {
	t.Helper()

	if length := valueLen(got); length != want {
		errorf(t, "length differs%s\nwant: %d\n got: %d", messageSuffix(msgAndArgs...), want, length)
	}
}

func assertSame[T comparable](t *testing.T, want, got T, msgAndArgs ...any) {
	t.Helper()

	if got != want {
		errorf(t, "expected same value%s\nwant: %#v\n got: %#v", messageSuffix(msgAndArgs...), want, got)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		failf(t, "expected no error, got %v", err)
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

func requireTrue(t *testing.T, got bool, msgAndArgs ...any) {
	t.Helper()

	if !got {
		failf(t, "expected true%s", messageSuffix(msgAndArgs...))
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
