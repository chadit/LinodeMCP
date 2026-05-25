package profiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chadit/LinodeMCP/internal/profiles"
)

func TestLongviewCategory(t *testing.T) {
	t.Parallel()

	assert.Contains(t, profiles.Categories("linode_longview_plan"), "monitor")
	assert.Contains(t, profiles.Categories("linode_longview_client_create"), "monitor")
}
