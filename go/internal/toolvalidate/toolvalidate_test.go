package toolvalidate_test

import (
	"strings"
	"testing"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/chadit/LinodeMCP/go/internal/toolvalidate"
)

const (
	domainCreateInput = "linode.mcp.v1.DomainCreateInput"
	domainUpdateInput = "linode.mcp.v1.DomainUpdateInput"
	exampleDomain     = "example.com"
	exampleSOA        = "admin@example.com"
	zoneMaster        = "master"
	zoneSlave         = "slave"

	keyDomain    = "domain"
	keyType      = "type"
	keySOAEmail  = "soa_email"
	keyStatus    = "status"
	keyMasterIPs = "master_ips"
	keyRetrySec  = "retry_sec"
	keyDomainID  = "domain_id"

	errPattern   = "domain must match the documented domain-name pattern"
	errSOAMaster = "soa_email is required for master domains"
	errDomainID  = "domain_id must be a positive integer"

	// protoPackage scopes the descriptor walk to this repo's contract; the
	// global registry also holds descriptor.proto and the validation options.
	protoPackage = "linode.mcp.v1"
)

// The rules are the tool's whole answer to a bad call, so every case here is
// one a client can make and the sentence it gets back. The cases that expect ""
// are the other half of the contract: the rules hold their peace where another
// part of the handler owns the answer, and a rule that spoke there would take
// the sentence a caller reads today away from them.
func TestCheckAnswersTheDeclaredSentence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		message   string
		arguments map[string]any
		want      string
	}{
		{
			name:      "absent domain",
			message:   domainCreateInput,
			arguments: map[string]any{keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      "domain is required",
		},
		{
			name:      "domain longer than the documented cap",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: strings.Repeat("a", 254), keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      "domain must be between 1 and 253 characters",
		},
		{
			name:      "domain with a space",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: "bad domain", keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      errPattern,
		},
		{
			name:      "domain without a tld",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: "localhost", keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      errPattern,
		},
		{
			name:      "domain label over 63 characters",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: strings.Repeat("a", 64) + ".com", keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      errPattern,
		},
		{
			name:      "numeric tld",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: "example.123", keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      errPattern,
		},
		{
			name:      "documented wildcard domain",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: "*.example.com", keyType: zoneMaster, keySOAEmail: exampleSOA},
			want:      "",
		},
		{
			name:      "master without an soa email",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster},
			want:      errSOAMaster,
		},
		{
			name:      "master with an emptied soa email",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: ""},
			want:      errSOAMaster,
		},
		{
			name:      "slave with an emptied master_ips",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneSlave, keyMasterIPs: []any{}},
			want:      "master_ips must include at least one value for slave domains",
		},
		{
			name:      "slave with a master ip",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneSlave, keyMasterIPs: []any{"192.0.2.20"}},
			want:      "",
		},
		{
			name:      "status naming no member",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyStatus: "edit_mode"},
			want:      "status must be one of: active, disabled",
		},
		{
			name:      "documented status",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyStatus: "active"},
			want:      "",
		},
		{
			name:      "absent type leaves the type rules to the hook",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain},
			want:      "",
		},
		{
			name:      "type naming no zone type leaves every later rule to the hook",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: "primary"},
			want:      "",
		},
		{
			name:      "type naming no zone type outranks a bad status",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: "primary", keyStatus: "edit_mode"},
			want:      "",
		},
		{
			name:      "status of the wrong type is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyStatus: float64(1)},
			want:      "",
		},
		{
			// A rule reads the message, and a value no JSON encoder can write
			// never reaches one, so it is left to whatever else reads it.
			name:      "argument no JSON encoder can write is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: make(chan int)},
			want:      "",
		},
		{
			// An enum takes a member name or its number, and a boolean is
			// neither, so the body reports the type rather than a rule
			// reporting an unknown member.
			name:      "enum given a boolean is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyStatus: true},
			want:      "",
		},
		{
			// An enum takes a member name or its number, and a list is neither.
			name:      "enum given a list is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyStatus: []any{}},
			want:      "",
		},
		{
			name:      "soa email of the wrong type is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: float64(1)},
			want:      "",
		},
		{
			name:      "json encoded master_ips is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneSlave, keyMasterIPs: `["192.0.2.20"]`},
			want:      "",
		},
		{
			name:      "json encoded tags is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, "tags": `[" prod "]`},
			want:      "",
		},
		{
			name:      "fractional integer is left to the body",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyRetrySec: 1.5},
			want:      "",
		},
		{
			name:      "integral json number reads as the integer it is",
			message:   domainCreateInput,
			arguments: map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, keySOAEmail: exampleSOA, keyRetrySec: 1.0},
			want:      "",
		},
		{
			name:    "every optional supplied with its zero",
			message: domainCreateInput,
			arguments: map[string]any{
				keyDomain: exampleDomain, keyType: zoneSlave, "axfr_ips": []any{}, "description": "",
				"expire_sec": 604800.0, "group": "", keyMasterIPs: []any{"192.0.2.20"},
				"refresh_sec": 14400.0, keyRetrySec: 0.0, keySOAEmail: "", keyStatus: "active",
				"tags": []any{}, "ttl_sec": 300.0,
			},
			want: "",
		},
		{
			name:      "absent domain id",
			message:   domainUpdateInput,
			arguments: map[string]any{"confirm": true},
			want:      errDomainID,
		},
		{
			name:      "zero domain id",
			message:   domainUpdateInput,
			arguments: map[string]any{keyDomainID: 0.0},
			want:      errDomainID,
		},
		{
			name:      "domain id that is not a number reads as absent",
			message:   domainUpdateInput,
			arguments: map[string]any{keyDomainID: "abc"},
			want:      errDomainID,
		},
		{
			name:      "negative domain id",
			message:   domainUpdateInput,
			arguments: map[string]any{keyDomainID: -5.0},
			want:      errDomainID,
		},
		{
			name:      "supplied domain id",
			message:   domainUpdateInput,
			arguments: map[string]any{keyDomainID: 5.0},
			want:      "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := toolvalidate.Check(testCase.message, testCase.arguments); got != testCase.want {
				t.Errorf("Check(%s) = %q, want %q", testCase.message, got, testCase.want)
			}
		})
	}
}

