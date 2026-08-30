package profiles_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// invalidSpelling is what a value outside the declared tag set spells, kept
// beside the tests that assert it so a reworded answer moves in one place.
const invalidSpelling = "Capability(invalid)"

// capabilitySpellingFixture mirrors testdata/profile/capability_spellings.json,
// the fixture the Python suite asserts against too.
type capabilitySpellingFixture struct {
	Spellings []struct {
		Member   string `json:"member"`
		Spelling string `json:"spelling"`
		Value    int    `json:"value"`
	} `json:"spellings"`
}

// readCapabilitySpellingFixture loads the shared spelling fixture.
func readCapabilitySpellingFixture(t *testing.T) *capabilitySpellingFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(
		"..", "..", "..", "testdata", "profile", "capability_spellings.json",
	))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}

	fixture := new(capabilitySpellingFixture)
	if err := json.Unmarshal(raw, fixture); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fixture.Spellings) == 0 {
		t.Fatal("the shared fixture names no spellings, so this test measures nothing")
	}

	return fixture
}

// capabilityTags names the tag each contract member declares the tier for.
// String reads a tier's name out of the generated enum by the tag's own number,
// so this table is what holds that numbering: without it, a constant inserted
// into the iota block would spell every later tag as its neighbor and nothing
// would say so.
func capabilityTags() map[string]profiles.Capability {
	return map[string]profiles.Capability{
		"TOOL_CAPABILITY_UNSPECIFIED": profiles.CapUnknown,
		"TOOL_CAPABILITY_READ":        profiles.CapRead,
		"TOOL_CAPABILITY_WRITE":       profiles.CapWrite,
		"TOOL_CAPABILITY_DESTROY":     profiles.CapDestroy,
		"TOOL_CAPABILITY_ADMIN":       profiles.CapAdmin,
		"TOOL_CAPABILITY_META":        profiles.CapMeta,
	}
}

// TestCapabilityStringMatchesSharedFixture holds the spelling every tag shows
// to the shared cross-language fixture.
//
// A tool answer that names a tier carries this string, and a catalog filter a
// caller types is matched against it, so the two engines have to spell one tier
// one way. Each row also pins the contract member the tag selects, which is the
// pairing that lets the spelling come off the generated enum.
func TestCapabilityStringMatchesSharedFixture(t *testing.T) {
	t.Parallel()

	fixture := readCapabilitySpellingFixture(t)
	tags := capabilityTags()

	for _, row := range fixture.Spellings {
		t.Run(row.Member, func(t *testing.T) {
			t.Parallel()

			member := linodev1.ToolCapability_name[int32(row.Value)]
			if member != row.Member {
				t.Errorf("ToolCapability_name[%d] = %q, want %q", row.Value, member, row.Member)
			}

			tag, named := tags[row.Member]
			if !named {
				t.Fatalf("no capability tag names the tier %s declares", row.Member)
			}

			if int(tag) != row.Value {
				t.Errorf("the tag for %s is %d, want %d", row.Member, int(tag), row.Value)
			}

			if got := tag.String(); got != row.Spelling {
				t.Errorf("Capability(%d).String() = %q, want %q", int(tag), got, row.Spelling)
			}
		})
	}
}

// TestSharedFixtureCoversEveryDeclaredMember fails when the contract declares a
// ToolCapability member the fixture leaves out, so a tier added to the proto
// cannot reach a tool answer with a spelling no language ever agreed on.
func TestSharedFixtureCoversEveryDeclaredMember(t *testing.T) {
	t.Parallel()

	fixture := readCapabilitySpellingFixture(t)

	pinned := make(map[int32]string, len(fixture.Spellings))
	for _, row := range fixture.Spellings {
		pinned[int32(row.Value)] = row.Spelling
	}

	for number, member := range linodev1.ToolCapability_name {
		if _, held := pinned[number]; !held {
			t.Errorf("the contract declares %s = %d and the shared fixture pins no spelling for it", member, number)
		}
	}
}

// TestCapabilityStringRefusesNumbersOutsideTheTagSet pins what a value the
// contract declares no member for shows. Error messages and the registration
// invariant report print this, so a tag read off a corrupt config has to say
// it names no tier rather than borrow a neighbor's.
func TestCapabilityStringRefusesNumbersOutsideTheTagSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		want       string
		capability profiles.Capability
	}{
		{name: "value past the last member reports invalid", capability: profiles.Capability(99), want: invalidSpelling},
		{name: "negative value reports invalid", capability: profiles.Capability(-1), want: invalidSpelling},
		// Every release target is 64-bit, so these two values are wider than the
		// int32 keys the generated name map uses. A lookup that narrows the tag
		// spells them as whatever tier their low 32 bits land on, which is the
		// neighbor Python's twin has never borrowed.
		{name: "value whose low bits land on the zero member reports invalid", capability: profiles.Capability(1 << 32), want: invalidSpelling},
		{name: "value whose low bits land on a real member reports invalid", capability: profiles.Capability(1<<32 | 1), want: invalidSpelling},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.capability.String(); got != tt.want {
				t.Errorf("Capability(%d).String() = %q, want %q", int(tt.capability), got, tt.want)
			}
		})
	}
}
