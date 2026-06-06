package linode_test

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func longviewCheckEqual(t *testing.T, expected, actual any, msgAndArgs ...any) {
	t.Helper()

	if reflect.DeepEqual(expected, actual) {
		return
	}

	t.Errorf("%s: expected %#v, got %#v", longviewFailureMessage("values differ", msgAndArgs...), expected, actual)
}

func longviewCheckEmpty(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if value == nil {
		return
	}

	if length, ok := longviewValueLen(value); ok && length == 0 {
		return
	}

	if reflect.DeepEqual(value, reflect.Zero(reflect.TypeOf(value)).Interface()) {
		return
	}

	t.Errorf("%s: expected empty value, got %#v", longviewFailureMessage("value is not empty", msgAndArgs...), value)
}

func longviewCheckNoError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	t.Errorf("expected no error: unexpected error: %v", err)
}

func longviewRequireNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err == nil {
		return
	}

	t.Fatalf("%s: unexpected error: %v", longviewFailureMessage("expected no error", msgAndArgs...), err)
}

func longviewRequireError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()

	if err != nil {
		return
	}

	t.Fatalf("%s: expected error", longviewFailureMessage("expected error", msgAndArgs...))
}

func longviewRequireAPIError(t *testing.T, err error, msgAndArgs ...any) *linode.APIError {
	t.Helper()

	return requireAPIError(t, err, msgAndArgs...)
}

func longviewCheckNil(t *testing.T, value any) {
	t.Helper()

	if longviewIsNil(value) {
		return
	}

	t.Errorf("expected nil: got %#v", value)
}

func longviewRequireNotNil(t *testing.T, value any, msgAndArgs ...any) {
	t.Helper()

	if !longviewIsNil(value) {
		return
	}

	t.Fatalf("%s: expected non-nil value", longviewFailureMessage("expected non-nil value", msgAndArgs...))
}

func longviewRequireLenOne(t *testing.T, value any) {
	t.Helper()

	actual, ok := longviewValueLen(value)
	if ok && actual == 1 {
		return
	}

	t.Fatalf("length differs: expected length 1, got %d for %#v", actual, value)
}

func longviewCheckTrue(t *testing.T, value bool) {
	t.Helper()

	if value {
		return
	}

	t.Errorf("expected true")
}

func longviewCheckFalse(t *testing.T, value bool, msgAndArgs ...any) {
	t.Helper()

	if !value {
		return
	}

	t.Errorf("%s: expected false", longviewFailureMessage("expected false", msgAndArgs...))
}

func longviewCheckInEpsilon(t *testing.T, expected, actual float64) {
	t.Helper()

	const epsilon = 0.001

	tolerance := math.Abs(expected) * epsilon
	if tolerance == 0 {
		tolerance = epsilon
	}

	if math.Abs(expected-actual) <= tolerance {
		return
	}

	t.Errorf("values differ: expected %v to be within relative epsilon %v of %v", actual, epsilon, expected)
}

func longviewFailureMessage(defaultMessage string, msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return defaultMessage
	}

	format, ok := msgAndArgs[0].(string)
	if !ok {
		return fmt.Sprint(msgAndArgs...)
	}

	if len(msgAndArgs) == 1 {
		return format
	}

	return fmt.Sprintf(format, msgAndArgs[1:]...)
}

func longviewIsNil(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	kind := v.Kind()

	if kind == reflect.Chan || kind == reflect.Func || kind == reflect.Interface || kind == reflect.Map ||
		kind == reflect.Pointer || kind == reflect.Slice {
		return v.IsNil()
	}

	return false
}

func longviewValueLen(value any) (int, bool) {
	if value == nil {
		return 0, true
	}

	v := reflect.ValueOf(value)
	kind := v.Kind()

	if kind == reflect.Array || kind == reflect.Chan || kind == reflect.Map || kind == reflect.Slice || kind == reflect.String {
		return v.Len(), true
	}

	return 0, false
}

func TestClientCreateLongviewClientSuccess(t *testing.T) {
	t.Parallel()

	want := linode.CreatedLongviewClient{
		APIKey:      longviewClientAPIKey,
		Apps:        linode.LongviewApps{Apache: true, MySQL: true},
		Created:     longviewClientCreated,
		ID:          789,
		InstallCode: longviewClientInstallCode,
		Label:       longviewClientLabel,
		Updated:     longviewClientUpdated,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		longviewCheckEqual(t, "/longview/clients", r.URL.Path, "request path should match")
		longviewCheckEmpty(t, r.URL.RawQuery, "request query should be empty")
		longviewCheckEqual(t, "Bearer my-token", r.Header.Get("Authorization"))
		longviewCheckEqual(t, "application/json", r.Header.Get("Content-Type"))

		var got linode.CreateLongviewClientRequest
		longviewCheckNoError(t, json.NewDecoder(r.Body).Decode(&got))
		longviewCheckEqual(t, longviewClientLabel, got.Label)

		w.Header().Set("Content-Type", "application/json")
		longviewCheckNoError(t, json.NewEncoder(w).Encode(want))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	got, err := client.CreateLongviewClient(t.Context(), req)

	longviewRequireNoError(t, err, "CreateLongviewClient should succeed on 200 response")
	longviewRequireNotNil(t, got, "result should not be nil")
	longviewCheckEqual(t, want, *got)
}

func TestClientCreateLongviewClientAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		longviewCheckEqual(t, http.MethodPost, r.Method, "request method should be POST")
		longviewCheckEqual(t, "/longview/clients", r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		longviewCheckNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	_, err := client.CreateLongviewClient(t.Context(), req)

	longviewRequireError(t, err, "CreateLongviewClient should surface API errors")

	apiErr := longviewRequireAPIError(t, err, "error should be an APIError")
	longviewCheckEqual(t, errForbidden, apiErr.Message)
}

func TestClientCreateLongviewClientDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		http.Error(w, "transient", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)
	req := &linode.CreateLongviewClientRequest{Label: longviewClientLabel}

	_, err := client.CreateLongviewClient(t.Context(), req)

	longviewRequireError(t, err, "CreateLongviewClient should return the transient error")

	apiErr := longviewRequireAPIError(t, err, "error should be an APIError")
	longviewCheckEqual(t, http.StatusInternalServerError, apiErr.StatusCode)

	longviewCheckEqual(t, int32(1), requestCount.Load(), "mutating Longview client creation must not be retried")
}
