package appinfo_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chadit/LinodeMCP/internal/appinfo"
)

func TestGet(t *testing.T) {
	t.Parallel()

	info := appinfo.Get()

	assert.Equal(t, appinfo.Version, info.Version)
	assert.Equal(t, appinfo.APIVersion, info.APIVersion)
	assert.Equal(t, "unknown", info.BuildDate)
	assert.NotEmpty(t, info.Platform)
}
