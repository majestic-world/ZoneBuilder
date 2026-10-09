package locale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"strings"
)

type entry struct {
	one, other string
	plural     bool
	params     map[string]bool
}

// loadCatalogs discovers all area directories and validates each language pair.
// Parsing happens once at startup, never once per frame or message lookup.
func loadCatalogs(catalogs fs.FS) (map[Language]map[string]entry, error) {
	all := map[Language]map[string]entry{PtBR: {}, En: {}}
	areas, err := fs.ReadDir(catalogs, ".")
	if err != nil {
		return nil, err
	}
	for _, area := range areas {
		if !area.IsDir() {
			return nil, fmt.Errorf("locale: unexpected catalog entry %s", area.Name())
		}
		pairs := make(map[Language]map[string]entry, 2)
		files, err := fs.ReadDir(catalogs, area.Name())
		if err != nil {
			return nil, err
		}
		for _, language := range []Language{PtBR, En} {
			name := path.Join(area.Name(), string(language)+".json")
			data, err := fs.ReadFile(catalogs, name)
			if err != nil {
				return nil, fmt.Errorf("locale: missing %s: %w", name, err)
			}
			pair, err := parseCatalog(data, area.Name())
			if err != nil {
				return nil, fmt.Errorf("locale: %s: %w", name, err)
			}
			pairs[language] = pair
		}
		for _, file := range files {
			if file.IsDir() || file.Name() != "pt-BR.json" && file.Name() != "en.json" {
				return nil, fmt.Errorf("locale: unexpected catalog file %s/%s", area.Name(), file.Name())
			}
		}
		for key, pt := range pairs[PtBR] {
			en, ok := pairs[En][key]
			if !ok {
				return nil, fmt.Errorf("locale: missing English translation for %s", key)
			}
			if pt.plural != en.plural || !maps.Equal(pt.params, en.params) {
				return nil, fmt.Errorf("locale: incompatible variants or parameters for %s", key)
			}
		}
		for key := range pairs[En] {
			if _, ok := pairs[PtBR][key]; !ok {
				return nil, fmt.Errorf("locale: missing Portuguese translation for %s", key)
			}
		}
		for language, pair := range pairs {
			for key, message := range pair {
				if _, exists := all[language][key]; exists {
					return nil, fmt.Errorf("locale: duplicate key %s", key)
				}
				all[language][key] = message
			}
		}
	}
	return all, nil
}

func parseCatalog(data []byte, area string) (map[string]entry, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, fmt.Errorf("expected JSON object: %v", err)
	}
	catalog := make(map[string]entry)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key := token.(string)
		if !strings.HasPrefix(key, area+".") || len(key) == len(area)+1 {
			return nil, fmt.Errorf("key %q must start with %s.", key, area)
		}
		if _, found := catalog[key]; found {
			return nil, fmt.Errorf("duplicate key %s", key)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		value, err := parseEntry(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		catalog[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing catalog content: %v", err)
	}
	return catalog, nil
}

func parseEntry(raw json.RawMessage) (entry, error) {
	var result entry
	if len(raw) == 0 {
		return result, fmt.Errorf("empty message")
	}
	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &result.one); err != nil {
			return result, err
		}
	} else {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		start, err := decoder.Token()
		if err != nil || start != json.Delim('{') {
			return result, fmt.Errorf("expected plural object: %v", err)
		}
		seen := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return result, err
			}
			variant := token.(string)
			if seen[variant] {
				return result, fmt.Errorf("duplicate plural variant %s", variant)
			}
			seen[variant] = true
			if variant != "one" && variant != "other" {
				return result, fmt.Errorf("unknown plural variant %s", variant)
			}
			var text string
			if err := decoder.Decode(&text); err != nil {
				return result, err
			}
			if variant == "one" {
				result.one = text
			} else {
				result.other = text
			}
		}
		if _, err := decoder.Token(); err != nil {
			return result, err
		}
		if !seen["one"] || !seen["other"] {
			return result, fmt.Errorf("plural requires one and other variants")
		}
		result.plural = true
	}
	if result.one == "" || result.plural && result.other == "" {
		return result, fmt.Errorf("empty translation")
	}
	params, err := placeholders(result.one)
	if err != nil {
		return result, err
	}
	if result.plural {
		other, err := placeholders(result.other)
		if err != nil {
			return result, err
		}
		if !maps.Equal(params, other) {
			return result, fmt.Errorf("plural variants have different parameters")
		}
	}
	result.params = params
	return result, nil
}

func placeholders(text string) (map[string]bool, error) {
	params := map[string]bool{}
	for len(text) > 0 {
		at := strings.IndexAny(text, "{}")
		if at < 0 {
			break
		}
		if text[at] != '{' {
			return nil, fmt.Errorf("unexpected closing brace")
		}
		text = text[at+1:]
		end := strings.IndexByte(text, '}')
		if end < 0 {
			return nil, fmt.Errorf("unclosed argument")
		}
		name := text[:end]
		if name == "" || !isASCIIName(name) {
			return nil, fmt.Errorf("invalid argument %q", name)
		}
		params[name] = true
		text = text[end+1:]
	}
	return params, nil
}

func isASCIIName(name string) bool {
	for i, r := range name {
		if i == 0 && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
		if i > 0 && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
