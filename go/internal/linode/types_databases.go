package linode

// DatabaseEngine represents a Managed Database engine.
type DatabaseEngine struct {
	ID      string `json:"id"`
	Engine  string `json:"engine"`
	Version string `json:"version"`
}
