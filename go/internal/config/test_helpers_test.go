package config_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func checkNoError(t *testing.T, err error, msg ...string) {
	t.Helper()

	if err != nil {
		if len(msg) > 0 {
			t.Fatalf("%s: %v", msg[0], err)
		}

		t.Fatalf("unexpected error: %v", err)
	}
}

func checkError(t *testing.T, err error, msg ...string) {
	t.Helper()

	if err == nil {
		if len(msg) > 0 {
			t.Fatalf("%s: got nil error", msg[0])
		}

		t.Fatal("expected error, got nil")
	}
}

func checkErrorIs(t *testing.T, err, target error, msg ...string) {
	t.Helper()

	if !errors.Is(err, target) {
		if len(msg) > 0 {
			t.Fatalf("%s: error = %v, want %v", msg[0], err, target)
		}

		t.Fatalf("error = %v, want %v", err, target)
	}
}

func checkNotNil[T any](t *testing.T, got *T, msg ...string) {
	t.Helper()

	if got == nil {
		if len(msg) > 0 {
			t.Fatal(msg[0])
		}

		t.Fatal("got nil")
	}
}

func checkEqual[T comparable](t *testing.T, want, got T, msg ...string) {
	t.Helper()

	if got != want {
		if len(msg) > 0 {
			t.Fatalf("%s: got %v, want %v", msg[0], got, want)
		}

		t.Fatalf("got %v, want %v", got, want)
	}
}

func checkDeepEqual(t *testing.T, want, got any, msg ...string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		if len(msg) > 0 {
			t.Fatalf("%s: got %#v, want %#v", msg[0], got, want)
		}

		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func checkTrue(t *testing.T, got bool, msg ...string) {
	t.Helper()

	if !got {
		if len(msg) > 0 {
			t.Fatal(msg[0])
		}

		t.Fatal("got false, want true")
	}
}

func checkFalse(t *testing.T, got bool, msg ...string) {
	t.Helper()

	if got {
		if len(msg) > 0 {
			t.Fatal(msg[0])
		}

		t.Fatal("got true, want false")
	}
}

func checkEventually(t *testing.T, condition func() bool, waitFor, tick time.Duration, msg string) {
	t.Helper()

	deadline := time.NewTimer(waitFor)
	defer deadline.Stop()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		if condition() {
			return
		}

		select {
		case <-deadline.C:
			t.Fatal(msg)
		case <-ticker.C:
		}
	}
}

func checkNotPanics(t *testing.T, callback func()) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("function panicked: %v", recovered)
		}
	}()

	callback()
}

func checkNotContains(t *testing.T, got, needle, msg string) {
	t.Helper()

	if strings.Contains(got, needle) {
		t.Fatalf("%s: %q contains %q", msg, got, needle)
	}
}
