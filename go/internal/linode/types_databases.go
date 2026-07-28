package linode

import "encoding/json"

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
	Engines    DatabaseTypeEngines `json:"engines"`
	Disk       int                 `json:"disk"`
	Memory     int                 `json:"memory"`
	VCPUs      int                 `json:"vcpus"`
	Deprecated bool                `json:"deprecated"`
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
	Version         string   `json:"version"`
	Created         string   `json:"created"`
	Label           string   `json:"label"`
	Region          string   `json:"region"`
	Type            string   `json:"type"`
	Engine          string   `json:"engine"`
	ReplicationType string   `json:"replication_type"`
	Updated         string   `json:"updated"`
	Status          string   `json:"status"`
	AllowList       []string `json:"allow_list"`
	ClusterSize     int      `json:"cluster_size"`
	ID              int      `json:"id"`
	SSLConnection   bool     `json:"ssl_connection"`
	Encrypted       bool     `json:"encrypted"`
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
	EngineConfig   map[string]any `json:"engine_config,omitempty"`
	Fork           map[string]any `json:"fork,omitempty"`
	PrivateNetwork map[string]any `json:"private_network,omitempty"`
	SSLConnection  *bool          `json:"ssl_connection,omitempty"`
	Label          string         `json:"label"`
	Type           string         `json:"type"`
	Engine         string         `json:"engine"`
	Region         string         `json:"region"`
	AllowList      []string       `json:"allow_list,omitempty"`
	ClusterSize    int            `json:"cluster_size,omitempty"`
}

// UpdateDatabaseInstanceRequest updates a MySQL Managed Database instance.
//
// PrivateNetwork is a json.RawMessage so the update can express three distinct
// wire states the Linode API treats differently: omitted (leave the VPC binding
// untouched), an object (attach or reconfigure), and an explicit null (detach
// from the VPC). A nil/empty map cannot carry the explicit-null case because
// omitempty would drop it, so the raw bytes are held verbatim instead.
type UpdateDatabaseInstanceRequest struct {
	AllowList      *[]string       `json:"allow_list,omitempty"`
	EngineConfig   map[string]any  `json:"engine_config,omitempty"`
	Label          *string         `json:"label,omitempty"`
	Type           *string         `json:"type,omitempty"`
	Updates        map[string]any  `json:"updates,omitempty"`
	Version        *string         `json:"version,omitempty"`
	PrivateNetwork json.RawMessage `json:"private_network,omitempty"`
}
