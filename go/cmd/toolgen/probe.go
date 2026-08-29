package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The seams this command's own tests drive it through.
//
// Every refusal in errors.go answers a declaration, and this repo's contract
// declares none of them: the proto is the surface that ships, so a message
// shaped to trip a check cannot live there. The tests synthesize those
// declarations instead and run them through here.
//
// Exported because a main package is importable only from the external test
// package beside it, so nothing outside this command can reach these. That is
// what makes them narrower than the alternative: a flag on the binary would be
// real surface added for one caller that never ships.

// ProbeRun is one synthesized declaration, run the way `make proto` runs the
// real ones: the contract build, then a renderer arm per language.
type ProbeRun struct {
	// Message is the synthesized input message, carrying the tool's whole
	// declaration the same way every *Input message in the contract does.
	Message protoreflect.MessageDescriptor
	// Beside is the rest of the surface this run sees, which is where a
	// removal finds its sibling state read. Usually empty.
	Beside map[string]protoreflect.MessageDescriptor
	// Name is the tool the message declares, which the route or the meta
	// marker on it names.
	Name string
	// Schemas is the directory the argument descriptions are read from. A
	// probe usually points it nowhere, since a missing schema is not an error
	// and an undescribed argument renders fine.
	Schemas string
	// PyOut and Ruff turn the Python arm on. It renders through the repo's
	// formatter, so a probe whose subject is a contract or Go refusal leaves
	// them empty and runs the Go arm alone.
	PyOut string
	Ruff  string
}

// Emit answers the files the declaration renders to, keyed by language and file
// name, or the refusal it raised on the way there.
func (p *ProbeRun) Emit() (map[string]string, error) {
	declared := map[string]protoreflect.MessageDescriptor{p.Name: p.Message}
	maps.Copy(declared, p.Beside)

	built, err := buildContract(p.Name, p.Message, newSchemaDocs(p.Schemas))
	if err != nil {
		return nil, err
	}

	if readErr := built.resolveStateRead(declared); readErr != nil {
		return nil, readErr
	}

	if walkErr := built.resolveDependencyWalks(declared); walkErr != nil {
		return nil, walkErr
	}

	if proseErr := built.resolvePreviewState(); proseErr != nil {
		return nil, proseErr
	}

	operations, err := probeGeneratedOperations(&built)
	if err != nil {
		return nil, err
	}

	return p.render([]contract{built}, operations)
}

// probeGeneratedOperations is the generated set one probe run sees, which is
// the operation the tool names and nothing else.
func probeGeneratedOperations(built *contract) ([]localOperation, error) {
	if built.LocalAnswer == nil {
		return nil, nil
	}

	operation := localOperation{
		arm: built.LocalAnswer.Arm, call: built.LocalAnswer.Call,
	}
	if err := checkGeneratedOperation(&operation); err != nil {
		return nil, err
	}

	return []localOperation{operation}, nil
}

// render runs the built contract through each arm this probe turned on.
func (p *ProbeRun) render(
	contracts []contract, operations []localOperation,
) (map[string]string, error) {
	arms := []renderer{goRenderer{}}
	if p.Ruff != "" {
		arms = append(arms, pyRenderer{outDir: p.PyOut, ruff: p.Ruff})
	}

	files := make(map[string]string, len(arms)*2)

	for _, arm := range arms {
		rendered, err := renderTree(arm, contracts, operations)
		if err != nil {
			return nil, err
		}

		for name, text := range rendered {
			files[arm.language()+"/"+name] = text
		}
	}

	return files, nil
}

// ProbeClaim is one claim a probe arm makes about one contract option, spelled
// in plain strings so a test can state what no real arm would.
type ProbeClaim struct {
	Option  string
	Home    string
	Emitted bool
}

// ProbeCoverage holds one probe arm's claims to the options the contract
// declares, which is the check every real arm passes on every run.
func ProbeCoverage(language string, claims []ProbeClaim) error {
	options, err := contractOptions()
	if err != nil {
		return err
	}

	stated := make([]optionClaim, 0, len(claims))
	for _, claim := range claims {
		stated = append(stated, optionClaim{
			option:  protoreflect.Name(claim.Option),
			home:    claim.Home,
			emitted: claim.Emitted,
		})
	}

	return checkArmCoverage(language, stated, options)
}

// ProbeArmClaims is what one registered language answers for, so a test can
// hold each claim to the tree the arm renders and to the file it names.
func ProbeArmClaims(language string) ([]ProbeClaim, error) {
	arm, err := armFor(language, &runPaths{
		goOut: ".", pyOut: ".", goAnswers: ".", pyAnswers: ".", ruff: ".",
	})
	if err != nil {
		return nil, err
	}

	claims := make([]ProbeClaim, 0, len(arm.lang.acts()))
	for _, claim := range arm.lang.acts() {
		claims = append(claims, ProbeClaim{
			Option:  string(claim.option),
			Home:    claim.home,
			Emitted: claim.emitted,
		})
	}

	return claims, nil
}

// ProbeNormalizeCalls is the support-layer function each declared transform
// renders to, keyed by the enum member it belongs to and holding the Go name
// beside the Python one. A test holds this to the enum the contract declares,
// which is what keeps a member from shipping with an arm missing.
func ProbeNormalizeCalls() map[string][]string {
	rendering := normalizeRendering()

	calls := make(map[string][]string, len(rendering))
	for transform, call := range rendering {
		calls[transform.String()] = []string{call.Go, call.Python}
	}

	return calls
}

