package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPending      = lipgloss.Color("3")  // yellow
	colorRunning      = lipgloss.Color("4")  // blue
	colorDone         = lipgloss.Color("2")  // green
	colorFailed       = lipgloss.Color("1")  // red
	colorWaitingHuman = lipgloss.Color("5")  // magenta
	colorMuted        = lipgloss.Color("8")  // gray
	colorAccent       = lipgloss.Color("14") // cyan

	stylePending      = lipgloss.NewStyle().Foreground(colorPending)
	styleRunning      = lipgloss.NewStyle().Foreground(colorRunning)
	styleDone         = lipgloss.NewStyle().Foreground(colorDone)
	styleFailed       = lipgloss.NewStyle().Foreground(colorFailed)
	styleWaitingHuman = lipgloss.NewStyle().Foreground(colorWaitingHuman)
	styleMuted        = lipgloss.NewStyle().Foreground(colorMuted)

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
	case "waiting_human":
		return styleWaitingHuman
	default:
		return styleMuted
	}
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
	case "waiting_human":
		return "!"
	default:
		return "?"
	}
}
