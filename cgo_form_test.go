package aravis

// This file contains no cgo: Go forbids `import "C"` in _test.go files. It only
// parses the package's own source, which needs nothing but go/ast and go/parser.
//
// It guards the P6 error contract structurally. cgo lets any C call be written
// in a two-result form, `v, err := C.f(...)`, whose second value is errno — not
// a failure report. libc does not clear errno on success, so any such site can
// turn a successful call into a non-nil error as soon as an unrelated syscall
// failed earlier on the same thread. Only a GError may decide that an Aravis
// call failed, so the two-result form must not appear at all.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoTwoResultCgoCalls fails on every assignment of the form
// `a, b := C.something(...)` in the package's non-test sources.
func TestNoTwoResultCgoCalls(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("cannot read the package directory: %v", err)
	}

	fset := token.NewFileSet()
	scanned := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}

		scanned++

		ast.Inspect(file, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
				return true
			}

			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "C" {
				return true
			}

			t.Errorf("%s: two-result form of C.%s: the second value is errno, not an error. "+
				"errno survives a successful call, so this reports failures that never happened. "+
				"Use the single-result form and let a GError decide.",
				fset.Position(assign.Pos()), selector.Sel.Name)

			return true
		})
	}

	// Without this the test would pass vacuously if the directory walk ever
	// stopped finding the package's sources.
	if scanned == 0 {
		t.Fatal("no non-test .go files were scanned; the guard would pass vacuously")
	}
}
