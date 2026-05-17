package profiles_test

// Shared tool-name constants used across the profiles test suite. Pulled out
// so goconst doesn't flag the same literal recurring in builtin_test.go,
// loader_test.go, and scope_test.go fixtures.
const (
	toolVolumesList    = "linode_volumes_list"
	toolVolumeCreate   = "linode_volume_create"
	toolVolumeDelete   = "linode_volume_delete"
	toolVolumeResize   = "linode_volume_resize"
	toolProfile        = "linode_profile"
	toolAccount        = "linode_account"
	toolInstancesList  = "linode_instances_list"
	toolInstanceDelete = "linode_instance_delete"
	profileNameCustom  = "my-custom"
)