// ProbeLocalArm is one synthesized engine operation, spelled in plain values so
// a test can state a declaration no member carries: an input outside the closed
// reader set, an input declared twice or unnamed, an ambient reading nothing
// renders, or an operation shaping no answer.
//
// It states no function name. Each language derives its own from the member, so
// the naming refusals are reached through ProbeLocalArmNames instead.
type ProbeLocalArm struct {
	Call string
	// Shapes is the message the operation fills on each side, keyed by the
	// direction's own member name. Left empty it states an operation that
	// shapes no answer at all. Generated messages rather than names, so a case
	// perturbing the table reaches the table check rather than the resolution
	// ProbeLocalArmBodies covers.
	Shapes map[string]proto.Message
	// Ambient is the readings the operation declares, each the member's own
	// name. A name outside the vocabulary states the reading nothing renders.
	Ambient []string
	// Reports is the conditions the operation declares, each the guard
	// member's own name, which is how a case states one a reader answers.
	Reports []string
	Inputs  []ProbeLocalInput
	// Verdicts is the per-entry vocabulary the operation words, each row a
	// verdict member's own name and the three words the contract declares for
	// it. Left empty it states an operation that answers about the call rather
	// than about each entry, which is what every shipped operation but one is.
	Verdicts []ProbeLocalVerdict
	Missing  bool
	TwoSided bool
}

// ProbeLocalVerdict is one wording row of a synthesized operation. Verdict is
// the vocabulary's own member name, and a name outside it states the row that
// words no verdict.
type ProbeLocalVerdict struct {
	Verdict string
	Bucket  string
	Reason  string
	Remedy  string
}

// ProbeLocalInput is one input of a synthesized operation. Kind is the reader
// set's own spelling, and a name outside it is what the table check refuses.
type ProbeLocalInput struct {
	Name     string
	Kind     string
	Optional bool
}

// ProbeLocalArms holds a synthesized engine table to the same check every run
// passes, which is the only way to reach a table refusal: the real one is read
// off the contract's own members and a probe cannot rewrite those.
func ProbeLocalArms(arms []ProbeLocalArm, tools []string) error {
	table, err := localArms()
	if err != nil {
		return err
	}

	for index := range arms {
		stated := &arms[index]

		call, known := linodev1.LocalCall_value[stated.Call]
		if !known {
			return fmt.Errorf("%w: %s", errUnservedLocalCall, stated.Call)
		}

		if stated.Missing {
			delete(table, linodev1.LocalCall(call))

			continue
		}

		table[linodev1.LocalCall(call)] = probeLocalArm(stated)
	}

	return checkLocalArms(table, slices.Sorted(maps.Keys(localArmNamings())), tools)
}

// ProbeLocalArmNames runs the whole naming pass over stated members, the ones
// among them that run two ways, and a stated language set. It is the only way
// to reach the naming refusals: the vocabulary is a proto enum and the spelling
// rules are one literal, so a case can add neither a member nor a language to
// the real ones.
func ProbeLocalArmNames(members, sided, languages, tools []string) error {
	return checkLocalArmNames(members, sided, languages, tools)
}

// ProbeSubsystemType is the type one operation's subsystem satisfies, which is
// how a case pins the derivation to the types the engines implement rather than
// to the derivation's own arithmetic.
func ProbeSubsystemType(member string) string {
	return localSubsystemType(member)
}

// ProbeSubsystemMethod is the method one language's subsystem serves one side
// of a stated operation through, pinned the same way and for the same reason.
// The direction is the member's own name, left empty on a one-sided operation.
func ProbeSubsystemMethod(language, member, direction string) string {
	return localSubsystemMethod(language, member, probeLocalDirection(direction))
}

// probeLocalDirection is the direction a stated member names, unset where a
// case states none.
func probeLocalDirection(member string) linodev1.LocalDirection {
	return linodev1.LocalDirection(linodev1.LocalDirection_value[member])
}

// ProbeGuardSentinel is the name one language's failure vocabulary carries a
// stated condition under.
func ProbeGuardSentinel(language, member string) string {
	return localGuardSentinel(language, linodev1.LocalGuard(linodev1.LocalGuard_value[member]))
}

// ProbeGuardCause is the neutral text one stated condition's own value carries,
// which both languages answer so a sentence naming the cause reads alike.
func ProbeGuardCause(member string) string {
	return localGuardCause(linodev1.LocalGuard(linodev1.LocalGuard_value[member]))
}

// ProbeLocalArmSpelling is the function one language derives for one side of a
// stated member, which is how a case pins the derivation to the names the
// engines define rather than to the derivation's own arithmetic.
func ProbeLocalArmSpelling(language, member, direction string) string {
	return localArmSpelling(language, member, probeLocalDirection(direction))
}

