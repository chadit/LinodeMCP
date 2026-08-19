package tools_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// errTagsArrayOfStrings is the message a malformed tags array answers with,
// named here because the assertion repeats across the cases below.
const errTagsArrayOfStrings = "tags must be an array of strings"

// errLinodesIntArray is the message a malformed id array answers with, named
// here because every rejection case below asserts the same one.
const errLinodesIntArray = "linodes must be an array of integers"

// The two list cases every repeated setter is held to, named because each of
// the three setters below runs them.
const (
	caseSuppliedEmpty = "supplied empty"
	caseOmitted       = "omitted"
)

// bodyRequest wraps one call's arguments the way the MCP layer hands them to a
// handler.
func bodyRequest(arguments map[string]any) *mcp.CallToolRequest {
	return &mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: arguments}}
}

// marshalBody renders a body the way the request layer will.
func marshalBody(t *testing.T, body *tools.WriteBody) string {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	return string(raw)
}

// TestWriteBodyKeepsDeclarationOrder is the reason this type exists: a map
// marshals sorted in Go and in insertion order in Python, so the same call
// would put different bytes on the wire in each language. The rule reaches
// inside a typed list item too, which is its own map in both languages.
func TestWriteBodyKeepsDeclarationOrder(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		"domain":    "example.com",
		"type":      "master",
		"soa_email": "admin@example.com",
		keyTTLSec:   300,
		keyImages: []any{map[string]any{
			keyDescription: "third",
			keyLabel:       "second",
			keyID:          imagePrivate15Fixture,
		}},
	}), 5)

	body.PutString("domain")
	body.PutString("type")
	body.SetString("soa_email")
	body.SetInt(keyTTLSec)
	body.SetMessageList(keyImages, imageItems())

	want := `{"domain":"example.com","type":"master","soa_email":"admin@example.com",` +
		`"ttl_sec":300,"images":[{"id":"private/15","label":"second","description":"third"}]}`
	if got := marshalBody(t, body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyCopiesByPresence: "" and 0 and [] are values the Linode API acts
// on, so a caller who supplies one must have it reach the wire. Dropping them
// would silently turn "clear this field" into "leave it alone".
func TestWriteBodyCopiesByPresence(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyDescription: "",
		keyTTLSec:      0,
		keyTags:        []any{},
	}), 3)

	body.SetString(keyDescription)
	body.SetInt(keyTTLSec)
	body.SetStringList(keyTags)

	want := `{"description":"","ttl_sec":0,"tags":[]}`
	if got := marshalBody(t, body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyOmitsWhatTheCallerDidNotSupply pins the other half of presence:
// an absent optional field is not a request to set it to its zero.
func TestWriteBodyOmitsWhatTheCallerDidNotSupply(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyDescription: "set"}), 3)

	body.SetString(keyDescription)
	body.SetString("soa_email")
	body.SetInt(keyTTLSec)
	body.SetStringList(keyTags)

	if got, want := marshalBody(t, body), `{"description":"set"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyPutsARequiredFieldTheCallerOmitted: a proto field without
// `optional` travels on every call, so its absence has to read as the zero
// value rather than as an omitted key.
func TestWriteBodyPutsARequiredFieldTheCallerOmitted(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 2)

	body.PutString("domain")
	body.PutInt("count")

	if got, want := marshalBody(t, body), `{"domain":"","count":0}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyAcceptsAnIntegralFloat: a JSON-RPC number arrives as a float, so
// 1.0 has to reach the wire as 1 rather than as 1.0, which the API rejects.
func TestWriteBodyAcceptsAnIntegralFloat(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyRetrySec: 1.0}), 1)

	body.SetInt(keyRetrySec)

	if message := body.Message(); message != "" {
		t.Fatalf("Message = %q, want none", message)
	}

	if got, want := marshalBody(t, body), `{"retry_sec":1}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyReportsTheFirstTypeFailure pins both halves of the rule: the
// message is the first thing wrong with the call, and nothing after it is read.
// A later field overwriting the message would report whichever failure happens
// to be declared last.
func TestWriteBodyReportsTheFirstTypeFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyDescription: "kept",
		keyRetrySec:    1.5,
		keyTags:        "not-an-array",
	}), 3)

	body.SetString(keyDescription)
	body.SetInt(keyRetrySec)
	body.SetStringList(keyTags)

	if got, want := body.Message(), "retry_sec must be an integer"; got != want {
		t.Errorf("Message = %q, want %q", got, want)
	}
}

// TestWriteBodyReportsTheTypeItDeclares covers the messages each kind answers
// with, which the hand-written builders these replace already word this way.
func TestWriteBodyReportsTheTypeItDeclares(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		set   func(*tools.WriteBody, string)
		want  string
	}{
		{
			name:  keyStatus,
			value: 1,
			set:   func(b *tools.WriteBody, name string) { b.SetString(name) },
			want:  "status must be a string",
		},
		{
			name:  keyTTLSec,
			value: "300",
			set:   func(b *tools.WriteBody, name string) { b.SetInt(name) },
			want:  "ttl_sec must be an integer",
		},
		{
			name:  "master_ips",
			value: `["192.0.2.20"]`,
			set:   func(b *tools.WriteBody, name string) { b.SetStringList(name) },
			want:  errMasterIPsArray,
		},
		{
			name:  keyTags,
			value: []any{"prod", 5},
			set:   func(b *tools.WriteBody, name string) { b.SetStringList(name) },
			want:  errTagsArrayOfStrings,
		},
		{
			name:  "restricted",
			value: boolStringTrue,
			set:   func(b *tools.WriteBody, name string) { b.SetBool(name) },
			want:  "restricted must be a boolean",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(map[string]any{testCase.name: testCase.value}), 1)
			testCase.set(body, testCase.name)

			if got := body.Message(); got != testCase.want {
				t.Errorf("Message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestWriteBodyCopiesABooleanByPresence: false is the value half of every
// on/off setting on the surface, so an absent flag and one the caller turned off
// have to reach the wire as different bodies.
func TestWriteBodyCopiesABooleanByPresence(t *testing.T) {
	t.Parallel()

	supplied := tools.NewWriteBody(bodyRequest(map[string]any{keyStackScriptIsPublic: false}), 1)
	supplied.SetBool(keyStackScriptIsPublic)

	if got, want := marshalBody(t, supplied), `{"is_public":false}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}

	absent := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)
	absent.SetBool(keyStackScriptIsPublic)

	if got, want := marshalBody(t, absent), `{}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyPutsABooleanTheCallerOmitted: a bool without `optional` travels
// on every call, so an omitted one reads as false.
func TestWriteBodyPutsABooleanTheCallerOmitted(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyInterfacePublic: true}), 2)

	body.PutBool(keyInterfacePublic)
	body.PutBool("restricted")

	if got, want := marshalBody(t, body), `{"public":true,"restricted":false}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyRefusesAWronglyTypedFieldThatAlwaysTravels: no rule can speak
