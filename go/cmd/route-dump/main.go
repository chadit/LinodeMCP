// Command route-dump AST-extracts every HTTP route the Go Linode client can
// build and prints them on stdout as JSON.
//
// Grepping for whole path literals cannot see a route assembled from a base
// constant and a format verb, such as
// fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", id), so it reports that
// route as unimplemented. Resolving the concatenation instead gives
// scripts/verify_route_evidence.py a real route surface to check the proto
// contract against.
//
// Source is read as text with go/parser and go/ast only. Importing
// internal/linode would fail the gate for the wrong reason: the genpb generated
// tree is gitignored and may be absent.
//
// Resolving zero routes exits non-zero. An empty dump is always a broken
// resolver, never a client with no routes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// dump is the JSON contract scripts/verify_route_evidence.py reads.
type dump struct {
	// Routes is the sorted "<METHOD> <path>" set the client can build, with
	// every path parameter collapsed to the {p} placeholder: the client
	// assembles paths from variables, so no parameter name is available. The
	// gate normalizes declared parameter names off before comparing.
	Routes []string `json:"routes"`
	// Contracted names call sites that take their route from the proto contract
	// instead of building a path. Nothing to resolve: the route is whatever the
	// named tool declares, and the gate reads it there.
	Contracted []contractedSite `json:"contracted"`
	// Unresolved names request call sites whose method or path the resolver
	// could not follow, as "<file>:<line> <function>: <reason>". Reporting
	// rather than dropping them means a route with no resolvable construction
	// site surfaces as missing evidence in the gate with a stated reason.
	Unresolved []string `json:"unresolved"`
}

// contractedSite is one call that takes its route from the contract instead of
// a path: by the tool it names, or by the generated *Input message it hands to
// the typed lookup. The site travels with either so the gate can name the call
// when the contract declares no such tool or message.
type contractedSite struct {
	Tool    string `json:"tool,omitempty"`
	Message string `json:"message,omitempty"`
	Site    string `json:"site"`
}

func main() {
	clientDir := flag.String("client-dir", defaultDirs,
		"comma-separated packages to scan, resolved relative to the working directory")

	flag.Parse()

	if err := run(splitDirs(*clientDir)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// defaultDirs are the places a route is reached from: the client package holds
// the request primitives, the generated tool package names its tool to those
// primitives without writing a path, and the profiles package reads the two
// routes its scope validator needs by handing generated message types to the
// typed lookup. Scanning the client alone reports every route reached from the
// other two as missing.
//
// All parse into one symbol table because a call and the primitive it reaches
// sit on opposite sides of a package boundary. A name declared in more than one
// is ambiguous and resolves to nothing, which surfaces the affected call sites
// rather than attributing a route to the wrong body.
const defaultDirs = "internal/linode,internal/gentools,internal/profiles"

// splitDirs drops empty entries so a trailing comma is not a directory named "".
func splitDirs(value string) []string {
	dirs := make([]string, 0, strings.Count(value, ",")+1)

	for entry := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			dirs = append(dirs, trimmed)
		}
	}

	return dirs
}

// run returns errors instead of exiting so main owns the sole os.Exit (revive
// deep-exit).
func run(dirs []string) error {
	pkg, err := parsePackage(dirs)
	if err != nil {
		return err
	}

	result, err := pkg.routeSurface()
	if err != nil {
		return err
	}

	// Contracted sites count as route surface: a client whose call sites have
	// all moved onto the proto contract builds no path in source, and that end
	// state must not read as a broken resolver.
	if len(result.Routes) == 0 && len(result.Contracted) == 0 {
		return fmt.Errorf("%s: %w", strings.Join(dirs, ", "), errNoRoutes)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if encErr := encoder.Encode(result); encErr != nil {
		return fmt.Errorf("encode: %w", encErr)
	}

	return nil
}
