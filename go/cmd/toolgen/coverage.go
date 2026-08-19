package main

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// The cross-arm coverage check.
//
// The two languages do not split the work the same way. Go emits a declared
// sentence into the factory; Python's driver reads that same option off the
// descriptor when the tool is called. Both are honest renderings of one
// contract, so the check here is not that the arms agree on where an option is
// acted. It is that each arm answers for every option at all: the failure this
// exists for is an option added to the contract, taught to one arm, and left
// out of the other, which produces two trees that compile, register the same
// tools, and quietly do different things.

// optionClaim is one arm's answer for one declared option.
type optionClaim struct {
	option protoreflect.Name
	// home is the file that acts the option for an arm that does not emit it,
	// "" for one that does. It is named rather than described so the claim can
	// be held to a file that still mentions the option.
	home string
	// emitted is whether this arm's rendered bytes move with the option.
	emitted bool
}

// contractOptions is every option the contract declares, sorted.
//
// Read out of the compiled descriptors rather than listed here: an option added
// to the proto joins this set the moment it compiles, which is what keeps the
// coverage check from being a list that goes quietly out of date while the
// contract grows past it. The short name is the key because an extension's full
// name is unique inside the package it is declared in, so the two cannot
// disagree.
func contractOptions() ([]protoreflect.Name, error) {
	found := make([]protoreflect.Name, 0)

	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != protoPackage {
			return true
		}

		extensions := file.Extensions()
		for i := range extensions.Len() {
			found = append(found, extensions.Get(i).Name())
		}

		return true
	})

	if len(found) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoContractOptions, protoPackage)
	}

	slices.Sort(found)

	return found, nil
}

// checkRendererCoverage holds every arm of a run to every option the contract
// declares, ahead of any rendering: an arm with nothing to say about a
// declaration stops the run before one tree is rewritten and the other is left
// carrying a capability it never learned.
func checkRendererCoverage(arms []languageArm) error {
	options, err := contractOptions()
	if err != nil {
		return err
	}

	for _, arm := range arms {
		if err := checkArmCoverage(arm.lang.language(), arm.lang.acts(), options); err != nil {
			return err
		}
	}

	return nil
}

// checkArmCoverage reads one arm's claims against the declared options. Every
// refusal names the language and the option, since the whole point is to say
// which arm went short and of what.
func checkArmCoverage(language string, claims []optionClaim, options []protoreflect.Name) error {
	answered := make(map[protoreflect.Name]bool, len(options))
	for _, option := range options {
		answered[option] = false
	}

	for _, claim := range claims {
		if err := readClaim(language, claim, answered); err != nil {
			return err
		}

		answered[claim.option] = true
	}

	for _, option := range options {
		if !answered[option] {
			return fmt.Errorf("%w: %s says nothing about %s", errUnclaimedOption, language, option)
		}
	}

	return nil
}

// readClaim refuses the three ways one claim can be wrong before it counts as
// an answer.
func readClaim(language string, claim optionClaim, answered map[protoreflect.Name]bool) error {
	already, declared := answered[claim.option]

	switch {
	case !declared:
		return fmt.Errorf("%w: %s claims %s", errUnknownClaim, language, claim.option)
	case already:
		return fmt.Errorf("%w: %s claims %s twice", errRepeatedClaim, language, claim.option)
	case !claim.emitted && claim.home == "":
		return fmt.Errorf("%w: %s neither emits %s nor names its home",
			errSilentClaim, language, claim.option)
	}

	return nil
}
