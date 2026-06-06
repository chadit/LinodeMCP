package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	accountServiceTransfersTestPath = "/account/service-transfers"
	transferTestToken               = "tok-abc"
	serviceTransferGetPath          = accountServiceTransfersTestPath + "/" + transferTestToken
)

func TestLinodeAccountServiceTransferCreateToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountServiceTransferCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeAccountServiceTransferCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeIDs: []any{float64(123)},
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_service_transfer_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, accountServiceTransfersTestPath, would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")
	})
}

func TestLinodeAccountServiceTransferAcceptToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountServiceTransferAcceptTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without accepting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, serviceTransferGetPath, map[string]any{keyToken: transferTestToken})
		_, _, handler := tools.NewLinodeAccountServiceTransferAcceptTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyToken:  transferTestToken,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_service_transfer_accept", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, serviceTransferGetPath+"/accept", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeAccountServiceTransferDeleteToolDryRun(t *testing.T) {
	assert := accountAssert{}
	require := accountRequire{}

	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeAccountServiceTransferDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, serviceTransferGetPath, map[string]any{keyToken: transferTestToken})
		_, _, handler := tools.NewLinodeAccountServiceTransferDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyToken:  transferTestToken,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_account_service_transfer_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, serviceTransferGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}
