package locale

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// Verify checks embedded catalogs and message keys used in Go source.
// Dynamic keys are rejected so new areas cannot evade coverage.
func Verify(source fs.FS) error {
	catalogs, err := fs.Sub(bundled, "catalog")
	if err != nil {
		return err
	}
	return Check(catalogs, source)
}

// Check validates discovered area pairs and locale calls against a source
// tree. Passing separate filesystems permits malformed-catalog tests.
func Check(catalogs, source fs.FS) error {
	messages, err := loadCatalogs(catalogs)
	if err != nil {
		return err
	}
	return fs.WalkDir(source, ".", func(name string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() {
			if name != "." && (strings.HasPrefix(item.Name(), ".") || item.Name() == "vendor" || item.Name() == "third_party" || item.Name() == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if path.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, 0)
		if err != nil {
			return err
		}
		aliases := map[string]bool{}
		for _, imported := range file.Imports {
			location, err := strconv.Unquote(imported.Path.Value)
			if err != nil || location != "zonebuilder/internal/locale" {
				continue
			}
			alias := "locale"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			aliases[alias] = true
		}
		var invalid error
		ast.Inspect(file, func(node ast.Node) bool {
			if invalid != nil {
				return false
			}
			switch n := node.(type) {
			case *ast.CallExpr:
				selector, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || !isLocaleSelector(selector, aliases) || len(n.Args) < 2 {
					return true
				}
				kind := selector.Sel.Name
				if kind != "Text" && kind != "Format" && kind != "Plural" {
					return true
				}
				invalid = checkLiteralKey(n.Args[1], kind == "Plural", messages, name)
			case *ast.CompositeLit:
				if isMessageType(n.Type, aliases) {
					invalid = checkMessageLiteral(n, messages, name)
				} else if mapType, ok := n.Type.(*ast.MapType); ok && isMessageType(mapType.Value, aliases) {
					for _, element := range n.Elts {
						field, ok := element.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						value, ok := field.Value.(*ast.CompositeLit)
						if ok && value.Type == nil {
							invalid = checkMessageLiteral(value, messages, name)
							if invalid != nil {
								break
							}
						}
					}
				}
			}
			return invalid == nil
		})
		return invalid
	})
}

func isLocaleSelector(selector *ast.SelectorExpr, aliases map[string]bool) bool {
	id, ok := selector.X.(*ast.Ident)
	return ok && aliases[id.Name]
}

func isMessageType(expr ast.Expr, aliases map[string]bool) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Message" && isLocaleSelector(selector, aliases)
}

func checkMessageLiteral(message *ast.CompositeLit, messages map[Language]map[string]entry, file string) error {
	var key ast.Expr
	plural := false
	for _, element := range message.Elts {
		field, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		id, ok := field.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch id.Name {
		case "Key":
			key = field.Value
		case "Plural":
			literal, ok := field.Value.(*ast.Ident)
			plural = ok && literal.Name == "true"
		}
	}
	if key != nil {
		return checkLiteralKey(key, plural, messages, file)
	}
	return nil // A zero-value Message represents the absence of a status.
}

func checkLiteralKey(expr ast.Expr, plural bool, messages map[Language]map[string]entry, file string) error {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return fmt.Errorf("locale: %s: dynamic key; use a literal key in each call or Message", file)
	}
	key, err := strconv.Unquote(literal.Value)
	if err != nil {
		return fmt.Errorf("locale: %s: %w", file, err)
	}
	message, ok := messages[PtBR][key]
	if !ok {
		return fmt.Errorf("locale: %s: untranslated key %s", file, key)
	}
	if message.plural != plural {
		return fmt.Errorf("locale: %s: incompatible plural variant for %s", file, key)
	}
	return nil
}