// probeLocalArm turns one stated entry into the table's own shape.
func probeLocalArm(stated *ProbeLocalArm) localArm {
	inputs := make([]*linodev1.LocalOperationInput, 0, len(stated.Inputs))
	for _, input := range stated.Inputs {
		inputs = append(inputs, &linodev1.LocalOperationInput{
			Name:     input.Name,
			Kind:     probeLocalKind(input.Kind),
			Optional: input.Optional,
		})
	}

	ambient := make([]linodev1.LocalAmbient, 0, len(stated.Ambient))
	for _, reading := range stated.Ambient {
		ambient = append(ambient, probeLocalReading(reading))
	}

	reports := make([]linodev1.LocalGuard, 0, len(stated.Reports))
	for _, guard := range stated.Reports {
		reports = append(reports, probeLocalGuard(guard))
	}

	verdicts := make([]*linodev1.LocalVerdictWording, 0, len(stated.Verdicts))
	for _, row := range stated.Verdicts {
		verdicts = append(verdicts, &linodev1.LocalVerdictWording{
			Verdict: linodev1.LocalVerdict(linodev1.LocalVerdict_value[row.Verdict]),
			Bucket:  row.Bucket,
			Reason:  row.Reason,
			Remedy:  row.Remedy,
		})
	}

	return localArm{
		declared: &linodev1.LocalOperation{
			Ambient:  ambient,
			Reports:  reports,
			TwoSided: stated.TwoSided,
			Input:    inputs,
			Verdict:  verdicts,
		},
		bodies: probeLocalShapes(stated.Shapes),
	}
}

// ProbeOperationInputUntyped runs one language's input typing over a table with
// one kind's row taken out, which is the only way to reach the refusal it
// raises: every kind the reader set carries has a row in both tables.
func ProbeOperationInputUntyped(language, kind string) error {
	operation := localOperation{
		call: linodev1.LocalCall_LOCAL_CALL_CATALOG_CAN_RUN,
		arm: probeLocalArm(&ProbeLocalArm{
			Inputs: []ProbeLocalInput{{Name: "calls", Kind: kind}},
		}),
	}

	if language == languageGo {
		types := goOperationInputTypes()
		delete(types, localReaderKey{kind: probeLocalKind(kind)})

		return checkGoOperationRendering(&operation, types)
	}

	types := pyOperationInputTypes()
	delete(types, localReaderKey{kind: probeLocalKind(kind)})

	_, err := pyInputParameters(&operation, types)

	return err
}

// ProbeVerdictVocabulary is every verdict the contract declares, in the
// vocabulary's own order and without its unset member, which is the set an
// operation wording any of them has to word all of.
func ProbeVerdictVocabulary() []string {
	declared := localVerdictVocabulary()
	members := make([]string, 0, len(declared))

	for _, verdict := range declared {
		members = append(members, verdict.String())
	}

	return members
}

// ProbeVerdictValues is every value a verdict sentence may name, spelled the
// way a sentence spells it, which is the whole of what a {placeholder} can
// read.
func ProbeVerdictValues() []string {
	declared := localVerdictValueVocabulary()
	stems := make([]string, 0, len(declared))

	for _, value := range declared {
		stems = append(stems, localVerdictValueStem(value))
	}

	return stems
}

// ProbeAmbientRenderings holds the reading vocabulary to a stated language set,
// which is the only way to reach the refusal it raises: the real set is the
// registry's own, and every language in it renders every reading today.
func ProbeAmbientRenderings(languages []string) error {
	return checkLocalAmbientRenderings(languages)
}

// ProbeAmbientVocabulary is every reading the contract declares, in the
// vocabulary's own order and without its unset member, which is the set every
// registered language has to render.
func ProbeAmbientVocabulary() []string {
	members := linodev1.LocalAmbient(0).Descriptor().Values()
	readings := make([]string, 0, members.Len())

	for index := range members.Len() {
		reading := linodev1.LocalAmbient(members.Get(index).Number())
		if reading != linodev1.LocalAmbient_LOCAL_AMBIENT_UNSPECIFIED {
			readings = append(readings, reading.String())
		}
	}

	return readings
}

// ProbeAmbientRendered answers how one language takes one reading: the local it
// arrives under, the expression its handler hands over, and whether the
// language states a rendering for it at all. A reading rendered in nothing is
// stated and arrives under no name.
func ProbeAmbientRendered(language, reading string) (string, string, bool) {
	taken, rendered := localAmbientRenderings()[language][probeLocalReading(reading)]

	return taken.name, taken.handed, rendered
}

// ProbeAmbientUnhanded runs the rendering check over the shipped table with one
// reading's expression taken out, which is the only way to reach the refusal it
// raises: every row the registry ships states what its handler hands over.
func ProbeAmbientUnhanded(language, reading string) error {
	renderings := localAmbientRenderings()
	member := probeLocalReading(reading)

	taken := renderings[language][member]
	taken.handed = ""
	renderings[language][member] = taken

	return checkAmbientRenderings([]string{language}, renderings)
}

// ProbeGeneratedArms runs the generated-operation derivation over a synthesized
// table, which is the only way to reach the refusals it raises: the real table
// is read off the contract's own members.
func ProbeGeneratedArms(arms []ProbeLocalArm) error {
	table, err := localArms()
	if err != nil {
		return err
	}

	for index := range arms {
		stated := &arms[index]

		call, known := linodev1.LocalCall_value[stated.Call]
		if !known {
			return fmt.Errorf("%w: %s", errUnservedLocalCall, stated.Call)
		}

		table[linodev1.LocalCall(call)] = probeLocalArm(stated)
	}

	generated, err := generatedOperations(table)
	if err != nil {
		return err
	}

	// Both arms, because what each can write is its own refusal: the ambient
	// readings and the untyped inputs are caught while rendering rather than
	// while deriving.
	if _, goErr := renderGoOperations(generated); goErr != nil {
		return goErr
	}

	_, pyErr := renderPyOperations(generated)

	return pyErr
}

