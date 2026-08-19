package toolhooks_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

// protoPackage scopes the descriptor walk to this repo's contract. The global
// registry also holds descriptor.proto and whatever a dependency links in, none
// of which can carry a tool option.
const protoPackage = "linode.mcp.v1"

// emptyJSONBody is the answer a route with nothing to report sends, which
// several stubs in this package hand back.
const emptyJSONBody = "{}"

// The two values several update tests share: the field a preview names and the
// label an update replaces the current one with.
const (
	keyDescription = "description"
	labelAfter     = "after"
)

// The two arguments linode_domain_record_get takes, and the sentences the hook
// answers with when either is not a positive integer.
const (
	domainIDArg = "domain_id"
)

// kindValidate is the one hook kind any tool declares today.
const kindValidate = "validate"

// kindExecute names the hook that makes a tool's live call. No message declares
// it yet: the attachment upload that needs it migrates with its family, and a
// kind declared ahead of its implementation stops the build by design.
const kindExecute = "execute"

// TestContractNamesEveryImplementedHook covers the half of the hook contract the
// compiler cannot: a hook the proto declares and this package does not implement
// fails the build, but a function implemented here that no *Input message
// declares has no such backstop, and it reads as behavior a tool has when
// nothing calls it.
//
// Both halves are read rather than listed: the declared side walks the
// descriptors and the implemented side parses this package's own source, so no
// hand-kept list can go stale between them.
func TestContractNamesEveryImplementedHook(t *testing.T) {
	t.Parallel()

	declared := declaredHooks(t)
	implemented := exportedFunctions(t)

	for _, name := range declared {
		if !slices.Contains(implemented, name) {
			t.Errorf("the contract declares %s, which this package does not implement", name)
		}
	}

	for _, name := range implemented {
		if !slices.Contains(declared, name) {
			t.Errorf("%s is implemented here but no tool declares it", name)
		}
	}
}

// TestEveryArgumentReaderHasTwoDeclarers holds the reader enum to the cap that
// keeps it from becoming a per-tool hand-list.
//
// A member is defined by the exact sentences it answers, so nothing stops one
// being added per wording until the enum has a member per tool, which is the
// per-tool hook again in a worse place to read it. The cap: two or more fields
// must declare a member. A tool whose wording would need one of its own
// converges onto an existing member, named and fixtured, or keeps its hook.
func TestEveryArgumentReaderHasTwoDeclarers(t *testing.T) {
	t.Parallel()

	declarers := map[linodev1.ArgumentReader][]string{}

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			message := messages.Get(i)
			fields := message.Fields()

			for j := range fields.Len() {
				field := fields.Get(j)

				reader, ok := proto.GetExtension(
					field.Options(), linodev1.E_ArgumentReader,
				).(linodev1.ArgumentReader)
				if !ok || reader == linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED {
					continue
				}

				declarers[reader] = append(declarers[reader],
					string(message.Name())+"."+string(field.Name()))
			}
		}

		return true
	})

	if len(declarers) == 0 {
		t.Fatal("no field declares a reader, so this asserted nothing")
	}

	for reader, fields := range declarers {
		if len(fields) < 2 {
			t.Errorf("%s is declared only by %v; a member needs two or more declarers"+
				" or it is a per-tool wording wearing a contract term", reader, fields)
		}
	}
}

// TestNoToolDeclaresAReaderBesideItsValidateHook holds the contract to the one
// combination the two renderer arms cannot both honor.
//
// A hook owns the whole argument check, so Go writes no path reads for a tool
// declaring one and its readers are dropped, while Python still acts them. Both
// emitter refuses the pair at build; this asserts the shipped contract carries
// none, so the refusal is a guard rather than something a tool is living with.
func TestNoToolDeclaresAReaderBesideItsValidateHook(t *testing.T) {
	t.Parallel()

	var hooked int

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			message := messages.Get(i)

			kinds, _ := proto.GetExtension(message.Options(), linodev1.E_ToolHooks).([]string)
			if !slices.Contains(kinds, "validate") {
				continue
			}

			hooked++

			fields := message.Fields()
			for j := range fields.Len() {
				field := fields.Get(j)

				reader, ok := proto.GetExtension(
					field.Options(), linodev1.E_ArgumentReader,
				).(linodev1.ArgumentReader)
				if ok && reader != linodev1.ArgumentReader_ARGUMENT_READER_UNSPECIFIED {
					t.Errorf("%s declares %s on %s beside its validate hook",
						message.Name(), reader, field.Name())
				}
			}
		}

		return true
	})

	if hooked == 0 {
		t.Log("no tool declares a validate hook, so this asserted nothing to break")
	}
}

