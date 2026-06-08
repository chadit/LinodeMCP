package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestClientAddImageShareGroupImagesSuccess(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/sharegroups/123/images" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123/images")
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

		images, isList := body["images"].([]any)
		if !isList || len(images) != 1 {
			t.Errorf(`body["images"] = %v, want one element`, body["images"])

			return
		}

		image, ok := images[0].(map[string]any)
		if !ok {
			t.Error("image payload should be an object")

			return
		}

		if !reflect.DeepEqual(image[keyID], privateImage15Fixture) {
			t.Errorf("image[keyID] = %v, want %v", image[keyID], privateImage15Fixture)
		}

		if !reflect.DeepEqual(image[keyLabel], imageLinuxDebianFixture) {
			t.Errorf("image[keyLabel] = %v, want %v", image[keyLabel], imageLinuxDebianFixture)
		}

		if !reflect.DeepEqual(image[keyDescription], "Official Debian Linux image for server deployment") {
			t.Errorf("image[keyDescription] = %v, want %v", image[keyDescription], "Official Debian Linux image for server deployment")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          sharedImage1Fixture,
			keyLabel:       imageLinuxDebianFixture,
			keyDescription: "Official Debian Linux image for server deployment",
			keyStatus:      imageStatusAvailableFixture,
			keyType:        "shared",
			"tags":         []string{"repair-image", "fix-1"},
			keyCreated:     "2025-08-04T10:07:59",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if image == nil {
		t.Fatal("image is nil")
	}

	if image.ID != sharedImage1Fixture {
		t.Errorf("image.ID = %v, want %v", image.ID, sharedImage1Fixture)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientAddImageShareGroupImagesNoRetry(t *testing.T) {
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

	image, err := client.AddImageShareGroupImages(t.Context(), 123, &linode.AddImageShareGroupImagesRequest{
		Images: []linode.ImageShareGroupImage{{ID: privateImage15Fixture}},
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if image != nil {
		t.Errorf("image = %v, want nil", image)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
