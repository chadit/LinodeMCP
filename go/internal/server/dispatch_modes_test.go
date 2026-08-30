package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

const (
	toolVolumeDelete   = "linode_volume_delete"
	toolInstanceDelete = "linode_instance_delete"
	volumePath789      = "/volumes/789"
	instancePath123    = "/linode/instances/123"
	keyVolumeID        = "volume_id"
	keyInstanceID      = "instance_id"
	keyMode            = "mode"
	keyPlanID          = "plan_id"
	keyYolo            = "yolo"
	modePlan           = "plan"
	modeApply          = "apply"

	// instanceStateJSON is the body the fake API serves for instance 123 on
	// every read, so plan and apply hash the same state.
	instanceStateJSON = `{"id":123,"label":"web-prod-01","status":"running"}`
)

// mutationLog records every mutating request a fake Linode API received, so a
// dispatch test can prove whether a destroy reached the wire.
type mutationLog struct {
	paths []string
	mu    sync.Mutex
}

func (l *mutationLog) add(path string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.paths = append(l.paths, path)
}

func (l *mutationLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.paths...)
}

// newProfileServer builds a Server running the named built-in profile against
// a fake API that serves instance 123 on GET and records every DELETE. The
// returned sink captures the audit event each dispatch writes.
func newProfileServer(t *testing.T, profile string) (*server.Server, *audit.CapturingSink, *mutationLog) {
	t.Helper()

	mutations := &mutationLog{}

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations.add(r.Method + " " + r.URL.Path)
			w.WriteHeader(http.StatusOK)

			return
		}

		if r.URL.Path != instancePath123 {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if _, err := w.Write([]byte(instanceStateJSON)); err != nil {
			t.Errorf("write instance: %v", err)
		}
	}))
	t.Cleanup(api.Close)

	cfg := baseTestConfig()
	cfg.ActiveProfile = profile
	cfg.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{profile: {Disabled: false}}

	env := cfg.Environments[envKeyDefault]
	env.Linode = config.LinodeConfig{APIURL: api.URL, Token: tokenShort}
	cfg.Environments[envKeyDefault] = env

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)

	return srv, sink, mutations
}

// eventMode returns the execution mode the audit sink recorded for the tool.
func eventMode(t *testing.T, sink *audit.CapturingSink, tool string) string {
	t.Helper()

	event := findEventByTool(sink.Events(), tool)
	if event == nil {
		t.Fatalf("no audit event for %s", tool)
	}

	return event.Mode
}

// TestDispatchYoloExecutesADestroyOnlyWhenTheProfileAllowsIt covers the yolo
// flag at the server's chokepoint: under a profile that allows it, a destroy
// with neither confirm nor a dry-run assertion runs and is audited as yolo;
// under any other profile the flag is ignored, the destroy gate refuses, and
// the event carries the normal mode.
func TestDispatchYoloExecutesADestroyOnlyWhenTheProfileAllowsIt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		profile       string
		wantMode      string
		wantMutations []string
		wantRefused   bool
	}{
		{
			name:          "emergency profile allows yolo",
			profile:       profiles.BuiltinEmergency,
			wantMode:      string(audit.ModeYolo),
			wantMutations: []string{http.MethodDelete + " " + volumePath789},
		},
		{
			name:        "full access profile ignores yolo",
			profile:     profiles.BuiltinFullAccess,
			wantMode:    string(audit.ModeNormal),
			wantRefused: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			srv, sink, mutations := newProfileServer(t, testCase.profile)

			isError, text := callServerTool(t, srv, nil, toolVolumeDelete, map[string]any{
				keyVolumeID: 789, keyYolo: true,
			})

			if isError != testCase.wantRefused {
				t.Fatalf("isError = %v, want %v; text: %s", isError, testCase.wantRefused, text)
			}

			if testCase.wantRefused && !strings.Contains(text, "is destructive") {
				t.Errorf("text = %q, want the destroy gate's refusal", text)
			}

			if got := mutations.snapshot(); strings.Join(got, ",") != strings.Join(testCase.wantMutations, ",") {
				t.Errorf("mutations = %v, want %v", got, testCase.wantMutations)
			}

			if got := eventMode(t, sink, toolVolumeDelete); got != testCase.wantMode {
				t.Errorf("audit mode = %q, want %q", got, testCase.wantMode)
			}
		})
	}
}

