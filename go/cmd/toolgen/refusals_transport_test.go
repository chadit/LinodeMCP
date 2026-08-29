package main_test

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	toolgen "github.com/chadit/LinodeMCP/go/cmd/toolgen"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The execute_transport refusals. A half-declared transport is the quietest
// failure of the set: the emitter would render a call with a blank where a name
// belongs, and the tool would send an empty part or answer an empty member.

// The response a transported probe answers with, and the argument that fills
// its one echoed member. Both are real: a synthesized response has no Go type,
// and the read tier resolves its echo against the call.
const (
	transportResponse = "linode.mcp.v1.OAuthClientThumbnail"
	transportIDArg    = "client_id"
	transportPath     = "/probes/{client_id}"
	transportPartName = "file"
)

func TestRefusesATransportDeclarationThatSaysTooLittle(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "transport carrying no arm",
			refusal: "errNoTransportArm",
			build:   transportProbe("ProbeTransportNoArmInput", &linodev1.ExecuteTransport{}),
		},
		{
			name:    "multipart naming no form field",
			refusal: "errTransportIncomplete",
			build: transportProbe("ProbeTransportNoPartInput", multipartTransport(&linodev1.MultipartUpload{
				FileArgument: transportIDArg,
			})),
		},
		{
			name:    "presign naming no URL member",
			refusal: "errTransportIncomplete",
			build: transportProbe("ProbeTransportNoURLInput", presignTransport(&linodev1.PresignTransfer{
				Direction:         linodev1.TransferDirection_TRANSFER_DIRECTION_DOWN,
				LocalPathArgument: transportIDArg,
				OverwriteArgument: transportIDArg,
				SizeField:         "size_bytes",
				EtagField:         "etag",
			})),
		},
		{
			name:    "raw body with no direction",
			refusal: "errNoTransferDirection",
			build: transportProbe("ProbeTransportNoDirectionInput", rawBodyTransport(&linodev1.RawBody{
				ContentType: "image/png",
				AnswerField: "thumbnail_png_base64",
			})),
		},
		{
			name:    "upward raw body declaring the answer member",
			refusal: "errTransportDirectionFields",
			build: transportProbe("ProbeTransportUpAnswersInput", rawBodyTransport(&linodev1.RawBody{
				Direction:      linodev1.TransferDirection_TRANSFER_DIRECTION_UP,
				ContentType:    "image/png",
				SourceArgument: transportIDArg,
				AnswerField:    "thumbnail_png_base64",
			})),
		},
	})
}

func TestRefusesATransportThatNamesSomethingTheMessageDoesNot(t *testing.T) {
	t.Parallel()

	runRefusals(t, []refusalCase{
		{
			name:    "argument the input does not declare",
			refusal: "errTransportUnknownArgument",
			build: transportProbe("ProbeTransportUnknownArgumentInput", multipartTransport(&linodev1.MultipartUpload{
				FileArgument: probeAbsentName,
				PartName:     transportPartName,
			})),
		},
		{
			name:    "assembled member the arm never fills",
			refusal: "errTransportMembers",
			build: transportProbe("ProbeTransportUnfilledMemberInput", multipartTransport(&linodev1.MultipartUpload{
				FileArgument: transportIDArg,
				PartName:     transportPartName,
			})),
		},
	})
}

// transportProbe is a read whose answer one member is assembled into, with the
// declared transport under test. The response is what makes the case reach the
// name checks: a tool with no assembled member is refused a step earlier.
func transportProbe(
	name string, transport *linodev1.ExecuteTransport,
) func(t *testing.T) *toolgen.ProbeRun {
	return func(t *testing.T) *toolgen.ProbeRun {
		t.Helper()

		return goProbe(probeMessage(t, name,
			getOptions(
				withPath(transportPath),
				withResponse(transportResponse),
				withTransport(transport),
			),
			probeField(transportIDArg, probeFirstNumber,
				descriptorpb.FieldDescriptorProto_TYPE_STRING,
				fieldOptions(withLocation(linodev1.FieldLocation_FIELD_LOCATION_PATH)))))
	}
}

func multipartTransport(arm *linodev1.MultipartUpload) *linodev1.ExecuteTransport {
	return &linodev1.ExecuteTransport{Kind: &linodev1.ExecuteTransport_Multipart{Multipart: arm}}
}

func rawBodyTransport(arm *linodev1.RawBody) *linodev1.ExecuteTransport {
	return &linodev1.ExecuteTransport{Kind: &linodev1.ExecuteTransport_RawBody{RawBody: arm}}
}

func presignTransport(arm *linodev1.PresignTransfer) *linodev1.ExecuteTransport {
	return &linodev1.ExecuteTransport{Kind: &linodev1.ExecuteTransport_Presign{Presign: arm}}
}

func withTransport(transport *linodev1.ExecuteTransport) func(*descriptorpb.MessageOptions) {
	return func(options *descriptorpb.MessageOptions) {
		proto.SetExtension(options, linodev1.E_ExecuteTransport, transport)
	}
}
