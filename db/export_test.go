package db

import "strings"

func ExportExtractSnippet(value, keyword string, contextChars int) string {
	return extractSnippet(value, strings.ToLower(keyword), len([]rune(keyword)), contextChars)
}

func ExportFTSMatch(keyword, column string) string {
	return ftsMatch(keyword, column)
}