// TestDispatchPlanThenApplyThroughTheServersPlanStore covers the two-stage
// flow end to end through the server's own wire: the plan store the server
// attaches to every call lets an apply find the plan a prior call made, the
// DELETE goes out on apply only, and each call is audited under its mode.
func TestDispatchPlanThenApplyThroughTheServersPlanStore(t *testing.T) {
	t.Parallel()

	srv, sink, mutations := newProfileServer(t, profiles.BuiltinFullAccess)

	isError, planText := callServerTool(t, srv, nil, toolInstanceDelete, map[string]any{
		keyInstanceID: 123, keyMode: modePlan,
	})
	if isError {
		t.Fatalf("plan refused: %s", planText)
	}

	var plan map[string]any
	if err := json.Unmarshal([]byte(planText), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}

	planID, _ := plan[keyPlanID].(string)
	if planID == "" {
		t.Fatalf("plan has no plan_id: %s", planText)
	}

	if got := mutations.snapshot(); len(got) != 0 {
		t.Fatalf("plan issued mutations %v, want none", got)
	}

	if got := eventMode(t, sink, toolInstanceDelete); got != string(audit.ModePlan) {
		t.Errorf("plan audit mode = %q, want %q", got, audit.ModePlan)
	}

	isError, applyText := callServerTool(t, srv, nil, toolInstanceDelete, map[string]any{
		keyInstanceID: 123, keyMode: modeApply, keyPlanID: planID,
	})
	if isError {
		t.Fatalf("apply refused: %s", applyText)
	}

	if !strings.Contains(applyText, "removed successfully") {
		t.Errorf("apply text = %q, want the removal confirmed", applyText)
	}

	if got := mutations.snapshot(); strings.Join(got, ",") != http.MethodDelete+" "+instancePath123 {
		t.Errorf("mutations = %v, want the one DELETE", got)
	}

	events := sink.Events()
	if last := events[len(events)-1]; last.Tool != toolInstanceDelete || last.Mode != string(audit.ModeApply) {
		t.Errorf("last audit event = %s/%s, want %s in %s mode", last.Tool, last.Mode, toolInstanceDelete, audit.ModeApply)
	}
}

// TestToolWrapperExecuteReportsACanceledContext covers the public Execute
// surface under a context that is already done: the caller reads the
// cancellation rather than the not-implemented sentinel.
func TestToolWrapperExecuteReportsACanceledContext(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := srv.Tools()[0].Execute(ctx, nil)
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}

	if errors.Is(err, server.ErrExecuteNotImplemented) {
		t.Error("a canceled call must not be reported as not implemented")
	}
}

// TestToolsExposeTheCapabilityTheCatalogReports pins the accessor the audit
// middleware and the invariant tests read: every tool handed out by Tools
// answers the same capability and raw schema the ToolInfos snapshot lists.
func TestToolsExposeTheCapabilityTheCatalogReports(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infos := make(map[string]server.ToolInfo, len(srv.ToolInfos()))
	for _, info := range srv.ToolInfos() {
		infos[info.Name] = info
	}

	type capabilityCarrier interface {
		Capability() profiles.Capability
		RawTool() mcp.Tool
	}

	for _, tool := range srv.Tools() {
		carrier, ok := tool.(capabilityCarrier)
		if !ok {
			t.Fatalf("tool %s does not expose its capability", tool.Name())
		}

		info, listed := infos[tool.Name()]
		if !listed {
			t.Fatalf("tool %s is served but not listed", tool.Name())
		}

		if carrier.Capability() != info.Capability {
			t.Errorf("%s capability = %v, want %v", tool.Name(), carrier.Capability(), info.Capability)
		}

		if raw := carrier.RawTool(); raw.Name != tool.Name() || string(raw.RawInputSchema) != string(info.RawInputSchema) {
			t.Errorf("%s raw tool disagrees with its listing", tool.Name())
		}
	}
}

// TestValidateScopesRefusesAConfigWithNoEnvironment covers scope validation
// on a config that declares no environment at all: there is no token to
// inspect, so the call fails naming the missing environment and never reaches
// the API.
func TestValidateScopesRefusesAConfigWithNoEnvironment(t *testing.T) {
	t.Parallel()

	cfg := baseTestConfig()
	cfg.Environments = nil

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := srv.ValidateScopes(t.Context())
	if !errors.Is(err, config.ErrEnvironmentNotFound) {
		t.Fatalf("error = %v, want %v", err, config.ErrEnvironmentNotFound)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

// TestValidateScopesReportsAProfileFetchFailure covers the API refusing the
// /profile read: the caller gets the fetch failure wrapped, with no comparison
// to act on.
func TestValidateScopesReportsAProfileFetchFailure(t *testing.T) {
	t.Parallel()

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(httpSrv.Close)

	srv, err := server.New(configWithEnv(httpSrv.URL, "revoked-token"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := srv.ValidateScopes(t.Context())
	if !errors.Is(err, profiles.ErrProfileFetchFailed) {
		t.Fatalf("error = %v, want %v", err, profiles.ErrProfileFetchFailed)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