// ProbeLocalArmAnswer is one answer row a stated declaration shapes: the
// direction's own member name, left empty on a one-sided operation, and the
// fully qualified message the shape names.
type ProbeLocalArmAnswer struct {
	Direction string
	Shape     string
}

// ProbeLocalArmBodies resolves the shapes one stated declaration names, which
// is how a case reaches the refusals the derivation raises rather than the ones
// the table check does. The shipped declarations ride on the contract's own
// members, so a case can perturb none of them.
func ProbeLocalArmBodies(call string, answers []ProbeLocalArmAnswer) error {
	declared := &linodev1.LocalOperation{
		Answers: make([]*linodev1.LocalOperationAnswer, 0, len(answers)),
	}

	for _, answer := range answers {
		declared.Answers = append(declared.Answers, &linodev1.LocalOperationAnswer{
			Direction: linodev1.LocalDirection(linodev1.LocalDirection_value[answer.Direction]),
			Shape:     answer.Shape,
		})
	}

	_, err := localArmBodies(linodev1.LocalCall(linodev1.LocalCall_value[call]), declared)

	return err
}

// ProbeAnswerShape is one stated answer row: the direction's own member name
// and the message the shape fills.
type ProbeAnswerShape struct {
	Shape     proto.Message
	Direction string
}

// ProbeAnswerShapes runs the shape walk over stated answers, which is the only
// way to reach the projection's refusals: the shipped declarations ride on the
// contract's own members and a case can perturb none of them.
func ProbeAnswerShapes(answers []ProbeAnswerShape) error {
	declared := &linodev1.LocalOperation{
		Answers: make([]*linodev1.LocalOperationAnswer, 0, len(answers)),
	}
	bodies := make(map[linodev1.LocalDirection]protoreflect.MessageDescriptor, len(answers))

	for _, answer := range answers {
		side := linodev1.LocalDirection(linodev1.LocalDirection_value[answer.Direction])
		declared.Answers = append(declared.Answers, &linodev1.LocalOperationAnswer{
			Direction: side,
		})
		bodies[side] = answer.Shape.ProtoReflect().Descriptor()
	}

	_, err := localAnswerShapes(map[linodev1.LocalCall]localArm{
		linodev1.LocalCall_LOCAL_CALL_UNSPECIFIED: {declared: declared, bodies: bodies},
	})

	return err
}

// ProbeAnswerCollision runs the shared-shape check over two stated shapes, each
// naming the message it was read from under one type name.
//
// No two messages the contract declares are spelled alike, so a case reaches
// the refusal by stating a pair that is, the way the cycle cases state an
// ordering no contract can produce.
func ProbeAnswerCollision(name, first, second string) error {
	return sameAnswerShape(
		&answerShape{name: name, full: first},
		&answerShape{name: name, full: second},
	)
}

// ProbeAnswerOrdering runs the dependency ordering over stated shapes, each
// named beside the shapes it reaches. It is how a case reaches the cycle
// refusal, which no message the contract declares can raise.
func ProbeAnswerOrdering(reaches map[string][]string) ([]string, error) {
	stated := make(map[string]answerShape, len(reaches))

	for name, nested := range reaches {
		members := make([]answerMember, 0, len(nested))
		for _, reached := range nested {
			members = append(members, answerMember{
				name: reached, kind: answerNested, shape: reached,
			})
		}

		stated[name] = answerShape{name: name, full: name, members: members}
	}

	ordered, err := orderedAnswerShapes(stated)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(ordered))
	for _, shape := range ordered {
		names = append(names, shape.name)
	}

	return names, nil
}

// ProbeAnswerBody is the plain body one stated shape projects to, which is how
// a case reads the projection's own member set rather than only the refusals
// around it.
func ProbeAnswerBody(message proto.Message) ([]string, error) {
	shape, err := buildAnswerShape(message.ProtoReflect().Descriptor())
	if err != nil {
		return nil, err
	}

	members := make([]string, 0, len(shape.members))
	for _, member := range shape.members {
		members = append(members, member.name)
	}

	return members, nil
}

// ProbeAnswerRecord runs the record check over one stated shape, taken as a
// record whether or not its own message declares one.
//
// Every message the contract declares as a record carries values alone, so a
// case reaches the nested refusal by stating a shape that carries another.
func ProbeAnswerRecord(message proto.Message) error {
	shape, err := buildAnswerShape(message.ProtoReflect().Descriptor())
	if err != nil {
		return err
	}

	shape.record = true

	return checkAnswerRecords([]answerShape{shape})
}

// ProbeRecordsReached runs the reached check over a stated shape set, each row
// naming a message and whether the shape read from it is a record.
//
// Every message the contract declares as a record is answered by an operation,
// so a case reaches the refusal by leaving one out of the set.
func ProbeRecordsReached(shapes map[string]bool) error {
	stated := make([]answerShape, 0, len(shapes))

	for _, full := range slices.Sorted(maps.Keys(shapes)) {
		stated = append(stated, answerShape{full: full, record: shapes[full]})
	}

	return checkRecordsReached(stated)
}

// ProbeDeclaredRecords is every message the linked-in contract declares a
// record, which is what the reached check holds the answer surface to.
func ProbeDeclaredRecords() []string {
	return declaredRecords()
}