// for a field whose value the message cannot hold, so the body builder is the
// only place left to report it.
func TestWriteBodyRefusesAWronglyTypedFieldThatAlwaysTravels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		write func(*tools.WriteBody)
		args  map[string]any
		want  string
	}{
		{
			name:  "boolean",
			write: func(body *tools.WriteBody) { body.PutBool(keyInterfacePublic) },
			args:  map[string]any{keyInterfacePublic: phase2NonBoolean},
			want:  keyInterfacePublic + " must be a boolean",
		},
		{
			name:  caseString,
			write: func(body *tools.WriteBody) { body.PutString(managedServiceLabelParam) },
			args:  map[string]any{managedServiceLabelParam: 7},
			want:  errLabelString,
		},
		{
			name:  schemaTypeInteger,
			write: func(body *tools.WriteBody) { body.PutInt(keySize) },
			args:  map[string]any{keySize: "big"},
			want:  keySize + " must be an integer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(test.args), 1)
			test.write(body)

			if got := body.Message(); got != test.want {
				t.Errorf("message = %q, want %q", got, test.want)
			}
		})
	}
}

// TestWriteBodyKeepsTheFirstFailureAcrossFieldsThatAlwaysTravel: the message a
// caller reads should be the first thing wrong with their call, so a later
// field neither overwrites it nor writes itself.
func TestWriteBodyKeepsTheFirstFailureAcrossFieldsThatAlwaysTravel(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		managedServiceLabelParam: 7,
		keySize:                  "big",
	}), 2)

	body.PutString(managedServiceLabelParam)
	body.PutInt(keySize)

	if got := body.Message(); got != errLabelString {
		t.Errorf("message = %q, want %q", got, errLabelString)
	}
}

// TestWriteBodyReadsAnIntegerList covers the shapes an id list arrives as and
// the entries it refuses. A null entry is refused where a null scalar reads as
// zero: an id nobody sent is not one of the ids.
func TestWriteBodyReadsAnIntegerList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
		body  string
		want  string
	}{
		{name: "decoded", value: []any{1.0, 2.0}, body: `{"linodes":[1,2]}`},
		{name: "native", value: []int{3}, body: `{"linodes":[3]}`},
		{name: "empty", value: []any{}, body: `{"linodes":[]}`},
		{name: "not a list", value: "1,2", want: errLinodesIntArray},
		{name: "text entry", value: []any{"1"}, want: errLinodesIntArray},
		{name: "fractional entry", value: []any{1.5}, want: errLinodesIntArray},
		{name: "null entry", value: []any{nil}, want: errLinodesIntArray},
		{name: "boolean entry", value: []any{true}, want: errLinodesIntArray},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(map[string]any{keyPlacementGroupLinodes: testCase.value}), 1)
			body.SetIntList(keyPlacementGroupLinodes)

			if got := body.Message(); got != testCase.want {
				t.Fatalf("Message = %q, want %q", got, testCase.want)
			}

			if testCase.want != "" {
				return
			}

			if got := marshalBody(t, body); got != testCase.body {
				t.Errorf("body = %s, want %s", got, testCase.body)
			}
		})
	}
}

// TestWriteBodyMarshalsEmptyAsAnObject: a mutation with nothing set still sends
// a body, which is what the routed call and its Python twin both put on the
// wire today.
func TestWriteBodyMarshalsEmptyAsAnObject(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 0)

	if got, want := marshalBody(t, body), `{}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyAcceptsANativeStringSlice: an argument decoded from JSON is
// always []any, but a caller assembling one in Go hands over []string, and both
// have to reach the same body.
func TestWriteBodyAcceptsANativeStringSlice(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{"master_ips": []string{"192.0.2.20"}}), 1)

	body.SetStringList("master_ips")

	if got, want := marshalBody(t, body), `{"master_ips":["192.0.2.20"]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestMarshalOrderedJSONReportsAValueItCannotWrite: the writer takes any value,
// so a caller outside the builder can hand it one JSON has no form for. Losing
// that quietly would put a body on the wire missing a field the caller set.
func TestMarshalOrderedJSONReportsAValueItCannotWrite(t *testing.T) {
	t.Parallel()

	_, err := tools.MarshalOrderedJSON([]string{"stream"}, []any{make(chan int)})

	if _, refused := errors.AsType[*json.UnsupportedTypeError](err); !refused {
		t.Errorf("error = %v, want a value JSON has no form for to be refused", err)
	}
}

// cardNumberMember is the free-form member the redaction cases withhold.
const cardNumberMember = "card_number"

// TestWriteBodySetObjectWritesAFreeFormObject: presence decides the member, so
// an omitted argument leaves the key off the wire while an explicit {} travels.
func TestWriteBodySetObjectWritesAFreeFormObject(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{
			name:      caseSupplied,
			arguments: map[string]any{keyPaymentData: map[string]any{cardNumberMember: "4111", "cvv": "737"}},
			want:      `{"data":{"card_number":"4111","cvv":"737"}}`,
		},
		{
			name:      caseSuppliedEmpty,
			arguments: map[string]any{keyPaymentData: map[string]any{}},
			want:      `{"data":{}}`,
		},
		{name: caseOmitted, arguments: map[string]any{}, want: jsonObjectEmpty},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)

			body.SetObject(keyPaymentData)

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetObjectRefusesANonObject names the field, since the caller
// sees the sentence without seeing which argument the tool was reading.
func TestWriteBodySetObjectRefusesANonObject(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{"credit card", float64(5), []any{"a"}, true, nil} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyPaymentData: raw}), 1)

		body.SetObject(keyPaymentData)

		if got, want := body.Message(), "data must be an object"; got != want {
			t.Errorf("message for %#v = %q, want %q", raw, got, want)
		}
	}
}

