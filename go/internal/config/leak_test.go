package config_test

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs the package suite under goleak as a whole-suite goroutine-leak
// net: a goroutine still running after all tests finish (a Watcher poll loop
// that wasn't stopped, for example) fails the build. synctest covers the
// bubble-scoped cases precisely; this catches anything outside a bubble.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
