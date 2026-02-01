package version_test

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/version"
)

// Note: These tests are NOT parallel because some modify the package-level
// BuildDate variable via SetBuildDate, which would cause data races.

func TestGet_ReturnsCorrectVersionInfo(t *testing.T) {
	info := version.Get()

	assert.Equal(t, version.Version, info.Version, "Version should match the constant.")
	assert.Equal(t, version.APIVersion, info.APIVersion, "APIVersion should match the constant.")
	assert.Equal(t, runtime.Version(), info.GoVersion, "GoVersion should match runtime.")
	expectedPlatform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	assert.Equal(t, expectedPlatform, info.Platform, "Platform should match runtime GOOS/GOARCH.")
}

func TestGet_BuildDateUnknownWhenEmpty(t *testing.T) {
	original := version.SetBuildDate("")
	t.Cleanup(func() { version.SetBuildDate(original) })

	info := version.Get()
	assert.Equal(t, "unknown", info.BuildDate, "BuildDate should be 'unknown' when empty.")
}

func TestGet_BuildDatePreservedWhenSet(t *testing.T) {
	original := version.SetBuildDate("2024-01-15T10:00:00Z")
	t.Cleanup(func() { version.SetBuildDate(original) })

	info := version.Get()
	assert.Equal(t, "2024-01-15T10:00:00Z", info.BuildDate, "BuildDate should be preserved when set.")
}

func TestGet_FeaturesPopulated(t *testing.T) {
	info := version.Get()

	require.NotNil(t, info.Features, "Features map should not be nil.")
	assert.Equal(t, "hello,version", info.Features["tools"])
	assert.Equal(t, "basic", info.Features["logging"])
	assert.Equal(t, "mcp", info.Features["protocol"])
}

func TestInfo_String(t *testing.T) {
	info := version.Get()
	s := info.String()

	assert.True(t, strings.HasPrefix(s, "LinodeMCP v"), "String should start with 'LinodeMCP v'.")
	assert.Contains(t, s, info.Version, "String should contain the version.")
	assert.Contains(t, s, info.APIVersion, "String should contain the API version.")
	assert.Contains(t, s, info.Platform, "String should contain the platform.")
}
