package linode_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs the package suite under goleak as a whole-suite goroutine-leak
// net: a goroutine still running after all tests finish (an HTTP client
// connection, retry, or circuit/rate-limit goroutine that wasn't cleaned up)
// fails the build. synctest covers the timed components; this is the broad net.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
