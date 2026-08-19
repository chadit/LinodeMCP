package tools_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// reservedIPGatewayKey is the nullable member this file reads by name to prove
// one restored null apart from the rest.
const reservedIPGatewayKey = "gateway"

// reservedIPNullableFields is the six members Linode documents as nullable on a
// reserved address, which is the shape the restoration exists for: every one is
// an optional scalar or a sub-message, so protojson emits nothing for it.
func reservedIPNullableFields() []string {
	return []string{
		reservedIPAssignedEntity, reservedIPGatewayKey, keyInterfaceID, keySupportTicketLinodeID, keyRDNS, keyReservedIPVPCNAT,
	}
}

// decodeReservedIP reads an API body the way a routed call does.
func decodeReservedIP(t *testing.T, raw json.RawMessage) *linodev1.ReservedIPAddress {
	t.Helper()

	address := &linodev1.ReservedIPAddress{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, address); err != nil {
		t.Fatalf("failed to decode reserved IP body: %v", err)
	}

	return address
}

// resultTextOf reads the one text member an MCP tool result carries.
func resultTextOf(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("result carries %d content members, want 1", len(result.Content))
	}

	content, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatalf("result content is %T, want text", result.Content[0])
	}

	return content.Text
}

// A body carrying every documented null comes back carrying every one of them,
// in the order the message declares, which is what a caller reading the answer
// tells "no gateway" from "the API said nothing about one" by.
func TestRestoresEveryExplicitNullTheBodyCarried(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"address":"192.0.2.10","assigned_entity":null,"gateway":null,` +
		`"interface_id":null,"linode_id":null,"prefix":24,"public":true,"rdns":null,` +
		`"region":"us-east","reserved":true,"subnet_mask":"255.255.255.0","tags":[],` +
		`"type":"ipv4","vpc_nat_1_1":null}`)

	result, err := tools.MarshalProtoToolResponseRestoringNulls(
		decodeReservedIP(t, raw), raw, "", reservedIPNullableFields(),
	)
	if err != nil {
		t.Fatalf("failed to marshal restored response: %v", err)
	}

	want := `{
  "address": "192.0.2.10",
  "assigned_entity": null,
  "gateway": null,
  "interface_id": null,
  "linode_id": null,
  "prefix": 24,
  "public": true,
  "rdns": null,
  "region": "us-east",
  "reserved": true,
  "subnet_mask": "255.255.255.0",
  "tags": [],
  "type": "ipv4",
  "vpc_nat_1_1": null
}`

	if got := resultTextOf(t, result); got != want {
		t.Errorf("restored response is\n%s\nwant\n%s", got, want)
	}
}

// Only a key the API actually sent as null is written back. Restoring the whole
// declared list would answer with five members the API never mentioned.
func TestRestoresOnlyTheNullsTheAPISent(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"address":"192.0.2.10","gateway":null,"region":"us-east"}`)

	result, err := tools.MarshalProtoToolResponseRestoringNulls(
		decodeReservedIP(t, raw), raw, "", reservedIPNullableFields(),
	)
	if err != nil {
		t.Fatalf("failed to marshal restored response: %v", err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(resultTextOf(t, result)), &fields); err != nil {
		t.Fatalf("failed to decode restored response: %v", err)
	}

	value, present := fields[reservedIPGatewayKey]
	if !present || string(value) != databaseJSONNull {
		t.Errorf("gateway is %q, want the null the API sent", value)
	}

	for _, name := range []string{reservedIPAssignedEntity, keyInterfaceID, keySupportTicketLinodeID, keyRDNS, keyReservedIPVPCNAT} {
		if _, invented := fields[name]; invented {
			t.Errorf("restored %s, which the API never sent", name)
		}
	}
}

