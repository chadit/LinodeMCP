package audit_test

// Shared string constants for the audit package's external test
// files. Extracted to satisfy goconst, which counts repeated
// literals across the package.
const (
	tcProfile = "profile"
	// argLinodeID is the conventional Linode-ID arg key. Reused in
	// event-construction fixtures across multiple test files.
	argLinodeID = "linode_id"
	// argRootPass is the canonical sensitive arg key used in the
	// redaction tests. Distinct from the redaction-list constant
	// (which is the runtime value) so a test reorder doesn't
	// accidentally tie them together.
	argRootPass = "root_pass"
	// argKeyLabel and argKeyToken are arg keys reused across the
	// redaction and SQLite test fixtures. Extracted to satisfy
	// goconst once they recur 3+ times across the package.
	argKeyLabel = "label"
	argKeyToken = "token"
	// colTool and colStatus are summary group-by column names reused
	// across the summary tests. Extracted to satisfy goconst.
	colTool   = "tool"
	colStatus = "status"
	// keyRegion and valUSEast are a region arg key/value pair reused
	// across the export, redaction, and SQLite fixtures. Extracted to
	// satisfy goconst.
	keyRegion = "region"
	valUSEast = "us-east"
	// PII arg names reused across the PII-redaction tests in
	// redact_test.go. Same goconst pressure as the credential-side
	// constants above.
	argTaxID    = "tax_id"
	argPhone    = "phone"
	argAddress1 = "address_1"
	argCity     = "city"
	// valFakeToken is a non-secret stand-in token value used across
	// multiple redaction tests. Extracted because the literal now
	// appears 3+ times.
	valFakeToken = "abc123"
)

// Repeated literals extracted to satisfy goconst.
const (
	tcBoom                 = "boom"
	tcCapability           = "capability"
	tcFannedOut            = "fanned_out"
	tcLinodeInstanceCreate = "linode_instance_create"
	tcLinodeInstanceDelete = "linode_instance_delete"
	tcLinodeInstanceList   = "linode_instance_list"
	tcPasswordCreated      = "password_created"
	tcSSHKeys              = "ssh_keys"
	tcToolA                = "tool_a"
)

// Repeated literals extracted to satisfy goconst.
const (
	tcEnvironment = "environment"
)
