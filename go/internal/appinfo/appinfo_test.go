package appinfo_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/appinfo"
)

func TestGet(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()

	if got, want := info.Version, appinfo.Version; got != want {
		t.Fatalf("Version = %q, want %q", got, want)
	}

	if got, want := info.APIVersion, appinfo.APIVersion; got != want {
		t.Fatalf("APIVersion = %q, want %q", got, want)
	}

	if got, want := info.BuildDate, "unknown"; got != want {
		t.Fatalf("BuildDate = %q, want %q", got, want)
	}

	if got, want := info.Commit, "unknown"; got != want {
		t.Fatalf("Commit = %q, want %q", got, want)
	}

	if info.Platform == "" {
		t.Fatal("Platform is empty")
	}
}
