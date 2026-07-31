package server_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

func TestMonitorServiceAlertDefinitionCloneRegisteredAsWrite(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, info := range srv.ToolInfos() {
		if info.Name != "linode_monitor_service_alert_definition_clone" {
			continue
		}

		if info.Capability != profiles.CapWrite {
			t.Errorf("info.Capability = %v, want %v", info.Capability, profiles.CapWrite)
		}

		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(info.RawInputSchema, &schema); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantRequired := []string{"service_type", "alert_id", "label", "confirm"}
		if !reflect.DeepEqual(schema.Required, wantRequired) {
			t.Errorf("schema.Required = %v, want %v", schema.Required, wantRequired)
		}

		return
	}

	t.Fatal("linode_monitor_service_alert_definition_clone should be registered")
}
