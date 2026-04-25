package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/MH4GF/tq/db"
)

// ── Color palette (256-color) ──

var (
	// Surface / background
	colorSurface   = lipgloss.Color("234") // panels, header, help bar
	colorHighlight = lipgloss.Color("237") // cursor row background
	colorBorder    = lipgloss.Color("240") // card borders

	// Text hierarchy
	colorTextSecondary = lipgloss.Color("245") // secondary
	colorTextMuted     = lipgloss.Color("243") // dimmed

	// Semantic status
	colorRunning = lipgloss.Color("75")  // #5fafff blue
	colorPending = lipgloss.Color("179") // #d7af5f amber
	colorDone    = lipgloss.Color("114") // #87d787 green
	colorFailed  = lipgloss.Color("204") // #ff5f87 red-pink
	colorAccent  = lipgloss.Color("80")  // #5fd7d7 teal
	colorWarning = lipgloss.Color("213") // magenta

	colorDoneDim   = lipgloss.Color("65")  // dim green for done actions
	colorFailedDim = lipgloss.Color("131") // dim red for failed actions
	colorInactive  = lipgloss.Color("242") // gray for archived/cancelled
)

// ── Status styles ──

var (
	stylePending    = lipgloss.NewStyle().Foreground(colorPending)
	styleRunning    = lipgloss.NewStyle().Foreground(colorRunning)
	styleDispatched = lipgloss.NewStyle().Foreground(colorAccent)
	styleDone       = lipgloss.NewStyle().Foreground(colorDone)
	styleFailed     = lipgloss.NewStyle().Foreground(colorFailed)
	styleWarning    = lipgloss.NewStyle().Foreground(colorWarning)
	styleMuted      = lipgloss.NewStyle().Foreground(colorTextMuted)

	styleDoneDim   = lipgloss.NewStyle().Foreground(colorDoneDim)
	styleFailedDim = lipgloss.NewStyle().Foreground(colorFailedDim)
	styleInactive  = lipgloss.NewStyle().Foreground(colorInactive)
)

// ── UI component styles ──

var (
	// Header / tabs
	styleHeaderBar   = lipgloss.NewStyle().Background(colorSurface)
	styleTabActive   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Background(lipgloss.Color("236"))
	styleTabInactive = lipgloss.NewStyle().Foreground(colorTextMuted)

	// Status strip
	styleStatusStrip = lipgloss.NewStyle().Background(lipgloss.Color("233"))
	styleStatNum     = lipgloss.NewStyle().Bold(true)
	styleStatLabel   = lipgloss.NewStyle().Foreground(colorTextMuted)

	// Project cards
	styleBorderChar  = lipgloss.NewStyle().Foreground(colorBorder)
	styleProjectName = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleBadge       = lipgloss.NewStyle().Foreground(colorTextMuted)
	styleWorkDir     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Help bar
	styleHelpBar = lipgloss.NewStyle().Background(colorSurface)
	styleHelpKey = lipgloss.NewStyle().Italic(true).Foreground(colorTextSecondary)

	// Activity
	styleActivityTS  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleActivityMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	// Detail view
	styleDetailBack = lipgloss.NewStyle().Foreground(colorTextMuted)
	styleFieldLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleFieldValue = lipgloss.NewStyle().Foreground(colorTextSecondary)

	// Gauge bar segments
	styleGaugeRunning = lipgloss.NewStyle().Foreground(colorRunning)
	styleGaugePending = lipgloss.NewStyle().Foreground(colorPending)
	styleGaugeDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("28")) // visible dark green
	styleGaugeFailed  = lipgloss.NewStyle().Foreground(colorFailed)

	// Schedule table
	styleTableHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func StatusStyle(status string) lipgloss.Style {
	switch status {
	case db.ActionStatusPending:
		return stylePending
	case db.ActionStatusRunning:
		return styleRunning
	case db.ActionStatusDone:
		return styleDone
	case db.ActionStatusFailed:
		return styleFailed
	case db.ActionStatusDispatched:
		return styleDispatched
	case db.ActionStatusCancelled, db.TaskStatusArchived:
		return styleInactive
	default:
		return styleMuted
	}
}

