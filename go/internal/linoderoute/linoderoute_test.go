package linoderoute_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linoderoute"
)

// capabilitiesManifestPath is the shared cross-language capability manifest.
const capabilitiesManifestPath = "../../../docs/contracts/tools-capabilities.txt"

// metaTier tags the tools the contract deliberately leaves unrouted.
const metaTier = "Meta"

// probeValue fills a slot when a test only cares that the route resolves.
const probeValue = "probe"

// unclosedTemplate is a broken path the descriptors cannot produce and the
// checks must still reject.
const unclosedTemplate = "/a/{b"

// tagDelete's path parameter is a user-chosen label, so it carries whatever a
// person typed into a path segment. unknownTool names no tool at all.
const (
	instanceDelete = "linode_instance_delete"
	tagDelete      = "linode_tag_delete"
	unknownTool    = "linode_not_a_tool"
)

const (
	methodDelete = "DELETE"
	methodGet    = "GET"
)

// The base path segments the two declarable surfaces resolve to.
const (
	segmentV4     = "v4"
	segmentV4Beta = "v4beta"
)

// Bases the surface swap leaves alone: an origin with no version segment, and
// a host that merely ends in one.
const (
	bareOriginBase = "http://127.0.0.1:8080"
	v4HostBase     = "https://v4.example.com"
)

// fakeTemplate is a route no contract declares, for the checks a generated
// contract cannot reach.
const fakeTemplate = "/fake"

// The configured apiUrl a default deployment carries, and the base the beta
// surface derives from it.
const (
	canonicalBase     = "https://api.linode.com/v4"
	canonicalBetaBase = "https://api.linode.com/v4beta"
)

// TestAllCoversEveryNonMetaTool reads the capability manifest rather than a
// count, so adding a tool moves both files or fails here.
func TestAllCoversEveryNonMetaTool(t *testing.T) {
	t.Parallel()

	want := routedTools(t)
	if len(want) == 0 {
		t.Fatal("len(want) = 0, want the capability manifest to list routed tools")
	}

	got := make([]string, 0, len(linoderoute.All()))
	for _, route := range linoderoute.All() {
		got = append(got, route.Tool)
	}

	if missing := difference(want, got); len(missing) > 0 {
		t.Errorf("tools with no declared route: %s", strings.Join(missing, ", "))
	}

	if extra := difference(got, want); len(extra) > 0 {
		t.Errorf("routes for tools the capability manifest does not list: %s", strings.Join(extra, ", "))
	}
}

// TestAllRoutesBuildAPath resolves every declared route from the real
// descriptors. Slot-versus-template agreement is checked by
// TestValidateAcceptsTheShippedContract, which runs the startup check itself.
func TestAllRoutesBuildAPath(t *testing.T) {
	t.Parallel()

	methods := []string{methodGet, "POST", "PUT", methodDelete}

	for _, route := range linoderoute.All() {
		if !slices.Contains(methods, route.Method) {
			t.Errorf("For(%q).Method = %v, want one of %v", route.Tool, route.Method, methods)
		}

		if !strings.HasPrefix(route.Template, "/") {
			t.Errorf("For(%q).Template = %v, want a rooted path", route.Tool, route.Template)
		}

		assertFills(t, &route)
	}
}

// TestForReturnsTheDeclaredRoute pins two routes by hand so a descriptor walk
// that keeps the count right but reads the wrong field still fails.
func TestForReturnsTheDeclaredRoute(t *testing.T) {
	t.Parallel()

	for _, expect := range []struct {
		tool     string
		method   string
		template string
		slots    []string
	}{
		{
			tool:     instanceDelete,
			method:   methodDelete,
			template: "/linode/instances/{instance_id}",
			slots:    []string{"instance_id"},
		},
		{
			tool:     tagDelete,
			method:   methodDelete,
			template: "/tags/{tag_label}",
			slots:    []string{"tag_label"},
		},
	} {
		got, err := linoderoute.For(expect.tool)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.Method != expect.method {
			t.Errorf("For(%q).Method = %v, want %v", expect.tool, got.Method, expect.method)
		}

		if got.Template != expect.template {
			t.Errorf("For(%q).Template = %v, want %v", expect.tool, got.Template, expect.template)
		}

		if !slices.Equal(got.Slots, expect.slots) {
			t.Errorf("For(%q).Slots = %v, want %v", expect.tool, got.Slots, expect.slots)
		}
	}
}

