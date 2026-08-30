package linode_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestIsObjectBody covers the inputs a live route cannot produce: an empty body
// never reaches the predicate because the JSON decode fails first, and a body
// carrying leading whitespace still has to read as an object.
func TestIsObjectBody(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		raw  string
		want bool
	}{
		"address object":         {raw: `{"address":"192.0.2.10"}`, want: true},
		"object behind newlines": {raw: "\r\n\t {}", want: true},
		"empty":                  {raw: "", want: false},
		"whitespace only":        {raw: "   ", want: false},
		"array behind spaces":    {raw: "  []", want: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := linode.IsObjectBody([]byte(testCase.raw)); got != testCase.want {
				t.Errorf("IsObjectBody(%q) = %v, want %v", testCase.raw, got, testCase.want)
			}
		})
	}
}
