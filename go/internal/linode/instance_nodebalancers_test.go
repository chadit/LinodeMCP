package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestListInstanceNodeBalancers(t *testing.T) {
	t.Parallel()

	nodeBalancers := []linode.NodeBalancer{{ID: 456, Label: "app-lb", Region: regionUSEast}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/linode/instances/123/nodebalancers" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/nodebalancers")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: nodeBalancers, keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListInstanceNodeBalancers(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if result[0].ID != 456 {
		t.Errorf("result[0].ID = %v, want %v", result[0].ID, 456)
	}

	if result[0].Label != "app-lb" {
		t.Errorf("result[0].Label = %v, want %v", result[0].Label, "app-lb")
	}
}

func TestListInstanceNodeBalancersRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		linodeID int
	}{
		{name: "zero linode ID", linodeID: 0},
		{name: "negative linode ID", linodeID: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := linode.NewClient("https://api.example.test/v4", "test-token", nil, linode.WithMaxRetries(0))

			result, err := client.ListInstanceNodeBalancers(t.Context(), tt.linodeID)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if result != nil {
				t.Errorf("result = %v, want nil", result)
			}

			if !errors.Is(err, linode.ErrLinodeIDPositive) {
				t.Errorf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
			}
		})
	}
}
