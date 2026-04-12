package goldenrules_test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

type violation struct {
	File string
	Line int
	Text string
}

type scanConfig struct {
	Dirs        []string
	FilePattern string
	LineRegexp  *regexp.Regexp
}

func projectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func scanFiles(t *testing.T, root string, cfg scanConfig) []violation {
	t.Helper()
	var violations []violation
	for _, dir := range cfg.Dirs {
		absDir := filepath.Join(root, dir)
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "vendor" || name == ".claude" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			matched, _ := filepath.Match(cfg.FilePattern, filepath.Base(path))
			if !matched {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			lineNum := 0
			relPath, _ := filepath.Rel(root, path)
			for scanner.Scan() {
				lineNum++
				line := scanner.Text()
				if cfg.LineRegexp.MatchString(line) {
					violations = append(violations, violation{
						File: relPath,
						Line: lineNum,
						Text: strings.TrimSpace(line),
					})
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	return violations
}

func reportViolations(t *testing.T, violations []violation) {
	t.Helper()
	for _, v := range violations {
		t.Errorf("%s:%d: %s", v.File, v.Line, v.Text)
	}
}

func TestGoldenRules(t *testing.T) {
	root := projectRoot(t)
	upperLayers := []string{"cmd", "dispatch", "tui"}

	t.Run("rule-04-no-concrete-db-in-upper-layers", func(t *testing.T) {
		violations := scanFiles(t, root, scanConfig{
			Dirs:        upperLayers,
			FilePattern: "*.go",
			LineRegexp:  regexp.MustCompile(`\*db\.DB`),
		})
		if len(violations) > 0 {
			reportViolations(t, violations)
		}
	})

	t.Run("rule-06-no-mocks-in-db-tests", func(t *testing.T) {
		violations := scanFiles(t, root, scanConfig{
			Dirs:        []string{"db"},
			FilePattern: "*.go",
			LineRegexp:  regexp.MustCompile(`type\s+(?:mock|fake|Mock|Fake)\w+`),
		})
		if len(violations) > 0 {
			reportViolations(t, violations)
		}
	})

	t.Run("rule-09-custom-errors-implement-Unwrap", func(t *testing.T) {
		violations := checkErrorUnwrap(t, root)
		if len(violations) > 0 {
			for _, v := range violations {
				t.Errorf("%s:%d: type %s has no Unwrap() error method", v.File, v.Line, v.Text)
			}
		}
	})

	t.Run("rule-10-metadata-via-constants", func(t *testing.T) {
		violations := scanFiles(t, root, scanConfig{
			Dirs:        upperLayers,
			FilePattern: "*.go",
			LineRegexp:  regexp.MustCompile(`metadata\["`),
		})
		if len(violations) > 0 {
			reportViolations(t, violations)
		}
	})

	t.Run("rule-11-raw-sql-only-in-db", func(t *testing.T) {
		// Ceiling: 30 as of 2026-04-12. Lower this as violations are fixed.
		const ceiling = 30
		violations := scanFiles(t, root, scanConfig{
			Dirs:        upperLayers,
			FilePattern: "*.go",
			LineRegexp:  regexp.MustCompile(`"(SELECT|INSERT |UPDATE |DELETE FROM|CREATE TABLE)`),
		})
		count := len(violations)
		if count > ceiling {
			t.Errorf("rule-11 violations (%d) exceed ceiling (%d); regression detected", count, ceiling)
			reportViolations(t, violations)
		} else if count > 0 {
			t.Logf("rule-11: %d known violations (ceiling %d)", count, ceiling)
			for _, v := range violations {
				t.Logf("  %s:%d: %s", v.File, v.Line, v.Text)
			}
		}
	})
}

func checkErrorUnwrap(t *testing.T, root string) []violation {
	t.Helper()

	typePattern := regexp.MustCompile(`type\s+(\w+Error)\s+struct`)

	type errorType struct {
		Name   string
		PkgDir string
		File   string
		Line   int
	}
	var types []errorType

	// Pass 1: find all custom error types
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == ".claude" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNum := 0
		relPath, _ := filepath.Rel(root, path)
		for scanner.Scan() {
			lineNum++
			if matches := typePattern.FindStringSubmatch(scanner.Text()); matches != nil {
				types = append(types, errorType{
					Name:   matches[1],
					PkgDir: filepath.Dir(path),
					File:   relPath,
					Line:   lineNum,
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking for error types: %v", err)
	}

	// Pass 2: check each type has Unwrap method
	var violations []violation
	for _, et := range types {
		unwrapPattern := regexp.MustCompile(
			fmt.Sprintf(`func\s+\(\w+\s+\*%s\)\s+Unwrap\(\)\s+error`, regexp.QuoteMeta(et.Name)),
		)
		found := false
		err := filepath.WalkDir(et.PkgDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && path != et.PkgDir {
				return filepath.SkipDir
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if unwrapPattern.MatchString(scanner.Text()) {
					found = true
					return filepath.SkipAll
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("checking Unwrap for %s: %v", et.Name, err)
		}
		if !found {
			violations = append(violations, violation{
				File: et.File,
				Line: et.Line,
				Text: et.Name,
			})
		}
	}
	return violations
}