// TestWriteBodySetObjectRootHoistsTheMembers: the argument's own keys are the
// body, sorted, with nothing named after the field the tool advertises.
func TestWriteBodySetObjectRootHoistsTheMembers(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyPaymentData: map[string]any{"theme": profilePreferenceValueDark, "collapsed": true},
	}), 1)

	body.SetObjectRoot(keyPaymentData)

	if got, want := marshalBody(t, body), `{"collapsed":true,"theme":"dark"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetObjectRootRefusesAnEmptyOrWrongArgument: the members are the
// body, so an empty object is an empty request rather than a field left off.
func TestWriteBodySetObjectRootRefusesAnEmptyOrWrongArgument(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{map[string]any{}, "dark", float64(5), []any{"a"}, true, nil} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyPaymentData: raw}), 1)

		body.SetObjectRoot(keyPaymentData)

		if got, want := body.Message(), "data must be a non-empty object"; got != want {
			t.Errorf("message for %#v = %q, want %q", raw, got, want)
		}
	}

	omitted := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)

	omitted.SetObjectRoot(keyPaymentData)

	if got, want := omitted.Message(), "data must be a non-empty object"; got != want {
		t.Errorf("message for an omitted argument = %q, want %q", got, want)
	}
}

// TestWriteBodySetObjectRootKeepsTheFirstFailure: the reported message is the
// first thing wrong with the call, so a hoist after one is skipped rather than
// overwriting it.
func TestWriteBodySetObjectRootKeepsTheFirstFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyLabel:       7,
		keyPaymentData: map[string]any{"theme": profilePreferenceValueDark},
	}), 2)

	body.SetString(keyLabel)
	body.SetObjectRoot(keyPaymentData)

	if got, want := body.Message(), keyLabel+" must be a string"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}

	if got := marshalBody(t, body); got != jsonObjectEmpty {
		t.Errorf("body = %s, want nothing written after a failure", got)
	}
}

// keyGrantSection is the repeated free-form field the object-list cases read,
// named here because every one of them writes the same section.
const keyGrantSection = "linode"

// errGrantsObjectArray is the sentence a malformed grant section answers with.
const errGrantsObjectArray = "linode must be an array of objects"

// TestWriteBodySetObjectListWritesEachEntry: a list of free-form objects keeps
// its order and every member, and an empty list clears the section rather than
// leaving it alone.
func TestWriteBodySetObjectListWritesEachEntry(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{
			name: caseSupplied,
			arguments: map[string]any{keyGrantSection: []any{
				map[string]any{keySupportTicketID: float64(1), "permissions": grantPermissionReadOnly},
				map[string]any{keySupportTicketID: float64(2), "permissions": grantPermissionReadWrite},
			}},
			want: `{"linode":[{"id":1,"permissions":"read_only"},{"id":2,"permissions":"read_write"}]}`,
		},
		{
			name:      caseSuppliedEmpty,
			arguments: map[string]any{keyGrantSection: []any{}},
			want:      `{"linode":[]}`,
		},
		{
			name:      "native",
			arguments: map[string]any{keyGrantSection: []map[string]any{{keySupportTicketID: float64(3)}}},
			want:      `{"linode":[{"id":3}]}`,
		},
		{name: caseOmitted, arguments: map[string]any{}, want: jsonObjectEmpty},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)

			body.SetObjectList(keyGrantSection)

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetObjectListRefusesAnEntryThatIsNotAnObject: one unreadable
// entry refuses the whole section, since a grant nobody could read is not one
// the API should be told to apply.
func TestWriteBodySetObjectListRefusesAnEntryThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{
		grantPermissionReadOnly,
		float64(5),
		true,
		nil,
		map[string]any{keySupportTicketID: float64(1)},
		[]any{map[string]any{keySupportTicketID: float64(1)}, grantPermissionReadOnly},
		[]any{nil},
	} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyGrantSection: raw}), 1)

		body.SetObjectList(keyGrantSection)

		if got := body.Message(); got != errGrantsObjectArray {
			t.Errorf("message for %#v = %q, want %q", raw, got, errGrantsObjectArray)
		}
	}
}

// TestWriteBodyRenameWritesTheDeclaredWireKey: the argument keeps the name the
// tool advertises and the body carries the one the API reads.
func TestWriteBodyRenameWritesTheDeclaredWireKey(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{"new_username": "chad", "email": "a@b.c"}), 2)

	body.Rename("new_username", "username")
	body.SetString("new_username")
	body.SetString("email")

	if got, want := marshalBody(t, body), `{"username":"chad","email":"a@b.c"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyRenameLeavesAnOmittedArgumentOff: a rename says what a supplied
// argument is called on the wire, not that it has to travel.
func TestWriteBodyRenameLeavesAnOmittedArgumentOff(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)

	body.Rename("new_username", "username")
	body.SetString("new_username")

	if got, want := marshalBody(t, body), jsonObjectEmpty; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyRenameCarriesIntoTheReportedBody: a preview stands in for the
// member by its wire key, so a renamed field is still the one withheld.
func TestWriteBodyRenameCarriesIntoTheReportedBody(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{"new_secret": "hunter2"}), 1)

	body.Rename("new_secret", "secret")
	body.SetString("new_secret")

	if got, want := marshalBody(t, body.Redacting("secret")), `{"secret":{"redacted":true}}`; got != want {
		t.Errorf("reported body = %s, want %s", got, want)
	}
}

// TestWriteBodyRenameRefusesNothingItWasNotToldAbout: a field with no rename
// keeps its own name, which is what leaves every other tool's body untouched.
func TestWriteBodyRenameRefusesNothingItWasNotToldAbout(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{managedServiceLabelParam: tagWeb, "new_username": "chad"}), 2)

	body.Rename("new_username", "username")
	body.SetString(managedServiceLabelParam)
	body.SetString("new_username")

	if got, want := marshalBody(t, body), `{"label":"web","username":"chad"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyRedactingStandsInForTheNamedMembers: the reported body keeps
// declaration order and every other member, so a preview still says what the
// call would send apart from the value it withholds.
func TestWriteBodyRedactingStandsInForTheNamedMembers(t *testing.T) {
	t.Parallel()

	arguments := map[string]any{keyPaymentData: map[string]any{cardNumberMember: "4111"}, "type": "credit_card"}
	body := tools.NewWriteBody(bodyRequest(arguments), 2)

	body.SetObject(keyPaymentData)
	body.PutString("type")

	reported := body.Redacting(keyPaymentData)

	if got, want := marshalBody(t, reported), `{"data":{"redacted":true},"type":"credit_card"}`; got != want {
		t.Errorf("reported body = %s, want %s", got, want)
	}

	if got, want := marshalBody(t, body), `{"data":{"card_number":"4111"},"type":"credit_card"}`; got != want {
		t.Errorf("live body = %s, want it untouched (%s)", got, want)
	}
}

// TestWriteBodyRedactingLeavesAnUnnamedMemberAlone guards the copy: naming one
// member must not turn the whole reported body into markers.
func TestWriteBodyRedactingLeavesAnUnnamedMemberAlone(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{managedServiceLabelParam: tagWeb}), 1)

	body.PutString(managedServiceLabelParam)

	if got, want := marshalBody(t, body.Redacting(keyPaymentData)), `{"label":"web"}`; got != want {
		t.Errorf("reported body = %s, want %s", got, want)
	}
}

// errPrivateNetworkObjectOrNull is the message a nullable object field answers
// a wrong type with, named here because every rejection case asserts the same.
const errPrivateNetworkObjectOrNull = "private_network must be an object or null"

// keyPrivateNetwork is the nullable object field the database update routes
// read an explicit null on.
const keyPrivateNetwork = "private_network"

// TestWriteBodySetObjectOrNullOmitsAnAbsentMember is the first of the three
// states this setter exists to keep apart: no argument means no key.
func TestWriteBodySetObjectOrNullOmitsAnAbsentMember(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)

	body.SetObjectOrNull(keyPrivateNetwork)

	if got, want := marshalBody(t, body), jsonObjectEmpty; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetObjectOrNullSendsAnExplicitNull is the state SetObject cannot
// express: the API reads it as a detach, so collapsing it into absence would
// report a change nothing performed.
func TestWriteBodySetObjectOrNullSendsAnExplicitNull(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyPrivateNetwork: nil}), 1)

	body.SetObjectOrNull(keyPrivateNetwork)

	if got, want := marshalBody(t, body), `{"private_network":null}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetObjectOrNullSendsASuppliedObject is the third state, which
