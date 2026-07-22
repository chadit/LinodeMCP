package profiles_test

// Shared tool-name constants used across the profiles test suite. Pulled out
// so goconst doesn't flag the same literal recurring in builtin_test.go,
// loader_test.go, and scope_test.go fixtures.
const (
	toolVolumesList              = "linode_volume_list"
	toolVolumeTypeList           = "linode_volume_type_list"
	toolVolumeCreate             = "linode_volume_create"
	toolVolumeClone              = "linode_volume_clone"
	toolVolumeDelete             = "linode_volume_delete"
	toolVolumeResize             = "linode_volume_resize"
	toolProfile                  = "linode_profile_get"
	toolAccount                  = "linode_account_get"
	toolInstancesList            = "linode_instance_list"
	toolLinodeInstanceConfigList = "linode_instance_config_list"
	toolInstanceDelete           = "linode_instance_delete"
	profileNameCustom            = "my-custom"

	toolDatabaseEngineGet           = "linode_database_engine_get"
	toolDatabaseEngineList          = "linode_database_engine_list"
	toolDatabaseTypeGet             = "linode_database_type_get"
	toolDatabaseTypeList            = "linode_database_type_list"
	toolDatabaseMySQLConfigGet      = "linode_database_mysql_config_get"
	toolDatabaseMySQLInstanceList   = "linode_database_mysql_instance_list"
	toolDatabaseMySQLInstanceDelete = "linode_database_mysql_instance_delete"
	toolDatabaseMySQLCredentialsGet = "linode_database_mysql_instance_credentials_get"
)
