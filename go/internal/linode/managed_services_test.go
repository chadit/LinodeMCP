package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedServicesPath      = "/managed/services"
	managedServiceLabel      = "prod-1"
	managedServiceType       = "url"
	managedServiceStatus     = "ok"
	managedServiceAddress    = "https://example.org"
	managedServiceCreated    = "2018-01-01T00:01:01"
	managedServiceUpdated    = "2018-03-01T00:01:01"
	managedServiceBody       = "it worked"
	managedServiceGroup      = "on-call"
	managedServiceNotes      = "The service name is my-cool-application"
	managedServiceRegion     = "us-east"
	managedServicesForbidden = "Forbidden"
)

func TestClientListManagedServicesSuccess(t *testing.T) {
	t.Parallel()

	body := managedServiceBody
	notes := managedServiceNotes
	region := managedServiceRegion

	services := linode.PaginatedResponse[linode.ManagedService]{
		Data: []linode.ManagedService{{
			ID:                9944,
			Label:             managedServiceLabel,
			ServiceType:       managedServiceType,
			Status:            managedServiceStatus,
			Address:           managedServiceAddress,
			Body:              &body,
			ConsultationGroup: managedServiceGroup,
			Created:           managedServiceCreated,
			Credentials:       []int{9991},
			Notes:             &notes,
			Region:            &region,
			Timeout:           30,
			Updated:           managedServiceUpdated,
		}},
		Page:    2,
		Pages:   3,
		Results: 51,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		assert.Equal(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(services))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListManagedServices(t.Context(), 2, 25)

	require.NoError(t, err, "ListManagedServices should succeed on 200 response")
	require.NotNil(t, result)
	require.Len(t, result.Data, 1)
	assert.Equal(t, 9944, result.Data[0].ID)
	assert.Equal(t, managedServiceLabel, result.Data[0].Label)
	assert.Equal(t, managedServiceType, result.Data[0].ServiceType)
	assert.Equal(t, managedServiceStatus, result.Data[0].Status)
	assert.Equal(t, managedServiceAddress, result.Data[0].Address)
	require.NotNil(t, result.Data[0].Body)
	assert.Equal(t, managedServiceBody, *result.Data[0].Body)
	assert.Equal(t, managedServiceGroup, result.Data[0].ConsultationGroup)
	assert.Equal(t, managedServiceCreated, result.Data[0].Created)
	assert.Equal(t, []int{9991}, result.Data[0].Credentials)
	require.NotNil(t, result.Data[0].Notes)
	assert.Equal(t, managedServiceNotes, *result.Data[0].Notes)
	require.NotNil(t, result.Data[0].Region)
	assert.Equal(t, managedServiceRegion, *result.Data[0].Region)
	assert.Equal(t, 30, result.Data[0].Timeout)
	assert.Equal(t, managedServiceUpdated, result.Data[0].Updated)
}

func TestClientListManagedServicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedService]{
			Data:    []linode.ManagedService{{ID: 9944}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListManagedServices(t.Context(), 0, 0)

	require.NoError(t, err, "read-only Managed services list should retry transient failures")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListManagedServicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedServices(t.Context(), 0, 0)

	require.Error(t, err, "ListManagedServices should fail on API errors")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "error should be an APIError")
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}
