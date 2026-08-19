package tools_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The members and values the nested fixture below is built from, named because
// the assertions spell them as often as the spec does.
const (
	keyRangeEntry      = "range"
	keyRanges          = "ranges"
	keyPrimary         = "primary"
	keyUnknown         = "extra"
	keyInterfacePublic = "public"
	keyInterfaceVPC    = "vpc"
	keyAddresses       = "addresses"
	blankWhitespace    = "  "
	nestedAddressOne   = "10.0.0.4"
	nestedAddressTwo   = "10.0.0.5"
	nestedLabelOne     = "first"
	nestedLabelTwo     = "second"
	caseNestedBoolean  = "boolean"
	valueDetach        = "detach"
)

// nestedVPCFields is the shape the recursive reader exists for: a named message
// whose members carry named messages of their own, two deep, which is what a
// Linode interface's vpc arm declares.
func nestedVPCFields() []tools.ItemField {
	return []tools.ItemField{
		{Name: keySubnetID, Kind: tools.ItemInt, Required: true},
		{Name: keyIPv4, Kind: tools.ItemMessage, Fields: []tools.ItemField{
			{Name: keyAddresses, Kind: tools.ItemMessageList, Fields: []tools.ItemField{
				{Name: keyAddress, Kind: tools.ItemString, Required: true},
				{Name: keyPrimary, Kind: tools.ItemBool},
			}},
			{Name: keyRanges, Kind: tools.ItemMessageList, Fields: []tools.ItemField{
				{Name: keyRangeEntry, Kind: tools.ItemString, Required: true},
			}},
		}},
	}
}

// nestedVPCArgument is one well-formed vpc argument and the nested member the
// refusal cases reach into. It is built fresh per case, so a case that rewrites
// a member cannot reach the one after it.
func nestedVPCArgument() (map[string]any, map[string]any) {
	ipv4 := map[string]any{
		keyAddresses: []any{
			map[string]any{keyAddress: nestedAddressOne, keyPrimary: true},
			map[string]any{keyAddress: nestedAddressTwo},
		},
		keyRanges: []any{map[string]any{keyRangeEntry: cidrV4}},
	}

	return map[string]any{keySubnetID: 456, keyIPv4: ipv4}, ipv4
}

// TestWriteBodySetMessageWritesEveryDepthInDeclarationOrder is the reason the
// recursive reader exists: declaration order is wire order at every depth, so a
// caller who scrambles a nested member still reaches the same bytes Python does.
func TestWriteBodySetMessageWritesEveryDepthInDeclarationOrder(t *testing.T) {
	t.Parallel()

	scrambled := map[string]any{
		keyIPv4: map[string]any{
			keyRanges:    []any{map[string]any{keyRangeEntry: cidrV4}},
			keyAddresses: []any{map[string]any{keyPrimary: true, keyAddress: nestedAddressOne}},
		},
		keySubnetID: 456,
	}

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyInterfaceVPC: scrambled}), 1)
	body.SetMessage(keyInterfaceVPC, nestedVPCFields())

	want := `{"vpc":{"subnet_id":456,"ipv4":{"addresses":[{"address":"10.0.0.4","primary":true}],` +
		`"ranges":[{"range":"10.0.0.0/24"}]}}}`
	if got := marshalBody(t, body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetMessageOmitsAbsentNestedMembers pins presence at depth: an
// optional member nobody sent is not a request to set it to its zero, whichever
// depth it sits at.
func TestWriteBodySetMessageOmitsAbsentNestedMembers(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{
		keyInterfaceVPC: map[string]any{keySubnetID: 456},
	}), 1)
	body.SetMessage(keyInterfaceVPC, nestedVPCFields())

	want := `{"vpc":{"subnet_id":456}}`
	if got := marshalBody(t, body); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// TestWriteBodySetMessageOmitsWhatTheCallerDidNotSupply keeps the field-level
