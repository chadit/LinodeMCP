package linode_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const longviewClientUpdateProtoID = 789

// TestClientUpdateLongviewClientProtoSuccess verifies UpdateLongviewClientProto
// sends a PUT to /longview/clients/{clientId} carrying the new label and decodes
// the response into the proto LongviewClient element.
func TestClientUpdateLongviewClientProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var sent map[string]any
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if sent[keyLabel] != longviewClientUpdatedLabel {
			t.Errorf("sent[%v] = %v, want %v", keyLabel, sent[keyLabel], longviewClientUpdatedLabel)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		// The apps object omits the MySQL flag on purpose: the API leaves out
		// app flags for services it did not detect, and the decoded element has
		// to report those as false rather than dropping the apps object.
		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:      longviewClientUpdateProtoID,
			keyLabel:   longviewClientUpdatedLabel,
			keyCreated: longviewClientCreated,
			keyUpdated: longviewClientUpdated,
			"apps":     map[string]any{"apache": true, "nginx": true},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	label := longviewClientUpdatedLabel

	got, err := client.UpdateLongviewClientProto(t.Context(), longviewClientUpdateProtoID,
		&linode.UpdateLongviewClientRequest{Label: &label})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != longviewClientUpdateProtoID {
		t.Errorf("got.GetId() = %v, want %v", got.GetId(), longviewClientUpdateProtoID)
	}

	if got.GetLabel() != longviewClientUpdatedLabel {
		t.Errorf("got.GetLabel() = %v, want %v", got.GetLabel(), longviewClientUpdatedLabel)
	}

	if got.GetCreated() != longviewClientCreated {
		t.Errorf("got.GetCreated() = %v, want %v", got.GetCreated(), longviewClientCreated)
	}

	if got.GetUpdated() != longviewClientUpdated {
		t.Errorf("got.GetUpdated() = %v, want %v", got.GetUpdated(), longviewClientUpdated)
	}

	if !got.GetApps().GetApache() || got.GetApps().GetMysql() || !got.GetApps().GetNginx() {
		t.Errorf("got.GetApps() = %+v, want apache and nginx only", got.GetApps())
	}
}

// TestClientUpdateLongviewClientProtoAPIError verifies UpdateLongviewClientProto
// propagates API errors instead of returning a zero-valued proto element.
func TestClientUpdateLongviewClientProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != tcLongviewClients789 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLongviewClients789)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	label := longviewClientUpdatedLabel

	got, err := client.UpdateLongviewClientProto(t.Context(), longviewClientUpdateProtoID,
		&linode.UpdateLongviewClientRequest{Label: &label})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %+v, want nil", got)
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