// travels exactly as the plain object setter sends it.
func TestWriteBodySetObjectOrNullSendsASuppliedObject(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyPrivateNetwork: map[string]any{"vpc_id": 7, "subnet_id": 3},
	}), 1)

	body.SetObjectOrNull(keyPrivateNetwork)

	if got, want := marshalBody(t, body), `{"private_network":{"subnet_id":3,"vpc_id":7}}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetObjectOrNullSendsAnEmptyObject holds the empty object apart
// from the null: {} is a value the caller supplied, not a detach.
func TestWriteBodySetObjectOrNullSendsAnEmptyObject(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyPrivateNetwork: map[string]any{},
	}), 1)

	body.SetObjectOrNull(keyPrivateNetwork)

	if got, want := marshalBody(t, body), `{"private_network":{}}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetObjectOrNullReportsANonObject names the field, which the
// caller cannot otherwise see, and names null as an accepted value.
func TestWriteBodySetObjectOrNullReportsANonObject(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{"detach", 5, []any{"a"}, true} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyPrivateNetwork: raw}), 1)

		body.SetObjectOrNull(keyPrivateNetwork)

		if got := body.Message(); got != errPrivateNetworkObjectOrNull {
			t.Errorf("message for %v = %q, want %q", raw, got, errPrivateNetworkObjectOrNull)
		}
	}
}

// TestWriteBodySetObjectOrNullKeepsTheFirstFailure holds the nullable setter to
// the rule every other one follows: a later field does not overwrite the report.
func TestWriteBodySetObjectOrNullKeepsTheFirstFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyPrivateNetwork: "detach",
		keyPaymentData:    5,
	}), 2)

	body.SetObjectOrNull(keyPrivateNetwork)
	body.SetObject(keyPaymentData)

	if got := body.Message(); got != errPrivateNetworkObjectOrNull {
		t.Errorf("message = %q, want %q", got, errPrivateNetworkObjectOrNull)
	}
}

// errImagesObjectArray is the sentence a malformed typed list answers with,
// which is the free-form object list's sentence unchanged: the typed reader
// adds member checks inside an item, not a new way to refuse the array.
const errImagesObjectArray = "images must be an array of objects"

// The item fixtures the typed-list cases share: a member no item message
// declares, its value, an item that omits its required member, and the sentence
// a non-integer id answers with.
const (
	keyBogus              = "bogus"
	testUnknownMember     = "surprise"
	testItemWithoutID     = "no id here"
	errImageLinodeIDIsInt = "images[0].linode_id must be an integer"
)

// imageItems is the item shape linode_image_sharegroup_create declares: a
// required id and two optional overrides, in proto declaration order.
func imageItems() []tools.ItemField {
	return []tools.ItemField{
		{Name: keyID, Kind: tools.ItemString, Required: true},
		{Name: keyLabel, Kind: tools.ItemString},
		{Name: keyDescription, Kind: tools.ItemString},
	}
}

// numberedItems carries an integer member, which no image item declares, so the
// integer branch has a shape of its own to be read through.
func numberedItems() []tools.ItemField {
	return []tools.ItemField{
		{Name: keyLinodeID, Kind: tools.ItemInt, Required: true},
		{Name: keyLabel, Kind: tools.ItemString},
	}
}

// TestWriteBodySetMessageListWritesEachItemInDeclarationOrder is the reason the
// typed reader exists: an item's members reach the wire in the order its
// message declares them, whatever order the caller sent them in.
func TestWriteBodySetMessageListWritesEachItemInDeclarationOrder(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{
			name: "scrambled members",
			arguments: map[string]any{keyImages: []any{
				map[string]any{
					keyDescription: "third",
					keyLabel:       "second",
					keyID:          imagePrivate15Fixture,
				},
			}},
			want: `{"images":[{"id":"private/15","label":"second","description":"third"}]}`,
		},
		{
			name: "optional members omitted",
			arguments: map[string]any{keyImages: []any{
				map[string]any{keyID: imagePrivate15Fixture},
			}},
			want: `{"images":` + imagePrivate15JSON + `}`,
		},
		{
			name: "several items",
			arguments: map[string]any{keyImages: []any{
				map[string]any{keyID: imagePrivate15Fixture, keyLabel: "first"},
				map[string]any{keyID: "private/16"},
			}},
			want: `{"images":[{"id":"private/15","label":"first"},{"id":"private/16"}]}`,
		},
		{
			name:      caseSuppliedEmpty,
			arguments: map[string]any{keyImages: []any{}},
			want:      `{"images":[]}`,
		},
		{
			name: "native slice of maps",
			arguments: map[string]any{keyImages: []map[string]any{
				{keyID: imagePrivate15Fixture},
			}},
			want: `{"images":` + imagePrivate15JSON + `}`,
		},
		{name: caseOmitted, arguments: map[string]any{}, want: jsonObjectEmpty},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)

			body.SetMessageList(keyImages, imageItems())

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetMessageListRefusesTheOuterShapeTheObjectListRefuses: the
// array itself is read by the free-form reader, so a JSON string and a
// non-object entry are refused with the sentence they always were.
func TestWriteBodySetMessageListRefusesTheOuterShapeTheObjectListRefuses(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{
		imagePrivate15JSON,
		float64(5),
		true,
		nil,
		map[string]any{keyID: imagePrivate15Fixture},
		[]any{imagePrivate15Fixture},
		[]any{map[string]any{keyID: imagePrivate15Fixture}, float64(2)},
		[]any{nil},
	} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyImages: raw}), 1)

		body.SetMessageList(keyImages, imageItems())

		if got := body.Message(); got != errImagesObjectArray {
			t.Errorf("message for %#v = %q, want %q", raw, got, errImagesObjectArray)
		}
	}
}

