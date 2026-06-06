package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const imageShareGroupTokenUpdateLabel = "Backend Services - Engineering"

func TestClientUpdateImageShareGroupTokenSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/images/sharegroups/tokens/"+shareGroupTokenUUIDFixture, r.URL.Path, "request path should include token UUID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		checkNoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
		checkEqual(t, imageShareGroupTokenUpdateLabel, body["label"], "label should be sent")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{
			TokenUUID:              shareGroupTokenUUIDFixture,
			Status:                 oauthClientStatus,
			Label:                  imageShareGroupTokenUpdateLabel,
			Created:                imageShareGroupTokenCreated,
			ValidForShareGroupUUID: shareGroupUUIDFixture,
			ShareGroupUUID:         shareGroupUUIDFixture,
			ShareGroupLabel:        shareGroupLabelFixture,
		}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	token, err := client.UpdateImageShareGroupToken(t.Context(), shareGroupTokenUUIDFixture, &linode.UpdateImageShareGroupTokenRequest{Label: imageShareGroupTokenUpdateLabel})

	requireNoError(t, err, "UpdateImageShareGroupToken should succeed")
	requireNotNil(t, token, "token should be returned")
	checkEqual(t, shareGroupTokenUUIDFixture, token.TokenUUID, "response should decode token UUID")
	checkEqual(t, imageShareGroupTokenUpdateLabel, token.Label, "response should decode label")
}

func TestClientUpdateImageShareGroupTokenEscapesTokenUUID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, "/images/sharegroups/tokens/token%2Fuuid%3Fquery", r.URL.EscapedPath(), "path parameter should be escaped")
		checkEmpty(t, r.URL.RawQuery, "encoded question mark should not become a query string")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{TokenUUID: "token/uuid?query", Label: imageShareGroupTokenUpdateLabel}), "encoding response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	_, err := client.UpdateImageShareGroupToken(t.Context(), "token/uuid?query", &linode.UpdateImageShareGroupTokenRequest{Label: imageShareGroupTokenUpdateLabel})

	requireNoError(t, err, "UpdateImageShareGroupToken should escape path parameters")
}

func TestClientUpdateImageShareGroupTokenEscapesDotSegments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		tokenUUID   string
		escapedPath string
	}{
		{name: "single dot", tokenUUID: ".", escapedPath: "/images/sharegroups/tokens/%2E"},
		{name: "double dot", tokenUUID: pathTraversalDotDot, escapedPath: "/images/sharegroups/tokens/%2E%2E"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
				checkEqual(t, testCase.escapedPath, r.URL.EscapedPath(), "dot segment should stay encoded")
				w.Header().Set("Content-Type", "application/json")
				checkNoError(t, json.NewEncoder(w).Encode(linode.ImageShareGroupToken{TokenUUID: testCase.tokenUUID, Label: imageShareGroupTokenUpdateLabel}), "encoding response should succeed")
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
			_, err := client.UpdateImageShareGroupToken(t.Context(), testCase.tokenUUID, &linode.UpdateImageShareGroupTokenRequest{Label: imageShareGroupTokenUpdateLabel})

			requireNoError(t, err, "UpdateImageShareGroupToken should encode dot segments")
		})
	}
}

func TestUpdateImageShareGroupTokenNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"try again"}]}`))
		checkNoError(t, err, "writing error response should succeed")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	_, err := client.UpdateImageShareGroupToken(t.Context(), shareGroupTokenUUIDFixture, &linode.UpdateImageShareGroupTokenRequest{Label: imageShareGroupTokenUpdateLabel})

	requireError(t, err, "transient failure should return an error")
	checkEqual(t, int32(1), calls.Load(), "mutating update should not be retried")
}
