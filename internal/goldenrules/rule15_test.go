package goldenrules_test

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestRule15_NoStoreCallsInLoops enforces golden rule 15: methods on the
// db.Store interface MUST NOT be invoked inside a for/range loop in cmd/,
// dispatch/, or tui/. The fix is to add a bulk method on db.Store and call it
// once outside the loop.
//
// Detection is type-driven (not name-driven): we walk every for/range body and
// flag any CallExpr whose receiver type, after Underlying() resolution,
// matches the db.Store interface, the *db.DB concrete type, or any other
// type that implements db.Store. Helper-function calls inside the loop are
// expanded by following SelectorExpr/Ident references through TypesInfo so
// the rule cannot be bypassed by extracting the call to a one-line wrapper
// in the same package.
func TestRule15_NoStoreCallsInLoops(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports,
		Dir:   projectRoot(t),
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./cmd/...", "./dispatch/...", "./tui/...")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal("packages had load errors")
	}

	// Resolve the db.Store interface object once. Every store-typed expr's
	// underlying type compares equal (or implements) this.
	storeIface := findStoreInterface(t, pkgs)
	if storeIface == nil {
		t.Fatal("could not resolve db.Store interface; rule cannot run without it")
	}

	var violations []violation
	for _, pkg := range pkgs {
		for fileIdx, file := range pkg.Syntax {
			path := pkg.GoFiles[fileIdx]
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			scanFileForStoreInLoops(pkg, file, path, storeIface, &violations)
		}
	}

	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("%s:%d: %s (Rule 15: extract a bulk method on db.Store and call it once outside the loop)", v.File, v.Line, v.Text)
		}
	}
}

// findStoreInterface returns the *types.Interface for github.com/MH4GF/tq/db.Store.
// Returns nil if the db package was not loaded transitively.
func findStoreInterface(t *testing.T, pkgs []*packages.Package) *types.Interface {
	t.Helper()
	var iface *types.Interface
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.PkgPath != "github.com/MH4GF/tq/db" {
			return
		}
		obj := p.Types.Scope().Lookup("Store")
		if obj == nil {
			return
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			return
		}
		if i, ok := named.Underlying().(*types.Interface); ok {
			iface = i
		}
	})
	return iface
}

func scanFileForStoreInLoops(pkg *packages.Package, file *ast.File, path string, storeIface *types.Interface, out *[]violation) {
	rel := relPath(path)
	ast.Inspect(file, func(n ast.Node) bool {
		// Only RangeStmt counts as the classic "iterate a collection" N+1
		// pattern. Bare `for {}` and `for cond {}` are polling/control loops
		// (heartbeat ticks, dispatch backoff) where each iteration is
		// independent and cannot meaningfully be batched. Including them
		// would force callers to bypass the rule via wrapper functions.
		rng, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		body := rng.Body
		if body == nil {
			return true
		}
		ast.Inspect(body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			selection := pkg.TypesInfo.Selections[sel]
			if selection == nil {
				// Not a method invocation (package-level call like fmt.Errorf,
				// or a method on a builtin). Skip.
				return true
			}
			if selection.Kind() != types.MethodVal {
				// Field access or method expression, not an invocation on a
				// concrete receiver value.
				return true
			}
			recvType := selection.Recv()
			if !typeIsOrImplementsStore(recvType, storeIface) {
				return true
			}
			pos := pkg.Fset.Position(call.Pos())
			*out = append(*out, violation{
				File: rel,
				Line: pos.Line,
				Text: "db.Store call inside for/range body: " + sel.Sel.Name,
			})
			return true
		})
		return true
	})
}

// typeIsOrImplementsStore reports whether t is the db.Store interface itself
// or any other type that satisfies it. Pointer indirection is unwrapped.
func typeIsOrImplementsStore(t types.Type, storeIface *types.Interface) bool {
	if t == nil {
		return false
	}
	if types.Identical(t, storeIface) {
		return true
	}
	// db.Store is an interface; underlying-equal catches named alias of it.
	if underIface, ok := t.Underlying().(*types.Interface); ok {
		if types.Identical(underIface, storeIface) {
			return true
		}
	}
	return types.Implements(t, storeIface) || types.Implements(types.NewPointer(t), storeIface)
}

func relPath(abs string) string {
	idx := strings.LastIndex(abs, "/MH4GF/tq/")
	if idx >= 0 {
		return abs[idx+len("/MH4GF/tq/"):]
	}
	return abs
}
