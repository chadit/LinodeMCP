// Package goname spells a contract name the way Go spells identifiers.
//
// Two places need the same answer and neither can import the other:
// cmd/toolgen writes the factory, handler, and hook-call names into the
// generated tree, and internal/toolhooks derives the name a declared hook must
// be implemented under. A word this package renders one way and that one
// renders another is a call to a function nobody wrote, which is why the table
// lives here rather than beside either caller.
package goname

import "strings"

// Exported renders an underscore- or hyphen-separated name in Go's exported
// spelling: linode_domain_record_get becomes LinodeDomainRecordGet, and
// linode_object_storage_object_acl_get becomes LinodeObjectStorageObjectACLGet
// rather than the ...Acl... revive rejects.
func Exported(name string) string {
	var built strings.Builder

	built.Grow(len(name))

	for word := range strings.SplitSeq(strings.ReplaceAll(name, "-", "_"), "_") {
		if word == "" {
			continue
		}

		if initialism, known := Initialism(word); known {
			built.WriteString(initialism)

			continue
		}

		built.WriteString(strings.ToUpper(word[:1]))
		built.WriteString(word[1:])
	}

	return built.String()
}

// Initialism returns the conventional Go spelling of a word that is an
// initialism, and whether the word is one.
//
// revive's var-naming rule reads a name like ListIpAcl as two misspelled
// initialisms and fails the build, so every one the tool surface spells has to
// be here rather than only the ones a current name happens to use.
func Initialism(word string) (string, bool) {
	switch word {
	case "id":
		return "ID", true
	case "ip":
		return "IP", true
	// The version keeps its lowercase v, which is how the whole repo spells
	// these: protoc-gen-go's IPv6Range, the client's GetIPv6RangeProto, the
	// paramIPv6Range constant.
	case "ipv4":
		return "IPv4", true
	case "ipv6":
		return "IPv6", true
	case "url":
		return "URL", true
	case "api":
		return "API", true
	case "acl":
		return "ACL", true
	case "uuid":
		return "UUID", true
	case "ssl":
		return "SSL", true
	case "vpc":
		return "VPC", true
	}

	return "", false
}
