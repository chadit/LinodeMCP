package linode

import (
	"context"
)

// Managed Databases split by engine: MySQL and PostgreSQL each own a route
// family.

// httpGetDatabaseInstance retrieves one MySQL Managed Database instance.
func (c *Client) httpGetDatabaseInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	return routedGet[DatabaseInstance](ctx, c, "GetDatabaseInstance", "linode_database_mysql_instance_get", instanceID)
}

// httpGetDatabasePostgreSQLInstance retrieves one PostgreSQL Managed Database instance.
func (c *Client) httpGetDatabasePostgreSQLInstance(ctx context.Context, instanceID int) (*DatabaseInstance, error) {
	return routedGet[DatabaseInstance](ctx, c, "GetDatabasePostgreSQLInstance", "linode_database_postgresql_instance_get", instanceID)
}