// An integer argument arrives as a string often enough that both languages read
// one, and each JSON reader used to be lenient in its own direction: Go's took
// the 123 out of "123/456" and let the rule pass on an id the path read then
// refused in other words, while Python's read spellings of its own such as
// "1_000". Only a string spelling one whole number is read now, so every case
// below is the same sentence in both languages.
func TestCheckReadsOnlyAnIntegerSpelledWhole(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		id   string
		want string
	}{
		"id carrying a path segment":      {id: "123/456", want: errDomainID},
		"id carrying a query":             {id: "1?foo=bar", want: errDomainID},
		"id carrying a traversal":         {id: "../7", want: errDomainID},
		"id grouped with underscores":     {id: "1_000", want: errDomainID},
		"id with a leading plus":          {id: "+123", want: errDomainID},
		"id with leading zeros":           {id: "007", want: errDomainID},
		"id in digits outside ascii":      {id: "١٢٣", want: errDomainID},
		"id with surrounding space":       {id: " 123 ", want: errDomainID},
		"id spelled whole":                {id: "5", want: ""},
		"id spelled with an exponent":     {id: "1e1", want: ""},
		"id spelled with a zero fraction": {id: "5.0", want: ""},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := toolvalidate.Check(domainUpdateInput, map[string]any{keyDomainID: testCase.id})
			if got != testCase.want {
				t.Errorf("Check(domain_id=%q) = %q, want %q", testCase.id, got, testCase.want)
			}
		})
	}
}

// A string naming no member of an enum is coerced at the top level of a call
// only, so one inside a nested message would leave the whole call to the body
// builder and quietly take every other rule with it. No field in the contract
// can carry a nested enum today, and this names the one that would rather than
// letting the gap reopen unannounced.
func TestNoConstrainedMessageCarriesANestedEnum(t *testing.T) {
	t.Parallel()

	var inspected int

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			inspected += assertNoNestedEnum(t, messages.Get(i))
		}

		return true
	})

	if inspected == 0 {
		t.Fatal("no constrained message declares a message-typed field, so this asserted nothing")
	}
}

// assertNoNestedEnum reports how many message-typed fields one constrained
// message declares, failing for any that reaches an enum.
func assertNoNestedEnum(t *testing.T, message protoreflect.MessageDescriptor) int {
	t.Helper()

	if !proto.HasExtension(message.Options(), validate.E_Message) {
		return 0
	}

	var inspected int

	fields := message.Fields()

	for i := range fields.Len() {
		field := fields.Get(i)
		if field.Kind() != protoreflect.MessageKind {
			continue
		}

		inspected++

		if reached := reachableEnum(field.Message(), map[protoreflect.FullName]bool{}); reached != "" {
			t.Errorf("%s.%s reaches enum %s, which no rule can answer for", message.FullName(), field.Name(), reached)
		}
	}

	return inspected
}

