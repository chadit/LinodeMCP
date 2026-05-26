package audit

import (
	"sort"
)

// RedactedValue is the placeholder string written into the audit
// event in place of a sensitive arg value. Exposed so tests can
// match it without re-defining the constant.
const RedactedValue = "[REDACTED]"

// RedactionFields is the canonical list of arg names whose values
// get replaced with RedactedValue before the audit event is written.
// Match semantics are EXACT field name (per the spec): no substring,
// no suffix, no case-fold. A variant like `cluster_root_pass` needs
// its own entry in this list.
//
// The cross-language parity test in event_test.go asserts that the
// Python equivalent at `python/src/linodemcp/audit/redact.py` carries
// the same set of names.
func RedactionFields() []string {
	// Returned by function rather than a package-level slice so the
	// global-state linter has nothing to flag. The list is small and
	// the audit middleware (Phase 1b) caches the lookup set so the
	// allocation per call doesn't matter.
	return []string{
		"api_key",
		"apiKey",
		"authorized_keys",
		"data",
		"kubeconfig",
		"pass",
		"password",
		"password_created",
		"private_key",
		"root_pass",
		"secret",
		"service_token",
		"ssh_key",
		"ssh_keys",
		"token",
		"token_uuid",
	}
}

// RedactionFieldSet returns the redaction list as a set for O(1)
// lookups inside the recursive walker. Equivalent to
// slices.Index(RedactionFields(), key) >= 0 but cheaper when called
// per-key over a deep args tree.
func RedactionFieldSet() map[string]struct{} {
	return setFromSlice(RedactionFields())
}

// RedactionFieldsPII is the conservative PII arg list that gets
// scrubbed in addition to RedactionFields when the audit.redact_pii
// config flag is true (Phase 4c, default true). Match semantics are
// the same as the credential list: exact field name, no substring.
//
// Each name was source-verified against the live tool schemas: every
// occurrence in current tools is unambiguously postal address / PII,
// never a non-sensitive filter or selector. Cross-language parity is
// asserted by the unit test that mirrors this list against the Python
// equivalent at `python/src/linodemcp/audit/redact.py`.
//
// Names deliberately left out so login identifiers stay readable in
// audit reports: email, first_name, last_name, company. Contact-specific
// name/email tool args use contact_name/contact_email and are redacted. Names dropped
// after source review because they collide with non-PII tool args:
// country (linode_regions_list filter), address (network/IP address
// in linode_instance_ips, linode_networking, linode_nodebalancers).
func RedactionFieldsPII() []string {
	return []string{
		"address_1",
		"address_2",
		"city",
		"contact_email",
		"contact_name",
		"phone",
		"phone_number",
		"phone_primary",
		"phone_secondary",
		"state",
		"tax_id",
		"zip",
	}
}

// RedactionFieldSetPII returns the PII redaction list as a set.
func RedactionFieldSetPII() map[string]struct{} {
	return setFromSlice(RedactionFieldsPII())
}

// Redact walks the args map and replaces sensitive (credential)
// values with RedactedValue. Returns the redacted copy and a sorted
// list of every key that was redacted (deduped across the recursive
// walk). Credentials are always redacted; this entry point does NOT
// touch PII fields. Use RedactWithPII for the combined set.
//
// The original args map is NOT mutated; callers that need the
// unredacted values can keep their own copy.
//
// The walk recurses into nested maps but does NOT recurse into
// slices of maps. That's a deliberate simplification: every
// sensitive arg in the current tool surface lives at the top level
// or inside a nested object literal, never inside an array element.
// If a future tool needs array-element redaction, the walker grows
// to handle it then.
func Redact(args map[string]any) (map[string]any, []string) {
	return redactWithFields(args, RedactionFieldSet())
}

// RedactWithPII walks the args map and replaces BOTH credential and
// PII values with RedactedValue. Used by the audit middleware when
// audit.redact_pii is true (Phase 4c default). When the operator
// opts out via audit.redact_pii: false, the middleware uses Redact
// instead so PII passes through in cleartext while credentials stay
// scrubbed.
func RedactWithPII(args map[string]any) (map[string]any, []string) {
	return redactWithFields(args, combinedRedactionFieldSet())
}

// combinedRedactionFieldSet returns the union of credential and PII
// redaction names. The disjoint-sets test in redact_test.go guards
// against an entry sneaking into both lists, so a plain merge here
// is safe.
func combinedRedactionFieldSet() map[string]struct{} {
	creds := RedactionFields()
	pii := RedactionFieldsPII()
	out := make(map[string]struct{}, len(creds)+len(pii))

	for _, name := range creds {
		out[name] = struct{}{}
	}

	for _, name := range pii {
		out[name] = struct{}{}
	}

	return out
}

// setFromSlice builds a presence-set from a name slice. The audit
// package builds these sets on every Redact call rather than caching;
// the lists are short enough that the allocation cost stays under the
// audit-overhead budget documented in the spec's Performance section.
func setFromSlice(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))

	for _, name := range names {
		out[name] = struct{}{}
	}

	return out
}

// redactWithFields is the shared walker entry point used by both
// Redact and RedactWithPII. Extracted so the credential-only and
// credential+PII paths share the same recursive copy logic.
func redactWithFields(
	args map[string]any,
	fields map[string]struct{},
) (map[string]any, []string) {
	redactedKeys := make(map[string]struct{})
	out := redactMap(args, fields, redactedKeys)
	keys := make([]string, 0, len(redactedKeys))

	for name := range redactedKeys {
		keys = append(keys, name)
	}

	sort.Strings(keys)

	return out, keys
}

// redactMap is the recursive worker. Walks the map, copying values
// into a new map; sensitive values are replaced.
func redactMap(
	source map[string]any,
	fields map[string]struct{},
	redactedKeys map[string]struct{},
) map[string]any {
	if source == nil {
		return nil
	}

	out := make(map[string]any, len(source))

	for key, value := range source {
		if _, sensitive := fields[key]; sensitive {
			out[key] = RedactedValue
			redactedKeys[key] = struct{}{}

			continue
		}

		nested, ok := value.(map[string]any)
		if !ok {
			out[key] = value

			continue
		}

		out[key] = redactMap(nested, fields, redactedKeys)
	}

	return out
}

// IsRedacted reports whether a value position holds the redaction
// placeholder. Used by tests to assert that a specific arg key was
// scrubbed without re-implementing the equality check.
func IsRedacted(value any) bool {
	asString, ok := value.(string)
	if !ok {
		return false
	}

	return asString == RedactedValue
}
