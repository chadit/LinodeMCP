package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeNodeBalancerGetToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLRejectLocalhost, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeNodeBalancerGetTool(cfg)

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingNodeBalancerID, args: map[string]any{}, wantContains: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorLinodeID}, wantContains: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue}, wantContains: errNodeBalancerIDInteger},
		{name: caseNegativeNodeBalancerID, args: map[string]any{keyNodeBalancerID: float64(-1)}, wantContains: errNodeBalancerIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			expectNoError(t, err, "handler should not return Go error")
			expectNotNil(t, result, "handler should return a result")
			checkTrueWithMode(t, false, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}
}
