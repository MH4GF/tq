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

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (db *DB) Search(keyword string, projectID int64) ([]SearchResult, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []SearchResult{}, nil
	}
	escaped := escapeLike(keyword)

	var projFilter string
	branchArgs := []any{escaped}
	if projectID != 0 {
		projFilter = " AND t.project_id = ?"
		branchArgs = append(branchArgs, projectID)
	}

	//nolint:dupword
	query := `
		SELECT 'task' AS entity_type, t.id AS entity_id, t.id AS task_id, t.project_id, 'title' AS field, t.title AS value, t.status, t.created_at
		FROM tasks t WHERE t.title LIKE '%' || ? || '%' ESCAPE '\'` + projFilter + `
		UNION ALL
		SELECT 'task', t.id, t.id, t.project_id, 'metadata', t.metadata, t.status, t.created_at
		FROM tasks t WHERE t.metadata LIKE '%' || ? || '%' ESCAPE '\'` + projFilter + `
		UNION ALL
		SELECT 'action', a.id, a.task_id, t.project_id, 'title', a.title, a.status, a.created_at
		FROM actions a JOIN tasks t ON a.task_id = t.id WHERE a.title LIKE '%' || ? || '%' ESCAPE '\'` + projFilter + `
		UNION ALL
		SELECT 'action', a.id, a.task_id, t.project_id, 'result', COALESCE(a.result, ''), a.status, a.created_at
		FROM actions a JOIN tasks t ON a.task_id = t.id WHERE COALESCE(a.result, '') LIKE '%' || ? || '%' ESCAPE '\'` + projFilter + `
		UNION ALL
		SELECT 'action', a.id, a.task_id, t.project_id, 'metadata', a.metadata, a.status, a.created_at
		FROM actions a JOIN tasks t ON a.task_id = t.id WHERE a.metadata LIKE '%' || ? || '%' ESCAPE '\'` + projFilter + `
		ORDER BY task_id DESC, entity_id DESC
		LIMIT 500
	`
	var args []any
	for range 5 {
		args = append(args, branchArgs...)
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
