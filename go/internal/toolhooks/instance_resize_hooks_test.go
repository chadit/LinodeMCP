package toolhooks_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	resizePath        = "/linode/instances/123/resize"
	resizeTargetType  = "g6-standard-2"
	resizeCurrentType = "g6-standard-1"
	resizeBillingNote = "Resizing changes the monthly price to match the new type."
	resizeBothTypes   = "Instance resizes from type g6-standard-1 to g6-standard-2; " +
		"it reboots and is unavailable during the resize."
	resizeTargetOnly = "Instance resizes to type g6-standard-2; " +
		"it reboots and is unavailable during the resize."
)

// resizeServer answers the instance read and the disk list a resize plan needs.
// A nil disks slice answers the list with a failure, which is the branch that
// decides whether a plan can be hashed at all.
func resizeServer(t *testing.T, instance map[string]any, disks []map[string]any) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/disks") {
			if disks == nil {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			if err := json.NewEncoder(w).Encode(map[string]any{"data": disks}); err != nil {
				t.Errorf("write disks: %v", err)
			}

			return
		}

		if err := json.NewEncoder(w).Encode(instance); err != nil {
			t.Errorf("write instance: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server
}

// TestLinodeInstanceResizePreviewWordsBothTypes covers the two sentences the
// change can be described by: the type it leaves known, and a read that
// answered without one.
func TestLinodeInstanceResizePreviewWordsBothTypes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state map[string]any
		name  string
		want  string
	}{
		{
			name:  "current type known",
			state: map[string]any{keyType: resizeCurrentType},
			want:  resizeBothTypes,
		},
		{
			name:  "current type unknown",
			state: map[string]any{},
			want:  resizeTargetOnly,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := simpleWriteStateServer(t, testCase.state)
			request := requestWith(map[string]any{
				argInstanceID: float64(123), keyType: resizeTargetType, keyDryRun: true,
			})

			result, err := toolhooks.LinodeInstanceResizePreview(
				t.Context(), &request, configFor(server.URL), http.MethodPost,
				resizePath, map[string]any{keyType: resizeTargetType},
			)
			if err != nil {
				t.Fatalf("LinodeInstanceResizePreview: %v", err)
			}

			envelope := simpleWritePreview(t, result)
			effects, warnings := previewSentences(t, envelope)

			wantOnly(t, effects, []string{testCase.want})
			wantOnly(t, warnings, []string{resizeBillingNote})
		})
	}
}

// TestLinodeInstanceResizePreviewReportsTheInstance pins the state half: the
// preview reads the Linode whose plan the resize replaces, not the projection a
// plan hashes.
func TestLinodeInstanceResizePreviewReportsTheInstance(t *testing.T) {
	t.Parallel()

	server := simpleWriteStateServer(t, map[string]any{
		argSimpleLabel: simpleWriteHostLabel, keyType: resizeCurrentType,
	})
	request := requestWith(map[string]any{
		argInstanceID: float64(123), keyType: resizeTargetType, keyDryRun: true,
	})

	result, err := toolhooks.LinodeInstanceResizePreview(
		t.Context(), &request, configFor(server.URL), http.MethodPost,
		resizePath, map[string]any{keyType: resizeTargetType},
	)
	if err != nil {
		t.Fatalf("LinodeInstanceResizePreview: %v", err)
	}

	envelope := simpleWritePreview(t, result)
	if got := simpleWriteState(t, envelope)[argSimpleLabel]; got != simpleWriteHostLabel {
		t.Errorf("current_state label = %v, want %v", got, simpleWriteHostLabel)
	}
}

// TestLinodeInstanceResizeFetchStateCarriesTheDisks proves the projection is
// the pair a resize moves. A plan hashing the instance alone would apply over a
// disk layout that changed under it.
func TestLinodeInstanceResizeFetchStateCarriesTheDisks(t *testing.T) {
	t.Parallel()

	server := resizeServer(t,
		map[string]any{keyType: resizeCurrentType},
		[]map[string]any{{keyGrantID: 456, sizeArg: 25600, "filesystem": "ext4"}})

	client := clientFor(t, server.URL)

	state, err := toolhooks.LinodeInstanceResizeFetchState(t.Context(), client, 123)
	if err != nil {
		t.Fatalf("LinodeInstanceResizeFetchState: %v", err)
	}

	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}

	const want = `{"type":"g6-standard-1","disks":[{"filesystem":"ext4","id":456,"size":25600}]}`
	if got := string(raw); got != want {
		t.Errorf("hashed state = %s, want %s", got, want)
	}
}

