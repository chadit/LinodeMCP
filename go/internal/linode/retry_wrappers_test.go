package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func writeRawTestResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	_, err := w.Write([]byte(body))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// dropConn hijacks the request's connection and closes it without writing a
// response. The client observes a transport error after it has already sent
// the request, simulating the worst case for retries: the server may have
// processed the call before the failure surfaced.
func dropConn(w http.ResponseWriter) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}

	_ = conn.Close()
}

// fastRetryOpts returns Option values with minimal delays for testing.
func fastRetryOpts() []linode.Option {
	return []linode.Option{
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1 * time.Millisecond),
		linode.WithMaxDelay(10 * time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	}
}

func TestCreateImageRouteAndRetrySafetyHappyPathUsesExactPOSTImagesRouteAndBody(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if diskID, ok := body["disk_id"].(float64); !ok || diskID != 123 {
			t.Errorf("body[disk_id] = %v, want %v", body["disk_id"], 123)
		}

		if !reflect.DeepEqual(body["label"], "custom-image") {
			t.Errorf("got %v, want %v", body["label"], "custom-image")
		}

		if !reflect.DeepEqual(body["description"], "test image") {
			t.Errorf("got %v, want %v", body["description"], "test image")
		}

		if !reflect.DeepEqual(body["cloud_init"], true) {
			t.Errorf("got %v, want %v", body["cloud_init"], true)
		}

		tags, ok := body["tags"].([]any)
		if !ok {
			t.Error("tags should decode as a JSON array")

			return
		}

		if len(tags) != 2 {
			t.Fatalf("len(tags) = %d, want %d", len(tags), 2)
		}

		if !reflect.DeepEqual(tags[0], "blue") {
			t.Errorf("tags[0] = %v, want %v", tags[0], "blue")
		}

		if !reflect.DeepEqual(tags[1], "green") {
			t.Errorf("tags[1] = %v, want %v", tags[1], "green")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:        "private/15",
			keyLabel:     "custom-image",
			keyStatus:    "creating",
			"created_by": "tester",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	image, err := client.CreateImage(t.Context(), &linode.CreateImageRequest{
		DiskID:      123,
		Label:       "custom-image",
		Description: "test image",
		CloudInit:   true,
		Tags:        []string{"blue", "green"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if image == nil {
		t.Fatal("image is nil")
	}

	if image.ID != "private/15" {
		t.Errorf("image.ID = %v, want %v", image.ID, "private/15")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestCreateImageRouteAndRetrySafetyTransientServerErrorIsNotReplayed(t *testing.T) {
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

	image, err := client.CreateImage(t.Context(), &linode.CreateImageRequest{DiskID: 123})
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

func TestCreateImageShareGroupTokenRouteAndRetrySafetyHappyPathUsesExactPOSTImageShareGroupTokensRouteAndBody(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/images/sharegroups/tokens" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/images/sharegroups/tokens")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body["label"], "release-token") {
			t.Errorf("got %v, want %v", body["label"], "release-token")
		}

		if !reflect.DeepEqual(body["valid_for_sharegroup_uuid"], shareGroupUUIDFixture) {
			t.Errorf("got %v, want %v", body["valid_for_sharegroup_uuid"], shareGroupUUIDFixture)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"token":                     "eyJhbGciOiJIUzI1NiJ9.test.signature",
			"token_uuid":                shareGroupTokenUUIDFixture,
			keyStatus:                   oauthClientStatus,
			keyLabel:                    "release-token",
			"created":                   imageShareGroupTokenCreated,
			"updated":                   nil,
			"expiry":                    nil,
			"valid_for_sharegroup_uuid": shareGroupUUIDFixture,
			"sharegroup_uuid":           shareGroupUUIDFixture,
			"sharegroup_label":          shareGroupLabelFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	token, err := client.CreateImageShareGroupToken(t.Context(), &linode.CreateImageShareGroupTokenRequest{
		Label:                  "release-token",
		ValidForShareGroupUUID: shareGroupUUIDFixture,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token == nil {
		t.Fatal("token is nil")
	}

	if token.TokenUUID != shareGroupTokenUUIDFixture {
		t.Errorf("token.TokenUUID = %v, want %v", token.TokenUUID, shareGroupTokenUUIDFixture)
	}

	if token.Token != "eyJhbGciOiJIUzI1NiJ9.test.signature" {
		t.Errorf("token.Token = %v, want %v", token.Token, "eyJhbGciOiJIUzI1NiJ9.test.signature")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestCreateImageShareGroupTokenRouteAndRetrySafetyTransientServerErrorIsNotReplayed(t *testing.T) {
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

	token, err := client.CreateImageShareGroupToken(t.Context(), &linode.CreateImageShareGroupTokenRequest{ValidForShareGroupUUID: shareGroupUUIDFixture})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if token != nil {
		t.Errorf("token = %v, want nil", token)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestImportDomainRouteAndRetrySafetyHappyPathUsesExactPOSTDomainsImportRouteAndBody(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/domains/import" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/import")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body["domain"], domainExample) {
			t.Errorf("got %v, want %v", body["domain"], domainExample)
		}

		if !reflect.DeepEqual(body["remote_nameserver"], remoteNameserverExample) {
			t.Errorf("got %v, want %v", body["remote_nameserver"], remoteNameserverExample)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     111,
			keyDomain: domainExample,
			keyType:   "master",
			keyStatus: oauthClientStatus,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	domain, err := client.ImportDomain(t.Context(), &linode.ImportDomainRequest{
		Domain:           domainExample,
		RemoteNameserver: remoteNameserverExample,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if domain == nil {
		t.Fatal("domain is nil")
	}

	if domain.ID != 111 {
		t.Errorf("domain.ID = %v, want %v", domain.ID, 111)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestImportDomainRouteAndRetrySafetyTransientServerErrorIsNotReplayed(t *testing.T) {
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

	domain, err := client.ImportDomain(t.Context(), &linode.ImportDomainRequest{Domain: domainExample, RemoteNameserver: remoteNameserverExample})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if domain != nil {
		t.Errorf("domain = %v, want nil", domain)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestCloneDomainRouteAndRetrySafetyHappyPathUsesExactPOSTDomainsCloneRouteAndBody(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/domains/111/clone" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/111/clone")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if !reflect.DeepEqual(body["domain"], domainExample) {
			t.Errorf("got %v, want %v", body["domain"], domainExample)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     222,
			keyDomain: domainExample,
			keyType:   "master",
			keyStatus: oauthClientStatus,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	domain, err := client.CloneDomain(t.Context(), 111, &linode.CloneDomainRequest{Domain: domainExample})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if domain == nil {
		t.Fatal("domain is nil")
	}

	if domain.ID != 222 {
		t.Errorf("domain.ID = %v, want %v", domain.ID, 222)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestCloneDomainRouteAndRetrySafetyTransientServerErrorIsNotReplayed(t *testing.T) {
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

	domain, err := client.CloneDomain(t.Context(), 111, &linode.CloneDomainRequest{Domain: domainExample})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if domain != nil {
		t.Errorf("domain = %v, want nil", domain)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

// Ensures the retry wrapper generator produces correct delegation for all method signatures used by the Linode client.
func TestRetryWrappersDelegationPatternsListRegionsReturnsSlice(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.URL.Path != "/regions" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/regions")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    []map[string]string{{keyID: "us-east"}},
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	regions, err := client.ListRegions(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(regions) != 1 {
		t.Fatalf("len(regions) = %d, want %d", len(regions), 1)
	}

	if regions[0].ID != managedServiceRegion {
		t.Errorf("regions[0].ID = %v, want %v", regions[0].ID, managedServiceRegion)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsGetFirewallReturnsPointer(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.URL.Path != "/networking/firewalls/1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/1")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     1,
			keyLabel:  "my-fw",
			keyStatus: statusEnabledFixture,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	firewall, err := client.GetFirewall(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall == nil {
		t.Fatal("firewall is nil")
	}

	if firewall.ID != 1 {
		t.Errorf("firewall.ID = %v, want %v", firewall.ID, 1)
	}

	if firewall.Label != "my-fw" {
		t.Errorf("firewall.Label = %v, want %v", firewall.Label, "my-fw")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsDeleteDomainReturnsErrorOnly(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

			return
		}

		if r.URL.Path != "/domains/1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/1")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	err := client.DeleteDomain(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsCreateFirewallDelegatesAndRetriesOn429(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"slow down"}]}`)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:    1,
			keyLabel: fwLabelNew,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	firewall, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall == nil {
		t.Fatal("firewall is nil")
	}

	if firewall.Label != fwLabelNew {
		t.Errorf("firewall.Label = %v, want %v", firewall.Label, fwLabelNew)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsUpdateFirewallIdAndRequestReturnsPointer(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:    1,
			keyLabel: "updated-fw",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	firewall, err := client.UpdateFirewall(t.Context(), 1, linode.UpdateFirewallRequest{
		Label: "updated-fw",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall == nil {
		t.Fatal("firewall is nil")
	}

	if firewall.Label != tcUpdatedFw {
		t.Errorf("firewall.Label = %v, want %v", firewall.Label, tcUpdatedFw)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestRetryWrappersDelegationPatternsListRegionsExhaustsRetriesOnPersistent500(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		writeRawTestResponse(t, w, `{"errors":[{"reason":"persistent failure"}]}`)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.ListRegions(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, "persistent failure") {
		t.Errorf("error %v is not an APIError containing %q", err, "persistent failure")
	}
	// fastRetryOpts sets MaxRetries=3: 1 initial attempt + 3 retries = 4 total requests.
	if requestCount.Load() != int32(4) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(4))
	}
}

func TestRetryWrappersDelegationPatternsGetFirewallNoRetryOn401(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusUnauthorized)
		writeRawTestResponse(t, w, `{"errors":[{"reason":"Invalid Token"}]}`)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.GetFirewall(t.Context(), 1)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, "Invalid Token") {
		t.Errorf("error %v is not an APIError containing %q", err, "Invalid Token")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestRetryWrappersDelegationPatternsDeleteDomainRecordTwoIdsReturnsError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	err := client.DeleteDomainRecord(t.Context(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

// TestRetryNonIdempotentDoesNotReplay verifies that a POST create is not
// retried on failures that may already have been applied server-side. A 5xx
// or a transport error (timeout, reset) on a create could mean the resource
// was made but the response was lost, so replaying it risks a duplicate.
func TestRetryNonIdempotentDoesNotReplay(t *testing.T) {
	t.Parallel()

	t.Run("no retry on 500", func(t *testing.T) {
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
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		if requestCount.Load() != int32(1) {
			t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
		}
	})

	t.Run("no retry on transport error", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		// Hijack and drop the connection so the client sees a transport error
		// after the request was already sent: the server-side-applied case.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)

			dropConn(w)
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		if requestCount.Load() != int32(1) {
			t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
		}
	})

	t.Run("idempotent GET still retries on transport error", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)

			dropConn(w)
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.ListRegions(t.Context())
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		// fastRetryOpts sets MaxRetries=3: 1 initial + 3 retries = 4 attempts.
		if requestCount.Load() != int32(4) {
			t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(4))
		}
	})
}

// TestRetryWrappersContextCancellationStopsRetry verifies that canceling
// the context during a retry sequence stops further attempts and returns
// a context-canceled error.
func TestRetryWrappersContextCancellationStopsRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	ctx, cancel := context.WithCancel(t.Context())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		cancel()

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusInternalServerError)
		writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.ListRegions(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want %v", err, context.Canceled)
	}
}

// TestRetryWrappersBodyForwardedOnRetry verifies that request bodies are
// correctly re-sent on retried POST requests, so the server receives the
// full payload even after an initial failure.
func TestRetryWrappersBodyForwardedOnRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			// 429 is the one transient failure a non-idempotent POST still
			// retries, so it is what exercises body re-send on this path.
			w.WriteHeader(http.StatusTooManyRequests)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"slow down"}]}`)

			return
		}

		body, _ := io.ReadAll(r.Body)
		capturedBody = body

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:    1,
			keyLabel: "test-fw",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	firewall, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{
		Label: "test-fw",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall == nil {
		t.Fatal("firewall is nil")
	}

	if firewall.Label != tcTestFw {
		t.Errorf("firewall.Label = %v, want %v", firewall.Label, tcTestFw)
	}

	if !strings.Contains(string(capturedBody), `"label"`) {
		t.Errorf("string(capturedBody) does not contain %v", `"label"`)
	}

	if !strings.Contains(string(capturedBody), `"test-fw"`) {
		t.Errorf("string(capturedBody) does not contain %v", `"test-fw"`)
	}
}

func TestGetSSHKeyRetriesHappyPathRetriesOnTransientError(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.URL.Path != "/profile/sshkeys/42" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/profile/sshkeys/42")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     42,
			keyLabel:  "my-key",
			keySSHKey: "ssh-rsa AAAA test@example.com",
			"created": "2024-01-01T00:00:00Z",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	sshKey, err := client.GetSSHKey(t.Context(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sshKey == nil {
		t.Fatal("sshKey is nil")
	}

	if sshKey.ID != 42 {
		t.Errorf("sshKey.ID = %v, want %v", sshKey.ID, 42)
	}

	if sshKey.Label != "my-key" {
		t.Errorf("sshKey.Label = %v, want %v", sshKey.Label, "my-key")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestGetSSHKeyRetriesNoRetryOn401(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusUnauthorized)

		_, err := w.Write([]byte(`{"errors":[{"reason":"Invalid Token"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.GetSSHKey(t.Context(), 42)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok || !strings.Contains(apiErr.Message, "Invalid Token") {
		t.Errorf("error %v is not an APIError containing %q", err, "Invalid Token")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

const domainZoneTTL = "$TTL 864000"

func TestGetDomainZoneFileRouteHappyPathUsesExactGETDomainZoneFileRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/domains/123/zone-file" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/123/zone-file")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"zone_file": []string{
				tcExampleCom123,
				domainZoneTTL,
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	zoneFile, err := client.GetDomainZoneFile(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if zoneFile == nil {
		t.Fatal("zoneFile is nil")
	}

	if !reflect.DeepEqual(zoneFile.ZoneFile, []string{tcExampleCom123, domainZoneTTL}) {
		t.Errorf("zoneFile.ZoneFile = %v, want %v", zoneFile.ZoneFile, []string{tcExampleCom123, domainZoneTTL})
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestGetDomainZoneFileRouteTransientServerErrorRetriesReadOnlyRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"zone_file": []string{domainZoneTTL},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	zoneFile, err := client.GetDomainZoneFile(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if zoneFile == nil {
		t.Fatal("zoneFile is nil")
	}

	if !reflect.DeepEqual(zoneFile.ZoneFile, []string{"$TTL 864000"}) {
		t.Errorf("zoneFile.ZoneFile = %v, want %v", zoneFile.ZoneFile, []string{"$TTL 864000"})
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestGetDomainZoneFileRoutePermanentAPIErrorIsNotRetried(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte(`{"errors":[{"reason":"domain not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	zoneFile, err := client.GetDomainZoneFile(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if zoneFile != nil {
		t.Errorf("zoneFile = %v, want nil", zoneFile)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestGetDomainRecordRouteHappyPathUsesExactGETDomainRecordRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/domains/123/records/456" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/domains/123/records/456")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:     456,
			"type":    "A",
			"name":    "www",
			"target":  "192.0.2.10",
			"ttl_sec": 300,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	record, err := client.GetDomainRecord(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record == nil {
		t.Fatal("record is nil")
	}

	if record.ID != 456 {
		t.Errorf("record.ID = %v, want %v", record.ID, 456)
	}

	if record.Type != "A" {
		t.Errorf("record.Type = %v, want %v", record.Type, "A")
	}

	if record.Name != "www" {
		t.Errorf("record.Name = %v, want %v", record.Name, "www")
	}

	if record.Target != "192.0.2.10" {
		t.Errorf("record.Target = %v, want %v", record.Target, "192.0.2.10")
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestGetDomainRecordRouteTransientServerErrorRetriesReadOnlyRequest(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 456,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	record, err := client.GetDomainRecord(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record == nil {
		t.Fatal("record is nil")
	}

	if record.ID != 456 {
		t.Errorf("record.ID = %v, want %v", record.ID, 456)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestGetDomainRecordRoutePermanentAPIErrorIsNotRetried(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusNotFound)

		_, err := w.Write([]byte(`{"errors":[{"reason":"record not found"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	record, err := client.GetDomainRecord(t.Context(), 123, 456)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if record != nil {
		t.Errorf("record = %v, want nil", record)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}
