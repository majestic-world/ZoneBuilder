// Package inflect holds the pt-BR rules for text built at runtime: a noun
// inflected to a count, shared by every user-facing or logged string built
// from a runtime count, and a list of items.
package inflect

import (
	"strconv"
	"strings"
)

// Count is n with the noun inflected for it: "1 vértice", "2 vértices",
// "0 vértices".
func Count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return strconv.Itoa(n) + " " + plural
}

// List joins items as a pt-BR list: "a", "a e b", "a, b e c".
func List(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " e " + items[len(items)-1]
}
