package audit_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs the package suite under goleak as a whole-suite goroutine-leak
// net: a goroutine still running after all tests finish (a retention sweep or
// SQLite background loop that wasn't stopped, for example) fails the build.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
