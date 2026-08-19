package toolhooks_test

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	presignedMethodID = "method"
	// caseAbsent names the row an argument is left out of.
	caseAbsent = "absent"
)

// TestPresignedURLCreateNormalizeUppercasesTheMethod pins the fold the enum
// rule depends on: a lowercase verb reaches the rule as the canonical one.
func TestPresignedURLCreateNormalizeUppercasesTheMethod(t *testing.T) {
	t.Parallel()

	for name, supplied := range map[string]any{
		caseAbsent: nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			arguments := map[string]any{argRegion: sampleRegion}
			if supplied != nil {
				arguments[presignedMethodID] = supplied
			}

			request := requestWith(arguments)
			toolhooks.LinodeObjectStoragePresignedURLCreateNormalize(&request)

			assertNormalizedMethod(t, &request, supplied)
		})
	}
}

// assertNormalizedMethod checks one normalize outcome: text folds to upper, a
// value of another type and an absent one are left exactly as they were.
func assertNormalizedMethod(t *testing.T, request *mcp.CallToolRequest, supplied any) {
	t.Helper()

	got, present := request.GetArguments()[presignedMethodID]

	text, isText := supplied.(string)
	if isText {
		if want := strings.ToUpper(text); got != want {
			t.Errorf("method = %v, want %v", got, want)
		}

		return
	}

	if supplied == nil && present {
		t.Errorf("method = %v, want it to stay absent", got)
	}

	if supplied != nil && got != supplied {
		t.Errorf("method = %v, want it untouched at %v", got, supplied)
	}
}
