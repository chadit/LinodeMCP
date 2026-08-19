package linoderoute

import "strings"

// hexDigits renders percent-escapes with uppercase hex, the form RFC 3986
// prefers. Nothing on either side normalizes URLs, so both clients have to
// escape the same label into the same bytes.
const hexDigits = "0123456789ABCDEF"

// EscapeSegment percent-encodes value so it fills exactly one path segment.
//
// The kept set is RFC 3986's unreserved characters plus the colon, stricter than
// net/url's PathEscape: that leaves the sub-delimiters ("@&+=,;$" among them)
// standing while the Python client's quote encodes them, and both clients have
// to build the same path for the same tool and values.
//
// The colon stays literal because both clients already agreed on that for the
// IPv6 and IP address routes ("/networking/ipv6/ranges/2001:db8::%2F64"), with
// tests on each side pinning the literal form. RFC 3986 allows an unencoded
// colon in a path segment, so keeping it is as conformant as encoding it.
//
// A value of nothing but dots must not pass through literally: "." and ".." are
// dot-segments, so the path addresses the parent collection after normalization,
// which for a DELETE is every resource in it. Encoding the dots names the same
// resource after server-side decoding while staying inert during normalization.
//
// Escaping here rather than at the hundreds of call sites keeps a forgotten call
// from addressing a different route without failing.
// It is exported for the generated previews, which fill a path template
// themselves instead of routing through Resolve and would otherwise report a
// slash-bearing id as extra path segments the live call never sends.
func EscapeSegment(value string) string {
	if allDots(value) {
		return strings.Repeat("%2E", len(value))
	}

	if !needsEscape(value) {
		return value
	}

	var built strings.Builder

	built.Grow(len(value))

	for i := range len(value) {
		char := value[i]
		if keptLiteral(char) {
			built.WriteByte(char)

			continue
		}

		// Shift right by 4 bits for the high nibble, mask the low 4 for the other.
		built.WriteByte('%')
		built.WriteByte(hexDigits[char>>4])
		built.WriteByte(hexDigits[char&0x0F])
	}

	return built.String()
}

// allDots reports whether value is nothing but dots, the shape that would read
// as a dot-segment once substituted into a path. The empty string answers true
// vacuously: writeValue rejects an empty value before escaping, and escaping
// zero dots renders zero escapes anyway.
func allDots(value string) bool {
	for i := range len(value) {
		if value[i] != '.' {
			return false
		}
	}

	return true
}

// needsEscape reports whether escaping value would change it. Nearly every path
// value is a Linode numeric id or a machine-generated label, so asking first
// keeps the common case from building a second copy of the string.
func needsEscape(value string) bool {
	for i := range len(value) {
		if !keptLiteral(value[i]) {
			return true
		}
	}

	return false
}

// keptLiteral reports whether a byte stands for itself in a path segment: the
// RFC 3986 unreserved set plus the colon, per the EscapeSegment comment.
// Non-ASCII bytes fall through to false, which encodes a multi-byte rune as the
// percent-escaped UTF-8 the API expects.
func keptLiteral(char byte) bool {
	switch {
	case char >= 'A' && char <= 'Z':
		return true
	case char >= 'a' && char <= 'z':
		return true
	case char >= '0' && char <= '9':
		return true
	default:
		return char == '-' || char == '.' || char == '_' || char == '~' || char == ':'
	}
}
