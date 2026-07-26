// Command route-dump AST-extracts every HTTP route the Go Linode client can
// build and prints them on stdout as JSON.
//
// A catalog scan that greps for whole path literals cannot see a route the
// client assembles from a base constant and a format verb, which is what
// fmt.Sprintf(endpointInstanceDeep+"/%s/interfaces", id) is. The scan then
// reports the route as unimplemented in Go while the client has had it all
// along, and each false positive costs a full investigation to disprove. This
// resolves the concatenation instead of grepping for it, so
// scripts/verify_route_evidence.py can check docs/contracts/tool-routes.txt
// against a real route surface rather than against whatever a text search
// happened to match.
//
// The tool reads .go source as text only with go/parser and go/ast. It never
// imports internal/linode or builds the package: the genpb generated tree is
// gitignored and may be absent, so a real import would fail the gate for the
// wrong reason. Zero third-party dependencies.
//
// Hard-fail contract: resolving zero routes exits non-zero and names the
// directory. An empty dump is always a broken resolver, never a client with no
// routes, and it must not reach the gate as data.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// dump is the JSON contract scripts/verify_route_evidence.py reads.
type dump struct {
	// Routes is the sorted "<METHOD> <path>" set the client can build, with
	// every path parameter collapsed to the {p} placeholder tool-routes.txt
	// uses.
	Routes []string `json:"routes"`
	// Unresolved names the request call sites whose method or path the
	// resolver could not follow, as "<file>:<line> <function>: <reason>". They
	// are reported rather than dropped: a route whose only construction site
	// lands here surfaces as missing evidence in the gate, and this is the
	// list that says why.
	Unresolved []string `json:"unresolved"`
}

func main() {
	clientDir := flag.String("client-dir", "internal/linode",
		"path to the Linode client package, resolved relative to the working directory")

	flag.Parse()

	if err := run(*clientDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses the client package, resolves its route surface, and prints it. It
// returns any error so main owns the sole os.Exit and flag.Parse (revive
// deep-exit).
func run(clientDir string) error {
	pkg, err := parsePackage(clientDir)
	if err != nil {
		return err
	}

	result, err := pkg.routeSurface()
	if err != nil {
		return err
	}

	if len(result.Routes) == 0 {
		return fmt.Errorf("%s: %w", clientDir, errNoRoutes)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")

	if encErr := encoder.Encode(result); encErr != nil {
		return fmt.Errorf("encode: %w", encErr)
	}

	return nil
}
