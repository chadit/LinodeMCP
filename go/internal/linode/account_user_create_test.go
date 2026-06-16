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

const (
	accountUserCreateUsername       = "new-user"
	accountUserCreateEmail          = "new-user@example.com"
	temporaryAccountUserCreateError = "temporary account user create failure"
)

func TestClientCreateAccountUserSuccess(t *testing.T) {
	t.Parallel()

	request := &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail}
	created := linode.AccountUser{Username: request.Username, Email: request.Email, UserType: accountUserTypeDefault}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountUsers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountUsers)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		var got linode.CreateAccountUserRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(&got, request) {
			t.Errorf("got %v, want %v", &got, request)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(created); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountUser(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.Username != created.Username {
		t.Errorf("got.Username = %v, want %v", got.Username, created.Username)
	}

	if got.Email != created.Email {
		t.Errorf("got.Email = %v, want %v", got.Email, created.Email)
	}
}

func TestClientCreateAccountUserAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountUsers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountUsers)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	got, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientCreateAccountUserNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	networkErr, ok := errors.AsType[*linode.NetworkError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.NetworkError", err)
	}

	if networkErr.Operation != "CreateAccountUser" {
		t.Errorf("networkErr.Operation = %v, want %v", networkErr.Operation, "CreateAccountUser")
	}
}

func TestClientCreateAccountUserDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != tcAccountUsers {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcAccountUsers)
		}

		w.WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryAccountUserCreateError}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	_, err := client.CreateAccountUser(t.Context(), &linode.CreateAccountUserRequest{Username: accountUserCreateUsername, Email: accountUserCreateEmail})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