// TestForRejectsUnknownTool: an undeclared name resolves to nothing rather than
// to an empty route a caller would send.
func TestForRejectsUnknownTool(t *testing.T) {
	t.Parallel()

	got, err := linoderoute.For(unknownTool)
	if !errors.Is(err, linoderoute.ErrNoRoute) {
		t.Fatalf("For() error = %v, want ErrNoRoute", err)
	}

	if got.Template != "" {
		t.Errorf("For() route = %+v, want the zero route", got)
	}
}

// TestEndpointFillsSlotsInOrder pins a two-slot route, which is where an
// argument order that only looks right would show up.
func TestEndpointFillsSlotsInOrder(t *testing.T) {
	t.Parallel()

	route, err := linoderoute.For("linode_image_sharegroup_image_delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := route.Endpoint(4242, 8615)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = "/images/sharegroups/4242/images/8615"
	if got != want {
		t.Errorf("Endpoint() = %v, want %v", got, want)
	}
}

// TestEndpointRendersADeclaredStateNumber pins the json.Number form, which is
// how a walk enrichment fills its slot straight off the fetched state.
func TestEndpointRendersADeclaredStateNumber(t *testing.T) {
	t.Parallel()

	route, err := linoderoute.For("linode_image_sharegroup_image_delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := route.Endpoint(json.Number("4242"), json.Number("8615"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const want = "/images/sharegroups/4242/images/8615"
	if got != want {
		t.Errorf("Endpoint() = %v, want %v", got, want)
	}
}

// TestEndpointRejectsArityMismatch is why Endpoint returns an error at all: a
// short path still reaches the API, addressing the parent collection.
func TestEndpointRejectsArityMismatch(t *testing.T) {
	t.Parallel()

	route, err := linoderoute.For(instanceDelete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, values := range [][]any{{}, {4242, 8615}} {
		got, err := route.Endpoint(values...)
		if !errors.Is(err, linoderoute.ErrValueCount) {
			t.Errorf("Endpoint(%v) error = %v, want ErrValueCount", values, err)
		}

		if got != "" {
			t.Errorf("Endpoint(%v) = %v, want an empty path", values, got)
		}
	}
}

// TestEndpointRejectsEmptyValue covers the other way to end up addressing a
// collection: a value that renders to nothing.
func TestEndpointRejectsEmptyValue(t *testing.T) {
	t.Parallel()

	route, err := linoderoute.For(tagDelete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := route.Endpoint("")
	if !errors.Is(err, linoderoute.ErrEmptyValue) {
		t.Errorf("Endpoint(\"\") error = %v, want ErrEmptyValue", err)
	}

	if got != "" {
		t.Errorf("Endpoint(\"\") = %v, want an empty path", got)
	}
}

// TestEndpointRendersIdentifierTypes pins the rejected set to the one the
// Python builder refuses, so a value the two clients disagree about cannot
// exist. Booleans are listed because Python has to name them (there, bool is an
// int), and pinning them here keeps a Go rewrite from reaching for a fmt.Sprint
// fallback that would send "true".
func TestEndpointRendersIdentifierTypes(t *testing.T) {
	t.Parallel()

	route, err := linoderoute.For(instanceDelete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, value := range []any{4242, int32(4242), int64(4242), "4242"} {
		got, buildErr := route.Endpoint(value)
		if buildErr != nil {
			t.Fatalf("unexpected error: %v", buildErr)
		}

		const want = "/linode/instances/4242"
		if got != want {
			t.Errorf("Endpoint(%v) = %v, want %v", value, got, want)
		}
	}

	for _, value := range []any{42.5, float32(42.5), true, nil, []int{4242}} {
		got, buildErr := route.Endpoint(value)
		if !errors.Is(buildErr, linoderoute.ErrValueType) {
			t.Errorf("Endpoint(%v) error = %v, want ErrValueType", value, buildErr)
		}

		if got != "" {
			t.Errorf("Endpoint(%v) = %v, want an empty path", value, got)
		}
	}
}

// TestEndpointEscapesEachValueIntoOneSegment pins the encoding vector by vector.
// The Python builder pins the same table against the same inputs: a label that
// escapes differently in one client addresses a resource the other never
// touches. The kept set is RFC 3986 unreserved, narrower than what
// net/url.PathEscape leaves alone, so the sub-delimiter cases below are the ones
// that would drift if this ever went back to the stdlib helper.
func TestEndpointEscapesEachValueIntoOneSegment(t *testing.T) {
	t.Parallel()

	route, err := linoderoute.For(tagDelete)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const prefix = "/tags/"

	for name, probe := range map[string]struct {
		value any
		want  string
	}{
		"unreserved letters":     {value: "plain", want: "plain"},
		"unreserved mixed case":  {value: "Prod-Web_01", want: "Prod-Web_01"},
		"space":                  {value: "has space", want: "has%20space"},
		"slash":                  {value: "a/b", want: "a%2Fb"},
		"sub-delimiters":         {value: "a:b@c", want: "a:b%40c"},
		"plus and comma":         {value: "a+b,c", want: "a%2Bb%2Cc"},
		"percent":                {value: "50%", want: "50%25"},
		"non-ASCII rune":         {value: "naïve", want: "na%C3%AFve"},
		"unreserved punctuation": {value: "-._~", want: "-._~"},
		"numeric identifier":     {value: 123, want: "123"},
		"ipv6 range":             {value: "2001:db8::/64", want: "2001:db8::%2F64"},
		"embedded dots":          {value: "ubuntu22.04", want: "ubuntu22.04"},
		"single dot-segment":     {value: ".", want: "%2E"},
		"double dot-segment":     {value: "..", want: "%2E%2E"},
	} {
		got, fillErr := route.Endpoint(probe.value)
		if fillErr != nil {
			t.Errorf("Endpoint(%v) error = %v, want nil", probe.value, fillErr)

			continue
		}

		if got != prefix+probe.want {
			t.Errorf("Endpoint(%v) for %s = %v, want %v", probe.value, name, got, prefix+probe.want)
		}
	}
}

// TestIsContractErrorNamesEveryFailureHere covers what a client's retry layer
// reads to tell a request it could not build from one the network lost. A
// sentinel added here and left out of the answer fails rather than letting a
// request quietly rejoin the retry path.
func TestIsContractErrorNamesEveryFailureHere(t *testing.T) {
	t.Parallel()

	for name, probe := range map[string]struct {
		tool   string
		values []any
	}{
		"no route":    {tool: unknownTool, values: []any{4242}},
		"value count": {tool: instanceDelete},
		"empty value": {tool: tagDelete, values: []any{""}},
		"value type":  {tool: instanceDelete, values: []any{42.5}},
	} {
		_, _, _, err := linoderoute.Resolve(probe.tool, probe.values...)
		assertContractError(t, name, err)
	}

	broken := linoderoute.Route{
		Tool: "t", Method: methodGet, Template: unclosedTemplate, Slots: []string{"b"},
	}

	_, templateErr := broken.Endpoint(probeValue)
	assertContractError(t, "template", templateErr)

	_, messageErr := linoderoute.ToolOf(&linodev1.Profile{})
	assertContractError(t, "message", messageErr)

	if linoderoute.IsContractError(errors.New("connection refused")) {
		t.Error("IsContractError() for a transport failure = true, want false")
	}

	if linoderoute.IsContractError(nil) {
		t.Error("IsContractError(nil) = true, want false")
	}
}

func assertContractError(t *testing.T, name string, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("error for %s = nil, want a failure to classify", name)
	}

	if !linoderoute.IsContractError(err) {
		t.Errorf("IsContractError(%v) for %s = false, want true", err, name)
	}
}

// TestValidateContractRejectsBrokenRoutes builds routes by hand because none of
// these shapes can come out of the descriptors.
func TestValidateContractRejectsBrokenRoutes(t *testing.T) {
	t.Parallel()

	for name, route := range map[string]linoderoute.Route{
		"slots disagree with template": {
			Tool: "t", Method: methodGet, Template: "/a/{b}", Slots: []string{"c"},
		},
		"slot count disagrees": {
			Tool: "t", Method: methodGet, Template: "/a/{b}", Slots: nil,
		},
		"unclosed slot": {
			Tool: "t", Method: methodGet, Template: unclosedTemplate, Slots: []string{"b"},
		},
		"empty slot name": {
			Tool: "t", Method: methodGet, Template: "/a/{}", Slots: []string{""},
		},
		"nested slot": {
			Tool: "t", Method: methodGet, Template: "/a/{b{c}", Slots: []string{"b"},
		},
	} {
		err := linoderoute.ValidateContract(nil, []linoderoute.Route{route})
		if !errors.Is(err, linoderoute.ErrTemplate) {
			t.Errorf("ValidateContract() error for %s = %v, want ErrTemplate", name, err)
		}
	}
}

func TestResolveReturnsMethodAndFilledPath(t *testing.T) {
	t.Parallel()

	method, endpoint, _, err := linoderoute.Resolve(instanceDelete, 4242)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if method != methodDelete {
		t.Errorf("Resolve() method = %v, want %v", method, methodDelete)
	}

	const want = "/linode/instances/4242"
	if endpoint != want {
		t.Errorf("Resolve() endpoint = %v, want %v", endpoint, want)
	}
}

// TestResolveReportsEitherFailure pins every failure because the client checks
// one error for all of them.
func TestResolveReportsEitherFailure(t *testing.T) {
	t.Parallel()

	for name, probe := range map[string]struct {
		want   error
		tool   string
		values []any
	}{
		"unknown tool":   {tool: unknownTool, values: []any{4242}, want: linoderoute.ErrNoRoute},
		"wrong arity":    {tool: instanceDelete, values: nil, want: linoderoute.ErrValueCount},
		"empty value":    {tool: tagDelete, values: []any{""}, want: linoderoute.ErrEmptyValue},
		"bad value type": {tool: instanceDelete, values: []any{42.5}, want: linoderoute.ErrValueType},
	} {
		method, endpoint, _, err := linoderoute.Resolve(probe.tool, probe.values...)
		if !errors.Is(err, probe.want) {
			t.Errorf("Resolve() error for %s = %v, want %v", name, err, probe.want)
		}

		if method != "" || endpoint != "" {
			t.Errorf("Resolve() for %s = (%v, %v), want nothing to send", name, method, endpoint)
		}
	}
}

// TestEndpointRejectsAMalformedTemplate covers the parse failure on the fill
// path, which a route built by hand can still reach.
func TestEndpointRejectsAMalformedTemplate(t *testing.T) {
	t.Parallel()

	route := linoderoute.Route{
		Tool: "t", Method: methodGet, Template: unclosedTemplate, Slots: []string{"b"},
	}

	got, err := route.Endpoint("x")
	if !errors.Is(err, linoderoute.ErrTemplate) {
		t.Errorf("Endpoint() error = %v, want ErrTemplate", err)
	}

	if got != "" {
		t.Errorf("Endpoint() = %v, want an empty path", got)
	}
}

// TestValidateAcceptsTheShippedContract runs the startup path: a false positive
// here stops the server from starting at all.
func TestValidateAcceptsTheShippedContract(t *testing.T) {
	t.Parallel()

	if err := linoderoute.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

// TestSurfaceSegmentNamesEverySurface pins the segment each declared surface
// resolves to. An undeclared surface reads as the zero value, which is why it
// has to answer v4 rather than fail.
func TestSurfaceSegmentNamesEverySurface(t *testing.T) {
	t.Parallel()

	for _, probe := range []struct {
		want    string
		surface linodev1.ApiSurface
	}{
		{segmentV4, linodev1.ApiSurface_API_SURFACE_UNSPECIFIED},
		{segmentV4, linodev1.ApiSurface_API_SURFACE_V4},
		{segmentV4Beta, linodev1.ApiSurface_API_SURFACE_V4BETA},
	} {
		got, err := linoderoute.SurfaceSegment(probe.surface)
		if err != nil {
			t.Errorf("SurfaceSegment(%v) = %v, want nil", probe.surface, err)
		}

		if got != probe.want {
			t.Errorf("SurfaceSegment(%v) = %v, want %v", probe.surface, got, probe.want)
		}
	}
}

// TestSurfaceSegmentRefusesAnUnknownSurface is the fail-closed half. A surface
// this build cannot address must not fall back to v4: the route would answer
// 404, or worse, address a different resource surface and look like it worked.
// The value is handed in directly because generation refuses to emit one.
func TestSurfaceSegmentRefusesAnUnknownSurface(t *testing.T) {
	t.Parallel()

	got, err := linoderoute.SurfaceSegment(linodev1.ApiSurface(99))
	if !errors.Is(err, linoderoute.ErrAPISurface) {
		t.Errorf("SurfaceSegment(99) error = %v, want ErrAPISurface", err)
	}

	if got != "" {
		t.Errorf("SurfaceSegment(99) = %v, want an empty segment", got)
	}
}

// TestBaseForRePointsOnlyADefaultSuffixedBase is the whole override rule in one
// table. The configured apiUrl wins in every row but the first: a surface picks
// among versions of one deployment, it never picks the deployment.
func TestBaseForRePointsOnlyADefaultSuffixedBase(t *testing.T) {
	t.Parallel()

	for name, probe := range map[string]struct {
		base, segment, want string
	}{
		"canonical base": {
			canonicalBase, segmentV4Beta, canonicalBetaBase,
		},
		"proxy keeps its path prefix": {
			"https://proxy.example/linode/v4", segmentV4Beta, "https://proxy.example/linode/v4beta",
		},
		"a base already on the beta surface is left alone": {
			canonicalBetaBase, segmentV4Beta, canonicalBetaBase,
		},
		"a bare origin has no version segment to swap": {
			bareOriginBase, segmentV4Beta, bareOriginBase,
		},
		"a trailing slash is not a version segment": {
			canonicalBase + "/", segmentV4Beta, canonicalBase + "/",
		},
		"a base whose host merely ends in v4": {
			v4HostBase, segmentV4Beta, v4HostBase,
		},
		"the default surface is a no-op": {
			canonicalBase, segmentV4, canonicalBase,
		},
	} {
		if got := linoderoute.BaseFor(probe.base, probe.segment); got != probe.want {
			t.Errorf("%s: BaseFor(%q, %q) = %v, want %v",
				name, probe.base, probe.segment, got, probe.want)
		}
	}
}

// assertFills builds one route's path and checks nothing is left unsubstituted.
func assertFills(t *testing.T, route *linoderoute.Route) {
	t.Helper()

	values := make([]any, len(route.Slots))
	for i := range values {
		values[i] = probeValue
	}

	got, err := route.Endpoint(values...)
	if err != nil {
		t.Errorf("For(%q).Endpoint() = %v, want nil", route.Tool, err)

		return
	}

	if strings.ContainsAny(got, "{}") {
		t.Errorf("For(%q).Endpoint() = %v, want no slot left in it", route.Tool, got)
	}
}

// routedTools names the non-Meta manifest tools, the set the contract must route.
func routedTools(t *testing.T) []string {
	t.Helper()

	tools := make([]string, 0)

	for name, tier := range manifestTools(t) {
		if tier != metaTier {
			tools = append(tools, name)
		}
	}

	return tools
}

// manifestTools reads the capability manifest into tool name to tier, the
// cross-language record of the surface the other language also ships.
func manifestTools(t *testing.T) map[string]string {
	t.Helper()

	file, err := os.Open(filepath.Clean(capabilitiesManifestPath))
	if err != nil {
		t.Fatalf("open capability manifest: %v", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close capability manifest: %v", closeErr)
		}
	}()

	tools := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		if name, tier := manifestEntry(t, scanner.Text()); name != "" {
			tools[name] = tier
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		t.Fatalf("read capability manifest: %v", scanErr)
	}

	if len(tools) == 0 {
		t.Fatal("len(manifestTools()) = 0, want the manifest to list tools")
	}

	return tools
}

// manifestEntry splits one manifest line, returning empty names for the lines
// that carry no entry.
func manifestEntry(t *testing.T, line string) (string, string) {
	t.Helper()

	if trimmed := strings.TrimSpace(line); trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", ""
	}

	name, tier, found := strings.Cut(line, "\t")
	if !found {
		t.Fatalf("capability manifest line %q is not <tool>\\t<Capability>", line)
	}

	return strings.TrimSpace(name), strings.TrimSpace(tier)
}

// difference returns the entries of left that right does not carry.
func difference(left, right []string) []string {
	held := make(map[string]struct{}, len(right))
	for _, entry := range right {
		held[entry] = struct{}{}
	}

	missing := make([]string, 0)

	for _, entry := range left {
		if _, ok := held[entry]; !ok {
			missing = append(missing, entry)
		}
	}

	slices.Sort(missing)

	return missing
}

// TestTargetRefusesASurfaceItCannotAddress reaches the refusal a generated
// contract cannot: the gates keep an unrenderable surface out of the shipped
// descriptors, so the route is built here instead. Nothing may be returned
// beside the error, since a caller that ignored it would send the request to
// whichever base the empty segment produced.
func TestTargetRefusesASurfaceItCannotAddress(t *testing.T) {
	t.Parallel()

	route := linoderoute.Route{
		Tool:     "linode_fake_get",
		Method:   methodGet,
		Template: "/fake/{fake_id}",
		Slots:    []string{"fake_id"},
		Surface:  linodev1.ApiSurface(99),
	}

	method, endpoint, segment, err := route.Target(probeValue)
	if !errors.Is(err, linoderoute.ErrAPISurface) {
		t.Fatalf("Target() error = %v, want ErrAPISurface", err)
	}

	if method != "" || endpoint != "" || segment != "" {
		t.Errorf("Target() = (%q, %q, %q), want all empty", method, endpoint, segment)
	}
}

// TestTargetAnswersTheSegmentTheSurfaceNames is the same path succeeding, so a
// route on the beta surface is proven to carry its segment out to the client
// before any tool declares one.
func TestTargetAnswersTheSegmentTheSurfaceNames(t *testing.T) {
	t.Parallel()

	route := linoderoute.Route{
		Tool:     "linode_fake_list",
		Method:   methodGet,
		Template: fakeTemplate,
		Surface:  linodev1.ApiSurface_API_SURFACE_V4BETA,
	}

	method, endpoint, segment, err := route.Target()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if method != methodGet || endpoint != fakeTemplate || segment != segmentV4Beta {
		t.Errorf("Target() = (%q, %q, %q), want (%q, %q, %q)",
			method, endpoint, segment, methodGet, fakeTemplate, segmentV4Beta)
	}
}

// TestValidateContractRejectsARouteOnAnUnaddressableSurface is the startup half
// of the same refusal: a contract carrying one has to stop the server rather
// than wait to surface as one tool's failed request.
func TestValidateContractRejectsARouteOnAnUnaddressableSurface(t *testing.T) {
	t.Parallel()

	route := linoderoute.Route{
		Tool:     "linode_fake_list",
		Method:   methodGet,
		Template: fakeTemplate,
		Surface:  linodev1.ApiSurface(99),
	}

	err := linoderoute.ValidateContract(nil, []linoderoute.Route{route})
	if !errors.Is(err, linoderoute.ErrAPISurface) {
		t.Errorf("ValidateContract() error = %v, want ErrAPISurface", err)
	}
}

// SurfacedTools is the contract's own answer to what is on another surface, so
// it has to name what the proto annotates rather than a list kept beside it.
func TestSurfacedToolsNamesTheAnnotatedTools(t *testing.T) {
	t.Parallel()

	surfaced := linoderoute.SurfacedTools()
	if len(surfaced) == 0 {
		t.Fatal("SurfacedTools() is empty, want the annotated tools")
	}

	if !slices.Contains(surfaced, "linode_lock_list") {
		t.Errorf("SurfacedTools() = %v, want it to name linode_lock_list", surfaced)
	}

	if slices.Contains(surfaced, tagDelete) {
		t.Errorf("SurfacedTools() = %v, want no unannotated tool in it", surfaced)
	}
}

// Repointable is what a startup check asks before deciding a configured base
// leaves the annotated tools unreachable.
func TestRepointableAcceptsOnlyADefaultSuffixedBase(t *testing.T) {
	t.Parallel()

	for base, want := range map[string]bool{
		canonicalBase:                     true,
		"https://proxy.example/linode/v4": true,
		canonicalBetaBase:                 false,
		bareOriginBase:                    false,
		canonicalBase + "/":               false,
		v4HostBase:                        false,
	} {
		if got := linoderoute.Repointable(base); got != want {
			t.Errorf("Repointable(%q) = %v, want %v", base, got, want)
		}
	}
}

// TestToolOfNamesTheDeclaredTool pins the two reads the scope validator makes
// through the lookup rather than through a spelled tool name: each input
// message answers the tool its tool_route option carries.
func TestToolOfNamesTheDeclaredTool(t *testing.T) {
	t.Parallel()

	for _, expect := range []struct {
		input proto.Message
		tool  string
	}{
		{input: &linodev1.ProfileGetInput{}, tool: "linode_profile_get"},
		{input: &linodev1.ProfileGrantsGetInput{}, tool: "linode_profile_grants_get"},
		{input: &linodev1.TagDeleteInput{}, tool: tagDelete},
	} {
		got, err := linoderoute.ToolOf(expect.input)
		if err != nil {
			t.Fatalf("ToolOf(%T) error = %v", expect.input, err)
		}

		if got != expect.tool {
			t.Errorf("ToolOf(%T) = %q, want %q", expect.input, got, expect.tool)
		}
	}
}

// TestToolOfRefusesAMessageWithNoRoute: a response type carries no tool_route,
// and the refusal names it so the caller hears which type was handed over.
func TestToolOfRefusesAMessageWithNoRoute(t *testing.T) {
	t.Parallel()

	got, err := linoderoute.ToolOf(&linodev1.Profile{})
	if !errors.Is(err, linoderoute.ErrNoToolRoute) {
		t.Fatalf("ToolOf(Profile) error = %v, want ErrNoToolRoute", err)
	}

	if want := "message declares no tool_route: linode.mcp.v1.Profile"; err.Error() != want {
		t.Errorf("ToolOf(Profile) error = %q, want %q", err, want)
	}

	if got != "" {
		t.Errorf("ToolOf(Profile) = %q, want no tool", got)
	}
}
