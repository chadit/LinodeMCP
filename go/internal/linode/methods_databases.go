package linode

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

const (
	endpointDatabaseEngines             = "/databases/engines"
	endpointDatabaseTypes               = "/databases/types"
	endpointDatabaseAllInstances        = "/databases/instances"
	endpointDatabaseInstances           = "/databases/mysql/instances"
	endpointDatabasePostgreSQLInstances = "/databases/postgresql/instances"
	endpointDatabaseMySQLConfig         = "/databases/mysql/config"
	endpointDatabasePostgreSQLConfig    = "/databases/postgresql/config"
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

// httpListDatabaseEnginesProto retrieves available Managed Database engines as
// proto messages for the proto-backed list path. page/page_size flow through
// withPaginationQuery, so the request matches httpListDatabaseEngines.
func (c *Client) httpListDatabaseEnginesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseEngine, error) {
	return listProtoElementsPaginated(ctx, c, "ListDatabaseEngines", endpointDatabaseEngines, page, pageSize,
		func() *linodev1.DatabaseEngine { return &linodev1.DatabaseEngine{} })
}

// ListDatabaseTypes retrieves available Managed Database node types.
func (c *Client) httpListDatabaseTypes(ctx context.Context, page, pageSize int) ([]DatabaseType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointDatabaseTypes, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDatabaseTypes", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[DatabaseType]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListDatabaseTypesProto retrieves available Managed Database node types as
// proto messages for the proto-backed list path. page/page_size flow through
// withPaginationQuery, so the request matches httpListDatabaseTypes.
func (c *Client) httpListDatabaseTypesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseType, error) {
	return listProtoElementsPaginated(ctx, c, "ListDatabaseTypes", endpointDatabaseTypes, page, pageSize,
		func() *linodev1.DatabaseType { return &linodev1.DatabaseType{} })
}

// GetDatabaseType retrieves a Managed Database node type by ID.
func (c *Client) httpGetDatabaseType(ctx context.Context, typeID string, page, pageSize int) (*DatabaseType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseTypes + "/" + url.PathEscape(typeID)
	endpoint = withPaginationQuery(endpoint, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseType", Err: err}
	}

	defer drainClose(resp)

	var databaseType DatabaseType
	if err := c.handleResponse(resp, &databaseType); err != nil {
		return nil, err
	}

	return &databaseType, nil
}

// httpGetDatabaseTypeProto retrieves one Managed Database type as a proto message.
func (c *Client) httpGetDatabaseTypeProto(ctx context.Context, typeID string, page, pageSize int) (*linodev1.DatabaseType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseTypes + "/" + url.PathEscape(typeID)
	endpoint = withPaginationQuery(endpoint, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseType", Err: err}
	}

	defer drainClose(resp)

	databaseType := &linodev1.DatabaseType{}
	if err := c.handleProtoResponse(resp, databaseType); err != nil {
		return nil, err
	}

	return databaseType, nil
}

// ListAllDatabaseInstances retrieves Managed Database instances across
// every engine via the cross-engine /databases/instances endpoint. The
// per-engine lists below hit /databases/mysql/instances and
// /databases/postgresql/instances instead.
func (c *Client) httpListAllDatabaseInstances(ctx context.Context, page, pageSize int) ([]DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointDatabaseAllInstances, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListAllDatabaseInstances", Err: err}
	}

	defer drainClose(resp)

	var response PaginatedResponse[DatabaseInstance]
	if err := c.handleResponse(resp, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}

// httpListAllDatabaseInstancesProto retrieves cross-engine Managed Database
// instances as proto messages for the proto-backed list path. page/page_size
// flow through withPaginationQuery, so the request matches
// httpListAllDatabaseInstances.
func (c *Client) httpListAllDatabaseInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	return listProtoElementsPaginated(ctx, c, "ListAllDatabaseInstances", endpointDatabaseAllInstances, page, pageSize,
		func() *linodev1.DatabaseInstance { return &linodev1.DatabaseInstance{} })
}

// httpListDatabaseInstancesProto retrieves MySQL Managed Database instances as
// proto messages for the proto-backed list path. page/page_size flow through
// withPaginationQuery, so the request matches httpListDatabaseInstances.
func (c *Client) httpListDatabaseInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	return listProtoElementsPaginated(ctx, c, "ListDatabaseInstances", endpointDatabaseInstances, page, pageSize,
		func() *linodev1.DatabaseInstance { return &linodev1.DatabaseInstance{} })
}