// TestWriteBodySetMessageListReportsTheMemberItDeclares covers every way one
// item can be wrong, each named with the index that carries it.
func TestWriteBodySetMessageListReportsTheMemberItDeclares(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		raw    any
		name   string
		want   string
		fields []tools.ItemField
	}{
		{
			name:   "string member is not a string",
			fields: imageItems(),
			raw: []any{map[string]any{
				keyID: imagePrivate15Fixture, keyLabel: float64(7),
			}},
			want: `images[0].label must be a string`,
		},
		{
			name:   "integer member is not an integer",
			fields: numberedItems(),
			raw:    []any{map[string]any{keyLinodeID: "twelve"}},
			want:   errImageLinodeIDIsInt,
		},
		{
			name:   "integer member is a fraction",
			fields: numberedItems(),
			raw:    []any{map[string]any{keyLinodeID: 1.5}},
			want:   errImageLinodeIDIsInt,
		},
		{
			name:   "integer member is null",
			fields: numberedItems(),
			raw:    []any{map[string]any{keyLinodeID: nil}},
			want:   errImageLinodeIDIsInt,
		},
		{
			name:   "integer member is a boolean",
			fields: numberedItems(),
			raw:    []any{map[string]any{keyLinodeID: true}},
			want:   errImageLinodeIDIsInt,
		},
		{
			name:   "required member is missing",
			fields: imageItems(),
			raw:    []any{map[string]any{keyLabel: testItemWithoutID}},
			want:   `images[0].id is required`,
		},
		{
			name:   "member the message does not declare",
			fields: imageItems(),
			raw: []any{map[string]any{
				keyID: imagePrivate15Fixture, keyBogus: testUnknownMember,
			}},
			want: `images[0] has no field "bogus"`,
		},
		{
			name:   "the failing item is the one reported",
			fields: imageItems(),
			raw: []any{
				map[string]any{keyID: imagePrivate15Fixture},
				map[string]any{keyLabel: testItemWithoutID},
			},
			want: `images[1].id is required`,
		},
		{
			name:   "the first failing item wins",
			fields: imageItems(),
			raw: []any{
				map[string]any{keyID: imagePrivate15Fixture},
				map[string]any{keyLabel: testItemWithoutID},
				map[string]any{keyBogus: testUnknownMember},
			},
			want: `images[1].id is required`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(map[string]any{keyImages: testCase.raw}), 1)

			body.SetMessageList(keyImages, testCase.fields)

			if got := body.Message(); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetMessageListReportsTheFirstMemberInDeclarationOrder: an item
// wrong in more than one way answers with its first declared problem, which is
// the rule the body already follows across fields.
func TestWriteBodySetMessageListReportsTheFirstMemberInDeclarationOrder(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyImages: []any{
		map[string]any{keyLabel: float64(7), keyBogus: testUnknownMember},
	}}), 1)

	body.SetMessageList(keyImages, imageItems())

	const want = `images[0].id is required`
	if got := body.Message(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestWriteBodySetMessageListAcceptsAnIntegralFloat: a JSON-RPC number reaches
// Go as a float and Python as an int, so an item's integer member has to read
// both shapes for one call to send one body.
func TestWriteBodySetMessageListAcceptsAnIntegralFloat(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyImages: []any{
		map[string]any{keyLinodeID: float64(12)},
		map[string]any{keyLinodeID: 13},
	}}), 1)

	body.SetMessageList(keyImages, numberedItems())

	const want = `{"images":[{"linode_id":12},{"linode_id":13}]}`
	if got := marshalBody(t, body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetMessageListKeepsTheFirstFailure holds the typed setter to the
// rule every other one follows: a later field does not overwrite the report.
func TestWriteBodySetMessageListKeepsTheFirstFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyImages:      []any{map[string]any{keyLabel: testItemWithoutID}},
		keyPaymentData: 5,
	}), 2)

	body.SetMessageList(keyImages, imageItems())
	body.SetObject(keyPaymentData)

	const want = `images[0].id is required`
	if got := body.Message(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// The string-to-string map the label cases read, and the two sentences it
// answers, which the emitted node-pool bodies and their fixtures also spell.
const (
	keyPoolLabels     = "labels"
	errLabelsObject   = "labels must be an object"
	errLabelValuesStr = "labels values must be strings"
	caseSupplied      = "supplied"
)

// TestWriteBodySetStringMapWritesEachMember: a typed map travels as it arrived,
// and an empty one is a value the same way an empty object is.
func TestWriteBodySetStringMapWritesEachMember(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{
			name:      caseSupplied,
			arguments: map[string]any{keyPoolLabels: map[string]any{keyLKETier: "app", "zone": "a"}},
			want:      `{"labels":{"tier":"app","zone":"a"}}`,
		},
		{
			name:      "supplied natively",
			arguments: map[string]any{keyPoolLabels: map[string]string{keyLKETier: "app"}},
			want:      `{"labels":{"tier":"app"}}`,
		},
		{
			name:      caseSuppliedEmpty,
			arguments: map[string]any{keyPoolLabels: map[string]any{}},
			want:      `{"labels":{}}`,
		},
		{name: caseOmitted, arguments: map[string]any{}, want: jsonObjectEmpty},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)

			body.SetStringMap(keyPoolLabels)

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetStringMapRefusesWhatIsNotTextKeyedText: the caller who sent an
// array and the caller who sent a number under a key have different things to
// fix, so the two sentences stay apart.
func TestWriteBodySetStringMapRefusesWhatIsNotTextKeyedText(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		raw  any
		want string
	}{
		{raw: "tier=app", want: errLabelsObject},
		{raw: float64(5), want: errLabelsObject},
		{raw: []any{"tier"}, want: errLabelsObject},
		{raw: nil, want: errLabelsObject},
		{raw: map[string]any{keyLKETier: float64(5)}, want: errLabelValuesStr},
		{raw: map[string]any{keyLKETier: nil}, want: errLabelValuesStr},
	} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyPoolLabels: testCase.raw}), 1)

		body.SetStringMap(keyPoolLabels)

		if got := body.Message(); got != testCase.want {
			t.Errorf("message for %#v = %q, want %q", testCase.raw, got, testCase.want)
		}
	}
}

// nodePoolItems is the item shape linode_lke_cluster_create declares: the
// required type/count pair, then the two members proto3 gives no `optional` to
// spell, so their presence is read from the item rather than the declaration.
func nodePoolItems() []tools.ItemField {
	return []tools.ItemField{
		{Name: keyType, Kind: tools.ItemString, Required: true},
		{Name: keyCount, Kind: tools.ItemInt, Required: true},
		{Name: keyAutoscaler, Kind: tools.ItemObject},
		{Name: keyTags, Kind: tools.ItemStringList},
	}
}

