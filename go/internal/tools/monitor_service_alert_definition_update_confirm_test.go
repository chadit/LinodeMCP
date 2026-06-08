package tools_test

import (
	"strings"
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
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				if result == nil {
					t.Fatal("result is nil")
				}

				if !result.IsError {
					t.Error("result.IsError = false, want true")
				}

				textContent, ok := result.Content[0].(mcp.TextContent)
				if !ok {
					t.Fatal("ok = false, want true")
				}

				if !strings.Contains(textContent.Text, "confirm=true") {
					t.Errorf("textContent.Text does not contain %v", "confirm=true")
				}
			})
		}
	})
}
