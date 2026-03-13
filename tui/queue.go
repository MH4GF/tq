package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/MH4GF/tq/db"
)

type QueueModel struct {
	actions      []db.Action
	cursor       int
	width        int
	height       int
	database     *db.DB
	message      string
	dateFilter   string
	detailAction *db.Action
	detailScroll int
}

func NewQueueModel(database *db.DB, dateFilter string) QueueModel {
	return QueueModel{database: database, dateFilter: dateFilter}
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
		return actionsLoadedMsg{actions: db.FilterByDate(actions, m.dateFilter)}
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

func (m QueueModel) InDetailView() bool {
	return m.detailAction != nil
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
		if m.detailAction != nil {
			return m.updateDetailView(msg), nil
		}
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
		case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
			if a := m.selectedAction(); a != nil && a.SessionID.Valid {
				return m, m.attachAction(a)
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("v"))):
			if a := m.selectedAction(); a != nil && a.Result.Valid && a.Result.String != "" {
				m.detailAction = a
				m.detailScroll = 0
			}
		}
	}
	return m, nil
}

func (m QueueModel) updateDetailView(msg tea.KeyMsg) QueueModel {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q"))):
		m.detailAction = nil
		m.detailScroll = 0
	case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
		m.detailScroll++
	case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	}
	return m
}

func (m QueueModel) attachAction(a *db.Action) tea.Cmd {
	return func() tea.Msg {
		target := fmt.Sprintf("%s:%s", a.SessionID.String, a.TmuxPane.String)
		if err := exec.Command("tmux", "select-window", "-t", target).Run(); err != nil {
			return actionUpdatedMsg{id: a.ID, action: fmt.Sprintf("attach failed: %v", err)}
		}
		return nil
	}
}

func (m QueueModel) View() string {
	if m.detailAction != nil {
		return RenderDetailView(m.detailAction, m.detailScroll, m.width, m.height)
	}

	if len(m.actions) == 0 {
		return styleMuted.Render("  No actions in queue")
	}

	var b strings.Builder
	header := fmt.Sprintf("  %-4s %-6s %-20s %s", "ID", "Status", "Title", "Task")
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

		taskStr := fmt.Sprintf("#%d", a.TaskID)

		line := fmt.Sprintf("%s%s %-4d %-14s %-20s %s",
			prefix,
			st.Render(icon),
			a.ID,
			st.Render(fmt.Sprintf("%-14s", a.Status)),
			a.Title,
			taskStr,
		)

		if i == m.cursor {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}

		if i == m.cursor && a.Result.Valid && a.Result.String != "" {
			label := "result"
			rst := StatusStyle(a.Status)
			lineWidth := lipgloss.Width(line)
			pad := 2
			labelLen := len(label) + 2 // "label: "
			remaining := m.width - lineWidth - pad - labelLen
			if remaining > 10 {
				line += "  " + rst.Render(label+": "+truncateResult(a.Result.String, remaining))
			}
		}

		b.WriteString(line + "\n")
	}

	if m.message != "" {
		b.WriteString("\n  " + styleDone.Render(m.message) + "\n")
	}

	return b.String()
}

func (m QueueModel) visibleRange() visibleRange {
	return calcVisibleRange(m.cursor, len(m.actions), m.height, 4)
}

func (m QueueModel) HelpKeys() []HelpKey {
	if m.detailAction != nil {
		return detailHelpKeys()
	}
	keys := commonHelpKeys()
	if a := m.selectedAction(); a != nil {
		if a.SessionID.Valid {
			keys = append(keys, HelpKey{"o", "attach"})
		}
		if a.Result.Valid && a.Result.String != "" {
			keys = append(keys, HelpKey{"v", "view result"})
		}
	}
	return keys
}

func (m QueueModel) SetSize(w, h int) QueueModel {
	m.width = w
	m.height = h
	return m
}
