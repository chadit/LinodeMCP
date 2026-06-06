package tools_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func imageAssertMessage(msgAndArgs []any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}

	format, ok := msgAndArgs[0].(string)
	if ok && len(msgAndArgs) > 1 {
		return ": " + fmt.Sprintf(format, msgAndArgs[1:]...)
	}

	return ": " + fmt.Sprint(msgAndArgs...)
}

func assertEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("not equal: expected %#v, got %#v%s", expected, actual, imageAssertMessage(msgAndArgs))
}

func assertTrue(t *testing.T, value bool, msgAndArgs ...any) bool {
	t.Helper()

	if value {
		return true
	}

	t.Errorf("expected true%s", imageAssertMessage(msgAndArgs))

	return false
}

func requireTrue(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()

	if value {
		return
	}

	t.Fatalf("expected true%s", imageAssertMessage(msgAndArgs))
}

func assertFalse(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()

	if !value {
		return
	}

	t.Errorf("expected false%s", imageAssertMessage(msgAndArgs))
}

func requireFalse(t *testing.T, value bool) {
	t.Helper()

	if !value {
		return
	}

	t.Fatal("expected false")
}

func assertNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if isNil(value) {
		return
	}

	t.Errorf("expected nil, got %#v%s", value, imageAssertMessage(msgAndArgs))
}

func requireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if !isNil(value) {
		return
	}

	t.Fatalf("expected non-nil%s", imageAssertMessage(msgAndArgs))
}

func assertNoError(t *testing.T, err error, msgAndArgs ...any) bool {
	t.Helper()

	if err == nil {
		return true
	}

	t.Errorf("unexpected error: %v%s", err, imageAssertMessage(msgAndArgs))

	return false
}

func requireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf("unexpected error: %v%s", err, imageAssertMessage(msgAndArgs))
}

func assertContains(t *testing.T, collection, contains any, msgAndArgs ...any) {
	t.Helper()

	if containsValue(collection, contains) {
		return
	}

	t.Errorf("%#v does not contain %#v%s", collection, contains, imageAssertMessage(msgAndArgs))
}

func assertNotContains(t *testing.T, collection, contains any, msgAndArgs ...any) {
	t.Helper()

	if !containsValue(collection, contains) {
		return
	}

	t.Errorf("%#v unexpectedly contains %#v%s", collection, contains, imageAssertMessage(msgAndArgs))
}

func assertEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if isEmpty(value) {
		return
	}

	t.Errorf("expected empty, got %#v%s", value, imageAssertMessage(msgAndArgs))
}

func assertNotEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if !isEmpty(value) {
		return
	}

	t.Errorf("expected non-empty%s", imageAssertMessage(msgAndArgs))
}

func assertLen(t *testing.T, value any, length int, msgAndArgs ...any) bool {
	t.Helper()

	actual, ok := valueLen(value)
	if ok && actual == length {
		return true
	}

	if !ok {
		t.Errorf("value has no length: %#v%s", value, imageAssertMessage(msgAndArgs))

		return false
	}

	t.Errorf("unexpected length: expected %d, got %d%s", length, actual, imageAssertMessage(msgAndArgs))

	return false
}

func requireLen(t *testing.T, value any, length int, msgAndArgs ...any) {
	t.Helper()

	actual, ok := valueLen(value)
	if ok && actual == length {
		return
	}

	if !ok {
		t.Fatalf("value has no length: %#v%s", value, imageAssertMessage(msgAndArgs))
	}

	t.Fatalf("unexpected length: expected %d, got %d%s", length, actual, imageAssertMessage(msgAndArgs))
}

func containsValue(collection, contains any) bool {
	if haystack, ok := collection.(string); ok {
		needle, ok := contains.(string)

		return ok && strings.Contains(haystack, needle)
	}

	collectionValue := reflect.ValueOf(collection)
	if !collectionValue.IsValid() {
		return false
	}

	if (collectionValue.Kind() == reflect.Pointer || collectionValue.Kind() == reflect.Interface) && !collectionValue.IsNil() {
		collectionValue = collectionValue.Elem()
	}

	if collectionValue.Kind() == reflect.Map {
		key := reflect.ValueOf(contains)
		if !key.IsValid() {
			return false
		}

		if !key.Type().AssignableTo(collectionValue.Type().Key()) {
			if !key.Type().ConvertibleTo(collectionValue.Type().Key()) {
				return false
			}

			key = key.Convert(collectionValue.Type().Key())
		}

		return collectionValue.MapIndex(key).IsValid()
	}

	if collectionValue.Kind() == reflect.Slice || collectionValue.Kind() == reflect.Array {
		// The module targets Go 1.26, so integer range is supported here.
		for index := range collectionValue.Len() {
			if reflect.DeepEqual(collectionValue.Index(index).Interface(), contains) {
				return true
			}
		}
	}

	return false
}

func isEmpty(value any) bool {
	if isNil(value) {
		return true
	}

	valueReflect := reflect.ValueOf(value)
	valueKind := valueReflect.Kind()

	if valueKind == reflect.Array || valueKind == reflect.Chan || valueKind == reflect.Map || valueKind == reflect.Slice || valueKind == reflect.String {
		return valueReflect.Len() == 0
	}

	if valueKind == reflect.Bool {
		return !valueReflect.Bool()
	}

	if valueKind >= reflect.Int && valueKind <= reflect.Int64 {
		return valueReflect.Int() == 0
	}

	if valueKind >= reflect.Uint && valueKind <= reflect.Uintptr {
		return valueReflect.Uint() == 0
	}

	if valueKind == reflect.Float32 || valueKind == reflect.Float64 {
		return valueReflect.Float() == 0
	}

	return reflect.DeepEqual(value, reflect.Zero(valueReflect.Type()).Interface())
}

func isNil(value any) bool {
	if value == nil {
		return true
	}

	valueReflect := reflect.ValueOf(value)
	valueKind := valueReflect.Kind()

	return (valueKind == reflect.Chan || valueKind == reflect.Func || valueKind == reflect.Interface || valueKind == reflect.Map || valueKind == reflect.Pointer || valueKind == reflect.Slice) && valueReflect.IsNil()
}

func valueLen(value any) (int, bool) {
	valueReflect := reflect.ValueOf(value)
	if !valueReflect.IsValid() {
		return 0, false
	}

	valueKind := valueReflect.Kind()
	if valueKind == reflect.Array || valueKind == reflect.Chan || valueKind == reflect.Map || valueKind == reflect.Slice || valueKind == reflect.String {
		return valueReflect.Len(), true
	}

	return 0, false
}
