package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	simpleWriteInstancePath = "/linode/instances/123"
	simpleWriteDiskPath     = "/linode/instances/123/disks/5"
	simpleWriteIPPath       = "/linode/instances/123/ips/192.0.2.10"
	simpleWriteNewLabel     = "newlabel"
	simpleWriteDiskLabel    = "boot-disk"
	simpleWriteHostLabel    = "snapshot-source"
	simpleWritePriorRDNS    = "prior.example.com"
	argDiskID               = "disk_id"
	argSimpleLabel          = "label"
	simpleWriteDiskSize     = 25600
	simpleWriteCloneEffect  = `Disk "boot-disk" (25600 MB) is cloned to a new disk on the same instance,` +
		" consuming 25600 MB of additional storage."
	simpleWriteResetEffect  = "The root password for disk 5 on instance 123 will be reset."
	simpleWriteResetWarning = "Existing disk root password access will be replaced."
)

// simpleWriteStateServer answers every request with one JSON body, which is all
// a preview hook needs: it makes exactly one fetch and reports what came back.
func simpleWriteStateServer(t *testing.T, state map[string]any) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(state); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// simpleWritePreview reads a preview result back as the envelope a caller gets,
// so a case asserts on fields rather than on substrings of the rendered JSON.
func simpleWritePreview(t *testing.T, result *mcp.CallToolResult) map[string]any {
	t.Helper()

	var envelope map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &envelope); err != nil {
		t.Fatalf("decode preview: %v", err)
	}

	return envelope
}

// simpleWriteState is the current_state one preview reported.
func simpleWriteState(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()

	state, isObject := envelope["current_state"].(map[string]any)
	if !isObject {
		t.Fatalf("current_state = %v, want an object", envelope["current_state"])
	}

	return state
}

// simpleWriteEffects is the side_effects list one preview reported.
func simpleWriteEffects(t *testing.T, envelope map[string]any) []any {
	t.Helper()

	effects, isList := envelope["side_effects"].([]any)
	if !isList {
		t.Fatalf("side_effects = %v, want a list", envelope["side_effects"])
	}

	return effects
}

// TestLinodeInstanceDiskClonePreviewNamesTheStorageTheCopyTakes pins what the
// clone preview owes a caller: the disk it copies, and the storage the copy
// consumes, which only the fetched size can say.
func TestLinodeInstanceDiskClonePreviewNamesTheStorageTheCopyTakes(t *testing.T) {
	t.Parallel()

	server := simpleWriteStateServer(t, map[string]any{
		argSimpleLabel: simpleWriteDiskLabel, "size": simpleWriteDiskSize,
	})
	request := requestWith(map[string]any{
		argLinodeID: float64(123), argDiskID: float64(5), keyDryRun: true,
	})

	result, err := toolhooks.LinodeInstanceDiskClonePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost,
		simpleWriteDiskPath+"/clone", map[string]any{},
	)
	if err != nil {
		t.Fatalf("LinodeInstanceDiskClonePreview: %v", err)
	}

	envelope := simpleWritePreview(t, result)
	if got := simpleWriteState(t, envelope)[argSimpleLabel]; got != simpleWriteDiskLabel {
		t.Errorf("current_state label = %v, want %v", got, simpleWriteDiskLabel)
	}

	effects := simpleWriteEffects(t, envelope)
	if len(effects) != 1 || effects[0] != simpleWriteCloneEffect {
		t.Errorf("side_effects = %v, want [%q]", effects, simpleWriteCloneEffect)
	}
}
