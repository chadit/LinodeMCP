package profiles_test

// Shared tool-name constants used across the profiles test suite. Pulled out
// so goconst does not flag the same literal recurring in builtin_test.go
// and loader_test.go fixtures.
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
	toolInstanceCreate              = "linode_instance_create"

	toolHello                = "hello"
	toolVersion              = "version"
	toolSecurityQuestionList = "linode_profile_security_question_list"
	toolEntityList           = "linode_entity_list"

	toolIamIdpConfigDelete      = "linode_iam_idp_config_delete"
	toolIamRolePermissionUpdate = "linode_iam_user_role_permission_update"
	toolAccountUserGrantsGet    = "linode_account_user_grants_get"
	toolAccountUserGrantsUpdate = "linode_account_user_grants_update"
)

// builtinProfileCount is how many profiles ship in the binary. Written out
// rather than read from the catalog so growing the catalog fails the count
// cases instead of silently agreeing with itself.
const builtinProfileCount = 9

// Scope strings the synthetic catalog pins, spelled once for goconst.
const (
	scopeAccountReadOnly        = "account:read_only"
	scopeAccountReadWrite       = "account:read_write"
	scopeDatabasesReadOnly      = "databases:read_only"
	scopeDatabasesReadWrite     = "databases:read_write"
	scopeDomainsReadOnly        = "domains:read_only"
	scopeDomainsReadWrite       = "domains:read_write"
	scopeFirewallReadOnly       = "firewall:read_only"
	scopeFirewallReadWrite      = "firewall:read_write"
	scopeIPsReadOnly            = "ips:read_only"
	scopeIPsReadWrite           = "ips:read_write"
	scopeImagesReadOnly         = "images:read_only"
	scopeImagesReadWrite        = "images:read_write"
	scopeLKEReadOnly            = "lke:read_only"
	scopeLKEReadWrite           = "lke:read_write"
	scopeLinodesReadOnly        = "linodes:read_only"
	scopeLinodesReadWrite       = "linodes:read_write"
	scopeLongviewReadOnly       = "longview:read_only"
	scopeLongviewReadWrite      = "longview:read_write"
	scopeNodebalancersReadOnly  = "nodebalancers:read_only"
	scopeNodebalancersReadWrite = "nodebalancers:read_write"
	scopeObjectStorageReadOnly  = "object_storage:read_only"
	scopeObjectStorageReadWrite = "object_storage:read_write"
	scopeStackscriptsReadOnly   = "stackscripts:read_only"
	scopeStackscriptsReadWrite  = "stackscripts:read_write"
	scopeVPCReadWrite           = "vpc:read_write"
	scopeVolumesReadOnly        = "volumes:read_only"
	scopeVolumesReadWrite       = "volumes:read_write"
)

// Category strings the synthetic catalog pins, spelled once for goconst.
const (
	categoryAccount       = "account"
	categoryBlockStorage  = "block_storage"
	categoryCompute       = "compute"
	categoryComputeDeep   = "compute_deep"
	categoryCore          = "core"
	categoryDNS           = "dns"
	categoryDatabases     = "databases"
	categoryIAM           = "iam"
	categoryLKE           = "lke"
	categoryLongview      = "longview"
	categoryMonitor       = "monitor"
	categoryNetworking    = "networking"
	categoryObjectStorage = "object_storage"
	categorySecurity      = "security"
	categoryVPCs          = "vpcs"
)