// TestFunctionNameSpellsToolAndKind pins the rule the emitter writes its calls
// through. A change here renames every hook at once, which is the point of the
// name being derived, so it is worth stating what the derivation is.
func TestFunctionNameSpellsToolAndKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tool string
		kind string
		want string
	}{
		{tool: "linode_domain_record_get", kind: kindValidate, want: "LinodeDomainRecordGetValidate"},
		{tool: "hello", kind: kindValidate, want: "HelloValidate"},
		{tool: "linode_vpc_ip_list", kind: kindValidate, want: "LinodeVPCIPListValidate"},
		// A doubled separator splits to an empty word, and capitalizing one
		// indexes past its end. No manifest name carries one, which is the
		// point: a name that never should reach here must not panic if it does.
		{tool: "linode__odd_tool", kind: kindValidate, want: "LinodeOddToolValidate"},
		{
			tool: "linode_support_ticket_attachment_create",
			kind: kindExecute,
			want: "LinodeSupportTicketAttachmentCreateExecute",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.tool, func(t *testing.T) {
			t.Parallel()

			if got := toolhooks.FunctionName(testCase.tool, testCase.kind); got != testCase.want {
				t.Errorf("FunctionName(%q, %q) = %q, want %q",
					testCase.tool, testCase.kind, got, testCase.want)
			}
		})
	}
}

// TestExecuteTakesTheCallTheHandlerWouldHaveMade pins the argument list
// cmd/toolgen writes an execute call through. The emitter substitutes the hook
// for client.CallRouteBody, so the two have to line up value for value: the
// path values in route order and the body the shared builder assembled. A
// rename or a reorder here breaks a generated file that no test in this package
// can see.
func TestExecuteTakesTheCallTheHandlerWouldHaveMade(t *testing.T) {
	t.Parallel()

	var (
		gotValues []any
		gotBody   any
	)

	var hook toolhooks.Execute = func(
		_ context.Context,
		_ *linode.Client,
		request *mcp.CallToolRequest,
		pathValues []any,
		body any,
	) error {
		gotValues = pathValues
		gotBody = body

		if request.GetInt("ticket_id", 0) != 42 {
			return errWrongTicket
		}

		return nil
	}

	request := requestWith(map[string]any{"ticket_id": 42, keyAttachmentFile: "/tmp/report.txt"})
	body := map[string]any{"file": "/tmp/report.txt"}

	if err := hook(t.Context(), nil, &request, []any{42}, body); err != nil {
		t.Fatalf("hook reported %v", err)
	}

	if len(gotValues) != 1 || gotValues[0] != 42 {
		t.Errorf("hook received path values %v, want [42]", gotValues)
	}

	if !reflect.DeepEqual(gotBody, body) {
		t.Errorf("hook received body %v, want %v", gotBody, body)
	}
}

// errWrongTicket reports an argument the hook was handed a different value for
// than the request carries.
var errWrongTicket = errors.New("hook read a ticket id the request does not carry")

// requestFor answers the request an answer hook takes: the same call
// requestWith builds, by pointer.
func requestFor(args map[string]any) *mcp.CallToolRequest {
	request := requestWith(args)

	return &request
}

func requestWith(args map[string]any) mcp.CallToolRequest {
	var request mcp.CallToolRequest

	request.Params.Arguments = args

	return request
}

// declaredHooks reads the function names the tool_hooks options resolve to.
func declaredHooks(t *testing.T) []string {
	t.Helper()

	names := make([]string, 0)

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		messages := file.Messages()
		for i := range messages.Len() {
			names = append(names, messageHooks(messages.Get(i))...)
		}

		return true
	})

	if len(names) == 0 {
		t.Fatal("no tool declares a hook, so the comparison would prove nothing")
	}

	return names
}

// messageHooks resolves one message's declared hook kinds to function names.
func messageHooks(message protoreflect.MessageDescriptor) []string {
	options := message.Options()

	tool := declaredToolName(options)
	if tool == "" {
		return nil
	}

	kinds, ok := proto.GetExtension(options, linodev1.E_ToolHooks).([]string)
	if !ok {
		return nil
	}

	names := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		names = append(names, toolhooks.FunctionName(tool, kind))
	}

	return names
}

// declaredToolName reads the tool one message belongs to out of whichever
// marker names it, "" when neither does.
func declaredToolName(options protoreflect.ProtoMessage) string {
	if route, ok := proto.GetExtension(options, linodev1.E_ToolRoute).(*linodev1.ToolRoute); ok {
		if route.GetTool() != "" {
			return route.GetTool()
		}
	}

	if meta, ok := proto.GetExtension(options, linodev1.E_ToolMeta).(*linodev1.ToolMeta); ok {
		return meta.GetTool()
	}

	return ""
}

// exportedFunctions parses this package's own source for its exported functions.
//
// FunctionName is left out: it is the naming rule the emitter and this test both
// read, not a hook a tool declares, so counting it would ask the contract to
// declare a hook for it.
func exportedFunctions(t *testing.T) []string {
	t.Helper()

	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}

	fset := token.NewFileSet()
	names := make([]string, 0, len(entries))

	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}

		names = append(names, exportedFuncNames(file)...)
	}

	return names
}

// exportedFuncNames lists the exported top-level functions one file declares,
// less the naming rule itself.
func exportedFuncNames(file *ast.File) []string {
	names := make([]string, 0)

	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || funcDecl.Recv != nil || funcDecl.Name == nil {
			continue
		}

		if funcDecl.Name.Name == "FunctionName" {
			continue
		}

		if ast.IsExported(funcDecl.Name.Name) {
			names = append(names, funcDecl.Name.Name)
		}
	}

	return names
}
