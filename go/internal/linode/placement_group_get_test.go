package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const (
	keyPlacementGroupType   = "placement_group_type"
	keyPlacementGroupPolicy = "placement_group_policy"
	keyIsCompliant          = "is_compliant"
	keyLinodeID             = "linode_id"
	keyMembers              = "members"
	keyRegion               = "region"
	placementGroupTypeLocal = "anti_affinity:local"
	placementGroupPolicy    = "strict"
	regionUSMIA             = "us-mia"
)

func TestClientGetPlacementGroupRoute(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/placement/groups/528" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/placement/groups/528")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID:                   528,
			keyLabel:                "PG_Miami_failover",
			keyRegion:               regionUSMIA,
			keyPlacementGroupType:   placementGroupTypeLocal,
			keyPlacementGroupPolicy: placementGroupPolicy,
			keyIsCompliant:          true,
			keyMembers: []map[string]any{{
				keyLinodeID:    123,
				keyIsCompliant: true,
			}},
			"migrations": map[string]any{
				"inbound":  []map[string]any{{keyLinodeID: 456}},
				"outbound": []map[string]any{{keyLinodeID: 789}},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	group, err := client.GetPlacementGroup(t.Context(), 528)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if group == nil {
		t.Fatal("group is nil")
	}

	if group.ID != 528 {
		t.Errorf("group.ID = %v, want %v", group.ID, 528)
	}

	if group.Label != tcPGMiamiFailover {
		t.Errorf("group.Label = %v, want %v", group.Label, tcPGMiamiFailover)
	}

	if group.Region != regionUSMIA {
		t.Errorf("group.Region = %v, want %v", group.Region, regionUSMIA)
	}

	if group.PlacementGroupType != placementGroupTypeLocal {
		t.Errorf("group.PlacementGroupType = %v, want %v", group.PlacementGroupType, placementGroupTypeLocal)
	}

	if group.PlacementGroupPolicy != placementGroupPolicy {
		t.Errorf("group.PlacementGroupPolicy = %v, want %v", group.PlacementGroupPolicy, placementGroupPolicy)
	}

	if !group.IsCompliant {
		t.Error("group.IsCompliant = false, want true")
	}

	if len(group.Members) != 1 {
		t.Fatalf("len(group.Members) = %d, want 1", len(group.Members))
	}

	if group.Members[0].LinodeID != 123 {
		t.Errorf("group.Members[0].LinodeID = %v, want %v", group.Members[0].LinodeID, 123)
	}

	if group.Migrations == nil {
		t.Fatal("group.Migrations is nil")
	}

	if len(group.Migrations.Inbound) != 1 {
		t.Fatalf("len(group.Migrations.Inbound) = %d, want 1", len(group.Migrations.Inbound))
	}

	if group.Migrations.Inbound[0].LinodeID != 456 {
		t.Errorf("group.Migrations.Inbound[0].LinodeID = %v, want %v", group.Migrations.Inbound[0].LinodeID, 456)
	}

	if requestCount.Load() != int32(1) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(1))
	}
}

func TestClientGetPlacementGroupRetriesTransientGET(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := requestCount.Add(1)

		w.Header().Set("Content-Type", tcApplicationJSON)

		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)

			if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errTemporaryFailure}}}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyID: 528, keyLabel: "retry-placement", keyRegion: regionUSMIA,
			keyPlacementGroupType: placementGroupTypeLocal, keyPlacementGroupPolicy: placementGroupPolicy,
			keyIsCompliant: true, keyMembers: []map[string]any{},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(
		srv.URL,
		"test-token",
		nil,
		linode.WithMaxRetries(1),
		linode.WithBaseDelay(time.Millisecond),
		linode.WithMaxDelay(time.Millisecond),
		linode.WithJitter(false),
	)

	group, err := client.GetPlacementGroup(t.Context(), 528)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if group == nil {
		t.Fatal("group is nil")
	}

	if group.Label != "retry-placement" {
		t.Errorf("group.Label = %v, want %v", group.Label, "retry-placement")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
