// Package locale renders user-facing messages from embedded pt-BR and English catalogs.
// Message keys are stable identifiers, not source-language text.
package locale

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

// Language is a supported interface language.
type Language string

const (
	PtBR Language = "pt-BR"
	En   Language = "en"
)

//go:embed catalog/*/*.json
var bundled embed.FS

var messages = func() map[Language]map[string]entry {
	catalogs, err := fs.Sub(bundled, "catalog")
	if err != nil {
		panic(err)
	}
	entries, err := loadCatalogs(catalogs)
	if err != nil {
		panic(err)
	}
	return entries
}()

// Normalize returns pt-BR for absent or unsupported preferences.
func Normalize(preference string) Language {
	if preference == string(En) {
		return En
	}
	return PtBR
}

// Text returns a static message. Missing keys and wrong message variants panic;
// development tests should catch these before an application is shipped.
func Text(language Language, key string) string {
	message := lookup(language, key)
	if message.plural {
		panic(fmt.Sprintf("locale: %s requires a count", key))
	}
	if len(message.params) != 0 {
		panic(fmt.Sprintf("locale: %s requires arguments", key))
	}
	return message.one
}

// Format substitutes named, already-presentable arguments into a full message.
// Use Number and Percent for numeric values before passing them as arguments.
func Format(language Language, key string, args map[string]string) string {
	message := lookup(language, key)
	if message.plural {
		panic(fmt.Sprintf("locale: %s requires a count", key))
	}
	return interpolate(message.one, message.params, args, "")
}

// Plural selects singular for exactly 1 and plural for all other counts.
// The localized count is available in the template as {count}.
func Plural(language Language, key string, count int, args map[string]string) string {
	message := lookup(language, key)
	if !message.plural {
		panic(fmt.Sprintf("locale: %s is not plural", key))
	}
	text := message.other
	if count == 1 {
		text = message.one
	}
	return interpolate(text, message.params, args, Number(language, float64(count), 0))
}

// Message retains message identity and data so visible status can be rerendered
// after a language change without repeating its original action.
type Message struct {
	Key    string
	Args   map[string]string
	Count  int
	Plural bool
}

// Render presents a retained message in the selected language.
func (message Message) Render(language Language) string {
	if message.Plural {
		return Plural(language, message.Key, message.Count, message.Args)
	}
	if len(message.Args) != 0 {
		return Format(language, message.Key, message.Args)
	}
	return Text(language, message.Key)
}

// Number formats a presentation-only decimal with grouping and fixed precision.
// It must not be used for numeric input, persisted projects or compiled XML.
func Number(language Language, value float64, decimals int) string {
	if decimals < 0 {
		panic("locale: negative decimal precision")
	}
	raw := strconv.FormatFloat(value, 'f', decimals, 64)
	decimal, group := byte('.'), byte(',')
	if Normalize(string(language)) == PtBR {
		decimal, group = ',', '.'
	}
	integerEnd := strings.IndexByte(raw, '.')
	if integerEnd < 0 {
		integerEnd = len(raw)
	}
	start := 0
	if raw[0] == '-' || raw[0] == '+' {
		start = 1
	}
	var out strings.Builder
	out.Grow(len(raw) + (integerEnd-start-1)/3)
	out.WriteString(raw[:start])
	for i := start; i < integerEnd; i++ {
		if i > start && (integerEnd-i)%3 == 0 {
			out.WriteByte(group)
		}
		out.WriteByte(raw[i])
	}
	if decimals > 0 {
		out.WriteByte(decimal)
		out.WriteString(raw[integerEnd+1:])
	}
	return out.String()
}

// Percent formats a fraction (0.875 -> 87.5%) for display only.
func Percent(language Language, fraction float64, decimals int) string {
	return Number(language, fraction*100, decimals) + "%"
}

func lookup(language Language, key string) entry {
	message, ok := messages[Normalize(string(language))][key]
	if !ok {
		panic(fmt.Sprintf("locale: missing key %q for %s", key, language))
	}
	return message
}

func interpolate(text string, params map[string]bool, args map[string]string, count string) string {
	for name := range params {
		if name == "count" && count != "" {
			continue
		}
		if _, ok := args[name]; !ok {
			panic(fmt.Sprintf("locale: missing argument %q", name))
		}
	}
	var out strings.Builder
	out.Grow(len(text))
	for len(text) != 0 {
		at := strings.IndexByte(text, '{')
		if at < 0 {
			out.WriteString(text)
			break
		}
		out.WriteString(text[:at])
		text = text[at+1:]
		end := strings.IndexByte(text, '}')
		name := text[:end]
		if name == "count" && count != "" {
			out.WriteString(count)
		} else {
			out.WriteString(args[name])
		}
		text = text[end+1:]
	}
	return out.String()
}
