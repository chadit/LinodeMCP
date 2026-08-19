package linoderoute_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// Which JSON shape a list route answers with is invisible from the response
// message, so it is declared. Reading it back by name here is what makes a tool
// that loses its annotation fail as a named difference rather than as a
// populated collection quietly answered as an empty one.
func TestListEnvelopeForReadsEveryDeclaredShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tool   string
		member string
		want   linoderoute.ListShape
	}{
		{
			tool:   "linode_instance_interface_list",
			member: "interfaces",
			want:   linoderoute.ListShapeKeyed,
		},
		{
			tool:   "linode_profile_security_question_list",
			member: "security_questions",
			want:   linoderoute.ListShapeKeyed,
		},
		{tool: "linode_instance_config_interface_list", want: linoderoute.ListShapeBare},
		{tool: "linode_region_availability_get", want: linoderoute.ListShapeBare},
		{tool: "linode_profile_device_list", want: linoderoute.ListShapeRequiredData},
		// Undeclared, which is the standard page nearly every collection sends.
		{tool: "linode_domain_list", want: linoderoute.ListShapeData},
		// An unknown tool answers the standard page too: the route resolution
		// that follows fails on the same name, so no request goes out on the
		// strength of this answer.
		{tool: "no_such_tool", want: linoderoute.ListShapeData},
	}

	for _, test := range tests {
		t.Run(test.tool, func(t *testing.T) {
			t.Parallel()

			got := linoderoute.ListEnvelopeFor(test.tool)

			if got.Shape != test.want {
				t.Errorf("got.Shape = %v, want %v", got.Shape, test.want)
			}

			if got.Member != test.member {
				t.Errorf("got.Member = %v, want %v", got.Member, test.member)
			}
		})
	}
}

// A member belongs to the keyed shape alone, so every other shape leaves it
// empty rather than carrying a name nothing reads.
func TestListEnvelopeForNamesAMemberOnlyWhenTheShapeUsesOne(t *testing.T) {
	t.Parallel()

	for _, tool := range linoderoute.Tools() {
		envelope := linoderoute.ListEnvelopeFor(tool.Name)

		if envelope.Shape == linoderoute.ListShapeKeyed && envelope.Member == "" {
			t.Errorf("%s declares a keyed envelope with no member", tool.Name)
		}

		if envelope.Shape != linoderoute.ListShapeKeyed && envelope.Member != "" {
			t.Errorf("%s declares member %q under a shape that reads no member",
				tool.Name, envelope.Member)
		}
	}
}