// The node-pool fixtures the two non-scalar item kinds share. The autoscaler
// keys and the tag values are named because goconst counts every repeat.
const (
	nodePoolTypeFixture   = "g6-standard-2"
	poolScaleMax          = "max"
	poolScaleMin          = "min"
	poolTagPrimary        = "team-a"
	poolTagSecondary      = "team-b"
	errPoolAutoscalerObj  = "node_pools[0].autoscaler must be an object"
	errPoolTagsStringList = "node_pools[0].tags must be an array of strings"
	jsonPoolBase          = `{"node_pools":[{"type":"g6-standard-2","count":3`
)

// TestWriteBodySetMessageListWritesTheNonScalarMembersANodePoolDeclares: the
// two kinds an item carries beyond scalars reach the wire under the same
// ordering the scalar members do, and each is optional because no proto3 map or
// list can declare presence.
func TestWriteBodySetMessageListWritesTheNonScalarMembersANodePoolDeclares(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		item map[string]any
		name string
		want string
	}{
		{
			name: "every member",
			item: map[string]any{
				keyType:       nodePoolTypeFixture,
				keyCount:      float64(3),
				keyAutoscaler: map[string]any{poolScaleMax: float64(5)},
				keyTags:       []any{poolTagPrimary, poolTagSecondary},
			},
			want: jsonPoolBase + `,"autoscaler":{"max":5},"tags":["team-a","team-b"]}]}`,
		},
		{
			name: "non-scalar members omitted",
			item: map[string]any{keyType: nodePoolTypeFixture, keyCount: float64(3)},
			want: jsonPoolBase + `}]}`,
		},
		{
			name: "empty object and empty list still travel",
			item: map[string]any{
				keyType:       nodePoolTypeFixture,
				keyCount:      float64(3),
				keyAutoscaler: map[string]any{},
				keyTags:       []any{},
			},
			want: jsonPoolBase + `,"autoscaler":{},"tags":[]}]}`,
		},
		{
			name: "native string slice",
			item: map[string]any{
				keyType:  nodePoolTypeFixture,
				keyCount: float64(3),
				keyTags:  []string{poolTagPrimary},
			},
			want: jsonPoolBase + `,"tags":["team-a"]}]}`,
		},
		{
			// Sorted rather than sent as given, which is the one thing Go's
			// map marshaling cannot be told otherwise about.
			name: "nested object members are key-sorted",
			item: map[string]any{
				keyType:       nodePoolTypeFixture,
				keyCount:      float64(3),
				keyAutoscaler: map[string]any{poolScaleMin: float64(1), poolScaleMax: float64(5)},
			},
			want: jsonPoolBase + `,"autoscaler":{"max":5,"min":1}}]}`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			arguments := map[string]any{keyNodePools: []any{testCase.item}}
			body := tools.NewWriteBody(bodyRequest(arguments), 1)

			body.SetMessageList(keyNodePools, nodePoolItems())

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetMessageListRefusesANonObjectItemMember: an object member is
// held to the shape the field-level object setter holds its own to, indexed
// because the caller has to know which item to fix.
func TestWriteBodySetMessageListRefusesANonObjectItemMember(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{
		nodePoolTypeFixture, float64(1), true, nil,
		[]any{},
		map[string]string{poolScaleMin: poolScaleMax},
	} {
		item := map[string]any{
			keyType: nodePoolTypeFixture, keyCount: float64(3), keyAutoscaler: raw,
		}
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyNodePools: []any{item}}), 1)

		body.SetMessageList(keyNodePools, nodePoolItems())

		if got := body.Message(); got != errPoolAutoscalerObj {
			t.Errorf("message for %#v = %q, want %q", raw, got, errPoolAutoscalerObj)
		}
	}
}

// TestWriteBodySetMessageListRefusesANonStringListItemMember: a list member is
// held to exactly what a top-level string list is, including the entry that is
// not text.
func TestWriteBodySetMessageListRefusesANonStringListItemMember(t *testing.T) {
	t.Parallel()

	for _, raw := range []any{
		poolTagPrimary, float64(1), true, nil,
		map[string]any{},
		[]any{poolTagPrimary, float64(2)},
		[]any{nil},
	} {
		item := map[string]any{
			keyType: nodePoolTypeFixture, keyCount: float64(3), keyTags: raw,
		}
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyNodePools: []any{item}}), 1)

		body.SetMessageList(keyNodePools, nodePoolItems())

		if got := body.Message(); got != errPoolTagsStringList {
			t.Errorf("message for %#v = %q, want %q", raw, got, errPoolTagsStringList)
		}
	}
}

// TestWriteBodySetStringOrNullKeepsTheThreeStatesApart is why the setter
// exists: the API reads a null as "clear this", which absence does not say.
func TestWriteBodySetStringOrNullKeepsTheThreeStatesApart(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: caseOmitted, arguments: map[string]any{}, want: jsonObjectEmpty},
		{
			name:      "explicit null",
			arguments: map[string]any{keyNotes: nil},
			want:      `{"notes":null}`,
		},
		{
			name:      "supplied text",
			arguments: map[string]any{keyNotes: "on call"},
			want:      `{"notes":"on call"}`,
		},
		{
			name:      "supplied empty text",
			arguments: map[string]any{keyNotes: ""},
			want:      `{"notes":""}`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)

			body.SetStringOrNull(keyNotes)

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetStringOrNullRefusesWhatIsNeitherTextNorNull names both states
// it accepts, since a caller who sent a number cannot tell from "must be a
// string" that null was available.
func TestWriteBodySetStringOrNullRefusesWhatIsNeitherTextNorNull(t *testing.T) {
	t.Parallel()

	const want = "notes must be a string or null"

	for _, raw := range []any{float64(1), true, []any{"on call"}, map[string]any{}} {
		body := tools.NewWriteBody(bodyRequest(map[string]any{keyNotes: raw}), 1)

		body.SetStringOrNull(keyNotes)

		if got := body.Message(); got != want {
			t.Errorf("message for %#v = %q, want %q", raw, got, want)
		}
	}
}

// TestWriteBodySetStringOrNullKeepsTheFirstFailure: the null branch returns no
// message, so the skip-after-failure rule has to hold across it.
func TestWriteBodySetStringOrNullKeepsTheFirstFailure(t *testing.T) {
	t.Parallel()

	arguments := map[string]any{keyLabel: float64(1), keyNotes: nil}
	body := tools.NewWriteBody(bodyRequest(arguments), 2)

	body.SetString(keyLabel)
	body.SetStringOrNull(keyNotes)

	if got, want := body.Message(), "label must be a string"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}

	if got := marshalBody(t, body); got != jsonObjectEmpty {
		t.Errorf("body = %s, want %s", got, jsonObjectEmpty)
	}
}

