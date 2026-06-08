package goldenrules_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestRule10_NoLiteralMetadataKeysInWrites enforces the writer side of golden
// rule 10: action/task metadata keys MUST go through the MetaKey* constants in
// dispatch/execute.go, not raw string literals. The existing line-based scan
// in TestGoldenRules ("rule-10-metadata-via-constants") only catches the
// reader shape `metadata["…"]`. Writer-side `MergeActionMetadata(id,
// map[string]any{"…": …})` and `db.ActionMetadataMerge{Updates: map[string]any
// {"…": …}}` slip through that regex — that gap is exactly how
// dispatch/execute.go:executeRemote could land a literal "remote_session" key
// while the rule was reported clean.
//
// Detection is type-driven (mirrors rule15/rule19): we resolve db.Store and
// the db.ActionMetadataMerge struct, walk every non-test file in
// cmd/dispatch/tui, and flag any string-literal key inside a `map[string]any`
// composite literal that is either:
//   - the 2nd argument of a db.Store.MergeActionMetadata call, or
//   - the `Updates` field of a db.ActionMetadataMerge composite literal
//     (which is the per-entry map fed to db.Store.BulkMergeActionMetadata).
//
// Detection limits: only inline composite literals are inspected. A map built
// in a variable and then passed in (`m := map[string]any{"foo": 1};
// store.MergeActionMetadata(id, m)`) is not traced through the assignment.
// That shape does not appear in the codebase today; if it ever does, this
// rule should be extended rather than worked around.
func TestRule10_NoLiteralMetadataKeysInWrites(t *testing.T) {
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

	storeIface := findStoreInterface(t, pkgs)
	if storeIface == nil {
		t.Fatal("could not resolve db.Store interface; rule cannot run without it")
	}
	mergeStruct := findActionMetadataMergeType(pkgs)
	if mergeStruct == nil {
		t.Fatal("could not resolve db.ActionMetadataMerge; rule cannot run without it")
	}

	var violations []violation
	for _, pkg := range pkgs {
		for fileIdx, file := range pkg.Syntax {
			path := pkg.GoFiles[fileIdx]
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			scanFileForLiteralMetadataKeys(pkg, file, path, storeIface, mergeStruct, &violations)
		}
	}

	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("%s:%d: %s (Rule 10: replace the literal with a MetaKey* constant in dispatch/execute.go)", v.File, v.Line, v.Text)
		}
	}
}

// findActionMetadataMergeType returns the *types.Named for
// github.com/MH4GF/tq/db.ActionMetadataMerge, or nil.
func findActionMetadataMergeType(pkgs []*packages.Package) *types.Named {
	var out *types.Named
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.PkgPath != "github.com/MH4GF/tq/db" {
			return
		}
		obj := p.Types.Scope().Lookup("ActionMetadataMerge")
		if obj == nil {
			return
		}
		if named, ok := obj.Type().(*types.Named); ok {
			out = named
		}
	})
	return out
}

func scanFileForLiteralMetadataKeys(pkg *packages.Package, file *ast.File, path string, storeIface *types.Interface, mergeStruct *types.Named, out *[]violation) {
	rel := relPath(path)
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "MergeActionMetadata" {
				return true
			}
			selection := pkg.TypesInfo.Selections[sel]
			if selection == nil || selection.Kind() != types.MethodVal {
				return true
			}
			if !typeIsOrImplementsStore(selection.Recv(), storeIface) {
				return true
			}
			if len(node.Args) < 2 {
				return true
			}
			collectLiteralStringKeys(pkg, node.Args[1], rel,
				"MergeActionMetadata uses string-literal metadata key", out)
		case *ast.CompositeLit:
			t := pkg.TypesInfo.TypeOf(node)
			if t == nil || !types.Identical(t, mergeStruct) {
				return true
			}
			for _, elt := range node.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				id, ok := kv.Key.(*ast.Ident)
				if !ok || id.Name != "Updates" {
					continue
				}
				collectLiteralStringKeys(pkg, kv.Value, rel,
					"ActionMetadataMerge.Updates uses string-literal metadata key", out)
			}
		}
		return true
	})
}

// collectLiteralStringKeys reports any string-literal key inside expr when
// expr is a `map[string]any` composite literal. Non-map expressions, dynamic
// keys (identifiers, selectors, function calls), and non-string key types
// are ignored — only `*ast.BasicLit` of kind STRING qualifies as a "raw
// literal key" the rule is about.
func collectLiteralStringKeys(pkg *packages.Package, expr ast.Expr, rel, label string, out *[]violation) {
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return
	}
	if !isStringKeyedAnyMap(cl.Type) {
		return
	}
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		bl, ok := kv.Key.(*ast.BasicLit)
		if !ok || bl.Kind != token.STRING {
			continue
		}
		pos := pkg.Fset.Position(kv.Key.Pos())
		*out = append(*out, violation{
			File: rel,
			Line: pos.Line,
			Text: fmt.Sprintf("%s %s", label, bl.Value),
		})
	}
}

// isStringKeyedAnyMap reports whether the composite literal type expression is
// `map[string]any` (or its equivalent `map[string]interface{}`). The check is
// AST-shape only — the rule narrows the scan via the surrounding call/struct
// context, so accepting both spellings here is enough.
func isStringKeyedAnyMap(typeExpr ast.Expr) bool {
	mt, ok := typeExpr.(*ast.MapType)
	if !ok {
		return false
	}
	keyID, ok := mt.Key.(*ast.Ident)
	if !ok || keyID.Name != "string" {
		return false
	}
	switch v := mt.Value.(type) {
	case *ast.Ident:
		return v.Name == "any"
	case *ast.InterfaceType:
		return v.Methods == nil || len(v.Methods.List) == 0
	}
	return false
}