// httpListDatabasePostgreSQLInstancesProto retrieves PostgreSQL Managed Database
// instances as proto messages for the proto-backed list path. page/page_size
// flow through withPaginationQuery, so the request matches
// httpListDatabasePostgreSQLInstances.
func (c *Client) httpListDatabasePostgreSQLInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	return listProtoElementsPaginated(ctx, c, "ListDatabasePostgreSQLInstances", endpointDatabasePostgreSQLInstances, page, pageSize,
		func() *linodev1.DatabaseInstance { return &linodev1.DatabaseInstance{} })
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

// ListDatabasePostgreSQLInstances retrieves PostgreSQL Managed Database instances.
func (c *Client) httpListDatabasePostgreSQLInstances(ctx context.Context, page, pageSize int) ([]DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := withPaginationQuery(endpointDatabasePostgreSQLInstances, page, pageSize)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "ListDatabasePostgreSQLInstances", Err: err}
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

// GetDatabasePostgreSQLInstance retrieves one PostgreSQL Managed Database instance.
func (c *Client) httpGetDatabasePostgreSQLInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabasePostgreSQLInstance", Err: err}
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

// httpGetDatabaseInstanceSSLProto retrieves a MySQL database SSL certificate as a
// proto message.
func (c *Client) httpGetDatabaseInstanceSSLProto(ctx context.Context, instanceID int) (*linodev1.DatabaseSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/ssl"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseInstanceSSL", Err: err}
	}

	defer drainClose(resp)

	ssl := &linodev1.DatabaseSSL{}
	if err := c.handleProtoResponse(resp, ssl); err != nil {
		return nil, err
	}

	return ssl, nil
}

// GetDatabasePostgreSQLInstanceSSL retrieves the SSL CA certificate for a PostgreSQL Managed Database instance.
func (c *Client) httpGetDatabasePostgreSQLInstanceSSL(ctx context.Context, instanceID int) (*DatabaseSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/ssl"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabasePostgreSQLInstanceSSL", Err: err}
	}

	defer drainClose(resp)

	var ssl DatabaseSSL
	if err := c.handleResponse(resp, &ssl); err != nil {
		return nil, err
	}

	return &ssl, nil
}

// httpGetDatabasePostgreSQLInstanceSSLProto retrieves a PostgreSQL database SSL
// certificate as a proto message.
func (c *Client) httpGetDatabasePostgreSQLInstanceSSLProto(ctx context.Context, instanceID int) (*linodev1.DatabaseSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/ssl"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabasePostgreSQLInstanceSSL", Err: err}
	}

	defer drainClose(resp)

	ssl := &linodev1.DatabaseSSL{}
	if err := c.handleProtoResponse(resp, ssl); err != nil {
		return nil, err
	}

	return ssl, nil
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

