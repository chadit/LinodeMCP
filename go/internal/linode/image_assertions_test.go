package linode_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func failureMessage(defaultMsg string, msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return defaultMsg
	}

	if msg, ok := msgAndArgs[0].(string); ok && msg != "" {
		return msg
	}

	return defaultMsg
}

func checkEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("%s: expected %#v, got %#v", failureMessage("values differ", msgAndArgs...), expected, actual)
}

func checkEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if length, ok := valueLen(value); ok && length == 0 {
		return
	}

	if reflect.DeepEqual(value, zeroValue(value)) {
		return
	}

	t.Errorf("%s: expected empty value, got %#v", failureMessage("value is not empty", msgAndArgs...), value)
}

func checkNoError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()

	if err == nil {
		return true
	}

	t.Errorf("%s: unexpected error: %v", failureMessage("expected no error", msgAndArgs...), err)

	return false
}

func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf("%s: unexpected error: %v", failureMessage("expected no error", msgAndArgs...), err)
}

func requireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		return
	}

	t.Fatalf("%s: expected error", failureMessage("expected error", msgAndArgs...))
}

func requireAPIError(t *testing.T, err error, msgAndArgs ...any) *linode.APIError {
	t.Helper()

	apiErr := findErrorType[*linode.APIError](err)
	if apiErr != nil {
		return apiErr
	}

	t.Fatalf("%s: expected API error, got %v", failureMessage("expected API error", msgAndArgs...), err)

	return nil
}

func requireNetworkError(t *testing.T, err error, msgAndArgs ...any) *linode.NetworkError {
	t.Helper()

	networkErr := findErrorType[*linode.NetworkError](err)
	if networkErr != nil {
		return networkErr
	}

	t.Fatalf("%s: expected network error, got %v", failureMessage("expected network error", msgAndArgs...), err)

	return nil
}

func findErrorType[T error](err error) T {
	var zero T

	if err == nil {
		return zero
	}

	currentValue := reflect.ValueOf(err)
	if currentValue.Type().AssignableTo(reflect.TypeFor[T]()) {
		matchedErr, ok := currentValue.Interface().(T)
		if ok {
			return matchedErr
		}
	}

	if unwrappedErrs, ok := err.(interface{ Unwrap() []error }); ok {
		for _, unwrappedErr := range unwrappedErrs.Unwrap() {
			matchedErr := findErrorType[T](unwrappedErr)
			if !isNil(matchedErr) {
				return matchedErr
			}
		}
	}

	if unwrappedErr, ok := err.(interface{ Unwrap() error }); ok {
		return findErrorType[T](unwrappedErr.Unwrap())
	}

	return zero
}

func requireErrorIs(t *testing.T, err, target error, msgAndArgs ...any) {
	t.Helper()

	if errors.Is(err, target) {
		return
	}

	t.Fatalf("%s: expected error %v to match %v", failureMessage("expected matching error", msgAndArgs...), err, target)
}

func checkNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if isNil(value) {
		return
	}

	t.Errorf("%s: expected nil, got %#v", failureMessage("expected nil", msgAndArgs...), value)
}

func requireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if !isNil(value) {
		return
	}

	t.Fatalf("%s: expected non-nil value", failureMessage("expected non-nil value", msgAndArgs...))
}

func checkLenOne(t *testing.T, value any, msgAndArgs ...any) bool {
	t.Helper()

	actual, ok := valueLen(value)
	if ok && actual == 1 {
		return true
	}

	t.Errorf("%s: expected length 1, got %d for %#v", failureMessage("length differs", msgAndArgs...), actual, value)

	return false
}

func requireLenOne(t *testing.T, value any) {
	t.Helper()

	actual, ok := valueLen(value)
	if ok && actual == 1 {
		return
	}

	t.Fatalf("length differs: expected length 1, got %d for %#v", actual, value)
}

func checkTrue(t *testing.T, value bool, msgAndArgs ...any) bool {
	t.Helper()

	if value {
		return true
	}

	t.Errorf("%s: expected true", failureMessage("expected true", msgAndArgs...))

	return false
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	var isNil bool

	func() {
		defer func() {
			if recover() != nil {
				isNil = false
			}
		}()

		isNil = reflect.ValueOf(value).IsNil()
	}()

	return isNil
}

func zeroValue(value any) any {
	if value == nil {
		return nil
	}

	return reflect.Zero(reflect.TypeOf(value)).Interface()
}

func valueLen(value any) (int, bool) {
	if value == nil {
		return 0, false
	}

	var (
		length      int
		validLength = true
	)

	func() {
		defer func() {
			if recover() != nil {
				length = 0
				validLength = false
			}
		}()

		length = reflect.ValueOf(value).Len()
	}()

	return length, validLength
}
