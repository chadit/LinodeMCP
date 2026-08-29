package main

import (
	"strings"
)

// The Python arm of the declared transports: the same declaration the Go arm
// renders as a call, rendered here as one spec literal the driver runs, which
// is the pattern the declared walks already follow.

// pyTransportLine is the driver keyword one transported tool passes. The
// literal goes out on one line and the formatter wraps it, the way every other
// rendered keyword is written.
func pyTransportLine(tool *pyTool) string {
	spec := tool.c.Transport

	switch spec.Kind {
	case transportMultipart:
		return "        transport=MultipartUpload(" + pyKeywords(
			"file_argument", pyQuote(spec.FileArgument),
			"part_name", pyQuote(spec.PartName),
		) + "),"
	case transportRawBody:
		return "        transport=RawBody(" + pyRawBodyKeywords(spec) + "),"
	case transportPresign:
		return "        transport=PresignTransfer(" + pyPresignKeywords(spec) + "),"
	case transportNone:
	}

	return ""
}

// pyRawBodyKeywords renders a bare-bytes transfer, naming only the end its
// direction reads.
func pyRawBodyKeywords(spec *transportSpec) string {
	if spec.Up {
		return pyKeywords(
			"content_type", pyQuote(spec.ContentType),
			"up", "True",
			"source_argument", pyQuote(spec.SourceArgument),
		)
	}

	return pyKeywords(
		"content_type", pyQuote(spec.ContentType),
		"answer_field", pyQuote(spec.AnswerField),
	)
}

// pyPresignKeywords renders a minted-URL transfer, naming the argument its own
// guard reads and the constants its answer carries.
func pyPresignKeywords(spec *transportSpec) string {
	pairs := []string{
		"url_field", pyQuote(spec.URLField),
		"local_path_argument", pyQuote(spec.LocalPathArgument),
	}

	pairs = append(pairs, pyPresignGuardKeywords(spec)...)

	pairs = append(pairs,
		"size_field", pyQuote(spec.SizeField),
		"etag_field", pyQuote(spec.ETagField))

	if len(spec.Constants) > 0 {
		constants := make([]string, 0, len(spec.Constants))
		for _, constant := range spec.Constants {
			constants = append(constants,
				pyQuote(constant.GetField())+": "+pyQuote(constant.GetText()))
		}

		pairs = append(pairs, "constants", "{"+strings.Join(constants, ", ")+"}")
	}

	return pyKeywords(pairs...)
}

// pyPresignGuardKeywords is the one argument a direction's own guard reads.
func pyPresignGuardKeywords(spec *transportSpec) []string {
	if spec.Up {
		return []string{"up", "True", "content_type_argument", pyQuote(spec.ContentTypeArgument)}
	}

	return []string{"overwrite_argument", pyQuote(spec.OverwriteArgument)}
}

// pyKeywords joins name and value pairs into a call's argument list.
func pyKeywords(pairs ...string) string {
	rendered := make([]string, 0, len(pairs)/2)

	var index int
	for ; index+1 < len(pairs); index += 2 {
		rendered = append(rendered, pairs[index]+"="+pairs[index+1])
	}

	return strings.Join(rendered, ", ")
}

// pyTransportImports is the spec types a module's transported tools spell.
func pyTransportImports(tools []*pyTool) string {
	named := make(map[string]bool, len(tools))

	for _, tool := range tools {
		if !tool.c.transported() {
			continue
		}

		switch tool.c.Transport.Kind {
		case transportMultipart:
			named["MultipartUpload"] = true
		case transportRawBody:
			named["RawBody"] = true
		case transportPresign:
			named["PresignTransfer"] = true
		case transportNone:
		}
	}

	return strings.Join(pySortedKeys(named), ", ")
}
