package linode

// DatabaseEngine represents a Managed Database engine.
type DatabaseEngine struct {
	ID      string `json:"id"`
	Engine  string `json:"engine"`
	Version string `json:"version"`
}

// DatabaseType represents a Managed Database node plan type.
type DatabaseType struct {
	ID         string              `json:"id"`
	Label      string              `json:"label"`
	Class      string              `json:"class"`
	Disk       int                 `json:"disk"`
	Memory     int                 `json:"memory"`
	VCPUs      int                 `json:"vcpus"`
	Deprecated bool                `json:"deprecated"`
	Engines    DatabaseTypeEngines `json:"engines"`
}

// DatabaseTypeEngines contains engine-specific node quantities and prices.
type DatabaseTypeEngines struct {
	MySQL      []DatabaseTypeEngine `json:"mysql"`
	PostgreSQL []DatabaseTypeEngine `json:"postgresql"`
}

// DatabaseTypeEngine represents pricing for one Managed Database node quantity.
type DatabaseTypeEngine struct {
	Quantity int   `json:"quantity"`
	Price    Price `json:"price"`
}

// DatabaseInstance represents a Managed Database instance.
type DatabaseInstance struct {
	ID              int      `json:"id"`
	Status          string   `json:"status"`
	Label           string   `json:"label"`
	Region          string   `json:"region"`
	Type            string   `json:"type"`
	Engine          string   `json:"engine"`
	Version         string   `json:"version"`
	ClusterSize     int      `json:"cluster_size"`
	ReplicationType string   `json:"replication_type"`
	SSLConnection   bool     `json:"ssl_connection"`
	Encrypted       bool     `json:"encrypted"`
	AllowList       []string `json:"allow_list"`
	Created         string   `json:"created"`
	Updated         string   `json:"updated"`
}

// DatabaseCredentials contains MySQL Managed Database credentials.
type DatabaseCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DatabaseSSL contains the SSL CA certificate for a MySQL Managed Database.
type DatabaseSSL struct {
	CACertificate string `json:"ca_certificate"`
}

// CreateDatabaseInstanceRequest creates or restores a MySQL Managed Database instance.
type CreateDatabaseInstanceRequest struct {
	Label          string         `json:"label"`
	Type           string         `json:"type"`
	Engine         string         `json:"engine"`
	Region         string         `json:"region"`
	AllowList      []string       `json:"allow_list,omitempty"`
	ClusterSize    int            `json:"cluster_size,omitempty"`
	EngineConfig   map[string]any `json:"engine_config,omitempty"`
	Fork           map[string]any `json:"fork,omitempty"`
	PrivateNetwork *bool          `json:"private_network,omitempty"`
	SSLConnection  *bool          `json:"ssl_connection,omitempty"`
}

// UpdateDatabaseInstanceRequest updates a MySQL Managed Database instance.
type UpdateDatabaseInstanceRequest struct {
	AllowList      *[]string      `json:"allow_list,omitempty"`
	EngineConfig   map[string]any `json:"engine_config,omitempty"`
	Label          *string        `json:"label,omitempty"`
	PrivateNetwork map[string]any `json:"private_network,omitempty"`
	Type           *string        `json:"type,omitempty"`
	Updates        map[string]any `json:"updates,omitempty"`
	Version        *string        `json:"version,omitempty"`
}
