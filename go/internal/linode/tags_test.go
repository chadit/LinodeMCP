package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const tagLabelFixture = "production"

func TestClientListTagsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/tags" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/tags")
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.Tag{{Label: tagLabelFixture}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListTags(t.Context(), 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if result.Data[0].Label != tagLabelFixture {
		t.Errorf("result.Data[0].Label = %v, want %v", result.Data[0].Label, tagLabelFixture)
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if result.Pages != 3 {
		t.Errorf("result.Pages = %v, want %v", result.Pages, 3)
	}

	if result.Results != 51 {
		t.Errorf("result.Results = %v, want %v", result.Results, 51)
	}
}

func TestClientListTagsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListTags(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientListTagsRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Tag{{Label: tagLabelFixture}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))

	result, err := client.ListTags(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if calls != int32(2) {
		t.Errorf("calls = %v, want %v", calls, int32(2))
	}
}

func TestClientListTaggedObjectsSuccess(t *testing.T) {
	t.Parallel()

	objects := linode.PaginatedResponse[linode.TaggedObject]{
		Data: []linode.TaggedObject{{
			keyID:    float64(123),
			keyLabel: nodeLabelWeb1,
			keyType:  managedLinodeSettingsSSHUser,
			"url":    "/v4/linode/instances/123",
		}},
		Page:    2,
		Pages:   3,
		Results: 75,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.EscapedPath() != "/tags/prod%2Fweb" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/tags/prod%2Fweb")
		}

		if r.URL.RawQuery != longviewSubscriptionsQuery {
			t.Errorf("r.URL.RawQuery = %v, want %v", r.URL.RawQuery, longviewSubscriptionsQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(objects); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListTaggedObjects(t.Context(), "prod/web", 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Page != 2 {
		t.Errorf("result.Page = %v, want %v", result.Page, 2)
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if !reflect.DeepEqual(result.Data[0][keyLabel], nodeLabelWeb1) {
		t.Errorf("result.Data[0][keyLabel] = %v, want %v", result.Data[0][keyLabel], nodeLabelWeb1)
	}

	if !reflect.DeepEqual(result.Data[0][keyType], managedLinodeSettingsSSHUser) {
		t.Errorf("result.Data[0][keyType] = %v, want %v", result.Data[0][keyType], managedLinodeSettingsSSHUser)
	}
}

func TestClientListTaggedObjectsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcTagsProd {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTagsProd)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListTaggedObjects(t.Context(), "prod", 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientListTaggedObjectsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if writeErr != nil {
				t.Errorf("unexpected error: %v", writeErr)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcTagsProd {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTagsProd)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.TaggedObject]{
			Data: []linode.TaggedObject{{keyID: float64(123), keyLabel: nodeLabelWeb1}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListTaggedObjects(t.Context(), "prod", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if len(result.Data) != 1 {
		t.Fatalf("len(result.Data) = %d, want 1", len(result.Data))
	}

	if !reflect.DeepEqual(result.Data[0][keyLabel], nodeLabelWeb1) {
		t.Errorf("result.Data[0][keyLabel] = %v, want %v", result.Data[0][keyLabel], nodeLabelWeb1)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientCreateTagSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/tags" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/tags")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], tagLabelFixture) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], tagLabelFixture)
		}

		if !reflect.DeepEqual(body["linodes"], []any{float64(101), float64(102)}) {
			t.Errorf("got %v, want %v", body["linodes"], []any{float64(101), float64(102)})
		}

		if !reflect.DeepEqual(body["domains"], []any{float64(201)}) {
			t.Errorf("got %v, want %v", body["domains"], []any{float64(201)})
		}

		if !reflect.DeepEqual(body["nodebalancers"], []any{float64(301)}) {
			t.Errorf("got %v, want %v", body["nodebalancers"], []any{float64(301)})
		}

		if !reflect.DeepEqual(body["volumes"], []any{float64(401)}) {
			t.Errorf("got %v, want %v", body["volumes"], []any{float64(401)})
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Tag{Label: tagLabelFixture}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	tag, err := client.CreateTag(t.Context(), &linode.CreateTagRequest{
		Label:         tagLabelFixture,
		Domains:       []int{201},
		Linodes:       []int{101, 102},
		NodeBalancers: []int{301},
		Volumes:       []int{401},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tag == nil {
		t.Fatal("tag is nil")
	}

	if tag.Label != tagLabelFixture {
		t.Errorf("tag.Label = %v, want %v", tag.Label, tagLabelFixture)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientCreateTagNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	client := linode.NewClient(url, "test-token", nil, linode.WithMaxRetries(0))

	tag, err := client.CreateTag(t.Context(), &linode.CreateTagRequest{Label: tagLabelFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if tag != nil {
		t.Errorf("tag = %v, want nil", tag)
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.NetworkError", err)
	}

	if netErr.Operation != "CreateTag" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "CreateTag")
	}
}

func TestClientCreateTagNoRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	tag, err := client.CreateTag(t.Context(), &linode.CreateTagRequest{Label: tagLabelFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if tag != nil {
		t.Errorf("tag = %v, want nil", tag)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientDeleteTagSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.EscapedPath() != "/tags/prod%2Fweb" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/tags/prod%2Fweb")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteTag(t.Context(), "prod/web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDeleteTagAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

		if r.URL.Path != tcTagsProd {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcTagsProd)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteTag(t.Context(), "prod")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientDeleteTagNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	client := linode.NewClient(url, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteTag(t.Context(), "prod")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	netErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.NetworkError", err)
	}

	if netErr.Operation != "DeleteTag" {
		t.Errorf("netErr.Operation = %v, want %v", netErr.Operation, "DeleteTag")
	}
}

func TestClientDeleteTagDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteTag(t.Context(), "prod")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
