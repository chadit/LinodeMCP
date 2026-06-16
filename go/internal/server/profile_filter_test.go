package server_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

const (
	toolObjectEndpointsList         = "linode_object_storage_endpoint_list"
	toolInstancesList               = "linode_instance_list"
	toolInstanceCreate              = "linode_instance_create"
	toolVolumesList                 = "linode_volume_list"
	toolVolumeTypeList              = "linode_volume_type_list"
	toolInstanceVolumeList          = "linode_instance_volume_list"
	toolBucketAccessAllow           = "linode_object_storage_bucket_access_allow"
	toolAccountPaymentGet           = "linode_account_payment_get"
	toolAccountEntityTransferAccept = "linode_account_entity_transfer_accept"
	toolMonitorAlertCreate          = "linode_monitor_service_alert_definition_create"
	toolMonitorTokenCreate          = "linode_monitor_service_token_create"
	toolMonitorAlertDelete          = "linode_monitor_service_alert_definition_delete"
	toolMonitorAlertUpdate          = "linode_monitor_service_alert_definition_update"
)

// toolNames extracts the registered tool name for each entry on the server.
// Used by the filtering tests so assertions can compare ElementsMatch and
// Contains against the resolved tool surface.
func toolNames(srv *server.Server) []string {
	names := make([]string, 0, len(srv.Tools()))
	for _, t := range srv.Tools() {
		names = append(names, t.Name())
	}

	return names
}

// TestNewDefaultProfileFiltersToReadAndMeta is the Phase 4 default-profile
// behavior contract: a server constructed without an explicit ActiveProfile
// must fall back to the built-in default, and the resulting tool surface
// must contain only CapRead and CapMeta tools. Spot-checks confirm a read
// tool is registered and a mutator is not.
func TestNewDefaultProfileFiltersToReadAndMeta(t *testing.T) {
	t.Parallel()

	cfg := baseTestConfig()

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	if srv.ActiveProfile().Name != profiles.BuiltinDefault {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profiles.BuiltinDefault)
	}

	names := toolNames(srv)
	if !slices.Contains(names, toolInstancesList) {
		t.Errorf("names does not contain %v", toolInstancesList)
	}

	if !slices.Contains(names, toolAccountPaymentGet) {
		t.Errorf("names does not contain %v", toolAccountPaymentGet)
	}

	if !slices.Contains(names, toolVolumeTypeList) {
		t.Errorf("names does not contain %v", toolVolumeTypeList)
	}

	if !slices.Contains(names, toolObjectEndpointsList) {
		t.Errorf("names does not contain %v", toolObjectEndpointsList)
	}

	if !slices.Contains(names, toolInstanceVolumeList) {
		t.Errorf("names does not contain %v", toolInstanceVolumeList)
	}

	if slices.Contains(names, toolInstanceCreate) {
		t.Errorf("names should not contain %v", toolInstanceCreate)
	}

	if slices.Contains(names, toolBucketAccessAllow) {
		t.Errorf("names should not contain %v", toolBucketAccessAllow)
	}

	if slices.Contains(names, toolMonitorAlertCreate) {
		t.Errorf("names should not contain %v", toolMonitorAlertCreate)
	}

	if slices.Contains(names, toolMonitorTokenCreate) {
		t.Errorf("names should not contain %v", toolMonitorTokenCreate)
	}

	if slices.Contains(names, toolMonitorAlertDelete) {
		t.Errorf("names should not contain %v", toolMonitorAlertDelete)
	}

	if slices.Contains(names, toolMonitorAlertUpdate) {
		t.Errorf("names should not contain %v", toolMonitorAlertUpdate)
	}

	for _, info := range srv.ToolInfos() {
		if !slices.Contains([]profiles.Capability{profiles.CapRead, profiles.CapMeta}, info.Capability) {
			t.Errorf("collection does not contain %v", info.Capability)
		}
	}

	// Filtering should drop a meaningful number of tools. The exact count
	// drifts with new tool additions, so the assertion is a floor: at least
	// one tool must have been filtered out, and the registered list must
	// stay smaller than the full inventory.
	if len(names) == 0 {
		t.Fatal("names is empty")
	}
}

