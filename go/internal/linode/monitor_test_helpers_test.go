package linode_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const monitorNumericTolerance = 0.001

func monitorCheckEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("%s: expected %#v, got %#v", monitorFailureMessage("values differ", msgAndArgs...), expected, actual)
}

func monitorCheckEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if length, ok := monitorValueLen(value); ok && length == 0 {
		return
	}

	if reflect.DeepEqual(value, monitorZeroValue(value)) {
		return
	}

	t.Errorf("%s: expected empty value, got %#v", monitorFailureMessage("value is not empty", msgAndArgs...), value)
}

func monitorCheckNoError(t *testing.T, err error) bool {
	t.Helper()

	if err == nil {
		return true
	}

	t.Errorf("expected no error: unexpected error: %v", err)

	return false
}

func monitorRequireNoError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf("expected no error: unexpected error: %v", err)
}

func monitorRequireNotNil(t *testing.T, value any) {
	t.Helper()

	if !monitorIsNil(value) {
		return
	}

	t.Fatalf("expected non-nil value")
}

func monitorRequireLenOne(t *testing.T, value any) {
	t.Helper()

	actual, ok := monitorValueLen(value)
	if ok && actual == 1 {
		return
	}

	t.Fatalf("length differs: expected length 1, got %d for %#v", actual, value)
}

func monitorRequireError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		return
	}

	t.Fatalf("expected error")
}

func monitorRequireAPIError(t *testing.T, err error) *linode.APIError {
	t.Helper()

	var apiErr *linode.APIError

	asError := errors.As
	if asError(err, &apiErr) {
		return apiErr
	}

	t.Fatalf("expected API error, got %v", err)

	return nil
}

func monitorCheckNil(t *testing.T, value any) {
	t.Helper()

	if monitorIsNil(value) {
		return
	}

	t.Errorf("expected nil, got %#v", value)
}

func monitorCheckHasKey(t *testing.T, values map[string]any, key string) {
	t.Helper()

	if _, ok := values[key]; ok {
		return
	}

	t.Errorf("missing key %q in %#v", key, values)
}

func monitorCheckNumericClose(t *testing.T, expected, actual any) {
	t.Helper()

	expectedFloat, expectedOK := monitorNumericValue(expected)
	actualFloat, actualOK := monitorNumericValue(actual)

	if !expectedOK || !actualOK {
		t.Errorf("values differ: expected numeric %#v, got %#v", expected, actual)

		return
	}

	tolerance := math.Abs(expectedFloat) * monitorNumericTolerance
	if expectedFloat == 0 {
		tolerance = monitorNumericTolerance
	}

	if math.Abs(expectedFloat-actualFloat) <= tolerance {
		return
	}

	t.Errorf("values differ: expected %#v, got %#v within relative tolerance %g", expected, actual, monitorNumericTolerance)
}

func monitorCheckNumericEqual(t *testing.T, expected, actual any) {
	t.Helper()

	expectedFloat, expectedOK := monitorNumericValue(expected)
	actualFloat, actualOK := monitorNumericValue(actual)

	if expectedOK && actualOK && expectedFloat == actualFloat {
		return
	}

	t.Errorf("values differ: expected %#v, got %#v", expected, actual)
}

func monitorCheckJSONEqual(t *testing.T, expected, actual string, msgAndArgs ...any) {
	t.Helper()

	var expectedJSON any
	if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
		t.Errorf("%s: invalid expected JSON: %v", monitorFailureMessage("invalid expected JSON", msgAndArgs...), err)

		return
	}

	var actualJSON any
	if err := json.Unmarshal([]byte(actual), &actualJSON); err != nil {
		t.Errorf("%s: invalid actual JSON: %v", monitorFailureMessage("invalid actual JSON", msgAndArgs...), err)

		return
	}

	monitorCheckEqual(t, expectedJSON, actualJSON, msgAndArgs...)
}

func monitorFailureMessage(defaultMsg string, msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return defaultMsg
	}

	msg, ok := msgAndArgs[0].(string)
	if !ok || msg == "" {
		return defaultMsg
	}

	if len(msgAndArgs) > 1 {
		return fmt.Sprintf(msg, msgAndArgs[1:]...)
	}

	return msg
}

func monitorIsNil(value any) bool {
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

func monitorZeroValue(value any) any {
	if value == nil {
		return nil
	}

	return reflect.Zero(reflect.TypeOf(value)).Interface()
}

func monitorValueLen(value any) (int, bool) {
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

func monitorNumericValue(value any) (float64, bool) {
	switch typedValue := value.(type) {
	case int:
		return float64(typedValue), true
	case int8:
		return float64(typedValue), true
	case int16:
		return float64(typedValue), true
	case int32:
		return float64(typedValue), true
	case int64:
		return float64(typedValue), true
	case uint:
		return float64(typedValue), true
	case uint8:
		return float64(typedValue), true
	case uint16:
		return float64(typedValue), true
	case uint32:
		return float64(typedValue), true
	case uint64:
		return float64(typedValue), true
	case float32:
		return float64(typedValue), true
	case float64:
		return typedValue, true
	default:
		return 0, false
	}
}
