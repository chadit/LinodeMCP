package toolhooks_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// The presigned-URL method reaches its enum rule upper-cased, so a caller who
// wrote "get" is not refused for a spelling the API does not care about.
func TestPresignedURLNormalizeUpperCasesTheMethod(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{"method": "get"})
	toolhooks.LinodeObjectStoragePresignedURLCreateNormalize(&request)

	if got := request.GetString("method", ""); got != "GET" {
		t.Errorf("method = %q, want it upper-cased", got)
	}

	absent := requestWith(map[string]any{})
	toolhooks.LinodeObjectStoragePresignedURLCreateNormalize(&absent)

	if _, supplied := absent.GetArguments()["method"]; supplied {
		t.Error("method was added, want it left absent")
	}
}
