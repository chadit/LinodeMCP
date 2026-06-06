package linode_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	managedServicesPath       = "/managed/services"
	managedServiceID          = 9944
	managedServicePath        = "/managed/services/9944"
	managedServiceDisablePath = "/managed/services/9944/disable"
	managedServiceEnablePath  = "/managed/services/9944/enable"
	managedServiceLabel       = "prod-1"
	managedServiceType        = "url"
	managedServiceStatus      = "ok"
	managedServiceAddress     = "https://example.org"
	managedServiceCreated     = "2018-01-01T00:01:01"
	managedServiceUpdated     = "2018-03-01T00:01:01"
	managedServiceBody        = "it worked"
	managedServiceGroup       = "on-call"
	managedServiceNotes       = "The service name is my-cool-application"
	managedServiceRegion      = "us-east"
	managedServicesForbidden  = "Forbidden"
)

func TestClientGetManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	body := managedServiceBody
	notes := managedServiceNotes
	region := managedServiceRegion

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedService{
			ID:                managedServiceID,
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
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.GetManagedService(t.Context(), managedServiceID)

	requireNoError(t, err, "GetManagedService should succeed on 200 response")
	requireNotNil(t, result)
	checkEqual(t, managedServiceID, result.ID)
	checkEqual(t, managedServiceLabel, result.Label)
	checkEqual(t, managedServiceType, result.ServiceType)
	checkEqual(t, managedServiceStatus, result.Status)
	checkEqual(t, managedServiceAddress, result.Address)
	requireNotNil(t, result.Body)
	checkEqual(t, managedServiceBody, *result.Body)
	checkEqual(t, managedServiceGroup, result.ConsultationGroup)
	checkEqual(t, managedServiceCreated, result.Created)
	checkEqual(t, []int{9991}, result.Credentials)
	requireNotNil(t, result.Notes)
	checkEqual(t, managedServiceNotes, *result.Notes)
	requireNotNil(t, result.Region)
	checkEqual(t, managedServiceRegion, *result.Region)
	checkEqual(t, 30, result.Timeout)
	checkEqual(t, managedServiceUpdated, result.Updated)
}

func TestClientGetManagedServiceRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedService{ID: managedServiceID}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.GetManagedService(t.Context(), managedServiceID)

	requireNoError(t, err, "read-only Managed service get should retry transient failures")
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientGetManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.GetManagedService(t.Context(), managedServiceID)

	requireError(t, err, "GetManagedService should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientDeleteManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")
		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DeleteManagedService(t.Context(), managedServiceID)

	requireNoError(t, err, "DeleteManagedService should succeed on 200 response")
}

func TestClientDeleteManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DeleteManagedService(t.Context(), managedServiceID)

	requireError(t, err, "DeleteManagedService should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientDeleteManagedServiceDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodDelete, r.Method, "request method should be DELETE")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	err := client.DeleteManagedService(t.Context(), managedServiceID)

	requireError(t, err, "DeleteManagedService should return the transient failure")
	checkEqual(t, int32(1), calls.Load(), "destructive DELETE should not be retried")
}

func TestClientDisableManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServiceDisablePath, r.URL.Path, "request path should include service ID and disable action")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err)
		checkEmpty(t, body, "disable request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DisableManagedService(t.Context(), managedServiceID)

	requireNoError(t, err, "DisableManagedService should succeed on 200 response")
}

func TestClientDisableManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServiceDisablePath, r.URL.Path, "request path should include service ID and disable action")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.DisableManagedService(t.Context(), managedServiceID)

	requireError(t, err, "DisableManagedService should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientDisableManagedServiceDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServiceDisablePath, r.URL.Path, "request path should include service ID and disable action")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	err := client.DisableManagedService(t.Context(), managedServiceID)

	requireError(t, err, "DisableManagedService should return the transient failure")
	checkEqual(t, int32(1), calls.Load(), "mutating POST should not be retried")
}

func TestClientEnableManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServiceEnablePath, r.URL.Path, "request path should include service ID and enable action")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		body, err := io.ReadAll(r.Body)
		checkNoError(t, err)
		checkEmpty(t, body, "enable request should not include a body")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.EnableManagedService(t.Context(), managedServiceID)

	requireNoError(t, err, "EnableManagedService should succeed on 200 response")
}

func TestClientEnableManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServiceEnablePath, r.URL.Path, "request path should include service ID and enable action")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	err := client.EnableManagedService(t.Context(), managedServiceID)

	requireError(t, err, "EnableManagedService should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientEnableManagedServiceDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServiceEnablePath, r.URL.Path, "request path should include service ID and enable action")
		w.WriteHeader(http.StatusInternalServerError)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(2), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	err := client.EnableManagedService(t.Context(), managedServiceID)

	requireError(t, err, "EnableManagedService should return the transient failure")
	checkEqual(t, int32(1), calls.Load(), "mutating POST should not be retried")
}

func TestClientListManagedServicesSuccess(t *testing.T) {
	t.Parallel()

	body := managedServiceBody
	notes := managedServiceNotes
	region := managedServiceRegion

	services := linode.PaginatedResponse[linode.ManagedService]{
		Data: []linode.ManagedService{{
			ID:                managedServiceID,
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
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		checkEqual(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
		checkEqual(t, managedIssueAuthHeader, r.Header.Get("Authorization"), "authorization header should use bearer token")

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(services))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	result, err := client.ListManagedServices(t.Context(), 2, 25)

	requireNoError(t, err, "ListManagedServices should succeed on 200 response")
	requireNotNil(t, result)
	requireLenOne(t, result.Data)
	checkEqual(t, managedServiceID, result.Data[0].ID)
	checkEqual(t, managedServiceLabel, result.Data[0].Label)
	checkEqual(t, managedServiceType, result.Data[0].ServiceType)
	checkEqual(t, managedServiceStatus, result.Data[0].Status)
	checkEqual(t, managedServiceAddress, result.Data[0].Address)
	requireNotNil(t, result.Data[0].Body)
	checkEqual(t, managedServiceBody, *result.Data[0].Body)
	checkEqual(t, managedServiceGroup, result.Data[0].ConsultationGroup)
	checkEqual(t, managedServiceCreated, result.Data[0].Created)
	checkEqual(t, []int{9991}, result.Data[0].Credentials)
	requireNotNil(t, result.Data[0].Notes)
	checkEqual(t, managedServiceNotes, *result.Data[0].Notes)
	requireNotNil(t, result.Data[0].Region)
	checkEqual(t, managedServiceRegion, *result.Data[0].Region)
	checkEqual(t, 30, result.Data[0].Timeout)
	checkEqual(t, managedServiceUpdated, result.Data[0].Updated)
}

func TestClientListManagedServicesRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")

		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: temporaryPaymentError}}}))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.ManagedService]{
			Data:    []linode.ManagedService{{ID: 9944}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(1), linode.WithBaseDelay(time.Millisecond), linode.WithJitter(false))
	result, err := client.ListManagedServices(t.Context(), 0, 0)

	requireNoError(t, err, "read-only Managed services list should retry transient failures")
	requireNotNil(t, result)
	checkEqual(t, int32(2), calls.Load(), "client should retry once")
}

func TestClientListManagedServicesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodGet, r.Method, "request method should be GET")
		checkEqual(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: managedServicesForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.ListManagedServices(t.Context(), 0, 0)

	requireError(t, err, "ListManagedServices should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestClientCreateManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.CreateManagedServiceRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got))
		checkEqual(t, managedServiceLabel, got.Label)
		checkEqual(t, managedServiceType, got.ServiceType)
		checkEqual(t, managedServiceAddress, got.Address)
		checkEqual(t, 30, got.Timeout)

		if got.Body == nil || got.ConsultationGroup == nil || got.Notes == nil {
			t.Errorf("request body missing optional managed service fields: %#v", got)

			return
		}

		checkEqual(t, managedServiceBody, *got.Body)
		checkEqual(t, managedServiceGroup, *got.ConsultationGroup)
		checkEqual(t, managedServiceNotes, *got.Notes)
		checkEqual(t, []int{9991}, got.Credentials)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedService{
			ID:                9944,
			Label:             got.Label,
			ServiceType:       got.ServiceType,
			Address:           got.Address,
			Timeout:           got.Timeout,
			Body:              got.Body,
			ConsultationGroup: *got.ConsultationGroup,
			Credentials:       got.Credentials,
			Notes:             got.Notes,
			Status:            managedServiceStatus,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	body := managedServiceBody
	consultationGroup := managedServiceGroup
	notes := managedServiceNotes
	req := &linode.CreateManagedServiceRequest{
		Label:             managedServiceLabel,
		ServiceType:       managedServiceType,
		Address:           managedServiceAddress,
		Timeout:           30,
		Body:              &body,
		ConsultationGroup: &consultationGroup,
		Credentials:       []int{9991},
		Notes:             &notes,
	}

	service, err := client.CreateManagedService(t.Context(), req)

	requireNoError(t, err, "CreateManagedService should succeed on 200 response")
	requireNotNil(t, service)
	checkEqual(t, 9944, service.ID)
	checkEqual(t, managedServiceLabel, service.Label)
	checkEqual(t, managedServiceStatus, service.Status)
}

func TestClientCreateManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"managed service could not be created"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.CreateManagedService(t.Context(), &linode.CreateManagedServiceRequest{Label: managedServiceLabel})

	requireError(t, err, "CreateManagedService should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientCreateManagedServiceDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, managedServicesPath, r.URL.Path, "request path should be /managed/services")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.CreateManagedService(t.Context(), &linode.CreateManagedServiceRequest{Label: managedServiceLabel})

	requireError(t, err, "CreateManagedService should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "CreateManagedService must not retry and replay a mutating request")
}

func TestClientUpdateManagedServiceSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer my-token", r.Header.Get("Authorization"))

		var got linode.UpdateManagedServiceRequest
		checkNoError(t, json.NewDecoder(r.Body).Decode(&got))

		if got.Label == nil || got.ServiceType == nil || got.Address == nil || got.Timeout == nil || got.Credentials == nil {
			t.Errorf("request body missing managed service update fields: %#v", got)

			return
		}

		checkEqual(t, managedServiceLabel, *got.Label)
		checkEqual(t, managedServiceType, *got.ServiceType)
		checkEqual(t, managedServiceAddress, *got.Address)
		checkEqual(t, 30, *got.Timeout)
		checkEqual(t, []int{9991}, *got.Credentials)

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(linode.ManagedService{
			ID:          managedServiceID,
			Label:       *got.Label,
			ServiceType: *got.ServiceType,
			Address:     *got.Address,
			Timeout:     *got.Timeout,
			Credentials: *got.Credentials,
			Status:      managedServiceStatus,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	label := managedServiceLabel
	serviceType := managedServiceType
	address := managedServiceAddress
	timeout := 30
	req := linode.UpdateManagedServiceRequest{
		Label:       &label,
		ServiceType: &serviceType,
		Address:     &address,
		Timeout:     &timeout,
		Credentials: &[]int{9991},
	}

	service, err := client.UpdateManagedService(t.Context(), managedServiceID, &req)

	requireNoError(t, err, "UpdateManagedService should succeed on 200 response")
	requireNotNil(t, service)
	checkEqual(t, managedServiceID, service.ID)
	checkEqual(t, managedServiceLabel, service.Label)
	checkEqual(t, managedServiceStatus, service.Status)
}

func TestClientUpdateManagedServiceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"errors":[{"reason":"managed service could not be updated"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	label := managedServiceLabel

	_, err := client.UpdateManagedService(t.Context(), managedServiceID, &linode.UpdateManagedServiceRequest{Label: &label})

	requireError(t, err, "UpdateManagedService should fail on API errors")
	apiErr := requireAPIError(t, err, "error should be an APIError")
	checkEqual(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestClientUpdateManagedServiceDoesNotRetry(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		checkEqual(t, http.MethodPut, r.Method, "request method should be PUT")
		checkEqual(t, managedServicePath, r.URL.Path, "request path should include service ID")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"errors":[{"reason":"temporary failure"}]}`))
		checkNoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))
	label := managedServiceLabel

	_, err := client.UpdateManagedService(t.Context(), managedServiceID, &linode.UpdateManagedServiceRequest{Label: &label})

	requireError(t, err, "UpdateManagedService should fail on 500 response")
	checkEqual(t, int32(1), calls.Load(), "UpdateManagedService must not retry and replay a mutating request")
}
