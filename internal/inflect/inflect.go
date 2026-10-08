// Package inflect holds the one rule for inflecting a noun to a count, shared
// by every user-facing or logged string built from a runtime count.
package inflect

import "strconv"

// Count is n with the noun inflected for it: "1 vértice", "2 vértices",
// "0 vértices".
func Count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(n) + " " + plural
}
