package server_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs the package suite under goleak as a whole-suite goroutine-leak
// net: a goroutine still running after all tests finish (a server background
// goroutine that wasn't drained, for example) fails the build.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