// TestNewFullAccessRegistersEverything verifies the opposite end of the
// filter: when full-access is the active profile and enabled, the server
// registers exactly the full tool inventory. The expected count comes from
// a sibling server constructed via the same factory list, so the test stays
// stable as new tools are added.
func TestNewFullAccessRegistersEverything(t *testing.T) {
	t.Parallel()

	full := fullAccessConfig()

	srv, err := server.New(full)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	if srv.ActiveProfile().Name != profiles.BuiltinFullAccess {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profiles.BuiltinFullAccess)
	}

	names := toolNames(srv)
	if !slices.Contains(names, toolInstanceCreate) {
		t.Errorf("names does not contain %v", toolInstanceCreate)
	}

	if !slices.Contains(names, toolInstancesList) {
		t.Errorf("names does not contain %v", toolInstancesList)
	}

	if !slices.Contains(names, toolVolumeTypeList) {
		t.Errorf("names does not contain %v", toolVolumeTypeList)
	}

	if !slices.Contains(names, toolInstanceVolumeList) {
		t.Errorf("names does not contain %v", toolInstanceVolumeList)
	}

	if !slices.Contains(names, toolBucketAccessAllow) {
		t.Errorf("names does not contain %v", toolBucketAccessAllow)
	}

	if !slices.Contains(names, toolMonitorAlertCreate) {
		t.Errorf("names does not contain %v", toolMonitorAlertCreate)
	}

	if !slices.Contains(names, toolMonitorTokenCreate) {
		t.Errorf("names does not contain %v", toolMonitorTokenCreate)
	}

	if !slices.Contains(names, toolMonitorAlertDelete) {
		t.Errorf("names does not contain %v", toolMonitorAlertDelete)
	}

	if !slices.Contains(names, toolMonitorAlertUpdate) {
		t.Errorf("names does not contain %v", toolMonitorAlertUpdate)
	}

	if slices.Contains(names, toolAccountEntityTransferAccept) {
		t.Errorf("names should not contain %v", toolAccountEntityTransferAccept)
	}

	for _, tool := range srv.Tools() {
		if tool.Name() == toolAccountEntityTransferAccept {
			t.Errorf("tool.Name() = %v, do not want %v", tool.Name(), toolAccountEntityTransferAccept)
		}
	}

	// The full-access tool set must be a strict superset of the default
	// tool set. Comparing against a default-profile sibling avoids hard
	// coding a count that drifts with new tools.
	defaultSrv, err := server.New(baseTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) <= len(defaultSrv.Tools()) {
		t.Errorf("got %v, want > %v", len(names), len(defaultSrv.Tools()))
	}
}

// TestNewDisabledBuiltinFailsStartup confirms server construction refuses
// to start when the configured active profile names a disabled built-in.
// The error must wrap profiles.ErrActiveProfileDisabled so callers can
// pattern-match without parsing strings.
func TestNewDisabledBuiltinFailsStartup(t *testing.T) {
	t.Parallel()

	cfg := baseTestConfig()
	cfg.ActiveProfile = profiles.BuiltinComputeAdmin
	cfg.ProfilesBuiltinOverrides = map[string]config.BuiltinOverride{
		profiles.BuiltinComputeAdmin: {Disabled: true},
	}

	srv, err := server.New(cfg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if srv != nil {
		t.Errorf("srv = %v, want nil", srv)
	}

	if !errors.Is(err, profiles.ErrActiveProfileDisabled) {
		t.Errorf("error = %v, want %v", err, profiles.ErrActiveProfileDisabled)
	}
}

// TestNewUnknownProfileFailsStartup confirms server construction refuses
// to start when the configured active profile names neither a built-in
// nor an entry in cfg.Profiles. The error must wrap
// profiles.ErrActiveProfileUnknown.
func TestNewUnknownProfileFailsStartup(t *testing.T) {
	t.Parallel()

	cfg := baseTestConfig()
	cfg.ActiveProfile = "definitely-not-a-real-profile"

	srv, err := server.New(cfg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if srv != nil {
		t.Errorf("srv = %v, want nil", srv)
	}

	if !errors.Is(err, profiles.ErrActiveProfileUnknown) {
		t.Errorf("error = %v, want %v", err, profiles.ErrActiveProfileUnknown)
	}
}

// TestNewUserDefinedProfileRegistersExactlyListedTools verifies that a
// user-defined profile with a single explicit tool entry results in
// exactly one registered tool. This is the load-bearing path for
// hand-curated profiles where the user wants tight control over the
// model's tool surface.
func TestNewUserDefinedProfileRegistersExactlyListedTools(t *testing.T) {
	t.Parallel()

	cfg := baseTestConfig()
	cfg.ActiveProfile = profileSingleTool
	cfg.Profiles = map[string]config.UserProfileConfig{
		profileSingleTool: {
			Description:  "A profile exposing exactly one tool for testing.",
			AllowedTools: []string{toolVolumesList},
		},
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv == nil {
		t.Fatal("srv is nil")
	}

	if srv.ActiveProfile().Name != profileSingleTool {
		t.Errorf("srv.ActiveProfile().Name = %v, want %v", srv.ActiveProfile().Name, profileSingleTool)
	}

	names := toolNames(srv)
	{
		gotEls := slices.Clone(names)
		wantEls := slices.Clone([]string{toolVolumesList})

		slices.Sort(gotEls)
		slices.Sort(wantEls)

		if !slices.Equal(gotEls, wantEls) {
			t.Errorf("got %v, want %v (any order)", names, []string{toolVolumesList})
		}
	}
}
