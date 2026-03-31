package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/MH4GF/tq/db"
)

// ── Color palette (256-color) ──

var (
	// Surface / background
	colorSurface   = lipgloss.Color("234") // panels, header, help bar
	colorHighlight = lipgloss.Color("236") // cursor row background
	colorBorder    = lipgloss.Color("235") // card borders

	// Text hierarchy
	colorTextSecondary = lipgloss.Color("245") // secondary
	colorTextMuted     = lipgloss.Color("239") // dimmed

	// Semantic status
	colorRunning = lipgloss.Color("75")  // #5fafff blue
	colorPending = lipgloss.Color("179") // #d7af5f amber
	colorDone    = lipgloss.Color("114") // #87d787 green
	colorFailed  = lipgloss.Color("204") // #ff5f87 red-pink
	colorAccent  = lipgloss.Color("80")  // #5fd7d7 teal
	colorWarning = lipgloss.Color("213") // magenta

	colorDoneDim   = lipgloss.Color("236") // very dim green for done actions
	colorFailedDim = lipgloss.Color("95")  // dim red-ish for failed actions
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
)

// ── UI component styles ──

var (
	// Header / tabs
	styleHeaderBar   = lipgloss.NewStyle().Background(colorSurface)
	styleBrand       = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
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
	styleWorkDir     = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	// Tree text
	styleHelp = lipgloss.NewStyle().Foreground(colorTextMuted)

	// Help bar
	styleHelpBar = lipgloss.NewStyle().Background(colorSurface)

	// Activity
	styleActivityTS  = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	styleActivityMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))

	// Detail view
	styleDetailHeader = lipgloss.NewStyle().Background(colorSurface).Foreground(colorAccent).Bold(true)
	styleDetailBack   = lipgloss.NewStyle().Foreground(colorTextMuted)
	styleFieldLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	styleFieldValue   = lipgloss.NewStyle().Foreground(colorTextSecondary)

	// Gauge bar segments
	styleGaugeRunning = lipgloss.NewStyle().Foreground(colorRunning)
	styleGaugePending = lipgloss.NewStyle().Foreground(colorPending)
	styleGaugeDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("22")) // dark green
	styleGaugeFailed  = lipgloss.NewStyle().Foreground(colorFailed)

	// Schedule table
	styleTableHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
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
	maxW := min(width, 80)

	// Header strip: ← esc  title  [status]
	st := StatusStyle(a.Status)
	headerLine := fmt.Sprintf(" %s  %s  %s",
		styleDetailBack.Render("← esc"),
		styleDetailHeader.Render(a.Title),
		st.Render(a.Status),
	)
	b.WriteString(headerLine + "\n")
	b.WriteString(styleBorderChar.Render(strings.Repeat("─", maxW)) + "\n")

	// Metadata grid
	fields := []struct{ label, value string }{
		{"   Action", fmt.Sprintf("#%d", a.ID)},
		{"     Task", fmt.Sprintf("#%d", a.TaskID)},
	}
	if a.CompletedAt.Valid {
		fields = append(fields, struct{ label, value string }{"Completed", db.FormatLocal(a.CompletedAt.String)})
	}
	for _, f := range fields {
		fmt.Fprintf(&b, "  %s  %s\n",
			styleFieldLabel.Render(f.label),
			styleFieldValue.Render(f.value),
		)
	}
	b.WriteString(styleBorderChar.Render(strings.Repeat("─", maxW)) + "\n")

	// Result body
	result := ""
	if a.Result.Valid {
		result = a.Result.String
	}
	lines := strings.Split(result, "\n")

	headerLines := 7 + len(fields)
	bodyHeight := height - headerLines
	if bodyHeight < 1 {
		bodyHeight = 10
	}

	if scroll > len(lines)-bodyHeight {
		scroll = max(0, len(lines)-bodyHeight)
	}

	end := min(scroll+bodyHeight, len(lines))
	for _, line := range lines[scroll:end] {
		b.WriteString("  " + line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("  " + formatHelp(detailHelpKeys())))
	return b.String()
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
func renderStatusStrip(running, pending, done, failed int, pendingLabel string, width int) string {
	parts := []string{
		styleStatNum.Foreground(colorRunning).Render(fmt.Sprintf("%d", running)) + " " + styleStatLabel.Render("running"),
		styleStatNum.Foreground(colorPending).Render(fmt.Sprintf("%d", pending)) + " " + styleStatLabel.Render(pendingLabel),
		styleStatNum.Foreground(colorDone).Render(fmt.Sprintf("%d", done)) + " " + styleStatLabel.Render("done"),
		styleStatNum.Foreground(colorFailed).Render(fmt.Sprintf("%d", failed)) + " " + styleStatLabel.Render("failed"),
	}
	inner := " " + strings.Join(parts, "   ")
	return styleStatusStrip.Width(width).Render(inner)
}
