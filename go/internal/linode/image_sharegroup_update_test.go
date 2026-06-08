package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	imageShareGroupID            = 54321
	updateImageShareGroupUpdated = "2025-04-16T22:44:02"
	updateImageShareGroupDesc    = "shared base images"
	updateImageShareGroupCreated = "2025-04-14T22:44:02"
)

func TestClientUpdateImageShareGroupSuccess(t *testing.T) {
	t.Parallel()

	label := shareGroupLabelFixture
	description := updateImageShareGroupDesc
	request := &linode.UpdateImageShareGroupRequest{Label: &label, Description: &description}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/images/sharegroups/54321" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/54321")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != "Bearer "+"test-token" {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), "Bearer "+"test-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		if !reflect.DeepEqual(body[keyLabel], label) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], label)
		}

		if !reflect.DeepEqual(body[keyDescription], description) {
			t.Errorf("body[keyDescription] = %v, want %v", body[keyDescription], description)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroup{
			ID:           imageShareGroupID,
			UUID:         shareGroupUUIDFixture,
			Label:        label,
			Description:  &description,
			IsSuspended:  false,
			Created:      updateImageShareGroupCreated,
			Updated:      &[]string{updateImageShareGroupUpdated}[0],
			ImagesCount:  2,
			MembersCount: 3,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != imageShareGroupID {
		t.Errorf("result.ID = %v, want %v", result.ID, imageShareGroupID)
	}

	if result.Label != label {
		t.Errorf("result.Label = %v, want %v", result.Label, label)
	}
}

func TestClientUpdateImageShareGroupAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: "label is invalid"}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != "label is invalid" {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, "label is invalid")
	}
}

func TestClientUpdateImageShareGroupNetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client := linode.NewClient(baseURL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.NetworkError", err)
	}

	if networkErr.Operation != "UpdateImageShareGroup" {
		t.Errorf("networkErr.Operation = %v, want %v", networkErr.Operation, "UpdateImageShareGroup")
	}
}

func TestClientUpdateImageShareGroupDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errTemporaryFailure}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.UpdateImageShareGroup(t.Context(), imageShareGroupID, &linode.UpdateImageShareGroupRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
