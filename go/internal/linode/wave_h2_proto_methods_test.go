package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	waveH2PublicKey  = "ssh-rsa AAAAB3NzaC1 user@host"
	waveH2TokenLabel = "renamed-label"
	waveH2Secret     = "secret-token-value"
	waveH2TFASecret  = "JBSWY3DPEHPK3PXP"
	waveH2SecretKey  = "secret"
	waveH2Desc       = "down"
	waveH2ReplyText  = "thanks"
	waveH2Created    = "2026-06-30"
)

// newWaveH2Client builds a no-retry client pointed at the test server.
func newWaveH2Client(url string) *linode.Client {
	return linode.NewClient(url, "my-token", nil, linode.WithMaxRetries(0))
}

func TestClientCreateSupportTicketProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != tcSupportTickets {
			t.Errorf("got %v %v, want POST %v", r.Method, r.URL.Path, tcSupportTickets)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		// The API returns more fields than the proto models; the proto decode
		// discards the unknown extra_field, matching Go's tolerant read path.
		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 987, "summary": "Need help", keyDescription: waveH2Desc,
			keyStatus: supportTicketStatusOpen, "extra_field": "ignored",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.CreateSupportTicketProto(t.Context(),
		&linode.CreateSupportTicketRequest{Summary: "Need help", Description: waveH2Desc})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 987 {
		t.Errorf("got.GetId() = %d, want 987", got.GetId())
	}

	if got.GetStatus() != supportTicketStatusOpen {
		t.Errorf("got.GetStatus() = %q, want %q", got.GetStatus(), supportTicketStatusOpen)
	}
}

func TestClientCreateSupportTicketProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	if _, err := client.CreateSupportTicketProto(t.Context(),
		&linode.CreateSupportTicketRequest{Summary: "s", Description: "d"}); err == nil {
		t.Fatal("expected error on a forbidden response, got nil")
	}
}

func TestClientCreateSupportTicketReplyProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcSupportTickets+"/55/replies" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcSupportTickets+"/55/replies")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 7788, keyDescription: waveH2ReplyText,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.CreateSupportTicketReplyProto(t.Context(), 55,
		&linode.CreateSupportTicketReplyRequest{Description: waveH2ReplyText})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != 7788 || got.GetDescription() != waveH2ReplyText {
		t.Errorf("got %+v, want id 7788 description %q", got, waveH2ReplyText)
	}
}

func TestClientCreateSSHKeyProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/sshkeys" {
			t.Errorf("r.URL.Path = %v, want /profile/sshkeys", r.URL.Path)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 42, keyLabel: "laptop", "ssh_key": waveH2PublicKey, keyCreated: waveH2Created,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.CreateSSHKeyProto(t.Context(),
		linode.CreateSSHKeyRequest{Label: "laptop", SSHKey: waveH2PublicKey})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The public key is public information and is kept in full.
	if got.GetSshKey() != waveH2PublicKey {
		t.Errorf("got.GetSshKey() = %q, want %q", got.GetSshKey(), waveH2PublicKey)
	}
}

func TestClientUpdateSSHKeyProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/profile/sshkeys/42" {
			t.Errorf("got %v %v, want PUT /profile/sshkeys/42", r.Method, r.URL.Path)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 42, keyLabel: waveH2TokenLabel, "ssh_key": waveH2PublicKey, keyCreated: waveH2Created,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.UpdateSSHKeyProto(t.Context(), 42, linode.UpdateSSHKeyRequest{Label: waveH2TokenLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetLabel() != waveH2TokenLabel {
		t.Errorf("got.GetLabel() = %q, want %q", got.GetLabel(), waveH2TokenLabel)
	}
}

func TestClientCreateProfileTokenProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcProfileTokens {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcProfileTokens)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 321, keyLabel: "ci", profileTokenScopesKey: "*", keyCreated: waveH2Created, keyToken: waveH2Secret,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.CreateProfileTokenProto(t.Context(),
		linode.CreateProfileTokenRequest{Label: "ci"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The one-time secret is returned by design and survives the proto decode.
	if got.GetToken() != waveH2Secret {
		t.Errorf("got.GetToken() = %q, want %q", got.GetToken(), waveH2Secret)
	}
}

func TestClientUpdateProfileTokenProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != tcProfileTokens+"/321" {
			t.Errorf("got %v %v, want PUT %v/321", r.Method, r.URL.Path, tcProfileTokens)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		// An update never returns a token secret; the metadata element models none.
		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 321, keyLabel: waveH2TokenLabel, profileTokenScopesKey: "*", keyCreated: waveH2Created,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.UpdateProfileTokenProto(t.Context(), "321",
		linode.UpdateProfileTokenRequest{keyLabel: waveH2TokenLabel})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetLabel() != waveH2TokenLabel {
		t.Errorf("got.GetLabel() = %q, want %q", got.GetLabel(), waveH2TokenLabel)
	}
}

func TestClientEnableProfileTFAProtoSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != tcProfileTfaEnable {
			t.Errorf("got %v %v, want POST %v", r.Method, r.URL.Path, tcProfileTfaEnable)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			waveH2SecretKey: waveH2TFASecret, keyTFAConfirmExpiry: "2026-06-30T12:15:00",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	got, err := client.EnableProfileTFAProto(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The one-time secret is returned by design; the handler adds the warning.
	if got.GetSecret() != waveH2TFASecret {
		t.Errorf("got.GetSecret() = %q, want %q", got.GetSecret(), waveH2TFASecret)
	}

	if got.GetWarning() != "" {
		t.Errorf("got.GetWarning() = %q, want empty (the handler sets it, not the client)", got.GetWarning())
	}
}

func TestClientEnableProfileTFAProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := newWaveH2Client(srv.URL)

	if _, err := client.EnableProfileTFAProto(t.Context()); err == nil {
		t.Fatal("expected error on a forbidden response, got nil")
	}
}
