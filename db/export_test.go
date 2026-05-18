package db

import "strings"

func ExportExtractSnippet(value, keyword string, contextChars int) string {
	return extractSnippet(value, strings.ToLower(keyword), len([]rune(keyword)), contextChars)
}

func ExportFTSMatch(keyword, column string) string {
	return ftsMatch(keyword, column)
}

// ExportDepSatisfied exposes the Go-side dependency-satisfaction verdict.
func ExportDepSatisfied(depType, blockerStatus string) bool {
	return depSatisfied(depType, blockerStatus)
}

// ExportDependencyGateAllows exposes the SQL-side verdict: whether
// dependencyGatePredicate admits actionID (i.e. all its blockers are
// satisfied). It evaluates the predicate in isolation, free of the
// project-dispatch and time gates that NextPending/CountPendingByDispatch
// also apply, so it is an exact mirror of depSatisfied for parity testing.
func ExportDependencyGateAllows(d *DB, actionID int64) (bool, error) {
	var allowed bool
	err := d.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM actions a WHERE a.id = ? AND `+dependencyGatePredicate+`)`,
		actionID,
	).Scan(&allowed)
	return allowed, err
}
