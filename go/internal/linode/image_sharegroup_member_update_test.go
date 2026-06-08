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

const imageShareGroupMemberUpdateLabel = "Engineering - Backend"

func TestClientUpdateImageShareGroupMemberSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/images/sharegroups/123/members/"+shareGroupUUIDExample {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/123/members/"+shareGroupUUIDExample)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyLabel], imageShareGroupMemberUpdateLabel) {
			t.Errorf("body[keyLabel] = %v, want %v", body[keyLabel], imageShareGroupMemberUpdateLabel)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroupMember{
			TokenUUID: shareGroupUUIDExample,
			Status:    oauthClientStatus,
			Label:     imageShareGroupMemberUpdateLabel,
			Created:   imageShareGroupTokenCreated,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	member, err := client.UpdateImageShareGroupMember(t.Context(), 123, shareGroupUUIDExample, &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if member == nil {
		t.Fatal("member is nil")
	}

	if member.TokenUUID != shareGroupUUIDExample {
		t.Errorf("member.TokenUUID = %v, want %v", member.TokenUUID, shareGroupUUIDExample)
	}

	if member.Label != imageShareGroupMemberUpdateLabel {
		t.Errorf("member.Label = %v, want %v", member.Label, imageShareGroupMemberUpdateLabel)
	}
}

func TestClientUpdateImageShareGroupMemberEscapesTokenUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.EscapedPath() != "/images/sharegroups/123/members/token%2Fuuid%3Fquery" {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), "/images/sharegroups/123/members/token%2Fuuid%3Fquery")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: "token/uuid?query", Label: imageShareGroupMemberUpdateLabel}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	_, err := client.UpdateImageShareGroupMember(t.Context(), 123, "token/uuid?query", &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientUpdateImageShareGroupMemberEscapesDotSegments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		tokenUUID   string
		escapedPath string
	}{
		{name: "single dot", tokenUUID: ".", escapedPath: "/images/sharegroups/123/members/%2E"},
		{name: "double dot", tokenUUID: pathTraversalDotDot, escapedPath: "/images/sharegroups/123/members/%2E%2E"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
				}

				if r.URL.EscapedPath() != testCase.escapedPath {
					t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), testCase.escapedPath)
				}

				w.Header().Set("Content-Type", tcApplicationJSON)

				if err := json.NewEncoder(w).Encode(linode.ImageShareGroupMember{TokenUUID: testCase.tokenUUID, Label: imageShareGroupMemberUpdateLabel}); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

			_, err := client.UpdateImageShareGroupMember(t.Context(), 123, testCase.tokenUUID, &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUpdateImageShareGroupMemberNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"try again"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	_, err := client.UpdateImageShareGroupMember(t.Context(), 123, shareGroupUUIDExample, &linode.UpdateImageShareGroupMemberRequest{Label: imageShareGroupMemberUpdateLabel})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
