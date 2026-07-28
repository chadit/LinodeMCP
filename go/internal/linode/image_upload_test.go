package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientUploadImageProtoSuccess covers the two-part body this endpoint
// answers with. The upload URL is a one-time credential the caller cannot
// re-request, so it has to come back alongside the created image rather than
// being dropped when only the image sub-object is decoded.
func TestClientUploadImageProtoSuccess(t *testing.T) {
	t.Parallel()

	const uploadURL = "https://upload.linode.com/objects/one-time"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/upload" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/upload")
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"upload_to": uploadURL,
			"image": map[string]any{
				keyID:     privateImage15Fixture,
				keyLabel:  imageLinuxDebianFixture,
				keyStatus: "pending_upload",
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	image, uploadTo, err := client.UploadImageProto(t.Context(), &linode.UploadImageRequest{
		Label:  imageLinuxDebianFixture,
		Region: regionUSEast,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if uploadTo != uploadURL {
		t.Errorf("uploadTo = %v, want %v", uploadTo, uploadURL)
	}

	if image == nil {
		t.Fatal("image is nil")
	}

	if image.GetId() != privateImage15Fixture {
		t.Errorf("image.GetId() = %v, want %v", image.GetId(), privateImage15Fixture)
	}

	if image.GetLabel() != imageLinuxDebianFixture {
		t.Errorf("image.GetLabel() = %v, want %v", image.GetLabel(), imageLinuxDebianFixture)
	}
}

// TestClientUploadImageProtoError proves a refusal reaches the caller instead of
// an empty image with a blank upload URL, which would look like a successful
// upload target the caller could never write to.
func TestClientUploadImageProtoError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "Label must be unique"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	image, uploadTo, err := client.UploadImageProto(t.Context(), &linode.UploadImageRequest{
		Label:  imageLinuxDebianFixture,
		Region: regionUSEast,
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if image != nil {
		t.Errorf("image = %+v, want nil", image)
	}

	if uploadTo != "" {
		t.Errorf("uploadTo = %v, want empty", uploadTo)
	}
}