func StatusDimStyle(status string) lipgloss.Style {
	switch status {
	case db.ActionStatusDone:
		return styleDoneDim
	case db.ActionStatusFailed:
		return styleFailedDim
	case db.ActionStatusCancelled, db.TaskStatusArchived:
		return styleInactive
	default:
		return StatusStyle(status)
	}
}

func truncateResult(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func RenderDetailView(a *db.Action, scroll, width, height int) string {
	var b strings.Builder
	pad := "  " // 2-col left margin
	bodyW := max(0, min(width, 80)-len(pad))

	// Top padding
	b.WriteString("\n")

	// Header strip: ← esc  title  status icon + status
	st := StatusStyle(a.Status)
	icon := StatusIcon(a.Status)
	headerLine := fmt.Sprintf("%s%s  %s  %s %s",
		pad,
		styleDetailBack.Render("← esc"),
		lipgloss.NewStyle().Bold(true).Render(a.Title),
		st.Render(icon),
		st.Render(a.Status),
	)
	b.WriteString(headerLine + "\n")
	b.WriteString(pad + styleBorderChar.Render(strings.Repeat("─", bodyW)) + "\n")

	// Metadata grid
	fields := []struct{ label, value string }{
		{"   Action", fmt.Sprintf("#%d", a.ID)},
		{"     Task", fmt.Sprintf("#%d", a.TaskID)},
	}
	if a.CompletedAt.Valid {
		fields = append(fields, struct{ label, value string }{"Completed", db.FormatLocal(a.CompletedAt.String)})
	}
	for _, f := range fields {
		fmt.Fprintf(&b, "%s%s  %s\n",
			pad,
			styleFieldLabel.Render(f.label),
			styleFieldValue.Render(f.value),
		)
	}
	b.WriteString(pad + styleBorderChar.Render(strings.Repeat("─", bodyW)) + "\n")

	// Result body with word wrap
	result := ""
	if a.Result.Valid {
		result = a.Result.String
	}
	var lines []string
	for rawLine := range strings.SplitSeq(result, "\n") {
		lines = append(lines, wrapLine(rawLine, bodyW)...)
	}

	// top(1) + header(1) + separator(1) + fields + separator(1) + padding before help(1) = 5 + len(fields)
	chromeLines := 5 + len(fields)
	bodyHeight := height - chromeLines
	if bodyHeight < 1 {
		bodyHeight = 10
	}

	if scroll > len(lines)-bodyHeight {
		scroll = max(0, len(lines)-bodyHeight)
	}

	end := min(scroll+bodyHeight, len(lines))
	for _, line := range lines[scroll:end] {
		b.WriteString(pad + line + "\n")
	}

	// Scroll indicator
	if len(lines) > bodyHeight {
		page := scroll/bodyHeight + 1
		totalPages := (len(lines)-1)/bodyHeight + 1
		indicator := styleMuted.Render(fmt.Sprintf("[%d/%d]", page, totalPages))
		b.WriteString(pad + indicator)
	}

	return b.String()
}

func RenderTaskDetailView(t *db.Task, history []db.TaskStatusHistoryEntry, notes []db.TaskNoteEntry, scroll, width, height int) string {
	pad := "  "
	bodyW := max(0, min(width, 80)-len(pad))

	var lines []string

	// Metadata grid
	fields := []struct{ label, value string }{
		{"     Task", fmt.Sprintf("#%d", t.ID)},
		{"   Status", t.Status},
		{"  Project", fmt.Sprintf("#%d", t.ProjectID)},
	}
	if t.WorkDir != "" {
		fields = append(fields, struct{ label, value string }{" Work dir", t.WorkDir})
	}
	if t.UpdatedAt.Valid {
		fields = append(fields, struct{ label, value string }{"  Updated", db.FormatLocal(t.UpdatedAt.String)})
	} else {
		fields = append(fields, struct{ label, value string }{"  Created", db.FormatLocal(t.CreatedAt)})
	}
	for _, f := range fields {
		lines = append(lines, fmt.Sprintf("%s  %s",
			styleFieldLabel.Render(f.label),
			styleFieldValue.Render(f.value),
		))
	}

	lines = append(lines, styleBorderChar.Render(strings.Repeat("─", bodyW)))
	lines = append(lines, styleFieldLabel.Render("status_history"))
	if len(history) == 0 {
		lines = append(lines, styleMuted.Render("  (none)"))
	} else {
		for _, h := range history {
			head := fmt.Sprintf("  %s  %s → %s",
				styleActivityTS.Render(h.At),
				StatusStyle(h.From).Render(h.From),
				StatusStyle(h.To).Render(h.To),
			)
			if h.Reason != "" {
				head += "  " + styleFieldValue.Render(h.Reason)
			}
			lines = append(lines, wrapLine(head, bodyW)...)
		}
	}

	lines = append(lines, styleBorderChar.Render(strings.Repeat("─", bodyW)))
	lines = append(lines, styleFieldLabel.Render("notes"))
	if len(notes) == 0 {
		lines = append(lines, styleMuted.Render("  (none)"))
	} else {
		for _, n := range notes {
			head := fmt.Sprintf("  %s  %s",
				styleActivityTS.Render(n.At),
				styleFieldValue.Bold(true).Render(n.Kind),
			)
			if n.Reason != "" {
				head += "  " + styleFieldValue.Render(n.Reason)
			}
			lines = append(lines, wrapLine(head, bodyW)...)
			if len(n.Metadata) > 0 {
				keys := make([]string, 0, len(n.Metadata))
				for k := range n.Metadata {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var parts []string
				for _, k := range keys {
					parts = append(parts, fmt.Sprintf("%s: %v", k, n.Metadata[k]))
				}
				meta := "    " + styleMuted.Render(strings.Join(parts, ", "))
				lines = append(lines, wrapLine(meta, bodyW)...)
			}
		}
	}

	var b strings.Builder
	b.WriteString("\n")
	headerLine := fmt.Sprintf("%s%s  %s  %s %s",
		pad,
		styleDetailBack.Render("← esc"),
		lipgloss.NewStyle().Bold(true).Render(t.Title),
		StatusStyle(t.Status).Render(StatusIcon(t.Status)),
		StatusStyle(t.Status).Render(t.Status),
	)
	b.WriteString(headerLine + "\n")
	b.WriteString(pad + styleBorderChar.Render(strings.Repeat("─", bodyW)) + "\n")

	bodyHeight := height - 4
	if bodyHeight < 1 {
		bodyHeight = 10
	}

	if scroll > len(lines)-bodyHeight {
		scroll = max(0, len(lines)-bodyHeight)
	}

	end := min(scroll+bodyHeight, len(lines))
	for _, line := range lines[scroll:end] {
		b.WriteString(pad + line + "\n")
	}

	if len(lines) > bodyHeight {
		page := scroll/bodyHeight + 1
		totalPages := (len(lines)-1)/bodyHeight + 1
		indicator := styleMuted.Render(fmt.Sprintf("[%d/%d]", page, totalPages))
		b.WriteString(pad + indicator)
	}

	return b.String()
}

// wrapLine wraps a single line to fit within maxWidth display columns.
// Uses lipgloss.Width for correct CJK/fullwidth character handling.
func wrapLine(line string, maxWidth int) []string {
	if maxWidth <= 0 || lipgloss.Width(line) <= maxWidth {
		return []string{line}
	}
	runes := []rune(line)
	var wrapped []string
	for len(runes) > 0 {
		w := 0
		breakAt := 0
		lastSpace := -1
		for i, r := range runes {
			cw := lipgloss.Width(string(r))
			if w+cw > maxWidth {
				break
			}
			w += cw
			breakAt = i + 1
			if r == ' ' {
				lastSpace = i
			}
		}
		if breakAt == 0 {
			breakAt = 1 // at least one rune
		}
		// Prefer breaking at space if we're not at the end
		if breakAt < len(runes) && lastSpace > breakAt/2 {
			breakAt = lastSpace
		}
		wrapped = append(wrapped, string(runes[:breakAt]))
		runes = runes[breakAt:]
		// Skip leading space on continuation
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return wrapped
}

// padRight pads a string to the given display width using spaces.
// Handles CJK/fullwidth characters correctly via lipgloss.Width.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// truncateDisplay truncates a string to fit within maxWidth display columns.
func truncateDisplay(s string, maxWidth int) string {
	runes := []rune(s)
	w := 0
	for i, r := range runes {
		cw := lipgloss.Width(string(r))
		if w+cw > maxWidth {
			return string(runes[:i])
		}
		w += cw
	}
	return s
}

// padWithBorder pads a line to width and appends a right border character.
func padWithBorder(line string, width int, border string) string {
	lineW := lipgloss.Width(line)
	borderW := lipgloss.Width(border)
	pad := width - lineW - borderW
	if pad > 0 {
		return line + strings.Repeat(" ", pad) + border
	}
	return line + border
}

func highlightLine(line string, width int) string {
	lineWidth := lipgloss.Width(line)
	if pad := width - lineWidth; pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return lipgloss.NewStyle().Background(colorHighlight).Render(line)
}

func StatusIcon(status string) string {
	switch status {
	case db.ActionStatusPending:
		return "○"
	case db.ActionStatusRunning:
		return "●"
	case db.ActionStatusDone:
		return "✓"
	case db.ActionStatusFailed:
		return "✗"
	case db.ActionStatusDispatched:
		return "⇢"
	case db.ActionStatusCancelled:
		return "⊘"
	case db.TaskStatusArchived:
		return "▪"
	default:
		return "?"
	}
}

// renderGaugeBar renders a proportional bar showing action status distribution.
func renderGaugeBar(running, pending, done, failed, width int) string {
	total := running + pending + done + failed
	if total == 0 || width <= 0 {
		return styleBorderChar.Render(strings.Repeat("─", width))
	}

	segments := []struct {
		count int
		style lipgloss.Style
	}{
		{running, styleGaugeRunning},
		{pending, styleGaugePending},
		{done, styleGaugeDone},
		{failed, styleGaugeFailed},
	}

	var b strings.Builder
	used := 0
	for i, seg := range segments {
		if seg.count == 0 {
			continue
		}
		w := seg.count * width / total
		if i == len(segments)-1 || used+w > width {
			w = width - used
		}
		if w > 0 {
			b.WriteString(seg.style.Render(strings.Repeat("━", w)))
			used += w
		}
	}
	for used < width {
		b.WriteString(styleBorderChar.Render("─"))
		used++
	}
	return b.String()
}

// renderStatusStrip renders the status numbers strip.
func renderStatusStrip(stats actionStats, maxInteractive, width int) string {
	parts := []string{
		styleStatNum.Foreground(colorRunning).Render(fmt.Sprintf("%d", stats.running)) + " " + styleStatLabel.Render("running"),
		styleStatNum.Foreground(colorPending).Render(fmt.Sprintf("%d", stats.pending)) + " " + styleStatLabel.Render(stats.pendingLabel),
		styleStatNum.Foreground(colorDone).Render(fmt.Sprintf("%d", stats.done)) + " " + styleStatLabel.Render("done"),
		styleStatNum.Foreground(colorFailed).Render(fmt.Sprintf("%d", stats.failed)) + " " + styleStatLabel.Render("failed"),
	}
	if maxInteractive > 0 {
		slotsColor := colorAccent
		if stats.runningInteractive >= maxInteractive {
			slotsColor = colorWarning
		}
		parts = append(parts,
			lipgloss.NewStyle().Foreground(slotsColor).Render("⚡")+
				styleStatNum.Foreground(slotsColor).Render(fmt.Sprintf("%d/%d", stats.runningInteractive, maxInteractive))+
				" "+styleStatLabel.Render("slots"))
	}
	inner := " " + strings.Join(parts, "   ")
	return styleStatusStrip.Width(width).Render(inner)
}
