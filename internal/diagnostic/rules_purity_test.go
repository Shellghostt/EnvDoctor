package diagnostic_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

func TestBuiltinRulesOnlyImportPureDependencies(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Ext(file) != ".go" || file == "fixture.go" || len(file) >= 8 && file[len(file)-8:] == "_test.go" {
			continue
		}
		t.Run(file, func(t *testing.T) { checkPureFile(t, file) })
	}
}

func checkPureFile(t *testing.T, file string) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"fmt": true, "sort": true, "strconv": true, "strings": true, "path": true, "regexp": true, "github.com/shellghostt/envdoctor/internal/collector": true}
	for _, i := range f.Imports {
		path, _ := strconv.Unquote(i.Path.Value)
		if !allowed[path] {
			t.Errorf("rule imports an impure or unreviewed dependency: %s", path)
		}
	}
	// The collector package exposes I/O constructors as well as snapshot types.
	// Importing it does not authorize rules to invoke those constructors.
	ast.Inspect(f, func(node ast.Node) bool {
		if selector, ok := node.(*ast.SelectorExpr); ok {
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "collector" && selector.Sel.Name != "Snapshot" {
				t.Errorf("rule refers to collector I/O or an unreviewed symbol: %s", selector.Sel.Name)
			}
		}
		return true
	})
}