// half of the same rule: a message field carries no proto3 `optional`, so
// absence is read from the arguments.
func TestWriteBodySetMessageOmitsWhatTheCallerDidNotSupply(t *testing.T) {
	t.Parallel()

	body := tools.NewWriteBody(bodyRequest(map[string]any{}), 1)
	body.SetMessage(keyInterfaceVPC, nestedVPCFields())

	if got := marshalBody(t, body); got != jsonObjectEmpty {
		t.Errorf("body = %s, want %s", got, jsonObjectEmpty)
	}
}

// TestWriteBodySetMessageOrNullKeepsTheThreeStatesApart is the detach case: an
// explicit null is the value the API acts on, and collapsing it into absence
// would report a detach nothing performed.
func TestWriteBodySetMessageOrNullKeepsTheThreeStatesApart(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		arguments map[string]any
		name      string
		want      string
	}{
		{name: caseOmitted, arguments: map[string]any{}, want: jsonObjectEmpty},
		{
			name:      "explicit null",
			arguments: map[string]any{keyInterfaceVPC: nil},
			want:      `{"vpc":null}`,
		},
		{
			name:      "supplied message",
			arguments: map[string]any{keyInterfaceVPC: map[string]any{keySubnetID: 456}},
			want:      `{"vpc":{"subnet_id":456}}`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(testCase.arguments), 1)
			body.SetMessageOrNull(keyInterfaceVPC, nestedVPCFields())

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetMessageOrNullRefusesANonObject pins the one sentence the
// nullable setter words differently from the plain one.
func TestWriteBodySetMessageOrNullRefusesANonObject(t *testing.T) {
	t.Parallel()

	want := "vpc must be an object or null"

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyInterfaceVPC: valueDetach}), 1)
	body.SetMessageOrNull(keyInterfaceVPC, nestedVPCFields())

	if got := body.Message(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestWriteBodySetMessageRefusalsNameThePathTheyFailedAt is what makes a nested
// failure actionable: the sentence addresses the member by the path the reader
// reached it through, and both languages take that wording from one emitter.
func TestWriteBodySetMessageRefusalsNameThePathTheyFailedAt(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		build func(argument, ipv4 map[string]any) any
		name  string
		want  string
	}{
		{
			name:  "field is not an object",
			build: func(_, _ map[string]any) any { return valueDetach },
			want:  "vpc must be an object",
		},
		{
			name: "required member missing at depth zero",
			build: func(argument, _ map[string]any) any {
				delete(argument, keySubnetID)

				return argument
			},
			want: "vpc.subnet_id is required",
		},
		{
			name: "nested message is not an object",
			build: func(argument, _ map[string]any) any {
				argument[keyIPv4] = []any{}

				return argument
			},
			want: "vpc.ipv4 must be an object",
		},
		{
			name: "nested list is not an array",
			build: func(argument, ipv4 map[string]any) any {
				ipv4[keyAddresses] = map[string]any{}

				return argument
			},
			want: "vpc.ipv4.addresses must be an array of objects",
		},
		{
			name: "required member missing inside a nested entry",
			build: func(argument, ipv4 map[string]any) any {
				ipv4[keyAddresses] = []any{
					map[string]any{keyAddress: nestedAddressOne},
					map[string]any{keyPrimary: true},
				}

				return argument
			},
			want: "vpc.ipv4.addresses[1].address is required",
		},
		{
			name: "wrong kind inside a nested entry",
			build: func(argument, ipv4 map[string]any) any {
				ipv4[keyAddresses] = []any{
					map[string]any{keyAddress: nestedAddressOne, keyPrimary: "yes"},
				}

				return argument
			},
			want: "vpc.ipv4.addresses[0].primary must be a boolean",
		},
		{
			name: "undeclared member on a nested message",
			build: func(argument, ipv4 map[string]any) any {
				ipv4[keyUnknown] = 1

				return argument
			},
			want: `vpc.ipv4 has no field "extra"`,
		},
		{
			name: "undeclared member on a nested entry",
			build: func(argument, ipv4 map[string]any) any {
				ipv4[keyAddresses] = []any{
					map[string]any{keyAddress: nestedAddressOne, keyUnknown: 1},
				}

				return argument
			},
			want: `vpc.ipv4.addresses[0] has no field "extra"`,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			argument := testCase.build(nestedVPCArgument())

			body := tools.NewWriteBody(bodyRequest(map[string]any{keyInterfaceVPC: argument}), 1)
			body.SetMessage(keyInterfaceVPC, nestedVPCFields())

			if got := body.Message(); got != testCase.want {
				t.Errorf("message = %q, want %q", got, testCase.want)
			}

			if got := marshalBody(t, body); got != jsonObjectEmpty {
				t.Errorf("body = %s, want %s on a refused call", got, jsonObjectEmpty)
			}
		})
	}
}

