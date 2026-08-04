package linode

import (
	"context"
	"net/http"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// Managed Databases split by engine: MySQL and PostgreSQL each own a route
// family, and only the instance list has a cross-engine variant.

// httpListDatabaseEnginesProto lists available Managed Database engines.
func (c *Client) httpListDatabaseEnginesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseEngine, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListDatabaseEngines",
		"linode_database_engine_list", "", nil, page, pageSize,
		func() *linodev1.DatabaseEngine { return &linodev1.DatabaseEngine{} })
}

// httpListDatabaseTypesProto lists available Managed Database node types.
func (c *Client) httpListDatabaseTypesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseType, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListDatabaseTypes",
		"linode_database_type_list", "", nil, page, pageSize,
		func() *linodev1.DatabaseType { return &linodev1.DatabaseType{} })
}

// httpGetDatabaseTypeProto retrieves one Managed Database node type.
func (c *Client) httpGetDatabaseTypeProto(ctx context.Context, typeID string, page, pageSize int) (*linodev1.DatabaseType, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequestQuery(ctx, "linode_database_type_get", pageQuery(page, pageSize), nil, typeID)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseType", err)
	}

	defer drainClose(resp)

	databaseType := &linodev1.DatabaseType{}
	if err := c.handleProtoResponse(resp, databaseType); err != nil {
		return nil, err
	}

	return databaseType, nil
}

// httpListAllDatabaseInstancesProto lists Managed Database instances across engines.
func (c *Client) httpListAllDatabaseInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListAllDatabaseInstances",
		"linode_database_instance_list", "", nil, page, pageSize,
		func() *linodev1.DatabaseInstance { return &linodev1.DatabaseInstance{} })
}

// httpListDatabaseInstancesProto lists MySQL Managed Database instances.
func (c *Client) httpListDatabaseInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListDatabaseInstances",
		"linode_database_mysql_instance_list", "", nil, page, pageSize,
		func() *linodev1.DatabaseInstance { return &linodev1.DatabaseInstance{} })
}

// httpListDatabasePostgreSQLInstancesProto lists PostgreSQL Managed Database instances.
func (c *Client) httpListDatabasePostgreSQLInstancesProto(ctx context.Context, page, pageSize int) ([]*linodev1.DatabaseInstance, error) {
	return listProtoElementsPaginatedRouted(ctx, c, "ListDatabasePostgreSQLInstances",
		"linode_database_postgresql_instance_list", "", nil, page, pageSize,
		func() *linodev1.DatabaseInstance { return &linodev1.DatabaseInstance{} })
}

// GetDatabaseInstance retrieves one MySQL Managed Database instance.
func (c *Client) httpGetDatabaseInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseInstance", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	var instance DatabaseInstance
	if err := c.handleResponse(resp, &instance); err != nil {
		return nil, err
	}

	return &instance, nil
}

// httpGetDatabaseInstanceProto retrieves one MySQL Managed Database instance.
// The GET returns a bare instance object, so it decodes straight into the element.
func (c *Client) httpGetDatabaseInstanceProto(ctx context.Context, instanceID int) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.DatabaseInstance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// httpGetDatabasePostgreSQLInstanceProto retrieves one PostgreSQL Managed Database instance.
func (c *Client) httpGetDatabasePostgreSQLInstanceProto(ctx context.Context, instanceID int) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	instance := &linodev1.DatabaseInstance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// httpGetDatabaseInstanceSSLProto retrieves a MySQL database SSL certificate.
func (c *Client) httpGetDatabaseInstanceSSLProto(ctx context.Context, instanceID int) (*linodev1.DatabaseSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_ssl_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseInstanceSSL", err)
	}

	defer drainClose(resp)

	ssl := &linodev1.DatabaseSSL{}
	if err := c.handleProtoResponse(resp, ssl); err != nil {
		return nil, err
	}

	return ssl, nil
}

