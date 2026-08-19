package tools_test

import (
	"net/url"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The page controls a route publishes, the two forwarded arguments the surface
// sends today, and one object prefix to send them under.
const (
	wantFirstPage      = "page=1&page_size=25"
	wantPageSizeUp     = "page=2&page_size=100"
	keySkipIPv6RDNS    = "skip_ipv6_rdns"
	keyObjectMarker    = "marker"
	objectPrefixImages = "images/"
)

// TestStandardPageQueryEncodesTheBoundsItReads covers the read tier's half of
// the page controls: a single-resource route that publishes them sends the same
// pair a collection does, so the two tiers cannot spell one page differently.
func TestStandardPageQueryEncodesTheBoundsItReads(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args    map[string]any
		name    string
		want    string
		message bool
	}{
		{
			// A caller who asked for no page gets none, which leaves the
			// route's own default in force rather than the reader's.
			name: "no controls when the caller asks for nothing",
			args: map[string]any{},
			want: "",
		},
		{
			name: "the page the caller asked for",
			args: map[string]any{keyPage: 2, keyPageSize: 100},
			want: wantPageSizeUp,
		},
		{
			name:    "a page size under the range is refused",
			args:    map[string]any{keyPageSize: 1},
			message: true,
		},
		{
			name:    "a page under one is refused",
			args:    map[string]any{keyPage: 0},
			message: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := createRequestWithArgs(t, testCase.args)

			query, message := tools.StandardPageQuery(&request)

			if testCase.message {
				if message == "" {
					t.Fatalf("StandardPageQuery(%v) reported nothing, want a range failure", testCase.args)
				}

				if query != "" {
					t.Errorf("query = %q, want it empty beside a failure", query)
				}

				return
			}

			if message != "" {
				t.Fatalf("StandardPageQuery(%v) = %q, want no failure", testCase.args, message)
			}

			if query != testCase.want {
				t.Errorf("query = %q, want %q", query, testCase.want)
			}
		})
	}
}

// TestWithQueryArgumentsSendsOnlyWhatTheCallerSupplied pins the rule that keeps
// a forwarded parameter honest: these are parameters the ROUTE filters on, so
// an absent one must not arrive as an empty value. An empty prefix and no
// prefix select different objects.
func TestWithQueryArgumentsSendsOnlyWhatTheCallerSupplied(t *testing.T) {
	t.Parallel()

	cases := []struct {
		args  map[string]any
		name  string
		query string
		want  string
	}{
		{
			name: "an absent argument is left out",
			args: map[string]any{},
			want: "",
		},
		{
			name: "an empty string is left out",
			args: map[string]any{keyReservedIPPrefix: ""},
			want: "",
		},
		{
			name: "text travels as it arrived",
			args: map[string]any{keyReservedIPPrefix: objectPrefixImages},
			want: keyReservedIPPrefix + "=images%2F",
		},
		{
			name: "a true flag travels",
			args: map[string]any{keySkipIPv6RDNS: true},
			want: keySkipIPv6RDNS + "=true",
		},
		{
			name: "a false flag does not",
			args: map[string]any{keySkipIPv6RDNS: false},
			want: "",
		},
		{
			name: "a number travels without a decimal tail",
			args: map[string]any{keyObjectMarker: float64(25)},
			want: keyObjectMarker + "=25",
		},
		{
			name: "an argument of an unreadable type is left out",
			args: map[string]any{keyReservedIPPrefix: []any{"a"}},
			want: "",
		},
		{
			name:  "an existing query keeps its own entries",
			args:  map[string]any{keyReservedIPPrefix: "logs/"},
			query: wantFirstPage,
			want:  wantFirstPage + "&" + keyReservedIPPrefix + "=logs%2F",
		},
		{
			name:  "an existing query survives an argument that does not travel",
			args:  map[string]any{},
			query: wantFirstPage,
			want:  wantFirstPage,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := createRequestWithArgs(t, testCase.args)

			got := tools.WithQueryArguments(&request, testCase.query,
				keyReservedIPPrefix, keySkipIPv6RDNS, keyObjectMarker)
			if got != testCase.want {
				t.Errorf("WithQueryArguments = %q, want %q", got, testCase.want)
			}

			if _, err := url.ParseQuery(got); err != nil {
				t.Errorf("WithQueryArguments produced %q, which does not parse: %v", got, err)
			}
		})
	}
}

