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

func TestClientAddImageShareGroupImagesSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/images/sharegroups/123/images", r.URL.Path, "request path should include share group ID and images suffix")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer "+"test-token", r.Header.Get("Authorization"))

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
			return
		}

		if !assert.Len(t, body["images"], 1) {
			return
		}

		image, ok := body["images"].([]any)[0].(map[string]any)
		if !assert.True(t, ok, "image payload should be an object") {
			return
		}

		assert.Equal(t, privateImage15Fixture, image[keyID], "image ID should be sent")
		assert.Equal(t, imageLinuxDebianFixture, image[keyLabel], "image label should be sent")
		assert.Equal(t, "Official Debian Linux image for server deployment", image[keyDescription], "image description should be sent")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:          sharedImage1Fixture,
			keyLabel:       imageLinuxDebianFixture,
			keyDescription: "Official Debian Linux image for server deployment",
			keyStatus:      imageStatusAvailableFixture,
			keyType:        "shared",
			"tags":         []string{"repair-image", "fix-1"},
			keyCreated:     "2025-08-04T10:07:59",
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	image, err := client.AddImageShareGroupImages(t.Context(), 123, &linode.AddImageShareGroupImagesRequest{
		Images: []linode.ImageShareGroupImage{{
			ID:          privateImage15Fixture,
			Label:       imageLinuxDebianFixture,
			Description: "Official Debian Linux image for server deployment",
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, image)
	assert.Equal(t, sharedImage1Fixture, image.ID, "image ID should match response")
	assert.Equal(t, int32(1), requestCount.Load(), "request should be sent once")
}

func TestClientAddImageShareGroupImagesNoRetry(t *testing.T) {
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
	image, err := client.AddImageShareGroupImages(t.Context(), 123, &linode.AddImageShareGroupImagesRequest{
		Images: []linode.ImageShareGroupImage{{ID: privateImage15Fixture}},
	})

	require.Error(t, err, "AddImageShareGroupImages should return the first transient error")
	assert.Nil(t, image, "image should be nil on error")
	assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent image share group image add must not be retried")
}