// TestWriteBodySetMessageListReadsNestedItems keeps the repeated field on the
// same reader: an item member carrying a message of its own is held to the same
// shape a field-level one is, addressed through the item's index.
func TestWriteBodySetMessageListReadsNestedItems(t *testing.T) {
	t.Parallel()

	fields := []tools.ItemField{
		{Name: keyLabel, Kind: tools.ItemString, Required: true},
		{Name: keyIPv4, Kind: tools.ItemMessage, Fields: []tools.ItemField{
			{Name: keyAddress, Kind: tools.ItemString, Required: true},
		}},
	}

	for _, testCase := range []struct {
		name      string
		want      string
		wantError string
		items     []any
	}{
		{
			name: "writes each depth in order",
			items: []any{map[string]any{
				keyIPv4:  map[string]any{keyAddress: nestedAddressOne},
				keyLabel: nestedLabelOne,
			}},
			want: `{"interfaces":[{"label":"first","ipv4":{"address":"10.0.0.4"}}]}`,
		},
		{
			name: "names the entry a nested failure sits in",
			items: []any{
				map[string]any{keyLabel: nestedLabelOne},
				map[string]any{keyLabel: nestedLabelTwo, keyIPv4: map[string]any{}},
			},
			want:      jsonObjectEmpty,
			wantError: "interfaces[1].ipv4.address is required",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(map[string]any{keyInterfaces: testCase.items}), 1)
			body.SetMessageList(keyInterfaces, fields)

			if got := body.Message(); got != testCase.wantError {
				t.Errorf("message = %q, want %q", got, testCase.wantError)
			}

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestWriteBodySetMessageListRefusesANonArray pins the outer shape a typed list
// is held to, which is the free-form object list's.
func TestWriteBodySetMessageListRefusesANonArray(t *testing.T) {
	t.Parallel()

	want := "interfaces must be an array of objects"

	body := tools.NewWriteBody(bodyRequest(map[string]any{keyInterfaces: map[string]any{}}), 1)
	body.SetMessageList(keyInterfaces, []tools.ItemField{{Name: keyLabel, Kind: tools.ItemString}})

	if got := body.Message(); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// TestWriteBodyItemBoolReadsOnlyABoolean keeps the new flat kind honest: a
// string that reads like a flag is not one, the same rule the field-level
// boolean setter holds.
func TestWriteBodyItemBoolReadsOnlyABoolean(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		value     any
		name      string
		want      string
		wantError string
	}{
		{name: caseNestedBoolean, value: false, want: `{"interfaces":[{"primary":false}]}`},
		{
			name:      caseFalse,
			value:     caseFalse,
			want:      jsonObjectEmpty,
			wantError: "interfaces[0].primary must be a boolean",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			body := tools.NewWriteBody(bodyRequest(map[string]any{
				keyInterfaces: []any{map[string]any{keyPrimary: testCase.value}},
			}), 1)
			body.SetMessageList(keyInterfaces, []tools.ItemField{{Name: keyPrimary, Kind: tools.ItemBool}})

			if got := body.Message(); got != testCase.wantError {
				t.Errorf("message = %q, want %q", got, testCase.wantError)
			}

			if got := marshalBody(t, body); got != testCase.want {
				t.Errorf("body = %s, want %s", got, testCase.want)
			}
		})
	}
}
