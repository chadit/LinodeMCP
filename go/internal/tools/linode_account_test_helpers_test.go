package tools_test

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

type accountAssert struct{}

type accountRequire struct{}

func accountMsg(args []any) string {
	if len(args) == 0 {
		return ""
	}

	format, ok := args[0].(string)
	if !ok {
		return ": " + fmt.Sprint(args...)
	}

	if len(args) > 1 {
		return ": " + fmt.Sprintf(format, args[1:]...)
	}

	return ": " + format
}

func (accountAssert) Equal(t *testing.T, want, got any, args ...any) bool {
	t.Helper()

	if reflect.DeepEqual(want, got) {
		return true
	}

	t.Errorf("values differ%s\nwant: %#v\n got: %#v", accountMsg(args), want, got)

	return false
}

func (accountAssert) True(t *testing.T, got bool, args ...any) bool {
	t.Helper()

	if got {
		return true
	}

	t.Errorf("value is false%s", accountMsg(args))

	return false
}

func (accountAssert) False(t *testing.T, got bool, args ...any) bool {
	t.Helper()

	if !got {
		return true
	}

	t.Errorf("value is true%s", accountMsg(args))

	return false
}

func (accountRequire) True(t *testing.T, got bool, args ...any) {
	t.Helper()

	if !got {
		t.Fatalf("value is false%s", accountMsg(args))
	}
}

func (accountRequire) False(t *testing.T, got bool, args ...any) {
	t.Helper()

	if got {
		t.Fatalf("value is true%s", accountMsg(args))
	}
}

func (accountAssert) NoError(t *testing.T, err error, args ...any) bool {
	t.Helper()

	if err == nil {
		return true
	}

	t.Errorf("unexpected error%s: %v", accountMsg(args), err)

	return false
}

func (accountRequire) NoError(t *testing.T, err error, args ...any) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error%s: %v", accountMsg(args), err)
	}
}

func (accountAssert) Nil(t *testing.T, got any, args ...any) bool {
	t.Helper()

	if accountNil(got) {
		return true
	}

	t.Errorf("value is not nil%s: %#v", accountMsg(args), got)

	return false
}

func (accountRequire) NotNil(t *testing.T, got any, args ...any) {
	t.Helper()

	if accountNil(got) {
		t.Fatalf("value is nil%s", accountMsg(args))
	}
}

func (accountAssert) Empty(t *testing.T, got any, args ...any) bool {
	t.Helper()

	if accountEmpty(got) {
		return true
	}

	t.Errorf("value is not empty%s: %#v", accountMsg(args), got)

	return false
}

func (accountAssert) NotEmpty(t *testing.T, got any, args ...any) bool {
	t.Helper()

	if !accountEmpty(got) {
		return true
	}

	t.Errorf("value is empty%s", accountMsg(args))

	return false
}

func (accountRequire) Len(t *testing.T, got any, want int, args ...any) {
	t.Helper()

	gotLen, ok := accountLen(got)
	if !ok {
		t.Fatalf("value has no length%s: %#v", accountMsg(args), got)
	}

	if gotLen != want {
		t.Fatalf("length differs%s\nwant: %d\n got: %d", accountMsg(args), want, gotLen)
	}
}

func (accountAssert) Contains(t *testing.T, collection, item any, args ...any) bool {
	t.Helper()

	if accountContains(collection, item) {
		return true
	}

	t.Errorf("value does not contain item%s\nvalue: %#v\n item: %#v", accountMsg(args), collection, item)

	return false
}

func (accountAssert) NotContains(t *testing.T, collection, item any, args ...any) bool {
	t.Helper()

	if !accountContains(collection, item) {
		return true
	}

	t.Errorf("value contains item%s\nvalue: %#v\n item: %#v", accountMsg(args), collection, item)

	return false
}

func (accountAssert) InDelta(t *testing.T, want, got any, delta float64, args ...any) bool {
	t.Helper()

	wantFloat, wantOK := accountFloat(want)
	gotFloat, gotOK := accountFloat(got)

	if !wantOK || !gotOK {
		t.Errorf("values are not numeric%s\nwant: %#v\n got: %#v", accountMsg(args), want, got)

		return false
	}

	if math.Abs(wantFloat-gotFloat) <= delta {
		return true
	}

	t.Errorf("values differ by more than delta%s\nwant: %v\n got: %v\ndelta: %v", accountMsg(args), want, got, delta)

	return false
}

func accountFloat(value any) (float64, bool) {
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

func accountNil(value any) bool {
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

func accountEmpty(value any) bool {
	if accountNil(value) {
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

func accountLen(value any) (int, bool) {
	if accountNil(value) {
		return 0, true
	}

	reflected := reflect.ValueOf(value)
	kind := reflected.Kind()

	if kind == reflect.Array || kind == reflect.Map || kind == reflect.Slice || kind == reflect.String {
		return reflected.Len(), true
	}

	return 0, false
}

func accountContains(collection, item any) bool {
	if collection == nil {
		return false
	}

	text, ok := collection.(string)
	if ok {
		return strings.Contains(text, fmt.Sprint(item))
	}

	reflected := reflect.ValueOf(collection)
	kind := reflected.Kind()

	if kind == reflect.Map {
		key := reflect.ValueOf(item)
		if key.IsValid() && key.Type().AssignableTo(reflected.Type().Key()) {
			return reflected.MapIndex(key).IsValid()
		}

		return false
	}

	if kind != reflect.Array && kind != reflect.Slice {
		return false
	}

	for index := range reflected.Len() {
		if reflect.DeepEqual(reflected.Index(index).Interface(), item) {
			return true
		}
	}

	return false
}
