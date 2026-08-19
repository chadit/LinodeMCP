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
	}
}

// readNormalizeFields resolves the rewrites a tool declares over its arguments.
//
// Refused beside a normalize hook: the hook already answers the whole map, so a
// declaration next to it would rewrite a value twice in an order nothing states.
func (c *contract) readNormalizeFields(options protoreflect.ProtoMessage) error {
	declared, _ := proto.GetExtension(options, linodev1.E_NormalizeFields).([]*linodev1.NormalizeField)
	if len(declared) == 0 {
		return nil
	}

	if c.Hooks.Normalize != "" {
		return fmt.Errorf("%w: %s", errNormalizeWithHook, c.Name)
	}

	rewritten := make(map[string]bool, len(declared))
	rewrites := make([]normalizeRewrite, 0, len(declared))

	for _, entry := range declared {
		rewrite, err := c.readNormalizeEntry(entry, rewritten)
		if err != nil {
			return err
		}

		rewrites = append(rewrites, rewrite)
	}

	c.Normalizes = rewrites

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
// itself rather than something that merely addresses one. The hooks this
// option replaces stepped around those names by hand, so the rule reads the
// name the way they did.
func namesSecret(name string) bool {
	markers := []string{"password", "secret", "token"}

	return slices.ContainsFunc(markers, func(marker string) bool {
		return strings.Contains(name, marker)
	})
}
