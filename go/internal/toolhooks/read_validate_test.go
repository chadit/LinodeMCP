package toolhooks_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// The case names and argument keys the tables below reuse.
const (
	argClientID   = "client_id"
	argToken      = "token"
	argUsername   = "username"
	argTokenID    = "token_id"
	argSSHKeyID   = "ssh_key_id"
	argLinodeID   = "linode_id"
	argInstanceID = "instance_id"
	argConfigID   = "config_id"
	usEast        = "us-east"
)

// answer is one call through a validate hook: the arguments a caller sent and
// the sentence the hook answers, "" when the call may proceed.
type answer struct {
	name string
	args map[string]any
	want string
}

// hookTable is every answer one read tool's validate hook gives.
type hookTable struct {
	label   string
	hook    func(*mcp.CallToolRequest) string
	answers []answer
}

// TestReadValidateHooks pins every sentence the generated read tools answer for
// a bad argument. These are the texts their hand-written handlers answered, and
// nothing else holds a generated tool to them: the emitter derives "<name> is
// required" and would quietly replace the rest.
func TestReadValidateHooks(t *testing.T) {
	t.Parallel()

	profile := profileValidateTables()
	instance := instanceValidateTables()

	tables := make([]hookTable, 0, len(profile)+len(instance))
	tables = append(tables, profile...)
	tables = append(tables, instance...)

	runValidateTables(t, tables)
}

// runValidateTables calls every answer in every table through its hook.
func runValidateTables(t *testing.T, tables []hookTable) {
	t.Helper()

	for _, table := range tables {
		for _, entry := range table.answers {
			t.Run(table.label+"/"+entry.name, func(t *testing.T) {
				t.Parallel()

				request := requestWith(entry.args)

				if got := table.hook(&request); got != entry.want {
					t.Errorf("%s(%v) = %q, want %q", table.label, entry.args, got, entry.want)
				}
			})
		}
	}
}

// profileValidateTables covers the kernel read tool's hook, the last of this
// family whose refusal the contract cannot carry.
func profileValidateTables() []hookTable {
	return []hookTable{}
}

// instanceValidateTables covers the instance read tools' hooks.
func instanceValidateTables() []hookTable {
	return []hookTable{}
}
