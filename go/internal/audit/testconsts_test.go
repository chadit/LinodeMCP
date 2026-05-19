package audit_test

// Shared string constants for the audit package's external test
// files. Extracted to satisfy goconst, which counts repeated
// literals across the package.
const (
	// argLinodeID is the conventional Linode-ID arg key. Reused in
	// event-construction fixtures across multiple test files.
	argLinodeID = "linode_id"
	// argRootPass is the canonical sensitive arg key used in the
	// redaction tests. Distinct from the redaction-list constant
	// (which is the runtime value) so a test reorder doesn't
	// accidentally tie them together.
	argRootPass = "root_pass"
)
