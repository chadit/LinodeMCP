package tools_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs the package suite under goleak as a whole-suite goroutine-leak
// net: a goroutine still running after all tests finish (most often an httptest
// server or HTTP client connection a handler test failed to close) fails the
// build.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
