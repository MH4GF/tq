package tui

import (
	"context"
	"errors"
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
	tabTasks tab = iota
	tabSchedules
)

// BackgroundFunc is a function that runs in the background (queue worker, watch, etc).
// It receives a context that is cancelled when the TUI exits.
type BackgroundFunc func(ctx context.Context) error

type Model struct {
	activeTab      tab
	tasks          TasksModel
	schedules      SchedulesModel
	width          int
	height         int
	quitting       bool
	cancel         context.CancelFunc
	bgCtx          context.Context
	backgrounds    []BackgroundFunc
	statusLine     string
	statusLineGen  int
	logCh          <-chan LogEntry
	logs           []LogEntry
	maxInteractive int
}

type tickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

const transientMessageTTL = 10 * time.Second

// clearAfterTTL fires msg after transientMessageTTL. Pair with a per-field
// generation counter so an earlier timer cannot clear a newer message.
func clearAfterTTL(msg tea.Msg) tea.Cmd {
	return tea.Tick(transientMessageTTL, func(time.Time) tea.Msg {
		return msg
	})
}

type backgroundStatusMsg struct {
	err error
}

type clearStatusLineMsg struct {
	gen int
}

func New(database db.Store, logCh <-chan LogEntry, maxInteractive int, backgrounds ...BackgroundFunc) Model {
	today := time.Now().Format("2006-01-02")
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		activeTab:      tabTasks,
		tasks:          NewTasksModel(database, today),
		schedules:      NewSchedulesModel(database),
		backgrounds:    backgrounds,
		logCh:          logCh,
		cancel:         cancel,
		bgCtx:          ctx,
		maxInteractive: maxInteractive,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.tasks.Init(), m.schedules.Init(), doTick()}

	if m.logCh != nil {
		cmds = append(cmds, waitForLog(m.logCh))
	}

	for _, bg := range m.backgrounds {
		cmds = append(cmds, func() tea.Msg {
			err := bg(m.bgCtx)
			return backgroundStatusMsg{err: err}
		})
	}

	return tea.Batch(cmds...)
}

// Layout constants
const (
	headerLines    = 1 // header strip
	gaugeLine      = 1 // gauge bar
	statusLine     = 1 // status strip
	activityLines  = 3 // activity log rows
	helpLine       = 1 // help bar
	separators     = 2 // borders between sections
	layoutOverhead = headerLines + gaugeLine + statusLine + activityLines + helpLine + separators
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		contentHeight := msg.Height - layoutOverhead
		m.tasks = m.tasks.SetSize(msg.Width, contentHeight)
		m.schedules = m.schedules.SetSize(msg.Width, contentHeight)
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.tasks.loadTasks(), m.schedules.loadSchedules(), doTick())

	case logMsg:
		m.logs = append(m.logs, LogEntry(msg))
		if len(m.logs) > 100 {
			m.logs = m.logs[len(m.logs)-100:]
		}
		return m, waitForLog(m.logCh)

	case backgroundStatusMsg:
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) {
			m.statusLine = fmt.Sprintf("background error: %v", msg.err)
			m.statusLineGen++
			return m, clearAfterTTL(clearStatusLineMsg{gen: m.statusLineGen})
		}
		return m, nil

	case clearStatusLineMsg:
		if msg.gen == m.statusLineGen {
			m.statusLine = ""
		}
		return m, nil

	case tea.KeyMsg:
		// When a tab is in a sub-mode, delegate all keys to it
		if m.activeTab == tabTasks && m.tasks.Mode() != modeNormal {
			var cmd tea.Cmd
			m.tasks, cmd = m.tasks.Update(msg)
			return m, cmd
		}
		if m.activeTab == tabSchedules && m.schedules.Mode() != schedModeNormal {
			var cmd tea.Cmd
			m.schedules, cmd = m.schedules.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"))):
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			switch m.activeTab {
			case tabTasks:
				m.activeTab = tabSchedules
			default:
				m.activeTab = tabTasks
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
			m.activeTab = tabTasks
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
			m.activeTab = tabSchedules
			return m, nil
		}
	}

	// Data messages go to their owning model regardless of active tab
	switch msg.(type) {
	case tasksLoadedMsg, actionAttachedMsg, clearTasksMessageMsg:
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

	// Detail views: full-screen mode — no gauge, status strip, or activity log
	if m.activeTab == tabTasks && m.tasks.Mode() == modeViewDetail {
		detailTasks := m.tasks.SetSize(m.width, m.height-1)
		b.WriteString(detailTasks.View())
		b.WriteString(m.renderHelp())
		return b.String()
	}
	if m.activeTab == tabSchedules && m.schedules.Mode() == schedModeDetail {
		detailScheds := m.schedules.SetSize(m.width, m.height-1)
		b.WriteString(detailScheds.View())
		b.WriteString(m.renderHelp())
		return b.String()
	}

	// Header strip
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	switch m.activeTab {
	case tabTasks:
		// Gauge bar + status strip (tasks tab only)
		stats := m.tasks.actionStats()
		b.WriteString(renderGaugeBar(stats.running, stats.pending, stats.done, stats.failed, m.width))
		b.WriteString("\n")
		b.WriteString(renderStatusStrip(stats, m.maxInteractive, m.width))
		b.WriteString("\n")
		b.WriteString(m.tasks.View())
	case tabSchedules:
		b.WriteString("\n\n") // space where gauge+status would be
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

func (m Model) renderHeader() string {
	tabs := []struct {
		label string
		t     tab
	}{
		{"Tasks", tabTasks},
		{"Schedules", tabSchedules},
	}

	var tabParts []string
	for _, t := range tabs {
		if m.activeTab == t.t {
			tabParts = append(tabParts, styleTabActive.Render(" "+t.label+" "))
		} else {
			tabParts = append(tabParts, styleTabInactive.Render(" "+t.label+" "))
		}
	}
	tabStr := strings.Join(tabParts, " ")

	gap := m.width - lipgloss.Width(tabStr) - 1
	gap = max(gap, 1)

	inner := strings.Repeat(" ", gap) + tabStr
	return styleHeaderBar.Width(m.width).Render(inner)
}

func (m Model) renderHelp() string {
	var keys []HelpKey
	switch m.activeTab {
	case tabTasks:
		keys = m.tasks.HelpKeys()
	case tabSchedules:
		keys = m.schedules.HelpKeys()
	}
	inner := "  " + formatHelp(keys)
	return "\n" + styleHelpBar.Width(m.width).Render(inner)
}

func (m Model) renderActivity() string {
	var b strings.Builder
	b.WriteString(styleBorderChar.Render(strings.Repeat("─", m.width)) + "\n")

	start := 0
	if len(m.logs) > activityLines {
		start = len(m.logs) - activityLines
	}
	shown := m.logs[start:]
	for _, e := range shown {
		ts := e.Time.Format("15:04")
		b.WriteString(styleActivityTS.Render("  "+ts+" ") + styleActivityMsg.Render(e.Message) + "\n")
	}
	return b.String()
}

func (m Model) ActiveTab() tab {
	return m.activeTab
}

func (m Model) Tasks() TasksModel {
	return m.tasks
}

func (m Model) IsQuitting() bool {
	return m.quitting
}
