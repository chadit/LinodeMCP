package tools_test

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeMonitorServiceAlertDefinitionUpdateToolConfirmRequired(t *testing.T) {
	t.Parallel()

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseFalseConfirmRejected, value: false, set: true},
			{name: caseStringConfirmRejected, value: boolStringTrue, set: true},
			{name: caseNumericConfirmRejected, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := monitorAlertDefinitionUpdateArgs()
				if !testCase.set {
					delete(args, keyConfirm)
				}

				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeMonitorServiceAlertDefinitionUpdateTool(cfg)

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)
				requireNoError(t, err, "handler should return confirmation failures as tool errors")
				requireNotNil(t, result, "result should not be nil")
				assertTrue(t, result.IsError, "missing or invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				requireTrue(t, ok, "content should be TextContent")
				assertContains(t, textContent.Text, "confirm=true", "response should require confirm=true")
			})
		}
	})
}
