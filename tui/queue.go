package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/MH4GF/tq/db"
)

type QueueModel struct {
	actions  []db.Action
	cursor   int
	width    int
	height   int
	database *db.DB
	message  string
}

func NewQueueModel(database *db.DB) QueueModel {
	return QueueModel{database: database}
}

type actionsLoadedMsg struct {
	actions []db.Action
}

type actionUpdatedMsg struct {
	id     int64
	action string
}

func (m QueueModel) loadActions() tea.Cmd {
	return func() tea.Msg {
		actions, err := m.database.ListActions("", nil)
		if err != nil {
			return actionsLoadedMsg{}
		}
		return actionsLoadedMsg{actions: actions}
	}
}

func (m QueueModel) Init() tea.Cmd {
	return m.loadActions()
}

func (m QueueModel) selectedAction() *db.Action {
	if m.cursor >= 0 && m.cursor < len(m.actions) {
		return &m.actions[m.cursor]
	}
	return nil
}

func (m QueueModel) Update(msg tea.Msg) (QueueModel, tea.Cmd) {
	switch msg := msg.(type) {
	case actionsLoadedMsg:
		m.actions = msg.actions
		if m.cursor >= len(m.actions) {
			m.cursor = max(0, len(m.actions)-1)
		}
	case actionUpdatedMsg:
		m.message = fmt.Sprintf("action #%d %s", msg.id, msg.action)
		return m, m.loadActions()
	case tea.KeyMsg:
		m.message = ""
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.actions)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			return m, m.loadActions()
		case key.Matches(msg, key.NewBinding(key.WithKeys("a"))):
			if a := m.selectedAction(); a != nil && (a.Status == "waiting_human" || a.Status == "failed") {
				return m, m.resetAction(a.ID)
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("x"))):
			if a := m.selectedAction(); a != nil && a.Status == "waiting_human" {
				return m, m.rejectAction(a.ID)
			}
		}
	}
	return m, nil
}

func (m QueueModel) resetAction(id int64) tea.Cmd {
	return func() tea.Msg {
		if err := m.database.ResetToPending(id); err != nil {
			return actionUpdatedMsg{id: id, action: "reset failed"}
		}
		return actionUpdatedMsg{id: id, action: "reset → pending"}
	}
}

func (m QueueModel) rejectAction(id int64) tea.Cmd {
	return func() tea.Msg {
		if err := m.database.MarkFailed(id, "rejected by human"); err != nil {
			return actionUpdatedMsg{id: id, action: "reject failed"}
		}
		return actionUpdatedMsg{id: id, action: "rejected → failed"}
	}
}

func (m QueueModel) View() string {
	if len(m.actions) == 0 {
		return styleMuted.Render("  No actions in queue")
	}

	var b strings.Builder
	header := fmt.Sprintf("  %-4s %-6s %-20s %-14s %-8s %s", "ID", "Status", "Template", "Source", "Pri", "Task")
	b.WriteString(styleMuted.Render(header) + "\n")
	b.WriteString(styleMuted.Render(strings.Repeat("─", min(m.width, 80))) + "\n")

	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		a := m.actions[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		icon := StatusIcon(a.Status)
		st := StatusStyle(a.Status)

		taskStr := "-"
		if a.TaskID.Valid {
			taskStr = fmt.Sprintf("#%d", a.TaskID.Int64)
		}

		line := fmt.Sprintf("%s%s %-4d %-14s %-20s %-8d %s",
			prefix,
			st.Render(icon),
			a.ID,
			st.Render(fmt.Sprintf("%-14s", a.Status)),
			a.TemplateID,
			a.Priority,
			taskStr,
		)

		if i == m.cursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(line + "\n")

		// Show result/reason for selected waiting_human action
		if i == m.cursor && a.Status == "waiting_human" && a.Result.Valid && a.Result.String != "" {
			reason := fmt.Sprintf("    %s", styleWaitingHuman.Render("reason: "+a.Result.String))
			b.WriteString(reason + "\n")
		}
	}

	if m.message != "" {
		b.WriteString("\n  " + styleDone.Render(m.message) + "\n")
	}

	return b.String()
}

type visibleRange struct {
	start, end int
}

func (m QueueModel) visibleRange() visibleRange {
	maxVisible := m.height - 4
	if maxVisible <= 0 {
		maxVisible = 20
	}
	total := len(m.actions)
	if total <= maxVisible {
		return visibleRange{0, total}
	}

	start := m.cursor - maxVisible/2
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return visibleRange{start, end}
}

func (m QueueModel) SetSize(w, h int) QueueModel {
	m.width = w
	m.height = h
	return m
}