// The comma-composed cases both languages are held to, named because the Python
// twin runs the same table and the two must agree byte for byte.
func commaListCases() []struct {
	name  string
	value any
	want  string
} {
	return []struct {
		name  string
		value any
		want  string
	}{
		{name: "one entry", value: "ssh-rsa AAA", want: `{"authorized_keys":["ssh-rsa AAA"]}`},
		{name: "trims each entry", value: " a , b ", want: `{"authorized_keys":["a","b"]}`},
		{name: "drops blank segments", value: "a,,b,", want: `{"authorized_keys":["a","b"]}`},
		{name: "keeps inner spaces", value: "ssh-rsa AAA me@host", want: `{"authorized_keys":["ssh-rsa AAA me@host"]}`},
		{name: "empty string", value: "", want: `{"authorized_keys":[]}`},
		{name: "separators only", value: " , , ", want: `{"authorized_keys":[]}`},
	}
}

// TestWriteBodyComposesACommaSeparatedList: the argument is one line of text and
// the member is an array, so the split is the whole difference between them.
func TestWriteBodyComposesACommaSeparatedList(t *testing.T) {
	t.Parallel()

	for _, testCase := range commaListCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(map[string]any{
				keyAuthorizedKeys: testCase.value,
			}), 1)

			body.SetCommaStringList(keyAuthorizedKeys)

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodyOmitsAnUnsuppliedCommaList pins the other half of presence: the
// composition is a wire shape, not a reason to send an empty array unasked.
func TestWriteBodyOmitsAnUnsuppliedCommaList(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)

	body.SetCommaStringList(keyAuthorizedKeys)

	if got, want := marshalBody(t, body), jsonObjectEmpty; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodyRefusesANonStringCommaList: the caller sends text, so the
// sentence names the shape they send rather than the array it becomes.
func TestWriteBodyRefusesANonStringCommaList(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyAuthorizedKeys: []any{"a"},
	}), 1)

	body.SetCommaStringList(keyAuthorizedKeys)

	if got, want := body.Message(), keyAuthorizedKeys+" must be a string"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestWriteBodyRenamesAComposedCommaList: the composition and the rename are
// separate declarations, so a field can carry both.
func TestWriteBodyRenamesAComposedCommaList(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{"users": "a, b"}), 1)
	body.Rename("users", "authorized_users")
	body.SetCommaStringList("users")

	if got, want := marshalBody(t, body), `{"authorized_users":["a","b"]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// firewallRules is the fold the firewall create declares: two flat policy
// arguments that the API reads inside its rules object, each defaulting to
// ACCEPT the way the hand-written builder does.
func firewallRules() []tools.FoldMember {
	return []tools.FoldMember{
		{Key: keyInboundPolicy, Argument: keyInboundPolicy, Kind: tools.ItemString, Value: policyAccept},
		{Key: keyOutboundPolicy, Argument: keyOutboundPolicy, Kind: tools.ItemString, Value: policyAccept},
	}
}

// A fold is what lets a tool take an argument the API reads one level down.
// Posting inbound_policy at the top of the body looks like a working call and
// changes nothing, because the API does not read it there.
func TestWriteBodyFoldsFlatArgumentsIntoTheMemberTheAPIReads(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{
			name:      "neither supplied",
			arguments: map[string]any{},
			want:      `{"rules":{"inbound_policy":"ACCEPT","outbound_policy":"ACCEPT"}}`,
		},
		{
			name:      "both supplied flat",
			arguments: map[string]any{keyInboundPolicy: policyDrop, keyOutboundPolicy: policyDrop},
			want:      `{"rules":{"inbound_policy":"DROP","outbound_policy":"DROP"}}`,
		},
		{
			// The caller wins per key: theirs is left alone and the other folds
			// in beside it, which is what keeps the flat arguments optional.
			name: "caller wrote one key",
			arguments: map[string]any{
				keyRules:         map[string]any{keyInboundPolicy: policyDrop},
				keyInboundPolicy: policyAccept,
			},
			want: `{"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}`,
		},
		{
			name: "caller wrote the whole object",
			arguments: map[string]any{
				keyRules: map[string]any{keyInboundPolicy: policyDrop, keyOutboundPolicy: policyDrop},
			},
			want: `{"rules":{"inbound_policy":"DROP","outbound_policy":"DROP"}}`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)
			body.Fold(keyRules, firewallRules())

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// The API refuses an instance create with no interfaces, so the tool builds one
// out of the firewall id and the route flags the caller sent flat. A caller who
// supplies the list gets it sent verbatim: an element beside theirs would
// attach an interface they never asked for.
func TestWriteBodyFoldsASynthesizedListElement(t *testing.T) {
	t.Parallel()

	members := []tools.FoldMember{
		{Key: keyFirewallID, Argument: keyFirewallID, Kind: tools.ItemInt},
		{Key: "default_route.ipv4", Argument: "route_ipv4", Kind: tools.ItemBool, Value: true},
		{Key: "default_route.ipv6", Argument: keyRouteIPv6, Kind: tools.ItemBool, Value: true},
		{Key: purposePublic, Kind: tools.ItemObject, Value: map[string]any{}},
	}

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{
			name:      "synthesized from the flat arguments",
			arguments: map[string]any{keyFirewallID: 7},
			want: `{"interfaces":[{"default_route":{"ipv4":true,"ipv6":true},` +
				`"firewall_id":7,"public":{}}]}`,
		},
		{
			// A flag the caller turned off travels as false rather than falling
			// back to the default the contract declares.
			name:      "a route flag the caller turned off",
			arguments: map[string]any{keyFirewallID: 7, keyRouteIPv6: false},
			want: `{"interfaces":[{"default_route":{"ipv4":true,"ipv6":false},` +
				`"firewall_id":7,"public":{}}]}`,
		},
		{
			name: "caller supplied the list",
			arguments: map[string]any{
				keyFirewallID: 7,
				keyInterfaces: []any{map[string]any{"vlan": map[string]any{}}},
			},
			want: `{"interfaces":[{"vlan":{}}]}`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)
			body.FoldList(keyInterfaces, members)

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// A folded argument is held to its declared kind the way every other body
// member is, and the failure names where the value was going. Both fold shapes
// answer it, since a synthesized element reads its members the same way.
func TestWriteBodyRefusesAFoldedArgumentOfTheWrongKind(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyInboundPolicy: 5}), 1)
	body.Fold(keyRules, firewallRules())

	if want := "rules.inbound_policy must be a string"; body.Message() != want {
		t.Errorf("message = %q, want %q", body.Message(), want)
	}

	listed := tools.NewWriteBody(bodyRequest(map[string]any{keyFirewallID: "seven"}), 1)
	listed.FoldList(keyInterfaces, []tools.FoldMember{
		{Key: keyFirewallID, Argument: keyFirewallID, Kind: tools.ItemInt},
	})

	if want := "interfaces.firewall_id must be an integer"; listed.Message() != want {
		t.Errorf("message = %q, want %q", listed.Message(), want)
	}
}