// GetDatabasePostgreSQLInstanceCredentials retrieves PostgreSQL Managed Database credentials.
func (c *Client) httpGetDatabasePostgreSQLInstanceCredentials(ctx context.Context, instanceID int) (*DatabaseCredentials, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/credentials"

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabasePostgreSQLInstanceCredentials", Err: err}
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

// ResetDatabasePostgreSQLInstanceCredentials resets PostgreSQL Managed Database credentials.
func (c *Client) httpResetDatabasePostgreSQLInstanceCredentials(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/credentials/reset"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ResetDatabasePostgreSQLInstanceCredentials", Err: err}
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
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

// CreateDatabasePostgreSQLInstance creates or restores a PostgreSQL Managed Database instance.
func (c *Client) httpCreateDatabasePostgreSQLInstance(ctx context.Context, req *CreateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodPost, endpointDatabasePostgreSQLInstances, req)
	if err != nil {
		return nil, &NetworkError{Operation: "CreateDatabasePostgreSQLInstance", Err: err}
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

// UpdateDatabasePostgreSQLInstance updates one PostgreSQL Managed Database instance.
func (c *Client) httpUpdateDatabasePostgreSQLInstance(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	resp, err := c.makeRequest(ctx, http.MethodPut, endpoint, req)
	if err != nil {
		return nil, &NetworkError{Operation: "UpdateDatabasePostgreSQLInstance", Err: err}
	}

	defer drainClose(resp)

	var instance DatabaseInstance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// writeDatabaseInstanceProto issues a create/update request and decodes the API
// body into the proto DatabaseInstance element. The MySQL and PostgreSQL create
// and update paths only differ by method, endpoint, and operation label, so they
// route through this one helper to keep dupl happy.
func (c *Client) writeDatabaseInstanceProto(ctx context.Context, operation, method, endpoint string, body any) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, method, endpoint, body)
	if err != nil {
		return nil, &NetworkError{Operation: operation, Err: err}
	}

	defer drainClose(resp)

	instance := &linodev1.DatabaseInstance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// httpCreateDatabaseInstanceProto creates a MySQL Managed Database instance and
// decodes the response into the proto element.
func (c *Client) httpCreateDatabaseInstanceProto(ctx context.Context, req *CreateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	return c.writeDatabaseInstanceProto(ctx, "CreateDatabaseInstance", http.MethodPost, endpointDatabaseInstances, req)
}

// httpCreateDatabasePostgreSQLInstanceProto creates a PostgreSQL Managed Database
// instance and decodes the response into the proto element.
func (c *Client) httpCreateDatabasePostgreSQLInstanceProto(ctx context.Context, req *CreateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	return c.writeDatabaseInstanceProto(ctx, "CreateDatabasePostgreSQLInstance", http.MethodPost, endpointDatabasePostgreSQLInstances, req)
}

// httpUpdateDatabaseInstanceProto updates a MySQL Managed Database instance and
// decodes the response into the proto element.
func (c *Client) httpUpdateDatabaseInstanceProto(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	endpoint := endpointDatabaseInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	return c.writeDatabaseInstanceProto(ctx, "UpdateDatabaseInstance", http.MethodPut, endpoint, req)
}

// httpUpdateDatabasePostgreSQLInstanceProto updates a PostgreSQL Managed Database
// instance and decodes the response into the proto element.
func (c *Client) httpUpdateDatabasePostgreSQLInstanceProto(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	return c.writeDatabaseInstanceProto(ctx, "UpdateDatabasePostgreSQLInstance", http.MethodPut, endpoint, req)
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

// DeleteDatabasePostgreSQLInstance deletes one PostgreSQL Managed Database instance.
func (c *Client) httpDeleteDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID))

	resp, err := c.makeRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "DeleteDatabasePostgreSQLInstance", Err: err}
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

// PatchDatabasePostgreSQLInstance applies security patches and updates to one PostgreSQL Managed Database instance.
func (c *Client) httpPatchDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/patch"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "PatchDatabasePostgreSQLInstance", Err: err}
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

// SuspendDatabasePostgreSQLInstance suspends one active PostgreSQL Managed Database instance.
func (c *Client) httpSuspendDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/suspend"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "SuspendDatabasePostgreSQLInstance", Err: err}
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

// ResumeDatabasePostgreSQLInstance resumes one suspended PostgreSQL Managed Database instance.
func (c *Client) httpResumeDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabasePostgreSQLInstances + "/" + url.PathEscape(strconv.Itoa(instanceID)) + "/resume"

	resp, err := c.makeRequest(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return &NetworkError{Operation: "ResumeDatabasePostgreSQLInstance", Err: err}
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

// GetDatabasePostgreSQLConfig retrieves PostgreSQL Managed Database advanced parameters.
func (c *Client) httpGetDatabasePostgreSQLConfig(ctx context.Context) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRequest(ctx, http.MethodGet, endpointDatabasePostgreSQLConfig, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabasePostgreSQLConfig", Err: err}
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

// httpGetDatabaseEngineProto retrieves one Managed Database engine as a proto
// message.
func (c *Client) httpGetDatabaseEngineProto(ctx context.Context, engineID string) (*linodev1.DatabaseEngine, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	endpoint := endpointDatabaseEngines + "/" + url.PathEscape(engineID)

	resp, err := c.makeRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, &NetworkError{Operation: "GetDatabaseEngine", Err: err}
	}

	defer drainClose(resp)

	engine := &linodev1.DatabaseEngine{}
	if err := c.handleProtoResponse(resp, engine); err != nil {
		return nil, err
	}

	return engine, nil
}