// TestFilterByMemberMatchesOneEntryOfTheList covers the comparison a repeated
// field needs and equality cannot express: a region publishes many capabilities
// and a caller asks about one.
func TestFilterByMemberMatchesOneEntryOfTheList(t *testing.T) {
	t.Parallel()

	type region struct {
		id           string
		capabilities []string
	}

	items := []region{
		{id: regionUSEast, capabilities: []string{serviceLinodes, serviceBlockStorage, "GPU " + serviceLinodes}},
		{id: regionEUWest, capabilities: []string{serviceLinodes}},
		{id: "ap-south", capabilities: nil},
	}

	capabilitiesOf := func(item region) []string { return item.capabilities }

	cases := []struct {
		name   string
		wanted string
		want   []string
	}{
		{name: "one region carries it", wanted: "GPU " + serviceLinodes, want: []string{regionUSEast}},
		{name: "case is ignored", wanted: "block storage", want: []string{regionUSEast}},
		{name: "two regions carry it", wanted: serviceLinodes, want: []string{regionUSEast, regionEUWest}},
		{name: "nothing carries it", wanted: "Managed Databases", want: []string{}},
		{name: "a partial value matches nothing", wanted: "GPU", want: []string{}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			kept := tools.FilterByMember(items, testCase.wanted, capabilitiesOf)

			ids := make([]string, 0, len(kept))
			for _, item := range kept {
				ids = append(ids, item.id)
			}

			if len(ids) != len(testCase.want) {
				t.Fatalf("FilterByMember(%q) kept %v, want %v", testCase.wanted, ids, testCase.want)
			}

			for i, id := range ids {
				if id != testCase.want[i] {
					t.Errorf("FilterByMember(%q) kept %v, want %v", testCase.wanted, ids, testCase.want)

					break
				}
			}
		})
	}
}

// TestNewMemberFilterAppliesTheMemberMatch pins the constructor the emitter
// names for a list-valued element field, since the emitted call site is the
// only place it appears.
func TestNewMemberFilterAppliesTheMemberMatch(t *testing.T) {
	t.Parallel()

	filter := tools.NewMemberFilter("capability", "Filter regions by capability.",
		func(item *linodev1.Region) []string { return item.GetCapabilities() })

	if filter.Param != "capability" {
		t.Errorf("filter.Param = %q, want %q", filter.Param, "capability")
	}

	items := []*linodev1.Region{
		{Id: regionUSEast, Capabilities: []string{serviceLinodes}},
		{Id: regionEUWest, Capabilities: []string{serviceBlockStorage}},
	}

	kept := filter.Match(items, "linodes")
	if len(kept) != 1 || kept[0].GetId() != regionUSEast {
		t.Errorf("filter.Match kept %v, want just %v", kept, regionUSEast)
	}
}

// A preview reports the route the live branch would call. Reporting the bare
// path over a call that carries page controls would describe a different
// request, which is the one thing a dry run cannot afford to do.
func TestPathWithQueryReportsTheRouteTheCallCarries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{name: "no controls supplied", query: "", want: "/linode/instances/5/firewalls"},
		{name: "controls supplied", query: "page=2", want: "/linode/instances/5/firewalls?page=2"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := tools.PathWithQuery("/linode/instances/5/firewalls", testCase.query)
			if got != testCase.want {
				t.Errorf("path = %q, want %q", got, testCase.want)
			}
		})
	}
}
