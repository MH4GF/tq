package goldenrules_test

import (
	"bufio"
	"database/sql"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/MH4GF/tq/testutil"
)

type rule17Query struct {
	File string
	Line int
	SQL  string
}

var (
	rule17ScanRegex      = regexp.MustCompile(`^SCAN\s+(\w+)`)
	rule17PrintfVerb     = regexp.MustCompile(`%[+\-#0 ]*\d*(?:\.\d+)?[a-zA-Z%]`)
	rule17DMLPrefix      = regexp.MustCompile(`(?i)^\s*(SELECT|INSERT|UPDATE|DELETE|REPLACE)\b`)
	rule17QueryMethodArg = map[string]int{
		"Query":           0,
		"QueryRow":        0,
		"Exec":            0,
		"QueryContext":    1,
		"QueryRowContext": 1,
		"ExecContext":     1,
	}
)

func checkRule17(t *testing.T, root string) {
	t.Helper()
	queries := rule17ExtractQueries(t, root)
	d := testutil.NewTestDB(t)

	var violations []string
	skipped := 0
	for _, q := range queries {
		if !rule17DMLPrefix.MatchString(q.SQL) {
			continue
		}
		scans, err := rule17Explain(d.DB, q.SQL)
		if err != nil {
			skipped++
			continue
		}
		for _, target := range scans {
			violations = append(violations, fmt.Sprintf("%s:%d SCAN %s", q.File, q.Line, target))
		}
	}
	sort.Strings(violations)
	violations = slices.Compact(violations)

	allowlistPath := filepath.Join(root, ".goldenrules-rule17-allowlist")
	expected, err := rule17ReadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("read allowlist %s: %v", allowlistPath, err)
	}

	newFindings := rule17Diff(violations, expected)
	staleEntries := rule17Diff(expected, violations)

	if len(newFindings) > 0 {
		t.Errorf("rule-17: new SCAN findings (not in .goldenrules-rule17-allowlist):")
		for _, v := range newFindings {
			t.Errorf("  + %s", v)
		}
		t.Errorf("If the SCAN is unavoidable (e.g., full-table list with no WHERE),")
		t.Errorf("add the line(s) above to .goldenrules-rule17-allowlist.")
		t.Errorf("Otherwise, add an index or rewrite the query so EXPLAIN reports SEARCH ... USING INDEX.")
	}
	if len(staleEntries) > 0 {
		t.Errorf("rule-17: stale entries in .goldenrules-rule17-allowlist (no longer reported):")
		for _, v := range staleEntries {
			t.Errorf("  - %s", v)
		}
		t.Errorf("Remove these lines from .goldenrules-rule17-allowlist.")
	}
	if len(newFindings) == 0 && len(staleEntries) == 0 {
		t.Logf("rule-17: %d allowlisted SCANs, %d queries skipped (EXPLAIN unparseable)", len(expected), skipped)
	}
}

func rule17ExtractQueries(t *testing.T, root string) []rule17Query {
	t.Helper()
	dir := filepath.Join(root, "db")
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}

	consts := make(map[string]string)
	for _, pkgName := range sortedKeys(pkgs) {
		pkg := pkgs[pkgName]
		for _, filePath := range sortedKeys(pkg.Files) {
			for _, decl := range pkg.Files[filePath].Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.CONST {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, name := range vs.Names {
						if i >= len(vs.Values) {
							continue
						}
						if v, ok := rule17EvalString(vs.Values[i], nil, consts); ok {
							consts[name.Name] = v
						}
					}
				}
			}
		}
	}

	var refs []rule17Query
	for _, pkgName := range sortedKeys(pkgs) {
		pkg := pkgs[pkgName]
		for _, filePath := range sortedKeys(pkg.Files) {
			file := pkg.Files[filePath]
			relPath, relErr := filepath.Rel(root, filePath)
			if relErr != nil {
				relPath = filePath
			}
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				locals := rule17CollectLocals(fd.Body, consts)
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					argIdx, ok := rule17QueryMethodArg[sel.Sel.Name]
					if !ok {
						return true
					}
					if argIdx >= len(call.Args) {
						return true
					}
					sql, ok := rule17EvalString(call.Args[argIdx], locals, consts)
					if !ok {
						return true
					}
					pos := fset.Position(call.Pos())
					refs = append(refs, rule17Query{File: relPath, Line: pos.Line, SQL: sql})
					return true
				})
			}
		}
	}
	return refs
}

func rule17CollectLocals(body *ast.BlockStmt, consts map[string]string) map[string]string {
	locals := make(map[string]string)
	tryBind := func(name string, value ast.Expr) {
		if _, exists := locals[name]; exists {
			return
		}
		if v, ok := rule17EvalString(value, locals, consts); ok {
			locals[name] = v
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		// Don't descend into nested function literals — their locals belong
		// to a different scope.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
				return true
			}
			id, ok := s.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			tryBind(id.Name, s.Rhs[0])
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.VAR && gd.Tok != token.CONST) {
				return true
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i < len(vs.Values) {
						tryBind(name.Name, vs.Values[i])
						continue
					}
					if id, ok := vs.Type.(*ast.Ident); ok && id.Name == "string" {
						if _, exists := locals[name.Name]; !exists {
							locals[name.Name] = ""
						}
					}
				}
			}
		}
		return true
	})
	return locals
}

func rule17EvalString(expr ast.Expr, locals, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return s, true
	case *ast.Ident:
		if v, ok := locals[e.Name]; ok {
			return v, true
		}
		if v, ok := consts[e.Name]; ok {
			return v, true
		}
		return "", false
	case *ast.ParenExpr:
		return rule17EvalString(e.X, locals, consts)
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		l, lok := rule17EvalString(e.X, locals, consts)
		r, rok := rule17EvalString(e.Y, locals, consts)
		if !lok || !rok {
			return "", false
		}
		return l + r, true
	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != "fmt" {
			return "", false
		}
		if sel.Sel.Name != "Sprintf" && sel.Sel.Name != "Sprint" {
			return "", false
		}
		if len(e.Args) == 0 {
			return "", false
		}
		format, ok := rule17EvalString(e.Args[0], locals, consts)
		if !ok {
			return "", false
		}
		return rule17PrintfVerb.ReplaceAllStringFunc(format, func(m string) string {
			if m == "%%" {
				return "%"
			}
			return "?"
		}), true
	}
	return "", false
}

func rule17Explain(db *sql.DB, query string) ([]string, error) {
	n := strings.Count(query, "?")
	args := make([]any, n)
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var scans []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			return nil, err
		}
		if m := rule17ScanRegex.FindStringSubmatch(detail); m != nil {
			scans = append(scans, m[1])
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return scans, nil
}

func rule17ReadAllowlist(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return slices.Compact(out), nil
}

func rule17Diff(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := set[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
