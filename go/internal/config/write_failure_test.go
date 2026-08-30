package config_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// TestWriteAtomicRefusesConfigJSONCannotEncode verifies a value JSON has no
// encoding for, a NaN sample rate, fails the write before any file is
// touched, so a nonsense value never replaces a good config.
func TestWriteAtomicRefusesConfigJSONCannotEncode(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.json")
	cfg := minimalWritableConfig()
	cfg.Observability.Tracing.SampleRate = math.NaN()

	err := config.WriteAtomic(path, cfg)

	unsupported, ok := errors.AsType[*json.UnsupportedValueError](err)
	if !ok {
		t.Fatalf("error = %v, want it to wrap the encoder's *json.UnsupportedValueError", err)
	}

	if unsupported.Str != "NaN" {
		t.Errorf("unsupported value = %q, want NaN", unsupported.Str)
	}

	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("stat after failed write = %v, want %v", statErr, fs.ErrNotExist)
	}
}

// TestWriteAtomicFailsWhenDirectoryCannotHoldTempFile verifies the write
// reports the directory it could not create the temp file in, for a parent
// that does not exist and for one the process cannot write to, and leaves
// no target file behind either way.
func TestWriteAtomicFailsWhenDirectoryCannotHoldTempFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		prepare     func(t *testing.T, root string) string
		name        string
		needNonRoot bool
	}{
		{
			name: "parent directory does not exist",
			prepare: func(t *testing.T, root string) string {
				t.Helper()

				return filepath.Join(root, "missing")
			},
		},
		{
			name: "parent directory is read-only",
			prepare: func(t *testing.T, root string) string {
				t.Helper()

				dir := filepath.Join(root, "sealed")
				if err := os.Mkdir(dir, 0o500); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

				return dir
			},
			needNonRoot: true,
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			if tcase.needNonRoot {
				requireNonRoot(t)
			}

			dir := tcase.prepare(t, t.TempDir())
			path := filepath.Join(dir, "config.yml")

			err := config.WriteAtomic(path, minimalWritableConfig())

			pathErr, ok := errors.AsType[*fs.PathError](err)
			if !ok {
				t.Fatalf("error = %v, want it to wrap the *fs.PathError the temp file open produced", err)
			}

			if filepath.Dir(pathErr.Path) != dir {
				t.Errorf("error names %q, want a temp file under %q", pathErr.Path, dir)
			}

			if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
				t.Errorf("stat after failed write = %v, want %v", statErr, fs.ErrNotExist)
			}
		})
	}
}

// TestWriteAtomicRefusesTargetThatIsADirectory verifies the final rename
// fails when the target path is a directory, the directory survives
// untouched, and the temp file is cleaned up so the parent is not littered.
func TestWriteAtomicRefusesTargetThatIsADirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()

	target := filepath.Join(parent, "config.yml")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := config.WriteAtomic(target, minimalWritableConfig())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !info.IsDir() {
		t.Error("target was replaced, want the directory left in place")
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") {
			t.Errorf("temp file %s left behind after the failed rename", entry.Name())
		}
	}
}
