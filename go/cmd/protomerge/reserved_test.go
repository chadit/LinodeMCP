package main_test

import (
	"strings"
	"testing"
)

// The workspace file, and the one proto path every case writes to.
const sampleRel = "proto/sample.proto"

// Every reserved form, beside a field whose own name is the keyword. ip.proto
// declares `bool reserved = 10;`, so the arm keys off the opening word.
const reservedProto = `syntax = "proto3";

package sample;

message Sample {
  reserved 3, 5 to 8;
  reserved "old_label";

  string label = 1;
  bool reserved = 10;
}
`

// Retirement travels as overlay text: upstream has no word for a spent number.
func TestReservedStatementRoundTrips(t *testing.T) {
	t.Parallel()

	output, code := runGate(t, treeWith(t, reservedProto))
	if code != 0 {
		t.Fatalf("a reserved statement does not round-trip (exit %d):\n%s", code, output)
	}

	if !strings.Contains(output, roundTripLine+"1 ") {
		t.Fatalf("expected a one-file scan, got:\n%s", output)
	}
}

// The statement arm must not widen into a catch-all: a field beside a reserved
// statement still goes through the declaration model.
func TestDeclarationBesideAReservedStatementIsStillRefused(t *testing.T) {
	t.Parallel()

	root := treeWith(t, reservedProto)
	plant(t, root, "  bool reserved = 10;", "  bool  reserved = 10;")

	output, code := runGate(t, root)
	if code == 0 {
		t.Fatalf("an unmodelled declaration passed beside a reserved statement:\n%s", output)
	}

	if !strings.Contains(output, "declaration does not parse") ||
		!strings.Contains(output, sampleRel+":10") {
		t.Fatalf("the refusal does not name the declaration:\n%s", output)
	}
}
