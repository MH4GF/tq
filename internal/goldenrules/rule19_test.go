package goldenrules_test

import (
	"bufio"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// TestRule19_NoTestOnlyStoreMethods enforces golden rule 19: every db.Store
// interface method (except the intentionally test-only TestHelper
// sub-interface) MUST have at least one non-test caller. A method whose only
// caller is its own self-test slips past Rule 13's `deadcode -test` because
// RTA marks it live (the test reaches it AND interface dispatch on the live
// *db.DB type over-approximates), yet it is dead from production's perspective
// (incident: PR #267, db.Store.ListTasksByProject — sole caller was
// TestListTasksByProject).
//
// "Non-test caller" spans the whole module, NOT just cmd/dispatch/tui/main:
// a Store method invoked internally by another db/ method (e.g.
// GetTaskActionCount called from db/task.go) IS exercised in production
// transitively, so excluding db/ would produce false positives. The rule
// only fires when there is ZERO non-`_test.go` caller anywhere — i.e. the
// method is reachable solely through test code.
//
// Detection is type-driven (mirrors Rule 15): we resolve the db.Store and
// db.TestHelper interfaces via go/types, then walk every non-test CallExpr
// and record method names whose Selection receiver is or implements db.Store.
// Any Store method (minus the TestHelper set) absent from that called-set is a
// violation. The check is direct only; a thin wrapper that forwards to a
// Store method still counts as a caller (the wrapper IS a non-test use).
// Allowlisted exceptions live in .goldenrules-rule19-allowlist.
func TestRule19_NoTestOnlyStoreMethods(t *testing.T) {
	root := projectRoot(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps |
			packages.NeedImports,
		Dir:   root,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
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
	testHelperMethods := dbInterfaceMethodNames(pkgs, "TestHelper")
	if len(testHelperMethods) == 0 {
		t.Fatal("could not resolve db.TestHelper methods; rule cannot run without the exclusion set")
	}

	// Store.Methods() is fully expanded, so embedded CommandWriter/
	// QueryReader/TestHelper/Migrate/Close methods are all present here.
	wantCaller := map[string]bool{}
	for m := range storeIface.Methods() {
		if testHelperMethods[m.Name()] {
			continue
		}
		wantCaller[m.Name()] = true
	}

	called := map[string]bool{}
	for _, pkg := range pkgs {
		for fileIdx, file := range pkg.Syntax {
			path := pkg.GoFiles[fileIdx]
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			collectStoreMethodCalls(pkg, file, storeIface, called)
		}
	}

	allowlist := loadRule19Allowlist(t, root)

	var dead, staleAllow []string
	for name := range wantCaller {
		if called[name] || allowlist[name] {
			continue
		}
		dead = append(dead, name)
	}
	for name := range allowlist {
		// Stale: the method regained a production caller, or no longer exists
		// on db.Store (renamed/removed). Either way the line must go.
		if called[name] || !wantCaller[name] {
			staleAllow = append(staleAllow, name)
		}
	}
	sort.Strings(dead)
	sort.Strings(staleAllow)

	for _, name := range dead {
		t.Errorf("db.Store.%s has no non-test caller anywhere in the module; it is reachable only "+
			"through its own self-test, so Rule 13's `deadcode -test` keeps it live while it is dead "+
			"from production. Remove it, or if it is a deliberate seam add it to "+
			".goldenrules-rule19-allowlist with a reason.", name)
	}
	for _, name := range staleAllow {
		t.Errorf(".goldenrules-rule19-allowlist entry %q is stale (method now has a production caller or no "+
			"longer exists on db.Store); remove the line.", name)
	}
}

// dbInterfaceMethodNames returns the fully-expanded method-name set of the
// named interface in github.com/MH4GF/tq/db (e.g. "TestHelper").
func dbInterfaceMethodNames(pkgs []*packages.Package, ifaceName string) map[string]bool {
	out := map[string]bool{}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.PkgPath != "github.com/MH4GF/tq/db" {
			return
		}
		obj := p.Types.Scope().Lookup(ifaceName)
		if obj == nil {
			return
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			return
		}
		iface, ok := named.Underlying().(*types.Interface)
		if !ok {
			return
		}
		for m := range iface.Methods() {
			out[m.Name()] = true
		}
	})
	return out
}

// collectStoreMethodCalls records into called the name of every method
// invocation in file whose receiver is or implements db.Store.
func collectStoreMethodCalls(pkg *packages.Package, file *ast.File, storeIface *types.Interface, called map[string]bool) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		selection := pkg.TypesInfo.Selections[sel]
		if selection == nil || selection.Kind() != types.MethodVal {
			return true
		}
		if typeIsOrImplementsStore(selection.Recv(), storeIface) {
			called[sel.Sel.Name] = true
		}
		return true
	})
}

// loadRule19Allowlist reads .goldenrules-rule19-allowlist (one Store method
// name per line; blank lines and # comments ignored).
func loadRule19Allowlist(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	f, err := os.Open(filepath.Join(root, ".goldenrules-rule19-allowlist"))
	if err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("opening rule19 allowlist: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[line] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading rule19 allowlist: %v", err)
	}
	return out
}
