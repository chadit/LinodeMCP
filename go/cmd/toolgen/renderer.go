package main

// The emitter has two halves. The front half reads the descriptors into the
// language-neutral contract model, and everything below the seam turns that
// model into one language's files. Splitting them here is what lets a second
// language be a renderer arm rather than a second emitter reading the
// descriptors again.

// emittedFile is one file a renderer produced, named relative to the output
// directory.
type emittedFile struct {
	Name string
	Text string
}

// renderer is one language's half of the emitter.
//
// The methods follow the shape emission already had: the driver groups the
// contract model by proto source file, hands each group to the language, asks
// once more for the registry that binds the whole cohort, and needs to know
// which files in the output tree were the language's own before it rewrites
// them.
type renderer interface {
	// renderGroup emits the file holding every tool declared in one proto
	// source file. Grouping follows the proto tree rather than the tool's
	// category in the server so a reader can find the emitted code from the
	// message they were just looking at.
	renderGroup(group string, tools []*contract) (emittedFile, error)

	// renderAnswers emits the file holding the value type each declared answer
	// shape fills and the projection that turns one into a plain body, plus the
	// failure vocabulary and the subsystem type each generated operation is
	// served through. It is a tree of its own because the engine builds these
	// values and implements those types, and the tool tree already reads the
	// engine, so the three cannot live in one package.
	renderAnswers(shapes []answerShape, operations []localOperation) (emittedFile, error)

	// renderOperations emits the arm behind each generated operation, which
	// lands with the handlers that call it: an arm reaches the engine's outcome
	// as well as the answer, and the tool tree already reads both.
	renderOperations(operations []localOperation) ([]emittedFile, error)

	// renderRegistry emits the file the server reads to register the cohort. A
	// tool that nothing registers passes every gate that reads the contract and
	// is still missing from the running server, so registration is emitted from
	// the same cohort as the factories.
	renderRegistry(contracts []contract) (emittedFile, error)

	// owns reports whether a file already in the output directory came from
	// this renderer, so a run clears its own stale output and nothing else.
	owns(name string) bool

	// language is the name docs/contracts/languages.txt registers this arm
	// under. The arm table is what pairs the two, and a refusal about coverage
	// has to say which language went short.
	language() string

	// acts is this arm's answer for every option the contract declares: the
	// ones its own rendering carries, and the file that acts each one it leaves
	// to the language's support layer. An option missing from here stops the
	// run, so a capability cannot be taught to one arm and land in the other's
	// tree as silence.
	acts() []optionClaim
}

// renderTree turns the contract model into one language's file set.
func renderTree(
	lang renderer, contracts []contract, operations []localOperation,
) (map[string]string, error) {
	grouped := make(map[string][]*contract, len(contracts))

	for i := range contracts {
		group := groupName(contracts[i].SourceFile)
		grouped[group] = append(grouped[group], &contracts[i])
	}

	files := make(map[string]string, len(grouped)+1)

	for group, tools := range grouped {
		file, err := lang.renderGroup(group, tools)
		if err != nil {
			return nil, err
		}

		files[file.Name] = file.Text
	}

	registry, err := lang.renderRegistry(contracts)
	if err != nil {
		return nil, err
	}

	files[registry.Name] = registry.Text

	written, err := lang.renderOperations(operations)
	if err != nil {
		return nil, err
	}

	for _, file := range written {
		files[file.Name] = file.Text
	}

	return files, nil
}
