package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/MH4GF/tq/db"
)

var (
	colorPending      = lipgloss.Color("3")  // yellow
	colorRunning      = lipgloss.Color("4")  // blue
	colorDone         = lipgloss.Color("2")  // green
	colorFailed       = lipgloss.Color("1")  // red
	colorWarning = lipgloss.Color("5")  // magenta
	colorMuted   = lipgloss.Color("8")  // gray
	colorAccent       = lipgloss.Color("14") // cyan

	stylePending      = lipgloss.NewStyle().Foreground(colorPending)
	styleRunning      = lipgloss.NewStyle().Foreground(colorRunning)
	styleDone         = lipgloss.NewStyle().Foreground(colorDone)
	styleFailed       = lipgloss.NewStyle().Foreground(colorFailed)
	styleWarning = lipgloss.NewStyle().Foreground(colorWarning)
	styleMuted   = lipgloss.NewStyle().Foreground(colorMuted)

	styleTabActive   = lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Underline(true)
	styleTabInactive = lipgloss.NewStyle().Foreground(colorMuted)

	styleTitle   = lipgloss.NewStyle().Bold(true)
	styleHelp    = lipgloss.NewStyle().Foreground(colorMuted)
	styleProject = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
)

func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "pending":
		return stylePending
	case "running":
		return styleRunning
	case "done":
		return styleDone
	case "failed":
		return styleFailed
	default:
		return styleMuted
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

	b.WriteString(styleTitle.Render("  Action Detail") + "\n")
	b.WriteString(styleMuted.Render(strings.Repeat("─", min(width, 80))) + "\n")

	st := StatusStyle(a.Status)
	b.WriteString(fmt.Sprintf("  ID:        %d\n", a.ID))
	b.WriteString(fmt.Sprintf("  Status:    %s\n", st.Render(a.Status)))
	b.WriteString(fmt.Sprintf("  Title:     %s\n", a.Title))
	b.WriteString(fmt.Sprintf("  Prompt:    %s\n", a.PromptID))
	b.WriteString(fmt.Sprintf("  Task:      #%d\n", a.TaskID))
	if a.CompletedAt.Valid {
		b.WriteString(fmt.Sprintf("  Completed: %s\n", a.CompletedAt.String))
	}
	b.WriteString(styleMuted.Render(strings.Repeat("─", min(width, 80))) + "\n")

	result := ""
	if a.Result.Valid {
		result = a.Result.String
	}
	lines := strings.Split(result, "\n")

	headerLines := 9
	bodyHeight := height - headerLines
	if bodyHeight < 1 {
		bodyHeight = 10
	}

	if scroll > len(lines)-bodyHeight {
		scroll = max(0, len(lines)-bodyHeight)
	}

	end := scroll + bodyHeight
	if end > len(lines) {
		end = len(lines)
	}

	for _, line := range lines[scroll:end] {
		b.WriteString("  " + line + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("  j/k: scroll  q: back"))
	return b.String()
}

func StatusIcon(status string) string {
	switch status {
	case "pending":
		return "○"
	case "running":
		return "●"
	case "done":
		return "✓"
	case "failed":
		return "✗"
	default:
		return "?"
	}
}
