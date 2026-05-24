package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

const (
	endpointDatabaseEngines     = "/databases/engines"
	endpointDatabaseInstances   = "/databases/mysql/instances"
	endpointDatabaseMySQLConfig = "/databases/mysql/config"
)

// ListDatabaseEngines retrieves available Managed Database engines.
func (c *Client) httpListDatabaseEngines(ctx context.Context, page, pageSize int) ([]DatabaseEngine, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointDatabaseEngines, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDatabaseEngines", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[DatabaseEngine]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// ListDatabaseInstances retrieves Managed Database instances.
func (c *Client) httpListDatabaseInstances(ctx context.Context, page, pageSize int) ([]DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointDatabaseInstances, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDatabaseInstances", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[DatabaseInstance]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// GetDatabaseInstance retrieves one MySQL Managed Database instance.
func (c *Client) httpGetDatabaseInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	var instance DatabaseInstance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// GetDatabaseInstanceSSL retrieves the SSL CA certificate for a MySQL Managed Database instance.
func (c *Client) httpGetDatabaseInstanceSSL(ctx context.Context, instanceID int) (*DatabaseSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/ssl"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseInstanceSSL", Err: err}
	}

	defer drainClose(resp)

	var ssl DatabaseSSL
	if err := c.handleResponse(resp, &ssl); err != nil {
		return nil, err
	}

	return &ssl, nil
}

// GetDatabaseInstanceCredentials retrieves MySQL Managed Database credentials.
func (c *Client) httpGetDatabaseInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/credentials"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseInstanceCredentials", Err: err}
	}

	defer drainClose(resp)

	var credentials DatabaseCredentials
	if err := c.handleResponse(resp, &credentials); err != nil {
		return nil, err
	}

	return &credentials, nil
}

// ResetDatabaseInstanceCredentials resets MySQL Managed Database credentials.
func (c *Client) httpResetDatabaseInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/credentials/reset"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ResetDatabaseInstanceCredentials", Err: err}
	}

	defer drainClose(resp)

	var credentials DatabaseCredentials
	if err := c.handleResponse(resp, &credentials); err != nil {
		return nil, err
	}

	return &credentials, nil
}

// CreateDatabaseInstance creates or restores a MySQL Managed Database instance.
func (c *Client) httpCreateDatabaseInstance(ctx context.Context, req *CreateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDatabaseInstances, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	var instance DatabaseInstance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// UpdateDatabaseInstance updates one MySQL Managed Database instance.
func (c *Client) httpUpdateDatabaseInstance(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	var instance DatabaseInstance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// DeleteDatabaseInstance deletes one MySQL Managed Database instance.
func (c *Client) httpDeleteDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// PatchDatabaseInstance applies security patches and updates to one MySQL Managed Database instance.
func (c *Client) httpPatchDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/patch"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "PatchDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// SuspendDatabaseInstance suspends one active MySQL Managed Database instance.
func (c *Client) httpSuspendDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/suspend"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "SuspendDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResumeDatabaseInstance resumes one suspended MySQL Managed Database instance.
func (c *Client) httpResumeDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/resume"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ResumeDatabaseInstance", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// GetDatabaseMySQLConfig retrieves MySQL Managed Database advanced parameters.
func (c *Client) httpGetDatabaseMySQLConfig(ctx context.Context) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointDatabaseMySQLConfig, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseMySQLConfig", Err: err}
	}

	defer drainClose(resp)

	var config map[string]any
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// GetDatabaseEngine retrieves a Managed Database engine by ID.
func (c *Client) httpGetDatabaseEngine(ctx context.Context, engineID string) (*DatabaseEngine, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseEngines + "/" + url.PathEscape(engineID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseEngine", Err: err}
	}

	defer drainClose(resp)

	var engine DatabaseEngine
	if err := c.handleResponse(resp, &engine); err != nil {
		return nil, err
	}

	return &engine, nil
}