// A tool declaring no field to restore answers exactly what the shared
// serializer would, so adding the option to one tool cannot reword another's.
func TestRestoringNothingMatchesTheSharedSerializer(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"address":"192.0.2.10","gateway":null,"region":"us-east"}`)
	address := decodeReservedIP(t, raw)

	restored, err := tools.MarshalProtoToolResponseRestoringNulls(address, raw, "", nil)
	if err != nil {
		t.Fatalf("failed to marshal restored response: %v", err)
	}

	shared, err := tools.MarshalProtoToolResponse(address)
	if err != nil {
		t.Fatalf("failed to marshal shared response: %v", err)
	}

	if got, want := resultTextOf(t, restored), resultTextOf(t, shared); got != want {
		t.Errorf("restoring nothing answered\n%s\nwant\n%s", got, want)
	}
}

// A body that is not an object carries no member to read, so the answer stands
// as the serializer built it rather than failing on the decode.
func TestANonObjectBodyRestoresNothing(t *testing.T) {
	t.Parallel()

	object := json.RawMessage(`{"address":"192.0.2.10","gateway":null}`)
	address := decodeReservedIP(t, object)

	for _, body := range []string{`[]`, databaseJSONNull, `"reserved"`} {
		result, err := tools.MarshalProtoToolResponseRestoringNulls(
			address, json.RawMessage(body), "", reservedIPNullableFields(),
		)
		if err != nil {
			t.Fatalf("failed to marshal over body %s: %v", body, err)
		}

		shared, err := tools.MarshalProtoToolResponse(address)
		if err != nil {
			t.Fatalf("failed to marshal shared response: %v", err)
		}

		if got, want := resultTextOf(t, result), resultTextOf(t, shared); got != want {
			t.Errorf("body %s changed the answer to\n%s\nwant\n%s", body, got, want)
		}
	}
}

// An envelope's own members are assembled from the call, so the restoration
// reaches only the member the API body decoded into.
func TestRestoresInsideTheMemberTheBodyDecodedInto(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"id":7,"label":"pg","region":"us-east","migrations":null}`)

	group := &linodev1.PlacementGroup{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, group); err != nil {
		t.Fatalf("failed to decode placement group: %v", err)
	}

	envelope := &linodev1.PlacementGroupWriteResponse{Message: keyCreated, PlacementGroup: group}

	result, err := tools.MarshalProtoToolResponseRestoringNulls(
		envelope, raw, "placement_group", []string{"migrations"},
	)
	if err != nil {
		t.Fatalf("failed to marshal restored envelope: %v", err)
	}

	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(resultTextOf(t, result)), &outer); err != nil {
		t.Fatalf("failed to decode restored envelope: %v", err)
	}

	var inner map[string]json.RawMessage
	if err := json.Unmarshal(outer["placement_group"], &inner); err != nil {
		t.Fatalf("failed to decode restored member: %v", err)
	}

	if value, present := inner["migrations"]; !present || string(value) != databaseJSONNull {
		t.Errorf("member migrations is %q, want the null the API sent", value)
	}

	if _, leaked := outer["migrations"]; leaked {
		t.Error("restored migrations onto the envelope rather than the member")
	}
}

// A string member that is not valid UTF-8 cannot serialize as protojson, and
// the restoring marshaller reports that instead of answering partial bytes.
func TestRestoringMarshallerReportsAnUnserializableMessage(t *testing.T) {
	t.Parallel()

	address := &linodev1.ReservedIPAddress{Address: "\xff\xfe"}

	_, err := tools.MarshalProtoToolResponseRestoringNulls(
		address, json.RawMessage(`{}`), "", reservedIPNullableFields(),
	)
	if err == nil {
		t.Fatal("marshaling invalid UTF-8 succeeded, want an error")
	}
}

// A member that names no message field of the response is a contract defect,
// refused by name rather than answered around.
func TestRestoringMarshallerRefusesAMemberTheResponseLacks(t *testing.T) {
	t.Parallel()

	address := &linodev1.ReservedIPAddress{Address: reservedIPAddressFixture}

	_, err := tools.MarshalProtoToolResponseRestoringNulls(
		address, json.RawMessage(`{"gateway": null}`), "no_such_member",
		reservedIPNullableFields(),
	)
	if !errors.Is(err, tools.ErrNullMember) {
		t.Fatalf("err = %v, want the missing-member refusal", err)
	}
}

// A body that opens as an object but does not parse reports the decode failure
// rather than restoring against garbage.
func TestRestoringMarshallerReportsAMalformedObjectBody(t *testing.T) {
	t.Parallel()

	address := &linodev1.ReservedIPAddress{Address: reservedIPAddressFixture}

	_, err := tools.MarshalProtoToolResponseRestoringNulls(
		address, json.RawMessage(`{"bad`), "", reservedIPNullableFields(),
	)
	if !errors.Is(err, tools.ErrResponseDecode) {
		t.Fatalf("err = %v, want the decode failure", err)
	}
}

// An envelope answering without its member has nothing to restore into: an
// object body reports the absent member, and a non-object body surfaces as the
// invalid serialization it would otherwise silently produce.
func TestRestoringMarshallerReportsAnAbsentMember(t *testing.T) {
	t.Parallel()

	cases := []struct {
		want error
		name string
		raw  string
	}{
		{name: "object body", raw: `{"gateway": null}`, want: tools.ErrResponseDecode},
		{name: "array body", raw: databaseJSONArray, want: tools.ErrResponseIndent},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			response := &linodev1.OAuthClientCreateWriteResponse{Message: keyCreated}

			_, err := tools.MarshalProtoToolResponseRestoringNulls(
				response, json.RawMessage(testCase.raw), "client",
				[]string{reservedIPGatewayKey},
			)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("err = %v, want %v", err, testCase.want)
			}
		})
	}
}

// A response serializing to JSON null carries no members at all, which is a
// defect the restoration names rather than treating as an empty object.
func TestRestoringMarshallerReportsANullResponseBody(t *testing.T) {
	t.Parallel()

	_, err := tools.MarshalProtoToolResponseRestoringNulls(
		structpb.NewNullValue(), json.RawMessage(`{"gateway": null}`), "struct_value",
		[]string{reservedIPGatewayKey},
	)
	if !errors.Is(err, tools.ErrNullMember) {
		t.Fatalf("err = %v, want the null-body refusal", err)
	}
}
