package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const tagLabelFixture = "production"

func TestClientListTagsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData:    []linode.Tag{{Label: tagLabelFixture}},
			keyPage:    2,
			keyPages:   3,
			keyResults: 51,
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListTags(t.Context(), 2, 25)

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, tagLabelFixture, result.Data[0].Label)
	assert.Equal(t, 2, result.Page)
	assert.Equal(t, 3, result.Pages)
	assert.Equal(t, 51, result.Results)
}

func TestClientListTagsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	result, err := client.ListTags(t.Context(), 0, 0)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestClientListTagsRetriesReadOnlyRoute(t *testing.T) {
	t.Parallel()

	var calls int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
			}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []linode.Tag{{Label: tagLabelFixture}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	result, err := client.ListTags(t.Context(), 0, 0)

	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, int32(2), calls, "read-only GET route may retry transient failures")
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
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/tags/prod%2Fweb", r.URL.EscapedPath(), "request path should URL-encode tag label")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(objects))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListTaggedObjects(t.Context(), "prod/web", 2, 25)

	require.NoError(t, err, "ListTaggedObjects should succeed on 200 response")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, 2, result.Page)
	require.Len(t, result.Data, 1)
	assert.Equal(t, nodeLabelWeb1, result.Data[0][keyLabel])
	assert.Equal(t, managedLinodeSettingsSSHUser, result.Data[0][keyType])
}

func TestClientListTaggedObjectsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/tags/prod", r.URL.Path, "request path should be /tags/prod")
		assert.Empty(t, r.URL.RawQuery, "omitted pagination should not include query parameters")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		assert.NoError(t, writeErr)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListTaggedObjects(t.Context(), "prod", 0, 0)

	require.Error(t, err, "ListTaggedObjects should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListTaggedObjectsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, writeErr)

			return
		}

		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, "/tags/prod", r.URL.Path, "request path should be /tags/prod")
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.TaggedObject]{
			Data: []linode.TaggedObject{{keyID: float64(123), keyLabel: nodeLabelWeb1}},
		}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListTaggedObjects(t.Context(), "prod", 0, 0)

	require.NoError(t, err, "ListTaggedObjects should succeed after retry")
	require.NotNil(t, result, "result should not be nil")
	require.Len(t, result.Data, 1)
	assert.Equal(t, nodeLabelWeb1, result.Data[0][keyLabel])
	assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
}

func TestClientCreateTagSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/tags", r.URL.Path, "request path should be /tags")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		assert.Equal(t, tagLabelFixture, body[keyLabel], "label should be sent")
		assert.Equal(t, []any{float64(101), float64(102)}, body["linodes"], "linode IDs should be sent")
		assert.Equal(t, []any{float64(201)}, body["domains"], "domain IDs should be sent")
		assert.Equal(t, []any{float64(301)}, body["nodebalancers"], "nodebalancer IDs should be sent")
		assert.Equal(t, []any{float64(401)}, body["volumes"], "volume IDs should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Tag{Label: tagLabelFixture}))
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

	require.NoError(t, err)
	require.NotNil(t, tag)
	assert.Equal(t, tagLabelFixture, tag.Label, "tag label should match response")
	assert.Equal(t, int32(1), requestCount.Load(), "request should be sent once")
}

func TestClientCreateTagNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	client := linode.NewClient(url, "test-token", nil, linode.WithMaxRetries(0))
	tag, err := client.CreateTag(t.Context(), &linode.CreateTagRequest{Label: tagLabelFixture})

	require.Error(t, err, "CreateTag should fail when the server is unreachable")
	assert.Nil(t, tag)

	var netErr *linode.NetworkError
	require.ErrorAs(t, err, &netErr, "error should be a NetworkError")
	require.NotNil(t, netErr, "NetworkError should be present")
	assert.Equal(t, "CreateTag", netErr.Operation)
}

func TestClientCreateTagNoRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
		assert.NoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
	tag, err := client.CreateTag(t.Context(), &linode.CreateTagRequest{Label: tagLabelFixture})

	require.Error(t, err, "CreateTag should return the first transient error")
	assert.Nil(t, tag, "tag should be nil on error")
	assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent tag creation must not be retried")
}

func TestClientDeleteTagSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/tags/prod%2Fweb", r.URL.EscapedPath(), "request path should URL-encode tag label")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteTag(t.Context(), "prod/web")

	require.NoError(t, err, "DeleteTag should succeed on 200 response")
}

func TestClientDeleteTagAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
		assert.Equal(t, "/tags/prod", r.URL.Path, "request path should be /tags/prod")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteTag(t.Context(), "prod")

	require.Error(t, err, "DeleteTag should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should wrap APIError")
	require.NotNil(t, apiErr, "APIError should be present")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientDeleteTagNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	client := linode.NewClient(url, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DeleteTag(t.Context(), "prod")

	require.Error(t, err, "DeleteTag should fail when the server is unreachable")

	var netErr *linode.NetworkError
	require.ErrorAs(t, err, &netErr, "error should be a NetworkError")
	require.NotNil(t, netErr, "NetworkError should be present")
	assert.Equal(t, "DeleteTag", netErr.Operation)
}

func TestClientDeleteTagDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	err := client.DeleteTag(t.Context(), "prod")

	require.Error(t, err, "DeleteTag should return the transient failure")
	assert.Equal(t, int32(1), requestCount.Load(), "destructive DELETE must not be retried")
}
