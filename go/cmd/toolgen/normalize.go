package main

import (
	"fmt"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// The declared argument rewrites, read once into the contract model and
// rendered by each arm as a call to its own support layer.

// normalizeRewrite is one declared transform and the arguments it rewrites, in
// the order the declaration reads them.
type normalizeRewrite struct {
	Fields    []string
	Transform linodev1.NormalizeTransform
}

// normalizeCall is the support-layer function one transform is rendered as in
// each language.
type normalizeCall struct {
	Go     string
	Python string
}

// normalizeRendering is what each declared transform renders to. A transform
// missing from here is refused at contract build, so a member added to the enum
// cannot reach one tree as a call and the other as silence.
func normalizeRendering() map[linodev1.NormalizeTransform]normalizeCall {
	return map[linodev1.NormalizeTransform]normalizeCall{
		linodev1.NormalizeTransform_NORMALIZE_TRANSFORM_TRIM: {
			Go: "TrimArguments", Python: "trim_arguments",
		},
		linodev1.NormalizeTransform_NORMALIZE_TRANSFORM_TRIM_LIST_DROP_BLANK: {
			Go: "TrimListDropBlank", Python: "trim_list_drop_blank",
		},
		linodev1.NormalizeTransform_NORMALIZE_TRANSFORM_TRIM_LIST: {
			Go: "TrimList", Python: "trim_list",
		},
	}
}

// normalizeFold is the declared convenience-into-object rewrite, read from
// normalize_fold. The semantics are fixed by the option's contract; only the
// three names travel here.
type normalizeFold struct {
	Source string
	Target string
	Key    string
}

// readNormalizeFields resolves the rewrites a tool declares over its arguments.
func (c *contract) readNormalizeFields(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_NormalizeFields).([]*linodev1.NormalizeField)

	rewritten := make(map[string]bool, len(declared))
	if len(declared) == 0 {
		return c.readNormalizeFold(options, rewritten)
	}

	rewrites := make([]normalizeRewrite, 0, len(declared))

	for _, entry := range declared {
		rewrite, err := c.readNormalizeEntry(entry, rewritten)
		if err != nil {
			return err
		}

		rewrites = append(rewrites, rewrite)
	}

	c.Normalizes = rewrites

	return c.readNormalizeFold(options, rewritten)
}

// readNormalizeFold resolves the declared convenience fold, holding both of
// its names to arguments the tool has and to the two shapes the rendered
// helper reads: an integer list folding into an open object.
func (c *contract) readNormalizeFold(
	options protoreflect.ProtoMessage, rewritten map[string]bool,
) error {
	if !proto.HasExtension(options, linodev1.E_NormalizeFold) {
		return nil
	}

	declared, _ := proto.GetExtension(options, linodev1.E_NormalizeFold).(*linodev1.NormalizeFold)

	if err := c.checkFoldNames(declared, rewritten); err != nil {
		return err
	}

	if err := c.checkFoldShapes(declared); err != nil {
		return err
	}

	c.Fold = &normalizeFold{
		Source: declared.GetSource(),
		Target: declared.GetTarget(),
		Key:    declared.GetKey(),
	}

	return nil
}

// checkFoldNames holds the fold's names to being distinct declared arguments
// no other rewrite touches, with an address to write to.
func (c *contract) checkFoldNames(
	declared *linodev1.NormalizeFold, rewritten map[string]bool,
) error {
	if declared.GetKey() == "" {
		return fmt.Errorf("%w: %s", errFoldEmptyKey, c.Name)
	}

	if declared.GetSource() == declared.GetTarget() {
		return fmt.Errorf("%w: %s folds %s into itself",
			errFoldSelfTarget, c.Name, declared.GetSource())
	}

	for _, name := range []string{declared.GetSource(), declared.GetTarget()} {
		if err := c.checkNormalizeField(name, rewritten); err != nil {
			return err
		}

		rewritten[name] = true
	}

	return nil
}

// checkFoldShapes holds the fold to the two shapes the rendered helper reads:
// the source an integer list, the target an open object.
func (c *contract) checkFoldShapes(declared *linodev1.NormalizeFold) error {
	source, sourceDeclared := c.bodyField(declared.GetSource())
	if !sourceDeclared || !source.Repeated || source.Kind != protoreflect.Int32Kind {
		return fmt.Errorf("%w: %s folds %s", errFoldSourceKind, c.Name, declared.GetSource())
	}

	target, targetDeclared := c.bodyField(declared.GetTarget())
	if !targetDeclared || !target.ObjectMap {
		return fmt.Errorf("%w: %s folds into %s", errFoldTargetKind, c.Name, declared.GetTarget())
	}

	return nil
}

// readNormalizeEntry resolves one declaration, recording the arguments it
// rewrites so a later one naming the same argument is refused.
func (c *contract) readNormalizeEntry(
	entry *linodev1.NormalizeField, rewritten map[string]bool,
) (normalizeRewrite, error) {
	names := entry.GetField()
	if len(names) == 0 {
		return normalizeRewrite{}, fmt.Errorf("%w: %s declares %s over nothing",
			errNormalizeNoField, c.Name, entry.GetTransform())
	}

	if _, rendered := normalizeRendering()[entry.GetTransform()]; !rendered {
		return normalizeRewrite{}, fmt.Errorf("%w: %s declares %s",
			errUnsupportedTransform, c.Name, entry.GetTransform())
	}

	for _, name := range names {
		if err := c.checkNormalizeField(name, rewritten); err != nil {
			return normalizeRewrite{}, err
		}

		rewritten[name] = true
	}

	return normalizeRewrite{Transform: entry.GetTransform(), Fields: names}, nil
}

// checkNormalizeField holds one named argument to being a value the tool has
// and may rewrite.
func (c *contract) checkNormalizeField(name string, rewritten map[string]bool) error {
	switch {
	case !slices.Contains(c.AllArguments, name):
		return fmt.Errorf("%w: %s rewrites %s", errNormalizeUnknownField, c.Name, name)
	case rewritten[name]:
		return fmt.Errorf("%w: %s rewrites %s", errNormalizeRepeatedField, c.Name, name)
	case namesSecret(name):
		return fmt.Errorf("%w: %s rewrites %s", errNormalizeSecretField, c.Name, name)
	}

	return nil
}

// namesSecret reports whether an argument name marks it as the credential
// itself rather than something that merely addresses one. The hand-written
// bodies this
// option replaces stepped around those names by hand, so the rule reads the
// name the way they did.
func namesSecret(name string) bool {
	markers := []string{"password", "secret", "token"}

	return slices.ContainsFunc(markers, func(marker string) bool {
		return strings.Contains(name, marker)
	})
}