// ProbeLocalShapeDifference runs the shape comparison over two generated
// messages, which is how a case reads the comparison's own answer rather than
// only the refusal built from it.
func ProbeLocalShapeDifference(filled, declared proto.Message) string {
	return localShapeDifference(
		filled.ProtoReflect().Descriptor(),
		declared.ProtoReflect().Descriptor(),
	)
}

// probeLocalShapes turns the stated shapes into the table's own keying, nil
// where the case states none.
func probeLocalShapes(
	stated map[string]proto.Message,
) map[linodev1.LocalDirection]protoreflect.MessageDescriptor {
	if len(stated) == 0 {
		return nil
	}

	shapes := make(map[linodev1.LocalDirection]protoreflect.MessageDescriptor, len(stated))

	for direction, message := range stated {
		side := linodev1.LocalDirection(linodev1.LocalDirection_value[direction])
		shapes[side] = message.ProtoReflect().Descriptor()
	}

	return shapes
}

// probeLocalKind resolves the reader set's spelling against the declared
// vocabulary, answering the unset member for a name the set does not carry,
// which is what a declaration naming a type no reader fills amounts to.
func probeLocalKind(name string) linodev1.LocalInputKind {
	member := probeLocalMember("LOCAL_INPUT_KIND_", name)

	return linodev1.LocalInputKind(linodev1.LocalInputKind_value[member])
}

// probeLocalReading resolves one ambient reading's spelling the same way,
// answering the unset member for a name the vocabulary does not carry.
func probeLocalReading(name string) linodev1.LocalAmbient {
	member := probeLocalMember("LOCAL_AMBIENT_", name)

	return linodev1.LocalAmbient(linodev1.LocalAmbient_value[member])
}

// probeLocalGuard resolves one condition's spelling the same way, answering the
// unset member for a name the vocabulary does not carry.
func probeLocalGuard(name string) linodev1.LocalGuard {
	member := probeLocalMember("LOCAL_GUARD_", name)

	return linodev1.LocalGuard(linodev1.LocalGuard_value[member])
}

// probeLocalMember is the enum member one stated spelling names. Derived rather
// than mapped, so a member added to either vocabulary is statable here without
// this file following it.
func probeLocalMember(prefix, name string) string {
	return prefix + strings.ToUpper(name)
}

// ProbeRefusals is every refusal this emitter can raise, by the name it is
// declared under. A test holds a synthesized declaration to the error itself
// rather than to its wording, and the accounting test holds this table to
// errors.go in both directions, so a refusal cannot be added or removed without
// this list following it.
//
// Split by vocabulary rather than kept as one literal: the whole table is past
// the size a reader or a complexity check can carry in one function.
func ProbeRefusals() map[string]error {
	named := probeContractRefusals()
	maps.Copy(named, probeLocalRefusals())

	return named
}

// probeLocalRefusals is the half a declared local answer raises.
func probeLocalRefusals() map[string]error {
	return map[string]error{
		"errLocalAnswerTier":          errLocalAnswerTier,
		"errLocalAnswerTwoAnswers":    errLocalAnswerTwoAnswers,
		"errNoLocalCall":              errNoLocalCall,
		"errUnservedLocalCall":        errUnservedLocalCall,
		"errLocalStateMismatch":       errLocalStateMismatch,
		"errLocalStateUnused":         errLocalStateUnused,
		"errLocalDirectionUnserved":   errLocalDirectionUnserved,
		"errLocalBindNoInput":         errLocalBindNoInput,
		"errLocalBindUnknownInput":    errLocalBindUnknownInput,
		"errLocalBindUnknownArgument": errLocalBindUnknownArgument,
		"errLocalBindRepeated":        errLocalBindRepeated,
		"errLocalBindMissing":         errLocalBindMissing,
		"errLocalBindKind":            errLocalBindKind,
		"errLocalRefuseNoGuard":       errLocalRefuseNoGuard,
		"errLocalRefuseSilent":        errLocalRefuseSilent,
		"errLocalRefuseGuardArm":      errLocalRefuseGuardArm,
		"errLocalRefuseRepeated":      errLocalRefuseRepeated,
		"errLocalRefuseUnworded":      errLocalRefuseUnworded,
		"errLocalRefuseArgument":      errLocalRefuseArgument,
		"errLocalRefusePlaceholder":   errLocalRefusePlaceholder,
		"errLocalArmUnnamed":          errLocalArmUnnamed,
		"errLocalArmUnstemmed":        errLocalArmUnstemmed,
		"errLocalArmSpellingShared":   errLocalArmSpellingShared,
		"errLocalArmInputKind":        errLocalArmInputKind,
		"errLocalArmInputRepeated":    errLocalArmInputRepeated,
		"errLocalArmInputUnnamed":     errLocalArmInputUnnamed,
		"errLocalArmAmbient":          errLocalArmAmbient,
		"errLocalArmToolNamed":        errLocalArmToolNamed,
		"errLocalArmUnshaped":         errLocalArmUnshaped,
		"errLocalShapeMemberKind":     errLocalShapeMemberKind,
		"errLocalRecordNested":        errLocalRecordNested,
		"errLocalRecordUnreached":     errLocalRecordUnreached,
		"errLocalShapeShared":         errLocalShapeShared,
		"errLocalShapeCycle":          errLocalShapeCycle,
		"errLocalAnswerShape":         errLocalAnswerShape,

		"errLocalArmSidedStateless":    errLocalArmSidedStateless,
		"errLocalArmReaderGuard":       errLocalArmReaderGuard,
		"errLocalArmInputUntyped":      errLocalArmInputUntyped,
		"errLocalVerdictUnnamed":       errLocalVerdictUnnamed,
		"errLocalVerdictRepeated":      errLocalVerdictRepeated,
		"errLocalVerdictIncomplete":    errLocalVerdictIncomplete,
		"errLocalVerdictUnworded":      errLocalVerdictUnworded,
		"errLocalVerdictWorded":        errLocalVerdictWorded,
		"errLocalVerdictPlaceholder":   errLocalVerdictPlaceholder,
		"errLocalArmAmbientUnrendered": errLocalArmAmbientUnrendered,
		"errLocalArmAmbientUnhanded":   errLocalArmAmbientUnhanded,
	}
}

