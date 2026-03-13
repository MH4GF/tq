package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/MH4GF/tq/db"
)

type tab int

const (
	tabQueue tab = iota
	tabTasks
	tabSchedules
)

// BackgroundFunc is a function that runs in the background (ralph loop, watch, etc).
// It receives a context that is cancelled when the TUI exits.
type BackgroundFunc func(ctx context.Context) error

type Model struct {
	activeTab   tab
	queue       QueueModel
	tasks       TasksModel
	schedules   SchedulesModel
	width       int
	height      int
	quitting    bool
	cancel      context.CancelFunc
	backgrounds []BackgroundFunc
	statusLine  string
	logCh       <-chan LogEntry
	logs        []LogEntry
}

type tickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type backgroundStatusMsg struct {
	name string
	err  error
}

func New(database *db.DB, logCh <-chan LogEntry, backgrounds ...BackgroundFunc) Model {
	today := time.Now().Format("2006-01-02")
	return Model{
		activeTab:   tabQueue,
		queue:       NewQueueModel(database, today),
		tasks:       NewTasksModel(database, today),
		schedules:   NewSchedulesModel(database),
		backgrounds: backgrounds,
		logCh:       logCh,
	}
}

func (m Model) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	cmds := []tea.Cmd{m.queue.Init(), m.tasks.Init(), m.schedules.Init(), doTick()}

	if m.logCh != nil {
		cmds = append(cmds, waitForLog(m.logCh))
	}

	for _, bg := range m.backgrounds {
		bg := bg
		cmds = append(cmds, func() tea.Msg {
			err := bg(ctx)
			return backgroundStatusMsg{err: err}
		})
	}

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := msg.Height - 14
		m.queue = m.queue.SetSize(msg.Width, contentHeight)
		m.tasks = m.tasks.SetSize(msg.Width, contentHeight)
		m.schedules = m.schedules.SetSize(msg.Width, contentHeight)
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.queue.loadActions(), m.tasks.loadTasks(), m.schedules.loadSchedules(), doTick())

	case logMsg:
		m.logs = append(m.logs, LogEntry(msg))
		if len(m.logs) > 100 {
			m.logs = m.logs[len(m.logs)-100:]
		}
		return m, waitForLog(m.logCh)

	case backgroundStatusMsg:
		if msg.err != nil && msg.err != context.Canceled {
			m.statusLine = fmt.Sprintf("background error: %v", msg.err)
		}
		return m, nil

	case tea.KeyMsg:
		// When queue tab is in detail view, delegate all keys to it
		if m.activeTab == tabQueue && m.queue.InDetailView() {
			var cmd tea.Cmd
			m.queue, cmd = m.queue.Update(msg)
			return m, cmd
		}
		// When tasks tab is in a sub-mode, delegate all keys to it
		if m.activeTab == tabTasks && m.tasks.Mode() != modeNormal {
			var cmd tea.Cmd
			m.tasks, cmd = m.tasks.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			switch m.activeTab {
			case tabQueue:
				m.activeTab = tabTasks
			case tabTasks:
				m.activeTab = tabSchedules
			default:
				m.activeTab = tabQueue
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
			m.activeTab = tabQueue
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
			m.activeTab = tabTasks
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
			m.activeTab = tabSchedules
			return m, nil
		}
	}

	// Data messages go to their owning model regardless of active tab
	switch msg.(type) {
	case actionsLoadedMsg, actionUpdatedMsg:
		var cmd tea.Cmd
		m.queue, cmd = m.queue.Update(msg)
		return m, cmd
	case tasksLoadedMsg:
		var cmd tea.Cmd
		m.tasks, cmd = m.tasks.Update(msg)
		return m, cmd
	case schedulesLoadedMsg:
		var cmd tea.Cmd
		m.schedules, cmd = m.schedules.Update(msg)
		return m, cmd
	}

	// Key messages go to the active tab only
	var cmd tea.Cmd
	switch m.activeTab {
	case tabQueue:
		m.queue, cmd = m.queue.Update(msg)
	case tabTasks:
		m.tasks, cmd = m.tasks.Update(msg)
	case tabSchedules:
		m.schedules, cmd = m.schedules.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	b.WriteString(m.renderTabs())
	b.WriteString("\n\n")

	switch m.activeTab {
	case tabQueue:
		b.WriteString(m.queue.View())
	case tabTasks:
		b.WriteString(m.tasks.View())
	case tabSchedules:
		b.WriteString(m.schedules.View())
	}

	b.WriteString("\n")
	b.WriteString(m.renderActivity())
	if m.statusLine != "" {
		b.WriteString(styleWarning.Render(m.statusLine) + "\n")
	}
	b.WriteString(m.renderHelp())

	return b.String()
}

func (m Model) renderTabs() string {
	tabs := []struct {
		label string
		key   string
		t     tab
	}{
		{"Queue", "1", tabQueue},
		{"Tasks", "2", tabTasks},
		{"Schedules", "3", tabSchedules},
	}

	var parts []string
	for _, t := range tabs {
		label := fmt.Sprintf("[%s] %s", t.key, t.label)
		if m.activeTab == t.t {
			parts = append(parts, styleTabActive.Render(label))
		} else {
			parts = append(parts, styleTabInactive.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(parts, "  "))
}

func (m Model) renderHelp() string {
	var keys []HelpKey
	switch m.activeTab {
	case tabQueue:
		keys = m.queue.HelpKeys()
	case tabTasks:
		keys = m.tasks.HelpKeys()
	case tabSchedules:
		keys = m.schedules.HelpKeys()
	}
	return styleHelp.Render(formatHelp(keys))
}

func (m Model) renderActivity() string {
	var b strings.Builder
	b.WriteString(styleMuted.Render("── Activity ──────────────────────") + "\n")

	start := 0
	if len(m.logs) > 9 {
		start = len(m.logs) - 9
	}
	shown := m.logs[start:]
	for _, e := range shown {
		ts := e.Time.Format("15:04")
		b.WriteString(styleMuted.Render("  "+ts+" ") + e.Message + "\n")
	}
	for i := len(shown); i < 9; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) ActiveTab() tab {
	return m.activeTab
}

func (m Model) Queue() QueueModel {
	return m.queue
}

func (m Model) Tasks() TasksModel {
	return m.tasks
}

func (m Model) IsQuitting() bool {
	return m.quitting
}
