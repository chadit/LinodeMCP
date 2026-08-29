package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
)

// The refusal sentences the meta tools answer, pinned through their generated
// handlers because the bodies they used to live in are declarations now.

func TestAuditReportRefusesAnUnknownReport(t *testing.T) {
	t.Parallel()

	request := createRequestWithArgs(t, map[string]any{managedContactNameParam: "does-not-exist"})

	_, _, handler := gentools.NewLinodeAuditReportTool(&config.Config{})

	result, err := handler(t.Context(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Fatal("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "unknown report") {
		t.Errorf("answer does not name the cause:\n%s", textContent.Text)
	}
}

// The generated handler is where a canceled call is answered now that version
// declares its answer, so the case reads it there rather than at a body that
// no longer exists.
func TestVersionRefusesACanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, _, handler := gentools.NewVersionTool(&config.Config{})

	if _, err := handler(ctx, mcp.CallToolRequest{}); err == nil {
		t.Fatal("version accepted a canceled context")
	}
}