// probeContractRefusals is every refusal the rest of the contract raises.
func probeContractRefusals() map[string]error {
	return map[string]error{
		"errToolNotDeclared":               errToolNotDeclared,
		"errHandwrittenNotDeclared":        errHandwrittenNotDeclared,
		"errNoLanguages":                   errNoLanguages,
		"errNoRendererArm":                 errNoRendererArm,
		"errNoOutputDir":                   errNoOutputDir,
		"errArmMisnamed":                   errArmMisnamed,
		"errNoContractOptions":             errNoContractOptions,
		"errUnclaimedOption":               errUnclaimedOption,
		"errUnknownClaim":                  errUnknownClaim,
		"errRepeatedClaim":                 errRepeatedClaim,
		"errSilentClaim":                   errSilentClaim,
		"errEmptyCohort":                   errEmptyCohort,
		"errMetaNotEmitted":                errMetaNotEmitted,
		"errNoDescription":                 errNoDescription,
		"errNoResponse":                    errNoResponse,
		"errNoErrorMessage":                errNoErrorMessage,
		"errNoSuccessMessage":              errNoSuccessMessage,
		"errNoConfirmMessage":              errNoConfirmMessage,
		"errUnusedSuccessMessage":          errUnusedSuccessMessage,
		"errNoWarningMessage":              errNoWarningMessage,
		"errUnusedWarningMessage":          errUnusedWarningMessage,
		"errNotAWriteEnvelope":             errNotAWriteEnvelope,
		"errNotAWritePage":                 errNotAWritePage,
		"errNotABodyRead":                  errNotABodyRead,
		"errNoMarkerCursor":                errNoMarkerCursor,
		"errGatedBodyRead":                 errGatedBodyRead,
		"errUnsupportedBodyKind":           errUnsupportedBodyKind,
		"errUnsupportedItemKind":           errUnsupportedItemKind,
		"errRecursiveItemMessage":          errRecursiveItemMessage,
		"errNoFieldLocation":               errNoFieldLocation,
		"errFoldNotPlaced":                 errFoldNotPlaced,
		"errRoutedToolArgument":            errRoutedToolArgument,
		"errRedactNotBody":                 errRedactNotBody,
		"errBodyNameNotBody":               errBodyNameNotBody,
		"errBodyNameNoop":                  errBodyNameNoop,
		"errNullableNotBody":               errNullableNotBody,
		"errNullableNotObject":             errNullableNotObject,
		"errBodyRootNotObject":             errBodyRootNotObject,
		"errBodyRootNotAlone":              errBodyRootNotAlone,
		"errReaderOnDestroy":               errReaderOnDestroy,
		"errUnsupportedReader":             errUnsupportedReader,
		"errPresentNotBody":                errPresentNotBody,
		"errReaderNotPath":                 errReaderNotPath,
		"errReaderNotPathText":             errReaderNotPathText,
		"errEnumMemberPlacement":           errEnumMemberPlacement,
		"errEnumMemberRepeated":            errEnumMemberRepeated,
		"errEnumMemberKind":                errEnumMemberKind,
		"errEnumMemberNoValues":            errEnumMemberNoValues,
		"errPresentTextShape":              errPresentTextShape,
		"errPresentBoolShape":              errPresentBoolShape,
		"errPresentStringShape":            errPresentStringShape,
		"errIDListShape":                   errIDListShape,
		"errRefuseArguments":               errRefuseArguments,
		"errRefuseDeclaredField":           errRefuseDeclaredField,
		"errAnyOfTooFew":                   errAnyOfTooFew,
		"errAnyOfUnknownField":             errAnyOfUnknownField,
		"errAnyOfDuplicateField":           errAnyOfDuplicateField,
		"errNormalizeNoField":              errNormalizeNoField,
		"errNormalizeUnknownField":         errNormalizeUnknownField,
		"errNormalizeRepeatedField":        errNormalizeRepeatedField,
		"errNormalizeSecretField":          errNormalizeSecretField,
		"errScanWithResource":              errScanWithResource,
		"errEnvelopeWithResource":          errEnvelopeWithResource,
		"errStateReadNotList":              errStateReadNotList,
		"errMatchUnknownField":             errMatchUnknownField,
		"errCompositeUnstaged":             errCompositeUnstaged,
		"errCompositeTooFew":               errCompositeTooFew,
		"errCompositeMember":               errCompositeMember,
		"errCompositeNoFields":             errCompositeNoFields,
		"errCompositeUnknownField":         errCompositeUnknownField,
		"errWalkSourceCount":               errWalkSourceCount,
		"errWalkNoEmit":                    errWalkNoEmit,
		"errWalkErrorWarning":              errWalkErrorWarning,
		"errWalkEmitKind":                  errWalkEmitKind,
		"errWalkEmitEmpty":                 errWalkEmitEmpty,
		"errWalkFilterShape":               errWalkFilterShape,
		"errWalkWarningShape":              errWalkWarningShape,
		"errBillingShape":                  errBillingShape,
		"errWalkEnrichShape":               errWalkEnrichShape,
		"errWalkEmitLabel":                 errWalkEmitLabel,
		"errWalkSentence":                  errWalkSentence,
		"errWalkNoState":                   errWalkNoState,
		"errWalkStateMember":               errWalkStateMember,
		"errDepWalkUnknownField":           errDepWalkUnknownField,
		"errDepWalkPlaceholder":            errDepWalkPlaceholder,
		"errFoldEmptyKey":                  errFoldEmptyKey,
		"errFoldSelfTarget":                errFoldSelfTarget,
		"errFoldSourceKind":                errFoldSourceKind,
		"errFoldTargetKind":                errFoldTargetKind,
		"errUnsupportedTransform":          errUnsupportedTransform,
		"errWalkTier":                      errWalkTier,
		"errWalkNoField":                   errWalkNoField,
		"errWalkUnknownField":              errWalkUnknownField,
		"errWalkRepeatedField":             errWalkRepeatedField,
		"errWalkSilent":                    errWalkSilent,
		"errWalkElementOnMap":              errWalkElementOnMap,
		"errWalkEmptyOnList":               errWalkEmptyOnList,
		"errWalkValueWithoutKey":           errWalkValueWithoutKey,
		"errWalkDuplicateKey":              errWalkDuplicateKey,
		"errWalkUnknownWithoutVocabulary":  errWalkUnknownWithoutVocabulary,
		"errWalkRequireUnknownName":        errWalkRequireUnknownName,
		"errWalkBoundKind":                 errWalkBoundKind,
		"errWalkMembersKind":               errWalkMembersKind,
		"errWalkUnknownEnum":               errWalkUnknownEnum,
		"errWalkPlaceholder":               errWalkPlaceholder,
		"errAnyOfTier":                     errAnyOfTier,
		"errReaderValuesOnEnum":            errReaderValuesOnEnum,
		"errReaderValuesAlone":             errReaderValuesAlone,
		"errReaderMessageAlone":            errReaderMessageAlone,
		"errReaderMessageArm":              errReaderMessageArm,
		"errReaderNotPathInteger":          errReaderNotPathInteger,
		"errCommaListNotBody":              errCommaListNotBody,
		"errCommaListNotString":            errCommaListNotString,
		"errEchoArgumentShape":             errEchoArgumentShape,
		"errEchoArgumentNoop":              errEchoArgumentNoop,
		"errNoGoType":                      errNoGoType,
		"errAmbiguousEnvelope":             errAmbiguousEnvelope,
		"errUnmatchedFilter":               errUnmatchedFilter,
		"errUnsupportedTier":               errUnsupportedTier,
		"errPyRender":                      errPyRender,
		"errPyFormat":                      errPyFormat,
		"errPyLineBudget":                  errPyLineBudget,
		"errPathArity":                     errPathArity,
		"errUnsupportedPathKind":           errUnsupportedPathKind,
		"errUnsupportedQuery":              errUnsupportedQuery,
		"errNoReadPreview":                 errNoReadPreview,
		"errNoStructFailPrefix":            errNoStructFailPrefix,
		"errNoNormalizePoint":              errNoNormalizePoint,
		"errNotAWrapper":                   errNotAWrapper,
		"errUnknownPlaceholder":            errUnknownPlaceholder,
		"errRepeatedPlaceholder":           errRepeatedPlaceholder,
		"errNotRepeated":                   errNotRepeated,
		"errUngatedExecute":                errUngatedExecute,
		"errNotAnAssembledRead":            errNotAnAssembledRead,
		"errNoTransportArm":                errNoTransportArm,
		"errTransportIncomplete":           errTransportIncomplete,
		"errTransportDirectionFields":      errTransportDirectionFields,
		"errNoTransferDirection":           errNoTransferDirection,
		"errTransportUnknownArgument":      errTransportUnknownArgument,
		"errTransportMembers":              errTransportMembers,
		"errNotADeleteEnvelope":            errNotADeleteEnvelope,
		"errNoStateRead":                   errNoStateRead,
		"errStateRouteWithNoReader":        errStateRouteWithNoReader,
		"errStateRouteUnknownTool":         errStateRouteUnknownTool,
		"errStateRouteNotRead":             errStateRouteNotRead,
		"errStateRouteNoResponse":          errStateRouteNoResponse,
		"errStateRouteUnknownSlot":         errStateRouteUnknownSlot,
		"errStateRouteSlotUnknownRead":     errStateRouteSlotUnknownRead,
		"errStateRouteSlotUnknownArgument": errStateRouteSlotUnknownArgument,
		"errStateRouteSlotSameName":        errStateRouteSlotSameName,
		"errStateRouteUnfilledQuery":       errStateRouteUnfilledQuery,
		"errStateRouteQueryAmbiguous":      errStateRouteQueryAmbiguous,
		"errStateRouteQuerySecret":         errStateRouteQuerySecret,
		"errStateRoutePayloadMember":       errStateRoutePayloadMember,
		"errStateRoutePayloadWithBodyKey":  errStateRoutePayloadWithBodyKey,

		"errPreviewTransportNotUpload":       errPreviewTransportNotUpload,
		"errPreviewTransportMember":          errPreviewTransportMember,
		"errPreviewTransportWithState":       errPreviewTransportWithState,
		"errPreviewSentenceNoTemplate":       errPreviewSentenceNoTemplate,
		"errPreviewSentenceNoLine":           errPreviewSentenceNoLine,
		"errPreviewSentenceUnclosed":         errPreviewSentenceUnclosed,
		"errPreviewSentenceUnknownArgument":  errPreviewSentenceUnknownArgument,
		"errPreviewSentenceUnreportable":     errPreviewSentenceUnreportable,
		"errPreviewSentenceSecretArgument":   errPreviewSentenceSecretArgument,
		"errPreviewSentenceUnreachable":      errPreviewSentenceUnreachable,
		"errPreviewSentenceNoState":          errPreviewSentenceNoState,
		"errPreviewPresentWithChoice":        errPreviewPresentWithChoice,
		"errPreviewNumberGuardMixed":         errPreviewNumberGuardMixed,
		"errPreviewUnchangedUnread":          errPreviewUnchangedUnread,
		"errPreviewUnchangedUnknownArgument": errPreviewUnchangedUnknownArgument,
		"errPreviewMatchWithChoice":          errPreviewMatchWithChoice,
		"errPreviewMatchWithTemplate":        errPreviewMatchWithTemplate,
		"errPreviewMatchArgument":            errPreviewMatchArgument,
		"errPreviewMatchNoArm":               errPreviewMatchNoArm,
		"errPreviewMatchArmEmpty":            errPreviewMatchArmEmpty,
		"errPreviewMatchArmRepeated":         errPreviewMatchArmRepeated,
		"errPreviewChangedUnknownArgument":   errPreviewChangedUnknownArgument,
		"errPreviewChangedWithUnchanged":     errPreviewChangedWithUnchanged,
		"errPreviewElementWithoutList":       errPreviewElementWithoutList,
		"errPreviewElementUnread":            errPreviewElementUnread,
		"errPreviewElementNotRepeated":       errPreviewElementNotRepeated,
		"errPreviewElementUnreportable":      errPreviewElementUnreportable,
		"errPreviewElementBesideLine":        errPreviewElementBesideLine,
		"errPreviewStateUnknownMember":       errPreviewStateUnknownMember,
		"errPreviewStateDeepMember":          errPreviewStateDeepMember,
		"errPreviewChoiceWithTemplate":       errPreviewChoiceWithTemplate,
		"errPreviewChoiceNoArgument":         errPreviewChoiceNoArgument,
		"errPreviewChoiceNoWording":          errPreviewChoiceNoWording,
		"errPreviewChoiceUnknownArgument":    errPreviewChoiceUnknownArgument,
		"errPreviewChoiceNotFlag":            errPreviewChoiceNotFlag,
		"errPreviewChoiceNotObject":          errPreviewChoiceNotObject,
		"errPreviewChoiceDeepMember":         errPreviewChoiceDeepMember,
		"errPreviewMatchSelector":            errPreviewMatchSelector,
		"errPreviewOmitsBodyWithoutBody":     errPreviewOmitsBodyWithoutBody,
		"errPreviewStandInNotItemList":       errPreviewStandInNotItemList,
		"errPreviewStandInUnknownMember":     errPreviewStandInUnknownMember,
		"errPreviewStandInNoText":            errPreviewStandInNoText,
		"errPreviewStandInRepeated":          errPreviewStandInRepeated,
		"errPreviewStandInTier":              errPreviewStandInTier,
		"errPartialTwoStage":                 errPartialTwoStage,
		"errNoTwoStageDriver":                errNoTwoStageDriver,
		"errUnstagedWalk":                    errUnstagedWalk,
		"errNoNullPayload":                   errNoNullPayload,
		"errUnknownNullField":                errUnknownNullField,
		"errUnsupportedDestroyShape":         errUnsupportedDestroyShape,
		"errUnplacedLocalMember":             errUnplacedLocalMember,
		"errUnknownResponseBody":             errUnknownResponseBody,
		"errUnusedResponseBody":              errUnusedResponseBody,
		"errEmptyDefault":                    errEmptyDefault,
		"errModifiedDefault":                 errModifiedDefault,
		"errDefaultNotToolArg":               errDefaultNotToolArg,
		"errUnsupportedToolArg":              errUnsupportedToolArg,
		"errUnreadableDefault":               errUnreadableDefault,
		"errMetaDryRun":                      errMetaDryRun,
		"errNoMetaAnswer":                    errNoMetaAnswer,
		"errNoMetaMessageField":              errNoMetaMessageField,
		"errNullsOverDecodedResponse":        errNullsOverDecodedResponse,
		"errNoScopes":                        errNoScopes,
		"errScopesOnMeta":                    errScopesOnMeta,
		"errScopesNoneBesideList":            errScopesNoneBesideList,
		"errScopeRepeated":                   errScopeRepeated,
		"errScopeUnrenderable":               errScopeUnrenderable,
		"errNoCategories":                    errNoCategories,
		"errCategoriesNoneBesideList":        errCategoriesNoneBesideList,
		"errCategoryRepeated":                errCategoryRepeated,
		"errCategoryUnrenderable":            errCategoryUnrenderable,
	}
}
