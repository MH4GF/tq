package tui

import "strings"

type HelpKey struct {
	Key  string
	Desc string
}

func detailHelpKeys() []HelpKey {
	return []HelpKey{
		{"j/k", "scroll"},
		{"esc/q", "back"},
	}
}

func commonHelpKeys() []HelpKey {
	return []HelpKey{
		{"j/k", "navigate"},
		{"tab", "switch"},
		{"esc/q", "quit"},
	}
}

func formatHelp(keys []HelpKey) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = styleHelpKey.Render(k.Key + ": " + k.Desc)
	}
	return strings.Join(parts, "  ")
}

type visibleRange struct {
	start, end int
}

func calcVisibleRange(cursor, total, height, headerRows int) visibleRange {
	maxVisible := height - headerRows
	if maxVisible <= 0 {
		maxVisible = 20
	}
	if total <= maxVisible {
		return visibleRange{0, total}
	}
	start := max(cursor-maxVisible/2, 0)
	end := start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return visibleRange{start, end}
}
