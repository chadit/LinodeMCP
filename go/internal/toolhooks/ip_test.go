package toolhooks_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// argTags is the tag list several previews are asked to replace.
const argTags = "tags"

// TestLinodeNetworkingReservedIPCreatePreviewCarriesTheBilling pins the two
// things a reservation preview owes a caller: that billing starts on create,
// and that the amount is region-dependent rather than known here.
func TestLinodeNetworkingReservedIPCreatePreviewCarriesTheBilling(t *testing.T) {
	t.Parallel()

	request := requestWith(map[string]any{
		argRegion: usEast,
		keyDryRun: true,
	})

	result, err := toolhooks.LinodeNetworkingReservedIPCreatePreview(
		t.Context(), &request, configFor(""), http.MethodPost,
		"/networking/reserved/ips", nil,
	)
	if err != nil {
		t.Fatalf("LinodeNetworkingReservedIPCreatePreview: %v", err)
	}

	text := resultText(t, result)
	for _, want := range []string{
		`"billing_delta"`,
		"Reserved IP billing begins when the address is created.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("preview is missing %q:\n%s", want, text)
		}
	}
}