// reachableEnum names the first enum a message reaches, or "" when it reaches
// none. Well-known types are skipped because their JSON reading is their own
// and takes no member name from the contract.
func reachableEnum(message protoreflect.MessageDescriptor, seen map[protoreflect.FullName]bool) protoreflect.FullName {
	if strings.HasPrefix(string(message.FullName()), "google.protobuf.") || seen[message.FullName()] {
		return ""
	}

	seen[message.FullName()] = true

	fields := message.Fields()

	for i := range fields.Len() {
		field := fields.Get(i)
		if field.Kind() == protoreflect.EnumKind {
			return field.Enum().FullName()
		}

		if field.Kind() != protoreflect.MessageKind {
			continue
		}

		if reached := reachableEnum(field.Message(), seen); reached != "" {
			return reached
		}
	}

	return ""
}

// A message declaring no rules is the whole rest of the surface, and the seam
// runs on every generated handler, so answering nothing there is what keeps the
// seam from changing any tool it was not pointed at.
func TestCheckIsSilentWhereNothingIsDeclared(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
	}{
		{name: "message with no rules", message: "linode.mcp.v1.DomainGetInput"},
		{name: "name the contract does not define", message: "linode.mcp.v1.NoSuchInput"},
		{name: "name that is not a message", message: "linode.mcp.v1.FieldLocation"},
		{name: "empty name", message: ""},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := toolvalidate.Check(testCase.message, map[string]any{keyDomain: ""}); got != "" {
				t.Errorf("Check(%q) = %q, want empty", testCase.message, got)
			}
		})
	}
}

// An argument the message does not declare belongs to whatever else reads it,
// so it neither feeds a rule nor stops one from running.
func TestCheckIgnoresUndeclaredArguments(t *testing.T) {
	t.Parallel()

	arguments := map[string]any{keyDomain: exampleDomain, keyType: zoneMaster, "nonsense": []any{1, 2}}

	if got := toolvalidate.Check(domainCreateInput, arguments); got != errSOAMaster {
		t.Errorf("Check = %q, want the soa_email sentence", got)
	}
}

// Every rule carries the sentence it answers with, which is what lets the
// package hand a violation's message straight to a caller. A rule declaring
// none would answer with protovalidate's own wording, putting words in front of
// a caller that no tool chose.
//
// Two rule shapes carry it in two places. A rule whose expression answers a
// bool declares the sentence as its message. A rule whose expression answers a
// string builds the sentence itself, so it can name the value it read, and
// protovalidate prints that result rather than the message: the message would
// be dead text, and buf lint reports one. Each shape is required to carry the
// sentence in its own place and nowhere else.
func TestEveryDeclaredRuleNamesItsSentence(t *testing.T) {
	t.Parallel()

	var checked int

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			checked += assertRulesNameTheirSentence(t, messages.Get(i))
		}

		return true
	})

	if checked == 0 {
		t.Fatal("no message declares a rule, so this asserted nothing")
	}
}

// assertRulesNameTheirSentence reports how many rules one message declares,
// failing for any that names no sentence, names it twice, or names no id.
func assertRulesNameTheirSentence(t *testing.T, message protoreflect.MessageDescriptor) int {
	t.Helper()

	rules, isRules := proto.GetExtension(message.Options(), validate.E_Message).(*validate.MessageRules)
	if !isRules || rules == nil {
		return 0
	}

	for _, rule := range rules.GetCel() {
		assertRuleNamesItsSentence(t, message.FullName(), rule)

		if rule.GetId() == "" {
			t.Errorf("%s declares a rule with no id", message.FullName())
		}
	}

	return len(rules.GetCel())
}

// assertRuleNamesItsSentence fails when one rule's sentence is missing from the
// place its expression shape puts it, or is declared in both places at once.
func assertRuleNamesItsSentence(t *testing.T, owner protoreflect.FullName, rule *validate.Rule) {
	t.Helper()

	if answersString(rule.GetExpression()) {
		if rule.GetMessage() != "" {
			t.Errorf("%s rule %q answers a string and also declares a message, which is never read",
				owner, rule.GetId())
		}

		return
	}

	if rule.GetMessage() == "" {
		t.Errorf("%s rule %q names no sentence", owner, rule.GetId())
	}
}

// answersString reports whether one expression answers a sentence rather than a
// bool. protovalidate reads the empty string as "this value is fine", so a rule
// that builds its own sentence returns ” on the passing arm of a conditional,
// which is the shape read here.
func answersString(expression string) bool {
	return strings.Contains(strings.ReplaceAll(expression, " ", ""), "?'':")
}
