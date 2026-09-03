package config_test

import (
	"errors"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

// TestPresignTTLBounds pins the window config load holds
// objectStorage.presignTtlSeconds to. The value travels as the object-url
// route's expires_in, which refuses anything outside 360..3600, and config was
// the one path that could still hand the API a value the route rejects.
//
// The explicit-zero row is the Go/Python alignment: both sides read 0 as unset
// and fill the default rather than refusing it.
func TestPresignTTLBounds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		block   string
		want    int
		refused bool
	}{
		"block omitted": {
			block: "",
			want:  config.DefaultPresignTTLSeconds,
		},
		"explicit zero means unset": {
			block: "objectStorage:\n  presignTtlSeconds: 0\n",
			want:  config.DefaultPresignTTLSeconds,
		},
		"inside the window": {
			block: "objectStorage:\n  presignTtlSeconds: 900\n",
			want:  900,
		},
		"below the floor": {
			block:   "objectStorage:\n  presignTtlSeconds: 359\n",
			refused: true,
		},
		"above the ceiling": {
			block:   "objectStorage:\n  presignTtlSeconds: 3601\n",
			refused: true,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(testCase.block))

			cfg, err := config.Load(path)
			if testCase.refused {
				if !errors.Is(err, config.ErrConfigInvalid) ||
					!errors.Is(err, config.ErrPresignTTLOutOfRange) {
					t.Errorf("err = %v, want it to wrap both %v and %v",
						err, config.ErrConfigInvalid, config.ErrPresignTTLOutOfRange)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if cfg.ObjectStorage.PresignTTLSeconds != testCase.want {
				t.Errorf("cfg.ObjectStorage.PresignTTLSeconds = %v, want %v",
					cfg.ObjectStorage.PresignTTLSeconds, testCase.want)
			}
		})
	}
}

// TestPresignTTLRefusesAWrongType pins the Go half of a cross-language parity:
// a quoted, boolean or sequence value never reaches the window check because
// the yaml decode into an int field refuses it first. Python has no schema and
// so raises its own config error at the same point, which
// python/tests/unit/test_config_object_storage.py pins.
func TestPresignTTLRefusesAWrongType(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"quoted number": "objectStorage:\n  presignTtlSeconds: \"900\"\n",
		"boolean":       "objectStorage:\n  presignTtlSeconds: true\n",
		"sequence":      "objectStorage:\n  presignTtlSeconds: [900]\n",
	}

	for name, block := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := writeConfigFile(t, dir, "config.yml", minimalConfigWith(block))

			if _, err := config.Load(path); !errors.Is(err, config.ErrConfigMalformed) {
				t.Errorf("err = %v, want it to wrap %v", err, config.ErrConfigMalformed)
			}
		})
	}
}
