package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const monitorAlertDefinitionGroupBy = "entity_id"

func monitorAlertDefinitionCloneRequest() *linode.CloneAlertDefinitionRequest {
	var (
		description string
		severity    int
	)

	channelIDs := []int{}
	entityIDs := []string{}
	groupBy := []string{monitorAlertDefinitionGroupBy}
	regions := []string{}
	ruleCriteria := map[string]any{}
	triggerConditions := map[string]any{}

	return &linode.CloneAlertDefinitionRequest{
		ChannelIDs:        &channelIDs,
		Description:       &description,
		EntityIDs:         &entityIDs,
		GroupBy:           &groupBy,
		Label:             monitorAlertDefinitionLabel + " Clone",
		Regions:           &regions,
		RuleCriteria:      &ruleCriteria,
		Severity:          &severity,
		TriggerConditions: &triggerConditions,
	}
}

func TestClientCloneMonitorServiceAlertDefinitionProtoSuccessAndEscapedPath(t *testing.T) {
	t.Parallel()

	wantPath := "/monitor/services/dbaas%2Fpostgres/alert-definitions/20000/clone"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.EscapedPath() != wantPath {
			t.Errorf("r.URL.EscapedPath() = %v, want %v", r.URL.EscapedPath(), wantPath)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)

			return
		}

		for _, key := range []string{"channel_ids", keyDescription, "entity_ids", "group_by", "regions", "rule_criteria", "severity", "trigger_conditions"} {
			if _, ok := body[key]; !ok {
				t.Errorf("body missing key %v", key)
			}
		}

		if !reflect.DeepEqual(body["channel_ids"], []any{}) {
			t.Errorf("body[channel_ids] = %#v, want empty array", body["channel_ids"])
		}

		if !reflect.DeepEqual(body["entity_ids"], []any{}) {
			t.Errorf("body[entity_ids] = %#v, want empty array", body["entity_ids"])
		}

		if !reflect.DeepEqual(body["regions"], []any{}) {
			t.Errorf("body[regions] = %#v, want empty array", body["regions"])
		}

		if !reflect.DeepEqual(body["rule_criteria"], map[string]any{}) {
			t.Errorf("body[rule_criteria] = %#v, want empty object", body["rule_criteria"])
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:          monitorAlertDefinitionID + 1,
			keyLabel:       monitorAlertDefinitionLabel + " Clone",
			keyServiceType: monitorServiceTypeWithSlash,
			keySeverity:    0,
			"group_by":     []string{monitorAlertDefinitionGroupBy},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CloneMonitorServiceAlertDefinitionProto(
		t.Context(),
		monitorServiceTypeWithSlash,
		monitorAlertDefinitionID,
		monitorAlertDefinitionCloneRequest(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.GetId() != int32(monitorAlertDefinitionID+1) {
		t.Errorf("got.GetId() = %v, want %v", got.GetId(), int32(monitorAlertDefinitionID+1))
	}

	if !reflect.DeepEqual(got.GetGroupBy(), []string{monitorAlertDefinitionGroupBy}) {
		t.Errorf("got.GetGroupBy() = %v, want %v", got.GetGroupBy(), []string{monitorAlertDefinitionGroupBy})
	}
}

func TestClientCloneMonitorServiceAlertDefinitionProtoDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))

	got, err := client.CloneMonitorServiceAlertDefinitionProto(
		t.Context(),
		monitorServiceTypeDatabase,
		monitorAlertDefinitionID,
		monitorAlertDefinitionCloneRequest(),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}

func TestClientCloneMonitorServiceAlertDefinitionProtoWrapsNetworkError(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://127.0.0.1:1", "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.CloneMonitorServiceAlertDefinitionProto(
		t.Context(),
		monitorServiceTypeDatabase,
		monitorAlertDefinitionID,
		monitorAlertDefinitionCloneRequest(),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	if _, ok := errors.AsType[*linode.NetworkError](err); !ok {
		t.Errorf("err = %T, want *linode.NetworkError", err)
	}
}
