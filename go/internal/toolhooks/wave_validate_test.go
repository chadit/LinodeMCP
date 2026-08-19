package toolhooks_test

import (
	"testing"
)

// The argument keys this wave's tools are addressed by, beyond the ones the
// read, service, and collection tables already name.
const (
	argAddress = "address"
	argAlertID = "alert_id"
	argName    = "name"
)

// The emitter derives "<name> is required" for a path field and nothing at all
// for a query one, so without these tables a generated tool could change its
// wording, or stop checking, with only a caller to notice.

// TestWaveValidateHooks pins the sentences the Read-tier tools migrated in this
// wave answer for a bad argument.
func TestWaveValidateHooks(t *testing.T) {
	t.Parallel()

	families := [][]hookTable{
		waveIPTables(),
		waveCollectionTables(),
	}

	tables := make([]hookTable, 0)
	for _, family := range families {
		tables = append(tables, family...)
	}

	runValidateTables(t, tables)
}

// waveIPTables covers the routes addressed by an IP address or a CIDR prefix,
// where a derived check can only report that the argument is missing.
func waveIPTables() []hookTable {
	return []hookTable{}
}

// waveCollectionTables covers the sub-resource lists and the monthly stats
// read, whose ids and bounds a derived check cannot report.
func waveCollectionTables() []hookTable {
	return []hookTable{}
}