// TestLinodeInstanceResizeFetchStateReportsAFailedDiskList proves a partial
// projection is refused rather than hashed: a plan that hashed the instance
// alone would compare it against the pair at apply time and refuse a resource
// nothing changed.
func TestLinodeInstanceResizeFetchStateReportsAFailedDiskList(t *testing.T) {
	t.Parallel()

	server := resizeServer(t, map[string]any{keyType: resizeCurrentType}, nil)
	client := clientFor(t, server.URL)

	state, err := toolhooks.LinodeInstanceResizeFetchState(t.Context(), client, 123)
	if !errors.Is(err, toolhooks.ErrResizeDiskList) {
		t.Errorf("fetch answered (%v, %v), want a failure naming the disk list", state, err)
	}
}

// TestLinodeInstanceResizeFetchStateReportsAFailedInstanceRead is the other
// half of the pair, split out because the instance read fails before there is a
// server path to distinguish.
func TestLinodeInstanceResizeFetchStateReportsAFailedInstanceRead(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := clientFor(t, server.URL)

	_, err := toolhooks.LinodeInstanceResizeFetchState(t.Context(), client, 123)
	if !errors.Is(err, toolhooks.ErrResizeInstanceRead) {
		t.Errorf("failure = %v, want it to name the instance read", err)
	}
}

// TestLinodeInstanceResizeDependencyWalkReadsBothStateShapes proves the walk
// words the change from the projection a plan hashes as readily as from the
// instance a preview reports, and answers nothing about a state it cannot read.
func TestLinodeInstanceResizeDependencyWalkReadsBothStateShapes(t *testing.T) {
	t.Parallel()

	server := resizeServer(t,
		map[string]any{keyType: resizeCurrentType},
		[]map[string]any{{keyGrantID: 456, sizeArg: 25600, "filesystem": "ext4"}})

	client := clientFor(t, server.URL)

	projection, err := toolhooks.LinodeInstanceResizeFetchState(t.Context(), client, 123)
	if err != nil {
		t.Fatalf("LinodeInstanceResizeFetchState: %v", err)
	}

	cases := []struct {
		state any
		name  string
		want  string
	}{
		{name: "the projection a plan hashes", state: projection, want: resizeBothTypes},
		{name: "the instance a preview reports", state: &linode.Instance{Type: resizeCurrentType}, want: resizeBothTypes},
		{name: "a state with no type", state: nil, want: resizeTargetOnly},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := requestWith(map[string]any{
				argInstanceID: float64(123), keyType: resizeTargetType,
			})

			details, err := toolhooks.LinodeInstanceResizeDependencyWalk(
				t.Context(), client, &request, testCase.state,
			)
			if err != nil {
				t.Fatalf("LinodeInstanceResizeDependencyWalk: %v", err)
			}

			if len(details.SideEffects) != 1 || details.SideEffects[0] != testCase.want {
				t.Errorf("side effects = %v, want %q", details.SideEffects, testCase.want)
			}

			if len(details.Warnings) != 1 || details.Warnings[0] != resizeBillingNote {
				t.Errorf("warnings = %v, want %q", details.Warnings, resizeBillingNote)
			}
		})
	}
}

// TestLinodeInstanceResizeDependencyWalkStopsWithTheCall proves the walk honors
// a canceled call rather than describing a change nobody is waiting on.
func TestLinodeInstanceResizeDependencyWalkStopsWithTheCall(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	request := requestWith(map[string]any{argInstanceID: float64(123), keyType: resizeTargetType})

	if _, err := toolhooks.LinodeInstanceResizeDependencyWalk(ctx, nil, &request, nil); err == nil {
		t.Error("walk answered a canceled call, want a failure")
	}
}
