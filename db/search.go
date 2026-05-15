package db

import (
	"fmt"
	"strings"
)

type SearchResult struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	TaskID     int64  `json:"task_id"`
	ProjectID  int64  `json:"project_id"`
	Field      string `json:"field"`
	Snippet    string `json:"snippet"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

func extractSnippet(value, lowerKeyword string, keywordRuneLen, contextChars int) string {
	normalized := strings.ReplaceAll(value, "\n", " ")
	runes := []rune(normalized)
	lowerRunes := []rune(strings.ToLower(normalized))
	kwRunes := []rune(lowerKeyword)

	idx := runeIndex(lowerRunes, kwRunes)
	if idx < 0 {
		if len(runes) > contextChars*2 {
			return string(runes[:contextChars*2]) + "..."
		}
		return normalized
	}

	start := idx - contextChars
	end := idx + keywordRuneLen + contextChars

	var prefix, suffix string
	if start < 0 {
		start = 0
	} else {
		prefix = "..."
	}
	if end > len(runes) {
		end = len(runes)
	} else {
		suffix = "..."
	}

	return prefix + string(runes[start:end]) + suffix
}

func runeIndex(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// minFTSKeywordRunes is the trigram tokenizer's minimum match length. Keywords
// shorter than this cannot be matched by FTS5 trigram (a known regression from
// the old substring LIKE: 1-2 character keywords, e.g. "go" or 2-kanji 熟語,
// no longer hit). Proper CJK morphological tokenization is a separate task.
const minFTSKeywordRunes = 3

// ftsMatch builds a column-scoped FTS5 MATCH expression for the trigram
// tokenizer. The keyword is treated as a single substring phrase (trigram does
// the substring matching); embedded double-quotes are doubled per FTS5 string
// literal rules. column is an internal constant, never user input.
func ftsMatch(keyword, column string) string {
	escaped := strings.ReplaceAll(keyword, `"`, `""`)
	return `{` + column + `} : "` + escaped + `"`
}

func (db *DB) Search(keyword string, projectID int64) ([]SearchResult, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []SearchResult{}, nil
	}
	// trigram cannot match fewer than 3 characters; short-circuit instead of
	// issuing six MATCH queries that can only return zero rows.
	if len([]rune(keyword)) < minFTSKeywordRunes {
		return []SearchResult{}, nil
	}

	var projFilter string
	matchTitle := ftsMatch(keyword, "title")
	matchMetadata := ftsMatch(keyword, "metadata")
	matchResult := ftsMatch(keyword, "result")
	matchReason := ftsMatch(keyword, "reason")

	// Build args in branch order; each branch binds its MATCH expression and,
	// when filtering by project, the project id.
	var args []any
	addBranch := func(match string) {
		args = append(args, match)
		if projectID != 0 {
			args = append(args, projectID)
		}
	}
	if projectID != 0 {
		projFilter = " AND t.project_id = ?"
	}

	// SELECT lists repeat ids (t.id, t.id / e.entity_id, e.entity_id) by design
	// to keep the entity_id/task_id columns aligned across branches.
	//nolint:dupword
	query := `
		SELECT 'task' AS entity_type, t.id AS entity_id, t.id AS task_id, t.project_id, 'title' AS field, t.title AS value, t.status, t.created_at
		FROM tasks_fts JOIN tasks t ON t.id = tasks_fts.rowid WHERE tasks_fts MATCH ?` + projFilter + `
		UNION ALL
		SELECT 'task', t.id, t.id, t.project_id, 'metadata', t.metadata, t.status, t.created_at
		FROM tasks_fts JOIN tasks t ON t.id = tasks_fts.rowid WHERE tasks_fts MATCH ?` + projFilter + `
		UNION ALL
		SELECT 'action', a.id, a.task_id, t.project_id, 'title', a.title, a.status, a.created_at
		FROM actions_fts JOIN actions a ON a.id = actions_fts.rowid JOIN tasks t ON t.id = a.task_id WHERE actions_fts MATCH ?` + projFilter + `
		UNION ALL
		SELECT 'action', a.id, a.task_id, t.project_id, 'result', COALESCE(a.result, ''), a.status, a.created_at
		FROM actions_fts JOIN actions a ON a.id = actions_fts.rowid JOIN tasks t ON t.id = a.task_id WHERE actions_fts MATCH ?` + projFilter + `
		UNION ALL
		SELECT 'action', a.id, a.task_id, t.project_id, 'metadata', a.metadata, a.status, a.created_at
		FROM actions_fts JOIN actions a ON a.id = actions_fts.rowid JOIN tasks t ON t.id = a.task_id WHERE actions_fts MATCH ?` + projFilter + `
		UNION ALL
		SELECT 'task', e.entity_id, e.entity_id, t.project_id, 'status_history_reason',
		       json_extract(e.payload, '$.reason') AS value,
		       t.status, e.created_at
		FROM events_fts JOIN events e ON e.id = events_fts.rowid JOIN tasks t ON t.id = e.entity_id WHERE events_fts MATCH ?` + projFilter + `
		ORDER BY task_id DESC, entity_id DESC, created_at DESC
		LIMIT 500
	`
	addBranch(matchTitle)    // tasks.title
	addBranch(matchMetadata) // tasks.metadata
	addBranch(matchTitle)    // actions.title
	addBranch(matchResult)   // actions.result
	addBranch(matchMetadata) // actions.metadata
	addBranch(matchReason)   // events.reason

	if len(args) != strings.Count(query, "?") {
		panic(fmt.Sprintf("db.Search: arg/placeholder mismatch: %d args, %d placeholders", len(args), strings.Count(query, "?")))
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	lowerKeyword := strings.ToLower(keyword)
	keywordLen := len([]rune(keyword))
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var value string
		if err := rows.Scan(&r.EntityType, &r.EntityID, &r.TaskID, &r.ProjectID, &r.Field, &value, &r.Status, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("search scan: %w", err)
		}
		r.Snippet = extractSnippet(value, lowerKeyword, keywordLen, 40)
		r.CreatedAt = FormatLocal(r.CreatedAt)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search iterate: %w", err)
	}
	return results, nil
}
