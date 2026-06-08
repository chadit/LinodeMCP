package appinfo_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs the package suite under goleak as a whole-suite goroutine-leak
// net: any goroutine still running after the tests finish fails the build.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
