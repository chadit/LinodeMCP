package toolhooks_test

import (
	"testing"
)

// TestCollectionValidateHooks pins the sentences the generated collection tools
// answer for a bad path argument, the way TestServiceValidateHooks does for the
// single-resource reads.
func TestCollectionValidateHooks(t *testing.T) {
	t.Parallel()

	families := [][]hookTable{
		collectionServiceTables(),
		collectionAccountTables(),
	}

	tables := make([]hookTable, 0)
	for _, family := range families {
		tables = append(tables, family...)
	}

	runValidateTables(t, tables)
}

// collectionServiceTables covers the lists nested under a service resource.
func collectionServiceTables() []hookTable {
	return []hookTable{}
}

// collectionAccountTables covers the account-level lists and the monitoring,
// Longview, support, tag, and region reads beside them.
func collectionAccountTables() []hookTable {
	return []hookTable{}
}
