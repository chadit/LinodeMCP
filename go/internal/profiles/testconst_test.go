package profiles_test

// Shared tool-name constants used across the profiles test suite. Pulled out
// so goconst doesn't flag the same literal recurring in builtin_test.go and
// loader_test.go fixtures.
const (
	toolVolumesList   = "linode_volumes_list"
	toolVolumeCreate  = "linode_volume_create"
	toolVolumeDelete  = "linode_volume_delete"
	toolVolumeResize  = "linode_volume_resize"
	profileNameCustom = "my-custom"
)
