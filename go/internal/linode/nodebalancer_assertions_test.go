package linode_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func nbFailureMessage(defaultMsg string, msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return defaultMsg
	}

	if msg, ok := msgAndArgs[0].(string); ok && msg != "" {
		return msg
	}

	return defaultMsg
}

func nbCheckEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("%s: expected %#v, got %#v", nbFailureMessage("values differ", msgAndArgs...), expected, actual)
}

func nbCheckEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if length, ok := nbValueLen(value); ok && length == 0 {
		return
	}

	if reflect.DeepEqual(value, nbZeroValue(value)) {
		return
	}

	t.Errorf("%s: expected empty value, got %#v", nbFailureMessage("value is not empty", msgAndArgs...), value)
}

func nbCheckNoError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()

	if err == nil {
		return true
	}

	t.Errorf("%s: unexpected error: %v", nbFailureMessage("expected no error", msgAndArgs...), err)

	return false
}

func nbRequireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf("%s: unexpected error: %v", nbFailureMessage("expected no error", msgAndArgs...), err)
}

func nbRequireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		return
	}

	t.Fatalf("%s: expected error", nbFailureMessage("expected error", msgAndArgs...))
}

func nbRequireErrorIs(t *testing.T, err, target error) {
	t.Helper()

	if errors.Is(err, target) {
		return
	}

	t.Fatalf("expected matching error: expected error %v to match %v", err, target)
}

func nbRequireAPIError(t *testing.T, err error, msgAndArgs ...any) *linode.APIError {
	t.Helper()

	apiErr := nbFindAPIError(err)
	if apiErr != nil {
		return apiErr
	}

	t.Fatalf("%s: expected API error, got %v", nbFailureMessage("expected API error", msgAndArgs...), err)

	return nil
}

func nbFindAPIError(err error) *linode.APIError {
	if err == nil {
		return nil
	}

	if apiErr, ok := any(err).(*linode.APIError); ok {
		return apiErr
	}

	if unwrappedErrs, ok := err.(interface{ Unwrap() []error }); ok {
		for _, unwrappedErr := range unwrappedErrs.Unwrap() {
			apiErr := nbFindAPIError(unwrappedErr)
			if apiErr != nil {
				return apiErr
			}
		}
	}

	if unwrappedErr, ok := err.(interface{ Unwrap() error }); ok {
		return nbFindAPIError(unwrappedErr.Unwrap())
	}

	return nil
}

func nbCheckNil(t *testing.T, value any) {
	t.Helper()

	if nbIsNil(value) {
		return
	}

	t.Errorf("expected nil: got %#v", value)
}

func nbRequireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if !nbIsNil(value) {
		return
	}

	t.Fatalf("%s: expected non-nil value", nbFailureMessage("expected non-nil value", msgAndArgs...))
}

func nbRequireLenOne(t *testing.T, value any) {
	t.Helper()

	actual, ok := nbValueLen(value)
	if ok && actual == 1 {
		return
	}

	t.Fatalf("length differs: expected length 1, got %d for %#v", actual, value)
}

func nbCheckTrue(t *testing.T, value bool, msgAndArgs ...any) bool {
	t.Helper()

	if value {
		return true
	}

	t.Errorf("%s: expected true", nbFailureMessage("expected true", msgAndArgs...))

	return false
}

func nbIsNil(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)

	kind := reflected.Kind().String()
	if kind == "chan" || kind == "func" || kind == "interface" || kind == "map" || kind == "ptr" || kind == "slice" {
		return reflected.IsNil()
	}

	return false
}

func nbZeroValue(value any) any {
	if value == nil {
		return nil
	}

	return reflect.Zero(reflect.TypeOf(value)).Interface()
}

func nbValueLen(value any) (int, bool) {
	if value == nil {
		return 0, false
	}

	reflected := reflect.ValueOf(value)

	kind := reflected.Kind().String()
	if kind == "array" || kind == "chan" || kind == "map" || kind == "slice" || kind == "string" {
		return reflected.Len(), true
	}

	return 0, false
}