// A member the tool always sends and takes no argument for: every instance this
// server creates is on the current Linode Interfaces generation.
func TestWriteBodyWritesAConstantMember(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)
	body.Constant("interface_generation", "linode")

	if want := `{"interface_generation":"linode"}`; marshalBody(t, body) != want {
		t.Errorf("body = %s, want %s", marshalBody(t, body), want)
	}
}

// A default fills a member the caller omitted, and yields to one they sent.
// The Object Storage upload presigns with an explicit Content-Type because the
// signature covers that header, so the value has to be in the presign body
// whether or not the caller named it.
func TestWriteBodyDefaultsOnlyWhatTheCallerOmitted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{
			name:      "fills an omitted member",
			arguments: map[string]any{},
			want:      `{"content_type":"application/octet-stream"}`,
		},
		{
			name:      "leaves a supplied member alone",
			arguments: map[string]any{"content_type": "text/plain"},
			want:      `{"content_type":"text/plain"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(tt.arguments), 1)
			body.SetString("content_type")
			body.Default("content_type", "application/octet-stream")

			if got := marshalBody(t, body); got != tt.want {
				t.Errorf("body = %s, want %s", got, tt.want)
			}
		})
	}
}

// A default after a type failure writes nothing, for the reason a fold does:
// the first failure answers the whole call.
func TestWriteBodyDefaultsNothingAfterAFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{managedServiceLabelParam: 5}), 2)
	body.PutString(managedServiceLabelParam)
	body.Default("content_type", "application/octet-stream")

	if want := errLabelString; body.Message() != want {
		t.Errorf("message = %q, want %q", body.Message(), want)
	}
}

// The first type failure answers the whole call, so a fold declared after one
// writes nothing rather than reporting a second problem.
func TestWriteBodyFoldsNothingAfterAFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{"label": 5}), 2)
	body.PutString("label")
	body.Fold(keyRules, firewallRules())
	body.FoldList(keyInterfaces, nil)
	body.Constant("interface_generation", "linode")

	if want := errLabelString; body.Message() != want {
		t.Errorf("message = %q, want %q", body.Message(), want)
	}
}

// An assembled member the caller supplied is still held to its shape: a fold
// merges into an object and a fold list sends an array of them.
func TestWriteBodyRefusesAMisshapedFoldTarget(t *testing.T) {
	t.Parallel()

	cases := []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{
			name:      "rules is not an object",
			arguments: map[string]any{keyRules: "ACCEPT"},
			want:      errRulesNotObject,
		},
		{
			name:      "interfaces is not an array of objects",
			arguments: map[string]any{keyInterfaces: purposePublic},
			want:      "interfaces must be an array of objects",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)
			body.Fold(keyRules, firewallRules())
			body.FoldList(keyInterfaces, nil)

			if body.Message() != testCase.want {
				t.Errorf("message = %q, want %q", body.Message(), testCase.want)
			}
		})
	}
}

// A folded argument the contract declares no default for is left out when the
// caller omits it, rather than traveling as its type's zero.
func TestWriteBodyOmitsAFoldedArgumentWithNoDefault(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)
	body.FoldList(keyInterfaces, []tools.FoldMember{
		{Key: keyFirewallID, Argument: keyFirewallID, Kind: tools.ItemInt},
	})

	if want := `{"interfaces":[{}]}`; marshalBody(t, body) != want {
		t.Errorf("body = %s, want %s", marshalBody(t, body), want)
	}
}

// firewallUpdateRules is the update's fold: the same two arguments the create
// folds, with no default, so a call naming neither leaves the member off.
func firewallUpdateRules() []tools.FoldMember {
	return []tools.FoldMember{
		{Key: keyInboundPolicy, Argument: keyInboundPolicy, Kind: tools.ItemString},
		{Key: keyOutboundPolicy, Argument: keyOutboundPolicy, Kind: tools.ItemString},
	}
}

// The firewall update reads `rules` as a replacement rather than a setting, so
// a call that names no policy has to leave the member off the wire entirely.
// Sending the empty object there would clear the ruleset the caller never
// mentioned, which is what separates FoldOptional from Fold.
func TestWriteBodyOmitsAnEmptyOptionalFold(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args map[string]any
		want string
	}{
		"neither policy": {
			args: map[string]any{managedServiceLabelParam: "renamed"},
			want: `{"label":"renamed"}`,
		},
		"one policy": {
			args: map[string]any{keyInboundPolicy: policyDrop},
			want: `{"rules":{"inbound_policy":"DROP"}}`,
		},
		"both policies": {
			args: map[string]any{keyInboundPolicy: policyDrop, keyOutboundPolicy: policyAccept},
			want: `{"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}`,
		},
		"caller wrote the member": {
			args: map[string]any{keyRules: map[string]any{keyInboundPolicy: policyDrop}},
			want: `{"rules":{"inbound_policy":"DROP"}}`,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.args), 2)
			body.SetString(managedServiceLabelParam)
			body.FoldOptional(keyRules, firewallUpdateRules())

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// An optional fold is held to the same two failures the always-sent one is:
// a member the caller supplied as something other than an object, and a folded
// argument of the wrong kind.
func TestWriteBodyRefusesABadOptionalFold(t *testing.T) {
	t.Parallel()

	object := tools.NewWriteBody(bodyRequest(map[string]any{keyRules: 5}), 1)
	object.FoldOptional(keyRules, firewallUpdateRules())

	if want := "rules must be an object"; object.Message() != want {
		t.Errorf("message = %q, want %q", object.Message(), want)
	}

	member := tools.NewWriteBody(bodyRequest(map[string]any{keyInboundPolicy: 5}), 1)
	member.FoldOptional(keyRules, firewallUpdateRules())

	if want := "rules.inbound_policy must be a string"; member.Message() != want {
		t.Errorf("message = %q, want %q", member.Message(), want)
	}
}

// The first type failure answers the whole call, so an optional fold declared
// after one writes nothing rather than reporting a second problem.
func TestWriteBodyOptionalFoldsNothingAfterAFailure(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{managedServiceLabelParam: 5}), 2)
	body.PutString(managedServiceLabelParam)
	body.FoldOptional(keyRules, firewallUpdateRules())

	if want := errLabelString; body.Message() != want {
		t.Errorf("message = %q, want %q", body.Message(), want)
	}
}
