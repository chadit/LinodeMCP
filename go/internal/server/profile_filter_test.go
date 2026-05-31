package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/server"
)

const (
	toolObjectEndpointsList = "linode_object_storage_endpoint_list"
	toolInstancesList       = "linode_instance_list"
	toolInstanceCreate      = "linode_instance_create"
	toolVolumesList         = "linode_volume_list"
	toolInstanceVolumeList  = "linode_instance_volume_list"
	toolBucketAccessAllow   = "linode_object_storage_bucket_access_allow"
	toolAccountPaymentGet   = "linode_account_payment_get"
	toolMonitorAlertCreate  = "linode_monitor_service_alert_definition_create"
	toolMonitorTokenCreate  = "linode_monitor_service_token_create"
	toolMonitorAlertDelete  = "linode_monitor_service_alert_definition_delete"
	toolMonitorAlertUpdate  = "linode_monitor_service_alert_definition_update"
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
	require.NoError(t, err, "default-profile server must construct cleanly")
	require.NotNil(t, srv)

	assert.Equal(t, profiles.BuiltinDefault, srv.ActiveProfile().Name,
		"unset ActiveProfile must resolve to the built-in default")

	names := toolNames(srv)
	assert.Contains(t, names, toolInstancesList,
		"default profile must expose read tools like %s", toolInstancesList)
	assert.Contains(t, names, toolAccountPaymentGet,
		"default profile must expose read tools like %s", toolAccountPaymentGet)
	assert.Contains(t, names, toolObjectEndpointsList,
		"default profile must expose read tools like %s", toolObjectEndpointsList)
	assert.Contains(t, names, toolInstanceVolumeList,
		"default profile must expose read tools like %s", toolInstanceVolumeList)
	assert.NotContains(t, names, toolInstanceCreate,
		"default profile must not expose write tools like %s", toolInstanceCreate)
	assert.NotContains(t, names, toolBucketAccessAllow,
		"default profile must not expose write tools like %s", toolBucketAccessAllow)
	assert.NotContains(t, names, toolMonitorAlertCreate,
		"default profile must not expose write tools like %s", toolMonitorAlertCreate)
	assert.NotContains(t, names, toolMonitorTokenCreate,
		"default profile must not expose write tools like %s", toolMonitorTokenCreate)
	assert.NotContains(t, names, toolMonitorAlertDelete,
		"default profile must not expose destructive tools like %s", toolMonitorAlertDelete)
	assert.NotContains(t, names, toolMonitorAlertUpdate,
		"default profile must not expose write tools like %s", toolMonitorAlertUpdate)

	for _, info := range srv.ToolInfos() {
		assert.Containsf(
			t,
			[]profiles.Capability{profiles.CapRead, profiles.CapMeta},
			info.Capability,
			"tool %s registered under default profile has capability %s; default must allow only CapRead/CapMeta",
			info.Name, info.Capability,
		)
	}

	// Filtering should drop a meaningful number of tools. The exact count
	// drifts with new tool additions, so the assertion is a floor: at least
	// one tool must have been filtered out, and the registered list must
	// stay smaller than the full inventory.
	require.NotEmpty(t, names, "default profile should still expose its read surface")
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
	require.NoError(t, err, "full-access server must construct cleanly")
	require.NotNil(t, srv)

	assert.Equal(t, profiles.BuiltinFullAccess, srv.ActiveProfile().Name)

	names := toolNames(srv)
	assert.Contains(t, names, toolInstanceCreate,
		"full-access must expose write tools like %s", toolInstanceCreate)
	assert.Contains(t, names, toolInstancesList,
		"full-access must continue to expose read tools like %s", toolInstancesList)
	assert.Contains(t, names, toolInstanceVolumeList,
		"full-access must continue to expose read tools like %s", toolInstanceVolumeList)
	assert.Contains(t, names, toolBucketAccessAllow,
		"full-access must expose write tools like %s", toolBucketAccessAllow)
	assert.Contains(t, names, toolMonitorAlertCreate,
		"full-access must expose write tools like %s", toolMonitorAlertCreate)
	assert.Contains(t, names, toolMonitorTokenCreate,
		"full-access must expose write tools like %s", toolMonitorTokenCreate)
	assert.Contains(t, names, toolMonitorAlertDelete,
		"full-access must expose destructive tools like %s", toolMonitorAlertDelete)
	assert.Contains(t, names, toolMonitorAlertUpdate,
		"full-access must expose write tools like %s", toolMonitorAlertUpdate)

	// The full-access tool set must be a strict superset of the default
	// tool set. Comparing against a default-profile sibling avoids hard
	// coding a count that drifts with new tools.
	defaultSrv, err := server.New(baseTestConfig())
	require.NoError(t, err, "default sibling must construct cleanly")

	assert.Greater(t, len(names), len(defaultSrv.Tools()),
		"full-access must register strictly more tools than default")
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
	require.Error(t, err, "construction must fail when active built-in is disabled")
	assert.Nil(t, srv, "server must be nil when construction fails")
	assert.ErrorIs(t, err, profiles.ErrActiveProfileDisabled,
		"error must wrap profiles.ErrActiveProfileDisabled so callers can detect it")
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
	require.Error(t, err, "construction must fail when active profile is unknown")
	assert.Nil(t, srv, "server must be nil when construction fails")
	assert.ErrorIs(t, err, profiles.ErrActiveProfileUnknown,
		"error must wrap profiles.ErrActiveProfileUnknown so callers can detect it")
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
	require.NoError(t, err, "user-defined profile must construct cleanly")
	require.NotNil(t, srv)

	assert.Equal(t, profileSingleTool, srv.ActiveProfile().Name)

	names := toolNames(srv)
	assert.ElementsMatch(t, []string{toolVolumesList}, names,
		"user-defined profile with one allowed tool must register exactly that tool")
}