// httpGetDatabasePostgreSQLInstanceSSLProto retrieves a PostgreSQL database SSL certificate.
func (c *Client) httpGetDatabasePostgreSQLInstanceSSLProto(ctx context.Context, instanceID int) (*linodev1.DatabaseSSL, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_ssl_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabasePostgreSQLInstanceSSL", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_credentials_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseInstanceCredentials", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_credentials_get", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("GetDatabasePostgreSQLInstanceCredentials", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_credentials_reset", nil, instanceID)
	if err != nil {
		return nil, wrapRequestError("ResetDatabaseInstanceCredentials", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_credentials_reset", nil, instanceID)
	if err != nil {
		return wrapRequestError("ResetDatabasePostgreSQLInstanceCredentials", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// decodeDatabaseInstanceProto reads a create or update response body into the
// proto DatabaseInstance element. Each write path keeps its own request call:
// the offline route gate only reads tool names passed as literals.
func (c *Client) decodeDatabaseInstanceProto(resp *http.Response) (*linodev1.DatabaseInstance, error) {
	instance := &linodev1.DatabaseInstance{}
	if err := c.handleProtoResponse(resp, instance); err != nil {
		return nil, err
	}

	return instance, nil
}

// httpCreateDatabaseInstanceProto creates a MySQL Managed Database instance.
func (c *Client) httpCreateDatabaseInstanceProto(ctx context.Context, req *CreateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateDatabaseInstance", err)
	}

	defer drainClose(resp)

	return c.decodeDatabaseInstanceProto(resp)
}

// httpCreateDatabasePostgreSQLInstanceProto creates a PostgreSQL Managed Database instance.
func (c *Client) httpCreateDatabasePostgreSQLInstanceProto(ctx context.Context, req *CreateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_create", req)
	if err != nil {
		return nil, wrapRequestError("CreateDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	return c.decodeDatabaseInstanceProto(resp)
}

// httpUpdateDatabaseInstanceProto updates a MySQL Managed Database instance.
func (c *Client) httpUpdateDatabaseInstanceProto(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_update", req, instanceID)
	if err != nil {
		return nil, wrapRequestError("UpdateDatabaseInstance", err)
	}

	defer drainClose(resp)

	return c.decodeDatabaseInstanceProto(resp)
}

// httpUpdateDatabasePostgreSQLInstanceProto updates a PostgreSQL Managed Database instance.
func (c *Client) httpUpdateDatabasePostgreSQLInstanceProto(ctx context.Context, instanceID int, req *UpdateDatabaseInstanceRequest) (*linodev1.DatabaseInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_update", req, instanceID)
	if err != nil {
		return nil, wrapRequestError("UpdateDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	return c.decodeDatabaseInstanceProto(resp)
}

// DeleteDatabaseInstance deletes one MySQL Managed Database instance.
func (c *Client) httpDeleteDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_delete", nil, instanceID)
	if err != nil {
		return wrapRequestError("DeleteDatabaseInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// DeleteDatabasePostgreSQLInstance deletes one PostgreSQL Managed Database instance.
func (c *Client) httpDeleteDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_delete", nil, instanceID)
	if err != nil {
		return wrapRequestError("DeleteDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// PatchDatabaseInstance applies security patches and updates to one MySQL Managed Database instance.
func (c *Client) httpPatchDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_patch", nil, instanceID)
	if err != nil {
		return wrapRequestError("PatchDatabaseInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// PatchDatabasePostgreSQLInstance applies security patches and updates to one PostgreSQL Managed Database instance.
func (c *Client) httpPatchDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_patch", nil, instanceID)
	if err != nil {
		return wrapRequestError("PatchDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// SuspendDatabaseInstance suspends one active MySQL Managed Database instance.
func (c *Client) httpSuspendDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_suspend", nil, instanceID)
	if err != nil {
		return wrapRequestError("SuspendDatabaseInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// SuspendDatabasePostgreSQLInstance suspends one active PostgreSQL Managed Database instance.
func (c *Client) httpSuspendDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_suspend", nil, instanceID)
	if err != nil {
		return wrapRequestError("SuspendDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResumeDatabaseInstance resumes one suspended MySQL Managed Database instance.
func (c *Client) httpResumeDatabaseInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_instance_resume", nil, instanceID)
	if err != nil {
		return wrapRequestError("ResumeDatabaseInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// ResumeDatabasePostgreSQLInstance resumes one suspended PostgreSQL Managed Database instance.
func (c *Client) httpResumeDatabasePostgreSQLInstance(ctx context.Context, instanceID int) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_instance_resume", nil, instanceID)
	if err != nil {
		return wrapRequestError("ResumeDatabasePostgreSQLInstance", err)
	}

	defer drainClose(resp)

	return c.handleResponse(resp, nil)
}

// GetDatabaseMySQLConfig retrieves MySQL Managed Database advanced parameters.
func (c *Client) httpGetDatabaseMySQLConfig(ctx context.Context) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_mysql_config_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseMySQLConfig", err)
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

	resp, err := c.makeRouteRequest(ctx, "linode_database_postgresql_config_get", nil)
	if err != nil {
		return nil, wrapRequestError("GetDatabasePostgreSQLConfig", err)
	}

	defer drainClose(resp)

	var config map[string]any
	if err := c.handleResponse(resp, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// httpGetDatabaseEngineProto retrieves one Managed Database engine.
func (c *Client) httpGetDatabaseEngineProto(ctx context.Context, engineID string) (*linodev1.DatabaseEngine, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	resp, err := c.makeRouteRequest(ctx, "linode_database_engine_get", nil, engineID)
	if err != nil {
		return nil, wrapRequestError("GetDatabaseEngine", err)
	}

	defer drainClose(resp)

	engine := &linodev1.DatabaseEngine{}
	if err := c.handleProtoResponse(resp, engine); err != nil {
		return nil, err
	}

	return engine, nil
}
